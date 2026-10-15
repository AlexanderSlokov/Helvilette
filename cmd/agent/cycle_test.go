// Tests for cycle.go: what the standalone agent runs for a commit, in which
// order, and what it records. ADR-0008 D1 and D5, ADR-0009 D4, D6 and D9.
package main

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"helvilette/pkg/manifest"
	"helvilette/pkg/types"
)

// twoPlaybooks targets role=edge-proxy with baseline (auto) then web (plan).
const twoPlaybooks = `apiVersion: helvilette.naughtian.org/v1alpha2
kind: PlaybookDeployment
metadata:
  name: edge-fleet
spec:
  playbooks:
    - name: baseline
      playbook: baseline.yml
      apply: auto
      nodeGroups:
        - name: all
          nodeSelector: {role: edge-proxy}
    - name: web
      playbook: web.yml
      nodeGroups:
        - name: proxies
          nodeSelector: {role: edge-proxy}
          ansible:
            extra_vars: {nginx_http_port: "80"}
`

// cycleAgent returns an agent labelled role=edge-proxy, wired to fakes, with a
// temporary workspace.
func cycleAgent(t *testing.T, repo *fakeRepo) (*Agent, *recordingRunner) {
	t.Helper()
	config := DefaultConfig()
	config.WorkspaceDir = t.TempDir()
	config.RepoURL = "git://git.example/playbooks"
	config.Labels = map[string]string{"role": "edge-proxy"}
	runner := &recordingRunner{failing: map[string]bool{}}
	agent := NewAgent(config)
	agent.repo, agent.playbooks = repo, runner
	return agent, runner
}

func repoWith(t *testing.T, commit, manifestText string) *fakeRepo {
	return &fakeRepo{t: t, commit: commit, files: map[string]string{manifestFileName: manifestText}}
}

func mustReadStatus(t *testing.T, a *Agent) types.NodeStatus {
	t.Helper()
	status, err := a.readStatus()
	if err != nil || status == nil {
		t.Fatalf("readStatus: %v, %v", status, err)
	}
	return *status
}

func TestRunCycle_RunsSelectedPlaybooksInFileOrder(t *testing.T) {
	agent, runner := cycleAgent(t, repoWith(t, "c1", twoPlaybooks))

	agent.runCycle()

	if got := runner.names(); !reflect.DeepEqual(got, []string{"baseline", "web"}) {
		t.Fatalf("ran %v, want [baseline web]", got)
	}
	status := mustReadStatus(t, agent)
	if status.Status != statusSuccess || status.CommitSHA != "c1" || status.Reason != "" {
		t.Errorf("status = %+v, want Success for c1 with no reason", status)
	}
}

func TestRunCycle_PassesModeFileAndGroup(t *testing.T) {
	agent, runner := cycleAgent(t, repoWith(t, "c1", twoPlaybooks))

	agent.runCycle()

	baseline, web := runner.runs[0], runner.runs[1]
	if baseline.mode != manifest.ApplyAuto || web.mode != manifest.ApplyPlan {
		t.Errorf("modes = %q, %q; want auto, then plan for the playbook without apply", baseline.mode, web.mode)
	}
	if web.file != filepath.Join(agent.checkoutDir(), "web.yml") || web.workDir != agent.checkoutDir() {
		t.Errorf("web run = %+v, want web.yml inside the checkout", web)
	}
	if web.group.Ansible.ExtraVars["nginx_http_port"] != "80" {
		t.Errorf("web group = %+v, want the matched group's extra_vars", web.group)
	}
}

// ADR-0009 D6: a failure stops the playbooks after it, and the record says so.
func TestRunCycle_StopsAtFirstFailureAndNamesSkipped(t *testing.T) {
	agent, runner := cycleAgent(t, repoWith(t, "c1", twoPlaybooks))
	runner.failing["baseline"] = true

	agent.runCycle()

	if got := runner.names(); !reflect.DeepEqual(got, []string{"baseline"}) {
		t.Fatalf("ran %v, want only baseline", got)
	}
	status := mustReadStatus(t, agent)
	if status.Status != statusFailed || !strings.Contains(status.Reason, `not run, because "baseline" failed: "web"`) {
		t.Errorf("status = %+v, want Failed naming web as not run", status)
	}
}

func TestRunCycle_DoesNotRerunTheSameCommit(t *testing.T) {
	repo := repoWith(t, "c1", twoPlaybooks)
	agent, runner := cycleAgent(t, repo)

	agent.runCycle()
	agent.runCycle()
	if len(runner.runs) != 2 {
		t.Fatalf("ran %d playbooks over two cycles of one commit, want 2", len(runner.runs))
	}

	repo.commit = "c2"
	agent.runCycle()
	if len(runner.runs) != 4 {
		t.Errorf("ran %d playbooks after a new commit, want 4", len(runner.runs))
	}
}

// ADR-0009 D9: no match is a declared state, recorded, with nothing run.
func TestRunCycle_RecordsNothingSelected(t *testing.T) {
	agent, runner := cycleAgent(t, repoWith(t, "c1", twoPlaybooks))
	agent.config.Labels = map[string]string{"role": "db"}

	agent.runCycle()

	status := mustReadStatus(t, agent)
	if len(runner.runs) != 0 || status.Status != statusNothingSelected || !strings.Contains(status.Reason, "role:db") {
		t.Errorf("runs = %v, status = %+v; want nothing run and NothingSelected naming the labels", runner.runs, status)
	}
}

func TestRunCycle_RecordsMissingManifest(t *testing.T) {
	agent, runner := cycleAgent(t, &fakeRepo{t: t, commit: "c1", files: map[string]string{"local.yml": "- hosts: all\n"}})

	agent.runCycle()

	status := mustReadStatus(t, agent)
	if len(runner.runs) != 0 || status.Status != statusFailed || !strings.Contains(status.Reason, "no helvilette.yml") {
		t.Errorf("runs = %v, status = %+v; want nothing run and a reason naming helvilette.yml", runner.runs, status)
	}
}

func TestRunCycle_RecordsInvalidManifest(t *testing.T) {
	agent, runner := cycleAgent(t, repoWith(t, "c1", "apiVersion: helvilette.naughtian.org/v1alpha1\n"))

	agent.runCycle()

	status := mustReadStatus(t, agent)
	if len(runner.runs) != 0 || status.Status != statusFailed || !strings.Contains(status.Reason, "nothing was run") {
		t.Errorf("runs = %v, status = %+v; want nothing run and the parse error recorded", runner.runs, status)
	}
}

// A pull failure leaves the machine and the last record as they were.
func TestRunCycle_SyncFailureRunsNothingAndKeepsRecord(t *testing.T) {
	agent, runner := cycleAgent(t, &fakeRepo{t: t, syncErr: errors.New("connection refused")})

	agent.runCycle()

	if status, _ := agent.readStatus(); len(runner.runs) != 0 || status != nil {
		t.Errorf("runs = %v, status = %+v; want nothing run and no record written", runner.runs, status)
	}
}

func TestRunCycle_UnwritableWorkspaceSkipsTheCycle(t *testing.T) {
	repo := repoWith(t, "c1", twoPlaybooks)
	agent, runner := cycleAgent(t, repo)
	agent.config.WorkspaceDir = filepath.Join(writeBlockerFile(t), "workspace")

	agent.runCycle()

	if repo.syncs != 0 || len(runner.runs) != 0 {
		t.Errorf("syncs = %d, runs = %v; want nothing attempted", repo.syncs, runner.runs)
	}
}

func TestEnsureWorkspace_ExplainsHowToFixIt(t *testing.T) {
	agent, _ := cycleAgent(t, repoWith(t, "c1", twoPlaybooks))
	agent.config.WorkspaceDir = filepath.Join(writeBlockerFile(t), "workspace")

	err := agent.ensureWorkspace()

	for _, want := range []string{agent.config.WorkspaceDir, "--workspace-dir", "nothing was run"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("error %v does not contain %q", err, want)
		}
	}
}

func TestNewRunID_CarriesTheShortCommit(t *testing.T) {
	id := newRunID("0123456789abcdef0123")
	if !strings.HasPrefix(id, "0123456789ab-") {
		t.Errorf("run ID %q does not start with the 12-character commit", id)
	}
}
