package web_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/web"
	"github.com/StevenACoffman/skillet/errs"
)

// fakeQueue and fakeReader stand in for the corpus.
//
// Hand-written and tiny, because the interfaces are: the whole reason `internal/web`
// declares its own three-method seam is that the server can then be exercised with no
// git worktree, no SQLite and no writer lock. A fake that needed configuring would be
// evidence the interfaces were too wide.
type fakeQueue struct {
	items []web.Item
	err   error
}

type fakeReader struct {
	page *web.Page
	hits []web.Hit
	err  error
}

// fakeExecutor records what reached the write coordinator.
type fakeExecutor struct {
	got     web.Command
	called  int
	outcome gnosis.Outcome
}

func (f *fakeQueue) Waiting(context.Context) ([]web.Item, error) { return f.items, f.err }

// Concept honours the interface's contract rather than the fake's convenience: a missing
// page is ENOTFOUND and never (nil, nil), which is what lets the handler map it to a 404
// through one translation function. A fake that returned two nils would have been
// testing a shape the real reader must not have.
func (f *fakeReader) Concept(context.Context, string) (*web.Page, error) {
	switch {
	case f.err != nil:
		return nil, f.err
	case f.page == nil:
		return nil, &errs.Error{Code: errs.ENOTFOUND, Message: "no such concept"}
	default:
		return f.page, nil
	}
}

func (f *fakeReader) Search(context.Context, string, int) ([]web.Hit, error) {
	return f.hits, f.err
}

func (f *fakeExecutor) Execute(_ context.Context, cmd web.Command) (gnosis.Outcome, error) {
	f.got = cmd
	f.called++
	if f.outcome.Status == "" {
		return gnosis.OK(map[string]any{"path": cmd.Path}), nil
	}
	return f.outcome, nil
}

// conflictItem is a queue entry carrying what §13 requires to decide with.
func conflictItem() web.Item {
	side := func(ref, text string) web.Side {
		return web.Side{
			Ref: ref, Path: "c/" + ref + ".md", Title: "Retry Budget", Text: text,
			Trust: gnosis.TierHumanReviewed, Durability: gnosis.DurabilityProvable,
			Centrality: "load-bearing",
			Sources: []web.Source{{
				Resource: "https://example.org/retry", Author: "vendor",
				UsageCount: 3, LastModified: "2026-01-02", Archived: true,
			}},
		}
	}
	return web.Item{
		Kind: web.ItemConflict, ID: "doc#claim-a", Summary: "two retry caps",
		Why:    "the interval predicate found disjoint bounds",
		Action: web.CommandAdjudicate,
		Sides: []web.Side{
			side("doc#claim-a", "retries are capped at three"),
			side("doc#claim-b", "retries are capped at five"),
		},
	}
}

// serve builds the real handler over fakes and returns a client for it.
func serve(t *testing.T, d web.Deps) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(web.NewServer(d))
	t.Cleanup(srv.Close)
	return srv
}

// get issues a GET with the authenticated-user header a proxy would set.
func get(t *testing.T, srv *httptest.Server, path, user string) (int, string) {
	t.Helper()
	return do(t, srv, http.MethodGet, path, user, "")
}

// do issues one request and returns the status and the body.
func do(t *testing.T, srv *httptest.Server, method, path, user, body string) (int, string) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path,
		strings.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if user != "" {
		req.Header.Set("X-Forwarded-User", user)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(out)
}

// TestHealthzTouchesNoDependency is liveness and not readiness, and the distinction has
// teeth: a health check that queried the corpus would fail while somebody held the
// writer lock, and a load balancer would remove a server that was working correctly.
func TestHealthzTouchesNoDependency(t *testing.T) {
	t.Parallel()

	// Every dependency nil — rules.md's "pass nil for what the test does not
	// exercise", and here the nils are the assertion.
	status, body := get(t, serve(t, web.Deps{}), "/healthz", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}
}

// TestHealthzAnswersWithoutAuthentication is the case a hand run found and this file did
// not cover: an orchestrator probing liveness cannot set a proxy header, because it is
// not a signed-in person.
//
// Authenticating the probe marks a working server dead and takes it out of rotation —
// the exact failure `handleHealthz` avoids by touching no dependency, reintroduced one
// layer up by middleware that ran first. Every test above passed, because none of them
// configured a header.
func TestHealthzAnswersWithoutAuthentication(t *testing.T) {
	t.Parallel()

	srv := serve(t, web.Deps{AuthHeader: "X-Forwarded-User"})
	if status, body := get(t, srv, "/healthz", ""); status != http.StatusOK {
		t.Fatalf("an unauthenticated liveness probe was refused: %d %s", status, body)
	}
	// And nothing else is exempt, which is what keeps this a carve-out rather than a
	// hole: a route added later is authenticated because it was not named here.
	if status, _ := get(t, srv, "/api/queue", ""); status == http.StatusOK {
		t.Error("the exemption reaches past the liveness probe")
	}
}

// TestAnUnknownPathIs404. The route table registers an explicit NotFoundHandler, and the
// index page is bound to the exact root — a bare "GET /" would have made every mistyped
// URL render the front page and report success.
func TestAnUnknownPathIs404(t *testing.T) {
	t.Parallel()

	srv := serve(t, web.Deps{})
	for _, path := range []string{"/nope", "/c", "/api/nope"} {
		if status, _ := get(t, srv, path, ""); status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, status)
		}
	}
}

// TestTheQueueCarriesTheEvidenceBesideTheDecision is §13's requirement and the failure
// it names is silent: the page renders either way, and a reviewer shown a decision
// without its evidence guesses instead of deferring.
func TestTheQueueCarriesTheEvidenceBesideTheDecision(t *testing.T) {
	t.Parallel()

	srv := serve(t, web.Deps{Queue: &fakeQueue{items: []web.Item{conflictItem()}}})
	status, body := get(t, srv, "/queue", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}
	for _, want := range []string{
		"retries are capped at three", // both claims,
		"retries are capped at five",  // side by side
		"human-reviewed",              // the trust tier
		"provable",                    // the durability class
		"load-bearing",                // the centrality class
		"vendor",                      // the source's author
		"2026-01-02",                  // and when it last moved
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the queue page does not show %q, which §13 requires to decide"+
				" with:\n%s", want, body)
		}
	}
}

// TestCorpusContentCannotInjectMarkup. The corpus body is model-written by design, which
// makes it the least trustworthy content this server renders.
func TestCorpusContentCannotInjectMarkup(t *testing.T) {
	t.Parallel()

	srv := serve(t, web.Deps{Reader: &fakeReader{page: &web.Page{
		ID: "abc", Title: "Retry", Body: "<script>alert(1)</script>",
	}}})
	status, body := get(t, srv, "/c/abc", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Errorf("a concept body reached the page unescaped:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("the body did not survive escaping at all:\n%s", body)
	}
}

// TestSearchRefusesALimitItCannotRead. A caller who typed `limit=ten` and silently got
// ten results learns the wrong thing about the API and finds out later.
func TestSearchRefusesALimitItCannotRead(t *testing.T) {
	t.Parallel()

	srv := serve(t, web.Deps{Reader: &fakeReader{}})
	for path, want := range map[string]int{
		"/api/search":                 http.StatusBadRequest,
		"/api/search?q=retry&limit=0": http.StatusBadRequest,
		"/api/search?q=retry&limit=x": http.StatusBadRequest,
		"/api/search?q=retry":         http.StatusOK,
	} {
		if status, body := get(t, srv, path, ""); status != want {
			t.Errorf("GET %s = %d, want %d: %s", path, status, want, body)
		}
	}
}

// decodeOutcome reads §8.0's envelope out of a response body.
func decodeOutcome(t *testing.T, body string) gnosis.Outcome {
	t.Helper()

	var out gnosis.Outcome
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("the response is not an outcome envelope: %v\n%s", err, body)
	}
	return out
}
