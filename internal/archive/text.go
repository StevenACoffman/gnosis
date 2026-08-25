package archive

import (
	"bytes"
	"unicode/utf8"
)

// IsText reports whether data may be treated as text.
//
// Requires: nothing.
// Ensures: true only for valid UTF-8 containing no NUL byte. This is a test, not a
// trust: SPEC §4.3 requires that a `.txt` which is really a binary be rejected as
// binary, whatever it is named, because the extension is the attacker's to choose
// and the bytes are not.
//
// Empty data is text. A zero-byte source is a strange thing to archive but it is
// not a binary, and reporting it as one would send a reader looking for a
// corruption that is not there.
func IsText(data []byte) bool {
	return !bytes.ContainsRune(data, 0) && utf8.Valid(data)
}

// hasOversizePayload reports whether data embeds a data URI longer than limit.
//
// Requires: limit is positive.
// Ensures: scans for the `data:` scheme and measures to the next delimiter. A
// payload above the limit reintroduces exactly the binary weight the allowlist
// excludes — a base64 raster inside an SVG or a markdown file — inside a file the
// allowlist admitted.
//
// This over-reports by design. A `data:` appearing in prose about data URIs, with
// a long unbroken run after it, is refused; the corpus loses one document to
// `referenced` and nobody is misled. Under-reporting would commit the raster.
func hasOversizePayload(data []byte, limit int64) bool {
	return largestPayload(data) > limit
}

// largestPayload measures the longest embedded data URI, or 0 when there is none.
//
// Requires: nothing.
// Ensures: the byte length of the longest payload found, whatever any limit is.
// Pure.
//
// It measures rather than compares so a refusal can say **how big**, which is the
// difference between a verdict an author can act on and one they can only argue
// with. `hasOversizePayload` is the comparison, and there is one measurement under
// both — a second traversal that counted differently would let the disposition and
// the explanation disagree about the same file.
//
// The longest rather than the first: a document with a small icon and a large raster
// is refused for the raster, and reporting the icon's size would send an author to
// edit the wrong line.
func largestPayload(data []byte) int64 {
	var largest int64
	rest := data
	for {
		i := bytes.Index(rest, []byte("data:"))
		if i < 0 {
			return largest
		}
		rest = rest[i+len("data:"):]
		if n := int64(payloadLen(rest)); n > largest {
			largest = n
		}
	}
}

// payloadLen measures to the first byte that cannot appear inside a data URI, so
// the surrounding markup or prose is not counted as payload.
func payloadLen(rest []byte) int {
	for i, b := range rest {
		switch b {
		case '"', '\'', '<', '>', ' ', '\t', '\n', '\r', ')':
			return i
		}
	}
	return len(rest)
}
