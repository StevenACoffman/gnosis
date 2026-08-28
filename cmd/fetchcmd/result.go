package fetchcmd

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// Source is what became of one fetched source.
type Source struct {
	URI string `json:"uri"`

	// SourceSHA256 is the fetched bytes' hash, carried so the caller can record
	// that it looked at this *version* — a check against a URI alone would let an
	// old observation vouch for new bytes.
	SourceSHA256 string `json:"source_sha256"`

	Disposition  archive.Disposition  `json:"disposition"`
	ArchivePath  string               `json:"archive_path,omitempty"`
	RecordPath   string               `json:"record_path"`
	RejectReason archive.RejectReason `json:"reject_reason,omitempty"`

	// Wrote reports whether anything reached the disk. It is false for a source
	// already in the archive, and false for every source under --dry-run.
	Wrote bool `json:"wrote"`

	// Revision is where the source stood in its own history when it was read —
	// today a git commit, and empty for every other adapter.
	//
	// **Reported here and stored nowhere**, which is what the entry behind it asked
	// for. A reader had no way to learn which revision a git-sourced record came
	// from and nothing said so; now the fetch says so, at the one moment the
	// information exists. Putting it on the record would make a record's identity
	// depend on the repository's activity rather than on the file's, which §4.3.1
	// forbids — so if it matters for a claim, it belongs in `log.md`, written by the
	// person who decided it mattered.
	Revision string `json:"revision,omitempty"`

	// Findings are every §9.3 finding in the source, populated only when the scan
	// is why it was refused.
	//
	// `RejectReason` is one reason because a disposition is one reason, and a source
	// carrying three hidden-character classes and a leaked key reports whichever
	// outranks. That is right for the record and useless to whoever has to fix the
	// source, which is what this is for.
	//
	// It is not gated behind a flag. The set is bounded by construction — one entry
	// per character class and one per matching rule — so there is nothing long enough
	// to need one, and a knob nobody needs is what §6.5 is about.
	Findings []string `json:"findings,omitempty"`
}

// Drifted is one recorded source measured against what was just fetched.
//
// It is a separate row from Source because the two answer different questions about
// the same fetch: a Source says what became of the bytes, and this says what the new
// bytes did to the claims resting on the old ones. Folding the state onto Source
// would also make it look like every fetch has one, and only a --recheck does.
type Drifted struct {
	URI string `json:"uri"`

	// SourceSHA256 is the recorded version this verdict is about, which is what
	// distinguishes two rows for one URI — a source fetched twice has two versions
	// and each is compared on its own.
	SourceSHA256 string `json:"source_sha256"`

	ArchivePath string            `json:"archive_path,omitempty"`
	State       gnosis.DriftState `json:"state"`

	// Missing are the passages the new bytes no longer contain, set only for
	// drift-unsupported.
	Missing []string `json:"missing,omitempty"`

	// Findings is one entry per claim that lost a passage (§14.3.2).
	Findings []finding.Diagnostic `json:"findings,omitempty"`

	// Resting is how many claims quote this source version. Zero explains a
	// drift-unchecked that means "there was nothing to check" rather than "the
	// check could not be made".
	Resting int `json:"resting"`
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

	// Drift is empty for a fetch, and one row per recorded source version for a
	// --recheck.
	Drift []Drifted `json:"drift,omitempty"`

	// Unsupported counts the sources that withdrew support, which is the only
	// drift state that asks anything of anybody. It is counted rather than left to
	// be derived because it decides this command's exit status, and a caller
	// branching on a count it has to compute from a list is a caller that can
	// compute it differently.
	Unsupported int `json:"unsupported"`

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

// addDrift records what a re-check established about one recorded source.
//
// Requires: rc came from bundle.Recheck.
// Ensures: one row appended, and Unsupported advanced only for the state that asks
// something of somebody — which is `Actionable`'s question, asked in one place so no
// reporter can answer it differently.
func (r *Result) addDrift(rc *bundle.Rechecked) {
	r.Drift = append(r.Drift, Drifted{
		URI:          rc.URI,
		SourceSHA256: rc.SourceSHA256,
		ArchivePath:  rc.ArchivePath,
		State:        rc.Drift.State,
		Missing:      rc.Drift.Missing,
		Findings:     rc.Findings,
		Resting:      rc.Resting,
	})
	if rc.Drift.State.Actionable() {
		r.Unsupported++
	}
}

// reportDrift renders what a --recheck found, for a person.
//
// Every state that says something is printed, including the ones that are not
// findings. §14.3.2's first consequence forbids rendering `drift-benign` as a
// *warning*, not reporting it at all: somebody who just ran a re-check over two
// hundred sources needs to see that it happened, and a command that printed only the
// failures would be indistinguishable from one that did not run.
//
// **The versions nothing rests on are counted rather than listed**, and running the
// command is what showed why. A re-check that finds changed bytes archives them, so
// the next re-check sees a second record for that URI whose text no claim cites yet;
// after ten re-checks a settled source contributes nine lines that mean "nothing
// happened" and one that matters. They are still in the machine envelope, where a
// consumer can have every row — this is the human report, and a report is allowed to
// summarise what a reader cannot act on.
//
// The archive path is printed under each row because the URI does not identify one:
// two versions of the same source are two rows with the same URI, and without the
// path a reader cannot tell which version a state is about.
func (c *Config) reportDrift(result *Result) {
	if len(result.Drift) == 0 {
		return
	}
	_, _ = fmt.Fprintf(c.Stdout, "\nre-checked %d recorded source(s):\n",
		len(result.Drift))

	unquoted := 0
	for i := range result.Drift {
		d := &result.Drift[i]
		if d.Resting == 0 {
			unquoted++
			continue
		}
		_, _ = fmt.Fprintf(c.Stdout, "  %s  %s\n", d.State, d.URI)
		if d.ArchivePath != "" {
			_, _ = fmt.Fprintf(c.Stdout, "    %s\n", d.ArchivePath)
		}
		for _, f := range d.Findings {
			_, _ = fmt.Fprintf(c.Stderr, "    %s: %s\n", f.Path, f.Message)
		}
	}
	if unquoted > 0 {
		_, _ = fmt.Fprintf(c.Stdout,
			"  %d version(s) no claim quotes: nothing to check\n", unquoted)
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
		return c.emit(result)
	}

	// Indexed rather than ranged by value: adding Findings pushed Source past
	// gocritic's 128-byte threshold, so a value range now copies each one. Same
	// finding the index package already took, and the same fix.
	for i := range result.Sources {
		s := &result.Sources[i]
		_, _ = fmt.Fprintf(c.Stdout, "%s  %s\n", s.Disposition, s.URI)
		if s.RejectReason != archive.ReasonNone {
			_, _ = fmt.Fprintf(c.Stdout, "    not archived: %s\n", s.RejectReason)
		}
		// Every finding, not only the reason. The reason says what became of the
		// source; these say what is in it, which is what somebody fixing it needs.
		for _, f := range s.Findings {
			_, _ = fmt.Fprintf(c.Stdout, "      %s\n", f)
		}
		if s.ArchivePath != "" {
			_, _ = fmt.Fprintf(c.Stdout, "    %s\n", s.ArchivePath)
		}
		// Printed next to the source rather than summarised, because a fetch of a
		// repository yields one line per file and the revision is the same for all
		// of them: a reader scanning for it finds it beside whichever file they
		// care about.
		if s.Revision != "" {
			_, _ = fmt.Fprintf(c.Stdout,
				"    revision %s (not recorded; note it in log.md if it matters)\n",
				s.Revision)
		}
	}

	verb := "wrote"
	if c.DryRun {
		verb = "would write"
	}
	_, _ = fmt.Fprintf(c.Stdout, "\n%d sources: %s %d, unchanged %d\n",
		len(result.Sources), verb, writeCount(result, c.DryRun), result.Unchanged)
	_, _ = fmt.Fprintf(c.Stdout, "%d durable, %d weak\n", result.Durable, result.Weak)
	c.reportDrift(result)
	if result.Unsupported == 0 {
		return nil
	}
	_, _ = fmt.Fprintf(c.Stderr, "\n%s\n", withdrawnMessage(result))
	return root.ExitError(root.CodeFindings)
}

// emit writes the machine envelope.
//
// A re-check that found withdrawn support is `findings` rather than `ok`, and the
// distinction is §17's: the examination completed and it found something. It is not
// `blocked`, because the fetch itself proceeded — the sources were read and the
// records written — and reporting the command as unable to continue would misdescribe
// what happened to a caller deciding whether to retry.
//
// The reason is `needs_human` because §14.3.2 says so in as many words: support
// withdrawn upstream "should reach a person", and there is no fix a tool can apply.
func (c *Config) emit(result *Result) error {
	if result.Unsupported == 0 {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
		return nil
	}
	err := c.EmitFindings(root.ReasonNeedsHuman, withdrawnMessage(result), result)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	return root.ExitError(root.CodeFindings)
}

// withdrawnMessage is the one sentence both output forms use, so the human reading
// stderr and the agent reading the envelope are told the same thing.
func withdrawnMessage(result *Result) string {
	return strconv.Itoa(result.Unsupported) +
		" source(s) no longer contain a passage this corpus quotes; " +
		"the archived copies are intact, so review the claims rather than re-fetching"
}

// writeCount reports what a dry run would have written, which is every source
// whose record is not already present — the same question, asked before the fact.
func writeCount(result *Result, dryRun bool) int {
	if !dryRun {
		return result.Wrote
	}
	return len(result.Sources)
}

// auditPaths are the record paths this run wrote, for the audit row.
//
// Requires: r came from fetchAll.
// Ensures: sorted, and only the paths that actually reached the disk — a row
// listing a record that was already there would claim work that did not happen,
// which is the same reporting error §9.2 keeps `Wrote` and `Unchanged` apart for.
// Pure.
func (r *Result) auditPaths() []string {
	out := make([]string, 0, len(r.Sources))
	for i := range r.Sources {
		if s := &r.Sources[i]; s.Wrote {
			out = append(out, s.RecordPath)
		}
	}
	sort.Strings(out)
	return out
}

// auditDetail is one sentence about what this run did, for a person reading the
// trail.
//
// Requires: r came from fetchAll.
// Ensures: the counts already on the Result, rendered. Pure.
//
// It is a method rather than a string assembled at the call site because every
// number it needs is already counted here, and the last pass removed a second
// accumulator from `exec` for exactly that reason.
func (r *Result) auditDetail() string {
	return strconv.Itoa(len(r.Sources)) + " source(s): " +
		strconv.Itoa(r.Wrote) + " written, " +
		strconv.Itoa(r.Unchanged) + " already present, " +
		strconv.Itoa(r.Durable) + " durable, " +
		strconv.Itoa(r.Weak) + " weak"
}
