package bundle_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// challengeID is the identifier the fixtures file challenges under.
const challengeID = gnosis.ID("01932c04-8b21-7f03-a5e1-3d92f7c04a1b")

// TestAppendChallengeKeepsEveryOtherKey is the case that decided the implementation:
// re-rendering the document through `renderConcept` would drop every key
// `conceptDoc` does not model, and a hand-written page carries several.
func TestAppendChallengeKeepsEveryOtherKey(t *testing.T) {
	t.Parallel()

	original := "---\n" +
		"type: Rule\n" +
		"title: Request Timeout\n" +
		"gnosis_id: 01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d\n" +
		"gnosis_limitations:\n" +
		"  - does not cover batch jobs\n" +
		"sources:\n" +
		"  - resource: https://example.org/vendor\n" +
		"    scope: true\n" +
		"gnosis_claims:\n" +
		"  - id: c1\n" +
		"    anchor: The timeout is 400ms.\n" +
		"---\n" +
		"# Request Timeout\n\nThe timeout is 400ms.\n"

	out, err := bundle.AppendChallenge([]byte(original), &gnosis.Challenge{
		ID: challengeID, Class: gnosis.ChallengeCoverage, By: "human:dana",
		At:        "2026-08-20T11:42:09Z",
		Rationale: "The quote is about connection retries; the claim is about requests.",
	})
	if err != nil {
		t.Fatalf("AppendChallenge: %v", err)
	}

	text := string(out)
	for _, key := range []string{
		"type: Rule", "title: Request Timeout", "gnosis_limitations",
		"does not cover batch jobs", "sources:", "scope: true", "gnosis_claims",
	} {
		if !strings.Contains(text, key) {
			t.Errorf("filing a challenge dropped %q:\n%s", key, text)
		}
	}
	if !strings.Contains(text, "gnosis_challenges:") ||
		!strings.Contains(text, "class: coverage") ||
		!strings.Contains(text, "state: open") {
		t.Errorf("the challenge did not land:\n%s", text)
	}
	// The body is what a challenge must never touch: it contests a claim, it does
	// not edit one.
	if !strings.HasSuffix(text, "# Request Timeout\n\nThe timeout is 400ms.\n") {
		t.Errorf("the body changed:\n%s", text)
	}
}

// TestAppendChallengeAppendsToAnExistingList. Appended rather than prepended, so the
// list reads in the order challenges were filed — the order --unanswered sorts by.
func TestAppendChallengeAppendsToAnExistingList(t *testing.T) {
	t.Parallel()

	original := "---\n" +
		"type: Rule\n" +
		"title: Request Timeout\n" +
		"gnosis_challenges:\n" +
		"  - id: 01932c04-8b21-7f03-a5e1-000000000001\n" +
		"    class: replay\n" +
		"    by: human:sam\n" +
		"    at: 2026-08-01T00:00:00Z\n" +
		"    rationale: |\n" +
		"      The quotation is not in the archived text.\n" +
		"    state: open\n" +
		"gnosis_claims:\n" +
		"  - id: c1\n" +
		"    anchor: The timeout is 400ms.\n" +
		"---\nBody.\n"

	out, err := bundle.AppendChallenge([]byte(original), &gnosis.Challenge{
		ID: challengeID, Class: gnosis.ChallengeScope, By: "human:dana",
		At: "2026-08-20T11:42:09Z", Rationale: "The limitations are incomplete.",
	})
	if err != nil {
		t.Fatalf("AppendChallenge: %v", err)
	}

	text := string(out)
	// Counted by `class:`, which only a challenge carries. Counting `- id:` would
	// count the claims list too, which is how the first version of this assertion
	// failed against correct output.
	if strings.Count(text, "class: ") != 2 {
		t.Errorf("want two challenges, got:\n%s", text)
	}
	// The new entry sits after the existing one and before the next top-level key,
	// which is where the block ends.
	first := strings.Index(text, "class: replay")
	second := strings.Index(text, "class: scope")
	claims := strings.Index(text, "gnosis_claims:")
	if first >= second || second >= claims {
		t.Errorf("the entry did not land at the end of the block:\n%s", text)
	}
}

// TestAppendChallengeRefusesADoubt. §10.7.2: a reader who cannot say why a claim is
// wrong has filed a doubt, and the corpus has no way to act on a doubt.
func TestAppendChallengeRefusesADoubt(t *testing.T) {
	t.Parallel()

	original := "---\ntype: Rule\ntitle: T\n---\nBody.\n"
	for name, rationale := range map[string]string{
		"empty":      "",
		"whitespace": "   \n\t",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := bundle.AppendChallenge([]byte(original), &gnosis.Challenge{
				ID: challengeID, Class: gnosis.ChallengeScope, By: "human:dana",
				At: "2026-08-20T11:42:09Z", Rationale: rationale,
			})
			if errs.ErrorCode(err) != errs.EINVALID {
				t.Errorf("a challenge with no rationale was accepted: %v", err)
			}
		})
	}
}

// TestAppendChallengeCarriesAMultilineRationale. Written as a block scalar because it
// is prose somebody typed: a colon, a quotation mark or a newline would each need
// escaping inline, and an escape somebody got wrong loses the objection.
func TestAppendChallengeCarriesAMultilineRationale(t *testing.T) {
	t.Parallel()

	original := "---\ntype: Rule\ntitle: T\n---\nBody.\n"
	out, err := bundle.AppendChallenge([]byte(original), &gnosis.Challenge{
		ID: challengeID, Class: gnosis.ChallengeContradiction, By: "human:dana",
		At: "2026-08-20T11:42:09Z",
		Rationale: "This conflicts with c/other.md: it says 3.\n" +
			"Nothing noticed because they share no source.",
	})
	if err != nil {
		t.Fatalf("AppendChallenge: %v", err)
	}
	if !strings.Contains(string(out), "Nothing noticed because they share no source.") {
		t.Errorf("the second line of the rationale is missing:\n%s", out)
	}
}

// TestAppendChallengeRefusesADocumentItCannotParse. A challenge against a malformed
// document is refused rather than written, because the check that the surgery kept
// every key cannot run against a document whose keys nobody could read.
func TestAppendChallengeRefusesADocumentItCannotParse(t *testing.T) {
	t.Parallel()

	_, err := bundle.AppendChallenge([]byte("no frontmatter here\n"), &gnosis.Challenge{
		ID: challengeID, Class: gnosis.ChallengeScope, By: "human:dana",
		At: "2026-08-20T11:42:09Z", Rationale: "The limitations are incomplete.",
	})
	if err == nil {
		t.Error("a challenge was filed against a document that does not parse")
	}
}
