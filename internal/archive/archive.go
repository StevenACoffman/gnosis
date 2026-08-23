// Package archive implements tier 0: the content-addressed evidence store and
// the fetch ledger (SPEC §4.2–4.3.1).
//
// The property tier 0 exists to buy is stated in §4.1. Validating a quote against
// the *live* source is a genuine property that is not a durable one — a moved PDF
// or a 404 leaves the quote on disk and the proof gone. Keeping what the quote was
// validated against turns one conflated failure into two with opposite responses:
// a quote absent from the archived text is fabrication and blocks, while archived
// text differing from current upstream is a source that moved and only flags.
//
// Everything here is pure. Decide reports what should be written and where; the
// caller writes it. That split is what lets the admission policy be tested from
// literals, and it is why the gates arrive as a Gates value rather than being read
// from `standards/` here: this package states what it needs, `standards` states
// what is on disk, and the shell joins them.
//
// Two invariants hold across the package and both are load-bearing:
//
// A record's name is the hash of its own bytes. sha256 of the file at
// evidence/fetch/ab/ab….json is ab…, checkable with shasum and no tooling. A
// rewritten record therefore lands at a different path, which makes append-only
// structural rather than conventional — the reason §4.3.1 chose one file per
// record over a shared ledger that git's union driver would merge silently.
//
// No record carries a timestamp. Tier 0 grows when the corpus learns something,
// not when somebody checks: a re-fetch of unchanged bytes produces the same path,
// finds the record present, and writes nothing. When we last looked is an
// observation and lives in .gnosis/checked.jsonl, per §10.7.4's rule that
// decisions are committed and observations are cached.
package archive

// Gates is the admission policy this package needs, as values.
//
// It deliberately mirrors part of standards.Archive rather than importing it:
// adapters do not import each other (PLAN §0.1), and the duplication is the price
// of stating the dependency as data. The shell builds one from the loaded
// standards file.
type Gates struct {
	// Allowlist is the set of lower-cased extensions archived directly.
	Allowlist []string

	// PerFileCap is the largest file archived directly, in bytes.
	PerFileCap int64

	// EmbeddedPayloadCap is the largest data URI tolerated inside an archived
	// file, in bytes.
	EmbeddedPayloadCap int64

	// ScanText reports why text may not be admitted, or "" when it may. It is
	// §9.3's admission scan, supplied rather than imported: `internal/scan` is a
	// sibling adapter and adapters do not import each other (PLAN §0.1), so this
	// package states what it needs and the shell joins them — the same shape the
	// gates themselves take.
	//
	// It is a function rather than data because what it decides is not a
	// threshold. A codepoint either is or is not U+202E, and there is no value
	// this struct could carry that would let a caller express that.
	//
	// **A nil ScanText means no scan, which fails open**, and that is the one
	// fail-open default in this package. It is here because the alternative — a
	// zero Gates that refuses everything — would make every test and every caller
	// that legitimately does not scan carry a stub. The shell always supplies one
	// and a test asserts the wiring rather than trusting it.
	ScanText func(string) RejectReason
}

// Candidate is one fetched source as the caller found it.
type Candidate struct {
	// URI is the source as fetched, recorded whatever the disposition.
	URI string

	// Bytes is what was fetched. It is never written to the archive as-is unless
	// the disposition is `archived`.
	Bytes []byte

	// MediaType is the type the fetch reported, or empty when it reported none.
	// It is provenance, not a gate: the text test decides, not the label.
	MediaType string

	// Extension is the source's extension including the dot, lower-cased, or
	// empty. Derived by the caller from the URI or filename.
	Extension string

	// Extraction is the text a pinned extractor recovered, or nil when none
	// applies. A PDF has none by design (§4.3), which is why it falls to
	// `referenced` rather than failing.
	Extraction *Extraction
}

// Extraction is text recovered from a source that could not be archived directly,
// together with the identity of what recovered it.
//
// Extraction fidelity is a trusted step — the proof becomes "this quote appears in
// the text we extracted from a source whose bytes hashed to X" — so the extractor
// and its version are recorded per file, and a re-extraction by a different
// stripper is visible rather than silent (§4.2).
type Extraction struct {
	Text             []byte
	Extractor        string
	ExtractorVersion string

	// Extension is the extraction's own extension, `.md` or `.txt` (§4.3).
	Extension string
}

// Outcome is what Decide concluded: the record to commit, and the bytes to store
// under it.
type Outcome struct {
	// Record is the ledger entry. It is always produced, including for
	// `referenced`, because for exactly the fetches that archive nothing the
	// ledger is the only record (§4.3.1).
	Record Record

	// Content is the bytes to write at Record.ArchivePath, and is nil precisely
	// when Record.Disposition is Referenced.
	Content []byte
}

// Decide applies SPEC §4.3's admission policy.
//
// Requires: c is non-nil and c.Bytes is what was fetched; g is loaded. c is not
// modified.
// Ensures: exactly one of the three dispositions, chosen by the rules and never
// by a caller. The result is a pure function of (c, g), so two users fetching one
// source produce byte-identical records — which is what makes the ledger merge
// (§4.6.1). A rejected source is never an error: it records why and falls through
// to Referenced, which is a supported outcome rather than a failure.
func Decide(c *Candidate, g Gates) Outcome {
	rec := &Record{
		URI:          c.URI,
		SourceSHA256: hashHex(c.Bytes),
		ByteSize:     int64(len(c.Bytes)),
		MediaType:    c.MediaType,
	}

	reason := admits(c.Extension, c.Bytes, g)
	if reason == "" {
		rec.Disposition = Archived
		rec.ArchivePath = TextPath(rec.SourceSHA256, c.Extension)
		return Outcome{Record: *rec, Content: c.Bytes}
	}
	return fallBack(rec, c, g, reason)
}

// fallBack tries extraction, and settles for a bare reference when that is all
// there is. It carries forward why the direct archive was refused, so a record
// that stored nothing still says what it could not store.
func fallBack(rec *Record, c *Candidate, g Gates, why RejectReason) Outcome {
	rec.Disposition = Referenced
	rec.RejectReason = why

	if c.Extraction == nil {
		return Outcome{Record: *rec}
	}
	ex := c.Extraction
	if reason := admits(ex.Extension, ex.Text, g); reason != "" {
		// The extraction failed its own gates. The recorded reason is the
		// extraction's, not the source's: it is the more specific fact, and the
		// source's failure is implied by there having been an extraction at all.
		rec.RejectReason = reason
		return Outcome{Record: *rec}
	}
	rec.Disposition = Extracted
	rec.RejectReason = ""
	rec.ArchivePath = TextPath(hashHex(ex.Text), ex.Extension)
	rec.Extractor = ex.Extractor
	rec.ExtractorVersion = ex.ExtractorVersion
	rec.ExtractedFrom = rec.SourceSHA256
	return Outcome{Record: *rec, Content: ex.Text}
}

// admits reports why data may not be archived directly, or "" when it may.
//
// The order is deliberate: cheap structural tests before the content scans, and
// the extension test first because it is the one a reader will have predicted.
func admits(ext string, data []byte, g Gates) RejectReason {
	switch {
	case !allowed(ext, g.Allowlist):
		return ReasonExtension
	case int64(len(data)) > g.PerFileCap:
		return ReasonOversize
	case !IsText(data):
		// The label is not trusted: a .txt that is really a binary is rejected as
		// binary, whatever it is named (§4.3).
		return ReasonBinary
	case hasOversizePayload(data, g.EmbeddedPayloadCap):
		return ReasonEmbeddedPayload
	case ext == ".svg":
		return sanitizeSVG(data)
	}
	// Last, and only over text: §9.3 runs before any model sees the content, and
	// scanning bytes that failed the text test would be scanning noise.
	if g.ScanText != nil {
		return g.ScanText(string(data))
	}
	return ""
}

// allowed reports whether ext is on the allowlist.
func allowed(ext string, allowlist []string) bool {
	for _, a := range allowlist {
		if ext == a {
			return true
		}
	}
	return false
}
