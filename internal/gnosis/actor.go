package gnosis

import "strings"

// The three kinds of actor, and the prefix each is written with.
//
// ActorUnset is the zero value and names nobody. It is not "the current user" and
// not "the system": a write whose approver was never populated has no approver,
// and the whole point of §4.6.2 carrying one on the command is that it cannot be
// silently supplied later.
const (
	ActorUnset Actor = ""

	// KindHuman is a person. §10.6.4 counts *distinct human actors* to decide
	// whether a review tier amplified anything, which is the reason kinds are
	// distinguished at all rather than left as free text: if a check could pass
	// for a person, that count is wrong in the direction that flatters the
	// corpus.
	KindHuman = "human"

	// KindAgent is a model-driven caller. Never an approver at a tier that
	// requires human review, and the type is what makes that checkable.
	KindAgent = "agent"

	// KindCheck is a deterministic check. §5.5 has `findings.opened_by` naming
	// one, because "who says so" is the first thing a reader asks of a finding
	// and a check name is as much an answer as a person is.
	KindCheck = "check"
)

// Actor identifies who or what performed or authorised an action.
//
// The form is `<kind>:<id>` — `human:priya`, `agent:ingest`, `check:duplicate`.
// The prefix is required rather than inferred: an unprefixed name cannot be told
// from a human one, and every place this value is counted or gated on cares which
// kind it is.
type Actor string

// ParseActor validates an actor string.
//
// Requires: nothing.
// Ensures: reports false for the empty string, for a missing or unknown kind, and
// for an empty identifier after the colon. It does not normalise case: `human:P`
// and `human:p` are different people as far as this type is concerned, because
// guessing that they are the same would merge two reviewers into one and §10.6.4
// counts them.
func ParseActor(s string) (Actor, bool) {
	kind, id, found := strings.Cut(s, ":")
	if !found || id == "" {
		return ActorUnset, false
	}
	switch kind {
	case KindHuman, KindAgent, KindCheck:
		return Actor(s), true
	default:
		return ActorUnset, false
	}
}

// Kind is the actor's kind, or "" for an unset or malformed actor.
func (a Actor) Kind() string {
	kind, id, found := strings.Cut(string(a), ":")
	if !found || id == "" {
		return ""
	}
	switch kind {
	case KindHuman, KindAgent, KindCheck:
		return kind
	default:
		return ""
	}
}

// ID is the actor's identifier, or "" for an unset or malformed actor.
func (a Actor) ID() string {
	if a.Kind() == "" {
		return ""
	}
	_, id, _ := strings.Cut(string(a), ":")
	return id
}

// IsHuman reports whether this actor is a person.
//
// Requires: nothing.
// Ensures: false for ActorUnset and for a malformed actor, so a tier requiring
// human review cannot be satisfied by a value nobody set or nobody can parse.
func (a Actor) IsHuman() bool { return a.Kind() == KindHuman }
