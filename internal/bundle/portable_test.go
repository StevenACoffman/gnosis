package bundle_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// TestPortableExcludesWhatOneUserLeftBeside. The name is the assertion: an export and a
// proof packet carry the corpus, and `.gnosis/` is not part of it.
//
// This is the test I am most afraid of losing. Everything under `.gnosis/` is a record
// of what one person asked a model and when, and the failure it guards is silent in
// both directions — an export succeeds, the recipient reads a session history nobody
// meant to send, and nothing anywhere reports that it happened.
func TestPortableExcludesWhatOneUserLeftBeside(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		rel  string
		want bool
	}{
		"a concept":                {rel: "c/0193-retry-cap.md", want: true},
		"archived text":            {rel: "evidence/text/ab/cd.txt", want: true},
		"a standards file":         {rel: "standards/promote.toml", want: true},
		"the log":                  {rel: "log.md", want: true},
		"the audit trail":          {rel: ".gnosis/audit.jsonl", want: false},
		"a cached prompt":          {rel: ".gnosis/prompts/abc.md", want: false},
		"a quarantined draft":      {rel: ".gnosis/quarantine/c/x.md", want: false},
		"git's own storage":        {rel: ".git/config", want: false},
		"a name that merely opens": {rel: ".gnosisrc", want: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := bundle.Portable(tc.rel); got != tc.want {
				t.Errorf("Portable(%q) = %v, want %v", tc.rel, got, tc.want)
			}
		})
	}
}

// TestPortablePathsIsSortedAndSkipsTheCache. Sorted is not a courtesy: a proof packet
// records artifacts in the order it is handed them, so an order that came from directory
// iteration would put different bytes in the packet on every run.
func TestPortablePathsIsSortedAndSkipsTheCache(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, rel := range []string{
		"c/b.md", "c/a.md", "evidence/text/x.txt", "log.md",
		".gnosis/audit.jsonl", ".gnosis/prompts/k.md", ".git/config",
	} {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	got, err := bundle.PortablePaths(dir)
	if err != nil {
		t.Fatalf("PortablePaths: %v", err)
	}
	want := []string{"c/a.md", "c/b.md", "evidence/text/x.txt", "log.md"}
	if !slices.Equal(got, want) {
		t.Errorf("PortablePaths = %q, want %q", got, want)
	}
}

// TestPortablePathsOnABundleHoldingNothing. A fresh bundle is the first thing anybody
// runs this against, and an error there would read as the command being broken.
func TestPortablePathsOnABundleHoldingNothing(t *testing.T) {
	t.Parallel()

	got, err := bundle.PortablePaths(t.TempDir())
	if err != nil {
		t.Fatalf("PortablePaths: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("PortablePaths = %q, want nothing", got)
	}
}
