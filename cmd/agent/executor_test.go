// Tests for executor.go: the ansible-playbook arguments for each mode, where
// the agent puts the files it hands to Ansible, and what it keeps afterwards.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helvilette/pkg/manifest"
)

func runFor(mode manifest.ApplyMode, ansible manifest.AnsibleConfig) playbookRun {
	return playbookRun{
		runID: "r1", name: "web", file: "/ws/checkout/web.yml", workDir: "/ws/checkout",
		mode: mode, group: manifest.NodeGroup{Name: "proxies", Ansible: ansible},
	}
}

func TestAnsibleRunnerArgs_AutoAppliesDirectly(t *testing.T) {
	args, err := ansibleRunner{workspaceDir: t.TempDir()}.args(runFor(manifest.ApplyAuto, manifest.AnsibleConfig{}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(args, " ") != "-i localhost, -c local /ws/checkout/web.yml" {
		t.Errorf("args = %v", args)
	}
}

// plan changes nothing, so it runs as a check.
func TestAnsibleRunnerArgs_PlanRunsAsCheck(t *testing.T) {
	args, err := ansibleRunner{workspaceDir: t.TempDir()}.args(runFor(manifest.ApplyPlan, manifest.AnsibleConfig{}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(args, " ") != "-i localhost, -c local /ws/checkout/web.yml --check --diff" {
		t.Errorf("args = %v", args)
	}
}

// vault-password-file is a path on the node, passed through.
func TestAnsibleRunnerArgs_PassesVaultPasswordFile(t *testing.T) {
	run := runFor(manifest.ApplyAuto, manifest.AnsibleConfig{VaultPasswordFile: "/etc/helvilette/vault-pass"})

	args, err := ansibleRunner{workspaceDir: t.TempDir()}.args(run)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, " "), "--vault-password-file /etc/helvilette/vault-pass") {
		t.Errorf("args = %v", args)
	}
}

// extra_vars go to the workspace, not the Git checkout, with mode 0600 because
// they may hold credentials.
func TestAnsibleRunnerArgs_WritesExtraVarsInWorkspace(t *testing.T) {
	runner := ansibleRunner{workspaceDir: t.TempDir()}
	run := runFor(manifest.ApplyAuto, manifest.AnsibleConfig{ExtraVars: map[string]string{"nginx_http_port": "80"}})

	args, err := runner.args(run)
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(runner.workspaceDir, extraVarsFileName)
	if args[len(args)-1] != "@"+want {
		t.Fatalf("last arg = %q, want %q", args[len(args)-1], "@"+want)
	}
	assertFileMode(t, want, 0600)
	assertJSONField(t, want, "nginx_http_port", "80")
}

func TestAnsibleRunnerArgs_ReportsExtraVarsWriteFailure(t *testing.T) {
	runner := ansibleRunner{workspaceDir: filepath.Join(t.TempDir(), "never-created")}
	run := runFor(manifest.ApplyAuto, manifest.AnsibleConfig{ExtraVars: map[string]string{"k": "v"}})

	if _, err := runner.args(run); err == nil {
		t.Error("expected an error when extra_vars cannot be written")
	}
}

// Without Ansible on PATH the run fails, and the reason is still saved where
// the output would have been.
func TestAnsibleRunnerRun_SavesWhyItDidNotRun(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	runner := ansibleRunner{workspaceDir: t.TempDir()}

	if runner.Run(runFor(manifest.ApplyAuto, manifest.AnsibleConfig{})) {
		t.Fatal("Run succeeded without ansible-playbook on PATH")
	}

	path := filepath.Join(runner.workspaceDir, "runs", "web.json")
	assertFileMode(t, path, 0600)
	assertJSONField(t, path, "error", "ansible-playbook executable not found in PATH")
}

func TestAsJSONOutput_WrapsNonJSON(t *testing.T) {
	out := asJSONOutput(runFor(manifest.ApplyAuto, manifest.AnsibleConfig{}), []byte("ERROR! no playbook"))
	if !json.Valid(out) || !strings.Contains(string(out), "ERROR! no playbook") {
		t.Errorf("output %s is not wrapped JSON carrying the raw text", out)
	}
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != want {
		t.Errorf("%s mode = %v, want %v", path, info.Mode().Perm(), want)
	}
}

// assertJSONField checks that the JSON object in path has key, and that its
// value contains want.
func assertJSONField(t *testing.T, path, key, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]string
	if err := json.Unmarshal(body, &doc); err != nil || !strings.Contains(doc[key], want) {
		t.Errorf("%s holds %s, want %q containing %q", path, body, key, want)
	}
}
