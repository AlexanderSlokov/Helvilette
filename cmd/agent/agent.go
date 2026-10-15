// The agent's life from start to stop:
//
//  1. If the last run was cut off (crash, kill, power loss), mark it and run
//     that commit again.
//  2. Run one cycle now, then one every pollInterval. A cycle is in cycle.go.
//  3. Log state changes of systemd units, when D-Bus is available.
//
// The agent talks to nothing but its Git server. What it did is written to
// last_run_summary.json in its workspace.
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"

	"helvilette/pkg/log"
	"helvilette/pkg/systemd"
	"helvilette/pkg/types"
)

// Values of types.NodeStatus.Status that the agent writes.
const (
	statusInProgress      = "InProgress"
	statusSuccess         = "Success"
	statusFailed          = "Failed"
	statusInterrupted     = "Failed (Interrupted)"
	statusNothingSelected = "NothingSelected"
)

// Agent represents the Helvilette agent
type Agent struct {
	config    AgentConfiguration
	repo      repoSyncer
	playbooks playbookRunner
	// lastCommit is the commit the last cycle ran. A cycle that pulls the same
	// commit again does nothing, like ansible-pull --only-if-changed. It is
	// empty after an interrupted run, so that commit runs again.
	lastCommit string
}

// NewAgent creates an agent that pulls with Git and runs playbooks with
// ansible-playbook.
//
//	err := NewAgent(cfg).Run(ctx)
func NewAgent(config AgentConfiguration) *Agent {
	return &Agent{
		config:    config,
		repo:      gitRepoSyncer{},
		playbooks: ansibleRunner{workspaceDir: config.WorkspaceDir},
	}
}

func (a *Agent) logger() zerolog.Logger {
	return log.WithComponent("agent").With().Str("node_id", a.config.NodeID).Logger()
}

// Run recovers from an interrupted run, then runs a cycle at once and on every
// poll interval until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	logger := a.logger()
	logger.Info().
		Str("repo_url", a.config.RepoURL).
		Str("branch", a.config.Branch).
		Dur("poll_interval", a.config.PollInterval).
		Str("workspace_dir", a.config.WorkspaceDir).
		Interface("labels", a.config.Labels).
		Msg("Helvilette Agent started")

	a.recoverInterruptedRun()
	stopWatching := a.watchSystemd(ctx)
	defer stopWatching()

	a.runCycle()
	ticker := time.NewTicker(a.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.runCycle()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// recoverInterruptedRun reads the status the previous process left behind.
//
//   - No file: first start, nothing to do.
//   - Unreadable file: warn; the next commit is treated as new.
//   - A finished run (any status but InProgress): remember its commit, so it
//     is not run again.
//   - InProgress: the process died mid-run. Mark it "Failed (Interrupted)"
//     and leave lastCommit empty, so the next cycle runs the commit again
//     from the start. Playbooks are expected to be idempotent.
func (a *Agent) recoverInterruptedRun() {
	logger := a.logger()
	last, err := a.readStatus()
	if err != nil {
		logger.Warn().Err(err).Msg("failed to read previous status; the next commit is treated as new")
		return
	}
	if last == nil {
		return
	}
	if last.Status != statusInProgress {
		a.lastCommit = last.CommitSHA
		return
	}
	last.Status = statusInterrupted
	last.Reason = "the agent stopped before this run finished (crash, kill or power loss); the commit runs again from the start"
	logger.Warn().Str("run_id", last.JobID).Str("commit", last.CommitSHA).Msg("detected interrupted run, running it again")
	a.writeStatusOrWarn(*last)
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
	// Atomic: this file is how the next start detects an interrupted run, so
	// a crash while writing it must not leave it half-written.
	return writeFileAtomic(a.statusFilePath(), data, 0644)
}

func (a *Agent) writeStatusOrWarn(status types.NodeStatus) {
	if err := a.writeStatus(status); err != nil {
		logger := a.logger()
		logger.Error().Err(err).Str("path", a.statusFilePath()).Msg("failed to write status to disk")
	}
}

// watchSystemd logs state changes of systemd units. It is optional: without
// D-Bus the agent still runs playbooks. Returns the cleanup to defer.
func (a *Agent) watchSystemd(ctx context.Context) func() {
	logger := a.logger()
	sdClient, err := systemd.NewClient()
	if err != nil {
		logger.Warn().Err(err).Msg("failed to connect to systemd D-Bus, systemd watching disabled")
		return func() { /* no client was opened, so there is nothing to close */ }
	}
	logger.Info().Msg("connected to systemd D-Bus")

	eventChan, err := systemd.NewWatcher(sdClient, []string{}).Watch(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("failed to start systemd watcher")
	} else {
		go a.handleSystemdEvents(ctx, eventChan)
	}
	return func() { sdClient.Close() }
}

func (a *Agent) handleSystemdEvents(ctx context.Context, eventChan <-chan systemd.UnitEvent) {
	logger := a.logger()
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
