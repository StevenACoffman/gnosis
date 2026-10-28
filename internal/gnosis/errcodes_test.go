package gnosis_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/toerr/errors/errcode"
)

// TestEveryCodeRoundTrips is the assertion the whole migration rests on.
//
// gnosis classifies errors with skillet's string vocabulary — it is what the §8.0
// envelope's `reason` tokens are built from and what 53 call sites branch on — and the
// migration re-expresses those classifications as `errcode` statuses. If any one of them
// read back as something else, every error of that class would be misreported at once,
// and the failure would be silent: a refusal would still be a refusal, just filed under
// the wrong reason.
func TestEveryCodeRoundTrips(t *testing.T) {
	t.Parallel()

	for status, want := range gnosis.Codes() {
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			err := errcode.New(status, "something went wrong")
			if got := errs.ErrorCode(err); got != want {
				t.Errorf("errs.ErrorCode(New(%v)) = %q, want %q", status, got, want)
			}
		})
	}
}

// TestTheCodesAreDistinct. Two constants mapping to one classification would make a
// migration that "worked" while collapsing two kinds of failure into one — and callers
// branch on the difference between, say, a missing document and an invalid one.
func TestTheCodesAreDistinct(t *testing.T) {
	t.Parallel()

	seen := map[string]errcode.StatusCode{}
	for status, code := range gnosis.Codes() {
		if prior, dup := seen[code]; dup {
			t.Errorf("%v and %v both classify as %q", prior, status, code)
		}
		seen[code] = status
	}
	if len(seen) != 5 {
		t.Errorf("got %d distinct classifications, want gnosis's five", len(seen))
	}
}

// TestAWrappedCodedErrorStillClassifies. The composed shape the migration uses for a
// coded wrap puts the code outside and the wrap inside, so a classifier reading through
// the chain has to find it.
func TestAWrappedCodedErrorStillClassifies(t *testing.T) {
	t.Parallel()

	inner := errcode.New(gnosis.CodeNotFound, "no such document")
	wrapped := errcode.Link(gnosis.CodeNotFound, "", inner)
	if got := errs.ErrorCode(wrapped); got != errs.ENOTFOUND {
		t.Errorf("a wrapped coded error classifies as %q, want not_found", got)
	}
}
