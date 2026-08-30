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
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"helvilette/pkg/git"
	"helvilette/pkg/log"
	"helvilette/pkg/systemd"
	"helvilette/pkg/types"
)

// AgentConfiguration holds the full configuration for the agent,
// modeled after Kubernetes KubeletConfiguration
type AgentConfiguration struct {
	OthelaURL    string            `yaml:"othelaURL"`
	NodeID       string            `yaml:"nodeID"`
	PollInterval time.Duration     `yaml:"pollInterval"`
	WorkspaceDir string            `yaml:"workspaceDir"`
	Labels       map[string]string `yaml:"labels"`
}

// Names of the sources a configuration value can come from, as reported in
// ConfigProvenance. Env and CLI sources name the specific variable or flag.
const (
	SourceDefault         = "default"
	SourceDefaultHostname = "default(hostname)"
	SourceConfigFile      = "config-file"
)

const labelsPrefix = "labels."

func sourceEnv(name string) string { return "env(" + name + ")" }

func sourceCLI(flag string) string { return "cli(--" + flag + ")" }

// ConfigProvenance records which source supplied each configuration value, keyed by the
// field's YAML name; individual labels are keyed as "labels.<key>". It lets an operator
// see why a node is configured the way it is without re-deriving the precedence rules
// against the machine's state. See docs/informations/ADRs/ADR-0001.md.
type ConfigProvenance map[string]string

// fallbackNodeID is used only when the hostname cannot be determined. It is deliberately
// not a plausible-looking node name, because a static default means every node that
// reaches it registers under the same identity.
const fallbackNodeID = "agent-unknown"

// defaultNodeID prefers the machine hostname so that agents stay distinguishable even
// when nodeID is never configured.
func defaultNodeID() (id string, fromHostname bool) {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h, true
	}
	return fallbackNodeID, false
}

// DefaultConfig returns the default agent configuration
func DefaultConfig() AgentConfiguration {
	nodeID, _ := defaultNodeID()
	return AgentConfiguration{
		OthelaURL:    "http://localhost:8080/api/v1",
		NodeID:       nodeID,
		PollInterval: 5 * time.Second,
		WorkspaceDir: "/tmp/helvilette",
		Labels:       make(map[string]string),
	}
}

func parseLabels(s string) map[string]string {
	labels := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			labels[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return labels
}

func initDefaultConfig() (AgentConfiguration, ConfigProvenance) {
	config := DefaultConfig()

	_, nodeIDFromHostname := defaultNodeID()
	nodeIDDefaultSource := SourceDefault
	if nodeIDFromHostname {
		nodeIDDefaultSource = SourceDefaultHostname
	}

	provenance := ConfigProvenance{
		"othelaURL":    SourceDefault,
		"nodeID":       nodeIDDefaultSource,
		"pollInterval": SourceDefault,
		"workspaceDir": SourceDefault,
	}
	return config, provenance
}

func overrideFromEnv(config *AgentConfiguration, provenance ConfigProvenance) {
	if url := os.Getenv("OTHELA_URL"); url != "" {
		config.OthelaURL = url
		provenance["othelaURL"] = sourceEnv("OTHELA_URL")
	}
	if nodeID := os.Getenv("NODE_ID"); nodeID != "" {
		config.NodeID = nodeID
		provenance["nodeID"] = sourceEnv("NODE_ID")
	}
	if interval := os.Getenv("POLL_INTERVAL"); interval != "" {
		if parsed, err := time.ParseDuration(interval); err == nil {
			config.PollInterval = parsed
			provenance["pollInterval"] = sourceEnv("POLL_INTERVAL")
		}
	}
	if dir := os.Getenv("WORKSPACE_DIR"); dir != "" {
		config.WorkspaceDir = dir
		provenance["workspaceDir"] = sourceEnv("WORKSPACE_DIR")
	}
	if labelsStr := os.Getenv("AGENT_LABELS"); labelsStr != "" {
		envLabels := parseLabels(labelsStr)
		for k, v := range envLabels {
			config.Labels[k] = v
			provenance[labelsPrefix+k] = sourceEnv("AGENT_LABELS")
		}
	}
}

func overrideFromFile(configPath string, config *AgentConfiguration, provenance ConfigProvenance) error {
	if configPath == "" {
		return nil
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var raw struct {
		OthelaURL    string            `yaml:"othelaURL"`
		NodeID       string            `yaml:"nodeID"`
		PollInterval string            `yaml:"pollInterval"`
		WorkspaceDir string            `yaml:"workspaceDir"`
		Labels       map[string]string `yaml:"labels"`
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil && err != io.EOF {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	if raw.OthelaURL != "" {
		config.OthelaURL = raw.OthelaURL
		provenance["othelaURL"] = SourceConfigFile
	}
	if raw.NodeID != "" {
		config.NodeID = raw.NodeID
		provenance["nodeID"] = SourceConfigFile
	}
	if raw.WorkspaceDir != "" {
		config.WorkspaceDir = raw.WorkspaceDir
		provenance["workspaceDir"] = SourceConfigFile
	}
	if raw.PollInterval != "" {
		d, err := time.ParseDuration(raw.PollInterval)
		if err != nil {
			return fmt.Errorf("invalid pollInterval in config file: %w", err)
		}
		config.PollInterval = d
		provenance["pollInterval"] = SourceConfigFile
	}
	for k, v := range raw.Labels {
		config.Labels[k] = v
		provenance[labelsPrefix+k] = SourceConfigFile
	}
	return nil
}

func overrideFromCLI(cliOthelaURL, cliNodeID, cliPollInterval, cliWorkspaceDir, cliLabels string, config *AgentConfiguration, provenance ConfigProvenance) {
	if cliOthelaURL != "" {
		config.OthelaURL = cliOthelaURL
		provenance["othelaURL"] = sourceCLI("othela-url")
	}
	if cliNodeID != "" {
		config.NodeID = cliNodeID
		provenance["nodeID"] = sourceCLI("node-id")
	}
	if cliPollInterval != "" {
		if parsed, err := time.ParseDuration(cliPollInterval); err == nil {
			config.PollInterval = parsed
			provenance["pollInterval"] = sourceCLI("poll-interval")
		}
	}
	if cliWorkspaceDir != "" {
		config.WorkspaceDir = cliWorkspaceDir
		provenance["workspaceDir"] = sourceCLI("workspace-dir")
	}
	if cliLabels != "" {
		cliParsedLabels := parseLabels(cliLabels)
		for k, v := range cliParsedLabels {
			config.Labels[k] = v
			provenance[labelsPrefix+k] = sourceCLI("labels")
		}
	}
}

func formatOthelaURL(url string) string {
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}
	if len(url) > 0 && url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}
	if len(url) < 7 || url[len(url)-7:] != "/api/v1" {
		url = url + "/api/v1"
	}
	return url
}

// LoadConfig merges default, yaml file, environment, and CLI configurations. The returned
// ConfigProvenance records which source won each value, so the agent can report it at startup.
func LoadConfig(configPath, cliOthelaURL, cliNodeID, cliPollInterval, cliWorkspaceDir, cliLabels string) (AgentConfiguration, ConfigProvenance, error) {
	config, provenance := initDefaultConfig()

	overrideFromEnv(&config, provenance)

	if err := overrideFromFile(configPath, &config, provenance); err != nil {
		return config, provenance, err
	}

	overrideFromCLI(cliOthelaURL, cliNodeID, cliPollInterval, cliWorkspaceDir, cliLabels, &config, provenance)

	config.OthelaURL = formatOthelaURL(config.OthelaURL)

	return config, provenance, nil
}

// configFields returns the resolved configuration as ordered field/value pairs, including
// one entry per label. Ordering is stable so that operators comparing two nodes, or two
// runs on the same node, can diff the output directly.
func configFields(config AgentConfiguration) [][2]string {
	fields := [][2]string{
		{"othelaURL", config.OthelaURL},
		{"nodeID", config.NodeID},
		{"pollInterval", config.PollInterval.String()},
		{"workspaceDir", config.WorkspaceDir},
	}

	labelKeys := make([]string, 0, len(config.Labels))
	for k := range config.Labels {
		labelKeys = append(labelKeys, k)
	}
	sort.Strings(labelKeys)
	for _, k := range labelKeys {
		fields = append(fields, [2]string{labelsPrefix + k, config.Labels[k]})
	}

	return fields
}

// FormatConfig renders the resolved configuration and the source of each value as an
// aligned block, for humans reading `--print-config`.
func FormatConfig(config AgentConfiguration, provenance ConfigProvenance) string {
	fields := configFields(config)

	nameWidth, valueWidth := 0, 0
	for _, f := range fields {
		if len(f[0]) > nameWidth {
			nameWidth = len(f[0])
		}
		if len(f[1]) > valueWidth {
			valueWidth = len(f[1])
		}
	}

	var b strings.Builder
	for _, f := range fields {
		source := provenance[f[0]]
		if source == "" {
			source = SourceDefault
		}
		fmt.Fprintf(&b, "%-*s = %-*s  source=%s\n", nameWidth, f[0], valueWidth, f[1], source)
	}
	return b.String()
}

// logEffectiveConfig reports the resolved configuration and where each value came from,
// so that a node's behaviour can be explained from its logs alone rather than by
// re-deriving the precedence rules against systemd units and container environments.
func logEffectiveConfig(config AgentConfiguration, provenance ConfigProvenance) {
	values := zerolog.Dict()
	sources := zerolog.Dict()
	for _, f := range configFields(config) {
		source := provenance[f[0]]
		if source == "" {
			source = SourceDefault
		}
		values = values.Str(f[0], f[1])
		sources = sources.Str(f[0], source)
	}

	log.Info().
		Dict("config", values).
		Dict("configSources", sources).
		Msg("effective configuration")

	if config.NodeID == fallbackNodeID {
		log.Warn().
			Str("nodeID", config.NodeID).
			Msg("nodeID is unset and the hostname could not be determined; set nodeID explicitly to keep this node distinguishable from others in the fleet")
	}
}

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
		// Neither RepoURL nor PlaybookPath — the job is undeliverable.
		// Before issue #25 this branch wrote PlaybookContent to a temp file,
		// but inline content delivery has been removed from the wire format.
		logger.Error().Str("job_id", job.JobID).Msg("job has neither repo_url nor playbook_path")
		b, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("job %q has neither repo_url nor playbook_path; inline content delivery is no longer supported", job.JobID),
		})
		return "Failed", b
	}

	ansiblePath, err := exec.LookPath("ansible-playbook")
	if err != nil {
		logger.Error().Err(err).Str("job_id", job.JobID).Msg("ansible-playbook executable not found in PATH")
		b, _ := json.Marshal(map[string]string{
			"error": "ansible-playbook executable not found in PATH",
		})
		return "Failed", b
	}

	cmd := exec.Command(ansiblePath, "-i", "localhost,", "-c", "local", playbookFile)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "ANSIBLE_STDOUT_CALLBACK=json")

	if len(job.ExtraVars) > 0 {
		extraVarsFile := filepath.Join(workDir, ".helvilette_extra_vars.json")
		data, _ := json.Marshal(job.ExtraVars)
		os.WriteFile(extraVarsFile, data, 0644)
		cmd.Args = append(cmd.Args, "-e", "@"+extraVarsFile)
	}

	logger.Debug().
		Str("command", cmd.String()).
		Str("work_dir", workDir).
		Msg("running ansible-playbook")

	output, err = cmd.CombinedOutput()

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

func main() {
	var (
		configFile   string
		othelaURL    string
		nodeID       string
		pollInterval string
		workspaceDir string
		labels       string
		printConfig  bool
	)

	var rootCmd = &cobra.Command{
		Use:   "agent",
		Short: "Helvilette Node Agent",
		Long:  `The Node Agent runs on client machines, polls Othela for jobs, and executes Ansible playbooks.`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, provenance, err := LoadConfig(configFile, othelaURL, nodeID, pollInterval, workspaceDir, labels)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to load configuration")
			}

			// Resolve and print without starting, so a config can be verified during
			// day-0 bring-up or in CI rather than by observing a running agent.
			if printConfig {
				fmt.Print(FormatConfig(cfg, provenance))
				return
			}

			logEffectiveConfig(cfg, provenance)

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
	rootCmd.Flags().StringVar(&workspaceDir, "workspace-dir", "", "Directory for storing agent workspace files")
	rootCmd.Flags().StringVar(&labels, "labels", "", "Comma-separated key=value labels (e.g. role=web,env=prod)")
	rootCmd.Flags().BoolVar(&printConfig, "print-config", false, "Print the resolved configuration and the source of each value, then exit")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
