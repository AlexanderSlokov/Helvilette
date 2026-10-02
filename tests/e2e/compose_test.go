package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// composeFile is the single definition of the e2e stack. The suite drives this
// file rather than restating the topology in Go: the previous suite declared the
// same four services the compose file did, and the pair had to be kept in step
// by hand. Issue #33 is what that looks like when it fails. See ADR-0007.
const composeFile = "e2e.compose.yml"

// upTimeout bounds `docker compose up --wait`. Generous because a cold run
// compiles both binaries and pulls three base images.
const upTimeout = 12 * time.Minute

// repoRoot resolves the repository root, which is both the compose project
// directory and the build context for every image.
func repoRoot() (string, error) {
	return filepath.Abs("../..")
}

// compose runs a docker compose subcommand against the e2e stack and returns its
// combined output. The output is returned on failure too: a compose error is
// usually only intelligible from what the daemon printed.
func compose(timeout time.Duration, args ...string) (string, error) {
	root, err := repoRoot()
	if err != nil {
		return "", fmt.Errorf("cannot resolve repository root: %w", err)
	}

	full := append([]string{"compose", "-f", filepath.Join(root, composeFile)}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Dir = root

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := runWithTimeout(cmd, timeout); err != nil {
		return out.String(), fmt.Errorf("docker %s: %w", strings.Join(full, " "), err)
	}
	return out.String(), nil
}

func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) error {
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return fmt.Errorf("timed out after %s", timeout)
	}
}

// composeUp builds the images and starts the stack, returning once every service
// reports healthy. Readiness comes from the healthcheck blocks in the compose
// file, so it is declared once and honoured by `make up` as well.
func composeUp() (string, error) {
	return compose(upTimeout, "up", "--detach", "--build", "--wait", "--wait-timeout", "600")
}

// composeDown removes the stack and its volumes.
func composeDown() (string, error) {
	return compose(3*time.Minute, "down", "--volumes", "--remove-orphans")
}

// serviceLogs returns everything one service has written to its container log.
// For the systemd nodes that is the boot transcript; per-unit output lives in
// the journal and is read with journal instead.
func serviceLogs(service string) (string, error) {
	return compose(time.Minute, "logs", "--no-color", service)
}

// exec runs a command inside a running service.
func execIn(service string, args ...string) (string, error) {
	return compose(2*time.Minute, append([]string{"exec", "-T", service}, args...)...)
}

// journal reads one syslog identifier out of a node's journal. This is the
// operator path Helvilette is meant to support: `journalctl -t 'helvilette*'`
// selects the product without knowing how its units were named. See ADR-0007 D2.
func journal(service, identifier string) (string, error) {
	return execIn(service, "journalctl", "--no-pager", "-t", identifier, "-o", "cat")
}

// seedCommit adds a commit to a served repository and returns the new SHA. The
// helper lives in the git server image so a test and a human at a terminal use
// the same one.
func seedCommit(repo, path, content string) (string, error) {
	out, err := execIn("git-server", "seed-commit.sh", repo, path, content)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(lastLine(out)), nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// journalTail is how much of a node's journal a failure report carries. Enough
// to see the last job cycle, bounded so the report stays readable.
const journalTail = "40"

// diagnose gathers what explains a failed spec, while the stack is still up.
//
// It has to run from inside the suite: AfterSuite tears the stack down, so a
// CI step that shells out afterwards finds no containers and prints nothing,
// which is exactly what the first version of this did.
func diagnose() string {
	var report strings.Builder

	if out, err := compose(time.Minute, "ps", "-a"); err == nil {
		report.WriteString("--- compose ps ---\n" + out)
	}

	othela, err := journal("othela", "helvilette-othela")
	if err == nil {
		if failures := failedAnsibleTasks(othela); failures != "" {
			report.WriteString("--- failed ansible tasks, per Othela's reports ---\n" + failures)
		}
	}

	for _, service := range []string{"othela", "node-1", "node-2"} {
		report.WriteString("--- journal " + service + " (last " + journalTail + ") ---\n")
		report.WriteString(journalOf(service))
	}

	return report.String()
}

// journalOf returns the tail of one service's Helvilette journal, or the reason
// it could not be read. Never returns an error: a diagnostic that fails to
// gather must not replace the failure it was gathering for.
func journalOf(service string) string {
	out, err := execIn(service, "journalctl", "--no-pager", "-t", "helvilette-othela",
		"-t", "helvilette-agent", "-o", "cat", "-n", journalTail)
	if err != nil {
		return "unavailable: " + err.Error() + "\n"
	}
	return out
}

// ansibleReport is the shape Othela logs on a report: the status it recorded and
// the Ansible JSON callback output verbatim.
type ansibleReport struct {
	Message string `json:"message"`
	NodeID  string `json:"node_id"`
	Status  string `json:"status"`
	TaskLog struct {
		Plays []struct {
			Tasks []struct {
				Task  struct{ Name string } `json:"task"`
				Hosts map[string]struct {
					Failed bool   `json:"failed"`
					Msg    string `json:"msg"`
					Stderr string `json:"stderr"`
				} `json:"hosts"`
			} `json:"tasks"`
		} `json:"plays"`
	} `json:"task_logs"`
}

// failedAnsibleTasks pulls the task name and message of every failed Ansible
// task out of Othela's journal. Without this a failing playbook shows up only as
// "exit status 2", and the reason sits unread inside a report payload.
//
// Identical failures are collapsed with a count. An agent retries every poll
// interval, so a single broken task otherwise fills the report with dozens of
// copies of itself and buries anything else that failed.
func failedAnsibleTasks(journalLines string) string {
	counts := make(map[string]int)
	var order []string

	for line := range strings.SplitSeq(journalLines, "\n") {
		var entry ansibleReport
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Message != "report received" || entry.Status != "Failed" {
			continue
		}
		for _, failure := range describeFailures(entry) {
			if counts[failure] == 0 {
				order = append(order, failure)
			}
			counts[failure]++
		}
	}

	var out strings.Builder
	for _, failure := range order {
		fmt.Fprintf(&out, "%s  (seen %d times)\n", failure, counts[failure])
	}
	return out.String()
}

// describeFailures renders one line per failed task in a single report.
func describeFailures(entry ansibleReport) []string {
	var failures []string

	for _, play := range entry.TaskLog.Plays {
		for _, task := range play.Tasks {
			for host, result := range task.Hosts {
				if !result.Failed {
					continue
				}
				failure := fmt.Sprintf("%s/%s: %s\n  msg: %s", entry.NodeID, host, task.Task.Name, result.Msg)
				if result.Stderr != "" {
					failure += "\n  stderr: " + result.Stderr
				}
				failures = append(failures, failure)
			}
		}
	}

	return failures
}
