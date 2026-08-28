package lint

import (
	"strings"

	"github.com/StevenACoffman/skillet/finding"
)

// leadCheck reports a normative claim whose lead restates background instead of the
// conclusion (§17.4).
//
// The rule is BLUF — say the conclusion, then explain it — and §17.4's justification is
// not courtesy: it is an information architecture for decision making. Both of this
// corpus's readers need excerpts. An agent retrieving under a context budget takes the
// first *n* tokens of a ranked result, and if those tokens are background the result is
// true, relevant, and useless with no way to know the answer was three lines down. A
// person scanning a conflict queue is comparing two claims rather than reading two
// documents, and conclusion-first is what makes a queue scannable at all.
//
// **One finding with two lexical shapes**, because "the conclusion is not first" shows up
// two ways and both use `standards/indicators.toml` — which is the reader those
// conclusion-role rows have never had since they shipped:
//
//   - The lead **opens with a reason marker**: "Because latency is high, cap retries."
//     The claim leads with its derivation.
//   - The lead **carries a conclusion marker after its start**: "Latency is high,
//     therefore cap retries." The conclusion is buried behind the reasoning that
//     produced it.
//
// A conclusion marker *at the start* is correct rather than a finding — "Therefore, cap
// retries" is conclusion-first with a connective attached, and reporting it would teach
// authors to strip the connectives that make prose readable.
//
// **Warning, never a gate**, and silent for a claim with no lead: a lead is optional in a
// reply, and §5.8.3's argument one field over settles that reporting is a review signal
// where refusing is a gate.
func leadCheck() Check {
	return Check{
		Name:       "lead",
		Categories: []string{"lead"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someNormativeClaimHasALead,
		Run:        buriedConclusions,
	}
}

// someNormativeClaimHasALead reports whether there is anything to check, and says which
// of the three ways there is not.
//
// Requires: nothing.
// Ensures: a reason whenever it declines, naming the missing thing rather than the check.
// Pure.
func someNormativeClaimHasALead(snap *Snapshot) (bool, string) {
	if len(snap.Indicators.Reason) == 0 && len(snap.Indicators.Conclusion) == 0 {
		return false, "standards/indicators.toml declares no indicator words"
	}
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		declared, ok := snap.Vocabulary.TypeNamed(doc.Type)
		if !ok || !declared.Normative {
			continue
		}
		for _, claim := range doc.Claims {
			if strings.TrimSpace(claim.Lead) != "" {
				return true, ""
			}
		}
	}
	return false, "no claim of a prescribing type carries a lead yet"
}

// buriedConclusions reports each normative claim whose lead does not state its
// conclusion first.
//
// Requires: the indicator words are loaded.
// Ensures: one diagnostic per claim; claims with no lead are silent. Pure.
func buriedConclusions(snap *Snapshot) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		declared, ok := snap.Vocabulary.TypeNamed(doc.Type)
		if !ok || !declared.Normative {
			continue
		}
		for j := range doc.Claims {
			if d := buriedConclusion(snap, doc, &doc.Claims[j]); d != nil {
				out = append(out, *d)
			}
		}
	}
	return out
}

// buriedConclusion reports one claim's lead, or nil when it states its conclusion first.
func buriedConclusion(snap *Snapshot, doc *Document, claim *Claim) *finding.Diagnostic {
	lead := strings.TrimSpace(claim.Lead)
	if lead == "" {
		return nil
	}
	var why string
	switch {
	case opensWith(lead, snap.Indicators.Reason):
		why = "it opens with a reason, so the claim leads with its derivation"
	case carriesAfterStart(lead, snap.Indicators.Conclusion):
		why = "its conclusion sits behind the reasoning that produced it"
	default:
		return nil
	}
	return &finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "lead",
		Path:     doc.Path,
		Message: "claim " + claim.ID + "'s lead restates background rather than the " +
			"conclusion — " + why + ": " + excerpt(lead) +
			". State what follows, then explain it: an agent retrieving under a context" +
			" budget takes the first few words and cannot tell the answer was later",
		Action: finding.ActionHuman,
	}
}

// opensWith reports whether text begins with one of the markers, on a word boundary.
func opensWith(text string, markers []string) bool {
	lower := strings.ToLower(text)
	for _, m := range markers {
		if !strings.HasPrefix(lower, m) {
			continue
		}
		rest := lower[len(m):]
		if rest == "" || !isWordChar(rest[0]) {
			return true
		}
	}
	return false
}

// carriesAfterStart reports whether one of the markers appears somewhere other than the
// opening.
//
// **Not merely "contains".** A conclusion marker at the start is conclusion-first with a
// connective attached, which is correct — reporting it would teach authors to strip the
// connectives that make prose readable, which is the opposite of what §17.4 wants.
func carriesAfterStart(text string, markers []string) bool {
	lower := " " + strings.ToLower(text) + " "
	for _, m := range markers {
		at := strings.Index(lower, " "+m+" ")
		if at > 0 {
			return true
		}
	}
	return false
}

// isWordChar reports whether b continues a word, so a marker must end at a boundary.
func isWordChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	default:
		return b == '_' || b == '-'
	}
}
