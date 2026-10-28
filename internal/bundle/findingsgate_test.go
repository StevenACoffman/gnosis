package bundle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// findingsFile writes a findings file and returns its path.
func findingsFile(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "findings.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write findings: %v", err)
	}
	return path
}

// TestGateBlocksOnErrorAndNotOnWarning is the severity model §16.1 shares across the
// family: only error blocks, and a warning that blocked would make every advisory
// finding a build failure.
func TestGateBlocksOnErrorAndNotOnWarning(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gated, err := bundle.GateFindings(dir, findingsFile(t, `{"diagnostics":[
		{"severity":"warning","category":"orphan","path":"c/a.md","message":"no links"},
		{"severity":"error","category":"conformance","path":"c/b.md","message":"no type"},
		{"severity":"warning","category":"lead","path":"c/c.md","message":"buried"}
	]}`))
	if err != nil {
		t.Fatalf("GateFindings: %v", err)
	}
	if len(gated.Blocking) != 1 || gated.Warnings != 2 {
		t.Fatalf("got %d blocking and %d warnings, want 1 and 2",
			len(gated.Blocking), gated.Warnings)
	}
	if !gated.Blocks() {
		t.Error("an error-severity finding did not block")
	}
	if !gated.SelfTested {
		t.Error("the self-test did not run")
	}
	if !strings.Contains(bundle.GateReason(gated), "1 blocking finding") {
		t.Errorf("the reason miscounts or mis-pluralises: %q", bundle.GateReason(gated))
	}
}

// TestGateReadsBothShapesTheFamilyProduces, because a gate that could not read the tool
// it ships with would be a gate nobody uses, and one that could not read the family's
// wire format would not be interoperable.
func TestGateReadsBothShapesTheFamilyProduces(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"a bare finding.Result": `{"diagnostics":[
			{"severity":"error","category":"conformance","path":"c/a.md","message":"x"}]}`,
		"a gnosis envelope": `{"status":"findings","code":3,"data":{"diagnostics":[
			{"severity":"error","category":"conformance","path":"c/a.md","message":"x"}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gated, err := bundle.GateFindings(t.TempDir(), findingsFile(t, body))
			if err != nil {
				t.Fatalf("GateFindings: %v", err)
			}
			if len(gated.Blocking) != 1 {
				t.Errorf("got %d blocking findings, want 1", len(gated.Blocking))
			}
		})
	}
}

// TestGateRefusesAShapeItDoesNotKnow, naming both it accepts. A gate that inferred
// structure would be deciding what to block on from a shape it did not recognise, and a
// caller handed the wrong wrapper needs to be told which two are right.
func TestGateRefusesAShapeItDoesNotKnow(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"an array":      `[{"severity":"error","message":"x"}]`,
		"a bare object": `{"findings":[]}`,
		"not json":      `severity: error`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := bundle.GateFindings(t.TempDir(), findingsFile(t, body))
			if errs.ErrorCode(err) != errs.EINVALID {
				t.Fatalf("an unknown shape was accepted: %v", err)
			}
			for _, want := range []string{"finding.Result", "data.diagnostics"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal does not name %q: %v", want, err)
				}
			}
		})
	}
}

// TestGateReportsWhichActRan is §17.1's requirement at the seam that answers it: a
// corpus with no critique reports structural-only, and the same findings against a
// corpus whose ledger holds one report semantic-clean.
func TestGateReportsWhichActRan(t *testing.T) {
	t.Parallel()

	const clean = `{"diagnostics":[
		{"severity":"warning","category":"orphan","path":"c/a.md","message":"no links"}]}`
	file := findingsFile(t, clean)

	fresh := t.TempDir()
	gated, err := bundle.GateFindings(fresh, file)
	if err != nil {
		t.Fatalf("GateFindings: %v", err)
	}
	if gated.SemanticReview != gnosis.SemanticStructuralOnly {
		t.Errorf("a corpus with no critique reports %v", gated.SemanticReview)
	}
	if gated.Blocks() {
		t.Error("a warning blocked")
	}

	critiqued := t.TempDir()
	withWriter(t, critiqued, func(w *bundle.Writer) {
		if err := w.RecordCritique(&bundle.Critique{
			ClaimRef: "c/a.md#c1", Key: "k1", Examined: []string{"the scope"},
		}); err != nil {
			t.Fatalf("RecordCritique: %v", err)
		}
	})
	gated, err = bundle.GateFindings(critiqued, file)
	if err != nil {
		t.Fatalf("GateFindings: %v", err)
	}
	if gated.SemanticReview != gnosis.SemanticClean {
		t.Errorf("a corpus with a critique reports %v — the same findings, and the "+
			"difference is whether anybody looked", gated.SemanticReview)
	}
}

// TestGateCarriesUnexaminedThrough. A gate that dropped them would ship on exactly the
// silence they exist to break.
func TestGateCarriesUnexaminedThrough(t *testing.T) {
	t.Parallel()

	gated, err := bundle.GateFindings(t.TempDir(), findingsFile(t, `{"diagnostics":[],
		"unexamined":[{"aspect":"the source methodology",
		"reason":"the excerpt does not include it"}]}`))
	if err != nil {
		t.Fatalf("GateFindings: %v", err)
	}
	if len(gated.Unexamined) != 1 ||
		gated.Unexamined[0].Aspect != "the source methodology" {
		t.Errorf("the unexamined aspects did not survive: %+v", gated.Unexamined)
	}
}

// TestAGateThatCannotProveItRefusesBlocks is the fail-closed half, and it is the one
// clause a reader would not expect: a pass from a gate whose classifier is broken is a
// green light of unknown provenance, so the absence of a self-test blocks by itself.
func TestAGateThatCannotProveItRefusesBlocks(t *testing.T) {
	t.Parallel()

	// Constructed rather than produced, because a gate whose self-test fails cannot
	// be obtained from GateFindings — which is the point: the state is unreachable
	// while the classifier works, and this pins what happens if it stops.
	broken := &bundle.Gated{SelfTested: false}
	if !broken.Blocks() {
		t.Error("a gate that could not demonstrate it still refuses did not block")
	}
	if !strings.Contains(bundle.GateReason(broken), "defect in gnosis") {
		t.Errorf("the reason does not say whose problem it is: %q",
			bundle.GateReason(broken))
	}
	passing := &bundle.Gated{SelfTested: true}
	if passing.Blocks() || bundle.GateReason(passing) != "" {
		t.Error("a clean verdict blocked or carried a reason")
	}
}
