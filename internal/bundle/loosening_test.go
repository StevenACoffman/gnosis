package bundle_test

import (
	"slices"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/standards"
)

// countable are the archive thresholds whose loosening has an exact finding delta.
//
// It mirrors the switch in describeLoosening, and the mirroring is the point: this
// file is the guard that switch never had.
var countable = []string{
	"corpus_budget", "corpus_warn_fraction", "staleness_days", "in_degree_cut",
}

// unreadHere are the archive thresholds describeLoosening reports as read by nothing.
//
// Empty as of 2026-09-02, when `in_degree_cut` gained the `durability` check as its
// reader and moved into `countable` above. The variable stays rather than being deleted:
// it is half of a two-direction guard, and the direction it watches — this file calling
// a threshold unread while `standards.Unread` calls it consumed — is the bug that was
// actually there.
var unreadHere = []string{}

// TestTheClassifierAgreesWithUnread is the guard the switch never had, and it exists
// because the switch was wrong.
//
// `describeLoosening` classified `staleness_days` as "nothing reads this threshold
// yet, so moving it changes no finding". That was true when written and stopped being
// true when the `stale` check gained its window — so widening the staleness window
// silenced `stale` findings while `standards check --log` recorded that it cost
// nothing. **That is the exact reassurance §6.2 exists to withhold, produced by the
// tool §6.2 asked for**, and nothing caught it because the classification lived in one
// switch and the truth lived in another.
//
// `standards.Unread` is the function that already knows, for the reason its own
// comment gives: nothing at runtime can discover what branches on a number, so the
// knowledge is static and recorded in one place. This test makes the *second* place
// answer to the first.
//
// Both directions matter. A threshold this file calls unread and `Unread` calls
// consumed is the bug that was there. A threshold this file counts and `Unread` calls
// unread is the reverse: a delta computed from a check that does not read it, which
// would be a number with nothing behind it.
func TestTheClassifierAgreesWithUnread(t *testing.T) {
	t.Parallel()

	unread := standards.Unread()

	for _, key := range unreadHere {
		if !slices.Contains(unread, key) {
			t.Errorf("describeLoosening reports %q as read by nothing and "+
				"standards.Unread does not list it: a loosening of a threshold "+
				"something reads is being recorded as costing nothing", key)
		}
	}
	for _, key := range countable {
		if slices.Contains(unread, key) {
			t.Errorf("describeLoosening computes a finding delta for %q and "+
				"standards.Unread reports nothing reads it: the delta would be a "+
				"number with nothing behind it", key)
		}
	}
}

// TestStalenessDaysIsNotUnread is the specific regression, named so a failure says
// what broke rather than which set disagreed.
func TestStalenessDaysIsNotUnread(t *testing.T) {
	t.Parallel()

	if slices.Contains(standards.Unread(), "staleness_days") {
		t.Error("staleness_days is unread again; the stale check has lost its window, " +
			"and the loosening report has gone back to saying a wider window is free")
	}
}

// TestEveryComparableThresholdIsClassified. `standards.CompareArchive` can report
// seven keys, and a key it reports that this file has not classified falls to the
// admission wording — which is the safe direction and is still wrong for anything
// that is not admission.
//
// The list is written out rather than derived, because deriving it from
// CompareArchive would mean constructing two Archives that differ in every value,
// which is a fixture that has to be maintained in step with the same set this asserts.
func TestEveryComparableThresholdIsClassified(t *testing.T) {
	t.Parallel()

	comparableKeys := []string{
		"allowlist", "per_file_cap", "embedded_payload_cap",
		"corpus_budget", "corpus_warn_fraction", "staleness_days", "in_degree_cut",
	}
	classified := append(append([]string{}, countable...), unreadHere...)
	// The remainder are the two gate-affecting caps and the allowlist, which the
	// switch handles explicitly or by the admission default.
	classified = append(classified, "per_file_cap", "embedded_payload_cap", "allowlist")

	for _, key := range comparableKeys {
		if !slices.Contains(classified, key) {
			t.Errorf("%q can be reported as loosened and this file does not say "+
				"which category it is in", key)
		}
	}
	for _, key := range classified {
		if !slices.Contains(comparableKeys, key) {
			t.Errorf("%q is classified here and CompareArchive cannot report it; "+
				"the classification is describing a threshold that never loosens", key)
		}
	}
}
