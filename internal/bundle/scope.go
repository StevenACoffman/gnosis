package bundle

import (
	"os"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// defaultStatus is what a document with no OKF `status` is.
//
// OKF §5.4 treats an absent status as current, and the index's own column defaults to
// the same word — so `--status stable` finds a page that never declared one, which is
// what a reader means by it. Naming the default here rather than at the comparison keeps
// the two from disagreeing.
const defaultStatus = "stable"

// ScopeQuery is the restriction a caller asked for (§11.3).
//
// Every field is a zero-value-means-unrestricted filter, which is what lets one value
// carry six independent questions: a caller sets the ones they mean and the rest admit
// everything.
type ScopeQuery struct {
	// Type restricts to documents of one OKF type. Empty admits every type.
	Type string

	// Status restricts to one OKF §5.4 lifecycle value; a document declaring none is
	// `stable`. Empty admits every status.
	//
	// **The vocabulary is OKF's and is deliberately not closed here.** OKF §11 forbids
	// rejecting a document for the shape of an optional family, and a parser that
	// accepted only the words gnosis happens to write would refuse a conformant
	// corpus using its own — so this compares, and never validates.
	Status string

	// Under restricts to a bundle-relative path prefix. §11.3's checkable property is
	// about this one: "a query restricted to a subtree can never return a concept
	// outside it."
	Under string

	// Trust restricts to documents at or above a §14.1 tier, and Trusted reports
	// whether it was asked for.
	//
	// **Comma-ok rather than a zero Tier**, because the zero Tier is `unverified` —
	// the weakest — so a zero value would read as a filter admitting everything while
	// looking like a filter that was set.
	Trust   gnosis.Tier
	Trusted bool

	// Fresh restricts to documents whose sources are all verified and unexpired
	// (§14.3). It means exactly `fresh`: a document citing no source is
	// `not_applicable`, which is a different state and does not pass.
	Fresh bool

	// Provable restricts to documents whose every source can still be checked
	// offline (§14.4).
	Provable bool
}

// SearchScope answers whether one document is inside a query's restriction.
//
// **One value for six filters, loaded once.** Five of them need the bundle — the index
// holds no document type, no trust fold, and no freshness — and asking per result would
// walk the corpus once per hit. Assembling the answers first makes `Admits` a map
// lookup, and makes "restrict the results to documents with property P" one decision
// with six inputs rather than six branches that could disagree about what a restriction
// means.
//
// **Not named `Scope`.** §17.2 already uses that word for what a claim asserts, and
// §11.3's is a query restriction; one name for two would be disambiguated at every call
// site.
type SearchScope struct {
	query    ScopeQuery
	admitted map[string]bool
}

// Asked reports whether this query restricts anything.
//
// Requires: nothing.
// Ensures: false for the zero value, so a caller can skip the bundle read entirely —
// which is the difference between a plain search touching only the index and one paying
// for a corpus walk. Pure.
func (q *ScopeQuery) Asked() bool {
	return q.Type != "" || q.Status != "" || q.Under != "" || q.Trusted ||
		q.Fresh || q.Provable
}

// LoadScope reads what a query needs to decide (§11.3).
//
// Requires: dir is a bundle root; q may be the zero value.
// Ensures: a scope admitting everything when the query restricts nothing, without
// reading the bundle at all. Otherwise every document is judged once, so Admits is a
// lookup.
//
// **A path the corpus does not hold is not admitted**, which matters because the caller
// filters *index* results: a document deleted since the last rebuild is in the index and
// not in the bundle, and a restricted query must not return it. An unrestricted query
// still does, which is `index-drift`'s finding to report rather than a search's to hide.
func LoadScope(dir string, q ScopeQuery) (*SearchScope, error) {
	scope := &SearchScope{query: q}
	if !q.Asked() {
		return scope, nil
	}

	docs, err := Load(os.DirFS(dir))
	if err != nil {
		return nil, err
	}
	var (
		checks map[string]Check
		uris   map[string]string
	)
	if q.Fresh {
		if checks, uris, err = archiveIndex(dir); err != nil {
			return nil, err
		}
	}
	durability := map[string]gnosis.Durability{}
	if q.Provable {
		if durability, err = DurabilityByPath(os.DirFS(dir)); err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	scope.admitted = make(map[string]bool, len(docs))
	for i := range docs {
		doc := &docs[i]
		scope.admitted[doc.Path] = admits(&q, doc, now, checks, uris, durability)
	}
	return scope, nil
}

// Admits reports whether a document is inside the restriction.
//
// Requires: path is bundle-relative, as an index result carries it.
// Ensures: true for every path when the query restricted nothing. Pure.
func (s *SearchScope) Admits(path string) bool {
	if !s.query.Asked() {
		return true
	}
	return s.admitted[path]
}

// Restricted reports whether this scope filters anything, so a caller can say that a
// short result set was cut rather than empty.
func (s *SearchScope) Restricted() bool { return s.query.Asked() }

// admits is the per-document decision.
//
// Requires: checks and uris are populated when q.Fresh; durability when q.Provable.
// Ensures: every named restriction must hold — the filters are a conjunction, because a
// caller naming two means both. Pure.
//
// **Split by what each filter reads, which is why it is two functions and not six.**
// Three of them answer from the document's own frontmatter and three from a signal
// derived over the corpus, and that difference is the one a reader needs: the first
// group costs nothing and the second is why this value exists at all. The complexity
// limit asked for a split and reading it agreed with where.
func admits(
	q *ScopeQuery, doc *Document, now time.Time,
	checks map[string]Check, uris map[string]string,
	durability map[string]gnosis.Durability,
) bool {
	return admitsDeclared(q, doc) &&
		admitsDerived(q, doc, now, checks, uris, durability)
}

// admitsDeclared answers the filters a document's own frontmatter settles.
//
// Requires: nothing.
// Ensures: true when none of the three is asked for. Pure.
func admitsDeclared(q *ScopeQuery, doc *Document) bool {
	switch {
	case q.Under != "" && !underPrefix(doc.Path, q.Under):
		return false
	case q.Type != "" && string(doc.Type) != q.Type:
		return false
	case q.Status != "" && statusOr(doc.Status) != q.Status:
		return false
	default:
		return true
	}
}

// admitsDerived answers the filters a derived signal settles (§14.1, §14.3, §14.4).
//
// Requires: checks and uris are populated when q.Fresh; durability when q.Provable.
// Ensures: true when none of the three is asked for, without touching the maps — which
// is what lets LoadScope skip reading tier 0 for a query that does not need it. Pure.
func admitsDerived(
	q *ScopeQuery, doc *Document, now time.Time,
	checks map[string]Check, uris map[string]string,
	durability map[string]gnosis.Durability,
) bool {
	switch {
	case q.Trusted && !meetsTier(doc, q.Trust):
		return false
	case q.Fresh &&
		describeFreshness(now, doc, checks, uris).State != gnosis.FreshnessFresh:
		return false
	case q.Provable && durability[doc.Path] != gnosis.DurabilityProvable:
		return false
	default:
		return true
	}
}

// underPrefix reports whether a document sits under a path prefix.
//
// **A path-segment prefix, not a string prefix.** `c/retry` must not admit
// `c/retry-budget.md`, which a bare `strings.HasPrefix` would: §11.3's property is about
// a *subtree*, and a filter matching a sibling whose name starts with the same letters
// would be returning a concept outside the restriction while claiming otherwise.
func underPrefix(path, under string) bool {
	clean := strings.TrimSuffix(under, "/")
	return path == clean || strings.HasPrefix(path, clean+"/")
}

// statusOr is a document's effective OKF §5.4 status, defaulting the absent one.
func statusOr(declared string) string {
	if strings.TrimSpace(declared) == "" {
		return defaultStatus
	}
	return declared
}

// meetsTier reports whether a document's derived trust reaches a tier.
//
// **At or above, rather than exactly.** A reader asking for `machine-confirmed` wants
// what has been confirmed at least that far, and a filter that excluded
// `human-reviewed` from it would answer a question nobody asks. §14.1's tiers are
// ordered for exactly this.
func meetsTier(doc *Document, want gnosis.Tier) bool {
	return describeTrust(doc).State >= want
}

// ParseScopeQuery turns flag values into a query, or reports which one is unusable.
//
// Requires: nothing; every empty value means unrestricted.
// Ensures: EINVALID naming the accepted values for a flag that names something the
// vocabulary does not. Pure.
//
// The trust tier is parsed and the status is not, and the asymmetry is the point:
// §14.1's three tiers are gnosis's own closed set, so a fourth word is a mistake worth
// catching, while OKF owns the status vocabulary and §11 forbids refusing a corpus for
// using its own.
func ParseScopeQuery(docType, status, under, trust string, fresh, provable bool) (
	ScopeQuery, error,
) {
	const op = "bundle.ParseScopeQuery"

	q := ScopeQuery{
		Type: strings.TrimSpace(docType), Status: strings.TrimSpace(status),
		Under: strings.TrimSpace(under), Fresh: fresh, Provable: provable,
	}
	if raw := strings.TrimSpace(trust); raw != "" {
		tier, ok := gnosis.ParseTier(raw)
		if !ok {
			return ScopeQuery{}, &errs.Error{
				Code: errs.EINVALID,
				Message: op + ": trust " + raw + " is not one of " +
					strings.Join(gnosis.Tiers(), ", "),
			}
		}
		q.Trust, q.Trusted = tier, true
	}
	return q, nil
}
