// Package gitops provides go-git helpers for gh-commandeer.
package gitops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// OpenRepo opens the git repository rooted at dir or any parent directory.
func OpenRepo(dir string) (*git.Repository, error) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("open git repo at %q: %w", dir, err)
	}
	return repo, nil
}

// OriginURL returns the first fetch URL of the "origin" remote.
func OriginURL(repo *git.Repository) (string, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", fmt.Errorf("get origin remote: %w", err)
	}
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return "", errors.New("origin remote has no URLs")
	}
	return urls[0], nil
}

// AddRemoteIfNotExists adds a remote with the given name and URL.
// Returns (true, nil) when the remote is newly created, (false, nil) when it
// already exists with the same URL, or (false, error) on any problem including
// a conflicting URL.
func AddRemoteIfNotExists(repo *git.Repository, name, url string) (bool, error) {
	existing, err := repo.Remote(name)
	if err == nil {
		urls := existing.Config().URLs
		if len(urls) > 0 {
			if urls[0] == url {
				return false, nil
			}
			return false, fmt.Errorf(
				"remote %q already exists with URL %q (want %q)",
				name,
				urls[0],
				url,
			)
		}
		return false, fmt.Errorf("remote %q already exists with no URLs (want %q)", name, url)
	}
	if !errors.Is(err, git.ErrRemoteNotFound) {
		return false, fmt.Errorf("check remote %q: %w", name, err)
	}
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: name,
		URLs: []string{url},
	})
	if err != nil {
		return false, fmt.Errorf("create remote %q: %w", name, err)
	}
	return true, nil
}

// DeleteRemote removes the named remote and reports whether it existed.
// Returns (false, nil) if the remote did not exist.
func DeleteRemote(repo *git.Repository, name string) (bool, error) {
	err := repo.DeleteRemote(name)
	if errors.Is(err, git.ErrRemoteNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("delete remote %q: %w", name, err)
	}
	return true, nil
}

// FetchRemote fetches all refs from the named remote.
// Progress output (e.g. pack counts) is written to progress; pass nil to suppress it.
// git.NoErrAlreadyUpToDate is treated as success.
func FetchRemote(
	ctx context.Context,
	repo *git.Repository,
	remoteName string,
	auth *githttp.BasicAuth,
	progress io.Writer,
) error {
	err := repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: remoteName,
		Auth:       auth,
		Progress:   progress,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("fetch remote %q: %w", remoteName, err)
	}
	return nil
}

// CheckoutPRBranch ensures localBranch is checked out, creating it from
// remoteName/remoteBranch if it does not yet exist locally.
// Returns true if a new branch was created, false if an existing branch was reused.
// The branch upstream is always set so that git push/pull/status work without extra arguments.
func CheckoutPRBranch(
	repo *git.Repository,
	localBranch, remoteName, remoteBranch string,
) (bool, error) {
	w, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("get worktree: %w", err)
	}

	branchRef := plumbing.NewBranchReferenceName(localBranch)
	_, refErr := repo.Reference(branchRef, true)
	created := errors.Is(refErr, plumbing.ErrReferenceNotFound)

	switch {
	case created:
		remoteRef := plumbing.NewRemoteReferenceName(remoteName, remoteBranch)
		ref, err := repo.Reference(remoteRef, true)
		if err != nil {
			return false, fmt.Errorf("resolve %s/%s: %w", remoteName, remoteBranch, err)
		}
		if err := w.Checkout(&git.CheckoutOptions{
			Branch: branchRef,
			Create: true,
			Hash:   ref.Hash(),
		}); err != nil {
			return false, fmt.Errorf("checkout branch %q: %w", localBranch, err)
		}
	case refErr != nil:
		return false, fmt.Errorf("check branch %q: %w", localBranch, refErr)
	default:
		if err := w.Checkout(&git.CheckoutOptions{Branch: branchRef}); err != nil {
			return false, fmt.Errorf("switch to branch %q: %w", localBranch, err)
		}
	}

	// Set or refresh the upstream tracking config.
	// Equivalent to: git branch --set-upstream-to=remoteName/remoteBranch localBranch
	repoCfg, err := repo.Config()
	if err != nil {
		return created, fmt.Errorf("get config to set upstream for %q: %w", localBranch, err)
	}
	repoCfg.Branches[localBranch] = &config.Branch{
		Name:   localBranch,
		Remote: remoteName,
		Merge:  plumbing.NewBranchReferenceName(remoteBranch),
	}
	if err := repo.SetConfig(repoCfg); err != nil {
		return created, fmt.Errorf("set upstream for %q: %w", localBranch, err)
	}
	return created, nil
}

// PushToPR pushes localBranch to remoteBranch on remoteName.
// Equivalent to: git push [--force] remoteName localBranch:remoteBranch
func PushToPR(
	ctx context.Context,
	repo *git.Repository,
	remoteName, localBranch, remoteBranch string,
	force bool,
	auth *githttp.BasicAuth,
) error {
	refspec := config.RefSpec(
		"refs/heads/" + localBranch + ":refs/heads/" + remoteBranch,
	)
	err := repo.PushContext(ctx, &git.PushOptions{
		RemoteName: remoteName,
		RefSpecs:   []config.RefSpec{refspec},
		Force:      force,
		Auth:       auth,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("push %s → %s/%s: %w", localBranch, remoteName, remoteBranch, err)
	}
	return nil
}

// CurrentBranch returns the short name of the current HEAD branch.
func CurrentBranch(repo *git.Repository) (string, error) {
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("get HEAD: %w", err)
	}
	if !head.Name().IsBranch() {
		return "", errors.New("HEAD is not on a branch (detached HEAD state)")
	}
	return head.Name().Short(), nil
}

// TokenAuth returns a go-git BasicAuth value for a GitHub personal access token.
func TokenAuth(token string) *githttp.BasicAuth {
	return &githttp.BasicAuth{
		Username: "x-oauth-basic",
		Password: token,
	}
}

// StorePRNumber writes prNum into .git/config under
// [gh-commandeer "branchName"] pr = N so that it can be recalled later.
func StorePRNumber(repo *git.Repository, branchName string, prNum int) error {
	repoCfg, err := repo.Config()
	if err != nil {
		return fmt.Errorf("get config: %w", err)
	}
	repoCfg.Raw.Section("gh-commandeer").Subsection(branchName).SetOption("pr", strconv.Itoa(prNum))
	if err := repo.SetConfig(repoCfg); err != nil {
		return fmt.Errorf("store PR number for %q: %w", branchName, err)
	}
	return nil
}

// LoadPRNumber reads the PR number stored for branchName by StorePRNumber.
// Returns (0, false, nil) if no number has been stored for the branch.
func LoadPRNumber(repo *git.Repository, branchName string) (int, bool, error) {
	repoCfg, err := repo.Config()
	if err != nil {
		return 0, false, fmt.Errorf("get config: %w", err)
	}
	val := repoCfg.Raw.Section("gh-commandeer").Subsection(branchName).Option("pr")
	if val == "" {
		return 0, false, nil
	}
	prNum, err := strconv.Atoi(val)
	if err != nil {
		return 0, false, fmt.Errorf(
			"invalid stored PR number %q for branch %q: %w",
			val,
			branchName,
			err,
		)
	}
	return prNum, true, nil
}

// ClearPRNumber removes the stored PR number for branchName.
func ClearPRNumber(repo *git.Repository, branchName string) error {
	repoCfg, err := repo.Config()
	if err != nil {
		return fmt.Errorf("get config: %w", err)
	}
	repoCfg.Raw.Section("gh-commandeer").Subsection(branchName).RemoveOption("pr")
	if err := repo.SetConfig(repoCfg); err != nil {
		return fmt.Errorf("clear PR number for %q: %w", branchName, err)
	}
	return nil
}
