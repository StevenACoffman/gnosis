package standards_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/standards"
	"github.com/StevenACoffman/skillet/errs"
)

// TestTheSampleSeedLoads. A seed its own loader rejects would break every bundle
// created from it, which is why each standards file has this test.
func TestTheSampleSeedLoads(t *testing.T) {
	t.Parallel()

	s, err := standards.LoadSample(standards.DefaultSample())
	if err != nil {
		t.Fatalf("the embedded seed does not load: %v", err)
	}
	if s.Seed.Value == 0 {
		t.Error("the seed is zero; a draw keyed on nothing is not reproducible on purpose")
	}
	if strings.TrimSpace(s.Seed.Rationale) == "" {
		t.Error("the seed carries no rationale")
	}
}

// TestASeedWithNoRationaleIsRefused, which is the structural half of §6.2: there is
// no way to express a value here without saying why it holds.
func TestASeedWithNoRationaleIsRefused(t *testing.T) {
	t.Parallel()

	_, err := standards.LoadSample([]byte("[seed]\nvalue = 1\n"))
	if err == nil {
		t.Fatal("a seed with no rationale loaded")
	}
	if errs.ErrorCode(err) != errs.EINVALID {
		t.Errorf("code = %q, want EINVALID", errs.ErrorCode(err))
	}
}

// TestAMistypedSampleKeyIsAnError, not a silent no-op. This is §5.2's reason for
// TOML: an author who believes they changed the seed and did not would produce a
// draw nobody can account for.
func TestAMistypedSampleKeyIsAnError(t *testing.T) {
	t.Parallel()

	_, err := standards.LoadSample([]byte("[sed]\nvalue = 1\nrationale = \"x\"\n"))
	if err == nil {
		t.Fatal("a mistyped key loaded")
	}
	if !strings.Contains(err.Error(), "unrecognised key") {
		t.Errorf("error = %v", err)
	}
}
