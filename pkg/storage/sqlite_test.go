package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"helvilette/pkg/types"
)

// newTestSQLiteStore creates a SQLiteStore in a temp directory.
// The DB is automatically cleaned up when the test finishes.
func newTestSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore(%q) failed: %v", dbPath, err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// --- NodeStore tests ---

func TestSQLiteNodeStore_RegisterAndGetLabels(t *testing.T) {
	store := newTestSQLiteStore(t)
	labels := map[string]string{"role": "web", "env": "prod"}

	if err := store.Register("node-1", labels); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, ok := store.GetLabels("node-1")
	if !ok {
		t.Fatal("expected node-1 to be registered")
	}
	if got["role"] != "web" || got["env"] != "prod" {
		t.Errorf("unexpected labels: %v", got)
	}
}

func TestSQLiteNodeStore_GetLabels_NotRegistered(t *testing.T) {
	store := newTestSQLiteStore(t)

	_, ok := store.GetLabels("unknown")
	if ok {
		t.Error("expected ok=false for unregistered node")
	}
}

func TestSQLiteNodeStore_IsRegistered(t *testing.T) {
	store := newTestSQLiteStore(t)

	if store.IsRegistered("node-1") {
		t.Error("expected false before registration")
	}

	store.Register("node-1", nil)

	if !store.IsRegistered("node-1") {
		t.Error("expected true after registration")
	}
}

func TestSQLiteNodeStore_RegisterOverwritesLabels(t *testing.T) {
	store := newTestSQLiteStore(t)

	store.Register("node-1", map[string]string{"v": "1"})
	store.Register("node-1", map[string]string{"v": "2"})

	labels, _ := store.GetLabels("node-1")
	if labels["v"] != "2" {
		t.Errorf("expected overwritten label v=2, got v=%s", labels["v"])
	}
}

// --- ReportStore tests ---

func TestSQLiteReportStore_SaveAndList(t *testing.T) {
	store := newTestSQLiteStore(t)

	r1 := types.Report{
		NodeID:   "node-1",
		JobID:    "job-1",
		Status:   "Success",
		TaskLogs: json.RawMessage(`{}`),
	}
	r2 := types.Report{
		NodeID:   "node-2",
		JobID:    "job-2",
		Status:   "Failed",
		TaskLogs: json.RawMessage(`{"error": "timeout"}`),
	}

	if err := store.Save(r1); err != nil {
		t.Fatalf("Save r1 failed: %v", err)
	}
	if err := store.Save(r2); err != nil {
		t.Fatalf("Save r2 failed: %v", err)
	}

	reports, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}

	if reports[0].NodeID != "node-1" {
		t.Errorf("expected first report node-1, got %s", reports[0].NodeID)
	}
	if reports[1].Status != "Failed" {
		t.Errorf("expected second report Failed, got %s", reports[1].Status)
	}
}

func TestSQLiteReportStore_ListEmpty(t *testing.T) {
	store := newTestSQLiteStore(t)

	reports, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(reports) != 0 {
		t.Errorf("expected 0 reports, got %d", len(reports))
	}
}

// --- Schema and persistence tests ---

func TestSQLiteStore_SchemaCreatedAutomatically(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "auto-schema.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	// Should be able to use immediately without manual schema setup
	if err := store.Register("test", map[string]string{"a": "b"}); err != nil {
		t.Fatalf("Register on fresh DB failed: %v", err)
	}
}

func TestSQLiteStore_PersistsAcrossReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "persist.db")

	// Open, write, close
	store1, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("first open failed: %v", err)
	}
	store1.Register("node-1", map[string]string{"role": "db"})
	store1.Save(types.Report{
		NodeID:   "node-1",
		JobID:    "job-1",
		Status:   "Success",
		TaskLogs: json.RawMessage(`{}`),
	})
	store1.Close()

	// Reopen and verify data survived
	store2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("second open failed: %v", err)
	}
	defer store2.Close()

	if !store2.IsRegistered("node-1") {
		t.Error("node-1 should survive DB reopen")
	}
	labels, _ := store2.GetLabels("node-1")
	if labels["role"] != "db" {
		t.Errorf("expected role=db after reopen, got %v", labels)
	}

	reports, _ := store2.List()
	if len(reports) != 1 {
		t.Fatalf("expected 1 report after reopen, got %d", len(reports))
	}
	if reports[0].JobID != "job-1" {
		t.Errorf("expected job-1 after reopen, got %s", reports[0].JobID)
	}
}

func TestSQLiteStore_CreatesDirIfMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep", "dir")
	dbPath := filepath.Join(dir, "state.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore should create missing dirs: %v", err)
	}
	defer store.Close()

	// Verify the directory was created
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}

func TestSQLiteNodeStore_NilLabels(t *testing.T) {
	store := newTestSQLiteStore(t)

	// Registering with nil labels should not panic or error
	if err := store.Register("node-nil", nil); err != nil {
		t.Fatalf("Register with nil labels failed: %v", err)
	}

	if !store.IsRegistered("node-nil") {
		t.Error("expected node-nil to be registered")
	}

	labels, ok := store.GetLabels("node-nil")
	if !ok {
		t.Error("expected ok=true for registered node with nil labels")
	}
	// nil labels should be stored as null/empty map
	if labels == nil {
		// This is acceptable -- nil map is fine
	}
}
