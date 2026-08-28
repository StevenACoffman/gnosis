package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
)

const namedID = gnosis.ID("0192a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b")

// named builds a corpus of one identified document at a path with a title.
func named(path, title string) *lint.Snapshot {
	return &lint.Snapshot{Documents: []lint.Document{
		{ID: namedID, Path: path, Type: "Reference", Title: title},
	}}
}

// TestAHandWrittenFileIsNamedRatherThanDrifted is the adversarial case, and the one that
// decides whether this check is usable on a corpus somebody started themselves.
//
// A document gnosis never assigned an identifier to has no slug convention to violate.
// Reporting every such file would make this the loudest check on any corpus before its
// first promotion — which is every corpus at the moment somebody starts caring about it.
func TestAHandWrittenFileIsNamedRatherThanDrifted(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{Documents: []lint.Document{
		{Path: "c/my-notes.md", Type: "Reference", Title: "Something Else Entirely"},
	}}
	if reason := skipReason(t, snap, "filename-drift"); !strings.Contains(reason, "identifier") {
		t.Errorf("the skip does not say why: %q", reason)
	}
}

// TestADriftedSlugIsReportedAndNothingIsRenamed is the check, and the message carries the
// part that keeps it from alarming anybody: the identifier still resolves every link, so
// the drift is cosmetic and the next write fixes it.
func TestADriftedSlugIsReportedAndNothingIsRenamed(t *testing.T) {
	t.Parallel()
	snap := named("c/"+string(namedID)+"-old-title.md", "A New Title")
	got := runNamed(t, snap, "filename-drift")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d: %v", len(got), got)
	}
	for _, want := range []string{"a-new-title", "old-title", "still resolves", "next write"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not carry %q:\n%s", want, got[0])
		}
	}
}

// TestAMatchingSlugIsSilent keeps the check from firing on the state it is asking for.
func TestAMatchingSlugIsSilent(t *testing.T) {
	t.Parallel()
	snap := named("c/"+string(namedID)+"-a-new-title.md", "A New Title")
	if got := runNamed(t, snap, "filename-drift"); len(got) != 0 {
		t.Errorf("a matching slug was reported:\n%s", strings.Join(got, "\n"))
	}
}

// TestAnUntitledDocumentIsNotDrifted keeps one finding from becoming two. A document with
// no title is `conformance`'s finding, and a slug derived from nothing is "untitled" —
// which every such document would then share, turning one problem into a collision.
func TestAnUntitledDocumentIsNotDrifted(t *testing.T) {
	t.Parallel()
	snap := named("c/"+string(namedID)+"-whatever.md", "")
	if got := runNamed(t, snap, "filename-drift"); len(got) != 0 {
		t.Errorf("an untitled document was reported as drifted:\n%s", strings.Join(got, "\n"))
	}
}

// TestANormativeConceptWithNoLimitsIsReported is §17.2: a page that says what must be
// done and nothing about what it does not cover asserts a scope nobody examined.
func TestANormativeConceptWithNoLimitsIsReported(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Vocabulary: lint.Vocabulary{
			Declared: true,
			Types: []lint.VocabType{
				{Key: "Rule", Normative: true}, {Key: "Reference"},
			},
		},
		Documents: []lint.Document{
			{ID: namedID, Path: "c/rule.md", Type: "Rule", Title: "A Rule"},
			// A Reference records rather than prescribes: asking a fact to bound
			// itself is asking the wrong question.
			{ID: namedID, Path: "c/ref.md", Type: "Reference", Title: "A Fact"},
			{
				ID: namedID, Path: "c/bounded.md", Type: "Rule", Title: "A Bounded Rule",
				Limitations: []string{"does not cover batch jobs"},
			},
		},
	}
	got := runNamed(t, snap, "limitations")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	if !strings.Contains(got[0], "c/rule.md") {
		t.Errorf("the wrong document was reported:\n%s", got[0])
	}
}

// TestACorpusThatOnlyRecordsSkipsLimitations is the absence case: a vocabulary whose
// every type merely records has nothing for §17.2 to ask of it.
func TestACorpusThatOnlyRecordsSkipsLimitations(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Vocabulary: lint.Vocabulary{
			Declared: true, Types: []lint.VocabType{{Key: "Reference"}},
		},
		Documents: []lint.Document{
			{ID: namedID, Path: "c/a.md", Type: "Reference", Title: "A Fact"},
		},
	}
	if reason := skipReason(t, snap, "limitations"); !strings.Contains(reason, "prescribes") {
		t.Errorf("the skip does not say why: %q", reason)
	}
}
