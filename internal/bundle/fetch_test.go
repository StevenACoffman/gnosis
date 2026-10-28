package bundle_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// TestAFailedFetchNamesItsOperationOnce. A non-2xx response is an error naming the
// status, and its message once began "bundle.Fetcher.fetchURL: bundle.Fetcher.fetchURL:".
// The over-limit response is built the same way, but reaching it takes a 64 MiB body.
func TestAFailedFetchNamesItsOperationOnce(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	f := bundle.Fetcher{HTTP: srv.Client()}
	_, err := f.Fetch(t.Context(), srv.URL+"/missing")
	if err == nil {
		t.Fatal("a 404 was not an error")
	}
	const op = "bundle.Fetcher.fetchURL"
	if n := strings.Count(err.Error(), op); n != 1 {
		t.Errorf("the error names %s %d times, want once: %v", op, n, err)
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("the error does not name the status: %v", err)
	}
}
