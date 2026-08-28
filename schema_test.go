package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/index"
)

// schemaHeading is the section whose SQL block this test checks.
const schemaHeading = "### 5.5 Derived Index"

// notBuilt is the marker a specified table carries when it has no migration.
const notBuilt = "-- not built:"

// tableDecl matches a table declaration at the start of a line in §5.5's block: either
// `name(cols…)` or `name USING fts5(…)`. Indented lines are continuations of the column
// list and are deliberately not matched.
var tableDecl = regexp.MustCompile(`^([a-z_]+)\s*(\(|USING )`)

// TestTheSchemaBlockMatchesTheMigrations walks §5.5 against a real database in both
// directions, the way spec_test.go walks §12.1's table against the check registry.
//
// **The gap this closes is not a missing table; it is a block that could not say which
// tables it had.** §5.5 presented ten as the schema while `internal/index` created six,
// and a reader had no way to tell a table that was late from one that was deliberately
// unbuilt. That is the same shape `coverage` had when it was specified, listed as
// enforced, and unbuilt — and the confusion between "unbuilt" and "unbuildable" has cost
// this backlog four corrections.
//
// **The marker lives in the spec rather than in a Go list**, so the documentation and the
// thing documented are one artifact and cannot drift apart. A second list here would be
// the hand-maintained allowlist this codebase has twice deleted.
//
// Both directions fail, for different mistakes:
//
//   - **Specified, unmarked, and absent from the database.** A table a reader would
//     expect to query. This is the direction §5.5 was silently wrong in.
//   - **Marked `not built` and present.** The table arrived and nobody removed the note,
//     so the specification now understates what the code does — which is how a real
//     capability stays undiscovered.
func TestTheSchemaBlockMatchesTheMigrations(t *testing.T) {
	t.Parallel()

	specified, marked := schemaTables(t)
	if len(specified) == 0 {
		t.Fatalf("no table declarations found under %s; the block moved or its shape "+
			"changed", schemaHeading)
	}

	db, err := index.Open(t.Context(), filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("open a fresh index: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	built := map[string]bool{}
	names, err := db.Tables(t.Context())
	if err != nil {
		t.Fatalf("read the tables: %v", err)
	}
	for _, n := range names {
		built[n] = true
	}

	for _, name := range specified {
		switch {
		case built[name] && marked[name]:
			t.Errorf("%s is marked %q in %s and the migrations create it; remove the "+
				"marker so the block stops understating what the code does",
				name, notBuilt, schemaHeading)
		case !built[name] && !marked[name]:
			t.Errorf("%s is specified in %s with no migration and no %q marker; either "+
				"build it or record why it is not built, because a reader cannot tell "+
				"a late table from a deliberate one", name, schemaHeading, notBuilt)
		}
	}
}

// schemaTables reads §5.5's SQL block, returning every declared table name and the set
// carrying a `not built` marker.
//
// The marker is read from the comment lines **immediately above** a declaration, which is
// where a reader looking at the table will see it. A marker floating elsewhere in the
// block would be true documentation and false structure.
func schemaTables(t *testing.T) ([]string, map[string]bool) {
	t.Helper()

	names := make([]string, 0)
	marked := map[string]bool{}
	pending := false
	for _, line := range strings.Split(schemaBlock(t), "\n") {
		switch {
		case strings.HasPrefix(line, notBuilt):
			pending = true
		case strings.HasPrefix(line, "--"):
			// An ordinary comment neither sets nor clears the marker, so a reason
			// may run to several lines.
		case tableDecl.MatchString(line):
			names = append(names, tableDecl.FindStringSubmatch(line)[1])
			if pending {
				marked[names[len(names)-1]] = true
			}
			pending = false
		case strings.TrimSpace(line) == "":
			pending = false // a blank line ends a comment's reach
		}
	}
	sort.Strings(names)
	return names, marked
}

// schemaBlock returns the body of §5.5's fenced SQL block.
func schemaBlock(t *testing.T) string {
	t.Helper()

	body, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatalf("read %s: %v", specFile, err)
	}
	rest := string(body)
	at := strings.Index(rest, schemaHeading)
	if at < 0 {
		t.Fatalf("%s not found in %s", schemaHeading, specFile)
	}
	rest = rest[at:]

	open := strings.Index(rest, "```sql")
	if open < 0 {
		t.Fatalf("no sql block under %s", schemaHeading)
	}
	rest = rest[open+len("```sql"):]
	if end := strings.Index(rest, "```"); end >= 0 {
		rest = rest[:end]
	}
	return rest
}
