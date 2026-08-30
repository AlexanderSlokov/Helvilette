package playbook

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestDir creates a temporary test directory with playbook structure
func setupTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create test-collection with playbook.yml
	collectionDir := filepath.Join(tmpDir, "test-collection")
	if err := os.MkdirAll(collectionDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifestContent := `apiVersion: helvilette.io/v1alpha1
kind: PlaybookDeployment
metadata:
  name: test-deployment
spec:
  repo: git://git.example.com/repo
  playbook: playbook.yml
  nodeGroups:
    - name: group1
      nodeSelector:
        role: proxy
`
	if err := os.WriteFile(filepath.Join(collectionDir, "helvilette.yml"), []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create another-collection with helvilette.yml
	anotherDir := filepath.Join(tmpDir, "another-collection")
	if err := os.MkdirAll(anotherDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest2 := `apiVersion: helvilette.io/v1alpha1
kind: PlaybookDeployment
metadata:
  name: another-deployment
spec:
  repo: git://git.example.com/repo
  playbook: playbook.yml
  nodeGroups:
    - name: group1
      nodeSelector:
        role: web
`
	if err := os.WriteFile(filepath.Join(anotherDir, "helvilette.yml"), []byte(manifest2), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a nested collection with helvilette.yml
	nestedDir := filepath.Join(tmpDir, "nested", "collection")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest3 := `apiVersion: helvilette.io/v1alpha1
kind: PlaybookDeployment
metadata:
  name: nested-deployment
spec:
  repo: git://git.example.com/repo
  playbook: playbook.yml
  nodeGroups:
    - name: group1
      nodeSelector:
        role: db
`
	if err := os.WriteFile(filepath.Join(nestedDir, "helvilette.yml"), []byte(manifest3), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a directory without helvilette.yml (should be ignored)
	emptyDir := filepath.Join(tmpDir, "no-manifest")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a hidden directory (should be ignored)
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "helvilette.yml"), []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	return tmpDir
}

func TestScan_FindsAllPlaybooks(t *testing.T) {
	tmpDir := setupTestDir(t)

	loader, err := NewLoader(tmpDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}

	playbooks, err := loader.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should find exactly 3 playbooks
	// Should NOT include: no-manifest, .hidden (hidden dir)
	if len(playbooks) != 3 {
		t.Errorf("expected 3 playbooks, got %d", len(playbooks))
	}

	// Verify names
	names := make(map[string]bool)
	for _, pb := range playbooks {
		names[pb.Name] = true
	}

	if !names["test-collection"] {
		t.Error("expected to find test-collection")
	}
	if !names["another-collection"] {
		t.Error("expected to find another-collection")
	}
	if !names["nested/collection"] {
		t.Error("expected to find nested/collection")
	}
}

func TestGet_ReturnsMetadata(t *testing.T) {
	tmpDir := setupTestDir(t)

	loader, err := NewLoader(tmpDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}

	playbooks, err := loader.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Get by ID
	pb, err := loader.Get(playbooks[0].ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if pb.ID == "" {
		t.Error("expected non-empty ID")
	}
	if pb.Name == "" {
		t.Error("expected non-empty Name")
	}
	if pb.FullPath == "" {
		t.Error("expected non-empty FullPath")
	}
	if pb.ModTime.IsZero() {
		t.Error("expected non-zero ModTime")
	}
}

func TestScan_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	loader, err := NewLoader(tmpDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}

	playbooks, err := loader.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(playbooks) != 0 {
		t.Errorf("expected 0 playbooks in empty dir, got %d", len(playbooks))
	}
}

func TestNewLoader_InvalidDir(t *testing.T) {
	_, err := NewLoader("/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestGetByName(t *testing.T) {
	tmpDir := setupTestDir(t)

	loader, err := NewLoader(tmpDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}

	_, err = loader.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	pb, err := loader.GetByName("test-collection")
	if err != nil {
		t.Fatalf("GetByName failed: %v", err)
	}

	if pb.Name != "test-collection" {
		t.Errorf("expected name test-collection, got %s", pb.Name)
	}

	// Non-existent name
	_, err = loader.GetByName("does-not-exist")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID("test-collection")
	id2 := GenerateID("test-collection")
	id3 := GenerateID("another-collection")

	// Same input should produce same ID
	if id1 != id2 {
		t.Error("same input should produce same ID")
	}

	// Different input should produce different ID
	if id1 == id3 {
		t.Error("different input should produce different ID")
	}

	// ID should be 16 hex characters
	if len(id1) != 16 {
		t.Errorf("expected ID length 16, got %d", len(id1))
	}
}
