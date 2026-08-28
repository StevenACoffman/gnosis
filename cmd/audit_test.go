package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// outstanding runs the report and decodes it.
func outstanding(t *testing.T, bundleDir string) (paths []string, drafts int, err error) {
	t.Helper()

	stdout, _, err := run(t, "--bundle", bundleDir, "--jsonl", "audit", "--outstanding")
	var env struct {
		Data struct {
			Abandoned []struct {
				Path     string `json:"path"`
				Attempts int    `json:"attempts"`
				Reason   string `json:"reason"`
			} `json:"abandoned"`
			Drafts int `json:"drafts"`
		} `json:"data"`
	}
	if jErr := json.Unmarshal([]byte(stdout), &env); jErr != nil {
		t.Fatalf("stdout is not one JSON line: %v\n%s", jErr, stdout)
	}
	for _, a := range env.Data.Abandoned {
		paths = append(paths, a.Path)
	}
	return paths, env.Data.Drafts, err
}

// TestAnAbandonedDecisionIsReported is what §15 asked for: the states were already
// recorded and nothing enumerated them, so a promotion somebody was asked about and
// walked away from was indistinguishable from a draft nobody had looked at.
func TestAnAbandonedDecisionIsReported(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	// Ask, and answer nothing. An empty stdin declines the confirmation, which is
	// exactly the walk-away this report exists to surface.
	if _, _, err := run(t, "--bundle", bundleDir,
		"promote", "--apply", "--approver", "human:priya",
		"--rationale", "read the source myself", path,
	); err == nil {
		t.Fatal("a promotion with no confirmation succeeded")
	}

	paths, drafts, err := outstanding(t, bundleDir)
	if err == nil {
		t.Error("an outstanding decision exited zero")
	}
	if len(paths) != 1 || paths[0] != path {
		t.Fatalf("abandoned = %v, want [%s]", paths, path)
	}
	// The denominator travels with it, because §17 forbids a count presented as
	// health and one outstanding decision means something different against one
	// draft than against four hundred.
	if drafts != 1 {
		t.Errorf("drafts = %d, want 1", drafts)
	}
}

// TestADecidedPromotionIsNotOutstanding is the subtraction that matters most: a
// report listing decisions already taken is one nobody reads twice.
func TestADecidedPromotionIsNotOutstanding(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	// Asked, walked away, then came back and decided — which is the ordinary shape
	// of a promotion that needed thinking about.
	if _, _, err := run(t, "--bundle", bundleDir,
		"promote", "--apply", "--approver", "human:priya",
		"--rationale", "read the source myself", path,
	); err == nil {
		t.Fatal("a promotion with no confirmation succeeded")
	}
	if _, stderr, err := runWithStdin(t, path+"\n", "--bundle", bundleDir,
		"promote", "--apply", "--approver", "human:priya",
		"--rationale", "read the source myself", path,
	); err != nil {
		t.Fatalf("promote: %v\n%s", err, stderr)
	}

	paths, _, err := outstanding(t, bundleDir)
	if err != nil {
		t.Errorf("a settled corpus exited non-zero: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("a decided promotion is still outstanding: %v", paths)
	}
}

// TestADraftNobodyAskedAboutIsNotAbandoned keeps the report from reporting a fresh
// corpus as a pile of neglect on its first day.
//
// That draft is unexamined, `quarantine` lists it, and a warning true of everything
// on day one teaches a reader to skip the category.
func TestADraftNobodyAskedAboutIsNotAbandoned(t *testing.T) {
	t.Parallel()
	bundleDir, _ := waiting(t)

	paths, drafts, err := outstanding(t, bundleDir)
	if err != nil {
		t.Errorf("a corpus nobody has been asked about exited non-zero: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("an unexamined draft is reported as abandoned: %v", paths)
	}
	// It is still counted as a draft, so the report says what it looked at.
	if drafts != 1 {
		t.Errorf("drafts = %d, want 1", drafts)
	}
}

// TestARefusedClaimIsRecorded is the asymmetry the entry names: gnosis recorded what a
// source supports and kept no trace of what it was found not to support, so the same
// assertion could be offered again with nothing saying it had been tried.
func TestARefusedClaimIsRecorded(t *testing.T) {
	t.Parallel()

	bundleDir, uri := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	key, _ := firstPrompt(t, ingest(t, bundleDir, uri))

	// A quotation the archived text does not contain: refused, and the claim text is
	// what the corpus had no way to keep.
	fabricated := "```yaml\ntitle: Cache Lifetime\ntype: Reference\nclaims:\n" +
		"  - text: The cache is shared across every session.\n    quotes:\n" +
		"      - The cache is shared across every session and never cleared\n```\n"
	if _, _, err := run(t, "--bundle", bundleDir, "--jsonl", "admit",
		"--key", key, "--submitter", "agent:test", reply(t, fabricated),
	); err == nil {
		t.Fatal("a fabricated quotation was admitted")
	}

	stdout, stderr, err := run(t, "--bundle", bundleDir, "audit", "--unsupported")
	if err == nil {
		t.Error("a withheld claim exited zero")
	}
	if !strings.Contains(stdout, "shared across every session") {
		t.Errorf("the report does not say what was withheld:\n%s", stdout)
	}
	if !strings.Contains(stdout, "agent:test") {
		t.Errorf("the report does not say who asserted it:\n%s", stdout)
	}
	if !strings.Contains(stderr, "1 claim withheld") {
		t.Errorf("the summary does not count it: %s", stderr)
	}
}

// TestASupportedReplyWithholdsNothing is the calibration case: without it the test
// above would pass for a report that listed every admit.
func TestASupportedReplyWithholdsNothing(t *testing.T) {
	t.Parallel()

	bundleDir, uri := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	key, _ := firstPrompt(t, ingest(t, bundleDir, uri))
	if _, stderr, err := run(t, "--bundle", bundleDir, "--jsonl", "admit",
		"--key", key, "--submitter", "agent:test", reply(t, goodReply),
	); err != nil {
		t.Fatalf("admit: %v\n%s", err, stderr)
	}

	if _, stderr, err := run(t, "--bundle", bundleDir, "audit", "--unsupported"); err != nil {
		t.Errorf("a corpus withholding nothing exited non-zero: %v\n%s", err, stderr)
	} else if !strings.Contains(stderr, "no claims were withheld") {
		t.Errorf("the empty case says nothing: %s", stderr)
	}
}

// TestAuditTakesExactlyOneReport keeps two lists off one stream, and keeps a bare
// invocation from acquiring a default that would change meaning when `--reversed`
// lands (§10.6.5).
func TestAuditTakesExactlyOneReport(t *testing.T) {
	t.Parallel()
	bundleDir, _ := waiting(t)

	for _, args := range [][]string{
		{"audit"},
		{"audit", "--outstanding", "--unsupported"},
	} {
		_, stderr, err := run(t, append([]string{"--bundle", bundleDir}, args...)...)
		if err == nil {
			t.Errorf("%v was accepted", args)
		}
		if !strings.Contains(stderr, "exactly one report") {
			t.Errorf("%v: the usage error does not say why: %s", args, stderr)
		}
	}
}

// TestChurnListsOnlySourcesThatMoved is the computable half of FPF's Effort field: how
// often a claim's sources move. It needed no new field — a source fetched twice has two
// records, so the record count per source *is* how many times the bytes changed.
func TestChurnListsOnlySourcesThatMoved(t *testing.T) {
	t.Parallel()

	bundleDir := t.TempDir()
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	moving := filepath.Join(t.TempDir(), "moving.md")
	still := filepath.Join(t.TempDir(), "still.md")
	writeSource(t, moving, "Vendor documentation.\n\n"+quoted+"\n")
	writeSource(t, still, "A page that never changes and holds one settled sentence.\n")
	for _, src := range []string{moving, still} {
		if _, stderr, err := run(t, "--bundle", bundleDir, "--jsonl", "fetch", src); err != nil {
			t.Fatalf("fetch: %v\n%s", err, stderr)
		}
	}

	// Move one of them twice, so tier 0 holds three versions of it and one of the
	// other.
	for _, text := range []string{"# One\n\n" + quoted + "\n", "# Two\n\n" + quoted + "\n"} {
		writeSource(t, moving, text)
		if _, stderr, err := run(t, "--bundle", bundleDir, "--jsonl", "fetch", moving); err != nil {
			t.Fatalf("re-fetch: %v\n%s", err, stderr)
		}
	}

	stdout, stderr, err := run(t, "--bundle", bundleDir, "audit", "--churn")
	if err != nil {
		t.Fatalf("churn: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, "3 versions") || !strings.Contains(stdout, "moving.md") {
		t.Errorf("the moved source is not reported:\n%s", stdout)
	}
	// The settled one is not a row. A register with a 1 beside every source would
	// bury the handful that moved among the hundreds that did not.
	if strings.Contains(stdout, "still.md") {
		t.Errorf("a source that never moved is a row:\n%s", stdout)
	}
	// And the denominator travels with it, because a count with no denominator is
	// what §17 forbids.
	if !strings.Contains(stderr, "recorded source") {
		t.Errorf("the summary carries no denominator: %s", stderr)
	}
}

// TestChurnOnASettledCorpusIsNotAFinding. A source that moves is doing what sources
// do; §14.3.2 calls a benign drift "not a downgrade of trust", and rendering churn as
// something to fix would train a reader past the row that matters. Withdrawn support is
// already a finding where it happens.
func TestChurnOnASettledCorpusIsNotAFinding(t *testing.T) {
	t.Parallel()
	bundleDir, _ := relayBundle(t)

	_, stderr, err := run(t, "--bundle", bundleDir, "audit", "--churn")
	if err != nil {
		t.Fatalf("churn on a settled corpus exited non-zero: %v\n%s", err, stderr)
	}
	if !strings.Contains(stderr, "no source has moved") {
		t.Errorf("the settled case says nothing: %s", stderr)
	}
}

// TestGainedCountsWhatLanded is Hamming's asymmetry corrected: every other report in
// this command counts something wrong, and a corpus whose only visible signal is
// problems-found rewards contributing less.
func TestGainedCountsWhatLanded(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	// Promote it, so the trail holds a fetch, an admit, and a promotion.
	if _, stderr, err := runWithStdin(t, path+"\n", "--bundle", bundleDir,
		"promote", "--apply", "--approver", "human:priya",
		"--rationale", "read the source myself", path,
	); err != nil {
		t.Fatalf("promote: %v\n%s", err, stderr)
	}

	stdout, stderr, err := run(t, "--bundle", bundleDir, "audit", "--gained")
	if err != nil {
		t.Fatalf("--gained exited non-zero on good news: %v\n%s", err, stderr)
	}
	for _, want := range []string{"documents promoted", "replies admitted", "sources archived"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the report omits %q:\n%s", want, stdout)
		}
	}
	// The window travels with the counts, because a count with no period is
	// uninterpretable.
	if !strings.Contains(stderr, "since ") {
		t.Errorf("the report does not say what period it covers: %s", stderr)
	}
}

// TestNothingGainedIsSaidRatherThanPrintedAsZeroes keeps "nothing since Tuesday" from
// rendering like "we did not look".
func TestNothingGainedIsSaidRatherThanPrintedAsZeroes(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	stdout, stderr, err := run(t, "--bundle", dir, "audit", "--gained")
	if err != nil {
		t.Fatalf("--gained on an untouched corpus: %v\n%s", err, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("an empty window printed rows:\n%s", stdout)
	}
	if !strings.Contains(stderr, "nothing gained since") {
		t.Errorf("the empty case says nothing: %s", stderr)
	}
}

// TestTheWindowExcludesWhatIsOutsideIt, and a date gnosis cannot parse is a usage error
// naming the form rather than a Go parse error.
func TestTheWindowExcludesWhatIsOutsideIt(t *testing.T) {
	t.Parallel()
	bundleDir, _ := waiting(t)

	// A window that starts tomorrow contains nothing this fixture did.
	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format(time.DateOnly)
	_, stderr, err := run(t, "--bundle", bundleDir, "audit", "--gained", "--since", tomorrow)
	if err != nil {
		t.Fatalf("--gained with a future window: %v\n%s", err, stderr)
	}
	if !strings.Contains(stderr, "nothing gained") {
		t.Errorf("a future window counted past work: %s", stderr)
	}

	_, stderr, err = run(t, "--bundle", bundleDir, "audit", "--gained", "--since", "last week")
	if err == nil {
		t.Fatal("an unparsable date was accepted")
	}
	if !strings.Contains(stderr, "YYYY-MM-DD") {
		t.Errorf("the refusal does not name the form: %s", stderr)
	}
}

// TestSinceAppliesOnlyToGained keeps a flag from being silently ignored on a report it
// does not bound.
func TestSinceAppliesOnlyToGained(t *testing.T) {
	t.Parallel()
	bundleDir, _ := waiting(t)

	_, stderr, err := run(t, "--bundle", bundleDir,
		"audit", "--churn", "--since", "2026-01-01")
	if err == nil {
		t.Fatal("--since was accepted on --churn")
	}
	if !strings.Contains(stderr, "bounds --gained") {
		t.Errorf("the refusal does not say what --since is for: %s", stderr)
	}
}

// TestSubjectsSurfacesTheDriftTriggerThroughTheCommand is the end-to-end shape §5.8.2.1
// names as the detector's trigger, and the only one that says a corpus is quietly
// ambiguous: two groups writing about one key, each reading sources the other did not.
//
// Through the command rather than the fold, because the fold cannot show the thing that
// makes this usable — that a reader gets it from one flag on a corpus with no index,
// which is the state a new corpus is in when the ambiguity first forms.
func TestSubjectsSurfacesTheDriftTriggerThroughTheCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, _, err := run(t, "--bundle", dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	appendFile(t, filepath.Join(dir, "ontology.toml"), "\n[[subjects]]\n"+
		"key = \"retry.max_attempts\"\ndimension = \"count\"\n"+
		"desc = \"attempts before abandoning\"\n"+
		"aliases = [\"retry budget\", \"retry cap\"]\n")

	claim := func(name, surface, evidence string) {
		t.Helper()
		doc := "---\ntype: Rule\ntitle: " + name + "\ngnosis_claims:\n" +
			"  - id: " + name + "1\n    anchor: A sentence about retries.\n" +
			"    subject: " + surface + "\n" +
			"    archive_paths: [" + evidence + "]\n---\n\nA sentence about retries.\n"
		if err := os.WriteFile(
			filepath.Join(dir, "c", name+".md"),
			[]byte(doc),
			0o600,
		); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	claim("eng", "retry budget", "evidence/text/eng.md")
	claim("sup", "retry cap", "evidence/text/sup.md")

	stdout, stderr, err := run(t, "--bundle", dir, "audit", "--subjects")
	// ok whatever it finds: a population looks like coverage and can be raised by
	// declaring subjects nobody uses, so exiting non-zero would make it a target.
	if err != nil {
		t.Fatalf("audit --subjects: %v\n%s", err, stderr)
	}
	for _, want := range []string{
		"retry.max_attempts",
		"2 claims", "2 documents",
		// Both spellings, which is the "cluster of new aliases" signal.
		"retry budget", "retry cap",
		"no shared source",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the report does not carry %q:\n%s", want, stdout)
		}
	}
}

// TestSubjectsSaysNothingYetRatherThanPrintingAnEmptyTable is §17's distinction, which
// this report needs more than most: a blank table on a fresh corpus reads as a clean
// bill of health for a question nobody asked.
func TestSubjectsSaysNothingYetRatherThanPrintingAnEmptyTable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, _, err := run(t, "--bundle", dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	stdout, stderr, err := run(t, "--bundle", dir, "audit", "--subjects")
	if err != nil {
		t.Fatalf("audit --subjects on a fresh bundle: %v\n%s", err, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("a fresh corpus produced rows:\n%s", stdout)
	}
	if !strings.Contains(stderr, "no claim names a subject yet") {
		t.Errorf("the empty case does not say so: %s", stderr)
	}
}

// appendFile adds to an existing file, for a fixture extending what init wrote.
func appendFile(t *testing.T, path, extra string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if _, err := f.WriteString(extra); err != nil {
		t.Fatalf("append %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
}
