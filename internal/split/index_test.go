package split_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/split"
)

// indexLink is one numbered entry of an index: its text and the file it links.
var indexLink = regexp.MustCompile(`(?m)^\d+\. \[((?:\\.|[^\]\\])*)\]\(([^)]+)\)$`)

func TestTheIndexLinksEveryPartInReadingOrder(t *testing.T) {
	sec := sectionText()
	cases := map[string]struct {
		src   string
		texts []string // each entry's link text, as written
	}{
		"chapters under a title": {
			src:   "# Book\n\nfront\n\n## One\n\n" + sec + "## Two\n\n" + sec + "## Three\n\n" + sec,
			texts: []string{"Book", "Two", "Three"},
		},
		"front matter has no heading of its own": {
			src:   "A preamble.\n\n#### Dedication\n\nFor someone.\n\n# One\n\n" + sec + "# Two\n\n" + sec + "# Three\n\n" + sec,
			texts: []string{"Front matter", "One", "Two", "Three"},
		},
		"brackets in a heading are escaped": {
			src:   "# Preface\n\n" + sec + "# [Draft] Notes\n\n" + sec + "# Back\\slash\n\n" + sec,
			texts: []string{"Preface", `\[Draft\] Notes`, `Back\\slash`},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			plan, err := split.Split(
				[]byte(tc.src),
				split.Options{Source: "book.md", Limit: outlineLimit},
			)
			if err != nil {
				t.Fatalf("Split: %v", err)
			}
			if plan.Index.Name != "book-00-index.md" {
				t.Errorf("index name = %q, want book-00-index.md", plan.Index.Name)
			}
			texts, targets := entries(string(plan.Index.Content))
			if names := partNames(plan); !slices.Equal(targets, names) {
				t.Errorf("index links %q, want every part in order %q", targets, names)
			}
			if !slices.Equal(texts, tc.texts) {
				t.Errorf("link texts = %q, want %q", texts, tc.texts)
			}
		})
	}
}

func partNames(plan *split.Plan) []string {
	names := make([]string, 0, len(plan.Parts))
	for i := range plan.Parts {
		names = append(names, plan.Parts[i].Name)
	}
	return names
}

// entries returns each numbered entry's link text, as written, and its target.
func entries(index string) (texts, targets []string) {
	for _, m := range indexLink.FindAllStringSubmatch(index, -1) {
		texts, targets = append(texts, m[1]), append(targets, m[2])
	}
	return texts, targets
}

func TestTheIndexReadsBackAsTheIndexOfItsSource(t *testing.T) {
	sec := sectionText()
	src := "# Book\n\n## One\n\n" + sec + "## Two\n\n" + sec + "## Three\n\n" + sec
	plan, err := split.Split([]byte(src), split.Options{Source: "book.md", Limit: outlineLimit})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	prov, ok := split.ReadProvenance(plan.Index.Content)
	if !ok || !prov.Index || prov.Source != "book.md" || prov.Version != split.Version {
		t.Errorf("index provenance = %+v, %v; want book.md's index", prov, ok)
	}
	if !strings.HasPrefix(string(plan.Index.Content), "---\n"+split.IndexKey+":") {
		t.Errorf("index does not open with %s:\n%s", split.IndexKey, plan.Index.Content)
	}
}

func TestASourceThatFitsHasNoIndex(t *testing.T) {
	plan, err := split.Split([]byte("# Notes\n\nShort.\n"),
		split.Options{Source: "notes.md", Limit: outlineLimit})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if plan.Index.Name != "" || plan.Index.Content != nil {
		t.Errorf("index = %+v, want none for a source with no parts", plan.Index)
	}
}
