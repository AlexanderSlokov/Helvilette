// Agent configuration: where each value came from, and how the sources layer.
// An explicit YAML file outranks the ambient environment, which outranks the
// defaults, and every resolved value records its origin so a node can explain
// itself from its own logs. See ADR-0001.
//
// Split out of main.go, which exceeded the 500-line ceiling in AGENTS.md.
// See BACKLOG 6.7. Since ADR-0008 the agent names the repository it pulls
// (repoURL, branch) instead of an Othela URL; othelaURL, OTHELA_URL and
// --othela-url were removed outright, with no deprecation, because nobody
// used them yet (issue #43).
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

// ConfigProvenance records which source supplied each configuration value, keyed by the
// field's YAML name; individual labels are keyed as "labels.<key>". It lets an operator
// see why a node is configured the way it is without re-deriving the precedence rules
// against the machine's state. See docs/informations/ADRs/ADR-0001.md.
type ConfigProvenance map[string]string

// fallbackNodeID is used only when the hostname cannot be determined. It is deliberately
// not a plausible-looking node name, because a static default means every node that
// reaches it registers under the same identity.
const fallbackNodeID = "agent-unknown"

// defaultNodeID prefers the machine hostname so that agents stay distinguishable even
// when nodeID is never configured.
func defaultNodeID() (id string, fromHostname bool) {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h, true
	}
	return fallbackNodeID, false
}

// defaultWorkspaceDir matches StateDirectory=helvilette/agent in the unit. It was
// /tmp/helvilette, which a reboot may wipe together with last_run_summary.json,
// so a power cut mid-apply left no trace of the interrupted run. Issue #43.
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

// LoadConfig merges defaults, the environment, the YAML file and the CLI flags,
// in rising precedence. The returned ConfigProvenance records which source won
// each value, so the agent can report it at startup.
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

// requireRepoURL refuses to start an agent that has nothing to pull. It runs
// after --print-config, so an operator can still inspect an incomplete setup.
func requireRepoURL(config AgentConfiguration) error {
	if config.RepoURL != "" {
		return nil
	}
	return fmt.Errorf("repoURL is not set, so the agent has nothing to pull and did not start. " +
		"Set repoURL in agent.yaml, REPO_URL, or --repo-url, for example https://git.example.com/org/playbooks.git")
}

// fileConfigLayer reads the YAML config file. Unknown keys are errors, so a
// misspelt key, or the removed othelaURL, fails at startup instead of leaving
// the agent on defaults.
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

// rawConfigFile is agent.yaml as written: every value a string, parsed when
// applied, so a bad duration names the source it came from.
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
