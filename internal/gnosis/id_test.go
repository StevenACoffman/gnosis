package gnosis_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// validV7 is a well-formed lowercase UUIDv7. The version nibble is the first
// character of the third group.
const validV7 = "01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d"

func TestParseIDAcceptsV7(t *testing.T) {
	t.Parallel()
	got, err := gnosis.ParseID(validV7)
	if err != nil {
		t.Fatalf("ParseID(%q) = %v, want no error", validV7, err)
	}
	if got.String() != validV7 {
		t.Errorf("round trip lost data: got %q, want %q", got.String(), validV7)
	}
}

func TestParseIDRejects(t *testing.T) {
	t.Parallel()
	// Every rejection must classify as EINVALID. A caller distinguishes bad
	// input from a missing entity by the code, so a wrong code here would make
	// a malformed identifier look like ENOTFOUND at the boundary.
	tests := map[string]string{
		"empty":            "",
		"too short":        "01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3",
		"too long":         validV7 + "d",
		"version 4":        "01932b7c-1f4e-4a3d-9c2b-5e8f0a1b2c3d",
		"uppercase hex":    "01932B7C-1f4e-7a3d-9c2b-5e8f0a1b2c3d",
		"hyphen misplaced": "01932b7c1-f4e-7a3d-9c2b-5e8f0a1b2c3d",
		"non-hex letter":   "01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2z3d",
		"braced":           "{01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3}",
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := gnosis.ParseID(in)
			if err == nil {
				t.Fatalf("ParseID(%q) = %q, want an error", in, got)
			}
			if code := errs.ErrorCode(err); code != errs.EINVALID {
				t.Errorf("ParseID(%q) code = %q, want %q", in, code, errs.EINVALID)
			}
			if got != "" {
				t.Errorf("ParseID(%q) returned %q alongside an error, want empty", in, got)
			}
		})
	}
}

func TestParseIDIsIdempotentOverItsOwnOutput(t *testing.T) {
	t.Parallel()
	// The property that matters for the redundant-record comparison: an
	// identifier read from a file and one read from the index must compare
	// equal, so parsing must not normalise anything.
	first, err := gnosis.ParseID(validV7)
	if err != nil {
		t.Fatalf("ParseID: %v", err)
	}
	second, err := gnosis.ParseID(first.String())
	if err != nil {
		t.Fatalf("ParseID of own output: %v", err)
	}
	if first != second {
		t.Errorf("parse is not stable: %q then %q", first, second)
	}
}
