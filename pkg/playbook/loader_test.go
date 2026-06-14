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
	
	playbookContent := `---
- name: Test Playbook
  hosts: localhost
  tasks:
    - name: Test task
      debug:
        msg: "Hello from test"
`
	if err := os.WriteFile(filepath.Join(collectionDir, "playbook.yml"), []byte(playbookContent), 0644); err != nil {
		t.Fatal(err)
	}
	
	// Create another-collection with playbook.yml
	anotherDir := filepath.Join(tmpDir, "another-collection")
	if err := os.MkdirAll(anotherDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(anotherDir, "playbook.yml"), []byte("---\n- name: Another\n"), 0644); err != nil {
		t.Fatal(err)
	}
	
	// Create a directory without playbook.yml (should be ignored)
	emptyDir := filepath.Join(tmpDir, "no-playbook")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}
	
	// Create a hidden directory (should be ignored)
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "playbook.yml"), []byte("---\n"), 0644); err != nil {
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
	
	// Should find exactly 2 playbooks (test-collection and another-collection)
	// Should NOT include: no-playbook (no playbook.yml), .hidden (hidden dir)
	if len(playbooks) != 2 {
		t.Errorf("expected 2 playbooks, got %d", len(playbooks))
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
}

func TestLoad_ReadsContent(t *testing.T) {
	tmpDir := setupTestDir(t)
	
	loader, err := NewLoader(tmpDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}
	
	playbooks, err := loader.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	
	// Find test-collection
	var testPB *Playbook
	for i := range playbooks {
		if playbooks[i].Name == "test-collection" {
			testPB = &playbooks[i]
			break
		}
	}
	
	if testPB == nil {
		t.Fatal("test-collection not found")
	}
	
	content, err := loader.Load(testPB.ID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	
	if content == "" {
		t.Error("expected non-empty content")
	}
	
	// Verify content contains expected text
	if len(content) < 10 {
		t.Error("content too short")
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

func TestLoad_NotFound(t *testing.T) {
	tmpDir := setupTestDir(t)
	
	loader, err := NewLoader(tmpDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}
	
	// Must scan first to populate cache
	_, err = loader.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	
	// Try to load non-existent playbook
	_, err = loader.Load("non-existent-id")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
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
