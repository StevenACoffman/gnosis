package web

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// defaultSearchLimit bounds a query that names no limit.
//
// Ten, matching `search`'s own default, because §13 says the web search is "the same
// ladder as the CLI, same scoping flags" — a web result set of a different size would be
// a second answer to one question.
const defaultSearchLimit = 10

// errNoDependency is what a route answers when the shell did not supply what it needs.
//
// Reported per request rather than refused at startup, which is rules.md's advice for
// deferred setup and the right trade here: a viewer with no index should still serve the
// queue, and a server that refused to start would take both down for one missing half.
var errNoDependency = errors.New("this server was not given what this route needs")

// handleHealthz answers whether the process is alive.
//
// **Liveness and not readiness.** It touches no dependency: a health check that queried
// the corpus would fail while somebody held the writer lock, and a load balancer would
// take out a server that was working correctly and merely waiting.
func handleHealthz() http.Handler {
	type response struct {
		Status string `json:"status"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		encode(w, http.StatusOK, response{Status: "ok"})
	})
}

// handleQueue lists what is waiting for a person.
func handleQueue(queue Queue) http.Handler {
	type response struct {
		Items []Item `json:"items"`

		// Count is carried rather than left to the caller to take `len` of, because
		// §17 forbids a count presented as health and this is the one place the
		// number means something plain: how many decisions are outstanding.
		Count int `json:"count"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if queue == nil {
			encodeError(w, gnosis.ReasonNoBundle, errNoDependency)
			return
		}
		items, err := queue.Waiting(r.Context())
		if err != nil {
			encodeError(w, gnosis.ReasonNoBundle, err)
			return
		}
		encode(w, http.StatusOK, response{Items: items, Count: len(items)})
	})
}

// handleDecide runs one mutation through the write coordinator.
//
// # Everything this does not do is the point
//
// It does not decide anything itself: the command goes to the same `Executor` the CLI
// drives, so the gate, the audit row and the git commit are the ones that already exist
// — §13's "there is no web-only write path".
//
// It does not read an approver from the body. `Command` has no such field (§4.6.2.1),
// and the actor comes from the authenticated request. A body naming one is dropped by
// the decoder, which is a stronger guarantee than a check somebody can forget.
func handleDecide(executor Executor) http.Handler {
	type response struct {
		Outcome gnosis.Outcome `json:"outcome"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if executor == nil {
			encodeError(w, gnosis.ReasonNoBundle, errNoDependency)
			return
		}
		cmd, err := decode[Command](r)
		if err != nil {
			encodeError(w, gnosis.ReasonUsage, err)
			return
		}
		if problems := cmd.Valid(r.Context()); len(problems) > 0 {
			// Every problem at once, so a caller fixing one is not told about the
			// next on the following round trip — the rule the reply parsers follow.
			encode(w, http.StatusBadRequest, gnosis.BadUsage(
				"the decision is incomplete: "+joinProblems(problems)))
			return
		}
		if actorOf(r) == gnosis.ActorUnset {
			// Refused rather than executed with an empty approver. §9.5's rule that
			// an approval is not self-granted is unenforceable without knowing who
			// approved, and an unset actor here means the server was configured with
			// no authentication and somebody sent a write to it anyway.
			encodeOutcome(w, gnosis.Blocked(gnosis.ReasonNeedsHuman,
				"this server cannot say who is deciding, so it cannot record a"+
					" decision; run it behind an authenticating proxy", nil))
			return
		}
		outcome, err := executor.Execute(r.Context(), cmd)
		if err != nil {
			encodeError(w, gnosis.ReasonNoBundle, err)
			return
		}
		encode(w, statusFor(outcome), response{Outcome: outcome})
	})
}

// handleConcept renders one document as JSON.
func handleConcept(reader Reader) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reader == nil {
			encodeError(w, gnosis.ReasonNoBundle, errNoDependency)
			return
		}
		page, err := reader.Concept(r.Context(), r.PathValue("id"))
		if err != nil {
			// ENOTFOUND becomes 404 through httpStatus rather than through a check
			// here: a missing page is an answer and not a failure, and the mapping is
			// written once.
			encodeError(w, gnosis.ReasonNoBundle, err)
			return
		}
		encode(w, http.StatusOK, page)
	})
}

// handleSearch answers a query at document grain.
func handleSearch(reader Reader) http.Handler {
	type response struct {
		Query string `json:"query"`
		Hits  []Hit  `json:"hits"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reader == nil {
			encodeError(w, gnosis.ReasonNoBundle, errNoDependency)
			return
		}
		query := r.URL.Query().Get("q")
		if query == "" {
			encode(w, http.StatusBadRequest,
				gnosis.BadUsage("search needs a query in ?q="))
			return
		}
		limit, err := searchLimit(r.URL.Query().Get("limit"))
		if err != nil {
			encodeError(w, gnosis.ReasonUsage, err)
			return
		}
		hits, err := reader.Search(r.Context(), query, limit)
		if err != nil {
			encodeError(w, gnosis.ReasonNoBundle, err)
			return
		}
		encode(w, http.StatusOK, response{Query: query, Hits: hits})
	})
}

// searchLimit reads the bound a query asked for.
//
// Requires: raw is the query parameter, which may be empty.
// Ensures: defaultSearchLimit for an absent value, and EINVALID for one that is not a
// positive number — never a silent fallback, because a caller who typed `limit=ten` and
// got ten results by coincidence learns the wrong thing about the API. Pure.
func searchLimit(raw string) (int, error) {
	const op = "web.searchLimit"

	if raw == "" {
		return defaultSearchLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 {
		return 0, &errs.Error{
			Code: errs.EINVALID, Message: op + ": limit must be a positive number, got " + raw,
		}
	}
	return limit, nil
}

// joinProblems renders a validation map deterministically.
//
// Sorted by field, because map iteration is randomized and a message whose clauses moved
// between two identical requests is one a caller cannot test against.
func joinProblems(problems map[string]string) string {
	fields := make([]string, 0, len(problems))
	for field := range problems {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	var b strings.Builder
	for i, field := range fields {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(field)
		b.WriteString(": ")
		b.WriteString(problems[field])
	}
	return b.String()
}
