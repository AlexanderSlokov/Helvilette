package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func registerNode(server *Server, nodeID string, labels map[string]string) {
	reqBody := NodeRegistration{
		NodeID: nodeID,
		Labels: labels,
	}
	data, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/nodes/register", bytes.NewReader(data))
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)
}

func TestSyncEndpoint(t *testing.T) {
	job := Job{
		JobID:           "test-job-123",
		PlaybookContent: "test playbook content",
	}
	server := NewServerWithJob(job)

	// Must register node first
	registerNode(server, "agent-01", nil)

	req := httptest.NewRequest("GET", "/api/v1/sync/agent-01", nil)
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var resultJob Job
	if err := json.NewDecoder(w.Body).Decode(&resultJob); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resultJob.JobID != job.JobID {
		t.Errorf("expected JobID %q, got %q", job.JobID, resultJob.JobID)
	}

	if resultJob.PlaybookContent != job.PlaybookContent {
		t.Errorf("expected PlaybookContent %q, got %q", job.PlaybookContent, resultJob.PlaybookContent)
	}
}

func TestSyncEndpoint_DifferentNodeIDs(t *testing.T) {
	server := NewServerWithJob(Job{JobID: "job-1", PlaybookContent: "content"})

	nodeIDs := []string{"node-1", "node-2", "agent-alpha"}
	for _, nodeID := range nodeIDs {
		// Must register node first
		registerNode(server, nodeID, nil)

		t.Run(nodeID, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/sync/"+nodeID, nil)
			w := httptest.NewRecorder()

			server.Router().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200 for node %s, got %d", nodeID, w.Code)
			}
		})
	}
}

func TestReportEndpoint(t *testing.T) {
	server := NewServer()

	report := Report{
		NodeID:   "agent-01",
		JobID:    "job-123",
		Status:   "Success",
		TaskLogs: json.RawMessage(`{"result": "ok"}`),
	}

	body, _ := json.Marshal(report)
	req := httptest.NewRequest("POST", "/api/v1/report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify report was stored
	reports := server.GetReports()
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}

	if reports[0].NodeID != report.NodeID {
		t.Errorf("expected NodeID %q, got %q", report.NodeID, reports[0].NodeID)
	}

	if reports[0].JobID != report.JobID {
		t.Errorf("expected JobID %q, got %q", report.JobID, reports[0].JobID)
	}

	if reports[0].Status != report.Status {
		t.Errorf("expected Status %q, got %q", report.Status, reports[0].Status)
	}
}

func TestReportEndpoint_InvalidJSON(t *testing.T) {
	server := NewServer()

	req := httptest.NewRequest("POST", "/api/v1/report", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d", w.Code)
	}
}

func TestReportEndpoint_EmptyBody(t *testing.T) {
	server := NewServer()

	req := httptest.NewRequest("POST", "/api/v1/report", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty body, got %d", w.Code)
	}
}

func TestMultipleReports(t *testing.T) {
	server := NewServer()

	reports := []Report{
		{NodeID: "node-1", JobID: "job-1", Status: "Success", TaskLogs: json.RawMessage(`{}`)},
		{NodeID: "node-2", JobID: "job-2", Status: "Failed", TaskLogs: json.RawMessage(`{}`)},
		{NodeID: "node-1", JobID: "job-3", Status: "Success", TaskLogs: json.RawMessage(`{}`)},
	}

	for _, report := range reports {
		body, _ := json.Marshal(report)
		req := httptest.NewRequest("POST", "/api/v1/report", bytes.NewReader(body))
		w := httptest.NewRecorder()
		server.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("failed to submit report: %d", w.Code)
		}
	}

	storedReports := server.GetReports()
	if len(storedReports) != 3 {
		t.Errorf("expected 3 reports, got %d", len(storedReports))
	}
}

func TestSetCurrentJob(t *testing.T) {
	server := NewServer()

	newJob := Job{
		JobID:           "new-job-456",
		PlaybookContent: "new content",
	}
	server.SetCurrentJob(newJob)

	// Must register node first
	registerNode(server, "test-agent", nil)

	// Verify through sync endpoint
	req := httptest.NewRequest("GET", "/api/v1/sync/test-agent", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	var resultJob Job
	json.NewDecoder(w.Body).Decode(&resultJob)

	if resultJob.JobID != newJob.JobID {
		t.Errorf("expected JobID %q after SetCurrentJob, got %q", newJob.JobID, resultJob.JobID)
	}
}
