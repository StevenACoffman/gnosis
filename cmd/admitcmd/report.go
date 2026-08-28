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

// writeAccepted says what the reply became, which is not one thing.
//
// **A reply lands three ways and the first version knew about one.** It reported every
// accepted reply as "quarantined <path>; review it, then promote it", read off the
// presence of a path — so an accretion and then a rewrite of an existing concept both
// told their caller to promote a document already in the corpus. The advice was not
// merely wrong; it named a command that would refuse.
//
// Each branch is chosen by a key only that outcome carries, rather than by a kind field
// the coordinator would have to remember to set. The data a report needs and the data
// that identifies it are the same data.
func writeAccepted(c *Config, outcome gnosis.Outcome) {
	data, _ := outcome.Data.(map[string]any)
	path, hasPath := data["path"].(string)
	added, accreted := countOf(data, "added")
	diff, rewritten := data["diff"].(string)

	switch {
	case accreted && hasPath:
		writeAccretion(c, path, added, data)
	case rewritten && hasPath:
		_, _ = fmt.Fprintf(c.Stdout, "rewrote %s\n", path)
		if diff != "" {
			_, _ = fmt.Fprintf(c.Stderr, "\n%s", diff)
		}
		_, _ = fmt.Fprintf(c.Stderr,
			"every quotation it already held survived; `git diff` shows the prose\n")
	case hasPath:
		_, _ = fmt.Fprintf(c.Stdout, "quarantined %s\n", path)
		_, _ = fmt.Fprintf(c.Stdout, "\nReview it, then: gnosis promote %s\n", path)
	default:
		_, _ = fmt.Fprintf(c.Stdout, "the reply checks out; nothing was written\n")
	}
}

// countOf reads an integer the envelope may carry as either Go type.
//
// An in-process outcome holds it as an int; a decoded envelope makes it a float,
// because JSON has one number type. Both mean the same thing and a report that
// understood only one would be correct in tests and wrong through the CLI, or the
// reverse.
func countOf(data map[string]any, key string) (int, bool) {
	if f, ok := data[key].(float64); ok {
		return int(f), true
	}
	n, ok := data[key].(int)
	return n, ok
}

// writeAccretion reports evidence appended to a concept that already exists.
//
// The unmatched claims are named rather than counted, because the remedy differs per
// claim: one may be a paraphrase of something the document already says, and another
// may be knowledge the document does not hold — which `gnosis synthesize` is for, and
// accretion deliberately will not do (§6.3).
func writeAccretion(c *Config, path string, added int, data map[string]any) {
	if added == 0 {
		_, _ = fmt.Fprintf(c.Stdout,
			"%s already holds every quotation this reply offered\n", path)
	} else {
		_, _ = fmt.Fprintf(c.Stdout, "%s gained %d quotation(s)\n", path, added)
	}
	unmatched, _ := data["unmatched"].([]string)
	if len(unmatched) == 0 {
		return
	}
	_, _ = fmt.Fprintf(c.Stderr,
		"\n%d claim(s) match nothing this document says, so no evidence was attached"+
			" — accretion adds evidence and never a claim (§6.3):\n", len(unmatched))
	for _, u := range unmatched {
		_, _ = fmt.Fprintf(c.Stderr, "  %s\n", u)
	}
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
