package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

// pages builds a corpus from (path, title, quote) triples.
func pages(triples ...[3]string) *lint.Snapshot {
	snap := &lint.Snapshot{}
	for _, t := range triples {
		doc := lint.Document{Path: t[0], Type: "Reference", Title: t[1]}
		if t[2] != "" {
			doc.Claims = []lint.Claim{{ID: "c1", Quotes: []string{t[2]}}}
		}
		snap.Documents = append(snap.Documents, doc)
	}
	return snap
}

// TestPagesCitingNothingDoNotAllCollide is the adversarial case, and it would have made
// this check unusable on the corpus it is aimed at.
//
// Every hand-written page cites no evidence, so every one of them shares the empty set.
// A naive grouping would report the whole corpus as one giant duplicate-evidence
// collision — the loudest possible way to say nothing, on exactly the corpus §4.6.1's
// merge scenario produces.
func TestPagesCitingNothingDoNotAllCollide(t *testing.T) {
	t.Parallel()
	snap := pages(
		[3]string{"c/a.md", "Alpha", ""},
		[3]string{"c/b.md", "Beta", ""},
		[3]string{"c/c.md", "Gamma", ""},
	)
	if got := runNamed(t, snap, "duplicate"); len(got) != 0 {
		t.Errorf("pages citing nothing were reported as sharing evidence:\n%s",
			strings.Join(got, "\n"))
	}
}

// TestFoldEqualTitlesCollideAndTheMessageNamesTheMerge is §4.6.1's whole point: this
// condition is produced by a *clean merge* with no conflict marker, and a reader who
// thinks it means careless copying will go looking for the wrong thing.
func TestFoldEqualTitlesCollideAndTheMessageNamesTheMerge(t *testing.T) {
	t.Parallel()
	snap := pages(
		[3]string{"c/a.md", "Retry Budget", ""},
		[3]string{"c/b.md", "retry   budget", ""},
	)
	got := runNamed(t, snap, "duplicate")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	for _, want := range []string{"c/a.md", "c/b.md", "clean merge", "identity is assigned"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not carry %q:\n%s", want, got[0])
		}
	}
}

// TestThreeCollidingPagesAreOneFinding keeps the report about the collision rather than
// about the documents — `claim-anchor`'s rule for colliding anchors, one level up.
func TestThreeCollidingPagesAreOneFinding(t *testing.T) {
	t.Parallel()
	snap := pages(
		[3]string{"c/a.md", "Retry Budget", ""},
		[3]string{"c/b.md", "Retry Budget", ""},
		[3]string{"c/c.md", "Retry Budget", ""},
	)
	if got := runNamed(t, snap, "duplicate"); len(got) != 1 {
		t.Errorf("three colliding pages produced %d findings, want 1", len(got))
	}
}

// TestIdenticalEvidenceIsADifferentFindingFromAnIdenticalTitle keeps the two signals
// apart: §4.6.1 names both, and the remedies differ — merge the pages, versus decide
// which page owns the subject.
func TestIdenticalEvidenceIsADifferentFindingFromAnIdenticalTitle(t *testing.T) {
	t.Parallel()
	snap := pages(
		[3]string{"c/a.md", "Alpha", "the service retries three times"},
		[3]string{"c/b.md", "Beta", "the service retries three times"},
	)
	got := runNamed(t, snap, "duplicate")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	if !strings.HasPrefix(got[0], "duplicate-evidence") {
		t.Errorf("a shared evidence set was reported as a title collision:\n%s", got[0])
	}
}

// TestAnUntitledPageDoesNotCollideWithEveryOther is the same trap as the empty evidence
// set: a document with no title is `conformance`'s finding, and grouping on "" would put
// every one of them in one collision.
func TestAnUntitledPageDoesNotCollideWithEveryOther(t *testing.T) {
	t.Parallel()
	snap := pages(
		[3]string{"c/a.md", "", ""},
		[3]string{"c/b.md", "", ""},
	)
	if got := runNamed(t, snap, "duplicate"); len(got) != 0 {
		t.Errorf("untitled pages collided:\n%s", strings.Join(got, "\n"))
	}
}

// TestOneDocumentSkipsRatherThanPasses is the absence case: a corpus of one cannot have
// a duplicate, and being told it has none answers a question that could not be asked.
func TestOneDocumentSkipsRatherThanPasses(t *testing.T) {
	t.Parallel()
	snap := pages([3]string{"c/a.md", "Alpha", ""})
	if reason := skipReason(t, snap, "duplicate"); !strings.Contains(reason, "fewer than two") {
		t.Errorf("the skip does not say why: %q", reason)
	}
}
