package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
)

// TestEverySkipCarriesAReason is the guarantee three other repositories now cite.
//
// `skillet` resolved its open decision on naming a general `Applicability` type as
// **a rule rather than a type**, and the rule is this package's own sentence: a
// check that silently declines to run is indistinguishable from a check that found
// nothing. `skillsaw`, `adh`, and `canonizer` all point at `internal/lint` as the
// reference, and the part that makes it a fuller implementation than the prior art
// it was credited to is that the *reason* is a first-class output rather than an
// inference from a missing finding.
//
// Until now that lived in prose and in Run's six lines. The failure it does not
// catch is `Applies` returning `(false, "")` — a check that declines and says
// nothing, which produces a Skip a consumer cannot render and cannot act on. That
// is a behavioural property, not a type-system one: the compiler is perfectly happy
// with an empty string.
//
// The registry is constructed here rather than taken from Checks, because the
// property has to hold for a check that does not exist yet.
func TestEverySkipCarriesAReason(t *testing.T) {
	t.Parallel()

	const name = "always-inapplicable"
	registry := []lint.Check{{
		Name:       name,
		Categories: []string{"probe"},
		Applies: func(*lint.Snapshot) (bool, string) {
			return false, "this check never applies, and here is why"
		},
		Run: func(*lint.Snapshot) []finding.Diagnostic {
			t.Error("a check that does not apply was run anyway")
			return nil
		},
	}}

	report := lint.Run(&lint.Snapshot{}, registry)
	if len(report.Skipped) != 1 {
		t.Fatalf("Skipped = %+v, want exactly one entry", report.Skipped)
	}
	if report.Skipped[0].Check != name {
		t.Errorf("the skip does not name the check: %+v", report.Skipped[0])
	}
	if report.Skipped[0].Reason == "" {
		t.Error("the skip carries no reason; a consumer cannot tell a suppressed " +
			"check from a clean one")
	}
	if len(report.Diagnostics) != 0 {
		t.Errorf("a skipped check produced diagnostics: %+v", report.Diagnostics)
	}
}

// TestNoRealCheckSkipsWithoutSayingWhy walks the shipped registry over a corpus
// designed to make as many checks decline as possible: empty, no links, no claims,
// no index, no log. Nearly everything should skip, and every skip must carry a
// reason.
//
// This is the direction that actually rots. A check added later with
// `Applies: func(*Snapshot) (bool, string) { return false, "" }` compiles, runs, and
// silently suppresses itself, and no other test in this package would notice.
func TestNoRealCheckSkipsWithoutSayingWhy(t *testing.T) {
	t.Parallel()

	report := lint.Run(&lint.Snapshot{}, lint.Checks(testNow()))
	if len(report.Skipped) == 0 {
		t.Fatal("nothing skipped on an empty corpus; this test is not exercising the path")
	}
	for _, skip := range report.Skipped {
		if strings.TrimSpace(skip.Reason) == "" {
			t.Errorf("%s skipped with no reason", skip.Check)
		}
		if skip.Check == "" {
			t.Errorf("a skip names no check: %+v", skip)
		}
	}
}

// TestSkippedIsNeverNil, because a consumer that had to distinguish "nothing
// skipped" from "no result" would eventually get it wrong in the direction that
// omits the skip report — which §12 makes mandatory precisely because a corpus can
// lint clean by not applying.
func TestSkippedIsNeverNil(t *testing.T) {
	t.Parallel()

	// A corpus where every check applies: linked documents, an index, a log.
	snap := linkedCorpus()
	snap.HasIndex = true
	snap.HasLog = true

	report := lint.Run(snap, lint.Checks(testNow()))
	if report.Skipped == nil {
		t.Error("Skipped is nil rather than empty")
	}
	if report.Diagnostics == nil {
		t.Error("Diagnostics is nil rather than empty")
	}
}
