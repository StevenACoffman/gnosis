package archive

// The three dispositions of SPEC §4.3, and the evidence durability each buys.
//
// DispositionUnset is the zero value and asserts nothing. It exists because the
// alternative — a zero value meaning Archived — would have an unpopulated record
// claim the strongest durability in the set, which is the one mistake this type
// must not make. Decide never returns it.
const (
	DispositionUnset Disposition = ""

	// Archived: text-like, on the allowlist, under the cap. Durable — the quote
	// validates offline, forever.
	Archived Disposition = "archived"

	// Extracted: binary or oversize, but a text extraction passes. Durable
	// against the extraction, which is what a quote was ever checked against.
	Extracted Disposition = "extracted"

	// Referenced: neither the source nor an extraction can be archived. Weak —
	// hash and URI only, no offline proof. A first-class, non-failing outcome:
	// OKF §5.1 already contemplates a source a consumer cannot dereference, and
	// what gnosis adds is that the weakness is visible per claim rather than
	// averaged away (§14.4).
	Referenced Disposition = "referenced"
)

// The reasons a source may not be archived directly.
//
// ReasonNone is the zero value and, as with DispositionUnset, asserts nothing: an
// empty reason on a Referenced record would be a record that stored nothing and
// declines to say what it could not store.
const (
	ReasonNone RejectReason = ""

	// ReasonExtension: not on the allowlist. The common case, and not a fault.
	ReasonExtension RejectReason = "extension-not-allowed"

	// ReasonOversize: above the per-file cap.
	ReasonOversize RejectReason = "oversize"

	// ReasonBinary: failed the text test. Reported even for an allowed extension,
	// because the label is not trusted.
	ReasonBinary RejectReason = "binary"

	// ReasonEmbeddedPayload: a data URI above the embedded-payload cap, which
	// reintroduces the binary weight the allowlist excludes inside a file the
	// allowlist admitted.
	ReasonEmbeddedPayload RejectReason = "embedded-payload"

	// ReasonHiddenCharacters: text carrying characters no reviewer can see
	// (§9.3). Ingested text is text an agent will obey, and instructions written
	// in zero-width or bidi-override characters are invisible in every editor a
	// reviewer might open the document in.
	ReasonHiddenCharacters RejectReason = "hidden-characters"

	// The SVG rejections of §4.4. Sanitization is a refusal and never a rewrite,
	// so what is committed is what was fetched.
	ReasonSVGScript        RejectReason = "svg-script"
	ReasonSVGEventHandler  RejectReason = "svg-event-handler"
	ReasonSVGExternalRef   RejectReason = "svg-external-reference"
	ReasonSVGDoctype       RejectReason = "svg-doctype"
	ReasonSVGForeignObject RejectReason = "svg-foreign-object"
	ReasonSVGMalformed     RejectReason = "svg-malformed"
)

// Disposition is what became of a fetched source.
type Disposition string

// RejectReason says why a source was not archived directly. It is recorded on
// every Referenced record, and on no Archived one.
type RejectReason string

// Durable reports whether a quote can still be validated offline.
//
// Requires: nothing.
// Ensures: false for DispositionUnset, so an unpopulated record never reads as
// proof.
func (d Disposition) Durable() bool {
	return d == Archived || d == Extracted
}
