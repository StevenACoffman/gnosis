package bundle

import (
	"sort"
	"time"

	"github.com/StevenACoffman/gnosis/internal/okf"
)

// staleAfterKey is the OKF field carrying a document's declared expiry.
const staleAfterKey = "stale_after"

// staleAfter reads a document's declared expiry.
//
// Requires: doc is parsed.
// Ensures: the zero time when the key is absent, empty, or not a date. **A
// malformed date is not an error and not today**: treating it as an error would
// make one typo unlint-able, and treating it as expired would report a document
// stale on the strength of a value nobody can read. The zero time means "declared
// none", which is what the corpus can actually justify believing — and the OKF
// conformance check is where a malformed field belongs.
//
// Only the date form is accepted. OKF requires `YYYY-MM-DD` and §14.3 rests its
// determinism argument on that shape: a date "keeps the staleness decision a plain
// date comparison with no reference to when the concept was read." Accepting a
// timestamp here would quietly admit a value whose comparison depends on a zone.
func staleAfter(doc *okf.Document) time.Time {
	raw, ok := doc.Text(staleAfterKey)
	if !ok {
		return time.Time{}
	}
	at, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return time.Time{}
	}
	return at
}

// sourceKeys identifies the source versions a document rests on, as archive paths.
//
// Requires: claims are the document's parsed claims.
// Ensures: sorted and deduplicated, so two runs over one document agree and a
// snapshot is comparable. Derived from the archive paths the claims name rather
// than from `sources[]`, and that is the load-bearing choice: `sources[]` records
// the URI an author cited, while an archive path names the exact **version** whose
// bytes the quotation was checked against. Freshness is a question about the
// version, so keying on the URI would let a check of v1 vouch for a claim resting
// on v2.
func sourceKeys(claims []DocClaim) []string {
	seen := map[string]bool{}
	for i := range claims {
		for _, p := range claims[i].ArchivePaths {
			seen[p] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
