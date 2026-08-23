package ontology_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/ontology"
)

// withSubject renders a vocabulary holding one subject, so a test can vary only
// the part it is about.
func withSubject(body string) string {
	return "version = 1\n\n[[subjects]]\nkey = \"retry.max_attempts\"\n" +
		"dimension = \"count\"\ndesc = \"how many attempts\"\n" + body
}

// TestARejectionIsAccepted, which is the ordinary case and the one that must not
// be made awkward: a vocabulary that recorded refusals grudgingly would not get
// them recorded.
func TestARejectionIsAccepted(t *testing.T) {
	t.Parallel()
	src := withSubject(`aliases = ["retry budget"]

  [[subjects.rejected]]
  alias = "retry policy"
  reason = "covers backoff and jitter too"
`)
	o, err := ontology.Load([]byte(src))
	if err != nil {
		t.Fatalf("a well-formed rejection was refused: %v", err)
	}
	// And a rejected phrase does not resolve, which is the point of refusing it.
	if _, ok := o.ResolveSubject("retry policy"); ok {
		t.Error("a rejected alias resolves")
	}
	if _, ok := o.ResolveSubject("retry budget"); !ok {
		t.Error("an admitted alias does not resolve")
	}
}

// TestARejectionNeedsItsReason. A refusal nobody explained records that somebody
// said no and not what they knew, which leaves the next person to work it out
// again — the exact re-litigation the list exists to prevent.
func TestARejectionNeedsItsReason(t *testing.T) {
	t.Parallel()
	src := withSubject(`
  [[subjects.rejected]]
  alias = "retry policy"
`)
	_, err := ontology.Load([]byte(src))
	if err == nil {
		t.Fatal("a rejection with no reason loaded")
	}
	if !strings.Contains(err.Error(), "no reason") {
		t.Errorf("the error does not say what is missing: %v", err)
	}
	if !strings.Contains(err.Error(), "retry.max_attempts") {
		t.Errorf("the error does not name the entry: %v", err)
	}
}

// TestAPhraseCannotBeBothAdmittedAndRefused. §5.8.2.1 makes resolution exclusive,
// so the corpus would be claiming the phrase both does and does not resolve, and
// whichever the loader applied first would silently become the answer.
func TestAPhraseCannotBeBothAdmittedAndRefused(t *testing.T) {
	t.Parallel()
	src := withSubject(`aliases = ["retry budget"]

  [[subjects.rejected]]
  alias = "retry budget"
  reason = "changed my mind halfway down the file"
`)
	_, err := ontology.Load([]byte(src))
	if err == nil {
		t.Fatal("a phrase that is both an alias and a rejection loaded")
	}
	if !strings.Contains(err.Error(), "both an alias and a rejection") {
		t.Errorf("error = %v", err)
	}
}

// TestTheContradictionIsCaughtAcrossFolding, or the check is defeated by
// capitalisation — which is exactly how the contradiction would arrive in
// practice, since nobody writes the same phrase twice deliberately.
func TestTheContradictionIsCaughtAcrossFolding(t *testing.T) {
	t.Parallel()
	src := withSubject(`aliases = ["Retry Budget"]

  [[subjects.rejected]]
  alias = "retry budget"
  reason = "ambiguous"
`)
	if _, err := ontology.Load([]byte(src)); err == nil {
		t.Error("a differently-cased contradiction loaded")
	}
}

// TestRejectionsAreOptional, so an existing vocabulary keeps loading and the
// requirement applies to what is written rather than to what is absent.
func TestRejectionsAreOptional(t *testing.T) {
	t.Parallel()
	if _, err := ontology.Load([]byte(withSubject("aliases = [\"retry budget\"]\n"))); err != nil {
		t.Errorf("a vocabulary with no rejections was refused: %v", err)
	}
}

// TestTypesCarryRejectionsToo, since §5.8.2 puts aliases on both and the argument
// for recording a refusal does not distinguish them.
func TestTypesCarryRejectionsToo(t *testing.T) {
	t.Parallel()
	src := "version = 1\n\n[[types]]\nkey = \"Procedure\"\ndesc = \"steps\"\n" +
		"aliases = [\"Runbook\"]\n\n  [[types.rejected]]\n  alias = \"Guide\"\n"

	_, err := ontology.Load([]byte(src))
	if err == nil {
		t.Fatal("a type rejection with no reason loaded")
	}
	if !strings.Contains(err.Error(), "type") {
		t.Errorf("the error does not say which kind: %v", err)
	}
}

// TestTheSameAliasIsNotRejectedTwice, because two entries for one phrase mean two
// people wrote a reason and only one will be read.
func TestTheSameAliasIsNotRejectedTwice(t *testing.T) {
	t.Parallel()
	src := withSubject(`
  [[subjects.rejected]]
  alias = "retry policy"
  reason = "one reason"

  [[subjects.rejected]]
  alias = "retry policy"
  reason = "a different reason"
`)
	if _, err := ontology.Load([]byte(src)); err == nil {
		t.Error("one phrase was rejected twice")
	}
}
