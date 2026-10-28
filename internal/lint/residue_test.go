package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
)

// subjectSurface is the phrase this fixture's vocabulary resolves.
const subjectSurface = "retries"

// residueInput is one claim in the fixture.
type residueInput struct {
	claimID string
	anchor  string
	subject string
	bounded bool
}

// withBound and withoutBound are the two claims that matter, named rather than
// positional.
//
// Not `bounded`, which `interval_test.go` already uses for a whole snapshot in this same
// test package — two unrelated things under one name is what a reader disambiguates at
// every call site, and here the compiler caught it.
//
// The bound is the only thing that decides residue, so it is the only thing these two
// differ in — and a five-field positional literal at every call site said nothing about
// which field was doing the work.
// withBound's subject is fixed, because a claim whose prose parses to a bound under this
// vocabulary is about the one subject it declares — a parameter every caller passed
// "retries" to claimed a choice nobody was making, which is `weighed`'s finding one file
// over and a linter's here.
func withBound(claimID, anchor string) residueInput {
	return residueInput{
		claimID: claimID, anchor: anchor, subject: subjectSurface, bounded: true,
	}
}

func withoutBound(claimID, anchor, subject string) residueInput {
	return residueInput{claimID: claimID, anchor: anchor, subject: subject}
}

// docIDOf is the document identifier a fixture claim lives under.
//
// Derived from the claim id so the test asserting a pair's stable identity can compute
// the same reference the check does, without either of them carrying a literal UUID that
// the other has to match by eye.
func docIDOf(claimID string) gnosis.ID {
	return gnosis.ID("01932b7c-0000-7000-8000-00000000000" + claimID[len(claimID)-1:])
}

// refOf is the claim reference for a fixture claim.
func refOf(claimID string) string {
	return gnosis.ClaimRef(docIDOf(claimID), claimID)
}

// subjected builds a corpus of claims about one subject, some of which parse to a bound.
func subjected(claims ...residueInput) *lint.Snapshot {
	snap := &lint.Snapshot{
		Vocabulary: lint.Vocabulary{
			Declared: true,
			Types:    []lint.VocabType{{Key: "Rule"}},
			SubjectOf: map[gnosis.Surface]gnosis.SubjectKey{
				subjectSurface: "retry.max_attempts",
			},
		},
		Bounds: map[string]*lint.Bound{},
	}
	for _, in := range claims {
		snap.Documents = append(snap.Documents, lint.Document{
			Path: "c/" + in.claimID + ".md", ID: docIDOf(in.claimID), Type: "Rule",
			Claims: []lint.Claim{{
				ID: in.claimID, Anchor: in.anchor, Subject: in.subject,
			}},
		})
		if in.bounded {
			snap.Bounds[in.claimID] = &lint.Bound{
				SubjectKey: "retry.max_attempts", Dimension: "count",
				Op: "<=", Value: 3,
			}
		}
	}
	return snap
}

// TestUnseparatedPairsReportOnlyWhatThePredicateCouldNotDecide is the whole selection
// rule, and the failure I am afraid of is over-reporting: a residue that fired on every
// pair of claims about one subject would be noise wearing a finding's name, and a reader
// would learn to skip the category that exists to reach a judge.
func TestUnseparatedPairsReportOnlyWhatThePredicateCouldNotDecide(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		snap *lint.Snapshot
		want int
	}{
		"both parsed is the interval predicate's pair": {
			snap: subjected(
				withBound("c1", "at most three"),
				withBound("c2", "at most five"),
			),
			want: 0,
		},
		"one side states no bound": {
			snap: subjected(
				withBound("c1", "at most three"),
				withoutBound("c2", "retries are bounded sensibly", "retries"),
			),
			want: 1,
		},
		"neither states a bound": {
			snap: subjected(
				withoutBound("c1", "retries are limited", "retries"),
				withoutBound("c2", "retries are generous", "retries"),
			),
			want: 1,
		},
		"a claim with no resolvable subject is nobody's pair": {
			snap: subjected(
				withoutBound("c1", "retries are limited", "retries"),
				withoutBound("c2", "something else", ""),
			),
			want: 0,
		},
		"one claim alone is not a pair": {
			snap: subjected(
				withoutBound("c1", "retries are limited", "retries"),
			),
			want: 0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := residueFindings(lint.Run(tc.snap, []lint.Check{conflictCheckOf(t)}))
			if len(got) != tc.want {
				t.Fatalf("got %d unseparated pairs, want %d: %+v", len(got), tc.want, got)
			}
		})
	}
}

// TestAResidueFindingNamesItsIdAndItsReason. Both halves are what a reader acts on: the
// id is what a deferral is recorded against, and the reason says which claim somebody
// could rewrite rather than sending them to search for it.
func TestAResidueFindingNamesItsIdAndItsReason(t *testing.T) {
	t.Parallel()

	snap := subjected(
		withBound("c1", "at most three"),
		withoutBound("c2", "retries are bounded sensibly", "retries"),
	)
	got := residueFindings(lint.Run(snap, []lint.Check{conflictCheckOf(t)}))
	if len(got) != 1 {
		t.Fatalf("want one finding, got %+v", got)
	}
	want := lint.ResidueID(refOf("c1"), refOf("c2"))
	if !strings.Contains(got[0].Message, want) {
		t.Errorf("the finding does not name its id %q:\n%s", want, got[0].Message)
	}
	if !strings.Contains(got[0].Message, "the second states no bound") {
		t.Errorf("the finding does not say which side gave the predicate nothing:\n%s",
			got[0].Message)
	}
	// Advisory, because §10.2.2 makes a derived comparison a finding and never a
	// verdict, and nothing blocks on a conflict.
	if got[0].Severity != finding.SeverityWarning {
		t.Errorf("severity = %v, want warning", got[0].Severity)
	}
	if got[0].Action != finding.ActionHuman {
		t.Errorf("action = %v, want a human's: no predicate can settle this",
			got[0].Action)
	}
}

// TestResidueIDIsOrderFreeAndStable. A deferral is recorded against this id, so an id
// that depended on the order the checks happened to visit two claims would leave the
// corpus unable to match what somebody decided to live with.
func TestResidueIDIsOrderFreeAndStable(t *testing.T) {
	t.Parallel()

	a, b := "doc-a#c1", "doc-b#c2"
	if lint.ResidueID(a, b) != lint.ResidueID(b, a) {
		t.Error("the id depends on the order of the pair")
	}
	if lint.ResidueID(a, b) == lint.ResidueID(a, "doc-c#c3") {
		t.Error("two different pairs share an id")
	}
	if got := len(lint.ResidueID(a, b)); got != 16 {
		t.Errorf("id length = %d, want 16: it goes into frontmatter a person reads", got)
	}
}

// residueFindings filters a report down to the unseparated pairs.
func residueFindings(report lint.Report) []finding.Diagnostic {
	var out []finding.Diagnostic
	for _, d := range report.Diagnostics {
		if strings.HasPrefix(d.Category, "conflict:unseparated") {
			out = append(out, d)
		}
	}
	return out
}

// conflictCheckOf is the registered conflict check, found by name rather than rebuilt.
//
// A test that constructed its own check would be testing a copy: what ships is what
// `lint.Checks` registers, and the applicability rule is part of what is under test here
// — a corpus of subjected claims must not be reported as having nothing to examine.
func conflictCheckOf(t *testing.T) lint.Check {
	t.Helper()

	for _, check := range lint.Checks(now()) {
		if check.Name == "conflict" {
			return check
		}
	}
	t.Fatal("no conflict check is registered")
	return lint.Check{}
}
