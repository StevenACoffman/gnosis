package bundle

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/okf"
	"github.com/StevenACoffman/skillet/errs"
)

// conflictIndent is the indentation of an entry's fields, matching `gnosis_challenges`.
//
// The same depth deliberately: two families rendered at different indentations in one
// frontmatter block reads as one of them being wrong, and a reviewer looking at the diff
// should not have to decide which.
const conflictIndent = "    "

// AppendConflict records a deferred contradiction on a document (§5.4).
//
// Requires: existing parses as a concept; e is valid.
// Ensures: the same document plus one entry — byte-identical body, every top-level key
// kept, and exactly one more conflict — or an error naming which of those failed. Pure.
//
// # Why the guard is checked rather than asserted
//
// The insertion is surgery on frontmatter text, because §5.2 forbids re-encoding: an
// encoder normalises quoting and key order and drops comments, so a round trip through
// one would not reproduce what the author wrote. Surgery can drop a key, move the body,
// or write into the wrong block — and the last of those looks like success. The warrant
// and the challenge both carry this guard, and it caught a real defect for each.
func AppendConflict(existing []byte, e *gnosis.ConflictEdge) ([]byte, error) {
	const op = "bundle.AppendConflict"

	before, err := okf.Parse(existing)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	if !e.Valid() {
		return nil, &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": a deferral needs the concept, the finding, who deferred" +
				" it, when, and why — §17.0 asks for all three of the last, because a" +
				" conflict that stopped being reported without them went quiet rather" +
				" than being decided about",
		}
	}
	if already(before, e.Finding) {
		// Refused rather than appended twice. A second entry for one finding would
		// leave two deferrals of one conflict with two reasons, and no way to say
		// which is current.
		return nil, &errs.Error{
			Code:    errs.ECONFLICT,
			Message: op + ": conflict " + e.Finding + " is already deferred here",
		}
	}

	out, err := insertConflict(op, existing, e)
	if err != nil {
		return nil, err
	}
	if err := sameDocumentPlusOneConflict(op, before, out); err != nil {
		return nil, err
	}
	return out, nil
}

// already reports whether the document already defers this finding.
func already(doc *okf.Document, finding string) bool {
	for _, edge := range conflictsOf(doc) {
		if edge.Finding == finding {
			return true
		}
	}
	return false
}

// insertConflict writes the entry into the frontmatter text.
//
// Appended rather than prepended, as a challenge is: the list then reads in the order
// the deferrals were made, which is the order a reviewer reading the diff expects.
func insertConflict(op string, existing []byte, e *gnosis.ConflictEdge) ([]byte, error) {
	lines := strings.Split(string(existing), "\n")
	end, err := frontmatterEnd(op, lines)
	if err != nil {
		return nil, err
	}
	entry := renderConflict(e)

	at, found := listEnd(lines, end, conflictsKey)
	if !found {
		at = end
		entry = append([]string{conflictsKey + ":"}, entry...)
	}
	out := make([]string, 0, len(lines)+len(entry))
	out = append(out, lines[:at]...)
	out = append(out, entry...)
	out = append(out, lines[at:]...)
	return []byte(strings.Join(out, "\n")), nil
}

// renderConflict is one `gnosis_conflicts` entry, as lines.
//
// Requires: e is valid.
// Ensures: deterministic — no clock and no map iteration, so a preview and an apply
// produce the same bytes. Pure.
//
// The reason is a block scalar for `renderChallenge`'s reason: it is prose a person wrote
// and may carry a colon, a quote or a newline, any of which would need escaping inline.
func renderConflict(e *gnosis.ConflictEdge) []string {
	out := []string{
		"  - concept: " + e.Concept.String(),
		conflictIndent + "finding: " + e.Finding,
		conflictIndent + "state: " + string(e.State),
		conflictIndent + "by: " + e.By,
		conflictIndent + "at: " + e.At,
		conflictIndent + "reason: |",
	}
	for _, line := range strings.Split(strings.TrimRight(e.Reason, "\n"), "\n") {
		out = append(out, conflictIndent+"  "+line)
	}
	return out
}

// sameDocumentPlusOneConflict is the invariant, checked rather than asserted.
//
// Every clause is here because the surgery could break it and nothing else would notice:
// a dropped key is what re-rendering would cause, a changed body is what a mis-placed
// insertion would cause, and a count that did not rise is what writing into the wrong
// block would cause — which would otherwise look like success.
func sameDocumentPlusOneConflict(op string, before *okf.Document, out []byte) error {
	after, err := okf.Parse(out)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	if after.Body != before.Body {
		return &errs.Error{
			Code: errs.EINVALID, Message: op + ": the body changed",
		}
	}
	for _, key := range topLevelKeys(before) {
		if _, ok := after.Fields[key]; !ok {
			return &errs.Error{
				Code: errs.EINVALID, Message: op + ": the key " + key + " was dropped",
			}
		}
	}
	if len(conflictsOf(after)) != len(conflictsOf(before))+1 {
		return &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": the document does not hold exactly one more conflict," +
				" so the entry went somewhere other than the conflicts block",
		}
	}
	return nil
}

// deferConflict records a person's decision to live with a contradiction (§17.0).
//
// Requires: cmd validated; w holds the lock when the command applies.
// Ensures: on apply, the document carries one more conflict edge and the trail carries a
// row; on preview, nothing is written and the outcome says what would be recorded.
//
// **A preview computes the append and discards it**, which is `challenge`'s rule: a
// deferral against a document that does not parse, or one already deferred, is refused
// before somebody types the reason twice.
//
// **A preview writes no audit row.** A preview is a read, and a mutation log that also
// holds reads is a log somebody stops reading.
func (c *Coordinator) deferConflict(
	_ context.Context, w *Writer, cmd *command.Defer,
) (gnosis.Outcome, error) {
	const op = "bundle.Coordinator.deferConflict"

	edge := gnosis.ConflictEdge{
		Concept: cmd.Concept, Finding: cmd.Finding, State: gnosis.ConflictDeferred,
		By: string(cmd.By), At: c.now().Format(time.RFC3339), Reason: cmd.Reason,
	}
	data := map[string]any{
		"path": cmd.Path, "effect": cmd.Eff.String(),
		"finding": cmd.Finding, "concept": cmd.Concept.String(), "by": string(cmd.By),
	}
	existing, err := readConcept(op, c.Dir, cmd.Path)
	if err != nil {
		return deferRefused(op, cmd, err)
	}
	if _, err = AppendConflict(existing, &edge); err != nil {
		return deferRefused(op, cmd, err)
	}
	if !cmd.Eff.Writes() {
		data["deferred"] = false
		data["would_defer"] = true
		return gnosis.OK(data), nil
	}
	if _, err = w.FileConflict(cmd.Path, &edge); err != nil {
		return deferRefused(op, cmd, err)
	}
	data["deferred"] = true
	c.record(w, &audit.Row{
		At: c.now(), Op: audit.OpDefer, Actor: string(cmd.By),
		Paths:   []string{cmd.Path},
		Outcome: string(gnosis.StatusOK),
		Detail:  "deferred conflict " + cmd.Finding + ": " + cmd.Reason,
	})
	return gnosis.OK(data), nil
}

// FileConflict writes a deferral onto a document.
//
// Requires: the writer holds the lock; rel is a bundle-relative concept path.
// Ensures: the document carries one more conflict edge, written atomically, or an error
// naming why not. ENOTFOUND when the path holds no document.
func (w *Writer) FileConflict(rel string, e *gnosis.ConflictEdge) (string, error) {
	const op = "bundle.Writer.FileConflict"

	if err := w.held(op); err != nil {
		return "", err
	}
	existing, err := readConcept(op, w.dir, rel)
	if err != nil {
		return "", err
	}
	out, err := AppendConflict(existing, e)
	if err != nil {
		return "", err
	}
	if wErr := w.WriteConcept(rel, out); wErr != nil {
		return "", wErr
	}
	return filepath.Join(w.dir, filepath.FromSlash(rel)), nil
}

// deferRefused reports a deferral that could not be recorded.
//
// **Blocked rather than an error for an EINVALID or an ECONFLICT**, which is the shape
// every refusal in this package takes: the corpus is intact, the caller's request was
// not something it could act on, and a person changing the invocation fixes it.
func deferRefused(op string, cmd *command.Defer, cause error) (gnosis.Outcome, error) {
	code := errs.ErrorCode(cause)
	if code != errs.EINVALID && code != errs.ECONFLICT && code != errs.ENOTFOUND {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: cause}
	}
	return gnosis.Blocked(gnosis.ReasonNeedsHuman, cause.Error(), map[string]any{
		"path": cmd.Path, "finding": cmd.Finding, "deferred": false,
	}), nil
}

// SubjectOwners maps each declared subject key to who is accountable for it (§10.6.2.1).
//
// Requires: fsys is a bundle's filesystem.
// Ensures: an entry only for a subject that declares an owner; empty for a corpus with
// no vocabulary or none declared. Never an error — a corpus whose ontology will not parse
// has that reported by `vocabulary`, and a review queue that refused to render because
// of it would take the queue down for a fault it is not about.
//
// **This is deliberately not on the lint snapshot.** §10.6.2.1's third reason for
// refusing a `role` on the warrant is that a warrant field is inside what the gate reads,
// so only a comment would keep it unread — whereas the ontology is in neither
// `gate.Candidate` nor `gate.Corpus`, and reading a subject's owner would require
// visibly widening the gate's inputs. Keeping the owner off the snapshot is what makes
// that argument true rather than a promise: no check can reach it.
func SubjectOwners(fsys fs.FS) map[gnosis.SubjectKey]string {
	o := ontologyFrom(fsys)
	if o == nil {
		return map[gnosis.SubjectKey]string{}
	}
	out := map[gnosis.SubjectKey]string{}
	for i := range o.Subjects {
		if owner := strings.TrimSpace(o.Subjects[i].Owner); owner != "" {
			out[o.Subjects[i].Key] = owner
		}
	}
	return out
}

// Adjudications is every recorded decision in the corpus, for §10.6.2's domain history.
//
// Requires: docs came from Load.
// Ensures: one entry per claim carrying a warrant with an adjudicator, in corpus order.
// Pure.
//
// **It reads the warrant and the claim's subject and nothing else**, which is §10.6.2's
// own description of the query: "domain history is computed — it is a query over
// `gnosis_warrant` and a claim's `subject`, needing no roster". The absent roster is the
// point rather than a simplification.
//
// **The surface is resolved rather than counted as written**, and `resolve` is a
// parameter so this stays pure. A claim's `subject` is the phrase its author typed
// (§5.5.1 keeps it that way so an unresolvable one is a finding), and §5.8.2's aliases
// mean "retries" and "retry budget" are one subject — counting the strings would split
// one person's history across the spellings their colleagues happened to use.
func Adjudications(
	docs []Document, resolve func(string) (gnosis.SubjectKey, bool),
) []gnosis.Adjudicated {
	var out []gnosis.Adjudicated
	for i := range docs {
		claims := docs[i].Claims
		for j := range claims {
			if strings.TrimSpace(claims[j].Warrant.By) == "" {
				continue
			}
			key, ok := resolve(claims[j].Subject)
			if !ok {
				// A decision about something the vocabulary cannot resolve counts
				// toward no domain. Dropping it is right: the alternative buckets it
				// under a surface phrase, and §10.6.2's count is about a domain
				// rather than about a wording.
				continue
			}
			out = append(out, gnosis.Adjudicated{By: claims[j].Warrant.By, Subject: key})
		}
	}
	return out
}

// DeclaredTerms maps every declared subject surface to what the vocabulary says it means.
//
// Requires: fsys is a bundle's filesystem.
// Ensures: an entry per surface — key and alias alike — for every subject carrying a
// description; empty for a corpus with no vocabulary. Never an error, for
// `SubjectOwners`'s reason: a viewer that refused to render a page because the ontology
// will not parse would take the corpus offline for a fault `vocabulary` already reports.
//
// **Aliases are included, and that is the point of surfacing definitions at all.** §5.8.2
// makes an alias the phrase an author actually writes, so a page that used "retry budget"
// and a glossary that only knew `retry.max_attempts` would define a term the reader
// cannot see and stay silent about the one they can.
func DeclaredTerms(fsys fs.FS) map[string]string {
	o := ontologyFrom(fsys)
	if o == nil {
		return nil
	}
	out := map[string]string{}
	for i := range o.Subjects {
		sub := &o.Subjects[i]
		if strings.TrimSpace(sub.Desc) == "" {
			continue
		}
		out[sub.Key.String()] = sub.Desc
		for _, alias := range sub.Aliases {
			out[alias] = sub.Desc
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
