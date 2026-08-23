package bundle

import (
	"strconv"

	"github.com/StevenACoffman/gnosis/internal/gate"
	"github.com/StevenACoffman/gnosis/internal/scan"
)

// scanCandidate runs §9.3's admission scan over the bytes that would be written.
//
// Requires: after is the exact candidate content, frontmatter included.
// Ensures: a gate.Scan naming what was found and which stages looked. Pure.
//
// The whole file is scanned rather than the parsed body, and that is deliberate.
// Frontmatter is where a `subject`, a `type`, or a source URI lives, every one of
// which a later command reads and some of which an agent acts on; a zero-width
// character hiding in a key is exactly as effective there as in prose, and
// scanning only the body would leave the machine-read half unexamined.
//
// Scanning happens here rather than at parse time because a document that fails to
// parse still gets scanned this way. That is the case that most needs it: a
// candidate whose frontmatter is malformed *because* of a hidden character would
// otherwise be reported as a conformance failure with no mention of why.
func scanCandidate(after []byte) gate.Scan {
	coverage := scan.TextCoverage()
	found := scan.Hidden(string(after))

	rendered := make([]string, 0, len(found))
	for _, f := range found {
		rendered = append(rendered, string(f.Class)+" "+f.Rune+
			" ×"+strconv.Itoa(f.Count)+" at byte "+strconv.Itoa(f.Offset))
	}
	return gate.Scan{
		Findings:      rendered,
		StagesRun:     coverage.Ran,
		StagesMissing: coverage.Missing,
	}
}
