package archive_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/archive"
)

// gates mirrors the seed in standards/archive.toml. It is a literal here rather
// than loaded, because this package does not import standards — the shell joins
// them, and a test that reached across would assert the layering away.
func gates() archive.Gates {
	return archive.Gates{
		Allowlist:          []string{".md", ".txt", ".svg"},
		PerFileCap:         262144,
		EmbeddedPayloadCap: 8192,
	}
}

func TestArchivesAllowedText(t *testing.T) {
	t.Parallel()
	body := []byte("# Title\n\nA sentence worth quoting.\n")
	got := archive.Decide(&archive.Candidate{
		URI: "https://example.org/a.md", Bytes: body, Extension: ".md",
	}, gates())

	if got.Record.Disposition != archive.Archived {
		t.Fatalf("disposition = %q, want archived (reason %q)",
			got.Record.Disposition, got.Record.RejectReason)
	}
	if !bytes.Equal(got.Content, body) {
		t.Error("archived content is not the fetched bytes")
	}
	if got.Record.RejectReason != archive.ReasonNone {
		t.Errorf("an archived record carries a reject reason: %q", got.Record.RejectReason)
	}
	want := "evidence/text/" + sum(body)[:2] + "/" + sum(body) + ".md"
	if got.Record.ArchivePath != want {
		t.Errorf("archive path = %q, want %q", got.Record.ArchivePath, want)
	}
}

// TestRejectionFallsThroughToReferenced is PLAN §3's requirement and §4.3's
// policy: a rejected file records its reason and is referenced, not failed.
func TestRejectionFallsThroughToReferenced(t *testing.T) {
	t.Parallel()
	big := bytes.Repeat([]byte("x"), 300_000)
	cases := map[string]struct {
		c    archive.Candidate
		want archive.RejectReason
	}{
		"not on the allowlist": {
			archive.Candidate{URI: "u", Bytes: []byte("hi"), Extension: ".pdf"},
			archive.ReasonExtension,
		},
		"over the cap": {
			archive.Candidate{URI: "u", Bytes: big, Extension: ".md"},
			archive.ReasonOversize,
		},
		"binary wearing a text extension": {
			archive.Candidate{URI: "u", Bytes: []byte("ok\x00then"), Extension: ".txt"},
			archive.ReasonBinary,
		},
		"invalid utf-8": {
			archive.Candidate{URI: "u", Bytes: []byte{0xff, 0xfe, 0xfd}, Extension: ".txt"},
			archive.ReasonBinary,
		},
		"embedded raster": {
			archive.Candidate{
				URI:       "u",
				Bytes:     []byte("![x](data:image/png;base64," + strings.Repeat("A", 9000) + ")"),
				Extension: ".md",
			},
			archive.ReasonEmbeddedPayload,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := archive.Decide(&tc.c, gates())
			assertReferenced(t, &got, tc.want, tc.c.Bytes)
		})
	}
}

// assertReferenced checks everything a referenced record must and must not carry.
// It is a helper rather than an inline block because the checks are the same for
// every rejection, and the reason each one differs is the only interesting part.
func assertReferenced(t *testing.T, got *archive.Outcome, want archive.RejectReason, src []byte) {
	t.Helper()
	if got.Record.Disposition != archive.Referenced {
		t.Fatalf("disposition = %q, want referenced", got.Record.Disposition)
	}
	if got.Record.RejectReason != want {
		t.Errorf("reason = %q, want %q", got.Record.RejectReason, want)
	}
	if got.Content != nil {
		t.Error("a referenced record carries content")
	}
	if got.Record.ArchivePath != "" {
		t.Errorf("a referenced record names an archive path: %q", got.Record.ArchivePath)
	}
	// The hash is recorded for every fetch including referenced (§9.2): for
	// exactly these, the ledger is the only record there will ever be.
	if got.Record.SourceSHA256 != sum(src) {
		t.Error("a referenced record does not record the source hash")
	}
}

func TestExtractionRescuesAnUnarchivableSource(t *testing.T) {
	t.Parallel()
	source := []byte("%PDF-1.7\x00binary")
	text := []byte("The extracted prose.\n")
	got := archive.Decide(&archive.Candidate{
		URI: "https://example.org/p.pdf", Bytes: source, Extension: ".pdf",
		Extraction: &archive.Extraction{
			Text: text, Extractor: "JohannesKaufmann/html-to-markdown", ExtractorVersion: "1.2.3",
			Extension: ".md",
		},
	}, gates())

	if got.Record.Disposition != archive.Extracted {
		t.Fatalf("disposition = %q, want extracted (reason %q)",
			got.Record.Disposition, got.Record.RejectReason)
	}
	if !bytes.Equal(got.Content, text) {
		t.Error("stored content is not the extraction")
	}
	// The chain from stored text back to fetched bytes must be in the record,
	// not inferred: the archived path hashes the extraction, and ExtractedFrom
	// names the source it came from.
	if got.Record.ArchivePath != "evidence/text/"+sum(text)[:2]+"/"+sum(text)+".md" {
		t.Errorf("archive path does not address the extraction: %q", got.Record.ArchivePath)
	}
	if got.Record.ExtractedFrom != sum(source) {
		t.Error("the record does not link the extraction to its source")
	}
	if got.Record.Extractor == "" || got.Record.ExtractorVersion == "" {
		t.Error("extractor identity is not recorded, so a re-extraction would be silent")
	}
	if got.Record.RejectReason != archive.ReasonNone {
		t.Errorf("an extracted record carries a reject reason: %q", got.Record.RejectReason)
	}
}

// TestAnInadmissibleExtractionReportsItsOwnReason: the extraction's failure is
// the more specific fact, and the source's is implied by there having been an
// extraction attempt at all.
func TestAnInadmissibleExtractionReportsItsOwnReason(t *testing.T) {
	t.Parallel()
	got := archive.Decide(&archive.Candidate{
		URI: "u", Bytes: []byte("\x00"), Extension: ".bin",
		Extraction: &archive.Extraction{
			Text: bytes.Repeat([]byte("y"), 300_000), Extension: ".md",
			Extractor: "e", ExtractorVersion: "1",
		},
	}, gates())

	if got.Record.Disposition != archive.Referenced {
		t.Fatalf("disposition = %q, want referenced", got.Record.Disposition)
	}
	if got.Record.RejectReason != archive.ReasonOversize {
		t.Errorf("reason = %q, want the extraction's own failure", got.Record.RejectReason)
	}
}

// TestDurableIsFalseForTheZeroValue: an unpopulated record must never read as
// proof.
func TestDurableIsFalseForTheZeroValue(t *testing.T) {
	t.Parallel()
	var d archive.Disposition
	if d != archive.DispositionUnset {
		t.Errorf("the zero disposition is %q, not unset", d)
	}
	if d.Durable() {
		t.Error("the zero disposition claims durable evidence")
	}
	if archive.Referenced.Durable() {
		t.Error("referenced claims durable evidence")
	}
	for _, ok := range []archive.Disposition{archive.Archived, archive.Extracted} {
		if !ok.Durable() {
			t.Errorf("%q is not durable", ok)
		}
	}
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
