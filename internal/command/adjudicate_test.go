package command_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// TestAdjudicateRefusesAnAgent is the rule that separates this command from the other
// two writes a non-person may make: challenging and discarding grant nothing, and an
// adjudication is the corpus's one route for content that cannot be checked.
func TestAdjudicateRefusesAnAgent(t *testing.T) {
	t.Parallel()

	cmd := &command.Adjudicate{
		Path: "c/a.md", ClaimID: "c1", By: gnosis.Actor("agent:ingest"),
		Rationale: "the vendor limit is newer", Eff: command.EffectApply,
	}
	err := cmd.Validate()
	if errs.ErrorCode(err) != errs.EINVALID {
		t.Fatalf("an agent adjudication was accepted: %v", err)
	}
	if !strings.Contains(err.Error(), "human decision") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// TestAdjudicateRefusesASelfCoSignature. A second signature by the same person checked
// nothing, and it is the cheapest way to satisfy the requirement without meeting it.
func TestAdjudicateRefusesASelfCoSignature(t *testing.T) {
	t.Parallel()

	cmd := &command.Adjudicate{
		Path: "c/a.md", ClaimID: "c1", By: gnosis.Actor("human:priya"),
		CoSigner: gnosis.Actor("human:priya"), Rationale: "the vendor limit is newer",
		Eff: command.EffectApply,
	}
	if err := cmd.Validate(); errs.ErrorCode(err) != errs.EINVALID {
		t.Errorf("a self co-signature was accepted: %v", err)
	}
}

// TestAdjudicateReportsEveryProblemAtOnce, so a caller fixing one is not told about
// the next on the following run.
func TestAdjudicateReportsEveryProblemAtOnce(t *testing.T) {
	t.Parallel()

	err := (&command.Adjudicate{}).Validate()
	if err == nil {
		t.Fatal("a zero Adjudicate was accepted")
	}
	for _, want := range []string{
		"path is empty", "claim is empty", "by is unset",
		"rationale is empty", "effect is",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}
}

// TestAdjudicateAcceptsACompleteDecision is the negative half of the three above: a
// validator that refused everything would pass each of them and be useless.
func TestAdjudicateAcceptsACompleteDecision(t *testing.T) {
	t.Parallel()

	cmd := &command.Adjudicate{
		Path: "c/a.md", ClaimID: "c1", By: gnosis.Actor("human:priya"),
		CoSigner: gnosis.Actor("human:marcus"), Rationale: "the vendor limit is newer",
		Eff: command.EffectPreview,
	}
	if err := cmd.Validate(); err != nil {
		t.Errorf("a complete decision was refused: %v", err)
	}
}

// TestChallengeRefusesAClassNobodyDeclared, and names the six: a challenger who
// mistyped one needs the list, and one who invented one needs to know it is closed.
func TestChallengeRefusesAClassNobodyDeclared(t *testing.T) {
	t.Parallel()

	cmd := &command.Challenge{
		Path: "c/a.md", Class: gnosis.ChallengeClass("wrongness"),
		By: gnosis.Actor("human:dana"), Rationale: "it is wrong",
		Eff: command.EffectApply,
	}
	err := cmd.Validate()
	if errs.ErrorCode(err) != errs.EINVALID {
		t.Fatalf("an undeclared class was accepted: %v", err)
	}
	for _, class := range gnosis.ChallengeClasses() {
		if !strings.Contains(err.Error(), class) {
			t.Errorf("the refusal does not name the class %q: %v", class, err)
		}
	}
}

// TestChallengeAcceptsAnAgent. Challenging grants no authority, and a check that
// noticed a contradiction the selector is blind to is exactly the informant §6.2.1
// wants and has no person attached.
func TestChallengeAcceptsAnAgent(t *testing.T) {
	t.Parallel()

	cmd := &command.Challenge{
		Path: "c/a.md", Class: gnosis.ChallengeContradiction,
		By: gnosis.Actor("check:conflict"), Rationale: "this contradicts c/other.md",
		Eff: command.EffectApply,
	}
	if err := cmd.Validate(); err != nil {
		t.Errorf("an agent challenge was refused: %v", err)
	}
}
