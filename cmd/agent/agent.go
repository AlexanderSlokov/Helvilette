// The agent's lifecycle: registering with Othela, polling for work, reporting
// results, and watching the systemd units its playbooks manage.
//
// Split out of main.go, which exceeded the 500-line ceiling in AGENTS.md.
// See BACKLOG 6.7.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"helvilette/pkg/log"
	"helvilette/pkg/systemd"
	"helvilette/pkg/types"
)

// Agent represents the Helvilette agent
type Agent struct {
	config     AgentConfiguration
	lastJobID  string
	httpClient *http.Client
}

// Type aliases for backward compatibility within this package
type Job = types.Job
type Report = types.Report

// NewAgent creates a new agent with the given configuration
func NewAgent(config AgentConfiguration) *Agent {
	return &Agent{
		config:     config,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// RegisterNode registers this agent with Othela
func (a *Agent) RegisterNode(ctx context.Context) error {
	logger := log.WithComponent("agent").With().Str("node_id", a.config.NodeID).Logger()
	url := fmt.Sprintf("%s/nodes/register", a.config.OthelaURL)

	reqBody := struct {
		NodeID string            `json:"node_id"`
		Labels map[string]string `json:"labels"`
	}{
		NodeID: a.config.NodeID,
		Labels: a.config.Labels,
	}

	data, _ := json.Marshal(reqBody)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			resp, err := a.httpClient.Post(url, "application/json", bytes.NewReader(data))
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				logger.Info().Msg("Successfully registered with Othela")
				return nil
			}

			status := "unknown"
			if resp != nil {
				status = resp.Status
				resp.Body.Close()
			}

			logger.Warn().Err(err).Str("status", status).Msg("Failed to register with Othela, retrying in 5s...")

			// Wait before retrying, but allow context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
}

// Poll fetches a job from Othela and returns it
func (a *Agent) Poll() (*Job, error) {
	url := fmt.Sprintf("%s/sync/%s", a.config.OthelaURL, a.config.NodeID)
	resp, err := a.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Othela: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("conflict (409): %s", string(body))
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Othela returned status %d: %s", resp.StatusCode, string(body))
	}

	var job Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("failed to decode job: %w", err)
	}

	return &job, nil
}

// statusFilePath returns the path to the node's local status file
func (a *Agent) statusFilePath() string {
	return filepath.Join(a.config.WorkspaceDir, "last_run_summary.json")
}

// readStatus reads the last persisted status from disk
func (a *Agent) readStatus() (*types.NodeStatus, error) {
	data, err := os.ReadFile(a.statusFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var status types.NodeStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// writeStatus persists the node's current status to disk
func (a *Agent) writeStatus(status types.NodeStatus) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	// Ensure workspace exists
	if err := os.MkdirAll(a.config.WorkspaceDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(a.statusFilePath(), data, 0644)
}

// SendReport sends an execution report to Othela
func (a *Agent) SendReport(report Report) error {
	url := fmt.Sprintf("%s/report", a.config.OthelaURL)
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	resp, err := a.httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("othela returned with status: %s", resp.Status)
	}

	return nil
}

// ProcessJob handles a job from Othela
func (a *Agent) ProcessJob(job *Job) error {
	if job == nil {
		return nil
	}

	logger := log.WithComponent("agent")

	// Check if this is a new job
	if job.JobID == a.lastJobID {
		return nil // No new job
	}

	logger.Info().
		Str("job_id", job.JobID).
		Bool("has_path", job.PlaybookPath != "").
		Msg("processing new job")

	// 1. Write InProgress status to disk BEFORE executing
	nodeStatus := types.NodeStatus{
		JobID:     job.JobID,
		CommitSHA: job.Version,
		Status:    "InProgress",
		AppliedAt: time.Now(),
	}
	if err := a.writeStatus(nodeStatus); err != nil {
		logger.Error().Err(err).Msg("failed to write initial status to disk")
	}

	// Execute the playbook
	status, output := a.ExecutePlaybook(job)

	// Only consider the job processed if it didn't fail due to initial fetching
	if status == "Success" {
		a.lastJobID = job.JobID
	}

	// 2. Update status AFTER execution
	nodeStatus.Status = status
	if err := a.writeStatus(nodeStatus); err != nil {
		logger.Error().Err(err).Msg("failed to write final status to disk")
	}

	// Send report
	report := Report{
		NodeID:     a.config.NodeID,
		JobID:      job.JobID,
		Status:     status,
		TaskLogs:   json.RawMessage(output),
		ObservedAt: time.Now(),
		NodeStatus: nodeStatus,
	}

	logger.Info().
		Str("job_id", job.JobID).
		Str("status", status).
		Msg("sending report to Othela")

	return a.SendReport(report)
}

// Run starts the agent main loop
func (a *Agent) Run(ctx context.Context) error {
	logger := log.WithComponent("agent").With().Str("node_id", a.config.NodeID).Logger()

	logger.Info().
		Str("othela_url", a.config.OthelaURL).
		Dur("poll_interval", a.config.PollInterval).
		Str("workspace_dir", a.config.WorkspaceDir).
		Interface("labels", a.config.Labels).
		Msg("Helvilette Agent started")

	// First register the node
	if err := a.RegisterNode(ctx); err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// Check for interrupted jobs
	lastStatus, err := a.readStatus()
	if err != nil {
		logger.Warn().Err(err).Msg("failed to read previous status")
	} else if lastStatus != nil && lastStatus.Status == "InProgress" {
		logger.Warn().Str("job_id", lastStatus.JobID).Msg("detected interrupted job, reporting failure")

		lastStatus.Status = "Failed (Interrupted)"
		if err := a.writeStatus(*lastStatus); err != nil {
			logger.Error().Err(err).Msg("failed to update status of interrupted job")
		}

		failReport := Report{
			NodeID:     a.config.NodeID,
			JobID:      lastStatus.JobID,
			Status:     "Failed",
			TaskLogs:   json.RawMessage(`{"error": "job interrupted by agent reboot/disconnect"}`),
			ObservedAt: time.Now(),
			NodeStatus: *lastStatus,
		}

		if err := a.SendReport(failReport); err != nil {
			logger.Error().Err(err).Msg("failed to send report for interrupted job")
		} else {
			a.lastJobID = lastStatus.JobID
		}
	} else if lastStatus != nil {
		a.lastJobID = lastStatus.JobID
	}

	// Initialize systemd client
	sdClient, err := systemd.NewClient()
	if err != nil {
		logger.Warn().Err(err).Msg("failed to connect to systemd D-Bus, systemd watching disabled")
	} else {
		defer sdClient.Close()
		logger.Info().Msg("connected to systemd D-Bus")

		watcher := systemd.NewWatcher(sdClient, []string{})
		eventChan, err := watcher.Watch(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("failed to start systemd watcher")
		} else {
			go a.handleSystemdEvents(ctx, eventChan)
		}
	}

	ticker := time.NewTicker(a.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			job, err := a.Poll()
			if err != nil {
				logger.Error().Err(err).Msg("failed to poll Othela")
				continue
			}
			if err := a.ProcessJob(job); err != nil {
				logger.Error().Err(err).Msg("failed to process job")
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (a *Agent) handleSystemdEvents(ctx context.Context, eventChan <-chan systemd.UnitEvent) {
	logger := log.WithComponent("agent").With().Str("node_id", a.config.NodeID).Logger()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			logger.Info().
				Str("unit", event.Unit.Name).
				Str("active_state", event.Unit.ActiveState).
				Str("sub_state", event.Unit.SubState).
				Str("event_type", event.EventType).
				Msg("systemd unit event")
		}
	}
}
