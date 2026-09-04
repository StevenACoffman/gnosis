// Package minecmd implements the "mine" CLI command.
package minecmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/mine"
)

// longHelp is the command's prose, extracted from New for `misscmd`'s reason.
const longHelp = `Find questions this corpus should have answered (SPEC §9.6).

The manifesto's requirement that every corrective interaction be accretive cannot depend
on a model choosing to record it. This is the half that needs no cooperation: hand it a
session transcript and it reports the questions somebody had to ask more than once.

  retried    asked again in the same session. The first answer did not land.
  recurring  asked in more than one session. Nobody wrote it down, and the next
             person will ask it too.

Repetition rather than sentiment, deliberately. Detecting disappointment needs a
vocabulary of what disappointment sounds like, a vocabulary is a standards value with a
rationale, and nobody can write that rationale from measurement yet. Re-asking is
observable without any of it — that is what re-asking means.

This reports and never writes. A chat answer cites no archived source, and the promote
gate refuses a document that declares none — so a command that filed these
automatically would fill quarantine with drafts that can never promote. Writing a
candidate up means ` + "`gnosis fetch`" + ` and ` + "`gnosis ingest`" + `, where evidence exists.

It reads a transcript and nothing else: no bundle, no lock, no network, no model.

--session - reads from stdin, which is the Stop-hook contract: the hook hands the
session over and gnosis never goes looking for it. Wiring the hook is your tool's
business and deliberately not described here — a hook configuration format written into
this help would be another tool's format, kept in a second place, rotting quietly.`

// defaultWidth is how much of a question a line shows.
//
// Eighty, which is the terminal width every other report in this tool assumes. It is a
// display bound and not a judgement about questions: --width prints them whole.
const defaultWidth = 80

// Config holds the configuration for the mine command.
type Config struct {
	*root.Config

	// Session is the transcript to read, or `-` for stdin.
	Session string

	// Width is how much of a question to print. Zero prints it whole.
	Width int

	// Flags and Command are this command's own — see proofcmd for why declaring them
	// is not boilerplate.
	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result is the payload: what was mined, and from how much.
type Result struct {
	Candidates []mine.Candidate `json:"candidates"`

	// Sessions and Exchanges are the population the candidates came from, so a short
	// list can be read as a fraction of something. **Not omitempty**: a transcript
	// that yielded no exchange at all is a different report from one that yielded
	// many and repeated none, and a JSON reader cannot tell an absent field from a
	// zero one.
	Sessions  int `json:"sessions"`
	Exchanges int `json:"exchanges"`
}

// New registers the mine command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("mine").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Session, 0, "session", "",
		"session transcript to read, or - for stdin")
	cfg.Flags.IntVar(&cfg.Width, 0, "width", defaultWidth,
		"how much of each question to print; 0 prints it whole")
	cfg.Command = &ff.Command{
		Name:      "mine",
		Usage:     "gnosis mine --session FILE",
		ShortHelp: "report questions somebody had to ask more than once",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: read, normalize, report.
func (c *Config) exec(_ context.Context, args []string) error {
	if len(args) != 0 {
		return c.usage(errors.New(
			"mine takes no arguments; --session carries the transcript, because ff" +
				" ends flag parsing at the first positional"))
	}
	if strings.TrimSpace(c.Session) == "" {
		return c.usage(errors.New("--session is required; pass a transcript or - for stdin"))
	}
	if c.Width < 0 {
		return c.usage(fmt.Errorf("--width cannot be negative, got %d", c.Width))
	}

	sessions, err := c.read()
	if err != nil {
		return c.fail(err)
	}
	result := &Result{
		Candidates: mine.Report(mine.Candidates(sessions)),
		Sessions:   len(sessions),
	}
	for i := range sessions {
		result.Exchanges += len(mine.Exchanges(&sessions[i]))
	}
	return c.report(result)
}

// read normalizes the transcript from a file or from stdin.
//
// `-` is the stdin spelling `admit`, `critic` and `file` already use, and it is a flag
// value rather than a bare dash for the reason recorded there: `ff` consumes a bare `-`
// as the end-of-flags terminator.
func (c *Config) read() ([]mine.Session, error) {
	if c.Session == "-" {
		sessions, err := mine.ReadClaudeCode(c.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading the transcript from stdin: %w", err)
		}
		return sessions, nil
	}
	f, err := os.Open(c.Session)
	if err != nil {
		return nil, fmt.Errorf("reading the transcript: %w", err)
	}
	defer func() { _ = f.Close() }()

	sessions, err := mine.ReadClaudeCode(f)
	if err != nil {
		return nil, fmt.Errorf("reading the transcript: %w", err)
	}
	return sessions, nil
}

// report renders the outcome.
//
// **Nothing exits non-zero**, and no rate is reported. §17 forbids a count presented as
// health, and this count goes up when the corpus is used and down only when somebody
// writes an answer — a percentage here would be the most target-shaped number the tool
// could produce, because it improves when people stop asking.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("mine: %w", err)
		}
		return nil
	}
	for i := range result.Candidates {
		cand := &result.Candidates[i]
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%d\t%s\n",
			cand.Reason, cand.Asked, mine.OneLine(cand.Question, c.Width))
	}
	writeSummary(c.Stderr, result)
	return nil
}

// writeSummary says what the candidates were drawn from, and what to do with them.
func writeSummary(w io.Writer, result *Result) {
	_, _ = fmt.Fprintf(w, "%d candidate(s) from %d exchange(s) in %d session(s)\n",
		len(result.Candidates), result.Exchanges, result.Sessions)
	if len(result.Candidates) == 0 {
		return
	}
	// The next step is named, because a candidate list with no route out of it is a
	// report somebody reads once.
	_, _ = fmt.Fprintf(w,
		"these are questions, not documents: `gnosis fetch` a source and `gnosis"+
			" ingest` it, so what gets written down carries evidence\n")
}

// fail and usage adapt root's reporting to this command's name. The reason is fixed for
// `proofcmd`'s reason: every way this command fails is the transcript being unreadable.
func (c *Config) fail(cause error) error {
	return fmt.Errorf("mine: %w", c.Fail(root.ReasonNoBundle, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("mine: %w", c.Usage(cause))
}
