package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// TestDiscardIsAPreviewByDefault. Tier 1 is not committed, so there is no `git
// checkout` after a mistake — the field whose zero value writes would be the one
// that loses somebody's draft.
func TestDiscardIsAPreviewByDefault(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	stdout, stderr, err := run(t, "--bundle", bundleDir,
		"quarantine", "--discard", path, "--by", "human:priya", "--reason", "wrong source")
	if err != nil {
		t.Fatalf("discard preview: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, "would discard") {
		t.Errorf("the preview does not say it is one:\n%s", stdout)
	}
	if _, rErr := bundle.ReadQuarantined(bundleDir, path); rErr != nil {
		t.Errorf("a preview removed the draft: %v", rErr)
	}
}

// TestDiscardWithApplyDropsTheDraft, which is the route a refused candidate now
// has: fix the input, re-admit, drop the old draft.
func TestDiscardWithApplyDropsTheDraft(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	stdout, stderr, err := run(t, "--bundle", bundleDir, "quarantine",
		"--discard", path, "--by", "human:priya", "--reason", "the source was wrong", "--apply")
	if err != nil {
		t.Fatalf("discard: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, "discarded") {
		t.Errorf("stdout does not report the discard:\n%s", stdout)
	}
	if _, rErr := bundle.ReadQuarantined(bundleDir, path); rErr == nil {
		t.Error("the draft survived --apply")
	}

	waitingNow, err := bundle.Quarantined(bundleDir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(waitingNow) != 0 {
		t.Errorf("the queue still holds %v", waitingNow)
	}
}

// TestADiscardIsAudited. §15 audits every mutation, and this one leaves nothing
// else behind: a promotion at least leaves the document as evidence of itself,
// where a discard leaves an empty directory. The row is the only record there will
// ever be.
func TestADiscardIsAudited(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	const why = "the archived source was the wrong revision"
	if _, stderr, err := run(t, "--bundle", bundleDir, "quarantine",
		"--discard", path, "--by", "human:priya", "--reason", why, "--apply"); err != nil {
		t.Fatalf("discard: %v\n%s", err, stderr)
	}

	trail, err := bundle.AuditTrail(bundleDir)
	if err != nil {
		t.Fatalf("read the trail: %v", err)
	}
	var found bool
	for i := range trail.Rows {
		row := &trail.Rows[i]
		if row.Op != "discard" {
			continue
		}
		found = true
		if row.Actor != "human:priya" {
			t.Errorf("actor = %q", row.Actor)
		}
		if !strings.Contains(row.Detail, why) {
			t.Errorf("the row does not carry the reason: %q", row.Detail)
		}
		// The hash of what was dropped is the one piece of evidence that survives.
		if row.HashBefore == "" {
			t.Error("the row does not record what was dropped")
		}
		if row.HashAfter != "" {
			t.Errorf("a discard recorded a hash after: %q", row.HashAfter)
		}
	}
	if !found {
		t.Error("no discard row in the trail")
	}
}

// TestADiscardNeedsAnActorAndAReason. Neither is defaulted: a discard whose actor
// gnosis supplied would be a trail row nobody can be asked about, and a trail of
// discards with no account of what was wrong cannot say whether the drafts were junk
// or somebody was clearing their queue.
func TestADiscardNeedsAnActorAndAReason(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"no actor":       {"--reason", "junk"},
		"no reason":      {"--by", "human:priya"},
		"unparsed actor": {"--by", "priya", "--reason", "junk"},
		"empty reason":   {"--by", "human:priya", "--reason", "  "},
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bundleDir, path := waiting(t)

			args := append([]string{
				"--bundle", bundleDir, "quarantine",
				"--discard", path, "--apply",
			}, extra...)
			if _, _, err := run(t, args...); err == nil {
				t.Error("the discard was accepted")
			}
			if _, rErr := bundle.ReadQuarantined(bundleDir, path); rErr != nil {
				t.Errorf("a refused discard removed the draft: %v", rErr)
			}
		})
	}
}

// TestDiscardingNothingIsNotASuccess. A mistyped path must not read as a completed
// cleanup, or the real draft stays in the queue while its owner believes it is gone.
func TestDiscardingNothingIsNotASuccess(t *testing.T) {
	t.Parallel()
	bundleDir, _ := waiting(t)

	stdout, _, err := run(t, "--bundle", bundleDir, "--jsonl", "quarantine",
		"--discard", "c/never-existed.md", "--by", "human:priya",
		"--reason", "typo", "--apply")
	if err == nil {
		t.Error("discarding an absent draft reported success")
	}
	var env struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if uErr := json.Unmarshal([]byte(firstLine(stdout)), &env); uErr != nil {
		t.Fatalf("decode %q: %v", stdout, uErr)
	}
	if env.Status == "ok" {
		t.Errorf("status = %q for a path that holds nothing", env.Status)
	}
}

// TestDiscardRefusesTraversal. A quarantined path arrives from a model's reply, so
// `../../etc/whatever` is an input this will actually receive — and a discard is the
// one operation whose whole job is to remove a file.
func TestDiscardRefusesTraversal(t *testing.T) {
	t.Parallel()
	bundleDir, _ := waiting(t)

	for _, bad := range []string{"../escaped.md", "../../etc/passwd", "/absolute.md"} {
		if _, _, err := run(t, "--bundle", bundleDir, "quarantine",
			"--discard", bad, "--by", "human:priya", "--reason", "probe", "--apply",
		); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

// TestQuarantineHelpRefusesEditing. The decision is recorded where the author
// reading `refused` is standing, because that is the moment they go looking for a
// way to fix the file by hand.
func TestQuarantineHelpRefusesEditing(t *testing.T) {
	t.Parallel()

	_, stderr, _ := run(t, "quarantine", "-h")
	for _, want := range []string{"no --edit", "--discard"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the help does not mention %q:\n%s", want, stderr)
		}
	}
}

// TestAPersonsDeclineReachesTheCommittedTier is the resolution of §10.7.4's question:
// a decision to decline is a decision, and the per-user trail is gitignored.
//
// Three events wear the word "declined" and only this one belongs in the corpus's own
// history. The gate's refusal is recomputable — run the gate again. A person who was
// asked and walked away decided nothing, and `audit --outstanding` is what surfaces
// that. A person who looked at the draft and dropped it decided something no
// re-computation can recover, because the reason existed only in their head until they
// typed it.
func TestAPersonsDeclineReachesTheCommittedTier(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	const why = "the vendor page had been replaced; the quotation is from the old one"
	if _, stderr, err := run(t, "--bundle", bundleDir, "quarantine",
		"--discard", path, "--by", "human:priya", "--reason", why, "--apply",
	); err != nil {
		t.Fatalf("discard: %v\n%s", err, stderr)
	}

	log := readLog(t, bundleDir)
	for _, want := range []string{path, "human:priya", why} {
		if !strings.Contains(log, want) {
			t.Errorf("log.md does not record %q:\n%s", want, log)
		}
	}
}

// TestAnAgentsDiscardStaysInTheTrail is the other half of the rule, and without it
// the committed history fills with noise.
//
// `Discard.By` may be an agent deliberately — dropping a draft grants no authority —
// and an agent clearing a reply its own gate refused is housekeeping rather than
// adjudication. §10.7.4 is about decisions, and a decision is a person's. The reason
// is still recorded, in the per-user trail, where an agent's housekeeping belongs.
func TestAnAgentsDiscardStaysInTheTrail(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	if _, stderr, err := run(t, "--bundle", bundleDir, "quarantine",
		"--discard", path, "--by", "agent:test", "--reason", "the reply quoted nothing",
		"--apply",
	); err != nil {
		t.Fatalf("discard: %v\n%s", err, stderr)
	}
	if log := readLog(t, bundleDir); strings.Contains(log, path) {
		t.Errorf("an agent's housekeeping reached the committed history:\n%s", log)
	}
}

// TestAPreviewDeclinesNothing keeps §4.6.2's guarantee at the new write. A preview
// reports what would happen and writes nothing — including nothing committed.
func TestAPreviewDeclinesNothing(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	if _, stderr, err := run(t, "--bundle", bundleDir, "quarantine",
		"--discard", path, "--by", "human:priya", "--reason", "just looking",
	); err != nil {
		t.Fatalf("discard preview: %v\n%s", err, stderr)
	}
	if log := readLog(t, bundleDir); strings.Contains(log, path) {
		t.Errorf("a preview filed a decision:\n%s", log)
	}
}

// readLog reads log.md, treating an absent file as empty — which is what it is for a
// corpus nobody has filed anything in.
func readLog(t *testing.T, bundleDir string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(bundleDir, bundle.LogFile))
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read log.md: %v", err)
	}
	return string(raw)
}
