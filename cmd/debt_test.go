package cmd_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// promoted drives the whole cycle to a document in the corpus, carried by a person
// over the signals this build cannot run. It is the only way to produce debt, which
// is the point: there is no `--force`.
func promoted(t *testing.T) (bundleDir, path string) {
	t.Helper()
	bundleDir, path = waiting(t)

	_, stderr, err := runWithStdin(t, path+"\n", "--bundle", bundleDir,
		"promote", "--apply", "--approver", "human:priya",
		"--rationale", "read the source myself", path)
	if err != nil {
		t.Fatalf("promote: %v\n%s", err, stderr)
	}
	return bundleDir, path
}

// TestDebtNamesWhatWasCarried. `audit.Row.Signals` was written and read by nothing
// for a whole phase, which made the field a promise rather than a mechanism — the
// trap §6.5.1 is about, one layer up. This is the reader.
func TestDebtNamesWhatWasCarried(t *testing.T) {
	t.Parallel()
	bundleDir, path := promoted(t)

	stdout, stderr, err := run(t, "--bundle", bundleDir, "debt")
	if err != nil {
		t.Fatalf("debt: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, path) {
		t.Errorf("the report does not name the document:\n%s", stdout)
	}
	if !strings.Contains(stdout, "conflict") {
		t.Errorf("the report does not name the unrun signal:\n%s", stdout)
	}
	if !strings.Contains(stdout, "human:priya") {
		t.Errorf("the report does not name who carried it:\n%s", stdout)
	}
}

// TestDebtKeepsTheSummaryOffStdout. Found by running the command: the per-signal
// counts were printed above the entries, and `conflict\t1` reads as the same shape
// as `conflict\tc/a.md\thuman:priya` — so a reader could not tell a total from a
// row, and neither could `cut`. Data on stdout, summary on stderr, as everywhere
// else here.
func TestDebtKeepsTheSummaryOffStdout(t *testing.T) {
	t.Parallel()
	bundleDir, path := promoted(t)

	stdout, stderr, err := run(t, "--bundle", bundleDir, "debt")
	if err != nil {
		t.Fatalf("debt: %v\n%s", err, stderr)
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line == "" {
			continue
		}
		if n := len(strings.Split(line, "\t")); n != 3 {
			t.Errorf("stdout line has %d fields, want signal/path/actor: %q", n, line)
		}
		if !strings.Contains(line, path) {
			t.Errorf("a stdout line names no document: %q", line)
		}
	}
	if !strings.Contains(stderr, "conflict: 1") {
		t.Errorf("the per-signal summary is not on stderr:\n%s", stderr)
	}
}

// TestDebtOnACleanCorpusIsOK, not a finding. A corpus carrying documents over unrun
// checks is the expected state of this build, and a corpus that has promoted nothing
// is the expected state of a fresh one. Exiting non-zero on either would teach a
// reader to ignore the command.
func TestDebtOnACleanCorpusIsOK(t *testing.T) {
	t.Parallel()
	bundleDir, _ := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	stdout, stderr, err := run(t, "--bundle", bundleDir, "debt")
	if err != nil {
		t.Fatalf("debt on an unwritten corpus failed: %v\n%s", err, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("an empty register printed rows:\n%s", stdout)
	}
}

// TestDebtEnvelopeCarriesTheDenominator. §17 forbids a count presented as health,
// and a carried count with no denominator is exactly that: 34 means something
// different against 40 promotions than against 4000.
func TestDebtEnvelopeCarriesTheDenominator(t *testing.T) {
	t.Parallel()
	bundleDir, path := promoted(t)

	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl", "debt")
	if err != nil {
		t.Fatalf("debt: %v\n%s", err, stderr)
	}
	var env struct {
		Status string `json:"status"`
		Data   struct {
			Promotions int            `json:"promotions"`
			BySignal   map[string]int `json:"by_signal"`
			Carried    []struct {
				Path      string `json:"path"`
				Signal    string `json:"signal"`
				Actor     string `json:"actor"`
				Rationale string `json:"rationale"`
			} `json:"carried"`
		} `json:"data"`
	}
	if err = json.Unmarshal([]byte(firstLine(stdout)), &env); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if env.Status != "ok" {
		t.Errorf("status = %q, want ok", env.Status)
	}
	if env.Data.Promotions != 1 {
		t.Errorf("promotions = %d, want 1", env.Data.Promotions)
	}
	if env.Data.BySignal["conflict"] != 1 {
		t.Errorf("by_signal = %v, want conflict counted", env.Data.BySignal)
	}
	if len(env.Data.Carried) == 0 {
		t.Fatal("no carried entries")
	}
	got := env.Data.Carried[0]
	if got.Path != path || got.Actor != "human:priya" {
		t.Errorf("carried = %+v", got)
	}
	// The rationale is what distinguishes this from a bypass. A register that
	// recorded who signed but not why would answer half the question.
	if !strings.Contains(got.Rationale, "read the source myself") {
		t.Errorf("the entry does not carry the rationale: %q", got.Rationale)
	}
}

// TestDebtSampleIsReproducibleAndSaysSo. §6.2.1 requires the specific draw to be
// inspectable, so the seed is reported with the sample — a draw nobody can repeat is
// not a measurement, and one whose seed is hidden is reproducible in principle only.
func TestDebtSampleIsReproducibleAndSaysSo(t *testing.T) {
	t.Parallel()
	bundleDir, _ := promoted(t)

	first, stderr, err := run(t, "--bundle", bundleDir, "--jsonl", "debt", "--sample", "1")
	if err != nil {
		t.Fatalf("debt --sample: %v\n%s", err, stderr)
	}
	second, _, err := run(t, "--bundle", bundleDir, "--jsonl", "debt", "--sample", "1")
	if err != nil {
		t.Fatalf("debt --sample: %v", err)
	}
	if firstLine(first) != firstLine(second) {
		t.Errorf("two draws at one seed differ:\n%s\n%s", first, second)
	}

	var env struct {
		Data struct {
			Sampled    int    `json:"sampled"`
			Seed       uint64 `json:"seed"`
			Promotions int    `json:"promotions"`
		} `json:"data"`
	}
	if err = json.Unmarshal([]byte(firstLine(first)), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Sampled != 1 {
		t.Errorf("sampled = %d, want 1", env.Data.Sampled)
	}
	if env.Data.Seed == 0 {
		t.Error("the sample does not report its seed; the draw cannot be repeated")
	}
	// The denominator is the trail's, not the sample's. A sample that shrank its
	// own denominator would report a rate of one.
	if env.Data.Promotions != 1 {
		t.Errorf("promotions = %d, want the trail's count", env.Data.Promotions)
	}
}

// TestDebtTakesNoArguments, so a mistyped flag is a usage error rather than a
// silently ignored word.
func TestDebtTakesNoArguments(t *testing.T) {
	t.Parallel()
	bundleDir, _ := promoted(t)

	if _, _, err := run(t, "--bundle", bundleDir, "debt", "conflict"); err == nil {
		t.Error("a positional argument was accepted")
	}
}

// firstLine is the envelope, for a command that also writes to stderr.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
