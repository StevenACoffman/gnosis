package standards

import (
	_ "embed"
)

// ArchiveFileName is where the archive gates live, relative to the bundle root.
const ArchiveFileName = "standards/archive.toml"

// defaultArchive is the seed, embedded rather than encoded from Go values because
// its comments carry the empirical basis a reviewer reads before changing a
// number. Marshalling an Archive back to TOML would drop every one of them.
//
//go:embed archive.toml
var defaultArchive []byte

// Archive is the admission policy of SPEC §4.3: which fetched sources become
// durable evidence, and which fall through to a bare reference.
//
// Every field is a Value, so no gate here can be moved without saying why.
type Archive struct {
	// Allowlist is the set of extensions archived directly. Everything else is
	// either extracted or referenced; the list is not a security boundary but a
	// weight budget.
	Allowlist Value[[]string] `toml:"allowlist"`

	// PerFileCap is the largest file archived directly, in bytes.
	PerFileCap Value[int64] `toml:"per_file_cap"`

	// CorpusBudget is the total archive size, in bytes, past which the repository
	// is considered to have grown without anyone deciding to let it.
	CorpusBudget Value[int64] `toml:"corpus_budget"`

	// CorpusWarnFraction is the share of CorpusBudget at which a warning is
	// reported, so the ceiling is never reached silently.
	CorpusWarnFraction Value[float64] `toml:"corpus_warn_fraction"`

	// EmbeddedPayloadCap is the largest data URI tolerated inside an archived
	// file, in bytes. Without it a base64 raster inside an SVG or a markdown file
	// reintroduces exactly the binary weight the allowlist excludes.
	EmbeddedPayloadCap Value[int64] `toml:"embedded_payload_cap"`

	// HTMLExtractor names the one pinned extractor, and Version pins it. Both are
	// recorded with every extracted record, so a re-extraction by a different
	// stripper is visible rather than silent (§4.2).
	HTMLExtractor        Value[string] `toml:"html_extractor"`
	HTMLExtractorVersion Value[string] `toml:"html_extractor_version"`

	// StalenessDays is the default window after which an unchecked source is
	// reported stale. A document may override it with an absolute date; this is
	// only the default for one that does not (§14.3).
	StalenessDays Value[int] `toml:"staleness_days"`

	// InDegreeCut is the inbound-link count above which a document is treated as
	// central, which raises the evidence a claim in it must carry (§14.4.1).
	InDegreeCut Value[int] `toml:"in_degree_cut"`
}

// DefaultArchive returns the gates a new bundle begins with.
//
// Requires: nothing.
// Ensures: the result is accepted by LoadArchive — pinned by a test, because a
// seed its own loader rejects would break every bundle created from it. The
// returned slice is a copy, so a caller cannot corrupt the seed for the next one.
func DefaultArchive() []byte {
	out := make([]byte, len(defaultArchive))
	copy(out, defaultArchive)
	return out
}

// LoadArchive parses and validates the archive gates.
//
// Requires: src is TOML.
// Ensures: returns EINVALID naming the problem — a syntax error, an unrecognised
// key, a value with no rationale, or a gate outside the range in which it means
// anything. On success every gate is populated and justified.
func LoadArchive(src []byte) (*Archive, error) {
	const op = "standards.LoadArchive"

	var a Archive
	if err := decode(op, src, &a); err != nil {
		return nil, err
	}
	if err := checkRationales(op, &a); err != nil {
		return nil, err
	}
	if err := a.validate(op); err != nil {
		return nil, err
	}
	return &a, nil
}
