package fetchcmd

import (
	"fmt"

	"github.com/StevenACoffman/gnosis/internal/archive"
)

// Source is what became of one fetched source.
type Source struct {
	URI          string               `json:"uri"`
	Disposition  archive.Disposition  `json:"disposition"`
	ArchivePath  string               `json:"archive_path,omitempty"`
	RecordPath   string               `json:"record_path"`
	RejectReason archive.RejectReason `json:"reject_reason,omitempty"`

	// Wrote reports whether anything reached the disk. It is false for a source
	// already in the archive, and false for every source under --dry-run.
	Wrote bool `json:"wrote"`
}

// Result is the payload.
//
// Wrote and Unchanged are counted separately because a staleness sweep over a
// settled corpus writes nothing, and a run that reported "500 sources fetched"
// would be reporting work that did not happen (§9.2).
type Result struct {
	Sources   []Source `json:"sources"`
	Wrote     int      `json:"wrote"`
	Unchanged int      `json:"unchanged"`

	// Durable and Weak split the sources by whether a quotation can still be
	// checked offline. It is surfaced here rather than left to be counted,
	// because `referenced` is a supported outcome and therefore an easy one to
	// stop noticing.
	Durable int `json:"durable"`
	Weak    int `json:"weak"`
}

// add records one source's outcome and updates the counts.
func (r *Result) add(s *Source) {
	r.Sources = append(r.Sources, *s)
	if s.Wrote {
		r.Wrote++
	} else {
		r.Unchanged++
	}
	if s.Disposition.Durable() {
		r.Durable++
	} else {
		r.Weak++
	}
}

// report renders the outcome.
//
// A weak source is never a finding. §4.3 admits `referenced` deliberately, and
// exiting non-zero on it would push real knowledge out of the corpus to protect a
// property those claims were never going to have. The count is reported so the
// weakness is visible; §14.4 is where it is weighed, against how load-bearing the
// claims that rest on it turn out to be.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
		return nil
	}

	for _, s := range result.Sources {
		_, _ = fmt.Fprintf(c.Stdout, "%s  %s\n", s.Disposition, s.URI)
		if s.RejectReason != archive.ReasonNone {
			_, _ = fmt.Fprintf(c.Stdout, "    not archived: %s\n", s.RejectReason)
		}
		if s.ArchivePath != "" {
			_, _ = fmt.Fprintf(c.Stdout, "    %s\n", s.ArchivePath)
		}
	}

	verb := "wrote"
	if c.DryRun {
		verb = "would write"
	}
	_, _ = fmt.Fprintf(c.Stdout, "\n%d sources: %s %d, unchanged %d\n",
		len(result.Sources), verb, writeCount(result, c.DryRun), result.Unchanged)
	_, _ = fmt.Fprintf(c.Stdout, "%d durable, %d weak\n", result.Durable, result.Weak)
	return nil
}

// writeCount reports what a dry run would have written, which is every source
// whose record is not already present — the same question, asked before the fact.
func writeCount(result *Result, dryRun bool) int {
	if !dryRun {
		return result.Wrote
	}
	return len(result.Sources)
}
