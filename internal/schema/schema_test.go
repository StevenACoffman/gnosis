package schema_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/schema"
)

// vocabulary is a generated region, named as the real one is.
func vocabulary(body string) []schema.Region {
	return []schema.Region{{Name: "vocabulary", Body: body}}
}

// TestAFileWithNoMarkersIsNeverOverwritten is the fail-closed rule and the one the
// backlog entry singles out.
//
// A file predating the tool was not written under its contract, so treating its
// existence as consent is fail-open — and §5.7 records the cost: an ETH Zurich study
// found auto-generated context files *reduced* agent success in five of eight
// settings, so silently converting somebody's hand-written AGENTS.md into a generated
// one is a change with measured evidence against it.
func TestAFileWithNoMarkersIsNeverOverwritten(t *testing.T) {
	t.Parallel()

	const existing = "# AGENTS\n\nHand-written, by a person, before gnosis existed.\n"
	got, outcome := schema.Merge(existing, vocabulary("- Reference"))

	if outcome != schema.OutcomeUnmarked {
		t.Fatalf("outcome = %v, want unmarked", outcome)
	}
	// The caller must not write this to the existing path, and Merged is what says so.
	if outcome.Merged() {
		t.Error("an unmarked file was reported as writable")
	}
	// What comes back is the text for a sibling file, so it must not contain the
	// person's prose — writing it beside them and duplicating their words would be a
	// second surprise.
	if strings.Contains(got, "Hand-written") {
		t.Errorf("the sibling text carries the person's prose:\n%s", got)
	}
	if !strings.Contains(got, "Reference") {
		t.Errorf("the sibling text carries no generated content:\n%s", got)
	}
}

// TestAnUnterminatedMarkerRefuses is the rule the entry does not state and the code
// needs.
//
// Reading an unterminated marker as "everything to the end of the file" would let a
// single typo hand a whole document to the generator. The extent of the region is
// unknown, so nothing is written.
func TestAnUnterminatedMarkerRefuses(t *testing.T) {
	t.Parallel()

	for name, existing := range map[string]string{
		"begin with no end": "# AGENTS\n\n<!-- gnosis:begin vocabulary -->\n" +
			"- Reference\n\nAnd then a person's paragraph, unclosed.\n",
		// The mirror image. It means the same thing to a writer: the file's regions
		// cannot be determined, so the file may not be written.
		"end with no begin": "# AGENTS\n\n- Reference\n" +
			"<!-- gnosis:end vocabulary -->\n",
		// A half-written marker: the prefix with no terminator at all.
		"a truncated marker": "# AGENTS\n\n<!-- gnosis:begin vocabular\n- Reference\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, outcome := schema.Merge(existing, vocabulary("- Reference"))
			if outcome != schema.OutcomeMalformed {
				t.Fatalf("outcome = %v, want malformed", outcome)
			}
			if got != "" {
				t.Errorf("a refusal returned content to write:\n%s", got)
			}
		})
	}
}

// TestTheRefusalNamesTheMarker keeps the diagnostic actionable. A caller told
// "malformed" with no name has to search the file, which is the work the tool exists to
// have already done.
func TestTheRefusalNamesTheMarker(t *testing.T) {
	t.Parallel()

	const existing = "<!-- gnosis:begin commands -->\n- gnosis fetch\n"
	if got, bad := schema.Unclosed(existing); !bad || got != "commands" {
		t.Errorf("Unclosed = (%q, %v), want (commands, true)", got, bad)
	}
	if _, bad := schema.Unclosed("no markers here at all\n"); bad {
		t.Error("a clean file was reported as malformed")
	}
	// A half-written marker has no name to report and is still a refusal, which is
	// the pair the first version of this collapsed.
	if _, bad := schema.Unclosed("<!-- gnosis:begin vocabular\n"); !bad {
		t.Error("a truncated marker was not reported")
	}
}

// TestEverythingOutsideAMarkerIsPreservedByteForByte is the second rule, and the
// "byte for byte" is the part worth asserting.
//
// A person's prose is not the generator's to tidy. A formatter that re-wrapped it, or
// normalised its whitespace, or reordered a list would be the silent rewrite this
// contract exists to prevent — and it would look like an improvement in the diff.
func TestEverythingOutsideAMarkerIsPreservedByteForByte(t *testing.T) {
	t.Parallel()

	// Deliberately untidy: trailing spaces, a tab, inconsistent blank lines, a line
	// far past any sane width. Every one of these is something a formatter would fix.
	const before = "#    AGENTS   \n\n\n\tWhy this corpus exists, at length and on one " +
		"very long line that no formatter would leave alone.   \n\n"
	const after = "\n\nA closing note.\t\n   \n"

	existing := before +
		"<!-- gnosis:begin vocabulary -->\nstale content\n<!-- gnosis:end vocabulary -->\n" +
		after

	got, outcome := schema.Merge(existing, vocabulary("- Reference\n- Standard"))
	if outcome != schema.OutcomeMerged {
		t.Fatalf("outcome = %v, want merged", outcome)
	}
	if !strings.HasPrefix(got, before) {
		t.Errorf("the text before the region changed:\n%q", got)
	}
	if !strings.HasSuffix(got, after) {
		t.Errorf("the text after the region changed:\n%q", got)
	}
	if strings.Contains(got, "stale content") {
		t.Error("the generated region was not replaced")
	}
	if !strings.Contains(got, "- Standard") {
		t.Error("the new content is not there")
	}
}

// TestARegionTheFileDoesNotCarryIsNotInserted, because where a region belongs in
// somebody's document is their decision.
//
// A generator that appended a missing region would be choosing the shape of a file it
// was told to leave alone — and it would do so on every run, so a person who deleted a
// region they did not want would get it back.
func TestARegionTheFileDoesNotCarryIsNotInserted(t *testing.T) {
	t.Parallel()

	const existing = "# AGENTS\n\n<!-- gnosis:begin vocabulary -->\nold\n" +
		"<!-- gnosis:end vocabulary -->\n"
	got, outcome := schema.Merge(existing, []schema.Region{
		{Name: "vocabulary", Body: "- Reference"},
		{Name: "commands", Body: "- gnosis fetch"},
	})
	if outcome != schema.OutcomeMerged {
		t.Fatalf("outcome = %v, want merged", outcome)
	}
	if !strings.Contains(got, "- Reference") {
		t.Error("the region the file carries was not updated")
	}
	if strings.Contains(got, "gnosis fetch") {
		t.Errorf("a region the file does not carry was inserted:\n%s", got)
	}
}

// TestRenderIsItsOwnFixedPoint is what `--check` rests on: merging over a freshly
// written file must change nothing, or every run would report drift against itself.
func TestRenderIsItsOwnFixedPoint(t *testing.T) {
	t.Parallel()

	regions := []schema.Region{
		{Name: "vocabulary", Body: "- Reference\n- Standard"},
		{Name: "commands", Body: "- gnosis fetch\n- gnosis lint"},
	}
	rendered := schema.Render("# AGENTS\n\nGenerated by gnosis.", regions)

	got, outcome := schema.Merge(rendered, regions)
	if outcome != schema.OutcomeMerged {
		t.Fatalf("outcome = %v, want merged", outcome)
	}
	if got != rendered {
		t.Errorf("merging over a rendered file changed it:\n%q\n%q", rendered, got)
	}
}

// TestTheZeroOutcomeAssertsNothing is the discipline every enum here follows. A merge
// nobody performed must not read as one that succeeded.
func TestTheZeroOutcomeAssertsNothing(t *testing.T) {
	t.Parallel()

	var zero schema.Outcome
	if zero != schema.OutcomeUnwritten {
		t.Errorf("the zero outcome is %v, not unwritten", zero)
	}
	if zero.Merged() {
		t.Error("the zero outcome reports a completed merge")
	}
	if zero.String() != "unwritten" {
		t.Errorf("the zero outcome renders as %q", zero.String())
	}
}

// TestEveryOutcomeRendersAWord: an agent branches on the word, so an outcome
// marshalling as "invalid" would be one no caller could act on.
func TestEveryOutcomeRendersAWord(t *testing.T) {
	t.Parallel()

	for outcome, want := range map[schema.Outcome]string{
		schema.OutcomeUnwritten: "unwritten",
		schema.OutcomeMerged:    "merged",
		schema.OutcomeUnmarked:  "unmarked",
		schema.OutcomeMalformed: "malformed",
	} {
		text, err := outcome.MarshalText()
		if err != nil {
			t.Fatalf("marshal %v: %v", outcome, err)
		}
		if string(text) != want {
			t.Errorf("marshalled as %q, want %q", text, want)
		}
	}
	if got := schema.Outcome(99).String(); got != "invalid" {
		t.Errorf("an undeclared outcome renders as %q", got)
	}
}

// TestOnlyTheMechanicalPartsAreGenerated is §5.7's restraint, asserted rather than
// trusted.
//
// gnosis generates the vocabulary and the command list and never the prose that tells
// an agent how to think — and the evidence for that is measured: an ETH Zurich study
// found auto-generated context files reduced agent success in five of eight settings.
// A third region explaining how to ingest would be exactly what that study measured.
func TestOnlyTheMechanicalPartsAreGenerated(t *testing.T) {
	t.Parallel()

	got := schema.SchemaRegions(
		[]schema.TypeEntry{{Key: "Reference", Desc: "a page of facts"}},
		[]schema.CommandEntry{{Name: "fetch", Help: "archive a source"}},
	)
	if len(got) != 2 {
		t.Fatalf("want two regions, got %d: %+v", len(got), got)
	}
	if got[0].Name != schema.RegionVocabulary || got[1].Name != schema.RegionCommands {
		t.Errorf("the regions are %q and %q", got[0].Name, got[1].Name)
	}
}

// TestBothListsAreSorted is what `--check` rests on: the type list comes from a file
// and the command list from a registration order, and a document that reordered itself
// between two runs over one corpus would report drift against itself.
func TestBothListsAreSorted(t *testing.T) {
	t.Parallel()

	got := schema.SchemaRegions(
		[]schema.TypeEntry{{Key: "Standard"}, {Key: "Reference"}, {Key: "Decision"}},
		[]schema.CommandEntry{{Name: "search"}, {Name: "admit"}, {Name: "fetch"}},
	)
	assertOrder(t, got[0].Body, []string{"Decision", "Reference", "Standard"})
	assertOrder(t, got[1].Body, []string{"admit", "fetch", "search"})
}

// TestADeprecatedTypeIsAnnouncedNotDropped is §5.8.1's soft-deprecation applied here.
//
// Announce, then enforce. A vocabulary listing that silently dropped a deprecated key
// would enforce without announcing: an author would find documents rejected for using a
// word the schema had quietly stopped mentioning.
func TestADeprecatedTypeIsAnnouncedNotDropped(t *testing.T) {
	t.Parallel()

	got := schema.SchemaRegions([]schema.TypeEntry{
		{Key: "Runbook", Desc: "steps to follow", Deprecated: "Procedure"},
	}, nil)
	body := got[0].Body
	if !strings.Contains(body, "Runbook") {
		t.Errorf("a deprecated type was dropped:\n%s", body)
	}
	// And it points at the replacement, because "deprecated" alone tells an author to
	// stop and not what to do instead.
	if !strings.Contains(body, "Procedure") {
		t.Errorf("the deprecation does not name its replacement:\n%s", body)
	}
}

// TestAnEmptyVocabularySaysSo keeps a bare heading with nothing under it from reading
// as a rendering bug rather than as an answer.
func TestAnEmptyVocabularySaysSo(t *testing.T) {
	t.Parallel()

	got := schema.SchemaRegions(nil, nil)
	if !strings.Contains(got[0].Body, "declares no types") {
		t.Errorf("an empty vocabulary renders as:\n%q", got[0].Body)
	}
	if strings.TrimSpace(got[1].Body) == "" {
		t.Error("an empty command list renders as nothing at all")
	}
}

// assertOrder checks that the wanted strings appear in the body in the given order.
func assertOrder(t *testing.T, body string, want []string) {
	t.Helper()

	at := 0
	for _, w := range want {
		found := strings.Index(body[at:], w)
		if found < 0 {
			t.Fatalf("%q is missing or out of order in:\n%s", w, body)
		}
		at += found + len(w)
	}
}
