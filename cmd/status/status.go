// Package status implements the "status" CLI command.
package status

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gh-commandeer/cmd/root"
	"github.com/StevenACoffman/gh-commandeer/pkg/cmdutil"
	"github.com/StevenACoffman/gh-commandeer/pkg/github"
	"github.com/StevenACoffman/gh-commandeer/pkg/gitops"
)

// Config holds the configuration for the status command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the status command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	name := parent.Command.Name
	cfg.Flags = ff.NewFlagSet("status").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "status",
		Usage:     name + " status [FLAGS] [<pr-number>]",
		ShortHelp: "show PR details without making any changes",
		LongHelp: `Print the title, contributor, branch, and maintainer-edit permission for a PR.

<pr-number> can be omitted if the branch was checked out with ` + name + `.`,
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

	// CurrentBranch is only needed to look up a stored PR number when no
	// explicit <pr-number> is given. Skip it (and its detached-HEAD error)
	// when the caller has already supplied the argument.
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

	maintainerEdits := "allowed"
	if !pr.AllowMaintainerEdits {
		maintainerEdits = "not allowed (contributor must enable before you can push)"
	}

	fmt.Fprintf(cfg.Stdout, "PR #%d: %s\n", pr.Number, pr.Title)
	fmt.Fprintf(cfg.Stdout, "  contributor:       %s\n", pr.ContributorLogin)
	fmt.Fprintf(cfg.Stdout, "  branch:            %s → %s\n", pr.HeadBranch, pr.BaseBranch)
	fmt.Fprintf(cfg.Stdout, "  maintainer edits:  %s\n", maintainerEdits)
	fmt.Fprintf(cfg.Stdout, "  fork:              %s\n", pr.ForkURL(originURL))
	fmt.Fprintf(
		cfg.Stdout,
		"  url:               https://github.com/%s/%s/pull/%d\n",
		owner,
		repoName,
		pr.Number,
	)
	return nil
}
