// Tests for agent.go: polling Othela, reporting back, and declining to repeat a
// job the node has already run.
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewAgent(t *testing.T) {
	config := AgentConfiguration{
		OthelaURL:    "http://test:8080/api/v1",
		NodeID:       "test-agent",
		PollInterval: 0,
	}

	agent := NewAgent(config)

	if agent == nil {
		t.Fatal("NewAgent returned nil")
	}

	if agent.config.OthelaURL != config.OthelaURL {
		t.Errorf("OthelaURL = %q, want %q", agent.config.OthelaURL, config.OthelaURL)
	}

	if agent.config.NodeID != config.NodeID {
		t.Errorf("NodeID = %q, want %q", agent.config.NodeID, config.NodeID)
	}
}

func TestAgent_Poll_Success(t *testing.T) {
	// Create mock server
	expectedJob := Job{
		JobID:        "test-job-123",
		RepoURL:      "git://git-server:9418/nginx-collection",
		Version:      "main",
		PlaybookPath: "playbook.yml",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sync/test-agent" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedJob)
	}))
	defer server.Close()

	config := AgentConfiguration{
		OthelaURL: server.URL + "/api/v1",
		NodeID:    "test-agent",
	}
	agent := NewAgent(config)

	job, err := agent.Poll()
	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}

	if job.JobID != expectedJob.JobID {
		t.Errorf("JobID = %q, want %q", job.JobID, expectedJob.JobID)
	}

	if job.RepoURL != expectedJob.RepoURL {
		t.Errorf("RepoURL = %q, want %q", job.RepoURL, expectedJob.RepoURL)
	}

	if job.PlaybookPath != expectedJob.PlaybookPath {
		t.Errorf("PlaybookPath = %q, want %q", job.PlaybookPath, expectedJob.PlaybookPath)
	}
}

func TestAgent_Poll_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := AgentConfiguration{
		OthelaURL: server.URL + "/api/v1",
		NodeID:    "test-agent",
	}
	agent := NewAgent(config)

	_, err := agent.Poll()
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestAgent_Poll_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	config := AgentConfiguration{
		OthelaURL: server.URL + "/api/v1",
		NodeID:    "test-agent",
	}
	agent := NewAgent(config)

	_, err := agent.Poll()
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestAgent_Poll_ConnectionError(t *testing.T) {
	config := AgentConfiguration{
		OthelaURL: "http://localhost:99999/api/v1", // Invalid port
		NodeID:    "test-agent",
	}
	agent := NewAgent(config)

	_, err := agent.Poll()
	if err == nil {
		t.Error("expected error for connection failure")
	}
}

func TestAgent_SendReport_Success(t *testing.T) {
	var receivedReport Report

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/report" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		json.NewDecoder(r.Body).Decode(&receivedReport)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := AgentConfiguration{
		OthelaURL: server.URL + "/api/v1",
		NodeID:    "test-agent",
	}
	agent := NewAgent(config)

	report := Report{
		NodeID:   "test-agent",
		JobID:    "job-123",
		Status:   "Success",
		TaskLogs: json.RawMessage(`{"result": "ok"}`),
	}

	err := agent.SendReport(report)
	if err != nil {
		t.Fatalf("SendReport failed: %v", err)
	}

	if receivedReport.NodeID != report.NodeID {
		t.Errorf("NodeID = %q, want %q", receivedReport.NodeID, report.NodeID)
	}

	if receivedReport.JobID != report.JobID {
		t.Errorf("JobID = %q, want %q", receivedReport.JobID, report.JobID)
	}
}

func TestAgent_SendReport_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := AgentConfiguration{
		OthelaURL: server.URL + "/api/v1",
		NodeID:    "test-agent",
	}
	agent := NewAgent(config)

	report := Report{
		NodeID: "test-agent",
		JobID:  "job-123",
		Status: "Success",
	}

	err := agent.SendReport(report)
	if err == nil {
		t.Error("expected error for server error response")
	}
}

// TestAgent_ExecutePlaybook_RejectsEmptyJob verifies that a job with neither
// RepoURL nor PlaybookPath is rejected with a clear error. Before issue #25
// this branch silently wrote empty PlaybookContent to a temp file.

func TestAgent_ProcessJob_SkipsSameJob(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := AgentConfiguration{
		OthelaURL: server.URL + "/api/v1",
		NodeID:    "test-agent",
	}
	agent := NewAgent(config)

	// Set lastJobID to same as incoming job
	agent.lastJobID = "job-123"

	job := &Job{
		JobID:        "job-123",
		RepoURL:      "git://git-server:9418/test",
		PlaybookPath: "playbook.yml",
	}

	err := agent.ProcessJob(job)
	if err != nil {
		t.Fatalf("ProcessJob failed: %v", err)
	}

	// Should not have made any HTTP calls since job was skipped
	if callCount != 0 {
		t.Errorf("expected 0 HTTP calls for same job, got %d", callCount)
	}
}
