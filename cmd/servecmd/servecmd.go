// Package servecmd implements the "serve" CLI command.
package servecmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/web"
)

// defaultAuthHeader is the header a reverse proxy conventionally sets to the
// authenticated user, and the one `leafwiki` uses.
//
// A default rather than a required flag, because §13 wants reverse-proxy auth to be the
// *easy* mode — a server that refused to start without being told the conventional name
// would push operators toward `--read-only` or toward turning authentication off.
const defaultAuthHeader = "X-Forwarded-User"

// shutdownGrace bounds how long an in-flight request may finish after a signal.
//
// Ten seconds, which is rules.md's figure and long enough for the slowest thing this
// server does — a queue page that runs the gate over every draft. A shutdown that killed
// that mid-write would leave the corpus fine, because the writer commits atomically, and
// the reviewer confused, because their decision would vanish.
const shutdownGrace = 10 * time.Second

// maxSocketPath is how long a Unix socket path may be.
//
// 104 is macOS's `sun_path`; Linux allows 108. The smaller is used on both, because the
// cost of being conservative is a path nobody wanted to use anyway, and the cost of
// being permissive is a "bind: invalid argument" on one platform and not the other —
// the shape of bug that gets reported as "it works on my machine".
const maxSocketPath = 104

// longHelp is the command's prose, extracted from New for `misscmd`'s reason.
const longHelp = `Serve the review queue and the viewer (SPEC §13).

One process, because it carries two roles: §4.6's write coordinator and the viewer.
Two servers would be two authorities over one bundle.

What this adds to the CLI is the queue — quarantined drafts, contradictions and open
challenges, each with what §13 requires to decide with beside it: both claims, their
sources and those sources' recorded signals, the trust tier, the durability class. A
non-engineer decides here without a terminal, or correctly decides to defer.

No prose is edited here. The corpus body is model-written by design; this surface writes
warrants, adjudications and approvals, and there is no route that does anything else.

  --addr    listen on a TCP address, behind an authenticating proxy
  --socket  listen on a Unix socket, where filesystem permissions are the guard

Both may be given: one handler, two listeners (§4.6.2.2).

AUTHENTICATION IS THE PROXY'S. --auth-header names the request header the proxy sets to
the signed-in user, and gnosis trusts it completely. That is only sound if the proxy
strips it from incoming requests: a server reachable directly with this enabled lets any
caller name themselves. --auth-header "" disables authentication and is refused unless
--read-only is also given, because an unauthenticated writable server records decisions
nobody made.

A promotion the gate escalates cannot be completed over the wire at all (§4.6.2.1). The
confirmation phrase is a person typing a document's path, which defeats muscle memory at
a terminal and costs a program nothing; the replacement is a queue token that does not
exist yet. Escalated promotions stay terminal-only.`

// Config holds the configuration for the serve command.
type Config struct {
	*root.Config

	// Addr is the TCP address to listen on, or empty for none.
	Addr string

	// Socket is the Unix socket path to listen on, or empty for none.
	Socket string

	// AuthHeader names the header the reverse proxy sets. Empty disables
	// authentication, which is refused unless ReadOnly.
	AuthHeader string

	// ReadOnly refuses every mutation (§13, "for a shared instance").
	ReadOnly bool

	// Flags and Command are this command's own — see proofcmd for why declaring them
	// is not boilerplate.
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the serve command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("serve").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Addr, 0, "addr", "",
		"TCP address to listen on, behind an authenticating proxy")
	cfg.Flags.StringVar(&cfg.Socket, 0, "socket", "",
		"Unix socket path to listen on")
	cfg.Flags.StringVar(&cfg.AuthHeader, 0, "auth-header", defaultAuthHeader,
		"request header the proxy sets to the authenticated user; \"\" disables auth")
	cfg.Flags.BoolVar(&cfg.ReadOnly, 0, "read-only",
		"serve reads and refuse every decision")
	cfg.Command = &ff.Command{
		Name:      "serve",
		Usage:     "gnosis serve --addr HOST:PORT [--socket PATH] [--read-only]",
		ShortHelp: "serve the review queue and the viewer",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: validate, listen, serve, shut down.
func (c *Config) exec(ctx context.Context, args []string) error {
	if err := c.validate(args); err != nil {
		return c.usage(err)
	}
	listeners, err := c.listen(ctx)
	if err != nil {
		return c.fail(err)
	}
	handler := web.NewServer(web.Deps{
		Queue:      &queue{dir: c.Bundle, rules: c.Rules},
		Reader:     &reader{dir: c.Bundle},
		Executor:   &executor{dir: c.Bundle, rules: c.Rules, actorOf: web.ActorOf},
		AuthHeader: c.AuthHeader,
		ReadOnly:   c.ReadOnly,
	})
	return c.run(ctx, handler, listeners)
}

// validate settles the invocation before anything binds a port.
//
// **`--auth-header ""` on a writable server is refused here**, which is the one check in
// this command that is not about typing. §4.6.2.1's whole argument is that an approver
// must come from the transport; a transport that authenticates nobody supplies an empty
// one, and a server that accepted decisions anyway would record them as made by nobody.
func (c *Config) validate(args []string) error {
	if len(args) != 0 {
		return errors.New("serve takes no arguments; --addr and --socket name where" +
			" to listen")
	}
	if c.Addr == "" && c.Socket == "" {
		return errors.New("serve needs --addr, --socket, or both; there is no default" +
			" port, because a knowledge base that started listening somewhere nobody" +
			" chose is one nobody meant to publish")
	}
	if strings.TrimSpace(c.AuthHeader) == "" && !c.ReadOnly {
		return errors.New("--auth-header \"\" turns authentication off, which is only" +
			" allowed with --read-only: a server that cannot say who is deciding" +
			" cannot record a decision (§4.6.2.1)")
	}
	return nil
}

// listen opens every listener the invocation asked for.
//
// Requires: validate passed, so at least one is named.
// Ensures: every listener is open, or none is and the error names which failed. Opened
// before anything reports success, because a server that printed its address and then
// discovered the port was taken would have told the operator it was up.
func (c *Config) listen(ctx context.Context) ([]net.Listener, error) {
	// A ListenConfig rather than `net.Listen`, so a bind that blocks — a DNS lookup
	// on a hostname in --addr — is cancelled with the process rather than outliving
	// the signal that asked it to stop.
	var cfg net.ListenConfig

	var out []net.Listener
	if c.Addr != "" {
		l, err := cfg.Listen(ctx, "tcp", c.Addr)
		if err != nil {
			return nil, fmt.Errorf("listening on %s: %w", c.Addr, err)
		}
		out = append(out, l)
	}
	if c.Socket != "" {
		l, err := c.listenSocket(ctx, &cfg)
		if err != nil {
			closeAll(out)
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

// listenSocket opens the Unix socket, naming the two failures that read as nonsense.
//
// Both were found by running it. A socket file left by a killed process makes the bind
// report "address already in use", which sends an operator looking for a running server
// that is not there; a path over the platform's limit reports "invalid argument", which
// names nothing at all and is the likelier of the two to be hit, because a socket under
// a long temporary directory is the ordinary way to try this out.
//
// Naming them is the difference between a five-second fix and a hunt, and it costs one
// `os.Stat` and one length comparison on a path that is opened once per process.
func (c *Config) listenSocket(
	ctx context.Context, cfg *net.ListenConfig,
) (net.Listener, error) {
	if len(c.Socket) >= maxSocketPath {
		return nil, fmt.Errorf(
			"--socket %s is %d bytes and the platform's limit is %d: this is the"+
				" kernel's `sun_path`, not gnosis's, and the fix is a shorter path"+
				" rather than a shorter name — try one directly under /tmp",
			c.Socket, len(c.Socket), maxSocketPath)
	}
	l, err := cfg.Listen(ctx, "unix", c.Socket)
	if err == nil {
		return l, nil
	}
	if _, statErr := os.Stat(c.Socket); statErr == nil {
		return nil, fmt.Errorf(
			"%s already exists: another gnosis may be serving, or one was killed and"+
				" left it behind — remove it and try again: %w", c.Socket, err)
	}
	return nil, fmt.Errorf("listening on %s: %w", c.Socket, err)
}

// run serves every listener until the context is cancelled, then drains.
//
// The shape is rules.md's: serve in goroutines, shut down on `ctx.Done()` with a bounded
// timeout, and wait. `http.Server.Shutdown` closes the listeners and lets in-flight
// requests finish, which is what makes a restart invisible to a reviewer mid-decision.
func (c *Config) run(
	ctx context.Context, handler http.Handler, listeners []net.Listener,
) error {
	server := &http.Server{
		Handler: handler,
		// A read timeout, because a served corpus is reachable by whatever the proxy
		// lets through and a request that never finishes its headers holds a
		// connection open indefinitely.
		ReadHeaderTimeout: 10 * time.Second,
	}
	var wg sync.WaitGroup
	for _, l := range listeners {
		_, _ = fmt.Fprintf(c.Stderr, "listening on %s\n", l.Addr())
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := server.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
				_, _ = fmt.Fprintf(c.Stderr, "serve: %v\n", err)
			}
		}()
	}

	<-ctx.Done()
	// `WithoutCancel` rather than `Background`: the parent is already cancelled — that
	// is why we are here — so inheriting it would make Shutdown return at once and
	// drop every in-flight request. This keeps the context's values, which a
	// background context would discard, and drops only the cancellation.
	shutdownCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx), shutdownGrace)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_, _ = fmt.Fprintf(c.Stderr, "shutdown: %v\n", err)
	}
	wg.Wait()
	return nil
}

// closeAll releases listeners opened before a later one failed.
func closeAll(listeners []net.Listener) {
	for _, l := range listeners {
		_ = l.Close()
	}
}

// fail and usage adapt root's reporting to this command's name. The reason is fixed for
// `proofcmd`'s reason: every way this command fails is that it could not start.
func (c *Config) fail(cause error) error {
	return fmt.Errorf("serve: %w", c.Fail(gnosis.ReasonNoBundle, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("serve: %w", c.Usage(cause))
}
