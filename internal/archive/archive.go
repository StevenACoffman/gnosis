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

import "strconv"

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
	// # A nil ScanText refuses, and it used to admit
	//
	// **This was the one fail-open default in this package**, justified on the
	// grounds that the alternative made every caller carry a stub — so a nil meant
	// "no scan" and the source was admitted unexamined, with a single test standing
	// between the shell and no §9.3 at all.
	//
	// That stopped being defensible when the candidate path was built the other way:
	// a nil ruleset there degrades toward *more* blocking, reports the stages it
	// could not run, and routes the document to a person. Two halves of one security
	// stage failing in opposite directions is worse than either choice made twice.
	//
	// So a nil now refuses with ReasonUnscanned, and a caller that genuinely does not
	// scan says so with NoScan. The stub the old default avoided is one identifier,
	// and it is visible in the code where a nil was invisible.
	ScanText func(string) RejectReason
}

// Bound is what a size check found, and what it was measured against.
//
// It carries the measurement because a refusal that names only a reason is one an
// author can argue with and not act on: `embedded-payload` says nothing about which
// payload or how big, and the obvious next move is to argue the cap down. A refusal
// that says "9,012 bytes against a cap of 8,192" is one somebody truncates an example
// to satisfy.
//
// The zero value asserts nothing: ReasonNone and no measurement, which is the honest
// reading of a check that has not run.
type Bound struct {
	// Reason is why the artifact exceeded a bound, or ReasonNone.
	Reason RejectReason

	// Found is the measurement that exceeded, in bytes — the artifact's own size for
	// ReasonOversize, and the longest embedded payload for ReasonEmbeddedPayload.
	Found int64

	// Limit is the declared cap Found exceeded, in bytes.
	Limit int64
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

	// Revision is where the source stood in its own history when it was read —
	// today only a git commit, and empty for every other adapter.
	//
	// **It is provenance for the person doing the fetch and it is never part of a
	// record.** `Decide` does not read it, and a test asserts that the record's
	// canonical bytes are identical with and without it, because the temptation to
	// add it is obvious and §4.3.1 is what it would break: a record's name is the
	// hash of its own content, so a field that varies with the *repository's*
	// activity would make one unrelated push re-record every file in the tree,
	// identical to its predecessor but for a hash nobody reads. Tier 0 grows when
	// the corpus learns something, not when somebody pushes.
	//
	// That is the same argument §20.6 makes for leaving the commit out of the URI,
	// and it applies here for the same reason: the commit is not a property of the
	// bytes.
	Revision string
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
		SourceSHA256: SourceHash(c.Bytes),
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
	if g.ScanText == nil {
		// Fail closed. A source admitted because nobody wired a scanner is exactly
		// the outcome §9.3 exists to prevent, and it is not recoverable afterwards:
		// tier 0 is append-only.
		return ReasonUnscanned
	}
	return g.ScanText(string(data))
}

// NoScan is the explicit opt-out from §9.3's scan, for a caller that has decided not
// to run it.
//
// Requires: nothing.
// Ensures: ReasonNone for any text. Pure.
//
// It exists so that "this Gates does not scan" is a statement in the code rather
// than the absence of one. A nil ScanText refuses; naming NoScan is how a caller
// says it means to skip, and a reader grepping for it finds every place that does.
//
// A function rather than a package variable, so it is neither mutable global state
// nor something another package could reassign.
func NoScan(string) RejectReason { return ReasonNone }

// Exceeded reports whether a bound was passed.
//
// Requires: nothing; the zero Bound has exceeded nothing.
// Ensures: true exactly when Reason is set, so a caller branches on one thing rather
// than on a reason string it has to know the vocabulary of.
func (b Bound) Exceeded() bool { return b.Reason != ReasonNone }

// Detail renders the measurement for a reader, or "" when nothing was exceeded.
//
// Requires: nothing.
// Ensures: one sentence naming the reason, the measurement, and the cap. Pure.
//
// It is a method rather than formatting at each call site for the reason
// `scan.Describe` is one function: the promote gate and `fetch` both report this, and
// two renderings would let them describe one file two ways.
//
// It does **not** repeat the reason token, and that was found by running the command:
// `fetch` prints the reason on its own line and the finding indented beneath it, so a
// Detail beginning "embedded-payload:" produced the token twice in three lines. The
// prose stands alone instead — "an embedded payload is 9017 bytes against a declared
// cap of 8192" needs no label, where a scan finding like "zero-width U+200B" is not a
// sentence and does.
func (b Bound) Detail() string {
	if !b.Exceeded() {
		return ""
	}
	what := "the document"
	if b.Reason == ReasonEmbeddedPayload {
		what = "an embedded payload"
	}
	return what + " is " + strconv.FormatInt(b.Found, 10) +
		" bytes against a declared cap of " + strconv.FormatInt(b.Limit, 10)
}

// Oversize applies §9.3 stage 4's two declared bounds to text, and is the only
// exported way to ask that question.
//
// Requires: data is the whole artifact being bounded; the caps are the values from
// `standards/archive.toml`. A non-positive cap disables its own check, which is what
// a caller that has not loaded standards holds.
// Ensures: a Bound naming which cap was exceeded and by how much, or the zero Bound.
// Pure.
//
// # Why this exists and what it does not duplicate
//
// §9.3 stage 4 is "oversize / binary, bounded, with the bound in `standards/`", and
// until now the bound existed for a *fetched source* and not for a *candidate
// document* — the archive gate bounds what arrives from upstream, and a document a
// model wrote is neither fetched nor archived. The resolution is to apply the **same
// declared caps** to the candidate rather than to invent a second bound, which is
// what §6.5 forbids.
//
// The substantive half — finding a data URI and measuring it — is
// `hasOversizePayload`, which this calls and `admits` calls. There is one
// implementation of it. What is written twice is the length comparison, and that is
// deliberate: `admits` checks the file cap *before* the text test and the payload cap
// *after* it, because a binary must be reported as binary whatever its size, and
// folding those two cases into one call would lose the ordering that makes the reason
// correct. A one-line comparison stated in two places with different neighbours is
// cheaper than an ordering nobody can see.
//
// The two caps are adjacent int64s and nothing in the signature stops a caller
// swapping them, so a test distinguishes them: a payload under the payload cap but
// over the file cap, and the converse.
func Oversize(data []byte, perFileCap, embeddedPayloadCap int64) Bound {
	if size := int64(len(data)); perFileCap > 0 && size > perFileCap {
		return Bound{Reason: ReasonOversize, Found: size, Limit: perFileCap}
	}
	// The guard is not symmetry for its own sake. The payload measurement compares
	// against the limit it is given, so at zero every data URI exceeds it — which
	// for a caller holding no standards would report a document about icons as an
	// oversize payload. A disabled cap must disable its check.
	if embeddedPayloadCap > 0 {
		if found := largestPayload(data); found > embeddedPayloadCap {
			return Bound{
				Reason: ReasonEmbeddedPayload, Found: found, Limit: embeddedPayloadCap,
			}
		}
	}
	return Bound{}
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
