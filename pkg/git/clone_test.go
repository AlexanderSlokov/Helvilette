package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const testBranch = "main"

// sourceRepo is a real on-disk repository standing in for a remote Git server.
// EnsureRepo's whole job is ref resolution across a fetch, which a fake cannot
// exercise: the bug in issue #32 lived in how go-git updates refs/remotes/*
// versus refs/heads/*. So this is a named local fixture, not a mock.
type sourceRepo struct {
	dir  string
	repo *git.Repository
	t    *testing.T
}

func newSourceRepo(t *testing.T) *sourceRepo {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "origin")
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit(%s): %v", dir, err)
	}

	head := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(testBranch))
	if err := repo.Storer.SetReference(head); err != nil {
		t.Fatalf("point HEAD at %s: %v", testBranch, err)
	}

	return &sourceRepo{dir: dir, repo: repo, t: t}
}

// commitFile writes name and commits it, returning the new commit SHA.
func (s *sourceRepo) commitFile(name, content string) string {
	s.t.Helper()

	if err := os.WriteFile(filepath.Join(s.dir, name), []byte(content), 0644); err != nil {
		s.t.Fatalf("write %s: %v", name, err)
	}

	wt, err := s.repo.Worktree()
	if err != nil {
		s.t.Fatalf("worktree: %v", err)
	}
	if _, err := wt.Add(name); err != nil {
		s.t.Fatalf("add %s: %v", name, err)
	}

	hash, err := wt.Commit("add "+name, &git.CommitOptions{
		Author: &object.Signature{Name: "Tester", Email: "test@example.com", When: time.Now()},
	})
	if err != nil {
		s.t.Fatalf("commit %s: %v", name, err)
	}

	return hash.String()
}

// TestEnsureRepo_PicksUpCommitsPushedAfterClone is the regression test for
// issue #32. Before the fix, EnsureRepo resolved the branch through
// ResolveRevision, which matches refs/heads/<branch> — a ref that fetch never
// advances — so Othela re-checked-out its clone-time commit on every poll and
// a manifest pushed after startup was never seen.
func TestEnsureRepo_PicksUpCommitsPushedAfterClone(t *testing.T) {
	source := newSourceRepo(t)
	source.commitFile("first.yml", "one")

	cacheDir := filepath.Join(t.TempDir(), "cache")

	firstSHA, err := EnsureRepo(source.dir, cacheDir, testBranch)
	if err != nil {
		t.Fatalf("first EnsureRepo: %v", err)
	}

	secondCommit := source.commitFile("second.yml", "two")

	secondSHA, err := EnsureRepo(source.dir, cacheDir, testBranch)
	if err != nil {
		t.Fatalf("second EnsureRepo: %v", err)
	}

	if secondSHA == firstSHA {
		t.Fatalf("EnsureRepo returned the clone-time commit %s again; the new commit %s was not picked up", firstSHA, secondCommit)
	}
	if secondSHA != secondCommit {
		t.Errorf("EnsureRepo returned %s, want the branch tip %s", secondSHA, secondCommit)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "second.yml")); err != nil {
		t.Errorf("second.yml missing from the worktree after sync: %v", err)
	}
}

// TestEnsureRepo_DropsFilesRemovedUpstream covers the other half of convergence:
// a manifest deleted upstream must disappear from the cache, or Othela keeps
// dispatching a playbook the operator has retired.
func TestEnsureRepo_DropsFilesRemovedUpstream(t *testing.T) {
	source := newSourceRepo(t)
	source.commitFile("keep.yml", "keep")
	source.commitFile("drop.yml", "drop")

	cacheDir := filepath.Join(t.TempDir(), "cache")
	if _, err := EnsureRepo(source.dir, cacheDir, testBranch); err != nil {
		t.Fatalf("first EnsureRepo: %v", err)
	}

	source.removeFile("drop.yml")

	if _, err := EnsureRepo(source.dir, cacheDir, testBranch); err != nil {
		t.Fatalf("second EnsureRepo: %v", err)
	}

	if _, err := os.Stat(filepath.Join(cacheDir, "drop.yml")); !os.IsNotExist(err) {
		t.Errorf("drop.yml still present after upstream deletion, stat err = %v", err)
	}
}

func (s *sourceRepo) removeFile(name string) {
	s.t.Helper()

	wt, err := s.repo.Worktree()
	if err != nil {
		s.t.Fatalf("worktree: %v", err)
	}
	if _, err := wt.Remove(name); err != nil {
		s.t.Fatalf("remove %s: %v", name, err)
	}
	if _, err := wt.Commit("remove "+name, &git.CommitOptions{
		Author: &object.Signature{Name: "Tester", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		s.t.Fatalf("commit removal of %s: %v", name, err)
	}
}

func TestEnsureRepo_LocalEditsAreDiscarded(t *testing.T) {
	source := newSourceRepo(t)
	source.commitFile("manifest.yml", "upstream")

	cacheDir := filepath.Join(t.TempDir(), "cache")
	if _, err := EnsureRepo(source.dir, cacheDir, testBranch); err != nil {
		t.Fatalf("first EnsureRepo: %v", err)
	}

	tampered := filepath.Join(cacheDir, "manifest.yml")
	if err := os.WriteFile(tampered, []byte("edited by hand"), 0644); err != nil {
		t.Fatalf("tamper with cache: %v", err)
	}

	if _, err := EnsureRepo(source.dir, cacheDir, testBranch); err != nil {
		t.Fatalf("second EnsureRepo: %v", err)
	}

	got, err := os.ReadFile(tampered)
	if err != nil {
		t.Fatalf("read back manifest: %v", err)
	}
	if string(got) != "upstream" {
		t.Errorf("worktree content = %q, want %q — the cache is not an editing surface", got, "upstream")
	}
}

// TestEnsureRepo_ReclonesUnusableCache covers the half-finished clone: before
// the fix, a destDir that existed but would not open failed on every later
// sync forever, with no way back short of an operator deleting it.
func TestEnsureRepo_ReclonesUnusableCache(t *testing.T) {
	source := newSourceRepo(t)
	want := source.commitFile("manifest.yml", "one")

	cacheDir := filepath.Join(t.TempDir(), "cache")
	if err := os.MkdirAll(filepath.Join(cacheDir, "leftover"), 0755); err != nil {
		t.Fatalf("stage unusable cache: %v", err)
	}

	got, err := EnsureRepo(source.dir, cacheDir, testBranch)
	if err != nil {
		t.Fatalf("EnsureRepo over an unusable cache: %v", err)
	}
	if got != want {
		t.Errorf("EnsureRepo = %s, want %s", got, want)
	}
}

func TestEnsureRepo_ResolvesTag(t *testing.T) {
	source := newSourceRepo(t)
	tagged := source.commitFile("manifest.yml", "one")
	source.tag("v1.0.0", tagged)
	source.commitFile("later.yml", "two")

	cacheDir := filepath.Join(t.TempDir(), "cache")

	got, err := EnsureRepo(source.dir, cacheDir, "v1.0.0")
	if err != nil {
		t.Fatalf("EnsureRepo at tag: %v", err)
	}
	if got != tagged {
		t.Errorf("EnsureRepo = %s, want the tagged commit %s", got, tagged)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "later.yml")); !os.IsNotExist(err) {
		t.Errorf("later.yml should not exist at tag v1.0.0, stat err = %v", err)
	}
}

func (s *sourceRepo) tag(name, commit string) {
	s.t.Helper()

	if _, err := s.repo.CreateTag(name, plumbing.NewHash(commit), nil); err != nil {
		s.t.Fatalf("tag %s at %s: %v", name, commit, err)
	}
}

// TestEnsureRepo_UnknownRefNamesWhatWasTried keeps the error actionable: the
// operator must be able to read the offending ref and the refs searched
// straight out of the message.
func TestEnsureRepo_UnknownRefNamesWhatWasTried(t *testing.T) {
	source := newSourceRepo(t)
	source.commitFile("manifest.yml", "one")

	cacheDir := filepath.Join(t.TempDir(), "cache")

	_, err := EnsureRepo(source.dir, cacheDir, "no-such-branch")
	if err == nil {
		t.Fatal("EnsureRepo succeeded on an unknown ref, want an error")
	}

	for _, want := range []string{"no-such-branch", "refs/remotes/origin/no-such-branch", "refs/tags/no-such-branch"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestEnsureRepo_EmptyRefUsesTheClonedBranch(t *testing.T) {
	source := newSourceRepo(t)
	source.commitFile("first.yml", "one")

	cacheDir := filepath.Join(t.TempDir(), "cache")
	if _, err := EnsureRepo(source.dir, cacheDir, ""); err != nil {
		t.Fatalf("first EnsureRepo with empty ref: %v", err)
	}

	want := source.commitFile("second.yml", "two")

	got, err := EnsureRepo(source.dir, cacheDir, "")
	if err != nil {
		t.Fatalf("second EnsureRepo with empty ref: %v", err)
	}
	if got != want {
		t.Errorf("EnsureRepo = %s, want the branch tip %s", got, want)
	}
}
