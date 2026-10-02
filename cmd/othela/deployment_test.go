package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"helvilette/pkg/log"
)

// The deployment artifacts that invoke othela. Each is checked against the
// flags the CLI actually registers.
const (
	othelaUnitPath = "../../tests/images/node/helvilette-othela.service"
	composePath    = "../../e2e.compose.yml"
)

// TestOthelaUnitFlagsExistOnTheCLI is the guard against issue #33 recurring.
//
// The e2e stack invoked othela with --fleet-repo for weeks while the CLI only
// defined --playbook-dir. Nothing failed until a human started the stack by
// hand, because nothing connected the deployment artifacts to the flag set.
// Since ADR-0007 the flags live in the systemd unit rather than in a compose
// `command:`, so that is what this reads.
func TestOthelaUnitFlagsExistOnTheCLI(t *testing.T) {
	flags := longFlagsIn(t, execStartOf(t, othelaUnitPath))
	if len(flags) == 0 {
		t.Fatalf("%s passes no long flags; the guard would pass vacuously", othelaUnitPath)
	}

	assertFlagsRegistered(t, othelaUnitPath, flags)
}

// TestComposeOverrideFlagsExistOnTheCLI covers the other way flags can reach
// othela: a `command:` on the compose service, overriding the image's default.
// There is none today. The check stays so that adding one cannot reintroduce
// the drift, and it does not fail when the key is absent.
func TestComposeOverrideFlagsExistOnTheCLI(t *testing.T) {
	command := composeCommandFor(t, "othela")
	if len(command) == 0 {
		t.Skip("othela service defines no command:; flags come from the systemd unit")
	}

	assertFlagsRegistered(t, composePath, longFlagsIn(t, strings.Join(command, " ")))
}

func assertFlagsRegistered(t *testing.T, source string, flags []string) {
	t.Helper()

	for _, name := range flags {
		if rootCmd.Flags().Lookup(name) == nil {
			t.Errorf("%s passes --%s, which othela does not define", source, name)
		}
	}
}

// execStartOf returns the ExecStart= value of a systemd unit, with the
// backslash line continuations systemd allows folded into one line.
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
func longFlagsIn(t *testing.T, command string) []string {
	t.Helper()

	var names []string
	for _, match := range longFlagPattern.FindAllStringSubmatch(command, -1) {
		names = append(names, match[1])
	}
	return names
}

// composeCommandFor returns the command: list of one service, or nil when the
// service does not set one.
func composeCommandFor(t *testing.T, service string) []string {
	t.Helper()

	raw, err := os.ReadFile(filepath.FromSlash(composePath))
	if err != nil {
		t.Fatalf("read compose file %s: %v", composePath, err)
	}

	// Services are decoded one at a time: other services may write command: as a
	// single shell string, which does not fit the list shape this one uses.
	var compose struct {
		Services map[string]yaml.Node `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &compose); err != nil {
		t.Fatalf("parse compose file %s: %v", composePath, err)
	}

	node, ok := compose.Services[service]
	if !ok {
		t.Fatalf("%s has no %s service", composePath, service)
	}

	var svc struct {
		Command []string `yaml:"command"`
	}
	if err := node.Decode(&svc); err != nil {
		t.Fatalf("decode %s service: %v", service, err)
	}
	return svc.Command
}

// TestOthelaUnitIdentifierMatchesTheBinary guards a value now written in two
// places. A native journal send does not inherit the unit's SyslogIdentifier= —
// that applies to the stream transport only — so the binary derives its own. If
// the two drift, `journalctl -t helvilette-othela` silently stops finding the
// control plane. See ADR-0007 D2.
func TestOthelaUnitIdentifierMatchesTheBinary(t *testing.T) {
	want := log.IdentifierFor("/usr/local/bin/othela")

	if got := syslogIdentifierOf(t, othelaUnitPath); got != want {
		t.Errorf("%s declares SyslogIdentifier=%s, but the binary sends %s", othelaUnitPath, got, want)
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
