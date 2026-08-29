package git

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// EnsureRepo ensures that the git repository is cloned locally and updated to the specified version.
func EnsureRepo(url, destDir, version string) error {
	repo, err := openOrCloneRepo(url, destDir)
	if err != nil {
		return err
	}

	return syncWorktree(repo, version)
}

func openOrCloneRepo(url, destDir string) (*git.Repository, error) {
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		return cloneRepo(url, destDir)
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat destination dir %s: %w", destDir, err)
	}

	return openAndFetchRepo(destDir)
}

func cloneRepo(url, destDir string) (*git.Repository, error) {
	repo, err := git.PlainClone(destDir, false, &git.CloneOptions{
		URL: url,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repo from %s: %w", url, err)
	}
	return repo, nil
}

func openAndFetchRepo(destDir string) (*git.Repository, error) {
	repo, err := git.PlainOpen(destDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open existing repo at %s: %w", destDir, err)
	}

	err = repo.Fetch(&git.FetchOptions{
		Force: true,
		Tags:  git.AllTags,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return nil, fmt.Errorf("failed to fetch repo at %s: %w", destDir, err)
	}

	return repo, nil
}

func syncWorktree(repo *git.Repository, version string) error {
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if version != "" {
		return checkoutVersion(wt, repo, version)
	}
	return pullLatest(wt)
}

func checkoutVersion(wt *git.Worktree, repo *git.Repository, version string) error {
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

	return nil
}

func pullLatest(wt *git.Worktree) error {
	err := wt.Pull(&git.PullOptions{
		RemoteName: "origin",
		Force:      true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull latest changes: %w", err)
	}
	return nil
}
