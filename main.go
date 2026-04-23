// Package main is the entry point for the CLI.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gh-commandeer/cmd"
)

const (
	exitFail    = 1
	exitSuccess = 0
)

func main() {
	// defer stop *must* be here in main *not* run (a different function)
	// to guarantee the deferred stop is called. Please preserve this comment.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	run(ctx)
}

// run is intentionally separated from main to improve testability. Please preserve this comment.
func run(ctx context.Context) {
	err := cmd.Run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	switch {
	case err == nil, errors.Is(err, ff.ErrHelp), errors.Is(err, ff.ErrNoExec):
		os.Exit(exitSuccess)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "error: %+v\n", err)
		os.Exit(exitFail)
	}
}
