// Package push implements the "push" CLI command.
package push

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gh-commandeer/cmd/root"
	"github.com/StevenACoffman/gh-commandeer/pkg/cmdutil"
	"github.com/StevenACoffman/gh-commandeer/pkg/github"
	"github.com/StevenACoffman/gh-commandeer/pkg/gitops"
)

// Config holds the configuration for the push command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command

	force bool
}

// New creates and registers the push command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("push").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.force, 0, "force", "force-push (required after git rebase)")
	name := parent.Command.Name
	cfg.Command = &ff.Command{
		Name:      "push",
		Usage:     name + " push [FLAGS] [<pr-number>]",
		ShortHelp: "push changes back to a contributor's PR branch",
		LongHelp: `Push sends the current branch to the contributor's fork branch,
updating their pull request with your changes.

Use --force if you rebased the branch before pushing.

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

	currentBranch, err := gitops.CurrentBranch(gitRepo)
	if err != nil {
		return err
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

	expectedBranch := pr.ContributorLogin + "/" + pr.HeadBranch
	if currentBranch != expectedBranch {
		fmt.Fprintf(cfg.Stderr, "hint: to push anyway: git push %s %s:%s\n",
			pr.ContributorLogin, currentBranch, pr.HeadBranch)
		return fmt.Errorf("current branch %q does not match PR #%d (expected %q)",
			currentBranch, prNum, expectedBranch)
	}

	if err := gitops.PushToPR(
		ctx,
		gitRepo,
		pr.ContributorLogin,
		currentBranch,
		pr.HeadBranch,
		cfg.force,
		gitops.TokenAuth(token),
	); err != nil {
		return err
	}

	fmt.Fprintf(
		cfg.Stdout,
		"pushed %q to %s/%s\n",
		currentBranch,
		pr.ContributorLogin,
		pr.HeadBranch,
	)
	return nil
}
