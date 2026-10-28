package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// actorKey addresses the authenticated caller in a request context.
//
// An unexported empty struct type, which is the collision-proof form: context keys are
// compared by type *and* value, so a package that used the string "actor" would silently
// shadow or be shadowed by anything else that did. Zero allocation, and unreachable from
// outside this package.
type actorKey struct{}

// authenticate turns the reverse proxy's header into an actor on the request context.
//
// Requires: header names the request header the proxy sets, or is empty.
// Ensures: a request reaching a handler carries an actor, or carries the empty one when
// no header was configured.
//
// # Reverse-proxy auth is first-class, and the trust boundary is stated
//
// §13: authenticated, "with reverse-proxy auth supported as a first-class mode, so it
// drops behind existing SSO instead of owning credentials". The header is therefore
// **trusted**, and that is only sound behind a proxy that strips it from client
// requests. `serve`'s help says so in those words, because a server exposed directly
// with this enabled lets any caller name themselves.
//
// # The liveness probe is the one exemption, and it is exempt because it answers nothing
//
// An orchestrator probing `/healthz` cannot set a proxy header — it is not a signed-in
// person — so authenticating the probe marks a working server dead and takes it out of
// rotation. That is the failure `handleHealthz` was written to avoid one layer down, and
// putting authentication in front of it reintroduced it; a hand run found it, in the one
// configuration the tests had not covered.
//
// The exemption is safe because the handler touches no dependency and returns a
// constant: it reveals that a process is listening, which anybody who can reach the port
// already knows. Every other path stays authenticated, including ones added later, which
// is why this is a carve-out rather than per-route authentication.
//
// The actor is prefixed `human:` because §14.1's fold keys on that prefix and §10.6.4
// counts distinct human adjudicators. A proxy-authenticated identity is a person who
// signed in to something; an agent runtime calling through the same proxy would be
// claiming to be one, which is the case §4.6.2.1 wants distinguishable — and the
// distinction lives in who the proxy will authenticate, not here.
func authenticate(header string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if header == "" || r.URL.Path == healthzPath {
				next.ServeHTTP(w, r)
				return
			}
			name := strings.TrimSpace(r.Header.Get(header))
			if name == "" {
				encodeOutcome(w, gnosis.Blocked(gnosis.ReasonNeedsHuman,
					"this request carries no authenticated user in "+header+
						"; the proxy in front of this server sets it",
					map[string]any{"header": header}))
				return
			}
			actor, ok := gnosis.ParseActor(gnosis.KindHuman + ":" + name)
			if !ok {
				// A proxy that authenticated somebody whose name gnosis cannot
				// parse is a configuration problem, and saying which header and
				// which value is what makes it a five-minute one.
				encodeOutcome(w, gnosis.Blocked(gnosis.ReasonNeedsHuman,
					"the user named in "+header+" is not an actor gnosis can read: "+
						name, map[string]any{"header": header, "user": name}))
				return
			}
			next.ServeHTTP(w, r.WithContext(
				context.WithValue(r.Context(), actorKey{}, actor)))
		})
	}
}

// refuseMutations blocks every write when the server is read-only.
//
// **In middleware rather than in each handler**, so there is one place to be wrong about
// it. A per-handler check is a check somebody forgets on the handler they add next, and
// the failure is silent in the worst direction: a shared read-only instance that writes.
//
// The test is the HTTP method rather than the route, which is deliberately broader than
// the route table: a route added later that mutates will be a POST, and this refuses it
// without anybody having remembered to.
func refuseMutations(readOnly bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !readOnly || r.Method == http.MethodGet || r.Method == http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			encodeOutcome(w, gnosis.Blocked(gnosis.ReasonNeedsHuman,
				"this server is read-only; the decision has to be made where the"+
					" corpus can be written", map[string]any{"method": r.Method}))
		})
	}
}
