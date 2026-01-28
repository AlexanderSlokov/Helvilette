package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// Job represents a job that Othela sends to Agents
type Job struct {
	JobID           string `json:"job_id"`
	PlaybookContent string `json:"playbook_content"`
}

// Report represents the execution report from an Agent
type Report struct {
	NodeID   string          `json:"node_id"`
	JobID    string          `json:"job_id"`
	Status   string          `json:"status"`
	TaskLogs json.RawMessage `json:"task_log"`
}

// Server represents the Othela control plane server
type Server struct {
	router     *mux.Router
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
