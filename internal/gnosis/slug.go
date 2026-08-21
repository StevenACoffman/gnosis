package gnosis

import "strings"

// Slug is the advisory, human-readable half of a filename. The identifier
// prefix is authoritative; the slug exists so a diff and a pull request are
// readable, and it is rewritten whenever the title changes.
type Slug string

// SlugFrom derives a slug from a title.
//
// Requires: nothing; an empty or punctuation-only title is acceptable input.
// Ensures: the result contains only lowercase letters, digits, and single
// hyphens, has no leading or trailing hyphen, and is "untitled" when the title
// yields no usable characters. SlugFrom is idempotent.
func SlugFrom(title string) Slug {
	var b strings.Builder
	lastHyphen := true // leading hyphens are suppressed
	for _, r := range strings.ToLower(title) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	s := strings.TrimSuffix(b.String(), "-")
	if s == "" {
		return "untitled"
	}
	return Slug(s)
}

// String renders the slug.
func (s Slug) String() string { return string(s) }
