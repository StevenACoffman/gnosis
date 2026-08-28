package bundle_test

import (
	"testing"
	"testing/fstest"

	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// pinnedCorpus is one claim whose prose states a bound the parser can read, and whose
// frontmatter pins a different one.
//
// Both halves matter: the pin must win, and the prose must still be there for
// `constraint-drift` to compare it against.
const pinnedCorpus = `---
type: Rule
title: "Retry Budget"
gnosis_id: 01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d
gnosis_claims:
  - id: c1
    anchor: "Retries must be no more than three."
    subject: "retry.max_attempts"
    gnosis_constraint:
      op: "<="
      value: 5
      raw: "5"
---

# Retry Budget

Retries must be no more than three.
`

// vocabTOML declares the one subject the corpus above names.
const vocabTOML = `[[types]]
key = "Rule"
desc = "prescribes"
normative = true
expects_subject = true

[[subjects]]
key = "retry.max_attempts"
dimension = "count"
desc = "how many attempts"
`

// TestAPinnedConstraintOutranksTheProse is §10.2.1's precedence rule, and it is the first
// input path that can make `claim_subjects.derived` false.
//
// **Until 2026-08-27 that column had one attainable value.** `subjectRow` hard-coded
// `Derived: true` because nothing parsed `gnosis_constraint`, so the two columns whose
// whole purpose is telling a parsed value from a pinned one meant neither — the mirror of
// stored state with no reader, and the same defect wearing the other face.
func TestAPinnedConstraintOutranksTheProse(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"c/01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d-retry.md": &fstest.MapFile{
			Data: []byte(pinnedCorpus),
		},
		"ontology.toml": &fstest.MapFile{Data: []byte(vocabTOML)},
	}
	snap, err := bundle.Snapshot(fsys, bundle.IndexState{}, bundle.FreshnessState{})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	b, ok := snap.Bounds["c1"]
	if !ok {
		t.Fatalf("no bound for the pinned claim; got %d bounds", len(snap.Bounds))
	}
	if !b.Pinned {
		t.Error("the bound is not marked pinned, so `constraint-drift` will never look" +
			" at it and `derived` is still a column with one value")
	}
	// The pin, not the prose: the anchor says three and the pin says five.
	if b.Value != 5 {
		t.Errorf("value = %v, want the pinned 5 rather than the prose's 3", b.Value)
	}
	if b.Op != "<=" {
		t.Errorf("op = %q, want %q", b.Op, "<=")
	}
}

// TestAnUnreadablePinLeavesTheClaimDerived is the adversarial case.
//
// OKF §11 requires malformed frontmatter to be tolerated rather than to fail the read: a
// corpus that will not open because one pin is mistyped is a corpus nobody can lint back
// into shape. What must not happen is the pin silently becoming a *reading* — a mapping
// with no operator must not land as a bound of zero, which is what a zero Constraint
// would assert.
func TestAnUnreadablePinLeavesTheClaimDerived(t *testing.T) {
	t.Parallel()

	broken := `---
type: Rule
title: "Retry Budget"
gnosis_id: 01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d
gnosis_claims:
  - id: c1
    anchor: "Retries must be no more than three."
    subject: "retry.max_attempts"
    gnosis_constraint:
      operator: "<="
      amount: 5
---

# Retry Budget

Retries must be no more than three.
`
	fsys := fstest.MapFS{
		"c/01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d-retry.md": &fstest.MapFile{
			Data: []byte(broken),
		},
		"ontology.toml": &fstest.MapFile{Data: []byte(vocabTOML)},
	}
	snap, err := bundle.Snapshot(fsys, bundle.IndexState{}, bundle.FreshnessState{})
	if err != nil {
		t.Fatalf("a mistyped pin failed the whole read: %v", err)
	}

	b, ok := snap.Bounds["c1"]
	if !ok {
		t.Fatal("no bound at all; a mistyped pin dropped the claim's subject too")
	}
	if b.Pinned {
		t.Error("an unreadable pin was recorded as a pin")
	}
	if b.Value != 3 {
		t.Errorf("value = %v, want the prose's 3; an unreadable pin must fall back to"+
			" the text rather than to a bound of zero", b.Value)
	}
}

// TestAnIntegerPinIsRead is the case YAML makes easy to lose: `value: 5` decodes to int
// and `value: 2.5` to float64, and a reader accepting only float64 would silently drop
// every whole-numbered pin — which is most of them.
func TestAnIntegerPinIsRead(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"c/01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d-retry.md": &fstest.MapFile{
			Data: []byte(pinnedCorpus),
		},
		"ontology.toml": &fstest.MapFile{Data: []byte(vocabTOML)},
	}
	snap, err := bundle.Snapshot(fsys, bundle.IndexState{}, bundle.FreshnessState{})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if b := snap.Bounds["c1"]; !b.Pinned || b.Value != 5 {
		t.Errorf("an integer-valued pin was not read: %+v", b)
	}
}
