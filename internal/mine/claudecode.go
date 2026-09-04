package mine

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/StevenACoffman/skillet/errs"
)

// This file is the normalizing seam §9.6 asks for: everything that knows what another
// tool's session file looks like is here, and a format change costs this file and
// nothing else. The rest of the package works on `Session` and has never heard of JSONL.
//
// **Reading another tool's on-disk format will rot**, which the specification says
// plainly. The mitigations are that the seam is one file, that every field it reads is
// optional, and that a line it cannot understand is skipped rather than fatal — so a
// format that grows a field, renames one this does not read, or adds a record type
// degrades to mining fewer turns instead of failing.

// maxLineBytes bounds one transcript record.
//
// Generous rather than tuned: a single turn can carry a pasted file, and the cost of the
// bound being too small is a silently short read. It is a memory guard against a
// corrupt file claiming one enormous line, not a judgement about how long a turn should
// be.
const maxLineBytes = 16 * 1024 * 1024

// sessionLine is one record of a Claude Code session transcript.
//
// Only the fields this package uses are declared. Unknown fields are ignored by
// encoding/json, which is the property that lets a newer writer's file still read.
type sessionLine struct {
	Type        string `json:"type"`
	UUID        string `json:"uuid"`
	SessionID   string `json:"sessionId"`
	Timestamp   string `json:"timestamp"`
	IsSidechain bool   `json:"isSidechain"`

	// Message carries the content, whose shape differs by writer: a string for a
	// simple turn and a list of typed parts for a structured one. json.RawMessage
	// defers that decision to contentText, where both are handled.
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// contentPart is one element of a structured message content list.
type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ReadClaudeCode normalizes a Claude Code session transcript.
//
// Requires: r yields JSON lines, one record each, as the transcript is written.
// Ensures: one Session per distinct session identifier, each turn in file order; a line
// that is not JSON, not a turn, or carries no text is skipped. Not pure — it reads.
//
// # Skipping rather than failing, and the direction is deliberate
//
// A transcript is another tool's working file, and it is being read while that tool may
// still be appending to it: the last line can be half written. Failing the whole read on
// one unparseable record would make mining fail exactly when a session ends, which is
// when the Stop hook runs it. Skipping loses one turn, which affects a report and
// nothing else — no corpus content depends on this.
//
// **Sidechains are excluded.** A subagent's conversation is a tool the assistant used,
// not a question a person asked, and mining them would report the agent's own prompts as
// things the corpus should have answered.
func ReadClaudeCode(r io.Reader) ([]Session, error) {
	const op = "mine.ReadClaudeCode"

	var (
		order []string
		byID  = map[string]*Session{}
	)
	scanner := bufio.NewScanner(r)
	// A transcript line carries whole messages and outgrows the default 64KB buffer;
	// without this the scan stops at the first long turn and reports success, which is
	// a short read wearing a clean exit.
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	for scanner.Scan() {
		turn, sessionID, ok := readTurn(scanner.Bytes())
		if !ok {
			continue
		}
		session, known := byID[sessionID]
		if !known {
			session = &Session{ID: sessionID}
			byID[sessionID] = session
			order = append(order, sessionID)
		}
		session.Turns = append(session.Turns, turn)
	}
	if err := scanner.Err(); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}

	out := make([]Session, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

// readTurn normalizes one transcript line, reporting whether it is a turn at all.
func readTurn(line []byte) (turn Turn, sessionID string, ok bool) {
	var rec sessionLine
	if err := json.Unmarshal(line, &rec); err != nil {
		return Turn{}, "", false
	}
	if rec.IsSidechain || rec.SessionID == "" {
		return Turn{}, "", false
	}
	var role Role
	switch rec.Type {
	case "user":
		role = RoleUser
	case "assistant":
		role = RoleAssistant
	default:
		// Every other record type is session bookkeeping — modes, snapshots, titles,
		// hook results. Skipped by name rather than by absence of content, so a new
		// bookkeeping type does not start being mined as a turn.
		return Turn{}, "", false
	}
	text := contentText(rec.Message.Content)
	if strings.TrimSpace(text) == "" {
		return Turn{}, "", false
	}
	return Turn{
		ID: rec.UUID, Role: role, Text: text, At: parseTime(rec.Timestamp),
	}, rec.SessionID, true
}

// contentText is the prose in a message's content, in either shape it arrives in.
//
// Only `text` parts are kept. A tool call and its result are what the assistant *did*,
// and mining them would report a file path as something the corpus should explain.
func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var simple string
	if err := json.Unmarshal(raw, &simple); err == nil {
		return simple
	}
	var parts []contentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	var out []string
	for _, part := range parts {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			out = append(out, part.Text)
		}
	}
	return strings.Join(out, "\n")
}

// parseTime reads a record's timestamp, or reports the zero time.
//
// The zero time means "no answer" rather than "long ago" — `HeadTime`'s rule — and
// nothing here compares timestamps, so an unreadable one costs an ordering hint and not
// a candidate.
func parseTime(s string) time.Time {
	at, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return at.UTC()
}
