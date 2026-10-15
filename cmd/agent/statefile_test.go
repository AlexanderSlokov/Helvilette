// Tests for statefile.go: an atomic write leaves the old file or the new one,
// and nothing else behind.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomic_ReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "last_run_summary.json")
	if err := os.WriteFile(path, []byte(`{"old":true}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(path, []byte(`{"new":true}`), 0644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"new":true}` {
		t.Errorf("content = %q, want %q", got, `{"new":true}`)
	}
}

func TestWriteFileAtomic_LeavesNoTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	if err := writeFileAtomic(filepath.Join(dir, "state.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		t.Errorf("directory holds %v, want only state.json", entries)
	}
}

func TestWriteFileAtomic_AppliesPermission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "extra_vars.json")
	if err := writeFileAtomic(path, []byte("{}"), 0600); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestWriteFileAtomic_FailsWhenDirectoryIsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "state.json")
	if err := writeFileAtomic(path, []byte("{}"), 0644); err == nil {
		t.Error("expected an error for a missing parent directory")
	}
}
