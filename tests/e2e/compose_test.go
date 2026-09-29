package e2e_test

import (
	"bytes"
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
