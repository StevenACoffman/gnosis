package standards_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/standards"
)

// TestEveryDeclaredValueIsClassified. The maintained map is a second place to
// remember, and this is what makes forgetting it loud. A threshold added to a
// struct and not classified shows up here by name.
//
// The failure message is the point: it tells a maintainer which of the three
// states to record, rather than reporting a count.
//
// **What this test cannot do is check whether the classification is right**, and that
// is worth saying here because the list below looks like it does. Comparing `Unread()`
// to a literal is comparing one copy of a list to another; the two agree by
// construction. `TestTheClassificationAgreesWithTheSource`, at the repository root, is
// what holds the map to account — it scans for `.<Field>.Value` outside this package,
// so the compiler's own symbols are the evidence. This one still earns its place: it
// names the value gnosis deliberately declares and does not read, and deleting that
// name when a reader lands is a one-line ritual a source scan cannot ask for.
func TestEveryDeclaredValueIsClassified(t *testing.T) {
	t.Parallel()
	// The one value gnosis declares and does not read. §14.4.1 wants it for
	// "unprovable AND load-bearing" and `unprovable` is Phase 3, so it has nothing
	// to narrow yet. Deleting this line when the reader lands is the whole ritual.
	want := []string{"in_degree_cut"}

	got := standards.Unread()
	if !slices.Equal(got, want) {
		t.Errorf("unread values = %v, want %v\n"+
			"an unexpected name means a threshold was declared without recording "+
			"what reads it; a missing name means the reader landed and the map was "+
			"not updated", got, want)
	}
}

func TestPinnedValuesAreNotReportedAsUnread(t *testing.T) {
	t.Parallel()
	pinned := standards.Pinned()
	if len(pinned) == 0 {
		t.Fatal("no value is pinned; the extractor pair should be")
	}
	for _, p := range pinned {
		if slices.Contains(standards.Unread(), p) {
			t.Errorf("%q is reported both pinned and unread", p)
		}
	}
}

// TestTheSeedTunesNothing. The finding must be silent on a fresh bundle — the
// failure that made this function narrower than its first draft.
func TestTheSeedTunesNothing(t *testing.T) {
	t.Parallel()
	arch, promo := seed(t)
	if got := standards.Tuned(arch, promo); len(got) != 0 {
		t.Errorf("a freshly initialised bundle reports %v", got)
	}
}

// seed loads the values a new bundle begins with.
func seed(t *testing.T) (*standards.Archive, *standards.Promote) {
	t.Helper()
	a, err := standards.LoadArchive(standards.DefaultArchive())
	if err != nil {
		t.Fatalf("the archive seed does not load: %v", err)
	}
	p, err := standards.LoadPromote(standards.DefaultPromote())
	if err != nil {
		t.Fatalf("the promote seed does not load: %v", err)
	}
	return a, p
}

// TestOnlyAnUnreadEditIsReported. A tuned value that something reads is doing its
// job and is not a finding; the same edit to a value nothing reads is.
func TestOnlyAnUnreadEditIsReported(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		edit func(*standards.Archive)
		want []string
	}{
		"an unread value moved": {
			func(a *standards.Archive) { a.InDegreeCut.Value = 9 },
			[]string{"in_degree_cut"},
		},
		"a consumed value moved": {func(a *standards.Archive) { a.StalenessDays.Value = 30 }, nil},
		"a pinned value moved": {
			func(a *standards.Archive) { a.HTMLExtractorVersion.Value = "v9" },
			nil,
		},
		"only a rationale reworded": {
			func(a *standards.Archive) { a.InDegreeCut.Rationale = "different words, same number" },
			nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			arch, promo := seed(t)
			tc.edit(arch)
			got := standards.Tuned(arch, promo)
			if !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestABundleWithNoStandardsTunesNothing. Both files are optional and a nil must
// not read as "every value differs from the seed".
func TestABundleWithNoStandardsTunesNothing(t *testing.T) {
	t.Parallel()
	if got := standards.Tuned(nil, nil); len(got) != 0 {
		t.Errorf("a bundle declaring no standards reports %v", got)
	}
}

// TestUnreadNamesAKeyAReaderCanSearchFor. The report is only useful if the name
// it prints is the name in the file.
func TestUnreadNamesAKeyAReaderCanSearchFor(t *testing.T) {
	t.Parallel()
	src := string(standards.DefaultArchive()) + string(standards.DefaultPromote())
	for _, k := range append(standards.Unread(), standards.Pinned()...) {
		if !strings.Contains(src, "["+k+"]") {
			t.Errorf("%q is not a table name in either seed", k)
		}
	}
}
