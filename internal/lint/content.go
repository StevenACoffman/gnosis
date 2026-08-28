package lint

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
)

// placeholderPattern matches the `{{NAME}}` markers a template leaves behind.
//
// Deliberately narrow: only upper-case, digits, underscore, and hyphen inside the
// braces. A wider pattern would collide with template syntaxes a document might
// legitimately be *about* — a page documenting Go templates or Jinja is not a page
// with an unfilled placeholder, and a check that cannot tell those apart is a check
// people learn to ignore.
var placeholderPattern = regexp.MustCompile(`\{\{[A-Z0-9_-]+\}\}`)

// schemaVersionCheck reports documents written under older corpus conventions.
//
// This is a fourth kind of drift and the one with no other detector (SPEC
// §5.5.1.1): `stale_after` is the source changing, `index-drift` is the cache
// falling behind, `schema-shape` is the database not matching its migrations, and
// this is the corpus's own conventions changing underneath documents that already
// exist.
//
// It reports and never rewrites. Whether an older document should be brought
// forward is a decision per convention change — some are worth backfilling and
// most are not — and a check that decided for you would make the cheap changes
// expensive.
func schemaVersionCheck() Check {
	return Check{
		Name:       "schema-version",
		Categories: []string{"schema-version"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies: func(snap *Snapshot) (bool, string) {
			if snap.SchemaVersion == 0 {
				return false, "this build declares no schema version, so nothing can be older than it"
			}
			// Derived applicability, the same rule the orphan and index-drift
			// checks follow. Until *some* document declares a version, none do,
			// and reporting every document in the corpus on the day versioning
			// is introduced would teach a reader to ignore the check before it
			// ever says anything useful. It activates when the corpus starts
			// versioning, and then finds exactly the documents left behind.
			for i := range snap.Documents {
				if snap.Documents[i].SchemaVersion != nil {
					return true, ""
				}
			}
			return false, "no document declares a schema version yet, so the corpus has not started versioning"
		},
		Run: func(snap *Snapshot) []finding.Diagnostic {
			out := make([]finding.Diagnostic, 0)
			for i := range snap.Documents {
				d := &snap.Documents[i]
				if d.SchemaVersion != nil && *d.SchemaVersion >= snap.SchemaVersion {
					continue
				}
				out = append(out, finding.Diagnostic{
					Severity: finding.SeverityWarning,
					Category: "schema-version",
					Path:     d.Path,
					Message:  schemaVersionMessage(d.SchemaVersion, snap.SchemaVersion),
					Action:   finding.ActionHuman,
				})
			}
			return out
		},
	}
}

// schemaVersionMessage distinguishes "written before versioning" from "written
// under a known older version", because the two call for different reading: the
// first predates the convention entirely and the second can be diffed against it.
func schemaVersionMessage(got *int, want int) string {
	if got == nil {
		return fmt.Sprintf(
			"declares no schema version; the corpus is at %d, so this predates versioning", want)
	}
	return fmt.Sprintf("written under schema version %d; the corpus is at %d", *got, want)
}

// placeholderCheck reports unfilled template markers left in a document.
//
// This is what an agent leaves behind when it runs out of material: the structure
// of a page with `{{SUMMARY}}` where the summary goes. It reads as a finished
// document to every other check — it conforms, it has a type, its links resolve —
// which is exactly why it needs one of its own.
func placeholderCheck() Check {
	return Check{
		Name:       "placeholder",
		Categories: []string{"placeholder"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    always,
		Run: func(snap *Snapshot) []finding.Diagnostic {
			out := make([]finding.Diagnostic, 0)
			for i := range snap.Documents {
				d := &snap.Documents[i]
				for _, marker := range dedupeInOrder(placeholderPattern.FindAllString(d.Body, -1)) {
					out = append(out, finding.Diagnostic{
						Severity: finding.SeverityError,
						Category: "placeholder",
						Path:     d.Path,
						Message:  "unfilled template marker " + marker,
						Action:   finding.ActionHuman,
					})
				}
			}
			return out
		},
	}
}

// emptySectionCheck reports headings with no content beneath them.
//
// A heading is a promise about what follows. An empty one is the same failure as a
// placeholder wearing better clothes — the page looks complete in an outline and
// answers nothing — and it is likewise invisible to every structural check.
//
// Warning rather than error, and level decides what counts as empty: a heading
// followed by a *deeper* one is a parent whose content is its subsections, while a
// heading followed by a same-or-shallower one with nothing between is a promise
// nobody kept. Getting that wrong reports every parent heading in the corpus.
func emptySectionCheck() Check {
	return Check{
		Name:       "empty-section",
		Categories: []string{"empty-section"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    always,
		Run: func(snap *Snapshot) []finding.Diagnostic {
			out := make([]finding.Diagnostic, 0)
			for i := range snap.Documents {
				d := &snap.Documents[i]
				for _, heading := range emptySections(d.Body) {
					out = append(out, finding.Diagnostic{
						Severity: finding.SeverityWarning,
						Category: "empty-section",
						Path:     d.Path,
						Message:  "section " + strconv.Quote(heading) + " has no content",
						Action:   finding.ActionHuman,
					})
				}
			}
			return out
		},
	}
}

// emptySections returns the headings in body that are followed by no prose before
// the next heading or the end.
//
// A following heading of any level ends the section without emptying it: nesting
// is structure, not omission. Only a heading with nothing but blank lines under it
// is reported.
func emptySections(body string) []string {
	var (
		out     []string
		pending string
		level   int
		filled  bool
	)
	// closedBy reports the pending section, unless it was filled or the heading
	// that closed it is deeper — a parent's content is its subsections.
	closedBy := func(nextLevel int) {
		if pending != "" && !filled && nextLevel <= level {
			out = append(out, pending)
		}
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if heading, hLevel, ok := headingText(trimmed); ok {
			closedBy(hLevel)
			pending, level, filled = heading, hLevel, false
			continue
		}
		if trimmed != "" {
			filled = true
		}
	}
	// End of document closes at the shallowest possible level: nothing follows, so
	// a trailing heading has no subsections to be the parent of.
	closedBy(0)
	return out
}

// headingText returns the text and level of an ATX heading line.
func headingText(trimmed string) (text string, level int, ok bool) {
	if !strings.HasPrefix(trimmed, "#") {
		return "", 0, false
	}
	rest := strings.TrimLeft(trimmed, "#")
	level = len(trimmed) - len(rest)
	if rest == "" || !strings.HasPrefix(rest, " ") {
		// "#hashtag" is not a heading; a heading needs a space after the marks.
		return "", 0, false
	}
	return strings.TrimSpace(rest), level, true
}

// dedupeInOrder removes repeats while preserving first-seen order, so a marker
// appearing five times in one document produces one finding rather than five.
func dedupeInOrder(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
