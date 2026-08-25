package bundle_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// TestASourceThatHeldStillIsNotARow keeps the register measuring churn rather than
// listing the archive.
//
// A source fetched once has not moved, and a register with a 1 beside every source
// would bury the handful that moved among the hundreds that did not — §12's argument
// about a warning true of everything, applied to a count.
func TestASourceThatHeldStillIsNotARow(t *testing.T) {
	t.Parallel()

	got := bundle.Churned(map[string][]string{
		"https://example.org/still.md": {"aaa"},
	}, nil)
	if len(got.Sources) != 0 {
		t.Errorf("an unmoved source is a row: %+v", got.Sources)
	}
	// It is still counted, so the report says what it looked at.
	if got.Recorded != 1 {
		t.Errorf("recorded = %d, want 1", got.Recorded)
	}
}

// TestTheOutcomesAreCountedSeparately is the split that makes the report worth
// reading: six moves that kept their passages and one that lost a passage are
// different events, and §14.3.2 exists because collapsing them puts the cheapest
// maintenance task in the same bucket as the most serious evidentiary one.
func TestTheOutcomesAreCountedSeparately(t *testing.T) {
	t.Parallel()

	const uri = "https://example.org/churns.md"
	observed := []bundle.Check{
		{URI: uri, SourceSHA256: "v1", Drift: gnosis.DriftBenign.String()},
		{URI: uri, SourceSHA256: "v2", Drift: gnosis.DriftBenign.String()},
		{URI: uri, SourceSHA256: "v3", Drift: gnosis.DriftUnsupported.String()},
	}

	got := bundle.Churned(map[string][]string{
		// v4 has no observation at all: nobody has compared it.
		uri: {"v1", "v2", "v3", "v4"},
	}, observed)
	if len(got.Sources) != 1 {
		t.Fatalf("want one row, got %+v", got.Sources)
	}
	row := got.Sources[0]
	switch {
	case row.Versions != 4:
		t.Errorf("versions = %d, want 4", row.Versions)
	case row.Benign != 2:
		t.Errorf("benign = %d, want 2", row.Benign)
	case row.Unsupported != 1:
		t.Errorf("unsupported = %d, want 1", row.Unsupported)
	case row.Unchecked != 1:
		t.Errorf("unchecked = %d, want 1", row.Unchecked)
	}
}

// TestAnUncomparedVersionIsNotACheapOne is the honest-silence case.
//
// A source with six versions and six unchecked ones has told the reader nothing, and a
// report that omitted the unchecked count would read as six cheap changes — the
// `drift-unchecked`-as-`drift-benign` collapse §14.3.2 refuses, arriving through a
// summary rather than through a verdict.
func TestAnUncomparedVersionIsNotACheapOne(t *testing.T) {
	t.Parallel()

	got := bundle.Churned(map[string][]string{
		"https://example.org/unknown.md": {"v1", "v2", "v3"},
	}, nil)
	if len(got.Sources) != 1 {
		t.Fatalf("want one row, got %+v", got.Sources)
	}
	row := got.Sources[0]
	if row.Unchecked != 3 || row.Benign != 0 {
		t.Errorf("an uncompared source reads as %+v", row)
	}
}

// TestWithdrawnSupportSortsFirst is the ordering, and it is by what a row costs rather
// than by how much it moved.
//
// A source that lost a passage belongs at the top whatever its version count, because
// that is the one row where the answer is not "re-archive".
func TestWithdrawnSupportSortsFirst(t *testing.T) {
	t.Parallel()

	const (
		noisy = "https://example.org/noisy.md"
		lost  = "https://example.org/lost.md"
	)
	got := bundle.Churned(map[string][]string{
		// Nine versions and nothing lost.
		noisy: {"a", "b", "c", "d", "e", "f", "g", "h", "i"},
		// Two versions and one passage gone.
		lost: {"v1", "v2"},
	}, []bundle.Check{
		{URI: lost, SourceSHA256: "v2", Drift: gnosis.DriftUnsupported.String()},
	})

	if len(got.Sources) != 2 {
		t.Fatalf("want two rows, got %+v", got.Sources)
	}
	if got.Sources[0].URI != lost {
		t.Errorf("the source that lost a passage sorted below the noisy one: %+v",
			got.Sources)
	}
}

// TestAnEmptyRegisterIsEmptyNotNil keeps the envelope unambiguous: `[]` says the
// report ran and found nothing, `null` says nothing at all.
func TestAnEmptyRegisterIsEmptyNotNil(t *testing.T) {
	t.Parallel()

	if got := bundle.Churned(nil, nil); got.Sources == nil {
		t.Error("an empty register is nil rather than empty")
	}
}

// TestACurrentVersionIsNotAnUncheckedOne is the collapse this nearly acquired.
//
// `drift-none` means somebody compared and the bytes had not moved. Counting that as
// unchecked would say nobody looked — the checked/unchecked collapse this codebase
// refuses everywhere else, arriving through a `default` branch that swallowed a case
// while a comment claimed it meant the same thing.
func TestACurrentVersionIsNotAnUncheckedOne(t *testing.T) {
	t.Parallel()

	const uri = "https://example.org/settled.md"
	got := bundle.Churned(map[string][]string{uri: {"v1", "v2"}}, []bundle.Check{
		{URI: uri, SourceSHA256: "v1", Drift: gnosis.DriftBenign.String()},
		{URI: uri, SourceSHA256: "v2", Drift: gnosis.DriftNone.String()},
	})
	if len(got.Sources) != 1 {
		t.Fatalf("want one row, got %+v", got.Sources)
	}
	row := got.Sources[0]
	if row.Current != 1 || row.Unchecked != 0 {
		t.Errorf("a compared, unmoved version reads as %+v", row)
	}
	// And every version is accounted for, which is what makes the row readable.
	if row.Benign+row.Unsupported+row.Current+row.Unchecked != row.Versions {
		t.Errorf("the counts do not sum to the versions: %+v", row)
	}
}
