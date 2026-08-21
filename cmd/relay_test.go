package cmd_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd/root"
)

const sourceText = "Vendor documentation.\n\n" +
	"The cache is cleared on restart and holds nothing across sessions.\n"

// relayBundle fetches one local source into a bundle and returns the bundle path
// and the source URI as tier 0 recorded it.
func relayBundle(t *testing.T) (bundleDir, uri string) {
	t.Helper()
	bundleDir = t.TempDir()
	src := filepath.Join(t.TempDir(), "cache.md")
	if err := os.WriteFile(src, []byte(sourceText), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if _, _, err := run(t, "--bundle", bundleDir, "--jsonl", "fetch", src); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	return bundleDir, src
}

// ingest emits prompts and returns the decoded payload.
func ingest(t *testing.T, bundleDir, uri string) map[string]any {
	t.Helper()
	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl", "ingest",
		"--model", "test-model", "--model-version", "v1", uri)
	if err != nil {
		t.Fatalf("ingest: %v\n%s", err, stderr)
	}
	return decodeData(t, stdout)
}

func decodeData(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var env struct {
		Status root.Status    `json:"status"`
		Data   map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout is not one JSON line: %v\n%s", err, stdout)
	}
	return env.Data
}

// firstPrompt returns the key and on-disk path of the one emitted prompt.
func firstPrompt(t *testing.T, data map[string]any) (key, path string) {
	t.Helper()
	prompts, _ := data["prompts"].([]any)
	if len(prompts) != 1 {
		t.Fatalf("want one prompt, got %v", prompts)
	}
	p, _ := prompts[0].(map[string]any)
	key, _ = p["key"].(string)
	path, _ = p["path"].(string)
	return key, path
}

// answer writes a reply quoting the source, and returns its path.
func answer(t *testing.T, quote string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "reply.md")
	body := "```yaml\n" +
		"title: Cache Lifetime\n" +
		"type: Reference\n" +
		"claims:\n" +
		"  - text: The cache is cleared on restart.\n" +
		"    quotes:\n" +
		"      - " + quote + "\n" +
		"```\n"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}
	return file
}

// TestTheRelayRoundTrips: ingest emits a prompt, an answer quoting the archived
// text is admitted, and the document lands in quarantine rather than the corpus.
func TestTheRelayRoundTrips(t *testing.T) {
	t.Parallel()
	bundleDir, uri := relayBundle(t)

	key, promptPath := firstPrompt(t, ingest(t, bundleDir, uri))
	if promptPath == "" {
		t.Fatal("no prompt was written")
	}
	prompt, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(promptPath)))
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	if !strings.Contains(string(prompt), "The cache is cleared on restart") {
		t.Error("the prompt does not carry the archived text")
	}

	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl",
		"admit", "--key", key, "--submitter", "agent:test",
		answer(t, "The cache is cleared on restart and holds nothing"))
	if err != nil {
		t.Fatalf("admit: %v\n%s", err, stderr)
	}

	data := decodeData(t, stdout)
	path, _ := data["path"].(string)
	if !strings.HasPrefix(path, "c/") {
		t.Fatalf("path = %q, want a c/ document", path)
	}
	// Quarantine, not the corpus. Getting the rest of the way is `promote`.
	if _, err = os.Stat(filepath.Join(bundleDir, filepath.FromSlash(path))); err == nil {
		t.Error("admit wrote straight into the bundle")
	}
	if _, err = os.Stat(filepath.Join(bundleDir, ".gnosis", "quarantine",
		filepath.FromSlash(path))); err != nil {
		t.Errorf("the document is not in quarantine: %v", err)
	}
}

// TestASecondIngestMakesNoCall is §6.1's determinism win, observed rather than
// asserted: after a reply is admitted, ingesting the same source emits nothing.
func TestASecondIngestMakesNoCall(t *testing.T) {
	t.Parallel()
	bundleDir, uri := relayBundle(t)

	key, _ := firstPrompt(t, ingest(t, bundleDir, uri))
	if _, _, err := run(t, "--bundle", bundleDir, "--jsonl",
		"admit", "--key", key, "--submitter", "agent:test",
		answer(t, "The cache is cleared on restart and holds nothing")); err != nil {
		t.Fatalf("admit: %v", err)
	}

	second := ingest(t, bundleDir, uri)
	if got := second["emitted"]; got != float64(0) {
		t.Errorf("emitted = %v, want 0 — the cache did not prevent a second ask", got)
	}
	if got := second["cached"]; got != float64(1) {
		t.Errorf("cached = %v, want 1", got)
	}
	// And the key is the same one, so the hit was real rather than a miss that
	// happened to write nothing.
	againKey, _ := firstPrompt(t, second)
	if againKey != key {
		t.Errorf("the key changed between runs: %s then %s", key, againKey)
	}
}

// TestChangingTheModelReAsks. A reply is a claim about what a particular model
// said, so serving one model's answer to another's question is a substitution
// nobody was told about.
func TestChangingTheModelReAsks(t *testing.T) {
	t.Parallel()
	bundleDir, uri := relayBundle(t)

	key, _ := firstPrompt(t, ingest(t, bundleDir, uri))
	if _, _, err := run(t, "--bundle", bundleDir, "--jsonl",
		"admit", "--key", key, "--submitter", "agent:test",
		answer(t, "The cache is cleared on restart and holds nothing")); err != nil {
		t.Fatalf("admit: %v", err)
	}

	stdout, _, err := run(t, "--bundle", bundleDir, "--jsonl", "ingest",
		"--model", "a-different-model", "--model-version", "v1", uri)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if got := decodeData(t, stdout)["emitted"]; got != float64(1) {
		t.Errorf("emitted = %v, want 1 — a different model reused an answer", got)
	}
}

// TestCacheOnlyRefusesAndLists is what a CI job branches on: findings, not an
// error, because the corpus is in a state the caller asked about and the tool
// worked correctly.
func TestCacheOnlyRefusesAndLists(t *testing.T) {
	t.Parallel()
	bundleDir, uri := relayBundle(t)

	stdout, _, err := run(t, "--bundle", bundleDir, "--jsonl", "ingest",
		"--model", "test-model", "--model-version", "v1", "--cache-only", uri)
	if err == nil {
		t.Fatal("--cache-only succeeded with nothing cached")
	}

	var exitErr root.ExitError
	if !errors.As(err, &exitErr) || root.Code(exitErr) != root.CodeFindings {
		t.Fatalf("err = %v, want exit %d (findings)", err, root.CodeFindings)
	}
	var env struct {
		Status root.Status `json:"status"`
	}
	if uErr := json.Unmarshal([]byte(stdout), &env); uErr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", uErr, stdout)
	}
	if env.Status != root.StatusFindings {
		t.Errorf("status = %q, want findings", env.Status)
	}
	// Nothing was emitted, which is the whole point of the flag.
	if _, sErr := os.Stat(filepath.Join(bundleDir, ".gnosis", "prompts")); sErr == nil {
		t.Error("--cache-only wrote prompts")
	}
}

// TestAFabricatedQuotationIsRefused, and the reply is still cached: the model call
// is spent either way, and paying twice to learn the same thing is not a policy.
func TestAFabricatedQuotationIsRefused(t *testing.T) {
	t.Parallel()
	bundleDir, uri := relayBundle(t)
	key, _ := firstPrompt(t, ingest(t, bundleDir, uri))

	_, stderr, err := run(t, "--bundle", bundleDir,
		"admit", "--key", key, "--submitter", "agent:test",
		answer(t, "A sentence that appears nowhere in the source at all"))
	if err == nil {
		t.Fatal("a fabricated quotation was admitted")
	}
	if !strings.Contains(stderr, "not in the archive") {
		t.Errorf("stderr does not say the quotation was absent: %q", stderr)
	}

	cached, cErr := os.ReadFile(filepath.Join(bundleDir, ".gnosis", "cache",
		key[:2], key+".json"))
	if cErr != nil {
		t.Fatalf("a refused reply was not cached: %v", cErr)
	}
	if !strings.Contains(string(cached), "appears nowhere") {
		t.Error("the cached entry is not the reply that was sent")
	}
}

// TestAnUncheckableQuotationIsNotCalledFabricated. quotecheck reports Unchecked
// for a passage too short to be evidence, and reporting that as Missing would
// accuse an agent of fabricating a quotation that may well be accurate.
func TestAnUncheckableQuotationIsNotCalledFabricated(t *testing.T) {
	t.Parallel()
	bundleDir, uri := relayBundle(t)
	key, _ := firstPrompt(t, ingest(t, bundleDir, uri))

	_, stderr, err := run(t, "--bundle", bundleDir,
		"admit", "--key", key, "--submitter", "agent:test",
		answer(t, "the cache"))
	if err == nil {
		t.Fatal("an uncheckable quotation was admitted")
	}
	if !strings.Contains(stderr, "could not be checked") {
		t.Errorf("stderr does not distinguish unchecked from missing: %q", stderr)
	}
	if strings.Contains(stderr, "not in the archive") {
		t.Errorf("a too-short quotation was reported as fabricated: %q", stderr)
	}
}

// TestDryRunWritesNoDocument, and still reports the same verdict.
func TestDryRunWritesNoDocument(t *testing.T) {
	t.Parallel()
	bundleDir, uri := relayBundle(t)
	key, _ := firstPrompt(t, ingest(t, bundleDir, uri))

	stdout, _, err := run(t, "--bundle", bundleDir, "--jsonl",
		"admit", "--key", key, "--submitter", "agent:test", "--dry-run",
		answer(t, "The cache is cleared on restart and holds nothing"))
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if got := decodeData(t, stdout)["would_quarantine"]; got != true {
		t.Errorf("would_quarantine = %v, want true", got)
	}

	entries, _ := os.ReadDir(filepath.Join(bundleDir, ".gnosis", "quarantine"))
	if len(entries) != 0 {
		t.Errorf("a dry run quarantined something: %v", entries)
	}
}

// TestAnUnfetchedSourceIsRefused: ingest builds prompts from the archive, so a
// source nobody fetched has no text to ask about.
func TestAnUnfetchedSourceIsRefused(t *testing.T) {
	t.Parallel()
	_, _, err := run(t, "--bundle", t.TempDir(), "ingest",
		"--model", "m", "--model-version", "v", "https://example.org/never-fetched")
	if err == nil {
		t.Fatal("an unfetched source was ingested")
	}
}

// TestModelIsRequired. A default would put a value nobody chose into every cache
// key, and the first person to change it would invalidate the whole cache without
// having decided to.
func TestModelIsRequired(t *testing.T) {
	t.Parallel()
	bundleDir, uri := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "ingest", uri); err == nil {
		t.Fatal("ingest ran with no --model")
	}
}
