// Reporting the resolved configuration: `--print-config` for a human, and one
// structured log line at startup, both carrying the source of every value.
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog"

	"helvilette/pkg/log"
)

// configFields returns the resolved configuration as ordered field/value pairs, including
// one entry per label. Ordering is stable so that operators comparing two nodes, or two
// runs on the same node, can diff the output directly.
func configFields(config AgentConfiguration) [][2]string {
	fields := [][2]string{
		{"repoURL", config.RepoURL},
		{"branch", config.Branch},
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
		nameWidth = max(nameWidth, len(f[0]))
		valueWidth = max(valueWidth, len(f[1]))
	}

	var b strings.Builder
	for _, f := range fields {
		fmt.Fprintf(&b, "%-*s = %-*s  source=%s\n", nameWidth, f[0], valueWidth, f[1], sourceOrDefault(provenance, f[0]))
	}
	return b.String()
}

// sourceOrDefault reports a field with no recorded source as a default rather
// than dropping it.
func sourceOrDefault(provenance ConfigProvenance, field string) string {
	if source := provenance[field]; source != "" {
		return source
	}
	return SourceDefault
}

// logEffectiveConfig reports the resolved configuration and where each value came from,
// so that a node's behaviour can be explained from its logs alone rather than by
// re-deriving the precedence rules against systemd units and container environments.
func logEffectiveConfig(config AgentConfiguration, provenance ConfigProvenance) {
	values := zerolog.Dict()
	sources := zerolog.Dict()
	for _, f := range configFields(config) {
		values = values.Str(f[0], f[1])
		sources = sources.Str(f[0], sourceOrDefault(provenance, f[0]))
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
