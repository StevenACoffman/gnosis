package index_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/index"
)

// TestFloorBreached covers the arithmetic and, more importantly, the two cases
// where the answer must be "no" for a reason that is not arithmetic.
func TestFloorBreached(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		previous int
		current  int
		fraction float64
		want     bool
	}{
		// The case this exists for: a wrong --bundle or an unstaged c/.
		"corpus vanished":    {500, 0, 0.5, true},
		"corpus nearly gone": {500, 3, 0.5, true},
		"just under half":    {500, 249, 0.5, true},

		// Ordinary work must not trip it.
		"exactly at the floor": {500, 250, 0.5, false},
		"just over":            {500, 251, 0.5, false},
		"unchanged":            {500, 500, 0.5, false},
		"grew":                 {500, 900, 0.5, false},
		"one document deleted": {500, 499, 0.5, false},

		// A fresh bundle has no prior count. A floor that fired here would break
		// `init` followed by `rebuild` on an empty corpus, which is the ordinary
		// path, in order to protect the rare one.
		"first ever rebuild": {0, 0, 0.5, false},
		"first with content": {0, 40, 0.5, false},
		"negative previous":  {-1, 0, 0.5, false},

		// A misconfigured floor must not wedge the corpus. The loader rejects a
		// bad value; if one reaches here the conservative direction is to proceed.
		"zero fraction": {500, 0, 0, false},
		"above one":     {500, 400, 1.5, false},
		"negative":      {500, 0, -0.5, false},

		// A floor of exactly 1 means any loss at all refuses, which is a coherent
		// thing to ask for and must work.
		"floor of one, lost one":  {500, 499, 1, true},
		"floor of one, unchanged": {500, 500, 1, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := index.FloorBreached(tc.previous, tc.current, tc.fraction)
			if got != tc.want {
				t.Errorf("FloorBreached(%d, %d, %v) = %v, want %v",
					tc.previous, tc.current, tc.fraction, got, tc.want)
			}
		})
	}
}
