package scan

import (
	"regexp"
	"sort"
)

// The two hidden channels Markdown offers. Written as patterns rather than parsed with
// goldmark because the question is narrower than a parse: not "what is this document"
// but "which bytes does no reader see", and a comment the parser would treat as an HTML
// block or as inline HTML hides its text either way.
const (
	// htmlCommentPattern captures an HTML comment's body, across lines.
	htmlCommentPattern = `(?s)<!--(.*?)-->`

	// linkLabelPattern captures the title of a link-label definition on its own line:
	// `[label]: destination "title"`, with the title quoted or parenthesised. The
	// `[//]: # (text)` idiom for a Markdown comment is this form, with "#" as the
	// destination, and its text is the title.
	linkLabelPattern = `(?m)^[ ]{0,3}\[[^\]\n]+\]:[ \t]*\S+[ \t]+(?:"([^"\n]*)"|'([^'\n]*)'|\(([^)\n]*)\))[ \t]*$`
)

// Channel is text a Markdown renderer never shows: the inside of an HTML comment, or
// the title of a link-label definition. Offset is the byte offset of Text in the
// scanned text.
type Channel struct {
	Offset int
	Text   string
}

// HiddenChannels returns the text of every HTML comment and link-label definition title
// in text, in document order.
//
// Requires: nothing.
// Ensures:  every Channel's Text is text[Offset:Offset+len(Text)]; pure.
//
// It exists so a rule can look for concealment wording where concealment is the only
// reading. "Do not tell the user" in prose is a sentence about a feature; the same words
// in a comment are an instruction addressed to whatever reads the source, written where
// no reviewer will see it.
func HiddenChannels(text string) []Channel {
	var out []Channel
	for _, pattern := range []string{htmlCommentPattern, linkLabelPattern} {
		re := regexp.MustCompile(pattern)
		for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
			// The first non-empty capture group is the channel's text; the patterns
			// have one group (comments) or three alternatives (titles).
			for g := 2; g+1 < len(m); g += 2 {
				if m[g] >= 0 {
					out = append(out, Channel{Offset: m[g], Text: text[m[g]:m[g+1]]})
					break
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Offset < out[j].Offset })
	return out
}
