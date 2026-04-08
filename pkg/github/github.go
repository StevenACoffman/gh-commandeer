// Package github provides GitHub API helpers for gh-commandeer.
package github

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	ghapi "github.com/google/go-github/v84/github"
)

// PRInfo holds the metadata needed to commandeer a pull request.
type PRInfo struct {
	Number               int
	Title                string
	ContributorLogin     string
	ForkCloneURL         string // HTTPS clone URL for the contributor's fork
	ForkSSHURL           string // SSH clone URL for the contributor's fork
	HeadBranch           string // branch name on the fork (PR head)
	BaseBranch           string // base branch of the target repo (e.g. main)
	AllowMaintainerEdits bool
}

// ForkURL returns the clone URL for the fork that matches the protocol of originURL.
// If originURL is an SSH remote, the SSH clone URL is returned; otherwise HTTPS.
func (pr PRInfo) ForkURL(originURL string) string {
	if strings.HasPrefix(originURL, "git@") || strings.Contains(originURL, "github.com:") {
		return pr.ForkSSHURL
	}
	return pr.ForkCloneURL
}

// NewClient returns an authenticated GitHub API client.
func NewClient(token string) *ghapi.Client {
	return ghapi.NewClient(nil).WithAuthToken(token)
}

// GetPRInfo fetches PR metadata from the GitHub API.
func GetPRInfo(
	ctx context.Context,
	client *ghapi.Client,
	owner, repo string,
	prNum int,
) (PRInfo, error) {
	pr, _, err := client.PullRequests.Get(ctx, owner, repo, prNum)
	if err != nil {
		return PRInfo{}, fmt.Errorf("get PR %s/%s#%d: %w", owner, repo, prNum, err)
	}
	if pr.Head == nil || pr.Head.Repo == nil {
		return PRInfo{}, fmt.Errorf(
			"PR %s/%s#%d: head repo is nil (fork may have been deleted)",
			owner,
			repo,
			prNum,
		)
	}
	return PRInfo{
		Number:               prNum,
		Title:                pr.GetTitle(),
		ContributorLogin:     pr.Head.GetUser().GetLogin(),
		ForkCloneURL:         pr.Head.Repo.GetCloneURL(),
		ForkSSHURL:           pr.Head.Repo.GetSSHURL(),
		HeadBranch:           pr.Head.GetRef(),
		BaseBranch:           pr.Base.GetRef(),
		AllowMaintainerEdits: pr.GetMaintainerCanModify(),
	}, nil
}

// ParseOwnerRepo extracts owner and repo from an HTTPS or SSH GitHub remote URL.
//
//	https://github.com/owner/repo.git  →  owner, repo
//	git@github.com:owner/repo.git      →  owner, repo
func ParseOwnerRepo(remoteURL string) (owner, repo string, err error) {
	u := strings.TrimSuffix(remoteURL, ".git")

	if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
		parsed, parseErr := url.Parse(u)
		if parseErr == nil && parsed.Hostname() == "github.com" {
			parts := strings.SplitN(strings.TrimPrefix(parsed.Path, "/"), "/", 2)
			if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				return parts[0], parts[1], nil
			}
		}
		return "", "", fmt.Errorf("cannot parse owner/repo from %q", remoteURL)
	}

	if strings.Contains(u, "github.com:") {
		_, path, _ := strings.Cut(u, "github.com:")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], nil
		}
		return "", "", fmt.Errorf("cannot parse owner/repo from %q", remoteURL)
	}

	return "", "", fmt.Errorf("unsupported remote URL (expected github.com): %q", remoteURL)
}
