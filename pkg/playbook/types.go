// Package playbook provides functionality for discovering and loading
// Ansible playbooks from the filesystem.
package playbook

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"helvilette/pkg/manifest"
)

// Playbook represents metadata about an Ansible playbook or collection.
type Playbook struct {
	ID       string             `json:"id"`        // Unique ID (hash of path)
	Name     string             `json:"name"`      // Directory or file name
	Path     string             `json:"path"`      // Relative path from base dir
	FullPath string             `json:"full_path"` // Absolute path to playbook.yml
	ModTime  time.Time          `json:"mod_time"`  // Last modified time
	Manifest *manifest.Manifest `json:"manifest,omitempty"`
}

// GenerateID creates a unique ID from a path using SHA256.
func GenerateID(path string) string {
	hash := sha256.Sum256([]byte(path))
	return hex.EncodeToString(hash[:8]) // First 8 bytes = 16 hex chars
}
