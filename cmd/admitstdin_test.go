package cmd_test

import (
	"strings"
	"testing"
)

// TestAReplyArrivesOnStdin is what §8.2's relay chaining actually needed.
//
// The entry read `adh run --relay` as one invocation that emits a prompt and blocks
// reading the reply. It is not: adh emits and stops, and a second invocation resumes
// from `--response <file>`, where `-` is stdin. gnosis already had that shape in two
// commands — so what was missing was reading the reply from a pipe, and this is the
// round trip closing without a temporary file.
func TestAReplyArrivesOnStdin(t *testing.T) {
	t.Parallel()

	bundleDir, uri := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	key, promptPath := firstPrompt(t, ingest(t, bundleDir, uri))

	stdout, stderr, err := runWithStdin(t, goodReply, "--bundle", bundleDir, "--jsonl",
		"admit", "--key", key, "--submitter", "agent:test", "--stdin")
	if err != nil {
		t.Fatalf("admit from stdin: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, "quarantined") {
		t.Errorf("the reply was not filed: %s", stdout)
	}
	// And it went the whole way through, rather than being read and dropped: the
	// prompt it answered is gone, which only a filed reply does.
	if exists(t, bundleDir, promptPath) {
		t.Error("the prompt survived a reply admitted from stdin")
	}
}

// TestAnEmptyStdinIsReportedAsItself is the failure this path makes easy to produce.
//
// `gnosis admit --key K - < /dev/null`, or a pipeline whose first stage failed, hands
// over nothing. Passed through, that surfaces as "the reply is not valid YAML", which
// sends a reader to inspect a reply that does not exist. The cause is knowable here,
// so it is named here.
func TestAnEmptyStdinIsReportedAsItself(t *testing.T) {
	t.Parallel()

	bundleDir, uri := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	key, _ := firstPrompt(t, ingest(t, bundleDir, uri))

	_, stderr, err := runWithStdin(t, "", "--bundle", bundleDir,
		"admit", "--key", key, "--submitter", "agent:test", "--stdin")
	if err == nil {
		t.Fatal("an empty reply was accepted")
	}
	if !strings.Contains(stderr, "empty") {
		t.Errorf("stderr does not name the cause: %s", stderr)
	}
}

// TestAFileIsStillReadFromDisk keeps the new path from swallowing the old one.
//
// The defect worth guarding is an ordinary reply file being ignored in favour of
// whatever happens to be on stdin, so this hands over a reply that would be refused on
// stdin and one that passes in a file: if the path were ignored, it would fail.
func TestAFileIsStillReadFromDisk(t *testing.T) {
	t.Parallel()

	bundleDir, uri := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	key, _ := firstPrompt(t, ingest(t, bundleDir, uri))

	// stdin carries a reply that would be refused; the file carries one that passes.
	// If the path were ignored, this would fail.
	_, stderr, err := runWithStdin(t, "not yaml at all\n", "--bundle", bundleDir,
		"--jsonl", "admit", "--key", key, "--submitter", "agent:test",
		reply(t, goodReply))
	if err != nil {
		t.Fatalf("a named reply file was not read: %v\n%s", err, stderr)
	}
}

// TestStdinAndAFileAreAlternatives is the mistake the flag makes possible.
//
// A caller who scripted one and typed the other has given two answers to one question,
// and reading either silently would file a reply they did not choose. Both named is a
// usage error, and so is neither.
func TestStdinAndAFileAreAlternatives(t *testing.T) {
	t.Parallel()

	bundleDir, uri := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	key, _ := firstPrompt(t, ingest(t, bundleDir, uri))

	for name, args := range map[string][]string{
		"both": {
			"admit", "--key", key, "--submitter", "agent:test",
			"--stdin", "ignored.md",
		},
		"neither": {"admit", "--key", key, "--submitter", "agent:test"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, stderr, err := runWithStdin(t, goodReply,
				append([]string{"--bundle", bundleDir}, args...)...)
			if err == nil {
				t.Fatal("the invocation was accepted")
			}
			if !strings.Contains(stderr, "--stdin") {
				t.Errorf("stderr does not name the alternatives: %s", stderr)
			}
		})
	}
}
