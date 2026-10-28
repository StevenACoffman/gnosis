package relay_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/finding"
)

// TestParseCriticReplyReadsAVerdict, including the case a corpus most wants recorded:
// a critic that looked and found nothing.
func TestParseCriticReplyReadsAVerdict(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		src           string
		findings      int
		firstCategory string
	}{
		"a finding with a category": {
			src: "```yaml\n" +
				"findings:\n" +
				"  - category: scope\n" +
				"    message: The claim generalises beyond the one deployment measured.\n" +
				"examined:\n  - whether the quotation supports the asserted scope\n" +
				"not_examined:\n  - aspect: the source's methodology\n" +
				"    reason: this excerpt does not include it\n" +
				"```\n",
			findings: 1, firstCategory: "critic:scope",
		},
		// Finding nothing is a real answer, and Parsed is what separates it from a
		// reply nobody read.
		"nothing found": {
			src: "```yaml\nfindings: []\n" +
				"examined:\n  - whether the quotation supports the asserted scope\n```\n",
			findings: 0,
		},
		"a category nobody listed is filed as it is": {
			src: "```yaml\n" +
				"findings:\n  - category: Circular Reasoning\n    message: It assumes it.\n" +
				"examined:\n  - the inference\n```\n",
			findings: 1, firstCategory: "critic:circular-reasoning",
		},
		"no category at all": {
			src: "```yaml\n" +
				"findings:\n  - message: Something is wrong and I cannot name the kind.\n" +
				"examined:\n  - the inference\n```\n",
			findings: 1, firstCategory: "critic:unclassified",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := relay.ParseCriticReply([]byte(tc.src))
			if err != nil {
				t.Fatalf("ParseCriticReply: %v", err)
			}
			assertGapsWhole(t, got.NotExamined)
			if !got.Parsed {
				t.Error("a parsed reply does not report itself parsed")
			}
			if len(got.Findings) != tc.findings {
				t.Fatalf("got %d findings, want %d", len(got.Findings), tc.findings)
			}
			if tc.findings > 0 && got.Findings[0].Category != tc.firstCategory {
				t.Errorf("category = %q, want %q",
					got.Findings[0].Category, tc.firstCategory)
			}
		})
	}
}

// assertGapsWhole is the invariant every accepted reply carries: an unexamined aspect
// arrives with both halves or does not arrive at all.
//
// A helper rather than four lines in the subtest, because the parser is what is under
// test in three more places below and each of them wants the same guarantee.
func assertGapsWhole(t *testing.T, gaps []finding.Unexamined) {
	t.Helper()

	for _, gap := range gaps {
		if !gap.Valid() {
			t.Errorf("an unexamined aspect arrived half-recorded: %+v", gap)
		}
	}
}

// TestParseCriticReplyRefusesSilence is the strictness §10.5 asks for: a reply with no
// finding in an area cannot be told from one that never looked, and the gate ships on
// that silence.
func TestParseCriticReplyRefusesSilence(t *testing.T) {
	t.Parallel()

	for name, src := range map[string]string{
		"no examined key": "```yaml\nfindings: []\n```\n",
		"an empty list":   "```yaml\nfindings: []\nexamined: []\n```\n",
		"blank bullets":   "```yaml\nfindings: []\nexamined:\n  - \"\"\n  - \"  \"\n```\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := relay.ParseCriticReply([]byte(src))
			if errs.ErrorCode(err) != errs.EINVALID {
				t.Fatalf("a reply declaring nothing examined was accepted: %v", err)
			}
			if !strings.Contains(err.Error(), "examined") {
				t.Errorf("the refusal does not name the missing block: %v", err)
			}
		})
	}
}

// TestParseCriticReplyReportsEveryProblemAtOnce, because each round trip an agent
// spends learning about the next defect costs a model call.
func TestParseCriticReplyReportsEveryProblemAtOnce(t *testing.T) {
	t.Parallel()

	src := "```yaml\nfindings:\n  - category: scope\n    message: \"\"\n" +
		"  - category: omission\n    message: \"   \"\nexamined: []\n```\n"
	_, err := relay.ParseCriticReply([]byte(src))
	if err == nil {
		t.Fatal("a reply with two empty findings and no examined block was accepted")
	}
	for _, want := range []string{"examined", "finding 1", "finding 2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}
}

// TestParseCriticReplyHoldsTheSameBlockRuleAsExtraction, which is the point of the
// shared helper: two parsers deciding separately what a well-formed reply looks like is
// one place for them to disagree.
func TestParseCriticReplyHoldsTheSameBlockRuleAsExtraction(t *testing.T) {
	t.Parallel()

	for name, src := range map[string]string{
		"no block":   "I had a look and it seems fine.\n",
		"two blocks": "```yaml\nexamined:\n  - a\n```\n```yaml\nexamined:\n  - b\n```\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := relay.ParseCriticReply([]byte(src)); errs.ErrorCode(err) != errs.EINVALID {
				t.Errorf("a reply that is not exactly one block was accepted: %v", err)
			}
		})
	}
}

// TestParseCriticReplyRefusesAGapWithNoReason is the half that makes an unexamined
// aspect actionable, and skillet's own comment on the type gives the rule: "an
// unparseable answer must not advance anything on trust, and silently discarding half of
// one is how a reply that says nothing passes for a reply that found nothing."
func TestParseCriticReplyRefusesAGapWithNoReason(t *testing.T) {
	t.Parallel()

	for name, src := range map[string]string{
		"an aspect with no reason": "```yaml\nfindings: []\n" +
			"examined:\n  - the scope\n" +
			"not_examined:\n  - aspect: the methodology\n```\n",
		"a reason with no aspect": "```yaml\nfindings: []\n" +
			"examined:\n  - the scope\n" +
			"not_examined:\n  - reason: the excerpt is short\n```\n",
		"both blank": "```yaml\nfindings: []\n" +
			"examined:\n  - the scope\n" +
			"not_examined:\n  - aspect: \"  \"\n    reason: \"\"\n```\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := relay.ParseCriticReply([]byte(src))
			if errs.ErrorCode(err) != errs.EINVALID {
				t.Fatalf("a half-recorded gap was accepted: %v", err)
			}
			if !strings.Contains(err.Error(), "unexamined entry") {
				t.Errorf("the refusal does not name what is wrong: %v", err)
			}
		})
	}
}
