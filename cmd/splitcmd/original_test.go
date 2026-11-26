package splitcmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd/root"
)

// shelves lays out a sources tree and an originals root beside it, and writes a
// three-chapter book well over max bytes at sources/kleppmann/book.md.
func shelves(t *testing.T) (sources, originals, path string) {
	t.Helper()
	base := t.TempDir()
	sources = filepath.Join(base, "source_materials")
	originals = filepath.Join(base, "split_source_originals")
	if err := os.MkdirAll(filepath.Join(sources, "kleppmann"), 0o755); err != nil {
		t.Fatal(err)
	}
	chapter := strings.Repeat(strings.Repeat("word ", 15)+"\n", 40) + "\n"
	src := "# Book\n\n## One\n\n" + chapter + "## Two\n\n" + chapter + "## Three\n\n" + chapter
	path = filepath.Join(sources, "kleppmann", "book.md")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	return sources, originals, path
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestOriginalsMovesASplitSourceToTheSamePlaceUnderIt(t *testing.T) {
	sources, originals, path := shelves(t)
	want, _ := os.ReadFile(path)

	out, err := run(t, "split", maxBytes, "--sources", sources, "--originals", originals, path)
	if err != nil {
		t.Fatalf("split: %v\n%s", err, out)
	}
	moved := filepath.Join(originals, "kleppmann", "book.md")
	if got, _ := os.ReadFile(moved); !bytes.Equal(got, want) {
		t.Fatalf("%s does not hold the original", moved)
	}
	if exists(path) {
		t.Error("the original is still among the sources")
	}
	if got := parts(t, filepath.Dir(path)); len(got) < 2 {
		t.Errorf("parts beside the source = %v, want several", got)
	}
	if !strings.Contains(out, "original  moved to "+moved) {
		t.Errorf("output does not report the move:\n%s", out)
	}
}

func TestASourceThatFitsIsNotMoved(t *testing.T) {
	sources, originals, _ := shelves(t)
	small := filepath.Join(sources, "kleppmann", "notes.md")
	if err := os.WriteFile(small, []byte("# Notes\n\nShort.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "split", maxBytes, "--sources", sources, "--originals", originals, small)
	if err != nil {
		t.Fatalf("split: %v\n%s", err, out)
	}
	if !exists(small) || exists(filepath.Join(originals, "kleppmann", "notes.md")) {
		t.Error("a source that needed no split was moved")
	}
}

func TestAMovedOriginalIsSplitBackIntoTheSources(t *testing.T) {
	sources, originals, path := shelves(t)
	args := []string{"split", maxBytes, "--sources", sources, "--originals", originals}
	if out, err := run(t, append(args, path)...); err != nil {
		t.Fatalf("first split: %v\n%s", err, out)
	}
	moved := filepath.Join(originals, "kleppmann", "book.md")

	out, err := run(t, append(args, "--replace", moved)...)
	if err != nil {
		t.Fatalf("split of the moved original: %v\n%s", err, out)
	}
	if strings.Contains(out, "created") || !strings.Contains(out, "unchanged") {
		t.Errorf("the parts should be found unchanged beside the sources:\n%s", out)
	}
	if !exists(moved) {
		t.Error("the moved original was moved again")
	}
	if got := parts(t, filepath.Dir(moved)); len(got) != 0 {
		t.Errorf("parts were written among the originals: %v", got)
	}
}

func TestAnOriginalIsNeverWrittenOverAnotherFile(t *testing.T) {
	sources, originals, path := shelves(t)
	other := filepath.Join(originals, "kleppmann", "book.md")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("# Another edition\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "split", maxBytes, "--replace",
		"--sources", sources, "--originals", originals, path)
	if exitCode(err) != root.ExitError(root.CodeFindings) {
		t.Fatalf("err = %v, want the findings exit\n%s", err, out)
	}
	if got, _ := os.ReadFile(other); string(got) != "# Another edition\n" {
		t.Error("the file at the destination was overwritten")
	}
	if !exists(path) {
		t.Error("the original was removed though it could not move")
	}
}

func TestAnOriginalStaysWhileAPartConflicts(t *testing.T) {
	sources, originals, path := shelves(t)
	if out, err := run(t, "split", maxBytes, path); err != nil {
		t.Fatalf("split: %v\n%s", err, out)
	}
	edited := parts(t, filepath.Dir(path))[0]
	if err := os.WriteFile(edited, []byte("edited by hand\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "split", maxBytes, "--sources", sources, "--originals", originals, path)
	if exitCode(err) != root.ExitError(root.CodeFindings) {
		t.Fatalf("err = %v, want the findings exit\n%s", err, out)
	}
	if !exists(path) || exists(filepath.Join(originals, "kleppmann", "book.md")) {
		t.Error("the original moved while one of its parts conflicts")
	}
}

func TestADryRunMovesNoOriginal(t *testing.T) {
	sources, originals, path := shelves(t)
	out, err := run(t, "split", "--dry-run", maxBytes,
		"--sources", sources, "--originals", originals, path)
	if err != nil {
		t.Fatalf("split --dry-run: %v\n%s", err, out)
	}
	if !exists(path) || exists(originals) {
		t.Error("a dry run moved the original or created --originals")
	}
	if !strings.Contains(out, "original  moved to ") {
		t.Errorf("a dry run should report the move it would make:\n%s", out)
	}
}

func TestASourceOutsideBothRootsIsRefused(t *testing.T) {
	sources, originals, _ := shelves(t)
	elsewhere := filepath.Join(filepath.Dir(sources), "book.md")
	if err := os.WriteFile(elsewhere, []byte("# Book\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "split", maxBytes, "--sources", sources, "--originals", originals,
		elsewhere)
	if exitCode(err) != root.ExitError(root.CodeFindings) {
		t.Fatalf("err = %v, want the findings exit\n%s", err, out)
	}
	if !strings.Contains(out, "under neither --sources") {
		t.Errorf("output does not say why it was refused:\n%s", out)
	}
}

func TestSourcesInsideOriginalsIsAUsageError(t *testing.T) {
	sources, _, path := shelves(t)
	out, err := run(t, "split", maxBytes, "--sources", sources,
		"--originals", filepath.Dir(sources), path)
	if exitCode(err) != root.ExitError(root.CodeUsage) {
		t.Fatalf("err = %v, want the usage exit\n%s", err, out)
	}
	if !exists(path) {
		t.Error("the source moved")
	}
}
