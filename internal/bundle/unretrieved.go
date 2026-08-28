package bundle

import (
	"sort"
	"time"
)

// Reach is what §12.2's claim-grain question can honestly answer.
//
// **A count over a window and never a fraction** (§17, §12.2). A
// proportion-of-the-corpus-ever-retrieved figure is the most target-shaped number this
// specification could produce: it looks like progress, and it rises when a claim is
// deleted.
type Reach struct {
	// Window is the moment the report counts from.
	Window time.Time `json:"window"`

	// Claims is how many claims the corpus holds.
	Claims int `json:"claims"`

	// Observed is how many were returned by a search since Window.
	Observed int `json:"observed"`

	// Quiet names the claims not observed returned since Window, best-identified
	// first. **Not "never retrieved"** — recording is best-effort and per-user, so
	// this is what *this* user's searches were seen to return.
	Quiet []QuietClaim `json:"quiet"`
}

// QuietClaim is one claim nothing was seen to return, with enough to find it.
type QuietClaim struct {
	ClaimID string `json:"claim_id"`
	Path    string `json:"path"`

	// Lead is the claim's conclusion, or empty when extraction has not written one.
	// A claim with no lead is also a claim `search --claims` cannot return at all,
	// which is a different problem wearing this one's clothes — so the report says
	// which of the two it is rather than listing them together.
	Lead string `json:"lead,omitempty"`
}

// Unreached folds a corpus and a retrieval log into §12.2's report.
//
// Requires: claims is every claim in the corpus; log is this user's history; since bounds
// the window.
// Ensures: Quiet is sorted by path then claim id, so two runs over one state produce the
// same report. Pure.
//
// **A claim with no lead is excluded from Quiet and counted nowhere else**, because it
// cannot be returned by `search --claims` at all (§5.5.3) — reporting it as unreached
// would blame a claim for a gap in extraction, and the shortfall it belongs to is the one
// `search --claims` already prints beside its results.
func Unreached(claims []QuietClaim, log map[string]Retrieval, since time.Time) *Reach {
	out := &Reach{Window: since, Quiet: make([]QuietClaim, 0)}
	for _, c := range claims {
		if c.Lead == "" {
			continue
		}
		out.Claims++
		if r, ok := log[c.ClaimID]; ok && !r.LastAt.Before(since) {
			out.Observed++
			continue
		}
		out.Quiet = append(out.Quiet, c)
	}
	sort.Slice(out.Quiet, func(i, j int) bool {
		if out.Quiet[i].Path != out.Quiet[j].Path {
			return out.Quiet[i].Path < out.Quiet[j].Path
		}
		return out.Quiet[i].ClaimID < out.Quiet[j].ClaimID
	})
	return out
}
