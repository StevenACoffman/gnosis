package bundle

import (
	"context"
	"os"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/okf"
	"github.com/StevenACoffman/skillet/errs"
)

// statusKey is OKF §5.4's per-claim lifecycle field. `deprecated` means kept for links
// and history and no longer current — which is the whole of what supersession does to
// the losing claim.
const statusKey = "status"

// entryBounds is where one list entry starts and stops.
//
// Named for the entry rather than for the claim because `claimBounds` is already the
// function that reads a claim's parsed constraint, and two unrelated things under one
// name is what a reader has to disambiguate at every call site.
type entryBounds struct{ start, end int }

// Deprecate is the document a superseded claim would produce (§10.4).
//
// Requires: existing is a concept's bytes; claimID names a claim it declares.
// Ensures: the named claim carries `status: deprecated`, every top-level key survives,
// and the body is byte-identical. ENOTFOUND when no such claim exists; EINVALID when
// the claim is already deprecated, because superseding one twice records a second
// decision about a claim nobody was using. Pure.
//
// **Never a deletion.** OKF §5.4's word for this state is "kept for links and history",
// and §10.4 turns that into a property of the corpus: it can always answer what we
// believed in March and why we changed, which a delete makes impossible and a rewrite
// makes unverifiable.
func Deprecate(existing []byte, claimID string) ([]byte, error) {
	const op = "bundle.Deprecate"

	before, err := okf.Parse(existing)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	out, err := setClaimField(op, existing, claimID, statusKey, "deprecated")
	if err != nil {
		return nil, err
	}
	if err := sameDocumentDeprecating(op, before, out, claimID); err != nil {
		return nil, err
	}
	return out, nil
}

// Supersedes is the document a winning claim would produce (§10.4).
//
// Requires: existing is a concept's bytes; claimID names a claim it declares; loser is
// the identifier of the claim being replaced.
// Ensures: the named claim's `gnosis_supersedes` list gains the loser, every top-level
// key survives, and the body is byte-identical. ENOTFOUND when no such claim exists.
// Pure.
//
// **The edge and the warrant are separate writes, and `warrant`'s check is what joins
// them.** A claim carrying a supersession and no `gnosis_warrant` is reported, which is
// how a supersession recorded without its reasoning becomes visible rather than
// silently acceptable — so this function does not require a warrant and does not write
// one.
func Supersedes(existing []byte, claimID, loser string) ([]byte, error) {
	const op = "bundle.Supersedes"

	before, err := okf.Parse(existing)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	out, err := appendClaimListItem(op, existing, claimID, supersedesKey, loser)
	if err != nil {
		return nil, err
	}
	if err := sameDocumentSuperseding(op, before, out, claimID, loser); err != nil {
		return nil, err
	}
	return out, nil
}

// setClaimField writes `key: value` into a claim's entry, replacing an existing line.
//
// Requires: existing parses; claimID names a declared claim.
// Ensures: ENOTFOUND when no entry carries the id. Pure.
func setClaimField(op string, existing []byte, claimID, key, value string) ([]byte, error) {
	lines, at, indent, err := claimEntry(op, existing, claimID)
	if err != nil {
		return nil, err
	}
	line := indent + key + ": " + value
	for i := at.start; i < at.end; i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), key+":") {
			out := make([]string, len(lines))
			copy(out, lines)
			out[i] = line
			return []byte(strings.Join(out, "\n")), nil
		}
	}
	return spliced(lines, at.end, []string{line}), nil
}

// appendClaimListItem adds one item to a claim's list-valued field, creating the field
// when the claim has none.
//
// Requires: existing parses; claimID names a declared claim.
// Ensures: ENOTFOUND when no entry carries the id; the item is appended rather than
// prepended, so the list reads in the order the decisions were made. Pure.
func appendClaimListItem(
	op string, existing []byte, claimID, key, item string,
) ([]byte, error) {
	lines, at, indent, err := claimEntry(op, existing, claimID)
	if err != nil {
		return nil, err
	}
	for i := at.start; i < at.end; i++ {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), key+":") {
			continue
		}
		// The field exists: its items run until the next line at the field's own
		// indentation, which is the same rule every other block here follows.
		last := i
		for j := i + 1; j < at.end; j++ {
			if !strings.HasPrefix(lines[j], indent+"  ") {
				break
			}
			last = j
		}
		return spliced(lines, last+1, []string{indent + "  - " + item}), nil
	}
	return spliced(lines, at.end, []string{indent + key + ":", indent + "  - " + item}), nil
}

// claimEntry locates a claim's entry and the indentation of its fields.
//
// Requires: existing parses as a concept.
// Ensures: ENOTFOUND naming the claim when the document declares no such entry. Pure.
func claimEntry(
	op string, existing []byte, claimID string,
) ([]string, entryBounds, string, error) {
	lines := strings.Split(string(existing), "\n")
	fence, err := frontmatterEnd(op, lines)
	if err != nil {
		return nil, entryBounds{}, "", err
	}
	block, ok := blockAt(lines, fence, claimsKey)
	if !ok {
		return nil, entryBounds{}, "", &errs.Error{
			Code: errs.ENOTFOUND, Message: op + ": the document declares no claims",
		}
	}
	start, marker, ok := entryStart(lines, block, fence, claimID)
	if !ok {
		return nil, entryBounds{}, "", &errs.Error{
			Code:    errs.ENOTFOUND,
			Message: op + ": the document declares no claim " + claimID,
		}
	}
	end := entryFieldEnd(lines, start, fence, marker)
	return lines, entryBounds{start: start, end: end}, marker + "  ", nil
}

// spliced inserts lines at an index, leaving the input untouched.
func spliced(lines []string, at int, insert []string) []byte {
	out := make([]string, 0, len(lines)+len(insert))
	out = append(out, lines[:at]...)
	out = append(out, insert...)
	out = append(out, lines[at:]...)
	return []byte(strings.Join(out, "\n"))
}

// sameDocumentDeprecating is the invariant for a deprecation, checked rather than
// asserted.
//
// Ensures: nil only when the document parses, keeps every top-level key and its body,
// and marks the named claim deprecated. Pure.
func sameDocumentDeprecating(
	op string, before *okf.Document, out []byte, claimID string,
) error {
	after, err := unchangedApartFrom(op, before, out, "deprecating")
	if err != nil {
		return err
	}
	claims := docClaims(after)
	for i := range claims {
		if claims[i].ID == claimID && claims[i].Status == "deprecated" {
			return nil
		}
	}
	return &errs.Error{
		Code: errs.EINVALID, Op: op,
		Message: op + ": claim " + claimID +
			" was not marked deprecated; nothing was written",
	}
}

// sameDocumentSuperseding is the invariant for a supersession edge.
//
// Ensures: nil only when the document parses, keeps every top-level key and its body,
// and records the loser on the named claim. Pure.
func sameDocumentSuperseding(
	op string, before *okf.Document, out []byte, claimID, loser string,
) error {
	after, err := unchangedApartFrom(op, before, out, "superseding")
	if err != nil {
		return err
	}
	claims := docClaims(after)
	for i := range claims {
		if claims[i].ID != claimID {
			continue
		}
		for _, s := range claims[i].Supersedes {
			if s == loser {
				return nil
			}
		}
	}
	return &errs.Error{
		Code: errs.EINVALID, Op: op,
		Message: op + ": claim " + claimID + " does not record superseding " + loser +
			"; nothing was written",
	}
}

// unchangedApartFrom re-parses a candidate and checks what every surgery here must
// preserve: the body, and every top-level key.
//
// Requires: verb names the operation, for the message.
// Ensures: the parsed document, or EINVALID naming which property failed. Pure.
//
// It is shared by both supersession writes because the two properties are the same two
// in both cases, and a second copy would be a second place for one of them to be
// dropped — which is the failure mode the checks exist for.
func unchangedApartFrom(
	op string, before *okf.Document, out []byte, verb string,
) (*okf.Document, error) {
	after, err := okf.Parse(out)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	if after.Body != before.Body {
		return nil, &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": " + verb + " would change the document body, which it" +
				" never does; nothing was written",
		}
	}
	for _, key := range topLevelKeys(before) {
		if _, ok := after.Fields[key]; !ok {
			return nil, &errs.Error{
				Code: errs.EINVALID, Op: op,
				Message: op + ": " + verb + " would drop the frontmatter key " + key +
					"; nothing was written",
			}
		}
	}
	return after, nil
}

// loserRef is how the winning claim names the claim it replaced.
//
// Requires: loser is the losing document's bytes, so its identifier can be read from
// the thing being superseded rather than from the caller's argument.
// Ensures: `<gnosis_id>#<claim>`, or EINVALID when the losing document declares no
// identifier.
//
// **`<gnosis_id>#<claim>`, and it was `<path>#<claim>` for one day.** A claim identifier
// is document-local — `c1` means whatever the page it sits on says it means — so an edge
// carrying the identifier alone is unreadable the moment the two claims are on different
// pages, which is the ordinary case; that much the first version got right. What it got
// wrong is the left half. §5.4 requires these edges to name "**identifiers, never
// paths**… an edge that survives reorganization is the point", and a path carries the
// slug, which §5.1.1 changes whenever somebody retitles the concept. The edge would then
// name a file that does not exist, pointing at the one claim nobody is looking at any
// more.
//
// A reader still sees a path: §5.6 makes the presented path a view computed from the
// index, resolving *to* the canonical form and never the reverse.
//
// **A document with no identifier is refused rather than referenced by path.** §5.1.2
// quarantines an unidentified document precisely because nothing durable can point at
// one, and writing an edge to it would be committing a reference that can never resolve.
func loserRef(op string, loser []byte, claimID string) (string, error) {
	doc, err := okf.Parse(loser)
	if err != nil {
		return "", &errs.Error{Code: errs.EINVALID, Op: op, Err: err}
	}
	raw, ok := doc.Text(idKey)
	if !ok {
		return "", &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": the losing document declares no " + idKey + ", so nothing" +
				" durable can point at it (§5.1.2); `gnosis index rebuild` reports it",
		}
	}
	id, err := gnosis.ParseID(raw)
	if err != nil {
		return "", &errs.Error{Code: errs.EINVALID, Op: op, Err: err}
	}
	return gnosis.ClaimRef(id, claimID), nil
}

// supersede records that one claim replaced another (§10.4).
//
// Requires: cmd validated; w holds the lock when the command applies.
// Ensures: on apply, the winner records the edge and the loser is deprecated; on
// preview, nothing is written and every refusal an apply would reach is reported.
//
// **The winner's edge is written first, and the order is the safe one.** A crash
// between the two writes leaves an edge pointing at a claim still marked current —
// visible, repairable by running the command again, and already reported by `warrant`
// if no decision was recorded. The reverse leaves a deprecated claim nothing supersedes,
// which reads as a claim somebody abandoned and cannot be told from one.
//
// **An episodic claim is refused**, and it is a consequence of the type rather than a
// rule about supersession: §5.8.3.1 makes an episodic type's claims ineligible for
// conflict detection, because two reports of different moments cannot contradict. A
// corpus adjudicating "we set it to 3 in March" against "we set it to 5 in June" would
// be adjudicating its own history.
func (c *Coordinator) supersede(
	_ context.Context, w *Writer, cmd *command.Supersede,
) (gnosis.Outcome, error) {
	const op = "bundle.Coordinator.supersede"

	data := map[string]any{
		"loser":  cmd.LoserPath + " " + cmd.LoserClaim,
		"winner": cmd.WinnerPath + " " + cmd.WinnerClaim,
		"effect": cmd.Eff.String(), "by": string(cmd.By),
	}
	loser, err := readConcept(op, c.Dir, cmd.LoserPath)
	if err != nil {
		return supersedeRefused(data, err)
	}
	if why := episodicRefusal(c.Dir, loser); why != "" {
		return gnosis.Blocked(gnosis.ReasonNeedsHuman, why, data), nil
	}
	deprecated, err := Deprecate(loser, cmd.LoserClaim)
	if err != nil {
		return supersedeRefused(data, err)
	}

	winner := deprecated
	if cmd.WinnerPath != cmd.LoserPath {
		if winner, err = readConcept(op, c.Dir, cmd.WinnerPath); err != nil {
			return supersedeRefused(data, err)
		}
	}
	ref, err := loserRef(op, loser, cmd.LoserClaim)
	if err != nil {
		return supersedeRefused(data, err)
	}
	winner, err = Supersedes(winner, cmd.WinnerClaim, ref)
	if err != nil {
		return supersedeRefused(data, err)
	}
	data["loser_ref"] = ref

	if !cmd.Eff.Writes() {
		data["superseded"] = false
		data["would_supersede"] = true
		return gnosis.OK(data), nil
	}
	if wErr := c.writeSupersession(w, cmd, deprecated, winner); wErr != nil {
		return gnosis.Outcome{}, wErr
	}
	data["superseded"] = true
	c.record(w, &audit.Row{
		At: c.now(), Op: audit.OpSupersede, Actor: string(cmd.By),
		Paths:   []string{cmd.WinnerPath, cmd.LoserPath},
		Outcome: string(gnosis.StatusOK),
		Detail: cmd.WinnerClaim + " supersedes " + ref +
			"; the losing claim is deprecated and kept",
	})
	return gnosis.OK(data), nil
}

// writeSupersession performs the two writes in the safe order.
//
// Requires: w holds the lock; deprecated and winner are the rendered documents.
// Ensures: one write when both claims live in one document, two otherwise, with the
// winner's edge first.
func (c *Coordinator) writeSupersession(
	w *Writer, cmd *command.Supersede, deprecated, winner []byte,
) error {
	const op = "bundle.Coordinator.writeSupersession"

	if cmd.WinnerPath == cmd.LoserPath {
		// One document holds both claims, so the winner's bytes already carry the
		// deprecation — one write, and no order to get wrong.
		if err := w.WriteConcept(cmd.WinnerPath, winner); err != nil {
			return &errs.Error{Op: op, Err: err}
		}
		return nil
	}
	if err := w.WriteConcept(cmd.WinnerPath, winner); err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	if err := w.WriteConcept(cmd.LoserPath, deprecated); err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	return nil
}

// episodicRefusal reports why an episodic claim may not be superseded, or "".
//
// Requires: bundleDir is a bundle root; loser is the losing document's bytes.
// Ensures: "" when the vocabulary does not declare the document's type episodic, which
// includes a bundle whose ontology will not load — a corpus with a broken vocabulary is
// `doctor`'s finding, and refusing a supersession over it would be a second report of
// one problem.
func episodicRefusal(bundleDir string, loser []byte) string {
	doc, err := okf.Parse(loser)
	if err != nil {
		return ""
	}
	vocab := vocabulary(os.DirFS(bundleDir))
	declared, ok := vocab.TypeNamed(gnosis.TypeKey(doc.Type()))
	if !ok || !declared.Episodic {
		return ""
	}
	return "the losing claim is of the episodic type " + doc.Type() +
		", whose claims cannot contradict each other (§5.8.3.1): two reports of" +
		" different moments are both true, and superseding one would be the corpus" +
		" adjudicating its own history"
}

// supersedeRefused turns a refusal into an outcome a caller can act on.
func supersedeRefused(data map[string]any, err error) (gnosis.Outcome, error) {
	const op = "bundle.Coordinator.supersede"
	switch errs.ErrorCode(err) {
	case errs.ENOTFOUND, errs.EINVALID:
		return gnosis.Blocked(gnosis.ReasonNeedsHuman, err.Error(), data), nil
	default:
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
}
