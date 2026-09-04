package web

import "github.com/StevenACoffman/gnosis/internal/gnosis"

// The kinds of thing that can be waiting for a person.
const (
	// ItemDraft is a quarantined document the gate has judged.
	ItemDraft ItemKind = "draft"

	// ItemConflict is a contradiction the deterministic predicates found and cannot
	// settle (§10.2).
	ItemConflict ItemKind = "conflict"

	// ItemChallenge is a reader's contest of a claim, open and unanswered (§10.7).
	ItemChallenge ItemKind = "challenge"
)

// ItemKind says what sort of decision an item is asking for.
type ItemKind string

// Item is one thing waiting for a person, carrying what §13 requires to decide with.
//
// # The field list is a specification requirement, not an aesthetic
//
// §13: "The queue MUST present enough to decide with, and this is the higher-leverage
// investment than any authorization rule. For a conflict, that means both claims side by
// side, each one's sources with their OKF §5.1 credibility signals (`author`,
// `usage_count`, `last_modified`), the durability class of each (§14.4), the centrality
// class (§14.4.1), and the required `rationale` field. If the queue shows enough, a
// non-expert correctly recognizes when to defer; if it shows too little, even an expert
// guesses."
//
// So `Sides` is a slice rather than a single subject: a conflict has two and a draft has
// one, and a type that could hold only one would have made the conflict view a second
// type with a second renderer that could disagree with this one.
//
// **It is a view model built by the shell.** A handler that assembled it would be a
// handler that needed the corpus, which this package cannot import — so the constraint
// that looked like an obstacle is what keeps the presentation logic out of the layer
// that fetches.
type Item struct {
	Kind ItemKind `json:"kind"`

	// ID addresses the item for the action that resolves it. For a draft it is the
	// bundle-relative path; for a conflict or a challenge it is the claim reference,
	// which survives a retitle (§5.4).
	ID string `json:"id"`

	// Summary is one line a reviewer scans. The list is read before it is read
	// carefully, and an item nobody can triage from the list costs a page load to
	// dismiss.
	Summary string `json:"summary"`

	// Why is what the machine already decided, so a reviewer is not re-deriving it:
	// the gate's verdict for a draft, the predicate that fired for a conflict.
	Why string `json:"why,omitempty"`

	// Sides are the claims in question — two for a conflict, one otherwise.
	Sides []Side `json:"sides,omitempty"`

	// Action names the mutation that resolves this, so the form and the item cannot
	// disagree about what button to draw.
	Action CommandKind `json:"action"`
}

// Side is one claim in a decision, with everything §13 asks be shown beside it.
type Side struct {
	// Ref addresses the claim, and Path locates it for a person (§5.6: the path is a
	// view, the identifier is the address).
	Ref  string `json:"ref"`
	Path string `json:"path"`

	// Title and Text are what a reader recognises it by, and what it asserts.
	Title string `json:"title"`
	Text  string `json:"text"`

	// Trust, Durability and Centrality are the three derived signals §13 names.
	//
	// **All three, because they answer different questions and no two of them
	// substitute.** Trust says who confirmed it, durability says whether it can still
	// be checked offline, and centrality says how much it matters if it is wrong —
	// §14.4 is explicit that trust and durability are orthogonal and combining them
	// would be the score §17 refuses.
	Trust      gnosis.Tier       `json:"trust"`
	Durability gnosis.Durability `json:"durability"`
	Centrality string            `json:"centrality,omitempty"`

	// Sources are where the claim's evidence came from, with OKF §5.1's objective
	// signals. Never a score: §14.2 refuses a stored credibility number, and a queue
	// that showed one would be asking the reviewer to defer to it.
	Sources []Source `json:"sources,omitempty"`
}

// Source is one place a claim rests, with the signals OKF §5.1 records.
type Source struct {
	// Resource is the URI or the scope descriptor, as the document declares it.
	Resource string `json:"resource"`

	// Author, UsageCount and LastModified are OKF §5.1's objective signals, presented
	// and never combined. A reviewer weighing a source is doing the inference §14.2
	// says must stay theirs.
	Author       string `json:"author,omitempty"`
	UsageCount   int    `json:"usage_count,omitempty"`
	LastModified string `json:"last_modified,omitempty"`

	// Archived says whether tier 0 holds the bytes this rests on. A source nobody
	// fetched cannot be re-read, which is what makes a claim on it unprovable
	// (§14.4) — and it is the difference between "check it yourself" and "you
	// cannot".
	Archived bool `json:"archived"`
}

// Page is one rendered concept for the viewer.
type Page struct {
	ID    string `json:"gnosis_id"`
	Path  string `json:"path"`
	Title string `json:"title"`
	Type  string `json:"type"`
	Body  string `json:"body"`

	// Trust and Freshness are what §13 requires shown beside a concept, and Findings
	// are the open conflicts it asks be shown *inline* rather than on a separate page
	// — a contradiction a reader has to go looking for is one they do not find.
	Trust     gnosis.Tier `json:"trust"`
	Freshness string      `json:"freshness,omitempty"`
	Findings  []string    `json:"findings,omitempty"`

	// Links are the resolved outbound links §8.3 requires rendered inline, with each
	// target's title and identifier rather than the href alone. *As We May Think* §6's
	// second indictment is the reason: "having found one item, one has to emerge from
	// the system and re-enter on a new path".
	Links []Link `json:"links,omitempty"`
}

// Link is one resolved outbound reference.
type Link struct {
	ID    string `json:"gnosis_id"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// Hit is one search result.
type Hit struct {
	ID    string `json:"gnosis_id"`
	Path  string `json:"path"`
	Title string `json:"title"`
}
