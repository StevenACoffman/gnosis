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
		// NoScan rather than nil. This package's tests are about the admission
		// policy, not about §9.3, and a nil now refuses — which is the fail-closed
		// direction the default was changed to. Saying so is one identifier, and it
		// is what makes every non-scanning caller findable by grep.
		ScanText: archive.NoScan,
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

// TestOversizeDistinguishesItsTwoCaps is the mitigation for a signature that cannot
// carry the distinction: the two parameters are adjacent int64 byte counts, and
// nothing in the type system stops a caller swapping them.
//
// Each case passes a size that is over one cap and under the other, so a swap makes
// the reported reason wrong in a way this test sees.
func TestOversizeDistinguishesItsTwoCaps(t *testing.T) {
	t.Parallel()

	// 300 bytes of prose with no data URI: over a 100-byte file cap, and its
	// payload cap is irrelevant because there is no payload.
	prose := []byte(strings.Repeat("a", 300))
	// A small file carrying a large data URI: under the file cap, over the payload
	// cap.
	payload := []byte("![](data:image/png;base64," + strings.Repeat("A", 200) + ")")

	cases := map[string]struct {
		data                           []byte
		perFileCap, embeddedPayloadCap int64
		want                           archive.RejectReason
	}{
		"over the file cap":              {prose, 100, 10_000, archive.ReasonOversize},
		"under the file cap":             {prose, 10_000, 10_000, archive.ReasonNone},
		"over the payload cap":           {payload, 10_000, 100, archive.ReasonEmbeddedPayload},
		"under the payload cap":          {payload, 10_000, 10_000, archive.ReasonNone},
		"a zero file cap disables it":    {prose, 0, 10_000, archive.ReasonNone},
		"a zero payload cap disables it": {payload, 10_000, 0, archive.ReasonNone},
		"both zero is no stage at all":   {payload, 0, 0, archive.ReasonNone},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := archive.Oversize(tc.data, tc.perFileCap, tc.embeddedPayloadCap)
			if got.Reason != tc.want {
				t.Errorf("Oversize = %q, want %q", got.Reason, tc.want)
			}
			assertBoundIsCoherent(t, got)
		})
	}
}

// TestAZeroPayloadCapDoesNotFlagEveryDataURI. `hasOversizePayload` compares against
// the limit it is given, so at zero it would flag *every* data URI — which for a
// caller holding no standards would report a document about icons as an oversize
// payload. A disabled cap must disable its check.
func TestAZeroPayloadCapDoesNotFlagEveryDataURI(t *testing.T) {
	t.Parallel()

	tiny := []byte("![icon](data:image/gif;base64,R0lGOD)")
	if got := archive.Oversize(tiny, 10_000, 0); got.Exceeded() {
		t.Errorf("a disabled payload cap flagged %q", got.Reason)
	}
}

// TestABoundNamesTheMeasurementAndTheCap is the whole point of the change: a refusal
// reading only `embedded-payload` is one an author can argue with and not act on, and
// the obvious next move is to argue the cap down rather than truncate the example.
func TestABoundNamesTheMeasurementAndTheCap(t *testing.T) {
	t.Parallel()

	payload := []byte("![](data:image/png;base64," + strings.Repeat("A", 9000) + ")")
	got := archive.Oversize(payload, 262_144, 8_192)

	if !got.Exceeded() {
		t.Fatalf("a 9000-byte payload against an 8192-byte cap passed: %+v", got)
	}
	if got.Found != int64(9000+len("image/png;base64,")) {
		t.Errorf("Found = %d, want the payload's measured length", got.Found)
	}
	if got.Limit != 8_192 {
		t.Errorf("Limit = %d, want the declared cap", got.Limit)
	}
	detail := got.Detail()
	for _, want := range []string{"9017", "8192", "an embedded payload"} {
		if !strings.Contains(detail, want) {
			t.Errorf("Detail() omits %q: %q", want, detail)
		}
	}
}

// TestTheLargestPayloadIsReported, not the first. A document with a small icon and a
// large raster is refused for the raster, and reporting the icon's size would send an
// author to edit the wrong line.
func TestTheLargestPayloadIsReported(t *testing.T) {
	t.Parallel()

	both := []byte(
		"![icon](data:image/gif;base64," + strings.Repeat("B", 40) + ")\n" +
			"![big](data:image/png;base64," + strings.Repeat("A", 9000) + ")\n",
	)
	got := archive.Oversize(both, 262_144, 8_192)
	if !got.Exceeded() {
		t.Fatal("the large payload was not found")
	}
	if got.Found < 9000 {
		t.Errorf("Found = %d, want the larger payload's length", got.Found)
	}
}

// TestTheFileBoundNamesTheFile. The two reasons want different wording, because an
// author truncates a payload and splits a document, and a message that said "an
// embedded payload" for an oversize file would send them to look for one.
func TestTheFileBoundNamesTheFile(t *testing.T) {
	t.Parallel()

	got := archive.Oversize([]byte(strings.Repeat("a", 500)), 100, 10_000)
	if got.Reason != archive.ReasonOversize {
		t.Fatalf("reason = %q", got.Reason)
	}
	if !strings.Contains(got.Detail(), "the document is 500 bytes") {
		t.Errorf("Detail() does not name the document's size: %q", got.Detail())
	}
	if strings.Contains(got.Detail(), "payload") {
		t.Errorf("an oversize document was described as a payload: %q", got.Detail())
	}
}

// TestTheZeroBoundAssertsNothing. Same discipline as every other zero value here: a
// value nobody populated must not read as a check that passed.
func TestTheZeroBoundAssertsNothing(t *testing.T) {
	t.Parallel()

	var b archive.Bound
	if b.Exceeded() {
		t.Error("the zero Bound reports an exceeded cap")
	}
	if b.Detail() != "" {
		t.Errorf("the zero Bound rendered %q", b.Detail())
	}
	if b.Reason != archive.ReasonNone {
		t.Errorf("the zero Bound carries reason %q", b.Reason)
	}
}

// assertBoundIsCoherent checks the invariants every Bound has, whatever produced it.
//
// Extracted from the table above because the linter reported the complexity and was
// right: the table's cases vary one thing — which cap is exceeded — and four
// invariants bolted onto each case made a per-case assertion out of a per-value
// property. These hold for any Bound, so they belong in one place that says so.
func assertBoundIsCoherent(t *testing.T, b archive.Bound) {
	t.Helper()

	if b.Exceeded() != (b.Reason != archive.ReasonNone) {
		t.Errorf("Exceeded() = %t for reason %q", b.Exceeded(), b.Reason)
	}
	if !b.Exceeded() {
		if b.Detail() != "" {
			t.Errorf("a passing bound rendered %q", b.Detail())
		}
		return
	}
	// A refusal that does not name a measurement above a positive cap is the
	// unactionable refusal this type exists to replace.
	if b.Limit <= 0 {
		t.Errorf("a refusal reports a cap of %d", b.Limit)
	}
	if b.Found <= b.Limit {
		t.Errorf("a refusal reports %d bytes, which is not over the cap of %d",
			b.Found, b.Limit)
	}
	if b.Detail() == "" {
		t.Error("a refusal rendered nothing for a reader")
	}
}
