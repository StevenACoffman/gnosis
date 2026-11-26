package splitcmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd/root"
)

func TestSplitWritesAnIndexLinkingEveryPart(t *testing.T) {
	path := book(t)
	out, err := run(t, "split", maxBytes, path)
	if err != nil {
		t.Fatalf("split: %v\n%s", err, out)
	}
	dir := filepath.Dir(path)
	index, err := os.ReadFile(filepath.Join(dir, "book-00-index.md"))
	if err != nil {
		t.Fatalf("no index beside the parts: %v\n%s", err, out)
	}
	links := regexp.MustCompile(`\]\(([^)]+)\)`).FindAllStringSubmatch(string(index), -1)
	written := parts(t, dir)
	if len(links) != len(written) {
		t.Fatalf("index links %d files, want the %d parts:\n%s", len(links), len(written), index)
	}
	for i, l := range links {
		if filepath.Join(dir, l[1]) != written[i] {
			t.Errorf("entry %d links %s, want %s", i+1, l[1], filepath.Base(written[i]))
		}
	}
	if !strings.Contains(out, "index") || !strings.Contains(out, "book-00-index.md") {
		t.Errorf("output does not report the index:\n%s", out)
	}
}

func TestASourceThatFitsGetsNoIndex(t *testing.T) {
	small := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(small, []byte("# Notes\n\nShort.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, "split", maxBytes, small); err != nil {
		t.Fatalf("split: %v\n%s", err, out)
	}
	if m, _ := filepath.Glob(filepath.Join(filepath.Dir(small), "*index*")); len(m) != 0 {
		t.Errorf("wrote %v for a source with no parts", m)
	}
}

func TestReplaceRegeneratesTheIndexAndNeverAHandEditedOne(t *testing.T) {
	path := book(t)
	if out, err := run(t, "split", maxBytes, path); err != nil {
		t.Fatalf("split: %v\n%s", err, out)
	}
	index := filepath.Join(filepath.Dir(path), "book-00-index.md")
	want, _ := os.ReadFile(index)

	// An index an earlier split wrote: same source, other bytes.
	older := "---\ngnosis_split_index:\n  source: book.md\n  splitter_version: 1\n---\n\n# Old\n"
	if err := os.WriteFile(index, []byte(older), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, "split", maxBytes, "--replace", path); err != nil {
		t.Fatalf("split --replace: %v\n%s", err, out)
	}
	if got, _ := os.ReadFile(index); !bytes.Equal(got, want) {
		t.Errorf("--replace left the older index:\n%s", got)
	}

	// The same file edited by hand no longer says split wrote it.
	if err := os.WriteFile(index, []byte("# My own contents page\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, "split", maxBytes, "--replace", path)
	if exitCode(err) != root.ExitError(root.CodeFindings) {
		t.Fatalf("err = %v, want the findings exit\n%s", err, out)
	}
	if got, _ := os.ReadFile(index); string(got) != "# My own contents page\n" {
		t.Error("a hand-edited index was overwritten")
	}
}

func TestAnIndexOfAnotherWidthIsALeftover(t *testing.T) {
	path := book(t)
	stale := filepath.Join(filepath.Dir(path), "book-000-index.md")
	idx := "---\ngnosis_split_index:\n  source: book.md\n  splitter_version: 2\n---\n\n# Old\n"
	if err := os.WriteFile(stale, []byte(idx), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "split", maxBytes, path)
	if exitCode(err) != root.ExitError(root.CodeFindings) || !strings.Contains(out, "leftover") {
		t.Fatalf("err = %v; want the stale index reported as a leftover\n%s", err, out)
	}
	if out, err := run(t, "split", maxBytes, "--replace", path); err != nil {
		t.Fatalf("split --replace: %v\n%s", err, out)
	}
	if exists(stale) {
		t.Error("--replace kept the stale index")
	}
}
