package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"helvilette/pkg/log"
	"helvilette/pkg/systemd"
	"helvilette/pkg/types"
)

// AgentConfig holds configuration for the agent
type AgentConfig struct {
	OthelaURL    string
	NodeID       string
	PollInterval time.Duration
}

// DefaultConfig returns the default agent configuration
func DefaultConfig() AgentConfig {
	return AgentConfig{
		OthelaURL:    "http://localhost:8080/api/v1",
		NodeID:       "agent-01",
		PollInterval: 5 * time.Second,
	}
}

// Agent represents the Helvilette agent
type Agent struct {
	config     AgentConfig
	lastJobID  string
	httpClient *http.Client
}

// Type aliases for backward compatibility within this package
type Job = types.Job
type Report = types.Report

// NewAgent creates a new agent with the given configuration
func NewAgent(config AgentConfig) *Agent {
	return &Agent{
		config:     config,
		httpClient: &http.Client{Timeout: 10 * time.Second},
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Othela returned status: %s", resp.Status)
	}

	var job Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("failed to decode job: %w", err)
	}

	return &job, nil
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
		return fmt.Errorf("Othela returned status: %s", resp.Status)
	}

	return nil
}

// ExecutePlaybook runs an Ansible playbook and returns the result
func (a *Agent) ExecutePlaybook(jobID, content string) (status string, output []byte) {
	tmpFile := fmt.Sprintf("/tmp/helvilette_job_%s.yml", jobID)
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		return "Failed", []byte(fmt.Sprintf(`{"error": "Failed to write playbook: %v"}`, err))
	}

	cmd := exec.Command("ansible-playbook", "-i", "localhost,", "-c", "local", tmpFile)
	cmd.Env = append(os.Environ(), "ANSIBLE_STDOUT_CALLBACK=json")

	output, err := cmd.CombinedOutput()

	status = "Success"
	if err != nil {
		status = "Failed"
	}

	// Verify if output is valid JSON
	if !json.Valid(output) {
		safeOutput, _ := json.Marshal(map[string]string{
			"raw_output": string(output),
			"error":      "Output was not valid JSON",
		})
		return status, safeOutput
	}

	return status, output
}

// ProcessJob handles a job from Othela
func (a *Agent) ProcessJob(job *Job) error {
	// Check if this is a new job
	if job.JobID == a.lastJobID {
		return nil // No new job
	}

	a.lastJobID = job.JobID

	// Execute the playbook
	status, output := a.ExecutePlaybook(job.JobID, job.PlaybookContent)

	// Send report
	report := Report{
		NodeID:   a.config.NodeID,
		JobID:    job.JobID,
		Status:   status,
		TaskLogs: json.RawMessage(output),
	}

	return a.SendReport(report)
}

// Run starts the agent main loop
func (a *Agent) Run(ctx context.Context) error {
	logger := log.WithComponent("agent").With().Str("node_id", a.config.NodeID).Logger()

	logger.Info().
		Str("othela_url", a.config.OthelaURL).
		Dur("poll_interval", a.config.PollInterval).
		Msg("Helvilette Agent started")

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

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	agent := NewAgent(DefaultConfig())

	go func() {
		sig := <-sigChan
		log.Info().Str("signal", sig.String()).Msg("received shutdown signal")
		cancel()
	}()

	agent.Run(ctx)
}
