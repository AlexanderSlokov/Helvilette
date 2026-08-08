package storage

import (
	"sync"

	"helvilette/pkg/types"
)

// MemoryNodeStore implements NodeStore using an in-memory map.
// Migrated from cmd/othela/server.go (formerly InMemoryNodeRegistry).
// Thread-safe via sync.RWMutex.
type MemoryNodeStore struct {
	mu    sync.RWMutex
	nodes map[string]map[string]string // nodeID -> labels
}

// NewMemoryNodeStore creates a ready-to-use in-memory node store.
func NewMemoryNodeStore() *MemoryNodeStore {
	return &MemoryNodeStore{
		nodes: make(map[string]map[string]string),
	}
}

func (s *MemoryNodeStore) Register(nodeID string, labels map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[nodeID] = labels
	return nil
}

func (s *MemoryNodeStore) GetLabels(nodeID string) (map[string]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	labels, ok := s.nodes[nodeID]
	return labels, ok
}

func (s *MemoryNodeStore) IsRegistered(nodeID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.nodes[nodeID]
	return ok
}

// MemoryReportStore implements ReportStore using an in-memory slice.
// Replaces the raw []Report field that was in Server struct.
// Thread-safe via sync.RWMutex.
type MemoryReportStore struct {
	mu      sync.RWMutex
	reports []types.Report
}

// NewMemoryReportStore creates a ready-to-use in-memory report store.
func NewMemoryReportStore() *MemoryReportStore {
	return &MemoryReportStore{
		reports: make([]types.Report, 0),
	}
}

func (s *MemoryReportStore) Save(report types.Report) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reports = append(s.reports, report)
	return nil
}

func (s *MemoryReportStore) List() ([]types.Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy to avoid data races on the caller side
	result := make([]types.Report, len(s.reports))
	copy(result, s.reports)
	return result, nil
}
