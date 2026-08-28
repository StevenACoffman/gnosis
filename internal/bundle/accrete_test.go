package bundle_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/okf"
	"github.com/StevenACoffman/gnosis/internal/relay"
)

const accretedID = gnosis.ID("0192a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b")

// accretionFixture is a document with one claim carrying one quotation.
func accretionFixture(t *testing.T) *okf.Document {
	t.Helper()
	const src = `---
type: Rule
title: "Retry Budget"
gnosis_id: 0192a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b
gnosis_schema_version: 1
sources:
  - resource: "https://one.example/doc"
gnosis_claims:
  - id: c1
    anchor: "The retry budget is three attempts."
    gnosis_evidence:
      - "retries are capped at three"
    archive_paths:
      - evidence/text/one.md
---

# Retry Budget

The retry budget is three attempts.

`
	doc, err := okf.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return doc
}

// TestAccretionNeverChangesTheBody is §6.3's whole distinction, and the reason the
// invariant is checked rather than promised.
//
// Accretion appends evidence "with no body rewrite". A reply claim the document does
// not already make has no paragraph, so adding it would change the body — which is
// `synthesize`'s gated operation under the cheaper name. Those claims are reported and
// never applied.
func TestAccretionNeverChangesTheBody(t *testing.T) {
	t.Parallel()
	doc := accretionFixture(t)
	before := doc.Body

	reply := &relay.Reply{
		Type: "Rule", Title: "Retry Budget", SourceURI: "https://two.example/doc",
		Claims: []relay.Claim{
			// Matches the existing claim: a new quotation for it.
			{
				Text:         "The retry budget is three attempts.",
				Quotes:       []string{"three attempts, no more"},
				ArchivePaths: []string{"evidence/text/two.md"},
			},
			// Matches nothing: must be reported, not added.
			{
				Text:   "Backoff is exponential.",
				Quotes: []string{"backoff doubles"},
			},
		},
	}

	got, err := bundle.Accrete(doc, accretedID, reply)
	if err != nil {
		t.Fatalf("accrete: %v", err)
	}
	if got.Added != 1 {
		t.Errorf("added = %d, want 1", got.Added)
	}
	if len(got.Unmatched) != 1 || !strings.Contains(got.Unmatched[0], "Backoff") {
		t.Errorf("the unmatched claim was not reported: %v", got.Unmatched)
	}

	after, err := okf.Parse(got.Content)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if after.Body != before {
		t.Errorf("the body changed:\n--- before ---\n%q\n--- after ---\n%q",
			before, after.Body)
	}
	// The new quotation landed, and the old one survived.
	for _, want := range []string{"retries are capped at three", "three attempts, no more"} {
		if !strings.Contains(string(got.Content), want) {
			t.Errorf("the document lost or never gained %q:\n%s", want, got.Content)
		}
	}
	// And the second source was recorded, because §6.3 says sources update too.
	if !strings.Contains(string(got.Content), "two.example") {
		t.Errorf("the new source was not recorded:\n%s", got.Content)
	}
}

// TestAQuotationIsNotAppendedTwice keeps a re-run from growing the evidence list. The
// comparison is fold-normalised for the same reason the evidence invariant is: a
// passage re-offered with different whitespace is the same passage.
func TestAQuotationIsNotAppendedTwice(t *testing.T) {
	t.Parallel()
	reply := &relay.Reply{
		Type: "Rule", Title: "Retry Budget", SourceURI: "https://one.example/doc",
		Claims: []relay.Claim{{
			Text: "The retry budget is three attempts.",
			// Same quotation, reflowed.
			Quotes: []string{"retries   are capped\nat three"},
		}},
	}
	got, err := bundle.Accrete(accretionFixture(t), accretedID, reply)
	if err != nil {
		t.Fatalf("accrete: %v", err)
	}
	if got.Added != 0 {
		t.Errorf("a quotation already present was appended again (added = %d)", got.Added)
	}
	if len(got.Content) != 0 {
		t.Error("a no-op accretion produced a document to write")
	}
}

// TestAnAccretionThatAddsNothingWritesNothing is the idempotence a second ingest of the
// same source depends on: it must be a no-op rather than a fresh commit.
func TestAnAccretionThatAddsNothingWritesNothing(t *testing.T) {
	t.Parallel()
	reply := &relay.Reply{
		Type: "Rule", Title: "Retry Budget",
		Claims: []relay.Claim{{Text: "Nothing this document says.", Quotes: []string{"x"}}},
	}
	got, err := bundle.Accrete(accretionFixture(t), accretedID, reply)
	if err != nil {
		t.Fatalf("accrete: %v", err)
	}
	if got.Added != 0 || len(got.Content) != 0 {
		t.Errorf("an accretion matching nothing produced work: %+v", got)
	}
	if len(got.Unmatched) != 1 {
		t.Errorf("the unmatched claim was not reported: %v", got.Unmatched)
	}
}
