package web

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// templateFS carries the pages into the binary.
//
// §13: "Embedded assets; stdlib net/http and html/template. No SPA build in the release
// path." A knowledge base a team cannot operate is one they stop using, and a release
// that needs a Node toolchain to produce a stylesheet is one more thing to operate.
//
//go:embed templates/*.html
var templateFS embed.FS

// page renders one HTML page.
//
// Requires: name is a template file under templates/.
// Ensures: a parsed set carrying the layout, the stylesheet and that page's `body`, or
// the parse error.
//
// **Each page is its own set**, because every page file defines `body` and one set could
// hold only one of them. Parsing per page also means a malformed page breaks that page
// rather than the server.
func page(name string) (*template.Template, error) {
	const op = "web.page"

	tmpl, err := template.ParseFS(templateFS,
		"templates/layout.html", "templates/style.html", "templates/"+name)
	if err != nil {
		// Wrapped so the page that failed is in the message. A parse failure here is
		// a build defect rather than a runtime one, and the fastest fix is knowing
		// which of five files it is.
		return nil, &errs.Error{Op: op, Message: op + ": " + name, Err: err}
	}
	return tmpl, nil
}

// renderPage writes a parsed page, or reports the setup failure that stopped it.
//
// The parse error is checked **per request** rather than at startup, which is rules.md's
// deferred-setup rule: a template that failed to parse would otherwise surface once, in
// whichever request happened to be first, and look like a transient fault.
func renderPage(w http.ResponseWriter, tmpl *template.Template, err error, data any) {
	if err != nil {
		encodeError(w, gnosis.ReasonNoBundle, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Rendered straight to the response rather than buffered first. A template that
	// fails mid-render leaves a truncated page, which is visible; buffering would cost
	// a copy of every page to turn that into a 500 nobody can act on either.
	_ = tmpl.ExecuteTemplate(w, "layout", data)
}

// handleIndex is the front page: what this server is for, and the way to the queue.
func handleIndex() http.Handler {
	type data struct{ Title string }
	tmpl, err := page("index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		renderPage(w, tmpl, err, data{Title: "gnosis"})
	})
}

// handleQueuePage renders the review queue §13 asks for.
//
// **The page shows the evidence beside the decision**, which is the section's own
// argument: "if the queue shows enough, a non-expert correctly recognizes when to defer;
// if it shows too little, even an expert guesses". Every field the template reads is one
// §13 names.
func handleQueuePage(queue Queue) http.Handler {
	type data struct {
		Title string
		Items []Item
	}
	tmpl, err := page("queue.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if queue == nil {
			encodeError(w, gnosis.ReasonNoBundle, errNoDependency)
			return
		}
		items, qErr := queue.Waiting(r.Context())
		if qErr != nil {
			encodeError(w, gnosis.ReasonNoBundle, qErr)
			return
		}
		renderPage(w, tmpl, err, data{Title: "Review queue", Items: items})
	})
}

// handleConceptPage renders one concept for a reader.
//
// **The body is escaped and not rendered as markdown**, which is a deliberate first cut
// rather than an oversight: the corpus is model-written (§13), so its body is the least
// trustworthy content this server handles, and `html/template` escaping it is what makes
// the page safe with no allow-list to maintain. A markdown renderer is a second pass and
// arrives with its own sanitiser.
func handleConceptPage(reader Reader) http.Handler {
	type data struct {
		Title string
		Page  *Page
	}
	tmpl, err := page("concept.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reader == nil {
			encodeError(w, gnosis.ReasonNoBundle, errNoDependency)
			return
		}
		concept, rErr := reader.Concept(r.Context(), r.PathValue("id"))
		if rErr != nil {
			encodeError(w, gnosis.ReasonNoBundle, rErr)
			return
		}
		renderPage(w, tmpl, err, data{Title: concept.Title, Page: concept})
	})
}
