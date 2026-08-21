package gnosis_test

import (
	"reflect"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// Three well-formed identifiers, kept short at the call sites.
const (
	idA = "01932b7c-1f4e-7a3d-9c2b-000000000001"
	idB = "01932b7c-1f4e-7a3d-9c2b-000000000002"
	idC = "01932b7c-1f4e-7a3d-9c2b-000000000003"
)

func TestReconcile(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		observed []gnosis.Observed
		indexed  []gnosis.Indexed
		want     []gnosis.Resolution
	}{
		"agreement yields nothing": {
			observed: []gnosis.Observed{{Path: "c/a.md", ID: idA}},
			indexed:  []gnosis.Indexed{{Path: "c/a.md", ID: idA}},
			want:     nil,
		},
		"new document": {
			observed: []gnosis.Observed{{Path: "c/a.md", ID: idA}},
			want: []gnosis.Resolution{
				{Kind: gnosis.KindIndex, ID: idA, Paths: []string{"c/a.md"}},
			},
		},
		"moved document": {
			observed: []gnosis.Observed{{Path: "c/new.md", ID: idA}},
			indexed:  []gnosis.Indexed{{Path: "c/old.md", ID: idA}},
			want: []gnosis.Resolution{{
				Kind: gnosis.KindUpdatePath, ID: idA,
				Paths: []string{"c/old.md", "c/new.md"},
			}},
		},
		"deleted document": {
			indexed: []gnosis.Indexed{{Path: "c/a.md", ID: idA}},
			want: []gnosis.Resolution{
				{Kind: gnosis.KindTombstone, ID: idA, Paths: []string{"c/a.md"}},
			},
		},
		"document with no identifier": {
			observed: []gnosis.Observed{{Path: "c/stray.md"}},
			want: []gnosis.Resolution{
				{Kind: gnosis.KindQuarantine, Paths: []string{"c/stray.md"}},
			},
		},
		"same path, different identifier": {
			observed: []gnosis.Observed{{Path: "c/a.md", ID: idB}},
			indexed:  []gnosis.Indexed{{Path: "c/a.md", ID: idA}},
			want: []gnosis.Resolution{{
				Kind: gnosis.KindConflict, ID: idB,
				Paths: []string{"c/a.md"}, Other: idA,
			}},
		},
		"unrelated changes do not interfere": {
			observed: []gnosis.Observed{
				{Path: "c/a.md", ID: idA},
				{Path: "c/moved.md", ID: idB},
			},
			indexed: []gnosis.Indexed{
				{Path: "c/b.md", ID: idB},
				{Path: "c/gone.md", ID: idC},
			},
			want: []gnosis.Resolution{
				{Kind: gnosis.KindIndex, ID: idA, Paths: []string{"c/a.md"}},
				{Kind: gnosis.KindTombstone, ID: idC, Paths: []string{"c/gone.md"}},
				{Kind: gnosis.KindUpdatePath, ID: idB, Paths: []string{"c/b.md", "c/moved.md"}},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := gnosis.Reconcile(tc.observed, tc.indexed)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Reconcile()\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// TestDuplicateChoosesNoWinner is the property most likely to be "helpfully"
// broken by a later change. Two documents carrying one identifier is a copy or
// a bad merge, and picking one would silently discard whichever copy holds a
// colleague's work. Every path must survive into the resolution.
func TestDuplicateChoosesNoWinner(t *testing.T) {
	t.Parallel()
	got := gnosis.Reconcile([]gnosis.Observed{
		{Path: "c/second.md", ID: idA},
		{Path: "c/first.md", ID: idA},
		{Path: "c/third.md", ID: idA},
	}, nil)

	if len(got) != 1 {
		t.Fatalf("got %d resolutions, want exactly 1: %+v", len(got), got)
	}
	r := got[0]
	if r.Kind != gnosis.KindDuplicate {
		t.Errorf("Kind = %q, want %q", r.Kind, gnosis.KindDuplicate)
	}
	want := []string{"c/first.md", "c/second.md", "c/third.md"}
	if !reflect.DeepEqual(r.Paths, want) {
		t.Errorf("Paths = %v, want all three sorted: %v", r.Paths, want)
	}
}

// TestDuplicateSuppressesOtherVerdicts checks that a duplicated identifier is
// reported once, as a duplicate, rather than also as a move or an addition. A
// second verdict would invite a reader to act on one and ignore the collision.
func TestDuplicateSuppressesOtherVerdicts(t *testing.T) {
	t.Parallel()
	got := gnosis.Reconcile(
		[]gnosis.Observed{{Path: "c/a.md", ID: idA}, {Path: "c/copy.md", ID: idA}},
		[]gnosis.Indexed{{Path: "c/a.md", ID: idA}},
	)
	if len(got) != 1 || got[0].Kind != gnosis.KindDuplicate {
		t.Errorf("got %+v, want exactly one duplicate resolution", got)
	}
}

// TestConflictIsNotAlsoATombstone checks the displaced identifier is reported
// once. Without suppression a reader would see both "these disagree" and
// "the old one vanished", which are the same event described twice.
func TestConflictIsNotAlsoATombstone(t *testing.T) {
	t.Parallel()
	got := gnosis.Reconcile(
		[]gnosis.Observed{{Path: "c/a.md", ID: idB}},
		[]gnosis.Indexed{{Path: "c/a.md", ID: idA}},
	)
	if len(got) != 1 {
		t.Fatalf("got %d resolutions, want 1: %+v", len(got), got)
	}
	if got[0].Kind != gnosis.KindConflict || got[0].Other != idA {
		t.Errorf("got %+v, want a conflict naming %q as displaced", got[0], idA)
	}
}

// TestReconcileIsDeterministic pins SPEC §18.3: two runs over the same corpus
// must be comparable, and map iteration is randomised.
func TestReconcileIsDeterministic(t *testing.T) {
	t.Parallel()
	observed := []gnosis.Observed{
		{Path: "c/z.md", ID: idC}, {Path: "c/a.md", ID: idA}, {Path: "c/n.md"},
	}
	indexed := []gnosis.Indexed{{Path: "c/old.md", ID: idB}}

	first := gnosis.Reconcile(observed, indexed)
	for range 20 {
		if got := gnosis.Reconcile(observed, indexed); !reflect.DeepEqual(got, first) {
			t.Fatalf("output varies between runs:\n got %+v\nfirst %+v", got, first)
		}
	}
}

// TestReconcileDoesNotMutateItsInput protects the pure-core contract: a caller
// that reuses its slices must see them unchanged.
func TestReconcileDoesNotMutateItsInput(t *testing.T) {
	t.Parallel()
	observed := []gnosis.Observed{{Path: "c/b.md", ID: idA}, {Path: "c/a.md", ID: idA}}
	indexed := []gnosis.Indexed{{Path: "c/x.md", ID: idB}}
	obsCopy := append([]gnosis.Observed(nil), observed...)
	idxCopy := append([]gnosis.Indexed(nil), indexed...)

	gnosis.Reconcile(observed, indexed)

	if !reflect.DeepEqual(observed, obsCopy) {
		t.Errorf("observed was mutated: %+v, want %+v", observed, obsCopy)
	}
	if !reflect.DeepEqual(indexed, idxCopy) {
		t.Errorf("indexed was mutated: %+v, want %+v", indexed, idxCopy)
	}
}
