package command

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// Challenge is a reader contesting an accepted claim (§10.7).
//
// # Why a challenge is a command and not a flag on a read
//
// It mutates the committed tier: the challenged document gains a
// `gnosis_challenges` entry, which travels to every other user through the same
// `git pull` that carries the claim. §4.6.2 makes every write a command value so
// gating is a property of the type — and a challenge needs the preview half more
// than most writes do, because the person filing it is often the person least
// familiar with the corpus's conventions and the diff lands on somebody else's page.
//
// # What it deliberately does not carry
//
// **No severity.** §10.7.3's first property is that an unverified challenge does not
// block: only a `replay` challenge gnosis has itself verified becomes
// error-severity, and letting a challenger choose would make the front door a denial
// of service. The class is what a challenge asserts; severity is what the corpus
// concludes.
//
// **No claim identifier.** §10.7.4 files a challenge against the document, and a
// reader who has noticed something wrong has not always worked out which claim
// carries it — which is part of what they are asking somebody to do.
type Challenge struct {
	// Path is the document being contested, relative to the bundle root.
	Path string

	// Class states what kind of thing would settle the dispute (§10.7.1). It is a
	// closed set, and a class outside it is refused here rather than written: a
	// challenge nothing can route is one nobody will answer.
	Class gnosis.ChallengeClass

	// By is the challenger.
	//
	// **An agent may file one**, which is the same latitude `Discard.By` has and for
	// a stronger reason: challenging grants no authority and admits nothing, and
	// §10.7.3 says being wrong must cost the challenger nothing. A check that
	// noticed a contradiction the selector is blind to is exactly the informant
	// §6.2.1 wants and has no person attached.
	By gnosis.Actor

	// Rationale is why the claim is wrong. Required and non-empty (§10.7.2): a
	// reader who cannot say why has filed a doubt rather than a challenge, and the
	// corpus has no way to act on a doubt.
	Rationale string

	// Eff is preview or apply, and preview is the zero value for the reason §4.6.2
	// gives — the field's default must not be the one that writes.
	Eff Effect
}

// Op names the operation.
func (c *Challenge) Op() string { return "challenge" }

// Effect reports whether this command writes.
func (c *Challenge) Effect() Effect { return c.Eff }

// Validate reports why this command is not executable, or nil.
//
// Requires: nothing; a zero Challenge is a valid input and is rejected.
// Ensures: every problem at once, EINVALID, so a caller fixing one is not told about
// the next on the following run.
//
// The class check names the six rather than saying "invalid class", because a
// challenger who mistyped one needs the list and a challenger who invented one needs
// to know it is closed.
func (c *Challenge) Validate() error {
	const op = "command.Challenge.Validate"

	var bad []string
	if strings.TrimSpace(c.Path) == "" {
		bad = append(bad, "path is empty")
	}
	if _, ok := gnosis.ParseChallengeClass(string(c.Class)); !ok {
		bad = append(bad, "class is "+string(c.Class)+"; one of "+
			strings.Join(gnosis.ChallengeClasses(), ", "))
	}
	if !c.Eff.Valid() {
		bad = append(bad, "effect is "+c.Eff.String()+"; set preview or apply")
	}
	if c.By == gnosis.ActorUnset {
		bad = append(bad, "by is unset")
	} else if c.By.Kind() == "" {
		bad = append(bad, "by "+string(c.By)+
			" is not <kind>:<id> with kind one of human, agent, check")
	}
	if strings.TrimSpace(c.Rationale) == "" {
		bad = append(bad, "rationale is empty; a reader who cannot say why a claim "+
			"is wrong has filed a doubt, and the corpus cannot act on a doubt")
	}
	if len(bad) == 0 {
		return nil
	}
	return &errs.Error{Code: errs.EINVALID, Message: op + ": " + strings.Join(bad, "; ")}
}
