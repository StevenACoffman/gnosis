package gnosis

import "strings"

// CriticCategoryPrefix marks a category a cold critic produced (§10.5).
//
// It is exported because two packages need the same answer to "did a critic say this":
// `relay` stamps it onto a verdict and the findings gate reads it back. A prefix agreed
// by inspection across two packages is a shared decision reflected in two modules, which
// is the information-leakage red flag — and the leak here would be silent, because a
// gate that stopped recognising critic findings would report every corpus as
// structurally checked only.
const CriticCategoryPrefix = "critic:"

// The four states §17.1 requires a gate to report.
//
// SemanticReviewUnknown is the zero value, and it is the honest one: a gate that could
// not read the record must not claim a structural pass was all that ran, and must not
// claim a critic ran either.
const (
	SemanticReviewUnknown SemanticReview = iota

	// SemanticStructuralOnly: nothing semantic has been run over this corpus. A pass
	// here means the corpus is **internally honest**, not that anybody agrees with it.
	SemanticStructuralOnly

	// SemanticClean: a critic has examined this corpus and reported nothing.
	//
	// It is a state of its own because a clean critic produces no findings, so "no
	// critic finding" and "no critic ran" are indistinguishable from the findings
	// alone — and only one of them is evidence that anybody looked.
	SemanticClean

	// SemanticFindings: a critic examined the corpus and had something to say.
	SemanticFindings
)

// SemanticReview is which of §17.1's two acts a gate's verdict rests on.
//
// §17.1: "`gate` MUST report which act ran… A structural pass reported as 'verified' is
// exactly the overclaim this section exists to prevent." The gap it names is Gettier's —
// a quotation can validate byte-exact, the claim can be true, and the quotation can fail
// to bear on the claim — and no amount of strengthening the structural checks closes it.
//
// **Four states rather than a bool**, for the reason `Freshness` has four: the two ways
// of having no critic finding are different facts, and a flag would collapse them into
// the flattering one.
type SemanticReview int

// String is the token the envelope carries.
func (s SemanticReview) String() string {
	switch s {
	case SemanticFindings:
		return "semantic-findings"
	case SemanticClean:
		return "semantic-clean"
	case SemanticStructuralOnly:
		return "structural-only"
	case SemanticReviewUnknown:
		return "unknown"
	default:
		// An unrecognised state claims the least: not that a critic ran, and not
		// that one did not.
		return "unknown"
	}
}

// MarshalText renders the state as a word in the machine envelope, which is what §17.1
// asks for when it requires a `semantic_review` field.
func (s SemanticReview) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// Reviewed reports whether a critic is known to have examined the corpus.
//
// Requires: nothing.
// Ensures: false for `unknown` as well as for `structural-only`. Pure.
//
// The two false cases differ in what is known and agree in what may be claimed, which
// is the whole point of keeping them apart in the type and together here: a caller
// asking "may I say this was reviewed" gets one answer, and a caller asking "why not"
// reads the state.
func (s SemanticReview) Reviewed() bool {
	return s == SemanticClean || s == SemanticFindings
}

// FoldSemanticReview derives §17.1's state from what a gate can see.
//
// Requires: categories are the categories of the findings the gate was handed;
// critiques is how many critiques this corpus's coverage ledger holds; ledgerRead
// reports whether that ledger could be read at all.
// Ensures: SemanticReviewUnknown when the ledger could not be read, whatever the
// findings say — except that a critic finding is itself proof a critic ran, so it wins.
// Pure, total.
//
// **A critic finding outranks an unreadable ledger**, and that ordering is the one
// non-obvious rule here: the finding is direct evidence of the act, while the ledger is
// a record of it, and evidence beats bookkeeping. The reverse would let a missing
// `.gnosis/` turn a corpus with critic findings in hand into `unknown`.
func FoldSemanticReview(categories []string, critiques int, ledgerRead bool) SemanticReview {
	for _, c := range categories {
		if strings.HasPrefix(c, CriticCategoryPrefix) {
			return SemanticFindings
		}
	}
	switch {
	case !ledgerRead:
		return SemanticReviewUnknown
	case critiques > 0:
		return SemanticClean
	default:
		return SemanticStructuralOnly
	}
}
