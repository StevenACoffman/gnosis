package relay

import (
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/StevenACoffman/skillet/errs"
)

// answerReplyDoc is the wire shape of an answer, decoded before it is judged.
type answerReplyDoc struct {
	Title      string   `yaml:"title"`
	Answer     string   `yaml:"answer"`
	Cites      []string `yaml:"cites"`
	Unanswered string   `yaml:"unanswered"`
}

// AnswerReply is what an agent sent back for one answer prompt (§8.3).
//
// Parsed is carried for `Reply.Parsed`'s reason: a reply may legitimately decline — the
// claims did not answer the question — and a zero AnswerReply is not that. Without the
// flag a caller cannot tell a considered "the corpus does not say" from a value nobody
// filled in, and only the first is worth filing.
type AnswerReply struct {
	Parsed bool `json:"parsed"`

	// Title names the concept this answer would become. Required whenever there is an
	// answer, and empty for a declination — which is not a concept and needs no name.
	//
	// **Required here rather than supplied by a flag at filing time.** The answerer
	// knows what the answer is about and the person running `gnosis file` may be a
	// script; a title invented at the filing step would name the question rather than
	// the finding, and §5.1.1 puts the title in the path where every later reader sees
	// it.
	Title string `json:"title,omitempty"`

	// Answer is the prose. Empty when the reply declined, which is an ordinary
	// outcome and the one §17.0.1 most wants recordable.
	Answer string `json:"answer"`

	// Cites are the claim references the answer rests on, in the order given.
	//
	// **Required whenever there is an answer**, and that is this parser's one
	// strictness. An answer citing nothing is a model's recollection wearing the
	// corpus's authority — indistinguishable, to every later reader, from a claim
	// somebody sourced. There is no way to check it afterwards, which is why it is
	// checked here.
	Cites []string `json:"cites"`

	// Unanswered is what the question asked that the claims did not cover, or empty.
	//
	// The same half `finding.Unexamined` carries one relay over: an answer that is
	// silent about its own gaps reads as complete, and a reader cannot tell a full
	// answer from a partial one. It is not required — a reply covering everything
	// asked has nothing to put here, and demanding a gap teaches a model to invent
	// one.
	Unanswered string `json:"unanswered,omitempty"`
}

// Answered reports whether the reply carries an answer rather than a declination.
//
// A predicate rather than a comparison at each call site, so "what counts as an answer"
// is one decision. `gnosis file` and the envelope both ask it.
func (r *AnswerReply) Answered() bool { return strings.TrimSpace(r.Answer) != "" }

// ParseAnswer reads an agent's answer to one question.
//
// Requires: src is the agent's whole response, which may carry prose around the fenced
// block; allowed is the set of claim references the prompt offered, and may be empty
// only when the caller genuinely offered none.
// Ensures: EINVALID naming every defect at once, or a reply whose citations are all
// references the prompt actually carried. **Rejected whole or accepted whole**, for
// ParseReply's reason: a half-applied answer would file prose whose support nobody
// checked.
//
// Pure — no clock, no I/O.
//
// # Why the allowed set is a parameter
//
// A citation that names a claim the prompt never showed is the failure this whole relay
// is arranged to prevent, and it cannot be detected from the reply alone. Passing the
// set in keeps the check here, where the reply is refused whole, rather than in a caller
// that has already begun writing — and keeps this function pure, which a lookup against
// the corpus would not.
func ParseAnswer(src []byte, allowed []string) (AnswerReply, error) {
	const op = "relay.ParseAnswer"

	block, err := oneBlock(op, string(src))
	if err != nil {
		return AnswerReply{}, err
	}
	var doc answerReplyDoc
	if uErr := yaml.Unmarshal([]byte(block), &doc); uErr != nil {
		return AnswerReply{}, &errs.Error{Code: errs.EINVALID, Op: op, Err: uErr}
	}
	return validateAnswer(op, &doc, allowed)
}

// validateAnswer turns a decoded document into an AnswerReply, or reports why it cannot.
//
// Every problem at once, so an agent fixing one is not told about the next on the
// following round trip — which costs a model call to learn.
func validateAnswer(op string, doc *answerReplyDoc, allowed []string) (AnswerReply, error) {
	cites, bad := checkCites(doc.Cites, allowed)
	out := AnswerReply{
		Parsed:     true,
		Title:      strings.TrimSpace(doc.Title),
		Answer:     strings.TrimSpace(doc.Answer),
		Unanswered: strings.TrimSpace(doc.Unanswered),
		Cites:      cites,
	}
	if out.Answered() && out.Title == "" {
		bad = append(bad, "the answer names no title, so the concept it would be"+
			" filed as has no name and no path")
	}
	if out.Answered() && len(out.Cites) == 0 {
		bad = append(bad, "the answer cites no claim, so nothing in the corpus"+
			" supports it and no later reader could check it")
	}
	if !out.Answered() && out.Unanswered == "" {
		bad = append(bad, "the reply neither answers nor says what it could not"+
			" answer, which is indistinguishable from an empty reply")
	}
	if len(bad) > 0 {
		return AnswerReply{}, &errs.Error{
			Code: errs.EINVALID, Message: op + ": " + strings.Join(bad, "; "),
		}
	}
	return out, nil
}

// checkCites keeps the citations the prompt actually carried and reports the rest.
//
// Separated from the shape checks above because they are two questions — *is this
// citable* and *is this a reply at all* — and a complexity linter counting the branches
// is what pointed at their being in one function. The split also puts the loop where a
// reader looking for "how is a fabricated citation caught" will look for it.
func checkCites(raw, allowed []string) (cites, problems []string) {
	offered := make(map[string]bool, len(allowed))
	for _, ref := range allowed {
		offered[strings.TrimSpace(ref)] = true
	}
	cites = make([]string, 0, len(raw))
	for i, entry := range raw {
		ref := strings.TrimSpace(entry)
		switch {
		case ref == "":
			problems = append(problems, "citation "+ordinal(i)+" is blank")
		case !offered[ref]:
			// Named in full, because the recovery is to copy a heading and the
			// caller cannot copy what they were not shown.
			problems = append(problems, "citation "+ordinal(i)+" names "+ref+
				", which this prompt did not carry; cite a heading from the prompt")
		default:
			cites = append(cites, ref)
		}
	}
	return cites, problems
}
