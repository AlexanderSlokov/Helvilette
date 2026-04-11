package git

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// EnsureRepo ensures that the git repository is cloned and at the correct version.
func EnsureRepo(url, destDir, version string) error {
	_, err := os.Stat(destDir)
	var repo *git.Repository

	// If directory does not exist, clone it
	if os.IsNotExist(err) {
		repo, err = git.PlainClone(destDir, false, &git.CloneOptions{
			URL: url,
		})
		if err != nil {
			return fmt.Errorf("failed to clone repo: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to stat destination dir: %w", err)
	} else {
		// Directory exists, open as repo
		repo, err = git.PlainOpen(destDir)
		if err != nil {
			return fmt.Errorf("failed to open existing repo at %s: %w", destDir, err)
		}

		err = repo.Fetch(&git.FetchOptions{
			Force: true,
			Tags:  git.AllTags,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return fmt.Errorf("failed to fetch repo: %w", err)
		}
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if version != "" {
		hash, err := repo.ResolveRevision(plumbing.Revision(version))
		if err != nil {
			return fmt.Errorf("failed to resolve version %s: %w", version, err)
		}

		err = wt.Checkout(&git.CheckoutOptions{
			Hash:  *hash,
			Force: true,
		})
		if err != nil {
			return fmt.Errorf("failed to checkout version %s: %w", version, err)
		}
	} else {
		// Default to pulling the remote changes for tracking branch
		err = wt.Pull(&git.PullOptions{
			RemoteName: "origin",
			Force:      true,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return fmt.Errorf("failed to pull latest changes: %w", err)
		}
	}

	return nil
}
