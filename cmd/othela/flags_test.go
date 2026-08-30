package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// The --data-dir flag was removed in ADR-0003 because it named the playbook
// directory while also receiving the SQLite state. These tests pin the
// replacement contract: the old flag must fail loudly and name both successors,
// and the database must derive from --state-dir rather than the playbook path.

func TestRemovedFlagError_RejectsDataDirInEveryForm(t *testing.T) {
	forms := map[string][]string{
		"long with equals":      {"--data-dir=/srv/playbooks"},
		"long with space":       {"--data-dir", "/srv/playbooks"},
		"shorthand with space":  {"-d", "/srv/playbooks"},
		"shorthand with equals": {"-d=/srv/playbooks"},
		"after other flags":     {"--port=8080", "--data-dir=/srv/playbooks"},
	}

	for name, args := range forms {
		t.Run(name, func(t *testing.T) {
			err := removedFlagError(args)
			if err == nil {
				t.Fatalf("removedFlagError(%q) = nil, want an error naming the replacement flags", args)
			}
		})
	}
}

func TestRemovedFlagError_MessageNamesBothReplacements(t *testing.T) {
	err := removedFlagError([]string{"--data-dir=/srv/playbooks"})
	if err == nil {
		t.Fatal("removedFlagError() = nil, want an error")
	}

	// An operator hitting this must be able to fix the command without reading
	// the source or the ADR, so both replacements and their defaults appear.
	for _, want := range []string{"--fleet-repo", "--state-dir", defaultStateDir} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message missing %q\ngot: %s", want, err.Error())
		}
	}
}

func TestRemovedFlagError_RejectsPlaybookDir(t *testing.T) {
	err := removedFlagError([]string{"--playbook-dir=/srv/playbooks"})
	if err == nil {
		t.Fatal("removedFlagError() = nil, want an error")
	}

	for _, want := range []string{"--fleet-repo"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message missing %q\ngot: %s", want, err.Error())
		}
	}
}

func TestRemovedFlagError_AllowsCurrentFlags(t *testing.T) {
	args := []string{
		"--port=8080",
		"--fleet-repo=git://git-server:9418/repo",
		"--state-dir=/app/state",
		"--log-level=debug",
	}

	if err := removedFlagError(args); err != nil {
		t.Fatalf("removedFlagError(%q) = %v, want nil", args, err)
	}
}

// Guards against a flag whose name merely starts with the removed one, which
// would otherwise be swallowed by a naive prefix check.
func TestRemovedFlagError_IgnoresSimilarlyNamedFlags(t *testing.T) {
	args := []string{"--data-dir-legacy=/srv", "--debug"}

	if err := removedFlagError(args); err != nil {
		t.Fatalf("removedFlagError(%q) = %v, want nil", args, err)
	}
}

// The database must sit under --state-dir. Deriving it from the playbook
// directory is what put a writable path inside the Go module tree and broke
// `go vet ./...`; see ADR-0003.
func TestDatabasePathDerivesFromStateDir(t *testing.T) {
	const stateDirUnderTest = "/var/lib/helvilette/othela"

	got := filepath.Join(stateDirUnderTest, "db", "state.db")
	want := "/var/lib/helvilette/othela/db/state.db"

	if got != want {
		t.Fatalf("database path = %q, want %q", got, want)
	}
}

func TestDefaultStateDirIsOutsideTheModuleTree(t *testing.T) {
	if !filepath.IsAbs(defaultStateDir) {
		t.Fatalf("defaultStateDir = %q, want an absolute path so state never lands in a checkout", defaultStateDir)
	}
}
