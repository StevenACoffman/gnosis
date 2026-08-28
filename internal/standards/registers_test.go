package standards_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/standards"
)

// TestTheShippedRegistersMatchRealSentences is §18.4.1 applied to this list, and it is
// the case a word list cannot state about itself.
//
// The matcher is whole-word, so a marker only fires on the exact phrase. Real prose
// inflects the copula — *restarts **are** associated with*, *the failure **was**
// associated with* — and a list carrying only the "is" form matches none of it. That
// defect was found by a fixture omitting the bare form and reading as a code error;
// pinning it here means the file is checked against sentences of the shape a claim and a
// quotation actually have.
func TestTheShippedRegistersMatchRealSentences(t *testing.T) {
	t.Parallel()

	got, err := standards.LoadRegisters(standards.DefaultRegisters())
	if err != nil {
		t.Fatalf("the shipped register list does not load: %v", err)
	}

	cases := []struct {
		sentence string
		role     standards.Register
	}{
		{"Pod restarts are associated with lower memory use.", standards.RegisterAssociation},
		{"The failure was associated with a full disk.", standards.RegisterAssociation},
		{"Queue depth correlates with p99 latency.", standards.RegisterAssociation},
		{"Restarting the pod causes the leak to clear.", standards.RegisterIntervention},
		{"Raising the timeout leads to fewer retries.", standards.RegisterIntervention},
	}
	for _, c := range cases {
		if !matches(got.Words(c.role), c.sentence) {
			t.Errorf("no %s marker matches %q", c.role, c.sentence)
		}
	}
}

// TestOrdinaryProseCarriesNoRegister is the negative half. "because" contains "cause",
// and a list matching on substrings would put every explanatory sentence in this corpus
// on the intervention rung.
func TestOrdinaryProseCarriesNoRegister(t *testing.T) {
	t.Parallel()

	got, err := standards.LoadRegisters(standards.DefaultRegisters())
	if err != nil {
		t.Fatalf("the shipped register list does not load: %v", err)
	}

	ordinary := []string{
		"The cache is cleared because the process restarted.",
		"We chose SQLite for the reasons recorded in log.md.",
		"The causal analysis is out of scope for this document.",
	}
	for _, s := range ordinary {
		for _, role := range []standards.Register{
			standards.RegisterIntervention, standards.RegisterAssociation,
		} {
			if matches(got.Words(role), s) {
				t.Errorf("%q matched a %s marker", s, role)
			}
		}
	}
}

// matches reports whether any marker appears in s as a whole word, the way the check
// does. It is written here rather than imported because internal/lint and
// internal/standards are siblings and do not import each other.
func matches(words []string, s string) bool {
	padded := " " + strings.ToLower(s) + " "
	for _, w := range words {
		if strings.Contains(padded, " "+w+" ") ||
			strings.Contains(padded, " "+w+",") ||
			strings.Contains(padded, " "+w+".") {
			return true
		}
	}
	return false
}
