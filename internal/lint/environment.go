package lint

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
)

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

	// SchemaMissing and SchemaUnexpected are the difference between the schema
	// the database has and the schema the migrations describe. Empty when the
	// index is absent, since there is nothing to compare.
	SchemaMissing    []string `json:"schema_missing,omitempty"`
	SchemaUnexpected []string `json:"schema_unexpected,omitempty"`
}

// Diagnose reports what is wrong with the apparatus.
//
// Requires: env is non-nil and was gathered from one bundle at one moment. It is
// taken by pointer for its size, not so it can be modified: nothing here writes
// to it.
// Ensures: diagnostics are sorted, so two runs over one bundle are comparable.
// Only two conditions block — an unusable vocabulary and an index written by a
// newer gnosis — because those are the two where continuing would mean judging
// the corpus against something other than its own rules.
func Diagnose(env *Environment) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	out = append(out, diagnoseVocabulary(env)...)
	out = append(out, diagnoseBundleFiles(env)...)
	out = append(out, diagnoseIndex(env)...)
	finding.Sort(out)
	return out
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
	out := make([]finding.Diagnostic, 0, 2)
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
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityError,
			Category: "index",
			Path:     ".gnosis/index.db",
			Message: fmt.Sprintf(
				"the index is missing %d schema object(s) (%s); delete it and run "+
					"`gnosis index rebuild`",
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
