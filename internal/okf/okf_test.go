package okf_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/okf"
	"github.com/StevenACoffman/skillet/errs"
)

// conformant is a minimal valid concept: OKF §4.1 requires `type` and nothing
// else, so this is the smallest document that must be accepted.
const conformant = "---\ntype: Reference\n---\nbody text\n"

func TestParseAcceptsMinimal(t *testing.T) {
	t.Parallel()
	doc, err := okf.Parse([]byte(conformant))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := doc.Type(); got != "Reference" {
		t.Errorf("Type() = %q, want %q", got, "Reference")
	}
	if doc.Body != "body text\n" {
		t.Errorf("Body = %q, want %q", doc.Body, "body text\n")
	}
}

// TestParseAcceptsWhatOKFForbidsRejecting covers OKF §11's negative
// requirements one at a time. Each of these is a condition a contributor might
// plausibly "tighten" into a rejection, which would make gnosis non-conformant.
func TestParseAcceptsWhatOKFForbidsRejecting(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"unknown type": "---\ntype: SomethingNobodyRegistered\n---\n",
		"unknown extra key": "---\ntype: Reference\nvendor_private_field: 42\n" +
			"---\n",
		"missing every optional family": "---\ntype: Reference\n---\n",
		"bare verified mapping": "---\ntype: Reference\n" +
			"verified: {by: 'human:sarah', at: 2026-08-19T00:00:00Z}\n---\n",
		"verified as a list": "---\ntype: Reference\nverified:\n" +
			"  - {by: 'human:sarah', at: 2026-08-19T00:00:00Z}\n---\n",
		"broken link in body": "---\ntype: Reference\n---\n" +
			"see [gone](/c/01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d-gone.md)\n",
		"nested producer structure": "---\ntype: Reference\nsources:\n" +
			"  - id: a\n    resource: https://example.org\n---\n",
		"empty body": "---\ntype: Reference\n---\n",
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := okf.Parse([]byte(src)); err != nil {
				t.Errorf("Parse rejected a conformant document: %v\nsource:\n%s", err, src)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"no frontmatter":     "just a markdown file\n",
		"empty type":         "---\ntype: \"\"\n---\n",
		"missing type":       "---\ntitle: no type here\n---\n",
		"type is not text":   "---\ntype: [a, b]\n---\n",
		"type is null":       "---\ntype: null\n---\n",
		"type coerced float": "---\ntype: 1.20\n---\n",
		"type coerced int":   "---\ntype: 0755\n---\n",
		"unparsable yaml":    "---\ntype: Reference\n  bad: indent: here\n---\n",
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := okf.Parse([]byte(src))
			if err == nil {
				t.Fatalf("Parse accepted an invalid document:\n%s", src)
			}
			if code := errs.ErrorCode(err); code != errs.EINVALID {
				t.Errorf("code = %q, want %q", code, errs.EINVALID)
			}
		})
	}
}

// TestRoundTripIsByteExact is the property that makes in-place rewriting safe.
// A parse that normalised quoting, reordered keys, or dropped a comment would
// silently rewrite a foreign producer's file on the first time gnosis touched it.
// TestUnquotedScalarDiagnosticNamesTheCause covers the one YAML footgun that
// survives 1.2 semantics: an unquoted numeric-looking value is coerced, so the
// key is present but not text. Reporting that as "missing" would send a reader
// hunting for a key that is plainly in the file, so the message must name the
// decoded type and say to quote it.
func TestUnquotedScalarDiagnosticNamesTheCause(t *testing.T) {
	t.Parallel()
	_, err := okf.Parse([]byte("---\ntype: 1.20\n---\n"))
	if err == nil {
		t.Fatal("Parse accepted a coerced type")
	}
	msg := err.Error()
	for _, want := range []string{"float64", "quote"} {
		if !strings.Contains(msg, want) {
			t.Errorf("diagnostic %q does not mention %q", msg, want)
		}
	}
}

func TestRoundTripIsByteExact(t *testing.T) {
	t.Parallel()
	sources := []string{
		conformant,
		"---\ntype: Reference\n---\n",
		// Quoting styles an encoder would normalise.
		"---\ntype: 'Reference'\ntitle: \"quoted\"\n---\nbody\n",
		// A comment, which no decode-then-encode cycle preserves.
		"---\ntype: Reference # why this type\n---\nbody\n",
		// Key order an encoder would sort.
		"---\nzeta: 1\ntype: Reference\nalpha: 2\n---\nbody\n",
		// Blank lines and trailing whitespace inside the block.
		"---\ntype: Reference\n\ntitle: spaced\n---\nbody\n",
		// Body containing something that looks like a delimiter.
		"---\ntype: Reference\n---\nbody\n---\nnot frontmatter\n",
	}
	for _, src := range sources {
		doc, err := okf.Parse([]byte(src))
		if err != nil {
			t.Errorf("Parse(%q): %v", src, err)
			continue
		}
		if got := okf.Render(doc); !bytes.Equal(got, []byte(src)) {
			t.Errorf("round trip changed the document:\n got %q\nwant %q", got, src)
		}
	}
}

// TestCRLFIsNormalisedNotPreserved pins the one documented exception to
// byte-exact round-tripping. skillet's splitter normalises line endings before
// this package sees them, so a Windows-authored document round-trips to its LF
// form. Asserting it here means the exception is a decision on record rather
// than a surprise found later in a diff.
func TestCRLFIsNormalisedNotPreserved(t *testing.T) {
	t.Parallel()
	src := "---\r\ntype: Reference\r\n---\r\nbody\r\n"
	want := "---\ntype: Reference\n---\nbody\n"
	doc, err := okf.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := okf.Render(doc); string(got) != want {
		t.Errorf("CRLF round trip = %q, want the LF form %q", got, want)
	}
}

func TestTextReportsNonStringsAbsent(t *testing.T) {
	t.Parallel()
	// Coercing a non-string would turn a producer's type mismatch into a
	// plausible-looking value, which is worse than reporting it missing.
	doc, err := okf.Parse([]byte("---\ntype: Reference\ncount: 3\ntitle: ok\n---\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if v, ok := doc.Text("count"); ok {
		t.Errorf("Text(\"count\") = %q, true; want absent for a non-string", v)
	}
	if v, ok := doc.Text("title"); !ok || v != "ok" {
		t.Errorf("Text(\"title\") = %q, %v; want \"ok\", true", v, ok)
	}
	if _, ok := doc.Text("nope"); ok {
		t.Error("Text of a missing key reported present")
	}
}
