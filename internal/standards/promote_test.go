package standards_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/standards"
)

// TestPromoteSeedLoads is the one test this file cannot do without: a seed its own
// loader rejects would break every bundle created from it.
func TestPromoteSeedLoads(t *testing.T) {
	t.Parallel()
	p, err := standards.LoadPromote(standards.DefaultPromote())
	if err != nil {
		t.Fatalf("the embedded seed does not load: %v", err)
	}
	if p.HedgingMax.Value != 3 {
		t.Errorf("hedging_max = %d, want 3", p.HedgingMax.Value)
	}
	if p.RebuildFloorFraction.Value != 0.5 {
		t.Errorf("rebuild_floor_fraction = %v, want 0.5", p.RebuildFloorFraction.Value)
	}
}

// TestPromoteSeedIsACopy: a caller that edits the returned bytes must not corrupt
// the seed for the next one.
func TestPromoteSeedIsACopy(t *testing.T) {
	t.Parallel()
	first := standards.DefaultPromote()
	first[0] = 'X'
	if second := standards.DefaultPromote(); second[0] == 'X' {
		t.Error("DefaultPromote returns the package's own slice")
	}
}

// TestPromoteRationaleIsRequired. hedging_max spent its whole life as a literal in
// Go with no justification, which is the state §6.5 forbids precisely because
// nobody can tell a measured threshold from an invented one afterwards.
func TestPromoteRationaleIsRequired(t *testing.T) {
	t.Parallel()
	// A file declaring a value and no rationale at all.
	bare := "[hedging_max]\nvalue = 3\n\n[rebuild_floor_fraction]\nvalue = 0.5\n" +
		"rationale = \"why\"\n"

	_, err := standards.LoadPromote([]byte(bare))
	if err == nil {
		t.Fatal("a value with no rationale loaded")
	}
	if !strings.Contains(err.Error(), "hedging_max") {
		t.Errorf("the error does not name the offending key: %v", err)
	}
}

// TestPromoteRejectsUnrecognisedKeys, because a mistyped key would leave a
// threshold the author believes they changed.
func TestPromoteRejectsUnrecognisedKeys(t *testing.T) {
	t.Parallel()
	src := string(standards.DefaultPromote()) +
		"\n[hedging_maxx]\nvalue = 9\nrationale = \"typo\"\n"

	_, err := standards.LoadPromote([]byte(src))
	if err == nil {
		t.Fatal("a mistyped key loaded silently")
	}
	if !strings.Contains(err.Error(), "hedging_maxx") {
		t.Errorf("the error does not name it: %v", err)
	}
}

func TestPromoteValidation(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		old, new string
		want     string
	}{
		"negative hedging": {"value = 3", "value = -1", "hedging_max"},
		"zero floor":       {"value = 0.5", "value = 0", "rebuild_floor_fraction"},
		"floor above one":  {"value = 0.5", "value = 2", "rebuild_floor_fraction"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := strings.Replace(string(standards.DefaultPromote()), tc.old, tc.new, 1)
			if src == string(standards.DefaultPromote()) {
				t.Fatalf("the fixture no longer matches the seed: %q", tc.old)
			}
			_, err := standards.LoadPromote([]byte(src))
			if err == nil {
				t.Fatalf("%s loaded", name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// TestZeroHedgingIsAllowed. A limit of zero means no softening phrase at all is
// tolerated, which is a coherent thing to ask for even if nobody should, and the
// validation must not confuse "strict" with "unintended".
func TestZeroHedgingIsAllowed(t *testing.T) {
	t.Parallel()
	src := strings.Replace(string(standards.DefaultPromote()), "value = 3", "value = 0", 1)

	if _, err := standards.LoadPromote([]byte(src)); err != nil {
		t.Errorf("a hedging limit of zero was rejected: %v", err)
	}
}

// TestPromoteLoosenings covers both directions, and they disagree — which is the
// argument for keeping direction in Go. A *higher* hedging limit admits more; a
// *lower* rebuild floor refuses fewer rebuilds. A file declaring its own direction
// would let either be inverted in the commit that moved it.
func TestPromoteLoosenings(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		old, new string
		key      string
	}{
		"higher hedging limit": {"value = 3", "value = 9", "hedging_max"},
		"lower rebuild floor":  {"value = 0.5", "value = 0.1", "rebuild_floor_fraction"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			old := loadPromote(t, string(standards.DefaultPromote()))
			cur := loadPromote(t,
				strings.Replace(string(standards.DefaultPromote()), tc.old, tc.new, 1))

			got := standards.ComparePromote(old, cur)
			if len(got) != 1 {
				t.Fatalf("want exactly %s, got %+v", tc.key, got)
			}
			if got[0].Key != tc.key {
				t.Errorf("reported %q, want %q", got[0].Key, tc.key)
			}
			if strings.TrimSpace(got[0].Rationale) == "" {
				t.Error("the report carries no rationale")
			}
		})
	}
}

// TestPromoteTighteningIsNotReported, so the report answers one question.
func TestPromoteTighteningIsNotReported(t *testing.T) {
	t.Parallel()
	for _, tc := range [][2]string{
		{"value = 3", "value = 1"},
		{"value = 0.5", "value = 0.9"},
	} {
		old := loadPromote(t, string(standards.DefaultPromote()))
		cur := loadPromote(t,
			strings.Replace(string(standards.DefaultPromote()), tc[0], tc[1], 1))
		if got := standards.ComparePromote(old, cur); len(got) != 0 {
			t.Errorf("a tightening was reported: %+v", got)
		}
	}
}

// TestComparePromoteToleratesNil, because a bundle may have this file and not the
// other, or neither.
func TestComparePromoteToleratesNil(t *testing.T) {
	t.Parallel()
	p := loadPromote(t, string(standards.DefaultPromote()))
	if got := standards.ComparePromote(nil, p); got != nil {
		t.Errorf("a first-ever file reported loosenings: %+v", got)
	}
	if got := standards.ComparePromote(p, nil); got != nil {
		t.Errorf("comparing against nothing reported loosenings: %+v", got)
	}
}

func loadPromote(t *testing.T, src string) *standards.Promote {
	t.Helper()
	p, err := standards.LoadPromote([]byte(src))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return p
}
