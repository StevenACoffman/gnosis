package archive

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// sanitizeSVG reports why an SVG may not be archived, or "" when it may.
//
// SVG is on the allowlist because diagrams are genuinely useful evidence and are
// diffable text. It is also XML, which makes it the one allowed format that can
// attack a reader — so §4.4 requires that sanitization be a **rejection and never
// a rewrite**. A file that would need stripping is refused, and what is committed
// is exactly what was fetched. Rewriting would break the one property tier 0 sells:
// that the archived bytes are the bytes the quote was checked against.
//
// Requires: nothing.
// Ensures: pure, and fails closed. Malformed XML is refused rather than
// best-effort parsed, because a document two parsers disagree about is a document
// whose rendered form is not the one that was scanned.
//
// This walks the token stream rather than matching patterns. A regex for
// `<script` is defeated by an entity, a namespace prefix, or unusual whitespace;
// the tokeniser has already resolved all three by the time an element is named.
func sanitizeSVG(data []byte) RejectReason {
	dec := xml.NewDecoder(bytes.NewReader(data))
	// Go's decoder does not resolve external entities, but a DOCTYPE is refused
	// outright below, so no entity table is consulted either way.
	dec.Strict = true

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return ReasonNone
		}
		if err != nil {
			return ReasonSVGMalformed
		}
		if reason := inspect(tok); reason != ReasonNone {
			return reason
		}
	}
}

// inspect reports why one token is unacceptable, or "" when it is not.
func inspect(tok xml.Token) RejectReason {
	switch t := tok.(type) {
	case xml.Directive:
		// A Directive is a `<!…>`: DOCTYPE, and with it entity declarations —
		// XXE and the billion-laughs expansion. There is no legitimate use of one
		// in a diagram.
		return ReasonSVGDoctype
	case xml.StartElement:
		return inspectElement(&t)
	default:
		return ReasonNone
	}
}

// inspectElement applies the element and attribute rules of §4.4.
func inspectElement(el *xml.StartElement) RejectReason {
	switch strings.ToLower(el.Name.Local) {
	case "script":
		return ReasonSVGScript
	case "foreignobject":
		// foreignObject embeds arbitrary HTML inside the SVG, which puts the whole
		// HTML attack surface back inside a format admitted for being markup.
		return ReasonSVGForeignObject
	}
	for i := range el.Attr {
		if reason := inspectAttr(&el.Attr[i]); reason != ReasonNone {
			return reason
		}
	}
	return ReasonNone
}

// inspectAttr applies the attribute rules: no event handlers, no external
// references.
func inspectAttr(attr *xml.Attr) RejectReason {
	name := strings.ToLower(attr.Name.Local)
	if strings.HasPrefix(name, "on") {
		return ReasonSVGEventHandler
	}
	// href and xlink:href cover <use> across documents and <image> with a remote
	// URI, which is why they are not separate rules: both are the same fact about
	// the same attribute.
	if name == "href" && !isFragment(attr.Value) {
		return ReasonSVGExternalRef
	}
	return ReasonNone
}

// isFragment reports whether a reference stays inside this document.
//
// Only a `#`-prefixed target qualifies. A relative path is not treated as local:
// the archive is served from a path with no authenticated origin (§4.4), but a
// relative reference still reaches whatever else is stored beside it, and nothing
// in a diagram needs to.
func isFragment(v string) bool {
	return strings.HasPrefix(strings.TrimSpace(v), "#")
}
