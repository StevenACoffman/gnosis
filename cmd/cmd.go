// Package cmd is the dispatcher for the gnosis CLI.
// It registers all commands and routes incoming arguments
// to the matching command implementation.
package cmd

// climax:name gnosis
// climax:root-pkg root
// climax:env-prefix GNOSIS

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/StevenACoffman/gnosis/cmd/doctorcmd"
	"github.com/StevenACoffman/gnosis/cmd/graphcmd"
	"github.com/StevenACoffman/gnosis/cmd/indexcmd"
	"github.com/StevenACoffman/gnosis/cmd/initcmd"
	"github.com/StevenACoffman/gnosis/cmd/lintcmd"
	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/cmd/searchcmd"
	"github.com/StevenACoffman/gnosis/cmd/showcmd"
	"github.com/StevenACoffman/gnosis/cmd/version"
)

// Run parses args and dispatches to the matching command.
// args must not include the executable name (pass os.Args[1:]).
//
// Every flag can be set via a GNOSIS_-prefixed environment variable.
// The mapping rule is: prepend GNOSIS_, uppercase, replace dashes with
// underscores.
//
// Flags supplied on the command line always take precedence over env vars.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	r := root.New(stdin, stdout, stderr)
	version.New(r)
	initcmd.New(r)
	doctorcmd.New(r)
	lintcmd.New(r)
	indexcmd.New(r)
	searchcmd.New(r)
	showcmd.New(r)
	graphcmd.New(r)
	// register new commands here

	if err := r.Command.Parse(args, ff.WithEnvVarPrefix("GNOSIS")); err != nil {
		_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command))
		return fmt.Errorf("parse: %w", err)
	}

	// An unmatched token leaves the selected command a group parent (Exec == nil)
	// with a leftover positional. Without this guard it falls through to Run,
	// returns ff.ErrNoExec, and exits 0 — indistinguishable from a bare invocation.
	if sel := r.Command.GetSelected(); sel.Exec == nil {
		if rest := sel.Flags.GetArgs(); len(rest) > 0 {
			_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(sel))
			return fmt.Errorf("%s: unknown subcommand %q", sel.Name, rest[0])
		}
	}

	if err := r.Command.Run(ctx); err != nil {
		// Don't print usage help for ErrNoExec (no subcommand given) or
		// ExitError (command already reported its own outcome).
		var exitErr root.ExitError
		if !errors.Is(err, ff.ErrNoExec) && !errors.As(err, &exitErr) {
			_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command.GetSelected()))
		}
		return err
	}

	return nil
}
