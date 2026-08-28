package bundle_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// TestAPromptWithNoKindIsRefused is the adversarial case and the reason the field is
// not defaulted.
//
// A meta file written before this field existed decodes with an empty Kind. Reading
// that as the commoner case — a source prompt — would let a reply about an existing
// concept be admitted as a brand-new document, silently duplicating a page instead of
// updating it. Refusing costs a re-ingest; defaulting costs a corpus that quietly grows
// a second copy of everything it revisits.
func TestAPromptWithNoKindIsRefused(t *testing.T) {
	t.Parallel()

	meta := bundle.PromptMeta{Key: "k1", URI: "https://x", ArchivePath: "evidence/text/x.md"}
	err := meta.Valid()
	if err == nil {
		t.Fatal("a meta with no kind was accepted")
	}
	for _, want := range []string{"kind is unset", "new document"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
}

// TestAConceptPromptMustCarryTheHashItWasComputedAgainst is §9.4's approved-diff window
// one level up. Between emitting a prompt about a concept and admitting the reply, the
// concept can change; without the hash there is nothing to compare and the answer lands
// on bytes it was never about.
func TestAConceptPromptMustCarryTheHashItWasComputedAgainst(t *testing.T) {
	t.Parallel()

	noHash := bundle.PromptMeta{
		Key: "k1", Kind: bundle.PromptAccrete, DocumentPath: "c/a.md",
	}
	err := noHash.Valid()
	if err == nil {
		t.Fatal("a concept prompt with no document hash was accepted")
	}
	if !strings.Contains(err.Error(), "bytes it was not computed against") {
		t.Errorf("the refusal does not say what breaks: %v", err)
	}

	whole := bundle.PromptMeta{
		Key: "k1", Kind: bundle.PromptAccrete,
		DocumentPath: "c/a.md", DocumentHash: "abc123",
	}
	if err := whole.Valid(); err != nil {
		t.Errorf("a complete concept prompt was refused: %v", err)
	}
}

// TestEachKindRequiresItsOwnFields keeps the two shapes from borrowing each other's
// completeness: a source prompt with a document path is not thereby answerable, and a
// concept prompt is not made valid by an archive path.
func TestEachKindRequiresItsOwnFields(t *testing.T) {
	t.Parallel()

	sourceNoArchive := bundle.PromptMeta{
		Key: "k1", Kind: bundle.PromptSource, DocumentPath: "c/a.md",
	}
	if err := sourceNoArchive.Valid(); err == nil {
		t.Error("a source prompt with no archive path was accepted")
	}
	conceptNoDoc := bundle.PromptMeta{
		Key: "k1", Kind: bundle.PromptAccrete, ArchivePath: "evidence/text/x.md",
	}
	if err := conceptNoDoc.Valid(); err == nil {
		t.Error("a concept prompt with no document was accepted")
	}
}
