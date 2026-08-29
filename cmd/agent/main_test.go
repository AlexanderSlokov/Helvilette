package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	// The default is the hostname, so that nodes stay distinguishable when nodeID is
	// never configured. Only an unavailable hostname falls back to a static value.
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		if config.NodeID != hostname {
			t.Errorf("unexpected default NodeID: got %q, want the hostname %q", config.NodeID, hostname)
		}
	} else if config.NodeID != fallbackNodeID {
		t.Errorf("unexpected default NodeID: got %q, want %q", config.NodeID, fallbackNodeID)
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
	config, _, err := LoadConfig("", "http://cli:8080", "cli-node", "10s", "/tmp/cli", "role=db")
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

// writeConfigFile writes a YAML config into a temp dir and returns its path.
func writeConfigFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	return path
}

// The config file is an explicit, version-controlled artifact and outranks ambient
// environment variables. See docs/informations/ADRs/ADR-0001.md.
func TestLoadConfig_YAMLOverridesEnv(t *testing.T) {
	t.Setenv("OTHELA_URL", "http://from-env:8080")
	t.Setenv("NODE_ID", "env-node")
	t.Setenv("POLL_INTERVAL", "30s")
	t.Setenv("WORKSPACE_DIR", "/tmp/env")

	path := writeConfigFile(t, `othelaURL: "http://from-file:8080"
nodeID: "file-node"
pollInterval: "7s"
workspaceDir: "/tmp/file"
`)

	config, _, err := LoadConfig(path, "", "", "", "", "")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.OthelaURL != "http://from-file:8080/api/v1" {
		t.Errorf("expected the config file to win, got %s", config.OthelaURL)
	}
	if config.NodeID != "file-node" {
		t.Errorf("expected file-node, got %s", config.NodeID)
	}
	if config.PollInterval.String() != "7s" {
		t.Errorf("expected 7s, got %s", config.PollInterval)
	}
	if config.WorkspaceDir != "/tmp/file" {
		t.Errorf("expected /tmp/file, got %s", config.WorkspaceDir)
	}
}

// Env still fills in whatever the config file leaves unset.
func TestLoadConfig_EnvFillsGapsInYAML(t *testing.T) {
	t.Setenv("NODE_ID", "env-node")
	t.Setenv("POLL_INTERVAL", "30s")

	path := writeConfigFile(t, `othelaURL: "http://from-file:8080"
`)

	config, _, err := LoadConfig(path, "", "", "", "", "")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.OthelaURL != "http://from-file:8080/api/v1" {
		t.Errorf("expected http://from-file:8080/api/v1, got %s", config.OthelaURL)
	}
	if config.NodeID != "env-node" {
		t.Errorf("expected env-node to survive, got %s", config.NodeID)
	}
	if config.PollInterval.String() != "30s" {
		t.Errorf("expected 30s to survive, got %s", config.PollInterval)
	}
}

// CLI flags stay the highest-priority source, above both the file and the environment.
func TestLoadConfig_CLIOverridesYAMLAndEnv(t *testing.T) {
	t.Setenv("OTHELA_URL", "http://from-env:8080")
	t.Setenv("NODE_ID", "env-node")

	path := writeConfigFile(t, `othelaURL: "http://from-file:8080"
nodeID: "file-node"
`)

	config, _, err := LoadConfig(path, "http://from-cli:8080", "cli-node", "", "", "")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.OthelaURL != "http://from-cli:8080/api/v1" {
		t.Errorf("expected the CLI flag to win, got %s", config.OthelaURL)
	}
	if config.NodeID != "cli-node" {
		t.Errorf("expected cli-node, got %s", config.NodeID)
	}
}

// Labels merge per key across sources, with the higher-priority source winning
// only the keys it actually sets.
func TestLoadConfig_LabelsMergePerKey(t *testing.T) {
	t.Setenv("AGENT_LABELS", "env=production,region=eu-west,owner=sre")

	path := writeConfigFile(t, `labels:
  role: "edge-proxy"
  region: "us-east"
`)

	config, _, err := LoadConfig(path, "", "", "", "", "role=db")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Only set in the environment, so it survives.
	if config.Labels["owner"] != "sre" {
		t.Errorf("expected owner=sre to survive from env, got %q", config.Labels["owner"])
	}
	if config.Labels["env"] != "production" {
		t.Errorf("expected env=production to survive from env, got %q", config.Labels["env"])
	}
	// Set in both env and file: the file wins.
	if config.Labels["region"] != "us-east" {
		t.Errorf("expected the file to win region, got %q", config.Labels["region"])
	}
	// Set in the file and on the CLI: the CLI wins.
	if config.Labels["role"] != "db" {
		t.Errorf("expected the CLI to win role, got %q", config.Labels["role"])
	}
}

// A misspelled key must fail at startup instead of leaving the agent on defaults.
func TestLoadConfig_UnknownKeyIsRejected(t *testing.T) {
	// The exact spelling that shipped in the README before this was fixed.
	path := writeConfigFile(t, `otherlaUrl: "http://othela-server:8080/api/v1"
nodeId: "node-01"
`)

	_, _, err := LoadConfig(path, "", "", "", "", "")
	if err == nil {
		t.Fatal("expected unknown config keys to be rejected, got no error")
	}
}

// Provenance must name the source that actually won each value, so that an operator can
// explain a node's behaviour from its logs alone.
func TestLoadConfig_ProvenanceReportsWinningSource(t *testing.T) {
	t.Setenv("OTHELA_URL", "http://from-env:8080")
	t.Setenv("NODE_ID", "env-node")
	t.Setenv("AGENT_LABELS", "owner=sre,region=eu-west")

	path := writeConfigFile(t, `nodeID: "file-node"
workspaceDir: "/tmp/file"
labels:
  region: "us-east"
  role: "edge-proxy"
`)

	_, provenance, err := LoadConfig(path, "", "", "", "", "role=db")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	want := map[string]string{
		"othelaURL":     "env(OTHELA_URL)",   // only the environment set it
		"nodeID":        SourceConfigFile,    // file beat the environment
		"workspaceDir":  SourceConfigFile,    // only the file set it
		"pollInterval":  SourceDefault,       // nothing set it
		"labels.owner":  "env(AGENT_LABELS)", // only the environment set it
		"labels.region": SourceConfigFile,    // file beat the environment
		"labels.role":   "cli(--labels)",     // CLI beat the file
	}
	for field, wantSource := range want {
		if provenance[field] != wantSource {
			t.Errorf("provenance[%q] = %q, want %q", field, provenance[field], wantSource)
		}
	}
}

// When nodeID falls through to the hostname, the provenance must say so rather than
// reporting a bare "default", since the two have very different implications for a fleet.
func TestLoadConfig_ProvenanceDistinguishesHostnameDefault(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		t.Skip("hostname unavailable on this machine")
	}

	config, provenance, err := LoadConfig("", "", "", "", "", "")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.NodeID != hostname {
		t.Errorf("expected nodeID to default to the hostname %q, got %q", hostname, config.NodeID)
	}
	if provenance["nodeID"] != SourceDefaultHostname {
		t.Errorf("provenance[nodeID] = %q, want %q", provenance["nodeID"], SourceDefaultHostname)
	}
}

func TestFormatConfig(t *testing.T) {
	config := AgentConfiguration{
		OthelaURL:    "http://othela:8080/api/v1",
		NodeID:       "node-01",
		PollInterval: 5 * time.Second,
		WorkspaceDir: "/var/lib/helvilette/workspace",
		Labels:       map[string]string{"role": "edge-proxy", "env": "production"},
	}
	provenance := ConfigProvenance{
		"othelaURL":    SourceConfigFile,
		"nodeID":       sourceEnv("NODE_ID"),
		"workspaceDir": SourceConfigFile,
		"labels.role":  SourceConfigFile,
		// pollInterval and labels.env are deliberately absent.
	}

	out := FormatConfig(config, provenance)

	for _, want := range []string{
		"othelaURL    = http://othela:8080/api/v1",
		"source=config-file",
		"nodeID       = node-01",
		"source=env(NODE_ID)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	// A field with no recorded source must still be reported, not silently dropped.
	if !strings.Contains(out, "pollInterval = 5s") {
		t.Errorf("expected pollInterval to be reported, got:\n%s", out)
	}

	// Labels are sorted so two runs, or two nodes, can be diffed directly.
	envAt, roleAt := strings.Index(out, "labels.env"), strings.Index(out, "labels.role")
	if envAt == -1 || roleAt == -1 {
		t.Fatalf("expected both labels to be reported, got:\n%s", out)
	}
	if envAt > roleAt {
		t.Errorf("expected labels to be sorted, got:\n%s", out)
	}
}

// An empty config file is valid and leaves the lower-priority sources intact.
func TestLoadConfig_EmptyFileIsNotAnError(t *testing.T) {
	t.Setenv("NODE_ID", "env-node")

	path := writeConfigFile(t, "")

	config, _, err := LoadConfig(path, "", "", "", "", "")
	if err != nil {
		t.Fatalf("expected an empty config file to be accepted, got: %v", err)
	}
	if config.NodeID != "env-node" {
		t.Errorf("expected env-node, got %s", config.NodeID)
	}
}
