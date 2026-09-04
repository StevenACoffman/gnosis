package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// transcript is two sessions asking one question twice, in the shape a session store
// writes: bookkeeping records, a sidechain, and content in both the string and the
// structured-parts form.
const transcript = `{"type":"mode","mode":"normal","sessionId":"s1"}
{"type":"user","uuid":"u1","sessionId":"s1","timestamp":"2026-09-03T10:00:00Z","message":{"role":"user","content":"how do I rotate the signing key?"}}
{"type":"assistant","uuid":"a1","sessionId":"s1","message":{"role":"assistant","content":[{"type":"text","text":"run the rotate script"}]}}
{"type":"user","uuid":"s1x","isSidechain":true,"sessionId":"s1","message":{"role":"user","content":"a subagent's own prompt"}}
{"type":"user","uuid":"u2","sessionId":"s2","timestamp":"2026-09-03T11:00:00Z","message":{"role":"user","content":"How do I rotate the signing key?"}}
{"type":"assistant","uuid":"a2","sessionId":"s2","message":{"role":"assistant","content":"there is a script for it"}}
{"type":"user","uuid":"u3","sessionId":"s2","message":{"role":"user","content":"what is the retry cap?"}}
{"type":"assistant","uuid":"a3","sessionId":"s2","message":{"role":"assistant","content":"three"}}
`

// TestMineReportsARecurringQuestionAndNothingElse. What I am afraid of is a miner that
// reports every exchange: a list nobody reads is worse than no list, because it looks
// like one.
func TestMineReportsARecurringQuestionAndNothingElse(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := run(t, "--jsonl", "mine", "--session", transcriptFile(t))
	if err != nil {
		t.Fatalf("mine: %v\n%s", err, stderr)
	}
	data := decodeData(t, stdout)
	cands, _ := data["candidates"].([]any)
	if len(cands) != 1 {
		t.Fatalf("got %d candidates, want the one repeated question: %v", len(cands), cands)
	}
	first, _ := cands[0].(map[string]any)
	if first["reason"] != "recurring" {
		t.Errorf("reason = %v, want recurring: it was asked in two sessions",
			first["reason"])
	}
	// The population travels with the candidates, so a short list can be read as a
	// fraction of something rather than as a number.
	if data["sessions"] != float64(2) || data["exchanges"] != float64(3) {
		t.Errorf("population = %v sessions, %v exchanges; want 2 and 3",
			data["sessions"], data["exchanges"])
	}
}

// TestMineReadsATranscriptFromStdin, which is the seam a Stop hook hands a session over
// through: the hook pipes the session in and gnosis never goes looking for it.
func TestMineReadsATranscriptFromStdin(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := runWithStdin(t, transcript, "mine", "--session", "-")
	if err != nil {
		t.Fatalf("mine: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, "recurring") {
		t.Errorf("the piped transcript produced no candidate:\n%s\n%s", stdout, stderr)
	}
	// It reports and never writes, which is the whole disposition: a chat answer
	// cites no archived source, so a filed one could never pass the promote gate.
	if !strings.Contains(stderr, "questions, not documents") {
		t.Errorf("the report does not say what these are: %s", stderr)
	}
}

// TestMineRefusesWithoutATranscript. There is no default session to read: guessing at
// one would make the command's input depend on which tool wrote the last file somewhere.
func TestMineRefusesWithoutATranscript(t *testing.T) {
	t.Parallel()

	_, stderr, err := run(t, "mine")
	if err == nil {
		t.Fatal("mine ran with no transcript")
	}
	if !strings.Contains(stderr, "--session") {
		t.Errorf("the refusal does not name the missing flag: %s", stderr)
	}
}

// transcriptFile writes the fixture and returns its path.
func transcriptFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(transcript), 0o600); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	return path
}
