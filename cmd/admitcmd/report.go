package admitcmd

import (
	"fmt"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// report relays the coordinator's envelope.
//
// It is a relay rather than a re-render: the coordinator already produced §8.0's
// envelope, and taking it apart to rebuild it here would give two places where the
// status and the exit code could disagree.
func (c *Config) report(outcome gnosis.Outcome) error {
	if c.JSONL {
		if err := c.EmitOutcome(outcome); err != nil {
			return fmt.Errorf("admit: %w", err)
		}
		return exitFor(outcome)
	}

	if outcome.Status == root.StatusOK {
		writeAccepted(c, outcome)
		return nil
	}
	_, _ = fmt.Fprintf(c.Stderr, "%s: %s\n", outcome.Reason, outcome.Message)
	writeRefusals(c, outcome)
	return exitFor(outcome)
}

// exitFor turns an envelope into an exit status, or nil for success.
func exitFor(outcome gnosis.Outcome) error {
	if outcome.Code == root.CodeOK {
		return nil
	}
	return root.ExitError(outcome.Code)
}

func writeAccepted(c *Config, outcome gnosis.Outcome) {
	data, _ := outcome.Data.(map[string]any)
	if path, ok := data["path"].(string); ok {
		_, _ = fmt.Fprintf(c.Stdout, "quarantined %s\n", path)
		_, _ = fmt.Fprintf(c.Stdout, "\nReview it, then: gnosis promote %s\n", path)
		return
	}
	_, _ = fmt.Fprintf(c.Stdout, "the reply checks out; nothing was written\n")
}

// writeRefusals lists the two failure kinds under their own headings, because the
// responses differ: an unsupported claim needs the agent to quote accurately, and
// an unchecked one needs a longer quotation or an archived source to check against.
func writeRefusals(c *Config, outcome gnosis.Outcome) {
	data, _ := outcome.Data.(map[string]any)
	writeList(c, "not found in the archive", data["unsupported"])
	writeList(c, "could not be checked at all", data["unchecked"])
}

func writeList(c *Config, heading string, raw any) {
	items, _ := raw.([]string)
	if len(items) == 0 {
		return
	}
	_, _ = fmt.Fprintf(c.Stderr, "\n%s:\n", heading)
	for _, item := range items {
		_, _ = fmt.Fprintf(c.Stderr, "  %s\n", item)
	}
}
