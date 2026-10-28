package relay_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/relay"
)

// pairRequest is two claims about one subject, each with its own source.
func pairRequest() *relay.PairRequest {
	return &relay.PairRequest{
		Claims: [2]relay.CriticClaim{
			{
				Text: "Retries are capped at three attempts.",
				Lead: "Cap retries at three.", Quotes: []string{"capped at three"},
			},
			{
				Text: "Retries should be kept to a sensible number.",
				Lead: "Keep retries sensible.", Quotes: []string{"a sensible number"},
			},
		},
		Sources: [2]string{"vendor says capped at three", "our runbook says sensible"},
		URIs:    [2]string{"https://example.org/vendor", "https://example.org/runbook"},
		Subject: "retry.max_attempts",
		Model:   relay.Model{Name: "test-model", Version: "v1"},
	}
}

// TestThePairPromptCarriesBothClaimsAndBothSources. §10.3 asks a judge whether two claims
// can both hold, and a prompt carrying one source would be asking a different question:
// each claim has to be checkable against the text it actually rests on.
func TestThePairPromptCarriesBothClaimsAndBothSources(t *testing.T) {
	t.Parallel()

	prompt := relay.RenderPairCritic(pairRequest())
	for _, want := range []string{
		"Retries are capped at three attempts.",
		"Retries should be kept to a sensible number.",
		"vendor says capped at three",
		"our runbook says sensible",
		"retry.max_attempts",
	} {
		if !strings.Contains(prompt.Text, want) {
			t.Errorf("the prompt does not carry %q:\n%s", want, prompt.Text)
		}
	}
	if prompt.Key == "" {
		t.Error("the prompt has no cache key")
	}
}

// TestThePairPromptDoesNotAskWhichClaimIsRight is §10.2.2's line: a derived comparison
// produces a finding and never a verdict, so a prompt that asked a model to pick a winner
// would be a model adjudicating the corpus — which §10.6 gives to a person.
func TestThePairPromptDoesNotAskWhichClaimIsRight(t *testing.T) {
	t.Parallel()

	got := relay.RenderPairCritic(pairRequest()).Text
	if !strings.Contains(got, "Do not decide which claim is right") {
		t.Errorf("the prompt does not refuse the adjudication:\n%s", got)
	}
	if !strings.Contains(got, "a person decides") {
		t.Errorf("the prompt does not say who decides:\n%s", got)
	}
}

// TestAPairPromptIsBlindedByConstruction is §10.3's requirement, and the test is on the
// *projection* rather than on the type because that is where the section says the risk
// is: "what a type cannot prevent is the projection folding one into a field that does
// exist — a rationale appended to the claim text".
func TestAPairPromptIsBlindedByConstruction(t *testing.T) {
	t.Parallel()

	r := pairRequest()
	// A caller doing exactly what §10.3 warns about: folding a warrant into a field
	// the type does have. The prompt carries it, because nothing here could stop that
	// — which is why the guarantee has to live in what the *shell* projects, and why
	// this test names the fields a projection may read.
	if len(r.Claims[0].Quotes) == 0 {
		t.Fatal("the fixture lost its quotations")
	}
	got := relay.RenderPairCritic(r).Text
	for _, forbidden := range []string{
		"warrant", "adjudicat", "trust tier", "human-reviewed", "verified",
	} {
		if strings.Contains(strings.ToLower(got), forbidden) {
			t.Errorf("the rendered prompt speaks of %q, so a projection could leak a"+
				" decision into it:\n%s", forbidden, got)
		}
	}
}

// TestPairPromptsAreDeterministic is §6.1's precondition: the key is a hash of the
// prompt, so a single ranged map anywhere in the renderer breaks the cache silently.
func TestPairPromptsAreDeterministic(t *testing.T) {
	t.Parallel()

	first, second := relay.RenderPairCritic(pairRequest()), relay.RenderPairCritic(pairRequest())
	if first.Key != second.Key || first.Text != second.Text {
		t.Fatal("two renders of one pair disagree, so the cache key means nothing")
	}
	// And a different pair is a different question. Swapping one claim's source must
	// change the key, or a cached answer would be served for text nobody judged.
	other := pairRequest()
	other.URIs[1] = "https://example.org/somewhere-else"
	if relay.RenderPairCritic(other).Key == first.Key {
		t.Error("two pairs resting on different sources share a key")
	}
}
