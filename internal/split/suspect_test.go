package split_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/split"
)

// chapterText is prose long enough that two of them exceed the test limit, so the
// split must cut at whatever heading separates them.
func chapterText() string {
	return strings.Repeat(strings.Repeat("word ", 15)+"\n", 40) + "\n"
}

func TestCutHeadingsThatReadAsSomethingElseAreSuspect(t *testing.T) {
	ch := chapterText()
	cases := map[string]struct {
		src    string
		reason string // substring of the one expected suspect's reason
	}{
		"chart label as an H1": {
			src:    "# Book\n\n" + ch + "2013\n2014\n\n# of Sales Employees\n\n" + ch,
			reason: "chart label",
		},
		"terminal output above a line of dashes": {
			src:    "# Book\n\n" + ch + "$ go run ./client\n--------ADD-------added task: 1\n-------------------\n\n" + ch,
			reason: "setext heading spanning 2 lines",
		},
		"a shell command as a heading": {
			src:    "# Book\n\n" + ch + "## $ go test ./...\n\n" + ch,
			reason: "shell command",
		},
		"an assignment as a heading": {
			src:    "# Book\n\n" + ch + "## xgb = XGBClassifier(subsample=0.63)\n\n" + ch,
			reason: "code",
		},
		"a chapter nested under the preface": {
			src:    "# Preface\n\n" + ch + "## Chapter 1: Why\n\n" + ch + "## Chapter 2: How\n\n" + ch,
			reason: `nested under "Preface"`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			plan, err := split.Split([]byte(tc.src), split.Options{Source: "book.md", Limit: 5000})
			if err != nil {
				t.Fatalf("Split: %v", err)
			}
			if len(plan.Suspects) != 1 || !strings.Contains(plan.Suspects[0].Reason, tc.reason) {
				t.Fatalf("suspects = %+v, want one whose reason has %q", plan.Suspects, tc.reason)
			}
		})
	}
}

func TestRealChapterHeadingsAreNotSuspect(t *testing.T) {
	ch := chapterText()
	cases := map[string]string{
		"atx chapters":               "# Book\n\n## Chapter 1: Why\n\n" + ch + "## Chapter 2: How\n\n" + ch,
		"one-line setext":            "Book\n====\n\nOne\n---\n\n" + ch + "Two\n---\n\n" + ch,
		"chapters under book":        "# Designing Things\n\n" + ch + "## Chapter 1: Why\n\n" + ch,
		"lower-case front matter":    "# Book\n\n" + ch + "## preface\n\n" + ch,
		"a name starting lower-case": "# Book\n\n" + ch + "## gRPC on Cloud Run\n\n" + ch,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			plan, err := split.Split([]byte(src), split.Options{Source: "book.md", Limit: 5000})
			if err != nil {
				t.Fatalf("Split: %v", err)
			}
			if len(plan.Parts) < 2 || len(plan.Suspects) != 0 {
				t.Fatalf(
					"parts = %d, suspects = %+v; want a split with none",
					len(plan.Parts),
					plan.Suspects,
				)
			}
		})
	}
}
