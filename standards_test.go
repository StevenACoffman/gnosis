package main

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/standards"
)

// standardsPackage is where the classification lives. References from inside it are
// not evidence that anything *reads* a value: the loader, the validator, and the
// loosening comparison all touch every field by construction.
const standardsPackage = "internal/standards"

// TestTheClassificationAgreesWithTheSource is what the backlog entry asked for and
// did not have.
//
// `standards.Unread` records what reads each declared threshold, and the entry's own
// honest note was that "the test is what makes it survivable". That test compared
// `Unread()` to a literal list — a second copy of the same list, so the two agreed by
// construction and neither was evidence. Forgetting to classify a new threshold was
// caught; misclassifying one was not.
//
// The available evidence is the source. Every real read of a standards value goes
// through `.<Field>.Value`, because the fields are `Value[T]` and the number is behind
// that selector — so a scan for that selector outside the package is a fact about the
// binary rather than a restatement of the map.
//
// # Both directions fail, and they fail for different mistakes
//
//   - **Classified consumed or pinned, referenced nowhere.** A dead knob claimed live.
//     This is the direction that matters: `describeLoosening` reported `staleness_days`
//     as read by nothing for two phases, so widening the window silenced findings while
//     `standards check --log` recorded that it cost nothing.
//   - **Classified unread, referenced somewhere.** A value acquired a reader and
//     nobody recorded it. Safe — it shows up as a false alarm in `doctor` — but it is
//     the false alarm somebody has to chase, and the entry filed it for that reason.
//
// # Why the selector and not the field name
//
// `.Allowlist` and `.PerFileCap` are also fields of `archive.Gates`, which the
// standards *flow into*; matching the bare name would count the destination as a
// reader of the source. Requiring `.Value` distinguishes them, because a `Gates` field
// is a plain scalar and a standards field is not. That is why this test can assert the
// dangerous direction rather than merely reporting it.
func TestTheClassificationAgreesWithTheSource(t *testing.T) {
	t.Parallel()

	source := goSourceOutside(t, standardsPackage)
	for _, d := range standards.Declarations() {
		read := strings.Contains(source, "."+d.Field+".Value")
		switch d.Reads {
		case standards.ReadingConsumed, standards.ReadingPinned:
			if !read {
				t.Errorf("%s is classified %s and nothing outside %s reads "+
					"`.%s.Value`; a dead knob claimed live is the failure this "+
					"file exists to prevent",
					d.Key, d.Reads, standardsPackage, d.Field)
			}
		case standards.ReadingUnread:
			if read {
				t.Errorf("%s is classified unread and something outside %s reads "+
					"`.%s.Value`; record what reads it, or `doctor` will keep "+
					"reporting a live threshold as dead",
					d.Key, standardsPackage, d.Field)
			}
		default:
			t.Errorf("%s has an unrecognised classification %v", d.Key, d.Reads)
		}
	}
}

// TestEveryDeclaredValueHasAKeyAndAField guards the scan's own premise. A declaration
// with an empty field name would make every assertion above vacuously pass.
func TestEveryDeclaredValueHasAKeyAndAField(t *testing.T) {
	t.Parallel()

	declared := standards.Declarations()
	if len(declared) == 0 {
		t.Fatal("no standards values are declared; the scan would assert nothing")
	}
	for _, d := range declared {
		if d.Field == "" || d.Key == "" {
			t.Errorf("declaration %+v is missing a name", d)
		}
	}
}

// goSourceOutside concatenates every non-test Go file in the module except those under
// the given directory.
//
// Test files are excluded on purpose: a fixture that sets a threshold to exercise the
// loosening comparison is not a consumer, and counting it would let a value look live
// because its own test touches it.
//
// The walk starts at the working directory, which `go test` sets to this package's own
// — the repository root. That is the whole mechanism, as it is for `spec_test.go`: no
// runtime.Caller and no os.Getwd.
//
// It walks an `fs.FS` rather than the filesystem directly, and that is the linter's
// point rather than a style preference: a path handed back by a walk and then reopened
// can be a different file by the time it is read, and an `fs.FS` rooted at the
// repository cannot escape it. The reading is done through the same value that
// produced the name.
func goSourceOutside(t *testing.T, skip string) string {
	t.Helper()

	fsys := os.DirFS(".")
	var b strings.Builder
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			// Nothing in a vendored or hidden tree is this repository's own code.
			if name := d.Name(); name != "." && strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			if path == skip {
				return fs.SkipDir
			}
			return nil
		case !strings.HasSuffix(path, ".go"), strings.HasSuffix(path, "_test.go"):
			return nil
		}
		raw, rErr := fs.ReadFile(fsys, path)
		if rErr != nil {
			return rErr
		}
		b.Write(raw)
		return nil
	})
	if err != nil {
		t.Fatalf("walk the module's source: %v", err)
	}
	if b.Len() == 0 {
		t.Fatal("no source was read; the scan would assert nothing")
	}
	return b.String()
}
