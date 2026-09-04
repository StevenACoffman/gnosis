package bundle

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// TestAnAnnouncementRoundTrips is the property the whole mechanism rests on: `log.md`
// is the stored previous authority, so what `adjudicate` writes is what `doctor` reads.
// The two are otherwise a handshake nothing checks — and a prefix they disagreed about
// would leave every corpus reported as never having announced anything.
//
// It is a white-box test, and the filename says so: `_internal_test.go` is the form the
// repository's linter exempts from the black-box rule, which is the honest label here
// because the property *is* internal — a handshake between two unexported functions in
// one package. Reaching it through `export_test.go` would add two seam helpers to
// satisfy a form rather than a need. `LastAnnouncedAuthority` is the exported half and
// has its own case below.
func TestAnAnnouncementRoundTrips(t *testing.T) {
	t.Parallel()

	for name, move := range map[string]gnosis.AuthorityMove{
		"sole to paired": {
			From: gnosis.AuthoritySole, To: gnosis.AuthorityPaired, Adjudicators: 2,
		},
		"paired to quorum": {
			From: gnosis.AuthorityPaired, To: gnosis.AuthorityQuorum, Adjudicators: 4,
		},
		"quorum back to paired": {
			From: gnosis.AuthorityQuorum, To: gnosis.AuthorityPaired, Adjudicators: 3,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			line := renderAuthorityMove(move, "human:priya adjudicated c1 in c/a.md")
			got, ok := parseAuthorityMove(line)
			if !ok {
				t.Fatalf("the line does not parse back: %q", line)
			}
			if got != move.To {
				t.Errorf("parsed %v from %q, want %v", got, line, move.To)
			}
		})
	}
}

// TestLastAnnouncedAuthorityTakesTheLast, because an authority moves in both directions:
// a corpus that went sole → paired → sole has announced twice, and the earlier entry is
// history rather than the current claim.
func TestLastAnnouncedAuthorityTakesTheLast(t *testing.T) {
	t.Parallel()

	lines := []string{
		"## 2026-09-01",
		"",
		renderAuthorityMove(gnosis.AuthorityMove{
			From: gnosis.AuthoritySole, To: gnosis.AuthorityPaired, Adjudicators: 2,
		}, "priya"),
		"## 2026-09-03",
		"",
		renderAuthorityMove(gnosis.AuthorityMove{
			From: gnosis.AuthorityPaired, To: gnosis.AuthoritySole, Adjudicators: 1,
		}, "marcus left"),
		"- Declined `c/a.md` (human:priya): a draft nobody wanted",
	}
	got, found := LastAnnouncedAuthority(lines)
	if !found || got != gnosis.AuthoritySole {
		t.Errorf("LastAnnouncedAuthority = (%v, %v), want (sole, true)", got, found)
	}
}

// TestAnUnreadableLineCountsAsNoAnnouncement, which is the safe direction: a hand-edited
// log makes `doctor` say the current authority was never announced, and the remedy is to
// announce it. A mangled line read *as* an announcement would silence the check that
// exists because silence is the failure.
func TestAnUnreadableLineCountsAsNoAnnouncement(t *testing.T) {
	t.Parallel()

	for name, lines := range map[string][]string{
		"an empty log":       {},
		"prose only":         {"## 2026-09-03", "", "We talked about the retry budget."},
		"another entry kind": {"- Declined `c/a.md` (human:priya): junk"},
		"the marker mangled": {"- Adjudication authority sole to paired"},
		"an unknown word":    {"- Adjudication authority sole → committee, 2 adjudicators"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got, found := LastAnnouncedAuthority(lines); found {
				t.Errorf("LastAnnouncedAuthority = (%v, true) for %q", got, lines)
			}
		})
	}
}
