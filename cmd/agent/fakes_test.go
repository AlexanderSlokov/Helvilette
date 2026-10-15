// Named fakes for the agent's two external dependencies, Git and Ansible, so the
// cycle can be tested without a Git server or Ansible installed (AGENTS.md:
// mock external I/O with named fake classes).
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeRepo stands in for gitRepoSyncer. Each Sync writes files into the
// checkout directory and reports commit as the resolved SHA.
type fakeRepo struct {
	t       *testing.T
	commit  string
	files   map[string]string // repo-relative path to content
	syncErr error
	syncs   int
}

func (f *fakeRepo) Sync(_, dir, _ string) (string, error) {
	f.syncs++
	if f.syncErr != nil {
		return "", f.syncErr
	}
	if err := os.RemoveAll(dir); err != nil {
		f.t.Fatalf("fakeRepo: %v", err)
	}
	for name, content := range f.files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			f.t.Fatalf("fakeRepo: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			f.t.Fatalf("fakeRepo: %v", err)
		}
	}
	return f.commit, nil
}

// recordingRunner stands in for ansibleRunner. It records every run and fails
// the playbooks named in failing.
type recordingRunner struct {
	runs    []playbookRun
	failing map[string]bool
}

func (r *recordingRunner) Run(run playbookRun) bool {
	r.runs = append(r.runs, run)
	return !r.failing[run.name]
}

func (r *recordingRunner) names() []string {
	names := make([]string, 0, len(r.runs))
	for _, run := range r.runs {
		names = append(names, run.name)
	}
	return names
}
