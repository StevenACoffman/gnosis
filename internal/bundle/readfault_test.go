package bundle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// TestAReadFaultNamesItsOperationOnce. A document that cannot be read, rather than one
// that is missing, is a tool failure, so it comes back as an error. readConcept already
// names the caller's operation on that error, and each of these paths once handed it to
// a refusal helper that named the operation again:
// "bundle.Coordinator.supersede: bundle.Coordinator.supersede: read …".
//
// A directory where the document should be is the fault: os.ReadFile fails on it with
// an error that is not fs.ErrNotExist, on every platform, without touching permissions.
func TestAReadFaultNamesItsOperationOnce(t *testing.T) {
	t.Parallel()

	concept, err := gnosis.ParseID("01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d")
	if err != nil {
		t.Fatalf("ParseID: %v", err)
	}
	for _, tc := range []struct {
		name, op string
		cmd      command.Command
	}{
		{
			name: "defer",
			op:   "bundle.Coordinator.deferConflict",
			cmd: &command.Defer{
				Path: "c/broken.md", Concept: concept, Finding: "f1",
				By: "human:priya", Reason: "waiting on the vendor", Eff: command.EffectPreview,
			},
		},
		{
			name: "adjudicate",
			op:   "bundle.Coordinator.adjudicate",
			cmd: &command.Adjudicate{
				Path: "c/broken.md", ClaimID: "c1", By: "human:priya",
				Rationale: "the benchmark settles it", Eff: command.EffectPreview,
			},
		},
		{
			name: "supersede, the loser",
			op:   "bundle.Coordinator.supersede",
			cmd: &command.Supersede{
				LoserPath: "c/broken.md", LoserClaim: "c1",
				WinnerPath: "c/a.md", WinnerClaim: "c1",
				By: "human:priya", Eff: command.EffectPreview,
			},
		},
		{
			name: "supersede, the winner",
			op:   "bundle.Coordinator.supersede",
			cmd: &command.Supersede{
				LoserPath: "c/a.md", LoserClaim: "c2",
				WinnerPath: "c/broken.md", WinnerClaim: "c1",
				By: "human:priya", Eff: command.EffectPreview,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "c", "broken.md"), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(dir, "c", "a.md"), []byte(superseded), 0o600,
			); err != nil {
				t.Fatal(err)
			}

			c := bundle.Coordinator{Dir: dir}
			got, err := c.Execute(t.Context(), tc.cmd)
			if err == nil {
				t.Fatalf("a read fault was not an error: %+v", got)
			}
			if n := strings.Count(err.Error(), tc.op); n != 1 {
				t.Errorf("the error names %s %d times, want once: %v", tc.op, n, err)
			}
		})
	}
}
