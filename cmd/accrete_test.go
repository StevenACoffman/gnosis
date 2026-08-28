package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAccretionAppendsEvidenceAndNeverAClaim is §6.3's distinction through the command,
// and the message is half of what is being tested.
//
// A reply about a concept that already exists appends its quotations to the claims that
// document already makes. A claim the document does not make gets no paragraph and no
// evidence, because adding one would rewrite the body — which is `synthesize`'s gated
// operation under the cheaper name.
func TestAccretionAppendsEvidenceAndNeverAClaim(t *testing.T) {
	t.Parallel()
	dir, key := conceptPrompt(t)

	const reply = "```yaml\ntype: Rule\ntitle: Retry Budget\nclaims:\n" +
		"  - text: The service retries three times.\n    quotes:\n" +
		"      - The service retries three times before giving up on the request.\n" +
		"  - text: Backoff doubles on each attempt.\n    quotes:\n" +
		"      - Backoff doubles on each successive attempt until the ceiling.\n```\n"
	replyPath := filepath.Join(dir, "reply.md")
	if err := os.WriteFile(replyPath, []byte(reply), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}

	before := readFile(t, filepath.Join(dir, "c", "retry.md"))
	stdout, stderr, err := run(t,
		"--bundle", dir, "admit", "--key", key, "--submitter", "human:steve", replyPath)
	if err != nil {
		t.Fatalf("admit: %v\n%s\n%s", err, stdout, stderr)
	}

	// It must not say "quarantined" or send the reader to promote a document that is
	// already in the corpus — the first version did both.
	if strings.Contains(stdout, "quarantined") || strings.Contains(stdout, "gnosis promote") {
		t.Errorf("an accretion was reported as a quarantine:\n%s", stdout)
	}
	if !strings.Contains(stdout, "gained 1 quotation") {
		t.Errorf("the report does not say what was added:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Backoff") {
		t.Errorf("the unmatched claim was not named:\n%s", stderr)
	}

	after := readFile(t, filepath.Join(dir, "c", "retry.md"))
	if !strings.Contains(after, "giving up on the request") {
		t.Errorf("the new quotation was not appended:\n%s", after)
	}
	// The body is the invariant: everything after the closing delimiter is unchanged.
	if bodyOf(before) != bodyOf(after) {
		t.Errorf("accretion changed the body:\n--- before ---\n%s\n--- after ---\n%s",
			bodyOf(before), bodyOf(after))
	}
}

// TestAStaleConceptPromptIsRefused is §9.4's approved-diff window one level up: between
// emitting a prompt about a concept and admitting the reply, the concept can change,
// and an answer computed against bytes that are gone must not land.
func TestAStaleConceptPromptIsRefused(t *testing.T) {
	t.Parallel()
	dir, key := conceptPrompt(t)

	// Somebody edits the document after the prompt was emitted.
	docPath := filepath.Join(dir, "c", "retry.md")
	if err := os.WriteFile(docPath,
		[]byte(readFile(t, docPath)+"\nA sentence added afterwards.\n"), 0o600); err != nil {
		t.Fatalf("edit: %v", err)
	}

	const reply = "```yaml\ntype: Rule\ntitle: Retry Budget\nclaims:\n" +
		"  - text: The service retries three times.\n    quotes:\n" +
		"      - The service retries three times before giving up on the request.\n```\n"
	replyPath := filepath.Join(dir, "reply.md")
	if err := os.WriteFile(replyPath, []byte(reply), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}

	_, stderr, err := run(t,
		"--bundle", dir, "admit", "--key", key, "--submitter", "human:steve", replyPath)
	if err == nil {
		t.Error("a reply computed against stale bytes was applied")
	}
	if !strings.Contains(stderr, "changed since this prompt was emitted") {
		t.Errorf("the refusal does not say why: %s", stderr)
	}
}

// bodyOf returns everything after a document's closing frontmatter delimiter.
//
// The leading delimiter has no newline before it, so splitting the whole document on
// "\n---\n" finds the *closing* one first and yields two parts, not three. Trimming the
// opening delimiter before splitting is what makes the second part the body — a
// distinction the first version of this helper got wrong and blamed on the code.
func bodyOf(doc string) string {
	rest := strings.TrimPrefix(doc, "---\n")
	parts := strings.SplitN(rest, "\n---\n", 2)
	if len(parts) < 2 {
		return doc
	}
	return parts[1]
}

// conceptPrompt builds a bundle holding one archived source and one concept, emits a
// prompt bound to that concept, and returns the bundle and the prompt's key.
func conceptPrompt(t *testing.T) (dir, key string) {
	t.Helper()
	dir = t.TempDir()
	if _, _, err := run(t, "--bundle", dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	src := filepath.Join(dir, "source.txt")
	const text = "The service retries three times before giving up on the request.\n" +
		"Backoff doubles on each successive attempt until the ceiling.\n"
	if err := os.WriteFile(src, []byte(text), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	uri := "file://" + src
	if _, stderr, err := run(t, "--bundle", dir, "fetch", uri); err != nil {
		t.Fatalf("fetch: %v\n%s", err, stderr)
	}

	const doc = `---
type: Rule
title: "Retry Budget"
gnosis_id: 0192a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b
gnosis_schema_version: 1
gnosis_claims:
  - id: c1
    anchor: "The service retries three times."
    gnosis_evidence:
      - "retries three times"
---

# Retry Budget

The service retries three times.

`
	if err := os.WriteFile(filepath.Join(dir, "c", "retry.md"), []byte(doc), 0o600); err != nil {
		t.Fatalf("write concept: %v", err)
	}

	if _, stderr, err := run(t, "--bundle", dir, "ingest",
		"--into", "c/retry.md", "--model", "test", "--model-version", "1", uri,
	); err != nil {
		t.Fatalf("ingest: %v\n%s", err, stderr)
	}
	return dir, promptKey(t, dir)
}

// promptKey reads the key of the one prompt the bundle holds.
func promptKey(t *testing.T, dir string) string {
	t.Helper()
	metas, err := filepath.Glob(filepath.Join(dir, ".gnosis", "prompts", "*.json"))
	if err != nil || len(metas) != 1 {
		t.Fatalf("want one prompt meta, got %v (%v)", metas, err)
	}
	return strings.TrimSuffix(filepath.Base(metas[0]), ".json")
}
