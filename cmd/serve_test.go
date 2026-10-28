package cmd_test

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd"
)

// listenSignal is a writer that reports the first line naming a bound address.
//
// A writer rather than a poll, so the test synchronises on the event it cares about —
// the listener being open — instead of on the clock. Writes after the first are passed
// through and dropped: the shutdown may log, and a test that blocked the server's
// stderr would deadlock the thing it is draining.
type listenSignal struct {
	once  sync.Once
	bound chan<- string
}

// TestServeRefusesToListenNowhere. There is no default port, deliberately: a knowledge
// base that started listening somewhere nobody chose is one nobody meant to publish.
func TestServeRefusesToListenNowhere(t *testing.T) {
	t.Parallel()

	_, stderr, err := run(t, "--bundle", corpus(t), "serve")
	if err == nil {
		t.Fatal("serve started with no listener")
	}
	if !strings.Contains(stderr, "--addr") || !strings.Contains(stderr, "--socket") {
		t.Errorf("the refusal does not name where it could listen: %s", stderr)
	}
}

// TestServeRefusesAnUnauthenticatedWritableServer is §4.6.2.1's rule at the invocation.
//
// A transport that authenticates nobody supplies an empty approver, and §9.5's rule that
// an approval is not self-granted is unenforceable without one — so a writable server
// with authentication off would record decisions as made by nobody. The refusal is here
// rather than at the first write, because by then somebody is already running it.
func TestServeRefusesAnUnauthenticatedWritableServer(t *testing.T) {
	t.Parallel()

	bundleDir := corpus(t)
	_, stderr, err := run(t, "--bundle", bundleDir, "serve",
		"--addr", "127.0.0.1:0", "--auth-header", "")
	if err == nil {
		t.Fatal("an unauthenticated writable server started")
	}
	if !strings.Contains(stderr, "--read-only") {
		t.Errorf("the refusal does not name the one way this is allowed: %s", stderr)
	}
}

// TestServeStartsAndDrains runs the real binary path: bind, answer, shut down on the
// context the dispatcher was given.
//
// **The listener is opened before anything reports success**, which is what this asserts
// by using port 0 and reading back the address: a server that printed an address and
// then discovered the port was taken would have told the operator it was up.
func TestServeStartsAndDrains(t *testing.T) {
	t.Parallel()

	bundleDir := corpus(t)
	ctx, cancel := context.WithCancel(t.Context())
	bound := make(chan string, 1)
	stderr := &listenSignal{bound: bound}
	done := make(chan struct{})
	go func() {
		defer close(done)
		var out bytes.Buffer
		_ = cmd.Run(ctx, []string{
			"--bundle", bundleDir, "serve", "--addr", "127.0.0.1:0", "--read-only",
		}, strings.NewReader(""), &out, stderr)
	}()

	// Wait for the bind rather than for a duration. A sleep long enough to be reliable
	// on a loaded machine costs every run; one short enough not to is a test that fails
	// for reasons unrelated to the code — which is why this repository forbids
	// time.Sleep in tests, and a linter is what remembered that here.
	line := <-bound
	if !strings.Contains(line, "listening on 127.0.0.1:") {
		t.Fatalf("the server did not report where it bound: %s", line)
	}

	// Cancelling the dispatcher's context is how a signal reaches this in production.
	// The server drains and returns; if it did not, this test would hang rather than
	// fail, which is the honest failure for a shutdown that never completes.
	cancel()
	<-done
}

func (l *listenSignal) Write(p []byte) (int, error) {
	if line := string(p); strings.Contains(line, "listening on") {
		l.once.Do(func() { l.bound <- line })
	}
	return len(p), nil
}
