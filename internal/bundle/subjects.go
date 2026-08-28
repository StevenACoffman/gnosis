package bundle

import (
	"cmp"
	"os"
	"slices"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/errs"
)

// SubjectPopulation is what one subject key has accumulated.
//
// It is an instrument rather than a report of problems, and that is the whole reason
// it exists. §5.8.2.1's alias-collision rule fires only when somebody *declares* a
// colliding alias; it cannot fire when two groups have been using one word
// differently and neither has declared it, which is the ordinary way the problem
// arises. Detecting that needs a threshold — how bimodal, how many new surfaces — and
// §6.2 forbids inventing one. This produces the population a threshold could later be
// calibrated against.
type SubjectPopulation struct {
	Key gnosis.SubjectKey `json:"key"`

	// Claims and Documents are how much of the corpus rests on this key.
	Claims    int `json:"claims"`
	Documents int `json:"documents"`

	// Surfaces are the distinct phrases authors actually wrote for this key,
	// sorted. A key reached through four spellings is the "cluster of new aliases"
	// signal in its observable form, and it is a count nobody has to threshold to
	// read.
	Surfaces []string `json:"surfaces"`

	// DisjointEvidence reports the condition recorded as the drift detector's
	// trigger: two documents carrying claims on this key, neither citing any
	// archived file the other does.
	//
	// **Observable with no threshold, which is why it is the trigger.** Two groups
	// writing about one key from sources that do not overlap is the shape the
	// silent-drift failure actually takes — each half internally consistent, and
	// nothing comparing them. It is not a finding: disjoint evidence is often just
	// two teams reading different documentation about the same thing.
	//
	// A document citing nothing is excluded from the comparison. Two empty sets are
	// disjoint by the letter of it, and counting them would make this true of every
	// hand-written corpus, which is the opposite of a signal.
	DisjointEvidence bool `json:"disjoint_evidence"`
}

// Population is the whole vocabulary's occupancy, plus what did not land in it.
type Population struct {
	// Subjects are the declared keys carrying at least one claim, sorted by key.
	Subjects []SubjectPopulation `json:"subjects"`

	// Undeclared counts keys declared in ontology.toml that no claim names. It is
	// a count rather than a list because the `ontology` check already names the
	// unused *types*, and a reader wanting the same for subjects is asking a
	// vocabulary question rather than a population one.
	Undeclared int `json:"undeclared_keys"`

	// Unresolved counts claims whose subject phrase resolves to no key. They are
	// absent from Subjects because they belong to no key — the `subject-unknown`
	// check reports them individually, and the count is here so the totals add up.
	Unresolved int `json:"unresolved_claims"`
}

// subjectTally accumulates one key's claims as documents are walked.
type subjectTally struct {
	claims   int
	docs     map[string][]string
	surfaces map[string]bool
}

// Any reports whether the corpus has any subject population at all.
//
// A separate question from the counts, because "nothing yet" and "we did not look"
// must not render alike — the same reason Freshness keeps unknown apart from stale.
func (p *Population) Any() bool { return len(p.Subjects) > 0 || p.Unresolved > 0 }

// Subjects folds a corpus into its per-subject population.
//
// Requires: snap is fully populated; a zero Snapshot describes an empty corpus.
// Ensures: sorted by key, so two runs over one corpus are comparable. Pure — no I/O,
// no clock.
//
// It reports no severity and emits no finding, which is why it lives here rather than
// in `lint`. §17 forbids presenting a count as health, and a population is the most
// tempting such count there is: it looks like coverage and it can be raised by
// declaring subjects nobody uses.
func Subjects(snap *lint.Snapshot) *Population {
	claims := map[gnosis.SubjectKey]*subjectTally{}
	out := &Population{Subjects: []SubjectPopulation{}}

	for i := range snap.Documents {
		doc := &snap.Documents[i]
		for j := range doc.Claims {
			claim := &doc.Claims[j]
			if claim.Subject == "" {
				continue
			}
			key, ok := snap.Vocabulary.ResolvesSubject(claim.Subject)
			if !ok {
				out.Unresolved++
				continue
			}
			tally, seen := claims[key]
			if !seen {
				tally = &subjectTally{}
				claims[key] = tally
			}
			tally.add(doc.Path, claim.Subject, claim.ArchivePaths)
		}
	}

	out.Undeclared = undeclaredSubjectKeys(snap, claims)

	for key, tally := range claims {
		out.Subjects = append(out.Subjects, tally.population(key))
	}
	slices.SortFunc(out.Subjects, func(a, b SubjectPopulation) int {
		return cmp.Compare(a.Key, b.Key)
	})
	return out
}

// add records one claim under this key.
func (t *subjectTally) add(path, surface string, evidence []string) {
	if t.docs == nil {
		t.docs, t.surfaces = map[string][]string{}, map[string]bool{}
	}
	t.claims++
	t.surfaces[surface] = true
	t.docs[path] = append(t.docs[path], evidence...)
}

// population renders the tally, deciding the disjointness question once.
func (t *subjectTally) population(key gnosis.SubjectKey) SubjectPopulation {
	surfaces := make([]string, 0, len(t.surfaces))
	for s := range t.surfaces {
		surfaces = append(surfaces, s)
	}
	slices.Sort(surfaces)

	return SubjectPopulation{
		Key:              key,
		Claims:           t.claims,
		Documents:        len(t.docs),
		Surfaces:         surfaces,
		DisjointEvidence: disjoint(t.docs),
	}
}

// disjoint reports whether two documents cite evidence sets sharing nothing.
//
// Requires: nothing.
// Ensures: false when fewer than two documents cite anything at all. Pure.
func disjoint(docs map[string][]string) bool {
	sets := make([][]string, 0, len(docs))
	for _, paths := range docs {
		if len(paths) > 0 {
			sets = append(sets, paths)
		}
	}
	if len(sets) < 2 {
		return false
	}
	for i := range sets {
		for j := i + 1; j < len(sets); j++ {
			if !sharesAPath(sets[i], sets[j]) {
				return true
			}
		}
	}
	return false
}

// sharesAPath reports whether two evidence sets have any archived file in common.
func sharesAPath(a, b []string) bool {
	for _, p := range a {
		if slices.Contains(b, p) {
			return true
		}
	}
	return false
}

// undeclaredSubjectKeys counts declared keys no claim reached.
func undeclaredSubjectKeys(snap *lint.Snapshot, claimed map[gnosis.SubjectKey]*subjectTally) int {
	declared := map[gnosis.SubjectKey]bool{}
	for _, key := range snap.Vocabulary.SubjectOf {
		declared[key] = true
	}
	unused := 0
	for key := range declared {
		if _, ok := claimed[key]; !ok {
			unused++
		}
	}
	return unused
}

// LoadPopulation reads a bundle and folds it into its subject population.
//
// Requires: dir is a bundle root.
// Ensures: the corpus is read once and the fold is pure, so the report can be tested
// from literals rather than from a bundle arranged to hold each case.
//
// It builds the snapshot with a zero index and zero freshness, which Snapshot
// documents as meaning "no index" — this report reads documents and the vocabulary
// and nothing else, and requiring a rebuilt index to ask how the vocabulary is
// occupied would make the instrument unavailable exactly when a corpus is new.
func LoadPopulation(dir string) (*Population, error) {
	const op = "bundle.LoadPopulation"

	snap, err := Snapshot(os.DirFS(dir), IndexState{}, FreshnessState{})
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return Subjects(snap), nil
}
