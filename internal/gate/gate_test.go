package gate_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gate"
)

const archived = "evidence/text/ab/abcd.md"

// admissible is a candidate that passes every signal that can run. Each test
// spoils exactly one thing, so the reader can see which.
func admissible() gate.Candidate {
	return gate.Candidate{
		Path: "c/01932b7c-cache.md",
		After: []byte("---\ntype: Reference\n---\n" +
			"The cache is cleared on restart and holds nothing across sessions."),
		Doc: gate.Document{
			Type:  "Reference",
			Title: "Cache Lifetime",
			Body:  "The cache is cleared on restart and holds nothing across sessions.",
			Claims: []gate.Claim{{
				ID: "claim-1", Enforced: true,
				Quotes:       []string{"The cache is cleared on restart"},
				ArchivePaths: []string{archived},
			}},
			Sources: []gate.Source{{Resource: "https://example.org/cache.md"}},
		},
	}
}

func corpus() gate.Corpus {
	return gate.Corpus{
		ArchivedText: map[string]string{
			archived: "Documentation. The cache is cleared on restart, and is per-process.",
		},
		FetchedURIs:  map[string]bool{"https://example.org/cache.md": true},
		TitlesByFold: map[string][]string{"another document": {"c/other.md"}},
	}
}

// ptr is here because admissible returns a value and Evaluate takes a pointer;
// gocritic objects to a helper that copies a Candidate, so the copy happens at
// the one call site that needs a fresh one.
// admissiblePtr is a fresh admissible candidate. Each test gets its own so a
// spoiled field cannot leak into the next.
func admissiblePtr() *gate.Candidate {
	c := admissible()
	return &c
}

func limits() gate.Limits { return gate.Limits{HedgingMax: 2, MinPassageWords: 6} }

func evaluate(c *gate.Candidate) gate.Report {
	corp := corpus()
	return gate.Evaluate(c, &corp, limits())
}

// verdictOf finds one signal's verdict in a report.
func verdictOf(t *testing.T, r *gate.Report, s gate.Signal) gate.Result {
	t.Helper()
	for _, res := range r.Results {
		if res.Signal == s {
			return res
		}
	}
	t.Fatalf("no result for signal %q in %+v", s, r.Results)
	return gate.Result{}
}

// TestNothingIsApprovedWhileTwoSignalsCannotRun is the honest state of this
// build, asserted so nobody mistakes it for a defect later. `security` and
// `conflict` have no subsystem to read, they report unchecked, and unchecked
// withholds approval exactly as failure does.
func TestNothingIsApprovedWhileTwoSignalsCannotRun(t *testing.T) {
	t.Parallel()
	report := evaluate(admissiblePtr())

	if report.Approved() {
		t.Fatal("a candidate was approved while two signals could not run")
	}
	failed, unchecked := report.Withheld()
	if len(failed) != 0 {
		t.Errorf("a fully admissible candidate failed signals: %v", failed)
	}
	if len(unchecked) != 2 {
		t.Fatalf("unchecked = %v, want exactly security and conflict", unchecked)
	}
	if unchecked[0] != gate.SignalConflict || unchecked[1] != gate.SignalSecurity {
		t.Errorf("unchecked = %v, want [conflict security]", unchecked)
	}
}

// TestFailedAndUncheckedAreReportedSeparately: the two call for opposite
// responses. A failure is something the author must fix; an unchecked signal is
// something this build cannot do, and collapsing them would send an author
// hunting for a defect that is not in their document.
func TestFailedAndUncheckedAreReportedSeparately(t *testing.T) {
	t.Parallel()
	c := admissible()
	c.Doc.Type = ""

	report := evaluate(&c)
	failed, unchecked := report.Withheld()
	if len(failed) != 1 || failed[0] != gate.SignalConformance {
		t.Errorf("failed = %v, want [conformance]", failed)
	}
	if len(unchecked) != 2 {
		t.Errorf("unchecked = %v, want the two unbuilt signals", unchecked)
	}
}

// TestEverySignalThatCanRunPasses on an admissible candidate. Without this the
// suite could pass with every signal broken, since nothing is approved anyway.
func TestEverySignalThatCanRunPasses(t *testing.T) {
	t.Parallel()
	report := evaluate(admissiblePtr())

	for _, s := range []gate.Signal{
		gate.SignalEvidence, gate.SignalProvenance, gate.SignalConformance,
		gate.SignalDuplication, gate.SignalHedging,
	} {
		if got := verdictOf(t, &report, s); got.Verdict != gate.VerdictPass {
			t.Errorf("%s = %v: %s", s, got.Verdict, got.Detail)
		}
	}
}

func TestSignalsRejectWhatTheyShould(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		spoil  func(*gate.Candidate)
		signal gate.Signal
		detail string
	}{
		"fabricated quotation": {
			func(c *gate.Candidate) {
				c.Doc.Claims[0].Quotes = []string{"a sentence nobody ever wrote down"}
			},
			gate.SignalEvidence, "unsupported",
		},
		"no quotation at all": {
			func(c *gate.Candidate) { c.Doc.Claims[0].Quotes = nil },
			gate.SignalEvidence, "no quotation offered",
		},
		"referenced source only": {
			func(c *gate.Candidate) { c.Doc.Claims[0].ArchivePaths = nil },
			gate.SignalEvidence, "referenced only",
		},
		"archive path missing from tier 0": {
			func(c *gate.Candidate) { c.Doc.Claims[0].ArchivePaths = []string{"evidence/text/zz/gone.md"} },
			gate.SignalEvidence,
			"missing from tier 0",
		},
		"source never fetched": {
			func(c *gate.Candidate) {
				c.Doc.Sources = []gate.Source{{Resource: "https://example.org/unknown"}}
			},
			gate.SignalProvenance, "no fetch record",
		},
		"no sources declared": {
			func(c *gate.Candidate) { c.Doc.Sources = nil },
			gate.SignalProvenance, "no sources declared",
		},
		"no type": {
			func(c *gate.Candidate) { c.Doc.Type = "" },
			gate.SignalConformance, "type",
		},
		"empty body": {
			func(c *gate.Candidate) { c.Doc.Body = "   " },
			gate.SignalConformance, "body",
		},
		"title already taken": {
			func(c *gate.Candidate) { c.Doc.Title = "Another Document" },
			gate.SignalDuplication, "c/other.md",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := admissible()
			tc.spoil(&c)

			report := evaluate(&c)
			got := verdictOf(t, &report, tc.signal)
			if got.Verdict != gate.VerdictFail {
				t.Fatalf("%s = %v, want fail: %s", tc.signal, got.Verdict, got.Detail)
			}
			if !strings.Contains(got.Detail, tc.detail) {
				t.Errorf("detail %q does not mention %q", got.Detail, tc.detail)
			}
		})
	}
}

// TestTitleCaseIsFolded is what makes duplication the merge reconciliation step
// of §4.6.1 rather than a check for literal copy-paste. Two people documenting
// one subject differ in capitalisation as often as not.
func TestTitleCaseIsFolded(t *testing.T) {
	t.Parallel()
	for _, title := range []string{"Another Document", "another document", "ANOTHER  Document"} {
		c := admissible()
		c.Doc.Title = title

		report := evaluate(&c)
		if got := verdictOf(t, &report, gate.SignalDuplication); got.Verdict != gate.VerdictFail {
			t.Errorf("%q was not seen as a duplicate: %s", title, got.Detail)
		}
	}
}

// TestADocumentIsNotADuplicateOfItself: a revision replaces a document at its own
// path, and refusing that would make every correction impossible.
func TestADocumentIsNotADuplicateOfItself(t *testing.T) {
	t.Parallel()
	c := admissible()
	c.Path = "c/other.md"
	c.Before = []byte("the previous version")
	c.Doc.Title = "Another Document"

	report := evaluate(&c)
	if got := verdictOf(t, &report, gate.SignalDuplication); got.Verdict != gate.VerdictPass {
		t.Errorf("a revision was refused as a duplicate: %s", got.Detail)
	}
}

// TestScopeDescriptorSatisfiesProvenance. OKF §5.1 permits a source a consumer
// cannot dereference; such a source has said what it is, which a URI that merely
// happens to be absent from tier 0 has not.
func TestScopeDescriptorSatisfiesProvenance(t *testing.T) {
	t.Parallel()
	c := admissible()
	c.Doc.Sources = []gate.Source{{Resource: "internal incident review, 2026-Q1", Scope: true}}

	report := evaluate(&c)
	if got := verdictOf(t, &report, gate.SignalProvenance); got.Verdict != gate.VerdictPass {
		t.Errorf("a declared scope descriptor was refused: %s", got.Detail)
	}
}

// TestAPassSaysWhatItChecked. "No enforced claims" and "three claims, all
// supported" are different passes, and a reader deciding how much a verdict is
// worth needs to know which one they have.
func TestAPassSaysWhatItChecked(t *testing.T) {
	t.Parallel()
	c := admissible()
	c.Doc.Claims = nil

	report := evaluate(&c)
	got := verdictOf(t, &report, gate.SignalEvidence)
	if got.Verdict != gate.VerdictPass {
		t.Fatalf("a document with no enforced claims failed evidence: %s", got.Detail)
	}
	if !strings.Contains(got.Detail, "no enforced claims") {
		t.Errorf("the detail does not distinguish this from a checked pass: %q", got.Detail)
	}
}

// TestEvaluateIsPure, because a preview and an apply are one computation and must
// not be able to disagree (§9.4).
func TestEvaluateIsPure(t *testing.T) {
	t.Parallel()
	first, err := json.Marshal(evaluate(admissiblePtr()))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for range 20 {
		again, merr := json.Marshal(evaluate(admissiblePtr()))
		if merr != nil {
			t.Fatalf("marshal: %v", merr)
		}
		if !bytes.Equal(again, first) {
			t.Fatalf("two evaluations differ:\n%s\n%s", first, again)
		}
	}
}

// TestResultsAreSorted, so two runs produce comparable output and a diff of two
// reports means something.
func TestResultsAreSorted(t *testing.T) {
	t.Parallel()
	report := evaluate(admissiblePtr())
	if len(report.Results) != 7 {
		t.Fatalf("got %d results, want one per signal", len(report.Results))
	}
	for i := 1; i < len(report.Results); i++ {
		if report.Results[i-1].Signal > report.Results[i].Signal {
			t.Errorf("out of order at %d: %q then %q",
				i, report.Results[i-1].Signal, report.Results[i].Signal)
		}
	}
}

// TestVerdictsEncodeAsWords: an agent branches on "unchecked", not on the integer
// 0, whose meaning depends on declaration order.
func TestVerdictsEncodeAsWords(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(evaluate(admissiblePtr()))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"verdict":"pass"`, `"verdict":"unchecked"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("encoding does not contain %s:\n%s", want, b)
		}
	}
}

// TestZeroReportIsNotApproved: a report nobody populated must not read as
// permission to write.
func TestZeroReportIsNotApproved(t *testing.T) {
	t.Parallel()
	var r gate.Report
	if r.Approved() {
		t.Error("the zero Report approves")
	}
	if gate.VerdictUnchecked.Approves() {
		t.Error("the zero Verdict approves")
	}
	if gate.Verdict(99).Approves() {
		t.Error("an out-of-range verdict approves")
	}
}
