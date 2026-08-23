package bundle

import (
	"strconv"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/relay"
)

// renderQuarantined writes a checked reply as an OKF document.
//
// Requires: every claim in the reply passed the quote check.
// Ensures: the frontmatter carries what §5.5.1 requires to be recoverable **from
// the document alone** — each claim's assigned id, its anchor, its quotations, and
// the archived files those quotations were found in. The index is a derived cache
// (§4.5), so a claim address that lived only there would be lost by a rebuild and
// unavailable to anyone reading the file.
//
// The rendering is deterministic: no clock, no map iteration, no generated content
// beyond the identifiers the caller supplies. Two runs over one reply produce
// byte-identical documents, which is what lets the promote gate's preview mean
// anything.
//
// It is written by hand rather than through a YAML encoder for the reason §5.2
// gives: an encoder normalises quoting and key order and drops comments, so a
// round trip through one would not reproduce what was written. Here it also keeps
// the frontmatter's shape visible next to the spec section that requires it.
func renderQuarantined(id gnosis.ID, reply *relay.Reply, k *checked) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString("type: ")
	b.WriteString(reply.Type)
	b.WriteString("\ntitle: ")
	b.WriteString(yamlScalar(reply.Title))
	b.WriteString("\ngnosis_id: ")
	b.WriteString(id.String())
	b.WriteString("\ngnosis_schema_version: ")
	b.WriteString(strconv.Itoa(gnosis.SchemaVersion))
	b.WriteString("\n")
	writeSources(&b, reply)
	writeClaims(&b, reply)
	b.WriteString("---\n\n")

	b.WriteString("# ")
	b.WriteString(reply.Title)
	b.WriteString("\n\n")
	for i := range k.claims {
		b.WriteString(k.claims[i].Text)
		b.WriteString("\n\n")
	}
	return []byte(b.String())
}

// writeSources emits the OKF sources list.
func writeSources(b *strings.Builder, reply *relay.Reply) {
	b.WriteString("sources:\n")
	b.WriteString("  - resource: ")
	b.WriteString(yamlScalar(reply.SourceURI))
	b.WriteString("\n")
}

// writeClaims emits gnosis_claims: one entry per claim the reply offered, each
// carrying the address §5.5.1 requires.
//
// The anchor is the reply's own wording rather than a segmented part, because an
// anchor has to be findable in the document and the document is written from these
// same strings. Where segmentation split a claim, the parts appear in the body and
// the anchor still locates the passage they came from.
func writeClaims(b *strings.Builder, reply *relay.Reply) {
	if len(reply.Claims) == 0 {
		return
	}
	b.WriteString("gnosis_claims:\n")
	for i := range reply.Claims {
		c := &reply.Claims[i]
		b.WriteString("  - id: ")
		b.WriteString(c.ID)
		b.WriteString("\n    anchor: ")
		b.WriteString(yamlScalar(c.Text))
		b.WriteString("\n    gnosis_evidence:\n")
		for _, q := range c.Quotes {
			b.WriteString("      - ")
			b.WriteString(yamlScalar(q))
			b.WriteString("\n")
		}
		if len(c.ArchivePaths) > 0 {
			b.WriteString("    archive_paths:\n")
			for _, p := range c.ArchivePaths {
				b.WriteString("      - ")
				b.WriteString(p)
				b.WriteString("\n")
			}
		}
	}
}

// yamlScalar quotes a value so it survives the round trip.
//
// Double-quoted with escapes rather than folded or literal block style: a claim's
// anchor may contain a colon, a leading dash, or a `#`, any of which changes what
// an unquoted scalar means. Quoting unconditionally costs two characters and
// removes the question.
func yamlScalar(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
