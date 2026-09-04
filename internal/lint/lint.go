// Package lint is the corpus health registry.
//
// Every check is a pure function over a Snapshot: the caller gathers the facts
// from disk and the index, and nothing here performs I/O. That split is what
// makes each check testable from a literal rather than from a fixture directory.
//
// Two structural properties apply to every check, and both come from SPEC §12:
//
// Applicability is derived, not declared. An orphan check is meaningless in a
// corpus that has no links yet, and reporting every document as orphaned on day
// one would teach a reader to ignore the check. Each check reports whether the
// corpus exhibits the convention it examines, and is skipped when it does not.
//
// A run states what it skipped. A check that silently declines to run is
// indistinguishable from a check that found nothing, and the difference matters
// to anyone deciding whether the corpus is healthy or merely unexamined.
package lint

import (
	"sort"
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// Snapshot is everything the checks need, gathered by the caller.
//
// It is a value, not an interface: the checks are pure functions and there is
// nothing here to substitute. Assembling one is the shell's job.
type Snapshot struct {
	// Documents is every document found in the bundle.
	Documents []Document

	// Links is every link found in every document body.
	Links []Link

	// Resolutions is the output of gnosis.Reconcile for this corpus.
	Resolutions []gnosis.Resolution

	// LogLines is log.md split into lines, or nil when the bundle has none.
	// OKF §9 makes log.md optional, so absent is not a finding.
	LogLines []string

	// HasLog distinguishes an absent log from an empty one.
	HasLog bool

	// SchemaVersion is the conventions version this build of gnosis writes.
	// Documents declaring an older one are reported by the schema-version check.
	SchemaVersion int

	// ArchivedText is the set of paths under evidence/text/ that exist, so the
	// archive-path check can resolve a claim's addresses without touching a disk.
	ArchivedText map[string]bool

	// RecordedText is the set of archive paths tier 0's fetch records name.
	//
	// It is the other half of ArchivedText, and the pair is what makes tier 0's
	// closure checkable: §4.3.1 makes the records authoritative and the text the
	// thing they account for, so a path in one set and not the other means the store
	// and the ledger disagree. Gathered rather than derived because it comes from
	// reading every record, which is the shell's work.
	RecordedText map[string]bool

	// SourceChecks is when this user last verified each source version, keyed as
	// Document.SourceKeys are. Per-user by §4.3.1, which is why it is gathered
	// rather than derived: two colleagues at one commit hold different values and
	// are both right.
	SourceChecks map[string]time.Time

	// StalenessDays is the declared window after which an unverified source is
	// reported, from standards/. Zero disables the window, which is the state of a
	// corpus whose standards did not load.
	StalenessDays int

	// Bounds maps a claim id to the reading its prose parses to (§10.2.1). Empty for a
	// corpus whose claims name no declared subject.
	Bounds map[string]*Bound

	// Sources maps an archived file to the source and version it holds, from tier 0's
	// records. Empty for a corpus that has fetched nothing.
	Sources map[string]SourceVersion

	// LanguageMarkers is §10.3's lexical class, from standards/.
	LanguageMarkers []LanguageMarker

	// ArchiveText is the text of the archived files claims actually cite.
	//
	// **Only the cited paths.** Reading a corpus's whole archive into a snapshot
	// would hold every fetched page in memory to re-validate a handful of
	// quotations, and the archive is the largest thing a bundle owns.
	ArchiveText map[string]string

	// SchemaDoc is AGENTS.md's text, or empty when the bundle has none.
	SchemaDoc string

	// Commands are this binary's registered subcommand names.
	//
	// Filled by the command layer rather than by `bundle`, because the registry lives
	// there and nowhere else — the same route `schemacmd` uses to hand PlanSchemaDoc
	// its list. §5.7.1's point is that the binary describes itself.
	Commands []string

	// Indicators is the closed lexical class of §9.4.1, from standards/. The reason
	// words gate segmentation in the shell; the conclusion words are read here.
	Indicators Indicators

	// Strength is the claim-strength markers of §17.3.1, from standards/.
	//
	// Gathered rather than derived, like the vocabulary: the check stays pure and the
	// closed lexical class stays in data where a corpus can extend it.
	Strength Strengths

	// Registers is the causal-register class of §17.3.1.1, from standards/. Read by
	// `coverage`'s second axis, which compares the rung a claim asserts against the
	// rung its own quotations support.
	Registers Registers

	// Vocabulary is ontology.toml flattened to what the checks compare against.
	// Its zero value states that the bundle declares none, which skips the three
	// checks that read it rather than failing them.
	Vocabulary Vocabulary

	// Authority is the corpus's derived adjudication authority (§10.6.1), from the
	// distinct human actors in its warrants and verification lists.
	//
	// Gathered rather than derived here for the reason the vocabulary is: the fold is
	// the domain's, the count is the shell's, and a check that walked the corpus to
	// derive it would be re-deriving one number per check. Its zero value is `sole`,
	// which requires nothing — so a snapshot nobody populated cannot make a corpus
	// unable to adjudicate (§10.6.3).
	Authority gnosis.Authority

	// ChallengeDays is the declared window after which an open challenge is
	// reported, from standards/. Zero skips the check rather than reporting every
	// challenge the moment it is filed.
	ChallengeDays int

	// InDegreeCut is the inbound-link count at which a document is treated as
	// central, from standards/. Zero means the standards did not load, which
	// **skips** the durability check rather than making every document central —
	// an in-degree of zero is at or above a cut of zero, so a missing threshold
	// would report the whole corpus (§14.4.1).
	InDegreeCut int

	// HasIndex reports whether the bundle has a derived index at all. A bundle
	// freshly cloned has none, and in that state every document differs from the
	// index trivially — which is why the index-relative checks are skipped
	// rather than run against nothing.
	HasIndex bool
}

// Document is the subset of a document the checks examine.
//
// SchemaVersion is nil for a document that declares none, which is the state the
// schema-version check reports (§5.5.1.1).
type Document struct {
	ID            gnosis.ID
	Path          string
	Type          gnosis.TypeKey
	Title         string
	Body          string
	SchemaVersion *int

	// Claims are the document's declared claims and the evidence they name.
	// Empty for a document that declares none, which most Phase 2 documents do.
	Claims []Claim

	// StaleAfter is the date the author asked for this to be revisited by, or the
	// zero time when they declared none. An absolute date rather than a duration,
	// on OKF's determinism argument (§14.3): a date keeps the staleness decision a
	// plain comparison with no reference to when the document was read.
	StaleAfter time.Time

	// SourceKeys identify the source versions this document rests on, in the same
	// form checked.jsonl keys them. Empty for a document citing nothing, whose
	// freshness is not_applicable rather than unknown.
	SourceKeys []string

	// Limitations is what this concept declares it does not cover (§17.2). Empty for
	// a document declaring none.
	Limitations []string

	// Challenges are the contests readers have filed against this document
	// (§10.7.4), in declaration order — which is the order they were filed.
	Challenges []gnosis.Challenge

	// Evidence is everything this document's assertions rest on, with what each
	// one buys (§14.4). Empty for a document resting on nothing.
	//
	// **At the document grain, and §14.4's table is written at the claim grain.**
	// The reason is a limit of what frontmatter can currently say: a `referenced`
	// source has no archived text and therefore no `archive_paths` entry, so the
	// only place a claim's own evidence can name one is the document's OKF
	// `sources` list — which is per document. Until a claim can name a referenced
	// source, `partly-provable` and `unprovable` have no expressible population at
	// the claim grain, and computing them there would produce a signal that is
	// always `provable` or `not-applicable`. §12's own row for `durability` says
	// "concept", which is the same conclusion reached from the other direction.
	Evidence []Evidence
}

// Evidence is one thing a document rests on, and what it buys.
//
// A struct rather than parallel slices because the pair belongs together: a
// support with no source cannot be reported to anybody, and a source with no
// support cannot be folded. The URI is carried so a finding can name what to go
// and replace, which is the only action a `durability` finding has.
type Evidence struct {
	// URI is the source, as tier 0's record names it, or the resource string the
	// document declared when no record matches it.
	URI string

	// Support is what this source buys a claim that cites it. SupportNone for a
	// resource tier 0 has no record of, which contributes to no state — that gap
	// belongs to `gate:provenance` and to `archive-closure`.
	Support gnosis.Support
}

// Claim is the subset of a claim the checks examine: its identity, its address,
// and where it says its evidence lives.
type Claim struct {
	ID string

	// Anchor is the span of the document this claim addresses (§5.5.1), as the
	// frontmatter states it. Empty for a claim declaring none, which is not this
	// package's finding to report — an address that stopped resolving is.
	Anchor string

	// Lead is the claim's conclusion, stated first (§17.4), or empty when extraction
	// has written none. §17.4's check reads it; nothing else does yet.
	Lead string

	// Quotes are the passages this claim offers as evidence, as frontmatter declares
	// them.
	//
	// The passages rather than a count, because `coverage`'s finding has to say how
	// thin the evidence is and a number cannot be quoted back at a reader. Empty for
	// a claim offering none, which is a different state from a claim whose quotation
	// failed to validate — that one is the gate's.
	Quotes []string

	// Subject is the surface phrase naming what this claim is about, as the author
	// wrote it. Empty for a claim declaring none. It is the surface rather than the
	// resolved key because both readings are findings: an unresolvable phrase is
	// `subject-unknown`, and resolving it here would discard the evidence for that.
	Subject string

	// Warrant is the record of a human adjudication (§10.6.4), or the zero value for
	// a claim carrying none.
	//
	// **It is deliberately not on gate.Claim**, for the reason `Subject` is not: the
	// promote gate must not read a warrant field, and §10.6.2.1 makes that a
	// structural guarantee rather than a comment — the gate's inputs would have to be
	// visibly widened, which is a change a reviewer has to argue for.
	Warrant gnosis.Warrant

	// Supersedes are the claims this one replaced after an adjudicated conflict
	// (§10.4). Empty for a claim replacing nothing.
	Supersedes []string

	ArchivePaths []string
}

// Link is one link found in a body.
type Link struct {
	FromID gnosis.ID
	// ToID is empty when the href resolves to no document in the bundle.
	ToID gnosis.ID
	Href string
	// External is true for a link leaving the bundle; those are never orphans
	// or broken links as far as this corpus is concerned.
	External bool
}

// Check is one named health check.
//
// Applies reports whether the corpus exhibits the convention this check
// examines. When it returns false the check is skipped and the reason is
// surfaced, rather than the check running and reporting noise.
type Check struct {
	Name string

	// Categories are the finding.Diagnostic categories this check may emit.
	//
	// Declared rather than discovered, because the emitted vocabulary is **not
	// enumerable by inspection**: most categories are string literals inside a Run
	// body, but `identity` and `index-drift` come out of resolutionCategory, so a
	// grep for literals finds neither. That made §12's check table unverifiable
	// against the code — the one direction it can drift in without anybody noticing.
	//
	// A test walks the registry and asserts this set against the table, and asserts
	// that a check firing on a fixture emits only what it declared. The field is a
	// second place to remember, and what makes it survivable is that both directions
	// are checked: a category emitted and not declared fails, and a category
	// declared and absent from the spec fails too.
	Categories []string

	// Actions are the `finding.Action` values this check may attach to a
	// diagnostic: whether a tool could fix what it reports, or whether a person
	// must.
	//
	// Declared for the same reason Categories is, and it is the same kind of fact:
	// an action is a field set inside a Run body, so it is not enumerable by
	// inspection either — `identity` and `index-drift` both get theirs from
	// `diagnoseResolution`, which chooses between two depending on the resolution
	// kind.
	//
	// It exists because §12.1's table said what each check emits and not what a
	// reader could do about it. A finding a tool will fix and one that needs a
	// person are different work, and a table that cannot tell them apart makes a
	// reader open the code to find out.
	//
	// **Declaring an action is not promising a fixer.** There is no `--fix`, and the
	// column reports what a fixer *could* do. Building one is a much larger decision
	// than describing the possibility, and conflating them would ship it by accident.
	Actions []finding.Action

	Applies func(*Snapshot) (bool, string)
	Run     func(*Snapshot) []finding.Diagnostic
}

// Report is the outcome of a run.
type Report struct {
	Diagnostics []finding.Diagnostic `json:"diagnostics"`
	Skipped     []Skip               `json:"skipped,omitempty"`
}

// Skip records a check that did not run, and why.
type Skip struct {
	Check  string `json:"check"`
	Reason string `json:"reason"`
}

// Checks returns the Phase 1 registry, ordered by name.
// Checks returns the registry as of a moment.
//
// Requires: nothing.
// Ensures: every returned check has a non-empty Name, an Applies, and a Run.
// The slice is freshly built per call, so a caller cannot mutate the registry
// another caller will see.
//
// The clock is a parameter because one check needs it and a check that read the
// clock itself could not be tested for the boundary cases that matter — a document
// expiring today, and one expiring tomorrow.
func Checks(now time.Time) []Check {
	checks := []Check{
		conformanceCheck(),
		identityCheck(),
		indexDriftCheck(),
		brokenLinkCheck(),
		orphanCheck(),
		logFormatCheck(),
		schemaVersionCheck(),
		placeholderCheck(),
		emptySectionCheck(),
		archiveClosureCheck(),
		archivePathCheck(),
		claimAnchorCheck(),
		conflictCheck(),
		durabilityCheck(),
		warrantCheck(),
		coSignCheck(),
		unansweredChallengeCheck(now),
		coverageCheck(),
		rungCheck(),
		dimensionDriftCheck(),
		constraintDriftCheck(),
		constraintCoverageCheck(),
		commandCheck(),
		evidenceCheck(),
		languageCheck(),
		duplicateCheck(),
		filenameDriftCheck(),
		limitationsCheck(),
		leadCheck(),
		subjectMissingCheck(),
		subjectUnknownCheck(),
		ontologyCheck(),
		staleCheck(now),
	}
	sort.Slice(checks, func(i, j int) bool { return checks[i].Name < checks[j].Name })
	return checks
}

// Run executes every check whose Applies reports true.
//
// Requires: snap is fully populated by the caller; a zero Snapshot is valid and
// describes an empty corpus.
// Ensures: diagnostics are sorted, so two runs over one corpus are comparable.
// Every check that did not run appears in Skipped with a reason — a check that
// silently declines is indistinguishable from one that found nothing.
func Run(snap *Snapshot, checks []Check) Report {
	report := Report{Diagnostics: []finding.Diagnostic{}, Skipped: []Skip{}}
	for _, c := range checks {
		if ok, reason := c.Applies(snap); !ok {
			report.Skipped = append(report.Skipped, Skip{Check: c.Name, Reason: reason})
			continue
		}
		report.Diagnostics = append(report.Diagnostics, c.Run(snap)...)
	}
	finding.Sort(report.Diagnostics)
	return report
}

// always is the applicability of a check that is meaningful on any corpus,
// including an empty one.
func always(*Snapshot) (bool, string) { return true, "" }
