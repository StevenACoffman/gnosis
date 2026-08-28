package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
)

// bounded builds a corpus of claims on one subject, each with a parsed reading.
func bounded(ops ...[2]any) *lint.Snapshot {
	snap := &lint.Snapshot{Bounds: map[string]*lint.Bound{}}
	doc := lint.Document{Path: "c/a.md", Type: "Rule"}
	for i, o := range ops {
		id := string(rune('a' + i))
		doc.Claims = append(doc.Claims, lint.Claim{
			ID: id, Anchor: "Retries are bounded (" + id + ").",
		})
		snap.Bounds[id] = &lint.Bound{
			SubjectKey: "retry.max_attempts", Dimension: "count",
			Op: o[0].(string), Value: o[1].(float64),
			PatternID: "test-pattern", Raw: "n",
		}
	}
	snap.Documents = []lint.Document{doc}
	return snap
}

// TestDifferentBoundsThatOverlapAreNotAConflict is the adversarial case, and the one
// that decides whether this predicate is usable.
//
// `<= 3` and `<= 5` differ and are perfectly compatible — a corpus stating a bound twice
// is a well-specified corpus, not a contradictory one. A predicate that fired on
// difference rather than on disjointness would report almost every corpus that says
// anything twice, and be switched off in a week.
func TestDifferentBoundsThatOverlapAreNotAConflict(t *testing.T) {
	t.Parallel()

	for _, pair := range [][2][2]any{
		{{"<=", 3.0}, {"<=", 5.0}}, // both ceilings
		{{">=", 3.0}, {">=", 5.0}}, // both floors
		{{">=", 1.0}, {"<=", 5.0}}, // a range, stated in two claims
		{{"==", 3.0}, {"<=", 5.0}}, // an exact value inside a ceiling
		{{"==", 3.0}, {">=", 3.0}}, // the boundary itself: 3 satisfies >= 3
	} {
		snap := bounded(pair[0], pair[1])
		if got := runNamed(t, snap, "conflict"); len(got) != 0 {
			t.Errorf("%v and %v were reported as conflicting:\n%s",
				pair[0], pair[1], strings.Join(got, "\n"))
		}
	}
}

// TestDisjointBoundsConflict is the predicate: two claims whose admissible ranges share
// no value cannot both hold.
func TestDisjointBoundsConflict(t *testing.T) {
	t.Parallel()

	for _, pair := range [][2][2]any{
		{{"<=", 3.0}, {">=", 5.0}},
		{{"==", 3.0}, {"==", 5.0}},
		{{"==", 3.0}, {">=", 5.0}},
	} {
		snap := bounded(pair[0], pair[1])
		got := runNamed(t, snap, "conflict")
		if len(got) != 1 {
			t.Errorf("%v and %v produced %d findings, want 1", pair[0], pair[1], len(got))
		}
	}
}

// TestTheEnumerationPredicateIsSubsumed records §10.2's sixth predicate as covered
// rather than missing. Two claims asserting `==` on one subject with different values
// are two disjoint intervals; a second predicate firing on the same pair would report one
// problem twice.
func TestTheEnumerationPredicateIsSubsumed(t *testing.T) {
	t.Parallel()
	got := runNamed(t, bounded([2]any{"==", 3.0}, [2]any{"==", 5.0}), "conflict")
	if len(got) != 1 {
		t.Fatalf("want exactly one finding for two disjoint exact values, got %d", len(got))
	}
}

// TestAConflictShowsBothParses is §10.2.2's requirement, and the reason a derived finding
// is dismissible at all: an adjudicator sees the reading beside the claim rather than a
// verdict they have to trust.
func TestAConflictShowsBothParses(t *testing.T) {
	t.Parallel()
	got := runNamed(t, bounded([2]any{"<=", 3.0}, [2]any{">=", 5.0}), "conflict")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d", len(got))
	}
	for _, want := range []string{
		"retry.max_attempts",
		"{op: <=, value: 3}", "{op: >=, value: 5}",
		"c/a.md", "test-pattern",
		"nothing blocks on this",
	} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not show %q:\n%s", want, got[0])
		}
	}
}

// TestDifferentDimensionsAreNotCompared keeps a category error from reading as a
// conflict. Comparing a count to a duration blames the claims for the comparison.
func TestDifferentDimensionsAreNotCompared(t *testing.T) {
	t.Parallel()
	snap := bounded([2]any{"<=", 3.0}, [2]any{">=", 5.0})
	b := snap.Bounds["b"]
	b.Dimension = "duration"
	snap.Bounds["b"] = b

	if got := runNamed(t, snap, "conflict"); len(got) != 0 {
		t.Errorf("a count and a duration were compared:\n%s", strings.Join(got, "\n"))
	}
}

// TestDifferentSubjectsAreNotCompared is the candidate narrowing §10.2 describes,
// arriving with the predicate that needs it: two claims are a candidate pair only when
// they share a subject key.
func TestDifferentSubjectsAreNotCompared(t *testing.T) {
	t.Parallel()
	snap := bounded([2]any{"<=", 3.0}, [2]any{">=", 5.0})
	b := snap.Bounds["b"]
	b.SubjectKey = "request.timeout"
	snap.Bounds["b"] = b

	if got := runNamed(t, snap, "conflict"); len(got) != 0 {
		t.Errorf("two subjects were compared:\n%s", strings.Join(got, "\n"))
	}
}

// TestAClaimThatParsedToNothingIsNotCompared keeps a NULL reading out of the comparison.
// A claim with a subject and no quantity is what §10.2.3 counts as a candidate lost — it
// is not a bound, and comparing it as one would invent an assertion.
func TestAClaimThatParsedToNothingIsNotCompared(t *testing.T) {
	t.Parallel()
	snap := bounded([2]any{"<=", 3.0}, [2]any{">=", 5.0})
	b := snap.Bounds["b"]
	b.Op, b.Value = "", 0
	snap.Bounds["b"] = b

	if got := runNamed(t, snap, "conflict"); len(got) != 0 {
		t.Errorf("an unparsed claim was compared as a bound of zero:\n%s",
			strings.Join(got, "\n"))
	}
}

// TestTwoEpisodesDoNotContradictEachOther is §5.8.3.1's derived behaviour, and the guard
// must exist before the first episode does.
//
// "We set the retry budget to 3 in March" and "we set it to 5 in June" present to this
// predicate as one subject with disjoint values. Reporting them would be the corpus
// adjudicating its own history — and §10.4's supersession would then deprecate the loser
// of a conflict that should never have been one.
func TestTwoEpisodesDoNotContradictEachOther(t *testing.T) {
	t.Parallel()
	snap := episodes("Episode", "Episode")
	if got := runNamed(t, snap, "conflict"); len(got) != 0 {
		t.Errorf("two reports of different moments were adjudicated:\n%s",
			strings.Join(got, "\n"))
	}
}

// TestAnEpisodeDoesNotContradictARule is the other half, and the reason the exclusion is
// per claim rather than per pair: an episode records what happened and a rule states what
// holds. A finding pairing them would ask a reader to adjudicate a fact against a policy.
func TestAnEpisodeDoesNotContradictARule(t *testing.T) {
	t.Parallel()
	snap := episodes("Episode", "Rule")
	if got := runNamed(t, snap, "conflict"); len(got) != 0 {
		t.Errorf("an episode was adjudicated against a rule:\n%s", strings.Join(got, "\n"))
	}
}

// TestTwoRulesStillContradict keeps the exclusion from swallowing the predicate: the
// guard is about a type, and a corpus that declares one must not lose conflict detection
// on every other.
func TestTwoRulesStillContradict(t *testing.T) {
	t.Parallel()
	snap := episodes("Rule", "Rule")
	if got := runNamed(t, snap, "conflict"); len(got) != 1 {
		t.Errorf("two rules with disjoint bounds produced %d findings, want 1", len(got))
	}
}

// episodes builds two documents of the given types, each bounding one subject disjointly.
func episodes(typeA, typeB string) *lint.Snapshot {
	snap := &lint.Snapshot{
		Vocabulary: lint.Vocabulary{
			Declared: true,
			Types: []lint.VocabType{
				{Key: "Episode", Episodic: true},
				{Key: "Rule"},
			},
		},
		Bounds: map[string]*lint.Bound{
			"a": {
				SubjectKey: "retry.max_attempts", Dimension: "count",
				Op: "<=", Value: 3, PatternID: "p",
			},
			"b": {
				SubjectKey: "retry.max_attempts", Dimension: "count",
				Op: ">=", Value: 5, PatternID: "p",
			},
		},
	}
	for i, typ := range []string{typeA, typeB} {
		id := string(rune('a' + i))
		snap.Documents = append(snap.Documents, lint.Document{
			Path: "c/" + id + ".md", Type: gnosis.TypeKey(typ),
			Claims: []lint.Claim{{ID: id, Anchor: "A bound (" + id + ")."}},
		})
	}
	return snap
}
