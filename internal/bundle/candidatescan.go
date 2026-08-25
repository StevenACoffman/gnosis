package bundle

import (
	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/gate"
	"github.com/StevenACoffman/gnosis/internal/scan"
)

// scanCandidate runs §9.3's admission scan over the bytes that would be written.
//
// Requires: after is the exact candidate content, frontmatter included; rules is
// the loaded stage 2 and 3 ruleset, or nil when it could not be loaded; limits
// carries stage 4's bounds, or zeroes when standards were not loaded.
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
//
// # This function is where §9.3's coverage claim is made honest
//
// It is the only production caller of `scan.CoverageOf`, and that is a design
// choice rather than an accident. Coverage cannot be checked by the type system —
// nothing stops a caller naming a stage it did not perform — so the stages it
// claims and the code performing them are put in one place, adjacent, where a
// reader compares them by looking. Each stage is claimed **inside the branch that
// runs it**, so adding a stage without claiming it, or claiming one without running
// it, requires editing two lines that touch.
//
// A missing capability degrades and **says so**: a nil ruleset drops stages 2 and 3,
// a zero cap drops stage 4, and both appear in Missing. That is the honest
// direction, because a scan that could not run its stages and reported no findings
// would be indistinguishable from a document with nothing wrong with it — and
// because `security` reads Missing to decide whether to route the candidate to a
// person.
func scanCandidate(after []byte, rules *scan.Ruleset, limits gate.Limits) gate.Scan {
	text := string(after)

	// Stage 1 has no configuration and always runs: its constants are codepoint
	// ranges from the Unicode standard, so there is nothing a caller could fail to
	// supply.
	ran := []string{scan.StageHidden}
	hidden := scan.Hidden(text)

	var matched []scan.Match
	if rules.Runs() {
		ran = append(ran, scan.StageInjection, scan.StageSecrets)
		matched = rules.Patterns(text)
	}

	findings := scan.Describe(hidden, matched)

	// Stage 4 applies the archive's own declared caps to the candidate. Until now
	// the bound existed for a fetched source and not for the document a model wrote
	// out of it — the more dangerous artifact, since it is the one filed into the
	// corpus for an agent to obey.
	if limits.PerFileCap > 0 || limits.EmbeddedPayloadCap > 0 {
		ran = append(ran, scan.StageOversize)
		// The measurement, not just the reason. A candidate refused for an oversize
		// payload has no §9.5.1 human path — a scan failure is `refused` — so the
		// finding is the whole of what the author has to work from, and "which
		// payload, how big, against what" is the difference between truncating an
		// example and arguing the cap down.
		if b := archive.Oversize(
			after, limits.PerFileCap, limits.EmbeddedPayloadCap,
		); b.Exceeded() {
			findings = append(findings, b.Detail())
		}
	}

	coverage := scan.CoverageOf(ran...)
	return gate.Scan{
		Findings:      findings,
		StagesRun:     coverage.Ran,
		StagesMissing: coverage.Missing,
	}
}
