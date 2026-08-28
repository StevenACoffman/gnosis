package gnosis

import "github.com/StevenACoffman/skillet/quotecheck"

// The four drift states of SPEC §14.3.2.
//
// DriftUnchecked is the zero value, for the same reason FreshnessUnknown is: §14.3.2
// says of it that "neither of the above may be asserted", and a zero value of Benign
// would assert one of them — that every claim resting on this source is still
// supported — on every source nobody has re-checked, which is every source until
// somebody runs the re-check. This is the fourth place the discipline appears, after
// `quotecheck.Unchecked`, `gate.VerdictUnchecked`, and `FreshnessUnknown`.
//
// DriftNone is not in §14.3.2's table, and adding it is deliberate. That table
// describes a source whose "bytes differ", which reads as a precondition the caller
// must establish — and a precondition stated only in prose has no failure mode. So
// the byte comparison is inside the function and its answer is a state, which makes
// Drift total: there is no way to call it wrongly and no fourth answer it cannot
// give.
//
// An integer enumeration rather than a string one, on `Freshness`'s argument: a
// string type's zero value is "", which is none of these states, and a comment
// claiming it means `unchecked` does not make it so.
const (
	DriftUnchecked DriftState = iota
	DriftNone
	DriftBenign
	DriftUnsupported
)

// DriftState is how an archived source stands against its upstream.
type DriftState int

// Drifted is what a re-check concluded about one source.
type Drifted struct {
	State DriftState

	// Missing are the passages the new bytes no longer contain, deduplicated and in
	// the order the quotations gave them. Set only for DriftUnsupported, because it
	// is the only state that names anything: §14.3.2 asks for "a finding per affected
	// claim, naming the passage", and a passage is what lets the caller find the
	// claims — the join is by quotation, not by document.
	Missing []string
}

// String renders the state as §14.3.2's vocabulary writes it.
func (d DriftState) String() string {
	switch d {
	case DriftNone:
		return "drift-none"
	case DriftBenign:
		return "drift-benign"
	case DriftUnsupported:
		return "drift-unsupported"
	case DriftUnchecked:
		return "drift-unchecked"
	default:
		return "invalid"
	}
}

// MarshalText renders the state as a word in the machine envelope, so an agent
// branches on "drift-unchecked" rather than on the integer 0, whose meaning depends
// on declaration order.
func (d DriftState) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// Actionable reports whether this state asks somebody to do something.
//
// True for Unsupported alone. Benign is explicitly not a finding (§14.3.2: "not a
// downgrade of trust... rendering it as a warning would train readers past the state
// that matters"), None is nothing happening, and Unchecked is the absence of an
// answer rather than a bad one — a caller wanting to chase what was not checked
// should ask for that, and this is not it. The shape follows
// `Freshness.Trustworthy`, which refuses the same collapse from the other end.
func (d DriftState) Actionable() bool { return d == DriftUnsupported }

// Drift decides how an archived source stands against freshly fetched bytes.
//
// Requires: archivedSHA256 is the hash tier 0 recorded for this source and
// upstreamSHA256 the hash of what was just fetched, either of which may be empty
// when it could not be established; quotes are the quotations this corpus recorded
// against this source, in `gnosis_evidence` form; upstream is the newly fetched text.
// Ensures: a state, and the missing passages when and only when the state is
// DriftUnsupported. Pure — no fetching, no clock, no disk — so the decision can be
// pinned exactly by a test and the network stays in the shell (§4.6).
//
// # Why the caution runs one way
//
// The three answers cost different amounts to be wrong about, so the order of the
// tests is the order of increasing willingness to assert:
//
//   - **A missing passage wins outright.** It is the strongest signal §10 can receive
//     short of a contradiction, and one lost passage is not offset by ten that held.
//   - **Anything unchecked blocks benign.** A quotation too short to split yields
//     `quotecheck.Unchecked`, and calling the source benign on the strength of its
//     neighbours would claim support for a claim nobody verified. Reporting the
//     source as unchecked understates what was learned; the reverse overstates it.
//   - **Benign requires every passage found, and at least one.** No passages is not
//     vacuous agreement: a source no claim quotes has nothing that could be found or
//     lost, so nothing was checked and that is what it says.
//
// # Two ways this could report a catastrophe that did not happen
//
// Both are guarded here rather than in the caller, because both are properties of
// the inputs and the caller cannot see the consequence.
//
// **Empty upstream text.** A fetch that returned nothing — a 404 body, a redirect to
// a login page, a truncated read — hashes to something that differs from the archive,
// and every recorded passage is then genuinely absent from it. Handing that to
// `quotecheck` would report `drift-unsupported` for every claim in the corpus resting
// on that source: the most serious event this system can report, manufactured by a
// network error. A source whose upstream came back empty is unchecked.
//
// **A hash nobody computed.** An empty hash on either side means the comparison could
// not be made, not that the bytes are the same — the same reason an absent entry in
// `checked.jsonl` means never-checked rather than checked-in-1970 (§14.3).
//
// A passage failing against the **archived** bytes is a different event entirely and
// this function has nothing to say about it: that is corruption, it fails hard, and
// §4.3.1 keeps it that way. This is only ever about the archive disagreeing with
// upstream, where the archive is by definition intact.
func Drift(archivedSHA256, upstreamSHA256, upstream string, quotes []string) Drifted {
	if archivedSHA256 == "" || upstreamSHA256 == "" {
		return Drifted{State: DriftUnchecked}
	}
	if archivedSHA256 == upstreamSHA256 {
		return Drifted{State: DriftNone}
	}
	if upstream == "" {
		return Drifted{State: DriftUnchecked}
	}

	findings := quotecheck.Check(quotes, []quotecheck.Source{{
		Name: "upstream", Text: upstream,
	}})
	if len(findings) == 0 {
		return Drifted{State: DriftUnchecked}
	}

	missing, unchecked := fold(findings)
	switch {
	case len(missing) > 0:
		return Drifted{State: DriftUnsupported, Missing: missing}
	case unchecked:
		return Drifted{State: DriftUnchecked}
	default:
		return Drifted{State: DriftBenign}
	}
}

// fold reduces the per-passage findings to the two facts the state depends on.
//
// Requires: findings came from quotecheck.Check.
// Ensures: the missing passages deduplicated in first-seen order, and whether
// anything went unchecked. Pure.
//
// Separated from Drift because the linter reported Drift's complexity and was right:
// the function had come to hold two guards, a hash comparison, a scan, and a
// classification. The split is by question rather than by line count — Drift decides
// a state, this counts what the passages did.
func fold(findings []quotecheck.Finding) (missing []string, unchecked bool) {
	seen := map[string]bool{}
	for _, f := range findings {
		switch {
		case f.Missing():
			if !seen[f.Passage] {
				seen[f.Passage] = true
				missing = append(missing, f.Passage)
			}
		case f.Status == quotecheck.Unchecked:
			unchecked = true
		}
	}
	return missing, unchecked
}
