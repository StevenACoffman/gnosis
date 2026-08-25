package standards_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/standards"
)

// TestTheShippedSuiteIsEmptyAndValid is the state §11.0.2 asks for and the one a
// reader is most likely to mistake for an oversight.
//
// Cases are "authored when a real query disappoints, never invented up front", so the
// file ships with none — and it must still load, because a suite that failed to parse
// until somebody added a case would be a suite nobody added a case to.
func TestTheShippedSuiteIsEmptyAndValid(t *testing.T) {
	t.Parallel()

	got, err := standards.LoadRetrieval(standards.DefaultRetrieval())
	if err != nil {
		t.Fatalf("the shipped case file does not load: %v", err)
	}
	if len(got.Cases) != 0 {
		t.Errorf("the shipped file invents %d case(s)", len(got.Cases))
	}
}

// TestACaseHoldsWhenEveryExpectedTitleCameBack, and the extra-results half is the
// part worth stating: a corpus that grows a second relevant document has not
// regressed, and a case failing on that would train an author to delete the case
// rather than read the result.
func TestACaseHoldsWhenEveryExpectedTitleCameBack(t *testing.T) {
	t.Parallel()

	c := standards.Case{Query: "retry budget", Titles: []string{"Retry Budget"}, Why: "x"}
	for name, tc := range map[string]struct {
		got  []string
		want standards.Verdict
	}{
		"exactly":                {[]string{"Retry Budget"}, standards.VerdictHeld},
		"with something else":    {[]string{"Deploy Runbook", "Retry Budget"}, standards.VerdictHeld},
		"differing only in case": {[]string{"retry budget"}, standards.VerdictHeld},
		"with a stray space":     {[]string{" Retry Budget "}, standards.VerdictHeld},
		"absent":                 {[]string{"Deploy Runbook"}, standards.VerdictFailed},
		"nothing at all":         {nil, standards.VerdictFailed},
		// Not a substring match: a document about caches must not satisfy an
		// expectation of a document about cache lifetime.
		"a prefix of it": {[]string{"Retry"}, standards.VerdictFailed},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := c.Grade(tc.got); got != tc.want {
				t.Errorf("Grade(%q) = %v, want %v", tc.got, got, tc.want)
			}
		})
	}
}

// TestACaseMayRequireThatNothingIsHeld is the half §11.0.2 asks for by name.
//
// A corpus that answers every query with its best guess cannot say "we do not know",
// and that is the answer §14.3's whole vocabulary exists to make expressible.
func TestACaseMayRequireThatNothingIsHeld(t *testing.T) {
	t.Parallel()

	c := standards.Case{Query: "ingress annotations", Nothing: true, Why: "out of scope"}
	if got := c.Grade(nil); got != standards.VerdictHeld {
		t.Errorf("an empty result failed a nothing-case: %v", got)
	}
	if got := c.Grade([]string{"Something"}); got != standards.VerdictFailed {
		t.Errorf("a nothing-case held with a result: %v", got)
	}
	// And it names no missing title, because none was expected — the verdict is what
	// reports that failure, and a list of absent titles would invent an expectation.
	if m := c.Missing([]string{"Something"}); len(m) != 0 {
		t.Errorf("a nothing-case named %q as missing", m)
	}
}

// TestTheUnrunVerdictIsTheZeroValue is the discipline every enum here follows: a case
// nobody graded must not read as one that passed.
func TestTheUnrunVerdictIsTheZeroValue(t *testing.T) {
	t.Parallel()

	var zero standards.Verdict
	if zero != standards.VerdictUnrun || zero.String() != "unrun" {
		t.Errorf("the zero verdict is %v", zero)
	}
}

// TestAnUngradableCaseIsRefusedAtLoad is where the file's own contradictions are
// caught, and the contradiction is the interesting one: a case expecting titles *and*
// declaring nothing is not a typo, and grading it either way would make the file mean
// whichever the implementation checked first.
func TestAnUngradableCaseIsRefusedAtLoad(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ src, want string }{
		"no query": {
			`[[case]]
titles = ["A"]
why = "x"`, "no query",
		},
		"both titles and nothing": {
			`[[case]]
query = "q"
titles = ["A"]
nothing = true
why = "x"`, "cannot require both",
		},
		"neither": {
			`[[case]]
query = "q"
why = "x"`, "nothing about the corpus would make it fail",
		},
		"no why": {
			`[[case]]
query = "q"
titles = ["A"]`, "no `why`",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := standards.LoadRetrieval([]byte(tc.src))
			if err == nil {
				t.Fatalf("an ungradable case loaded: %s", tc.src)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not say why: %v", err)
			}
		})
	}
}

// TestEveryMalformedCaseIsNamedAtOnce, so one run tells an author everything to fix
// rather than making them re-run per problem.
func TestEveryMalformedCaseIsNamedAtOnce(t *testing.T) {
	t.Parallel()

	_, err := standards.LoadRetrieval([]byte(`
[[case]]
titles = ["A"]
why = "x"

[[case]]
query = "q"
titles = ["A"]
`))
	if err == nil {
		t.Fatal("two malformed cases loaded")
	}
	if !strings.Contains(err.Error(), "case 1") || !strings.Contains(err.Error(), "case 2") {
		t.Errorf("only one problem was named: %v", err)
	}
}
