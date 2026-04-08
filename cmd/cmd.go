// Package cmd is the dispatcher; it routes CLI arguments to the matching command.
package cmd

// climax:name gh-commandeer
// climax:root-pkg root

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/StevenACoffman/gh-commandeer/cmd/push"
	"github.com/StevenACoffman/gh-commandeer/cmd/restore"
	"github.com/StevenACoffman/gh-commandeer/cmd/root"
	"github.com/StevenACoffman/gh-commandeer/cmd/status"
	"github.com/StevenACoffman/gh-commandeer/cmd/version"
)

// Run parses args and dispatches to the matching command.
// args must not include the executable name (pass os.Args[1:]).
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	r := root.New(stdout, stderr)
	version.New(r)
	push.New(r)
	status.New(r)
	restore.New(r)
	// register new commands here

	if err := r.Command.Parse(args); err != nil {
		fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command))
		return fmt.Errorf("parse: %w", err)
	}

	if err := r.Command.Run(ctx); err != nil {
		if !errors.Is(err, ff.ErrNoExec) {
			fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command.GetSelected()))
		}
		return err
	}

	return nil
}
