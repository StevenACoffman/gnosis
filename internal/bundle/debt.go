package bundle

import (
	"sort"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// Carried is one document that entered the corpus over a signal that could not run.
//
// It is per (path, signal) rather than per path, because the question a reader has
// is "which claims were admitted with no conflict check" — a per-signal question —
// and a row listing three signals against one path would have to be re-grouped by
// every caller that asked it.
type Carried struct {
	// Path is the document, relative to the bundle root.
	Path string `json:"path"`

	// Signal is the gate signal that did not run.
	Signal string `json:"signal"`

	// Actor is who carried it. §10.6.4 counts distinct humans, and a debt register
	// that could not say who signed is a register of the wrong thing.
	Actor string `json:"actor"`

	// Rationale is the sentence the approver supplied, as the audit row recorded
	// it. Empty for a promotion the gate approved on its own, which cannot happen
	// while any signal is unchecked but is not this type's business to assume.
	Rationale string `json:"rationale,omitempty"`
}

// Debt is what the corpus was admitted over, per signal.
//
// SPEC §9.5 calls this the debt register and it is the field the whole
// carry-with-a-reason design rests on: a promotion carried by a person over an
// unrun check is defensible only if the corpus can later find every claim admitted
// that way. Otherwise "a human approved it" is indistinguishable from a bypass, and
// the unrun check quietly becomes a check that never runs.
type Debt struct {
	// BySignal counts the carried promotions per signal, so a reader sees the
	// shape before the list. Sorted by signal name in Signals.
	BySignal map[string]int `json:"by_signal"`

	// Signals are the signal names present, sorted, because a map has no order and
	// a report that reordered itself between runs would be unusable.
	Signals []string `json:"signals"`

	// Carried is every (path, signal) pair, sorted by path then signal.
	Carried []Carried `json:"carried"`

	// Rows is how many promotion rows the trail held, carried or not. It is the
	// denominator: "34 documents were admitted with no conflict check" means
	// something different against 40 promotions than against 4000.
	Rows int `json:"promotions"`

	// Unreadable is how many trail lines would not parse. A debt count computed
	// from a damaged trail is a **floor** rather than a total, and this is what
	// lets a reader say so. Zero for an intact trail.
	Unreadable int `json:"unreadable_lines,omitempty"`
}

// Complete reports whether the debt count is a total rather than a floor.
//
// Requires: nothing; the zero Debt is complete, describing a corpus with no
// promotions.
// Ensures: false exactly when the trail had lines that would not parse. A caller
// must render the difference: an incomplete count invites the reader to conclude
// the debt is smaller than it is, which is the direction that flatters.
func (d *Debt) Complete() bool { return d.Unreadable == 0 }

// Owed computes the debt register from a write trail.
//
// Requires: trail came from AuditTrail.
// Ensures: every promotion row carrying signals appears once per signal, sorted by
// path then signal so two runs over one trail are comparable. Rows counts every
// promotion row and Unreadable carries the trail's damage forward, so a caller can
// tell a total from a floor. Pure.
//
// Only successful promotions count. A *refused* promotion also records its unrun
// signals — §9.5's reasoning is that "a document that never landed may have been
// blocked by a check nobody has built" — but that is a different question with a
// different answer, and folding it in here would make the register report documents
// which are not in the corpus as debt the corpus is carrying. `quarantine` is where
// the refused ones are visible.
func Owed(trail *Trail) *Debt {
	d := &Debt{
		BySignal: map[string]int{},
		Signals:  []string{},
		Carried:  []Carried{},
	}
	d.Unreadable = len(trail.Malformed)

	for i := range trail.Rows {
		row := &trail.Rows[i]
		if row.Op != audit.OpPromote || row.Outcome != string(gnosis.StatusOK) {
			continue
		}
		d.Rows++
		for _, signal := range row.Signals {
			d.BySignal[signal]++
			d.Carried = append(d.Carried, Carried{
				Path:      primaryPath(row),
				Signal:    signal,
				Actor:     row.Actor,
				Rationale: row.Detail,
			})
		}
	}

	for signal := range d.BySignal {
		d.Signals = append(d.Signals, signal)
	}
	sort.Strings(d.Signals)
	sort.Slice(d.Carried, func(i, j int) bool {
		if d.Carried[i].Path != d.Carried[j].Path {
			return d.Carried[i].Path < d.Carried[j].Path
		}
		return d.Carried[i].Signal < d.Carried[j].Signal
	})
	return d
}

// Paths are the distinct documents carrying debt, sorted.
//
// Requires: d came from Owed.
// Ensures: each path once however many signals it carries, which is the population
// a sample should be drawn from — sampling the (path, signal) pairs would draw the
// same document repeatedly and call it three observations. Pure.
func (d *Debt) Paths() []string {
	seen := make(map[string]bool, len(d.Carried))
	out := make([]string, 0, len(d.Carried))
	for i := range d.Carried {
		if p := d.Carried[i].Path; !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// Restricted is the debt narrowed to the given paths.
//
// Requires: d came from Owed; paths is a subset of d.Paths().
// Ensures: a Debt whose Carried holds only those paths, with BySignal and Signals
// recomputed to match. Rows and Unreadable are **not** narrowed: they describe the
// trail rather than the selection, and a sample that also shrank its own
// denominator would report a rate of one. Pure.
func (d *Debt) Restricted(paths []string) *Debt {
	keep := make(map[string]bool, len(paths))
	for _, p := range paths {
		keep[p] = true
	}
	out := &Debt{
		BySignal:   map[string]int{},
		Signals:    []string{},
		Carried:    make([]Carried, 0, len(paths)),
		Rows:       d.Rows,
		Unreadable: d.Unreadable,
	}
	for i := range d.Carried {
		if !keep[d.Carried[i].Path] {
			continue
		}
		out.Carried = append(out.Carried, d.Carried[i])
		out.BySignal[d.Carried[i].Signal]++
	}
	for signal := range out.BySignal {
		out.Signals = append(out.Signals, signal)
	}
	sort.Strings(out.Signals)
	return out
}

// primaryPath is the path an audit row is about, or "" for a row that touched none.
//
// A promotion always names one. The empty case is here rather than assumed away
// because a row is validated for its op, actor, and time and deliberately not for
// its paths (§15) — a refused promotion legitimately touches nothing.
func primaryPath(row *audit.Row) string {
	if len(row.Paths) == 0 {
		return ""
	}
	return row.Paths[0]
}
