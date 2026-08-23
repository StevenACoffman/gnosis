package gnosis_test

import (
	"encoding/json"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// TestTheZeroOutcomeIsInvalid: an envelope nobody populated must not read as
// success. This is the reason Code is not given an unset value — zero is a real
// code and means OK — and the reason an Outcome is built through constructors.
func TestTheZeroOutcomeIsInvalid(t *testing.T) {
	t.Parallel()
	var o gnosis.Outcome
	if o.Valid() {
		t.Error("the zero Outcome is valid")
	}
	if o.Status != gnosis.StatusUnset {
		t.Errorf("the zero status is %q, not unset", o.Status)
	}
}

// TestConstructorsPairStatusAndCode covers every pairing SPEC §8.0 defines. A
// mismatched pair — StatusError with code 0, say — would have a CI job branch on
// a status that its exit code contradicts.
func TestConstructorsPairStatusAndCode(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		got    gnosis.Outcome
		status gnosis.Status
		code   gnosis.Code
	}{
		"ok":       {gnosis.OK(nil), gnosis.StatusOK, gnosis.CodeOK},
		"findings": {gnosis.Findings("r", "m", nil), gnosis.StatusFindings, gnosis.CodeFindings},
		"blocked":  {gnosis.Blocked("r", "m", nil), gnosis.StatusBlocked, gnosis.CodeBlocked},
		"error":    {gnosis.Failure("r", "m"), gnosis.StatusError, gnosis.CodeError},
		"usage":    {gnosis.BadUsage("m"), gnosis.StatusError, gnosis.CodeUsage},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if tc.got.Status != tc.status {
				t.Errorf("status = %q, want %q", tc.got.Status, tc.status)
			}
			if tc.got.Code != tc.code {
				t.Errorf("code = %d, want %d", tc.got.Code, tc.code)
			}
			if !tc.got.Valid() {
				t.Error("a constructed outcome is not valid")
			}
		})
	}
}

// TestMismatchedPairsAreInvalid, so Valid is a real check at a transport boundary
// rather than a restatement of the constructors.
func TestMismatchedPairsAreInvalid(t *testing.T) {
	t.Parallel()
	for _, o := range []gnosis.Outcome{
		{Status: gnosis.StatusOK, Code: gnosis.CodeError},
		{Status: gnosis.StatusError, Code: gnosis.CodeOK},
		{Status: gnosis.StatusFindings, Code: gnosis.CodeBlocked},
		{Status: "invented", Code: gnosis.CodeOK},
	} {
		if o.Valid() {
			t.Errorf("%+v is valid", o)
		}
	}
}

// TestFindingsIsNotAnError is SPEC §17's insistence, asserted: a corpus with
// problems must not look like a broken tool, because a CI job branches on exactly
// that difference.
func TestFindingsIsNotAnError(t *testing.T) {
	t.Parallel()
	f := gnosis.Findings(gnosis.ReasonDuplicateIdentity, "two documents", nil)
	if f.Status == gnosis.StatusError {
		t.Error("a finding reports as a tool error")
	}
	if f.Code == gnosis.CodeError {
		t.Error("a finding exits with the broken-tool code")
	}
}

// TestEmptyFieldsAreOmitted: an ok outcome must not carry an empty reason, or a
// caller checking for the field's presence learns nothing.
func TestEmptyFieldsAreOmitted(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(gnosis.OK(nil))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got, want := string(b), `{"status":"ok","code":0}`; got != want {
		t.Errorf("encoding = %s, want %s", got, want)
	}
}
