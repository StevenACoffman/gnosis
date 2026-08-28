package indexcmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd"
)

// letters supplies the per-document suffix. A string index rather than
// `string(rune('a'+i))`, which converts an int to a rune and is a genuine overflow
// for any i a caller could pass — the linter is right that a fixture helper should
// not be the one place an out-of-range index becomes a valid identifier.
const letters = "abcdef"

// TestTwoRebuildsReportOneDigest is SPEC §18.3 at the dispatcher, which is where the
// requirement is actually stated: "`index rebuild` twice from the same bundle".
//
// The property is what makes §4.6's per-user index safe. Two colleagues at one commit
// hold different files, and if their indexes could differ, a disagreement between them
// might be about their caches rather than about the corpus — an argument nobody could
// settle. Now they compare a string.
//
// It is stated over content rather than over the file's bytes. A SQLite file is not
// byte-stable, so a byte comparison would fail on a database that is correct, and a
// determinism test that fails on correct output is one somebody turns off.
func TestTwoRebuildsReportOneDigest(t *testing.T) {
	t.Parallel()
	dir := smallCorpus(t, 3)

	first := digestFrom(t, dir)
	second := digestFrom(t, dir)
	if first != second {
		t.Errorf("two rebuilds of one bundle disagree:\n%s\n%s", first, second)
	}
	if first == "" {
		t.Error("the rebuild reported no digest")
	}
}

// TestADifferentBundleReportsADifferentDigest is the negative half. Without it a
// digest that hashed a constant would satisfy every other test here.
func TestADifferentBundleReportsADifferentDigest(t *testing.T) {
	t.Parallel()

	if digestFrom(t, smallCorpus(t, 2)) == digestFrom(t, smallCorpus(t, 3)) {
		t.Error("two different corpora reported one digest")
	}
}

// TestCheckReportsTheDigestWithoutWriting. A caller comparing their index against a
// colleague's is asking a read-only question, and making them write to get the answer
// would be the opposite of §4.5.
func TestCheckReportsTheDigestWithoutWriting(t *testing.T) {
	t.Parallel()
	dir := smallCorpus(t, 2)

	built := digestFrom(t, dir)
	if built == "" {
		t.Fatal("the rebuild reported no digest")
	}
	if checked := digestFrom(t, dir, "--check"); checked != built {
		t.Errorf("--check and a rebuild disagree about one index:\n%s\n%s",
			checked, built)
	}
}

// TestAnEditedDocumentChangesTheDigest, or the digest is not reading the rows and
// two colleagues could agree while holding different corpora.
func TestAnEditedDocumentChangesTheDigest(t *testing.T) {
	t.Parallel()
	dir := smallCorpus(t, 2)
	before := digestFrom(t, dir)

	// Retitle the first document, which changes a column the digest covers.
	id := docID(0)
	name := filepath.Join(dir, "c", id+"-doc.md")
	body := "---\ntype: Reference\ntitle: Retitled\ngnosis_id: " + id + "\n---\nbody\n"
	if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if after := digestFrom(t, dir); after == before {
		t.Error("retitling a document did not change the digest")
	}
}

// smallCorpus writes n documents and returns the bundle root. It is separate from
// floor_test.go's corpusOf, which is fixed at the size those tests turn on.
func smallCorpus(t *testing.T, n int) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "c"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for i := range n {
		id := docID(i)
		body := "---\ntype: Reference\ntitle: Doc " + letters[i:i+1] +
			"\ngnosis_id: " + id + "\n---\nbody\n"
		name := filepath.Join(dir, "c", id+"-doc.md")
		if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

// docID is a valid identifier for the nth fixture document.
func docID(i int) string {
	return validID[:len(validID)-1] + letters[i:i+1]
}

// digestFrom runs `index rebuild` and decodes the digest out of the envelope.
func digestFrom(t *testing.T, dir string, extra ...string) string {
	t.Helper()

	args := append([]string{"--bundle", dir, "--jsonl", "index", "rebuild"}, extra...)
	var out, errOut bytes.Buffer
	if err := cmd.Run(t.Context(), args, strings.NewReader(""), &out, &errOut); err != nil {
		t.Fatalf("index rebuild: %v\n%s", err, errOut.String())
	}
	var env struct {
		Data struct {
			Digest string `json:"digest"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("decode %q: %v", out.String(), err)
	}
	return env.Data.Digest
}
