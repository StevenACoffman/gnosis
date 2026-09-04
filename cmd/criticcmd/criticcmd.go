// Package criticcmd implements the "critic" CLI command.
package criticcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/finding"
)

// longHelp is the command's prose, extracted from New because a page of help text inside
// a constructor puts the function over the length limit and none of it is logic.
const longHelp = `Ask a cold critic whether a claim's source supports it, and file the answer.

Structural verification and semantic review are two acts, and this is the second one.
Every deterministic check in this tool asks whether a claim is supported *in the way it
says it is*: the quotation appears in the archived text, byte for byte. None of them can
ask whether the quotation bears on the claim. A quote can validate exactly and still not
support what it is offered for, and no amount of tightening the quote check closes that
gap — it makes the justification stronger without making it related.

So this emits a prompt and stops. gnosis never calls a model: an agent, a person or a
script answers, and the answer comes back with --response.

The critic is blinded, and that is a requirement rather than a nicety. The prompt
carries the claim, its quotations and the archived source, and never the warrant, the
status, the trust tier or the verification history — a judge shown the conclusion a
corpus already reached tends to find support for it, and its agreement then carries no
information beyond the fact that it was told.

--sample N draws N claims from the whole corpus, reproducibly, seeded from
standards/sample.toml. The default is five, and five is not arbitrary: the median of a
population lies between the smallest and the largest of any five random samples with
93.75% probability. That is enough to say something real about a corpus at the cost of
five prompts.

The draw is the point rather than a saving. An exhaustive deterministic pass is precise
and systematically blind; a small random sample is imprecise and unbiased. Preferring
the first because it produces an exact number is choosing precision with unknown
systemic error over imprecision with quantifiable error.

--path P critiques the claims of one document instead, and ignores --sample: naming a
document has already chosen the population.

Claims citing no archived source are skipped and counted. A critic judges a claim
against its source, and one with no source has nothing to be judged against.

Every reply carries what it examined and what it did not, and that block is recorded in
.gnosis/coverage.jsonl and fed into later prompts for the same claim — so a second
critique is steered toward ground nobody has covered. It records what was *looked at*
and never what was concluded, which is why feeding it forward is the opposite of
contamination.

Nothing here blocks. A verdict is advisory: gnosis stamps every finding a warning, the
reply has no way to ask for anything else, and this command exits 0 whatever comes back.
A critic that could stop a build would be a model gating the corpus, and a critic that
learned declaring a gap has consequences would stop declaring gaps.`

// Config holds the configuration for the critic command.
type Config struct {
	*root.Config

	// Model and ModelVersion identify what will answer, and are part of every key.
	Model        string
	ModelVersion string

	// Path critiques one document's claims instead of a sample.
	Path string

	// SampleN is how many claims to draw. Zero takes standards/sample.toml's default.
	SampleN int

	// Key names the prompt a reply answers, in response mode.
	Key string

	// Response is the file holding the reply, or `-` for stdin.
	Response string

	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the critic command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("critic").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Model, 0, "model", "", "what will answer; part of the cache key")
	cfg.Flags.StringVar(&cfg.ModelVersion, 0, "model-version", "",
		"the model's version; part of the cache key")
	cfg.Flags.StringVar(&cfg.Path, 0, "path", "", "critique one document's claims")
	cfg.Flags.IntVar(&cfg.SampleN, 0, "sample", 0,
		"how many claims to draw; the default lives in standards/sample.toml")
	cfg.Flags.StringVar(&cfg.Key, 0, "key", "", "the prompt a reply answers")
	cfg.Flags.StringVar(&cfg.Response, 0, "response", "",
		"file holding the reply, or - for stdin")
	cfg.Command = &ff.Command{
		Name: "critic",
		Usage: "gnosis critic --model M [--path P | --sample N] |" +
			" gnosis critic --key K --response FILE",
		ShortHelp: "ask a cold critic whether a claim's source supports it",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: take the lock, then emit or file.
func (c *Config) exec(ctx context.Context, args []string) error {
	if err := c.validate(args); err != nil {
		return c.usage(err)
	}

	w, err := bundle.AcquireWriter(ctx, c.Bundle)
	if err != nil {
		if bundle.WriterBusy(err) {
			return c.fail(root.ReasonWriterBusy, err)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	defer w.Release()

	if c.Response != "" {
		return c.file(w)
	}
	return c.emit(w)
}

// validate settles the flag combination before the lock is taken, so a mistake in the
// invocation is reported as one rather than after a wait behind somebody's ingest.
func (c *Config) validate(args []string) error {
	if len(args) != 0 {
		return errors.New("critic takes no arguments; use --path to critique one" +
			" document or --sample to draw from the corpus")
	}
	if c.Response != "" {
		if strings.TrimSpace(c.Key) == "" {
			return errors.New("--response needs the --key it answers;" +
				" `gnosis critic` prints the key for each prompt")
		}
		return nil
	}
	if strings.TrimSpace(c.Key) != "" {
		return errors.New("--key names a reply to file and needs --response")
	}
	if strings.TrimSpace(c.Model) == "" {
		// Not defaulted, for `ingest`'s reason: a default model would put a value
		// nobody chose into every cache key, and the first person to change it would
		// invalidate the whole cache without having decided to.
		return errors.New("--model is required; it is part of the cache key")
	}
	if c.Path != "" && c.SampleN != 0 {
		return errors.New("--path critiques one document's claims and --sample draws" +
			" from the corpus; they answer different questions")
	}
	if c.SampleN < 0 {
		return fmt.Errorf("--sample must be positive, got %d", c.SampleN)
	}
	return nil
}

// emit writes the prompts and reports what was drawn.
func (c *Config) emit(w *bundle.Writer) error {
	out, err := w.CriticPrompts(&bundle.CriticOptions{
		Model:   relay.Model{Name: c.Model, Version: c.ModelVersion},
		Path:    c.Path,
		SampleN: c.SampleN,
		Warn:    c.Stderr,
	})
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	if c.JSONL {
		if eErr := c.EmitOK(out); eErr != nil {
			return fmt.Errorf("critic: %w", eErr)
		}
		return nil
	}
	for i := range out.Prompts {
		p := &out.Prompts[i]
		if p.Cached {
			_, _ = fmt.Fprintf(c.Stdout, "%s\tcached\n", p.Key)
			continue
		}
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\n", p.Key, p.Path)
	}
	c.summarise(out)
	return nil
}

// summarise says what the draw covered, on stderr with the counts.
//
// **The population is always printed and the skipped count only when there is one**,
// and the asymmetry is deliberate. A sample with no population beside it is a number a
// reader cannot place — five of what? — while "0 claims cite no archived source" on
// every clean run is the line a reader learns to skip, and its absence is what makes
// its presence mean something.
func (c *Config) summarise(out *bundle.Critiqued) {
	_, _ = fmt.Fprintf(c.Stderr, "%d prompt(s) from %d critiquable claim(s)",
		len(out.Prompts), out.Population)
	if out.Seed != 0 {
		_, _ = fmt.Fprintf(c.Stderr, ", seed %d", out.Seed)
	}
	_, _ = fmt.Fprintln(c.Stderr)
	if out.Skipped > 0 {
		_, _ = fmt.Fprintf(c.Stderr,
			"%d claim(s) cite no archived source and cannot be critiqued against"+
				" one; `gnosis fetch` then `gnosis ingest` is what gives them one\n",
			out.Skipped)
	}
}

// file records a reply and reports the verdict.
func (c *Config) file(w *bundle.Writer) error {
	reply, err := c.readResponse()
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	verdict, err := w.FileCritique(c.Key, reply, c.Stderr)
	if err != nil {
		if errs.ErrorCode(err) == errs.EINVALID {
			// The agent replied badly, which is a finding about the reply rather
			// than a broken tool: it can be told exactly what to fix and asked again.
			return c.fail(root.ReasonUnparsable, err)
		}
		return c.fail(root.ReasonNeedsHuman, err)
	}
	return c.report(verdict)
}

// readResponse reads the reply from a file or from stdin.
//
// `-` is the stdin spelling `admit` already uses, and it is a flag value rather than a
// bare dash for the reason recorded there: `ff` consumes a bare `-` as the end-of-flags
// terminator.
func (c *Config) readResponse() ([]byte, error) {
	if c.Response == "-" {
		body, err := io.ReadAll(c.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading the reply from stdin: %w", err)
		}
		return body, nil
	}
	body, err := os.ReadFile(c.Response)
	if err != nil {
		return nil, fmt.Errorf("reading the reply: %w", err)
	}
	return body, nil
}

// report renders a filed verdict.
//
// **It exits 0 whatever the critic found**, which is §10.5's rule and not an oversight:
// a verdict is advisory, and a command that failed on one would make a critic's
// willingness to declare a gap expensive — at which point it stops declaring them.
func (c *Config) report(verdict *bundle.Verdict) error {
	if c.JSONL {
		if err := c.EmitOK(verdict); err != nil {
			return fmt.Errorf("critic: %w", err)
		}
		return nil
	}
	for _, f := range verdict.Findings {
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\t%s\t%s\n",
			f.Severity, f.Category, f.Path, f.Message)
	}
	// The path beside the reference, because §5.6 makes the presented path a view and
	// a reader handed a UUID has to go and find the page. The reference is what the
	// ledger is keyed on and survives a retitle; the path is what a person navigates.
	_, _ = fmt.Fprintf(c.Stderr, "%d finding(s) on %s (%s)\n",
		len(verdict.Findings), verdict.Path, verdict.ClaimRef)
	writeAngles(c.Stderr, "examined", verdict.Examined)
	writeGaps(c.Stderr, verdict.NotExamined)
	if verdict.Moved {
		_, _ = fmt.Fprintf(c.Stderr,
			"note: the document changed after the prompt was emitted, so this"+
				" verdict is about text the corpus no longer holds\n")
	}
	return nil
}

// writeAngles renders what the critic examined, or nothing when it examined nothing it
// could name.
//
// The angles are printed rather than counted, because the whole point of the block is
// that "three areas examined" tells a reader nothing about whether theirs was one.
func writeAngles(w io.Writer, label string, angles []string) {
	if len(angles) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "%s:\n", label)
	for _, angle := range angles {
		_, _ = fmt.Fprintf(w, "  - %s\n", angle)
	}
}

// writeGaps renders what the critic did not examine, with the reason for each.
//
// **Both halves, because the reason is the half a reader acts on.** "The source's
// methodology" says nothing about whether the gap can be closed; "this excerpt does not
// include it" says a better excerpt would close it, and "I ran out of context" says
// another critique would. Printing the aspect alone would leave a reader with a list of
// things nobody looked at and no way to tell which are worth another prompt.
func writeGaps(w io.Writer, gaps []finding.Unexamined) {
	if len(gaps) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "not examined:\n")
	for _, gap := range gaps {
		_, _ = fmt.Fprintf(w, "  - %s — %s\n", gap.Aspect, gap.Reason)
	}
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("critic: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("critic: %w", c.Usage(cause))
}
