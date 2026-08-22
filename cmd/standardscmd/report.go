package standardscmd

import (
	"fmt"
	"strconv"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// Result is the payload.
type Result struct {
	Since      string            `json:"since"`
	Loosenings []bundle.Loosened `json:"loosenings"`
	Logged     bool              `json:"logged"`
}

// report renders the outcome.
//
// A loosening is a **finding**, not an error and not a pass. The tool worked and
// the corpus is in a state a person should look at, which is exactly the
// distinction §17 insists on and the one a CI job branches on. It is also not
// blocking: §6.2 asks that a loosening be recorded, never that it be prevented,
// and a gate here would make the tool refuse a threshold change somebody made
// deliberately.
func (c *Config) report(found []bundle.Loosened) error {
	result := Result{Since: c.Since, Loosenings: found, Logged: c.Log && len(found) > 0}

	if len(found) == 0 {
		if c.JSONL {
			if err := c.EmitOK(&result); err != nil {
				return fmt.Errorf("standards check: %w", err)
			}
			return nil
		}
		_, _ = fmt.Fprintf(c.Stdout, "nothing loosened since %s\n", c.Since)
		return nil
	}

	message := strconv.Itoa(len(found)) + " threshold(s) loosened since " + c.Since
	if c.JSONL {
		if err := c.EmitFindings(root.ReasonNeedsHuman, message, &result); err != nil {
			return fmt.Errorf("standards check: %w", err)
		}
		return root.ExitError(root.CodeFindings)
	}

	_, _ = fmt.Fprintf(c.Stderr, "%s:\n", message)
	for i := range found {
		_, _ = fmt.Fprintf(c.Stderr, "  %s\n", found[i].LogEntry())
	}
	if result.Logged {
		_, _ = fmt.Fprintf(c.Stdout, "\nfiled in %s\n", logFile)
	} else {
		_, _ = fmt.Fprintf(c.Stderr,
			"\nrecord these in %s (SPEC §6.2), or re-run with --log\n", logFile)
	}
	return root.ExitError(root.CodeFindings)
}
