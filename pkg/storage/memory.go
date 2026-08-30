package storage

import (
	"sync"
	"time"

	"helvilette/pkg/types"
)

// MemoryNodeStore implements NodeStore using an in-memory map.
// Migrated from cmd/othela/server.go (formerly InMemoryNodeRegistry).
// Thread-safe via sync.RWMutex.
type MemoryNodeStore struct {
	mu    sync.RWMutex
	nodes map[string]Node // nodeID -> Node
}

// NewMemoryNodeStore creates a ready-to-use in-memory node store.
func NewMemoryNodeStore() *MemoryNodeStore {
	return &MemoryNodeStore{
		nodes: make(map[string]Node),
	}
}

func (s *MemoryNodeStore) Register(nodeID string, labels map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, exists := s.nodes[nodeID]
	if !exists {
		node = Node{
			NodeID:     nodeID,
			Registered: time.Now(),
		}
	}
	node.Labels = labels
	node.LastSeen = time.Now()
	s.nodes[nodeID] = node
	return nil
}

func (s *MemoryNodeStore) GetLabels(nodeID string) (map[string]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	node, ok := s.nodes[nodeID]
	return node.Labels, ok
}

func (s *MemoryNodeStore) IsRegistered(nodeID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.nodes[nodeID]
	return ok
}

func (s *MemoryNodeStore) UpdateStatus(nodeID string, status types.NodeStatus, observedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, exists := s.nodes[nodeID]
	if !exists {
		return nil // or error, but this is a mock store
	}
	node.Status = status
	node.ObservedAt = observedAt
	s.nodes[nodeID] = node
	return nil
}

func (s *MemoryNodeStore) ListNodes() ([]Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var nodes []Node
	for _, n := range s.nodes {
		nodes = append(nodes, n)
	}
	return nodes, nil
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
