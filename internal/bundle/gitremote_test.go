package bundle_test

import (
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// gitDaemon serves a bare clone of origin over the git protocol on loopback and
// returns the `git://` URL of it.
//
// Requires: origin is a repository on disk; git is on PATH.
// Ensures: a URL that speaks the real protocol, and a daemon that is killed when the
// test ends. Skips rather than fails when the daemon cannot be started, because
// `git daemon` is a separate binary in some distributions' packaging and a test that
// failed for that reason would be reporting the packaging.
//
// **A real port negotiation is the whole point.** The existing tests clone a local
// path, which go-git handles without a transport at all: no capability advertisement,
// no shallow-clone negotiation, no wire protocol. The entry behind this test says so
// in as many words, and the way to answer it is a server rather than a mock — a mock
// of a protocol is a mock of one's own belief about the protocol.
//
// The port comes from the kernel (`127.0.0.1:0`) rather than a literal, because a
// hardcoded port makes two concurrent runs of one package fight and fail on whichever
// runner is busiest. The listener is closed before the daemon binds, which is a race
// the kernel does not close — and the alternative, `--inetd` with hand-piped
// connections, would put a socket relay in a fixture testing a clone.
func gitDaemon(t *testing.T, origin string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}

	serve := t.TempDir()
	runGit(t, serve, "clone", "--bare", origin, "repo.git")

	port := freePort(t)
	//nolint:gosec // G204: every argument is a literal or a path this fixture made.
	daemon := exec.CommandContext(t.Context(), "git", "daemon",
		"--reuseaddr", "--listen=127.0.0.1", "--port="+port,
		"--base-path="+serve, "--export-all", serve)
	daemon.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	if err := daemon.Start(); err != nil {
		t.Skipf("git daemon will not start: %v", err)
	}
	t.Cleanup(func() {
		if daemon.Process != nil {
			_ = daemon.Process.Kill()
		}
		// Reaped, so the test does not leave a zombie for the run to trip over.
		_ = daemon.Wait()
	})

	waitForListener(t, "127.0.0.1:"+port)
	return "git://127.0.0.1:" + port + "/repo.git"
}

// freePort asks the kernel for a port nobody is using.
func freePort(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a port: %v", err)
	}
	tcp, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("a tcp listener reports %T as its address", l.Addr())
	}
	port := strconv.Itoa(tcp.Port)
	if cErr := l.Close(); cErr != nil {
		t.Fatalf("release the reserved port: %v", cErr)
	}
	return port
}

// waitForListener blocks until something accepts on addr, or the test's deadline.
//
// A ticker rather than a sleep: this repository forbids `time.Sleep` in tests, and the
// reason applies here rather than being a style rule — a fixed sleep is either longer
// than the wait or shorter than it needs to be, and on a loaded runner it is both at
// different moments.
func waitForListener(t *testing.T, addr string) {
	t.Helper()

	dialer := net.Dialer{Timeout: time.Second}
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		conn, err := dialer.DialContext(t.Context(), "tcp", addr)
		if err == nil {
			_ = conn.Close()
			return
		}
		select {
		case <-tick.C:
		case <-t.Context().Done():
			t.Skipf("git daemon never accepted on %s", addr)
		}
	}
}

// TestClonesOverTheGitProtocol is the entry: the adapter's tests cloned a local
// repository built by git itself, which covers the walk, the URI rewrite and the
// cleanup — and not the wire.
//
// What this adds is a real transport: capability advertisement, and a shallow
// single-branch negotiation the local path never performs. The assertions are the same
// ones the local test makes, deliberately — the claim being checked is that the
// adapter behaves the same way when there is a protocol in between.
func TestClonesOverTheGitProtocol(t *testing.T) {
	t.Parallel()

	origin := originRepo(t, map[string]string{
		"README.md":     "# Origin\n\nA quotable claim over the wire.\n",
		"docs/note.txt": "nested\n",
	})
	url := gitDaemon(t, origin)

	var f bundle.Fetcher
	got, err := f.Fetch(t.Context(), url)
	if err != nil {
		t.Fatalf("fetch over git://: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2: %+v", len(got), got)
	}
	for i := range got {
		if !strings.HasPrefix(got[i].URI, "git://127.0.0.1:") {
			t.Errorf("a candidate does not record the remote: %q", got[i].URI)
		}
		if !strings.Contains(got[i].URI, "#") {
			t.Errorf("a candidate does not name its path in the repository: %q", got[i].URI)
		}
		if len(got[i].Revision) != 40 {
			t.Errorf("a candidate reports revision %q", got[i].Revision)
		}
	}
}

// TestAServerThatHangsUpIsAnError is the failure worth having a test for, and the
// assertion is on *which* failure it is.
//
// A clone that dies mid-protocol must not come back as an empty candidate set. That
// would be a source that silently archives nothing: `fetch` would report zero sources
// and exit zero, and a corpus would go on citing a URI nobody has any evidence for.
// The distinction between "no files" and "could not read" is the same one every
// `unchecked` state in this codebase exists to preserve, at the transport.
func TestAServerThatHangsUpIsAnError(t *testing.T) {
	t.Parallel()

	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			conn, aErr := l.Accept()
			if aErr != nil {
				return
			}
			// Accept and hang up, which is what a server under load, a
			// misconfigured proxy, or a killed daemon does.
			_ = conn.Close()
		}
	}()

	var f bundle.Fetcher
	got, fErr := f.Fetch(t.Context(), "git://"+l.Addr().String()+"/repo.git")
	if fErr == nil {
		t.Fatalf("a server that hung up produced %d candidates and no error", len(got))
	}
	if len(got) != 0 {
		t.Errorf("a failed clone still produced candidates: %+v", got)
	}
}

// TestAuthenticationIsNotTested records what this deliberately does not cover, because
// a gap nobody names is indistinguishable from a gap nobody noticed.
//
// Authentication needs a credential store — an ssh agent, a token helper, a
// `.netrc` — and a test that fabricates one tests the fabrication: it would assert
// that go-git reads the credentials the fixture handed it, which is true of any
// library and says nothing about whether a real clone against a private remote works.
// The honest coverage is a person cloning a private repository once, and this is the
// note saying so.
//
// It is a test rather than a comment so it appears in the run and cannot be deleted
// silently along with the thing it is about.
func TestAuthenticationIsNotTested(t *testing.T) {
	t.Parallel()
	t.Skip("authentication needs a real credential store; see this test's comment")
}
