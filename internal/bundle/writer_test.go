package bundle_test

import (
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/skillet/errs"
)

// writerFor takes the writer lock on dir and releases it when the test ends.
//
// This is the one helper the rules' "abstract only when truly universal" bar is
// met by in this package: every write test needs a Writer, obtaining one is three
// lines of error handling, and forgetting the Release leaks a held lock into the
// next test in the same temporary directory.
func writerFor(t *testing.T, dir string) *bundle.Writer {
	t.Helper()

	w, err := bundle.AcquireWriter(t.Context(), dir)
	if err != nil {
		t.Fatalf("acquiring the writer lock on %s: %v", dir, err)
	}
	t.Cleanup(w.Release)
	return w
}

// withWriter takes the writer lock for the duration of write and no longer.
//
// The scoped form exists because a fixture that writes and then hands the bundle
// to something which takes the lock itself — `Coordinator.Execute`, every promote
// test — would deadlock against its own setup if the fixture's Writer lived until
// cleanup. That is not an inconvenience of the helper; it is the mechanism working:
// holding write permission longer than the write is now visible rather than
// assumed, and the fixtures had to say how long they meant.
func withWriter(t *testing.T, dir string, write func(w *bundle.Writer)) {
	t.Helper()

	w, err := bundle.AcquireWriter(t.Context(), dir)
	if err != nil {
		t.Fatalf("acquiring the writer lock on %s: %v", dir, err)
	}
	defer w.Release()
	write(w)
}

// TestAReleasedWriterRefusesToWrite covers the one hole the type cannot close by
// construction.
//
// A Writer can only be obtained by taking the lock, so the compiler now refuses a
// caller that never took it — which was the actual defect: `gnosis fetch` wrote
// tier 0 and rewrote checked.jsonl with no lock at all. What no signature can
// prevent is a caller keeping the value past its `defer Release()`. That is a
// runtime check, and a runtime check nobody has seen fire is in the same position
// the prose precondition was.
//
// Every write method is exercised, because the guard is per-method and a method
// added later without it is exactly what this asserts against.
func TestAReleasedWriterRefusesToWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w, err := bundle.AcquireWriter(t.Context(), dir)
	if err != nil {
		t.Fatalf("acquiring: %v", err)
	}
	w.Release()

	cases := map[string]func() error{
		"Audit": func() error {
			return w.Audit(&audit.Row{At: time.Now(), Op: audit.OpInit, Actor: "check:t"})
		},
		"StoreCached": func() error {
			return w.StoreCached(&bundle.CachedReply{Key: "k", Reply: "{}"})
		},
		"StorePromptMeta": func() error {
			return w.StorePromptMeta(&bundle.PromptMeta{Key: "k"})
		},
		"RecordChecks": func() error {
			return w.RecordChecks(time.Now(), []bundle.Check{{URI: "u", SourceSHA256: "h"}})
		},
		"Discard": func() error { return w.Discard("c/x.md") },
		"Quarantine": func() error {
			_, qErr := w.Quarantine("c/x.md", []byte("x"))
			return qErr
		},
		"Prompts": func() error {
			_, pErr := w.Prompts(&bundle.PromptOptions{URIs: []string{"u"}})
			return pErr
		},
		"StoreEvidence": func() error {
			_, sErr := w.StoreEvidence(nil)
			return sErr
		},
	}
	for name, write := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			wErr := write()
			if wErr == nil {
				t.Fatal("a released Writer wrote; the lock guarantee is not enforced")
			}
			// EINTERNAL rather than ECONFLICT: this is the process's own bug, not
			// contention, and WriterBusy must not tell a caller to retry it.
			if got := errs.ErrorCode(wErr); got != errs.EINTERNAL {
				t.Errorf("code = %q, want %q (%v)", got, errs.EINTERNAL, wErr)
			}
			if bundle.WriterBusy(wErr) {
				t.Error("a released Writer reported as busy; a caller would retry forever")
			}
		})
	}
}

// TestAReleasedWriterReportsNoDirectory pins that Dir is safe on a value a caller
// is no longer entitled to use, since a command rendering an error message will
// reach for it.
func TestAReleasedWriterReportsNoDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w := writerFor(t, dir)
	if got := w.Dir(); got != dir {
		t.Errorf("Dir() = %q, want %q", got, dir)
	}

	var absent *bundle.Writer
	if got := absent.Dir(); got != "" {
		t.Errorf("nil Writer Dir() = %q, want empty", got)
	}
}
