package promotecmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// needsConfirmation reports whether the coordinator asked for a human.
//
// Requires: outcome came from a promote command.
// Ensures: true only for a blocked needs_human outcome. A refusal is not a
// confirmation prompt in disguise, and reading one as the other is the mistake
// that would turn this whole path into a `--yes`.
func needsConfirmation(outcome gnosis.Outcome) bool {
	return outcome.Status == gnosis.StatusBlocked &&
		outcome.Reason == gnosis.ReasonNeedsHuman
}

// confirmAndRetry asks the person to type the path, then re-runs the promotion.
//
// Requires: the previous outcome was needs_human and --apply was given.
// Ensures: with --jsonl, no prompt and the original outcome unchanged. Otherwise
// the promotion is re-executed with the typed phrase, and its own outcome is
// returned whatever it says — including a second refusal, because the coordinator
// re-runs the gate and is the only thing entitled to decide.
//
// **The gate is re-evaluated on the retry rather than reused.** That costs a
// second evaluation and buys the property §9.4 is about: the bytes approved are
// the bytes written, and a human staring at a prompt is exactly the window in
// which a file changes underneath one. A confirmation is authorisation to promote
// *this document*, not a token that makes an earlier verdict reusable.
func (c *Config) confirmAndRetry(
	ctx context.Context, coordinator *bundle.Coordinator,
	cmd *command.Promote, blocked gnosis.Outcome,
) (gnosis.Outcome, error) {
	if c.JSONL {
		// A machine caller cannot type a phrase, and prompting on a pipe hangs.
		// The envelope already carries what is required; returning it unchanged is
		// the whole behaviour.
		return blocked, nil
	}

	_, _ = fmt.Fprintf(c.Stderr, "%s\n\n", blocked.Message)
	_, _ = fmt.Fprintf(c.Stderr,
		"To promote it anyway, type the document's path exactly:\n  %s\n> ", cmd.Path)

	typed, err := readPhrase(c.Stdin)
	if err != nil {
		return gnosis.Outcome{}, c.fail(root.ReasonNeedsHuman, err)
	}
	if typed != cmd.Path {
		// Not an error: declining to confirm is a legitimate answer, and the most
		// common one when somebody reads what they were about to do.
		_, _ = fmt.Fprintln(c.Stderr, "not confirmed; nothing was written")
		return blocked, nil
	}

	cmd.Confirmation = typed
	outcome, err := coordinator.Execute(ctx, cmd)
	if err != nil {
		return gnosis.Outcome{}, c.fail(root.ReasonNeedsHuman, err)
	}
	return outcome, nil
}

// readPhrase reads one line, trimming only the line ending and surrounding
// spaces.
//
// It does not lower-case, expand, or otherwise normalise. A path is
// case-sensitive on the filesystems that matter, and a confirmation that accepted
// a near-miss would be confirming a document the person did not name.
//
// End of input is not an error: a promotion run with no terminal attached and no
// piped answer is unconfirmed, which is the same as declining.
func readPhrase(stdin io.Reader) (string, error) {
	if stdin == nil {
		return "", nil
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read the confirmation: %w", err)
	}
	return strings.TrimSpace(line), nil
}
