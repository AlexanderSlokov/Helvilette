// Package types contains shared data types for Helvilette components.
// These types are used for communication between Othela (Control Plane) and Agent.
package types

import "encoding/json"

// Job represents a job that Othela sends to Agents.
// It contains an Ansible playbook to be executed on the target node.
type Job struct {
	JobID           string `json:"job_id"`
	PlaybookContent string `json:"playbook_content"`
	PlaybookPath    string `json:"playbook_path,omitempty"` // Full path to run from (enables role resolution)
}

// Report represents the execution report from an Agent.
// It is sent back to Othela after a job execution completes.
type Report struct {
	NodeID   string          `json:"node_id"`
	JobID    string          `json:"job_id"`
	Status   string          `json:"status"`   // Success, Failed
	TaskLogs json.RawMessage `json:"task_log"` // Ansible JSON output
}
