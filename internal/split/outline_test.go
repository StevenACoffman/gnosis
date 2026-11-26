package split_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/quick"

	"github.com/StevenACoffman/gnosis/internal/split"
)

// outlineLimit holds one sectionText body beside the frontmatter reserve but not two,
// and three bodies exceed it, so a three-section book is split and how its sections
// group into parts is decided by its headings alone.
const outlineLimit = 5000

// outlineHeading is one ATX heading of a generated book.
type outlineHeading struct {
	level int
	title string
	start int // byte offset of the heading line
}

// outlineBook is a generated Markdown book and the headings it was built from, so a
// property can be checked against the outline without parsing the Markdown again.
type outlineBook struct {
	src      string
	headings []outlineHeading
}

// sectionText is a section body of about 1,700 bytes.
func sectionText() string {
	return strings.Repeat(strings.Repeat("word ", 15)+"\n", 22) + "\n"
}

func TestAPartsTrailIsTheOutlineAboveIt(t *testing.T) {
	sec := sectionText()
	cases := map[string]struct {
		src    string
		trails []string // each part's trail, joined by "/"
	}{
		"front matter is a sibling of the chapters": {
			src: "# Preface\n\n" + sec +
				"# Chapter 1: Why\n\n" + sec +
				"# Chapter 2: How\n\n" + sec,
			trails: []string{"Preface", "Chapter 1: Why", "Chapter 2: How"},
		},
		"an opening heading that encloses the book is its title": {
			src: "# Book\n\nfront\n\n" +
				"## One\n\n" + sec +
				"## Two\n\n" + sec +
				"## Three\n\n" + sec,
			trails: []string{"Book", "Book/Two", "Book/Three"},
		},
		"a title set at the level of the chapters still encloses them": {
			src: "# Book\n\nfront\n\n" +
				"# One\n\n" + sec +
				"# Two\n\n" + sec +
				"# Three\n\n" + sec,
			trails: []string{"Book", "Book/Two", "Book/Three"},
		},
		"front matter above a shallower heading is its sibling": {
			src: "## Foreword\n\n" + sec +
				"# Part I\n\n" + sec +
				"# Part II\n\n" + sec,
			trails: []string{"Foreword", "Part I", "Part II"},
		},
		"front matter before the first chapter stays together": {
			src: "A preamble no heading opens.\n\n" +
				"#### Dedication\n\nFor someone.\n\n" +
				"#### Contents\n\nOne, Two.\n\n" +
				"# One\n\n" + sec +
				"# Two\n\n" + sec +
				"# Three\n\n" + sec,
			trails: []string{"", "One", "Two", "Three"},
		},
		"text before the first heading joins the first chapter": {
			src: "A preamble no heading opens.\n\n" +
				"# One\n\n" + sec +
				"# Two\n\n" + sec +
				"# Three\n\n" + sec,
			trails: []string{"", "Two", "Three"},
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
			var got []string
			for i := range plan.Parts {
				got = append(got, strings.Join(plan.Parts[i].Headings, "/"))
			}
			if strings.Join(got, " | ") != strings.Join(tc.trails, " | ") {
				t.Fatalf("trails = %q, want %q", got, tc.trails)
			}
			if len(plan.Suspects) != 0 {
				t.Errorf("suspects = %+v, want none", plan.Suspects)
			}
		})
	}
}

// Generate builds a book of up to 12 headings at levels 1 to 3, in any order, each
// with a body of up to about 1,500 bytes. It sometimes opens on a preamble no heading
// opens, and its first heading is sometimes front matter, "Preface".
func (outlineBook) Generate(r *rand.Rand, _ int) reflect.Value {
	var b strings.Builder
	var hs []outlineHeading
	if r.Intn(4) == 0 {
		b.WriteString("A preamble no heading opens.\n\n")
	}
	line := strings.Repeat("word ", 15) + "\n"
	for i := range 1 + r.Intn(12) {
		h := outlineHeading{level: 1 + r.Intn(3), title: "H" + strconv.Itoa(i), start: b.Len()}
		if i == 0 && r.Intn(3) == 0 {
			h.title = "Preface"
		}
		hs = append(hs, h)
		b.WriteString(strings.Repeat("#", h.level) + " " + h.title + "\n\n")
		b.WriteString(strings.Repeat(line, r.Intn(20)) + "\n")
	}
	return reflect.ValueOf(outlineBook{src: b.String(), headings: hs})
}

// ancestors returns, for each heading, the titles of the headings enclosing it
// followed by its own: each enclosing heading is the nearest earlier one at a
// shallower level. A book opening on a heading other than front matter opens on its
// title, which encloses every heading after it whatever its level.
func (bk outlineBook) ancestors() [][]string {
	out := make([][]string, len(bk.headings))
	var stack []outlineHeading
	floor := 0 // how much of the stack no later heading can close
	for i, h := range bk.headings {
		if i == 1 && bk.headings[0].start == 0 && bk.headings[0].title != "Preface" {
			floor = 1
		}
		for len(stack) > floor && stack[len(stack)-1].level >= h.level {
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, h)
		for _, a := range stack {
			out[i] = append(out[i], a.title)
		}
	}
	return out
}

// TestEveryPartSitsWhereTheOutlinePutsIt checks the two kinds of part a split makes:
//
//   - a part with a trail is one heading's section: it opens on that heading, every
//     later heading in it falls within that section, and its trail is the heading's
//     outline — the headings enclosing it, then its own;
//   - a part without one is front matter, sections with no common parent, and holds
//     no heading shallower than its first, which would open a section the empty
//     trail hides.
//
// MinBytes is left off: joining small sections puts siblings in one part on purpose.
func TestEveryPartSitsWhereTheOutlinePutsIt(t *testing.T) {
	property := func(bk outlineBook) bool {
		plan, err := split.Split(
			[]byte(bk.src),
			split.Options{Source: "book.md", Limit: outlineLimit},
		)
		if err != nil {
			t.Logf("Split: %v\n%s", err, bk.src)
			return false
		}
		outline := bk.ancestors()
		for i := range plan.Parts {
			if fault := bk.misplaced(&plan.Parts[i], outline); fault != "" {
				t.Logf("%s\n%s", fault, bk.src)
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 500}); err != nil {
		t.Fatal(err)
	}
}

// misplaced says how p breaks the outline, or returns "" when it does not.
func (bk outlineBook) misplaced(p *split.Part, outline [][]string) string {
	var in []int // indexes of the headings starting in p
	for j, h := range bk.headings {
		if h.start >= p.Start && h.start < p.End {
			in = append(in, j)
		}
	}
	if len(in) == 0 {
		return ""
	}
	section := len(p.Headings) > 0
	first := bk.headings[in[0]]
	for _, j := range in[1:] {
		h := bk.headings[j]
		within := slices.Contains(outline[j][:len(outline[j])-1], first.title)
		if section && !within || !section && h.level < first.level {
			return fmt.Sprintf("%s holds %s after %s, outside its section",
				p.Name, h.title, first.title)
		}
	}
	if section && (first.start != p.Start || !reflect.DeepEqual(p.Headings, outline[in[0]])) {
		return fmt.Sprintf("%s has trail %q, want %q, opening on %s",
			p.Name, p.Headings, outline[in[0]], first.title)
	}
	return ""
}
