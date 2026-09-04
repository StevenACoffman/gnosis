package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scriptedAnswerer produces a reply derived only from the prompt it was handed.
//
// Requires: prompt is a rendered ask prompt, exactly as `gnosis ask` wrote it.
// Ensures: a reply in the format the prompt asks for, citing the first claim heading the
// prompt carries — or errNoClaimInPrompt when it carries none. Pure.
//
// It performs no reasoning, which is the point: what is under test is the **contract** —
// that the prompt carries references an answer can cite, that a well-formed answer
// survives filing, and that the draft lands in quarantine rather than in the corpus.
func scriptedAnswerer(prompt string) (string, error) {
	ref := firstCitedRef(prompt)
	if ref == "" {
		return "", errNoClaimInPrompt
	}
	// The reply's shape is copied from the prompt's own "Reply format" section rather
	// than from gnosis's parser, for the reason the extraction fixture gives: an agent
	// reads the instructions it was given.
	return "```yaml\n" +
		"title: What The Corpus Says\n" +
		"answer: >-\n" +
		"  The corpus addresses this in one claim, quoted in the evidence below.\n" +
		"cites:\n" +
		"  - " + ref + "\n" +
		"unanswered: >-\n" +
		"  Anything the retrieved claim does not state.\n" +
		"```\n", nil
}

// firstCitedRef is the first claim reference the prompt offers, which the prompt writes
// as a level-three heading above each claim.
//
// Discovered rather than assumed, for `fencedAfter`'s reason: an agent reading the
// prompt sees the headings and copies one, and a fixture that knew the reference from
// the fixture data would keep passing after the prompt stopped carrying it.
func firstCitedRef(prompt string) string {
	for _, line := range strings.Split(prompt, "\n") {
		if ref, ok := strings.CutPrefix(line, "### "); ok {
			return strings.TrimSpace(ref)
		}
	}
	return ""
}

// TestAskAndFileRoundTrip is §18.6's relay test for the third relay.
//
// The scripted answerer can see **only the prompt**, so if `RenderAsk` stops carrying
// the claim references, it cannot cite, the answer is refused, and this fails. That is
// the property a scripted model buys and a real one cannot: the assertion is on what the
// agent was *sent*.
func TestAskAndFileRoundTrip(t *testing.T) {
	t.Parallel()

	bundleDir := criticBundle(t)
	indexBundle(t, bundleDir)

	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl",
		"ask", "--model", "test-model", "--model-version", "v1", "cache", "restart")
	if err != nil {
		t.Fatalf("ask: %v\n%s", err, stderr)
	}
	data := decodeData(t, stdout)
	if data["answerability"] != "ready" {
		t.Fatalf("answerability = %v, want ready; remedy: %v",
			data["answerability"], data["remedy"])
	}
	prompt, ok := data["prompt"].(map[string]any)
	if !ok {
		t.Fatalf("an answerable question emitted no prompt: %v", data)
	}
	key, _ := prompt["key"].(string)
	path, _ := prompt["path"].(string)

	body, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	reply, err := scriptedAnswerer(string(body))
	if err != nil {
		t.Fatalf("the scripted answerer could not answer: %v\n%s", err, body)
	}

	filedOut, stderr, err := run(t, "--bundle", bundleDir, "--jsonl",
		"file", "--key", key, "--response", answerFile(t, reply), "--by", "human:steve")
	if err != nil {
		t.Fatalf("file: %v\n%s", err, stderr)
	}
	filed := decodeData(t, filedOut)
	rel, _ := filed["path"].(string)

	// The draft is in quarantine and **not** in the corpus. A filed answer that
	// landed straight in `c/` would have skipped the gate §8.3 requires it to pass.
	if _, sErr := os.Stat(filepath.Join(bundleDir, filepath.FromSlash(rel))); !os.IsNotExist(sErr) {
		t.Errorf("the draft was written into the corpus at %s rather than quarantined", rel)
	}
	quarantined := filepath.Join(bundleDir, ".gnosis", "quarantine", filepath.FromSlash(rel))
	draft, err := os.ReadFile(quarantined)
	if err != nil {
		t.Fatalf("the draft is not in quarantine: %v", err)
	}
	// Its evidence is the cited claim's own, which is what makes it checkable by the
	// gate rather than a page of prose asserting things.
	if !strings.Contains(string(draft), "archive_paths:") {
		t.Errorf("the draft carries no archived source, so the gate has nothing to"+
			" check it against:\n%s", draft)
	}
	if !strings.Contains(string(draft), "The cache is cleared on restart") {
		t.Errorf("the draft did not inherit the cited claim's quotation:\n%s", draft)
	}
	assertGateAccepts(t, bundleDir, rel)
}

// assertGateAccepts runs the draft through the promote gate and fails on any signal that
// **ran and failed**.
//
// This is the assertion a hand run had to find first, and it is here so it is not found
// by hand again. A filed draft declaring no sources fails the provenance signal — "a
// document asserting claims and citing nothing is exactly what this corpus exists to
// refuse" — and every test above it still passed, because they check what the draft says
// and not whether the gate will have it.
//
// A signal that **could not run** is not a failure here. `conflict` needs a corpus the
// fixture does not build, and the gate's own answer to that is `needs_human`, which is
// the correct outcome rather than a defect in what was filed.
func assertGateAccepts(t *testing.T, bundleDir, rel string) {
	t.Helper()

	_, stderr, _ := run(t, "--bundle", bundleDir, "promote", "--approver", "human:test", rel)
	if strings.Contains(stderr, "failed:") {
		t.Errorf("the filed draft fails a signal the gate could run:\n%s", stderr)
	}
}

// TestAskRefusesASilentCorpusWithoutFailing is §17.0.1's requirement: a refusal is an
// ordinary outcome, and an exit code that said otherwise would make "the corpus does not
// know" indistinguishable from "the command broke" — at which point a script's first
// response to a silent corpus is to retry it.
func TestAskRefusesASilentCorpusWithoutFailing(t *testing.T) {
	t.Parallel()

	bundleDir := corpus(t)
	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl",
		"ask", "--model", "test-model", "kubernetes ingress annotations")
	if err != nil {
		t.Fatalf("a refusal was reported as a failure: %v\n%s", err, stderr)
	}
	data := decodeData(t, stdout)
	if data["answerability"] != "silent" {
		t.Errorf("answerability = %v, want silent", data["answerability"])
	}
	if data["prompt"] != nil {
		t.Error("a prompt was emitted for a question nothing was retrieved for")
	}
	if remedy, _ := data["remedy"].(string); remedy == "" {
		t.Error("the refusal names no remedy, so a reader is told what happened and" +
			" not what to do about it")
	}
}

// TestFileRefusesADeclination. "The corpus does not say" is a real answer and it is not
// a concept: filing it would put a statement of absence where the next reader retrieves
// it as a claim.
func TestFileRefusesADeclination(t *testing.T) {
	t.Parallel()

	bundleDir := criticBundle(t)
	indexBundle(t, bundleDir)
	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl",
		"ask", "--model", "test-model", "cache", "restart")
	if err != nil {
		t.Fatalf("ask: %v\n%s", err, stderr)
	}
	prompt, _ := decodeData(t, stdout)["prompt"].(map[string]any)
	key, _ := prompt["key"].(string)

	declination := "```yaml\nanswer: \"\"\ncites: []\n" +
		"unanswered: these claims say nothing about the question\n```\n"
	_, stderr, err = run(t, "--bundle", bundleDir,
		"file", "--key", key, "--response", answerFile(t, declination))
	if err == nil {
		t.Fatal("a declination was filed as a concept")
	}
	if !strings.Contains(stderr, "declined") {
		t.Errorf("the refusal does not say what was wrong: %s", stderr)
	}
}

// TestFileRefusesACitationTheCorpusDoesNotHold. The prompt offered it, so the parser
// admits it; between the prompt and the reply the claim was deleted. Filing it anyway
// would produce a draft that passes the gate on the quotations that remain and asserts
// the rest.
func TestFileRefusesACitationTheCorpusDoesNotHold(t *testing.T) {
	t.Parallel()

	bundleDir := criticBundle(t)
	indexBundle(t, bundleDir)
	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl",
		"ask", "--model", "test-model", "cache", "restart")
	if err != nil {
		t.Fatalf("ask: %v\n%s", err, stderr)
	}
	data := decodeData(t, stdout)
	prompt, _ := data["prompt"].(map[string]any)
	key, _ := prompt["key"].(string)
	cites, _ := data["cites"].([]any)
	ref, _ := cites[0].(string)

	// The document goes away after the prompt was emitted.
	matches, _ := filepath.Glob(filepath.Join(bundleDir, "c", "*.md"))
	for _, m := range matches {
		if rErr := os.Remove(m); rErr != nil {
			t.Fatalf("remove concept: %v", rErr)
		}
	}

	reply := "```yaml\ntitle: Gone\nanswer: something\ncites:\n  - " + ref + "\n```\n"
	_, stderr, err = run(t, "--bundle", bundleDir,
		"file", "--key", key, "--response", answerFile(t, reply))
	if err == nil {
		t.Fatal("a draft was filed citing a claim the corpus no longer holds")
	}
	if !strings.Contains(stderr, ref) {
		t.Errorf("the refusal does not name the citation that is gone: %s", stderr)
	}
}

// indexBundle builds the index the claim query reads.
func indexBundle(t *testing.T, bundleDir string) {
	t.Helper()

	if _, stderr, err := run(t, "--bundle", bundleDir, "index", "rebuild"); err != nil {
		t.Fatalf("index rebuild: %v\n%s", err, stderr)
	}
}
