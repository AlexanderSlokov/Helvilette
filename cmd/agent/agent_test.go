// Tests for agent.go: the agent's wiring, and recovering from a run that a
// crash, a kill or a power loss cut off (ADR-0008 D5).
package main

import (
	"os"
	"path/filepath"
	"testing"

	"helvilette/pkg/types"
)

// writeBlockerFile returns a regular file, so a directory under it can never
// be created, whoever runs the test.
func writeBlockerFile(t *testing.T) string {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(blocker, nil, 0644); err != nil {
		t.Fatal(err)
	}
	return blocker
}

func TestNewAgent_UsesGitAndAnsible(t *testing.T) {
	config := DefaultConfig()
	config.WorkspaceDir = "/srv/ws"

	agent := NewAgent(config)

	if _, ok := agent.repo.(gitRepoSyncer); !ok {
		t.Errorf("repo = %T, want gitRepoSyncer", agent.repo)
	}
	if runner, ok := agent.playbooks.(ansibleRunner); !ok || runner.workspaceDir != "/srv/ws" {
		t.Errorf("playbooks = %#v, want ansibleRunner on the workspace", agent.playbooks)
	}
}

// ADR-0008 D5: an in-progress record at startup means the last run was cut
// off. It is marked interrupted and the same commit runs again.
func TestRecoverInterruptedRun_RerunsTheSameCommit(t *testing.T) {
	agent, runner := cycleAgent(t, repoWith(t, "c1", twoPlaybooks))
	agent.writeStatusOrWarn(types.NodeStatus{JobID: "c1-old", CommitSHA: "c1", Status: statusInProgress})

	agent.recoverInterruptedRun()
	if status := mustReadStatus(t, agent); status.Status != statusInterrupted || status.Reason == "" {
		t.Fatalf("status = %+v, want %q with a reason", status, statusInterrupted)
	}

	agent.runCycle()
	if len(runner.runs) != 2 {
		t.Errorf("ran %d playbooks after an interrupted run of the same commit, want 2", len(runner.runs))
	}
}

// A finished run, failed or not, is not repeated after a restart.
func TestRecoverInterruptedRun_KeepsAFinishedCommit(t *testing.T) {
	agent, runner := cycleAgent(t, repoWith(t, "c1", twoPlaybooks))
	agent.writeStatusOrWarn(types.NodeStatus{JobID: "c1-old", CommitSHA: "c1", Status: statusFailed})

	agent.recoverInterruptedRun()
	agent.runCycle()

	if len(runner.runs) != 0 {
		t.Errorf("ran %v after a restart on an already-run commit, want nothing", runner.names())
	}
}

func TestRecoverInterruptedRun_ToleratesNoRecord(t *testing.T) {
	agent, _ := cycleAgent(t, repoWith(t, "c1", twoPlaybooks))

	agent.recoverInterruptedRun()

	if agent.lastCommit != "" {
		t.Errorf("lastCommit = %q, want empty on a first start", agent.lastCommit)
	}
}
