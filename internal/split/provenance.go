package split

import (
	"github.com/goccy/go-yaml"

	"github.com/StevenACoffman/skillet/frontmatter"
)

// Provenance is what a part's frontmatter says about where it came from: the
// source it was cut from and the splitter version that cut it.
//
// It is the reading half of the block render writes, and the only other place the
// block's shape is known. A caller deciding whether a file on disk is a part of some
// source asks this rather than matching the frontmatter's text, so the format can
// change in one package.
type Provenance struct {
	Source  string
	Version int
}

// ReadProvenance reads the gnosis_split block at the top of content.
//
// Requires: nothing; content may be any file.
// Ensures: ok is true exactly when content begins with frontmatter holding a
// gnosis_split block that names a source. A hand-written file, a file with other
// frontmatter, and malformed YAML are all "not a part", never an error: the
// question asked is whether split owns this file, and for all three it does not.
func ReadProvenance(content []byte) (Provenance, bool) {
	block, _ := frontmatter.Split(string(content))
	if block == "" {
		return Provenance{}, false
	}
	var rec provenance
	if err := yaml.Unmarshal([]byte(block), &rec); err != nil || rec.Split.Source == "" {
		return Provenance{}, false
	}
	return Provenance{Source: rec.Split.Source, Version: rec.Split.Version}, true
}
