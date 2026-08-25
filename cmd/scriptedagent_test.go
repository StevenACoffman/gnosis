package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minQuoteWords mirrors the prompt's own minimum, which mirrors
// quotecheck.MinPassageWords. It is stated here rather than imported because the
// point of this fixture is to behave like an agent that has read only the prompt —
// an agent with access to gnosis's constants would be testing gnosis against
// itself.
const (
	minQuoteWords = 6

	// sourceMarker is the heading the prompt puts above the archived text. The agent
	// finds the fenced source by it, and the damage cases below cut at it, so it is
	// stated once: a mutation that stopped finding the section would stop mutating,
	// and pass.
	sourceMarker = "## Source text"
)

// errNoQuotableSource is what the scripted agent returns when the prompt gave it
// nothing it could quote.
//
// It is the fixture's most important failure. An agent that cannot find quotable
// source text in the prompt is reporting a defect in the *prompt*, not in itself, and
// a fixture that papered over it by falling back to invented text would be a playback
// rather than a test.
var errNoQuotableSource = errors.New(
	"the prompt carries no source text long enough to quote")

// scriptedAgent produces a reply derived only from the prompt it was handed.
//
// Requires: prompt is a rendered extraction prompt, exactly as `gnosis ingest` wrote
// it to disk.
// Ensures: a reply in the format the prompt asks for, quoting text the prompt
// contained — or errNoQuotableSource when the prompt contained none. Pure.
//
// # Why this exists, and what §18.6's "scripted model" means here
//
// §18.6 says the relay test needs a scripted model: "real binary, real prompt,
// reasoning replaced by a local server speaking the model protocol from a script".
// **gnosis speaks no model protocol.** It writes a prompt file and reads a reply file,
// because the relay was designed so that gnosis never calls a model — so the seam is
// *prompt file → agent → reply file*, and the local server is a function.
//
// The translation preserves the property that matters. This agent can see **only the
// prompt**, so it cannot quote source text the prompt failed to carry: if `Render`
// stops fencing the archived text, this returns errNoQuotableSource, `admit` never
// runs, and the fixture fails. That is §18.6's "assert on what the agent *sent*" in
// the shape this architecture has — here the prompt **is** the request, so asserting
// on the request means deriving the reply from it and nothing else.
//
// It deliberately does no reasoning. Picking the first long-enough sentence is not
// extraction and is not meant to be; what is under test is the contract — that the
// prompt carries what a replier needs and that a well-formed reply survives `admit` —
// which is the question a scripted model answers and a real one cannot answer
// reproducibly.
func scriptedAgent(prompt string) (string, error) {
	source, err := fencedSource(prompt)
	if err != nil {
		return "", err
	}
	quote, err := firstQuotableSentence(source)
	if err != nil {
		return "", err
	}
	// The reply format is copied from the prompt's own "Reply format" section rather
	// than from gnosis's renderer, for the same reason the word minimum is: an agent
	// reads the instructions it was given.
	return "```yaml\n" +
		"title: Cache Lifetime\n" +
		"type: Reference\n" +
		"claims:\n" +
		"  - text: " + quote + ".\n" +
		"    quotes:\n" +
		"      - " + quote + "\n" +
		"```\n", nil
}

// fencedSource extracts the archived text the prompt fenced.
//
// Requires: nothing; a prompt with no fence is the case this reports.
// Ensures: the text between the first pair of fences, or errNoQuotableSource. Pure.
//
// The fence is found rather than assumed to be a known string: an agent reading the
// prompt sees a run of backticks and does not know how long gnosis chose to make it.
// It is deliberately long there because the archived text is untrusted and could
// otherwise close its own block — and this fixture is the reader that discovers the
// length rather than being told it.
func fencedSource(prompt string) (string, error) {
	at := strings.Index(prompt, sourceMarker)
	if at < 0 {
		return "", errNoQuotableSource
	}
	rest := prompt[at+len(sourceMarker):]

	open := strings.Index(rest, "``````")
	if open < 0 {
		return "", errNoQuotableSource
	}
	// The fence runs to the end of its line.
	lineEnd := strings.IndexByte(rest[open:], '\n')
	if lineEnd < 0 {
		return "", errNoQuotableSource
	}
	fence := strings.TrimRight(rest[open:open+lineEnd], "\r")
	body := rest[open+lineEnd+1:]

	end := strings.Index(body, fence)
	if end < 0 {
		return "", errNoQuotableSource
	}
	return body[:end], nil
}

// firstQuotableSentence is the first sentence of at least minQuoteWords words.
//
// Requires: nothing.
// Ensures: a sentence with no trailing punctuation, or errNoQuotableSource when the
// source has none long enough. Pure.
//
// Splitting on ". " is exactly the naive splitter `internal/segment` exists to
// replace, and that is fine here: this is a fixture standing in for a replier, not a
// segmenter. Using gnosis's own segmenter would make the test agree with gnosis by
// construction, which is the failure mode a scripted agent is supposed to avoid.
func firstQuotableSentence(source string) (string, error) {
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		for _, sentence := range strings.Split(line, ". ") {
			sentence = strings.TrimRight(strings.TrimSpace(sentence), ".")
			if len(strings.Fields(sentence)) >= minQuoteWords {
				return sentence, nil
			}
		}
	}
	return "", errNoQuotableSource
}

// scriptedReply runs the scripted agent over an emitted prompt and writes its reply
// to a file, which is what `admit` consumes.
func scriptedReply(t *testing.T, bundleDir, promptPath string) string {
	t.Helper()

	prompt, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(promptPath)))
	if err != nil {
		t.Fatalf("read the emitted prompt: %v", err)
	}
	reply, err := scriptedAgent(string(prompt))
	if err != nil {
		t.Fatalf("the scripted agent could not answer the real prompt: %v", err)
	}
	file := filepath.Join(t.TempDir(), "reply.md")
	if wErr := os.WriteFile(file, []byte(reply), 0o600); wErr != nil {
		t.Fatalf("write reply: %v", wErr)
	}
	return file
}

// TestAScriptedAgentsReplyIsAdmitted is the seam §18.6 says nothing tests.
//
// Every other relay test hand-writes its reply, which the relay's design made easy
// and which proves nothing about the prompt: a test that authors both halves of a
// contract has not checked the contract. Here the reply is derived from the emitted
// prompt and nothing else, so the assertion is that **the prompt carries what a
// replier needs**.
func TestAScriptedAgentsReplyIsAdmitted(t *testing.T) {
	t.Parallel()
	bundleDir, uri := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	key, promptPath := firstPrompt(t, ingest(t, bundleDir, uri))

	reply := scriptedReply(t, bundleDir, promptPath)
	_, stderr, err := run(t, "--bundle", bundleDir, "--jsonl",
		"admit", "--key", key, "--submitter", "agent:scripted", reply)
	if err != nil {
		t.Fatalf("admit refused a reply derived from its own prompt: %v\n%s", err, stderr)
	}
}

// TestThePromptCarriesWhatARelierNeeds asserts on the request, which is §18.6's
// first carried-over discipline: "a fixture that only dictates replies is a playback,
// not a test".
//
// Each of these is something the prompt is *supposed* to carry, and the fixture must
// fail when it does not — the contract is checked in both directions.
func TestThePromptCarriesWhatARelierNeeds(t *testing.T) {
	t.Parallel()
	bundleDir, uri := relayBundle(t)
	key, promptPath := firstPrompt(t, ingest(t, bundleDir, uri))
	if key == "" {
		t.Fatal("no cache key")
	}

	prompt, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(promptPath)))
	if err != nil {
		t.Fatalf("read the emitted prompt: %v", err)
	}
	text := string(prompt)

	for name, want := range map[string]string{
		"the source it is about":      uri,
		"the archived text's hash":    "Archived text hash:",
		"the verbatim requirement":    "verbatim",
		"the word minimum":            "6 words",
		"the reply format":            "```yaml",
		"the prompt-injection rule":   "is data",
		"the source text's own words": "cleared on restart",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the prompt does not carry %s (%q)", name, want)
		}
	}

	// And the agent can act on it, which is the assertion the individual strings are
	// only evidence for.
	if _, aErr := scriptedAgent(text); aErr != nil {
		t.Errorf("an agent reading the real prompt cannot answer it: %v", aErr)
	}
}

// TestAPromptWithNoSourceTextCannotBeAnswered is the adversarial half, and it is what
// makes the fixture a test rather than a playback.
//
// §4.1's whole argument is that a prompt built from text nobody kept produces
// quotations nobody can verify. If `Render` ever stopped fencing the archived text,
// every hand-written-reply test in this package would keep passing. This one would not.
func TestAPromptWithNoSourceTextCannotBeAnswered(t *testing.T) {
	t.Parallel()
	bundleDir, uri := relayBundle(t)
	_, promptPath := firstPrompt(t, ingest(t, bundleDir, uri))

	prompt, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(promptPath)))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	cases := map[string]string{
		"the source section removed": string(prompt)[:sourceSection(t, string(prompt))],
		"the fence emptied":          emptyFence(t, string(prompt)),
		"nothing at all":             "",
	}
	for name, damaged := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if reply, aErr := scriptedAgent(damaged); aErr == nil {
				t.Errorf("the agent answered a prompt with no quotable source: %q", reply)
			}
		})
	}
}

// TestAReplyQuotingTextTheSourceLacksIsRefused is the other adversarial direction:
// the fixture must not be able to get an invented quotation past `admit`.
//
// It is the property the whole corpus rests on (§9.4), and asserting it here rather
// than only in the gate's own tests is deliberate — this is the path a real agent
// takes, and a fabrication that survived the relay would survive it whatever the gate
// asserts in isolation.
func TestAReplyQuotingTextTheSourceLacksIsRefused(t *testing.T) {
	t.Parallel()
	bundleDir, uri := relayBundle(t)
	key, _ := firstPrompt(t, ingest(t, bundleDir, uri))

	fabricated := filepath.Join(t.TempDir(), "reply.md")
	body := "```yaml\ntitle: Cache Lifetime\ntype: Reference\nclaims:\n" +
		"  - text: A sentence nobody ever wrote.\n    quotes:\n" +
		"      - A sentence nobody ever wrote in any source at all\n```\n"
	if err := os.WriteFile(fabricated, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, _, err := run(t, "--bundle", bundleDir, "--jsonl",
		"admit", "--key", key, "--submitter", "agent:scripted", fabricated,
	); err == nil {
		t.Error("a fabricated quotation was admitted")
	}
}

// emptyFence keeps a prompt's shape and removes the archived text from inside its
// fence, which is the mutation "the source section removed" does not cover: a prompt
// that still announces a source and carries none.
//
// It discovers the fence the way the agent does rather than assuming its length, so
// the mutation cannot silently stop mutating if the fence changes.
func emptyFence(t *testing.T, prompt string) string {
	t.Helper()

	at := sourceSection(t, prompt)
	open := strings.Index(prompt[at:], "``````")
	if open < 0 {
		t.Fatal("the prompt has no fence")
	}
	open += at
	lineEnd := strings.IndexByte(prompt[open:], '\n')
	if lineEnd < 0 {
		t.Fatal("the prompt's fence has no end of line")
	}
	fence := strings.TrimRight(prompt[open:open+lineEnd], "\r")
	return prompt[:open+lineEnd+1] + fence + "\n"
}

// sourceSection is where the prompt's source section starts.
//
// It exists so the damage cases cannot cut at a -1: a mutation built from an index
// that was never found would silently produce the whole prompt back, and the test
// asserting the agent cannot answer it would fail for the wrong reason — or worse,
// a differently-worded heading would make it pass.
func sourceSection(t *testing.T, prompt string) int {
	t.Helper()

	at := strings.Index(prompt, sourceMarker)
	if at < 0 {
		t.Fatalf("the prompt has no %q section to damage", sourceMarker)
	}
	return at
}
