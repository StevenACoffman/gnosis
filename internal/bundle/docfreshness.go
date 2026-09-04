package bundle

import (
	"os"
	"sort"
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// DocFreshness is one document's freshness, as a reader should see it.
type DocFreshness struct {
	State gnosis.Freshness `json:"state"`

	// CheckedAt is when the least recently verified of its sources was verified,
	// or the zero time when none has been. The *least* recent, because a document
	// resting on four sources is only as verified as its weakest one and reporting
	// the newest would let one re-fetch vouch for three nobody has looked at.
	CheckedAt time.Time `json:"checked_at,omitempty"`

	// StaleAfter is the date its author asked for it to be revisited by, if any.
	StaleAfter time.Time `json:"stale_after,omitempty"`

	// Why is one sentence a person can act on, populated for every state
	// including the good one. "Fresh" alone does not say what was checked.
	Why string `json:"why"`

	// Drift is the weakest verdict any of this document's sources reached against
	// upstream (§14.3.2), or empty when none was compared. See ClaimFreshness.Drift
	// for why it sits beside the freshness rather than inside it.
	Drift string `json:"drift,omitempty"`

	// Claims is each declared claim's own freshness, in the order the document
	// declares them, and empty for a document declaring none.
	//
	// **The document line is the weakest of these and stays that way.** Reporting
	// per claim is not a replacement for the conservative answer: a reader who
	// wants to know whether to trust the page still gets the same verdict, and one
	// who wants to know *which sentence* rests on the unverified source no longer
	// has to work it out. The old behaviour was the right conservative answer and
	// the wrong useful one, and both are now available.
	Claims []ClaimFreshness `json:"claims,omitempty"`
}

// ClaimFreshness is one claim's freshness, computed over the sources that claim
// cites rather than over the document's.
type ClaimFreshness struct {
	ID string `json:"id"`

	// Anchor is the span of the body the claim addresses (§5.5.1), carried so a
	// reader can find the sentence without opening the file. Empty for a claim
	// declaring none, which is `lint`'s finding rather than this function's.
	Anchor string `json:"anchor,omitempty"`

	State     gnosis.Freshness `json:"state"`
	CheckedAt time.Time        `json:"checked_at,omitempty"`
	Why       string           `json:"why"`

	// Sources are the distinct sources this claim's evidence came from, sorted, or
	// empty when none of its archive paths resolves to a record.
	//
	// **Distinct sources, and never a count.** A claim resting on four independent
	// sources and one resting on one look identical in frontmatter — both carry
	// `archive_paths` — and the honest fix is to say which, not how many. Four paths
	// may be four versions of one page, so a count of paths would report one source
	// as four; and a count of *sources* would still be the inheritance §1.1's local
	// reductionism refuses, where corroboration is a number a reader can compare
	// rather than a set they can examine. Nothing here sums them.
	//
	// Empty for a claim whose paths resolve to no record, rather than a list of the
	// paths themselves. "Cites a source tier 0 has no record of" is `lint`'s finding
	// — `archive-unrecorded` — and reporting it here too would show one defect twice.
	Sources []string `json:"sources,omitempty"`

	// Drift is what the last re-check concluded about this claim's evidence against
	// upstream (§14.3.2), or empty when no comparison has been made.
	//
	// **Beside the freshness rather than folded into it**, because §14.3.2 keeps the
	// two apart on purpose: freshness answers "when was this checked" and drift
	// answers "does upstream still say it". A claim can be freshly checked and have
	// lost its support, and that is the pair a reader most needs to see — which is
	// why reporting only the first was the gap this field closes.
	//
	// The weakest of the claim's sources, for the reason CheckedAt is the oldest: a
	// claim resting on three sources one of which withdrew support has lost support,
	// and reporting the best of the three would let two intact sources vouch for the
	// one that moved.
	Drift string `json:"drift,omitempty"`
}

// FreshnessFor reports one document's freshness state, and each of its claims'.
//
// Requires: now is the moment to judge against; rel is a bundle-relative document
// path.
// Ensures: `not_applicable` for a document citing no archived source, `unknown`
// for one whose sources have never been verified, `stale` past its declared date,
// and `fresh` otherwise. Never `fresh` for a document nobody has checked, which is
// the collapse §14.3's four states exist to prevent.
//
// The clock is a parameter for the same reason it is everywhere else here: this is
// the function whose boundary cases — a document expiring today, one expiring
// tomorrow — are the ones worth pinning.
func FreshnessFor(bundleDir, rel string, now time.Time) (DocFreshness, error) {
	doc, err := documentAt(bundleDir, rel)
	if err != nil {
		return DocFreshness{}, err
	}

	checks, uris, err := archiveIndex(bundleDir)
	if err != nil {
		return DocFreshness{}, err
	}
	return describeFreshness(now, doc, checks, uris), nil
}

// documentAt reads the one document a read-path command was asked about.
//
// Requires: bundleDir is a bundle root; rel is a bundle-relative document path.
// Ensures: ENOTFOUND when no document in the bundle carries that path, so a caller
// never receives a nil document and a nil error. The returned pointer is into a
// freshly loaded slice, so a caller may not assume it survives another load.
//
// Shared by every per-document read signal — freshness and trust so far — because
// the knowledge it holds is one decision: which file the reference names, and that a
// path the corpus does not hold is ENOTFOUND rather than an empty answer. Two copies
// of that would be two places for the second one to disagree.
//
// **It walks the whole bundle to find one document**, which is what Load does, and
// two signals on one command therefore walk it twice. Left as is deliberately: the
// walk is the shell's cheapest I/O, `show` already opens the index beside it, and a
// combined reader is the right change at the third signal rather than the second —
// measured then, not guessed at now.
func documentAt(bundleDir, rel string) (*Document, error) {
	const op = "bundle.documentAt"

	docs, err := Load(os.DirFS(bundleDir))
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	for i := range docs {
		if docs[i].Path == rel {
			return &docs[i], nil
		}
	}
	return nil, &errs.Error{
		Code: errs.ENOTFOUND, Message: op + ": no document at " + rel,
	}
}

// describeFreshness is the pure half: everything above it is I/O.
//
// The document and its claims go through one measurement — `verified` — so the two
// answers cannot disagree about what a check is worth. That matters more here than
// it looks: the document line is defined as the weakest claim, and computing it
// separately would let a rounding or an ordering difference make a page read fresher
// than the sentence it is made of.
func describeFreshness(
	now time.Time, doc *Document, checks map[string]Check, uris map[string]string,
) DocFreshness {
	out := DocFreshness{
		StaleAfter: doc.StaleAfter,
		Claims:     claimFreshness(now, doc, checks, uris),
	}
	state, oldest := verified(now, doc.SourceKeys, doc.StaleAfter, checks)
	out.State = state
	out.CheckedAt = oldest
	out.Why = whyDocument(state, oldest, doc.StaleAfter)
	out.Drift = weakestDrift(doc.SourceKeys, checks)
	return out
}

// claimFreshness is each claim's own freshness.
//
// Requires: doc came from Load.
// Ensures: one entry per declared claim, in declaration order, or nil for a document
// declaring none. Pure.
//
// A claim's freshness is a join it already has the key for: `archive_paths` names the
// evidence, and `checked.jsonl` says when this user last verified it. The document's
// own `stale_after` applies to every claim under it, because the date is a statement
// about what the document asserts (§14.3.0) and a claim is one of those assertions.
func claimFreshness(
	now time.Time, doc *Document, checks map[string]Check, uris map[string]string,
) []ClaimFreshness {
	if len(doc.Claims) == 0 {
		return nil
	}
	out := make([]ClaimFreshness, 0, len(doc.Claims))
	for i := range doc.Claims {
		c := &doc.Claims[i]
		state, oldest := verified(now, c.ArchivePaths, doc.StaleAfter, checks)
		out = append(out, ClaimFreshness{
			ID:        c.ID,
			Anchor:    c.Anchor,
			State:     state,
			CheckedAt: oldest,
			Why:       whyClaim(state, oldest, doc.StaleAfter),
			Drift:     weakestDrift(c.ArchivePaths, checks),
			Sources:   citedSources(c.ArchivePaths, uris),
		})
	}
	return out
}

// verified is the one measurement both grains use.
//
// Requires: paths are archive paths; staleAfter is the declared expiry or the zero
// time; checks maps a verified archive path to when it was verified.
// Ensures: the state, and the least recent check backing it — the zero time whenever
// no check does. Pure.
//
// The order of the questions is §14.3's and reversing any pair would collapse a state
// it keeps apart. Is there anything to check against? Then: has every source been
// looked at? Only then does the declared date get asked, because a document with a
// future `stale_after` that nobody ever checked must not read as fresh.
func verified(
	now time.Time, paths []string, staleAfter time.Time, checks map[string]Check,
) (gnosis.Freshness, time.Time) {
	if len(paths) == 0 {
		return gnosis.FreshnessNotApplicable, time.Time{}
	}
	var oldest time.Time
	for _, p := range paths {
		c, ok := checks[p]
		if !ok {
			return gnosis.FreshnessUnknown, time.Time{}
		}
		if oldest.IsZero() || c.At.Before(oldest) {
			oldest = c.At
		}
	}
	return gnosis.FreshnessOf(now, oldest, staleAfter, true), oldest
}

// weakestDrift is the most serious drift verdict across a set of archived sources.
//
// Requires: paths are archive paths; checks came from archiveIndex.
// Ensures: "" when nothing was compared, otherwise the worst verdict present. Pure.
//
// Worst rather than newest, and the ordering is the one §14.3.2 already implies:
// withdrawn support outranks everything, and a source nobody could check outranks one
// that was fine. A claim resting on three sources one of which lost its support has
// lost support, and reporting the best of the three would let two intact sources vouch
// for the one that moved — the same collapse §14.3 refuses when it takes the *oldest*
// check rather than the newest.
func weakestDrift(paths []string, checks map[string]Check) string {
	worst := ""
	for _, p := range paths {
		switch d := checks[p].Drift; d {
		case gnosis.DriftUnsupported.String():
			return d
		case gnosis.DriftUnchecked.String():
			worst = d
		case "":
			// Never compared. It says less than `drift-unchecked`, which is a
			// verdict somebody reached, so it does not outrank one.
		default:
			if worst == "" {
				worst = d
			}
		}
	}
	return worst
}

// whyDocument is the sentence for a whole document.
func whyDocument(state gnosis.Freshness, oldest, staleAfter time.Time) string {
	switch state {
	case gnosis.FreshnessNotApplicable:
		return "it cites no archived source, so there is nothing to be fresh against"
	case gnosis.FreshnessUnknown:
		return "at least one of its sources has never been verified against " +
			"upstream; run `gnosis fetch` to find out"
	case gnosis.FreshnessStale:
		return "its author asked for it to be revisited by " +
			staleAfter.Format(time.DateOnly) + ", which has passed"
	case gnosis.FreshnessFresh:
		return "its sources were all verified unchanged, least recently on " +
			oldest.Format(time.DateOnly)
	default:
		// A state added to the vocabulary and not handled here produces no
		// sentence rather than a wrong one.
		return ""
	}
}

// whyClaim is the sentence for one claim.
//
// Worded separately rather than shared with whyDocument, because the useful sentence
// is different at this grain: a document's is about the page, and a claim's has to say
// that the *rest* of the page may be fine — which is the whole reason for reporting
// per claim, and a sentence that said "it" would leave a reader where they started.
func whyClaim(state gnosis.Freshness, oldest, staleAfter time.Time) string {
	switch state {
	case gnosis.FreshnessNotApplicable:
		return "this claim cites no archived source"
	case gnosis.FreshnessUnknown:
		return "the evidence for this claim has never been verified against upstream"
	case gnosis.FreshnessStale:
		return "the document asked to be revisited by " +
			staleAfter.Format(time.DateOnly) + ", which has passed"
	case gnosis.FreshnessFresh:
		return "its evidence was verified unchanged on " + oldest.Format(time.DateOnly)
	default:
		return ""
	}
}

// citedSources is the distinct sources a set of archive paths came from.
//
// Named for the claim's side of the join rather than as `sourcesOf`, which already
// reads a document's declared OKF `sources` list. Two functions answering "which
// sources" from different evidence would be a name a reader has to disambiguate.
//
// Requires: paths are archive paths; uris maps an archive path to the source its
// record names.
// Ensures: sorted, deduplicated, and empty rather than nil when nothing resolves.
// Pure.
//
// Deduplicated because a source fetched twice has two records and may have two
// archive paths (§4.1), and a claim citing both cites one source at two versions.
// Reporting it twice would make a single page look like corroboration, which is the
// error this whole field exists to prevent — and it is the error a *count* would make
// unavoidable.
//
// Sorted because a map has no order, and a report that reordered a claim's evidence
// between two runs would be one nobody could diff.
func citedSources(paths []string, uris map[string]string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		uri, ok := uris[p]
		if !ok || seen[uri] {
			continue
		}
		seen[uri] = true
		out = append(out, uri)
	}
	sort.Strings(out)
	return out
}
