package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"helvilette/pkg/log"
)

// agentUnitPath is the systemd unit the e2e stack boots the agent with. It is the
// only place the agent's flags are written down outside this package.
const agentUnitPath = "../../tests/images/node/helvilette-agent.service"

// TestAgentUnitFlagsExistOnTheCLI fails when the systemd unit passes a flag the
// agent does not define. Without it, a renamed or removed flag is only noticed
// when the unit fails to start on a real machine.
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

// TestAgentUnitIdentifierMatchesTheBinary guards a value now written in two
// places. A native journal send does not inherit the unit's SyslogIdentifier= —
// that applies to the stream transport only — so the binary derives its own. If
// the two drift, `journalctl -t helvilette-agent` silently stops finding the
// agents.
func TestAgentUnitIdentifierMatchesTheBinary(t *testing.T) {
	want := log.IdentifierFor("/usr/local/bin/agent")

	if got := syslogIdentifierOf(t, agentUnitPath); got != want {
		t.Errorf("%s declares SyslogIdentifier=%s, but the binary sends %s", agentUnitPath, got, want)
	}
}

// syslogIdentifierOf returns a unit's SyslogIdentifier= value.
func syslogIdentifierOf(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("read unit %s: %v", path, err)
	}

	for line := range strings.SplitSeq(string(raw), "\n") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(line), "SyslogIdentifier="); ok {
			return after
		}
	}

	t.Fatalf("no SyslogIdentifier= in %s; journalctl -t would not find this unit", path)
	return ""
}
