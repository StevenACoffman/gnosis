// Package okf parses and renders Open Knowledge Format v0.2 concept documents.
//
// The package is pure: every function takes bytes or values and returns values.
// Nothing here touches the filesystem, and the caller owns all I/O.
//
// Two properties drive the design and are easy to lose in a later change.
//
// Round-trip. Render(Parse(x)) must equal x byte for byte, because gnosis
// rewrites filenames and links in place and must not disturb anything else. That
// is achieved by retaining the frontmatter block verbatim rather than
// re-encoding it: every YAML encoder normalises quoting, indentation, and key
// order, and comments do not survive a decode at all. A document is therefore
// re-emitted from the bytes it arrived in, and only a deliberate mutation
// rewrites the block. Phase 1 performs no mutation, so no encoder exists here
// yet — one arrives with the first writer that needs it.
//
// Permissiveness. OKF §11 requires a consumer NOT to reject a document for a
// missing optional family, an unknown type, an unknown key, a broken link, or a
// bare `verified` mapping. Those are the conditions a well-meaning contributor
// is most likely to "fix" into non-conformance, so each has a test asserting
// acceptance rather than a comment asking for restraint.
package okf

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/frontmatter"
)

// requiredKey is the one frontmatter field OKF §4.1 makes mandatory. A concept
// carrying only this key is fully conformant.
const requiredKey = "type"

// Document is a parsed OKF concept.
//
// Fields carries every frontmatter key, including ones gnosis does not
// understand, so a reader can consult them. Rendering does not use Fields — see
// the package comment — which is why an unknown key cannot be lost by a
// round trip even if it decodes to a shape gnosis has no type for.
type Document struct {
	Fields map[string]any
	Body   string

	// block is the frontmatter exactly as it arrived, without its delimiters.
	// Render re-emits it unchanged.
	block string
}

// Parse reads an OKF concept document.
//
// Requires: src is UTF-8 text.
// Ensures: returns EINVALID only when the document has no frontmatter block,
// the block is unparsable YAML, or the required `type` key is absent or empty.
// No other condition is grounds for rejection — see OKF §11.
func Parse(src []byte) (*Document, error) {
	const op = "okf.Parse"

	block, body := frontmatter.Split(string(src))
	if block == "" {
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": document has no frontmatter block",
		}
	}

	fields := map[string]any{}
	if err := yaml.Unmarshal([]byte(block), &fields); err != nil {
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": frontmatter is not valid YAML",
		}
	}

	// YAML 1.2 still coerces unquoted numeric-looking scalars: `type: 1.20`
	// decodes to float64(1.2), and `type: 0755` to an integer. Reporting those
	// as a missing key would send a reader looking for something that is
	// plainly present, so the diagnostic distinguishes absent from unquoted.
	raw, present := fields[requiredKey]
	t, isText := raw.(string)
	switch {
	case !present || raw == nil:
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": frontmatter must carry a " + requiredKey,
		}
	case !isText:
		return nil, &errs.Error{
			Code: errs.EINVALID,
			Message: fmt.Sprintf(
				"%s: %s decoded as %T, not text — quote the value", op, requiredKey, raw),
		}
	case t == "":
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": " + requiredKey + " must not be empty",
		}
	}

	return &Document{Fields: fields, Body: body, block: block}, nil
}

// Render writes the document back out.
//
// Requires: doc came from Parse.
// Ensures: the result equals the bytes Parse was given, with one deliberate
// exception — CRLF line endings are normalised to LF, because skillet's
// frontmatter splitter normalises them before the block can be captured. That
// normalisation is the family's, not this package's, and a document written on
// Windows therefore round-trips to its LF form rather than to itself.
func Render(doc *Document) []byte {
	out := make([]byte, 0, len(doc.block)+len(doc.Body)+8)
	out = append(out, "---\n"...)
	out = append(out, doc.block...)
	// Split returns the block without its trailing newline; the closing
	// delimiter must start a line of its own.
	if !strings.HasSuffix(doc.block, "\n") {
		out = append(out, '\n')
	}
	out = append(out, "---\n"...)
	out = append(out, doc.Body...)
	return out
}

// Type returns the concept's declared type.
//
// Requires: doc came from Parse, so the key is present and non-empty.
// Ensures: never empty.
func (d *Document) Type() string {
	t, _ := d.Fields[requiredKey].(string)
	return t
}

// Text returns a string-valued frontmatter field, and whether it was present
// and actually a string.
//
// A key holding a non-string is reported absent rather than coerced. OKF
// permits producer-defined keys of any shape, and coercing one would turn a
// real mismatch into a plausible-looking value.
func (d *Document) Text(key string) (string, bool) {
	v, ok := d.Fields[key].(string)
	return v, ok
}
