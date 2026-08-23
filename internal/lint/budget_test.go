package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

const mib = 1024 * 1024

func TestDiagnoseBudget(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		size lint.ArchiveSize
		want string // "" for silence, otherwise the severity
	}{
		"empty archive": {
			lint.ArchiveSize{Bytes: 0, Budget: 256 * mib, WarnFraction: 0.8}, "",
		},
		"well under": {
			lint.ArchiveSize{Bytes: 10 * mib, Budget: 256 * mib, WarnFraction: 0.8}, "",
		},
		"just under the threshold": {
			lint.ArchiveSize{Bytes: 204 * mib, Budget: 256 * mib, WarnFraction: 0.8}, "",
		},
		"at the threshold": {
			lint.ArchiveSize{Bytes: 205 * mib, Budget: 256 * mib, WarnFraction: 0.8}, "warning",
		},
		"between threshold and budget": {
			lint.ArchiveSize{Bytes: 250 * mib, Budget: 256 * mib, WarnFraction: 0.8}, "warning",
		},
		"exactly at the budget": {
			lint.ArchiveSize{Bytes: 256 * mib, Budget: 256 * mib, WarnFraction: 0.8}, "warning",
		},
		"over the budget": {
			lint.ArchiveSize{Bytes: 300 * mib, Budget: 256 * mib, WarnFraction: 0.8}, "error",
		},
		// No budget declared is not the same as a budget of zero being exceeded.
		"no budget": {
			lint.ArchiveSize{Bytes: 900 * mib, Budget: 0, WarnFraction: 0.8}, "",
		},
		"no warn fraction": {
			lint.ArchiveSize{Bytes: 900 * mib, Budget: 256 * mib}, "",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := lint.DiagnoseBudget(&tc.size)
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("wanted silence, got %+v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("got %d diagnostics, want 1", len(got))
			}
			if string(got[0].Severity) != tc.want {
				t.Errorf("severity = %q, want %q", got[0].Severity, tc.want)
			}
		})
	}
}

// TestTheWarningNamesBothNumbers, or a reader cannot tell how much room is left.
func TestTheWarningNamesBothNumbers(t *testing.T) {
	t.Parallel()
	size := lint.ArchiveSize{Bytes: 250 * mib, Budget: 256 * mib, WarnFraction: 0.8}

	got := lint.DiagnoseBudget(&size)
	if len(got) != 1 {
		t.Fatalf("got %d diagnostics", len(got))
	}
	for _, want := range []string{"250.0 MiB", "256.0 MiB"} {
		if !strings.Contains(got[0].Message, want) {
			t.Errorf("message %q omits %q", got[0].Message, want)
		}
	}
}

// TestTheWarningNamesTheLargestFiles. §9.2 requires it, and the reason is that a
// caller told the archive is big and not told what is big in it has to go and
// look, which is the work the report was supposed to save.
func TestTheWarningNamesTheLargestFiles(t *testing.T) {
	t.Parallel()
	size := lint.ArchiveSize{
		Bytes: 250 * mib, Budget: 256 * mib, WarnFraction: 0.8,
		Largest: []lint.ArchiveFile{
			{Path: "evidence/text/aa/huge.md", Bytes: 90 * mib},
			{Path: "evidence/text/bb/big.md", Bytes: 40 * mib},
		},
	}

	got := lint.DiagnoseBudget(&size)
	if !strings.Contains(got[0].Message, "huge.md") {
		t.Errorf("the message does not name the largest file: %q", got[0].Message)
	}
	if !strings.Contains(got[0].Message, "90.0 MiB") {
		t.Errorf("the message does not size it: %q", got[0].Message)
	}
}

// TestBinaryUnits: the budget is declared in them, so reporting 268.4 MB against
// a 256 MiB budget would make a reader do arithmetic to find out if they are over.
func TestBinaryUnits(t *testing.T) {
	t.Parallel()
	size := lint.ArchiveSize{Bytes: 300 * mib, Budget: 256 * mib, WarnFraction: 0.8}

	msg := lint.DiagnoseBudget(&size)[0].Message
	if !strings.Contains(msg, "MiB") {
		t.Errorf("message uses decimal units: %q", msg)
	}
	if strings.Contains(msg, "MB") && !strings.Contains(msg, "MiB") {
		t.Errorf("message mixes unit systems: %q", msg)
	}
}

// TestSortArchiveFilesIsStable, so two runs over one archive produce the same
// list and a diff of two reports means something.
func TestSortArchiveFilesIsStable(t *testing.T) {
	t.Parallel()
	files := []lint.ArchiveFile{
		{Path: "b.md", Bytes: 100},
		{Path: "a.md", Bytes: 100},
		{Path: "c.md", Bytes: 200},
	}
	lint.SortArchiveFiles(files)

	if files[0].Path != "c.md" {
		t.Errorf("biggest first failed: %+v", files)
	}
	// Ties break on path.
	if files[1].Path != "a.md" || files[2].Path != "b.md" {
		t.Errorf("ties did not break on path: %+v", files)
	}
}
