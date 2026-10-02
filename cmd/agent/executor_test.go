// Tests for executor.go: what the agent refuses to run.
package main

import (
	"testing"
)

func TestAgent_ExecutePlaybook_RejectsEmptyJob(t *testing.T) {
	agent := NewAgent(DefaultConfig())

	job := &Job{
		JobID: "test-empty-123",
	}

	status, output := agent.ExecutePlaybook(job)
	if status != "Failed" {
		t.Errorf("expected status %q, got %q", "Failed", status)
	}

	if len(output) == 0 {
		t.Fatal("expected non-empty error output")
	}
}
