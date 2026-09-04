package relay

import (
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/finding"
)

// criticCategoryPrefix namespaces a verdict's category.
//
// The `gate:` prefix was added on 2026-08-27 after a lint check took a name the promote
// gate's evidence signal already had, and §12.1 records what that cost: a table that
// could not express both. This one is added **before** the collision — a critic verdict
// and a lint check are two mechanisms answering at two moments, and a reader seeing
// `scope` in an envelope should not have to work out which produced it.
//
// **It is the domain's constant rather than this package's own**, because the findings
// gate reads the prefix back to answer §17.1's question of whether a semantic review
// ran. Two packages agreeing on a marker by inspection is one design decision in two
// modules, and the failure would be silent: a gate that stopped recognising critic
// findings would report every corpus as structurally checked only.
const criticCategoryPrefix = gnosis.CriticCategoryPrefix

// criticUnclassified is the category of a finding whose reply named none.
//
// Named rather than blank, on `finding.Action`'s reasoning one field over: an absent
// category in the envelope reads as a finding nobody could route, and a named one
// records that it arrived unclassified rather than that somebody classified it wrongly.
const criticUnclassified = criticCategoryPrefix + "unclassified"

// CriticFinding is one thing a cold critic reported about a claim.
//
// **There is no severity field, and its absence is the mechanism.** §10.5 makes a
// verdict advisory: a critic that could return a blocking severity would be a model
// gating the corpus, which §9.5.1 refuses on the promotion path for the same reason —
// gnosis's escape from a red gate is a person, not a counter. The caller stamps every
// verdict a warning, and no reply can ask for anything else because there is nowhere to
// ask.
type CriticFinding struct {
	// Category is the kind of defect, namespaced. Never empty: a reply naming none
	// gets criticUnclassified.
	Category string `json:"category"`

	// Message is what is wrong, in the critic's own words.
	Message string `json:"message"`
}

// CriticReply is what an agent sent back for one critic prompt.
//
// Parsed is carried for `Reply.Parsed`'s reason: an empty findings list is a legitimate
// answer — "I looked and found nothing" is the answer a corpus most wants to be able to
// record — and a zero CriticReply is not. Without the flag the two are one value, and a
// caller would file a critique nobody performed.
type CriticReply struct {
	Parsed   bool            `json:"parsed"`
	Findings []CriticFinding `json:"findings"`

	// Examined and NotExamined are §10.5's additive block.
	//
	// **Examined is required at the parse**, which is the one strictness worth
	// spending here: a reply with no finding in an area is otherwise
	// indistinguishable from that area not having been looked at, and the gate ships
	// on that silence. NotExamined may be empty — a critic that examined everything
	// it could see is making a claim, and refusing it would teach the critic to
	// invent a gap.
	//
	// **NotExamined is the family's type and Examined is a list of phrases**, and the
	// asymmetry is the point. `finding.Unexamined` carries `{Aspect, Reason}` and its
	// `Valid()` requires both, because *why not* is the half that makes a gap
	// actionable: "the source's methodology" tells a reader nothing, and "the excerpt
	// does not include it" tells them whether a better excerpt would fix it. An
	// examined aspect needs no reason — "examined X because Y" is not something a
	// critic has to say.
	//
	// Using skillet's type rather than mirroring it also means the ledger, this
	// reply, and `gnosis gate`'s output speak one shape. Mirroring it was what shipped
	// first, and the gate reading `finding.Unexamined` through from another tool while
	// gnosis's own critic produced half of it is what showed the gap.
	Examined    []string             `json:"examined"`
	NotExamined []finding.Unexamined `json:"not_examined,omitempty"`
}

// criticReplyDoc mirrors the on-disk shape, so the decoded form and the validated form
// are different types and there is no way to obtain a CriticReply that was not checked.
type criticReplyDoc struct {
	Findings []struct {
		Category string `yaml:"category"`
		Message  string `yaml:"message"`
	} `yaml:"findings"`
	Examined    []string `yaml:"examined"`
	NotExamined []struct {
		Aspect string `yaml:"aspect"`
		Reason string `yaml:"reason"`
	} `yaml:"not_examined"`
}

// ParseCriticReply reads an agent's verdict on one claim.
//
// Requires: src is the agent's whole response, which may carry prose around the fenced
// block.
// Ensures: EINVALID naming every defect at once, or a reply whose findings all carry a
// message and a namespaced category. **Rejected whole or accepted whole**, for
// ParseReply's reason: a half-applied verdict would record a critique nobody gave.
//
// Pure — no clock, no I/O. The caller stamps the severity and the time.
func ParseCriticReply(src []byte) (CriticReply, error) {
	const op = "relay.ParseCriticReply"

	block, err := oneBlock(op, string(src))
	if err != nil {
		return CriticReply{}, err
	}
	var doc criticReplyDoc
	if uErr := yaml.Unmarshal([]byte(block), &doc); uErr != nil {
		return CriticReply{}, &errs.Error{Code: errs.EINVALID, Op: op, Err: uErr}
	}
	return validateCriticReply(op, &doc)
}

// validateCriticReply turns a decoded document into a CriticReply, or reports why it
// cannot.
//
// Every problem at once, so an agent fixing one is not told about the next on the
// following round trip — which costs a model call to learn.
func validateCriticReply(op string, doc *criticReplyDoc) (CriticReply, error) {
	var bad []string
	if len(trimmed(doc.Examined)) == 0 {
		bad = append(bad, "`examined` is empty; a reply that says nothing about what "+
			"it looked at cannot be told from one that looked at nothing, which is "+
			"the silence this block exists to break")
	}
	out := CriticReply{
		Parsed:   true,
		Findings: make([]CriticFinding, 0, len(doc.Findings)),
		Examined: trimmed(doc.Examined),
	}
	for i := range doc.NotExamined {
		gap := finding.Unexamined{
			Aspect: strings.TrimSpace(doc.NotExamined[i].Aspect),
			Reason: strings.TrimSpace(doc.NotExamined[i].Reason),
		}
		if !gap.Valid() {
			// Refused rather than half-recorded, and skillet's own comment on the
			// type gives the reason: "an unparseable answer must not advance
			// anything on trust, and silently discarding half of one is how a reply
			// that says nothing passes for a reply that found nothing."
			bad = append(bad, "the unexamined entry "+ordinal(i)+
				" names no aspect or no reason, and the reason is the half that says"+
				" whether a better excerpt would close the gap")
			continue
		}
		out.NotExamined = append(out.NotExamined, gap)
	}
	for i := range doc.Findings {
		f := &doc.Findings[i]
		if strings.TrimSpace(f.Message) == "" {
			bad = append(bad, "finding "+ordinal(i)+" carries no message")
			continue
		}
		out.Findings = append(out.Findings, CriticFinding{
			Category: criticCategory(f.Category),
			Message:  strings.TrimSpace(f.Message),
		})
	}
	if len(bad) > 0 {
		return CriticReply{}, &errs.Error{
			Code: errs.EINVALID, Message: op + ": " + strings.Join(bad, "; "),
		}
	}
	return out, nil
}

// criticCategory namespaces a reply's category, defaulting the empty one.
//
// **An unfamiliar category is filed as it is rather than refused**, and the direction is
// deliberate: §10.3 lists the reasoning classes the corpus cares about and does not
// close the set, so a parser that rejected an unlisted one would silence a real finding
// over a label. A category is a routing hint, and the message is the finding.
func criticCategory(raw string) string {
	slug := strings.ToLower(strings.Join(strings.Fields(raw), "-"))
	slug = strings.TrimPrefix(slug, criticCategoryPrefix)
	if slug == "" {
		return criticUnclassified
	}
	return criticCategoryPrefix + slug
}

// trimmed drops the blank entries from a list and returns nil when nothing is left.
//
// A reply may carry an empty bullet where a model started a line and thought better of
// it. Keeping one would put an empty angle into the coverage ledger, where it would
// steer the next critic toward nothing at all.
func trimmed(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
