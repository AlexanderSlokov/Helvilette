package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// Define the Job structure that Othela sends to Agents
type Job struct {
	JobID           string `json:"job_id"`
	PlaybookContent string `json:"playbook_content"`
}

// Define the Report structure that Agents send back
type Report struct {
	NodeID   string          `json:"node_id"`
	JobID    string          `json:"job_id"`
	Status   string          `json:"status"`
	TaskLogs json.RawMessage `json:"task_log"` // Keep raw JSON to print flexibly
}

// In-memory store for our mock job
var currentJob Job

func main() {
	// Initialize our mock job with a simple Ansible Playbook
	currentJob = Job{
		JobID: "job-" + fmt.Sprintf("%d", time.Now().Unix()),
		// A simple playbook that prints "Hello Wunjo!" using the debug module
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

	r := mux.NewRouter()

	// API Endpoint 1: Sync (Agent polls for work)
	r.HandleFunc("/api/v1/sync/{node_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		nodeID := vars["node_id"]
		
		log.Printf("[SYNC] Node %s is asking for work...", nodeID)

		// In a real system, we would lookup specific jobs for this node.
		// Here we just return the global mock job.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(currentJob)
	}).Methods("GET")

	// API Endpoint 2: Report (Agent sends back results)
	r.HandleFunc("/api/v1/report", func(w http.ResponseWriter, r *http.Request) {
		var report Report
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("---------------------------------------------------")
		log.Printf("[REPORT] Received Report from Node: %s, Job: %s", report.NodeID, report.JobID)
		log.Printf("[REPORT] Status: %s", report.Status)
		log.Printf("[REPORT] Full Output (JSON):\n%s", string(report.TaskLogs))
		log.Printf("---------------------------------------------------")

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Report received")
	}).Methods("POST")

	srvAddr := ":8080"
	log.Printf("Othela Control Plane is listening on %s...", srvAddr)
	log.Fatal(http.ListenAndServe(srvAddr, r))
}
