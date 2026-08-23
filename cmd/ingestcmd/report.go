package ingestcmd

import (
	"fmt"
	"strconv"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// Result is the payload.
type Result struct {
	Prompts []bundle.Pending `json:"prompts"`

	// Emitted and Cached are counted separately because a run in which every
	// prompt was already answered did no work, and reporting "12 prompts" would
	// claim it did.
	Emitted int `json:"emitted"`
	Cached  int `json:"cached"`
}

// report renders the outcome.
//
// Under --cache-only a prompt with no cached reply is a **finding**, not an error:
// the corpus is in a state the caller asked about and the tool worked correctly.
// A CI job branches on exactly that difference, and returning an error would make
// "this needs new model calls" indistinguishable from "gnosis is broken".
func (c *Config) report(pending []bundle.Pending) error {
	result := Result{Prompts: pending}
	for i := range pending {
		if pending[i].Cached {
			result.Cached++
		} else {
			result.Emitted++
		}
	}

	if c.CacheOnly && result.Emitted > 0 {
		return c.reportMisses(&result)
	}
	if c.JSONL {
		if err := c.EmitOK(&result); err != nil {
			return fmt.Errorf("ingest: %w", err)
		}
		return nil
	}
	c.writeHuman(&result)
	return nil
}

// reportMisses is the --cache-only refusal, listing what is missing so a caller
// can act without a second run.
func (c *Config) reportMisses(result *Result) error {
	message := strconv.Itoa(result.Emitted) + " prompts have no cached reply"

	if c.JSONL {
		if err := c.EmitFindings(root.ReasonNeedsHuman, message, result); err != nil {
			return fmt.Errorf("ingest: %w", err)
		}
		return root.ExitError(root.CodeFindings)
	}
	_, _ = fmt.Fprintf(c.Stderr, "%s:\n", message)
	for i := range result.Prompts {
		if !result.Prompts[i].Cached {
			_, _ = fmt.Fprintf(c.Stderr, "  %s  %s\n",
				result.Prompts[i].Key[:12], result.Prompts[i].URI)
		}
	}
	return root.ExitError(root.CodeFindings)
}

// writeHuman renders for a person, naming the file to answer rather than only the
// key: a reader whose next action is to open a prompt should not have to construct
// the path.
func (c *Config) writeHuman(result *Result) {
	for i := range result.Prompts {
		p := &result.Prompts[i]
		if p.Cached {
			_, _ = fmt.Fprintf(c.Stdout, "cached   %s\n", p.URI)
			continue
		}
		_, _ = fmt.Fprintf(c.Stdout, "emitted  %s\n         %s\n", p.URI, p.Path)
	}
	_, _ = fmt.Fprintf(c.Stdout, "\n%d emitted, %d already answered\n",
		result.Emitted, result.Cached)
	if result.Emitted > 0 {
		_, _ = fmt.Fprintf(c.Stdout,
			"\nAnswer each prompt, then: gnosis admit --key <key> <reply-file>\n")
	}
}
