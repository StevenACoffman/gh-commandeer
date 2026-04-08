package gitops

import (
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
)

func newMemRepo(t *testing.T) *git.Repository {
	t.Helper()
	repo, err := git.Init(memory.NewStorage(), nil)
	if err != nil {
		t.Fatalf("init in-memory repo: %v", err)
	}
	return repo
}

func newFSRepo(t *testing.T) *git.Repository {
	t.Helper()
	repo, err := git.PlainInit(t.TempDir(), false)
	if err != nil {
		t.Fatalf("init filesystem repo: %v", err)
	}
	return repo
}

func TestAddRemoteIfNotExists(t *testing.T) {
	const (
		name = "alice"
		url1 = "https://github.com/alice/repo.git"
		url2 = "https://github.com/alice/other.git"
	)

	t.Run("adds new remote", func(t *testing.T) {
		repo := newMemRepo(t)
		added, err := AddRemoteIfNotExists(repo, name, url1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !added {
			t.Error("want added=true for new remote, got false")
		}
		remote, err := repo.Remote(name)
		if err != nil {
			t.Fatalf("remote not found after add: %v", err)
		}
		if got := remote.Config().URLs[0]; got != url1 {
			t.Errorf("remote URL = %q, want %q", got, url1)
		}
	})

	t.Run("no-op when same URL exists", func(t *testing.T) {
		repo := newMemRepo(t)
		if _, err := AddRemoteIfNotExists(repo, name, url1); err != nil {
			t.Fatalf("first add: %v", err)
		}
		added, err := AddRemoteIfNotExists(repo, name, url1)
		if err != nil {
			t.Fatalf("unexpected error on repeat: %v", err)
		}
		if added {
			t.Error("want added=false for existing remote with same URL, got true")
		}
	})

	t.Run("error when different URL exists", func(t *testing.T) {
		repo := newMemRepo(t)
		if _, err := AddRemoteIfNotExists(repo, name, url1); err != nil {
			t.Fatalf("first add: %v", err)
		}
		_, err := AddRemoteIfNotExists(repo, name, url2)
		if err == nil {
			t.Fatal("want error for conflicting URL, got nil")
		}
	})
}

func TestStorePRNumber(t *testing.T) {
	runTest := func(t *testing.T, repo *git.Repository) {
		t.Helper()
		const branch = "alice/feature"

		// Before storing: should return not found.
		n, ok, err := LoadPRNumber(repo, branch)
		if err != nil {
			t.Fatalf("LoadPRNumber before store: %v", err)
		}
		if ok {
			t.Fatalf("expected not found before store, got %d", n)
		}

		if err := StorePRNumber(repo, branch, 42); err != nil {
			t.Fatalf("StorePRNumber: %v", err)
		}

		n, ok, err = LoadPRNumber(repo, branch)
		if err != nil {
			t.Fatalf("LoadPRNumber after store: %v", err)
		}
		if !ok {
			t.Fatal("expected found after store, got not found")
		}
		if n != 42 {
			t.Errorf("LoadPRNumber = %d, want 42", n)
		}

		// A different branch should still return not found.
		n, ok, err = LoadPRNumber(repo, "bob/other")
		if err != nil {
			t.Fatalf("LoadPRNumber different branch: %v", err)
		}
		if ok {
			t.Errorf("expected not found for different branch, got %d", n)
		}

		// After clearing: should return not found again.
		if err := ClearPRNumber(repo, branch); err != nil {
			t.Fatalf("ClearPRNumber: %v", err)
		}
		_, ok, err = LoadPRNumber(repo, branch)
		if err != nil {
			t.Fatalf("LoadPRNumber after clear: %v", err)
		}
		if ok {
			t.Error("expected not found after clear, got found")
		}
	}

	t.Run("in-memory storage", func(t *testing.T) {
		runTest(t, newMemRepo(t))
	})

	t.Run("filesystem storage", func(t *testing.T) {
		runTest(t, newFSRepo(t))
	})
}
