// Package filecmd implements the "file" CLI command.
package filecmd

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
	"github.com/StevenACoffman/skillet/errs"
)

// longHelp is the command's prose, extracted from New for `misscmd`'s reason.
const longHelp = `File a good answer back into the corpus as a concept (SPEC §8.3).

Karpathy's point is that good answers are valuable and should not disappear into chat
history. gnosis's condition is that a synthesized answer is exactly as capable of being
wrong as an ingested source, so this lands the draft in quarantine and stops. What makes
it part of the corpus is ` + "`gnosis promote`" + `, the same gate every ingested
document passes.

The draft's evidence is the evidence of the claims the answer cited: their quotations
and their archived sources travel onto it. That is what makes it checkable by the same
machinery — and it means a filed answer cannot introduce a quotation nobody archived,
which is what stops this being a route for unsourced content into a sourced corpus.

A reply that declined to answer is refused rather than filed. "The corpus does not say"
is a real answer and it is not a concept; filing it would put a statement of absence
where the next reader retrieves it as a claim.

Takes the lock, because filing a draft writes one.`

// Config holds the configuration for the file command.
type Config struct {
	*root.Config

	// Key names the ask prompt this answers.
	Key string

	// Response is the file holding the reply, or `-` for stdin.
	Response string

	// Type is the OKF type the draft declares. Empty takes the default, which is the
	// one that prescribes nothing.
	Type string

	// By is who is filing, for the trail.
	By string

	// Flags and Command are this command's own — see proofcmd for why declaring them
	// is not boilerplate.
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the file command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("file").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Key, 0, "key", "", "the ask prompt this reply answers")
	cfg.Flags.StringVar(&cfg.Response, 0, "response", "",
		"file holding the reply, or - for stdin")
	cfg.Flags.StringVar(&cfg.Type, 0, "type", "",
		"the OKF type the draft declares; the default prescribes nothing")
	cfg.Flags.StringVar(&cfg.By, 0, "by", "", "who is filing, for the audit trail")
	cfg.Command = &ff.Command{
		Name:      "file",
		Usage:     "gnosis file --key K --response FILE [--type T] [--by ACTOR]",
		ShortHelp: "file an answer back as a quarantined concept",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: take the lock, parse, quarantine.
func (c *Config) exec(ctx context.Context, args []string) error {
	if err := c.validate(args); err != nil {
		return c.usage(err)
	}
	reply, err := c.readResponse()
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}

	w, err := bundle.AcquireWriter(ctx, c.Bundle)
	if err != nil {
		if bundle.WriterBusy(err) {
			return c.fail(root.ReasonWriterBusy, err)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	defer w.Release()

	filed, err := w.FileAnswer(&bundle.FileOptions{
		Key: c.Key, Reply: reply, Type: c.Type, Actor: c.By, Warn: c.Stderr,
	})
	if err != nil {
		if errs.ErrorCode(err) == errs.EINVALID {
			// The agent replied badly, which is a finding about the reply rather than
			// a broken tool: it can be told exactly what to fix and asked again.
			return c.fail(root.ReasonUnparsable, err)
		}
		return c.fail(root.ReasonNeedsHuman, err)
	}
	return c.report(filed)
}

// validate settles the invocation before the lock is taken.
func (c *Config) validate(args []string) error {
	if len(args) != 0 {
		return errors.New("file takes no arguments; --key and --response carry both" +
			" values, because ff ends flag parsing at the first positional")
	}
	if strings.TrimSpace(c.Key) == "" {
		return errors.New("--key is required; `gnosis ask` prints the key for the" +
			" prompt this answers")
	}
	if strings.TrimSpace(c.Response) == "" {
		return errors.New("--response is required; pass a file or - for stdin")
	}
	return nil
}

// readResponse reads the reply from a file or from stdin.
//
// `-` is the stdin spelling `admit` and `critic` already use, and it is a flag value
// rather than a bare dash for the reason recorded there: `ff` consumes a bare `-` as the
// end-of-flags terminator.
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

// report renders what was filed.
//
// The next command is named, because a draft in quarantine that nobody promotes is
// indistinguishable from one nobody filed — and this is the point where the caller has
// the context to decide.
func (c *Config) report(filed *bundle.Filed) error {
	if c.JSONL {
		if err := c.EmitOK(filed); err != nil {
			return fmt.Errorf("file: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(c.Stdout, "%s\n", filed.Path)
	_, _ = fmt.Fprintf(c.Stderr,
		"%q drafted from %d cited claim(s); it is in quarantine until"+
			" `gnosis promote %s` passes the gate\n",
		filed.Title, len(filed.Cites), filed.Path)
	if filed.Unanswered != "" {
		// Carried onto the terminal because it is the half of the answer that says
		// what it does not cover, and a draft filed without anybody reading it is one
		// whose limits nobody knows.
		_, _ = fmt.Fprintf(c.Stderr, "the answer did not cover: %s\n", filed.Unanswered)
	}
	return nil
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("file: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("file: %w", c.Usage(cause))
}
