// Package mine finds exchanges a corpus should have answered (SPEC §9.6).
//
// # Why this exists
//
// The manifesto requires every corrective interaction to be accretive, and that cannot
// depend on a model choosing to record it. §9.6 names two mechanisms that do not need
// cooperation: a Stop hook that hands the session over, and deterministic mining of what
// it contains. This package is the second — no network, no model, no clock.
//
// # What it reports and what it does not do
//
// It reports **candidates**: questions somebody had to ask more than once. It does not
// write to the corpus, and that is a finding rather than an omission — see Candidates.
package mine

import (
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// The two speakers a mined session distinguishes.
//
// RoleUser is the zero value, which is the safe default here: a turn whose role could not
// be read is treated as something a person said, so it can become a question but never an
// answer. The opposite default would let an unreadable line supply corpus content.
const (
	RoleUser Role = iota
	RoleAssistant
)

// The reasons an exchange becomes a candidate.
//
// Both are the same observation at two scopes — a question asked more than once — and
// they mean different things, which is why they are two constants and not a count.
const (
	// ReasonRetried: the question was asked again in the same session. The earlier
	// answer did not land, and the later one is what the person went away with.
	ReasonRetried Reason = "retried"

	// ReasonRecurring: the question was asked in more than one session. Nobody wrote
	// it down, and the next person will ask it too.
	ReasonRecurring Reason = "recurring"
)

// Role is who spoke.
type Role int

// Reason is why an exchange is worth writing up.
type Reason string

// Turn is one thing somebody said, normalized out of whatever store it came from.
//
// **The normalized shape is deliberately thin.** A session store carries tool calls,
// attachments, token counts, model names and a tree of parent pointers; none of that
// bears on the question this package asks, and a type that carried it would make the
// adapter's job "translate everything" rather than "translate what is used". §9.6's
// requirement is one seam so a foreign format change costs one file, and a narrow seam
// is what makes that affordable.
type Turn struct {
	// ID identifies the turn within its session, so a candidate can name where it
	// came from and a reader can go and look.
	ID string

	Role Role
	Text string

	// At is when it was said. It comes from the session file rather than a clock, so
	// this package stays pure.
	At time.Time
}

// Session is one conversation.
type Session struct {
	// ID is the session's own identifier, as its store names it.
	ID string

	// Turns are in the order they happened.
	Turns []Turn
}

// Exchange is one question and the answer it got.
type Exchange struct {
	// Session and QuestionID locate it.
	Session    string
	QuestionID string
	AnswerID   string

	Question string
	Answer   string
	At       time.Time
}

// Exchanges pairs each user turn with the assistant text that followed it.
//
// Requires: s.Turns are in the order they happened.
// Ensures: one exchange per user turn that got an answer, in order; a user turn followed
// by nothing is dropped, because a question with no answer is not something to write up.
// Pure.
//
// **Consecutive assistant turns are joined rather than treated as separate answers.** A
// session store splits one reply across many records — text, a tool call, more text — and
// counting those as three answers to one question would triple every candidate and make
// the report's numbers meaningless.
func Exchanges(s *Session) []Exchange {
	var out []Exchange
	for i := range s.Turns {
		if s.Turns[i].Role != RoleUser || strings.TrimSpace(s.Turns[i].Text) == "" {
			continue
		}
		answer, answerID := answerAfter(s.Turns, i)
		if answer == "" {
			continue
		}
		out = append(out, Exchange{
			Session:    s.ID,
			QuestionID: s.Turns[i].ID,
			AnswerID:   answerID,
			Question:   strings.TrimSpace(s.Turns[i].Text),
			Answer:     answer,
			At:         s.Turns[i].At,
		})
	}
	return out
}

// answerAfter joins the assistant turns following position i, up to the next question.
func answerAfter(turns []Turn, i int) (answer, firstID string) {
	var parts []string
	for _, turn := range turns[i+1:] {
		if turn.Role == RoleUser {
			break
		}
		if text := strings.TrimSpace(turn.Text); text != "" {
			if firstID == "" {
				firstID = turn.ID
			}
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n"), firstID
}

// key folds a question so two spellings of one question compare equal.
//
// `gnosis.Surface(...).Fold()` is the corpus's own normalization — the one the
// duplication signal and the coverage ledger already compare with — rather than a second
// one written here. Two normalizations in one codebase is two answers to "are these the
// same question", and they diverge on the day one of them is improved.
func key(question string) string { return gnosis.Surface(question).Fold() }
