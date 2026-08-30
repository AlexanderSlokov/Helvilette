// Package types contains shared data types for Helvilette components.
// These types are used for communication between Othela (Control Plane) and Agent.
package types

import (
	"encoding/json"
	"time"
)

// Job represents a unit of work that Othela dispatches to an Agent.
// A Job always references a playbook via Git (RepoURL + Version) or a local
// path (PlaybookPath). Inline content delivery was removed; see issue #25.
type Job struct {
	JobID        string            `json:"job_id"`
	RepoURL      string            `json:"repo_url,omitempty"`      // git@github.com:org/playbooks.git
	Version      string            `json:"version,omitempty"`       // commit SHA, tag, or branch
	PlaybookPath string            `json:"playbook_path,omitempty"` // full path or repo-relative path
	ExtraVars    map[string]string `json:"extra_vars,omitempty"`    // variables to pass to Ansible
}

// NodeStatus represents the current state of a node regarding its last run playbook.
// It is recorded by the agent locally before execution and reported after completion.
type NodeStatus struct {
	JobID     string    `json:"job_id"`
	CommitSHA string    `json:"commit_sha"`
	Status    string    `json:"status"` // e.g., "InProgress", "Success", "Failed"
	AppliedAt time.Time `json:"applied_at"`
}

// Report represents the execution report from an Agent.
// It is sent back to Othela after a job execution completes.
type Report struct {
	NodeID     string          `json:"node_id"`
	JobID      string          `json:"job_id"`
	Status     string          `json:"status"`   // Success, Failed
	TaskLogs   json.RawMessage `json:"task_log"` // Ansible JSON output
	ObservedAt time.Time       `json:"observed_at"` // When the node observed this state
	NodeStatus NodeStatus      `json:"node_status,omitempty"` // Complete node state summary
}
