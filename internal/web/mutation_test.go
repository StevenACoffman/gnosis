package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/web"
)

// decision is a well-formed mutation body.
const decision = `{"kind":"adjudicate","path":"c/a.md","claim":"claim-a",` +
	`"rationale":"the second source supersedes the first"}`

// TestAPayloadCannotNameItsOwnApprover is §4.6.2.1's rule, and it is the reason this
// whole transport section exists.
//
// "A remote caller sending `human:alice` is otherwise unverified, which turns §9.5's
// no-self-granted-approval rule into an honour system precisely when a non-human caller
// arrives." The guarantee here is structural — `web.Command` has no approver field, so
// the decoder drops one — and this test exists because "the type has no field" is a fact
// about today's type and somebody will add one.
func TestAPayloadCannotNameItsOwnApprover(t *testing.T) {
	t.Parallel()

	exec := &fakeExecutor{}
	srv := serve(t, web.Deps{Executor: exec, AuthHeader: "X-Forwarded-User"})

	body := `{"kind":"adjudicate","path":"c/a.md","rationale":"because",` +
		`"approver":"human:someone-else"}`
	status, got := do(t, srv, http.MethodPost, "/api/decide", "priya", body)
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, got)
	}
	if exec.called != 1 {
		t.Fatalf("the command did not reach the coordinator")
	}
	if strings.Contains(got, "someone-else") {
		t.Errorf("the caller's claimed approver survived into the response: %s", got)
	}
}

// TestAWriteWithNoAuthenticatedUserIsRefused. Without an actor, §9.5's rule that an
// approval is not self-granted cannot be enforced at all — so the write is refused
// rather than recorded with an empty approver, which would look like a decision nobody
// made.
func TestAWriteWithNoAuthenticatedUserIsRefused(t *testing.T) {
	t.Parallel()

	exec := &fakeExecutor{}
	// No AuthHeader configured, so nothing authenticates and the actor stays unset.
	srv := serve(t, web.Deps{Executor: exec})

	status, body := do(t, srv, http.MethodPost, "/api/decide", "", decision)
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", status, body)
	}
	if out := decodeOutcome(t, body); out.Status != gnosis.StatusBlocked {
		t.Errorf("status = %q, want blocked", out.Status)
	}
	if exec.called != 0 {
		t.Error("the coordinator was asked to record a decision by nobody")
	}
}

// TestAMissingProxyHeaderIsRefusedEverywhere. The header is trusted, so the case where
// it is absent must not fall through to an anonymous read either: a server configured to
// authenticate and reachable without doing so is one whose operator believes it is
// protected.
func TestAMissingProxyHeaderIsRefusedEverywhere(t *testing.T) {
	t.Parallel()

	srv := serve(t, web.Deps{
		Queue: &fakeQueue{}, AuthHeader: "X-Forwarded-User",
	})
	status, body := get(t, srv, "/api/queue", "")
	if status != http.StatusConflict {
		t.Fatalf("an unauthenticated read was served: %d %s", status, body)
	}
	if !strings.Contains(body, "X-Forwarded-User") {
		t.Errorf("the refusal does not name the header the proxy must set: %s", body)
	}
}

// TestReadOnlyRefusesEveryWriteAndServesEveryRead is §13's "read-only mode by flag, for
// a shared instance".
//
// The refusal is on the method rather than on the route, deliberately: a route added
// later that mutates will be a POST, and this refuses it without anybody having
// remembered to.
func TestReadOnlyRefusesEveryWriteAndServesEveryRead(t *testing.T) {
	t.Parallel()

	exec := &fakeExecutor{}
	srv := serve(t, web.Deps{
		Queue: &fakeQueue{}, Executor: exec,
		AuthHeader: "X-Forwarded-User", ReadOnly: true,
	})

	status, body := do(t, srv, http.MethodPost, "/api/decide", "priya", decision)
	if status != http.StatusConflict {
		t.Fatalf("a read-only server accepted a write: %d %s", status, body)
	}
	if exec.called != 0 {
		t.Error("the write reached the coordinator on a read-only server")
	}
	if status, body := get(t, srv, "/api/queue", "priya"); status != http.StatusOK {
		t.Errorf("a read-only server refused a read: %d %s", status, body)
	}
}

// TestADecisionWithNoRationaleIsRefused, and every problem is named at once: a caller
// fixing one field and learning about the next on the following round trip is the
// pattern the reply parsers refuse for the same reason.
func TestADecisionWithNoRationaleIsRefused(t *testing.T) {
	t.Parallel()

	exec := &fakeExecutor{}
	srv := serve(t, web.Deps{Executor: exec, AuthHeader: "X-Forwarded-User"})

	status, body := do(t, srv, http.MethodPost, "/api/decide", "priya", `{"kind":"nope"}`)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", status, body)
	}
	for _, want := range []string{"kind", "path", "rationale"} {
		if !strings.Contains(body, want) {
			t.Errorf("the refusal does not name %q: %s", want, body)
		}
	}
	if exec.called != 0 {
		t.Error("an incomplete decision reached the coordinator")
	}
}

// TestNoRouteWritesAConceptBody is §13's "No prose editing" asserted rather than
// commented.
//
// It walks what the server actually answers, so a route added later that accepted a body
// edit would have to make this test pass — which means arguing with the specification
// rather than forgetting it.
func TestNoRouteWritesAConceptBody(t *testing.T) {
	t.Parallel()

	exec := &fakeExecutor{}
	srv := serve(t, web.Deps{
		Queue: &fakeQueue{}, Reader: &fakeReader{page: &web.Page{ID: "abc"}},
		Executor: exec, AuthHeader: "X-Forwarded-User",
	})

	body := `{"kind":"edit","path":"c/a.md","rationale":"x","body":"new prose"}`
	for _, path := range []string{"/api/decide", "/api/concepts/abc", "/c/abc", "/queue"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch} {
			status, got := do(t, srv, method, path, "priya", body)
			if status == http.StatusOK {
				t.Errorf("%s %s was accepted, so something writes prose: %s",
					method, path, got)
			}
		}
	}
	if exec.called != 0 {
		t.Error("an edit reached the coordinator")
	}
}
