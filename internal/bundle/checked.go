package bundle

import (
	"strconv"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/segment"
)

// checked is the outcome of validating one reply's claims against tier 0.
type checked struct {
	// claims is every segmented claim, in order.
	claims []segment.Claim

	// supported is every passage the archive was found to contain, fold-normalised.
	//
	// **Per passage rather than per claim, because the synthesis gate asks a
	// different question from the admission gate.** Admission passes a claim with
	// *at least one* supporting quotation; a rewrite that kept a claim while
	// dropping the passage that made it checkable has lost evidence, and a
	// claim-level answer cannot see that.
	supported map[string]bool

	// unsupported names claims whose quotations were looked for and not found.
	// This is fabrication or corruption, and it blocks.
	//
	// Each entry is `describe`'s form — "claim 2: <text truncated>" — because its
	// reader is a refusal message somebody is looking at now, and the position tells
	// them which entry of the reply in front of them to fix.
	unsupported []string

	// withheld is the same claims as their own text, for the durable record.
	//
	// **Two slices because they have two readers and the difference is time.** The
	// position in "claim 2" is meaningful while the reply is on screen and meaningless
	// in a trail read next month, where the reply is gone and the numbering refers to
	// nothing. Truncation is wrong there for the same reason: the record of what a
	// source was found not to support is the assertion, not a preview of it.
	withheld []string

	// unchecked names claims whose quotations could not be checked at all —
	// every passage shorter than MinPassageWords, or no archived text to check
	// against. This is the outcome PLAN §3 says matters here for the first time:
	// a claim whose passages were never checked must not read as clean.
	unchecked []string
}

// outcome renders a refusal, keeping the two failure kinds apart.
func (k *checked) outcome(cmd *command.Admit) gnosis.Outcome {
	var parts []string
	if len(k.unsupported) > 0 {
		parts = append(parts, strconv.Itoa(len(k.unsupported))+
			" claims quote text that is not in the archive")
	}
	if len(k.unchecked) > 0 {
		parts = append(parts, strconv.Itoa(len(k.unchecked))+
			" claims could not be checked at all")
	}
	return gnosis.Blocked(gnosis.ReasonNeedsHuman, strings.Join(parts, "; "),
		map[string]any{
			"key":         cmd.Key,
			"effect":      cmd.Eff.String(),
			"unsupported": k.unsupported,
			"unchecked":   k.unchecked,
		})
}
