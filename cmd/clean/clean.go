// Package clean implements the "clean" CLI command.
package clean

import (
	"context"
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gh-commandeer/cmd/root"
	"github.com/StevenACoffman/gh-commandeer/pkg/cmdutil"
	"github.com/StevenACoffman/gh-commandeer/pkg/gitops"
)

// Config holds the configuration for the clean command. Keep and Yes are bound
// in New and read by exec; they are not mutated elsewhere.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command

	Keep string
	Yes  bool
}

// New creates and registers the clean command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	name := parent.Command.Name
	cfg.Flags = ff.NewFlagSet("clean").SetParent(parent.Flags)
	cfg.Flags.StringVar(
		&cfg.Keep,
		0,
		"keep",
		"",
		"comma-separated remote names to preserve (default \""+cmdutil.DefaultKeepRemotes+"\")",
	)
	cfg.Flags.BoolVar(
		&cfg.Yes,
		'y',
		"yes",
		"skip confirmation prompts (required when stdin is not a terminal)",
	)
	cfg.Command = &ff.Command{
		Name:      "clean",
		Usage:     name + " clean [FLAGS]",
		ShortHelp: "remove fork remotes and orphaned branches left behind by previous PRs",
		LongHelp: `Clean removes the per-contributor fork remotes that ` + name + ` adds when
checking out a PR. Any remote whose name is not in --keep (default "` +
			cmdutil.DefaultKeepRemotes + `")
is offered for deletion; after the deletion, any local branch whose upstream
remote was just removed is offered for deletion as well.

The current HEAD branch and branches tracking a local (non-remote) upstream are
never deleted. Pass --yes to skip the confirmation prompts; this is required
when stdin is not a terminal.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("clean: unexpected argument %q", args[0])
	}

	repo, err := gitops.OpenRepo(".")
	if err != nil {
		return err
	}
	currentBranch, _ := gitops.CurrentBranch(repo) // detached HEAD is fine: "" excludes nothing.
	keep := cmdutil.ParseKeepList(cfg.Keep)

	// Phase 1: list and (with confirmation) remove non-kept remotes.
	candidates, err := gitops.RemotesExcept(repo, keep)
	if err != nil {
		return err
	}
	var deletedRemotes []string
	if len(candidates) == 0 {
		fmt.Fprintln(cfg.Stdout, "no fork remotes to clean")
	} else {
		fmt.Fprintf(cfg.Stdout, "The following remotes are not in --keep (%v):\n", keep)
		tw := tabwriter.NewWriter(cfg.Stdout, 0, 0, 2, ' ', 0)
		for _, r := range candidates {
			fmt.Fprintf(tw, "  %s\t%s\n", r.Name, r.URL)
		}
		if err := tw.Flush(); err != nil {
			return fmt.Errorf("write remote list: %w", err)
		}

		confirmed, err := cfg.confirm(fmt.Sprintf("Remove all %d remotes?", len(candidates)))
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(cfg.Stdout, "no changes made")
			return nil
		}

		deletedRemotes = make([]string, 0, len(candidates))
		var errs []error
		for _, r := range candidates {
			if _, derr := gitops.DeleteRemote(repo, r.Name); derr != nil {
				errs = append(errs, derr)
				fmt.Fprintf(
					cfg.Stderr,
					"warning: could not remove remote %q: %v\n",
					r.Name,
					derr,
				)
				continue
			}
			deletedRemotes = append(deletedRemotes, r.Name)
			fmt.Fprintf(cfg.Stdout, "removed remote %q\n", r.Name)
		}
		fmt.Fprintf(cfg.Stdout, "removed %d remotes\n", len(deletedRemotes))
		if err := errors.Join(errs...); err != nil {
			return err
		}
	}

	// Phase 2: orphaned branches whose tracked remote was just removed.
	if len(deletedRemotes) == 0 {
		return nil
	}
	orphans, err := gitops.OrphanedBranches(repo, deletedRemotes, currentBranch)
	if err != nil {
		return err
	}
	if len(orphans) == 0 {
		return nil
	}

	fmt.Fprintln(cfg.Stdout, "The following local branches now have no upstream remote:")
	tw := tabwriter.NewWriter(cfg.Stdout, 0, 0, 2, ' ', 0)
	for _, b := range orphans {
		fmt.Fprintf(tw, "  %s\t(was tracking %s)\n", b.Name, b.Remote)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("write orphan list: %w", err)
	}

	confirmed, err := cfg.confirm(fmt.Sprintf("Remove all %d orphaned branches?", len(orphans)))
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(cfg.Stdout, "kept orphaned branches in place")
		return nil
	}

	var errs []error
	deleted := 0
	for _, b := range orphans {
		if derr := gitops.DeleteBranchAndRef(repo, b.Name); derr != nil {
			errs = append(errs, derr)
			fmt.Fprintf(cfg.Stderr, "warning: could not remove branch %q: %v\n", b.Name, derr)
			continue
		}
		deleted++
		fmt.Fprintf(cfg.Stdout, "removed branch %q\n", b.Name)
	}
	fmt.Fprintf(cfg.Stdout, "removed %d branches\n", deleted)
	return errors.Join(errs...)
}

// confirm honours --yes, refuses to silently skip a prompt when stdin is not a
// terminal, and otherwise delegates to cmdutil.Confirm. Prompts go to Stderr so
// they do not interleave with the data written to Stdout.
func (cfg *Config) confirm(question string) (bool, error) {
	if cfg.Yes {
		return true, nil
	}
	if !cfg.StdinIsTTY {
		return false, errors.New(
			"clean: stdin is not a terminal; pass --yes to confirm non-interactively",
		)
	}
	return cmdutil.Confirm(cfg.Stdin, cfg.Stderr, question)
}
