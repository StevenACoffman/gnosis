package bundle_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/skillet/textnorm"
)

// supports indexes quotations as the archive check would.
func supports(quotes ...string) map[string]bool {
	out := map[string]bool{}
	for _, q := range quotes {
		out[textnorm.Fold(q)] = true
	}
	return out
}

// TestARewriteThatSwapsOneQuotationForAnotherIsRefused is the adversarial case, and the
// reason the gate is a set comparison rather than a count.
//
// A rewrite that drops one passage and adds another balances: same number of claims,
// same number of quotations, and a reader skimming a diff sees prose that reads better.
// What went missing is in frontmatter, and it is the passage that made the original
// claim checkable.
func TestARewriteThatSwapsOneQuotationForAnotherIsRefused(t *testing.T) {
	t.Parallel()

	reply := &relay.Reply{
		Type: "Rule", Title: "Retry Budget",
		Claims: []relay.Claim{{
			Text:   "Retries are bounded.",
			Quotes: []string{"a different passage entirely"},
		}},
	}
	got, err := bundle.Synthesize(accretionFixture(t), accretedID, reply,
		supports("a different passage entirely"))
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if got.Approved() {
		t.Error("a rewrite that swapped the evidence was approved")
	}
	if len(got.Dropped) != 1 || !strings.Contains(got.Dropped[0], "retries are capped") {
		t.Errorf("the lost quotation was not named: %v", got.Dropped)
	}
	if len(got.Content) != 0 {
		t.Error("a refused rewrite produced a document to write")
	}
}

// TestARewriteMayRewordAnythingItKeepsTheEvidenceFor is the other half: the gate is
// stated over quotations, not paragraphs, so a rewrite is free to reorganise.
func TestARewriteMayRewordAnythingItKeepsTheEvidenceFor(t *testing.T) {
	t.Parallel()

	reply := &relay.Reply{
		Type: "Rule", Title: "Retry Budget",
		Claims: []relay.Claim{{
			Text:   "Wholly different wording for the same fact.",
			Quotes: []string{"retries are capped at three"},
		}},
	}
	got, err := bundle.Synthesize(accretionFixture(t), accretedID, reply,
		supports("retries are capped at three"))
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if !got.Approved() {
		t.Fatalf("a rewrite keeping every quotation was refused: dropped=%v unvalidated=%v",
			got.Dropped, got.Unvalidated)
	}
	if !strings.Contains(string(got.Content), "Wholly different wording") {
		t.Errorf("the new prose is not in the document:\n%s", got.Content)
	}
	// Identity survives a body replacement: §5.1 assigns it once and never rewrites
	// it, which is what makes "what did we believe in March" answerable.
	if !strings.Contains(string(got.Content), string(accretedID)) {
		t.Errorf("the rewrite minted a new identity:\n%s", got.Content)
	}
}

// TestAQuotationTheArchiveDoesNotSupportIsRefused keeps the ordinary evidence invariant
// applying to a rewrite. Both refusals are reported together, so one round trip tells a
// caller everything rather than the first kind twice.
func TestAQuotationTheArchiveDoesNotSupportIsRefused(t *testing.T) {
	t.Parallel()

	reply := &relay.Reply{
		Type: "Rule", Title: "Retry Budget",
		Claims: []relay.Claim{{
			Text: "Retries are bounded.",
			Quotes: []string{
				"retries are capped at three", // kept, and supported
				"something no source says",    // invented
			},
		}},
	}
	got, err := bundle.Synthesize(accretionFixture(t), accretedID, reply,
		supports("retries are capped at three"))
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if got.Approved() {
		t.Error("a rewrite quoting something unsupported was approved")
	}
	if len(got.Dropped) != 0 {
		t.Errorf("nothing was dropped, but it reported %v", got.Dropped)
	}
	if len(got.Unvalidated) != 1 {
		t.Fatalf("unvalidated = %v, want the one invented quotation", got.Unvalidated)
	}
}

// TestAReflowedQuotationIsTheSameQuotation keeps the gate from reporting a loss the
// rewrite did not cause: the evidence invariant compares under fold, so a passage
// re-offered with different whitespace has been kept.
func TestAReflowedQuotationIsTheSameQuotation(t *testing.T) {
	t.Parallel()

	reply := &relay.Reply{
		Type: "Rule", Title: "Retry Budget",
		Claims: []relay.Claim{{
			Text:   "Retries are bounded.",
			Quotes: []string{"retries   are capped\nat three"},
		}},
	}
	got, err := bundle.Synthesize(accretionFixture(t), accretedID, reply,
		supports("retries are capped at three"))
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if len(got.Dropped) != 0 {
		t.Errorf("a reflowed quotation was reported as dropped: %v", got.Dropped)
	}
	if !got.Approved() {
		t.Errorf("the rewrite was refused: unvalidated=%v", got.Unvalidated)
	}
}

// TestARewriteWithNoTitleIsRefusedBeforeTheGate is the constructor check: a document
// needs a title and a type to be rendered at all, and reaching the evidence gate with
// neither would report an evidence problem for a structural one.
func TestARewriteWithNoTitleIsRefusedBeforeTheGate(t *testing.T) {
	t.Parallel()
	_, err := bundle.Synthesize(accretionFixture(t), accretedID,
		&relay.Reply{Type: "Rule"}, nil)
	if err == nil {
		t.Fatal("a rewrite with no title was accepted")
	}
	if !strings.Contains(err.Error(), "title") {
		t.Errorf("the refusal does not say what is missing: %v", err)
	}
}
