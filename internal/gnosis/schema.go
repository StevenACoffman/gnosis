package gnosis

// SchemaVersion is the version of the corpus conventions this build writes.
//
// It advances only when a change makes existing documents *wrong* — a newly
// required frontmatter key, a changed meaning for an existing one, a different
// shape for `ontology.toml`. It does not advance for a new optional key, a new
// check, or an edit to the specification's prose, because a version that moved on
// every change would report every document as outdated and mean nothing.
//
// Version 1 is the Phase 1 conventions: `gnosis_id` in frontmatter, concepts under
// `c/<uuid7>-<slug>.md`, `type` required per OKF §4.1. The next advance is already
// known: SPEC §5.5.1 requires `gnosis_claims`, and no document written under
// version 1 carries it.
const SchemaVersion = 1
