package gnosis

// The four states a corpus can be in with respect to a question (SPEC §17.0.1).
//
// AnswerabilitySilent is the zero value, and that is the safety property: a fold nobody ran,
// a question nobody retrieved for, and a corpus that genuinely holds nothing all read as
// silent. The opposite default would have an unpopulated value claim the corpus can
// answer, which is the confident-answer-from-nothing failure this type exists to name.
const (
	AnswerabilitySilent Answerability = iota
	AnswerabilityUnevidenced
	AnswerabilityUnresolved
	AnswerabilityReady
)

// Answerability is what the corpus can offer a question, folded from the claims
// retrieved for it.
//
// **Not `Support`, which this package already uses** for §14.4's "what one archived
// source buys a claim that cites it". Two unrelated things under one name is what a
// reader disambiguates at every call site — the rule `Critique` records and the second
// time it has been caught here, this time by the compiler rather than by a reader.
//
// # Three refusals, not one
//
// §17.0.1 requires `ask` to be able to say "the corpus does not support an answer to
// this", and names what a refusal must distinguish: no claim on the subject, claims found
// but none with evidence, or claims that contradict each other with no adjudication. A
// boolean would collapse all three into "no", and the distinction is the entire value —
// **only the third is a `conflict` waiting to be filed**, and only the second is fixed by
// ingesting a source rather than by writing a concept.
//
// The read path has no other gate. A write is admitted or refused, a finding blocks or
// does not; a retrieval that cannot refuse produces the same shape of output for a
// question the corpus cannot answer as for one it can, and the caller cannot tell which
// they got.
type Answerability int

// Retrieved is one claim as the answerability fold needs to see it.
//
// It is deliberately narrower than anything the corpus holds. The fold asks three
// questions and a wider type would invite a fourth — whether the claim is *good* — which
// is the critic's question (§10.5) and not a retrieval gate's.
type Retrieved struct {
	// Evidenced says whether the claim offers any passage as support.
	Evidenced bool

	// Contested says whether the claim is under an open contradiction or challenge
	// that nobody has adjudicated.
	Contested bool
}

// String is the token the envelope carries.
func (s Answerability) String() string {
	switch s {
	case AnswerabilityReady:
		return "ready"
	case AnswerabilityUnresolved:
		return "unresolved"
	case AnswerabilityUnevidenced:
		return "unevidenced"
	case AnswerabilitySilent:
		return "silent"
	default:
		// An unrecognised state reports the weakest thing it could be, for Tier's
		// reason: a value that fell through to a name asserting the corpus can answer
		// is the one failure direction this type cannot afford.
		return "silent"
	}
}

// MarshalText renders the state as a word rather than as an integer whose meaning
// depends on the order of the constants above. `Tier`, `Freshness` and `DriftState`
// carry this method for the same reason.
func (s Answerability) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// Answerable reports whether a prompt should be emitted at all.
//
// A predicate rather than a comparison at each call site, because "which states are
// answerable" is one decision and spreading it across callers is how two of them come to
// disagree — the reading `IsHumanActor` was extracted for.
func (s Answerability) Answerable() bool { return s == AnswerabilityReady }

// FoldAnswerability derives what the corpus can offer from the claims retrieved for a question.
//
// Requires: nothing. A nil or empty slice is the ordinary state for a question about
// something nobody has written down.
// Ensures: AnswerabilitySilent for no claims; AnswerabilityUnresolved when any retrieved claim is
// contested; AnswerabilityUnevidenced when none is contested and none carries evidence;
// AnswerabilityReady otherwise. Pure, total, and it never returns an error.
//
// # Contested outranks ready, and that is the fold's one real decision
//
// A question retrieving one adjudicated claim and one contested one could be answered
// from the first. Answering it would be picking a side of an open contradiction and
// handing the result out under the corpus's authority — the most expensive output §17.0.1
// names, and the one a reader has no way to detect, because the answer looks exactly like
// an answer from a corpus that agrees with itself.
//
// The cost is admitted: a corpus with one unresolved claim about a subject refuses
// questions it could partly answer. That is the direction to fail in. The remedy is
// adjudication, it is a thing a person can do, and the refusal names it — whereas the
// remedy for a confident wrong answer is finding out, which nothing here can arrange.
func FoldAnswerability(claims []Retrieved) Answerability {
	if len(claims) == 0 {
		return AnswerabilitySilent
	}
	evidenced := false
	for _, claim := range claims {
		if claim.Contested {
			return AnswerabilityUnresolved
		}
		evidenced = evidenced || claim.Evidenced
	}
	if !evidenced {
		return AnswerabilityUnevidenced
	}
	return AnswerabilityReady
}

// Remedy is what a person can do about a refusal.
//
// Requires: nothing.
// Ensures: a sentence for every state, including the answerable one, so a caller
// rendering this cannot produce an empty line. Pure.
//
// **The remedy travels with the state for `finding.Unexamined`'s reason**: a refusal
// naming a state tells a reader what happened and not what to do, and the two are
// different halves. Each of the three refusals has a different remedy, which is the
// clearest argument that they had to be three states.
func Remedy(s Answerability) string {
	switch s {
	case AnswerabilityReady:
		return "the corpus can support an answer"
	case AnswerabilityUnresolved:
		return "a retrieved claim is contested and nobody has adjudicated it;" +
			" `gnosis adjudicate` is what closes that"
	case AnswerabilityUnevidenced:
		return "claims were found and none carries evidence; fetching and ingesting" +
			" a source is what gives them some"
	case AnswerabilitySilent:
		return "the corpus holds no claim on this subject; nothing here is broken and" +
			" nothing was retrieved"
	default:
		return "the corpus holds no claim on this subject; nothing here is broken and" +
			" nothing was retrieved"
	}
}
