package web

import "net/http"

// healthzPath is the liveness probe's path, named once because two places must agree
// about it: the route below and the authentication exemption in `middleware.go`. A
// literal in both is how the exemption comes to cover a path that no longer exists.
const healthzPath = "/healthz"

// addRoutes registers every route, and never returns an error.
//
// **Fallible setup does not belong here** (rules.md §7): a route table that could fail
// would make "the server started" mean something different from "the server can answer",
// and the caller resolves those failures before calling this.
//
// Every `mux.Handle` call in the package is in this function, so the question "what does
// this server expose" has exactly one answer to read. That matters more here than in an
// ordinary service: §13 forbids a prose-editing route, and an absence is only checkable
// when the presences are in one list.
func addRoutes(mux *http.ServeMux, d Deps) {
	mux.Handle("GET "+healthzPath, handleHealthz())
	mux.Handle("GET /api/queue", handleQueue(d.Queue))
	mux.Handle("POST /api/decide", handleDecide(d.Executor))
	mux.Handle("GET /api/concepts/{id}", handleConcept(d.Reader))
	mux.Handle("GET /api/search", handleSearch(d.Reader))

	// `{$}` matches the root path **exactly**. A bare "GET /" would match every
	// unmatched GET, so the index page would answer for every mistyped URL and the
	// NotFound handler below would only ever see other methods.
	mux.Handle("GET /{$}", handleIndex())
	mux.Handle("GET /queue", handleQueuePage(d.Queue))
	mux.Handle("GET /c/{id}", handleConceptPage(d.Reader))

	// Explicit rather than relying on the mux's default, which is the same handler and
	// says nothing. A reader of this list can see that an unmatched path is a 404 by
	// decision.
	mux.Handle("/", http.NotFoundHandler())
}
