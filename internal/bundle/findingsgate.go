package bundle

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/finding"
)

// Gated is a findings gate's verdict (§8.4).
type Gated struct {
	// Blocking are the error-severity findings, sorted. Only these stop anything:
	// §16.1's severity model is shared across the family and warning does not block.
	Blocking []finding.Diagnostic `json:"blocking"`

	// Warnings is how many non-blocking findings came with them, so a passing gate
	// does not read as an empty report.
	Warnings int `json:"warnings"`

	// SemanticReview is which of §17.1's two acts this verdict rests on.
	//
	// **§17.1 requires the field**, and the reason is the overclaim it prevents: a
	// structural pass means the corpus is internally honest, not that anybody agrees
	// with it, and reporting one as "verified" is exactly the claim that section
	// exists to refuse.
	SemanticReview gnosis.SemanticReview `json:"semantic_review"`

	// Unexamined are the aspects the findings' producer declared it did not look at
	// (§16.1's `finding.Unexamined`).
	//
	// Carried through rather than summarised: a gate that dropped them would ship on
	// exactly the silence they exist to break, which is the failure §10.5 records for
	// a critic's coverage block one layer over.
	Unexamined []finding.Unexamined `json:"unexamined,omitempty"`

	// SelfTested reports that the blocking decision demonstrated it can still refuse.
	//
	// It is a field rather than an assumption because a gate that cannot fail is a
	// green light of unknown provenance — the argument §9.3's scan and §4.3's archive
	// gates already turn on — and a caller reading a pass needs to see that the proof
	// ran on this invocation rather than in a test once.
	SelfTested bool `json:"self_tested"`
}

// Blocks reports whether this verdict stops the caller.
//
// Requires: nothing.
// Ensures: true when any finding is error-severity, **or when the self-test did not
// pass**. Pure.
//
// The second clause is the fail-closed half and it is the one worth stating: a gate that
// could not prove it still refuses has not established that its silence means anything,
// so it refuses instead. That is `ReasonUnscanned`'s direction in §9.3, applied one
// layer up.
func (g *Gated) Blocks() bool {
	return !g.SelfTested || len(g.Blocking) > 0
}

// GateFindings judges a findings file (§8.4).
//
// Requires: path names a file holding a `finding.Result` or a gnosis envelope carrying
// one; bundleDir is a bundle root, which need not exist — a caller gating another tool's
// findings may have no corpus to hand.
// Ensures: the blocking findings sorted, the count of warnings beside them, §17.1's
// semantic-review state, and a self-test result computed on this call. EINVALID naming
// **both** accepted shapes when neither parses, because a caller handed the wrong
// wrapper would otherwise go and check their JSON syntax.
//
// It reads the coverage ledger to tell "no critic finding" from "no critic ran", which
// is why it takes a bundle at all. A bundle it cannot read yields `unknown` rather than
// `structural-only`: claiming a structural pass was all that happened is a claim, and an
// unreadable ledger is not evidence for it.
func GateFindings(bundleDir, path string) (*Gated, error) {
	const op = "bundle.GateFindings"

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	result, err := parseFindings(op, raw)
	if err != nil {
		return nil, err
	}

	blocking, warnings, categories := splitBySeverity(result)
	out := &Gated{
		Blocking: blocking, Warnings: warnings,
		Unexamined: result.Unexamined, SelfTested: selfTest(),
	}

	critiques, readErr := LoadCritiques(bundleDir)
	out.SemanticReview = gnosis.FoldSemanticReview(
		categories, len(critiques), readErr == nil)
	return out, nil
}

// parseFindings reads either shape the family produces.
//
// Requires: raw is the file's bytes.
// Ensures: EINVALID naming both shapes when neither matches. Pure.
//
// **Two shapes, and both are real.** `finding.Result` is what §16.1 makes the family's
// wire format, so `canonizer gate` and this one read each other's output. gnosis's own
// commands wrap it in §8.0's envelope, and a gate that could not read the tool it ships
// with would be a gate nobody uses. Anything else is refused rather than guessed at: a
// findings gate that inferred structure would be deciding what to block on from a shape
// it did not recognise.
func parseFindings(op string, raw []byte) (*finding.Result, error) {
	var result finding.Result
	if err := json.Unmarshal(raw, &result); err == nil && result.Diagnostics != nil {
		return &result, nil
	}

	var envelope struct {
		Data struct {
			Diagnostics []finding.Diagnostic `json:"diagnostics"`
			Unexamined  []finding.Unexamined `json:"unexamined"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil &&
		envelope.Data.Diagnostics != nil {
		return &finding.Result{
			Diagnostics: envelope.Data.Diagnostics,
			Unexamined:  envelope.Data.Unexamined,
		}, nil
	}
	return nil, &errs.Error{
		Code: errs.EINVALID,
		Message: op + ": the file is neither a finding.Result (a JSON object with a" +
			" `diagnostics` array) nor a gnosis envelope (one with `data.diagnostics`)." +
			" `gnosis lint --jsonl` writes the second",
	}
}

// splitBySeverity separates the blocking findings from the rest and collects their
// categories.
//
// **Not `classify`**, which this package already uses for picking a fetch adapter — the
// compiler caught the collision, which is the rule against one name for dissimilar
// things being enforced rather than remembered.
//
// Requires: result came from parseFindings.
// Ensures: the blocking findings sorted, so two runs over one file render identically;
// the categories in file order, which is all FoldSemanticReview needs. Pure.
//
// It is a function rather than three lines inside GateFindings **so the self-test can
// run the real decision.** The first version of that test asked
// `finding.SeverityError.Blocking()` and passed — which proves skillet's method works
// and says nothing about whether this gate uses it. `govet` found it by noticing the
// planted finding's other fields were never read: the test was theatre, and the dead
// fields were the evidence.
func splitBySeverity(
	result *finding.Result,
) (blocking []finding.Diagnostic, warnings int, categories []string) {
	categories = make([]string, 0, len(result.Diagnostics))
	for i := range result.Diagnostics {
		d := &result.Diagnostics[i]
		categories = append(categories, d.Category)
		if d.Severity.Blocking() {
			blocking = append(blocking, *d)
			continue
		}
		warnings++
	}
	finding.Sort(blocking)
	return blocking, warnings, categories
}

// selfTest proves this gate's blocking decision can still refuse.
//
// Requires: nothing.
// Ensures: true only when the real classifier puts a planted error-severity finding in
// the blocking set and leaves a warning out of it. Pure.
//
// §8.4 says the gate "runs its self-test", and the promote gate's precedent (§4.3) is
// what that means: a gate proves it can fail **on every invocation**, not in a test
// somebody ran once. The distinction is not ceremony — a gate whose classifier was
// broken by a refactor would pass every corpus silently, and the only observable
// difference between that and a healthy corpus is this check.
//
// **It runs `splitBySeverity`, which is the code the real findings go through.** A self-test
// over a private copy of the rule, or over the severity predicate alone, would prove
// something other than what ships — which is what the first version of this did.
func selfTest() bool {
	blocking, warnings, _ := splitBySeverity(&finding.Result{Diagnostics: []finding.Diagnostic{
		{
			Severity: finding.SeverityError,
			Category: "gate:self-test",
			Message:  "planted, so a pass means the gate can still refuse",
		},
		{
			Severity: finding.SeverityWarning,
			Category: "gate:self-test",
			Message:  "planted, so a pass means a warning still does not block",
		},
	}})
	return len(blocking) == 1 && warnings == 1
}

// GateReason names why a gate blocked, for the envelope.
//
// Requires: g came from GateFindings.
// Ensures: a non-empty reason whenever Blocks reports true. Pure.
//
// The self-test failure is named separately from a blocking finding because the two send
// a caller to opposite places: one is a problem with the corpus and the other is a
// problem with gnosis, and reporting "3 blocking findings" for the second would send
// somebody to read a corpus that is fine.
func GateReason(g *Gated) string {
	switch {
	case !g.SelfTested:
		return "the gate could not demonstrate that it still refuses a blocking " +
			"finding, so its silence would mean nothing; this is a defect in gnosis " +
			"rather than in the corpus"
	case len(g.Blocking) > 0:
		return lint.Noun(len(g.Blocking), "blocking finding") + "; " +
			strings.ToLower(g.SemanticReview.String()) + " review"
	default:
		return ""
	}
}
