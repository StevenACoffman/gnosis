package mine

import (
	"sort"
	"strings"
)

// Candidate is a question somebody had to ask more than once.
type Candidate struct {
	// Question is the first spelling of it, which is chronological and stable — the
	// report's order has to be reproducible, and picking the "best" phrasing would be
	// a judgement nothing here is equipped to make.
	Question string `json:"question"`

	// Answer is the last one given, because that is what the person went away with:
	// under ReasonRetried the earlier answers are the ones that did not land.
	Answer string `json:"answer"`

	// Reason says which observation produced it.
	Reason Reason `json:"reason"`

	// Asked is how many times the question was put. Never a rate and never a score
	// (§17): it is a count of occurrences, and it falls only when somebody writes the
	// answer down.
	Asked int `json:"asked"`

	// Sessions and Turns are where it came from, so a reader can go and look. A
	// candidate nobody can trace back is a suggestion, and §1.1's rule about naming a
	// witness applies to this report as much as to a claim.
	Sessions []string `json:"sessions"`
	Turns    []string `json:"turns"`
}

// tally accumulates exchanges by folded question.
//
// A type rather than three locals threaded between two functions, which is what the
// complexity limit asked for and reading it agrees with: the maps and the order slice
// are one thing — the running count — and naming it lets the accumulate step and the
// select step each be read on their own.
type tally struct {
	// order is the folded keys in first-seen order, which is what makes the output
	// chronological and stable. Map iteration is randomized, so a report built from
	// the map alone would come out differently on every run.
	order []string

	byKey       map[string]*Candidate
	seenSession map[string]map[string]bool
}

// Candidates finds the questions a corpus should have answered.
//
// Requires: sessions came from an adapter; each session's turns are in order.
// Ensures: one candidate per repeated question, ordered by first occurrence, which is
// chronological and stable across runs. Pure — no clock, no I/O, no map iteration
// reaching the output.
//
// # Repetition rather than sentiment, and it is the stronger signal
//
// §9.6's reference detects retry chains by reading negative feedback: "a prompt re-asked
// after negative feedback implies the earlier attempt failed". Reading feedback needs a
// vocabulary of what disappointment sounds like, a vocabulary is a `standards/` value
// with a rationale (§6.2), and nobody can write that rationale from measurement yet.
//
// The repetition is observable without any of it. A question asked twice in one session
// *is* a retry — that is what re-asking means — and a question asked in two sessions is
// one nobody wrote down. Both signals reduce to one comparison, the comparison is the
// corpus's own fold, and neither carries a number somebody chose.
//
// **What is not built, so its absence reads as a decision**: labelling outcomes from
// feedback signals, which is §9.6's third item. It needs the vocabulary above. The
// trigger is a corpus with enough mined sessions to write one from measurement.
//
// # It reports and never files
//
// §9.6 describes the Stop-hook companion as filing wiki-touching turns "as candidate
// answers, subject to §8.3's `file` gate". Building that would produce drafts the gate
// **must** refuse: a chat answer cites no archived source, and the promote gate fails a
// document declaring none — "a document asserting claims and citing nothing is exactly
// what this corpus exists to refuse". An automatic filer would fill quarantine with
// drafts that can never promote, which is a queue that only grows and a reader who
// learns to ignore it. So this reports, and writing a candidate up means `gnosis fetch`
// and `gnosis ingest`, where evidence exists.
func Candidates(sessions []Session) []Candidate {
	var t tally
	for i := range sessions {
		for _, ex := range Exchanges(&sessions[i]) {
			t.add(&ex)
		}
	}
	return t.repeated()
}

// add folds one exchange into the running count.
func (t *tally) add(ex *Exchange) {
	k := key(ex.Question)
	if k == "" {
		return
	}
	if t.byKey == nil {
		t.byKey = map[string]*Candidate{}
		t.seenSession = map[string]map[string]bool{}
	}
	cand, known := t.byKey[k]
	if !known {
		cand = &Candidate{Question: ex.Question}
		t.byKey[k] = cand
		t.seenSession[k] = map[string]bool{}
		t.order = append(t.order, k)
	}
	cand.Asked++
	// The last answer wins, because that is what the person went away with: under
	// ReasonRetried the earlier ones are precisely the answers that did not land.
	cand.Answer = ex.Answer
	cand.Turns = append(cand.Turns, ex.QuestionID)
	if !t.seenSession[k][ex.Session] {
		t.seenSession[k][ex.Session] = true
		cand.Sessions = append(cand.Sessions, ex.Session)
	}
}

// repeated is the questions that were asked more than once, in first-seen order.
//
// Two is not a tuned threshold: it is what "again" means. A question asked once is a
// question, and a question asked twice is one the corpus should have answered the first
// time.
func (t *tally) repeated() []Candidate {
	out := make([]Candidate, 0, len(t.order))
	for _, k := range t.order {
		cand := t.byKey[k]
		if cand.Asked < 2 {
			continue
		}
		cand.Reason = ReasonRetried
		if len(cand.Sessions) > 1 {
			// Across sessions outranks within one. Both are true of a question asked
			// twice in each of two sessions, and the recurring reading is the one that
			// says the corpus is missing something rather than that one conversation
			// went badly.
			cand.Reason = ReasonRecurring
		}
		out = append(out, *cand)
	}
	return out
}

// Report renders candidates for a person, one per line, most-asked first.
//
// Requires: candidates came from Candidates.
// Ensures: a stable ordering — by count, then by first occurrence, so two runs over one
// session set produce identical output. Pure.
//
// **Sorted by count and not presented as a rate.** §17 forbids a count shown as health,
// and this is a count of things nobody wrote down: it goes up when the corpus is used
// and down only when somebody writes an answer. A percentage here would be the most
// target-shaped number the tool could produce.
func Report(candidates []Candidate) []Candidate {
	out := make([]Candidate, len(candidates))
	copy(out, candidates)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Asked > out[j].Asked })
	return out
}

// OneLine collapses a candidate's question so it fits a terminal line.
//
// Requires: nothing.
// Ensures: whitespace collapsed and the result no longer than width runes, with an
// ellipsis when it was cut. Pure.
func OneLine(s string, width int) string {
	flat := strings.Join(strings.Fields(s), " ")
	runes := []rune(flat)
	if width <= 0 || len(runes) <= width {
		return flat
	}
	return string(runes[:width]) + "…"
}
