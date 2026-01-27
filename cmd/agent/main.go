package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

const (
	OthelaURL = "http://localhost:8080/api/v1"
	NodeID    = "agent-01"
	PollInterval = 5 * time.Second
)

type Job struct {
	JobID           string `json:"job_id"`
	PlaybookContent string `json:"playbook_content"`
}

type Report struct {
	NodeID   string          `json:"node_id"`
	JobID    string          `json:"job_id"`
	Status   string          `json:"status"`
	TaskLogs json.RawMessage `json:"task_log"`
}

var lastJobID string

func main() {
	log.Printf("Helvilette Agent (%s) started given Othela at %s", NodeID, OthelaURL)

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for range ticker.C {
		pollOthela()
	}
}

func pollOthela() {
	// 1. Fetch Job
	resp, err := http.Get(fmt.Sprintf("%s/sync/%s", OthelaURL, NodeID))
	if err != nil {
		log.Printf("[ERROR] Failed to connect to Othela: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[ERROR] Othela returned status: %s", resp.Status)
		return
	}

	var job Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		log.Printf("[ERROR] Failed to decode job: %v", err)
		return
	}

	// Simple check: Only run if it's a new job (in a real agent we might re-run based on policy)
	if job.JobID == lastJobID {
		// log.Println("[INFO] No new job. Waiting...") // Reduce noise
		return
	}

	log.Printf("[INFO] Received New Job: %s", job.JobID)
	lastJobID = job.JobID

	// 2. Execute Job
	status, output := executePlaybook(job.PlaybookContent)

	// 3. Report Results
	sendReport(job.JobID, status, output)
}

func executePlaybook(content string) (string, []byte) {
	tmpFile := fmt.Sprintf("/tmp/helvilette_job_%s.yml", lastJobID)
	if err := ioutil.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		return "Failed", []byte(fmt.Sprintf(`{"error": "Failed to write playbook: %v"}`, err))
	}

	cmd := exec.Command("ansible-playbook", "-i", "localhost,", "-c", "local", tmpFile)
	
	// CRITICAL: Set the callback to JSON for machine-readable output
	cmd.Env = append(os.Environ(), "ANSIBLE_STDOUT_CALLBACK=json")
	
	// We might also want to set ANSIBLE_LOAD_CALLBACK_PLUGINS=1 or ensure the plugin is available,
	// but usually 'json' is a built-in plugin in modern Ansible.

	log.Printf("[EXEC] Running Ansible Playbook...")
	output, err := cmd.CombinedOutput()
	
	status := "Success"
	if err != nil {
		log.Printf("[EXEC] Playbook failed: %v", err)
		status = "Failed"
	} else {
		log.Printf("[EXEC] Playbook completed successfully.")
	}

	// Verify if output is valid JSON. If Ansible crashes hard, it might not be JSON.
	if !json.Valid(output) {
		// Wrap non-JSON output in a JSON object so the report doesn't break
		safeOutput, _ := json.Marshal(map[string]string{
			"raw_output": string(output),
			"error": "Output was not valid JSON",
		})
		return status, safeOutput
	}

	return status, output
}

func sendReport(jobID, status string, output []byte) {
	report := Report{
		NodeID:   NodeID,
		JobID:    jobID,
		Status:   status,
		TaskLogs: json.RawMessage(output),
	}

	data, _ := json.Marshal(report)
	resp, err := http.Post(fmt.Sprintf("%s/report", OthelaURL), "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("[ERROR] Failed to send report: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[REPORT] Sent report for Job %s. Server replied: %s", jobID, resp.Status)
}
