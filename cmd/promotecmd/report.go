package promotecmd

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/gate"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// report renders the outcome and returns the matching exit status.
func (c *Config) report(outcome gnosis.Outcome) error {
	if c.JSONL {
		if err := c.EmitOutcome(outcome); err != nil {
			return fmt.Errorf("promote: %w", err)
		}
		return exitFor(outcome)
	}
	if outcome.Status == gnosis.StatusOK {
		c.writePromoted(outcome)
		return nil
	}
	_, _ = fmt.Fprintf(c.Stderr, "%s: %s\n", outcome.Reason, outcome.Message)
	c.writeSignals(outcome)
	return exitFor(outcome)
}

// exitFor turns an envelope into an exit status, or nil for success.
func exitFor(outcome gnosis.Outcome) error {
	if outcome.Code == root.CodeOK {
		return nil
	}
	return root.ExitError(outcome.Code)
}

// writePromoted reports a successful promotion, naming the debt when there is
// one.
//
// A promotion carried over unrun signals prints them, because the person who
// authorised it should see what they took on written back to them rather than
// only in a file they will not open.
func (c *Config) writePromoted(outcome gnosis.Outcome) {
	data, _ := outcome.Data.(map[string]any)
	path, _ := data["path"].(string)

	if wrote, _ := data["wrote"].(bool); !wrote {
		_, _ = fmt.Fprintf(c.Stdout, "%s: would be promoted\n", path)
		_, _ = fmt.Fprintf(c.Stderr, "\nPreview only. Re-run with --apply to write it.\n")
		return
	}
	_, _ = fmt.Fprintf(c.Stdout, "promoted %s\n", path)

	if carried, ok := data["carried"].([]string); ok && len(carried) > 0 {
		_, _ = fmt.Fprintf(c.Stderr,
			"\nCarried over %d signal(s) that could not run: %s\n"+
				"Recorded in the audit trail; re-examine when those checks land.\n",
			len(carried), strings.Join(carried, ", "))
	}
}

// writeSignals prints what the gate withheld on, keeping the two kinds apart.
//
// The separation is the point. A failure is something the author fixes; an
// unchecked signal is something this build cannot do, and printing them in one
// list would send somebody looking for a defect in a document that has none.
func (c *Config) writeSignals(outcome gnosis.Outcome) {
	data, _ := outcome.Data.(map[string]any)
	writeNamed(c, "failed", data["failed"])
	writeNamed(c, "could not run", data["unchecked"])
}

// writeNamed prints one labelled signal list, whatever concrete slice type the
// envelope carried it as.
//
// The type switch exists because the same field arrives as []gate.Signal from a
// direct call and as []any after a JSON round trip, and a renderer handling only
// one of them would print "[conflict security]" as a single blob — or nothing at
// all. Both were true of the first draft.
func writeNamed(c *Config, label string, v any) {
	var names []string
	switch got := v.(type) {
	case []gate.Signal:
		for _, s := range got {
			names = append(names, string(s))
		}
	case []string:
		names = got
	case []any:
		for _, e := range got {
			names = append(names, fmt.Sprint(e))
		}
	case nil:
		return
	default:
		names = []string{fmt.Sprint(got)}
	}
	if len(names) == 0 {
		return
	}
	_, _ = fmt.Fprintf(c.Stderr, "  %s: %s\n", label, strings.Join(names, ", "))
}
