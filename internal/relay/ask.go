package relay

import "strings"

// AskClaim is one retrieved claim as an answering model may see it.
//
// **It carries the claim's identifier**, unlike `CriticClaim`, and the two omissions are
// opposite by design. A critic must not know how a claim was decided, because a judge
// shown the conclusion finds support for it. An answerer must be able to *cite*, because
// an answer that names no claim is a model's recollection wearing the corpus's authority
// — the most expensive output §17.0.1 describes.
//
// The identifier is the claim reference (§5.4: identifiers, never paths), so a citation
// survives the document being retitled.
type AskClaim struct {
	// Ref addresses the claim, as gnosis.ClaimRef writes it.
	Ref string

	// Text is what the claim asserts, and Lead its author's summary (§17.4), or empty.
	Text string
	Lead string

	// Quotes are the passages the claim rests on. A claim with none does not reach
	// this type: `FoldAnswerability` refuses the question first, because an answer
	// assembled from unevidenced claims is the confident answer from nothing.
	Quotes []string

	// Title and Path locate the claim for a human reading the prompt. They are prose
	// rather than address: a citation is by Ref.
	Title string
	Path  string
}

// AskRequest is everything needed to render one answer prompt (§8.3).
//
// There is no `SourceHash` field, and its absence is the difference from the other two
// relays. An extraction prompt is about one archived text and a critique is about one
// claim against one source; an answer is assembled from *several* claims, so what plays
// the source hash's part in the cache key is a hash over the retrieved set. A question
// whose retrieval changed is a different question, and §6.1's key must say so.
type AskRequest struct {
	// Question is the caller's, verbatim.
	Question string

	// Claims are what retrieval found, in the order it found them. The order is part
	// of the prompt and therefore part of the key, so a caller handing them over in a
	// map's iteration order would make the cache useless.
	Claims []AskClaim

	// Model is what will answer.
	Model Model
}

// RenderAsk builds the answer prompt for one question (§8.3).
//
// Requires: r.Claims is what retrieval returned and every claim carries at least one
// quotation — `gnosis.FoldAnswerability` is what establishes that, and this does not
// re-check it.
// Ensures: pure and deterministic — no clock, no randomness, no map iteration, so two
// runs over one question and one corpus state produce byte-identical prompts. That is
// the precondition for the key meaning anything, and a test pins it.
//
// Everything the corpus supplies is fenced, and so is the question. The question is the
// **only** input here that did not come from the corpus, which makes it the most
// obviously untrusted and the easiest to forget: a question carrying a newline and a
// plausible heading could otherwise append instructions to this prompt.
func RenderAsk(r *AskRequest) Prompt {
	body := renderAsk(r)
	return Prompt{
		Key:  Key(HashText(claimSpan(r.Claims)), HashText(body), r.Model.Name, r.Model.Version),
		Text: body,
	}
}

// claimSpan is the retrieved set, as a string that changes when the set does.
//
// It stands in for the source hash the other two relays key on. Two questions retrieving
// different claims are different questions even when the words match, and a key that
// could not tell them apart would serve an answer assembled from claims the second
// question never saw.
func claimSpan(claims []AskClaim) string {
	refs := make([]string, 0, len(claims))
	for _, claim := range claims {
		refs = append(refs, claim.Ref)
	}
	return strings.Join(refs, sep)
}

// renderAsk writes the prompt.
//
// The order is instructions, then reply format, then the untrusted material — the order
// both other prompts use, so a reader comparing them is not learning a third layout.
func renderAsk(r *AskRequest) string {
	var b strings.Builder

	b.WriteString("# Answer one question from this corpus, and only from it\n\n")
	b.WriteString("## Rules\n\n")
	b.WriteString(askRules())
	b.WriteString("\n## Reply format\n\n")
	b.WriteString(askReplyFormat())
	b.WriteString("\n## The question\n\n")
	b.WriteString(fence)
	b.WriteString("\n")
	b.WriteString(oneLine(r.Question))
	b.WriteString("\n")
	b.WriteString(fence)
	b.WriteString("\n\n## The claims retrieved for it\n\n")
	for i := range r.Claims {
		writeAskClaim(&b, &r.Claims[i])
	}
	return b.String()
}

// writeAskClaim renders one claim inside its own fence.
//
// One fence per claim rather than one around all of them: a claim whose text ended a
// shared block would put every claim after it outside the fence, and the reader has no
// way to notice. The identifier is outside the fence, where a citation instruction can
// point at it without the model having to parse the block to find one.
func writeAskClaim(b *strings.Builder, claim *AskClaim) {
	b.WriteString("### ")
	b.WriteString(claim.Ref)
	b.WriteString("\n\n")
	if claim.Title != "" {
		b.WriteString("From ")
		b.WriteString(oneLine(claim.Title))
		b.WriteString("\n\n")
	}
	b.WriteString(fence)
	b.WriteString("\nClaim: ")
	b.WriteString(oneLine(claim.Text))
	b.WriteString("\n")
	if claim.Lead != "" {
		b.WriteString("Its author's summary: ")
		b.WriteString(oneLine(claim.Lead))
		b.WriteString("\n")
	}
	b.WriteString("Passages it rests on:\n")
	if len(claim.Quotes) == 0 {
		// A claim reaching here with no quotation is the caller's defect — the
		// population excludes them — but saying so in the prompt is better than a
		// heading with nothing under it, which reads as a formatting fault and invites
		// the model to supply the missing part. `writeClaim` carries the same line one
		// relay over, and this one was written after a hand run showed the gap.
		b.WriteString("(none — do not answer from this claim)\n")
	}
	for _, q := range claim.Quotes {
		b.WriteString("- ")
		b.WriteString(oneLine(q))
		b.WriteString("\n")
	}
	b.WriteString(fence)
	b.WriteString("\n\n")
}
