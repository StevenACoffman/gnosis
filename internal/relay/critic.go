package relay

import (
	"strings"

	"github.com/StevenACoffman/skillet/finding"
)

// CriticClaim is everything a cold critic may see about the claim it is judging.
//
// # The three fields are the blinding, and the blinding is a requirement
//
// SPEC §10.3: the prompt MUST NOT include the existing adjudication, warrant, status,
// trust tier, or verification history of the claim. The reason is expectancy bias — a
// judge shown the conclusion a corpus already reached tends to find support for it, and
// its agreement then carries no information beyond the fact that it was told.
//
// So the rule is enforced by what this type **cannot express**. There is no warrant
// field, no status, no tier, no verification list; `relay` imports nothing of gnosis's
// domain, so it could not hold a `gnosis.Warrant` even if somebody tried. A caller
// assembling one of these has no field to put the forbidden thing in, which is a
// stronger guarantee than a comment asking them not to — and this specification has
// already recorded, twice, what a guarantee that lives in a comment is worth.
//
// The cost of getting it wrong is specific and slow: an adjudicated claim would
// accumulate critic agreements that merely echo the original decision, and §10.6.5's
// reversal record would be the only surviving signal that the decision had ever been
// questioned.
type CriticClaim struct {
	// Text is the assertion, as the document states it.
	Text string

	// Lead is the claim's conclusion in its author's words (§17.4), or empty.
	Lead string

	// Quotes are the passages offered as evidence for it.
	Quotes []string
}

// CriticRequest is everything needed to render one cold-critic prompt.
//
// The archived text rather than the URI, for `Request`'s reason: a critic judging
// against a live page would be judging text nobody kept, and its verdict would be
// unreproducible by construction.
type CriticRequest struct {
	// URI is the source, for the prompt's own prose.
	URI string

	// SourceHash is the archived text's content hash, and the first component of the
	// cache key.
	SourceHash string

	// Text is the archived text itself.
	Text string

	// Claim is what is being judged, blinded.
	Claim CriticClaim

	// Examined and NotExamined are what earlier critiques of this claim reported
	// looking at and declining to look at (§10.5).
	//
	// **Feeding these does not compromise cold-context independence**, and the reason
	// is the whole justification for persisting them: a coverage record says what was
	// *looked at*, never what was *concluded* or how the claim was produced. It biases
	// a fresh critic toward unexamined ground, which is the opposite of contamination.
	//
	// The reason travels with an unexamined aspect because it is what tells the next
	// critic whether the gap is closeable at all: "the excerpt does not include it"
	// says the ground is unreachable from this prompt, and steering toward it would
	// send a second critic at a wall the first already described.
	Examined    []string
	NotExamined []finding.Unexamined

	// Model is what will answer.
	Model Model
}

// RenderCritic builds the cold-critic prompt for one claim (§10.5).
//
// Requires: r.Text is the archived text the claim's quotations came from; r.SourceHash
// is its content hash.
// Ensures: pure and deterministic — no clock, no randomness, no map iteration, so two
// runs over one claim and one coverage state produce byte-identical prompts. That is
// the precondition for the key meaning anything, and a test pins it.
//
// **A second critique of the same claim is a different question, and gets a different
// key.** The coverage block is part of the prompt, so once a verdict lands the next
// prompt asks about what nobody has looked at yet — and serving the first critique's
// cached answer to it would be the substitution §6.1's key exists to prevent.
//
// Everything the corpus supplies is fenced, not only the source. The claim text and its
// quotations reached the corpus through a model once already, and a claim carrying a
// newline and a plausible heading could otherwise append instructions to this prompt.
func RenderCritic(r *CriticRequest) Prompt {
	body := renderCritique(r)
	return Prompt{
		Key:  Key(r.SourceHash, HashText(body), r.Model.Name, r.Model.Version),
		URI:  r.URI,
		Text: body,
	}
}

// renderCritique writes the prompt.
//
// The order is instructions, then reply format, then the untrusted material — the same
// order the extraction prompt uses, so a reader comparing the two is not learning a
// second layout.
func renderCritique(r *CriticRequest) string {
	var b strings.Builder

	b.WriteString("# Judge one claim against its source\n\n")
	b.WriteString("Source: ")
	b.WriteString(r.URI)
	b.WriteString("\nArchived text hash: ")
	b.WriteString(r.SourceHash)
	b.WriteString("\n\n## Rules\n\n")
	b.WriteString(criticRules())
	b.WriteString("\n## Reply format\n\n")
	b.WriteString(criticReplyFormat())
	writeCoverage(&b, r)
	b.WriteString("\n## The claim\n\n")
	b.WriteString(fence)
	b.WriteString("\n")
	writeClaim(&b, &r.Claim)
	b.WriteString(fence)
	b.WriteString("\n\n## Source text\n\n")
	b.WriteString(fence)
	b.WriteString("\n")
	b.WriteString(r.Text)
	if !strings.HasSuffix(r.Text, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(fence)
	b.WriteString("\n")
	return b.String()
}

// writeCoverage adds what earlier critiques looked at, and says nothing when there were
// none.
//
// **Silent on a first critique rather than printing two empty headings.** A prompt
// carrying "Already examined: (none)" spends the reader's attention saying that nothing
// happened, and every token in a prompt is one the model weighs.
func writeCoverage(b *strings.Builder, r *CriticRequest) {
	if len(r.Examined) == 0 && len(r.NotExamined) == 0 {
		return
	}
	b.WriteString("\n## What earlier critiques covered\n\n")
	b.WriteString("These say what was *looked at*, never what was concluded. Prefer" +
		" ground nobody has covered.\n\n")
	writeList(b, "Already examined", r.Examined)
	writeGaps(b, "Declared not examined", r.NotExamined)
}

// writeGaps renders the unexamined aspects with the reasons that make them legible.
//
// Both halves, because the reason is what a later critic acts on: an aspect nobody could
// reach from this excerpt is not ground to steer toward, and an aspect skipped for want
// of time is.
func writeGaps(b *strings.Builder, label string, gaps []finding.Unexamined) {
	if len(gaps) == 0 {
		return
	}
	b.WriteString(label)
	b.WriteString(":\n\n")
	for _, gap := range gaps {
		b.WriteString("- ")
		b.WriteString(oneLine(gap.Aspect))
		b.WriteString(" — ")
		b.WriteString(oneLine(gap.Reason))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// writeList renders one labelled bullet list, or nothing when it is empty.
func writeList(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString(label)
	b.WriteString(":\n\n")
	for _, item := range items {
		b.WriteString("- ")
		b.WriteString(oneLine(item))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// writeClaim renders the claim inside the fence.
//
// The quotations are labelled as *offered* rather than as evidence, because whether they
// are evidence is the question being asked. A prompt that called them evidence would
// have answered half of it.
func writeClaim(b *strings.Builder, claim *CriticClaim) {
	b.WriteString("Claim: ")
	b.WriteString(oneLine(claim.Text))
	b.WriteString("\n")
	if claim.Lead != "" {
		b.WriteString("Its author's summary: ")
		b.WriteString(oneLine(claim.Lead))
		b.WriteString("\n")
	}
	b.WriteString("Quotations offered as support:\n")
	if len(claim.Quotes) == 0 {
		// A claim reaching here with no quotation is the caller's defect — the
		// population excludes them — but saying so in the prompt is better than a
		// heading with nothing under it, which reads as a formatting fault and
		// invites the model to invent the missing part.
		b.WriteString("(none offered — say so rather than supplying any)\n")
		return
	}
	for _, q := range claim.Quotes {
		b.WriteString("- ")
		b.WriteString(oneLine(q))
		b.WriteString("\n")
	}
}

// oneLine collapses whitespace so a value cannot break the line it sits on.
//
// It is the second half of fencing: the fence stops the untrusted block ending early,
// and this stops a single field carrying a newline and a plausible heading into the
// middle of one.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
