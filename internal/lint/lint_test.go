package lint_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
)

const (
	idA = gnosis.ID("01932b7c-1f4e-7a3d-9c2b-000000000001")
	idB = gnosis.ID("01932b7c-1f4e-7a3d-9c2b-000000000002")
)

// linkedCorpus has one internal link, so the link-graph checks apply.
func linkedCorpus() *lint.Snapshot {
	return &lint.Snapshot{
		Documents: []lint.Document{
			{ID: idA, Path: "c/a.md", Type: "Reference", Title: "A"},
			{ID: idB, Path: "c/b.md", Type: "Reference", Title: "B"},
		},
		Links: []lint.Link{{FromID: idA, ToID: idB, Href: "/c/b.md"}},
	}
}

// TestLinkChecksAreSkippedBeforeTheCorpusHasLinks is the derived-applicability
// property from SPEC §12. In a corpus with no internal links every document is
// trivially an orphan, and reporting that on day one would teach a reader to
// ignore the check.
func TestLinkChecksAreSkippedBeforeTheCorpusHasLinks(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Documents: []lint.Document{{ID: idA, Path: "c/a.md", Type: "Reference"}},
	}
	report := lint.Run(snap, lint.Checks(testNow()))

	skipped := map[string]string{}
	for _, s := range report.Skipped {
		skipped[s.Check] = s.Reason
	}
	for _, name := range []string{"orphan", "broken-link"} {
		reason, ok := skipped[name]
		if !ok {
			t.Errorf("%s ran on a corpus with no links; it should be skipped", name)
			continue
		}
		if reason == "" {
			t.Errorf("%s was skipped without a reason", name)
		}
	}
	for _, d := range report.Diagnostics {
		if d.Category == "orphan" || d.Category == "broken-link" {
			t.Errorf("skipped check still produced %+v", d)
		}
	}
}

// TestSkipsAreAlwaysReported guards the other half of §12: a check that
// silently declines is indistinguishable from one that found nothing.
func TestSkipsAreAlwaysReported(t *testing.T) {
	t.Parallel()
	report := lint.Run(&lint.Snapshot{}, lint.Checks(testNow()))
	if len(report.Skipped) == 0 {
		t.Fatal("an empty corpus skipped no checks; expected the link and log checks")
	}
	for _, s := range report.Skipped {
		if s.Check == "" || s.Reason == "" {
			t.Errorf("skip entry is incomplete: %+v", s)
		}
	}
}

// TestBrokenLinkIsNeverAnError is the OKF §6.1 property. A link to a document
// that does not exist "may simply represent not-yet-written knowledge", so
// promoting it to an error would block a corpus for having a gap in it.
func TestBrokenLinkIsNeverAnError(t *testing.T) {
	t.Parallel()
	snap := linkedCorpus()
	snap.Links = append(snap.Links, lint.Link{FromID: idA, Href: "/c/missing.md"})

	report := lint.Run(snap, lint.Checks(testNow()))

	var found bool
	for _, d := range report.Diagnostics {
		if d.Category != "broken-link" {
			continue
		}
		found = true
		if d.Severity != finding.SeverityWarning {
			t.Errorf("broken-link severity = %q, want %q", d.Severity, finding.SeverityWarning)
		}
	}
	if !found {
		t.Error("no broken-link diagnostic for a link resolving to nothing")
	}
}

// TestExternalLinksAreNotBroken checks the corpus does not report on targets it
// has no business resolving.
func TestExternalLinksAreNotBroken(t *testing.T) {
	t.Parallel()
	snap := linkedCorpus()
	snap.Links = append(snap.Links,
		lint.Link{FromID: idA, Href: "https://example.org", External: true})

	for _, d := range lint.Run(snap, lint.Checks(testNow())).Diagnostics {
		if d.Category == "broken-link" {
			t.Errorf("external link reported as broken: %+v", d)
		}
	}
}

// TestDuplicateIdentityIsTheOnlyBlockingIdentityFinding pins the severity
// policy: a duplicate is where acting automatically would discard somebody's
// work, so it must stop a caller that gates on blocking findings, and the other
// outcomes must not.
func TestDuplicateIdentityIsTheOnlyBlockingIdentityFinding(t *testing.T) {
	t.Parallel()
	tests := map[gnosis.Kind]bool{
		gnosis.KindDuplicate:  true,
		gnosis.KindConflict:   true,
		gnosis.KindQuarantine: false,
		gnosis.KindUpdatePath: false,
		gnosis.KindIndex:      false,
		gnosis.KindTombstone:  false,
	}
	for kind, wantBlocking := range tests {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			// HasIndex is set so that both the identity check and the
			// index-drift check are live; which of the two owns a kind is the
			// subject of TestIndexRelativeFindingsNeedAnIndex, not of this test.
			snap := &lint.Snapshot{HasIndex: true, Resolutions: []gnosis.Resolution{{
				Kind: kind, ID: idA, Paths: []string{"c/a.md", "c/copy.md"}, Other: idB,
			}}}
			report := lint.Run(snap, lint.Checks(testNow()))
			if len(report.Diagnostics) != 1 {
				t.Fatalf(
					"got %d diagnostics, want 1: %+v",
					len(report.Diagnostics),
					report.Diagnostics,
				)
			}
			d := report.Diagnostics[0]
			if got := d.Severity.Blocking(); got != wantBlocking {
				t.Errorf(
					"%s blocking = %v, want %v (severity %q)",
					kind,
					got,
					wantBlocking,
					d.Severity,
				)
			}
			if d.Message == "" {
				t.Error("diagnostic carries no message")
			}
		})
	}
}

// TestDuplicateMessageNamesEveryPath: a reader adjudicating a collision needs
// both copies. Naming only one would invite acting on the wrong file.
func TestDuplicateMessageNamesEveryPath(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{Resolutions: []gnosis.Resolution{{
		Kind: gnosis.KindDuplicate, ID: idA,
		Paths: []string{"c/first.md", "c/second.md"},
	}}}
	msg := lint.Run(snap, lint.Checks(testNow())).Diagnostics[0].Message
	for _, want := range []string{"c/first.md", "c/second.md", "no winner"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not mention %q", msg, want)
		}
	}
}

func TestLogFormat(t *testing.T) {
	t.Parallel()
	t.Run("absent log is not a finding", func(t *testing.T) {
		t.Parallel()
		report := lint.Run(&lint.Snapshot{HasLog: false}, lint.Checks(testNow()))
		for _, d := range report.Diagnostics {
			if d.Category == "log-format" {
				t.Errorf("absent log produced %+v", d)
			}
		}
	})
	t.Run("malformed heading is reported", func(t *testing.T) {
		t.Parallel()
		snap := &lint.Snapshot{HasLog: true, LogLines: []string{
			"# Update Log", "## 2026-08-19", "* did a thing", "## August 19th", "* another",
		}}
		report := lint.Run(snap, lint.Checks(testNow()))
		if len(report.Diagnostics) != 1 {
			t.Fatalf("got %d diagnostics, want 1: %+v", len(report.Diagnostics), report.Diagnostics)
		}
		if report.Diagnostics[0].Path != "log.md:4" {
			t.Errorf("path = %q, want log.md:4", report.Diagnostics[0].Path)
		}
	})
}

// TestRunIsDeterministic pins SPEC §18.3 for the lint surface.
func TestRunIsDeterministic(t *testing.T) {
	t.Parallel()
	snap := linkedCorpus()
	snap.HasIndex = true
	snap.Links = append(snap.Links, lint.Link{FromID: idB, Href: "/c/gone.md"})
	snap.Resolutions = []gnosis.Resolution{
		{Kind: gnosis.KindTombstone, ID: idB, Paths: []string{"c/x.md"}},
		{Kind: gnosis.KindIndex, ID: idA, Paths: []string{"c/a.md"}},
	}

	first := lint.Run(snap, lint.Checks(testNow()))
	for range 20 {
		if got := lint.Run(snap, lint.Checks(testNow())); !reflect.DeepEqual(got, first) {
			t.Fatalf("output varies between runs:\n got %+v\nfirst %+v", got, first)
		}
	}
}

// TestIndexRelativeFindingsNeedAnIndex pins the split between the two identity
// checks. Before `index rebuild` has ever run, every document is trivially
// absent from the index, and one finding per document would teach a reader to
// ignore the check. What the files say about themselves is reported regardless,
// because deleting the index does not make a duplicate identifier stop being
// one.
func TestIndexRelativeFindingsNeedAnIndex(t *testing.T) {
	t.Parallel()
	resolutions := []gnosis.Resolution{
		{Kind: gnosis.KindIndex, ID: idA, Paths: []string{"c/a.md"}},
		{Kind: gnosis.KindDuplicate, ID: idB, Paths: []string{"c/b.md", "c/copy.md"}},
	}

	t.Run("without an index", func(t *testing.T) {
		t.Parallel()
		report := lint.Run(&lint.Snapshot{Resolutions: resolutions}, lint.Checks(testNow()))
		if len(report.Diagnostics) != 1 {
			t.Fatalf("got %d diagnostics, want only the duplicate: %+v",
				len(report.Diagnostics), report.Diagnostics)
		}
		if !report.Diagnostics[0].Severity.Blocking() {
			t.Error("the duplicate stopped blocking when the index went away")
		}
		if !skipped(report, "index-drift") {
			t.Error("index-drift did not report why it was skipped")
		}
	})

	t.Run("with an index", func(t *testing.T) {
		t.Parallel()
		snap := &lint.Snapshot{HasIndex: true, Resolutions: resolutions}
		report := lint.Run(snap, lint.Checks(testNow()))
		if len(report.Diagnostics) != 2 {
			t.Fatalf("got %d diagnostics, want 2: %+v",
				len(report.Diagnostics), report.Diagnostics)
		}
		if skipped(report, "index-drift") {
			t.Error("index-drift was skipped even though the bundle has an index")
		}
	})
}

// skipped reports whether the named check appears in the report's skip list.
func skipped(report lint.Report, check string) bool {
	for _, s := range report.Skipped {
		if s.Check == check {
			return s.Reason != ""
		}
	}
	return false
}

// testNow is a fixed clock for the registry, so a check that reads one is pinned
// rather than asserted against a range. The date is arbitrary and chosen only to
// be far from any fixture's boundary.
func testNow() time.Time {
	return time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
}

// TestEveryCheckDeclaresValidActions is half of what makes the Actions field
// evidence: a declaration that is empty, or that names something `finding` does not
// define, would put a word in §12.1's table that no reader could act on.
func TestEveryCheckDeclaresValidActions(t *testing.T) {
	t.Parallel()

	for _, c := range lint.Checks(time.Now()) {
		if len(c.Actions) == 0 {
			t.Errorf("%s declares no actions", c.Name)
		}
		for _, a := range c.Actions {
			if !a.Valid() {
				t.Errorf("%s declares %q, which finding does not define", c.Name, a)
			}
		}
	}
}

// TestEveryEmittedActionWasDeclared is the other half, and its limit is worth
// stating: it is only as complete as the snapshot below makes checks fire.
//
// That is the same partial guarantee `Categories` has, for the same reason — an action
// is a field inside a `Run` body, so nothing can enumerate it without running the
// check. What this rules out is the drift that actually happens: somebody changes a
// diagnostic's action and leaves the declaration, so §12.1's table says a tool could
// fix something a person now has to.
func TestEveryEmittedActionWasDeclared(t *testing.T) {
	t.Parallel()

	checks := lint.Checks(time.Now())
	declared := map[string]map[finding.Action]bool{}
	for _, c := range checks {
		for _, category := range c.Categories {
			set := map[finding.Action]bool{}
			for _, a := range c.Actions {
				set[a] = true
			}
			declared[category] = set
		}
	}

	report := lint.Run(brokenCorpus(), checks)
	if len(report.Diagnostics) == 0 {
		t.Fatal("the fixture fired no checks, so this test asserts nothing")
	}
	for _, d := range report.Diagnostics {
		set, known := declared[d.Category]
		if !known {
			// A category emitted and not declared: the Categories test's subject,
			// asserted here too because this loop is where it becomes visible.
			t.Errorf("%q was emitted and no check declares it", d.Category)
			continue
		}
		if !set[d.Action] {
			t.Errorf("%q emitted action %q, which its check does not declare",
				d.Category, d.Action)
		}
	}
}

// brokenCorpus is a snapshot arranged to trip as many checks as one snapshot can.
//
// Deliberately not a realistic corpus: every field here is set to whatever makes a
// check fire, which is the opposite of a fixture that demonstrates ordinary use. Its
// only job is to produce diagnostics whose actions can be compared against what was
// declared.
func brokenCorpus() *lint.Snapshot {
	return &lint.Snapshot{
		Documents: []lint.Document{
			// No type: conformance. An empty section: empty-section.
			{ID: idA, Path: "c/a.md", Title: "A", Body: "# A\n\n## Empty\n"},
			// An unfilled template marker: placeholder. `{{NAME}}` rather than a
			// "TODO", because that is the form the check looks for — the first
			// version of this fixture wrote TODO and fired nothing.
			{
				ID: idB, Path: "c/b.md", Title: "B", Type: "Reference",
				Body: "# B\n\nSee {{OWNER}} for details.\n",
			},
		},
		// Supplied directly rather than derived: `Resolutions` is `gnosis.Reconcile`'s
		// output and the snapshot is a value, so nothing computes it here. The
		// duplicate is the case worth covering — `identity` is the only check
		// declaring two actions, and the duplicate is the one that escalates to a
		// person.
		Resolutions: []gnosis.Resolution{
			{Kind: gnosis.KindDuplicate, ID: idA, Paths: []string{"c/a.md", "c/b.md"}},
			{Kind: gnosis.KindUpdatePath, ID: idB, Paths: []string{"c/b.md"}},
		},
		Links:    []lint.Link{{FromID: idA, Href: "/c/gone.md"}},
		LogLines: []string{"# Update Log", "", "## not-a-date", "* something"},
		HasLog:   true,
	}
}
