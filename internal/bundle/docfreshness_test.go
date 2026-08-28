package bundle_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// checkedAt is when the fixture's user verified a source. A fixed instant rather
// than time.Now, because every state here is a comparison against it.
var checkedAt = time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

// freshnessBundle archives two sources, records a check against only the first, and
// files a document with one claim resting on each.
//
// The asymmetry is the whole fixture: one claim's evidence has been verified and the
// other's has not, which is exactly the corpus state the entry describes — "one
// unverified source marks the whole page" — and the shape in which per-claim reporting
// either helps or does not.
func freshnessBundle(t *testing.T, staleAfter string) (dir, rel string) {
	t.Helper()

	dir = t.TempDir()
	verified := archiveSource(t, dir, "https://example.org/one.md",
		"Vendor documentation. "+quoteRun+", and the cache is per-process.\n")
	unverified := archiveSource(t, dir, "https://example.org/two.md",
		"Second vendor page. The queue drains in order and never reorders work.\n")

	withWriter(t, dir, func(w *bundle.Writer) {
		// Only the first source. An absent entry is §14.3's `unknown`, which is
		// what the second claim must report.
		if err := w.RecordChecks(checkedAt, []bundle.Check{{
			URI: "https://example.org/one.md", SourceSHA256: sourceHashOf(t, dir, verified),
		}}); err != nil {
			t.Fatalf("record checks: %v", err)
		}
	})

	front := "---\ntype: Reference\ntitle: Cache Lifetime\ngnosis_id: " + docID + "\n"
	if staleAfter != "" {
		front += "stale_after: " + staleAfter + "\n"
	}
	front += "gnosis_claims:\n" +
		"  - id: claim-verified\n" +
		"    anchor: the cache is cleared on restart\n" +
		"    archive_paths:\n      - " + verified + "\n" +
		"  - id: claim-unverified\n" +
		"    anchor: the queue drains in order\n" +
		"    archive_paths:\n      - " + unverified + "\n" +
		"  - id: claim-uncited\n" +
		"    anchor: nothing backs this one\n" +
		"---\nBody.\n"

	rel = docPath
	if err := os.MkdirAll(filepath.Join(dir, "c"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, filepath.FromSlash(rel)), []byte(front), 0o600,
	); err != nil {
		t.Fatalf("write document: %v", err)
	}
	return dir, rel
}

// archiveSource puts one source in tier 0 and returns its archive path.
func archiveSource(t *testing.T, dir, uri, text string) string {
	t.Helper()

	out := archive.Decide(&archive.Candidate{
		URI: uri, Bytes: []byte(text), Extension: ".md",
	}, archive.Gates{
		Allowlist: []string{".md"}, PerFileCap: 262144, EmbeddedPayloadCap: 8192,
		ScanText: archive.NoScan,
	})
	if out.Record.Disposition != archive.Archived {
		t.Fatalf("the fixture source was not archived: %q", out.Record.RejectReason)
	}
	withWriter(t, dir, func(w *bundle.Writer) {
		if _, err := w.StoreEvidence(&out); err != nil {
			t.Fatalf("store evidence: %v", err)
		}
	})
	return out.Record.ArchivePath
}

// sourceHashOf recovers a source's hash from the archived text, which is what
// checked.jsonl keys an observation by.
func sourceHashOf(t *testing.T, dir, archivePath string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(archivePath)))
	if err != nil {
		t.Fatalf("read archived text: %v", err)
	}
	return archive.SourceHash(raw)
}

// TestFreshnessIsReportedPerClaim is the entry: `show` reported the oldest check
// across every source a document cites, so one unverified source marked the whole
// page. That is the right conservative answer and the wrong useful one.
func TestFreshnessIsReportedPerClaim(t *testing.T) {
	t.Parallel()

	dir, rel := freshnessBundle(t, "")
	got, err := bundle.FreshnessFor(dir, rel, checkedAt.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("freshness: %v", err)
	}

	want := map[string]gnosis.Freshness{
		"claim-verified":   gnosis.FreshnessFresh,
		"claim-unverified": gnosis.FreshnessUnknown,
		// A claim citing no archived source is not unverified; there is nothing
		// for it to be verified against.
		"claim-uncited": gnosis.FreshnessNotApplicable,
	}
	if len(got.Claims) != len(want) {
		t.Fatalf("want %d claims, got %+v", len(want), got.Claims)
	}
	for i := range got.Claims {
		cl := &got.Claims[i]
		if cl.State != want[cl.ID] {
			t.Errorf("%s = %v, want %v", cl.ID, cl.State, want[cl.ID])
		}
		if cl.Why == "" {
			t.Errorf("%s reports no reason", cl.ID)
		}
		// The anchor travels with the state, because the point of the per-claim
		// report is to put the reader in front of the sentence.
		if cl.Anchor == "" {
			t.Errorf("%s carries no anchor", cl.ID)
		}
	}
	// And the claims are in declaration order, so the report matches the document.
	if got.Claims[0].ID != "claim-verified" || got.Claims[2].ID != "claim-uncited" {
		t.Errorf("claims are out of declaration order: %+v", got.Claims)
	}
}

// TestTheDocumentLineStaysTheWeakestClaim is the invariant that makes the addition
// safe.
//
// Per-claim reporting is an addition, not a replacement: a reader who wants to know
// whether to trust the page must still get the conservative verdict. If the document
// line were ever computed to be stronger than one of its claims, the useful answer
// would have quietly replaced the safe one.
func TestTheDocumentLineStaysTheWeakestClaim(t *testing.T) {
	t.Parallel()

	dir, rel := freshnessBundle(t, "")
	got, err := bundle.FreshnessFor(dir, rel, checkedAt.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("freshness: %v", err)
	}
	if got.State != gnosis.FreshnessUnknown {
		t.Errorf("the document reads %v with an unverified claim under it", got.State)
	}
	for i := range got.Claims {
		if got.Claims[i].State == gnosis.FreshnessFresh &&
			got.State == gnosis.FreshnessFresh {
			continue
		}
		if got.State.Trustworthy() && !got.Claims[i].State.Trustworthy() {
			t.Errorf("the document is trustworthy and claim %s is not",
				got.Claims[i].ID)
		}
	}
}

// TestADeclaredDateReachesEveryClaimUnderIt is §14.3.0's distinction applied at the
// new grain.
//
// `stale_after` is a statement about what the document asserts, and a claim is one of
// those assertions — so the date governs each of them. A per-claim report that only
// looked at check times would show a verified claim as fresh under a document its
// author had already asked to be revisited, which is the read-time dependence §14.3.0
// exists to prevent, reintroduced one level down.
func TestADeclaredDateReachesEveryClaimUnderIt(t *testing.T) {
	t.Parallel()

	dir, rel := freshnessBundle(t, "2026-04-02")
	got, err := bundle.FreshnessFor(dir, rel, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("freshness: %v", err)
	}
	for i := range got.Claims {
		cl := &got.Claims[i]
		if cl.ID == "claim-verified" && cl.State != gnosis.FreshnessStale {
			t.Errorf("a verified claim past the document's date reads %v", cl.State)
		}
		// The unverified one stays unknown rather than becoming stale: nobody has
		// looked, and a date cannot turn that into a verdict about the source.
		if cl.ID == "claim-unverified" && cl.State != gnosis.FreshnessUnknown {
			t.Errorf("an unchecked claim reads %v under an expired date", cl.State)
		}
	}
}

// TestADocumentWithNoClaimsReportsNone keeps the addition from inventing rows.
//
// Most hand-written Phase 2 documents declare no claims, and a report that emitted a
// placeholder for them would put a freshness verdict on something that asserts
// nothing enforceable.
func TestADocumentWithNoClaimsReportsNone(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "c"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	rel := "c/" + docID + "-plain.md"
	doc := "---\ntype: Reference\ntitle: Plain\ngnosis_id: " + docID + "\n---\nBody.\n"
	if err := os.WriteFile(
		filepath.Join(dir, filepath.FromSlash(rel)), []byte(doc), 0o600,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := bundle.FreshnessFor(dir, rel, checkedAt)
	if err != nil {
		t.Fatalf("freshness: %v", err)
	}
	if len(got.Claims) != 0 {
		t.Errorf("a document declaring no claims reported %+v", got.Claims)
	}
	if got.State != gnosis.FreshnessNotApplicable {
		t.Errorf("state = %v, want not_applicable", got.State)
	}
}

// TestAClaimReportsWhichSourcesSupportIt is the entry: a claim supported by four
// independent sources and one supported by one look identical in frontmatter, because
// both carry `archive_paths` and nothing said what those resolved to.
func TestAClaimReportsWhichSourcesSupportIt(t *testing.T) {
	t.Parallel()

	dir, rel := freshnessBundle(t, "")
	got, err := bundle.FreshnessFor(dir, rel, checkedAt.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("freshness: %v", err)
	}

	want := map[string][]string{
		"claim-verified":   {"https://example.org/one.md"},
		"claim-unverified": {"https://example.org/two.md"},
		// A claim citing no archived source cites no source, and reporting an
		// empty list rather than nothing at all would be the same statement.
		"claim-uncited": nil,
	}
	for i := range got.Claims {
		cl := &got.Claims[i]
		assertSameSources(t, cl.ID, cl.Sources, want[cl.ID])
	}
}

// TestTwoVersionsOfOneSourceAreOneSource is the error the field exists to prevent, and
// the reason it is a set rather than a count.
//
// A source fetched twice has two records and two archive paths (§4.1). A claim citing
// both cites one page at two moments — and a count would report it as corroboration by
// two, which is exactly the inheritance §1.1 refuses.
func TestTwoVersionsOfOneSourceAreOneSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	const uri = "https://example.org/one.md"
	first := archiveSource(t, dir, uri, "Vendor documentation. "+quoteRun+", per-process.\n")
	second := archiveSource(t, dir, uri,
		"Vendor documentation, revised. "+quoteRun+", and per-process.\n")
	if first == second {
		t.Fatal("the fixture archived one version twice; there is nothing to collapse")
	}

	doc := "---\ntype: Reference\ntitle: Cache Lifetime\ngnosis_id: " + docID + "\n" +
		"gnosis_claims:\n" +
		"  - id: claim-two-versions\n" +
		"    anchor: the cache is cleared on restart\n" +
		"    archive_paths:\n      - " + first + "\n      - " + second + "\n" +
		"---\nBody.\n"
	if err := os.MkdirAll(filepath.Join(dir, "c"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, filepath.FromSlash(docPath)), []byte(doc), 0o600,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := bundle.FreshnessFor(dir, docPath, checkedAt)
	if err != nil {
		t.Fatalf("freshness: %v", err)
	}
	if len(got.Claims) != 1 {
		t.Fatalf("want one claim, got %+v", got.Claims)
	}
	assertSameSources(t, got.Claims[0].ID, got.Claims[0].Sources, []string{uri})
}

// assertSameSources compares a claim's sources, treating nil and empty as the same
// absence — a reader asks which sources, and "none" has one meaning.
func assertSameSources(t *testing.T, id string, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("%s sources = %q, want %q", id, got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s sources[%d] = %q, want %q", id, i, got[i], want[i])
		}
	}
}
