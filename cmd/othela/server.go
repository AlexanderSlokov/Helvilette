package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"

	"helvilette/pkg/manifest"
	"helvilette/pkg/playbook"
	"helvilette/pkg/storage"
	"helvilette/pkg/types"
)

// Type aliases for backward compatibility within this package
type Job = types.Job
type Report = types.Report

// Server represents the Othela control plane server
type Server struct {
	router      *mux.Router
	loader      *playbook.Loader
	currentJob  Job // fallback / testing
	playbooks   []playbook.Playbook
	nodeStore   storage.NodeStore
	reportStore storage.ReportStore
	mu          sync.RWMutex  // protects currentJob and playbooks only
	ready       atomic.Bool   // readiness probe state
	debugMode   bool
}

// NewServer creates a new Othela server with default configuration
func NewServer() *Server {
	s := &Server{
		router:      mux.NewRouter(),
		nodeStore:   storage.NewMemoryNodeStore(),
		reportStore: storage.NewMemoryReportStore(),
	}
	s.ready.Store(true)

	// Initialize mock job
	s.currentJob = Job{
		JobID: "job-" + fmt.Sprintf("%d", time.Now().Unix()),
		PlaybookContent: `
- name: Helvilette Sanity Check
  hosts: localhost
  connection: local
  gather_facts: no
  tasks:
    - name: Say Hello
      debug:
        msg: "Hello Wunjo! This is Helvilette Othela speaking."
`,
	}

	s.setupRoutes()
	return s
}

// NewServerWithLoader creates a new Othela server with a playbook loader
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

	// Try to load first available playbook as fallback
	if err == nil && len(playbooks) > 0 {
		content, err := loader.Load(playbooks[0].ID)
		if err == nil {
			repoURL := os.Getenv("HELV_TEST_REPO_URL")
			if repoURL == "" {
				repoURL = "http://git-server:3000/helvilette/nginx-collection.git"
			}
			s.currentJob = Job{
				JobID:           "job-" + playbooks[0].ID,
				RepoURL:         repoURL,
				Version:         "main",
				PlaybookPath:    "playbook.yml",
				PlaybookContent: content, // fallback
			}
			log.Printf("[LOADER] Mocked GitOps Job with RepoURL: %s", s.currentJob.RepoURL)
		}
	}

	// Fallback to mock job if no playbooks found
	if s.currentJob.JobID == "" {
		s.currentJob = Job{
			JobID: "job-" + fmt.Sprintf("%d", time.Now().Unix()),
			PlaybookContent: `
- name: Helvilette Fallback Job
  hosts: localhost
  connection: local
  gather_facts: no
  tasks:
    - name: No playbooks found
      debug:
        msg: "No playbooks available in data/playbooks/"
`,
		}
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
	DebugMode   bool
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
		debugMode:   cfg.DebugMode,
	}
	s.ready.Store(true)

	// If loader is provided, scan playbooks (same logic as NewServerWithLoader)
	if cfg.Loader != nil {
		playbooks, err := cfg.Loader.Scan()
		if err == nil {
			s.playbooks = playbooks
		}

		if err == nil && len(playbooks) > 0 {
			content, loadErr := cfg.Loader.Load(playbooks[0].ID)
			if loadErr == nil {
				repoURL := os.Getenv("HELV_TEST_REPO_URL")
				if repoURL == "" {
					repoURL = "http://git-server:3000/helvilette/nginx-collection.git"
				}
				s.currentJob = Job{
					JobID:           "job-" + playbooks[0].ID,
					RepoURL:         repoURL,
					Version:         "main",
					PlaybookPath:    "playbook.yml",
					PlaybookContent: content,
				}
				log.Printf("[LOADER] Mocked GitOps Job with RepoURL: %s", s.currentJob.RepoURL)
			}
		}

		if s.currentJob.JobID == "" {
			s.currentJob = Job{
				JobID: "job-" + fmt.Sprintf("%d", time.Now().Unix()),
				PlaybookContent: `
- name: Helvilette Fallback Job
  hosts: localhost
  connection: local
  gather_facts: no
  tasks:
    - name: No playbooks found
      debug:
        msg: "No playbooks available in data/playbooks/"
`,
			}
		}
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.HandleFunc("/api/v1/nodes/register", s.handleRegisterNode).Methods("POST")
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

// GetReports returns all received reports.
func (s *Server) GetReports() []Report {
	reports, _ := s.reportStore.List()
	return reports
}

// SetDebug enables or disables debug logging
func (s *Server) SetDebug(debug bool) {
	s.debugMode = debug
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
	
	log.Printf("[REGISTER] Node %s registered with labels %v", req.NodeID, req.Labels)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(req)
}

// handleSync handles the sync endpoint - Agent polls for work
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["node_id"]

	if s.debugMode {
		log.Printf("[DEBUG] [SYNC] Node %s is asking for work...", nodeID)
	}

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
			group := matches[0] // take the first match
			
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
		if s.debugMode {
			log.Printf("[DEBUG] Node %s has labels %v, but no nodeSelectors matched", nodeID, labels)
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matchedJob)
}

// handleReport handles the report endpoint - Agent sends back results
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	var report Report
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.reportStore.Save(report); err != nil {
		log.Printf("[ERROR] Failed to save report: %v", err)
	}

	log.Printf("---------------------------------------------------")
	log.Printf("[REPORT] Received Report from Node: %s, Job: %s", report.NodeID, report.JobID)
	log.Printf("[REPORT] Status: %s", report.Status)
	log.Printf("[REPORT] Full Output (JSON):\n%s", string(report.TaskLogs))
	log.Printf("---------------------------------------------------")

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Report received")
}

// ListenAndServe starts the server on the specified address
func (s *Server) ListenAndServe(addr string) error {
	log.Printf("Othela Control Plane is listening on %s...", addr)
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
	if s.loader == nil {
		http.Error(w, "Playbook loader not configured", http.StatusServiceUnavailable)
		return
	}

	playbooks, err := s.loader.Scan()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[PLAYBOOKS] Returning %d playbooks", len(playbooks))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(playbooks)
}

// GetLoader returns the playbook loader (for testing)
func (s *Server) GetLoader() *playbook.Loader {
	return s.loader
}
