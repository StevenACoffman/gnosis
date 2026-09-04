package lint

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// logFileName is the corpus history, as a diagnostic names it.
//
// A literal rather than `bundle.LogFile`, because a check may not import the shell —
// the same layering rule that keeps this package's only internal import
// `internal/gnosis`. The two spellings are one word and cannot drift into being
// *wrong*, only into disagreeing about a filename OKF §9 fixes anyway.
const logFileName = "log.md"

// Environment is the apparatus around a corpus, gathered by the caller.
//
// It is the input to the second of this package's two passes. The distinction
// between them is worth keeping sharp, because it decides which command a reader
// runs next:
//
//   - A Snapshot describes the knowledge. Its findings say the corpus is wrong.
//   - An Environment describes the machinery. Its findings say gnosis cannot
//     judge whether the corpus is wrong.
//
// Conflating the two produces the worst possible report: a clean corpus that was
// never actually examined, or a vocabulary error dressed up as a bad document.
type Environment struct {
	// Bundle is the root that was inspected, for use in messages.
	Bundle string `json:"bundle"`

	// OntologyPresent reports whether ontology.toml exists at all.
	OntologyPresent bool `json:"ontology_present"`

	// OntologyError is non-empty when the file exists and does not load. The
	// text is the loader's own diagnostic, which already names the offending
	// key or line.
	OntologyError string `json:"ontology_error,omitempty"`

	// Types is how many types the vocabulary declares. Zero types means no
	// document can carry a recognised one, since OKF §4.1 requires one.
	Types int `json:"types"`

	// IndexDocPresent reports whether the OKF §8 entry point exists.
	IndexDocPresent bool `json:"index_doc_present"`

	// SchemaDocPresent reports whether the agent-facing schema document exists
	// (§5.7).
	//
	// It is reported rather than scaffolded by `init`, and the two were alternatives.
	// `AGENTS.md` is *generated*: `init` seeds the hand-editable files and leaves the
	// generated ones to the command that makes them, as it already does for
	// `index.db`. A scaffolded copy would also be rendered from an ontology `init`
	// had only just written, so it would be stale the first time anybody edited the
	// vocabulary — with nothing saying so, because a scaffolded file makes this check
	// dead.
	SchemaDocPresent bool `json:"schema_doc_present"`

	// StateIgnored reports whether .gitignore excludes the derived state
	// directory.
	StateIgnored bool `json:"state_ignored"`

	// IndexPresent reports whether the derived database exists.
	IndexPresent bool `json:"index_present"`

	// IndexVersion is the schema version the database reports, and SchemaVersion
	// is the one this binary writes. They differ in one direction gnosis can
	// repair and one it cannot.
	IndexVersion  int `json:"index_version"`
	SchemaVersion int `json:"schema_version"`

	// Documents and IndexedRows are the corpus and the index sizes. A mismatch
	// means the index no longer describes the bundle.
	Documents   int `json:"documents"`
	IndexedRows int `json:"indexed_rows"`

	// Archive is what tier 0 currently costs, against its declared budget.
	Archive ArchiveSize `json:"archive"`

	// StandardsError is non-empty when standards/archive.toml exists and does not
	// load. The text is the loader's own diagnostic, which already names the key.
	//
	// It is here for the same reason OntologyError is, and the reason is worth
	// stating because the first implementation got it wrong: `inspectArchive`
	// swallowed the error and reported a zero size, so a corpus with a malformed
	// standards file produced a silent clean bill of health from the one command
	// whose entire job is to report a broken apparatus. A check that cannot run
	// must say so.
	StandardsError string `json:"standards_error,omitempty"`

	// Audit is the write trail's own health. §15's argument for having it: every
	// other bullet in that section is enforced by a check, and the trail that
	// records the enforcement is written by the same process it records — so a
	// silent write failure leaves a corpus that looks correct and cannot show it.
	Audit AuditHealth `json:"audit"`

	// TunedButUnread names thresholds this bundle has moved off the seed that no
	// code branches on, so the edit had no effect. Gathered by the shell, which is
	// where the knowledge lives: what reads a value is a fact about the whole
	// program, and this package cannot import standards in any case.
	//
	// It is deliberately not "every unread value". That version reported a finding
	// on every freshly initialised bundle, naming something its owner could neither
	// build nor delete, and a warning true of every corpus is one readers learn to
	// skip.
	TunedButUnread []string `json:"tuned_but_unread,omitempty"`

	// MispinnedStandards names values the file pins to something other than what
	// this binary stamps. An extracted record carries the binary's constant, so a
	// file claiming a different extractor version describes provenance no record
	// in the corpus actually has.
	MispinnedStandards []string `json:"mispinned_standards,omitempty"`

	// Authority is the corpus's derived adjudication authority (§10.6.1), and
	// Adjudicators is the count behind it.
	//
	// **Both, because the authority alone is not actionable.** A reader told the
	// corpus is at `paired` cannot tell whether that is two people or three, and the
	// difference decides whether one departure changes what the corpus requires.
	// §10.6.3 asks for the move to be announced *and to say why*; the count is the
	// why, and it is the half that can be reported without a baseline.
	Authority gnosis.Authority `json:"authority"`

	Adjudicators int `json:"adjudicators"`

	// Announced is the authority `log.md` last recorded, and AnnouncedFound reports
	// whether it recorded one at all.
	//
	// **Two fields because "never announced" is not "announced as sole".** A corpus
	// that has never said anything and one that said `sole` are different states, and
	// a single value would collapse the first into the second — which is the state
	// §10.6.3's rule is *about*, since a corpus at `sole` that never announced it is
	// the ordinary starting point and one at `paired` that never announced it is the
	// silent move the rule refuses.
	Announced      gnosis.Authority `json:"announced"`
	AnnouncedFound bool             `json:"announced_found"`

	// GateSources says where each standards file's values come from: the bundle's
	// own file, or the embedded seed and the version that shipped it.
	//
	// **Reported rather than diagnosed, and §6.2.2 is why it exists at all.** A
	// value's prior reading comes from the file at a git revision, falling back to
	// the running binary's seed when the file was not there — so for a corpus that
	// never edited the file, both readings are the same values and no loosening can
	// be reported. A release that loosens a seed changes the effective gates
	// everywhere with nothing saying so. Naming the source is what lets a reader find
	// the entry in gnosis's own log.md.
	//
	// It is not a finding. "Your gates come from the seed" is true of every corpus on
	// its first day, and a warning true of everything teaches a reader to skip the
	// category — the same argument diagnoseUnread already lost once.
	GateSources []GateSource `json:"gate_sources,omitempty"`

	// SchemaMissing and SchemaUnexpected are the difference between the schema
	// the database has and the schema the migrations describe. Empty when the
	// index is absent, since there is nothing to compare.
	SchemaMissing    []string `json:"schema_missing,omitempty"`
	SchemaUnexpected []string `json:"schema_unexpected,omitempty"`
}

// GateSource is where one standards file's values came from.
type GateSource struct {
	// File is the bundle-relative path the values would be read from.
	File string `json:"file"`

	// Origin is "bundle" when the file is present, or "seed" when it is not and the
	// embedded default is in force.
	Origin string `json:"origin"`

	// Version is the gnosis version whose seed is in force, and is empty when the
	// bundle carries its own file. It is what a reader needs to find the log entry
	// recording a seed change.
	Version string `json:"version,omitempty"`
}

// diagnoseStandards reports a standards file that exists and cannot be read.
//
// It blocks nothing, unlike a broken vocabulary. An unusable ontology means every
// document is judged against nothing; an unusable standards file means the gates
// fall back to the embedded seed, which is a defined and reasonable state — the
// corpus is still checkable, just not against the thresholds somebody wrote.
//
// **The severity used to be SeverityError, which blocks, directly contradicting
// the paragraph above.** Three places agreed it should not block — this comment,
// Diagnose's contract, and the reasoning that a fallback to the seed is a defined
// state — and only the constant disagreed, so the constant was the defect. Found
// by reading, not by a failure: a wrong severity produces no test failure, it
// produces a non-zero exit on a corpus with nothing wrong with it.
func diagnoseStandards(env *Environment) []finding.Diagnostic {
	if env.StandardsError == "" {
		return nil
	}
	return []finding.Diagnostic{{
		Severity: finding.SeverityWarning,
		Category: "standards",
		Path:     "standards/archive.toml",
		Message: "the archive standards do not load, so the seed defaults are in " +
			"force and the budget is unreported: " + env.StandardsError,
		Action: finding.ActionGuided,
	}}
}

// diagnoseUnread reports an edit to the standards files that did nothing.
//
// Warnings rather than errors, and never automatic. The corpus is entirely
// checkable in both cases; what is wrong is that somebody changed a number and
// got no behaviour for it, and whether to revert the edit or build the reader is
// a judgement no tool should make.
func diagnoseUnread(env *Environment) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0, 2)
	if len(env.TunedButUnread) > 0 {
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "standards",
			Path:     "standards/",
			Message: "tuned away from the default and read by nothing, so the edit " +
				"has no effect: " + strings.Join(env.TunedButUnread, ", "),
			Action: finding.ActionHuman,
		})
	}
	if len(env.MispinnedStandards) > 0 {
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "standards",
			Path:     "standards/",
			Message: "pinned to something this gnosis does not implement, so records " +
				"carry provenance the file contradicts: " +
				strings.Join(env.MispinnedStandards, ", "),
			Action: finding.ActionHuman,
		})
	}
	return out
}

// Diagnose reports what is wrong with the apparatus.
//
// Requires: env is non-nil and was gathered from one bundle at one moment. It is
// taken by pointer for its size, not so it can be modified: nothing here writes
// to it.
// Ensures: diagnostics are sorted, so two runs over one bundle are comparable.
//
// **A finding blocks only where continuing would mean judging the corpus against
// something other than its own rules.** That is the rule; an earlier version of
// this contract stated a count instead — "only two conditions block" — which was
// true when written and silently false by the time three more error-severity cases
// existed. A rule survives the next case; a count does not.
//
// Today it admits five: a missing vocabulary, an unparsable one, a vocabulary with
// no types, an index from a newer gnosis, and an index missing schema objects. A
// damaged audit trail is deliberately not among them — it makes the corpus's
// history unrecountable and leaves the corpus itself perfectly checkable.
func Diagnose(env *Environment) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	out = append(out, diagnoseVocabulary(env)...)
	out = append(out, diagnoseBundleFiles(env)...)
	out = append(out, diagnoseIndex(env)...)
	out = append(out, diagnoseStandards(env)...)
	out = append(out, diagnoseUnread(env)...)
	out = append(out, diagnoseAudit(env)...)
	out = append(out, diagnoseAuthority(env)...)
	out = append(out, DiagnoseBudget(&env.Archive)...)
	finding.Sort(out)
	return out
}

// diagnoseAuthority reports an adjudication authority that moved without being
// announced (§10.6.3).
//
// Requires: env.Authority is derived from the corpus and env.Announced is what `log.md`
// last recorded.
// Ensures: nothing for a corpus whose announcement matches what it derives, and nothing
// for a fresh corpus at `sole` that has announced nothing — which is every corpus before
// its first adjudication, and reporting it would make the check fire on the ordinary
// state. Pure.
//
// **This is the half of the rule that `adjudicate` cannot keep.** That command announces
// a move it causes; nothing announces a move caused by a hand-edited `verified` list, a
// merged branch, or a colleague's warrant arriving through `git pull`. §10.6.3 says a
// tier change is announced *never silent*, and a rule that only holds when a particular
// command runs is a rule that holds by luck.
//
// Warning rather than error, and the remedy is a sentence in `log.md`: the corpus is not
// malformed, and blocking on it would make a colleague's arrival a build failure.
func diagnoseAuthority(env *Environment) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0, 1)
	switch {
	case env.AnnouncedFound && env.Announced == env.Authority:
		return out
	case !env.AnnouncedFound && env.Authority == gnosis.AuthoritySole:
		// A corpus that has adjudicated nothing is at `sole` and has nothing to
		// announce. §10.6.3 calls a single-curator corpus a supported configuration
		// rather than a degenerate one, and a finding here would say otherwise on
		// every bundle the day it is created.
		return out
	}

	was := "nothing in " + logFileName + " records what it was"
	if env.AnnouncedFound {
		was = logFileName + " last recorded " + env.Announced.String()
	}
	return append(out, finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "authority",
		Path:     logFileName,
		Message: "this corpus derives the adjudication authority " +
			env.Authority.String() + " and " + was +
			" — §10.6.3 requires a tier change to be announced rather than silent," +
			" because a gate that tightens or loosens without telling anyone is the" +
			" same failure as a threshold that moves quietly. Record the move and why",
		Action: finding.ActionHuman,
	})
}

// diagnoseVocabulary reports on ontology.toml.
func diagnoseVocabulary(env *Environment) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0, 1)
	switch {
	case !env.OntologyPresent:
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityError,
			Category: "vocabulary",
			Path:     "ontology.toml",
			Message:  "no vocabulary; run `gnosis init` to write the starter one",
			Action:   finding.ActionAutomatic,
		})
	case env.OntologyError != "":
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityError,
			Category: "vocabulary",
			Path:     "ontology.toml",
			Message:  env.OntologyError,
			Action:   finding.ActionHuman,
		})
	case env.Types == 0:
		// Not merely unhelpful: OKF §4.1 requires a type on every document, so a
		// vocabulary with none makes every document unclassifiable.
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityError,
			Category: "vocabulary",
			Path:     "ontology.toml",
			Message:  "the vocabulary declares no types, so no document can carry a known one",
			Action:   finding.ActionHuman,
		})
	}
	return out
}

// diagnoseBundleFiles reports on the files OKF and git hygiene expect.
func diagnoseBundleFiles(env *Environment) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0, 3)
	if !env.SchemaDocPresent {
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "bundle",
			Path:     "AGENTS.md",
			Message: "no schema document; §5.7 expects one and an agent arriving " +
				"has no account of this corpus's conventions — run `gnosis schema`",
			// Automatic: the fix is a command, which is what that action means. It
			// is not `guided`, because there is nothing for a person to decide —
			// the content is derived from the ontology and the binary.
			Action: finding.ActionAutomatic,
		})
	}
	if !env.IndexDocPresent {
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "bundle",
			Path:     "index.md",
			Message:  "no entry point; OKF §8 expects one and a reader arrives here first",
			Action:   finding.ActionAutomatic,
		})
	}
	if !env.StateIgnored {
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "hygiene",
			Path:     ".gitignore",
			Message: "the derived state directory is not ignored, so a cache would " +
				"be committed and conflict on every merge",
			Action: finding.ActionAutomatic,
		})
	}
	return out
}

// diagnoseIndex reports on the derived database.
func diagnoseIndex(env *Environment) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0, 1)
	switch {
	case !env.IndexPresent:
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "index",
			Path:     ".gnosis/index.db",
			Message:  "no index; run `gnosis index rebuild`",
			Action:   finding.ActionAutomatic,
		})
	case env.IndexVersion > env.SchemaVersion:
		// The one finding here a rebuild cannot fix. Rebuilding with an older
		// binary would write a schema the newer one has moved past, so the
		// instruction is to upgrade gnosis, not to run anything.
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityError,
			Category: "index",
			Path:     ".gnosis/index.db",
			Message: fmt.Sprintf(
				"the index is at schema %d but this gnosis writes %d; upgrade gnosis",
				env.IndexVersion, env.SchemaVersion),
			Action: finding.ActionHuman,
		})
	case len(env.SchemaMissing) > 0:
		// Blocking, and distinct from a version mismatch: user_version says how
		// far migration was *recorded*, and each migration commits separately,
		// so an interrupted run leaves a database claiming a version whose
		// schema is not all there. Every later command would then fail on a
		// missing table with SQLite's own error rather than this one.
		//
		// **The remedy names a command, and until 2026-08-27 it could not.** This
		// diagnostic declared ActionAutomatic while advising a manual `rm`: plain
		// `rebuild` opens the existing database and migration skips every statement
		// because user_version is already current, so it failed on the missing table
		// rather than recreating it. `--recreate` is what makes the action code true.
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityError,
			Category: "index",
			Path:     ".gnosis/index.db",
			Message: fmt.Sprintf(
				"the index is missing %d schema object(s) (%s); run "+
					"`gnosis index rebuild --recreate`",
				len(env.SchemaMissing), strings.Join(env.SchemaMissing, ", ")),
			Action: finding.ActionAutomatic,
		})
	case len(env.SchemaUnexpected) > 0:
		// Not blocking, and never removed. An extra object is usually a newer
		// gnosis's work left behind by a downgrade, or a hand-edited database;
		// neither is gnosis's to clean up, and dropping something a person put
		// there deliberately is the worse error.
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "index",
			Path:     ".gnosis/index.db",
			Message: fmt.Sprintf(
				"the index holds %d object(s) the migrations do not describe (%s); "+
					"left in place",
				len(env.SchemaUnexpected), strings.Join(env.SchemaUnexpected, ", ")),
			Action: finding.ActionHuman,
		})
	case env.Documents != env.IndexedRows:
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "index",
			Path:     ".gnosis/index.db",
			Message: fmt.Sprintf(
				"the bundle has %d document(s) and the index %d; run `gnosis index rebuild`",
				env.Documents, env.IndexedRows),
			Action: finding.ActionAutomatic,
		})
	}
	return out
}
