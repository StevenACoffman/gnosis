// Package relay implements the two-phase ingest/admit protocol (SPEC §8.2) and
// the content-addressed response cache that makes it deterministic (§6.1).
//
// gnosis never calls a model, and every shape here follows from that. `ingest`
// renders prompts to disk and stops; an agent, a person, or a script brings back
// replies; `admit` consumes them. Nothing in this package opens a socket, reads a
// clock, or generates a random number — a prompt that varied between runs would
// make the cache key meaningless, and the key is the whole determinism claim.
//
// # What the cache buys
//
// A second run over unchanged inputs makes no model calls and reproduces
// byte-identically. §6.1 calls that the single largest determinism win available
// and it is also what makes a full corpus rebuild affordable rather than a fresh
// bill.
//
// # Everything here is strict
//
// A reply arrives from a model and is therefore the least trustworthy input the
// system takes. It is parsed strictly and rejected whole: a partially applied
// reply would put content into quarantine that no one — not the agent, not the
// reader — believes they approved.
package relay

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// sep separates the key's components.
//
// A separator is required rather than cosmetic. Concatenating the parts bare
// would let two different tuples hash alike — model "gpt" version "4o" and model
// "gpt4" version "o" produce the same bytes — and a cache collision means one
// source's reply answering for another's. A byte that cannot occur in any
// component is what makes the encoding injective.
const sep = "\x00"

// Key is the cache key of SPEC §6.1:
// sha256(source_content_hash ‖ prompt_hash ‖ model ‖ model_version).
//
// Requires: sourceHash is a content hash of the source the prompt is about;
// promptHash is a hash of the rendered prompt; model and modelVersion identify
// what will answer it.
// Ensures: deterministic and pure — the same tuple always yields the same key, on
// any machine, so two users at one commit look up the same reply.
//
// **The model and its version are in the key, and that is the load-bearing part.**
// A reply is a claim about what a particular model said about a particular text.
// Keying only on the text would serve one model's answer to another model's
// question, which is not a cache hit — it is a substitution nobody was told about.
// When the model changes, the corpus re-asks, and that cost is the honest one.
//
// Every component is required. An empty one is not rejected here — this function
// cannot know whether a caller legitimately has no model version — but callers
// should treat an empty component as a bug, because it collapses the keyspace in
// exactly the direction that causes collisions.
func Key(sourceHash, promptHash, model, modelVersion string) string {
	sum := sha256.Sum256([]byte(strings.Join(
		[]string{sourceHash, promptHash, model, modelVersion}, sep)))
	return hex.EncodeToString(sum[:])
}

// HashText is the hash used for a prompt's own content, so no two call sites can
// disagree about what a prompt hash is.
func HashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
