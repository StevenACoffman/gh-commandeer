// Package version implements the "version" CLI command.
package version

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"
	"github.com/StevenACoffman/gh-commandeer/cmd/root"
)

// Version is the application version string.
// Override at build time: go build -ldflags "-X 'github.com/StevenACoffman/gh-commandeer/cmd/version.Version=1.2.3'"
var Version = "dev"

// Config holds the configuration for the version command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the version command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("version").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "version",
		Usage:     "gh-commandeer version",
		ShortHelp: "print version information",
		LongHelp:  "Prints version information for the application.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	_, _ = fmt.Fprintln(cfg.Stdout, "version "+Version)
	return nil
}
