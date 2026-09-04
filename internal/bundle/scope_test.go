package bundle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// scopeBundle files three concepts differing in type, status and trust.
func scopeBundle(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "c", "sub"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, body := range map[string]string{
		"c/rule.md": "---\ntype: Rule\ntitle: Retry Cap\n" +
			"gnosis_id: 01932b7c-1f4e-7a3d-9c2b-000000000001\n" +
			"gnosis_claims:\n  - id: c1\n    anchor: Capped at three.\n" +
			"    verified:\n      - by: human:priya\n        at: 2026-08-01T00:00:00Z\n" +
			"---\nBody.\n",
		"c/reference.md": "---\ntype: Reference\ntitle: Queue Drain\n" +
			"gnosis_id: 01932b7c-1f4e-7a3d-9c2b-000000000002\n" +
			"status: deprecated\n" +
			"gnosis_claims:\n  - id: c1\n    anchor: Drains in order.\n" +
			"    verified:\n      - by: process:nightly\n        at: 2026-08-01T00:00:00Z\n" +
			"---\nBody.\n",
		"c/sub/nested.md": "---\ntype: Rule\ntitle: Nested\n" +
			"gnosis_id: 01932b7c-1f4e-7a3d-9c2b-000000000003\n" +
			"---\nBody.\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)),
			[]byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// TestScopeRestrictsBySixIndependentFilters walks §11.3's list, and the trust case is
// the one only a fold can answer: the index holds no tier.
func TestScopeRestrictsBySixIndependentFilters(t *testing.T) {
	t.Parallel()

	dir := scopeBundle(t)
	cases := map[string]struct {
		query   bundle.ScopeQuery
		admits  []string
		refuses []string
	}{
		"unrestricted admits everything, including a path the corpus lost": {
			query:  bundle.ScopeQuery{},
			admits: []string{"c/rule.md", "c/reference.md", "c/gone.md"},
		},
		"by type": {
			query:   bundle.ScopeQuery{Type: "Rule"},
			admits:  []string{"c/rule.md", "c/sub/nested.md"},
			refuses: []string{"c/reference.md"},
		},
		// A document declaring no status is stable, which OKF reads as current.
		"by status, defaulting the absent one": {
			query:   bundle.ScopeQuery{Status: "stable"},
			admits:  []string{"c/rule.md", "c/sub/nested.md"},
			refuses: []string{"c/reference.md"},
		},
		"by declared status": {
			query:   bundle.ScopeQuery{Status: "deprecated"},
			admits:  []string{"c/reference.md"},
			refuses: []string{"c/rule.md"},
		},
		"by subtree": {
			query:   bundle.ScopeQuery{Under: "c/sub"},
			admits:  []string{"c/sub/nested.md"},
			refuses: []string{"c/rule.md", "c/reference.md"},
		},
		"by trust, at or above": {
			query:   bundle.ScopeQuery{Trust: gnosis.TierMachineConfirmed, Trusted: true},
			admits:  []string{"c/rule.md", "c/reference.md"},
			refuses: []string{"c/sub/nested.md"},
		},
		"by trust, the top tier only": {
			query:   bundle.ScopeQuery{Trust: gnosis.TierHumanReviewed, Trusted: true},
			admits:  []string{"c/rule.md"},
			refuses: []string{"c/reference.md", "c/sub/nested.md"},
		},
		// A conjunction: naming two means both.
		"two filters at once": {
			query:   bundle.ScopeQuery{Type: "Rule", Under: "c/sub"},
			admits:  []string{"c/sub/nested.md"},
			refuses: []string{"c/rule.md"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scope, err := bundle.LoadScope(dir, tc.query)
			if err != nil {
				t.Fatalf("LoadScope: %v", err)
			}
			assertScope(t, scope, tc.admits, tc.refuses)
		})
	}
}

// assertScope checks both directions of one case.
//
// A helper because the table's own loop was past the complexity limit, and it earns
// being one: both directions are the assertion — a filter that admitted nothing would
// pass every "refuses" list, and one that admitted everything would pass every "admits"
// list. Checking one alone is how a broken filter looks correct.
func assertScope(t *testing.T, scope *bundle.SearchScope, admits, refuses []string) {
	t.Helper()

	for _, path := range admits {
		if !scope.Admits(path) {
			t.Errorf("%s was refused", path)
		}
	}
	for _, path := range refuses {
		if scope.Admits(path) {
			t.Errorf("%s was admitted", path)
		}
	}
}

// TestASubtreeQueryNeverReturnsAConceptOutsideIt is §11.3's own words, and it is a
// property rather than an example: a string prefix would admit a sibling whose name
// begins with the same letters, which is a concept outside the subtree returned by a
// query that claimed to be restricted to it.
func TestASubtreeQueryNeverReturnsAConceptOutsideIt(t *testing.T) {
	t.Parallel()

	dir := scopeBundle(t)
	scope, err := bundle.LoadScope(dir, bundle.ScopeQuery{Under: "c/sub"})
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	// Every path the corpus holds, and every path it does not: nothing outside the
	// prefix may be admitted, whatever it is called.
	for _, path := range []string{
		"c/rule.md", "c/reference.md", "c/subtle.md", "c/sub-other/x.md",
		"c/sub.md", "other/sub/x.md", "", "c",
	} {
		if scope.Admits(path) {
			t.Errorf("%q is outside c/sub and was admitted", path)
		}
	}
	if !scope.Admits("c/sub/nested.md") {
		t.Error("a document inside the subtree was refused")
	}
}

// TestAnUnrestrictedScopeReadsNothing. A plain search must still touch only the index,
// so the scope is free when no filter was named — asserted through a bundle path that
// does not exist, which any read would fail on.
func TestAnUnrestrictedScopeReadsNothing(t *testing.T) {
	t.Parallel()

	scope, err := bundle.LoadScope(filepath.Join(t.TempDir(), "absent"),
		bundle.ScopeQuery{})
	if err != nil {
		t.Fatalf("an unrestricted scope read the bundle: %v", err)
	}
	if scope.Restricted() || !scope.Admits("c/anything.md") {
		t.Error("an unrestricted scope filtered something")
	}
}

// TestParseScopeQueryRefusesATierNobodyDeclared, naming the three. §14.1's tiers are
// gnosis's own closed set, so a fourth word is a mistake worth catching — while the OKF
// status vocabulary is not closed here, because §11 forbids refusing a corpus for using
// its own.
func TestParseScopeQueryRefusesATierNobodyDeclared(t *testing.T) {
	t.Parallel()

	_, err := bundle.ParseScopeQuery("", "", "", "reviewed", false, false)
	if errs.ErrorCode(err) != errs.EINVALID {
		t.Fatalf("an unknown tier was accepted: %v", err)
	}
	for _, tier := range gnosis.Tiers() {
		if !strings.Contains(err.Error(), tier) {
			t.Errorf("the refusal does not name %q: %v", tier, err)
		}
	}
	// A status nobody has heard of is accepted, and compares to nothing.
	q, err := bundle.ParseScopeQuery("", "provisional", "", "", false, false)
	if err != nil || q.Status != "provisional" {
		t.Errorf("an OKF status this build does not know was refused: %v", err)
	}
}
