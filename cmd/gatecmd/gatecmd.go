// Package gatecmd implements the "gate" CLI command.
package gatecmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// longHelp is the command's prose, extracted from New because a page of help text inside
// a constructor puts the function over the length limit and none of it is logic.
const longHelp = `Block on error-severity findings, and say which kind of check produced them.

It reads a findings file and exits non-zero when any finding is error severity. Warnings
never block: the severity model is shared across this family of tools, so gnosis and
canonizer can gate on each other's findings without agreeing on anything else.

Two input shapes are accepted, because both are real. A finding.Result — a JSON object
with a diagnostics array — is the family's wire format. A gnosis envelope, which is what
gnosis lint --jsonl writes, carries the same array under data.diagnostics. Anything else
is refused naming both, rather than guessed at: a gate that inferred structure would be
deciding what to block on from a shape it did not recognise.

**It reports which act ran, and that is the requirement worth reading.** Structural
verification and semantic review are two different questions. Every deterministic check
asks whether a claim is supported in the way it says it is — the quotation is in the
archived text, byte for byte. None of them can ask whether the quotation bears on the
claim, and no amount of strengthening them closes that gap. So a structural pass means
the corpus is internally honest, not that anybody agrees with it, and reporting one as
"verified" is the overclaim this field exists to prevent.

Four states, because a clean critic produces no findings and "no critic finding" is
therefore not the same fact as "no critic ran":

  structural-only    nothing semantic has run over this corpus
  semantic-clean     a critic examined it and reported nothing
  semantic-findings  a critic examined it and had something to say
  unknown            the record could not be read, so neither can be claimed

Any aspect the findings' producer declared it did not examine is printed. A gate that
dropped those would ship on exactly the silence they exist to break.

The gate runs its self-test on every invocation: a planted error finding must be
classified as blocking and a planted warning must not. If the self-test fails the gate
blocks and says so is a defect in gnosis rather than in the corpus — a gate that cannot
demonstrate it still refuses is a green light of unknown provenance.

Reads only. It takes no lock and never writes.`

// Config holds the configuration for the gate command.
type Config struct {
	*root.Config

	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the gate command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("gate").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "gate",
		Usage:     "gnosis gate <FINDINGS-FILE>",
		ShortHelp: "block on error-severity findings, reporting which check produced them",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: read the file, judge, render.
func (c *Config) exec(_ context.Context, args []string) error {
	if len(args) != 1 {
		return c.usage(errors.New("gate needs exactly one findings file; " +
			"`gnosis lint --jsonl > findings.json` writes one"))
	}
	gated, err := bundle.GateFindings(c.Bundle, args[0])
	if err != nil {
		if errs.ErrorCode(err) == errs.EINVALID {
			// The file is the wrong shape, which is a mistake in the invocation
			// rather than a finding about the corpus.
			return c.usage(err)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	return c.report(gated)
}

// report renders the verdict and returns the matching exit code.
func (c *Config) report(gated *bundle.Gated) error {
	if c.JSONL {
		return c.emit(gated)
	}
	for _, d := range gated.Blocking {
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\t%s\t%s\n",
			d.Severity, d.Category, d.Path, d.Message)
	}
	c.summarise(gated)
	if gated.Blocks() {
		return root.ExitError(root.CodeFindings)
	}
	return nil
}

// emit writes the machine envelope and returns the matching exit code.
func (c *Config) emit(gated *bundle.Gated) error {
	if !gated.Blocks() {
		if err := c.EmitOK(gated); err != nil {
			return fmt.Errorf("gate: %w", err)
		}
		return nil
	}
	if err := c.EmitFindings(root.ReasonNeedsHuman, bundle.GateReason(gated),
		gated); err != nil {
		return fmt.Errorf("gate: %w", err)
	}
	return root.ExitError(root.CodeFindings)
}

// summarise writes the sentence §17.1 asks for, and the counts around it.
//
// **The sentence says what the pass covers rather than that it passed.** "0 blocking
// findings" and "0 blocking findings, and no critic has examined this corpus" send a
// reader to opposite conclusions about whether anything has been judged — which is the
// overclaim §17.1 names, and the reason the state is a word rather than a flag.
func (c *Config) summarise(gated *bundle.Gated) {
	_, _ = fmt.Fprintf(c.Stderr, "%d blocking, %d warning(s); %s\n",
		len(gated.Blocking), gated.Warnings, reviewSentence(gated.SemanticReview))
	for _, u := range gated.Unexamined {
		_, _ = fmt.Fprintf(c.Stderr, "not examined: %s — %s\n", u.Aspect, u.Reason)
	}
	if !gated.SelfTested {
		_, _ = fmt.Fprintf(c.Stderr, "%s\n", bundle.GateReason(gated))
	}
}

// reviewSentence renders §17.1's state as something a person can act on.
//
// The words are chosen so a passing gate cannot be misread: none of them is "verified",
// which is the one word §17.1 forbids for a structural pass.
func reviewSentence(state gnosis.SemanticReview) string {
	switch state {
	case gnosis.SemanticFindings:
		return "a critic examined this corpus and these findings include its verdicts"
	case gnosis.SemanticClean:
		return "a critic has examined this corpus and reported nothing"
	case gnosis.SemanticStructuralOnly:
		return "structural checks only — nothing has judged whether a quotation bears" +
			" on the claim it supports, so this says the corpus is internally honest" +
			" and not that anybody agrees with it"
	case gnosis.SemanticReviewUnknown:
		return "whether a critic has examined this corpus could not be determined," +
			" so neither a structural nor a semantic claim is made"
	default:
		return "the review state is unrecognised, so no claim is made about it"
	}
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("gate: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("gate: %w", c.Usage(cause))
}
