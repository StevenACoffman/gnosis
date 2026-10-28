package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExportOKFCarriesTheCorpusAndNotTheCache. The same property `proof create` has, and
// asserted separately because the two commands would fail it independently: an export
// that shipped `.gnosis/` hands a colleague's prompt cache and audit trail to whoever the
// bundle was shared with, and nothing in the transfer would report that it happened.
func TestExportOKFCarriesTheCorpusAndNotTheCache(t *testing.T) {
	t.Parallel()

	bundleDir := corpus(t)
	writePrivate(t, bundleDir)

	out := filepath.Join(t.TempDir(), "shared")
	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl",
		"export", "--format", "okf", "--out", out)
	if err != nil {
		t.Fatalf("export: %v\n%s", err, stderr)
	}
	if got := decodeData(t, stdout)["documents"]; got != float64(2) {
		t.Errorf("documents = %v, want 2", got)
	}
	if _, sErr := os.Stat(filepath.Join(out, ".gnosis")); !os.IsNotExist(sErr) {
		t.Error("the export carries .gnosis/, which is per-user state")
	}
	if _, sErr := os.Stat(filepath.Join(out, "ontology.toml")); sErr != nil {
		t.Errorf("the export is missing the vocabulary the concepts are typed against: %v", sErr)
	}

	// The receiver must be able to run gnosis against it, which is what "portable"
	// means. Linting the export is the cheapest end-to-end statement of that.
	if _, lintErr, lErr := run(t, "--bundle", out, "lint"); lErr != nil &&
		!strings.Contains(lintErr, "finding") {
		t.Errorf("the exported bundle does not lint as a bundle: %v\n%s", lErr, lintErr)
	}
}

// TestExportJSONLCarriesTheAddressesThatMakeAClaimCheckable. A stream of assertions with
// no archive paths hands a receiver a set of things to believe, which is the shape §1.1
// exists to reject.
func TestExportJSONLCarriesTheAddressesThatMakeAClaimCheckable(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := run(t, "--bundle", criticBundle(t), "export", "--format", "jsonl")
	if err != nil {
		t.Fatalf("export: %v\n%s", err, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	var found bool
	for _, line := range lines {
		var row struct {
			ID     string `json:"gnosis_id"`
			Claims []struct {
				ID           string   `json:"id"`
				ArchivePaths []string `json:"archive_paths"`
			} `json:"claims"`
		}
		if uErr := json.Unmarshal([]byte(line), &row); uErr != nil {
			t.Fatalf("a row is not JSON: %v\n%s", uErr, line)
		}
		if row.ID == "" {
			t.Errorf("a row carries no identifier: %s", line)
		}
		for _, claim := range row.Claims {
			if len(claim.ArchivePaths) > 0 {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no exported claim names where its evidence lives:\n%s", stdout)
	}
}

// TestExportRefusesAFormatItCannotWrite. An unknown format naming only itself would make
// the caller go and read the source; naming both alternatives is the whole content of a
// usage message.
func TestExportRefusesAFormatItCannotWrite(t *testing.T) {
	t.Parallel()

	bundleDir := corpus(t)
	for name, args := range map[string][]string{
		"a format nobody has": {"export", "--format", "yaml"},
		"no format at all":    {"export"},
		"okf with no out":     {"export", "--format", "okf"},
		"jsonl with an out":   {"export", "--format", "jsonl", "--out", "somewhere"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, stderr, err := run(t, append([]string{"--bundle", bundleDir}, args...)...)
			if err == nil {
				t.Fatal("the export ran on an invocation it cannot serve")
			}
			if !strings.Contains(stderr, "okf") && !strings.Contains(stderr, "jsonl") {
				t.Errorf("the refusal names neither format: %s", stderr)
			}
		})
	}
}

// writePrivate puts a per-user file where an export must not find it.
func writePrivate(t *testing.T, bundleDir string) {
	t.Helper()

	path := filepath.Join(bundleDir, ".gnosis", "audit.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write trail: %v", err)
	}
}
