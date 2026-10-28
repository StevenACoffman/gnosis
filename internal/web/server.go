package web

import (
	"context"
	"net/http"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// Deps is everything the server needs, supplied by the shell.
//
// A struct rather than a parameter list, because it is passed to `addRoutes` and to
// every handler constructor: a five-argument call repeated at fifteen registration sites
// is fifteen places to get the order wrong, and adding a dependency would touch all of
// them.
//
// **Any of them may be nil in a test.** rules.md says to pass nil for a dependency a
// test does not exercise, and the routes that would use it are the ones that test does
// not call.
type Deps struct {
	Queue    Queue
	Reader   Reader
	Executor Executor

	// AuthHeader names the request header the reverse proxy sets to the authenticated
	// user. Empty means no authentication, which NewServer permits only in read-only
	// mode — see `authenticate`.
	AuthHeader string

	// ReadOnly refuses every mutation. §13 asks for it "for a shared instance", and it
	// is enforced in middleware rather than per handler so there is one place to be
	// wrong about it.
	ReadOnly bool
}

// NewServer builds the handler §13 serves.
//
// Requires: d.Queue, d.Reader and d.Executor are supplied for the surfaces the caller
// intends to serve; a nil one makes its routes fail rather than panic at startup.
// Ensures: an `http.Handler` — never a named type, because nothing outside this package
// needs to know what it is.
//
// Global middleware is applied here and per-route middleware at the registration site,
// which is rules.md's split and worth keeping: a reader asking "what runs before this
// handler" looks in two places rather than n.
func NewServer(d Deps) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, d)

	var handler http.Handler = mux
	// Authentication is outermost so that no route — including one added later by
	// somebody who did not read this comment — is reachable unauthenticated. The
	// read-only refusal sits inside it, because refusing a mutation requires knowing
	// there is a caller to refuse.
	handler = refuseMutations(d.ReadOnly)(handler)
	handler = authenticate(d.AuthHeader)(handler)
	return handler
}

// ActorOf is the authenticated caller on a request context, or the empty actor when
// there is none.
//
// **Exported because the shell needs it and cannot have the key.** The context key is
// unexported, which is what makes it collision-proof; the executor that turns a web
// command into a coordinator command lives outside this package and has to read the
// actor to set the approver (§4.6.2.1). An accessor is the only way both can be true.
//
// It takes a context rather than a request so that the one caller outside this package —
// which holds neither — is not forced to carry one.
func ActorOf(ctx context.Context) gnosis.Actor {
	actor, _ := ctx.Value(actorKey{}).(gnosis.Actor)
	return actor
}

// actorOf is ActorOf for a handler, which always has the request.
func actorOf(r *http.Request) gnosis.Actor { return ActorOf(r.Context()) }
