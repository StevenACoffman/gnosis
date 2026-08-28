package bundle

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gate"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/okf"
	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/textnorm"
)

// quoteSet is a claim's evidence after a merge.
type quoteSet struct {
	quotes   []string
	archives []string
}

// Accreted is what appending a reply's evidence to an existing concept would do.
//
// It is a plan rather than a result: computing it writes nothing, so a caller can
// preview and apply the same value without the two paths being able to disagree.
type Accreted struct {
	// Path is the document, bundle-relative.
	Path string `json:"path"`

	// Content is the document to write. Empty when nothing was added.
	Content []byte `json:"-"`

	// Added is how many quotations were appended.
	Added int `json:"added"`

	// Unmatched are the reply's claims that match no claim already in the document.
	//
	// **They are reported and never added**, which is the whole difference between
	// accretion and synthesis. §6.3 says accretion appends evidence with "no body
	// rewrite"; a claim the document does not already make has no paragraph, and
	// adding one would change the body — which is `synthesize`'s gated operation
	// wearing the cheaper name.
	Unmatched []string `json:"unmatched,omitempty"`
}

// Accrete appends a checked reply's quotations to the claims a document already makes.
//
// Requires: existing is a parsed OKF concept; the reply's claims have been quote-checked
// against the archive the prompt was built from.
// Ensures: the returned body is byte-identical to the one that came in, or an error.
// A quotation already present on a claim is not added twice. Pure — no I/O, no clock.
//
// §6.3: "`gnosis ingest` appends `gnosis_evidence` entries and updates `sources`
// mechanically. No model, no body rewrite. This alone keeps the corpus current on
// facts." The body invariant is checked here rather than promised, because a promise
// that accretion did not rewrite a body is exactly the kind of claim §9.4 refuses to
// take on trust elsewhere.
func Accrete(existing *okf.Document, id gnosis.ID, reply *relay.Reply) (*Accreted, error) {
	const op = "bundle.Accrete"

	current := claimsOf(existing)
	out := &Accreted{}
	byAnchor := map[string]int{}
	for i := range current {
		byAnchor[textnorm.Fold(current[i].Text)] = i
	}

	for i := range reply.Claims {
		rc := &reply.Claims[i]
		at, ok := byAnchor[textnorm.Fold(rc.Text)]
		if !ok {
			out.Unmatched = append(out.Unmatched, rc.Text)
			continue
		}
		added, paths := mergeQuotes(&current[at], rc)
		out.Added += added
		current[at].Quotes = paths.quotes
		current[at].ArchivePaths = paths.archives
	}
	if out.Added == 0 {
		return out, nil
	}

	title, _ := existing.Text("title")
	doc := conceptDoc{
		Type: existing.Type(), Title: title, ID: id,
		SourceURI:  mergedSources(existing, reply.SourceURI),
		Claims:     make([]conceptClaim, 0, len(current)),
		Paragraphs: paragraphsOf(existing.Body),
	}
	for i := range current {
		doc.Claims = append(doc.Claims, conceptClaim{
			ID: current[i].ID, Anchor: current[i].Text, Lead: current[i].Lead,
			Quotes: current[i].Quotes, ArchivePaths: current[i].ArchivePaths,
		})
	}
	content := renderConcept(&doc)

	rendered, err := okf.Parse(content)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	if rendered.Body != existing.Body {
		// The invariant, checked rather than asserted. Reaching this means the
		// rendering changed a body while claiming only to add evidence, and writing
		// it would be a silent rewrite.
		return nil, &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": accretion would change the document body, which is " +
				"synthesis rather than accretion (§6.3); nothing was written",
		}
	}
	out.Content = content
	return out, nil
}

// mergeQuotes adds a reply claim's quotations to an existing claim's, without
// duplicating one already there.
//
// Requires: nothing.
// Ensures: order is existing-then-new, so a document's history reads down the page.
// Comparison is under textnorm.Fold, matching the evidence invariant: a quotation
// re-offered with different whitespace is the same quotation. Pure.
func mergeQuotes(existing *gate.Claim, incoming *relay.Claim) (int, quoteSet) {
	seen := map[string]bool{}
	for _, q := range existing.Quotes {
		seen[textnorm.Fold(q)] = true
	}
	out := quoteSet{quotes: existing.Quotes, archives: existing.ArchivePaths}
	added := 0
	for _, q := range incoming.Quotes {
		if seen[textnorm.Fold(q)] {
			continue
		}
		seen[textnorm.Fold(q)] = true
		out.quotes = append(out.quotes, q)
		added++
	}
	for _, p := range incoming.ArchivePaths {
		if !contains(out.archives, p) {
			out.archives = append(out.archives, p)
		}
	}
	return added, out
}

// mergedSources returns the document's sources with the reply's added if it is new.
func mergedSources(existing *okf.Document, uri string) []string {
	out := existingSources(existing)
	if uri != "" && !contains(out, uri) {
		out = append(out, uri)
	}
	return out
}

// existingSources reads the `sources[].resource` list a document already declares.
func existingSources(doc *okf.Document) []string {
	raw, ok := doc.Fields[sourcesKey].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if r := stringOr(m, "resource"); r != "" {
			out = append(out, r)
		}
	}
	return out
}

// paragraphsOf splits a rendered body back into the paragraphs renderConcept joins.
//
// Requires: body came from a document renderConcept wrote.
// Ensures: re-rendering the result reproduces the body. The leading `# Title` heading
// is dropped, because renderConcept writes it from the title rather than from the
// paragraph list. Pure.
func paragraphsOf(body string) []string {
	parts := strings.Split(strings.TrimSpace(body), "\n\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasPrefix(p, "# ") {
			continue
		}
		out = append(out, p)
	}
	return out
}

// contains reports whether s holds v.
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
