package bundle_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// TestAReflowedAnchorHasNoOffsetRatherThanAWrongOne is the adversarial case, and it is
// the one that would be silently wrong rather than visibly broken.
//
// `claim-anchor` matches an anchor under textnorm.Fold, which collapses whitespace runs
// — so an anchor separated from the body only by a reflowed line still *resolves*. Its
// offset does not: a position computed in fold space is a position in a string that is
// not on disk, and a reader sent to that byte has no way to tell they were misdirected.
// §5.5.2 defines NULL for exactly this, and NULL is what must come out.
func TestAReflowedAnchorHasNoOffsetRatherThanAWrongOne(t *testing.T) {
	t.Parallel()

	const body = "Intro line.\n\nThe retry budget is\nthree attempts.\n"
	docs := []bundle.Document{{
		ID: gnosis.ID("01J0000000000000000000000A"), Path: "c/a.md",
		Type: "Rule", Body: body,
		Claims: []bundle.DocClaim{
			// Differs from the body by a line break only: folds equal, bytes do not.
			{ID: "c1", Anchor: "The retry budget is three attempts."},
			{ID: "c2", Anchor: "Intro line."},
		},
	}}

	rows := bundle.ClaimRows(docs)
	if len(rows) != 2 {
		t.Fatalf("want two rows, got %d", len(rows))
	}
	byID := map[string]int{}
	for i := range rows {
		byID[rows[i].ID] = i
	}

	reflowed := rows[byID["c1"]]
	if reflowed.Pos != nil {
		t.Errorf("a reflowed anchor was given offset %d; a fold-space position written"+
			" into a raw-space column misdirects a reader with no way to notice",
			*reflowed.Pos)
	}
	// It still gets an address: the hash is what `claim-anchor` resolves on, and the
	// claim is not lost merely because its byte offset is unknown.
	if reflowed.AnchorHash == "" {
		t.Error("a claim with no locatable offset was given no anchor hash either")
	}

	exact := rows[byID["c2"]]
	if exact.Pos == nil || *exact.Pos != 0 {
		t.Errorf("an anchor at the first byte got %v, want 0 — zero is a real position,"+
			" which is why Pos is a pointer", exact.Pos)
	}
}

// TestAClaimWithoutAnAnchorIsNotAnAddress keeps the table free of rows that address
// nothing. An anchor that was never written is not an anchor that broke, and
// `conformance` is what reports the absence.
func TestAClaimWithoutAnAnchorIsNotAnAddress(t *testing.T) {
	t.Parallel()
	docs := []bundle.Document{{
		ID: gnosis.ID("01J0000000000000000000000A"), Path: "c/a.md", Type: "Rule",
		Body:        "Something.",
		Limitations: []string{"does not cover batch jobs"},
		Claims:      []bundle.DocClaim{{ID: "c1"}, {ID: "c2", Anchor: "  \n "}},
	}}
	if rows := bundle.ClaimRows(docs); len(rows) != 0 {
		t.Errorf("claims with no anchor produced %d rows", len(rows))
	}
}

// TestADocumentWithNoIdentityOwnsNoClaims is the foreign key stated as a test: a claim
// row points at a document, and a document gnosis never assigned an id to has nothing
// for it to point at.
func TestADocumentWithNoIdentityOwnsNoClaims(t *testing.T) {
	t.Parallel()
	docs := []bundle.Document{{
		Path: "c/a.md", Type: "Rule", Body: "Something.",
		Limitations: []string{"does not cover batch jobs"},
		Claims:      []bundle.DocClaim{{ID: "c1", Anchor: "Something."}},
	}}
	if rows := bundle.ClaimRows(docs); len(rows) != 0 {
		t.Errorf("an unidentified document produced %d claim rows", len(rows))
	}
}

// TestALeadReachesTheIndexAndAnAbsentOneDoesNot is §5.5.3's rule where it now has data.
//
// The empty string is a real lead — it would assert that the claim has no conclusion —
// so a claim extraction has not summarised gets NULL, and only a claim that has one is
// put in the search index. A row indexed on three NULL columns is a searchable entry
// matching nothing, which makes claim search return fewer results than there are claims
// with nothing saying why.
func TestALeadReachesTheIndexAndAnAbsentOneDoesNot(t *testing.T) {
	t.Parallel()

	docs := []bundle.Document{{
		ID: gnosis.ID("0192a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"), Path: "c/a.md",
		Type: "Rule", Body: "Retries are capped.\n\nBackoff is exponential.\n",
		Claims: []bundle.DocClaim{
			{ID: "c1", Anchor: "Retries are capped.", Lead: "Cap retries at three."},
			{ID: "c2", Anchor: "Backoff is exponential."},
		},
	}}
	rows := bundle.ClaimRows(docs)
	if len(rows) != 2 {
		t.Fatalf("want two rows, got %d", len(rows))
	}
	byID := map[string]string{}
	for _, r := range rows {
		byID[r.ID] = r.Lead
	}
	if byID["c1"] != "Cap retries at three." {
		t.Errorf("the declared lead did not reach the row: %q", byID["c1"])
	}
	if byID["c2"] != "" {
		t.Errorf("a lead was invented for a claim that declares none: %q", byID["c2"])
	}
}

// TestEveryDeclaredClaimFieldReachesTheSnapshot is the test that was missing, and the
// gap it closes is structural rather than about one field.
//
// The `lead` check was written, tested, and correct while `claimRefs` never copied
// `Lead` into the snapshot — so the check ran on every corpus and found nothing, and its
// own tests passed because they *constructed* a `lint.Snapshot` directly. A pure-core
// test that builds its own input cannot detect a shell that never supplies it, and only
// running the binary showed the check silently examining an empty field.
//
// So this walks the projection field by field rather than asserting one: a field added
// to DocClaim and forgotten in claimRefs fails here, which is the direction that drifts.
func TestEveryDeclaredClaimFieldReachesTheSnapshot(t *testing.T) {
	t.Parallel()

	docs := []bundle.Document{{
		ID: gnosis.ID("0192a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"), Path: "c/a.md",
		Type: "Rule", Body: "An assertion.\n",
		Limitations: []string{"does not cover batch jobs"},
		Claims: []bundle.DocClaim{{
			ID:           "c1",
			Anchor:       "An assertion.",
			Lead:         "The conclusion, first.",
			Subject:      "retry budget",
			Quotes:       []string{"a quoted passage from the source"},
			ArchivePaths: []string{"evidence/text/aa/x.txt"},
		}},
	}}

	// The document-level projection too: `Limitations` is a Document field and the
	// same seam swallowed `Lead` one level down.
	if projected := bundle.DocumentsForTest(docs); len(projected) != 1 ||
		len(projected[0].Limitations) != 1 {
		t.Errorf("Limitations did not reach the snapshot: %+v", projected)
	}

	got := bundle.ClaimRefsForTest(docs[0].Claims)
	if len(got) != 1 {
		t.Fatalf("want one claim, got %d", len(got))
	}
	for name, pair := range map[string][2]string{
		"ID":      {got[0].ID, "c1"},
		"Anchor":  {got[0].Anchor, "An assertion."},
		"Lead":    {got[0].Lead, "The conclusion, first."},
		"Subject": {got[0].Subject, "retry budget"},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s did not reach the snapshot: got %q, want %q",
				name, pair[0], pair[1])
		}
	}
	if len(got[0].Quotes) != 1 || len(got[0].ArchivePaths) != 1 {
		t.Errorf("the list fields did not reach the snapshot: %+v", got[0])
	}
}
