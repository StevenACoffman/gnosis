package command

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// Discard drops a quarantined draft without promoting it.
//
// # Why this exists, and why there is no edit
//
// A candidate the gate **refused** had no route to a fix. `promote` reports which
// signal failed and the author's only recourse was to re-run the whole relay, with
// nothing in the tool acknowledging that the draft was now junk. The obvious
// missing verb was `quarantine --edit`, and it is refused on purpose:
//
// Editing quarantined content by hand is how unvetted text acquires a human's
// authority without review. Tier 1 exists precisely to keep model-written content
// out of the working tree until a gate has looked at it (§9.3, §3083); a person
// opening that file, fixing the sentence the scan objected to, and promoting the
// result would produce a document that passed the gate and was never checked
// against anything — the quotation would validate against the archive because the
// person made it validate. That is the silent false pass the whole corpus is built
// to prevent, arrived at through the front door.
//
// So the sanctioned route for a refused candidate is **discard and re-admit**: fix
// the input — the source, the prompt, the model — and run the relay again. What is
// re-checked is a reply, by the same gate, from the same evidence. Discarding is
// the half of that which had no verb.
//
// # Why it is a command rather than a flag on a read
//
// It mutates: a file leaves tier 1 and a row enters the trail. §4.6.2 makes every
// write a command value so that gating is a property of the type, and this one is
// the cheapest possible demonstration that the shape holds — `audit.OpDiscard` was
// declared when the trail was built and had no writer until now.
type Discard struct {
	// Path is the quarantined document, relative to the bundle root. Traversal is
	// refused where the path is resolved, because a quarantined path arrives from
	// a model's reply and `../../etc/whatever` is an input this will receive.
	Path string

	// Eff is preview or apply. A preview reports what would be dropped and drops
	// nothing, which matters more here than for a promotion: the content is about
	// to be unrecoverable, and tier 1 is not committed, so there is no `git
	// checkout` afterwards.
	Eff Effect

	// By is who dropped it. Required for the reason every actor on a write is
	// required: a discard nobody can be asked about is exactly the event §15's
	// trail exists to prevent, and "the draft is gone and nobody knows who" is
	// worse here than for a promotion, because a promotion leaves the document
	// behind as evidence of itself.
	//
	// Unlike a promotion's Approver this may be an agent. Dropping a draft grants
	// no authority and admits nothing to the corpus, so requiring a person would be
	// a permission check where §10.6.4 says a record is what is wanted.
	By gnosis.Actor

	// Reason is why the draft is being dropped. Required, and this is the field
	// worth defending: the alternative is a trail full of discards with no account
	// of what was wrong, which answers "was this draft junk or was somebody
	// clearing their queue" with silence. It is the same argument §10.6.4 makes for
	// a warrant's rationale, at much lower stakes.
	Reason string
}

// Op names the operation.
func (d *Discard) Op() string { return "discard" }

// Effect reports whether this command writes.
func (d *Discard) Effect() Effect { return d.Eff }

// Validate reports why this command is not executable, or nil.
//
// Requires: nothing; a zero Discard is a valid input and is rejected.
// Ensures: every problem at once, EINVALID, for the same reasons Promote.Validate
// gives. The reason check is length-based and deliberately weak: it distinguishes an
// empty account from a present one and cannot grade one.
func (d *Discard) Validate() error {
	const op = "command.Discard.Validate"

	var bad []string
	if strings.TrimSpace(d.Path) == "" {
		bad = append(bad, "path is empty")
	}
	if !d.Eff.Valid() {
		bad = append(bad, "effect is "+d.Eff.String()+"; set preview or apply")
	}
	if d.By == gnosis.ActorUnset {
		bad = append(bad, "by is unset")
	} else if d.By.Kind() == "" {
		bad = append(bad, "by "+string(d.By)+
			" is not <kind>:<id> with kind one of human, agent, check")
	}
	if strings.TrimSpace(d.Reason) == "" {
		bad = append(bad, "reason is empty; a discard with no account of what was "+
			"wrong leaves a trail nobody can read")
	}
	if len(bad) == 0 {
		return nil
	}
	return &errs.Error{
		Code:    errs.EINVALID,
		Message: op + ": " + strings.Join(bad, "; "),
	}
}
