package bundle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/bundle"
)

func at(day int) time.Time {
	return time.Date(2026, 8, day, 9, 0, 0, 0, time.UTC)
}

// TestAnUncheckedBundleIsNotAnError, and returns empty rather than nil, so a
// caller need not distinguish "never checked anything" from "no result".
func TestAnUncheckedBundleIsNotAnError(t *testing.T) {
	t.Parallel()
	got, err := bundle.LoadChecks(t.TempDir())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil {
		t.Error("an absent file returned nil")
	}
	if len(got) != 0 {
		t.Errorf("got %d checks from an absent file", len(got))
	}
}

// TestACheckSurvivesARoundTrip, including the timestamp: this file is the only
// place the moment of a check is kept, so losing it in the encoding would make
// every source read as never-checked.
func TestACheckSurvivesARoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := writerFor(t, dir).RecordChecks(at(21), []bundle.Check{
		{URI: "https://example.org/a.md", SourceSHA256: "aaa"},
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	got, err := bundle.LoadChecks(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d checks, want 1", len(got))
	}
	for _, c := range got {
		if !c.At.Equal(at(21)) {
			t.Errorf("at = %v, want the recorded time", c.At)
		}
	}
}

// TestACheckIsAboutAVersionNotAURI. Learning that v1 is unchanged says nothing
// once v2 exists, and keying on the URI alone would let an old observation vouch
// for new bytes.
func TestACheckIsAboutAVersionNotAURI(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const uri = "https://example.org/a.md"

	w := writerFor(t, dir)
	if err := w.RecordChecks(at(20), []bundle.Check{
		{URI: uri, SourceSHA256: "version-one"},
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := w.RecordChecks(at(21), []bundle.Check{
		{URI: uri, SourceSHA256: "version-two"},
	}); err != nil {
		t.Fatalf("record: %v", err)
	}

	got, err := bundle.LoadChecks(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d checks, want one per version: %+v", len(got), got)
	}
}

// TestRecheckingOneVersionUpserts. §4.3.1 says nothing consumes the sequence, so
// an append-only log here would be the unbounded growth that section refused —
// 26,000 lines a year for a weekly sweep over 500 sources, read by nobody.
func TestRecheckingOneVersionUpserts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := []bundle.Check{{URI: "https://example.org/a.md", SourceSHA256: "aaa"}}

	w := writerFor(t, dir)
	for _, day := range []int{19, 20, 21} {
		if err := w.RecordChecks(at(day), c); err != nil {
			t.Fatalf("record day %d: %v", day, err)
		}
	}

	body, err := os.ReadFile(filepath.Join(dir, ".gnosis", "checked.jsonl"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if n := strings.Count(strings.TrimSpace(string(body)), "\n") + 1; n != 1 {
		t.Errorf("the file has %d lines after three checks of one version", n)
	}

	got, err := bundle.LoadChecks(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, check := range got {
		if !check.At.Equal(at(21)) {
			t.Errorf("at = %v, want the latest check", check.At)
		}
	}
}

// TestTheFileIsStable, so two runs over one state produce identical bytes and a
// diff of two checkouts means something.
func TestTheFileIsStable(t *testing.T) {
	t.Parallel()
	sources := []bundle.Check{
		{URI: "https://example.org/c.md", SourceSHA256: "ccc"},
		{URI: "https://example.org/a.md", SourceSHA256: "aaa"},
		{URI: "https://example.org/b.md", SourceSHA256: "bbb"},
	}

	first := writeAndRead(t, sources)
	// Reversed input must produce the same file: the order is the map's, and a
	// map's order is not one.
	reversed := []bundle.Check{sources[2], sources[1], sources[0]}
	if second := writeAndRead(t, reversed); second != first {
		t.Errorf("the file depends on input order:\n%s\n%s", first, second)
	}
}

func writeAndRead(t *testing.T, sources []bundle.Check) string {
	t.Helper()
	dir := t.TempDir()
	if err := writerFor(t, dir).RecordChecks(at(21), sources); err != nil {
		t.Fatalf("record: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, ".gnosis", "checked.jsonl"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(body)
}

// TestAMalformedLineIsAnError. This file decides whether a claim reads as
// verified, and silently dropping a line would make a source report as
// never-checked when the record exists and cannot be read.
func TestAMalformedCheckLineIsAnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := writerFor(t, dir).RecordChecks(at(21), []bundle.Check{
		{URI: "u", SourceSHA256: "aaa"},
	}); err != nil {
		t.Fatalf("record: %v", err)
	}

	path := filepath.Join(dir, ".gnosis", "checked.jsonl")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err = os.WriteFile(path, append(body, []byte("{not json\n")...), 0o600); err != nil {
		t.Fatalf("corrupt: %v", err)
	}

	if _, err = bundle.LoadChecks(dir); err == nil {
		t.Error("a corrupt check file read cleanly")
	}
}

// TestRecordingNothingIsANoOp, so a fetch that read no sources does not create an
// empty file.
func TestRecordingNothingIsANoOp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := writerFor(t, dir).RecordChecks(at(21), nil); err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gnosis", "checked.jsonl")); err == nil {
		t.Error("recording nothing created a file")
	}
}

// TestTheRecordIsPerUser: it lives under .gnosis/, which init gitignores, so two
// colleagues at one commit hold different records and are both right.
func TestTheCheckRecordIsPerUser(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := writerFor(t, dir).RecordChecks(at(21), []bundle.Check{
		{URI: "u", SourceSHA256: "aaa"},
	}); err != nil {
		t.Fatalf("record: %v", err)
	}

	var outside []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == ".gnosis":
			return filepath.SkipDir
		case d.IsDir():
			return nil
		}
		if strings.Contains(d.Name(), "checked") {
			outside = append(outside, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(outside) > 0 {
		t.Errorf("the check record is outside .gnosis/ and would be committed: %v", outside)
	}
}
