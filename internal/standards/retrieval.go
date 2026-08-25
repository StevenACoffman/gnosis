package standards

import (
	_ "embed"
	"sort"
	"strconv"
	"strings"

	"github.com/StevenACoffman/skillet/errs"
)

// RetrievalFileName is where a corpus's retrieval cases live, relative to the bundle
// root.
const RetrievalFileName = "standards/retrieval-cases.toml"

// The verdicts, with the one that asserts least as the zero value.
const (
	// VerdictUnrun is the zero value: the case was not graded. A default of Held
	// would report an unrun suite as a passing one, which is the collapse every
	// other zero value in this codebase is placed to refuse.
	VerdictUnrun Verdict = iota

	// VerdictHeld means the corpus answered as the case requires.
	VerdictHeld

	// VerdictFailed means it did not.
	VerdictFailed
)

// retrievalSeed is the case file every bundle starts from, and it deliberately
// contains no cases.
//
//go:embed retrieval-cases.toml
var retrievalSeed []byte

// Verdict is what one case's expectation did against one result set.
type Verdict int

// Retrieval is the labelled queries a corpus must answer (§11.0.2).
//
// # Why this file has no thresholds and no rationale field
//
// Every other file in this package holds tunables, and a `Value[T]` there carries the
// reason it is that number because §6.2 makes a threshold indefensible without one.
// This one holds no numbers at all. A case either holds or it does not, and there is no
// pass rate — §17 forbids presenting a count as health, and a retrieval score is the
// most tempting such count there is, because it looks like progress and can be raised
// by deleting the cases that fail.
//
// So the reason each case exists lives on the case, as prose, in `why`.
type Retrieval struct {
	Cases []Case `toml:"case"`
}

// Case is one query and what the corpus must answer.
type Case struct {
	// Query is what somebody asked, in the form `gnosis search` takes.
	Query string `toml:"query"`

	// Titles are the documents that must come back, by title.
	//
	// **Titles rather than identifiers, and §11.0.2's own proposal was wrong about
	// this.** Identifiers are assigned per corpus, so a case file naming them is
	// unportable — it cannot be reviewed by anyone reading it, cannot be lifted to
	// another bundle, and turns a failing case into an archaeology exercise. A title
	// is what the person authoring the case was actually looking for.
	Titles []string `toml:"titles"`

	// Nothing declares that the correct answer is that the corpus holds nothing.
	//
	// §11.0.2 asks for these by name and they are the half a search suite usually
	// omits: a corpus that answers every query with its best guess is a corpus that
	// cannot say "we do not know", and that is the answer §14.3's whole vocabulary
	// exists to make expressible.
	Nothing bool `toml:"nothing"`

	// Why is the disappointment that produced the case.
	//
	// Required, for §6.2's reason applied to a different artifact: a case with no
	// account of why it exists is one a later reader deletes when it fails, because
	// they cannot tell a real expectation from an invented one.
	Why string `toml:"why"`
}

// String renders a verdict.
func (v Verdict) String() string {
	switch v {
	case VerdictHeld:
		return "held"
	case VerdictFailed:
		return "failed"
	case VerdictUnrun:
		return "unrun"
	default:
		return "invalid"
	}
}

// MarshalText renders the verdict as a word in the machine envelope.
func (v Verdict) MarshalText() ([]byte, error) { return []byte(v.String()), nil }

// DefaultRetrieval is the embedded seed.
func DefaultRetrieval() []byte { return retrievalSeed }

// LoadRetrieval parses a retrieval-case file.
//
// Requires: src is the file's bytes.
// Ensures: EINVALID naming every malformed case at once, so one run tells an author
// everything to fix. A file with no cases is valid and is the shipped state. Pure.
func LoadRetrieval(src []byte) (*Retrieval, error) {
	const op = "standards.LoadRetrieval"

	var out Retrieval
	if err := decode(op, src, &out); err != nil {
		return nil, err
	}
	if why := malformed(&out); len(why) > 0 {
		return nil, &errs.Error{
			Code: errs.EINVALID, Message: op + ": " + strings.Join(why, "; "),
		}
	}
	return &out, nil
}

// malformed names every case that cannot be graded, and why.
//
// Requires: r is decoded.
// Ensures: one entry per problem, in case order. Pure.
//
// A case expecting titles *and* declaring nothing is the interesting refusal: it is not
// a typo but a contradiction, and grading it either way would make the file mean
// whichever the implementation happened to check first.
func malformed(r *Retrieval) []string {
	var why []string
	for i := range r.Cases {
		c := &r.Cases[i]
		at := "case " + strconv.Itoa(i+1)
		switch {
		case strings.TrimSpace(c.Query) == "":
			why = append(why, at+" has no query")
		case c.Nothing && len(c.Titles) > 0:
			why = append(why, at+" expects titles and also expects nothing; "+
				"a case cannot require both")
		case !c.Nothing && len(c.Titles) == 0:
			why = append(why, at+" expects neither titles nor nothing, so nothing "+
				"about the corpus would make it fail")
		case strings.TrimSpace(c.Why) == "":
			why = append(why, at+" has no `why`; a case with no account of the "+
				"disappointment that produced it is one a later reader deletes")
		}
	}
	return why
}

// Grade reports whether a case held against the titles a search returned.
//
// Requires: got is every title the search returned, in any order.
// Ensures: Held or Failed, never Unrun — a graded case has a verdict. Pure, which is
// the point: the search is the caller's and this is the whole of the judgement, so it
// is testable from literals (§4.6).
//
// # What "held" means, and what it deliberately does not
//
// Every expected title must appear. Extra results are **not** a failure: a corpus that
// grows a second relevant document has not regressed, and a case that failed on it
// would train an author to delete the case rather than read the result. This measures
// coverage and never precision, which is §11.0's own limit stated one level down.
//
// Matching is `Surface.Fold`-equivalent on case and whitespace via `strings.EqualFold`
// after trimming, because a case file is typed by hand and a title differing in a
// trailing space is the same title. It is not a substring match: "Cache" must not
// satisfy an expectation of "Cache Lifetime", or a case would pass on a document about
// something else.
func (c *Case) Grade(got []string) Verdict {
	if c.Nothing {
		if len(got) == 0 {
			return VerdictHeld
		}
		return VerdictFailed
	}
	for _, want := range c.Titles {
		if !containsTitle(got, want) {
			return VerdictFailed
		}
	}
	return VerdictHeld
}

// Missing is every expected title the search did not return, sorted.
//
// Requires: got is every title the search returned.
// Ensures: empty for a case that held, and empty for a `nothing` case whatever came
// back — there is no missing title when none was expected, and the verdict is what
// reports that failure. Pure.
func (c *Case) Missing(got []string) []string {
	if c.Nothing {
		return nil
	}
	var out []string
	for _, want := range c.Titles {
		if !containsTitle(got, want) {
			out = append(out, want)
		}
	}
	sort.Strings(out)
	return out
}

// containsTitle reports whether want is among got, ignoring case and surrounding
// space.
func containsTitle(got []string, want string) bool {
	w := strings.TrimSpace(want)
	for _, g := range got {
		if strings.EqualFold(strings.TrimSpace(g), w) {
			return true
		}
	}
	return false
}
