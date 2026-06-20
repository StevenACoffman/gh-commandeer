package gitops

import (
	"reflect"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
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

func addRemote(t *testing.T, repo *git.Repository, name string) {
	t.Helper()
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: name,
		URLs: []string{"git@example.com:" + name + "/repo.git"},
	}); err != nil {
		t.Fatalf("create remote %q: %v", name, err)
	}
}

func setBranch(t *testing.T, repo *git.Repository, name, remote string) {
	t.Helper()
	cfg, err := repo.Config()
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	cfg.Branches[name] = &config.Branch{
		Name:   name,
		Remote: remote,
		Merge:  plumbing.NewBranchReferenceName(name),
	}
	if err := repo.SetConfig(cfg); err != nil {
		t.Fatalf("set config: %v", err)
	}
}

func TestRemotesExcept(t *testing.T) {
	repo := newMemRepo(t)
	for _, name := range []string{"origin", "upstream", "mine", "alice", "bob"} {
		addRemote(t, repo, name)
	}

	cases := map[string]struct {
		keep      []string
		wantNames []string
	}{
		"default keep": {[]string{"origin", "upstream", "mine"}, []string{"alice", "bob"}},
		"keep origin":  {[]string{"origin"}, []string{"alice", "bob", "mine", "upstream"}},
		"empty keep":   {[]string{}, []string{"alice", "bob", "mine", "origin", "upstream"}},
		"keep unknown": {[]string{"nope"}, []string{"alice", "bob", "mine", "origin", "upstream"}},
		"keep all":     {[]string{"origin", "upstream", "mine", "alice", "bob"}, []string{}},
		"duplicate ok": {
			[]string{"origin", "origin"},
			[]string{"alice", "bob", "mine", "upstream"},
		},
		"case-sensitive": {
			[]string{"Origin"},
			[]string{"alice", "bob", "mine", "origin", "upstream"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := RemotesExcept(repo, tc.keep)
			if err != nil {
				t.Fatalf("RemotesExcept: %v", err)
			}
			names := make([]string, len(got))
			for i, r := range got {
				names[i] = r.Name
			}
			// Normalize empty slice vs nil for the assertion.
			if len(names) == 0 {
				names = []string{}
			}
			want := tc.wantNames
			if len(want) == 0 {
				want = []string{}
			}
			if !reflect.DeepEqual(names, want) {
				t.Errorf("got %v, want %v", names, want)
			}
		})
	}
}

func TestOrphanedBranches(t *testing.T) {
	repo := newMemRepo(t)
	for _, name := range []string{"keep1", "gone1", "gone2"} {
		addRemote(t, repo, name)
	}
	// "tracksGone" tracks a remote we will declare removed.
	setBranch(t, repo, "tracksGone", "gone1")
	// "tracksKeep" tracks a remote not in the removed set.
	setBranch(t, repo, "tracksKeep", "keep1")
	// "tracksLocal" tracks a local branch (remote=".") — never orphaned.
	setBranch(t, repo, "tracksLocal", ".")
	// "current" is the HEAD branch — excluded even if its remote was removed.
	setBranch(t, repo, "current", "gone2")
	// "noRemote" has an empty remote — excluded.
	setBranch(t, repo, "noRemote", "")

	got, err := OrphanedBranches(repo, []string{"gone1", "gone2"}, "current")
	if err != nil {
		t.Fatalf("OrphanedBranches: %v", err)
	}
	if len(got) != 1 || got[0].Name != "tracksGone" || got[0].Remote != "gone1" {
		t.Errorf("OrphanedBranches = %+v, want [{tracksGone gone1}]", got)
	}
}

func TestDeleteBranchAndRef(t *testing.T) {
	repo := newMemRepo(t)
	// Create a branch ref and its config so both teardown paths fire.
	hash := plumbing.NewHash("0123456789abcdef0123456789abcdef01234567")
	refName := plumbing.NewBranchReferenceName("alice/feature")
	if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, hash)); err != nil {
		t.Fatalf("set reference: %v", err)
	}
	cfg, err := repo.Config()
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	cfg.Branches["alice/feature"] = &config.Branch{
		Name:   "alice/feature",
		Remote: "alice",
		Merge:  plumbing.NewBranchReferenceName("feature"),
	}
	if err := repo.SetConfig(cfg); err != nil {
		t.Fatalf("set config: %v", err)
	}

	if err := DeleteBranchAndRef(repo, "alice/feature"); err != nil {
		t.Fatalf("DeleteBranchAndRef: %v", err)
	}
	if _, err := repo.Reference(refName, false); err == nil {
		t.Error("expected ref to be removed")
	}
	cfg, err = repo.Config()
	if err != nil {
		t.Fatalf("get config after delete: %v", err)
	}
	if _, ok := cfg.Branches["alice/feature"]; ok {
		t.Error("expected branch config to be removed")
	}

	// Idempotent: second call should succeed with no error.
	if err := DeleteBranchAndRef(repo, "alice/feature"); err != nil {
		t.Errorf("second DeleteBranchAndRef: %v", err)
	}
}
