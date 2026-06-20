package clean_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/StevenACoffman/gh-commandeer/cmd"
)

// makeRepo builds a temp-dir repo populated with origin + two fork remotes and
// a local branch tracking one of them. Returns the repo directory and the
// initialized *git.Repository so individual tests can inspect post-state.
func makeRepo(t *testing.T) (string, *git.Repository) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	for _, name := range []string{"origin", "alice", "bob"} {
		if _, err := repo.CreateRemote(&config.RemoteConfig{
			Name: name,
			URLs: []string{"git@example.com:" + name + "/repo.git"},
		}); err != nil {
			t.Fatalf("create remote %q: %v", name, err)
		}
	}
	cfg, err := repo.Config()
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	// alice/feature tracks alice — orphaned after we clean.
	cfg.Branches["alice/feature"] = &config.Branch{
		Name:   "alice/feature",
		Remote: "alice",
		Merge:  plumbing.NewBranchReferenceName("feature"),
	}
	// main tracks origin — must survive.
	cfg.Branches["main"] = &config.Branch{
		Name:   "main",
		Remote: "origin",
		Merge:  plumbing.NewBranchReferenceName("main"),
	}
	if err := repo.SetConfig(cfg); err != nil {
		t.Fatalf("set config: %v", err)
	}
	// Materialize the alice/feature ref so DeleteBranchAndRef has something to remove.
	hash := plumbing.NewHash("0123456789abcdef0123456789abcdef01234567")
	refName := plumbing.NewBranchReferenceName("alice/feature")
	if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, hash)); err != nil {
		t.Fatalf("set reference: %v", err)
	}
	return dir, repo
}

// runClean invokes cmd.Run with subcommand "clean" against the repo at dir,
// using the given stdin string and stdinIsTTY value. The stderr buffer is
// discarded since none of the cases assert against it; uncomment the return if
// a future case needs it.
func runClean(
	t *testing.T,
	dir, stdin string,
	stdinIsTTY bool,
	args ...string,
) (stdout string, err error) {
	t.Helper()
	t.Chdir(dir) // t.Chdir restores the original cwd via t.Cleanup automatically.

	var outBuf, errBuf bytes.Buffer
	err = cmd.Run(
		context.Background(),
		append([]string{"clean"}, args...),
		strings.NewReader(stdin),
		stdinIsTTY,
		&outBuf,
		&errBuf,
	)
	return outBuf.String(), err
}

func TestClean_YesRemovesRemotesAndOrphans(t *testing.T) {
	dir, repo := makeRepo(t)

	stdout, err := runClean(t, dir, "", true, "--yes")
	if err != nil {
		t.Fatalf("clean: %v\nstdout:\n%s", err, stdout)
	}
	for _, want := range []string{
		`removed remote "alice"`,
		`removed remote "bob"`,
		`removed branch "alice/feature"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in output:\n%s", want, stdout)
		}
	}

	remotes, err := repo.Remotes()
	if err != nil {
		t.Fatalf("list remotes: %v", err)
	}
	if len(remotes) != 1 || remotes[0].Config().Name != "origin" {
		names := make([]string, len(remotes))
		for i, r := range remotes {
			names[i] = r.Config().Name
		}
		t.Errorf("remotes after clean = %v, want [origin]", names)
	}

	cfg, err := repo.Config()
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if _, ok := cfg.Branches["alice/feature"]; ok {
		t.Error("alice/feature branch config not removed")
	}
	if _, ok := cfg.Branches["main"]; !ok {
		t.Error("main branch config should still exist")
	}
}

func TestClean_DeclineLeavesEverythingAlone(t *testing.T) {
	dir, repo := makeRepo(t)

	stdout, err := runClean(t, dir, "n\n", true)
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if !strings.Contains(stdout, "no changes made") {
		t.Errorf("expected 'no changes made' in output:\n%s", stdout)
	}
	remotes, err := repo.Remotes()
	if err != nil {
		t.Fatalf("list remotes: %v", err)
	}
	if len(remotes) != 3 {
		t.Errorf("remotes after decline = %d, want 3", len(remotes))
	}
}

func TestClean_NonTTYWithoutYesErrors(t *testing.T) {
	dir, _ := makeRepo(t)

	_, err := runClean(t, dir, "", false)
	if err == nil {
		t.Fatal("expected error when stdin is not a TTY and --yes not set")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error should mention --yes, got: %v", err)
	}
}

func TestClean_CustomKeepRemovesUpstream(t *testing.T) {
	dir, repo := makeRepo(t)
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "upstream",
		URLs: []string{"git@example.com:upstream/repo.git"},
	}); err != nil {
		t.Fatalf("create upstream: %v", err)
	}

	stdout, err := runClean(t, dir, "", true, "--yes", "--keep", "origin")
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if !strings.Contains(stdout, `removed remote "upstream"`) {
		t.Errorf("expected upstream removal under --keep=origin:\n%s", stdout)
	}
}

func TestClean_EmptyKeepFallsBackToDefault(t *testing.T) {
	dir, repo := makeRepo(t)
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "upstream",
		URLs: []string{"git@example.com:upstream/repo.git"},
	}); err != nil {
		t.Fatalf("create upstream: %v", err)
	}

	stdout, err := runClean(t, dir, "", true, "--yes", "--keep", "")
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if strings.Contains(stdout, `removed remote "upstream"`) {
		t.Errorf("upstream should be preserved under default keep:\n%s", stdout)
	}
	if strings.Contains(stdout, `removed remote "origin"`) {
		t.Errorf("origin should be preserved under default keep:\n%s", stdout)
	}
}

func TestClean_NoForkRemotes(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"git@example.com:origin/repo.git"},
	}); err != nil {
		t.Fatalf("create origin: %v", err)
	}

	stdout, err := runClean(t, dir, "", true, "--yes")
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if !strings.Contains(stdout, "no fork remotes to clean") {
		t.Errorf("expected 'no fork remotes to clean' in output:\n%s", stdout)
	}
}
