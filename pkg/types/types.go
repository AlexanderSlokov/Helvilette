// Package types contains shared data types for Helvilette components.
// These types are used for communication between Othela (Control Plane) and Agent.
package types

import "encoding/json"

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

// Report represents the execution report from an Agent.
// It is sent back to Othela after a job execution completes.
type Report struct {
	NodeID   string          `json:"node_id"`
	JobID    string          `json:"job_id"`
	Status   string          `json:"status"`   // Success, Failed
	TaskLogs json.RawMessage `json:"task_log"` // Ansible JSON output
}
