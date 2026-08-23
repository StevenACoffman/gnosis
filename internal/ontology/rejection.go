package ontology

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// checkRejections validates one entry's refused aliases.
//
// Requires: kind names what is being checked ("type" or "subject"); key names the
// entry, for the message; aliases are the phrases it does admit.
// Ensures: EINVALID when a rejection has no phrase, when it has no reason, or when
// a phrase appears as both an alias and a rejection.
//
// **The reason is required, and that is the whole mechanism.** A rejection with no
// reason records that somebody said no and not what they knew, which leaves the
// next person to work it out again — the exact re-litigation §5.8.2 adds this list
// to prevent. It is the same discipline as `standards.Value`'s rationale and it
// exists for the same reason: a justification that were merely conventional would
// be the first thing dropped by whoever was in a hurry.
//
// A phrase that is both admitted and refused is a contradiction rather than a
// nuance. §5.8.2.1 makes resolution exclusive, so the corpus would be claiming the
// phrase both does and does not resolve — and whichever the loader happened to
// apply first would become the answer, silently.
func checkRejections(op, kind, key string, aliases []string, rejected []Rejection) error {
	admitted := make(map[string]bool, len(aliases)+1)
	admitted[gnosis.Surface(key).Fold()] = true
	for _, a := range aliases {
		admitted[gnosis.Surface(a).Fold()] = true
	}

	seen := make(map[string]bool, len(rejected))
	for i := range rejected {
		r := &rejected[i]
		folded := gnosis.Surface(r.Alias).Fold()

		switch {
		case strings.TrimSpace(r.Alias) == "":
			return rejectionErr(op, kind, key, "a rejection names no alias")
		case strings.TrimSpace(r.Reason) == "":
			return rejectionErr(op, kind, key, "rejection "+quote(r.Alias)+
				" has no reason; a refusal nobody explained gets proposed again")
		case admitted[folded]:
			return rejectionErr(op, kind, key, quote(r.Alias)+
				" is both an alias and a rejection")
		case seen[folded]:
			return rejectionErr(op, kind, key, quote(r.Alias)+" is rejected twice")
		}
		seen[folded] = true
	}
	return nil
}

// rejectionErr renders a rejection problem, naming the entry it is in.
func rejectionErr(op, kind, key, detail string) error {
	return &errs.Error{
		Code:    errs.EINVALID,
		Message: fmt.Sprintf("%s: %s %q: %s", op, kind, key, detail),
	}
}

func quote(s string) string { return `"` + s + `"` }
