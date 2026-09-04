package bundle_test

import (
	"slices"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// TestMissReportCountsQuestionsAndEmissions is the defect running the binary found.
//
// Two `ingest` runs over one unanswered source write the same prompt twice — the reply
// is not cached, so nothing is skipped — and the log then holds two rows for one
// question. Both numbers are facts and they answer different questions: how often a
// model was consulted, and how often a prompt reached disk.
func TestMissReportCountsQuestionsAndEmissions(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	groups := bundle.MissReport([]bundle.Miss{
		{Op: "extract", Reason: gnosis.MissNoPath, Key: "k1", At: at},
		{Op: "extract", Reason: gnosis.MissNoPath, Key: "k1", At: at.Add(time.Hour)},
		{Op: "accrete", Reason: gnosis.MissNoPath, Key: "k2", At: at},
		{
			Op: "critique", Reason: gnosis.MissNoPredicate, Key: "k3", At: at,
			ChecksRun: []string{"coverage", "rung"},
		},
	})

	if len(groups) != 2 {
		t.Fatalf("want two groups, got %+v", groups)
	}
	// The actionable group is first, whatever the counts: sorting by count alone puts
	// the line nobody can act on at the top of every run.
	if !groups[0].Actionable || groups[0].Reason != gnosis.MissNoPredicate {
		t.Errorf("the actionable group is not first: %+v", groups)
	}
	if groups[0].Count != 1 || groups[0].Emissions != 1 {
		t.Errorf("critique group = %d questions, %d emissions",
			groups[0].Count, groups[0].Emissions)
	}
	if groups[1].Count != 2 || groups[1].Emissions != 3 {
		t.Errorf("extraction group = %d questions, %d emissions; want 2 and 3 — one "+
			"prompt was written twice", groups[1].Count, groups[1].Emissions)
	}
	if !slices.Equal(groups[1].Ops, []string{"accrete", "extract"}) {
		t.Errorf("ops = %q, want both operations sorted", groups[1].Ops)
	}
	if !slices.Equal(groups[0].ChecksRun, []string{"coverage", "rung"}) {
		t.Errorf("checksRun = %q", groups[0].ChecksRun)
	}
}

// TestMissReportKeepsARowItCannotPair. A row written before `Key` existed counts as its
// own question: dropping it would undercount a consultation that happened, and merging
// every such row into one would undercount worse.
func TestMissReportKeepsARowItCannotPair(t *testing.T) {
	t.Parallel()

	groups := bundle.MissReport([]bundle.Miss{
		{Op: "extract", Reason: gnosis.MissNoPath},
		{Op: "extract", Reason: gnosis.MissNoPath},
	})
	if len(groups) != 1 || groups[0].Count != 2 {
		t.Errorf("two keyless rows counted as %+v, want two questions", groups)
	}
}

// TestMissReportKeepsAnUnrecognisedReasonApart. A row from a later gnosis is evidence
// about the corpus, and a silently merged count is a wrong answer.
func TestMissReportKeepsAnUnrecognisedReasonApart(t *testing.T) {
	t.Parallel()

	groups := bundle.MissReport([]bundle.Miss{
		{Op: "extract", Reason: gnosis.MissNoPath, Key: "k1"},
		{Op: "mine", Reason: gnosis.MissReason("no_deterministic_miner"), Key: "k2"},
	})
	if len(groups) != 2 {
		t.Fatalf("an unrecognised reason was folded: %+v", groups)
	}
	for i := range groups {
		if groups[i].Reason == "no_deterministic_miner" && groups[i].Actionable {
			t.Error("a reason this build does not know was reported as actionable")
		}
	}
}

// TestRecordMissRefusesARowWithNoReason. The report's whole output is a count grouped by
// reason, so a row naming none would swell whichever group it defaulted to.
func TestRecordMissRefusesARowWithNoReason(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	withWriter(t, dir, func(w *bundle.Writer) {
		err := w.RecordMiss(&bundle.Miss{Op: "extract", Key: "k1"})
		if errs.ErrorCode(err) != errs.EINVALID {
			t.Errorf("a miss with no reason was recorded: %v", err)
		}
	})
}

// TestMissesRoundTripThroughTheLedger, appended and in order: the file is a log of
// events and the report is the interpretation of it.
func TestMissesRoundTripThroughTheLedger(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	at := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	withWriter(t, dir, func(w *bundle.Writer) {
		for _, key := range []string{"k1", "k2"} {
			if err := w.RecordMiss(&bundle.Miss{
				Op: "critique", Reason: gnosis.MissNoPredicate, Key: key,
				Candidate: "c/a.md#c1", At: at,
			}); err != nil {
				t.Fatalf("RecordMiss: %v", err)
			}
		}
	})

	got, err := bundle.LoadMisses(dir)
	if err != nil {
		t.Fatalf("LoadMisses: %v", err)
	}
	if len(got) != 2 || got[0].Key != "k1" || got[1].Key != "k2" {
		t.Fatalf("rows are not both there in order: %+v", got)
	}
	if got[0].Reason != gnosis.MissNoPredicate {
		t.Errorf("the reason did not survive the round trip: %v", got[0].Reason)
	}
}

// TestLoadMissesOnAFreshBundle. A corpus that has asked nothing is the ordinary state of
// a first run, not an error.
func TestLoadMissesOnAFreshBundle(t *testing.T) {
	t.Parallel()

	got, err := bundle.LoadMisses(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Errorf("LoadMisses on a fresh bundle = %v, %v", got, err)
	}
}
