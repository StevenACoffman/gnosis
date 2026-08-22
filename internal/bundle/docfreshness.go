package bundle

import (
	"os"
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
}

// FreshnessFor reports one document's freshness state.
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
	const op = "bundle.FreshnessFor"

	docs, err := Load(os.DirFS(bundleDir))
	if err != nil {
		return DocFreshness{}, &errs.Error{Op: op, Err: err}
	}
	var doc *Document
	for i := range docs {
		if docs[i].Path == rel {
			doc = &docs[i]
			break
		}
	}
	if doc == nil {
		return DocFreshness{}, &errs.Error{
			Code: errs.ENOTFOUND, Message: op + ": no document at " + rel,
		}
	}

	checks, err := checksByArchivePath(bundleDir)
	if err != nil {
		return DocFreshness{}, err
	}
	return describeFreshness(now, doc, checks), nil
}

// describeFreshness is the pure half: everything above it is I/O.
func describeFreshness(now time.Time, doc *Document, checks map[string]time.Time) DocFreshness {
	out := DocFreshness{StaleAfter: doc.StaleAfter}

	if len(doc.SourceKeys) == 0 {
		out.State = gnosis.FreshnessNotApplicable
		out.Why = "it cites no archived source, so there is nothing to be fresh against"
		return out
	}

	var oldest time.Time
	for _, k := range doc.SourceKeys {
		at, ok := checks[k]
		if !ok {
			out.State = gnosis.FreshnessUnknown
			out.Why = "at least one of its sources has never been verified against " +
				"upstream; run `gnosis fetch` to find out"
			return out
		}
		if oldest.IsZero() || at.Before(oldest) {
			oldest = at
		}
	}
	out.CheckedAt = oldest
	out.State = gnosis.FreshnessOf(now, oldest, doc.StaleAfter, true)

	switch out.State {
	case gnosis.FreshnessStale:
		out.Why = "its author asked for it to be revisited by " +
			doc.StaleAfter.Format(time.DateOnly) + ", which has passed"
	case gnosis.FreshnessFresh:
		out.Why = "its sources were all verified unchanged, least recently on " +
			oldest.Format(time.DateOnly)
	case gnosis.FreshnessUnknown, gnosis.FreshnessNotApplicable:
		// Unreachable: both are returned above. Handled so a future state added
		// to the vocabulary produces an empty Why rather than a wrong one.
	}
	return out
}
