package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
)

// healthy is the environment a freshly initialised bundle produces. Tests below
// perturb one field at a time, so a finding can only come from what they changed.
func healthy() lint.Environment {
	return lint.Environment{
		Bundle:           "/tmp/kb",
		OntologyPresent:  true,
		Types:            5,
		IndexDocPresent:  true,
		SchemaDocPresent: true,
		StateIgnored:     true,
		IndexPresent:     true,
		IndexVersion:     12,
		SchemaVersion:    12,
		Documents:        3,
		IndexedRows:      3,
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
		// §5.7 expects a schema document and it is generated, so its absence is a
		// command away — a warning rather than a block, like every other apparatus
		// file `doctor` reports.
		"no schema document": {
			mutate:       func(e *lint.Environment) { e.SchemaDocPresent = false },
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

// TestAnEditThatDidNothingIsReported. Both findings say the same thing in two
// registers — you changed a number and got no behaviour — and neither blocks,
// because the corpus remains entirely checkable either way.
func TestAnEditThatDidNothingIsReported(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		perturb func(*lint.Environment)
		want    string
	}{
		"a tuned value nothing reads": {
			func(e *lint.Environment) { e.TunedButUnread = []string{"in_degree_cut"} },
			"has no effect: in_degree_cut",
		},
		"a value pinned to another version": {
			func(e *lint.Environment) { e.MispinnedStandards = []string{"html_extractor_version"} },
			"provenance the file contradicts: html_extractor_version",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			env := healthy()
			tc.perturb(&env)
			got := lint.Diagnose(&env)
			if len(got) != 1 {
				t.Fatalf("got %d finding(s), want 1: %+v", len(got), got)
			}
			if got[0].Severity.Blocking() {
				t.Error("an ineffective edit blocked; the corpus is still checkable")
			}
			if !strings.Contains(got[0].Message, tc.want) {
				t.Errorf("message %q omits %q", got[0].Message, tc.want)
			}
		})
	}
}

// TestOnlyTheVocabularyAndTheIndexBlock pins the rule Diagnose states, because a
// wrong severity produces no failure anywhere — it produces a non-zero exit on a
// corpus with nothing wrong with it, which nobody notices until somebody's CI
// turns red for a reason they cannot act on.
//
// That is not hypothetical: `diagnoseStandards` carried SeverityError while its own
// comment said "It blocks nothing", and three places agreed with the comment. This
// test is what makes the fourth place agree out loud.
func TestOnlyTheVocabularyAndTheIndexBlock(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		perturb   func(*lint.Environment)
		wantBlock bool
	}{
		"no vocabulary":         {func(e *lint.Environment) { e.OntologyPresent = false }, true},
		"unparsable vocabulary": {func(e *lint.Environment) { e.OntologyError = "bad key" }, true},
		"no types":              {func(e *lint.Environment) { e.Types = 0 }, true},
		"an index from a newer gnosis": {
			func(e *lint.Environment) { e.IndexVersion = e.SchemaVersion + 1 }, true,
		},
		"an index missing schema objects": {
			func(e *lint.Environment) { e.SchemaMissing = []string{"documents"} }, true,
		},
		// Everything below leaves the corpus judgeable against its own rules.
		"unloadable standards": {
			func(e *lint.Environment) { e.StandardsError = "unrecognised key" }, false,
		},
		"no entry point":     {func(e *lint.Environment) { e.IndexDocPresent = false }, false},
		"no schema document": {func(e *lint.Environment) { e.SchemaDocPresent = false }, false},
		"state not ignored":  {func(e *lint.Environment) { e.StateIgnored = false }, false},
		"no index":           {func(e *lint.Environment) { e.IndexPresent = false }, false},
		"a drifted index":    {func(e *lint.Environment) { e.IndexedRows = 0 }, false},
		"a damaged audit trail": {
			func(e *lint.Environment) { e.Audit = lint.AuditHealth{Malformed: []int{2}} }, false,
		},
		"a tuned dead threshold": {
			func(e *lint.Environment) { e.TunedButUnread = []string{"in_degree_cut"} }, false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			env := healthy()
			tc.perturb(&env)

			var blocked bool
			for _, d := range lint.Diagnose(&env) {
				if d.Severity.Blocking() {
					blocked = true
				}
			}
			if blocked != tc.wantBlock {
				t.Errorf("blocking = %v, want %v", blocked, tc.wantBlock)
			}
		})
	}
}

// TestAnUnannouncedAuthorityIsReported is §10.6.3's other half — the one `adjudicate`
// cannot keep, because nothing announces a move caused by a hand-edited `verified`
// list, a merged branch, or a colleague's warrant arriving through `git pull`.
func TestAnUnannouncedAuthorityIsReported(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		env      lint.Environment
		reported bool
	}{
		"the log agrees with what the corpus derives": {
			env: lint.Environment{
				Authority: gnosis.AuthorityPaired,
				Announced: gnosis.AuthorityPaired, AnnouncedFound: true,
			},
		},
		// Every corpus before its first adjudication. §10.6.3 calls a single-curator
		// corpus supported rather than degenerate, and a finding here would say
		// otherwise on the day a bundle is created.
		"a fresh corpus at sole has announced nothing": {
			env: lint.Environment{Authority: gnosis.AuthoritySole},
		},
		"the authority moved and nothing said so": {
			env:      lint.Environment{Authority: gnosis.AuthorityPaired},
			reported: true,
		},
		"the log records something else": {
			env: lint.Environment{
				Authority: gnosis.AuthorityQuorum,
				Announced: gnosis.AuthorityPaired, AnnouncedFound: true,
			},
			reported: true,
		},
		// Scaling down is a move too: a corpus that stopped requiring a co-signer has
		// loosened, which is exactly what §10.6.3's "tightens or loosens" covers.
		"the authority fell and nothing said so": {
			env: lint.Environment{
				Authority: gnosis.AuthoritySole,
				Announced: gnosis.AuthorityQuorum, AnnouncedFound: true,
			},
			reported: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			found := firstOfCategory(lint.Diagnose(&tc.env), "authority")
			if tc.reported != (found != nil) {
				t.Fatalf("reported = %v, want %v", found != nil, tc.reported)
			}
			if found != nil {
				assertAuthorityFinding(t, found, tc.env.Authority)
			}
		})
	}
}

// firstOfCategory is the diagnostic a test is asking about, or nil.
func firstOfCategory(ds []finding.Diagnostic, category string) *finding.Diagnostic {
	for i := range ds {
		if ds[i].Category == category {
			return &ds[i]
		}
	}
	return nil
}

// assertAuthorityFinding checks what the finding has to say to be worth emitting.
//
// The severity is part of the assertion rather than an afterthought: a colleague's
// warrant arriving through `git pull` moves the authority, and a corpus that failed its
// build on that would teach people to stop pulling.
func assertAuthorityFinding(
	t *testing.T, d *finding.Diagnostic, derived gnosis.Authority,
) {
	t.Helper()

	if d.Severity != finding.SeverityWarning {
		t.Errorf("severity = %v; a colleague's arrival must not fail a build", d.Severity)
	}
	if !strings.Contains(d.Message, derived.String()) {
		t.Errorf("the finding does not name what the corpus derives: %q", d.Message)
	}
}
