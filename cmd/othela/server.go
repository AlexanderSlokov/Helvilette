package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"

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

// handleSync answers an agent polling for work: 200 with a job, 204 when no
// nodeGroup selects it, 403 when it has not registered.
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	nodeID := mux.Vars(r)["node_id"]

	logger.Debug().Str("node_id", nodeID).Msg("node polling for work")

	if !s.nodeStore.IsRegistered(nodeID) {
		http.Error(w, "node not registered, call POST /api/v1/nodes/register first", http.StatusForbidden)
		return
	}

	labels, _ := s.nodeStore.GetLabels(nodeID)

	job := s.jobForLabels(labels)
	if job == nil {
		// Graceful idle, not an error: a node no manifest selects is a normal
		// state, and 204 says so without filling its log with failures.
		logger.Debug().Str("node_id", nodeID).Any("labels", labels).Msg("no matching node selectors")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// jobForLabels returns the job a node carrying these labels should run, or nil
// when nothing selects it.
func (s *Server) jobForLabels(labels map[string]string) *Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, pb := range s.playbooks {
		if pb.Manifest == nil {
			continue
		}

		matches := manifest.MatchNodeGroups(pb.Manifest, labels)
		if len(matches) == 0 {
			continue
		}

		// ADR-0004 rejects exact-duplicate selectors at load time, so multiple
		// matches here can only arise from subset overlap (e.g. selectors
		// {role: proxy} and {role: proxy, tier: hot} both match a node carrying
		// both labels). First-match is kept for that edge case until full
		// subset-overlap rejection lands in v1beta1.
		return jobFor(pb, matches[0])
	}

	return s.injectedJob()
}

// jobFor builds the job that dispatches one nodeGroup of one playbook.
//
// spec.repo is read without a fallback. validateSpec has rejected an empty
// spec.repo since ADR-0002, and a manifest that fails validation never becomes a
// playbook, so it cannot be empty here. The HELV_TEST_REPO_URL fallback and the
// hardcoded git-server URL that used to stand in for it were unreachable test
// scaffolding in the dispatch path; see BACKLOG 6.2.
func jobFor(pb playbook.Playbook, group manifest.NodeGroup) *Job {
	return &Job{
		JobID:        "job-" + pb.ID + "-" + group.Name,
		RepoURL:      pb.Manifest.Spec.Repo,
		Version:      pb.Manifest.Spec.Branch,
		PlaybookPath: pb.Manifest.Spec.Playbook,
		ExtraVars:    group.Ansible.ExtraVars,
	}
}

// injectedJob returns the job handed to NewServerWithJob, which lets a test
// exercise dispatch without a fleet repository. It is suppressed the moment any
// manifest is loaded, so a real fleet that selects nothing stays fail-loud
// instead of quietly serving a fixture.
//
// Caller holds mu.
func (s *Server) injectedJob() *Job {
	if s.currentJob.JobID == "" {
		return nil
	}

	for _, pb := range s.playbooks {
		if pb.Manifest != nil {
			return nil
		}
	}

	return &s.currentJob
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
