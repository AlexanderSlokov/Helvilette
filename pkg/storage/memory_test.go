package storage

import (
	"encoding/json"
	"testing"

	"helvilette/pkg/types"
)

// --- MemoryNodeStore tests ---

func TestMemoryNodeStore_RegisterAndGetLabels(t *testing.T) {
	store := NewMemoryNodeStore()
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

func TestMemoryNodeStore_GetLabels_NotRegistered(t *testing.T) {
	store := NewMemoryNodeStore()

	_, ok := store.GetLabels("unknown")
	if ok {
		t.Error("expected ok=false for unregistered node")
	}
}

func TestMemoryNodeStore_IsRegistered(t *testing.T) {
	store := NewMemoryNodeStore()

	if store.IsRegistered("node-1") {
		t.Error("expected false before registration")
	}

	store.Register("node-1", nil)

	if !store.IsRegistered("node-1") {
		t.Error("expected true after registration")
	}
}

func TestMemoryNodeStore_RegisterOverwritesLabels(t *testing.T) {
	store := NewMemoryNodeStore()

	store.Register("node-1", map[string]string{"v": "1"})
	store.Register("node-1", map[string]string{"v": "2"})

	labels, _ := store.GetLabels("node-1")
	if labels["v"] != "2" {
		t.Errorf("expected overwritten label v=2, got v=%s", labels["v"])
	}
}

// --- MemoryReportStore tests ---

func TestMemoryReportStore_SaveAndList(t *testing.T) {
	store := NewMemoryReportStore()

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

func TestMemoryReportStore_ListEmpty(t *testing.T) {
	store := NewMemoryReportStore()

	reports, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(reports) != 0 {
		t.Errorf("expected 0 reports, got %d", len(reports))
	}
}

func TestMemoryReportStore_ListReturnsCopy(t *testing.T) {
	store := NewMemoryReportStore()

	store.Save(types.Report{NodeID: "n1", JobID: "j1", Status: "Success", TaskLogs: json.RawMessage(`{}`)})

	reports1, _ := store.List()
	reports2, _ := store.List()

	// Mutating one slice should not affect the other
	reports1[0].Status = "MUTATED"
	if reports2[0].Status == "MUTATED" {
		t.Error("List() should return a copy, not a reference to internal state")
	}
}
