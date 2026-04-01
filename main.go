// Package main is the entry point for the CLI.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v4"
	"github.com/StevenACoffman/gh-commandeer/cmd"
)

const (
	exitFail    = 1
	exitSuccess = 0
)

func main() {
	ctx := context.Background()
	err := cmd.Run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	switch {
	case err == nil, errors.Is(err, ff.ErrHelp), errors.Is(err, ff.ErrNoExec):
		os.Exit(exitSuccess)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "error: %+v\n", err)
		os.Exit(exitFail)
	}
}
