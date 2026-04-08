// Package root defines the root configuration for the CLI.
package root

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gh-commandeer/pkg/cmdutil"
	"github.com/StevenACoffman/gh-commandeer/pkg/github"
	"github.com/StevenACoffman/gh-commandeer/pkg/gitops"
)

// CmdName returns "gh commandeer" when running as a gh extension (GH_EXTENSION
// is set by gh) and "gh-commandeer" otherwise.
func CmdName() string {
	if os.Getenv("GH_EXTENSION") != "" {
		return "gh commandeer"
	}
	return "gh-commandeer"
}

// Config holds shared I/O writers, flags, and the root ff.Command.
// All subcommand configs embed *Config to inherit these.
type Config struct {
	Stdout  io.Writer
	Stderr  io.Writer
	Flags   *ff.FlagSet // shared flags inherited by subcommands: --owner, --repo, --token
	Command *ff.Command

	Owner   string
	Repo    string
	Token   string
	NoFetch bool

	rootFlags *ff.FlagSet // root-only flags: --no-fetch (not inherited by subcommands)
}

// New returns a new root Config with the given I/O writers.
func New(stdout, stderr io.Writer) *Config {
	var cfg Config
	cfg.Stdout = stdout
	cfg.Stderr = stderr

	name := CmdName()

	// Shared flags: inherited by all subcommands via SetParent(parent.Flags).
	cfg.Flags = ff.NewFlagSet(name)
	cfg.Flags.StringVar(
		&cfg.Owner,
		0,
		"owner",
		"",
		"GitHub repo owner (auto-detected from origin remote)",
	)
	cfg.Flags.StringVar(
		&cfg.Repo,
		0,
		"repo",
		"",
		"GitHub repo name (auto-detected from origin remote)",
	)
	cfg.Flags.StringVar(
		&cfg.Token,
		0,
		"token",
		"",
		"GitHub personal access token (default: $GH_TOKEN or $GITHUB_TOKEN)",
	)

	// Root-only flags: visible only in the root command's help output.
	cfg.rootFlags = ff.NewFlagSet(name + "-root").SetParent(cfg.Flags)
	cfg.rootFlags.BoolVar(
		&cfg.NoFetch,
		0,
		"no-fetch",
		"skip fetching the remote (use if refs are already up to date)",
	)

	cfg.Command = &ff.Command{
		Name:      name,
		Usage:     name + " [FLAGS] <pr-number>",
		ShortHelp: "check out a contributor's PR branch and push your changes back",
		LongHelp: `Adds the contributor's fork as a git remote, fetches it,
and checks out their PR branch so you can commit changes to it.

After making changes, optionally run 'git rebase origin/main', then:
  ` + name + ` push`,
		Flags: cfg.rootFlags,
		Exec:  cfg.exec,
	}
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	prNum, err := cmdutil.ParsePRNumber(args)
	if err != nil {
		return err
	}

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

	owner, repoName, err := cmdutil.ResolveOwnerRepo(originURL, cfg.Owner, cfg.Repo)
	if err != nil {
		return err
	}

	pr, err := github.GetPRInfo(ctx, github.NewClient(token), owner, repoName, prNum)
	if err != nil {
		return err
	}

	if !pr.AllowMaintainerEdits {
		fmt.Fprintf(
			cfg.Stderr,
			"warning: PR #%d does not allow maintainer edits — the contributor may need to enable it before you can push\n",
			prNum,
		)
	}

	remoteName := pr.ContributorLogin
	localBranch := pr.ContributorLogin + "/" + pr.HeadBranch
	forkURL := pr.ForkURL(originURL)

	added, err := gitops.AddRemoteIfNotExists(gitRepo, remoteName, forkURL)
	if err != nil {
		return err
	}
	if added {
		fmt.Fprintf(cfg.Stderr, "added remote %q → %s\n", remoteName, forkURL)
	} else {
		fmt.Fprintf(cfg.Stderr, "remote %q already exists\n", remoteName)
	}

	if !cfg.NoFetch {
		if err := gitops.FetchRemote(
			ctx,
			gitRepo,
			remoteName,
			gitops.TokenAuth(token),
			cfg.Stderr,
		); err != nil {
			return err
		}
	}

	created, err := gitops.CheckoutPRBranch(gitRepo, localBranch, remoteName, pr.HeadBranch)
	if err != nil {
		return err
	}

	if err := gitops.StorePRNumber(gitRepo, localBranch, prNum); err != nil {
		fmt.Fprintf(cfg.Stderr, "warning: could not store PR number: %v\n", err)
	}

	if created {
		fmt.Fprintf(
			cfg.Stdout,
			"checked out %q — make your changes, then run:\n  %s push\n",
			localBranch,
			CmdName(),
		)
	} else {
		fmt.Fprintf(
			cfg.Stdout,
			"switched to existing branch %q — make your changes, then run:\n  %s push\n",
			localBranch,
			CmdName(),
		)
	}
	return nil
}
