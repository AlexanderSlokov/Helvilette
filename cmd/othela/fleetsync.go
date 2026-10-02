// Fleet synchronisation: pulling the Git repository that holds the manifests and
// republishing what it contains. Split out of server.go, which crossed the
// 500-line ceiling in AGENTS.md when this landed. See BACKLOG 6.7.
package main

import (
	"context"
	"time"

	"helvilette/pkg/git"
	"helvilette/pkg/playbook"
)

// FleetSyncConfig names the fleet repository and how often to re-read it.
// Grouped into a struct so StartFleetSync does not take four positional
// strings-and-durations that are easy to transpose at the call site.
type FleetSyncConfig struct {
	Repo     string // Git URL of the repository holding helvilette.yml manifests
	Branch   string // Branch, tag or commit SHA to track
	CacheDir string // Local checkout, owned by Othela and treated as disposable
	Interval time.Duration
}

// StartFleetSync syncs the fleet repository once, then keeps re-syncing on
// cfg.Interval until ctx is cancelled.
//
// The first sync runs synchronously so a server that has started is a server
// whose playbooks are already loaded. Cancelling ctx stops the loop; before
// issue #32 the goroutine had no exit and outlived graceful shutdown.
//
//	ctx, cancel := context.WithCancel(context.Background())
//	server.StartFleetSync(ctx, FleetSyncConfig{Repo: url, Branch: "main", CacheDir: dir, Interval: time.Minute})
func (s *Server) StartFleetSync(ctx context.Context, cfg FleetSyncConfig) {
	s.syncFleetOnce(cfg)

	go func() {
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				logger.Info().Msg("fleet sync loop stopped")
				return
			case <-ticker.C:
				s.syncFleetOnce(cfg)
			}
		}
	}()
}

// syncFleetOnce pulls the fleet repository and republishes the playbooks it
// contains. Every failure path logs and returns, leaving the previously loaded
// playbooks in place: a transient Git outage must not empty the fleet.
func (s *Server) syncFleetOnce(cfg FleetSyncConfig) {
	commit, err := git.EnsureRepo(cfg.Repo, cfg.CacheDir, cfg.Branch)
	if err != nil {
		logger.Error().Err(err).Str("repo", cfg.Repo).Str("branch", cfg.Branch).
			Msg("failed to sync fleet repository — check repo URL accessibility, branch name and credentials")
		return
	}

	loader, err := s.ensureLoader(cfg.CacheDir)
	if err != nil {
		logger.Error().Err(err).Str("cache_dir", cfg.CacheDir).Msg("failed to create playbook loader")
		return
	}

	playbooks, err := loader.Scan()
	if err != nil {
		logger.Error().Err(err).Str("cache_dir", cfg.CacheDir).Msg("failed to scan playbooks")
		return
	}

	s.publishFleet(playbooks, commit)
}

// ensureLoader returns the loader for cacheDir, creating it on first use.
// Holds mu because the sync goroutine writes s.loader while GetLoader reads it.
func (s *Server) ensureLoader(cacheDir string) (*playbook.Loader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loader != nil && s.loader.BaseDir() == cacheDir {
		return s.loader, nil
	}

	loader, err := playbook.NewLoader(cacheDir)
	if err != nil {
		return nil, err
	}
	s.loader = loader

	return loader, nil
}

// publishFleet swaps in the newly scanned playbooks and reports the outcome.
// A commit that moved logs at info: before issue #32 a successful sync was
// invisible at the default level, so an operator could not tell a working
// poll loop from a dead one.
func (s *Server) publishFleet(playbooks []playbook.Playbook, commit string) {
	s.mu.Lock()
	previous := s.lastCommit
	s.playbooks = playbooks
	s.lastCommit = commit
	s.mu.Unlock()

	if previous == commit {
		logger.Debug().Str("fleet_commit", commit).Int("playbook_count", len(playbooks)).
			Msg("fleet sync complete, no change")
		return
	}

	logger.Info().Str("fleet_commit", commit).Str("previous_commit", previous).
		Int("playbook_count", len(playbooks)).Msg("fleet updated")
}

// GetLoader returns the playbook loader (for testing)
func (s *Server) GetLoader() *playbook.Loader {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loader
}

// LastCommit returns the commit SHA the fleet cache last resolved to, or an
// empty string before the first successful sync.
func (s *Server) LastCommit() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastCommit
}
