package lint

import (
	"sort"

	"github.com/StevenACoffman/skillet/finding"
)

// archiveClosureCheck reports where tier 0's store and its ledger disagree.
//
// §4.3.1 makes the fetch records authoritative and the archived text the thing they
// account for, so the two are a closed set: every file under `evidence/text/` should
// be named by a record, and every path a record names should be there. Nothing
// checked either direction.
//
// # Two backlog entries turned out to be one mechanism
//
// One asked for "an archived file that no `fetch.jsonl` row records", reasoning from
// bundle closure — VAC fails a bundle containing a file its manifest does not list,
// and `qvr sync` does the same from the other end. The other asked whether
// `evidence/text` has orphans, reasoning from a crash: `StoreEvidence` writes the
// content before the record, so an interruption between them leaves text with no
// record. **Those are the same file in the same state**, and the second entry is
// where the cost is stated — an orphan is inert, but it counts against the corpus
// budget (§4.3) and nothing ever collects it.
//
// The other direction is the cheap pair and belongs in the same walk: a record
// naming an archive path that is absent. It is distinct from `archive-path`, which
// reports a *claim* naming a missing file — that is a claim that cannot be verified,
// where this is the ledger and the store disagreeing about what tier 0 holds.
//
// # Severities differ, and the difference is the whole reading
//
// An orphan is a **warning**: nothing is lost, the corpus is merely carrying weight
// it cannot account for, and deleting it is a judgement about pruning rather than a
// repair. A record naming an absent file is an **error**: the ledger claims evidence
// that is not there, and §9.4's invariant — a quotation appears in the named
// archived file — has nothing to check against. The first is untidy and the second
// is a corpus that cannot fail honestly.
//
// It reports and never repairs. An orphan may be a crash remnant or the residue of a
// prune somebody meant, and those call for opposite responses.
func archiveClosureCheck() Check {
	return Check{
		Name:       "archive-closure",
		Categories: []string{"archive-orphan", "archive-unrecorded"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies: func(snap *Snapshot) (bool, string) {
			// Derived applicability, per §12. A corpus that has fetched nothing has
			// no closure to check, and "no orphans" and "nothing to look at" are
			// different answers.
			if len(snap.ArchivedText) == 0 && len(snap.RecordedText) == 0 {
				return false, "the bundle has no archived evidence yet"
			}
			return true, ""
		},
		Run: func(snap *Snapshot) []finding.Diagnostic {
			out := make([]finding.Diagnostic, 0)
			out = append(out, orphanedText(snap)...)
			out = append(out, unrecordedEvidence(snap)...)
			return out
		},
	}
}

// orphanedText reports archived files no fetch record names.
//
// One finding per file rather than one for the set: each is a separate decision
// about whether to keep or prune, and a single finding naming forty paths is one a
// reader defers rather than acts on.
func orphanedText(snap *Snapshot) []finding.Diagnostic {
	orphans := make([]string, 0)
	for path := range snap.ArchivedText {
		if !snap.RecordedText[path] {
			orphans = append(orphans, path)
		}
	}
	sort.Strings(orphans)

	out := make([]finding.Diagnostic, 0, len(orphans))
	for _, path := range orphans {
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "archive-orphan",
			Path:     path,
			Message: "no fetch record names this file, so tier 0 cannot say where it " +
				"came from; it counts against the corpus budget and nothing collects it" +
				" — a crash between the content write and the record write leaves this",
			Action: finding.ActionHuman,
		})
	}
	return out
}

// unrecordedEvidence reports fetch records naming archived text that is absent.
//
// An error rather than a warning: the ledger claims evidence the corpus does not
// hold, so §9.4's invariant has nothing to check a quotation against. That is the
// state where the corpus stops being able to fail honestly — a claim resting on this
// record is neither supported nor refuted.
func unrecordedEvidence(snap *Snapshot) []finding.Diagnostic {
	missing := make([]string, 0)
	for path := range snap.RecordedText {
		if !snap.ArchivedText[path] {
			missing = append(missing, path)
		}
	}
	sort.Strings(missing)

	out := make([]finding.Diagnostic, 0, len(missing))
	for _, path := range missing {
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityError,
			Category: "archive-unrecorded",
			Path:     path,
			Message: "a fetch record names this archived file and it is not in the " +
				"bundle; the ledger claims evidence tier 0 does not hold, so a " +
				"quotation resting on it can be neither verified nor refuted",
			Action: finding.ActionHuman,
		})
	}
	return out
}
