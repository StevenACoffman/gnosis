// Package adjudicatecmd implements the "adjudicate" CLI command.
package adjudicatecmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// longHelp is the command's prose, extracted from New because a page of help text
// inside a constructor puts the function over the length limit and none of it is logic.
const longHelp = `Record a human decision about a claim, and write its warrant.

When two claims conflict and a person decides, the decision is knowledge present in
neither source. It can carry no quotation by construction, so it fails the evidence
invariant that protects everything else — the highest-value artifact a team produces,
refused by the check that exists to protect quality. The warrant is what admits it:
the review, the people, the date, and the reasoning.

--rationale is required at every authority, including a corpus with one curator. It
matters more than the authorization rule and it is worth saying why: a permission bit
asks whether somebody is allowed to decide, while a rationale asks them to write down
why, in a commit, in front of colleagues — and somebody who cannot articulate a reason
usually stops before finishing the sentence. At sole it still works, because the
reader you are writing for is yourself in six months, and that reader has no other way
to reconstruct the decision.

The authority is derived from the adjudicators the corpus actually has, never
configured: one person is sole, two or three are paired, four or more are quorum. At
paired and quorum a decision needs a --co-signer, or an --override whose reason is
recorded. The override is the escape hatch and recording it is the whole mechanism — a
waived co-signature that leaves no trace cannot be told from an authority that was
never in force, and a queue that can block indefinitely stops being used. At quorum it
cannot be waived: with four adjudicators, one being unavailable is not a reason the
corpus cannot wait.

The authority in force is written onto the warrant, so scaling down never invalidates
what was already decided. A decision made under quorum stays exactly as valid when the
team shrinks to one.

--reverses names the warrant this decision overturns, never the claim. It is a link
and not a judgement: no score attaches to a reversed warrant and no reputation to its
author, because reversal is the ordinary consequence of deciding under incomplete
information — and a corpus that produced none would be one where nobody was deciding
anything contestable.

--challenge closes a reader's challenge. It closes the same way whether the claim
falls or stands, because a rejected challenge is recorded and never deleted: a claim
challenged three times and upheld three times is a different artifact from one never
questioned, and only one of them has evidence that anybody looked.

An adjudicator must be a person. Challenging and discarding may be done by an agent
because neither grants anything; this one does.

Without --apply it reports what would be written and writes nothing. Applying takes
the writer lock.`

// Config holds the configuration for the adjudicate command.
type Config struct {
	*root.Config

	// Claim names the claim being decided, within the document.
	Claim string

	// By is the adjudicator, and must be a person.
	By string

	// Rationale is why it was decided this way. Required.
	Rationale string

	// CoSigner is the second signature an escalated claim needs at paired or quorum.
	CoSigner string

	// Override is the recorded reason a required co-signature was waived.
	Override string

	// Reverses names the warrant this decision overturns.
	Reverses string

	// Challenge names a challenge this decision answers.
	Challenge string

	// Apply writes. Preview is the default for the reason §4.6.2 gives.
	Apply bool

	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the adjudicate command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("adjudicate").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Claim, 0, "claim", "", "the claim being decided")
	cfg.Flags.StringVar(&cfg.By, 0, "by", "", "who decided, as human:<id>")
	cfg.Flags.StringVar(&cfg.Rationale, 0, "rationale", "", "why it was decided this way")
	cfg.Flags.StringVar(&cfg.CoSigner, 0, "co-signer", "", "the second signature, as human:<id>")
	cfg.Flags.StringVar(&cfg.Override, 0, "override", "",
		"why a required co-signature was waived")
	cfg.Flags.StringVar(&cfg.Reverses, 0, "reverses", "",
		"the warrant this decision overturns")
	cfg.Flags.StringVar(&cfg.Challenge, 0, "challenge", "",
		"the challenge this decision answers")
	cfg.Flags.BoolVar(&cfg.Apply, 0, "apply", "write the warrant rather than preview it")
	cfg.Command = &ff.Command{
		Name: "adjudicate",
		Usage: "gnosis adjudicate --claim ID --by human:<id> --rationale R <PATH>" +
			" [--co-signer A] [--override R] [--reverses ID] [--challenge ID] [--apply]",
		ShortHelp: "record a human decision about a claim and write its warrant",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: parse the actors, hand the command over, render.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return c.usage(errors.New("adjudicate needs exactly one document path, after " +
			"the flags; try `gnosis adjudicate --claim c1 --by human:you --rationale ... " +
			"c/<id>-<slug>.md`"))
	}
	by, ok := gnosis.ParseActor(c.By)
	if !ok {
		return c.usage(errors.New("--by must be human:<id>; an adjudication is a " +
			"human decision (§10.6.4)"))
	}
	coSigner := gnosis.ActorUnset
	if c.CoSigner != "" {
		if coSigner, ok = gnosis.ParseActor(c.CoSigner); !ok {
			return c.usage(errors.New("--co-signer must be human:<id>"))
		}
	}

	coordinator := bundle.Coordinator{Dir: c.Bundle, Warn: c.Stderr, Rules: c.Rules}
	outcome, err := coordinator.Execute(ctx, &command.Adjudicate{
		Path:      args[0],
		ClaimID:   c.Claim,
		By:        by,
		Rationale: c.Rationale,
		CoSigner:  coSigner,
		Override:  c.Override,
		Reverses:  c.Reverses,
		Challenge: c.Challenge,
		Eff:       effect(c.Apply),
	})
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	return c.report(outcome)
}

// effect maps the flag to the command's field. Preview is the default because §4.6.2
// requires the zero value not to be the one that writes.
func effect(apply bool) command.Effect {
	if apply {
		return command.EffectApply
	}
	return command.EffectPreview
}

// report renders the outcome.
//
// A preview is `ok`: asking what would be written is a legitimate question with a
// successful answer. A refusal is `blocked` rather than an error — the corpus is
// intact, nothing was written, and what to do next is a person's decision.
func (c *Config) report(outcome gnosis.Outcome) error {
	if c.JSONL {
		if err := c.EmitOutcome(outcome); err != nil {
			return fmt.Errorf("adjudicate: %w", err)
		}
		return exitFor(outcome)
	}
	if outcome.Message != "" {
		_, _ = fmt.Fprintf(c.Stdout, "%s\n", outcome.Message)
	} else {
		_, _ = fmt.Fprintf(c.Stdout, "%s\n", decidedLine(outcome))
	}
	if outcome.Status == gnosis.StatusOK && !c.Apply {
		_, _ = fmt.Fprintf(c.Stderr,
			"nothing was written; re-run with --apply to record it\n")
	}
	return exitFor(outcome)
}

// decidedLine is the human sentence for a successful decision or preview.
//
// It reads the outcome's data rather than the command, so the line cannot describe
// something other than what the coordinator did — and the authority is reported because
// it is derived rather than chosen, so a caller who did not know the corpus had reached
// `paired` learns it here.
func decidedLine(outcome gnosis.Outcome) string {
	data, _ := outcome.Data.(map[string]any)
	claim, _ := data["claim"].(string)
	path, _ := data["path"].(string)
	authority, _ := data["authority"].(string)
	verb := "would adjudicate"
	if done, _ := data["adjudicated"].(bool); done {
		verb = "adjudicated"
	}
	line := verb + " claim " + claim + " in " + path + ", at " + authority
	if closed, _ := data["challenge_closed"].(string); closed != "" {
		line += "; challenge " + closed + " closed"
	}
	return line
}

// exitFor maps an outcome to this command's exit code.
func exitFor(outcome gnosis.Outcome) error {
	if outcome.Status == gnosis.StatusOK {
		return nil
	}
	return root.ExitError(root.CodeFindings)
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("adjudicate: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("adjudicate: %w", c.Usage(cause))
}
