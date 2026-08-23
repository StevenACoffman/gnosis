package bundle

import (
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/skillet/errs"
)

// ExtractorName identifies the one pinned extractor, and must match the value in
// standards/archive.toml. It is recorded on every extracted record so that a
// re-extraction by a different stripper is visible rather than silent (§4.2).
const ExtractorName = "JohannesKaufmann/html-to-markdown"

// ExtractorVersion pins it. An unversioned extractor is worse than none: every
// record would claim a provenance indistinguishable from the next release's.
const ExtractorVersion = "v2.5.2"

// Extract attaches a text extraction to a candidate that needs one.
//
// Requires: c is non-nil.
// Ensures: c.Extraction is set only when this candidate is HTML and the
// conversion produced non-empty text. Everything else is left alone and will fall
// to `referenced`, which is a supported outcome rather than a failure (§4.3) —
// there is deliberately no PDF extractor, and a format with no extractor is not
// an error.
//
// The conversion strips boilerplate. That is the point and it is also the risk:
// what reaches the archive is not what the server sent, so the proof becomes
// "this quote appears in the text we extracted from a source whose bytes hashed
// to X". Recording the extractor's identity is what keeps that statement checkable
// rather than merely asserted.
func Extract(c *archive.Candidate) error {
	const op = "bundle.Extract"

	if !isHTML(c) {
		return nil
	}
	md, err := htmltomarkdown.ConvertString(string(c.Bytes))
	if err != nil {
		// A conversion failure is not a fetch failure. The source is still
		// recorded — as `referenced`, with the reason its own gates produced.
		return &errs.Error{Op: op, Err: err}
	}
	if strings.TrimSpace(md) == "" {
		// An empty extraction is worse than none: it would archive a file no
		// quote can match and report the disposition as durable.
		return nil
	}
	c.Extraction = &archive.Extraction{
		Text:             []byte(md),
		Extractor:        ExtractorName,
		ExtractorVersion: ExtractorVersion,
		Extension:        ".md",
	}
	return nil
}

// isHTML reports whether a candidate should go through the extractor.
//
// Both the media type and the extension are consulted, and either suffices. A
// served page frequently has no extension in its path, and a saved page
// frequently has no media type; requiring both would silently skip the common
// cases in opposite directions.
func isHTML(c *archive.Candidate) bool {
	switch c.Extension {
	case ".html", ".htm":
		return true
	}
	return c.MediaType == "text/html" || c.MediaType == "application/xhtml+xml"
}
