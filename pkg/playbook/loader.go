package playbook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"helvilette/pkg/log"
)

var (
	// ErrNotFound is returned when a playbook is not found.
	ErrNotFound = errors.New("playbook not found")
)

// Loader discovers and loads Ansible playbooks from a base directory.
// It scans for directories containing playbook.yml files.
type Loader struct {
	baseDir   string
	playbooks map[string]*Playbook // indexed by ID
}

// NewLoader creates a new playbook loader for the given base directory.
func NewLoader(baseDir string) (*Loader, error) {
	absPath, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base directory: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("base directory does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("base path is not a directory: %s", absPath)
	}

	return &Loader{
		baseDir:   absPath,
		playbooks: make(map[string]*Playbook),
	}, nil
}

// Scan discovers all playbooks in the base directory.
// A playbook is identified by a directory containing a playbook.yml file.
func (l *Loader) Scan() ([]Playbook, error) {
	logger := log.WithComponent("playbook-loader")
	l.playbooks = make(map[string]*Playbook) // Reset cache

	entries, err := os.ReadDir(l.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read base directory: %w", err)
	}

	var result []Playbook

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip hidden directories
		if entry.Name()[0] == '.' {
			continue
		}

		// Check for playbook.yml in this directory
		playbookPath := filepath.Join(l.baseDir, entry.Name(), "playbook.yml")
		info, err := os.Stat(playbookPath)
		if err != nil {
			// No playbook.yml, skip this directory
			continue
		}

		relPath := entry.Name()
		pb := Playbook{
			ID:       GenerateID(relPath),
			Name:     entry.Name(),
			Path:     relPath,
			FullPath: playbookPath,
			ModTime:  info.ModTime(),
		}

		l.playbooks[pb.ID] = &pb
		result = append(result, pb)

		logger.Debug().
			Str("id", pb.ID).
			Str("name", pb.Name).
			Str("path", pb.FullPath).
			Msg("discovered playbook")
	}

	logger.Info().Int("count", len(result)).Msg("scan complete")
	return result, nil
}

// Get returns playbook metadata by ID.
func (l *Loader) Get(id string) (*Playbook, error) {
	pb, ok := l.playbooks[id]
	if !ok {
		return nil, ErrNotFound
	}
	return pb, nil
}

// Load reads and returns the playbook content by ID.
func (l *Loader) Load(id string) (string, error) {
	pb, err := l.Get(id)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(pb.FullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read playbook: %w", err)
	}

	return string(content), nil
}

// GetByName returns playbook metadata by name (directory name).
func (l *Loader) GetByName(name string) (*Playbook, error) {
	for _, pb := range l.playbooks {
		if pb.Name == name {
			return pb, nil
		}
	}
	return nil, ErrNotFound
}

// BaseDir returns the base directory path.
func (l *Loader) BaseDir() string {
	return l.baseDir
}
