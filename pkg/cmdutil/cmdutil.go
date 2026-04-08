// Package cmdutil provides shared helpers for gh-commandeer commands.
package cmdutil

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/go-git/go-git/v5"

	"github.com/StevenACoffman/gh-commandeer/pkg/github"
	"github.com/StevenACoffman/gh-commandeer/pkg/gitops"
)

// ResolveToken returns the explicit token if non-empty, then falls back to
// the GITHUB_TOKEN environment variable. Returns an error if neither is set.
func ResolveToken(token string) (string, error) {
	t := cmp.Or(token, os.Getenv("GITHUB_TOKEN"))
	if t == "" {
		return "", errors.New("GitHub token required: set --token or GITHUB_TOKEN")
	}
	return t, nil
}

// ResolveOwnerRepo returns owner and repo using explicit values when both are
// provided, or by parsing originURL. It is an error to supply only one of
// --owner or --repo.
func ResolveOwnerRepo(originURL, owner, repoName string) (string, string, error) {
	if (owner == "") != (repoName == "") {
		return "", "", errors.New("--owner and --repo must be used together")
	}
	if owner != "" {
		return owner, repoName, nil
	}
	o, r, err := github.ParseOwnerRepo(originURL)
	if err != nil {
		return "", "", fmt.Errorf("detect owner/repo from origin: %w", err)
	}
	return o, r, nil
}

// ParsePRNumber parses a PR number from the first positional argument.
// Returns an error if args is empty, the value is not an integer, or it is not positive.
func ParsePRNumber(args []string) (int, error) {
	if len(args) == 0 {
		return 0, errors.New("missing required argument: <pr-number>")
	}
	prNum, err := strconv.Atoi(args[0])
	if err != nil {
		return 0, fmt.Errorf("invalid PR number %q: %w", args[0], err)
	}
	if prNum <= 0 {
		return 0, fmt.Errorf("PR number must be positive, got %d", prNum)
	}
	return prNum, nil
}

// ResolvePRNumber returns the PR number from args[0] if provided, or looks it
// up from the stored branch config for branchName. This allows subcommands like
// "push" and "status" to work without a PR number argument when the branch was
// checked out with gh-commandeer.
func ResolvePRNumber(args []string, repo *git.Repository, branchName string) (int, error) {
	if len(args) > 0 {
		return ParsePRNumber(args)
	}
	prNum, ok, err := gitops.LoadPRNumber(repo, branchName)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf(
			"missing <pr-number> argument (and none stored for branch %q — was it checked out with gh-commandeer?)",
			branchName,
		)
	}
	return prNum, nil
}
