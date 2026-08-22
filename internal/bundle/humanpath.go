package bundle

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gate"
)

// authorisedBy reports why a human may not carry this promotion, or "" when they
// may.
//
// Requires: the gate decided NeedsHuman. Calling it for any other decision is a
// caller error — Approved needs no human and Refused admits none.
// Ensures: pure. Every unmet requirement is named at once, so one refusal tells a
// caller everything to fix rather than three round trips.
//
// The three requirements are §9.5's, and each blocks a different way of turning an
// escalation back into a bypass:
//
//   - **A person.** "No self-granted approval": an agent that could authorise its
//     own promotion would make the whole path decorative, and an agent is exactly
//     what produced the candidate. Actor.IsHuman is false for ActorUnset and for a
//     malformed actor, so neither an unset field nor a typo satisfies this.
//   - **The phrase.** Typing the document's path is `adh`'s discipline for
//     irreversible actions. A `--yes` is muscle memory; naming the file is not.
//   - **A rationale.** The one artifact that survives into the audit trail as an
//     explanation rather than a fact. §10.6.4's argument is that a required
//     rationale filters more bad adjudications than a permission check does,
//     because the reviewer has to write it where colleagues will read it.
func authorisedBy(cmd *command.Promote) string {
	var missing []string
	if !cmd.Approver.IsHuman() {
		missing = append(missing, "the approver must be a person (human:<id>); "+
			"this promotion cannot be self-granted by an agent")
	}
	if cmd.Confirmation != cmd.Path {
		missing = append(missing, "confirm by typing the document's path exactly: "+cmd.Path)
	}
	if strings.TrimSpace(cmd.Rationale) == "" {
		missing = append(
			missing,
			"state why you are promoting a candidate the gate could not fully check",
		)
	}
	return strings.Join(missing, "; ")
}

// unrunSignals names the signals a promotion is being carried over, for the audit
// row.
//
// Requires: nothing.
// Ensures: sorted, and never nil for a NeedsHuman decision.
//
// This is what makes the human path a debt register rather than a bypass, and the
// distinction rests entirely on this being recorded. A trail saying only "promoted
// by a human" is a trail that cannot answer the question that matters when §10
// lands: which claims in this corpus were admitted without a conflict check. A
// trail naming the signals can, and every one of those documents can then be
// re-examined.
func unrunSignals(report *gate.Report) []string {
	_, unchecked := report.Withheld()
	out := make([]string, 0, len(unchecked))
	for _, s := range unchecked {
		out = append(out, string(s))
	}
	return out
}
