package bundle_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// TestTrustForReportsBothGrains is the case the roll-up exists for: a page with one
// human-reviewed claim beside one nobody verified is not a reviewed page.
//
// It goes through TrustFor rather than through the pure fold, because the fold has
// its own test in internal/gnosis and the thing untested there is the projection —
// whether the shell hands the fold the actors the document actually declares. That
// seam is where `lead` and `limitations` each shipped a correct check reading an
// empty field.
func TestTrustForReportsBothGrains(t *testing.T) {
	t.Parallel()

	dir, rel := trustBundle(t)
	got, err := bundle.TrustFor(dir, rel)
	if err != nil {
		t.Fatalf("TrustFor: %v", err)
	}

	if got.State != gnosis.TierUnverified {
		t.Errorf("document tier = %v, want unverified: one of its claims carries no "+
			"verification, and the weakest claim is the document's answer", got.State)
	}
	if got.Why == "" {
		t.Error("the document tier carries no sentence")
	}
	if len(got.Claims) != 3 {
		t.Fatalf("got %d claim tiers, want 3", len(got.Claims))
	}

	want := map[string]gnosis.Tier{
		"claim-reviewed": gnosis.TierHumanReviewed,
		"claim-machine":  gnosis.TierMachineConfirmed,
		"claim-bare":     gnosis.TierUnverified,
	}
	for i := range got.Claims {
		cl := &got.Claims[i]
		if w, ok := want[cl.ID]; !ok {
			t.Errorf("unexpected claim %q", cl.ID)
		} else if cl.State != w {
			t.Errorf("claim %s tier = %v, want %v", cl.ID, cl.State, w)
		}
	}

	// The actors travel verbatim. `process:` is conformant OKF and gnosis.ParseActor
	// refuses it (§14.1.1), so a shell that parsed before folding would drop it — and
	// the claim would read as unverified rather than machine-confirmed.
	for i := range got.Claims {
		if cl := &got.Claims[i]; cl.ID == "claim-machine" {
			if len(cl.By) != 1 || cl.By[0] != "process:finance-nightly" {
				t.Errorf("claim-machine actors = %q, want the raw OKF form", cl.By)
			}
		}
	}
}

// TestTrustForOnAnAbsentDocument. ENOTFOUND rather than a zero value, because an
// unverified tier for a document the corpus does not hold is an answer about nothing.
func TestTrustForOnAnAbsentDocument(t *testing.T) {
	t.Parallel()

	_, err := bundle.TrustFor(t.TempDir(), "c/01932b7c-0000-7000-8000-000000000000-absent.md")
	if errs.ErrorCode(err) != errs.ENOTFOUND {
		t.Errorf("TrustFor on an absent document = %v, want ENOTFOUND", err)
	}
}

// TestTrustForADocumentDeclaringNoClaims. Unverified, and the sentence has to say
// which of the two unverified cases it is: nothing to fold, or nothing verified.
func TestTrustForADocumentDeclaringNoClaims(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConcept(t, dir, "---\ntype: Reference\ntitle: Bare\ngnosis_id: "+docID+
		"\n---\nBody.\n")

	got, err := bundle.TrustFor(dir, docPath)
	if err != nil {
		t.Fatalf("TrustFor: %v", err)
	}
	if got.State != gnosis.TierUnverified {
		t.Errorf("tier = %v, want unverified", got.State)
	}
	if len(got.Claims) != 0 {
		t.Errorf("got %d claim tiers for a document declaring none", len(got.Claims))
	}
	if got.Why != "it declares no claims, so there is no verification to fold" {
		t.Errorf("why = %q; it must distinguish an empty page from an unverified one",
			got.Why)
	}
}

// trustBundle files one document whose three claims sit at the three tiers.
//
// No archive and no checks: trust is a fold over `verified` and touches neither, and
// a fixture that set them up would be asserting that this function ignores them.
func trustBundle(t *testing.T) (dir, rel string) {
	t.Helper()

	dir = t.TempDir()
	writeConcept(t, dir, "---\ntype: Reference\ntitle: Retry Budget\ngnosis_id: "+
		docID+"\n"+
		"gnosis_claims:\n"+
		"  - id: claim-reviewed\n"+
		"    anchor: retries are capped at three\n"+
		"    verified:\n"+
		"      - by: human:priya\n        at: 2026-08-01T00:00:00Z\n"+
		"  - id: claim-machine\n"+
		"    anchor: the queue drains in order\n"+
		"    verified:\n"+
		"      - by: process:finance-nightly\n        at: 2026-08-02T00:00:00Z\n"+
		"  - id: claim-bare\n"+
		"    anchor: nothing has looked at this one\n"+
		"---\nBody.\n")
	return dir, docPath
}

// writeConcept files one concept at the fixture path.
func writeConcept(t *testing.T, dir, text string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, "c"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, filepath.FromSlash(docPath)), []byte(text), 0o600,
	); err != nil {
		t.Fatalf("write document: %v", err)
	}
}
