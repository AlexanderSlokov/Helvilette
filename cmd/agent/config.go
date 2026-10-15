// Agent configuration.
//
// Each value can come from four places. When two set the same value, the
// higher one wins:
//
//	--flag  >  agent.yaml  >  environment variable  >  built-in default
//
// The agent remembers which place each value came from, and prints it with
// --print-config and in its first log line, so an operator can see why a node
// behaves the way it does.
//
// The agent needs no control plane. It is told one Git repository (repoURL,
// branch) and pulls it itself.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// AgentConfiguration holds the full configuration for the agent,
// modeled after Kubernetes KubeletConfiguration
type AgentConfiguration struct {
	// RepoURL is the one repository this node pulls, like ansible-pull -U.
	RepoURL string `yaml:"repoURL"`
	// Branch is the ref to check out; empty means the remote's default branch.
	Branch       string            `yaml:"branch"`
	NodeID       string            `yaml:"nodeID"`
	PollInterval time.Duration     `yaml:"pollInterval"`
	WorkspaceDir string            `yaml:"workspaceDir"`
	Labels       map[string]string `yaml:"labels"`
}

// Names of the sources a configuration value can come from, as reported in
// ConfigProvenance. Env and CLI sources name the specific variable or flag.
const (
	SourceDefault         = "default"
	SourceDefaultHostname = "default(hostname)"
	SourceConfigFile      = "config-file"
)

const labelsPrefix = "labels."

func sourceEnv(name string) string { return "env(" + name + ")" }

func sourceCLI(flag string) string { return "cli(--" + flag + ")" }

// ConfigProvenance maps each setting to the place its value came from, for
// example "repoURL" -> "config-file" or "labels.role" -> "cli(--labels)".
type ConfigProvenance map[string]string

// fallbackNodeID is used only when the hostname cannot be read. It looks wrong on
// purpose: every node that falls back shares it, and an operator should notice.
const fallbackNodeID = "agent-unknown"

// defaultNodeID uses the hostname, so nodes stay distinguishable when nodeID is
// never set.
func defaultNodeID() (id string, fromHostname bool) {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h, true
	}
	return fallbackNodeID, false
}

// defaultWorkspaceDir holds the agent's state, including last_run_summary.json,
// the file that tells the agent after a crash that a run was cut off. It must
// survive a reboot, so it is under /var/lib, never /tmp. The systemd unit
// creates it (StateDirectory=helvilette/agent).
const defaultWorkspaceDir = "/var/lib/helvilette/agent"

// DefaultConfig returns the default agent configuration
func DefaultConfig() AgentConfiguration {
	nodeID, _ := defaultNodeID()
	return AgentConfiguration{
		NodeID:       nodeID,
		PollInterval: 5 * time.Second,
		WorkspaceDir: defaultWorkspaceDir,
		Labels:       make(map[string]string),
	}
}

func parseLabels(s string) map[string]string {
	labels := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			labels[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return labels
}

func initDefaultConfig() (AgentConfiguration, ConfigProvenance) {
	config := DefaultConfig()
	provenance := ConfigProvenance{}
	for _, s := range configSettings {
		provenance[s.key] = SourceDefault
	}
	if _, fromHostname := defaultNodeID(); fromHostname {
		provenance["nodeID"] = SourceDefaultHostname
	}
	return config, provenance
}

// LoadConfig builds the configuration from all four places, in the order shown
// at the top of this file, and says where each value came from.
//
//	cfg, provenance, err := LoadConfig(cliFlags{configFile: "/etc/helvilette/agent.yaml"})
func LoadConfig(cli cliFlags) (AgentConfiguration, ConfigProvenance, error) {
	config, provenance := initDefaultConfig()

	fileLayer, err := fileConfigLayer(cli.configFile)
	if err != nil {
		return config, provenance, err
	}
	for _, layer := range []configLayer{envConfigLayer(), fileLayer, cliConfigLayer(cli)} {
		if err := layer.applyTo(&config, provenance); err != nil {
			return config, provenance, err
		}
	}
	return config, provenance, nil
}

// requireRepoURL stops an agent that has no repository to pull. It is checked
// after --print-config, so an operator can still print an incomplete setup.
func requireRepoURL(config AgentConfiguration) error {
	if config.RepoURL != "" {
		return nil
	}
	return fmt.Errorf("repoURL is not set, so the agent has nothing to pull and did not start. " +
		"Set repoURL in agent.yaml, REPO_URL, or --repo-url, for example https://git.example.com/org/playbooks.git")
}

// fileConfigLayer reads agent.yaml. An unknown key stops the agent, so a typo
// fails loudly instead of the agent quietly running on a default. This also
// rejects othelaURL, which the agent no longer has.
func fileConfigLayer(path string) (configLayer, error) {
	layer := configLayer{kind: fromFile, values: map[string]string{}}
	if path == "" {
		return layer, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return layer, fmt.Errorf("failed to read config file: %w", err)
	}
	var raw rawConfigFile
	dec := yaml.NewDecoder(bytes.NewReader(content))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil && err != io.EOF {
		return layer, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}
	return raw.layer(), nil
}

// rawConfigFile is agent.yaml as written. Values stay strings here and are
// parsed later, so a bad pollInterval error can say it came from the file.
type rawConfigFile struct {
	RepoURL      string            `yaml:"repoURL"`
	Branch       string            `yaml:"branch"`
	NodeID       string            `yaml:"nodeID"`
	PollInterval string            `yaml:"pollInterval"`
	WorkspaceDir string            `yaml:"workspaceDir"`
	Labels       map[string]string `yaml:"labels"`
}

func (r rawConfigFile) layer() configLayer {
	return configLayer{
		kind: fromFile,
		values: map[string]string{
			"repoURL": r.RepoURL, "branch": r.Branch, "nodeID": r.NodeID,
			"pollInterval": r.PollInterval, "workspaceDir": r.WorkspaceDir,
		},
		labels: r.Labels,
	}
}
