package archive_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/archive"
)

// TestCleanSVGIsArchived: the format is on the allowlist because diagrams are
// useful evidence, and a scanner that refused the ordinary case would take it off
// the list in practice while leaving it on in the file.
func TestCleanSVGIsArchived(t *testing.T) {
	t.Parallel()
	clean := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">` +
		`<title>A box</title><rect x="1" y="1" width="8" height="8"/>` +
		`<use href="#glyph"/></svg>`)

	got := archive.Decide(&archive.Candidate{URI: "u", Bytes: clean, Extension: ".svg"}, gates())
	if got.Record.Disposition != archive.Archived {
		t.Fatalf("a clean SVG was refused: %q", got.Record.RejectReason)
	}
}

// TestActiveSVGIsRefused covers each rule in §4.4 by name. Sanitization is a
// rejection and never a rewrite: what is committed must be what was fetched, or
// the archived bytes stop being the bytes the quote was checked against.
func TestActiveSVGIsRefused(t *testing.T) {
	t.Parallel()
	const open = `<svg xmlns="http://www.w3.org/2000/svg">`
	cases := map[string]struct {
		body string
		want archive.RejectReason
	}{
		"script element": {
			open + `<script>alert(1)</script></svg>`, archive.ReasonSVGScript,
		},
		"event handler": {
			open + `<rect onload="alert(1)"/></svg>`, archive.ReasonSVGEventHandler,
		},
		"remote image": {
			open + `<image href="https://evil.example/x.png"/></svg>`, archive.ReasonSVGExternalRef,
		},
		"cross-document use": {
			open + `<use href="other.svg#g"/></svg>`, archive.ReasonSVGExternalRef,
		},
		"doctype": {
			`<!DOCTYPE svg><svg xmlns="http://www.w3.org/2000/svg"/>`, archive.ReasonSVGDoctype,
		},
		"entity declaration": {
			`<!DOCTYPE svg [<!ENTITY x SYSTEM "file:///etc/passwd">]>` + open + `</svg>`,
			archive.ReasonSVGDoctype,
		},
		"foreignObject": {
			open + `<foreignObject><body xmlns="http://www.w3.org/1999/xhtml"/></foreignObject></svg>`,
			archive.ReasonSVGForeignObject,
		},
		"malformed": {
			open + `<rect>`, archive.ReasonSVGMalformed,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := archive.Decide(&archive.Candidate{
				URI: "u", Bytes: []byte(tc.body), Extension: ".svg",
			}, gates())

			if got.Record.Disposition != archive.Referenced {
				t.Fatalf("an active SVG was archived: %+v", got.Record)
			}
			if got.Record.RejectReason != tc.want {
				t.Errorf("reason = %q, want %q", got.Record.RejectReason, tc.want)
			}
			if got.Content != nil {
				t.Error("a refused SVG produced content to write")
			}
		})
	}
}

// TestSVGScanIsNotTextual: the tokeniser has resolved namespace prefixes, case,
// and whitespace by the time an element is named, and a pattern match has not.
// Each of these defeats a `<script` grep and none defeats the parser.
func TestSVGScanIsNotTextual(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"namespace prefix": `<svg xmlns="http://www.w3.org/2000/svg" ` +
			`xmlns:s="http://www.w3.org/2000/svg"><s:script>x</s:script></svg>`,
		"mixed case":            `<svg xmlns="http://www.w3.org/2000/svg"><SCRIPT>x</SCRIPT></svg>`,
		"whitespace in the tag": "<svg xmlns=\"http://www.w3.org/2000/svg\"><script\n>x</script></svg>",
		"uppercase handler":     `<svg xmlns="http://www.w3.org/2000/svg"><rect ONCLICK="x"/></svg>`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := archive.Decide(&archive.Candidate{
				URI: "u", Bytes: []byte(body), Extension: ".svg",
			}, gates())
			if got.Record.Disposition == archive.Archived {
				t.Errorf("active content survived the scan: %s", body)
			}
		})
	}
}

// TestOversizePayloadInsideAnSVG: the allowlist admits SVG for being markup, and
// a base64 raster inside one is the binary weight that admission excludes.
func TestOversizePayloadInsideAnSVG(t *testing.T) {
	t.Parallel()
	body := `<svg xmlns="http://www.w3.org/2000/svg"><image href="#x" ` +
		`style="background:url(data:image/png;base64,` + strings.Repeat("A", 9000) + `)"/></svg>`

	got := archive.Decide(&archive.Candidate{
		URI: "u", Bytes: []byte(body), Extension: ".svg",
	}, gates())
	if got.Record.RejectReason != archive.ReasonEmbeddedPayload {
		t.Errorf("reason = %q, want the embedded payload", got.Record.RejectReason)
	}
}

// TestSmallDataURIIsFine: the cap exists for rasters, and refusing the small
// inline icons that legitimately appear would push real diagrams to `referenced`.
func TestSmallDataURIIsFine(t *testing.T) {
	t.Parallel()
	body := `<svg xmlns="http://www.w3.org/2000/svg"><desc>data:image/png;base64,AAAA</desc></svg>`

	got := archive.Decide(&archive.Candidate{
		URI: "u", Bytes: []byte(body), Extension: ".svg",
	}, gates())
	if got.Record.Disposition != archive.Archived {
		t.Errorf("a small inline payload was refused: %q", got.Record.RejectReason)
	}
}
