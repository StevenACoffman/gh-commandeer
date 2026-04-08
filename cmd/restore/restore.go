// Package restore implements the "restore" CLI command.
package restore

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gh-commandeer/cmd/root"
	"github.com/StevenACoffman/gh-commandeer/pkg/cmdutil"
	"github.com/StevenACoffman/gh-commandeer/pkg/github"
	"github.com/StevenACoffman/gh-commandeer/pkg/gitops"
)

// Config holds the configuration for the restore command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the restore command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("restore").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "restore",
		Usage:     "gh-commandeer restore [FLAGS] [<pr-number>]",
		ShortHelp: "remove the contributor's remote and clean up stored PR metadata",
		LongHelp: `Restore undoes what 'gh-commandeer <pr-number>' did:
it removes the contributor's fork remote and clears the stored PR number
from .git/config.

The local branch is left in place — delete it manually with:
  git branch -D <login>/<branch>

<pr-number> can be omitted if the branch was checked out with gh-commandeer.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	token, err := cmdutil.ResolveToken(cfg.Token)
	if err != nil {
		return err
	}

	gitRepo, err := gitops.OpenRepo(".")
	if err != nil {
		return err
	}

	originURL, err := gitops.OriginURL(gitRepo)
	if err != nil {
		return err
	}

	var currentBranch string
	if len(args) == 0 {
		currentBranch, err = gitops.CurrentBranch(gitRepo)
		if err != nil {
			return err
		}
	}

	prNum, err := cmdutil.ResolvePRNumber(args, gitRepo, currentBranch)
	if err != nil {
		return err
	}

	owner, repoName, err := cmdutil.ResolveOwnerRepo(originURL, cfg.Owner, cfg.Repo)
	if err != nil {
		return err
	}

	pr, err := github.GetPRInfo(ctx, github.NewClient(token), owner, repoName, prNum)
	if err != nil {
		return err
	}

	remoteName := pr.ContributorLogin
	localBranch := pr.ContributorLogin + "/" + pr.HeadBranch

	existed, err := gitops.DeleteRemote(gitRepo, remoteName)
	if err != nil {
		return err
	}
	if existed {
		fmt.Fprintf(cfg.Stdout, "removed remote %q\n", remoteName)
	} else {
		fmt.Fprintf(cfg.Stderr, "warning: remote %q not found, skipping\n", remoteName)
	}

	if err := gitops.ClearPRNumber(gitRepo, localBranch); err != nil {
		fmt.Fprintf(cfg.Stderr, "warning: could not clear stored PR number: %v\n", err)
	} else {
		fmt.Fprintf(cfg.Stdout, "cleared stored PR number for branch %q\n", localBranch)
	}

	fmt.Fprintf(cfg.Stdout, "note: local branch %q was not deleted\n", localBranch)
	return nil
}
