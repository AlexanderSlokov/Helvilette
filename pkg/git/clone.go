// Package git wraps go-git behind the narrow surface Helvilette needs: make a
// local directory hold a given ref of a remote repository, and report which
// commit that turned out to be.
package git

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	"helvilette/pkg/log"
)

var logger = log.WithComponent("fleet-git")

// remoteName is the only remote Helvilette ever creates. Ref resolution reads
// refs/remotes/<remoteName>/, so the two must stay in step.
const remoteName = "origin"

// fetchRefSpec is stated explicitly rather than inherited from the remote's
// stored config, so resolution below cannot be broken by a repository someone
// reconfigured by hand.
const fetchRefSpec = "+refs/heads/*:refs/remotes/" + remoteName + "/*"

// EnsureRepo makes destDir a checkout of url at ref and returns the resolved
// commit SHA.
//
// ref is a branch name, a tag name, or a full commit SHA; empty means the
// branch the clone landed on. destDir is treated as a disposable cache: local
// changes are discarded on every call, and a directory that is not a readable
// repository is deleted and cloned again. Callers compare the returned SHA
// across calls to tell a no-op sync from one that moved the fleet forward.
// See ADR-0005.
//
//	sha, err := git.EnsureRepo("https://git.example.com/fleet.git", "/var/lib/helvilette/othela/fleet", "main")
func EnsureRepo(url, destDir, ref string) (string, error) {
	repo, err := openOrCloneRepo(url, destDir)
	if err != nil {
		return "", err
	}

	hash, err := resolveRemoteRef(repo, ref)
	if err != nil {
		return "", err
	}

	if err := resetWorktree(repo, hash); err != nil {
		return "", err
	}

	return hash.String(), nil
}

func openOrCloneRepo(url, destDir string) (*git.Repository, error) {
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		return cloneRepo(url, destDir)
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat destination dir %s: %w", destDir, err)
	}

	repo, err := git.PlainOpen(destDir)
	if err != nil {
		return recloneUnusable(url, destDir, err)
	}

	if err := fetchRemote(repo, destDir); err != nil {
		return nil, err
	}

	return repo, nil
}

func cloneRepo(url, destDir string) (*git.Repository, error) {
	repo, err := git.PlainClone(destDir, false, &git.CloneOptions{
		URL:        url,
		RemoteName: remoteName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repo from %s into %s: %w", url, destDir, err)
	}

	logger.Info().Str("url", url).Str("dest_dir", destDir).Msg("fleet cache cloned")
	return repo, nil
}

// recloneUnusable discards destDir and clones again. Reached only when the
// directory exists but will not open as a repository — a half-finished clone,
// which every later sync would otherwise keep failing on. A fetch failure does
// not come here: a network outage must leave the existing checkout intact.
func recloneUnusable(url, destDir string, openErr error) (*git.Repository, error) {
	logger.Warn().Err(openErr).Str("dest_dir", destDir).
		Msg("fleet cache is not a readable git repository, discarding it and cloning again")

	if err := os.RemoveAll(destDir); err != nil {
		return nil, fmt.Errorf("failed to discard unusable repo at %s: %w", destDir, err)
	}

	return cloneRepo(url, destDir)
}

func fetchRemote(repo *git.Repository, destDir string) error {
	err := repo.Fetch(&git.FetchOptions{
		RemoteName: remoteName,
		RefSpecs:   []config.RefSpec{config.RefSpec(fetchRefSpec)},
		Force:      true,
		Tags:       git.AllTags,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("failed to fetch repo at %s: %w", destDir, err)
	}

	return nil
}

// resolveRemoteRef resolves ref against what the remote just advertised, never
// against a local branch.
//
// This is the fix for issue #32. Fetch advances refs/remotes/origin/* only, but
// go-git's ResolveRevision consults refs/heads/<ref> first (see
// plumbing.RefRevParseRules), and refs/heads/main is left wherever the clone
// landed. Othela therefore re-checked-out its clone-time commit on every poll
// and never saw a manifest pushed after startup. See ADR-0005.
func resolveRemoteRef(repo *git.Repository, ref string) (plumbing.Hash, error) {
	if ref == "" {
		clonedRef, err := clonedBranch(repo)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		ref = clonedRef
	}

	candidates := []plumbing.ReferenceName{
		plumbing.NewRemoteReferenceName(remoteName, ref),
		plumbing.NewTagReferenceName(ref),
	}
	for _, name := range candidates {
		if resolved, err := repo.Reference(name, true); err == nil {
			return resolved.Hash(), nil
		}
	}

	return resolveRefAsCommitSHA(repo, ref, candidates)
}

// resolveRefAsCommitSHA accepts a ref that named no remote branch and no tag,
// provided it is a commit SHA already present locally. candidates is carried in
// only so the error can name what was tried.
func resolveRefAsCommitSHA(repo *git.Repository, ref string, candidates []plumbing.ReferenceName) (plumbing.Hash, error) {
	hash := plumbing.NewHash(ref)
	if !hash.IsZero() {
		if _, err := repo.CommitObject(hash); err == nil {
			return hash, nil
		}
	}

	tried := make([]string, 0, len(candidates))
	for _, name := range candidates {
		tried = append(tried, name.String())
	}

	return plumbing.ZeroHash, fmt.Errorf(
		"cannot resolve ref %q: no such reference (tried %v) and not a commit present in the repository; "+
			"expected a branch name such as %q, a tag name, or a full 40-character commit SHA",
		ref, tried, "main")
}

// clonedBranch returns the short name of the branch HEAD points at. Used when
// no ref is configured: a full clone does not create refs/remotes/origin/HEAD,
// so the local HEAD is the only record of which branch the remote handed us.
func clonedBranch(repo *git.Repository) (string, error) {
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("no ref given and HEAD is unreadable: %w", err)
	}
	if !head.Name().IsBranch() {
		return "", fmt.Errorf("no ref given and HEAD is detached at %s, expected it to point at a branch", head.Hash())
	}

	return head.Name().Short(), nil
}

// resetWorktree makes the working tree match hash exactly, discarding local
// changes. The fleet cache is a derived artifact nobody edits, so a hard reset
// is the cheapest way to converge; it also moves the local branch, which keeps
// HEAD attached from one poll to the next.
func resetWorktree(repo *git.Repository, hash plumbing.Hash) error {
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if err := wt.Reset(&git.ResetOptions{Mode: git.HardReset, Commit: hash}); err != nil {
		return fmt.Errorf("failed to reset worktree to %s: %w", hash, err)
	}

	return nil
}
