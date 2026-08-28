package bundle

import (
	"strconv"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/relay"
)

// conceptDoc is a concept document as gnosis writes it.
//
// **One shape, two writers.** `admit` builds it from a checked reply and accretion
// builds it from an existing document plus new quotations; both render through
// `renderConcept`, so the frontmatter this corpus produces has one definition. Two
// renderers agreeing by inspection is the kind of claim §9.4 refuses elsewhere, and
// here it would fail silently — an accreted document that formatted its claims
// differently would read as an edit nobody made.
type conceptDoc struct {
	Type      string
	Title     string
	ID        gnosis.ID
	SourceURI []string

	// Claims are the addresses and their evidence, in the order they appear.
	Claims []conceptClaim

	// Paragraphs are the body, one per element, joined by a blank line.
	Paragraphs []string
}

// conceptClaim is one claim's frontmatter entry.
type conceptClaim struct {
	ID     string
	Anchor string

	// Lead is the claim's conclusion, stated first (§17.4). Empty when the reply
	// offered none, and then the key is omitted rather than written empty — an
	// empty `lead:` in frontmatter would assert that the claim has no conclusion,
	// which is a different thing from nobody having stated one (§5.5.3).
	Lead string

	Quotes       []string
	ArchivePaths []string
}

// renderConcept writes a concept document.
//
// Requires: doc.Type and doc.Title are non-empty.
// Ensures: deterministic — no clock, no map iteration, no generated content beyond
// what the caller supplies. Two calls over one value produce byte-identical output,
// which is what lets a preview mean anything. Pure.
//
// Written by hand rather than through a YAML encoder for the reason §5.2 gives: an
// encoder normalises quoting and key order and drops comments, so a round trip
// through one would not reproduce what was written.
func renderConcept(doc *conceptDoc) []byte {
	var b strings.Builder

	b.WriteString("---\ntype: ")
	b.WriteString(doc.Type)
	b.WriteString("\ntitle: ")
	b.WriteString(yamlScalar(doc.Title))
	b.WriteString("\ngnosis_id: ")
	b.WriteString(doc.ID.String())
	b.WriteString("\ngnosis_schema_version: ")
	b.WriteString(strconv.Itoa(gnosis.SchemaVersion))
	b.WriteString("\n")
	if len(doc.SourceURI) > 0 {
		b.WriteString("sources:\n")
		for _, uri := range doc.SourceURI {
			b.WriteString("  - resource: ")
			b.WriteString(yamlScalar(uri))
			b.WriteString("\n")
		}
	}
	writeConceptClaims(&b, doc.Claims)
	b.WriteString("---\n\n")

	b.WriteString("# ")
	b.WriteString(doc.Title)
	b.WriteString("\n\n")
	for _, p := range doc.Paragraphs {
		b.WriteString(p)
		b.WriteString("\n\n")
	}
	return []byte(b.String())
}

// writeConceptClaims emits gnosis_claims: one entry per claim, each carrying the
// address §5.5.1 requires to be recoverable from the document alone.
func writeConceptClaims(b *strings.Builder, claims []conceptClaim) {
	if len(claims) == 0 {
		return
	}
	b.WriteString("gnosis_claims:\n")
	for i := range claims {
		c := &claims[i]
		b.WriteString("  - id: ")
		b.WriteString(c.ID)
		b.WriteString("\n    anchor: ")
		b.WriteString(yamlScalar(c.Anchor))
		if c.Lead != "" {
			b.WriteString("\n    lead: ")
			b.WriteString(yamlScalar(c.Lead))
		}
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
	doc := conceptDoc{
		Type: reply.Type, Title: reply.Title, ID: id,
		SourceURI: []string{reply.SourceURI},
		Claims:    make([]conceptClaim, 0, len(reply.Claims)),
	}
	for i := range reply.Claims {
		c := &reply.Claims[i]
		// The anchor is the reply's own wording rather than a segmented part: an
		// anchor has to be findable in the document, and the document is written
		// from these same strings. Where segmentation split a claim, the parts
		// appear in the body and the anchor still locates the passage.
		doc.Claims = append(doc.Claims, conceptClaim{
			ID: c.ID, Anchor: c.Text, Lead: c.Lead,
			Quotes: c.Quotes, ArchivePaths: c.ArchivePaths,
		})
	}
	for i := range k.claims {
		doc.Paragraphs = append(doc.Paragraphs, k.claims[i].Text)
	}
	return renderConcept(&doc)
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
