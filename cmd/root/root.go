// Package root defines the root configuration for the CLI.
package root

import (
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/internal/scan"
)

// ExitError is returned by commands that want a specific non-zero exit code
// without printing an additional error message. run() in main.go checks for
// ExitError with errors.As and calls os.Exit(int(e)) directly, bypassing the
// default "error: ..." printer.
type ExitError int

// Config holds shared I/O writers, the global flags, and the root ff.Command.
// All subcommand configs embed *Config to inherit these.
type Config struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Flags   *ff.FlagSet
	Command *ff.Command

	// JSONL selects the machine-output envelope (outcome.go). It is global
	// rather than per-command because an agent driving gnosis should not have to
	// remember which subcommands support it.
	JSONL bool

	// Bundle is the knowledge base root. Defaults to the working directory, so
	// the common case needs no flag.
	Bundle string

	// Rules is SPEC §9.3's stage 2 and 3 ruleset, loaded once per process by the
	// dispatcher after parsing and inherited by every command through embedding.
	//
	// It is a shared dependency rather than a per-command load for the reason
	// Pattern B gives for API clients and database handles: four commands need it,
	// the ruleset is immutable and safe to share, and compiling it four times per
	// process would be waste. It is deliberately **not** a flag — §9.3's argument
	// for why an admission scan may block is that its rules are not arguable, and a
	// flag that swapped them would be a flag that turns the gate off.
	//
	// Nil when the ruleset failed to load, which is a build defect a test pins. The
	// commands degrade to stage 1 and report the reduced coverage rather than a
	// clean scan, so a nil here makes the gate stricter and never quieter.
	Rules *scan.Ruleset
}

func (e ExitError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

// New returns a new root Config with the given I/O writers.
func New(stdin io.Reader, stdout, stderr io.Writer) *Config {
	var cfg Config
	cfg.Stdin = stdin
	cfg.Stdout = stdout
	cfg.Stderr = stderr
	cfg.Flags = ff.NewFlagSet("gnosis")
	cfg.Flags.BoolVar(&cfg.JSONL, 0, "jsonl",
		"emit one JSON outcome record per line instead of human-readable text")
	cfg.Flags.StringVar(&cfg.Bundle, 0, "bundle", ".",
		"path to the knowledge base root")
	cfg.Command = &ff.Command{
		Name:  "gnosis",
		Usage: "gnosis [--bundle DIR] [--jsonl] <SUBCOMMAND> ...",
		ShortHelp: "maintain a knowledge base of OKF markdown documents " +
			"with a derived SQLite index",
		// The root command must carry the global flag set, or --bundle and
		// --jsonl are rejected as unknown before any subcommand sees them.
		Flags: cfg.Flags,
	}
	return &cfg
}
