// Where each setting is read from. configSettings lists every setting once,
// with its name in agent.yaml, as an environment variable and as a flag. Adding
// a setting means adding one line there; all three places pick it up.
package main

import (
	"fmt"
	"os"
	"time"
)

// configSetting is one scalar setting and its name in every source.
type configSetting struct {
	key    string // YAML key, also the ConfigProvenance key
	envVar string
	flag   string
	set    func(*AgentConfiguration, string) error
}

func setString(field func(*AgentConfiguration) *string) func(*AgentConfiguration, string) error {
	return func(c *AgentConfiguration, v string) error {
		*field(c) = v
		return nil
	}
}

// setPollInterval fails on a value that is not a duration, wherever it came
// from. Ignoring it would leave the agent on the default while the operator
// believes their value is in effect.
func setPollInterval(c *AgentConfiguration, v string) error {
	d, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("pollInterval %q is not a duration, expected a value such as %q: %w", v, "30s", err)
	}
	c.PollInterval = d
	return nil
}

var configSettings = []configSetting{
	{"repoURL", "REPO_URL", "repo-url", setString(func(c *AgentConfiguration) *string { return &c.RepoURL })},
	{"branch", "REPO_BRANCH", "branch", setString(func(c *AgentConfiguration) *string { return &c.Branch })},
	{"nodeID", "NODE_ID", "node-id", setString(func(c *AgentConfiguration) *string { return &c.NodeID })},
	{"pollInterval", "POLL_INTERVAL", "poll-interval", setPollInterval},
	{"workspaceDir", "WORKSPACE_DIR", "workspace-dir", setString(func(c *AgentConfiguration) *string { return &c.WorkspaceDir })},
}

// labelsSetting only names where labels come from. Labels are not replaced as
// a whole: each key is set separately, so env role=web plus flag env=prod
// gives both.
var labelsSetting = configSetting{key: "labels", envVar: "AGENT_LABELS", flag: "labels"}

type layerKind int

const (
	fromEnv layerKind = iota
	fromFile
	fromCLI
)

func (k layerKind) sourceOf(s configSetting) string {
	switch k {
	case fromEnv:
		return sourceEnv(s.envVar)
	case fromFile:
		return SourceConfigFile
	default:
		return sourceCLI(s.flag)
	}
}

// configLayer is what one place (env, file or flags) supplied. An empty value
// means "not set here", so the value from a lower place is kept.
type configLayer struct {
	kind   layerKind
	values map[string]string
	labels map[string]string
}

// applyTo copies this layer's values over config and records this layer as
// their source.
func (l configLayer) applyTo(config *AgentConfiguration, provenance ConfigProvenance) error {
	for _, s := range configSettings {
		v := l.values[s.key]
		if v == "" {
			continue
		}
		if err := s.set(config, v); err != nil {
			return fmt.Errorf("%s: %w", l.kind.sourceOf(s), err)
		}
		provenance[s.key] = l.kind.sourceOf(s)
	}
	for k, v := range l.labels {
		config.Labels[k] = v
		provenance[labelsPrefix+k] = l.kind.sourceOf(labelsSetting)
	}
	return nil
}

func envConfigLayer() configLayer {
	layer := configLayer{kind: fromEnv, values: map[string]string{}}
	for _, s := range configSettings {
		layer.values[s.key] = os.Getenv(s.envVar)
	}
	if raw := os.Getenv(labelsSetting.envVar); raw != "" {
		layer.labels = parseLabels(raw)
	}
	return layer
}

func cliConfigLayer(cli cliFlags) configLayer {
	layer := configLayer{
		kind: fromCLI,
		values: map[string]string{
			"repoURL": cli.repoURL, "branch": cli.branch, "nodeID": cli.nodeID,
			"pollInterval": cli.pollInterval, "workspaceDir": cli.workspaceDir,
		},
	}
	if cli.labels != "" {
		layer.labels = parseLabels(cli.labels)
	}
	return layer
}
