// Package challengecmd implements the "challenge" CLI command.
package challengecmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// longHelp is the command's prose, extracted from New because a page of help text
// inside a constructor puts the function over the length limit and none of it is logic.
const longHelp = `Contest an accepted claim, or list the contests already filed.

A challenge is a first-class operation because the gap it fills is the most capable
informant a corpus has: a reader who already knows a claim is wrong. Without it such
a person can open a pull request against a file, which routes a knowledge dispute
through a diff review, or say something in chat, which the corpus never hears.

--class states what kind of thing would settle the dispute, which is what decides
whether a person needs to be involved at all:

  replay           the archived source does not contain the quote — gnosis can
                   settle this one itself, by re-running the check
  contradiction    this claim conflicts with that one and nothing noticed
  coverage         the evidence does not support the scope the claim asserts
  rung             the claim is causal and its support is observational
  dimension-drift  this subject's values changed dimension
  scope            the stated limitations are incomplete

--rationale is required. A reader who cannot say why a claim is wrong has filed a
doubt rather than a challenge, and the corpus has no way to act on a doubt. It is the
same one-field discipline a warrant's rationale is, for the same reason: somebody who
cannot articulate the objection usually stops before finishing the sentence.

A challenge is written into the frontmatter of the document it contests, not into the
index. That is what makes it travel — it reaches every other user through the same
git pull that carries the claim — reconstructible by a rebuild, and reviewable, since
it arrives as a diff on the page it is about.

Filing does not block anything. Only a replay challenge gnosis has itself verified
becomes error-severity; the rest are warnings, because any challenge blocking on
assertion alone would make the front door a denial of service. What is guaranteed is
that silence is visible and ages: an open challenge appears here, --unanswered lists
them oldest first, and lint's unanswered-challenge check reports any older than the
window in standards/challenge.toml.

Being wrong costs the challenger nothing. No count of rejected challenges attaches to
an actor and none feeds a trust tier — if challenging carried risk, the people best
placed to challenge would be the ones who stopped.

The path goes **after** the flags. That is how this flag parser reads a command line —
the first positional argument ends the flags — so a path written first leaves --class
and --rationale unset and the invocation is refused for the wrong reason.

Without --apply it reports what would be filed and writes nothing. Listing reads
only: it takes no lock, so it is safe against a bundle somebody else is writing to.`

// Config holds the configuration for the challenge command.
type Config struct {
	*root.Config

	// Class is what kind of thing would settle the dispute (§10.7.1).
	Class string

	// Rationale is why the claim is wrong. Required when filing.
	Rationale string

	// By is the challenger. An agent may file one: challenging grants no authority,
	// and a check that noticed a contradiction the selector is blind to is exactly
	// the informant §6.2.1 wants and has no person attached.
	By string

	// List reports the challenges already filed instead of filing one.
	List bool

	// Unanswered restricts the listing to challenges still open.
	Unanswered bool

	// Apply writes. Preview is the default for the reason §4.6.2 gives.
	Apply bool

	Flags   *ff.FlagSet
	Command *ff.Command
}

// Filed is one challenge as the listing reports it.
type Filed struct {
	Path string `json:"path"`

	// ID is the challenge's own identifier, so `gnosis adjudicate` can name it.
	ID string `json:"id"`

	Class string `json:"class"`
	By    string `json:"by"`
	At    string `json:"at"`
	State string `json:"state"`

	// Rationale is carried in full rather than truncated. The listing's whole
	// purpose is to put a reader's objection in front of somebody who can answer
	// it, and a preview that cut it off would send them back to the file.
	Rationale string `json:"rationale"`
}

// Result is the listing payload.
type Result struct {
	Challenges []Filed `json:"challenges"`

	// Unanswered reports whether the list was restricted to open challenges, so a
	// short list is not mistaken for a quiet corpus.
	Unanswered bool `json:"unanswered"`
}

// New registers the challenge command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("challenge").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Class, 0, "class", "",
		"what would settle it: replay, contradiction, coverage, rung, dimension-drift, scope")
	cfg.Flags.StringVar(&cfg.Rationale, 0, "rationale", "", "why the claim is wrong")
	cfg.Flags.StringVar(&cfg.By, 0, "by", "", "who is challenging, as <kind>:<id>")
	cfg.Flags.BoolVar(&cfg.List, 0, "list", "list the challenges already filed")
	cfg.Flags.BoolVar(&cfg.Unanswered, 0, "unanswered",
		"with --list, report only the challenges still open, oldest first")
	cfg.Flags.BoolVar(&cfg.Apply, 0, "apply", "file the challenge rather than preview it")
	cfg.Command = &ff.Command{
		Name: "challenge",
		Usage: "gnosis challenge --class C --rationale R --by ACTOR <PATH> [--apply] |" +
			" gnosis challenge --list [--unanswered]",
		ShortHelp: "contest an accepted claim, or list the contests filed",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: either list, or file one through the coordinator.
func (c *Config) exec(ctx context.Context, args []string) error {
	if c.List || c.Unanswered {
		if len(args) != 0 {
			return c.usage(errors.New("--list takes no arguments"))
		}
		return c.list()
	}
	if len(args) != 1 {
		return c.usage(errors.New(
			"challenge needs exactly one document path; try " +
				"`gnosis challenge c/<id>-<slug>.md --class coverage --rationale ... --by human:you`"))
	}

	by, ok := gnosis.ParseActor(c.By)
	if !ok {
		return c.usage(errors.New(
			"--by must be <kind>:<id>, kind one of human, agent, check"))
	}
	class, ok := gnosis.ParseChallengeClass(c.Class)
	if !ok {
		return c.usage(fmt.Errorf("--class must be one of %s",
			strings.Join(gnosis.ChallengeClasses(), ", ")))
	}

	coordinator := bundle.Coordinator{Dir: c.Bundle, Warn: c.Stderr, Rules: c.Rules}
	outcome, err := coordinator.Execute(ctx, &command.Challenge{
		Path:      args[0],
		Class:     class,
		By:        by,
		Rationale: c.Rationale,
		Eff:       effect(c.Apply),
	})
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	return c.reportFiled(outcome)
}

// list reports the challenges the corpus already carries.
//
// Reads only, and it says so in the help: a reader deciding whether to file a
// challenge should not be queued behind somebody's ingest.
func (c *Config) list() error {
	docs, err := bundle.LoadChallenges(c.Bundle)
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	out := make([]Filed, 0, len(docs))
	for i := range docs {
		d := &docs[i]
		if c.Unanswered && !d.Challenge.Open() {
			continue
		}
		out = append(out, Filed{
			Path: d.Path, ID: d.Challenge.ID.String(),
			Class: string(d.Challenge.Class), By: d.Challenge.By,
			At: d.Challenge.At, State: string(stateOf(&d.Challenge)),
			Rationale: d.Challenge.Rationale,
		})
	}
	// Oldest first, which §10.7.3 asks for by name: the challenge that has waited
	// longest is the one whose silence is least defensible. Ties break on path so two
	// runs over one corpus agree.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At < out[j].At
		}
		return out[i].Path < out[j].Path
	})
	return c.report(&Result{Challenges: out, Unanswered: c.Unanswered})
}

// stateOf is the state to report, filling in the one a document left implicit.
//
// An absent `state:` reads as open, and the listing says `open` rather than leaving the
// column blank — a reader scanning for what is outstanding should not have to know
// which of Go's zero values means what.
func stateOf(ch *gnosis.Challenge) gnosis.ChallengeState {
	if ch.State == "" {
		return gnosis.ChallengeOpen
	}
	return ch.State
}

// effect maps the flag to the command's field. Preview is the default because §4.6.2
// requires the zero value not to be the one that writes.
func effect(apply bool) command.Effect {
	if apply {
		return command.EffectApply
	}
	return command.EffectPreview
}

// reportFiled renders the outcome of filing.
//
// A preview is `ok`: asking what would be filed is a legitimate question with a
// successful answer, and reporting it as blocked would make the safe path look like
// the failing one.
func (c *Config) reportFiled(outcome gnosis.Outcome) error {
	if c.JSONL {
		if err := c.EmitOutcome(outcome); err != nil {
			return fmt.Errorf("challenge: %w", err)
		}
		return exitFor(outcome)
	}
	// **The outcome's own sentence, and a rendered one when it has none.** A
	// successful write carries no Message by construction — `gnosis.OK` takes only
	// data — so printing Message alone put a blank line where the answer went. Found
	// by running the command.
	if outcome.Message != "" {
		_, _ = fmt.Fprintf(c.Stdout, "%s\n", outcome.Message)
	} else {
		_, _ = fmt.Fprintf(c.Stdout, "%s\n", filedLine(outcome))
	}
	if outcome.Status == gnosis.StatusOK && !c.Apply {
		_, _ = fmt.Fprintf(c.Stderr,
			"nothing was written; re-run with --apply to file it\n")
	}
	return exitFor(outcome)
}

// filedLine is the human sentence for a successful file or preview.
//
// It reads the outcome's data rather than being handed the command, so the line
// cannot describe something other than what the coordinator did — the same reason the
// envelope carries the data at all.
func filedLine(outcome gnosis.Outcome) string {
	data, _ := outcome.Data.(map[string]any)
	class, _ := data["class"].(string)
	path, _ := data["path"].(string)
	if filed, _ := data["filed"].(bool); filed {
		id, _ := data["id"].(string)
		return "filed " + class + " challenge " + id + " against " + path
	}
	return "would file a " + class + " challenge against " + path
}

// report renders a listing.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("challenge: %w", err)
		}
		return nil
	}
	for i := range result.Challenges {
		f := &result.Challenges[i]
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\t%s\t%s\t%s\n\t%s\n",
			f.State, f.Class, f.At, f.By, f.Path, oneLine(f.Rationale))
	}
	// The scope of the list is on stderr with the count, because "3 challenges" and
	// "3 open challenges" are different facts and a reader piping stdout keeps both.
	scope := "challenge(s)"
	if result.Unanswered {
		scope = "open challenge(s)"
	}
	_, _ = fmt.Fprintf(c.Stderr, "%d %s\n", len(result.Challenges), scope)
	return nil
}

// oneLine renders a rationale for a table row, keeping every word.
//
// Newlines become spaces rather than being truncated: the rationale is the whole
// point of the entry, and a listing that cut it off would send the reader to the file
// it exists to save them opening.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
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
	return fmt.Errorf("challenge: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("challenge: %w", c.Usage(cause))
}
