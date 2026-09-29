package playbook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"helvilette/pkg/log"
	"helvilette/pkg/manifest"

	"github.com/rs/zerolog"
)

// capturedLogs collects the JSON lines a Scan emits, so a test can assert on
// what an operator would actually see. Issue #34 was about missing log lines,
// which is only testable by reading them back.
type capturedLogs struct {
	entries []map[string]any
}

// captureScan runs scan at debug level with log output redirected, and returns
// every entry emitted.
func captureScan(t *testing.T, scan func()) *capturedLogs {
	t.Helper()

	var buf bytes.Buffer
	previousOut := log.SetOutput(&buf)
	previousLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	defer func() {
		zerolog.SetGlobalLevel(previousLevel)
		log.SetOutput(previousOut)
	}()

	scan()

	captured := &capturedLogs{}
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("log line is not JSON: %s (%v)", line, err)
		}
		captured.entries = append(captured.entries, entry)
	}

	return captured
}

// find returns the first entry whose message field equals msg.
func (c *capturedLogs) find(msg string) map[string]any {
	for _, entry := range c.entries {
		if entry["message"] == msg {
			return entry
		}
	}
	return nil
}

// findWhere returns the first entry where field equals value.
func (c *capturedLogs) findWhere(field string, value any) map[string]any {
	for _, entry := range c.entries {
		if entry[field] == value {
			return entry
		}
	}
	return nil
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

const validManifest = `apiVersion: ` + manifest.SupportedAPIVersion + `
kind: ` + manifest.SupportedKind + `
metadata:
  name: diagnostics-fixture
spec:
  repo: git://git.example.com/repo
  playbook: playbook.yml
  nodeGroups:
    - name: group1
      nodeSelector:
        role: proxy
`

// TestScan_ZeroCountExplainsItself is the regression test for issue #34: an
// operator saw {"count":0} and had nothing to act on. The closing line must now
// carry the directory scanned and how many files were examined, and must be a
// warning, because zero playbooks means no node gets dispatched anything.
func TestScan_ZeroCountExplainsItself(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, tmpDir, "baseline/playbook.yml", "- hosts: all\n")
	writeFile(t, tmpDir, "baseline/README.md", "docs\n")

	loader, err := NewLoader(tmpDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}

	logs := captureScan(t, func() {
		if _, err := loader.Scan(); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
	})

	entry := logs.find("scan complete")
	if entry == nil {
		t.Fatal("no 'scan complete' entry emitted")
	}
	if entry["level"] != "warn" {
		t.Errorf("level = %v, want warn when nothing was discovered", entry["level"])
	}
	if entry["base_dir"] != loader.BaseDir() {
		t.Errorf("base_dir = %v, want %s", entry["base_dir"], loader.BaseDir())
	}
	if entry["files_examined"].(float64) != 2 {
		t.Errorf("files_examined = %v, want 2", entry["files_examined"])
	}
	if entry["expected_filename"] != ManifestFilename {
		t.Errorf("expected_filename = %v, want %s", entry["expected_filename"], ManifestFilename)
	}
}

// TestScan_NearMissFilenameWarns covers the trap behind issue #34: a manifest
// named helvilette.yaml parses fine but is never loaded, and used to vanish
// without a word.
func TestScan_NearMissFilenameWarns(t *testing.T) {
	for _, name := range []string{"helvilette.yaml", "Helvilette.yml", "HELVILETTE.YAML"} {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			writeFile(t, tmpDir, filepath.Join("baseline", name), validManifest)

			loader, err := NewLoader(tmpDir)
			if err != nil {
				t.Fatalf("NewLoader failed: %v", err)
			}

			var playbooks []Playbook
			logs := captureScan(t, func() {
				playbooks, err = loader.Scan()
				if err != nil {
					t.Fatalf("Scan failed: %v", err)
				}
			})

			if len(playbooks) != 0 {
				t.Fatalf("got %d playbooks, want 0: only %s is accepted", len(playbooks), ManifestFilename)
			}

			entry := logs.findWhere("found", name)
			if entry == nil {
				t.Fatalf("no warning naming %s; entries = %v", name, logs.entries)
			}
			if entry["level"] != "warn" {
				t.Errorf("level = %v, want warn", entry["level"])
			}
			if entry["expected"] != ManifestFilename {
				t.Errorf("expected = %v, want %s", entry["expected"], ManifestFilename)
			}
		})
	}
}

func TestScan_SkippedPathsCarryAReason(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, tmpDir, "baseline/"+ManifestFilename, validManifest)
	writeFile(t, tmpDir, "baseline/playbook.yml", "- hosts: all\n")
	writeFile(t, tmpDir, ".git/objects/loose", "binary\n")

	loader, err := NewLoader(tmpDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}

	logs := captureScan(t, func() {
		if _, err := loader.Scan(); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
	})

	for _, reason := range []string{"hidden_dir", "not_a_manifest"} {
		if logs.findWhere("skip_reason", reason) == nil {
			t.Errorf("no entry with skip_reason=%q; entries = %v", reason, logs.entries)
		}
	}
}

func TestScan_RejectedManifestSaysWhy(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, tmpDir, "broken/"+ManifestFilename, "apiVersion: wrong/v1\nkind: Nonsense\n")

	loader, err := NewLoader(tmpDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}

	logs := captureScan(t, func() {
		if _, err := loader.Scan(); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
	})

	entry := logs.findWhere("skip_reason", "parse_rejected")
	if entry == nil {
		t.Fatalf("no entry with skip_reason=parse_rejected; entries = %v", logs.entries)
	}
	if entry["error"] == nil {
		t.Error("rejected manifest logged without the parse error")
	}

	summary := logs.find("scan complete")
	if summary["rejected"].(float64) != 1 {
		t.Errorf("rejected = %v, want 1", summary["rejected"])
	}
}

// TestScan_ConcurrentWithLookups guards the loader's map: Scan runs on Othela's
// fleet-sync goroutine while Get and GetByName are reached from HTTP handlers.
// Run with -race.
func TestScan_ConcurrentWithLookups(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, tmpDir, "baseline/"+ManifestFilename, validManifest)

	loader, err := NewLoader(tmpDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}
	if _, err := loader.Scan(); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	id := GenerateID("baseline")

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				if _, err := loader.Scan(); err != nil {
					t.Errorf("concurrent Scan failed: %v", err)
				}
				return
			}
			loader.Get(id)
			loader.GetByName("baseline")
		}(i)
	}
	wg.Wait()
}
