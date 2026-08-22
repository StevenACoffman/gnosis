package bundle_test

import (
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

func TestTrailWhole(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		trail   bundle.Trail
		wantErr bool
	}{
		"the zero trail":      {bundle.Trail{}, false},
		"rows and no damage":  {bundle.Trail{Rows: make([]audit.Row, 3)}, false},
		"one unreadable line": {bundle.Trail{Malformed: []int{2}}, true},
		"damage among rows": {
			bundle.Trail{Rows: make([]audit.Row, 9), Malformed: []int{4, 7}},
			true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := tc.trail.Whole()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Whole() = %v, want error: %v", err, tc.wantErr)
			}
			if err == nil {
				return
			}
			// The message has to locate the damage. "the trail will not parse" is
			// not a useful description of one bad row in ten thousand.
			for _, n := range []string{"2", "4", "7"} {
				if strings.Contains(err.Error(), n) {
					return
				}
			}
			t.Errorf("the error names no line: %v", err)
		})
	}
}

// TestAnEmptyTrailHasNoNewestRow. The zero time means "nothing written", and a
// caller comparing it against a commit timestamp must read that as unknown rather
// than as very old — otherwise every fresh corpus reports a stale trail.
func TestAnEmptyTrailHasNoNewestRow(t *testing.T) {
	t.Parallel()
	empty := bundle.Trail{}
	if !empty.Newest().IsZero() {
		t.Errorf("an empty trail's Newest is %v", empty.Newest())
	}
}

// TestNewestIsTheLastRow. The file is append-only and every writer holds the lock,
// so position is order and the last row is the newest.
func TestNewestIsTheLastRow(t *testing.T) {
	t.Parallel()
	first := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	trail := bundle.Trail{Rows: []audit.Row{{At: first}, {At: last}}}

	if got := trail.Newest(); !got.Equal(last) {
		t.Errorf("Newest = %v, want %v", got, last)
	}
}
