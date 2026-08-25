package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goodReply is one admit accepts: its quotation is in the fixture's source text.
const goodReply = "```yaml\ntitle: Cache Lifetime\ntype: Reference\nclaims:\n" +
	"  - text: The cache is cleared on restart.\n    quotes:\n" +
	"      - The cache is cleared on restart and holds nothing across sessions\n```\n"

// reply writes a well-formed extraction reply quoting the fixture source.
func reply(t *testing.T, body string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "reply.md")
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}
	return file
}

// exists reports whether a bundle-relative path is on disk.
func exists(t *testing.T, bundleDir, rel string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(bundleDir, filepath.FromSlash(rel)))
	return err == nil
}

// TestAFiledReplyRemovesItsPrompt is the entry: `.gnosis/prompts/` accumulated one
// file per unanswered question and nothing removed an answered one, so a reader
// listing the directory could not tell what was outstanding.
//
// The metadata goes with it. Leaving the sidecar behind would keep the directory
// honest and leave a second one that is not.
func TestAFiledReplyRemovesItsPrompt(t *testing.T) {
	t.Parallel()

	bundleDir, uri := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	key, promptPath := firstPrompt(t, ingest(t, bundleDir, uri))
	if !exists(t, bundleDir, promptPath) {
		t.Fatalf("the fixture emitted no prompt at %s", promptPath)
	}
	metaPath := ".gnosis/prompts/" + key + ".json"
	if !exists(t, bundleDir, metaPath) {
		t.Fatalf("the fixture wrote no metadata at %s", metaPath)
	}

	_, stderr, err := run(t, "--bundle", bundleDir, "--jsonl",
		"admit", "--key", key, "--submitter", "agent:test", reply(t, goodReply))
	if err != nil {
		t.Fatalf("admit: %v\n%s", err, stderr)
	}
	if exists(t, bundleDir, promptPath) {
		t.Error("the answered prompt is still on disk")
	}
	if exists(t, bundleDir, metaPath) {
		t.Error("the answered prompt's metadata is still on disk")
	}
}

// TestAnUnfiledReplyKeepsItsPrompt is the case that decided where the removal goes,
// and the reason the entry's own wording ("once the reply is cached") is wrong.
//
// Caching happens before the reply is even parsed. If the prompt were spent there, an
// agent told "the YAML is malformed, fix it" would find that the key no longer exists
// — `admit` refuses a key with no metadata — so the diagnostic would be advice nobody
// could take.
func TestAnUnfiledReplyKeepsItsPrompt(t *testing.T) {
	t.Parallel()

	bundleDir, uri := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	key, promptPath := firstPrompt(t, ingest(t, bundleDir, uri))
	metaPath := ".gnosis/prompts/" + key + ".json"

	// Not YAML at all, which is the failure an agent is most likely to make and be
	// asked to fix.
	if _, _, err := run(t, "--bundle", bundleDir, "--jsonl", "admit",
		"--key", key, "--submitter", "agent:test",
		reply(t, "I could not find anything to quote.\n"),
	); err == nil {
		t.Error("a malformed reply was accepted")
	}
	if !exists(t, bundleDir, promptPath) || !exists(t, bundleDir, metaPath) {
		t.Fatal("a rejected reply spent the prompt it could not answer")
	}

	// And the retry the diagnostic asks for actually works, which is the property
	// the two assertions above only stand in for.
	if _, stderr, err := run(t, "--bundle", bundleDir, "--jsonl", "admit",
		"--key", key, "--submitter", "agent:test", reply(t, goodReply),
	); err != nil {
		t.Fatalf("the corrected reply was refused: %v\n%s", err, stderr)
	}
	if exists(t, bundleDir, promptPath) {
		t.Error("the prompt survived the reply that answered it")
	}
}

// TestAPreviewLeavesThePromptAlone keeps §4.6.2's guarantee intact at the new write.
//
// A preview and an apply are one command differing in one field, and the preview must
// write nothing — including nothing removed. It holds structurally here rather than by
// a flag test, because a non-writing effect never reaches the filing step, and this is
// what would notice if that stopped being true.
func TestAPreviewLeavesThePromptAlone(t *testing.T) {
	t.Parallel()

	bundleDir, uri := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	key, promptPath := firstPrompt(t, ingest(t, bundleDir, uri))

	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl", "admit",
		"--key", key, "--submitter", "agent:test", "--dry-run",
		reply(t, goodReply))
	if err != nil {
		t.Fatalf("preview: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, "would_quarantine") {
		t.Fatalf("that was not a preview: %s", stdout)
	}
	if !exists(t, bundleDir, promptPath) {
		t.Error("a preview removed the prompt")
	}
}
