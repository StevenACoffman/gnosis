package split_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/split"
)

func TestReadProvenance(t *testing.T) {
	cases := map[string]struct {
		content string
		ok      bool
		want    split.Provenance
	}{
		"a part": {
			content: "---\ngnosis_split:\n  source: book.md\n  part: 2\n  splitter_version: 2\n---\n\n# Two\n",
			ok:      true,
			want:    split.Provenance{Source: "book.md", Version: 2},
		},
		"an index": {
			content: "---\ngnosis_split_index:\n  source: book.md\n  splitter_version: 2\n---\n\n# Parts\n",
			ok:      true,
			want:    split.Provenance{Source: "book.md", Version: 2, Index: true},
		},
		"hand-written":      {content: "# Notes\n"},
		"other frontmatter": {content: "---\ntitle: x\n---\n\n# X\n"},
		"malformed yaml":    {content: "---\ngnosis_split: [\n---\n"},
		"no source named":   {content: "---\ngnosis_split:\n  part: 1\n---\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := split.ReadProvenance([]byte(tc.content))
			if ok != tc.ok || got != tc.want {
				t.Errorf("ReadProvenance = %+v, %v; want %+v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// A part split writes must read back as split's own, or --replace could never
// recognise what it wrote.
func TestEveryPartReadsBackAsAPartOfItsSource(t *testing.T) {
	src := "# Book\n\n## One\n\n" + chapterText() + "## Two\n\n" + chapterText()
	plan, err := split.Split([]byte(src), split.Options{Source: "book.md", Limit: 5000})
	if err != nil {
		t.Fatal(err)
	}
	for i := range plan.Parts {
		got, ok := split.ReadProvenance(plan.Parts[i].Content)
		if !ok || got.Source != "book.md" || got.Version != split.Version {
			t.Errorf("%s reads back as %+v, %v", plan.Parts[i].Name, got, ok)
		}
	}
}
