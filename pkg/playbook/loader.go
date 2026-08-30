package playbook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"

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

// Scan discovers all playbooks in the base directory.
// A playbook is identified by a directory containing a playbook.yml file.
func (l *Loader) Scan() ([]Playbook, error) {
	logger := log.WithComponent("playbook-loader")
	l.playbooks = make(map[string]*Playbook)

	entries, err := os.ReadDir(l.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read base directory: %w", err)
	}

	var result []Playbook
	for _, entry := range entries {
		pb, ok := l.inspectDirEntry(entry, logger)
		if !ok {
			continue
		}
		l.playbooks[pb.ID] = pb
		result = append(result, *pb)
	}

	logger.Info().Int("count", len(result)).Msg("scan complete")
	return result, nil
}

func (l *Loader) inspectDirEntry(entry os.DirEntry, logger zerolog.Logger) (*Playbook, bool) {
	if shouldSkipEntry(entry) {
		return nil, false
	}

	dirName := entry.Name()
	playbookPath := filepath.Join(l.baseDir, dirName, "playbook.yml")
	info, err := os.Stat(playbookPath)
	if err != nil {
		return nil, false
	}

	pb := buildPlaybook(dirName, playbookPath, info.ModTime(), l.loadManifest(dirName, logger))
	logger.Debug().Str("id", pb.ID).Str("name", pb.Name).Str("path", pb.FullPath).Msg("discovered playbook")

	return pb, true
}

func shouldSkipEntry(entry os.DirEntry) bool {
	return !entry.IsDir() || entry.Name()[0] == '.'
}

func buildPlaybook(dirName, playbookPath string, modTime time.Time, m *manifest.Manifest) *Playbook {
	return &Playbook{
		ID:       GenerateID(dirName),
		Name:     dirName,
		Path:     dirName,
		FullPath: playbookPath,
		ModTime:  modTime,
		Manifest: m,
	}
}

func (l *Loader) loadManifest(dirName string, logger zerolog.Logger) *manifest.Manifest {
	manifestPath := filepath.Join(l.baseDir, dirName, "helvilette.yml")
	m, err := manifest.ParseFile(manifestPath)
	if err == nil {
		logger.Debug().Str("manifest", manifestPath).Msg("loaded manifest for playbook")
		return m
	}
	if !os.IsNotExist(err) {
		// Loud on purpose: without a manifest the playbook matches no nodeSelector,
		// so it goes silently undeployed unless the operator sees this line.
		logger.Warn().Err(err).Str("manifest", manifestPath).
			Msg("rejected manifest, playbook will not be dispatched to any node")
	}
	return nil
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
