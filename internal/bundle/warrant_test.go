package bundle_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// warranted is a two-claim document, so every test here can assert that the warrant
// landed on one claim and not the other.
const warranted = "---\n" +
	"type: Rule\n" +
	"title: Request Timeout\n" +
	"gnosis_limitations:\n" +
	"  - does not cover batch jobs\n" +
	"gnosis_claims:\n" +
	"  - id: c1\n" +
	"    anchor: The timeout is 400ms.\n" +
	"    verified:\n" +
	"      - by: human:priya\n        at: 2026-08-01T00:00:00Z\n" +
	"  - id: c2\n" +
	"    anchor: Retries share the budget.\n" +
	"---\n# Request Timeout\n\nThe timeout is 400ms. Retries share the budget.\n"

// TestAppendWarrantLandsOnTheNamedClaim is the property the surgery could plausibly
// break: an insertion point computed from the wrong entry produces a document that
// parses, keeps its keys and its body, and records a decision about the wrong claim.
func TestAppendWarrantLandsOnTheNamedClaim(t *testing.T) {
	t.Parallel()

	out, err := bundle.AppendWarrant([]byte(warranted), "c2", &gnosis.Warrant{
		By: "human:priya", At: "2026-08-19T14:02:11Z", Authority: "sole",
		Rationale: "Chose the vendor's published limit over the 2024 post.",
	})
	if err != nil {
		t.Fatalf("AppendWarrant: %v", err)
	}

	text := string(out)
	// The block sits inside c2's entry, which is after c2's anchor and after
	// everything c1 declared.
	c1 := strings.Index(text, "id: c1")
	c2 := strings.Index(text, "id: c2")
	warrant := strings.Index(text, "gnosis_warrant:")
	if warrant < c2 || c2 < c1 {
		t.Fatalf("the warrant did not land inside c2's entry:\n%s", text)
	}
	// And c1 keeps its own fields, including the verification list the entry
	// boundary has to step over.
	if !strings.Contains(text, "by: human:priya\n        at: 2026-08-01T00:00:00Z") {
		t.Errorf("c1's verification list was disturbed:\n%s", text)
	}
	if !strings.Contains(text, "gnosis_limitations") {
		t.Errorf("a top-level key was dropped:\n%s", text)
	}
	if !strings.HasSuffix(text, "The timeout is 400ms. Retries share the budget.\n") {
		t.Errorf("the body changed:\n%s", text)
	}
}

// TestAppendWarrantRefusesToOverwriteOne. §10.6.5 makes a reversal a new warrant
// naming the one it overturns; replacing one silently would delete the record that
// makes reversal informative.
func TestAppendWarrantRefusesToOverwriteOne(t *testing.T) {
	t.Parallel()

	once, err := bundle.AppendWarrant([]byte(warranted), "c1", &gnosis.Warrant{
		By: "human:priya", Rationale: "The vendor limit is newer.",
	})
	if err != nil {
		t.Fatalf("AppendWarrant: %v", err)
	}
	_, err = bundle.AppendWarrant(once, "c1", &gnosis.Warrant{
		By: "human:marcus", Rationale: "Actually the post was right.",
	})
	if errs.ErrorCode(err) != errs.EINVALID {
		t.Errorf("a second warrant overwrote the first: %v", err)
	}
}

// TestAppendWarrantRefusesAnUnknownClaim. ENOTFOUND rather than a warrant appended
// somewhere plausible: a decision recorded against a claim that does not exist is
// worse than a refused one, because nothing later would report it.
func TestAppendWarrantRefusesAnUnknownClaim(t *testing.T) {
	t.Parallel()

	_, err := bundle.AppendWarrant([]byte(warranted), "c9", &gnosis.Warrant{
		By: "human:priya", Rationale: "The vendor limit is newer.",
	})
	if errs.ErrorCode(err) != errs.ENOTFOUND {
		t.Errorf("a warrant was written for a claim that does not exist: %v", err)
	}
}

// TestAppendWarrantRefusesAWarrantWithNoReasoning, at every authority including sole.
func TestAppendWarrantRefusesAWarrantWithNoReasoning(t *testing.T) {
	t.Parallel()

	_, err := bundle.AppendWarrant([]byte(warranted), "c1", &gnosis.Warrant{
		By: "human:priya", Authority: "sole",
	})
	if errs.ErrorCode(err) != errs.EINVALID {
		t.Errorf("a warrant with no rationale was accepted: %v", err)
	}
}

// TestAppendWarrantOmitsTheFieldsNobodySet. An empty `co_signed_by:` asserts that
// nobody co-signed, which is a different thing from a decision that needed no
// co-signer.
func TestAppendWarrantOmitsTheFieldsNobodySet(t *testing.T) {
	t.Parallel()

	out, err := bundle.AppendWarrant([]byte(warranted), "c1", &gnosis.Warrant{
		By: "human:priya", Rationale: "The vendor limit is newer.",
	})
	if err != nil {
		t.Fatalf("AppendWarrant: %v", err)
	}
	for _, key := range []string{"co_signed_by:", "override:", "reverses:", "review:"} {
		if strings.Contains(string(out), key) {
			t.Errorf("%s was written for a decision that set none:\n%s", key, out)
		}
	}
}

// TestCloseChallengeClosesOnlyTheNamedOne. An entry boundary computed wrongly would
// close somebody else's challenge, and the document would parse and read as an
// ordinary resolution.
func TestCloseChallengeClosesOnlyTheNamedOne(t *testing.T) {
	t.Parallel()

	const two = "---\n" +
		"type: Rule\ntitle: T\n" +
		"gnosis_challenges:\n" +
		"  - id: 01932c04-8b21-7f03-a5e1-000000000001\n" +
		"    class: replay\n    by: human:sam\n    at: 2026-08-01T00:00:00Z\n" +
		"    rationale: |\n      Not in the archived text.\n    state: open\n" +
		"  - id: 01932c04-8b21-7f03-a5e1-000000000002\n" +
		"    class: scope\n    by: human:dana\n    at: 2026-08-02T00:00:00Z\n" +
		"    rationale: |\n      The limitations are incomplete.\n    state: open\n" +
		"---\nBody.\n"

	out, err := bundle.CloseChallenge([]byte(two), "01932c04-8b21-7f03-a5e1-000000000002")
	if err != nil {
		t.Fatalf("CloseChallenge: %v", err)
	}
	text := string(out)
	if strings.Count(text, "state: open") != 1 ||
		strings.Count(text, "state: closed") != 1 {
		t.Errorf("closing one challenge changed the other:\n%s", text)
	}
	// The closed one is the second, which is the one that was named.
	if strings.Index(text, "state: closed") < strings.Index(text, "class: scope") {
		t.Errorf("the wrong challenge was closed:\n%s", text)
	}
}

// TestCloseChallengeRefusesAnUnknownID rather than closing nothing quietly: a
// resolution that silently matched no challenge would report success while the
// objection stayed open.
func TestCloseChallengeRefusesAnUnknownID(t *testing.T) {
	t.Parallel()

	const one = "---\ntype: Rule\ntitle: T\n" +
		"gnosis_challenges:\n" +
		"  - id: 01932c04-8b21-7f03-a5e1-000000000001\n" +
		"    class: replay\n    by: human:sam\n    at: 2026-08-01T00:00:00Z\n" +
		"    rationale: |\n      Not in the archived text.\n    state: open\n" +
		"---\nBody.\n"

	_, err := bundle.CloseChallenge([]byte(one), "01932c04-8b21-7f03-a5e1-000000000009")
	if errs.ErrorCode(err) != errs.ENOTFOUND {
		t.Errorf("closing an unknown challenge did not report ENOTFOUND: %v", err)
	}
}
