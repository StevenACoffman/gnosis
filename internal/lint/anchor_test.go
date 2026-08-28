package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
)

// anchored builds a one-document corpus whose body is fixed and whose claims vary.
func anchored(body string, claims ...lint.Claim) *lint.Snapshot {
	return &lint.Snapshot{
		Documents: []lint.Document{{
			ID: idA, Path: "c/a.md", Type: "Reference", Title: "A",
			Body: body, Claims: claims,
		}},
	}
}

// categoriesOf collects the categories a report emitted, for an assertion that
// reads as one comparison.
func categoriesOf(report lint.Report) []string {
	out := make([]string, 0, len(report.Diagnostics))
	for _, d := range report.Diagnostics {
		out = append(out, d.Category)
	}
	return out
}

// TestAResolvingAnchorIsSilent, or the check reports every claim in every document
// and nobody reads it again.
func TestAResolvingAnchorIsSilent(t *testing.T) {
	t.Parallel()

	snap := anchored("The cache is cleared on restart and holds nothing.",
		lint.Claim{ID: "claim-1", Anchor: "The cache is cleared on restart"})
	report := lint.Run(snap, lint.Checks(testNow()))

	for _, got := range categoriesOf(report) {
		if strings.HasPrefix(got, "anchor-") {
			t.Errorf("a resolving anchor produced %q: %+v", got, report.Diagnostics)
		}
	}
}

// TestAnAbsentAnchorIsReported. §5.5.1 puts the address in the document so a rebuild
// can recover it, and an anchor that is not in the body recovers nothing.
func TestAnAbsentAnchorIsReported(t *testing.T) {
	t.Parallel()

	snap := anchored("The cache is cleared on restart.",
		lint.Claim{ID: "claim-1", Anchor: "A sentence nobody ever wrote"})
	report := lint.Run(snap, lint.Checks(testNow()))

	var found bool
	for _, d := range report.Diagnostics {
		if d.Category != "anchor-absent" {
			continue
		}
		found = true
		if !strings.Contains(d.Message, "claim-1") {
			t.Errorf("the finding does not name the claim: %q", d.Message)
		}
		if d.Path != "c/a.md" {
			t.Errorf("path = %q", d.Path)
		}
		// The limitation is stated in the finding, because a reader deciding what to
		// do needs to know the check cannot tell them which of the causes it is.
		if !strings.Contains(d.Message, "not yet possible") {
			t.Errorf("the finding overclaims what it knows: %q", d.Message)
		}
	}
	if !found {
		t.Errorf("an absent anchor was not reported: %v", categoriesOf(report))
	}
}

// TestAnchorComparisonIsFoldNormalised. §5.5.1 specifies a fold-normalised anchor,
// so a paragraph a formatter reflowed must not read as a lost address.
func TestAnchorComparisonIsFoldNormalised(t *testing.T) {
	t.Parallel()

	// Two spaces in the body, one in the anchor, and a typographic apostrophe on
	// one side only — all differences textnorm.Fold is supposed to absorb.
	snap := anchored("The cache’s contents  are cleared on restart.",
		lint.Claim{ID: "claim-1", Anchor: "The cache's contents are cleared on restart"})
	report := lint.Run(snap, lint.Checks(testNow()))

	for _, d := range report.Diagnostics {
		if d.Category == "anchor-absent" {
			t.Errorf("a reflowed anchor was reported as absent: %q", d.Message)
		}
	}
}

// TestAnchorComparisonKeepsCase. This is the opposite choice from the duplication
// signal's, and deliberately: an anchor locates a **quotation**, where case carries
// meaning, and a title is a name, where it does not. The gate's own self-test is
// what found that they are different questions.
func TestAnchorComparisonKeepsCase(t *testing.T) {
	t.Parallel()

	snap := anchored("The cache is cleared on restart.",
		lint.Claim{ID: "claim-1", Anchor: "THE CACHE IS CLEARED ON RESTART"})
	report := lint.Run(snap, lint.Checks(testNow()))

	var found bool
	for _, d := range report.Diagnostics {
		if d.Category == "anchor-absent" {
			found = true
		}
	}
	if !found {
		t.Error("a case-shifted anchor resolved; case is meaning in a quotation")
	}
}

// TestTwoClaimsOnOnePassageAreReported is the case that arrives without anybody
// making a mistake.
//
// Claim ids are UUIDv7 so they never collide, but two people adding claims to one
// document can independently anchor different ids to the same text, and git merges
// that cleanly — the §4.6.1 shape one level down. Nothing else in the corpus reports
// it, which is why it is worth a finding at all.
func TestTwoClaimsOnOnePassageAreReported(t *testing.T) {
	t.Parallel()

	sentence := "The cache is cleared on restart"
	snap := anchored(sentence+" and holds nothing.",
		lint.Claim{ID: "claim-b", Anchor: sentence},
		lint.Claim{ID: "claim-a", Anchor: sentence})
	report := lint.Run(snap, lint.Checks(testNow()))

	var found int
	for _, d := range report.Diagnostics {
		if d.Category != "anchor-collision" {
			continue
		}
		found++
		// One finding per group, not per claim: three claims on one passage is one
		// problem, and the message must name all of them.
		for _, id := range []string{"claim-a", "claim-b"} {
			if !strings.Contains(d.Message, id) {
				t.Errorf("the finding does not name %s: %q", id, d.Message)
			}
		}
		// Sorted, so the message does not reorder itself between runs.
		if strings.Index(d.Message, "claim-a") > strings.Index(d.Message, "claim-b") {
			t.Errorf("the claim ids are not sorted: %q", d.Message)
		}
	}
	if found != 1 {
		t.Errorf("got %d collision findings, want one per group: %v",
			found, categoriesOf(report))
	}
}

// TestACollisionIsFoldNormalisedToo. Two people writing the same sentence with
// different whitespace have still anchored to one passage, and an exact comparison
// would miss precisely the merge case this exists for.
func TestACollisionIsFoldNormalisedToo(t *testing.T) {
	t.Parallel()

	snap := anchored("The cache is cleared on restart and holds nothing.",
		lint.Claim{ID: "claim-1", Anchor: "The cache is cleared on restart"},
		lint.Claim{ID: "claim-2", Anchor: "The  cache is cleared on  restart"})
	report := lint.Run(snap, lint.Checks(testNow()))

	var found bool
	for _, d := range report.Diagnostics {
		if d.Category == "anchor-collision" {
			found = true
		}
	}
	if !found {
		t.Error("two whitespace-differing anchors on one passage were not reported")
	}
}

// TestCollisionsAreWithinADocument. Two documents quoting one sentence is ordinary
// and says nothing about either; only the scope of the comparison tells them apart.
func TestCollisionsAreWithinADocument(t *testing.T) {
	t.Parallel()

	sentence := "The cache is cleared on restart"
	snap := &lint.Snapshot{Documents: []lint.Document{
		{
			ID: idA, Path: "c/a.md", Type: "Reference", Body: sentence + ".",
			Claims: []lint.Claim{{ID: "claim-1", Anchor: sentence}},
		},
		{
			ID: idB, Path: "c/b.md", Type: "Reference", Body: sentence + ".",
			Claims: []lint.Claim{{ID: "claim-2", Anchor: sentence}},
		},
	}}
	report := lint.Run(snap, lint.Checks(testNow()))

	for _, d := range report.Diagnostics {
		if d.Category == "anchor-collision" {
			t.Errorf("two documents quoting one sentence was reported: %q", d.Message)
		}
	}
}

// TestTheCheckSkipsACorpusWithNoAnchors. Derived applicability, per §12: most Phase
// 2 documents are written by hand and declare no claims, and reporting nothing found
// is different from reporting there was nothing to look for.
func TestTheCheckSkipsACorpusWithNoAnchors(t *testing.T) {
	t.Parallel()

	// A claim with archive paths and no anchor: the archive-path check applies and
	// this one does not.
	snap := anchored("Prose with no claims addressed in it.",
		lint.Claim{ID: "claim-1", ArchivePaths: []string{"evidence/text/aa/aa.md"}})
	report := lint.Run(snap, lint.Checks(testNow()))

	var skipped bool
	for _, s := range report.Skipped {
		if s.Check == "claim-anchor" {
			skipped = true
			if s.Reason == "" {
				t.Error("claim-anchor skipped with no reason")
			}
		}
	}
	if !skipped {
		t.Errorf("the check ran on a corpus with no anchors: %+v", report.Skipped)
	}
}

// TestAClaimWithNoAnchorIsNotReportedHere. A claim declaring no address at all is a
// conformance question, not an address that stopped resolving, and reporting it here
// would put two different failures under one category.
func TestAClaimWithNoAnchorIsNotReportedHere(t *testing.T) {
	t.Parallel()

	snap := anchored("The cache is cleared on restart.",
		lint.Claim{ID: "claim-1", Anchor: "The cache is cleared on restart"},
		lint.Claim{ID: "claim-2"})
	report := lint.Run(snap, lint.Checks(testNow()))

	for _, d := range report.Diagnostics {
		if strings.HasPrefix(d.Category, "anchor-") {
			t.Errorf("an anchorless claim produced %q: %q", d.Category, d.Message)
		}
	}
}

// TestTwoAnchorlessClaimsDoNotCollide. Empty anchors are not one passage, and
// grouping them would report every pair of hand-written claims in the corpus.
func TestTwoAnchorlessClaimsDoNotCollide(t *testing.T) {
	t.Parallel()

	snap := anchored("Prose.",
		lint.Claim{ID: "claim-1", Anchor: "Prose"},
		lint.Claim{ID: "claim-2"},
		lint.Claim{ID: "claim-3"})
	report := lint.Run(snap, lint.Checks(testNow()))

	for _, d := range report.Diagnostics {
		if d.Category == "anchor-collision" {
			t.Errorf("anchorless claims were grouped: %q", d.Message)
		}
	}
}

// TestTheCheckIsDeterministic. The collision grouping comes out of a map, and a
// report that reordered itself between runs over one corpus would be unusable.
func TestTheCheckIsDeterministic(t *testing.T) {
	t.Parallel()

	snap := anchored("Alpha sentence. Beta sentence.",
		lint.Claim{ID: "c1", Anchor: "Alpha sentence"},
		lint.Claim{ID: "c2", Anchor: "Alpha sentence"},
		lint.Claim{ID: "c3", Anchor: "Beta sentence"},
		lint.Claim{ID: "c4", Anchor: "Beta sentence"})

	first := strings.Join(messagesOf(lint.Run(snap, lint.Checks(testNow()))), "|")
	for range 8 {
		got := strings.Join(messagesOf(lint.Run(snap, lint.Checks(testNow()))), "|")
		if got != first {
			t.Fatalf("two runs differ:\n%s\n%s", first, got)
		}
	}
}

// messagesOf is the report flattened for a determinism comparison.
func messagesOf(report lint.Report) []string {
	out := make([]string, 0, len(report.Diagnostics))
	for _, d := range report.Diagnostics {
		out = append(out, d.Category+":"+d.Message)
	}
	return out
}

// TestTheZeroClaimAnchorsNothing, so a claim nobody populated cannot be reported as
// having lost an address it never had. Same discipline as every other zero value
// here.
func TestTheZeroClaimAnchorsNothing(t *testing.T) {
	t.Parallel()

	var claim lint.Claim
	if claim.Anchor != "" {
		t.Error("the zero Claim carries an anchor")
	}
	if _, err := gnosis.ParseID(claim.ID); err == nil {
		t.Error("the zero Claim's id parses as an identifier")
	}
}
