package index_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/index"
)

// candidateCorpus indexes four documents so both rules have something to find.
//
//	alpha  — a claim resting on the incoming source          (rule 1)
//	beta   — links to alpha                                  (rule 2)
//	gamma  — alpha links to it                               (rule 2)
//	delta  — unrelated to any of them                        (neither)
//
// A real SQLite file rather than a mock, which is this package's rule: what is under test
// is a query, and a mock of a query only tests the mock.
func candidateCorpus(t *testing.T) *index.DB {
	t.Helper()

	db := openTemp(t)
	doc := func(id, path, title string) index.DocumentRow {
		return index.DocumentRow{ID: gnosis.ID(id), Path: path, Title: title}
	}
	claim := func(id, docID, lead string) index.ClaimRow {
		return index.ClaimRow{
			ID: id, DocumentID: gnosis.ID(docID), AnchorHash: id,
			Type: "Rule", Lead: lead,
		}
	}
	contents := index.Contents{
		Documents: []index.DocumentRow{
			doc("alpha", "c/alpha.md", "Alpha"), doc("beta", "c/beta.md", "Beta"),
			doc("gamma", "c/gamma.md", "Gamma"), doc("delta", "c/delta.md", "Delta"),
		},
		Claims: []index.ClaimRow{
			claim("a1", "alpha", "alpha says so"), claim("b1", "beta", "beta says so"),
			claim("g1", "gamma", "gamma says so"), claim("d1", "delta", "delta says so"),
		},
		ClaimSources: []index.ClaimSourceRow{
			{ClaimID: "a1", ArchivePath: "evidence/text/aa/one.md"},
			{ClaimID: "d1", ArchivePath: "evidence/text/zz/other.md"},
		},
		Links: []index.LinkRow{
			{SourceID: "beta", TargetID: "alpha", Href: "/c/alpha.md"},
			{SourceID: "alpha", TargetID: "gamma", Href: "/c/gamma.md"},
			{SourceID: "delta", Href: "https://example.org", External: true},
		},
	}
	if err := db.Replace(t.Context(), &contents); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	return db
}

// TestCandidatesFindWhatRestsOnTheSourceAndItsNeighbours is §6.2's promise: the corpus
// finds what an incoming source might update without asking a model per candidate.
//
// What I am afraid of is the opposite of a miss. A selector returning most of the corpus
// would be `llmwiki`'s ~47 candidates with a query in front of them — the thing §6.2 says
// gnosis MUST NOT do — so the test pins that an unrelated document stays out.
func TestCandidatesFindWhatRestsOnTheSourceAndItsNeighbours(t *testing.T) {
	t.Parallel()

	db := candidateCorpus(t)
	got, err := db.CandidatesFor(t.Context(), []string{"evidence/text/aa/one.md"}, 10)
	if err != nil {
		t.Fatalf("CandidatesFor: %v", err)
	}

	byClaim := map[string]index.CandidateRule{}
	for _, c := range got {
		byClaim[c.ClaimID] = c.Rule
	}
	if byClaim["a1"] != index.RuleSharedSource {
		t.Errorf("the claim resting on the source was found by %q, want shared-source",
			byClaim["a1"])
	}
	for _, id := range []string{"b1", "g1"} {
		if byClaim[id] != index.RuleOneHop {
			t.Errorf("%s was found by %q, want one-hop — §6.2 says linked to *or from*",
				id, byClaim[id])
		}
	}
	if _, found := byClaim["d1"]; found {
		t.Error("a document sharing neither a source nor a link was selected, which is" +
			" the scan §6.2 exists to refuse")
	}
}

// TestEachCandidateNamesTheRuleThatFoundIt. §6.2's claim is that selection is a query
// rather than a judgment, and a bare list is a ranking a reader has to trust.
func TestEachCandidateNamesTheRuleThatFoundIt(t *testing.T) {
	t.Parallel()

	got, err := candidateCorpus(t).CandidatesFor(
		t.Context(), []string{"evidence/text/aa/one.md"}, 10)
	if err != nil {
		t.Fatalf("CandidatesFor: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no candidates at all")
	}
	for _, c := range got {
		if c.Rule == "" {
			t.Errorf("%s names no rule", c.ClaimID)
		}
		if c.Path == "" || c.Title == "" {
			t.Errorf("%s carries no path or title, so a reader cannot see what it is",
				c.ClaimID)
		}
	}
}

// TestTheLimitBoundsTheWholeResult, and the shared-source rule keeps its place at the
// front: a bound that dropped the claims actually resting on the source in favour of
// their neighbours would invert §6.2's own ordering.
func TestTheLimitBoundsTheWholeResult(t *testing.T) {
	t.Parallel()

	db := candidateCorpus(t)
	got, err := db.CandidatesFor(t.Context(), []string{"evidence/text/aa/one.md"}, 1)
	if err != nil {
		t.Fatalf("CandidatesFor: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d candidates, want 1", len(got))
	}
	if got[0].Rule != index.RuleSharedSource {
		t.Errorf("the bounded result kept %q, want the shared source first", got[0].Rule)
	}
}

// TestASourceNothingRestsOnSelectsNothing. A source rejected by tier 0, or one nobody has
// cited yet, is an ordinary state — and an error here would make the first ingest of a
// new source look like a failure.
func TestASourceNothingRestsOnSelectsNothing(t *testing.T) {
	t.Parallel()

	db := candidateCorpus(t)
	for name, paths := range map[string][]string{
		"no archived text":    nil,
		"a path nobody cites": {"evidence/text/qq/nobody.md"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := db.CandidatesFor(t.Context(), paths, 10)
			if err != nil {
				t.Fatalf("CandidatesFor: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("got %+v, want nothing", got)
			}
		})
	}
}
