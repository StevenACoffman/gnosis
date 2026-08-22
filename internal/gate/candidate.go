package gate

// The seven signals of SPEC §9.5.
//
// SignalUnset is the zero value and names no signal, so a Result nobody populated
// cannot be mistaken for a conclusion about `evidence`.
const (
	SignalUnset Signal = ""

	// SignalEvidence: at least one validating quotation per enforced claim.
	SignalEvidence Signal = "evidence"

	// SignalProvenance: every source resolvable, or a declared scope descriptor.
	SignalProvenance Signal = "provenance"

	// SignalConformance: OKF §11 satisfied and `type` non-empty.
	SignalConformance Signal = "conformance"

	// SignalDuplication: no near-identical document by fold-normalised title.
	SignalDuplication Signal = "duplication"

	// SignalHedging: softening phrases below the declared count.
	SignalHedging Signal = "hedging"

	// SignalConflict: no open error-severity finding. Unimplementable until §10.
	SignalConflict Signal = "conflict"

	// SignalSecurity: admission scan clean. Unimplementable until §9.3.
	SignalSecurity Signal = "security"
)

// Signal names one of the gate's checks.
type Signal string

// Candidate is the diff the gate approves, and the diff the writer applies.
//
// It is a diff rather than a document because §9.4 requires the bytes checked to
// be the bytes written. Before and After are carried together so a signal that
// needs to know what is changing — rather than only what it is changing to — has
// it without a second read.
type Candidate struct {
	// Path is the destination in the bundle, relative to its root.
	Path string

	// Before is the document currently at Path, or nil when there is none. A
	// promotion of new knowledge and a revision of existing knowledge are
	// different events, and only this distinguishes them.
	Before []byte

	// After is exactly what will be written. The writer writes these bytes and
	// does not re-read the source.
	After []byte

	// Scan is what §9.3's admission scan found in After, and which of its stages
	// ran. Supplied by the shell for the same reason Doc is parsed there:
	// `internal/scan` is a sibling adapter.
	//
	// The zero value reports no findings and no stages, which the security signal
	// reads as Unchecked rather than clean — a candidate nobody scanned must not
	// pass the signal that exists to notice that.
	Scan Scan

	// Doc is After, parsed by the caller.
	//
	// The parse belongs to the caller because `internal/okf` is a sibling adapter
	// and adapters do not import each other (PLAN §0.1). The cost is this struct;
	// the benefit is that every signal here is testable from a literal.
	Doc Document
}

// Scan is the §9.3 admission scan's result for a candidate's bytes.
//
// It is a value rather than a call into `internal/scan` because adapters do not
// import each other, and the shape is deliberately narrower than that package's:
// this signal needs to know that something was found and what stages looked, not
// the codepoint of every occurrence. A caller wanting the detail has the scan
// package.
type Scan struct {
	// Findings is one rendered description per finding, already in a form a
	// person can read. Empty means the stages that ran found nothing, which is
	// not the same as clean — see Coverage.
	Findings []string

	// StagesRun and StagesMissing are §9.3's coverage. A scan with anything
	// missing cannot support a pass, however clean it came back.
	StagesRun     []string
	StagesMissing []string
}

// Document is the parsed view of After that the signals examine.
type Document struct {
	// Type is the OKF concept type. Empty fails conformance.
	Type string

	// Title is the human title, unnormalised. The duplication signal folds it.
	Title string

	// Body is the prose, frontmatter removed. The hedging signal reads it.
	Body string

	// Claims are the segmented claims this document asserts.
	Claims []Claim

	// Sources are the document's `sources[]` entries.
	Sources []Source
}

// Claim is one assertion and the evidence offered for it.
type Claim struct {
	// ID is the claim's assigned identifier, for naming it in a finding.
	ID string

	// Text is the claim as it will be verified.
	Text string

	// Enforced reports whether this claim's evidence is gated. A claim that is
	// not enforced still records its quotations; it simply does not block.
	Enforced bool

	// Quotes are the verbatim passages offered as evidence.
	Quotes []string

	// ArchivePaths are the tier-0 files those quotations should appear in. A
	// claim naming no archive path has no offline proof and cannot pass the
	// evidence signal, which is the `referenced` disposition surfacing here.
	ArchivePaths []string
}

// Source is one `sources[]` entry.
type Source struct {
	// Resource is the URI, or the scope descriptor when Scope is set.
	Resource string

	// Scope marks a source OKF §5.1 permits a consumer not to dereference — "a
	// population or scope descriptor it cannot" follow. Such a source satisfies
	// provenance by declaring itself unfollowable, which is honest; a URI that
	// merely happens to be absent from tier 0 does not.
	Scope bool
}
