package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"

	"helvilette/pkg/git"
	"helvilette/pkg/log"
	"helvilette/pkg/manifest"
	"helvilette/pkg/playbook"
	"helvilette/pkg/storage"
	"helvilette/pkg/types"
)

// Package-level logger for all Othela server operations.
var logger = log.WithComponent("othela")

// Type aliases for backward compatibility within this package
type Job = types.Job
type Report = types.Report

// Server represents the Othela control plane server
type Server struct {
	router      *mux.Router
	loader      *playbook.Loader
	currentJob  Job // fallback / testing
	playbooks   []playbook.Playbook
	lastCommit  string // SHA the fleet cache last resolved to
	nodeStore   storage.NodeStore
	reportStore storage.ReportStore
	// mu protects currentJob, playbooks, loader and lastCommit. loader joined
	// the list when StartFleetSync moved to a cancellable goroutine: it is
	// written by that goroutine and read by GetLoader.
	mu    sync.RWMutex
	ready atomic.Bool // readiness probe state
}

// NewServer creates a new Othela server with default configuration.
// The server starts with no default job; dispatch is driven entirely by
// manifest matching in handleSync.
func NewServer() *Server {
	s := &Server{
		router:      mux.NewRouter(),
		nodeStore:   storage.NewMemoryNodeStore(),
		reportStore: storage.NewMemoryReportStore(),
	}
	s.ready.Store(true)
	s.setupRoutes()
	return s
}

// NewServerWithLoader creates a new Othela server with a playbook loader.
// Playbooks are scanned for manifest matching; no fallback job is created.
func NewServerWithLoader(loader *playbook.Loader) *Server {
	s := &Server{
		router:      mux.NewRouter(),
		loader:      loader,
		nodeStore:   storage.NewMemoryNodeStore(),
		reportStore: storage.NewMemoryReportStore(),
	}
	s.ready.Store(true)

	// Scan playbooks and keep in memory for manifest matching
	playbooks, err := loader.Scan()
	if err == nil {
		s.playbooks = playbooks
	}

	s.setupRoutes()
	return s
}

// NewServerWithJob creates a server with a specific job (for testing)
func NewServerWithJob(job Job) *Server {
	s := &Server{
		router:      mux.NewRouter(),
		currentJob:  job,
		nodeStore:   storage.NewMemoryNodeStore(),
		reportStore: storage.NewMemoryReportStore(),
	}
	s.ready.Store(true)
	s.setupRoutes()
	return s
}

// ServerConfig holds injectable dependencies for creating a Server.
// Use this when you need to supply a non-default storage backend (e.g. SQLite).
type ServerConfig struct {
	NodeStore   storage.NodeStore
	ReportStore storage.ReportStore
	Loader      *playbook.Loader
}

// NewServerWithConfig creates a Server with externally provided dependencies.
// Falls back to in-memory stores if NodeStore or ReportStore is nil.
func NewServerWithConfig(cfg ServerConfig) *Server {
	nodeStore := cfg.NodeStore
	if nodeStore == nil {
		nodeStore = storage.NewMemoryNodeStore()
	}
	reportStore := cfg.ReportStore
	if reportStore == nil {
		reportStore = storage.NewMemoryReportStore()
	}

	s := &Server{
		router:      mux.NewRouter(),
		loader:      cfg.Loader,
		nodeStore:   nodeStore,
		reportStore: reportStore,
	}
	s.ready.Store(true)

	// If loader is provided, scan playbooks for manifest matching
	if cfg.Loader != nil {
		playbooks, err := cfg.Loader.Scan()
		if err == nil {
			s.playbooks = playbooks
		}
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.HandleFunc("/api/v1/nodes/register", s.handleRegisterNode).Methods("POST")
	s.router.HandleFunc("/api/v1/nodes", s.handleGetNodes).Methods("GET")
	s.router.HandleFunc("/api/v1/sync/{node_id}", s.handleSync).Methods("GET")
	s.router.HandleFunc("/api/v1/report", s.handleReport).Methods("POST")
	s.router.HandleFunc("/api/v1/playbooks", s.handlePlaybooks).Methods("GET")

	// Health & readiness probes (K8s-style)
	s.router.HandleFunc("/healthz", s.handleHealthz).Methods("GET")
	s.router.HandleFunc("/readyz", s.handleReadyz).Methods("GET")
}

// Router returns the HTTP router for the server
func (s *Server) Router() *mux.Router {
	return s.router
}

// GetCurrentJob returns the current job
func (s *Server) GetCurrentJob() Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentJob
}

// SetCurrentJob sets the current job
func (s *Server) SetCurrentJob(job Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentJob = job
}

// GetPlaybooks returns the current playbooks
func (s *Server) GetPlaybooks() []playbook.Playbook {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.playbooks
}

// SetPlaybooks sets the current playbooks
func (s *Server) SetPlaybooks(playbooks []playbook.Playbook) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playbooks = playbooks
}

// GetReports returns all received reports.
func (s *Server) GetReports() []Report {
	reports, _ := s.reportStore.List()
	return reports
}

type NodeRegistration struct {
	NodeID string            `json:"node_id"`
	Labels map[string]string `json:"labels"`
}

func (s *Server) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	var req NodeRegistration
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.nodeStore.Register(req.NodeID, req.Labels)

	logger.Info().Str("node_id", req.NodeID).Any("labels", req.Labels).Msg("node registered")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(req)
}

// handleSync handles the sync endpoint - Agent polls for work
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["node_id"]

	logger.Debug().Str("node_id", nodeID).Msg("node polling for work")

	if !s.nodeStore.IsRegistered(nodeID) {
		http.Error(w, "node not registered, call POST /api/v1/nodes/register first", http.StatusForbidden)
		return
	}

	labels, _ := s.nodeStore.GetLabels(nodeID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	var matchedJob *Job

	// 1. Try to match from playbooks (helvilette.yml)
	for _, pb := range s.playbooks {
		if pb.Manifest == nil {
			continue
		}

		matches := manifest.MatchNodeGroups(pb.Manifest, labels)
		if len(matches) > 0 {
			// ADR-0004 rejects exact-duplicate selectors at load time, so
			// multiple matches here can only arise from subset overlap (e.g.
			// selectors {role: proxy} and {role: proxy, tier: hot} both match
			// a node with both labels). First-match is kept for this edge case
			// until full subset-overlap rejection lands in v1beta1.
			group := matches[0]

			repoURL := pb.Manifest.Spec.Repo
			if repoURL == "" {
				repoURL = os.Getenv("HELV_TEST_REPO_URL")
				if repoURL == "" {
					repoURL = "http://git-server:3000/helvilette/nginx-collection.git"
				}
			}

			matchedJob = &Job{
				JobID:        "job-" + pb.ID + "-" + group.Name,
				RepoURL:      repoURL,
				Version:      pb.Manifest.Spec.Branch,
				PlaybookPath: pb.Manifest.Spec.Playbook,
				ExtraVars:    group.Ansible.ExtraVars,
			}
			break
		}
	}

	// 2. Fallback to currentJob if no manifests match but we have a mock/fallback job
	if matchedJob == nil && s.currentJob.JobID != "" {
		// Only fallback if there are no manifests at all (to preserve fail-loud when manifests exist)
		hasManifests := false
		for _, pb := range s.playbooks {
			if pb.Manifest != nil {
				hasManifests = true
				break
			}
		}

		if !hasManifests {
			matchedJob = &s.currentJob
		}
	}

	if matchedJob == nil {
		// Graceful idle: No matching nodeGroups
		logger.Debug().Str("node_id", nodeID).Any("labels", labels).Msg("no matching node selectors")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matchedJob)
}

// handleGetNodes returns all registered nodes and their statuses
func (s *Server) handleGetNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.nodeStore.ListNodes()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type NodeResponse struct {
		storage.Node
		NetworkStatus string `json:"network_status"`
	}

	var response []NodeResponse
	for _, n := range nodes {
		networkStatus := "Known"
		if time.Since(n.LastSeen) > 2*time.Minute {
			networkStatus = "Unknown"
		}
		response = append(response, NodeResponse{
			Node:          n,
			NetworkStatus: networkStatus,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleReport handles the report endpoint - Agent sends back results
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	var report Report
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.reportStore.Save(report); err != nil {
		logger.Error().Err(err).Str("node_id", report.NodeID).Str("job_id", report.JobID).Msg("failed to save report")
	}

	// Update node status based on the report
	if report.NodeStatus.JobID != "" {
		if err := s.nodeStore.UpdateStatus(report.NodeID, report.NodeStatus, report.ObservedAt); err != nil {
			logger.Error().Err(err).Str("node_id", report.NodeID).Str("job_id", report.JobID).Msg("failed to update node status")
		}
	}

	logger.Info().
		Str("node_id", report.NodeID).
		Str("job_id", report.JobID).
		Str("status", report.Status).
		RawJSON("task_logs", report.TaskLogs).
		Msg("report received")

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Report received")
}

// ListenAndServe starts the server on the specified address
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.router)
}

// NewHTTPServer creates a configured *http.Server with production-grade timeouts.
// Use this with graceful shutdown instead of ListenAndServe.
func (s *Server) NewHTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// SetReady sets the readiness state of the server.
// Set to false during shutdown to stop accepting new traffic.
func (s *Server) SetReady(ready bool) {
	s.ready.Store(ready)
}

// IsReady returns whether the server is ready to serve traffic.
func (s *Server) IsReady() bool {
	return s.ready.Load()
}

// handleHealthz responds with liveness status.
// This endpoint indicates the process is alive and can serve requests.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleReadyz responds with readiness status.
// Returns 503 when the server is shutting down or not yet initialized.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !s.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "not_ready"})
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handlePlaybooks lists all available playbooks
func (s *Server) handlePlaybooks(w http.ResponseWriter, r *http.Request) {
	playbooks := s.GetPlaybooks()

	logger.Debug().Int("count", len(playbooks)).Msg("returning playbooks")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(playbooks)
}

// FleetSyncConfig names the fleet repository and how often to re-read it.
// Grouped into a struct so StartFleetSync does not take four positional
// strings-and-durations that are easy to transpose at the call site.
type FleetSyncConfig struct {
	Repo     string // Git URL of the repository holding helvilette.yml manifests
	Branch   string // Branch, tag or commit SHA to track
	CacheDir string // Local checkout, owned by Othela and treated as disposable
	Interval time.Duration
}

// StartFleetSync syncs the fleet repository once, then keeps re-syncing on
// cfg.Interval until ctx is cancelled.
//
// The first sync runs synchronously so a server that has started is a server
// whose playbooks are already loaded. Cancelling ctx stops the loop; before
// issue #32 the goroutine had no exit and outlived graceful shutdown.
//
//	ctx, cancel := context.WithCancel(context.Background())
//	server.StartFleetSync(ctx, FleetSyncConfig{Repo: url, Branch: "main", CacheDir: dir, Interval: time.Minute})
func (s *Server) StartFleetSync(ctx context.Context, cfg FleetSyncConfig) {
	s.syncFleetOnce(cfg)

	go func() {
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				logger.Info().Msg("fleet sync loop stopped")
				return
			case <-ticker.C:
				s.syncFleetOnce(cfg)
			}
		}
	}()
}

// syncFleetOnce pulls the fleet repository and republishes the playbooks it
// contains. Every failure path logs and returns, leaving the previously loaded
// playbooks in place: a transient Git outage must not empty the fleet.
func (s *Server) syncFleetOnce(cfg FleetSyncConfig) {
	commit, err := git.EnsureRepo(cfg.Repo, cfg.CacheDir, cfg.Branch)
	if err != nil {
		logger.Error().Err(err).Str("repo", cfg.Repo).Str("branch", cfg.Branch).
			Msg("failed to sync fleet repository — check repo URL accessibility, branch name and credentials")
		return
	}

	loader, err := s.ensureLoader(cfg.CacheDir)
	if err != nil {
		logger.Error().Err(err).Str("cache_dir", cfg.CacheDir).Msg("failed to create playbook loader")
		return
	}

	playbooks, err := loader.Scan()
	if err != nil {
		logger.Error().Err(err).Str("cache_dir", cfg.CacheDir).Msg("failed to scan playbooks")
		return
	}

	s.publishFleet(playbooks, commit)
}

// ensureLoader returns the loader for cacheDir, creating it on first use.
// Holds mu because the sync goroutine writes s.loader while GetLoader reads it.
func (s *Server) ensureLoader(cacheDir string) (*playbook.Loader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loader != nil && s.loader.BaseDir() == cacheDir {
		return s.loader, nil
	}

	loader, err := playbook.NewLoader(cacheDir)
	if err != nil {
		return nil, err
	}
	s.loader = loader

	return loader, nil
}

// publishFleet swaps in the newly scanned playbooks and reports the outcome.
// A commit that moved logs at info: before issue #32 a successful sync was
// invisible at the default level, so an operator could not tell a working
// poll loop from a dead one.
func (s *Server) publishFleet(playbooks []playbook.Playbook, commit string) {
	s.mu.Lock()
	previous := s.lastCommit
	s.playbooks = playbooks
	s.lastCommit = commit
	s.mu.Unlock()

	if previous == commit {
		logger.Debug().Str("fleet_commit", commit).Int("playbook_count", len(playbooks)).
			Msg("fleet sync complete, no change")
		return
	}

	logger.Info().Str("fleet_commit", commit).Str("previous_commit", previous).
		Int("playbook_count", len(playbooks)).Msg("fleet updated")
}

// GetLoader returns the playbook loader (for testing)
func (s *Server) GetLoader() *playbook.Loader {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loader
}

// LastCommit returns the commit SHA the fleet cache last resolved to, or an
// empty string before the first successful sync.
func (s *Server) LastCommit() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastCommit
}
