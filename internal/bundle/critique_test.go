package bundle_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/skillet/finding"
)

// TestCoveredAnglesSubtractsWhatWasLaterExamined is the fold's whole reason for
// existing: an angle one critic declined and a later one covered is finished, and
// suggesting it again would steer the next critic back onto exhausted ground.
func TestCoveredAnglesSubtractsWhatWasLaterExamined(t *testing.T) {
	t.Parallel()

	critiques := []bundle.Critique{
		{
			ClaimRef: "c/a.md#c1",
			Examined: []string{"whether the quotation supports the scope"},
			NotExamined: []finding.Unexamined{
				{Aspect: "the source's methodology", Reason: "the excerpt stops short"},
				{Aspect: "the sample size", Reason: "the source does not report one"},
			},
		},
		{
			ClaimRef: "c/a.md#c1",
			Examined: []string{"the source's methodology"},
		},
		// A different claim's coverage must not leak into this one's.
		{
			ClaimRef: "c/b.md#c1",
			Examined: []string{"something else entirely"},
			NotExamined: []finding.Unexamined{
				{Aspect: "another claim's gap", Reason: "a different claim entirely"},
			},
		},
	}

	examined, notExamined := bundle.CoveredAngles(critiques, "c/a.md#c1")
	wantExamined := []string{
		"whether the quotation supports the scope", "the source's methodology",
	}
	if !slices.Equal(examined, wantExamined) {
		t.Errorf("examined = %q, want %q", examined, wantExamined)
	}
	if len(notExamined) != 1 || notExamined[0].Aspect != "the sample size" {
		t.Errorf("notExamined = %+v; the methodology was covered by the second"+
			" critique and must not be suggested again", notExamined)
	}
	// The reason travels with the aspect, because it is what the next critic acts on:
	// a gap this excerpt cannot close is not ground to steer anybody toward.
	if len(notExamined) == 1 && notExamined[0].Reason != "the source does not report one" {
		t.Errorf("the gap arrived without the reason that makes it legible: %+v",
			notExamined[0])
	}
}

// TestCoveredAnglesDeduplicatesCaseInsensitively. Two critics writing the same angle
// with different capitalisation have covered one angle, and a prompt listing both
// spends a bullet saying so twice.
func TestCoveredAnglesDeduplicatesCaseInsensitively(t *testing.T) {
	t.Parallel()

	critiques := []bundle.Critique{
		{ClaimRef: "c/a.md#c1", Examined: []string{"The source's methodology"}},
		{ClaimRef: "c/a.md#c1", Examined: []string{"the source's methodology"}},
		{ClaimRef: "c/a.md#c1", Examined: []string{"  the source's methodology  "}},
	}

	examined, _ := bundle.CoveredAngles(critiques, "c/a.md#c1")
	if len(examined) != 1 {
		t.Errorf("examined = %q, want one angle", examined)
	}
	// The first spelling survives, which is chronological order: the lists go into a
	// prompt whose hash is the cache key, so which one wins has to be stable.
	if examined[0] != "The source's methodology" {
		t.Errorf("examined[0] = %q, want the first spelling written", examined[0])
	}
}

// TestCritiquesRoundTripThroughTheLedger, and the ledger appends: the sequence is what
// the next prompt reads, so a writer that collapsed rows would discard the angles an
// earlier critic covered.
func TestCritiquesRoundTripThroughTheLedger(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	at := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	withWriter(t, dir, func(w *bundle.Writer) {
		for _, c := range []bundle.Critique{
			{
				ClaimRef: "c/a.md#c1", Key: "k1", Model: "m", At: at,
				Examined: []string{"the scope"},
				NotExamined: []finding.Unexamined{
					{Aspect: "the method", Reason: "no time in this pass"},
				},
			},
			{
				ClaimRef: "c/a.md#c1", Key: "k2", Model: "m", At: at.Add(time.Hour),
				Examined: []string{"the method"},
			},
		} {
			if err := w.RecordCritique(&c); err != nil {
				t.Fatalf("RecordCritique: %v", err)
			}
		}
	})

	got, err := bundle.LoadCritiques(dir)
	if err != nil {
		t.Fatalf("LoadCritiques: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 — the second critique overwrote the first", len(got))
	}
	if got[0].Key != "k1" || got[1].Key != "k2" {
		t.Errorf("rows are not in the order they were written: %+v", got)
	}
	if _, notExamined := bundle.CoveredAngles(got, "c/a.md#c1"); len(notExamined) != 0 {
		t.Errorf("notExamined = %+v after the gap was covered", notExamined)
	}
}

// TestLoadCritiquesOnAFreshBundle. Nothing critiqued is not an error, and it must not
// be: every critic run starts there.
func TestLoadCritiquesOnAFreshBundle(t *testing.T) {
	t.Parallel()

	got, err := bundle.LoadCritiques(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Errorf("LoadCritiques on a fresh bundle = %v, %v", got, err)
	}
}

// TestALedgerRowWrittenBeforeReasonsStillSteers is the compatibility the tolerant
// decoder buys, and the reason it is worth its weight.
//
// A row whose `not_examined` holds bare strings is still the record of what a critic
// declined, and this file exists to be matched against later. Failing the load would
// take the whole history down over a field nobody had time to depend on.
func TestALedgerRowWrittenBeforeReasonsStillSteers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	state := filepath.Join(dir, ".gnosis")
	if err := os.MkdirAll(state, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	row := `{"claim_ref":"c/a.md#c1","key":"k1","model":"m",` +
		`"examined":["the scope"],"not_examined":["the method"]}` + "\n"
	if err := os.WriteFile(filepath.Join(state, "coverage.jsonl"),
		[]byte(row), 0o600); err != nil {
		t.Fatalf("write ledger: %v", err)
	}

	got, err := bundle.LoadCritiques(dir)
	if err != nil {
		t.Fatalf("LoadCritiques: %v", err)
	}
	if len(got) != 1 || len(got[0].NotExamined) != 1 {
		t.Fatalf("the old row did not survive the read: %+v", got)
	}
	gap := got[0].NotExamined[0]
	if gap.Aspect != "the method" {
		t.Errorf("aspect = %q, want the string the old row held", gap.Aspect)
	}
	// The filled-in reason says what it is rather than inventing one. A row claiming
	// "the excerpt does not include it" would put words in an earlier critic's mouth.
	if !gap.Valid() || !strings.Contains(gap.Reason, "before critiques carried a reason") {
		t.Errorf("reason = %q, want one that says where it came from", gap.Reason)
	}
}
