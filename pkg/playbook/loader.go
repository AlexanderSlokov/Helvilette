package playbook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"helvilette/pkg/log"
	"helvilette/pkg/manifest"
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
	absPath, err := validateBaseDir(baseDir)
	if err != nil {
		return nil, err
	}

	return &Loader{
		baseDir:   absPath,
		playbooks: make(map[string]*Playbook),
	}, nil
}

func validateBaseDir(baseDir string) (string, error) {
	absPath, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("base directory does not exist: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("base path is not a directory: %s", absPath)
	}

	return absPath, nil
}

// Scan discovers all playbooks in the base directory recursively.
// A playbook is identified by the presence of a helvilette.yml manifest.
func (l *Loader) Scan() ([]Playbook, error) {
	logger := log.WithComponent("playbook-loader")
	l.playbooks = make(map[string]*Playbook)
	var result []Playbook

	err := filepath.Walk(l.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Warn().Err(err).Str("path", path).Msg("failed to access path during scan")
			return nil // continue walking
		}

		// Skip hidden directories (like .git, .hidden)
		if info.IsDir() && len(info.Name()) > 0 && info.Name()[0] == '.' && path != l.baseDir {
			return filepath.SkipDir
		}

		// We only care about helvilette.yml files
		if info.IsDir() || info.Name() != "helvilette.yml" {
			return nil
		}

		dirPath := filepath.Dir(path)
		// Use relative path from baseDir as the name for identification
		relPath, _ := filepath.Rel(l.baseDir, dirPath)
		if relPath == "." {
			relPath = "root" // edge case if helvilette.yml is at baseDir root
		}

		m, loadErr := manifest.ParseFile(path)
		if loadErr != nil {
			logger.Warn().Err(loadErr).Str("manifest", path).
				Msg("rejected manifest, playbook will not be dispatched to any node")
			return nil
		}

		pb := &Playbook{
			ID:       GenerateID(relPath),
			Name:     relPath,
			Path:     relPath,
			FullPath: path, // Keep path to manifest for reference
			ModTime:  info.ModTime(),
			Manifest: m,
		}

		logger.Debug().Str("id", pb.ID).Str("name", pb.Name).Str("manifest", path).Msg("discovered playbook manifest")
		l.playbooks[pb.ID] = pb
		result = append(result, *pb)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan base directory: %w", err)
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

// GetByName returns playbook metadata by name (relative directory path).
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
