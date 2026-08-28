package bundle_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

func promotion(eff command.Effect) *command.Promote {
	return &command.Promote{Path: "c/a.md", Eff: eff, Approver: "human:priya"}
}

// TestRejectedCommandIsAResultNotACrash: a command constructed wrongly exits
// usage and names the field. Returning an error would make a caller's mistake
// look like a tool failure.
func TestRejectedCommandIsAResultNotACrash(t *testing.T) {
	t.Parallel()
	c := bundle.Coordinator{Dir: t.TempDir()}

	got, err := c.Execute(t.Context(), &command.Promote{})
	if err != nil {
		t.Fatalf("a rejected command returned an error: %v", err)
	}
	if got.Code != gnosis.CodeUsage {
		t.Errorf("code = %d, want usage", got.Code)
	}
	if !got.Valid() {
		t.Errorf("the envelope is malformed: %+v", got)
	}
	if !strings.Contains(got.Message, "approver") {
		t.Errorf("the message does not name a missing field: %q", got.Message)
	}
}

// TestValidationHappensBeforeTheLock. A malformed command must not make a
// well-formed one wait, so the coordinator rejects it while another writer holds
// the bundle rather than queueing behind them.
func TestValidationHappensBeforeTheLock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	held, err := bundle.AcquireWriter(t.Context(), dir)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer held.Release()

	// A context already cancelled: if the coordinator reached the lock at all
	// this would report writer_busy instead of a usage error.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	c := bundle.Coordinator{Dir: dir}
	got, err := c.Execute(ctx, &command.Promote{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got.Code != gnosis.CodeUsage {
		t.Errorf("code = %d (reason %q), want usage — validation ran after the lock",
			got.Code, got.Reason)
	}
}

// TestContentionIsBlockedNotBroken: another writer holding the bundle is a state
// that resolves on its own, so it must be distinguishable from a fault.
func TestContentionIsBlockedNotBroken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	held, err := bundle.AcquireWriter(t.Context(), dir)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer held.Release()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	c := bundle.Coordinator{Dir: dir}
	got, err := c.Execute(ctx, promotion(command.EffectPreview))
	if err != nil {
		t.Fatalf("contention returned an error: %v", err)
	}
	if got.Status != gnosis.StatusBlocked {
		t.Errorf("status = %q, want blocked", got.Status)
	}
	if got.Reason != gnosis.ReasonWriterBusy {
		t.Errorf("reason = %q, want %q", got.Reason, gnosis.ReasonWriterBusy)
	}
}

// TestAPreviewAlsoTakesTheLock. It looks unnecessary and is not: a preview
// computes the diff the apply will use, and one racing a concurrent write would
// report a diff against a bundle that no longer exists — the window §9.4 closes.
func TestAPreviewAlsoTakesTheLock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	held, err := bundle.AcquireWriter(t.Context(), dir)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer held.Release()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	c := bundle.Coordinator{Dir: dir}
	got, err := c.Execute(ctx, promotion(command.EffectPreview))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got.Reason != gnosis.ReasonWriterBusy {
		t.Errorf("a preview did not wait for the lock: reason %q", got.Reason)
	}
}

// TestOneWriterAtATime is what the lock is for, exercised rather than described.
//
// Each holder appends "enter" and "exit" to a shared log and does real file work
// in between. With the lock the log is strictly nested — every enter immediately
// followed by its own exit. Without it the writes interleave and the pairing
// breaks. The file work is there to make the critical section long enough that an
// unlocked version would actually interleave; a bare counter would pass whether
// or not the lock worked, which is a test that proves nothing.
func TestOneWriterAtATime(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	var (
		mu  sync.Mutex
		log []string
		wg  sync.WaitGroup
	)
	note := func(s string) {
		mu.Lock()
		log = append(log, s)
		mu.Unlock()
	}

	const writers = 8
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock, err := bundle.AcquireWriter(t.Context(), dir)
			if err != nil {
				t.Errorf("acquire: %v", err)
				return
			}
			defer lock.Release()

			id := strconv.Itoa(i)
			note("enter " + id)
			if werr := churn(filepath.Join(dir, "contended.txt"), id); werr != nil {
				t.Errorf("churn: %v", werr)
				return
			}
			note("exit " + id)
		}()
	}
	wg.Wait()

	assertNested(t, log, writers)
}

// churn does enough file work that the critical section is wide enough to be
// raced. Without it a bare counter would pass whether or not the lock worked.
func churn(path, content string) error {
	for range 20 {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
		if _, err := os.ReadFile(path); err != nil {
			return err
		}
	}
	return nil
}

// assertNested checks that every enter is immediately followed by its own exit,
// which is what "one writer at a time" looks like in a log.
func assertNested(t *testing.T, log []string, writers int) {
	t.Helper()
	if len(log) != 2*writers {
		t.Fatalf("got %d log entries, want %d: %v", len(log), 2*writers, log)
	}
	for i := 0; i < len(log); i += 2 {
		enter, exit := log[i], log[i+1]
		if !strings.HasPrefix(enter, "enter ") ||
			exit != "exit "+strings.TrimPrefix(enter, "enter ") {
			t.Fatalf("writers overlapped at %d (%q then %q): %v", i, enter, exit, log)
		}
	}
}

// TestReleaseIsIdempotent, so a deferred Release needs no guard and a handler
// that released early does not panic when the defer runs.
func TestReleaseIsIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	lock, err := bundle.AcquireWriter(t.Context(), dir)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	lock.Release()
	lock.Release()

	var nilLock *bundle.Writer
	nilLock.Release()

	// And the lock is genuinely free afterwards.
	again, err := bundle.AcquireWriter(t.Context(), dir)
	if err != nil {
		t.Fatalf("the lock was not released: %v", err)
	}
	again.Release()
}

// TestReadersNeedNoWriter is SPEC §4.6's requirement, and it is a requirement
// rather than a nicety: a corpus has to be inspectable when nothing is serving.
func TestReadersNeedNoWriter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	held, err := bundle.AcquireWriter(t.Context(), dir)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer held.Release()

	// LoadIndex is the read path. It must answer while a writer holds the bundle.
	if _, lerr := bundle.LoadIndex(t.Context(), dir); lerr != nil {
		t.Errorf("a read failed while a writer held the lock: %v", lerr)
	}
}
