// Package storage defines the persistence interfaces for Othela's data layer.
// Adapters (memory, sqlite) implement these interfaces and are injected into
// the Server at startup. This follows the ports-and-adapters pattern so the
// storage backend can be swapped without touching HTTP handler code.
package storage

import "helvilette/pkg/types"

// NodeStore manages registered agent nodes and their labels.
//
// Usage:
//
//	store := storage.NewMemoryNodeStore()
//	store.Register("agent-01", map[string]string{"role": "web"})
//	labels, ok := store.GetLabels("agent-01")
type NodeStore interface {
	// Register adds or updates a node with the given labels.
	Register(nodeID string, labels map[string]string) error

	// GetLabels returns the labels for a node. The bool is false if the node
	// is not registered.
	GetLabels(nodeID string) (map[string]string, bool)

	// IsRegistered returns true if the node has been registered.
	IsRegistered(nodeID string) bool
}

// ReportStore persists execution reports sent back by Agents.
//
// Usage:
//
//	store := storage.NewMemoryReportStore()
//	store.Save(types.Report{NodeID: "agent-01", JobID: "job-1", Status: "Success"})
//	all, _ := store.List()
type ReportStore interface {
	// Save persists a single execution report.
	Save(report types.Report) error

	// List returns all stored reports, ordered by insertion time.
	List() ([]types.Report, error)
}
