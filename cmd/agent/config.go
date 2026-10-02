// Agent configuration: where each value came from, and how the sources layer.
// An explicit YAML file outranks the ambient environment, which outranks the
// defaults, and every resolved value records its origin so a node can explain
// itself from its own logs. See ADR-0001.
//
// Split out of main.go, which exceeded the 500-line ceiling in AGENTS.md.
// See BACKLOG 6.7.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"helvilette/pkg/log"
)

// AgentConfiguration holds the full configuration for the agent,
// modeled after Kubernetes KubeletConfiguration
type AgentConfiguration struct {
	OthelaURL    string            `yaml:"othelaURL"`
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

// DefaultConfig returns the default agent configuration
func DefaultConfig() AgentConfiguration {
	nodeID, _ := defaultNodeID()
	return AgentConfiguration{
		OthelaURL:    "http://localhost:8080/api/v1",
		NodeID:       nodeID,
		PollInterval: 5 * time.Second,
		WorkspaceDir: "/tmp/helvilette",
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

	_, nodeIDFromHostname := defaultNodeID()
	nodeIDDefaultSource := SourceDefault
	if nodeIDFromHostname {
		nodeIDDefaultSource = SourceDefaultHostname
	}

	provenance := ConfigProvenance{
		"othelaURL":    SourceDefault,
		"nodeID":       nodeIDDefaultSource,
		"pollInterval": SourceDefault,
		"workspaceDir": SourceDefault,
	}
	return config, provenance
}

func overrideFromEnv(config *AgentConfiguration, provenance ConfigProvenance) {
	if url := os.Getenv("OTHELA_URL"); url != "" {
		config.OthelaURL = url
		provenance["othelaURL"] = sourceEnv("OTHELA_URL")
	}
	if nodeID := os.Getenv("NODE_ID"); nodeID != "" {
		config.NodeID = nodeID
		provenance["nodeID"] = sourceEnv("NODE_ID")
	}
	if interval := os.Getenv("POLL_INTERVAL"); interval != "" {
		if parsed, err := time.ParseDuration(interval); err == nil {
			config.PollInterval = parsed
			provenance["pollInterval"] = sourceEnv("POLL_INTERVAL")
		}
	}
	if dir := os.Getenv("WORKSPACE_DIR"); dir != "" {
		config.WorkspaceDir = dir
		provenance["workspaceDir"] = sourceEnv("WORKSPACE_DIR")
	}
	if labelsStr := os.Getenv("AGENT_LABELS"); labelsStr != "" {
		envLabels := parseLabels(labelsStr)
		for k, v := range envLabels {
			config.Labels[k] = v
			provenance[labelsPrefix+k] = sourceEnv("AGENT_LABELS")
		}
	}
}

func overrideFromFile(configPath string, config *AgentConfiguration, provenance ConfigProvenance) error {
	if configPath == "" {
		return nil
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var raw struct {
		OthelaURL    string            `yaml:"othelaURL"`
		NodeID       string            `yaml:"nodeID"`
		PollInterval string            `yaml:"pollInterval"`
		WorkspaceDir string            `yaml:"workspaceDir"`
		Labels       map[string]string `yaml:"labels"`
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil && err != io.EOF {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	if raw.OthelaURL != "" {
		config.OthelaURL = raw.OthelaURL
		provenance["othelaURL"] = SourceConfigFile
	}
	if raw.NodeID != "" {
		config.NodeID = raw.NodeID
		provenance["nodeID"] = SourceConfigFile
	}
	if raw.WorkspaceDir != "" {
		config.WorkspaceDir = raw.WorkspaceDir
		provenance["workspaceDir"] = SourceConfigFile
	}
	if raw.PollInterval != "" {
		d, err := time.ParseDuration(raw.PollInterval)
		if err != nil {
			return fmt.Errorf("invalid pollInterval in config file: %w", err)
		}
		config.PollInterval = d
		provenance["pollInterval"] = SourceConfigFile
	}
	for k, v := range raw.Labels {
		config.Labels[k] = v
		provenance[labelsPrefix+k] = SourceConfigFile
	}
	return nil
}

func overrideFromCLI(cliOthelaURL, cliNodeID, cliPollInterval, cliWorkspaceDir, cliLabels string, config *AgentConfiguration, provenance ConfigProvenance) {
	if cliOthelaURL != "" {
		config.OthelaURL = cliOthelaURL
		provenance["othelaURL"] = sourceCLI("othela-url")
	}
	if cliNodeID != "" {
		config.NodeID = cliNodeID
		provenance["nodeID"] = sourceCLI("node-id")
	}
	if cliPollInterval != "" {
		if parsed, err := time.ParseDuration(cliPollInterval); err == nil {
			config.PollInterval = parsed
			provenance["pollInterval"] = sourceCLI("poll-interval")
		}
	}
	if cliWorkspaceDir != "" {
		config.WorkspaceDir = cliWorkspaceDir
		provenance["workspaceDir"] = sourceCLI("workspace-dir")
	}
	if cliLabels != "" {
		cliParsedLabels := parseLabels(cliLabels)
		for k, v := range cliParsedLabels {
			config.Labels[k] = v
			provenance[labelsPrefix+k] = sourceCLI("labels")
		}
	}
}

func formatOthelaURL(url string) string {
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}
	if len(url) > 0 && url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}
	if len(url) < 7 || url[len(url)-7:] != "/api/v1" {
		url = url + "/api/v1"
	}
	return url
}

// LoadConfig merges default, yaml file, environment, and CLI configurations. The returned
// ConfigProvenance records which source won each value, so the agent can report it at startup.
func LoadConfig(configPath, cliOthelaURL, cliNodeID, cliPollInterval, cliWorkspaceDir, cliLabels string) (AgentConfiguration, ConfigProvenance, error) {
	config, provenance := initDefaultConfig()

	overrideFromEnv(&config, provenance)

	if err := overrideFromFile(configPath, &config, provenance); err != nil {
		return config, provenance, err
	}

	overrideFromCLI(cliOthelaURL, cliNodeID, cliPollInterval, cliWorkspaceDir, cliLabels, &config, provenance)

	config.OthelaURL = formatOthelaURL(config.OthelaURL)

	return config, provenance, nil
}

// configFields returns the resolved configuration as ordered field/value pairs, including
// one entry per label. Ordering is stable so that operators comparing two nodes, or two
// runs on the same node, can diff the output directly.
func configFields(config AgentConfiguration) [][2]string {
	fields := [][2]string{
		{"othelaURL", config.OthelaURL},
		{"nodeID", config.NodeID},
		{"pollInterval", config.PollInterval.String()},
		{"workspaceDir", config.WorkspaceDir},
	}

	labelKeys := make([]string, 0, len(config.Labels))
	for k := range config.Labels {
		labelKeys = append(labelKeys, k)
	}
	sort.Strings(labelKeys)
	for _, k := range labelKeys {
		fields = append(fields, [2]string{labelsPrefix + k, config.Labels[k]})
	}

	return fields
}

// FormatConfig renders the resolved configuration and the source of each value as an
// aligned block, for humans reading `--print-config`.
func FormatConfig(config AgentConfiguration, provenance ConfigProvenance) string {
	fields := configFields(config)

	nameWidth, valueWidth := 0, 0
	for _, f := range fields {
		if len(f[0]) > nameWidth {
			nameWidth = len(f[0])
		}
		if len(f[1]) > valueWidth {
			valueWidth = len(f[1])
		}
	}

	var b strings.Builder
	for _, f := range fields {
		source := provenance[f[0]]
		if source == "" {
			source = SourceDefault
		}
		fmt.Fprintf(&b, "%-*s = %-*s  source=%s\n", nameWidth, f[0], valueWidth, f[1], source)
	}
	return b.String()
}

// logEffectiveConfig reports the resolved configuration and where each value came from,
// so that a node's behaviour can be explained from its logs alone rather than by
// re-deriving the precedence rules against systemd units and container environments.
func logEffectiveConfig(config AgentConfiguration, provenance ConfigProvenance) {
	values := zerolog.Dict()
	sources := zerolog.Dict()
	for _, f := range configFields(config) {
		source := provenance[f[0]]
		if source == "" {
			source = SourceDefault
		}
		values = values.Str(f[0], f[1])
		sources = sources.Str(f[0], source)
	}

	log.Info().
		Dict("config", values).
		Dict("configSources", sources).
		Msg("effective configuration")

	if config.NodeID == fallbackNodeID {
		log.Warn().
			Str("nodeID", config.NodeID).
			Msg("nodeID is unset and the hostname could not be determined; set nodeID explicitly to keep this node distinguishable from others in the fleet")
	}
}
