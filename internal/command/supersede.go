package command

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// Supersede records that one claim replaced another (§10.4).
//
// # Why the loser is kept
//
// Supersession, never deletion: the losing claim gets `status: deprecated`, which OKF
// §5.4 defines as "kept for links and history; no longer current", and the winner gets
// a `gnosis_supersedes` edge. The property that buys is the one a corpus exists for —
// it can always answer what we believed in March and why we changed — and both a delete
// and a rewrite destroy it, the first visibly and the second silently.
//
// # Why it carries no rationale
//
// The reasoning belongs to the warrant, and `lint`'s `warrant` check reports a claim
// that supersedes another and records no `gnosis_warrant`. Asking for a reason here as
// well would collect it in a place no reader looks and let the warrant stay empty —
// two fields for one obligation, of which only one is checked.
type Supersede struct {
	// LoserPath and LoserClaim address the claim being replaced.
	LoserPath  string
	LoserClaim string

	// WinnerPath and WinnerClaim address the claim replacing it. The paths may be
	// equal: two claims on one page can supersede each other, and that is one write
	// rather than two.
	WinnerPath  string
	WinnerClaim string

	// By is who recorded it. An agent may: the supersession is bookkeeping over a
	// decision that is recorded elsewhere, and the decision itself — the warrant —
	// is what requires a person.
	By gnosis.Actor

	// Eff is preview or apply.
	Eff Effect
}

// Op names the operation.
func (s *Supersede) Op() string { return "supersede" }

// Effect reports whether this command writes.
func (s *Supersede) Effect() Effect { return s.Eff }

// Validate reports why this command is not executable, or nil.
//
// Requires: nothing; a zero Supersede is a valid input and is rejected.
// Ensures: every problem at once, EINVALID.
//
// **A claim may not supersede itself**, which is the one case that would otherwise
// write a deprecation and an edge onto one entry and leave the corpus with a claim
// that replaced and outlived itself.
func (s *Supersede) Validate() error {
	const op = "command.Supersede.Validate"

	var bad []string
	for _, f := range [][2]string{
		{"loser path", s.LoserPath},
		{"loser claim", s.LoserClaim},
		{"winner path", s.WinnerPath},
		{"winner claim", s.WinnerClaim},
	} {
		if strings.TrimSpace(f[1]) == "" {
			bad = append(bad, f[0]+" is empty")
		}
	}
	if s.LoserPath == s.WinnerPath && s.LoserClaim == s.WinnerClaim &&
		s.LoserClaim != "" {
		bad = append(bad, "a claim cannot supersede itself")
	}
	if !s.Eff.Valid() {
		bad = append(bad, "effect is "+s.Eff.String()+"; set preview or apply")
	}
	if s.By == gnosis.ActorUnset {
		bad = append(bad, "by is unset")
	} else if s.By.Kind() == "" {
		bad = append(bad, "by "+string(s.By)+
			" is not <kind>:<id> with kind one of human, agent, check")
	}
	if len(bad) == 0 {
		return nil
	}
	return &errs.Error{Code: errs.EINVALID, Message: op + ": " + strings.Join(bad, "; ")}
}
