package standards_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/standards"
)

// TestSeedLoads is the one test this package cannot do without: a seed its own
// loader rejects would break every bundle created from it.
func TestSeedLoads(t *testing.T) {
	t.Parallel()
	a, err := standards.LoadArchive(standards.DefaultArchive())
	if err != nil {
		t.Fatalf("the embedded seed does not load: %v", err)
	}
	if got := a.PerFileCap.Value; got != 262144 {
		t.Errorf("per_file_cap = %d, want 262144", got)
	}
	if got := a.Allowlist.Value; len(got) != 3 {
		t.Errorf("allowlist = %v, want three entries", got)
	}
}

// TestSeedIsACopy: a caller that edits the returned bytes must not corrupt the
// seed for the next one.
func TestSeedIsACopy(t *testing.T) {
	t.Parallel()
	first := standards.DefaultArchive()
	first[0] = 'X'
	if second := standards.DefaultArchive(); second[0] == 'X' {
		t.Error("DefaultArchive returns the package's own slice")
	}
}

// TestRationaleIsRequired is the structural claim of §6.2: a value cannot be
// expressed without saying why. A rationale that were merely conventional would
// be the first thing dropped by whoever was in a hurry.
func TestRationaleIsRequired(t *testing.T) {
	t.Parallel()
	// The body is blanked, not the key removed: blank must fail the same way
	// absent does, or the check is bypassed by pressing the space bar.
	src := blankRationale(string(standards.DefaultArchive()), "per_file_cap")
	if src == string(standards.DefaultArchive()) {
		t.Fatal("the fixture no longer matches the seed's rationale syntax")
	}

	_, err := standards.LoadArchive([]byte(src))
	if err == nil {
		t.Fatal("a value with a blank rationale loaded")
	}
	if !strings.Contains(err.Error(), "per_file_cap") {
		t.Errorf("the error does not name the offending key: %v", err)
	}
}

// TestUnrecognisedKeyIsRejected: a mistyped key would leave a threshold the
// author believes they changed.
func TestUnrecognisedKeyIsRejected(t *testing.T) {
	t.Parallel()
	src := string(
		standards.DefaultArchive(),
	) + "\n[per_file_capp]\nvalue = 1\nrationale = \"typo\"\n"

	_, err := standards.LoadArchive([]byte(src))
	if err == nil {
		t.Fatal("a mistyped key loaded silently")
	}
	if !strings.Contains(err.Error(), "per_file_capp") {
		t.Errorf("the error does not name the unrecognised key: %v", err)
	}
}

// TestValidationRejectsTheUnintendable covers gates outside the range in which
// they mean anything — a cap of zero archives nothing and reports no error.
func TestValidationRejectsTheUnintendable(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		old, new string
		want     string
	}{
		"zero cap": {
			"value = 262144", "value = 0", "per_file_cap",
		},
		"cap above budget": {
			"value = 268435456", "value = 1024", "corpus_budget",
		},
		"fraction above one": {
			"value = 0.8", "value = 1.5", "corpus_warn_fraction",
		},
		"extension without a dot": {
			`value = [".md", ".txt", ".svg"]`, `value = ["md"]`, "leading dot",
		},
		"upper-case extension": {
			`value = [".md", ".txt", ".svg"]`, `value = [".MD"]`, "lower-case",
		},
		"empty allowlist": {
			`value = [".md", ".txt", ".svg"]`, `value = []`, "nothing could ever be archived",
		},
		"unpinned extractor": {
			`value = "v2.5.2"`, `value = ""`, "html_extractor_version",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := strings.Replace(string(standards.DefaultArchive()), tc.old, tc.new, 1)
			if src == string(standards.DefaultArchive()) {
				t.Fatalf("the fixture no longer matches the seed: %q not found", tc.old)
			}
			_, err := standards.LoadArchive([]byte(src))
			if err == nil {
				t.Fatalf("%s loaded", name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// TestValidationReportsEverythingAtOnce: a loader that reports one problem per
// run turns a five-value edit into five edit-and-rerun cycles.
func TestValidationReportsEverythingAtOnce(t *testing.T) {
	t.Parallel()
	src := strings.NewReplacer(
		"value = 262144", "value = -1",
		"value = 0.8", "value = 9",
		`value = [".md", ".txt", ".svg"]`, `value = ["md"]`,
	).Replace(string(standards.DefaultArchive()))

	_, err := standards.LoadArchive([]byte(src))
	if err == nil {
		t.Fatal("three bad gates loaded")
	}
	for _, want := range []string{"per_file_cap", "corpus_warn_fraction", "leading dot"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q omits %q", err, want)
		}
	}
}

// blankRationale replaces the named gate's rationale with whitespace, leaving the
// key present. Blank must fail the same way absent does.
func blankRationale(src, gate string) string {
	start := strings.Index(src, "["+gate+"]")
	if start < 0 {
		return src
	}
	rel := strings.Index(src[start:], `rationale = """`)
	if rel < 0 {
		return src
	}
	open := start + rel + len(`rationale = """`)
	rel = strings.Index(src[open:], `"""`)
	if rel < 0 {
		return src
	}
	return src[:open] + "\n   \n" + src[open+rel:]
}
