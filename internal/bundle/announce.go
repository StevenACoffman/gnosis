package bundle

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// authorityMarker opens the log line an authority move is announced on.
//
// A bullet, because OKF §9's entry form is a list and every other writer of this file
// already writes one — a bare line renders as a paragraph and reads as prose somebody
// typed rather than as an entry.
//
// The marker is what makes the line findable again, and finding it again is the whole
// mechanism: `log.md` is where the previous authority is *stored*, so a reader and a
// writer that disagreed about the prefix would leave `doctor` reporting every corpus as
// never having announced anything.
const authorityMarker = "- Adjudication authority "

// renderAuthorityMove writes the announcement §10.6.3 requires.
//
// Requires: move.Moved() is true; a caller announcing a move that did not happen would
// put a line in `log.md` saying nothing changed.
// Ensures: a line parseAuthorityMove reads back. Pure.
//
// The sentence is `gnosis.AuthorityMove`'s own, so the entry and the diagnostic that
// reports an unannounced move cannot describe one event two ways.
func renderAuthorityMove(move gnosis.AuthorityMove, why string) string {
	line := authorityMarker + move.String()
	if why != "" {
		line += ": " + why
	}
	return line
}

// parseAuthorityMove reads an announcement back.
//
// Requires: nothing; any line may be offered.
// Ensures: the authority the line announced as current, and false for a line this
// cannot read. Pure.
//
// **It returns the destination and not the whole move**, because that is the question a
// reader of the log has: what did the corpus last say it requires. The origin is in the
// line for a person, and reconstructing it would be parsing prose to recover a fact
// nothing needs.
//
// A line the scan cannot read counts as no announcement, which **over-reports** rather
// than under-reports: a hand-edited log makes `doctor` say the current authority was
// never announced, and the remedy is to announce it. The reverse — a mangled line read
// as an announcement — would silence the check that exists because silence is the
// failure.
func parseAuthorityMove(line string) (gnosis.Authority, bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(line), strings.TrimSpace(authorityMarker))
	if !found {
		return gnosis.AuthoritySole, false
	}
	_, after, found := strings.Cut(rest, "→")
	if !found {
		return gnosis.AuthoritySole, false
	}
	word, _, _ := strings.Cut(strings.TrimSpace(after), ",")
	return gnosis.AuthorityOf(strings.TrimSpace(word))
}

// LastAnnouncedAuthority is the authority `log.md` last recorded the corpus as
// requiring.
//
// Requires: lines are log.md's lines, as LoadLog returns them.
// Ensures: the last readable announcement and true, or (AuthoritySole, false) when the
// log holds none. Pure.
//
// **The last rather than the first**, because the log accretes and an authority moves in
// both directions: a corpus that went `sole → paired → sole` has announced twice, and
// the earlier entry is history rather than the current claim.
//
// **This is the stored baseline, and it is the reason there is no other one.** The
// backlog filed this behind "the same baseline `newly-orphaned` waits on"; it needs no
// such thing. A committed announcement *is* the previous value, it reaches every user
// through the same `git pull` that carries the corpus, and it is in the tier a colleague
// reads rather than in a per-user cache two colleagues could disagree about.
func LastAnnouncedAuthority(lines []string) (gnosis.Authority, bool) {
	out, found := gnosis.AuthoritySole, false
	for _, line := range lines {
		if authority, ok := parseAuthorityMove(line); ok {
			out, found = authority, true
		}
	}
	return out, found
}
