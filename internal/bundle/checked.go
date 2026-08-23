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

	// unsupported names claims whose quotations were looked for and not found.
	// This is fabrication or corruption, and it blocks.
	unsupported []string

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
