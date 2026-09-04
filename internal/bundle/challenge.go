package bundle

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/okf"
	"github.com/StevenACoffman/skillet/errs"
)

// challengeIndent is the indentation of a `gnosis_challenges` entry's fields, two
// spaces under the list marker, which is what renderConcept uses one key over.
const challengeIndent = "    "

// FiledChallenge is one challenge and the document carrying it.
//
// A pair rather than a map from path to challenges, because the listing is flat: it
// sorts every challenge in the corpus by age regardless of which page it sits on, and
// a map would make the caller rebuild that.
type FiledChallenge struct {
	Path      string
	Challenge gnosis.Challenge
}

// AppendChallenge is the document a filed challenge would produce (§10.7.4).
//
// Requires: existing is a concept's bytes; c carries a class, an actor and a non-empty
// rationale.
// Ensures: the returned bytes parse, hold every top-level key the original held, carry
// a byte-identical body, and carry one more challenge than they did. EINVALID when the
// original does not parse or when the result would fail any of those. Pure — no clock,
// no I/O.
//
// # Why this is textual surgery and not a re-render
//
// `Accrete` rewrites a document through `renderConcept`, which is safe there because
// the documents it edits are the ones gnosis wrote: `conceptDoc` models every key they
// carry. A challenged document is any document, including one somebody wrote by hand
// with `gnosis_limitations`, a scoped `sources` list, a `gnosis_constraint` pin, or a
// warrant — none of which `conceptDoc` models. Re-rendering would **silently drop
// them**, and §5.2's rule that frontmatter is retained verbatim is exactly this
// hazard: an encoder normalises what it understands and discards what it does not.
//
// So the block is inserted into the frontmatter as text, and the properties above are
// **checked rather than promised**. That is the same move `Accrete` makes when it
// re-parses its own output to prove it did not rewrite a body — a guarantee that lives
// in a comment is one that stops being true without telling anybody.
func AppendChallenge(existing []byte, c *gnosis.Challenge) ([]byte, error) {
	const op = "bundle.AppendChallenge"

	before, err := okf.Parse(existing)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	if strings.TrimSpace(c.Rationale) == "" {
		return nil, &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": a challenge needs a rationale (§10.7.2); a reader who" +
				" cannot say why a claim is wrong has filed a doubt, and the corpus" +
				" has no way to act on a doubt",
		}
	}

	out, err := insertChallenge(op, existing, c)
	if err != nil {
		return nil, err
	}
	if err := sameDocumentPlusOneChallenge(op, before, out); err != nil {
		return nil, err
	}
	return out, nil
}

// insertChallenge writes the entry into the frontmatter text.
//
// Requires: existing parses as a concept, so it opens with a frontmatter fence.
// Ensures: the entry is the last item of `gnosis_challenges`, and the key is created
// at the end of the frontmatter when the document has none. Pure.
//
// **Appended rather than prepended**, so the list reads in the order challenges were
// filed — which is the order `challenge --list --unanswered` sorts by and the order a
// reviewer reading the diff expects.
func insertChallenge(op string, existing []byte, c *gnosis.Challenge) ([]byte, error) {
	lines := strings.Split(string(existing), "\n")
	end, err := frontmatterEnd(op, lines)
	if err != nil {
		return nil, err
	}
	entry := renderChallenge(c)

	at, found := listEnd(lines, end, challengesKey)
	if !found {
		// No such key: the block goes at the end of the frontmatter, which is where
		// a top-level key can always be added without knowing anything about the
		// keys already there.
		at = end
		entry = append([]string{challengesKey + ":"}, entry...)
	}
	out := make([]string, 0, len(lines)+len(entry))
	out = append(out, lines[:at]...)
	out = append(out, entry...)
	out = append(out, lines[at:]...)
	return []byte(strings.Join(out, "\n")), nil
}

// frontmatterEnd is the index of the frontmatter's closing fence.
//
// Requires: lines is a parsed concept's lines, so line 0 is the opening fence.
// Ensures: the index of the closing `---`, or EINVALID naming the problem. Pure.
func frontmatterEnd(op string, lines []string) (int, error) {
	if len(lines) == 0 || strings.TrimRight(lines[0], " \t\r") != "---" {
		return 0, &errs.Error{
			Code: errs.EINVALID, Message: op + ": the document has no frontmatter fence",
		}
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t\r") == "---" {
			return i, nil
		}
	}
	return 0, &errs.Error{
		Code: errs.EINVALID, Message: op + ": the frontmatter is not closed",
	}
}

// listEnd is the index just past the last line of an existing top-level list block, and
// whether the document had one.
//
// Requires: end is the index of the frontmatter's closing fence; key is a top-level
// frontmatter key.
// Ensures: an index in (0, end] when found. Pure.
//
// **Keyed rather than fixed to `gnosis_challenges`**, because a second family needed the
// same surgery: `gnosis_conflicts` appends the same way, and two copies of this loop
// would be two readings of where a YAML block ends — the repetition this codebase treats
// as a design smell rather than as duplication to tolerate.
//
// The block ends at the first line that is neither blank nor indented, because that is
// the next top-level key — which is all a YAML block mapping's extent depends on, and
// the only part of YAML this function has to understand.
func listEnd(lines []string, end int, key string) (int, bool) {
	start := -1
	for i := 1; i < end; i++ {
		if strings.HasPrefix(lines[i], key+":") {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, false
	}
	last := start
	for i := start + 1; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(lines[i], " ") ||
			strings.HasPrefix(lines[i], "\t") {
			if trimmed != "" {
				last = i
			}
			continue
		}
		break
	}
	return last + 1, true
}

// renderChallenge is one `gnosis_challenges` entry, as lines.
//
// Requires: c carries a non-empty rationale.
// Ensures: deterministic — no clock and no map iteration, so a preview and an apply
// produce the same bytes. Pure.
//
// The rationale is written as a block scalar because it is prose a person wrote and may
// carry a colon, a quote, or a newline, any of which would need escaping inline. The
// state is written explicitly rather than omitted: `open` is the zero value in Go and
// an absent key is legible, but a reader scanning the frontmatter for what is
// outstanding should not have to know that.
func renderChallenge(c *gnosis.Challenge) []string {
	out := []string{
		"  - id: " + c.ID.String(),
		challengeIndent + "class: " + string(c.Class),
		challengeIndent + "by: " + c.By,
		challengeIndent + "at: " + c.At,
		challengeIndent + "rationale: |",
	}
	for _, line := range strings.Split(strings.TrimRight(c.Rationale, "\n"), "\n") {
		out = append(out, challengeIndent+"  "+line)
	}
	state := c.State
	if state == "" {
		state = gnosis.ChallengeOpen
	}
	return append(out, challengeIndent+"state: "+string(state))
}

// sameDocumentPlusOneChallenge is the invariant, checked rather than asserted.
//
// Requires: before is the parsed original; out is the candidate bytes.
// Ensures: nil only when out parses, keeps every top-level key before had, keeps the
// body byte-identical, and holds exactly one more challenge. EINVALID naming which of
// those failed. Pure.
//
// Every clause is here because the surgery could break it and nothing else would
// notice. A dropped key is the failure re-rendering would cause; a changed body is the
// failure a mis-placed insertion would cause; a challenge count that did not rise is
// the failure of writing into the wrong block, which would look like success.
func sameDocumentPlusOneChallenge(op string, before *okf.Document, out []byte) error {
	after, err := okf.Parse(out)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	if after.Body != before.Body {
		return &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": filing the challenge would change the document body," +
				" which a challenge never does; nothing was written",
		}
	}
	for _, key := range topLevelKeys(before) {
		if _, ok := after.Fields[key]; !ok {
			return &errs.Error{
				Code: errs.EINVALID, Op: op,
				Message: op + ": filing the challenge would drop the frontmatter key " +
					key + "; nothing was written",
			}
		}
	}
	if want := len(challengesOf(before)) + 1; len(challengesOf(after)) != want {
		return &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": the challenge did not land in gnosis_challenges;" +
				" nothing was written",
		}
	}
	return nil
}

// topLevelKeys is a parsed document's frontmatter keys, sorted so a diagnostic names
// the same one on every run.
func topLevelKeys(doc *okf.Document) []string {
	out := make([]string, 0, len(doc.Fields))
	for k := range doc.Fields {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// FileChallenge writes a reader's contest into the document it contests (§10.7.4).
//
// Requires: the writer holds the lock; rel is a bundle-relative concept path; c
// carries a class, an actor and a non-empty rationale.
// Ensures: the document on disk gains the challenge and loses nothing else, or nothing
// is written. ENOTFOUND when the bundle holds no document at rel.
//
// **It writes to the committed tier, not to the index.** `findings` lives in
// `.gnosis/index.db`, which is derived, gitignored and per-user — a challenge recorded
// only there would be invisible to everybody else, would not survive `index rebuild`,
// and would be a private note that looked like a corpus artifact. Committed, it travels
// on the same `git pull` that carries the claim, is reconstructible by a rebuild, and
// arrives as a diff on the document it contests, which is where a reviewer is already
// looking.
//
// That is the general rule stated in §10.7.4 and this is its clearest instance:
// **decisions are committed, observations are cached.** A challenge is a person saying
// something is wrong, which no rebuild can re-derive.
func (w *Writer) FileChallenge(rel string, c *gnosis.Challenge) (string, error) {
	const op = "bundle.Writer.FileChallenge"

	if err := w.held(op); err != nil {
		return "", err
	}
	full := filepath.Join(w.dir, filepath.FromSlash(path.Clean(rel)))
	existing, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", &errs.Error{
				Code: errs.ENOTFOUND, Message: op + ": no document at " + rel,
			}
		}
		return "", &errs.Error{Op: op, Err: err}
	}
	out, err := AppendChallenge(existing, c)
	if err != nil {
		return "", err
	}
	if err := w.WriteConcept(rel, out); err != nil {
		return "", err
	}
	return full, nil
}

// challenge files a reader's contest against a document (§10.7).
//
// Requires: cmd validated; w holds the lock when the command applies.
// Ensures: on apply, the document carries one more challenge and the trail carries a
// row; on preview, nothing is written and the outcome says what would be filed.
//
// **The identifier is minted here rather than by the caller**, because a challenge's
// id is an assigned identity like a document's (§5.1.3) and a caller supplying one
// could file two challenges under one id. It is a UUIDv7, so the list sorts into
// filing order without a separate index.
//
// **A preview writes no audit row**, which is the rule every command here follows: a
// preview is a read, and a mutation log that also holds reads is a log somebody stops
// reading.
func (c *Coordinator) challenge(
	_ context.Context, w *Writer, cmd *command.Challenge,
) (gnosis.Outcome, error) {
	const op = "bundle.Coordinator.challenge"

	filed := gnosis.Challenge{
		Class: cmd.Class, By: string(cmd.By), Rationale: cmd.Rationale,
		At: c.now().Format(time.RFC3339), State: gnosis.ChallengeOpen,
	}
	data := map[string]any{
		"path": cmd.Path, "effect": cmd.Eff.String(),
		"class": string(cmd.Class), "by": string(cmd.By),
	}
	if !cmd.Eff.Writes() {
		// The document is read and the append computed, so a preview reports the same
		// refusals an apply would — a challenge against a document that does not parse
		// is refused here rather than after somebody typed the rationale twice.
		if _, err := previewChallenge(c.Dir, cmd.Path, &filed); err != nil {
			return challengeRefused(op, cmd, err)
		}
		data["filed"] = false
		data["would_file"] = true
		return gnosis.OK(data), nil
	}

	id, err := gnosis.NewID()
	if err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	filed.ID = id
	if _, err = w.FileChallenge(cmd.Path, &filed); err != nil {
		return challengeRefused(op, cmd, err)
	}
	data["filed"] = true
	data["id"] = id.String()
	c.record(w, &audit.Row{
		At: c.now(), Op: audit.OpChallenge, Actor: string(cmd.By),
		Paths:   []string{cmd.Path},
		Outcome: string(gnosis.StatusOK),
		Detail: string(cmd.Class) + " challenge " + id.String() + ": " +
			cmd.Rationale,
	})
	return gnosis.OK(data), nil
}

// previewChallenge computes the document a challenge would produce, writing nothing.
//
// Requires: rel is a bundle-relative concept path.
// Ensures: the same error an apply would return, so a preview cannot pass where the
// write would fail. The bytes are discarded: a caller wanting the diff can run the
// apply against a clean tree, and returning a whole document in an outcome payload
// would put a page of markdown in a machine envelope.
func previewChallenge(bundleDir, rel string, c *gnosis.Challenge) ([]byte, error) {
	const op = "bundle.previewChallenge"

	full := filepath.Join(bundleDir, filepath.FromSlash(path.Clean(rel)))
	existing, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &errs.Error{
				Code: errs.ENOTFOUND, Message: op + ": no document at " + rel,
			}
		}
		return nil, &errs.Error{Op: op, Err: err}
	}
	return AppendChallenge(existing, c)
}

// challengeRefused turns a refusal into an outcome a caller can act on.
//
// A document that does not exist, or one whose frontmatter the append would damage,
// is `needs_human` rather than a tool failure: the corpus is intact, the challenge was
// not filed, and what to do next is a person's decision. An unexpected failure stays
// an error, because a caller must not be told to go and look at a disk fault.
func challengeRefused(
	op string, cmd *command.Challenge, err error,
) (gnosis.Outcome, error) {
	switch errs.ErrorCode(err) {
	case errs.ENOTFOUND, errs.EINVALID:
		return gnosis.Blocked(gnosis.ReasonNeedsHuman, err.Error(), map[string]any{
			"path": cmd.Path, "effect": cmd.Eff.String(), "class": string(cmd.Class),
		}), nil
	default:
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
}

// LoadChallenges reads every challenge the corpus carries (§10.7.4).
//
// Requires: bundleDir is a bundle root.
// Ensures: one entry per challenge, in document order and then declaration order;
// empty rather than nil for a corpus nobody has challenged. An error only when the
// walk itself fails — a document that will not parse contributes nothing rather than
// failing the listing, for the same reason `Load` returns it with Invalid set.
//
// It reads the bundle rather than the index because the challenges are committed and
// the index is a cache (§10.7.4): a reader asking what is outstanding must be told
// what the corpus holds, not what a rebuild last saw.
func LoadChallenges(bundleDir string) ([]FiledChallenge, error) {
	docs, err := Load(os.DirFS(bundleDir))
	if err != nil {
		return nil, err
	}
	out := make([]FiledChallenge, 0)
	for i := range docs {
		doc := &docs[i]
		for j := range doc.Challenges {
			out = append(out, FiledChallenge{
				Path: doc.Path, Challenge: doc.Challenges[j],
			})
		}
	}
	return out, nil
}

// CloseChallenge is the document a resolved challenge would produce (§10.7.3).
//
// Requires: existing is a concept's bytes; challengeID names a challenge the document
// carries.
// Ensures: the returned bytes parse, keep every top-level key, keep the body
// byte-identical, and record the named challenge as closed with no other challenge's
// state touched. ENOTFOUND when the document carries no such challenge. Pure.
//
// **Closed, never deleted.** A claim challenged three times and upheld three times is
// a different artifact from one never questioned, and only one of them has evidence
// that anybody looked. The reasoning is §10.6.5's reversal record pointed the other
// way: the disposition is the informative part, and deleting the challenge would leave
// the corpus unable to say the question had been raised.
func CloseChallenge(existing []byte, challengeID string) ([]byte, error) {
	const op = "bundle.CloseChallenge"

	before, err := okf.Parse(existing)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	lines := strings.Split(string(existing), "\n")
	end, err := frontmatterEnd(op, lines)
	if err != nil {
		return nil, err
	}
	block, ok := blockAt(lines, end, challengesKey)
	if !ok {
		return nil, &errs.Error{
			Code: errs.ENOTFOUND, Message: op + ": the document carries no challenges",
		}
	}
	start, marker, ok := entryStart(lines, block, end, challengeID)
	if !ok {
		return nil, &errs.Error{
			Code:    errs.ENOTFOUND,
			Message: op + ": the document carries no challenge " + challengeID,
		}
	}

	out := closedState(lines, start, entryFieldEnd(lines, start, end, marker), marker)
	joined := []byte(strings.Join(out, "\n"))
	if err := onlyThisChallengeClosed(op, before, joined, challengeID); err != nil {
		return nil, err
	}
	return joined, nil
}

// closedState rewrites the entry's `state:` line, or adds one when it has none.
//
// Requires: start is the entry's list-marker line; fieldEnd is one past its last
// field; marker is the marker's indentation.
// Ensures: a fresh slice — the input is not mutated, so a caller holding the original
// lines still has them. Pure.
func closedState(lines []string, start, fieldEnd int, marker string) []string {
	field := marker + "  "
	for i := start; i < fieldEnd; i++ {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "state:") {
			continue
		}
		out := make([]string, len(lines))
		copy(out, lines)
		out[i] = field + "state: " + string(gnosis.ChallengeClosed)
		return out
	}
	// A challenge filed by hand may declare no state, which reads as open. Adding the
	// key is what closing it means, and it goes at the end of the entry's fields
	// where every other key of that entry already is.
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:fieldEnd]...)
	out = append(out, field+"state: "+string(gnosis.ChallengeClosed))
	return append(out, lines[fieldEnd:]...)
}

// onlyThisChallengeClosed is the invariant, checked rather than asserted.
//
// Requires: before is the parsed original; out is the candidate; challengeID is the
// challenge being closed.
// Ensures: nil only when the document still parses, keeps every top-level key and its
// body, records that challenge as closed, and leaves every other challenge's state as
// it was. Pure.
//
// The last clause is the one worth checking: an entry boundary computed wrongly would
// close somebody else's challenge, and the document would parse and read as an ordinary
// resolution.
func onlyThisChallengeClosed(
	op string, before *okf.Document, out []byte, challengeID string,
) error {
	after, err := okf.Parse(out)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	if after.Body != before.Body {
		return &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": closing a challenge would change the document body," +
				" which it never does; nothing was written",
		}
	}
	for _, key := range topLevelKeys(before) {
		if _, ok := after.Fields[key]; !ok {
			return &errs.Error{
				Code: errs.EINVALID, Op: op,
				Message: op + ": closing a challenge would drop the frontmatter key " +
					key + "; nothing was written",
			}
		}
	}
	was := map[string]gnosis.ChallengeState{}
	previous := challengesOf(before)
	for i := range previous {
		was[previous[i].ID.String()] = previous[i].State
	}
	closed := false
	current := challengesOf(after)
	for i := range current {
		c := &current[i]
		id := c.ID.String()
		switch {
		case id == challengeID:
			closed = c.State == gnosis.ChallengeClosed
		case c.State != was[id]:
			return &errs.Error{
				Code: errs.EINVALID, Op: op,
				Message: op + ": closing " + challengeID + " would also change" +
					" challenge " + id + "; nothing was written",
			}
		}
	}
	if !closed {
		return &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": challenge " + challengeID +
				" was not recorded as closed; nothing was written",
		}
	}
	return nil
}
