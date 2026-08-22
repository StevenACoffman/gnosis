package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

// version returns a pointer to n, for the SchemaVersion field where nil and zero
// mean different things.
func version(n int) *int { return &n }

// TestSchemaVersionSeparatesUnversionedFromOlder is the distinction the check
// exists for. A document predating versioning and one written under a known older
// version are read differently — the second can be diffed against the current
// conventions, the first cannot — so the message must say which.
func TestSchemaVersionSeparatesUnversionedFromOlder(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		SchemaVersion: 3,
		Documents: []lint.Document{
			{Path: "c/old.md"}, // no version at all
			{Path: "c/older.md", SchemaVersion: version(1)},
			{Path: "c/current.md", SchemaVersion: version(3)},
			{Path: "c/ahead.md", SchemaVersion: version(4)},
		},
	}

	got := lint.Run(snap, lint.Checks(testNow())).Diagnostics
	byPath := map[string]string{}
	for _, d := range got {
		if d.Category == "schema-version" {
			byPath[d.Path] = d.Message
		}
	}

	if len(byPath) != 2 {
		t.Fatalf("got %d schema-version findings, want 2: %v", len(byPath), byPath)
	}
	if !strings.Contains(byPath["c/old.md"], "predates versioning") {
		t.Errorf("unversioned document message does not say so: %q", byPath["c/old.md"])
	}
	if !strings.Contains(byPath["c/older.md"], "version 1") {
		t.Errorf("older document message does not name its version: %q", byPath["c/older.md"])
	}
	if _, reported := byPath["c/ahead.md"]; reported {
		t.Error("a document ahead of the corpus was reported as behind it")
	}
}

// TestSchemaVersionSkipsUntilTheCorpusStartsVersioning is the applicability rule,
// and it matters on exactly one day: the one versioning is introduced. Every
// document predates it, so a check without this would report the entire corpus at
// once and be ignored from then on. It activates when some document declares a
// version, and then finds the ones left behind — which is the case worth catching.
func TestSchemaVersionSkipsUntilTheCorpusStartsVersioning(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		SchemaVersion: 3,
		Documents:     []lint.Document{{Path: "c/a.md"}, {Path: "c/b.md"}},
	}

	report := lint.Run(snap, lint.Checks(testNow()))
	for _, d := range report.Diagnostics {
		if d.Category == "schema-version" {
			t.Errorf("reported against a corpus with no version: %+v", d)
		}
	}
	var skipped bool
	for _, s := range report.Skipped {
		if s.Check == "schema-version" && s.Reason != "" {
			skipped = true
		}
	}
	if !skipped {
		t.Error("schema-version did not report why it was skipped")
	}
}

// TestPlaceholderIsAnError, because a page with an unfilled marker reads as
// finished to every other check: it conforms, it has a type, its links resolve.
func TestPlaceholderIsAnError(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{Documents: []lint.Document{
		{Path: "c/a.md", Body: "The retry budget is {{VALUE}} and see {{VALUE}} again."},
	}}

	got := lint.Run(snap, lint.Checks(testNow())).Diagnostics
	var found int
	for _, d := range got {
		if d.Category == "placeholder" {
			found++
			if !d.Severity.Blocking() {
				t.Error("an unfilled placeholder does not block")
			}
		}
	}
	if found != 1 {
		t.Errorf(
			"got %d placeholder findings, want 1 — repeats of one marker are one finding",
			found,
		)
	}
}

// TestPlaceholderDoesNotFireOnDocumentedSyntax guards the narrowness of the
// pattern. A page *about* templating is not a page with an unfilled placeholder,
// and a check that cannot tell them apart is one people learn to ignore.
func TestPlaceholderDoesNotFireOnDocumentedSyntax(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{Documents: []lint.Document{
		{Path: "c/a.md", Body: "Go templates use {{.Name}} and Jinja uses {{ name }}."},
	}}

	for _, d := range lint.Run(snap, lint.Checks(testNow())).Diagnostics {
		if d.Category == "placeholder" {
			t.Errorf("fired on documented template syntax: %+v", d)
		}
	}
}

// TestEmptySectionNesting: a heading followed by a subheading is ordinary
// structure. Only a heading with nothing but blank lines under it is empty.
func TestEmptySectionNesting(t *testing.T) {
	t.Parallel()
	body := strings.Join([]string{
		"# Top",          // followed by a subheading — structure, not empty
		"## Filled",      //
		"some prose",     //
		"## Empty",       // nothing follows before the next heading
		"",               //
		"## Also filled", //
		"more prose",     //
		"## Trailing",    // nothing follows to the end of the document
	}, "\n")
	snap := &lint.Snapshot{Documents: []lint.Document{{Path: "c/a.md", Body: body}}}

	var empty []string
	for _, d := range lint.Run(snap, lint.Checks(testNow())).Diagnostics {
		if d.Category == "empty-section" {
			empty = append(empty, d.Message)
		}
	}
	if len(empty) != 2 {
		t.Fatalf("got %d empty sections, want Empty and Trailing: %v", len(empty), empty)
	}
	for _, want := range []string{"Empty", "Trailing"} {
		var seen bool
		for _, m := range empty {
			if strings.Contains(m, want) {
				seen = true
			}
		}
		if !seen {
			t.Errorf("%q was not reported empty: %v", want, empty)
		}
	}
}

// TestHashtagIsNotAHeading: "#topic" has no space after the marks and is not a
// heading, so it neither opens nor closes a section.
func TestHashtagIsNotAHeading(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{Documents: []lint.Document{
		{Path: "c/a.md", Body: "## Real heading\n#nothashtag\n"},
	}}

	for _, d := range lint.Run(snap, lint.Checks(testNow())).Diagnostics {
		if d.Category == "empty-section" {
			t.Errorf("treated a hashtag as a heading: %+v", d)
		}
	}
}
