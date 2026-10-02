package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// agentUnitPath is the systemd unit the e2e stack boots the agent with. It is the
// only place the agent's flags are written down outside this package.
const agentUnitPath = "../../tests/images/node/helvilette-agent.service"

// TestAgentUnitFlagsExistOnTheCLI is the Agent half of the guard that already
// protects Othela. Issue #33 was a deployment artifact passing a flag the binary
// did not define, and nothing failed until a human started the stack by hand.
// This could not be written until newRootCmd was separated from main, because a
// command built inside main is unreachable from a test. See BACKLOG 6.7.
func TestAgentUnitFlagsExistOnTheCLI(t *testing.T) {
	flags := longFlagsIn(execStartOf(t, agentUnitPath))
	if len(flags) == 0 {
		t.Fatalf("%s passes no long flags; the guard would pass vacuously", agentUnitPath)
	}

	cmd := newRootCmd()
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("%s passes --%s, which the agent does not define", agentUnitPath, name)
		}
	}
}

// execStartOf returns a unit's ExecStart= value, with the backslash line
// continuations systemd allows folded into one line.
func execStartOf(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("read unit %s: %v", path, err)
	}

	folded := strings.ReplaceAll(string(raw), "\\\n", " ")
	for line := range strings.SplitSeq(folded, "\n") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(line), "ExecStart="); ok {
			return after
		}
	}

	t.Fatalf("no ExecStart= in %s", path)
	return ""
}

var longFlagPattern = regexp.MustCompile(`--([a-z0-9][a-z0-9-]*)`)

// longFlagsIn extracts every --flag name from a command line, dropping any
// =value suffix.
func longFlagsIn(command string) []string {
	var names []string
	for _, match := range longFlagPattern.FindAllStringSubmatch(command, -1) {
		names = append(names, match[1])
	}
	return names
}
