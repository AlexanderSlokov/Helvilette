package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"helvilette/pkg/log"
	"helvilette/pkg/systemd"
	"helvilette/pkg/types"
	"helvilette/pkg/git"
)

// AgentConfiguration holds the full configuration for the agent,
// modeled after Kubernetes KubeletConfiguration
type AgentConfiguration struct {
	OthelaURL    string        `yaml:"othelaURL"`
	NodeID       string        `yaml:"nodeID"`
	PollInterval time.Duration `yaml:"pollInterval"`
	WorkspaceDir string        `yaml:"workspaceDir"`
}

// DefaultConfig returns the default agent configuration
func DefaultConfig() AgentConfiguration {
	return AgentConfiguration{
		OthelaURL:    "http://localhost:8080/api/v1",
		NodeID:       "agent-01",
		PollInterval: 5 * time.Second,
		WorkspaceDir: "/tmp/helvilette",
	}
}

// LoadConfig merges default, yaml file, environment, and CLI configurations
func LoadConfig(configPath, cliOthelaURL, cliNodeID, cliPollInterval string) (AgentConfiguration, error) {
	config := DefaultConfig()

	// 1. Load from YAML file if provided
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return config, fmt.Errorf("failed to read config file: %w", err)
		}

		var raw struct {
			OthelaURL    string `yaml:"othelaURL"`
			NodeID       string `yaml:"nodeID"`
			PollInterval string `yaml:"pollInterval"`
			WorkspaceDir string `yaml:"workspaceDir"`
		}

		if err := yaml.Unmarshal(data, &raw); err != nil {
			return config, fmt.Errorf("failed to parse config file: %w", err)
		}

		if raw.OthelaURL != "" {
			config.OthelaURL = raw.OthelaURL
		}
		if raw.NodeID != "" {
			config.NodeID = raw.NodeID
		}
		if raw.WorkspaceDir != "" {
			config.WorkspaceDir = raw.WorkspaceDir
		}
		if raw.PollInterval != "" {
			d, err := time.ParseDuration(raw.PollInterval)
			if err != nil {
				return config, fmt.Errorf("invalid pollInterval in config file: %w", err)
			}
			config.PollInterval = d
		}
	}

	// 2. Override with Environment Variables
	if url := os.Getenv("OTHELA_URL"); url != "" {
		config.OthelaURL = url
	}
	if nodeID := os.Getenv("NODE_ID"); nodeID != "" {
		config.NodeID = nodeID
	}
	if interval := os.Getenv("POLL_INTERVAL"); interval != "" {
		if parsed, err := time.ParseDuration(interval); err == nil {
			config.PollInterval = parsed
		}
	}
	if dir := os.Getenv("WORKSPACE_DIR"); dir != "" {
		config.WorkspaceDir = dir
	}

	// 3. Override with CLI Flags (highest priority)
	if cliOthelaURL != "" {
		config.OthelaURL = cliOthelaURL
	}
	if cliNodeID != "" {
		config.NodeID = cliNodeID
	}
	if cliPollInterval != "" {
		if parsed, err := time.ParseDuration(cliPollInterval); err == nil {
			config.PollInterval = parsed
		}
	}

	// Format URL
	if !filepath.HasPrefix(config.OthelaURL, "http") {
		config.OthelaURL = "http://" + config.OthelaURL
	}
	if len(config.OthelaURL) > 0 && config.OthelaURL[len(config.OthelaURL)-1] == '/' {
		config.OthelaURL = config.OthelaURL[:len(config.OthelaURL)-1]
	}
	if len(config.OthelaURL) < 7 || config.OthelaURL[len(config.OthelaURL)-7:] != "/api/v1" {
		config.OthelaURL = config.OthelaURL + "/api/v1"
	}

	return config, nil
}

// Agent represents the Helvilette agent
type Agent struct {
	config     AgentConfiguration
	lastJobID  string
	httpClient *http.Client
}

// Job Add Type aliases for backward compatibility within this package
type Job = types.Job
type Report = types.Report

// NewAgent creates a new agent with the given configuration
func NewAgent(config AgentConfiguration) *Agent {
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
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

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
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("othela returned with status: %s", resp.Status)
	}

	return nil
}

// ExecutePlaybook runs an Ansible playbook and returns the result.
func (a *Agent) ExecutePlaybook(job *Job) (status string, output []byte) {
	logger := log.WithComponent("executor")

	var playbookFile string
	var workDir string

	// Ensure workspace exists
	if err := os.MkdirAll(a.config.WorkspaceDir, 0755); err != nil {
		logger.Error().Err(err).Str("dir", a.config.WorkspaceDir).Msg("failed to create workspace dir")
		return "Failed", []byte(fmt.Sprintf(`{"error": "Failed to create workspace: %v"}`, err))
	}

	if job.RepoURL != "" {
		reposDir := filepath.Join(a.config.WorkspaceDir, "repos")
		repoName := filepath.Base(job.RepoURL)
		repoDir := filepath.Join(reposDir, repoName)

		logger.Info().Str("job_id", job.JobID).Str("repo_url", job.RepoURL).Msg("ensuring git repo")
		if err := git.EnsureRepo(job.RepoURL, repoDir, job.Version); err != nil {
			logger.Error().Err(err).Str("repo", job.RepoURL).Msg("failed to ensure git repo")
			b, _ := json.Marshal(map[string]string{"error": fmt.Sprintf("Failed to pull git repo: %v", err)})
			return "Failed", b
		}

		if job.PlaybookPath != "" && !filepath.IsAbs(job.PlaybookPath) {
			playbookFile = filepath.Join(repoDir, job.PlaybookPath)
		} else {
			playbookFile = filepath.Join(repoDir, "playbook.yml")
		}
		// Run from the root of the repository so roles folders resolve
		workDir = repoDir

		logger.Info().
			Str("job_id", job.JobID).
			Str("repo_url", job.RepoURL).
			Str("version", job.Version).
			Str("work_dir", workDir).
			Msg("executing playbook from git repo")
	} else if job.PlaybookPath != "" {
		// Use provided path - enables role resolution
		// Ensure that the agent can read the file correctly by looking into the workspace dir.
		playbookFile = job.PlaybookPath
		workDir = filepath.Dir(playbookFile)
		logger.Info().
			Str("job_id", job.JobID).
			Str("playbook_path", playbookFile).
			Str("work_dir", workDir).
			Msg("executing playbook from source path")
	} else {
		// Fallback: write content to workspace file
		playbookFile = filepath.Join(a.config.WorkspaceDir, fmt.Sprintf("helvilette_job_%s.yml", job.JobID))
		workDir = a.config.WorkspaceDir
		if err := os.WriteFile(playbookFile, []byte(job.PlaybookContent), 0644); err != nil {
			logger.Error().Err(err).Str("job_id", job.JobID).Msg("failed to write playbook to workspace")
			return "Failed", []byte(fmt.Sprintf(`{"error": "Failed to write playbook: %v"}`, err))
		}
		logger.Info().
			Str("job_id", job.JobID).
			Str("temp_file", playbookFile).
			Msg("executing playbook from workspace file")
	}

	cmd := exec.Command("ansible-playbook", "-i", "localhost,", "-c", "local", playbookFile)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "ANSIBLE_STDOUT_CALLBACK=json")

	logger.Debug().
		Str("command", cmd.String()).
		Str("work_dir", workDir).
		Msg("running ansible-playbook")

	output, err := cmd.CombinedOutput()

	status = "Success"
	if err != nil {
		status = "Failed"
		logger.Warn().
			Err(err).
			Str("job_id", job.JobID).
			Msg("playbook execution failed")
	} else {
		logger.Info().
			Str("job_id", job.JobID).
			Msg("playbook execution succeeded")
	}

	// Verify if output is valid JSON
	if !json.Valid(output) {
		logger.Warn().Str("job_id", job.JobID).Msg("ansible output was not valid JSON")
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
	logger := log.WithComponent("agent")

	// Check if this is a new job
	if job.JobID == a.lastJobID {
		return nil // No new job
	}

	logger.Info().
		Str("job_id", job.JobID).
		Bool("has_path", job.PlaybookPath != "").
		Msg("processing new job")

	// Execute the playbook
	status, output := a.ExecutePlaybook(job)
	
	// Only consider the job processed if it didn't fail due to initial fetching
	if status == "Success" {
		a.lastJobID = job.JobID
	}

	// Send report
	report := Report{
		NodeID:   a.config.NodeID,
		JobID:    job.JobID,
		Status:   status,
		TaskLogs: json.RawMessage(output),
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
	var (
		configFile   string
		othelaURL    string
		nodeID       string
		pollInterval string
	)

	var rootCmd = &cobra.Command{
		Use:   "agent",
		Short: "Helvilette Node Agent",
		Long:  `The Node Agent runs on client machines, polls Othela for jobs, and executes Ansible playbooks.`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := LoadConfig(configFile, othelaURL, nodeID, pollInterval)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to load configuration")
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			agent := NewAgent(cfg)

			go func() {
				sig := <-sigChan
				log.Info().Str("signal", sig.String()).Msg("received shutdown signal")
				cancel()
			}()

			err = agent.Run(ctx)
			if err != nil {
				log.Fatal().Err(err).Msg("agent stopped with error")
			}
		},
	}

	rootCmd.Flags().StringVar(&configFile, "config", "", "Path to the YAML configuration file (e.g. /var/lib/helvilette/agent.yaml)")
	rootCmd.Flags().StringVar(&othelaURL, "othela-url", "", "URL of the Othela control plane")
	rootCmd.Flags().StringVar(&nodeID, "node-id", "", "Unique identifier for this node")
	rootCmd.Flags().StringVar(&pollInterval, "poll-interval", "", "Interval between polls to Othela (e.g. 5s)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
