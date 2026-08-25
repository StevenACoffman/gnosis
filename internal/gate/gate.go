// Package gate implements SPEC §9.5's promote gate: the deterministic signals
// that decide whether a quarantined document may enter the authoritative corpus.
//
// The gate approves a **diff**, not a document. Between checking a candidate and
// committing it there is a window, and a corpus whose gate can be raced is a
// corpus whose gate is decorative (§9.4). So a Candidate carries the exact bytes
// that will be written, the coordinator computes them once, and re-reading the
// file between the verdict and the write is a defect rather than an optimisation.
//
// Everything here is pure. The archived text a quotation is checked against, the
// titles already in the corpus, and the sources tier 0 holds all arrive as values
// in a Corpus, gathered by the shell — the same division `archive.Gates` and
// `lint.Snapshot` use, and the reason these signals are testable from literals.
//
// # Two signals cannot run yet, and the gate says so
//
// `conflict` needs §10's adjudication, which is Phase 3, and `security` can run
// only the scan stages that exist. Both report VerdictUnchecked, which withholds
// automatic approval — a gate that approved writes on evidence it never examined
// would leave a caller reading "approved" with no way to learn what was skipped.
//
// What that does *not* mean, any more, is that nothing can be promoted. An earlier
// version of this comment said exactly that and called it correct; it was half of
// an answer. A permanently red gate with no sanctioned way through it is a trap,
// and the way through is Decide: an unchecked signal routes a candidate to
// DecisionNeedsHuman, where a named person may promote it by confirming, on the
// record, with the unchecked signals written into the audit row. A *failed* signal
// routes to DecisionRefused, which nobody may override. See decision.go.
//
// # The gate proves it can fail, every time
//
// SelfTest plants a defect for each implemented signal and requires the signal to
// reject it, and pairs it with a control the signal must accept. Evaluate runs the
// whole battery on every invocation and refuses to gate if any pair misbehaves. A
// gate nobody has proven can fail is not a gate — and one proven at build time is
// not proven in the binary somebody is actually running.
package gate

import "sort"

// Corpus is what the signals need to know about everything outside the candidate.
//
// It is a value rather than an interface because the signals are pure functions
// and there is nothing here to substitute. Assembling one is the shell's job.
type Corpus struct {
	// ArchivedText maps an archive path (evidence/text/…) to that file's text.
	// The evidence signal checks quotations against these and nothing else: a
	// quote validated against the live upstream is a proof that expires (§4.1).
	ArchivedText map[string]string

	// FetchedURIs is every source URI tier 0 holds a record for, whatever the
	// disposition. A `referenced` source is still provenance — the hash and the
	// URI were recorded — which is why this is not restricted to archived ones.
	FetchedURIs map[string]bool

	// TitlesByFold maps a title normalised by gnosis.Surface.Fold — whitespace,
	// typography, and case — to the paths of documents already carrying it. The
	// shell must use that same normalizer building this map; a mismatch would
	// make the duplication signal silently find nothing. Empty for a corpus with
	// no documents, which is the state a first promotion runs against.
	TitlesByFold map[string][]string
}

// Limits are the tunable gate thresholds, projected from `standards/`.
//
// As with archive.Gates, this package states what it needs rather than importing
// the loader: adapters do not import each other, and the shell joins them.
type Limits struct {
	// HedgingMax is how many softening phrases a body may carry before the
	// hedging signal fails it.
	HedgingMax int

	// MinPassageWords is the shortest run of words a quotation is checked at.
	// Below it, quotecheck reports Unchecked rather than Found.
	MinPassageWords int

	// PerFileCap and EmbeddedPayloadCap are §9.3 stage 4's bounds, from
	// `standards/archive.toml`. Zero disables the stage, which is what a caller
	// that has not loaded standards holds.
	//
	// They are the *archive's* declared caps applied to a candidate document, and
	// that is deliberate rather than convenient. §9.3 stage 4 wants a bound "with
	// the bound in `standards/`"; the archive already has one, justified for prose;
	// and inventing a second number for the candidate is what §6.5 exists to
	// prevent. Two artifacts, one declared threshold.
	//
	// They live on Limits rather than being threaded separately because the
	// `security` signal needs them to say whether stage 4 ran, which makes them a
	// threshold the gate depends on — stated as data exactly as the two above are.
	PerFileCap         int64
	EmbeddedPayloadCap int64
}

// Result is one signal's conclusion.
type Result struct {
	Signal  Signal  `json:"signal"`
	Verdict Verdict `json:"verdict"`

	// Detail says what was examined and what was concluded, in a sentence a
	// person can act on. It is populated for a pass as well as a failure: "no
	// quotations to check" and "3 of 3 quotations found" are different passes,
	// and a reader deciding whether to trust the verdict needs to know which.
	Detail string `json:"detail"`
}

// Report is the gate's whole answer.
type Report struct {
	// Path is the candidate's destination in the bundle.
	Path string `json:"path"`

	// Results is one entry per signal, ordered by signal name so two runs over
	// one candidate produce comparable output.
	Results []Result `json:"results"`

	// Control is the planted-defect self-test. When it did not hold, no verdict
	// below means anything, and Approved is false whatever they say.
	Control ControlReport `json:"control"`
}

// Evaluate runs every signal over a candidate.
//
// Requires: c is non-nil and c.After is the exact bytes that would be written.
// corpus and limits are populated by the shell.
// Ensures: one Result per signal, ordered by signal name; the self-test runs
// first and its outcome is carried on the report. Pure — the same inputs yield
// the same report, which is what lets a preview and an apply be one computation
// (§9.4).
//
// A failing control short-circuits the signals rather than running them. Running
// checks whose controls just failed would produce verdicts nobody should read,
// and reporting them beside the control failure invites acting on them.
func Evaluate(c *Candidate, corpus *Corpus, limits Limits) Report {
	report := Report{Path: c.Path, Control: SelfTest()}
	if !report.Control.Held {
		return report
	}

	report.Results = []Result{
		evidence(c, corpus, limits),
		provenance(c, corpus),
		conformance(c),
		duplication(c, corpus),
		hedging(c, limits),
		conflict(),
		security(c),
	}
	sort.Slice(report.Results, func(i, j int) bool {
		return report.Results[i].Signal < report.Results[j].Signal
	})
	return report
}

// Withheld returns the signals that did not approve, split by why.
//
// Requires: nothing.
// Ensures: both slices are sorted and either may be empty. They are returned
// separately because the two call for opposite responses — a failure is something
// the author must fix, and an unchecked signal is something this build cannot do.
// Collapsing them would send an author looking for a defect in their document
// that is not there.
func (r *Report) Withheld() (failed, unchecked []Signal) {
	for _, res := range r.Results {
		switch res.Verdict {
		case VerdictFail:
			failed = append(failed, res.Signal)
		case VerdictPass:
		case VerdictUnchecked:
			unchecked = append(unchecked, res.Signal)
		default:
			// An out-of-range verdict is not a pass, and grouping it with the
			// unchecked ones keeps Approved and Withheld from disagreeing.
			unchecked = append(unchecked, res.Signal)
		}
	}
	sortSignals(failed)
	sortSignals(unchecked)
	return failed, unchecked
}

func sortSignals(s []Signal) {
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
}
