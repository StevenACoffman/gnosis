package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// claimMarker is the heading the critic prompt puts above the claim. The scripted
// critic finds the fenced claim by it, so it is stated once: a mutation that stopped
// finding the section would stop critiquing, and pass.
const claimMarker = "## The claim"

// errNoClaimInPrompt is what the scripted critic returns when the prompt gave it
// nothing to judge.
//
// It is this fixture's most important failure, for `errNoQuotableSource`'s reason: a
// critic that cannot find the claim is reporting a defect in the **prompt**, and a
// fixture that answered anyway — with a finding it invented, or with none — would be a
// playback rather than a test.
var errNoClaimInPrompt = errors.New("the prompt carries no claim to judge")

// scriptedCritic produces a verdict derived only from the prompt it was handed.
//
// Requires: prompt is a rendered critic prompt, exactly as `gnosis critic` wrote it.
// Ensures: a reply in the format the prompt asks for, naming what it examined — or
// errNoClaimInPrompt when the prompt carried no claim. Pure.
//
// It performs no judgement, which is the point: what is under test is the **contract**
// — that the prompt carries what a replier needs, that a well-formed verdict survives
// filing, and that the coverage it declares comes back round to steer the next prompt.
// A fixture that reasoned would be testing a model, and there is no reproducible one.
func scriptedCritic(prompt string) (string, error) {
	claim, err := fencedAfter(prompt, claimMarker)
	if err != nil {
		return "", err
	}
	subject, err := claimSentence(claim)
	if err != nil {
		return "", err
	}
	// The reply's shape is copied from the prompt's own "Reply format" section rather
	// than from gnosis's parser, for the reason the extraction fixture gives: an agent
	// reads the instructions it was given.
	return "```yaml\n" +
		"findings:\n" +
		"  - category: scope\n" +
		"    message: The source bounds what \"" + subject + "\" states, and the" +
		" claim does not.\n" +
		"examined:\n" +
		"  - whether the quotation supports the scope the claim asserts\n" +
		"not_examined:\n" +
		"  - aspect: the source's own methodology\n" +
		"    reason: the prompt carries an excerpt rather than the whole source\n" +
		"```\n", nil
}

// fencedAfter extracts the text a prompt fenced under one heading.
//
// The fence is discovered rather than assumed, for `fencedSource`'s reason: an agent
// reading the prompt sees a run of backticks and does not know how long gnosis chose to
// make it.
func fencedAfter(prompt, marker string) (string, error) {
	at := strings.Index(prompt, marker)
	if at < 0 {
		return "", errNoClaimInPrompt
	}
	rest := prompt[at+len(marker):]

	open := strings.Index(rest, "``````")
	if open < 0 {
		return "", errNoClaimInPrompt
	}
	lineEnd := strings.IndexByte(rest[open:], '\n')
	if lineEnd < 0 {
		return "", errNoClaimInPrompt
	}
	fence := strings.TrimRight(rest[open:open+lineEnd], "\r")
	body := rest[open+lineEnd+1:]

	end := strings.Index(body, fence)
	if end < 0 {
		return "", errNoClaimInPrompt
	}
	return body[:end], nil
}

// claimSentence is the assertion the fenced claim block states.
//
// It reads the `Claim:` line, which is the prompt's own label. A fixture that took the
// whole block would quote the label and the quotations back, which is not what a critic
// is looking at.
func claimSentence(block string) (string, error) {
	for _, line := range strings.Split(block, "\n") {
		if text, ok := strings.CutPrefix(strings.TrimSpace(line), "Claim:"); ok {
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				return trimmed, nil
			}
		}
	}
	return "", errNoClaimInPrompt
}

// TestTheCriticRoundTrips is §18.6's relay test for the second relay.
//
// The scripted critic can see **only the prompt**, so if `RenderCritic` stops fencing
// the claim or stops carrying the source, it cannot answer, the verdict is never filed,
// and this fails. That is the property a scripted model buys and a real one cannot: the
// assertion is on what the agent was *sent*.
func TestTheCriticRoundTrips(t *testing.T) {
	t.Parallel()

	bundleDir := criticBundle(t)
	key, promptPath := firstPrompt(t, criticRun(t, bundleDir))
	if promptPath == "" {
		t.Fatal("no critic prompt was written")
	}
	prompt, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(promptPath)))
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	if !strings.Contains(string(prompt), sourceText[:20]) {
		t.Error("the critic prompt does not carry the archived text")
	}

	verdict, err := scriptedCritic(string(prompt))
	if err != nil {
		t.Fatalf("the scripted critic could not answer: %v\n%s", err, prompt)
	}
	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl",
		"critic", "--key", key, "--response", answerFile(t, verdict))
	if err != nil {
		t.Fatalf("filing the verdict: %v\n%s", err, stderr)
	}

	data := decodeData(t, stdout)
	findings, _ := data["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("want one finding, got %v", data["findings"])
	}
	first, _ := findings[0].(map[string]any)
	if first["severity"] != "warning" {
		t.Errorf("severity = %v, want warning: a verdict is advisory and the reply "+
			"has no way to ask for anything else", first["severity"])
	}
	if got, _ := first["category"].(string); !strings.HasPrefix(got, "critic:") {
		t.Errorf("category = %q, want the critic namespace", got)
	}
}

// TestASecondCritiqueIsANewQuestion is the property that makes the coverage ledger
// worth keeping.
//
// §6.1's cache would otherwise serve the first verdict for the second ask. It does not,
// because the coverage goes into the prompt and the prompt goes into the key — so the
// second critique is asked about ground nobody has covered, and says so.
func TestASecondCritiqueIsANewQuestion(t *testing.T) {
	t.Parallel()

	bundleDir := criticBundle(t)
	key, promptPath := firstPrompt(t, criticRun(t, bundleDir))
	prompt, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(promptPath)))
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	verdict, err := scriptedCritic(string(prompt))
	if err != nil {
		t.Fatalf("the scripted critic could not answer: %v", err)
	}
	if _, stderr, fErr := run(t, "--bundle", bundleDir, "--jsonl",
		"critic", "--key", key, "--response", answerFile(t, verdict)); fErr != nil {
		t.Fatalf("filing the verdict: %v\n%s", fErr, stderr)
	}

	secondKey, secondPath := firstPrompt(t, criticRun(t, bundleDir))
	if secondKey == key {
		t.Fatal("the second critique reused the first's key, so the cache would" +
			" serve one question's answer for another")
	}
	second, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(secondPath)))
	if err != nil {
		t.Fatalf("read the second prompt: %v", err)
	}
	if !strings.Contains(string(second), "the source's own methodology") {
		t.Errorf("the second prompt does not carry what the first declined to"+
			" examine:\n%s", second)
	}
	// And what the first critique *did* examine is listed as covered, so the second
	// is steered off it rather than onto it.
	if !strings.Contains(string(second), "Already examined") {
		t.Errorf("the second prompt does not say what was covered:\n%s", second)
	}
}

// criticBundle fetches one source, files a claim citing it, and returns the bundle.
//
// The claim is written by hand rather than admitted through the relay: what is under
// test is the critic, and driving the extraction relay first would make this fixture
// fail for that relay's reasons.
func criticBundle(t *testing.T) string {
	t.Helper()

	bundleDir, _ := relayBundle(t)
	archive := firstArchivedFile(t, bundleDir)
	doc := "---\ntype: Reference\ntitle: Cache Lifetime\n" +
		"gnosis_id: 01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d\n" +
		"gnosis_claims:\n" +
		"  - id: claim-cache\n" +
		"    anchor: The cache is cleared on restart.\n" +
		"    lead: Restarting clears the cache.\n" +
		"    gnosis_evidence:\n" +
		"      - The cache is cleared on restart\n" +
		"    archive_paths:\n      - " + archive + "\n" +
		"---\n\n# Cache Lifetime\n\nThe cache is cleared on restart.\n"

	dir := filepath.Join(bundleDir, "c")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	name := "01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d-cache-lifetime.md"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(doc), 0o600); err != nil {
		t.Fatalf("write concept: %v", err)
	}
	return bundleDir
}

// firstArchivedFile is the one text file tier 0 holds, bundle-relative.
func firstArchivedFile(t *testing.T, bundleDir string) string {
	t.Helper()

	var found string
	root := filepath.Join(bundleDir, "evidence", "text")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || found != "" {
			return err
		}
		rel, rErr := filepath.Rel(bundleDir, path)
		if rErr != nil {
			return rErr
		}
		found = filepath.ToSlash(rel)
		return nil
	})
	if err != nil || found == "" {
		t.Fatalf("no archived text under %s: %v", root, err)
	}
	return found
}

// criticRun emits prompts and returns the decoded payload.
func criticRun(t *testing.T, bundleDir string) map[string]any {
	t.Helper()

	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl", "critic",
		"--model", "test-model", "--model-version", "v1")
	if err != nil {
		t.Fatalf("critic: %v\n%s", err, stderr)
	}
	return decodeData(t, stdout)
}

// answerFile writes a reply to a file and returns its path.
func answerFile(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "verdict.md")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}
	return path
}
