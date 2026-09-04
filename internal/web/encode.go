// Package web serves SPEC §13's viewer and review queue, and §4.6's write coordinator,
// in one process.
//
// # It cannot reach the corpus, and that is the design
//
// depguard forbids this package from importing `internal/bundle`, so a handler has no
// way to name the corpus. Everything it needs arrives through the three interfaces in
// `deps.go`, supplied by the shell. The payoff is not layering purity: the whole server
// is exercised against fakes with no git worktree and no SQLite, and a handler **cannot**
// write the corpus behind the coordinator's back, because there is no symbol for it.
//
// # One protocol, two listeners
//
// §4.6.2.2: `gnosis serve` carries the coordinator and the viewer in one process, because
// two servers would be two authorities over one bundle. `net/http` serves any
// `net.Listener`, so the socket and the TCP port are two listeners over one handler
// rather than two servers.
package web

import (
	"encoding/json"
	"net/http"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// encode writes one JSON value with its status.
//
// Requires: nothing has been written to w yet, because WriteHeader is called here.
// Ensures: the content type is set before the status, which is the order net/http
// requires — a header set after WriteHeader is silently discarded.
//
// Central rather than a `json.NewEncoder` in each handler: the header, the status order
// and the error handling are one decision, and a handler that encoded inline would be a
// handler free to get one of them wrong.
func encode[T any](w http.ResponseWriter, status int, v T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// The error is deliberately dropped. It means the client went away mid-write, the
	// status line is already sent, and there is no second response to send — logging it
	// on every cancelled request would be noise about the network rather than about
	// this server.
	_ = json.NewEncoder(w).Encode(v)
}

// decode reads one JSON request body.
//
// Requires: r.Body is not nil, which net/http guarantees for a server request.
// Ensures: EINVALID naming the failure, so the caller is told the body was wrong rather
// than that the server broke.
func decode[T any](r *http.Request) (T, error) {
	const op = "web.decode"

	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, &errs.Error{Code: errs.EINVALID, Op: op, Err: err}
	}
	return v, nil
}

// encodeOutcome writes a gnosis.Outcome as §8.0's envelope.
//
// **The same envelope the CLI writes, and that is a requirement rather than a
// convenience.** §8.0: "one verdict, two renderings — never two verdicts". A caller that
// already parses `gnosis --jsonl` parses this, and a served verdict that differed from a
// terminal one would be a verdict about the transport rather than about the corpus.
func encodeOutcome(w http.ResponseWriter, outcome gnosis.Outcome) {
	encode(w, statusFor(outcome), outcome)
}

// encodeError writes an error as §8.0's envelope, choosing the status from its code.
func encodeError(w http.ResponseWriter, reason string, err error) {
	code := errs.ErrorCode(err)
	outcome := gnosis.Failure(reason, err.Error())
	if code == errs.EINVALID {
		// §8.0 separates the two failure codes by what repairs them: code 2 means
		// "call me differently" and code 1 means "something is wrong that changing the
		// arguments will not fix". A malformed body is the first.
		outcome = gnosis.BadUsage(err.Error())
	}
	encode(w, httpStatus(code), outcome)
}

// statusFor maps §8.0's outcome status onto an HTTP status.
//
// Requires: outcome came from a command execution.
// Ensures: 200 for ok, 422 for findings, 409 for blocked, 500 otherwise. Pure.
//
// **`findings` is 422 and not 400.** A corpus with problems is not a malformed request:
// §8.0 spends a paragraph on exactly this distinction — "code 3 means gnosis worked and
// the corpus did not" — and a client that could not tell those apart would retry the
// same request. 409 for `blocked`, because a person must act before it can succeed and
// nothing about the request would change that.
func statusFor(outcome gnosis.Outcome) int {
	switch outcome.Status {
	case gnosis.StatusOK:
		return http.StatusOK
	case gnosis.StatusFindings:
		return http.StatusUnprocessableEntity
	case gnosis.StatusBlocked:
		return http.StatusConflict
	case gnosis.StatusError:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// httpStatus maps an errs code onto an HTTP status.
//
// Written once, here, for the reason rules.md gives for a single error-translation
// function: a mapping repeated per handler is a mapping that drifts, and the drift shows
// up as one endpoint calling a missing document 404 and another calling it 500.
func httpStatus(code string) int {
	switch code {
	case errs.ENOTFOUND:
		return http.StatusNotFound
	case errs.EINVALID:
		return http.StatusBadRequest
	case errs.EUNAUTHORIZED:
		return http.StatusUnauthorized
	case errs.ECONFLICT:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
