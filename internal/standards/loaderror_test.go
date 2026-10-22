package standards_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/standards"
	"github.com/StevenACoffman/skillet/errs"
)

// TestALoaderNamesItsOperationOnce pins the shape of a loader's refusal: one EINVALID
// whose message begins with the loader's operation and does not repeat it.
//
// Every one of these loaders once rendered `standards.LoadX: standards.LoadX: …`,
// because the leaf put the operation in both `Op` and `Message`, and the shape is easy
// to reintroduce by copying a neighbouring site — so the property is asserted.
func TestALoaderNamesItsOperationOnce(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		seed []byte
		load func([]byte) error
	}{
		"standards.LoadRegisters": {
			standards.DefaultRegisters(),
			func(b []byte) error { _, err := standards.LoadRegisters(b); return err },
		},
		"standards.LoadStrengths": {
			standards.DefaultStrengths(),
			func(b []byte) error { _, err := standards.LoadStrengths(b); return err },
		},
		"standards.LoadIndicators": {
			standards.DefaultIndicators(),
			func(b []byte) error { _, err := standards.LoadIndicators(b); return err },
		},
		"standards.LoadLanguage": {
			standards.DefaultLanguage(),
			func(b []byte) error { _, err := standards.LoadLanguage(b); return err },
		},
		"standards.LoadOperators": {
			standards.DefaultOperators(),
			func(b []byte) error { _, err := standards.LoadOperators(b); return err },
		},
	}
	for op, tc := range cases {
		t.Run(op, func(t *testing.T) {
			t.Parallel()

			// A top-level key no loader declares; it must precede any table header
			// to stay top-level, so it goes first.
			err := tc.load(append([]byte("not_a_key = 1\n"), tc.seed...))
			if err == nil {
				t.Fatal("an unknown key was accepted")
			}
			if got := errs.ErrorCode(err); got != errs.EINVALID {
				t.Errorf("code = %q, want %q", got, errs.EINVALID)
			}
			msg := err.Error()
			if !strings.HasPrefix(msg, op+": ") {
				t.Errorf("message %q does not begin with %q", msg, op+": ")
			}
			if n := strings.Count(msg, op); n != 1 {
				t.Errorf("message %q names %s %d times, want once", msg, op, n)
			}
		})
	}
}
