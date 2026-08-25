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

	"github.com/StevenACoffman/gnosis/cmd/admitcmd"
	"github.com/StevenACoffman/gnosis/cmd/auditcmd"
	"github.com/StevenACoffman/gnosis/cmd/debtcmd"
	"github.com/StevenACoffman/gnosis/cmd/doctorcmd"
	"github.com/StevenACoffman/gnosis/cmd/fetchcmd"
	"github.com/StevenACoffman/gnosis/cmd/graphcmd"
	"github.com/StevenACoffman/gnosis/cmd/indexcmd"
	"github.com/StevenACoffman/gnosis/cmd/ingestcmd"
	"github.com/StevenACoffman/gnosis/cmd/initcmd"
	"github.com/StevenACoffman/gnosis/cmd/lintcmd"
	"github.com/StevenACoffman/gnosis/cmd/logcmd"
	"github.com/StevenACoffman/gnosis/cmd/promotecmd"
	"github.com/StevenACoffman/gnosis/cmd/quarantinecmd"
	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/cmd/schemacmd"
	"github.com/StevenACoffman/gnosis/cmd/searchcmd"
	"github.com/StevenACoffman/gnosis/cmd/showcmd"
	"github.com/StevenACoffman/gnosis/cmd/standardscmd"
	"github.com/StevenACoffman/gnosis/cmd/version"
	"github.com/StevenACoffman/gnosis/internal/scan"
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
	schemacmd.New(r)
	searchcmd.New(r)
	showcmd.New(r)
	graphcmd.New(r)
	fetchcmd.New(r)
	ingestcmd.New(r)
	admitcmd.New(r)
	quarantinecmd.New(r)
	promotecmd.New(r)
	logcmd.New(r)
	standardscmd.New(r)
	auditcmd.New(r)
	debtcmd.New(r)
	// register new commands here

	if err := r.Command.Parse(args, ff.WithEnvVarPrefix("GNOSIS")); err != nil {
		_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command))
		return fmt.Errorf("parse: %w", err)
	}

	// Post-parse initialisation, which is where a shared dependency belongs: the
	// §9.3 ruleset is compiled once here and inherited by every command through
	// root.Config rather than loaded four times.
	//
	// A failure is reported and does not stop the run. The ruleset is embedded, so
	// the only way to fail is a build defect that a test already catches — and the
	// commands degrade to a stage-1 scan that *reports* the missing stages, which
	// makes the promote gate stricter rather than quieter. Refusing every command
	// including `lint` and `search` because a pattern file will not compile would
	// take a corpus offline for a reason unrelated to reading it.
	rules, rulesErr := scan.Rules()
	if rulesErr != nil {
		_, _ = fmt.Fprintf(stderr,
			"warning: the §9.3 pattern ruleset did not load, so injection and secret "+
				"scanning are unavailable and every scan will report them missing: %v\n",
			rulesErr)
	}
	r.Rules = rules

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
