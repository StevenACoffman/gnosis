package bundle_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
)

// TestTheCriticProjectionIsBlinded is §10.3's requirement at the seam a type cannot
// guard.
//
// `relay.CriticClaim` has three fields and none of them is a warrant, a status, a tier
// or a verification, so the prompt cannot carry one by construction. What construction
// cannot prevent is **this projection putting one into a field that does exist** — the
// rationale appended to the claim text, the status folded into the lead — and that is
// the change a future reader would make in good faith, because all of it is sitting on
// the same `DocClaim`.
//
// The fixture therefore gives one claim every forbidden value, with markers no ordinary
// text would contain, and asserts that what the prompt would be built from carries none
// of them.
//
// It reaches the seam through `export_test.go` rather than by living inside the
// package: the helpers there are compiled only under `go test`, so the test stays
// black-box and the projection stays unexported — which is what the repository's own
// linter asks for and the better answer anyway, since a test that could see the whole
// target could pass by reading a field the prompt never gets.
func TestTheCriticProjectionIsBlinded(t *testing.T) {
	t.Parallel()

	const (
		rationale = "RATIONALE-MARKER chose the vendor limit over the blog post"
		verifier  = "human:PRIYA-MARKER"
		override  = "OVERRIDE-MARKER marcus on leave"
	)
	docs := []bundle.Document{{
		ID: gnosis.ID("0192a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"), Path: "c/a.md",
		Hash: "hash",
		Claims: []bundle.DocClaim{{
			ID:           "c1",
			Anchor:       "The timeout is 400ms.",
			Lead:         "Cap the timeout at 400ms.",
			Quotes:       []string{"the timeout is 400ms in this release"},
			ArchivePaths: []string{"evidence/text/aa/x.txt"},
			Status:       "deprecated",
			Supersedes:   []string{"c/old.md#c9"},
			Verified:     []bundle.Verification{{By: verifier, At: "2026-08-01T00:00:00Z"}},
			Warrant: gnosis.Warrant{
				By: verifier, Authority: "quorum", Rationale: rationale,
				CoSignedBy: "human:MARCUS-MARKER", OverrideReason: override,
				Reverses: "REVERSES-MARKER",
			},
		}},
	}}
	sources := map[string]lint.SourceVersion{
		"evidence/text/aa/x.txt": {URI: "https://example.org/a", SHA256: "abc"},
	}

	targets, skipped := bundle.CritiquableForTest(docs, "", sources)
	if len(targets) != 1 || skipped != 0 {
		t.Fatalf("got %d targets and %d skipped, want 1 and 0", len(targets), skipped)
	}
	text, lead, quotes := bundle.CriticTargetView(&targets[0])

	blinded := strings.Join(append([]string{text, lead}, quotes...), "\n")
	for _, forbidden := range []string{
		"RATIONALE-MARKER", "PRIYA-MARKER", "MARCUS-MARKER", "OVERRIDE-MARKER",
		"REVERSES-MARKER", "deprecated", "quorum",
	} {
		if strings.Contains(blinded, forbidden) {
			t.Errorf("the critic projection carries %q, which §10.3 forbids: a judge "+
				"shown the conclusion a corpus already reached finds support for it,"+
				" and its agreement then says only that it was told\n%s",
				forbidden, blinded)
		}
	}
	// And it carries what a critic is for: the claim, its summary, its evidence.
	if text != "The timeout is 400ms." || lead != "Cap the timeout at 400ms." ||
		len(quotes) != 1 {
		t.Errorf("the projection dropped what a critic reads: %q, %q, %q",
			text, lead, quotes)
	}
}

// TestAClaimWithNoArchivedSourceIsSkipped. A critic judges a claim against its source
// (§17.1), so one with no source has nothing to be judged against — and asking anyway
// would produce an opinion about the claim's plausibility, which is the
// reasoning-without-evidence this corpus is built to refuse.
func TestAClaimWithNoArchivedSourceIsSkipped(t *testing.T) {
	t.Parallel()

	docs := []bundle.Document{{
		ID: gnosis.ID("0192a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"), Path: "c/a.md",
		Claims: []bundle.DocClaim{
			// No archive path at all.
			{ID: "c1", Anchor: "One.", Quotes: []string{"a quotation"}},
			// An archive path tier 0 has no record of, which is
			// `archive-closure`'s finding and not a claim a critic can judge.
			{
				ID: "c2", Anchor: "Two.", Quotes: []string{"a quotation"},
				ArchivePaths: []string{"evidence/text/zz/gone.txt"},
			},
			// A record but no quotation: nothing was offered to weigh.
			{
				ID: "c3", Anchor: "Three.",
				ArchivePaths: []string{"evidence/text/aa/x.txt"},
			},
		},
	}}
	sources := map[string]lint.SourceVersion{
		"evidence/text/aa/x.txt": {URI: "https://example.org/a", SHA256: "abc"},
	}

	targets, skipped := bundle.CritiquableForTest(docs, "", sources)
	if len(targets) != 0 || skipped != 3 {
		t.Errorf("got %d targets and %d skipped, want 0 and 3", len(targets), skipped)
	}
}
