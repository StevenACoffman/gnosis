// Package doctorcmd implements the "doctor" CLI command.
package doctorcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
)

// Config holds the configuration for the doctor command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result is the payload: what was inspected, and what was found.
type Result struct {
	Environment lint.Environment     `json:"environment"`
	Diagnostics []finding.Diagnostic `json:"diagnostics"`
}

// New registers the doctor command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("doctor").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "doctor",
		Usage:     "gnosis doctor",
		ShortHelp: "check that the knowledge base is set up correctly",
		LongHelp: `Check the apparatus around the corpus: vocabulary, entry point, index, git hygiene.

This is not ` + "`gnosis lint`" + `, and the difference decides what you do next.
` + "`lint`" + ` examines the knowledge and its findings say the corpus is wrong.
` + "`doctor`" + ` examines the machinery, and its findings say gnosis cannot judge
whether the corpus is wrong. A vocabulary that does not load is a doctor finding,
because until it loads no document can be classified at all.

Reports the environment it inspected alongside the findings, so a report pasted
into an issue is self-contained.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: gather the environment, diagnose it, render.
func (c *Config) exec(ctx context.Context, _ []string) error {
	env, err := bundle.Inspect(ctx, c.Bundle)
	if err != nil {
		return c.fail(err)
	}
	result := Result{Environment: env, Diagnostics: lint.Diagnose(&env)}

	if c.JSONL {
		return c.emit(&result)
	}
	return c.render(&result)
}

// emit writes the machine envelope and returns the matching exit code.
func (c *Config) emit(result *Result) error {
	if !blocking(result.Diagnostics) {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("doctor: %w", err)
		}
		return nil
	}
	if err := c.EmitFindings(reasonFor(result.Diagnostics),
		"the knowledge base is not set up correctly", result); err != nil {
		return fmt.Errorf("doctor: %w", err)
	}
	return root.ExitError(root.CodeFindings)
}

// render writes the human form.
func (c *Config) render(result *Result) error {
	for _, d := range result.Diagnostics {
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\t%s\t%s\n",
			d.Severity, d.Category, d.Path, d.Message)
	}
	env := result.Environment
	if line := seededGates(env.GateSources); line != "" {
		_, _ = fmt.Fprintln(c.Stderr, line)
	}
	_, _ = fmt.Fprintf(c.Stderr,
		"%s: %d document(s), %d indexed, %d type(s), schema %d/%d; %d finding(s)\n",
		env.Bundle, env.Documents, env.IndexedRows, env.Types,
		env.IndexVersion, env.SchemaVersion, len(result.Diagnostics))

	if blocking(result.Diagnostics) {
		return root.ExitError(root.CodeFindings)
	}
	return nil
}

// fail adapts root's reporting to this command's name. Reaching it means the
// filesystem misbehaved: everything gnosis can diagnose is a finding, not an
// error, which is the whole point of a diagnostic command.
func (c *Config) fail(cause error) error {
	return fmt.Errorf("doctor: %w", c.Fail(root.ReasonNoBundle, cause))
}

// blocking reports whether any diagnostic is error-severity.
func blocking(ds []finding.Diagnostic) bool {
	for _, d := range ds {
		if d.Severity.Blocking() {
			return true
		}
	}
	return false
}

// reasonFor picks the machine token for the most significant finding.
func reasonFor(ds []finding.Diagnostic) string {
	for _, d := range ds {
		if !d.Severity.Blocking() {
			continue
		}
		switch d.Category {
		case "vocabulary":
			return root.ReasonVocabularyInvalid
		case "index":
			return root.ReasonIndexDrift
		}
	}
	return root.ReasonNeedsHuman
}

// seededGates says which standards files fell back to the embedded seed, or "".
//
// Requires: sources came from the environment.
// Ensures: one line whatever the mix, naming the version once. Pure.
//
// **One line rather than one per file**, which the first version got wrong in the
// most predictable way: every file falls back on a fresh corpus, so it printed four
// identical sentences differing only in a filename. That is the same defect the
// `type-unused` finding was grouped to avoid, made twice in one afternoon — which is
// the argument for the rule rather than against the reviewer.
func seededGates(sources []lint.GateSource) string {
	var seeded []string
	version := ""
	for _, g := range sources {
		if g.Origin == "seed" {
			seeded = append(seeded, g.File)
			version = g.Version
		}
	}
	switch {
	case len(seeded) == 0:
		return ""
	case len(seeded) == len(sources):
		// The ordinary state, said in a way that does not read as a problem.
		return "standards/: no file written; every gate from the " + version + " seed"
	default:
		return "from the " + version + " seed: " + strings.Join(seeded, ", ")
	}
}
