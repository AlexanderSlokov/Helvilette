package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.OthelaURL != "http://localhost:8080/api/v1" {
		t.Errorf("unexpected default OthelaURL: %s", config.OthelaURL)
	}

	if config.NodeID != "agent-01" {
		t.Errorf("unexpected default NodeID: %s", config.NodeID)
	}

	if config.PollInterval.Seconds() != 5 {
		t.Errorf("unexpected default PollInterval: %v", config.PollInterval)
	}
}

func TestAgent_Poll_Success(t *testing.T) {
	// Create mock server
	expectedJob := Job{
		JobID:           "test-job-123",
		PlaybookContent: "test content",
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

	if job.PlaybookContent != expectedJob.PlaybookContent {
		t.Errorf("PlaybookContent = %q, want %q", job.PlaybookContent, expectedJob.PlaybookContent)
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

func TestAgent_ExecutePlaybook_WritesFile(t *testing.T) {
	agent := NewAgent(DefaultConfig())

	job := &Job{
		JobID:           "test-write-123",
		PlaybookContent: "test playbook content",
	}

	// This will fail because ansible-playbook is not available in test
	// but we can verify the file was written
	agent.ExecutePlaybook(job)

	tmpFile := filepath.Join(agent.config.WorkspaceDir, "helvilette_job_"+job.JobID+".yml")
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read playbook file: %v", err)
	}

	if string(data) != job.PlaybookContent {
		t.Errorf("file content = %q, want %q", string(data), job.PlaybookContent)
	}

	// Cleanup
	os.Remove(tmpFile)
}

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
		JobID:           "job-123",
		PlaybookContent: "content",
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

func TestParseLabels(t *testing.T) {
	labels := parseLabels("role=web, env=prod, region=us-east-1")
	if len(labels) != 3 {
		t.Errorf("expected 3 labels, got %d", len(labels))
	}
	if labels["role"] != "web" {
		t.Errorf("expected role=web, got %s", labels["role"])
	}
	if labels["env"] != "prod" {
		t.Errorf("expected env=prod, got %s", labels["env"])
	}
}

func TestLoadConfig_CLI_Overrides(t *testing.T) {
	config, err := LoadConfig("", "http://cli:8080", "cli-node", "10s", "/tmp/cli", "role=db")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	
	if config.OthelaURL != "http://cli:8080/api/v1" {
		t.Errorf("expected http://cli:8080/api/v1, got %s", config.OthelaURL)
	}
	if config.NodeID != "cli-node" {
		t.Errorf("expected cli-node, got %s", config.NodeID)
	}
	if config.Labels["role"] != "db" {
		t.Errorf("expected role=db, got %s", config.Labels["role"])
	}
}
