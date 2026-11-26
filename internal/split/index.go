package split

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

// IndexKey opens the frontmatter of a book's index. canonizer's distill skips any
// file that begins with it, because the index lists the parts and holds none of
// the book; SPEC §4.3.3 is where that contract is written down.
const IndexKey = "gnosis_split_index"

// Index is the book's entry point: a file beside its parts that links each of them
// in reading order. It is not a part — no stretch of the source is in it — so it is
// kept out of Parts and out of the verbatim, tiling and rendering checks.
type Index struct {
	Name    string
	Content []byte
}

type indexFrontmatter struct {
	Index indexRecord `yaml:"gnosis_split_index"`
}

type indexRecord struct {
	Source  string `yaml:"source"`
	SHA256  string `yaml:"source_sha256"`
	Parts   int    `yaml:"parts"`
	Version int    `yaml:"splitter_version"`
}

// buildIndex renders the index of parts.
//
// Requires: parts are rendered, in source order, and named with width digits.
// Ensures: the index is named <stem>-<width zeros>-index.md, so it sorts ahead of
// part 1; it records the source, its digest and the part count; and it links every
// part once, in order, by the part's own heading — "Front matter" for a part that
// has none. Pure.
func buildIndex(stem string, width int, sum string, parts []Part, opt Options) (Index, error) {
	name := fmt.Sprintf("%s-%0*d-index.md", stem, width, 0)
	fm, err := yaml.Marshal(indexFrontmatter{Index: indexRecord{
		Source: opt.Source, SHA256: sum, Parts: len(parts), Version: Version,
	}})
	if err != nil {
		return Index{}, fmt.Errorf("render %s: %w", name, err)
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fm)
	buf.WriteString("---\n\n# Parts of " + opt.Source + "\n\n")
	for i := range parts {
		buf.WriteString(strconv.Itoa(i+1) + ". " + navLink(&parts[i]) + "\n")
	}
	return Index{Name: name, Content: buf.Bytes()}, nil
}

// linkText escapes the characters that would end or break a link's text, so a
// heading such as "[Draft] Notes" still reads as one link.
func linkText(s string) string {
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`).Replace(s)
}
