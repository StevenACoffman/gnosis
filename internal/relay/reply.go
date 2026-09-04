package relay

import (
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/StevenACoffman/skillet/errs"
)

// Reply is what an agent sent back for one prompt.
//
// Parsed is carried rather than inferred from len(Claims) because an empty claim
// list is a legitimate answer — "this source supports nothing I can quote" is
// useful — and a zero Reply is not. Without the flag the two are the same value,
// and admit would treat a reply it never read as a source with nothing in it.
type Reply struct {
	Parsed bool    `json:"parsed"`
	Title  string  `json:"title"`
	Type   string  `json:"type"`
	Claims []Claim `json:"claims"`

	// SourceURI is the source this reply is about. It is **not** parsed from the
	// reply — it comes from the prompt the key identifies, and the caller sets it.
	// A model that could name its own source could cite one it never read.
	SourceURI string `json:"source_uri,omitempty"`
}

// Claim is one assertion and the quotations offered for it.
type Claim struct {
	Text   string   `json:"text"`
	Quotes []string `json:"quotes"`

	// Lead is the claim's conclusion, stated first, in its own words (§17.4).
	//
	// **Optional, and a reply omitting it is not refused.** §17.4 makes a lead a
	// *checked property*, and §5.8.3's argument one field over settles what that means
	// at admission: reporting is a review signal and refusing is a gate, and turning
	// one into the other would make the corpus decline knowledge over a summary. A
	// claim with no lead gets a NULL one, which is the state §5.5.3 already defines.
	//
	// It is authored rather than derived, and §17.4 records why deriving it is
	// refused: a rule that picked the conclusion clause would make the check testing
	// that rule vacuous, and a lead is the author's judgement about what the claim
	// ultimately asserts rather than the part of a sentence that survived a filter.
	Lead string `yaml:"lead" json:"lead,omitempty"`

	// ID is assigned by the caller, not by the model. An identifier a reply chose
	// could collide with one already in the corpus, or be reused across replies to
	// make two different claims look like one.
	ID string `json:"id,omitempty"`

	// ArchivePaths are the tier-0 files these quotations were checked against.
	// Also the caller's: letting a reply nominate its own archive would let it
	// choose a file its quotations happen to appear in, which is the check
	// answering to the thing it checks.
	ArchivePaths []string `json:"archive_paths,omitempty"`
}

// replyDoc mirrors the on-disk shape, so the decoded form and the validated form
// are different types and there is no way to obtain a Reply that was not checked.
type replyDoc struct {
	Title  string `yaml:"title"`
	Type   string `yaml:"type"`
	Claims []struct {
		Text   string   `yaml:"text"`
		Lead   string   `yaml:"lead"`
		Quotes []string `yaml:"quotes"`
	} `yaml:"claims"`
}

// ParseReply reads an agent's reply.
//
// Requires: src is the agent's whole response, which may carry prose around the
// fenced block.
// Ensures: EINVALID on anything that is not exactly one parsable yaml block with a
// title and a type. **The reply is rejected whole or accepted whole** — a partially
// applied reply would put content into quarantine that neither the agent nor the
// reader believes they approved, and quarantine is one gate away from the corpus.
//
// It is strict about the number of blocks as well as their content. Two yaml
// blocks means the agent answered twice or restated the format, and guessing which
// one is the answer is the sort of latitude that produces a corpus nobody can
// account for.
func ParseReply(src []byte) (Reply, error) {
	const op = "relay.ParseReply"

	block, err := oneBlock(op, string(src))
	if err != nil {
		return Reply{}, err
	}

	var doc replyDoc
	if err := yaml.Unmarshal([]byte(block), &doc); err != nil {
		return Reply{}, &errs.Error{Code: errs.EINVALID, Op: op, Err: err}
	}
	return validateReply(op, &doc)
}

// validateReply turns a decoded document into a Reply, or reports why it cannot.
func validateReply(op string, doc *replyDoc) (Reply, error) {
	var bad []string
	if strings.TrimSpace(doc.Title) == "" {
		bad = append(bad, "title is empty")
	}
	if strings.TrimSpace(doc.Type) == "" {
		bad = append(bad, "type is empty")
	}

	out := Reply{Parsed: true, Title: doc.Title, Type: doc.Type, Claims: []Claim{}}
	for i := range doc.Claims {
		c := &doc.Claims[i]
		if strings.TrimSpace(c.Text) == "" {
			bad = append(bad, "claim "+ordinal(i)+" has no text")
			continue
		}
		// A claim with no quotation is not silently dropped and not silently
		// admitted. It is reported here, because the alternative — letting the
		// evidence signal catch it two steps later — tells the agent about the
		// problem in a message about a document rather than about its reply.
		if len(c.Quotes) == 0 {
			bad = append(bad, "claim "+ordinal(i)+" offers no quotation")
			continue
		}
		out.Claims = append(out.Claims, Claim{Text: c.Text, Lead: c.Lead, Quotes: c.Quotes})
	}

	if len(bad) > 0 {
		return Reply{}, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": " + strings.Join(bad, "; "),
		}
	}
	return out, nil
}

// oneBlock is the reply-format policy both parsers hold: exactly one fenced yaml
// block, and a reply that is not that shape is rejected whole.
//
// Requires: op names the caller, for the message.
// Ensures: the block's contents, or EINVALID saying which way the reply was wrong.
// Pure.
//
// **Shared because the rule is shared, not because the scanner is.** Two parsers each
// deciding what a well-formed reply looks like is one place for them to disagree, and
// the disagreement would surface as a reply one accepts and the other refuses with
// nobody able to say which is right. The strictness is `ParseReply`'s and its reason is
// unchanged: two blocks means the agent answered twice or restated the format, and
// guessing which one is the answer is the latitude that produces a corpus nobody can
// account for.
func oneBlock(op, src string) (string, error) {
	blocks := yamlBlocks(src)
	switch {
	case len(blocks) == 0:
		return "", &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": no ```yaml block in the reply",
		}
	case len(blocks) > 1:
		return "", &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": the reply carries " +
				plural(len(blocks)) + "; exactly one is required",
		}
	}
	return blocks[0], nil
}

// yamlBlocks returns the contents of every ```yaml fence in src.
//
// It scans line by line rather than with a regular expression, because a fence
// inside the block would end it early under a non-greedy match and swallow the
// rest of the reply under a greedy one — and a reply about YAML is exactly the
// case where that happens.
func yamlBlocks(src string) []string {
	var (
		out     []string
		current []string
		inside  bool
	)
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case !inside && (trimmed == "```yaml" || trimmed == "```yml"):
			inside = true
			current = nil
		case inside && trimmed == "```":
			inside = false
			out = append(out, strings.Join(current, "\n"))
		case inside:
			current = append(current, line)
		}
	}
	// An unterminated fence is not treated as a block. Accepting it would mean
	// parsing whatever the reply happened to end with.
	return out
}

func plural(n int) string {
	if n == 1 {
		return "1 yaml block"
	}
	return strconv.Itoa(n) + " yaml blocks"
}

func ordinal(i int) string { return strconv.Itoa(i + 1) }
