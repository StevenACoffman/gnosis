package relay

import "strings"

// Model identifies what will answer a prompt.
//
// It is carried on the request rather than configured globally because it is part
// of the cache key (§6.1), and a value that can change between two lookups of the
// same key is not a key component — it is a race.
type Model struct {
	Name    string
	Version string
}

// Request is everything needed to render one extraction prompt.
//
// The archived text rather than the URI: §4.1's whole argument is that a quote
// validated against a live source is a proof that expires. A prompt built from the
// live page would ask a model about text nobody kept, and every quotation it
// returned would be unverifiable by construction.
type Request struct {
	// URI is the source, for the prompt's own prose and for the reply to echo.
	URI string

	// SourceHash is the archived text's content hash, and the first component of
	// the cache key.
	SourceHash string

	// Text is the archived text itself.
	Text string

	// Model is what will answer.
	Model Model
}

// Prompt is one rendered extraction prompt and the key its reply is cached under.
type Prompt struct {
	// Key is the §6.1 cache key.
	Key string

	// URI is the source this asks about.
	URI string

	// Text is the rendered prompt, exactly as it should be handed to a model.
	Text string
}

// Render builds the extraction prompt for one source.
//
// Requires: r.Text is the archived text; r.SourceHash is its content hash.
// Ensures: pure and deterministic. No clock, no randomness, no map iteration —
// two runs over one source produce byte-identical prompts, which is the
// precondition for the key meaning anything. A test pins this rather than trusting
// it, because a single ranged map would break it silently.
func Render(r *Request) Prompt {
	body := renderExtraction(r)
	return Prompt{
		Key:  Key(r.SourceHash, HashText(body), r.Model.Name, r.Model.Version),
		URI:  r.URI,
		Text: body,
	}
}

// renderExtraction writes the prompt.
//
// It asks for verbatim quotations and says why, because a model told only "cite
// your source" produces plausible paraphrase — and a paraphrase cannot be
// validated against the archived text, which means the claim it supports enters
// the corpus with no offline proof. The requirement is stated as a constraint on
// the output rather than as an instruction about care.
func renderExtraction(r *Request) string {
	var b strings.Builder

	b.WriteString("# Extract claims from one source\n\n")
	b.WriteString("Source: ")
	b.WriteString(r.URI)
	b.WriteString("\nArchived text hash: ")
	b.WriteString(r.SourceHash)
	b.WriteString("\n\n## Rules\n\n")
	b.WriteString(rules())
	b.WriteString("\n## Reply format\n\n")
	b.WriteString(replyFormat())
	b.WriteString("\n## Source text\n\n")
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
