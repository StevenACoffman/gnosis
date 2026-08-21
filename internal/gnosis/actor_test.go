package gnosis_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

func TestParseActor(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   string
		ok   bool
		kind string
		id   string
	}{
		"human":           {"human:priya", true, gnosis.KindHuman, "priya"},
		"agent":           {"agent:ingest", true, gnosis.KindAgent, "ingest"},
		"check":           {"check:duplicate", true, gnosis.KindCheck, "duplicate"},
		"empty":           {"", false, "", ""},
		"no prefix":       {"priya", false, "", ""},
		"unknown kind":    {"robot:x", false, "", ""},
		"empty id":        {"human:", false, "", ""},
		"colon only":      {":", false, "", ""},
		"id with a colon": {"agent:ingest:v2", true, gnosis.KindAgent, "ingest:v2"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := gnosis.ParseActor(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok {
				assertParts(t, got, tc.kind, tc.id)
				return
			}
			if got != gnosis.ActorUnset {
				t.Errorf("a rejected actor came back as %q, not unset", got)
			}
		})
	}
}

// assertParts checks an accepted actor's two accessors together, because a kind
// without its id is half an answer.
func assertParts(t *testing.T, got gnosis.Actor, kind, id string) {
	t.Helper()
	if got.Kind() != kind {
		t.Errorf("kind = %q, want %q", got.Kind(), kind)
	}
	if got.ID() != id {
		t.Errorf("id = %q, want %q", got.ID(), id)
	}
}

// TestOnlyAHumanIsHuman is the check §10.6.4 depends on: it counts distinct human
// actors to decide whether a review tier amplified anything, and if a check or an
// agent could pass for a person that count is wrong in the direction that
// flatters the corpus.
func TestOnlyAHumanIsHuman(t *testing.T) {
	t.Parallel()
	if !gnosis.Actor("human:priya").IsHuman() {
		t.Error("a human is not human")
	}
	for _, a := range []gnosis.Actor{
		gnosis.ActorUnset, "agent:ingest", "check:duplicate", "priya", "robot:x", "human:",
	} {
		if a.IsHuman() {
			t.Errorf("%q reads as human", a)
		}
	}
}

// TestCaseIsNotFolded: human:P and human:p are different people as far as this
// type is concerned, because guessing they are the same would merge two reviewers
// into one and §10.6.4 counts them.
func TestCaseIsNotFolded(t *testing.T) {
	t.Parallel()
	upper, ok := gnosis.ParseActor("human:P")
	if !ok {
		t.Fatal("human:P was rejected")
	}
	lower, _ := gnosis.ParseActor("human:p")
	if upper == lower {
		t.Error("two differently-cased identifiers folded into one actor")
	}
}

// TestMalformedActorAnswersNothing: an actor nobody can parse must not report a
// kind or an id, so a caller cannot act on half of a bad value.
func TestMalformedActorAnswersNothing(t *testing.T) {
	t.Parallel()
	for _, a := range []gnosis.Actor{gnosis.ActorUnset, "priya", "robot:x", "human:"} {
		if k := a.Kind(); k != "" {
			t.Errorf("%q reports kind %q", a, k)
		}
		if id := a.ID(); id != "" {
			t.Errorf("%q reports id %q", a, id)
		}
	}
}
