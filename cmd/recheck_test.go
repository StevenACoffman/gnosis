package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd/root"
)

// quoted is the passage the fixture's one claim rests on. It is a whole sentence
// because quotecheck refuses anything under six words, so a shorter fixture would
// exercise the refusal rather than the comparison.
const quoted = "The cache is cleared on restart and holds nothing across sessions."

// rechecked is the drift half of a fetch's payload, decoded.
type rechecked struct {
	URI   string `json:"uri"`
	State string `json:"state"`
	// Missing names the passages the new bytes no longer contain.
	Missing []string `json:"missing"`
	// Resting is how many claims quote this version; zero is the state that means
	// there was nothing to check.
	Resting  int `json:"resting"`
	Findings []struct {
		Path     string `json:"path"`
		Message  string `json:"message"`
		Severity string `json:"severity"`
	} `json:"findings"`
}

// recheckBundle fetches one local source, files a document that quotes it, and
// returns the bundle, the source path, and the document's path within the bundle.
//
// The document is written by hand rather than admitted through the relay, because
// what is under test is the join from a claim's `archive_paths` to a recorded
// source — and going through `ingest`/`admit` would make the fixture depend on the
// extraction path to set up a test about the archive.
func recheckBundle(t *testing.T, claims bool) (bundleDir, src, docPath string) {
	t.Helper()

	bundleDir = t.TempDir()
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	src = filepath.Join(t.TempDir(), "cache.md")
	writeSource(t, src, "Vendor documentation.\n\n"+quoted+"\n")

	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl", "fetch", src)
	if err != nil {
		t.Fatalf("fetch: %v\n%s", err, stderr)
	}
	archivePath := firstArchivePath(t, stdout)
	if !claims {
		return bundleDir, src, ""
	}

	docPath = filepath.Join("c", "cache-lifetime.md")
	doc := "---\n" +
		"type: Reference\n" +
		"title: Cache Lifetime\n" +
		"gnosis_id: 01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d\n" +
		"gnosis_claims:\n" +
		"  - id: claim-1\n" +
		"    anchor: " + quoted + "\n" +
		"    gnosis_evidence:\n" +
		"      - " + quoted + "\n" +
		"    archive_paths:\n" +
		"      - " + archivePath + "\n" +
		"---\n" + quoted + "\n"
	if wErr := os.WriteFile(
		filepath.Join(bundleDir, docPath), []byte(doc), 0o600,
	); wErr != nil {
		t.Fatalf("write document: %v", wErr)
	}
	return bundleDir, src, docPath
}

func writeSource(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
}

// firstArchivePath is where the one fetched source's text was archived.
func firstArchivePath(t *testing.T, stdout string) string {
	t.Helper()

	data := decodeData(t, stdout)
	sources, _ := data["sources"].([]any)
	if len(sources) != 1 {
		t.Fatalf("want one fetched source, got %v", sources)
	}
	s, _ := sources[0].(map[string]any)
	path, _ := s["archive_path"].(string)
	if path == "" {
		t.Fatalf("the fixture source was not archived: %v", s)
	}
	return path
}

// recheck runs the re-check and decodes its drift rows.
func recheck(t *testing.T, bundleDir string) (rows []rechecked, err error) {
	t.Helper()

	stdout, _, err := run(t, "--bundle", bundleDir, "--jsonl", "fetch", "--recheck")
	var env struct {
		Status root.Status `json:"status"`
		Data   struct {
			Drift       []rechecked `json:"drift"`
			Unsupported int         `json:"unsupported"`
		} `json:"data"`
	}
	if jErr := json.Unmarshal([]byte(stdout), &env); jErr != nil {
		t.Fatalf("stdout is not one JSON line: %v\n%s", jErr, stdout)
	}
	return env.Data.Drift, err
}

// TestARecheckResolvesUpstreamDriftToThreeStates is what the entry asked for: today
// the cheap maintenance case and the loss of a claim's upstream support both report
// as `stale`, which puts the smallest chore and the most serious evidentiary event
// in one bucket.
//
// It goes through the real binary rather than calling `gnosis.Drift`, because the
// decision was already unit-tested and what is unproven here is the *join* — that a
// claim's `archive_paths` finds the record, that the record's hash is the one the
// comparison uses, and that a re-fetch of the same URI is what supplies the new
// bytes.
func TestARecheckResolvesUpstreamDriftToThreeStates(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		rewrite string
		want    string
	}{
		"nothing moved": {
			rewrite: "Vendor documentation.\n\n" + quoted + "\n",
			want:    "drift-none",
		},
		"the page was reorganised around the passage": {
			// Retitled, re-wrapped, and extended. This is the ordinary case, and
			// the one a hash-only check would report as a regression.
			rewrite: "# Cache lifetime\n\nThe cache is cleared\non restart and " +
				"holds nothing across sessions.\n\nAdded later, nobody quotes it.\n",
			want: "drift-benign",
		},
		"the passage was removed": {
			rewrite: "Vendor documentation.\n\nThe cache now persists across " +
				"restarts and is never cleared.\n",
			want: "drift-unsupported",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bundleDir, src, _ := recheckBundle(t, true)
			writeSource(t, src, tc.rewrite)

			rows, err := recheck(t, bundleDir)
			if len(rows) != 1 {
				t.Fatalf("want one drift row, got %v (err %v)", rows, err)
			}
			if rows[0].State != tc.want {
				t.Errorf("state = %q, want %q", rows[0].State, tc.want)
			}
			// Only the actionable state exits non-zero. A benign re-archive that
			// failed a CI gate would train everybody to pass --no-recheck.
			if (err != nil) != (tc.want == "drift-unsupported") {
				t.Errorf("exit error = %v for state %s", err, tc.want)
			}
		})
	}
}

// TestWithdrawnSupportNamesTheClaimAndThePassage is §14.3.2's "a finding per affected
// claim, naming the passage".
//
// A row saying `drift-unsupported` and nothing else would leave a reader to search
// the corpus for which sentence stopped being true, which is the work the tool exists
// to have already done.
func TestWithdrawnSupportNamesTheClaimAndThePassage(t *testing.T) {
	t.Parallel()

	bundleDir, src, docPath := recheckBundle(t, true)
	writeSource(t, src, "Vendor documentation.\n\nThe cache now persists.\n")

	rows, err := recheck(t, bundleDir)
	if err == nil {
		t.Error("a source that withdrew support exited zero")
	}
	if len(rows) != 1 || len(rows[0].Findings) != 1 {
		t.Fatalf("want one finding on one row, got %v", rows)
	}
	found := rows[0].Findings[0]
	if found.Path != filepath.ToSlash(docPath) {
		t.Errorf("the finding points at %q, want %q", found.Path, docPath)
	}
	if found.Severity != "error" {
		t.Errorf("severity = %q, want error", found.Severity)
	}
	for _, want := range []string{"claim-1", "cache is cleared on restart"} {
		if !strings.Contains(found.Message, want) {
			t.Errorf("the message does not name %q: %s", want, found.Message)
		}
	}
	// And the passage is reported on the row itself, so a caller that reads the
	// drift and not the findings still learns which words went missing.
	if len(rows[0].Missing) != 1 {
		t.Errorf("the row names %v as missing", rows[0].Missing)
	}
}

// TestASourceNoClaimQuotesIsUnchecked is the state that exists so the other two can
// be trusted.
//
// The source moved and nothing in the corpus rests on it, so there is nothing to have
// been found or lost. Reporting that as benign would be a vacuous truth presented as
// a verification, which is the collapse every `Unchecked` zero value in this codebase
// is placed to refuse.
func TestASourceNoClaimQuotesIsUnchecked(t *testing.T) {
	t.Parallel()

	bundleDir, src, _ := recheckBundle(t, false)
	writeSource(t, src, "Rewritten from scratch, and nobody ever quoted it.\n")

	rows, err := recheck(t, bundleDir)
	if err != nil {
		t.Fatalf("recheck: %v", err)
	}
	if len(rows) != 1 || rows[0].State != "drift-unchecked" {
		t.Fatalf("want one unchecked row, got %v", rows)
	}
	// And it says which of the two unchecked reasons it is. "No claim quotes this"
	// and "the passages could not be re-checked" are the same word and different
	// facts, and a reader deciding whether to chase it needs the second one.
	if rows[0].Resting != 0 {
		t.Errorf("resting = %d, want 0", rows[0].Resting)
	}
}

// TestRepeatedRechecksDoNotBuryTheFinding is a defect running the command found and
// the fixtures did not.
//
// A re-check that sees changed bytes archives them, so the next re-check finds a
// second record for that URI whose text no claim cites yet. Those accumulate one per
// run, and the first version of this report listed each of them by URI with no
// version to tell them apart — so a settled corpus grew a page of lines meaning
// "nothing happened" around the one line that mattered.
func TestRepeatedRechecksDoNotBuryTheFinding(t *testing.T) {
	t.Parallel()

	bundleDir, src, _ := recheckBundle(t, true)

	// Two re-checks, each over changed bytes that still carry the passage, so tier 0
	// ends up holding three versions and only the first is quoted.
	for _, text := range []string{
		"# One\n\n" + quoted + "\n",
		"# Two\n\n" + quoted + "\n\nAdded.\n",
	} {
		writeSource(t, src, text)
		if _, _, err := run(t, "--bundle", bundleDir, "fetch", "--recheck"); err != nil {
			t.Fatalf("recheck: %v", err)
		}
	}

	writeSource(t, src, "# Three\n\nThe passage is gone from this revision.\n")
	stdout, stderr, err := run(t, "--bundle", bundleDir, "fetch", "--recheck")
	if err == nil {
		t.Error("a source that withdrew support exited zero")
	}
	if got := strings.Count(stdout, "drift-"); got != 1 {
		t.Errorf("the report lists %d states, want 1:\n%s", got, stdout)
	}
	if !strings.Contains(stdout, "no claim quotes") {
		t.Errorf("the unquoted versions are not accounted for:\n%s", stdout)
	}
	if !strings.Contains(stderr, "claim-1") {
		t.Errorf("the finding is not reported: %s", stderr)
	}
}

// TestARecheckRefusesToRunOverNothing keeps the flag from succeeding vacuously.
//
// Both cases are a person's mistake rather than a corpus state, and each exits 2 with
// a sentence naming what to do instead — a run that reported "0 sources re-checked,
// ok" would answer a question the caller did not ask.
func TestARecheckRefusesToRunOverNothing(t *testing.T) {
	t.Parallel()

	t.Run("a bundle that has fetched nothing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if _, _, err := run(t, "--bundle", dir, "init"); err != nil {
			t.Fatalf("init: %v", err)
		}
		_, stderr, err := run(t, "--bundle", dir, "fetch", "--recheck")
		if err == nil {
			t.Error("--recheck over an empty archive succeeded")
		}
		if !strings.Contains(stderr, "no fetched sources") {
			t.Errorf("stderr does not say why: %s", stderr)
		}
	})

	t.Run("a source this bundle never fetched", func(t *testing.T) {
		t.Parallel()
		bundleDir, _, _ := recheckBundle(t, true)
		_, stderr, err := run(t,
			"--bundle", bundleDir, "fetch", "--recheck", "/nowhere/absent.md")
		if err == nil {
			t.Error("--recheck of an unfetched source succeeded")
		}
		if !strings.Contains(stderr, "never fetched") {
			t.Errorf("stderr does not say why: %s", stderr)
		}
	})
}

// TestShowReportsWithdrawnSupport is the second half of the drift work, and the gap
// it closes is one this session filed against itself: the finding was printed once by
// `fetch --recheck` and stored nowhere, so a reader running `show` a week later saw
// `fresh` and was told nothing about the source having moved out from under the claim.
//
// Both signals, side by side, is the point. §14.3.2 keeps them apart because they
// answer different questions — freshness is "when was this checked", drift is "does
// upstream still say it" — and the pair worth showing is exactly the one that used to
// be impossible to see: checked recently, and no longer supported.
func TestShowReportsWithdrawnSupport(t *testing.T) {
	t.Parallel()

	bundleDir, src, _ := recheckBundle(t, true)
	if _, _, err := run(t, "--bundle", bundleDir, "index", "rebuild"); err != nil {
		t.Fatalf("index rebuild: %v", err)
	}
	writeSource(t, src, "Vendor documentation.\n\nThe cache now persists.\n")
	if _, err := recheck(t, bundleDir); err == nil {
		t.Fatal("the re-check did not report withdrawn support")
	}

	stdout, stderr, err := run(t, "--bundle", bundleDir,
		"show", "01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d")
	if err != nil {
		t.Fatalf("show: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, "drift-unsupported") {
		t.Errorf("show does not report that support was withdrawn:\n%s", stdout)
	}
	// And it is still reported as freshly checked, because it was. The two lines
	// together are the honest answer; either alone is a half-truth.
	if !strings.Contains(stdout, "fresh") {
		t.Errorf("show no longer reports the freshness:\n%s", stdout)
	}
}
