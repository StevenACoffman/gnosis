package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
)

// healthy is the environment a freshly initialised bundle produces. Tests below
// perturb one field at a time, so a finding can only come from what they changed.
func healthy() lint.Environment {
	return lint.Environment{
		Bundle:          "/tmp/kb",
		OntologyPresent: true,
		Types:           5,
		IndexDocPresent: true,
		StateIgnored:    true,
		IndexPresent:    true,
		IndexVersion:    12,
		SchemaVersion:   12,
		Documents:       3,
		IndexedRows:     3,
	}
}

// TestHealthyEnvironmentHasNoFindings is the baseline the rest depend on: if a
// correctly set-up bundle produced findings, every other assertion here would be
// measuring noise.
func TestHealthyEnvironmentHasNoFindings(t *testing.T) {
	t.Parallel()
	env := healthy()
	if got := lint.Diagnose(&env); len(got) != 0 {
		t.Errorf("a healthy environment produced %d finding(s): %+v", len(got), got)
	}
}

// TestBlockingConditions pins which conditions stop a caller. Only two do, and
// both are cases where continuing would mean judging the corpus against
// something other than its own rules.
func TestBlockingConditions(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		mutate       func(*lint.Environment)
		wantBlocking bool
	}{
		"no vocabulary": {
			mutate:       func(e *lint.Environment) { e.OntologyPresent = false; e.Types = 0 },
			wantBlocking: true,
		},
		"unloadable vocabulary": {
			mutate:       func(e *lint.Environment) { e.OntologyError = "line 4: bad key" },
			wantBlocking: true,
		},
		"vocabulary with no types": {
			mutate:       func(e *lint.Environment) { e.Types = 0 },
			wantBlocking: true,
		},
		"index from a newer gnosis": {
			mutate:       func(e *lint.Environment) { e.IndexVersion = 13 },
			wantBlocking: true,
		},
		"no index yet": {
			mutate:       func(e *lint.Environment) { e.IndexPresent = false; e.IndexedRows = 0 },
			wantBlocking: false,
		},
		"stale index": {
			mutate:       func(e *lint.Environment) { e.IndexedRows = 1 },
			wantBlocking: false,
		},
		"no entry point": {
			mutate:       func(e *lint.Environment) { e.IndexDocPresent = false },
			wantBlocking: false,
		},
		"derived state not ignored": {
			mutate:       func(e *lint.Environment) { e.StateIgnored = false },
			wantBlocking: false,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			env := healthy()
			tc.mutate(&env)
			assertReported(t, name, lint.Diagnose(&env), tc.wantBlocking)
		})
	}
}

// assertReported checks that a perturbed environment produced findings, that
// every one of them says something, and that blocking matches expectation.
func assertReported(t *testing.T, name string, got []finding.Diagnostic, wantBlocking bool) {
	t.Helper()
	if len(got) == 0 {
		t.Fatalf("%q produced no finding", name)
	}
	var blocking bool
	for _, d := range got {
		if d.Severity.Blocking() {
			blocking = true
		}
		if d.Message == "" {
			t.Errorf("%q produced a finding with no message: %+v", name, d)
		}
	}
	if blocking != wantBlocking {
		t.Errorf("%q blocking = %v, want %v: %+v", name, blocking, wantBlocking, got)
	}
}

// TestAnOlderIndexIsNotAFinding: Open migrates a database forward, so an index
// behind the binary is already repaired by the time anything looks at it.
// Reporting it would send a reader to fix something that has been fixed.
func TestAnOlderIndexIsNotAFinding(t *testing.T) {
	t.Parallel()
	env := healthy()
	env.IndexVersion = 11

	for _, d := range lint.Diagnose(&env) {
		if d.Category == "index" {
			t.Errorf("an index older than the binary was reported: %+v", d)
		}
	}
}

// TestVocabularyErrorIsCarriedVerbatim: the loader's diagnostic already names
// the offending key, and paraphrasing it would lose the only locating detail.
func TestVocabularyErrorIsCarriedVerbatim(t *testing.T) {
	t.Parallel()
	env := healthy()
	env.OntologyError = `unrecognised key(s): types.normatve`

	got := lint.Diagnose(&env)
	if len(got) == 0 {
		t.Fatal("an unloadable vocabulary produced no finding")
	}
	if !strings.Contains(got[0].Message, "types.normatve") {
		t.Errorf("message %q drops the key the loader named", got[0].Message)
	}
}

// TestOnlyOneVocabularyFinding: the vocabulary conditions are ordered, so a
// missing file does not also report zero types. Two findings for one cause would
// double-count in any summary.
func TestOnlyOneVocabularyFinding(t *testing.T) {
	t.Parallel()
	env := healthy()
	env.OntologyPresent = false
	env.OntologyError = "unread"
	env.Types = 0

	var n int
	for _, d := range lint.Diagnose(&env) {
		if d.Category == "vocabulary" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("got %d vocabulary findings for one cause, want 1", n)
	}
}
