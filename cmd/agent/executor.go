// Running a playbook: materialising the job's Git repository and handing it to
// ansible-playbook, then capturing what Ansible said about it.
//
// Split out of main.go, which exceeded the 500-line ceiling in AGENTS.md.
// See BACKLOG 6.7. ExecutePlaybook itself was later split into one function
// per step, from about 110 lines; see issue #43.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"helvilette/pkg/git"
	"helvilette/pkg/log"
)

// playbookTarget is where a job's playbook lives on this node, and the
// directory ansible-playbook runs from so that roles/ resolves.
type playbookTarget struct {
	file    string
	workDir string
}

// extraVarsFileName lives in the workspace, not in the checkout. Writing it into
// the repository dirtied the tree that git.EnsureRepo hard-resets. Issue #43.
const extraVarsFileName = "extra_vars.json"

// ExecutePlaybook runs the job's playbook and returns its status and Ansible's
// JSON output, or a JSON error document when it never got as far as Ansible.
//
//	status, output := agent.ExecutePlaybook(&Job{JobID: "j1", RepoURL: url, PlaybookPath: "site.yml"})
func (a *Agent) ExecutePlaybook(job *Job) (status string, output []byte) {
	if err := a.ensureWorkspace(); err != nil {
		return failedRun(job.JobID, err)
	}
	target, err := a.resolvePlaybook(job)
	if err != nil {
		return failedRun(job.JobID, err)
	}
	cmd, err := a.ansibleCommand(job, target)
	if err != nil {
		return failedRun(job.JobID, err)
	}
	return runAnsible(job.JobID, cmd)
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

// resolvePlaybook decides where the job's playbook is: in a Git checkout when
// the job names a repository, otherwise at the path it gives.
func (a *Agent) resolvePlaybook(job *Job) (playbookTarget, error) {
	if job.RepoURL != "" {
		return a.checkoutPlaybook(job)
	}
	if job.PlaybookPath != "" {
		logPlaybookTarget(job.JobID, job.PlaybookPath)
		return playbookTarget{file: job.PlaybookPath, workDir: filepath.Dir(job.PlaybookPath)}, nil
	}
	// Before issue #25 this case wrote PlaybookContent to a temp file, but
	// inline content delivery has been removed from the wire format.
	return playbookTarget{}, fmt.Errorf("job %q has neither repo_url nor playbook_path; "+
		"inline content delivery is no longer supported", job.JobID)
}

// checkoutPlaybook materialises the job's repository and returns the playbook
// inside it, defaulting to playbook.yml at the repository root.
func (a *Agent) checkoutPlaybook(job *Job) (playbookTarget, error) {
	repoDir := filepath.Join(a.config.WorkspaceDir, "repos", filepath.Base(job.RepoURL))
	// The resolved SHA is logged, not just the requested ref: a branch name
	// in the log says nothing about which code actually ran. See ADR-0005.
	commit, err := git.EnsureRepo(job.RepoURL, repoDir, job.Version)
	if err != nil {
		return playbookTarget{}, fmt.Errorf("failed to pull git repo %q at ref %q: %w", job.RepoURL, job.Version, err)
	}
	logger := log.WithComponent("executor")
	logger.Info().Str("job_id", job.JobID).Str("repo_url", job.RepoURL).
		Str("ref", job.Version).Str("commit", commit).Msg("repo checked out")

	file := filepath.Join(repoDir, "playbook.yml")
	if job.PlaybookPath != "" && !filepath.IsAbs(job.PlaybookPath) {
		file = filepath.Join(repoDir, job.PlaybookPath)
	}
	return playbookTarget{file: file, workDir: repoDir}, nil
}

func logPlaybookTarget(jobID, file string) {
	logger := log.WithComponent("executor")
	logger.Info().Str("job_id", jobID).Str("playbook_path", file).Msg("executing playbook from source path")
}

// ansibleCommand builds the ansible-playbook invocation for a resolved target.
func (a *Agent) ansibleCommand(job *Job, target playbookTarget) (*exec.Cmd, error) {
	ansiblePath, err := exec.LookPath("ansible-playbook")
	if err != nil {
		return nil, fmt.Errorf("ansible-playbook executable not found in PATH: %w", err)
	}
	args, err := a.ansibleArgs(job, target)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(ansiblePath, args...)
	cmd.Dir = target.workDir
	cmd.Env = append(os.Environ(), "ANSIBLE_STDOUT_CALLBACK=json")
	return cmd, nil
}

// ansibleArgs returns the arguments after the executable. Separate from
// ansibleCommand so it can be tested on a machine without Ansible.
func (a *Agent) ansibleArgs(job *Job, target playbookTarget) ([]string, error) {
	args := []string{"-i", "localhost,", "-c", "local", target.file}
	if len(job.ExtraVars) == 0 {
		return args, nil
	}
	path, err := a.writeExtraVars(job.ExtraVars)
	if err != nil {
		return nil, err
	}
	return append(args, "-e", "@"+path), nil
}

// writeExtraVars stores the job's variables in the workspace with mode 0600,
// since extra_vars may carry credentials. The error used to be discarded, so
// Ansible ran without the variables and nobody was told. Issue #43.
func (a *Agent) writeExtraVars(vars map[string]string) (string, error) {
	path := filepath.Join(a.config.WorkspaceDir, extraVarsFileName)
	body, err := json.Marshal(vars)
	if err != nil {
		return "", fmt.Errorf("cannot encode extra_vars %v as JSON: %w", vars, err)
	}
	if err := writeFileAtomic(path, body, 0600); err != nil {
		return "", fmt.Errorf("cannot write extra_vars; nothing was run: %w", err)
	}
	return path, nil
}

// runAnsible executes the command and returns its status with output that is
// always valid JSON, wrapping Ansible's raw text when it is not.
func runAnsible(jobID string, cmd *exec.Cmd) (status string, output []byte) {
	logger := log.WithComponent("executor")
	logger.Debug().Str("command", cmd.String()).Str("work_dir", cmd.Dir).Msg("running ansible-playbook")

	output, err := cmd.CombinedOutput()
	status = "Success"
	if err != nil {
		status = "Failed"
		logger.Warn().Err(err).Str("job_id", jobID).Msg("playbook execution failed")
	} else {
		logger.Info().Str("job_id", jobID).Msg("playbook execution succeeded")
	}
	return status, asJSONOutput(jobID, output)
}

// asJSONOutput passes valid JSON through and wraps anything else, so the
// report's task log is always parseable.
func asJSONOutput(jobID string, output []byte) []byte {
	if json.Valid(output) {
		return output
	}
	logger := log.WithComponent("executor")
	logger.Warn().Str("job_id", jobID).Msg("ansible output was not valid JSON")
	wrapped, _ := json.Marshal(map[string]string{
		"raw_output": string(output),
		"error":      "Output was not valid JSON",
	})
	return wrapped
}

// failedRun logs why a job stopped before or around Ansible and returns the
// reason as the JSON error document the report carries.
func failedRun(jobID string, err error) (string, []byte) {
	logger := log.WithComponent("executor")
	logger.Error().Err(err).Str("job_id", jobID).Msg("job failed before ansible-playbook finished")
	body, _ := json.Marshal(map[string]string{"error": err.Error()})
	return "Failed", body
}
