// Running one playbook with ansible-playbook and keeping what Ansible said.
//
// Split out of main.go, which exceeded the 500-line ceiling in AGENTS.md.
// See BACKLOG 6.7. ExecutePlaybook was later split into one function per step
// (issue #43), and then lost its job-resolution half when the agent stopped
// receiving jobs from Othela: cycle.go now decides what runs (ADR-0008 D1).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"helvilette/pkg/log"
	"helvilette/pkg/manifest"
)

// playbookRun is one playbook to run for one node, as cycle.go selected it.
type playbookRun struct {
	runID   string
	name    string // spec.playbooks[].name
	file    string // absolute path inside the checkout
	workDir string // checkout root, so roles/ resolves
	mode    manifest.ApplyMode
	group   manifest.NodeGroup // the matched nodeGroup: its extra_vars and vault file apply
}

// playbookRunner runs one playbook and reports whether it succeeded. Tests
// replace it, so the cycle can be tested without Ansible installed.
type playbookRunner interface {
	Run(run playbookRun) (succeeded bool)
}

// ansibleRunner runs playbooks with the ansible-playbook on PATH.
type ansibleRunner struct {
	workspaceDir string
}

// extraVarsFileName lives in the workspace, not in the checkout. Writing it into
// the repository dirtied the tree that git.EnsureRepo hard-resets. Issue #43.
const extraVarsFileName = "extra_vars.json"

// Run executes the playbook and saves Ansible's output, whatever the result.
//
//	ok := ansibleRunner{workspaceDir: ws}.Run(playbookRun{name: "web", file: f, workDir: d, mode: manifest.ApplyAuto})
func (r ansibleRunner) Run(run playbookRun) bool {
	cmd, err := r.command(run)
	if err != nil {
		logRunError(run, err)
		r.saveOutput(run, errorDocument(err))
		return false
	}
	succeeded, output := execute(run, cmd)
	r.saveOutput(run, output)
	return succeeded
}

// command builds the ansible-playbook invocation for a run.
func (r ansibleRunner) command(run playbookRun) (*exec.Cmd, error) {
	ansiblePath, err := exec.LookPath("ansible-playbook")
	if err != nil {
		return nil, fmt.Errorf("ansible-playbook executable not found in PATH: %w; nothing was run", err)
	}
	args, err := r.args(run)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(ansiblePath, args...)
	cmd.Dir = run.workDir
	cmd.Env = append(os.Environ(), "ANSIBLE_STDOUT_CALLBACK=json")
	return cmd, nil
}

// args returns the arguments after the executable. Separate from command so it
// can be tested on a machine without Ansible.
//
// A plan-mode playbook runs with --check --diff and changes nothing: it is the
// default for a playbook without `apply: auto` (ADR-0009 D4). Running a check
// before every auto apply is the next BACKLOG step (ADR-0008 D9).
func (r ansibleRunner) args(run playbookRun) ([]string, error) {
	args := []string{"-i", "localhost,", "-c", "local", run.file}
	if run.mode != manifest.ApplyAuto {
		args = append(args, "--check", "--diff")
	}
	if vaultFile := run.group.Ansible.VaultPasswordFile; vaultFile != "" {
		args = append(args, "--vault-password-file", vaultFile)
	}
	if len(run.group.Ansible.ExtraVars) == 0 {
		return args, nil
	}
	path, err := r.writeExtraVars(run.group.Ansible.ExtraVars)
	if err != nil {
		return nil, err
	}
	return append(args, "-e", "@"+path), nil
}

// writeExtraVars stores the group's variables in the workspace with mode 0600,
// since extra_vars may carry credentials. The error used to be discarded, so
// Ansible ran without the variables and nobody was told. Issue #43.
func (r ansibleRunner) writeExtraVars(vars map[string]string) (string, error) {
	path := filepath.Join(r.workspaceDir, extraVarsFileName)
	body, err := json.Marshal(vars)
	if err != nil {
		return "", fmt.Errorf("cannot encode extra_vars %v as JSON: %w", vars, err)
	}
	if err := writeFileAtomic(path, body, 0600); err != nil {
		return "", fmt.Errorf("cannot write extra_vars; nothing was run: %w", err)
	}
	return path, nil
}

// outputPath is where a playbook's latest Ansible output is kept. It replaces
// the task log Othela used to receive in a report; the versioned run record is
// a Phase 1 item. Mode 0600: a --diff can show secrets.
func (r ansibleRunner) outputPath(run playbookRun) string {
	name := strings.ReplaceAll(run.name, string(os.PathSeparator), "_")
	return filepath.Join(r.workspaceDir, "runs", name+".json")
}

func (r ansibleRunner) saveOutput(run playbookRun, output []byte) {
	path := r.outputPath(run)
	err := os.MkdirAll(filepath.Dir(path), 0700)
	if err == nil {
		err = writeFileAtomic(path, output, 0600)
	}
	if err != nil {
		logRunError(run, fmt.Errorf("cannot save Ansible output to %s: %w", path, err))
	}
}

// execute runs the command and returns whether it succeeded, with output that
// is always valid JSON.
func execute(run playbookRun, cmd *exec.Cmd) (bool, []byte) {
	logger := log.WithComponent("executor")
	logger.Debug().Str("command", cmd.String()).Str("work_dir", cmd.Dir).Msg("running ansible-playbook")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn().Err(err).Str("run_id", run.runID).Str("playbook", run.name).Str("mode", string(run.mode)).
			Msg("playbook execution failed")
		return false, asJSONOutput(run, output)
	}
	logger.Info().Str("run_id", run.runID).Str("playbook", run.name).Str("mode", string(run.mode)).
		Msg("playbook execution succeeded")
	return true, asJSONOutput(run, output)
}

// asJSONOutput passes valid JSON through and wraps anything else, so the
// saved output is always parseable.
func asJSONOutput(run playbookRun, output []byte) []byte {
	if json.Valid(output) {
		return output
	}
	logger := log.WithComponent("executor")
	logger.Warn().Str("run_id", run.runID).Str("playbook", run.name).Msg("ansible output was not valid JSON")
	wrapped, _ := json.Marshal(map[string]string{
		"raw_output": string(output),
		"error":      "Output was not valid JSON",
	})
	return wrapped
}

func errorDocument(err error) []byte {
	body, _ := json.Marshal(map[string]string{"error": err.Error()})
	return body
}

func logRunError(run playbookRun, err error) {
	logger := log.WithComponent("executor")
	logger.Error().Err(err).Str("run_id", run.runID).Str("playbook", run.name).Msg("playbook did not run")
}
