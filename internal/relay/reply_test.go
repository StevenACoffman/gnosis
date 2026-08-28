package relay_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/skillet/errs"
)

const goodReply = "Here is what I found.\n\n" +
	"```yaml\n" +
	"title: Cache Lifetime\n" +
	"type: Reference\n" +
	"claims:\n" +
	"  - text: The cache is cleared on restart.\n" +
	"    quotes:\n" +
	"      - The cache is cleared on restart and holds nothing\n" +
	"```\n"

func TestParsesAWellFormedReply(t *testing.T) {
	t.Parallel()
	got, err := relay.ParseReply([]byte(goodReply))
	if err != nil {
		t.Fatalf("a well-formed reply was rejected: %v", err)
	}
	if !got.Parsed {
		t.Error("Parsed is false on a successful parse")
	}
	if got.Title != "Cache Lifetime" || got.Type != "Reference" {
		t.Errorf("title/type = %q/%q", got.Title, got.Type)
	}
	if len(got.Claims) != 1 || len(got.Claims[0].Quotes) != 1 {
		t.Fatalf("claims = %+v", got.Claims)
	}
}

// TestTheZeroReplyIsNotAnEmptyReply. An empty claim list is a legitimate answer —
// "this source supports nothing I can quote" is useful — and a Reply nobody parsed
// is not. Without Parsed the two are the same value.
func TestTheZeroReplyIsNotAnEmptyReply(t *testing.T) {
	t.Parallel()
	var zero relay.Reply
	if zero.Parsed {
		t.Error("the zero Reply claims to have been parsed")
	}

	empty, err := relay.ParseReply([]byte(
		"```yaml\ntitle: Nothing Quotable\ntype: Reference\nclaims: []\n```\n"))
	if err != nil {
		t.Fatalf("an empty claim list was rejected: %v", err)
	}
	if !empty.Parsed {
		t.Error("a parsed empty reply does not say it was parsed")
	}
	if len(empty.Claims) != 0 {
		t.Errorf("claims = %+v, want none", empty.Claims)
	}
}

func TestMalformedRepliesAreRejectedWhole(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		src  string
		want string
	}{
		"no block": {"I could not find anything.", "no ```yaml block"},
		"not yaml": {"```yaml\n: : :\n```\n", "relay.ParseReply"},
		"no title": {"```yaml\ntype: Reference\nclaims: []\n```\n", "title is empty"},
		"no type":  {"```yaml\ntitle: A\nclaims: []\n```\n", "type is empty"},
		"claim with no text": {
			"```yaml\ntitle: A\ntype: R\nclaims:\n  - quotes: [\"a b c d e f\"]\n```\n",
			"claim 1 has no text",
		},
		"claim with no quotation": {
			"```yaml\ntitle: A\ntype: R\nclaims:\n  - text: An assertion.\n```\n",
			"claim 1 offers no quotation",
		},
		"two blocks": {
			"```yaml\ntitle: A\ntype: R\nclaims: []\n```\n\n```yaml\ntitle: B\ntype: R\nclaims: []\n```\n",
			"2 yaml blocks",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := relay.ParseReply([]byte(tc.src))
			if err == nil {
				t.Fatalf("%s was accepted: %+v", name, got)
			}
			if errs.ErrorCode(err) != errs.EINVALID {
				t.Errorf("code = %q, want EINVALID", errs.ErrorCode(err))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
			// Rejected whole: nothing partial comes back for a caller to use.
			if got.Parsed || len(got.Claims) != 0 {
				t.Errorf("a rejected reply returned content: %+v", got)
			}
		})
	}
}

// TestEveryProblemIsReportedAtOnce, so an agent asked to fix its reply learns
// everything wrong with it in one round trip rather than one problem per attempt.
func TestEveryProblemIsReportedAtOnce(t *testing.T) {
	t.Parallel()
	_, err := relay.ParseReply([]byte(
		"```yaml\nclaims:\n  - text: \"\"\n  - text: A\n```\n"))
	if err == nil {
		t.Fatal("a reply with four problems was accepted")
	}
	for _, want := range []string{"title is empty", "type is empty", "claim 1", "claim 2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q omits %q", err, want)
		}
	}
}

// TestAnUnterminatedFenceIsNotABlock. Accepting one would mean parsing whatever
// the reply happened to end with.
func TestAnUnterminatedFenceIsNotABlock(t *testing.T) {
	t.Parallel()
	_, err := relay.ParseReply([]byte("```yaml\ntitle: A\ntype: R\nclaims: []\n"))
	if err == nil {
		t.Fatal("an unterminated fence was parsed")
	}
}

// TestAReplyWithNoLeadIsAccepted is §5.8.3's argument one field over: §17.4 makes a lead
// a *checked property*, and reporting is a review signal where refusing is a gate.
// Turning one into the other would make the corpus decline knowledge over a summary.
func TestAReplyWithNoLeadIsAccepted(t *testing.T) {
	t.Parallel()

	const src = "```yaml\ntype: Rule\ntitle: Retry Budget\nclaims:\n" +
		"  - text: Retries are capped at three attempts.\n    quotes:\n" +
		"      - the service retries three times before giving up\n```\n"
	got, err := relay.ParseReply([]byte(src))
	if err != nil {
		t.Fatalf("a reply with no lead was refused: %v", err)
	}
	if len(got.Claims) != 1 {
		t.Fatalf("want one claim, got %d", len(got.Claims))
	}
	if got.Claims[0].Lead != "" {
		t.Errorf("a lead was invented: %q", got.Claims[0].Lead)
	}
}

// TestALeadIsCarriedFromTheReply is the field arriving where §17.4 needs it. It is
// authored by the model rather than derived, and §17.4 records why: a rule that picked
// the conclusion clause would make the check testing that rule vacuous.
func TestALeadIsCarriedFromTheReply(t *testing.T) {
	t.Parallel()

	const src = "```yaml\ntype: Rule\ntitle: Retry Budget\nclaims:\n" +
		"  - text: Retries are capped at three attempts.\n" +
		"    lead: Cap retries at three.\n    quotes:\n" +
		"      - the service retries three times before giving up\n```\n"
	got, err := relay.ParseReply([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Claims[0].Lead != "Cap retries at three." {
		t.Errorf("lead = %q, want the reply's own words", got.Claims[0].Lead)
	}
}

// TestThePromptAsksForALead is the contract in the direction that is easy to break: the
// field can be parsed and never requested, and then no model would ever send one.
func TestThePromptAsksForALead(t *testing.T) {
	t.Parallel()
	prompt := relay.Render(&relay.Request{
		URI: "https://x/doc", SourceHash: "abc", Text: "Some source text.",
		Model: relay.Model{Name: "test", Version: "1"},
	})
	for _, want := range []string{"lead:", "conclusion"} {
		if !strings.Contains(prompt.Text, want) {
			t.Errorf("the prompt does not ask for a lead (%q missing)", want)
		}
	}
}
