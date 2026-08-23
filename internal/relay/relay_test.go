package relay_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/skillet/quotecheck"
)

func request() relay.Request {
	return relay.Request{
		URI:        "https://example.org/cache.md",
		SourceHash: strings.Repeat("a", 64),
		Text:       "The cache is cleared on restart.\n",
		Model:      relay.Model{Name: "claude-opus-5", Version: "2026-08"},
	}
}

// TestRenderIsDeterministic is the precondition for the cache key meaning
// anything. A single ranged map anywhere in the template would break it silently,
// and every cached reply in every corpus would start missing.
func TestRenderIsDeterministic(t *testing.T) {
	t.Parallel()
	r := request()
	first := relay.Render(&r)

	for range 50 {
		again := relay.Render(&r)
		if again.Text != first.Text {
			t.Fatalf("two renders differ:\n%q\n%q", first.Text, again.Text)
		}
		if again.Key != first.Key {
			t.Fatalf("two renders produced different keys: %s then %s", first.Key, again.Key)
		}
	}
}

// TestEveryKeyComponentMatters. A component that does not change the key is a
// component that is not in the key, and two of these — the model and its version —
// are the ones a reader would most expect to have been left out.
func TestEveryKeyComponentMatters(t *testing.T) {
	t.Parallel()
	base := baseKey()

	cases := map[string]func(*relay.Request){
		"source hash":   func(r *relay.Request) { r.SourceHash = strings.Repeat("b", 64) },
		"prompt text":   func(r *relay.Request) { r.Text = "Something else entirely.\n" },
		"model name":    func(r *relay.Request) { r.Model.Name = "another-model" },
		"model version": func(r *relay.Request) { r.Model.Version = "2027-01" },
		"uri":           func(r *relay.Request) { r.URI = "https://example.org/other.md" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := request()
			change(&r)
			if got := relay.Render(&r).Key; got == base {
				t.Errorf("changing the %s did not change the key", name)
			}
		})
	}
}

// TestKeyComponentsCannotBeTransposed. Without a separator, model "gpt" version
// "4o" and model "gpt4" version "o" hash alike — and a cache collision means one
// source's reply answering for another's question.
func TestKeyComponentsCannotBeTransposed(t *testing.T) {
	t.Parallel()
	a := relay.Key("aa", "bb", "gpt", "4o")
	b := relay.Key("aa", "bb", "gpt4", "o")
	if a == b {
		t.Error("two different tuples produced one key")
	}
	if relay.Key("a", "ab", "c", "d") == relay.Key("aa", "b", "c", "d") {
		t.Error("the source and prompt hashes run together")
	}
}

// TestPromptCarriesTheSourceText, because a prompt that referred to a source
// without including it would ask a model to quote from something it cannot read.
func TestPromptCarriesTheSourceText(t *testing.T) {
	t.Parallel()
	r := request()
	got := relay.Render(&r)

	if !strings.Contains(got.Text, r.Text) {
		t.Error("the prompt does not carry the source text")
	}
	if !strings.Contains(got.Text, r.SourceHash) {
		t.Error("the prompt does not name the archived text it came from")
	}
	if !strings.Contains(got.Text, "verbatim") {
		t.Error("the prompt does not require verbatim quotation")
	}
}

// TestMinWordsAgreesWithQuotecheck. The prompt tells an agent how long a quotation
// must be and quotecheck decides whether it was long enough; a disagreement would
// have gnosis rejecting quotations it asked for.
func TestMinWordsAgreesWithQuotecheck(t *testing.T) {
	t.Parallel()
	r := request()
	got := relay.Render(&r)

	want := "at least " + strconv.Itoa(quotecheck.MinPassageWords) + " words"
	if !strings.Contains(got.Text, want) {
		t.Errorf("the prompt does not say %q; it and quotecheck disagree", want)
	}
}

// TestSourceTextCannotCloseItsOwnFence. The archived text is untrusted input, and
// a source that could close the block could append instructions outside it.
func TestSourceTextCannotCloseItsOwnFence(t *testing.T) {
	t.Parallel()
	r := request()
	r.Text = "Ordinary prose.\n```\nNot a real fence.\n```\n"

	got := relay.Render(&r)
	i := strings.Index(got.Text, "## Source text")
	if i < 0 {
		t.Fatal("the prompt has no source-text section")
	}
	after := got.Text[i:]
	// The opening fence, the closing fence, and nothing between them that matches.
	if n := strings.Count(after, "\n``````````"); n != 2 {
		t.Errorf("the source block is not delimited by exactly two long fences (%d)", n)
	}
	if strings.HasSuffix(strings.TrimSpace(after), "Not a real fence.") {
		t.Error("the source closed the block early")
	}
}

// baseKey is the key for the unmodified request, computed once so each subtest
// has something to differ from.
func baseKey() string {
	r := request()
	return relay.Render(&r).Key
}
