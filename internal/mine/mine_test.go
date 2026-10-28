package mine_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/mine"
)

// session builds a conversation from alternating question/answer pairs.
func session(id string, pairs ...string) mine.Session {
	s := mine.Session{ID: id}
	for i, text := range pairs {
		role := mine.RoleUser
		if i%2 == 1 {
			role = mine.RoleAssistant
		}
		s.Turns = append(s.Turns, mine.Turn{
			ID: id + "-" + string(rune('a'+i)), Role: role, Text: text,
		})
	}
	return s
}

// TestCandidatesReportsOnlyRepeatedQuestions is the whole selection rule, and the thing
// I am afraid of is the opposite failure: a miner that reported every exchange would
// produce a list nobody reads, which is worse than no list because it looks like one.
func TestCandidatesReportsOnlyRepeatedQuestions(t *testing.T) {
	t.Parallel()

	got := mine.Candidates([]mine.Session{
		session("s1",
			"how do I rotate the signing key?", "run the rotate script",
			"what is the retry cap?", "three",
			"how do I rotate the signing key?", "run the rotate script, then redeploy"),
	})
	if len(got) != 1 {
		t.Fatalf("got %d candidates, want the one repeated question: %+v", len(got), got)
	}
	if got[0].Asked != 2 {
		t.Errorf("asked = %d, want 2", got[0].Asked)
	}
	if got[0].Reason != mine.ReasonRetried {
		t.Errorf("reason = %q, want retried: it was re-asked inside one session",
			got[0].Reason)
	}
	// The last answer, because that is what the person went away with — the earlier
	// one is precisely the answer that did not land.
	if !strings.Contains(got[0].Answer, "redeploy") {
		t.Errorf("answer = %q, want the one that ended the retry", got[0].Answer)
	}
	if len(got[0].Turns) != 2 {
		t.Errorf("turns = %v, want both askings named so a reader can go and look",
			got[0].Turns)
	}
}

// TestAQuestionInTwoSessionsIsRecurring. The two reasons mean different things: one
// conversation going badly is not the same finding as nobody having written the answer
// down, and only the second says the corpus is missing something.
func TestAQuestionInTwoSessionsIsRecurring(t *testing.T) {
	t.Parallel()

	got := mine.Candidates([]mine.Session{
		session("s1", "how do I rotate the signing key?", "run the rotate script"),
		session("s2", "How do I rotate the signing key?", "there is a script"),
	})
	if len(got) != 1 {
		t.Fatalf("got %d candidates, want one: the question is the same one", len(got))
	}
	if got[0].Reason != mine.ReasonRecurring {
		t.Errorf("reason = %q, want recurring", got[0].Reason)
	}
	// Folded, so two spellings of one question are one candidate — the corpus's own
	// normalization rather than a second one written here.
	if got[0].Asked != 2 || len(got[0].Sessions) != 2 {
		t.Errorf("asked = %d across %v, want 2 across both sessions",
			got[0].Asked, got[0].Sessions)
	}
}

// TestAQuestionWithNoAnswerIsNotACandidate. A question nobody answered is not something
// to write up, and it is the ordinary state of the last turn in a session that is still
// going — which is exactly when a Stop hook runs this.
func TestAQuestionWithNoAnswerIsNotACandidate(t *testing.T) {
	t.Parallel()

	got := mine.Candidates([]mine.Session{
		session("s1", "what is the retry cap?"),
		session("s2", "what is the retry cap?"),
	})
	if len(got) != 0 {
		t.Errorf("got %+v, want nothing: neither asking got an answer", got)
	}
}

// TestExchangesJoinOneReplySplitAcrossRecords. A session store splits one reply into
// several records, and counting those as separate answers would multiply every count and
// make the report's numbers mean nothing.
func TestExchangesJoinOneReplySplitAcrossRecords(t *testing.T) {
	t.Parallel()

	s := mine.Session{ID: "s1", Turns: []mine.Turn{
		{ID: "1", Role: mine.RoleUser, Text: "why?"},
		{ID: "2", Role: mine.RoleAssistant, Text: "first part"},
		{ID: "3", Role: mine.RoleAssistant, Text: "second part"},
	}}
	got := mine.Exchanges(&s)
	if len(got) != 1 {
		t.Fatalf("got %d exchanges, want 1", len(got))
	}
	if !strings.Contains(got[0].Answer, "first part") ||
		!strings.Contains(got[0].Answer, "second part") {
		t.Errorf("the joined answer lost a part: %q", got[0].Answer)
	}
}

// TestReadClaudeCodeSkipsWhatIsNotATurn. The transcript is another tool's working file,
// read while that tool may still be appending: the last line can be half written, and a
// read that failed on it would fail exactly when a Stop hook runs.
func TestReadClaudeCodeSkipsWhatIsNotATurn(t *testing.T) {
	t.Parallel()

	src := strings.Join([]string{
		`{"type":"mode","mode":"normal","sessionId":"s1"}`,
		`{"type":"user","uuid":"u1","sessionId":"s1","timestamp":"2026-09-03T10:00:00Z",` +
			`"message":{"role":"user","content":"what is the retry cap?"}}`,
		`{"type":"assistant","uuid":"a1","sessionId":"s1",` +
			`"message":{"role":"assistant","content":[{"type":"text","text":"three"},` +
			`{"type":"tool_use","name":"Read"}]}}`,
		`{"type":"user","uuid":"x","isSidechain":true,"sessionId":"s1",` +
			`"message":{"role":"user","content":"a subagent's prompt"}}`,
		`{"type":"user","uuid":"trunc","sessionId":"s1","message":{"role":"user"`,
	}, "\n")

	got, err := mine.ReadClaudeCode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d sessions, want 1", len(got))
	}
	if len(got[0].Turns) != 2 {
		t.Fatalf("got %d turns, want the question and the answer only: %+v",
			len(got[0].Turns), got[0].Turns)
	}
	// A tool call is what the assistant *did*; mining it would report a file path as
	// something the corpus should explain.
	if strings.Contains(got[0].Turns[1].Text, "Read") {
		t.Errorf("a tool call reached the normalized turn: %q", got[0].Turns[1].Text)
	}
	if got[0].Turns[0].At.IsZero() {
		t.Error("the timestamp did not survive normalization")
	}
}
