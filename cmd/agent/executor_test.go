// Tests for executor.go: what the agent refuses to run, and where it puts the
// files it hands to Ansible.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// agentWithWorkspace returns an agent whose workspace is a fresh temporary
// directory, so no test touches the /var/lib default.
func agentWithWorkspace(t *testing.T) *Agent {
	t.Helper()
	config := DefaultConfig()
	config.WorkspaceDir = t.TempDir()
	return NewAgent(config)
}

func TestAgent_ExecutePlaybook_RejectsEmptyJob(t *testing.T) {
	agent := agentWithWorkspace(t)

	status, output := agent.ExecutePlaybook(&Job{JobID: "test-empty-123"})
	if status != "Failed" {
		t.Errorf("expected status %q, got %q", "Failed", status)
	}
	if !strings.Contains(string(output), "neither repo_url nor playbook_path") {
		t.Errorf("output %s does not say why the job was rejected", output)
	}
}

// Regression for issue #43: the workspace error must name the path and say how
// to choose another, now that the default is not writable by a normal user.
func TestAgent_ExecutePlaybook_ExplainsUnwritableWorkspace(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(blocker, nil, 0644); err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig()
	config.WorkspaceDir = filepath.Join(blocker, "workspace")

	status, output := NewAgent(config).ExecutePlaybook(&Job{JobID: "j1", PlaybookPath: "/srv/site.yml"})
	if status != "Failed" {
		t.Fatalf("expected status Failed, got %q", status)
	}
	for _, want := range []string{config.WorkspaceDir, "--workspace-dir", "nothing was run"} {
		if !strings.Contains(string(output), want) {
			t.Errorf("output %s does not contain %q", output, want)
		}
	}
}

// Regression for issue #43: extra_vars were written into the Git checkout,
// dirtying the tree EnsureRepo resets, and the write error was discarded.
func TestAgent_AnsibleArgs_WritesExtraVarsInWorkspace(t *testing.T) {
	agent := agentWithWorkspace(t)
	checkout := filepath.Join(agent.config.WorkspaceDir, "repos", "site")
	job := &Job{JobID: "j1", ExtraVars: map[string]string{"chrony_pool": "ntp.ubuntu.com"}}

	args, err := agent.ansibleArgs(job, playbookTarget{file: filepath.Join(checkout, "site.yml"), workDir: checkout})
	if err != nil {
		t.Fatalf("ansibleArgs: %v", err)
	}

	want := filepath.Join(agent.config.WorkspaceDir, extraVarsFileName)
	if args[len(args)-1] != "@"+want {
		t.Fatalf("last arg = %q, want %q", args[len(args)-1], "@"+want)
	}
	assertExtraVarsFile(t, want, job.ExtraVars)
}

func assertExtraVarsFile(t *testing.T, path string, want map[string]string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("extra_vars file: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil || got["chrony_pool"] != want["chrony_pool"] {
		t.Errorf("extra_vars file holds %s, want %v", body, want)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0600 {
		t.Errorf("extra_vars mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestAgent_AnsibleArgs_ReportsExtraVarsWriteFailure(t *testing.T) {
	config := DefaultConfig()
	config.WorkspaceDir = filepath.Join(t.TempDir(), "never-created")
	job := &Job{JobID: "j1", ExtraVars: map[string]string{"k": "v"}}

	if _, err := NewAgent(config).ansibleArgs(job, playbookTarget{file: "site.yml"}); err == nil {
		t.Error("expected an error when extra_vars cannot be written")
	}
}

func TestAgent_AnsibleArgs_OmitsExtraVarsWhenNone(t *testing.T) {
	agent := agentWithWorkspace(t)

	args, err := agent.ansibleArgs(&Job{JobID: "j1"}, playbookTarget{file: "site.yml"})
	if err != nil {
		t.Fatalf("ansibleArgs: %v", err)
	}
	if strings.Join(args, " ") != "-i localhost, -c local site.yml" {
		t.Errorf("args = %v", args)
	}
}

func TestAsJSONOutput_WrapsNonJSON(t *testing.T) {
	out := asJSONOutput("j1", []byte("ERROR! no playbook"))
	if !json.Valid(out) || !strings.Contains(string(out), "ERROR! no playbook") {
		t.Errorf("output %s is not wrapped JSON carrying the raw text", out)
	}
}
