package main

import (
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
)

// specFile is the specification, relative to this package's directory. `go test`
// sets the working directory to the package's own, so a relative path is the whole
// mechanism — no runtime.Caller, no os.Getwd.
const specFile = "SPEC.md"

// enforcedHeading is the section whose table this test checks.
const enforcedHeading = "### 12.1 What Is Actually Enforced"

// driftHeading is the section that must name the one finding category no lint check
// emits.
const driftHeading = "#### 14.3.2.1"

// TestTheEnforcementTableMatchesTheRegistry is the mechanism that makes SPEC §12.1
// more than a second place to drift.
//
// The specification carries sixty-three MUSTs and eleven lint checks, so almost
// every rule is convention and the useful table is the short one — the rules
// something actually checks. A hand-maintained table of twenty rows is a smaller
// burden than sixty-three inline tags, **and** it rots in a way tags do not: a
// checker that is deleted leaves the table claiming enforcement that no longer
// exists, and nobody reading the spec can tell.
//
// So the table is walked against `lint.Checks()` in both directions. A check named
// in the table and absent from the registry is the failure that matters; a check in
// the registry and absent from the table is a checker nobody documented. That buys
// the non-drift property of a generated table at the cost of a hand-written one.
//
// It lives at the repository root rather than in `internal/lint` because its subject
// is the correspondence between a repository-level document and the registry, and
// because here the fixture path is plainly `SPEC.md`.
func TestTheEnforcementTableMatchesTheRegistry(t *testing.T) {
	t.Parallel()

	named := checksNamedInSpec(t)
	registry := map[string]bool{}
	for _, c := range lint.Checks(time.Now()) {
		registry[c.Name] = true
	}

	for name := range registry {
		if !named[name] {
			t.Errorf("the lint check %q is not in SPEC %s; a checker nobody "+
				"documented is a rule readers cannot find", name, enforcedHeading)
		}
	}
	for name := range named {
		if strings.Contains(name, ".") || strings.Contains(name, " ") {
			continue // an enforcer that is not a lint check: a gate, a loader, a type
		}
		if !registry[name] && !gateSignals()[name] && !nonLintEnforcers()[name] {
			t.Errorf("SPEC %s names %q as enforced and no such check exists",
				enforcedHeading, name)
		}
	}
}

// TestEveryCheckDeclaresItsCategories, because the emitted vocabulary is not
// enumerable by inspection: most categories are string literals inside a Run body,
// but `identity` and `index-drift` come out of resolutionCategory, so a grep for
// literals finds neither. The declaration is what makes §12.1's Emits column
// checkable at all.
func TestEveryCheckDeclaresItsCategories(t *testing.T) {
	t.Parallel()

	for _, c := range lint.Checks(time.Now()) {
		if len(c.Categories) == 0 {
			t.Errorf("%s declares no categories", c.Name)
		}
		for _, category := range c.Categories {
			if strings.TrimSpace(category) == "" {
				t.Errorf("%s declares an empty category", c.Name)
			}
		}
		if c.Applies == nil {
			t.Errorf("%s has no Applies", c.Name)
		}
		if c.Run == nil {
			t.Errorf("%s has no Run", c.Name)
		}
	}
}

// TestTheDeclaredCategoriesAreInTheSpec closes the second direction: a category
// emitted by a check and absent from §12.1's Emits column is a finding a reader of
// the specification cannot look up.
func TestTheDeclaredCategoriesAreInTheSpec(t *testing.T) {
	t.Parallel()

	documented := backtickedInSection(t)
	for _, c := range lint.Checks(time.Now()) {
		for _, category := range c.Categories {
			if !documented[category] {
				t.Errorf("%s emits the category %q and SPEC %s does not document it",
					c.Name, category, enforcedHeading)
			}
		}
	}
}

// checksNamedInSpec is every name in the Enforced-by column of §12.1's table.
func checksNamedInSpec(t *testing.T) map[string]bool {
	t.Helper()

	out := map[string]bool{}
	for _, row := range tableRows(t) {
		cells := strings.Split(row, "|")
		if len(cells) < 4 {
			continue
		}
		for _, name := range backticked(cells[2]) {
			out[name] = true
		}
	}
	if len(out) == 0 {
		t.Fatalf("no enforcers found under %s; the table moved or its shape changed",
			enforcedHeading)
	}
	return out
}

// backtickedInSection is every backticked token anywhere in §12.1, which is the set
// a category is allowed to come from.
func backtickedInSection(t *testing.T) map[string]bool {
	t.Helper()

	out := map[string]bool{}
	for _, row := range tableRows(t) {
		for _, token := range backticked(row) {
			out[token] = true
		}
	}
	return out
}

// readSpec reads the specification.
//
// One reader for both tests here, because two would be two chances to disagree about
// which file is the subject — and the path is the whole mechanism that keeps this test
// free of runtime.Caller.
func readSpec(t *testing.T) string {
	t.Helper()

	src, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatalf("read %s: %v", specFile, err)
	}
	return string(src)
}

// tableRows is every table row in §12.1, header and separator excluded.
func tableRows(t *testing.T) []string {
	t.Helper()

	body := readSpec(t)
	start := strings.Index(body, enforcedHeading)
	if start < 0 {
		t.Fatalf("%s does not contain %q", specFile, enforcedHeading)
	}
	section := body[start:]
	if end := strings.Index(section, "\n## "); end > 0 {
		section = section[:end]
	}

	var out []string
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case !strings.HasPrefix(line, "|"):
			continue
		case strings.Contains(line, "---"):
			continue // the separator row
		case strings.Contains(line, "Enforced by"):
			continue // the header row
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		t.Fatalf("no table rows under %s", enforcedHeading)
	}
	return out
}

// backticked returns the `code`-spanned tokens in a cell.
func backticked(cell string) []string {
	var out []string
	for i, part := range strings.Split(cell, "`") {
		if i%2 == 1 && strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	sort.Strings(out)
	return out
}

// gateSignals are the promote-gate signals §12.1 lists as enforcers. They are not
// lint checks and have no entry in the registry, so the walk needs to know them by
// name rather than treating them as missing checks.
func gateSignals() map[string]bool {
	return map[string]bool{
		"security": true, "evidence": true, "provenance": true,
		"duplication": true, "hedging": true, "conflict": true,
	}
}

// nonLintEnforcers are the types and loaders §12.1 credits with enforcing a rule:
// the writer lock, the audit append, the standards loaders, and the scan ruleset.
// Each is checked by its own package's tests; what this test owns is that the table
// does not name something that has ceased to exist.
func nonLintEnforcers() map[string]bool {
	return map[string]bool{
		"schema-shape": true,
	}
}

// TestTheDriftCategoryIsDocumentedWhereItLives is the second half of §12.1's
// guarantee, and it exists because the first half cannot reach this category.
//
// `drift-unsupported` is opened by `fetch --recheck`, which does network I/O — and
// §4.6 forbids a lint check from doing that, so the category can never appear in
// `lint.Checks()`. Registering a no-network check so that it did would buy
// enumerability by putting a row in §12.1's table for something `lint` cannot run,
// which is a false statement in the very document the other test keeps true.
//
// So the category is a constant with one definition, and this asserts the
// specification names it. Two checked places rather than one checked place and one
// unchecked one.
func TestTheDriftCategoryIsDocumentedWhereItLives(t *testing.T) {
	t.Parallel()

	section := sectionAfter(t, driftHeading)
	if !strings.Contains(section, bundle.CategoryDriftUnsupported) {
		t.Errorf("SPEC %s does not name the %q category, so the one finding "+
			"category outside the lint registry is documented nowhere a reader "+
			"auditing the vocabulary would look",
			driftHeading, bundle.CategoryDriftUnsupported)
	}
}

// sectionAfter is one section's text, from its heading to the next heading of the
// same level or shallower.
//
// Bounded rather than read to the end of the file, because an unbounded search would
// pass on a mention anywhere later in the document — including in a section about
// something else. A test that cannot fail for the right reason is not evidence.
func sectionAfter(t *testing.T, heading string) string {
	t.Helper()

	spec := readSpec(t)
	at := strings.Index(spec, heading)
	if at < 0 {
		t.Fatalf("%s does not contain %q", specFile, heading)
	}
	body := spec[at+len(heading):]
	// The heading's own level, so a deeper subsection stays inside it.
	depth := strings.Count(strings.Fields(heading)[0], "#")
	if end := strings.Index(body, "\n"+strings.Repeat("#", depth)+" "); end >= 0 {
		body = body[:end]
	}
	return body
}

// TestTheFixableColumnMatchesTheDeclarations is what makes the column evidence rather
// than a third place to remember.
//
// §12.1's table now says what a reader can do about each finding, and the source of
// that answer is `Check.Actions`. A column typed by hand would drift the moment a
// diagnostic's action changed — and it would drift silently, because a table saying "a
// tool can fix this" reads as reassurance rather than as a claim to check.
//
// Both directions, as with the Emits column: an action declared and absent from the
// table fails, and a table cell naming an action the check does not declare fails.
func TestTheFixableColumnMatchesTheDeclarations(t *testing.T) {
	t.Parallel()

	documented := fixableColumn(t)
	for _, c := range lint.Checks(time.Now()) {
		cell, named := documented[c.Name]
		if !named {
			t.Errorf("the lint check %q has no row in SPEC %s", c.Name, enforcedHeading)
			continue
		}
		compareActions(t, c.Name, c.Actions, cell)
	}
}

// compareActions checks one row's Fixable cell against one check's declarations, both
// ways.
//
// Split out of the test because the linter reported its complexity and was right: the
// loop over checks and the two comparisons are different questions, and the outer one is
// only about finding the row.
func compareActions(t *testing.T, name string, actions []finding.Action, cell string) {
	t.Helper()

	for _, a := range actions {
		if !strings.Contains(cell, string(a)) {
			t.Errorf("%s declares action %q and SPEC's Fixable column says %q",
				name, a, cell)
		}
	}
	// And the other way: a cell naming an action the check does not emit would promise
	// a reader work a tool will not do.
	for _, word := range strings.Split(cell, ",") {
		word = strings.TrimSpace(word)
		if word == "" || word == "—" {
			continue
		}
		if !declares(actions, word) {
			t.Errorf("SPEC says %s is %q and the check does not declare it", name, word)
		}
	}
}

// declares reports whether the action set contains the named action.
func declares(actions []finding.Action, name string) bool {
	for _, a := range actions {
		if string(a) == name {
			return true
		}
	}
	return false
}

// fixableColumn maps each enforcer named in §12.1 to its Fixable cell.
//
// The column is read by position, which is why it is last in the table: a column
// inserted before the enforcer would shift what `checksNamedInSpec` reads and that test
// would keep passing against the wrong cell.
func fixableColumn(t *testing.T) map[string]string {
	t.Helper()

	const enforcer, fixable = 2, 4
	out := map[string]string{}
	for _, row := range tableRows(t) {
		cells := strings.Split(row, "|")
		if len(cells) <= fixable {
			continue
		}
		for _, name := range backticked(cells[enforcer]) {
			out[name] = strings.TrimSpace(cells[fixable])
		}
	}
	return out
}
