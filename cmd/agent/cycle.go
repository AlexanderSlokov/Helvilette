// One cycle of the standalone agent: pull the repository, read the
// helvilette.yml at its root, select the playbooks for this node's labels, and
// run them in file order, stopping at the first failure. This replaces polling
// Othela for a job (ADR-0008 D1, ADR-0009 D6, D9; issue #43).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"helvilette/pkg/git"
	"helvilette/pkg/manifest"
	"helvilette/pkg/types"
)

// manifestFileName is read only at the repository root (ADR-0009 D1).
const manifestFileName = "helvilette.yml"

// repoSyncer is the one Git operation the agent needs: make dir a checkout of
// url at ref and say which commit that is. Tests replace it.
type repoSyncer interface {
	Sync(url, dir, ref string) (commit string, err error)
}

// gitRepoSyncer syncs with pkg/git, which hard-resets local changes (ADR-0005).
type gitRepoSyncer struct{}

func (gitRepoSyncer) Sync(url, dir, ref string) (string, error) {
	return git.EnsureRepo(url, dir, ref)
}

func (a *Agent) checkoutDir() string {
	return filepath.Join(a.config.WorkspaceDir, "checkout")
}

// runCycle pulls and, when the commit is new, runs it. A commit that already
// ran, whatever its result, waits for the next commit.
func (a *Agent) runCycle() {
	logger := a.logger()
	if err := a.ensureWorkspace(); err != nil {
		logger.Error().Err(err).Msg("cycle skipped")
		return
	}
	commit, err := a.repo.Sync(a.config.RepoURL, a.checkoutDir(), a.config.Branch)
	if err != nil {
		logger.Error().Err(err).Str("repo_url", a.config.RepoURL).Str("branch", a.config.Branch).
			Msg("cannot pull the repository; nothing was run and this machine keeps its current state. Check repoURL, branch and the network")
		return
	}
	if commit == a.lastCommit {
		logger.Debug().Str("commit", commit).Msg("no new commit")
		return
	}
	a.runCommit(commit)
	a.lastCommit = commit
}

// ensureWorkspace creates the workspace, and says how to point it elsewhere
// when it cannot: since issue #43 the default is under /var/lib, which a
// non-root developer cannot write.
func (a *Agent) ensureWorkspace() error {
	if err := os.MkdirAll(a.config.WorkspaceDir, 0755); err != nil {
		return fmt.Errorf("cannot create workspace %q: %w; nothing was run. "+
			"Set --workspace-dir or WORKSPACE_DIR to a writable directory", a.config.WorkspaceDir, err)
	}
	return nil
}

// runCommit runs every playbook selected for this node at commit and records
// the outcome. The in-progress status is written before anything runs: it is
// what lets the next start detect an interrupted run.
func (a *Agent) runCommit(commit string) {
	status := types.NodeStatus{JobID: newRunID(commit), CommitSHA: commit, AppliedAt: time.Now()}
	selections, err := a.selectPlaybooks()
	if err != nil {
		a.finishRun(status, statusFailed, err.Error())
		return
	}
	if len(selections) == 0 {
		a.finishRun(status, statusNothingSelected, fmt.Sprintf(
			"no nodeGroup in %s matches this node's labels %v; nothing was run (ADR-0009 D9)", manifestFileName, a.config.Labels))
		return
	}
	status.Status = statusInProgress
	a.writeStatusOrWarn(status)
	result, reason := a.runSelections(status.JobID, selections)
	a.finishRun(status, result, reason)
}

// selectPlaybooks reads the root helvilette.yml and returns the playbooks that
// select this node, in file order.
func (a *Agent) selectPlaybooks() ([]manifest.PlaybookSelection, error) {
	m, err := manifest.ParseRepoManifest(filepath.Join(a.checkoutDir(), manifestFileName))
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("the repository has no %s at its root, so nothing was run. "+
			"Add one (ADR-0009); the ansible-pull fallback to <fqdn>.yml, <hostname>.yml, local.yml is not implemented yet",
			manifestFileName)
	}
	if err != nil {
		return nil, fmt.Errorf("%w; nothing was run", err)
	}
	return manifest.SelectPlaybooks(m, a.config.Labels), nil
}

// runSelections runs each selection in order and stops at the first failure,
// because a later playbook may depend on an earlier one (ADR-0009 D6).
func (a *Agent) runSelections(runID string, selections []manifest.PlaybookSelection) (status, reason string) {
	for i, selection := range selections {
		if !a.playbooks.Run(a.playbookRunFor(runID, selection)) {
			return statusFailed, failureReason(selections, i)
		}
	}
	return statusSuccess, ""
}

func (a *Agent) playbookRunFor(runID string, selection manifest.PlaybookSelection) playbookRun {
	return playbookRun{
		runID:   runID,
		name:    selection.Entry.Name,
		file:    filepath.Join(a.checkoutDir(), selection.Entry.Playbook),
		workDir: a.checkoutDir(),
		mode:    selection.Entry.EffectiveApply(),
		group:   selection.Group,
	}
}

// failureReason names the playbook that failed and every one skipped after it,
// so the record says what did not run and why (ADR-0009 D6).
func failureReason(selections []manifest.PlaybookSelection, failed int) string {
	reason := fmt.Sprintf("playbook %q failed", selections[failed].Entry.Name)
	var skipped []string
	for _, s := range selections[failed+1:] {
		skipped = append(skipped, fmt.Sprintf("%q", s.Entry.Name))
	}
	if len(skipped) > 0 {
		reason += fmt.Sprintf("; not run, because %q failed: %s", selections[failed].Entry.Name, strings.Join(skipped, ", "))
	}
	return reason
}

// finishRun writes the final status and logs it in one line.
func (a *Agent) finishRun(status types.NodeStatus, result, reason string) {
	status.Status, status.Reason = result, reason
	a.writeStatusOrWarn(status)

	logger := a.logger()
	event := logger.Info()
	if result == statusFailed {
		event = logger.Warn()
	}
	event.Str("run_id", status.JobID).Str("commit", status.CommitSHA).
		Str("status", result).Str("reason", reason).Msg("run finished")
}

// newRunID names a run after its commit and start time, so two runs of the
// same commit (an interrupted one and its re-run) stay distinguishable.
func newRunID(commit string) string {
	short := commit
	if len(short) > 12 {
		short = short[:12]
	}
	return short + "-" + time.Now().UTC().Format("20060102T150405Z")
}
