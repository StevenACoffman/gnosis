package relay

import "strings"

// PairRequest is everything needed to render one two-claim critic prompt (§10.3).
//
// # The blinding is the same construction and for a sharper reason
//
// §10.3 routes "semantic contradiction between claims that survive the decidable
// predicates" to a cold critic that "sees the two claims and their sources but never the
// reasoning that produced either". With two claims the expectancy bias has a second way
// in: a judge shown that one of them was adjudicated learns which side the corpus already
// took, and its agreement then carries no information beyond the fact that it was told.
//
// `CriticClaim` has no warrant, status or tier field and `relay` imports nothing of
// gnosis's domain, so neither claim can carry one. §10.3 names where the remaining risk
// is — "what a type cannot prevent is the projection folding one into a field that does
// exist" — and that is where the test is.
type PairRequest struct {
	// Claims are the two under comparison. An array rather than a slice: this prompt
	// is about exactly two, and a slice would let a caller pass one or five into a
	// template that reads like it compares a pair.
	Claims [2]CriticClaim

	// Sources are the archived texts the two rest on, in the same order, and URIs
	// names where each came from. A claim whose source is unarchived does not reach
	// here: judging against text nobody kept is what §4.1 refuses.
	Sources [2]string
	URIs    [2]string

	// Subject is what the corpus says both claims are about, which is why they are a
	// pair at all.
	Subject string

	// Model is what will answer.
	Model Model
}

// RenderPairCritic builds the two-claim prompt §10.3 routes the residue to.
//
// Requires: both claims carry their text and quotations; both sources are the archived
// text those quotations came from.
// Ensures: pure and deterministic — no clock, no randomness, no map iteration, so two
// runs over one pair produce byte-identical prompts. That is the precondition for the
// key meaning anything, and a test pins it.
//
// The key's first component is a hash over both source hashes, because this prompt is
// about two texts and either changing makes it a different question.
func RenderPairCritic(r *PairRequest) Prompt {
	body := renderPair(r)
	return Prompt{
		Key: Key(HashText(r.URIs[0]+sep+r.URIs[1]), HashText(body),
			r.Model.Name, r.Model.Version),
		URI:  r.URIs[0],
		Text: body,
	}
}

// renderPair writes the prompt.
//
// The order is instructions, then reply format, then the untrusted material — the order
// the other three prompts use, so a reader comparing them is not learning a fourth
// layout.
func renderPair(r *PairRequest) string {
	var b strings.Builder

	b.WriteString("# Judge whether two claims can both hold\n\n")
	b.WriteString("Both are said to be about: ")
	b.WriteString(oneLine(r.Subject))
	b.WriteString("\n\n## Rules\n\n")
	b.WriteString(pairRules())
	b.WriteString("\n## Reply format\n\n")
	b.WriteString(criticReplyFormat())

	for i := range r.Claims {
		b.WriteString("\n## Claim ")
		b.WriteString(claimLabel(i))
		b.WriteString("\n\n")
		b.WriteString(fence)
		b.WriteString("\n")
		writeClaim(&b, &r.Claims[i])
		b.WriteString(fence)
		b.WriteString("\n\n### Its source\n\n")
		b.WriteString("Source: ")
		b.WriteString(r.URIs[i])
		b.WriteString("\n\n")
		b.WriteString(fence)
		b.WriteString("\n")
		b.WriteString(r.Sources[i])
		if !strings.HasSuffix(r.Sources[i], "\n") {
			b.WriteString("\n")
		}
		b.WriteString(fence)
		b.WriteString("\n")
	}
	return b.String()
}

// claimLabel names a side without implying an order of merit.
//
// "A" and "B" rather than "first" and "second", because a critic told which claim came
// first has been told something about the corpus — the older claim is the one the
// newer one would be revising — and §10.3's blinding is about exactly that kind of
// leakage.
func claimLabel(i int) string {
	if i == 0 {
		return "A"
	}
	return "B"
}
