package bundle

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/okf"
	"github.com/StevenACoffman/skillet/errs"
)

// adjudication is what recording one decision needs, gathered.
//
// A struct rather than six parameters, because they belong together and because the
// alternative reads as a signature nobody can call correctly: two of them are byte
// slices that differ only in which side of the write they are on, which is exactly the
// pair a positional argument list gets wrong.
type adjudication struct {
	before    []byte
	after     []byte
	authority gnosis.Authority
	signers   map[string]bool
	data      map[string]any
}

// Reversal is one warrant that overturned another (§10.6.5).
//
// It carries both the reasoning and the identifier it reverses, because the pair is the
// answer: a reader asking what the corpus changed its mind about needs the reason, and a
// reader tracing a decision needs the link.
type Reversal struct {
	Path    string `json:"path"`
	ClaimID string `json:"claim"`

	// Reverses is the warrant overturned, never the claim. §10.6.5 is explicit about
	// that, and the distinction is what lets a claim be reversed twice and still say
	// which reasoning fell first.
	Reverses string `json:"reverses"`

	By        string `json:"by"`
	At        string `json:"at"`
	Rationale string `json:"rationale"`
}

// AppendWarrant is the document an adjudication would produce (§10.4, §10.6.4).
//
// Requires: existing is a concept's bytes; claimID names a claim the document
// declares; w carries an adjudicator and a non-empty rationale.
// Ensures: the returned bytes parse, hold every top-level key the original held, carry
// a byte-identical body, and carry the warrant on the named claim and on no other.
// ENOTFOUND when the document declares no such claim; EINVALID when the claim already
// carries a warrant, because overwriting one would erase a decision. Pure.
//
// **Textual surgery for the reason AppendChallenge is**, and more so: the documents
// an adjudication touches are the ones people write by hand, and re-rendering through
// `renderConcept` would drop every key `conceptDoc` does not model. The properties
// above are checked against the re-parsed output rather than promised.
//
// **An existing warrant is never overwritten.** §10.6.5 makes a reversal a *new*
// warrant naming the one it overturns, which is the only way a corpus can answer what
// it believed in March and why it changed. Silently replacing one would delete exactly
// the record that makes reversal informative.
func AppendWarrant(existing []byte, claimID string, w *gnosis.Warrant) ([]byte, error) {
	const op = "bundle.AppendWarrant"

	before, err := okf.Parse(existing)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	if strings.TrimSpace(w.Rationale) == "" {
		return nil, &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": a warrant needs a rationale (§10.6.4), at every" +
				" authority including sole",
		}
	}
	declared := docClaims(before)
	for i := range declared {
		if declared[i].ID == claimID && declared[i].Warrant.Adjudicated() {
			return nil, &errs.Error{
				Code: errs.EINVALID,
				Message: op + ": claim " + claimID + " already carries a warrant;" +
					" a reversal is a new warrant naming the one it overturns" +
					" (§10.6.5), never an edit of it",
			}
		}
	}

	out, err := insertWarrant(op, existing, claimID, w)
	if err != nil {
		return nil, err
	}
	if err := sameDocumentPlusOneWarrant(op, before, out, claimID); err != nil {
		return nil, err
	}
	return out, nil
}

// insertWarrant writes the block into the named claim's entry.
//
// Requires: existing parses; claimID names a declared claim.
// Ensures: the block sits at the end of that claim's fields, indented to match them;
// ENOTFOUND when no entry carries the id. Pure.
func insertWarrant(
	op string, existing []byte, claimID string, w *gnosis.Warrant,
) ([]byte, error) {
	lines := strings.Split(string(existing), "\n")
	end, err := frontmatterEnd(op, lines)
	if err != nil {
		return nil, err
	}
	at, indent, found := claimEntryEnd(lines, end, claimID)
	if !found {
		return nil, &errs.Error{
			Code:    errs.ENOTFOUND,
			Message: op + ": the document declares no claim " + claimID,
		}
	}
	entry := renderWarrant(indent, w)

	out := make([]string, 0, len(lines)+len(entry))
	out = append(out, lines[:at]...)
	out = append(out, entry...)
	out = append(out, lines[at:]...)
	return []byte(strings.Join(out, "\n")), nil
}

// claimEntryEnd is the index just past the named claim's last field, the indentation
// its fields use, and whether the claim was found.
//
// Requires: end is the index of the frontmatter's closing fence.
// Ensures: an index inside the frontmatter when found. Pure.
//
// **The id is matched on the entry's own `id:` field**, not on position: §5.5.1 makes
// a claim addressable by identity precisely so nothing has to count list items, and an
// entry's index changes whenever somebody adds one above it.
//
// It is three questions and three functions, which the complexity limit is what
// prompted and reading it is what settles: where the block is, which entry is the
// claim, and where that entry stops. The middle one is the only one that knows about
// identifiers.
func claimEntryEnd(lines []string, end int, claimID string) (int, string, bool) {
	block, ok := blockAt(lines, end, claimsKey)
	if !ok {
		return 0, "", false
	}
	start, marker, ok := entryStart(lines, block, end, claimID)
	if !ok {
		return 0, "", false
	}
	return entryFieldEnd(lines, start, end, marker), marker + "  ", true
}

// blockAt is the index of a top-level key's line inside the frontmatter.
//
// Requires: end is the closing fence's index.
// Ensures: the index and true, or (0, false) when the frontmatter has no such key.
// Pure.
func blockAt(lines []string, end int, key string) (int, bool) {
	for i := 1; i < end; i++ {
		if strings.HasPrefix(lines[i], key+":") {
			return i, true
		}
	}
	return 0, false
}

// entryStart is the index of the list-marker line opening the entry with this id, and
// the indentation that marker sits at.
//
// Requires: block is the index of the list's own key line.
// Ensures: the index and true, or (0, "", false) when no entry declares the id. The
// scan stops at the next top-level key, so it never reads into a sibling block. Pure.
//
// It is not specific to claims: `gnosis_claims` and `gnosis_challenges` are both lists
// of mappings whose first field is an id, and one function for both is what stops the
// two surgeries drifting apart in how they find an entry.
func entryStart(lines []string, block, end int, id string) (int, string, bool) {
	for i := block + 1; i < end; i++ {
		if !indented(lines[i]) {
			return 0, "", false
		}
		marker := listMarker(lines[i])
		if marker != "" && entryDeclares(lines[i], id) {
			return i, marker, true
		}
	}
	return 0, "", false
}

// entryFieldEnd is the index just past the last field of the entry opened at start.
//
// Requires: marker is the indentation the entry's list marker sits at.
// Ensures: an index in (start, end]. Pure.
//
// An entry ends where the next list marker at the same indentation begins, or where the
// block does — which is the whole of the YAML this has to understand, and the same rule
// listEnd uses one key over.
func entryFieldEnd(lines []string, start, end int, marker string) int {
	last := start
	for i := start + 1; i < end; i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if !strings.HasPrefix(lines[i], marker+"  ") {
			break
		}
		last = i
	}
	return last + 1
}

// indented reports whether a line sits inside a block rather than opening a new
// top-level key.
func indented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

// listMarker is the indentation before a YAML list marker, or "" when the line is not
// one. `  - id: c1` yields two spaces, which is what its fields are indented past.
func listMarker(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "- ") {
		return ""
	}
	return line[:len(line)-len(trimmed)]
}

// entryDeclares reports whether a list-marker line opens the claim with this id.
//
// It reads the id off the marker line itself, which is where renderConcept writes it
// and where every document in this corpus carries it. An entry whose `id:` sits on a
// later line is not matched, and that is a limit worth stating rather than guessing
// around: guessing would mean scanning forward for an `id:` and risking the *next*
// entry's, which is how a warrant lands on the wrong claim.
func entryDeclares(line, claimID string) bool {
	rest := strings.TrimSpace(strings.TrimLeft(line, " \t")[len("- "):])
	value, ok := strings.CutPrefix(rest, "id:")
	return ok && strings.TrimSpace(value) == claimID
}

// renderWarrant is the `gnosis_warrant` block, as lines at the given indent.
//
// Requires: w carries a non-empty rationale; indent is the claim's field indentation.
// Ensures: deterministic — no clock and no map iteration, so a preview and an apply
// produce the same bytes. Optional fields are omitted rather than written empty: an
// empty `co_signed_by:` asserts that nobody co-signed, which is a different thing from
// a decision that needed no co-signer. Pure.
func renderWarrant(indent string, w *gnosis.Warrant) []string {
	out := []string{indent + warrantKey + ":"}
	field := indent + "  "
	for _, kv := range [][2]string{
		{"by", w.By},
		{"at", w.At},
		{"tier", w.Authority},
		{"review", w.Review},
		{"reverses", w.Reverses},
		{"co_signed_by", w.CoSignedBy},
	} {
		if kv[1] != "" {
			out = append(out, field+kv[0]+": "+kv[1])
		}
	}
	out = append(out, field+"rationale: |")
	for _, line := range strings.Split(strings.TrimRight(w.Rationale, "\n"), "\n") {
		out = append(out, field+"  "+line)
	}
	if w.OverrideReason != "" {
		out = append(out, field+"override:", field+"  reason: |")
		for _, line := range strings.Split(
			strings.TrimRight(w.OverrideReason, "\n"), "\n",
		) {
			out = append(out, field+"    "+line)
		}
	}
	return out
}

// sameDocumentPlusOneWarrant is the invariant, checked rather than asserted.
//
// Requires: before is the parsed original; out is the candidate; claimID is the claim
// the warrant was for.
// Ensures: nil only when out parses, keeps every top-level key, keeps the body
// byte-identical, carries the warrant on the named claim, and adds it to no other.
// Pure.
//
// The last clause is the one the surgery could plausibly break: an insertion point
// computed from the wrong claim entry would land the block on a neighbour, and the
// document would parse, keep its keys and its body, and record a decision about the
// wrong assertion.
func sameDocumentPlusOneWarrant(
	op string, before *okf.Document, out []byte, claimID string,
) error {
	after, err := okf.Parse(out)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	if after.Body != before.Body {
		return &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": adjudicating would change the document body, which an" +
				" adjudication never does; nothing was written",
		}
	}
	for _, key := range topLevelKeys(before) {
		if _, ok := after.Fields[key]; !ok {
			return &errs.Error{
				Code: errs.EINVALID, Op: op,
				Message: op + ": adjudicating would drop the frontmatter key " + key +
					"; nothing was written",
			}
		}
	}
	return landedOnlyOn(op, before, after, claimID)
}

// landedOnlyOn reports whether the warrant reached the named claim and no other.
//
// Requires: before and after are the parsed documents; claimID is the claim the
// warrant was for.
// Ensures: nil only when the named claim now carries a warrant and no claim that
// lacked one has gained one. Pure.
//
// **This is the clause the surgery could plausibly break.** An insertion point computed
// from the wrong entry lands the block on a neighbour, and the document still parses,
// still keeps its keys, and still keeps its body — it simply records a decision about
// the wrong assertion, which is the one failure nothing downstream could detect.
func landedOnlyOn(op string, before, after *okf.Document, claimID string) error {
	was := map[string]bool{}
	previous := docClaims(before)
	for i := range previous {
		was[previous[i].ID] = previous[i].Warrant.Adjudicated()
	}
	found := false
	current := docClaims(after)
	for i := range current {
		c := &current[i]
		switch {
		case c.ID == claimID:
			found = c.Warrant.Adjudicated()
		case c.Warrant.Adjudicated() && !was[c.ID]:
			return &errs.Error{
				Code: errs.EINVALID, Op: op,
				Message: op + ": the warrant landed on claim " + c.ID + " rather than " +
					claimID + "; nothing was written",
			}
		}
	}
	if !found {
		return &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": the warrant did not land on claim " + claimID +
				"; nothing was written",
		}
	}
	return nil
}

// adjudicate records a decision and writes its warrant (§10.4, §10.6).
//
// Requires: cmd validated; w holds the lock when the command applies.
// Ensures: on apply, the named claim carries a warrant, any named challenge is closed,
// and the trail carries a row; on preview, nothing is written and the outcome reports
// what would happen — including every refusal an apply would reach, so a preview cannot
// pass where the write would fail.
//
// **The authority gate is here rather than in Validate**, because it depends on the
// corpus: whether a co-signer is required is a fold over the adjudicators the corpus
// actually has (§10.6.1) and over whether this claim is escalated. A pure value cannot
// know either.
func (c *Coordinator) adjudicate(
	_ context.Context, w *Writer, cmd *command.Adjudicate,
) (gnosis.Outcome, error) {
	const op = "bundle.Coordinator.adjudicate"

	docs, err := Load(os.DirFS(c.Dir))
	if err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	signers := adjudicators(docs)
	authority := gnosis.FoldAuthority(len(signers))
	warrant := gnosis.Warrant{
		By: string(cmd.By), At: c.now().Format(time.RFC3339),
		Authority: authority.String(), Rationale: cmd.Rationale,
		CoSignedBy: string(cmd.CoSigner), OverrideReason: cmd.Override,
		Reverses: cmd.Reverses,
	}
	data := map[string]any{
		"path": cmd.Path, "claim": cmd.ClaimID, "effect": cmd.Eff.String(),
		"by": string(cmd.By), "authority": authority.String(),
	}
	if refusal := unmetAuthority(authority, cmd); refusal != "" {
		return gnosis.Blocked(gnosis.ReasonNeedsHuman, refusal, data), nil
	}

	existing, err := readConcept(op, c.Dir, cmd.Path)
	if err != nil {
		return challengeRefused(op, &command.Challenge{
			Path: cmd.Path, Eff: cmd.Eff,
		}, err)
	}
	out, err := AppendWarrant(existing, cmd.ClaimID, &warrant)
	if err == nil && cmd.Challenge != "" {
		out, err = CloseChallenge(out, cmd.Challenge)
	}
	if err != nil {
		return challengeRefused(op, &command.Challenge{
			Path: cmd.Path, Eff: cmd.Eff,
		}, err)
	}

	if !cmd.Eff.Writes() {
		data["adjudicated"] = false
		data["would_adjudicate"] = true
		return gnosis.OK(data), nil
	}
	return c.recordAdjudication(op, w, cmd, &adjudication{
		before: existing, after: out, authority: authority,
		signers: signers, data: data,
	})
}

// recordAdjudication writes the decision and everything that records it.
//
// Requires: w holds the lock; a.after is the document AppendWarrant produced.
// Ensures: the document is written before anything describing it, so nothing claims a
// decision that is not on disk.
//
// **Split from adjudicate by what it knows rather than by when it runs.** That function
// answers whether the decision may proceed and what document it would produce; this one
// answers how a decision that has been made is recorded — three ledgers with three
// different failure rules, which is the knowledge worth keeping in one place.
func (c *Coordinator) recordAdjudication(
	op string, w *Writer, cmd *command.Adjudicate, a *adjudication,
) (gnosis.Outcome, error) {
	if err := w.WriteConcept(cmd.Path, a.after); err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	a.data["adjudicated"] = true
	if cmd.Challenge != "" {
		a.data["challenge_closed"] = cmd.Challenge
	}
	// **After the warrant lands, and the order is the safe one.** A crash between the
	// two leaves an authority that moved and was not announced, which `doctor` reports
	// and a second run repairs; the reverse announces a move that did not happen, and
	// nothing would ever contradict it.
	if move := c.announceAuthority(w, a.signers, cmd); move.Moved() {
		a.data["authority_moved"] = move.String()
	}
	c.record(w, &audit.Row{
		At: c.now(), Op: audit.OpAdjudicate, Actor: string(cmd.By),
		Paths: []string{cmd.Path}, HashBefore: hashOrEmpty(a.before),
		HashAfter: hashOrEmpty(a.after), Outcome: string(gnosis.StatusOK),
		Detail: "adjudicated " + cmd.ClaimID + " at " + a.authority.String() + ": " +
			cmd.Rationale,
	})
	return gnosis.OK(a.data), nil
}

// unmetAuthority reports why the authority in force refuses this decision, or "".
//
// Requires: authority is the corpus's current one.
// Ensures: "" when the decision may proceed. Pure.
//
// **Escalation is not consulted, and the effect is to require more rather than less.**
// §10.6.1 requires a co-signer for a *load-bearing or normative* claim, and this asks
// for one at paired and quorum whatever the claim is. That is the conservative
// direction and it is stated rather than hidden: a corpus with several adjudicators
// asking for a second signature on an ordinary claim costs one flag, where the
// alternative — deciding escalation here — would put §14.4.1's centrality computation
// inside a write path and let a link added later change what a past decision required.
// `lint`'s `co-sign` check reports the escalated cases against the recorded authority,
// which is where that judgement belongs.
func unmetAuthority(authority gnosis.Authority, cmd *command.Adjudicate) string {
	if !authority.RequiresCoSigner() {
		return ""
	}
	if cmd.CoSigner != gnosis.ActorUnset {
		return ""
	}
	if strings.TrimSpace(cmd.Override) == "" {
		return "this corpus is at " + authority.String() + ", so the decision needs a" +
			" --co-signer, or an --override whose reason is recorded: a waiver that" +
			" leaves no trace cannot be told from an authority that was never in force"
	}
	if !authority.OverridePermitted() {
		return "this corpus is at " + authority.String() + ", where a co-signature" +
			" cannot be waived: with four or more adjudicators, one being unavailable" +
			" is not a reason the corpus cannot wait"
	}
	return ""
}

// readConcept reads one document for a write path.
//
// Requires: rel is a bundle-relative concept path.
// Ensures: ENOTFOUND when the bundle holds no document there, distinguishable from a
// read failure — a caller naming a path that was never promoted has made a different
// mistake from one whose disk is failing.
func readConcept(op, bundleDir, rel string) ([]byte, error) {
	full := filepath.Join(bundleDir, filepath.FromSlash(path.Clean(rel)))
	data, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &errs.Error{
				Code: errs.ENOTFOUND, Message: op + ": no document at " + rel,
			}
		}
		return nil, &errs.Error{Op: op, Err: err}
	}
	return data, nil
}

// Reversals reads every warrant that overturned another (§10.6.5).
//
// Requires: bundleDir is a bundle root.
// Ensures: one entry per reversing warrant, in document order then claim order; empty
// rather than nil for a corpus that has reversed nothing.
//
// **Retrieval only, and that is a design constraint rather than a limitation.** §10.6.5
// makes this the one report a reversal feeds, deliberately: inferring reliability from
// reversal counts would be scoring with extra steps, and §17 refuses to score. Nothing
// here counts per actor, ranks anybody, or attaches a signal to a reversed warrant —
// reversal is the ordinary consequence of deciding under incomplete information, and a
// corpus that produced none would be one where nobody was deciding anything contestable.
func Reversals(bundleDir string) ([]Reversal, error) {
	docs, err := Load(os.DirFS(bundleDir))
	if err != nil {
		return nil, err
	}
	out := make([]Reversal, 0)
	for i := range docs {
		doc := &docs[i]
		for j := range doc.Claims {
			claim := &doc.Claims[j]
			if claim.Warrant.Reverses == "" {
				continue
			}
			out = append(out, Reversal{
				Path: doc.Path, ClaimID: claim.ID,
				Reverses: claim.Warrant.Reverses, By: claim.Warrant.By,
				At: claim.Warrant.At, Rationale: claim.Warrant.Rationale,
			})
		}
	}
	return out, nil
}

// announceAuthority records an authority this decision moved (§10.6.3).
//
// Requires: w holds the lock; signers is the corpus's adjudicator set *before* this
// decision; cmd is the adjudication that just landed.
// Ensures: the move, whether or not it moved — a caller reporting it asks `Moved`. A
// failure to write the entry is a note rather than the operation's failure.
//
// **The count after the decision needs no second read of the corpus.** The signers of
// this adjudication are known, and adding them to the set the command already loaded is
// what the corpus will derive on the next read. Reading it again would be a second
// answer to one question, computed after a window in which somebody else could write.
//
// **Best-effort, and the direction is stated.** The warrant is written; refusing the
// operation because a log line could not be appended would tell a caller to retry
// something that succeeded. What a lost entry costs is an unannounced move — which
// `doctor` reports, because that is the half of §10.6.3 that catches a move no command
// caused.
func (c *Coordinator) announceAuthority(
	w *Writer, signers map[string]bool, cmd *command.Adjudicate,
) gnosis.AuthorityMove {
	after := make(map[string]bool, len(signers)+2)
	for who := range signers {
		after[who] = true
	}
	for _, who := range []gnosis.Actor{cmd.By, cmd.CoSigner} {
		if gnosis.IsHumanActor(string(who)) {
			after[string(who)] = true
		}
	}

	move := gnosis.AuthorityMove{
		From:         gnosis.FoldAuthority(len(signers)),
		To:           gnosis.FoldAuthority(len(after)),
		Adjudicators: len(after),
	}
	if !move.Moved() {
		return move
	}
	why := string(cmd.By) + " adjudicated " + cmd.ClaimID + " in " + cmd.Path
	if err := w.Log(c.now(), renderAuthorityMove(move, why)); err != nil && c.Warn != nil {
		_, _ = fmt.Fprintf(c.Warn,
			"warning: the adjudication landed and the authority move was not recorded "+
				"in %s, so `gnosis doctor` will report it as unannounced: %v\n",
			LogFile, err)
	}
	return move
}
