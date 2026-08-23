package lint_test

import (
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
)

func at(day int) time.Time {
	return time.Date(2026, 8, day, 12, 0, 0, 0, time.UTC)
}

// auditOnly returns the diagnostics in the audit category, so a failure names this
// check rather than whatever else the environment reported.
func auditOnly(t *testing.T, h *lint.AuditHealth) []finding.Diagnostic {
	t.Helper()
	env := healthy()
	env.Audit = *h
	var out []finding.Diagnostic
	for _, d := range lint.Diagnose(&env) {
		if d.Category == "audit" {
			out = append(out, d)
		}
	}
	return out
}

func TestDiagnoseAudit(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		health lint.AuditHealth
		want   string // "" for silence, else a fragment of the message
	}{
		// The ordinary state, and it must be silent: a corpus with a healthy trail
		// is most corpora most of the time.
		"rows and no damage": {
			lint.AuditHealth{Rows: 12, Newest: at(22), Head: at(21)}, "",
		},
		"a malformed line": {
			lint.AuditHealth{Rows: 11, Malformed: []int{4}, Newest: at(22), Head: at(21)},
			"1 of 12 line(s)",
		},
		"several malformed lines name each": {
			lint.AuditHealth{Rows: 9, Malformed: []int{2, 7}, Newest: at(22), Head: at(21)},
			"line 2, 7",
		},
		// Neither direction is a finding. A commit newer than the last row is what
		// somebody editing a document by hand and committing it produces, which is
		// the ordinary use of a plain-text corpus — see AuditHealth.Head.
		"a commit newer than the last row": {
			lint.AuditHealth{Rows: 3, Newest: at(10), Head: at(20)}, "",
		},
		"a row newer than the last commit": {
			lint.AuditHealth{Rows: 3, Newest: at(20), Head: at(10)}, "",
		},
		"the trail could not be read": {
			lint.AuditHealth{Unreadable: "permission denied"}, "could not be read",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := auditOnly(t, &tc.health)
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("wanted silence, got %+v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("got %d diagnostics, want 1: %+v", len(got), got)
			}
			if !strings.Contains(got[0].Message, tc.want) {
				t.Errorf("message %q omits %q", got[0].Message, tc.want)
			}
		})
	}
}

// TestNoTimestampComparisonFires. §15 asks for the newest row to be compared
// against the newest commit; running it showed the comparison cannot mean what the
// section wants, because a person editing a document and committing it produces a
// commit newer than any audit row with nothing wrong. Every ordering is silent, and
// this is the test that keeps somebody from adding the check back without reading
// why it went.
//
// The failure §15 wants caught is caught by verifying each row after the append,
// which happens at the moment of the write rather than being inferred later.
func TestNoTimestampComparisonFires(t *testing.T) {
	t.Parallel()
	cases := map[string]lint.AuditHealth{
		"a commit long after the last row": {Rows: 3, Newest: at(1), Head: at(28)},
		"no commits at all":                {Rows: 3, Newest: at(20)},
		"no rows at all":                   {Head: at(20)},
		"neither is known":                 {},
		"a fresh empty bundle":             {Rows: 0},
	}
	for name, health := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := auditOnly(t, &health); len(got) != 0 {
				t.Errorf("a timestamp ordering produced %+v", got)
			}
		})
	}
}

// TestADamagedTrailDoesNotBlock. Diagnose blocks only where continuing would mean
// judging the corpus against something other than its own rules. A damaged trail
// makes the corpus's history unrecountable and leaves the corpus itself perfectly
// checkable, so blocking would fail `doctor` on a corpus with nothing wrong.
func TestADamagedTrailDoesNotBlock(t *testing.T) {
	t.Parallel()
	for name, health := range map[string]lint.AuditHealth{
		"malformed lines": {Rows: 1, Malformed: []int{2}},
		"unreadable":      {Unreadable: "permission denied"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, d := range auditOnly(t, &health) {
				if d.Severity.Blocking() {
					t.Errorf("%q blocked: %s", name, d.Message)
				}
			}
		})
	}
}

// TestAnUnreadableTrailReportsOnce. Reporting "0 rows" alongside is an observation
// nobody made, and it is the shape that let a malformed standards file produce a
// clean bill of health once already.
func TestAnUnreadableTrailReportsOnce(t *testing.T) {
	t.Parallel()
	got := auditOnly(t, &lint.AuditHealth{Unreadable: "permission denied", Head: at(20)})
	if len(got) != 1 {
		t.Errorf("got %d diagnostics, want only the read failure: %+v", len(got), got)
	}
}
