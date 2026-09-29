package playbook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"helvilette/pkg/log"
	"helvilette/pkg/manifest"
)

var (
	// ErrNotFound is returned when a playbook is not found.
	ErrNotFound = errors.New("playbook not found")
)

// ManifestFilename is the only filename Scan accepts. Anything that differs
// only in case or extension is reported as a near miss rather than loaded;
// see ADR-0006.
const ManifestFilename = "helvilette.yml"

// manifestStem is the part before the extension, used to spot near misses such
// as helvilette.yaml or Helvilette.yml.
const manifestStem = "helvilette"

// Loader discovers and loads Ansible playbooks from a base directory.
// It scans for directories containing a helvilette.yml manifest.
type Loader struct {
	baseDir string
	// mu guards playbooks. Scan runs on Othela's fleet-sync goroutine while
	// Get and GetByName are reached from HTTP handlers.
	mu        sync.RWMutex
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
		return "", fmt.Errorf("failed to resolve base directory %q: %w", baseDir, err)
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

// scanTally records what a single walk saw, so the closing log line can explain
// a count of zero instead of stating it without context. See ADR-0006.
type scanTally struct {
	filesExamined int
	skipped       int
	rejected      int
	nearMisses    int
}

// Scan discovers all playbooks in the base directory recursively.
// A playbook is identified by the presence of a helvilette.yml manifest.
//
// Every path the walk declines to load emits a log line: debug for the ordinary
// cases, warn for a manifest that failed validation or for a filename that only
// looks like a manifest. Issue #34 was filed because none of that was visible.
func (l *Loader) Scan() ([]Playbook, error) {
	logger := log.WithComponent("playbook-loader")
	discovered := make(map[string]*Playbook)
	tally := &scanTally{}
	var result []Playbook

	err := filepath.Walk(l.baseDir, func(path string, info os.FileInfo, walkErr error) error {
		pb, skip := l.evaluatePath(logger, tally, path, info, walkErr)
		if skip != nil {
			return skip
		}
		if pb == nil {
			return nil
		}
		discovered[pb.ID] = pb
		result = append(result, *pb)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan base directory %s: %w", l.baseDir, err)
	}

	l.mu.Lock()
	l.playbooks = discovered
	l.mu.Unlock()

	l.logScanOutcome(logger, tally, len(result))
	return result, nil
}

// evaluatePath decides what a single walked path is. It returns the playbook it
// produced, or a filepath.SkipDir sentinel, or neither when the path is simply
// not a manifest. Each outcome is logged before it returns.
func (l *Loader) evaluatePath(logger zerologger, tally *scanTally, path string, info os.FileInfo, walkErr error) (*Playbook, error) {
	if walkErr != nil {
		tally.skipped++
		logger.Warn().Err(walkErr).Str("path", path).Msg("failed to access path during scan, skipping")
		return nil, nil // continue walking
	}

	if info.IsDir() {
		return nil, l.evaluateDir(logger, tally, path, info)
	}

	tally.filesExamined++
	if info.Name() != ManifestFilename {
		l.logNonManifest(logger, tally, path, info.Name())
		return nil, nil
	}

	return l.loadManifest(logger, tally, path, info), nil
}

// evaluateDir skips dotted directories — .git above all, whose loose objects
// would otherwise be walked on every sync — and says so at debug level.
func (l *Loader) evaluateDir(logger zerologger, tally *scanTally, path string, info os.FileInfo) error {
	if path == l.baseDir || !strings.HasPrefix(info.Name(), ".") {
		return nil
	}

	tally.skipped++
	logger.Debug().Str("path", path).Str("skip_reason", "hidden_dir").Msg("skipped directory")
	return filepath.SkipDir
}

// logNonManifest distinguishes an ordinary file from one whose name only looks
// like a manifest. helvilette.yaml and Helvilette.yml are the two spellings
// operators reach for, and silently ignoring them is what issue #34 describes.
func (l *Loader) logNonManifest(logger zerologger, tally *scanTally, path, name string) {
	tally.skipped++

	if !isManifestNearMiss(name) {
		logger.Debug().Str("path", path).Str("skip_reason", "not_a_manifest").Msg("skipped file")
		return
	}

	tally.nearMisses++
	logger.Warn().Str("path", path).Str("found", name).Str("expected", ManifestFilename).
		Msg("filename looks like a manifest but does not match, playbook will not be dispatched — rename it")
}

// isManifestNearMiss reports whether name differs from ManifestFilename only in
// case or in the .yml/.yaml spelling.
func isManifestNearMiss(name string) bool {
	lower := strings.ToLower(name)
	return lower == manifestStem+".yml" || lower == manifestStem+".yaml"
}

func (l *Loader) loadManifest(logger zerologger, tally *scanTally, path string, info os.FileInfo) *Playbook {
	relPath, err := l.relativeDir(path)
	if err != nil {
		tally.skipped++
		logger.Warn().Err(err).Str("manifest", path).Msg("cannot place manifest relative to base dir, skipping")
		return nil
	}

	m, err := manifest.ParseFile(path)
	if err != nil {
		tally.rejected++
		logger.Warn().Err(err).Str("manifest", path).Str("skip_reason", "parse_rejected").
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
	return pb
}

// relativeDir names a playbook by its directory relative to the base dir.
// "root" stands in for a manifest sitting directly in the base dir, which has
// no relative name of its own.
func (l *Loader) relativeDir(manifestPath string) (string, error) {
	relPath, err := filepath.Rel(l.baseDir, filepath.Dir(manifestPath))
	if err != nil {
		return "", fmt.Errorf("path %s is not under base dir %s: %w", manifestPath, l.baseDir, err)
	}
	if relPath == "." {
		return "root", nil
	}

	return relPath, nil
}

// logScanOutcome closes every scan with the numbers behind the count. A count
// of zero is a warning, not an info line: it means no node will be dispatched
// anything, and issue #34 was filed because that state printed as {"count":0}
// with nothing else to act on.
func (l *Loader) logScanOutcome(logger zerologger, tally *scanTally, count int) {
	event := logger.Info()
	if count == 0 {
		event = logger.Warn()
	}

	event.
		Str("base_dir", l.baseDir).
		Int("count", count).
		Int("files_examined", tally.filesExamined).
		Int("skipped", tally.skipped).
		Int("rejected", tally.rejected).
		Int("near_misses", tally.nearMisses).
		Str("expected_filename", ManifestFilename).
		Msg("scan complete")
}

// Get returns playbook metadata by ID.
func (l *Loader) Get(id string) (*Playbook, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	pb, ok := l.playbooks[id]
	if !ok {
		return nil, ErrNotFound
	}
	return pb, nil
}

// GetByName returns playbook metadata by name (relative directory path).
func (l *Loader) GetByName(name string) (*Playbook, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

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
