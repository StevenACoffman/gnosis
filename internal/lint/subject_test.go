package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
)

// vocab is a two-type, one-subject vocabulary with an alias, which is the smallest
// shape that can tell the three checks apart.
func vocab() lint.Vocabulary {
	return lint.Vocabulary{
		Declared: true,
		Types: []lint.VocabType{
			{Key: "Rule", ExpectsSubject: true},
			{Key: "Reference", ExpectsSubject: false},
		},
		SubjectOf: map[gnosis.Surface]gnosis.SubjectKey{
			"retry.max_attempts": "retry.max_attempts",
			"retry budget":       "retry.max_attempts",
		},
	}
}

// runNamed runs one check by name and returns what it emitted, failing when the
// check declined to run — a skipped check reporting nothing is not the same as a
// clean one, and a test that could not tell them apart would pass either way.
func runNamed(t *testing.T, snap *lint.Snapshot, name string) []string {
	t.Helper()

	for _, c := range lint.Checks(now()) {
		if c.Name != name {
			continue
		}
		if ok, reason := c.Applies(snap); !ok {
			t.Fatalf("%s declined to run: %s", name, reason)
		}
		out := make([]string, 0)
		for _, d := range c.Run(snap) {
			out = append(out, d.Category+": "+d.Message)
		}
		return out
	}
	t.Fatalf("no check named %q is registered", name)
	return nil
}

// skipReason returns why a check declined, failing when it ran.
func skipReason(t *testing.T, snap *lint.Snapshot, name string) string {
	t.Helper()

	for _, c := range lint.Checks(now()) {
		if c.Name != name {
			continue
		}
		ok, reason := c.Applies(snap)
		if ok {
			t.Fatalf("%s ran; wanted it to decline", name)
		}
		return reason
	}
	t.Fatalf("no check named %q is registered", name)
	return ""
}

// TestAnUndeclaredVocabularySkipsRatherThanCondemns is the adversarial case, and it
// is the one that would do real damage if wrong.
//
// A bundle with no ontology.toml has nothing to check a type or a subject against.
// If these checks ran anyway they would report every type undeclared and every
// subject unknown — a corpus told its whole vocabulary is wrong when what is missing
// is the ruler. §17's distinction, and the reason Vocabulary.Declared is a field
// rather than len(Types) > 0.
func TestAnUndeclaredVocabularySkipsRatherThanCondemns(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Documents: []lint.Document{{
			Path: "c/a.md", Type: "Rule",
			Claims: []lint.Claim{{ID: "c1", Subject: "anything at all"}},
		}},
	}
	for _, name := range []string{"subject-missing", "subject-unknown", "ontology"} {
		if reason := skipReason(t, snap, name); !strings.Contains(reason, "ontology.toml") {
			t.Errorf("%s skipped for %q, which does not name the missing file", name, reason)
		}
	}
}

// TestASubjectResolvesThroughItsAlias is what §5.8.2.1 exists for: two functions
// writing the same thing their own way must reach one key.
//
// A check comparing against declared keys alone would report every alias as unknown,
// which teaches people to stop using them — and the aliases are the mechanism, not a
// convenience.
func TestASubjectResolvesThroughItsAlias(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Vocabulary: vocab(),
		Documents: []lint.Document{{
			Path: "c/a.md", Type: "Rule",
			Claims: []lint.Claim{
				{ID: "c1", Subject: "retry budget"},
				{ID: "c2", Subject: "retry.max_attempts"},
			},
		}},
	}
	if got := runNamed(t, snap, "subject-unknown"); len(got) != 0 {
		t.Errorf("an alias was reported unknown:\n%s", strings.Join(got, "\n"))
	}
}

// TestAnUnknownSubjectNamesThePhraseAndBothRemedies keeps the diagnostic actionable.
//
// A reader who is told only that something is unknown has to guess whether to fix the
// claim or extend the vocabulary, and those are opposite actions taken by different
// people.
func TestAnUnknownSubjectNamesThePhraseAndBothRemedies(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Vocabulary: vocab(),
		Documents: []lint.Document{{
			Path: "c/a.md", Type: "Rule",
			Claims: []lint.Claim{{ID: "c1", Subject: "retru budget"}},
		}},
	}
	got := runNamed(t, snap, "subject-unknown")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	for _, want := range []string{"retru budget", "c1", "declare it", "correct the claim"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not mention %q:\n%s", want, got[0])
		}
	}
}

// TestOnlyATypeExpectingASubjectIsAsked is the check's whole scope, and getting it
// wrong makes the corpus nag about every Reference it holds.
func TestOnlyATypeExpectingASubjectIsAsked(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Vocabulary: vocab(),
		Documents: []lint.Document{
			{Path: "c/rule.md", Type: "Rule", Claims: []lint.Claim{{ID: "r1"}}},
			{Path: "c/ref.md", Type: "Reference", Claims: []lint.Claim{{ID: "f1"}}},
			// An undeclared type belongs to the ontology check, not this one:
			// reporting it here would make one vocabulary edit look like two faults.
			{Path: "c/odd.md", Type: "Invented", Claims: []lint.Claim{{ID: "o1"}}},
		},
	}
	got := runNamed(t, snap, "subject-missing")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	if !strings.Contains(got[0], "r1") {
		t.Errorf("the wrong claim was reported: %s", got[0])
	}
	// The message must not read as a defect: §5.8.3 makes this a review signal, and
	// many claims of a normative type legitimately constrain nothing.
	if !strings.Contains(got[0], "not a defect") {
		t.Errorf("the finding reads as a defect rather than a review signal: %s", got[0])
	}
}

// TestAFreshCorpusIsNotToldItsVocabularyIsUnused is the noise case.
//
// The starter vocabulary ships six types and a new bundle uses one or two. Reported
// per type, `ontology` would be the loudest check in the tool on the day a corpus is
// created — and a check loudest when there is least to say teaches its reader to skip
// it. One finding, and none at all before there is a corpus to compare against.
func TestAFreshCorpusIsNotToldItsVocabularyIsUnused(t *testing.T) {
	t.Parallel()

	empty := &lint.Snapshot{Vocabulary: vocab()}
	if got := runNamed(t, empty, "ontology"); len(got) != 0 {
		t.Errorf("a bundle with no documents was told its vocabulary is unused:\n%s",
			strings.Join(got, "\n"))
	}

	started := &lint.Snapshot{
		Vocabulary: vocab(),
		Documents:  []lint.Document{{Path: "c/a.md", Type: "Rule"}},
	}
	got := runNamed(t, started, "ontology")
	if len(got) != 1 {
		t.Fatalf("want one grouped finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	if !strings.Contains(got[0], "Reference") {
		t.Errorf("the finding does not name the unused type: %s", got[0])
	}
}

// TestADeprecatedTypeStillInUseQuotesItsAnnouncement is §5.8.1's announce-then-enforce
// path working: the announcement *is* the message, and an author told only
// "deprecated" has no account of what to do instead.
func TestADeprecatedTypeStillInUseQuotesItsAnnouncement(t *testing.T) {
	t.Parallel()
	v := vocab()
	v.Types[1].Deprecated = "use Rule; Reference was split in March"

	snap := &lint.Snapshot{
		Vocabulary: v,
		Documents: []lint.Document{
			{Path: "c/a.md", Type: "Rule"},
			{Path: "c/b.md", Type: "Reference"},
		},
	}
	got := runNamed(t, snap, "ontology")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	if !strings.Contains(got[0], "split in March") {
		t.Errorf("the announcement's own message was dropped: %s", got[0])
	}
}

// TestADeprecatedTypeNobodyUsesIsSilent is the boundary the previous test does not
// cover: that state is the deprecation having worked, and reporting it would ask
// somebody to delete the entry currently telling authors what to use instead.
func TestADeprecatedTypeNobodyUsesIsSilent(t *testing.T) {
	t.Parallel()
	v := vocab()
	v.Types[1].Deprecated = "use Rule"

	snap := &lint.Snapshot{
		Vocabulary: v,
		Documents:  []lint.Document{{Path: "c/a.md", Type: "Rule"}},
	}
	if got := runNamed(t, snap, "ontology"); len(got) != 0 {
		t.Errorf("a retired type nobody uses was reported:\n%s", strings.Join(got, "\n"))
	}
}

// TestAnUndeclaredTypeBlamesTheVocabulary is OKF §11's negative requirement surfacing
// in a message: unknown `type` values MUST be tolerated, so the document is
// conformant and it is the vocabulary that has fallen behind. A finding phrased the
// other way would invite somebody to "fix" a conformant document.
func TestAnUndeclaredTypeBlamesTheVocabulary(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Vocabulary: vocab(),
		Documents: []lint.Document{
			{Path: "c/a.md", Type: "Rule"},
			{Path: "c/b.md", Type: "Runbook"},
			{Path: "c/c.md", Type: "Runbook"},
		},
	}
	got := runNamed(t, snap, "ontology")
	var undeclared string
	for _, g := range got {
		if strings.HasPrefix(g, "type-undeclared") {
			undeclared = g
		}
	}
	if undeclared == "" {
		t.Fatalf("no type-undeclared finding:\n%s", strings.Join(got, "\n"))
	}
	// The count, so a reader knows the size of the edit before opening anything.
	if !strings.Contains(undeclared, "2 documents") {
		t.Errorf("the finding does not say how many documents: %s", undeclared)
	}
	if !strings.Contains(undeclared, "conformant") {
		t.Errorf("the finding blames the document rather than the vocabulary: %s", undeclared)
	}
}
