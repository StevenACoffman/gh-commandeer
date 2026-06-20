// Package cmdutil provides shared helpers for gh-commandeer commands.
package cmdutil

import (
	"bufio"
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"

	"github.com/StevenACoffman/gh-commandeer/pkg/github"
	"github.com/StevenACoffman/gh-commandeer/pkg/gitops"
)

// DefaultKeepRemotes is the set of remote names preserved by `gh-commandeer
// clean` when --keep is omitted or empty. Exported so tests and the clean
// command can reference the same source of truth.
const DefaultKeepRemotes = "origin,upstream,mine"

// ResolveToken returns the explicit token if non-empty, then falls back to
// GH_TOKEN (set by gh when invoking extensions) and then GITHUB_TOKEN.
// Returns an error if none of the sources yield a token.
func ResolveToken(token string) (string, error) {
	t := cmp.Or(token, os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN"))
	if t == "" {
		return "", errors.New("GitHub token required: set --token, GH_TOKEN, or GITHUB_TOKEN")
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

// ParseKeepList splits a comma-separated list of remote names into a normalized
// slice. Whitespace around each entry is trimmed and empty entries are dropped.
// When raw is empty or contains only whitespace, DefaultKeepRemotes is used
// instead — callers therefore do not need to apply the default themselves.
func ParseKeepList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		raw = DefaultKeepRemotes
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Confirm prints "question [y/N]: " to out, reads a single line from in, and
// returns true only when the user types "y" or "yes" (case-insensitive). EOF,
// a blank line, or any other answer returns false; the default is therefore No,
// matching git's own destructive-prompt convention. A read error other than EOF
// is returned to the caller.
func Confirm(in io.Reader, out io.Writer, question string) (bool, error) {
	if _, err := fmt.Fprintf(out, "%s [y/N]: ", question); err != nil {
		return false, fmt.Errorf("write confirmation prompt: %w", err)
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
