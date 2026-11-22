package splitcmd

import "testing"

func TestDecide(t *testing.T) {
	part := []byte("---\ngnosis_split:\n  source: book.md\n  splitter_version: 1\n---\n\n# One\n")
	other := []byte("---\ngnosis_split:\n  source: other.md\n  splitter_version: 1\n---\n\n# One\n")
	planned := []byte(
		"---\ngnosis_split:\n  source: book.md\n  splitter_version: 2\n---\n\n# One\n",
	)
	cases := map[string]struct {
		existing []byte
		exists   bool
		replace  bool
		want     string
	}{
		"nothing there":            {exists: false, want: stateCreated},
		"same bytes":               {existing: planned, exists: true, want: stateUnchanged},
		"older part, no --replace": {existing: part, exists: true, want: stateConflict},
		"older part, --replace": {
			existing: part,
			exists:   true,
			replace:  true,
			want:     stateReplaced,
		},
		"hand-written file, --replace": {
			existing: []byte("# Mine\n"),
			exists:   true,
			replace:  true,
			want:     stateConflict,
		},
		"another source's part, --replace": {
			existing: other, exists: true, replace: true, want: stateConflict,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := decide(
				planned,
				tc.existing,
				tc.exists,
				"book.md",
				tc.replace,
			); got != tc.want {
				t.Errorf("decide = %q, want %q", got, tc.want)
			}
		})
	}
}
