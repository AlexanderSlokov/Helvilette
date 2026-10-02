// Running a playbook: materialising the job's Git repository and handing it to
// ansible-playbook, then capturing what Ansible said about it.
//
// Split out of main.go, which exceeded the 500-line ceiling in AGENTS.md.
// See BACKLOG 6.7.
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
		// The resolved SHA is logged, not just the requested ref: a branch name
		// in the log says nothing about which code actually ran. See ADR-0005.
		commit, err := git.EnsureRepo(job.RepoURL, repoDir, job.Version)
		if err != nil {
			logger.Error().Err(err).Str("repo", job.RepoURL).Str("ref", job.Version).Msg("failed to ensure git repo")
			b, _ := json.Marshal(map[string]string{"error": fmt.Sprintf("Failed to pull git repo: %v", err)})
			return "Failed", b
		}
		logger.Info().Str("job_id", job.JobID).Str("ref", job.Version).Str("commit", commit).Msg("repo checked out")

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
