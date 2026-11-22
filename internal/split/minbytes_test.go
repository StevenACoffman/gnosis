package split_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/split"
)

// flatBook is a book whose conversion put every section at one level: a few real
// chapters among many tiny sections, as Terraform: Up & Running came out.
func flatBook() string {
	var b strings.Builder
	b.WriteString("# Book\n\n")
	for _, title := range []string{"Preface", "Conventions", "Contact"} {
		b.WriteString("## " + title + "\n\nA line or two.\n\n")
	}
	b.WriteString("## Chapter 1\n\n" + chapterText())
	b.WriteString("## Conclusion\n\nShort.\n\n")
	b.WriteString("## Chapter 2\n\n" + chapterText())
	b.WriteString("## Conclusion\n\nShort.\n\n")
	return b.String()
}

func TestMinBytesJoinsSmallSectionsWithoutPassingTheLimit(t *testing.T) {
	const limit = 6000 // room for one chapter and its small neighbours, not two chapters
	cases := map[string]struct {
		min       int
		wantParts int
	}{
		"off keeps every section": {min: 0, wantParts: 7},
		"joins the tiny sections": {min: 1000, wantParts: 2},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			plan, err := split.Split([]byte(flatBook()), split.Options{
				Source: "book.md", Limit: limit, MinBytes: tc.min,
			})
			if err != nil {
				t.Fatalf("Split: %v", err)
			}
			if len(plan.Parts) != tc.wantParts {
				t.Fatalf("parts = %d, want %d", len(plan.Parts), tc.wantParts)
			}
			checkJoinedParts(t, plan, limit, tc.min > 0)
		})
	}
}

// checkJoinedParts asserts every part is under the limit and records the joining
// threshold exactly when one was set.
func checkJoinedParts(t *testing.T, plan *split.Plan, limit int, wantRecorded bool) {
	t.Helper()
	for i := range plan.Parts {
		p := &plan.Parts[i]
		if len(p.Content) > limit {
			t.Errorf("%s is %d bytes, over the limit", p.Name, len(p.Content))
		}
		if got := strings.Contains(string(p.Content), "min_part_bytes:"); got != wantRecorded {
			t.Errorf("%s records min_part_bytes = %v, want %v", p.Name, got, wantRecorded)
		}
	}
}
