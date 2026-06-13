package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"helvilette/pkg/playbook"
	"helvilette/pkg/types"
)

// Type aliases for backward compatibility within this package
type Job = types.Job
type Report = types.Report

// Server represents the Othela control plane server
type Server struct {
	router     *mux.Router
	loader     *playbook.Loader
	currentJob Job
	reports    []Report
	mu         sync.RWMutex
}

// NewServer creates a new Othela server with default configuration
func NewServer() *Server {
	s := &Server{
		router:  mux.NewRouter(),
		reports: make([]Report, 0),
	}

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
		router:  mux.NewRouter(),
		loader:  loader,
		reports: make([]Report, 0),
	}

	// Try to load first available playbook
	playbooks, err := loader.Scan()
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
		router:     mux.NewRouter(),
		currentJob: job,
		reports:    make([]Report, 0),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.HandleFunc("/api/v1/sync/{node_id}", s.handleSync).Methods("GET")
	s.router.HandleFunc("/api/v1/report", s.handleReport).Methods("POST")
	s.router.HandleFunc("/api/v1/playbooks", s.handlePlaybooks).Methods("GET")
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

// GetReports returns all received reports
func (s *Server) GetReports() []Report {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Report{}, s.reports...)
}

// handleSync handles the sync endpoint - Agent polls for work
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["node_id"]

	log.Printf("[SYNC] Node %s is asking for work...", nodeID)

	s.mu.RLock()
	job := s.currentJob
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// handleReport handles the report endpoint - Agent sends back results
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	var report Report
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.reports = append(s.reports, report)
	s.mu.Unlock()

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

