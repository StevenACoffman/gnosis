package standards_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/standards"
)

// TestNothingChangedLoosensNothing: the common case is no edit at all, and a
// report that fired on it would be ignored within a week.
func TestNothingChangedLoosensNothing(t *testing.T) {
	t.Parallel()
	a, b := load(t, string(standards.DefaultArchive())), load(t, string(standards.DefaultArchive()))
	if got := standards.CompareArchive(a, b); len(got) != 0 {
		t.Errorf("an unchanged file reported %d loosenings: %+v", len(got), got)
	}
}

// TestLoosenings covers each gate's direction, because getting one backwards
// would silently exempt it — the failure this mechanism exists to prevent.
func TestLoosenings(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		old, new string
		key      string
	}{
		"bigger file cap":    {"value = 262144", "value = 999999", "per_file_cap"},
		"bigger budget":      {"value = 268435456", "value = 999999999", "corpus_budget"},
		"bigger payload cap": {"value = 8192", "value = 65536", "embedded_payload_cap"},
		"longer staleness":   {"value = 180", "value = 365", "staleness_days"},
		"higher in-degree":   {"value = 5", "value = 50", "in_degree_cut"},
		"later warning":      {"value = 0.8", "value = 0.95", "corpus_warn_fraction"},
		"wider allowlist": {
			`[".md", ".txt", ".svg"]`,
			`[".md", ".txt", ".svg", ".rst"]`,
			"allowlist",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			old := load(t, string(standards.DefaultArchive()))
			cur := load(t, strings.Replace(string(standards.DefaultArchive()), tc.old, tc.new, 1))

			got := standards.CompareArchive(old, cur)
			if len(got) != 1 {
				t.Fatalf("want exactly %s, got %+v", tc.key, got)
			}
			if got[0].Key != tc.key {
				t.Errorf("reported %q, want %q", got[0].Key, tc.key)
			}
			if got[0].From == got[0].To {
				t.Errorf("from and to are identical: %+v", got[0])
			}
			if strings.TrimSpace(got[0].Rationale) == "" {
				t.Error("the report carries no rationale, so it reads without the reason")
			}
		})
	}
}

// TestTighteningIsNotReported: this answers one question, and a report that also
// listed tightenings would bury the answer.
func TestTighteningIsNotReported(t *testing.T) {
	t.Parallel()
	cases := map[string][2]string{
		"smaller cap":     {"value = 262144", "value = 1024"},
		"shorter window":  {"value = 180", "value = 30"},
		"narrower list":   {`[".md", ".txt", ".svg"]`, `[".md"]`},
		"earlier warning": {"value = 0.8", "value = 0.5"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			old := load(t, string(standards.DefaultArchive()))
			cur := load(t, strings.Replace(string(standards.DefaultArchive()), tc[0], tc[1], 1))
			if got := standards.CompareArchive(old, cur); len(got) != 0 {
				t.Errorf("a tightening was reported as a loosening: %+v", got)
			}
		})
	}
}

// TestRationaleAloneIsNotALoosening: rewording a justification is not a policy
// change, and reporting it would train a reader to skim the report.
func TestRationaleAloneIsNotALoosening(t *testing.T) {
	t.Parallel()
	old := load(t, string(standards.DefaultArchive()))
	cur := load(t, strings.Replace(
		string(standards.DefaultArchive()),
		"256 KiB. Well above any prose document",
		"Revised wording. Well above any prose document",
		1,
	))

	if got := standards.CompareArchive(old, cur); len(got) != 0 {
		t.Errorf("a reworded rationale reported as a loosening: %+v", got)
	}
}

// TestSwappingTwoExtensionsIsReported: a swap adds one and removes one, and the
// addition is a loosening whatever accompanied it. Netting them out would let any
// extension in behind a token removal.
func TestSwappingTwoExtensionsIsReported(t *testing.T) {
	t.Parallel()
	old := load(t, string(standards.DefaultArchive()))
	cur := load(t, strings.Replace(string(standards.DefaultArchive()),
		`[".md", ".txt", ".svg"]`, `[".md", ".txt", ".html"]`, 1))

	got := standards.CompareArchive(old, cur)
	if len(got) != 1 || got[0].Key != "allowlist" {
		t.Fatalf("a swap that admits .html was not reported: %+v", got)
	}
}

// TestCompareIsSorted, so two runs over one pair of files produce comparable
// output and a diff of the reports means something.
func TestCompareIsSorted(t *testing.T) {
	t.Parallel()
	old := load(t, string(standards.DefaultArchive()))
	cur := load(t, strings.NewReplacer(
		"value = 180", "value = 365",
		"value = 262144", "value = 999999",
		"value = 8192", "value = 65536",
	).Replace(string(standards.DefaultArchive())))

	got := standards.CompareArchive(old, cur)
	if len(got) != 3 {
		t.Fatalf("want three loosenings, got %+v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Key > got[i].Key {
			t.Errorf("out of order at %d: %q then %q", i, got[i-1].Key, got[i].Key)
		}
	}
}

// TestCompareToleratesNil, because a corpus with no previous standards file is
// the state every new bundle is in.
func TestCompareToleratesNil(t *testing.T) {
	t.Parallel()
	a := load(t, string(standards.DefaultArchive()))
	if got := standards.CompareArchive(nil, a); got != nil {
		t.Errorf("a first-ever standards file reported loosenings: %+v", got)
	}
	if got := standards.CompareArchive(a, nil); got != nil {
		t.Errorf("comparing against nothing reported loosenings: %+v", got)
	}
}

func load(t *testing.T, src string) *standards.Archive {
	t.Helper()
	a, err := standards.LoadArchive([]byte(src))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return a
}
