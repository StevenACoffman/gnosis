package gnosis_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

func TestSlugFrom(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		title string
		want  gnosis.Slug
	}{
		"plain":              {"Retry Budget", "retry-budget"},
		"punctuation":        {"Retry budget: how many?", "retry-budget-how-many"},
		"leading noise":      {"—— Retry", "retry"},
		"trailing noise":     {"Retry ——", "retry"},
		"collapses runs":     {"Retry   ///   Budget", "retry-budget"},
		"digits kept":        {"HTTP 429 handling", "http-429-handling"},
		"empty":              {"", "untitled"},
		"punctuation only":   {"?!—", "untitled"},
		"already a slug":     {"retry-budget", "retry-budget"},
		"non-ascii dropped":  {"Rétry", "r-try"},
		"underscores become": {"retry_budget", "retry-budget"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := gnosis.SlugFrom(tc.title); got != tc.want {
				t.Errorf("SlugFrom(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}

func TestSlugFromIsIdempotent(t *testing.T) {
	t.Parallel()
	// A slug is rewritten on every retitle, so a title that already reads as a
	// slug must not drift on each pass. Without this, a filename could change
	// on every write and churn the index for no reason.
	titles := []string{
		"Retry Budget", "", "?!—", "http-429-handling", "Retry   ///   Budget",
	}
	for _, title := range titles {
		once := gnosis.SlugFrom(title)
		twice := gnosis.SlugFrom(string(once))
		if once != twice {
			t.Errorf("SlugFrom(%q) = %q, but SlugFrom(%q) = %q", title, once, once, twice)
		}
	}
}

func TestSlugFromProducesOnlyAllowedCharacters(t *testing.T) {
	t.Parallel()
	// The filename contract from SPEC §5.1.1: the slug half must be safe to put
	// in a path on any filesystem, so the allowed set is closed.
	titles := []string{
		"Retry/Budget", "a..b", "  ", "CAPS", "tabs\tand\nnewlines", "emoji 🎉 here",
	}
	for _, title := range titles {
		assertPathSafeSlug(t, title, gnosis.SlugFrom(title))
	}
}

// assertPathSafeSlug fails t unless got satisfies every part of the slug
// contract. title is reported so a failure names the input that produced it.
func assertPathSafeSlug(t *testing.T, title string, got gnosis.Slug) {
	t.Helper()
	s := string(got)
	switch {
	case s == "":
		t.Errorf("SlugFrom(%q) returned empty; want a fallback", title)
	case strings.HasPrefix(s, "-"), strings.HasSuffix(s, "-"):
		t.Errorf("SlugFrom(%q) = %q has a boundary hyphen", title, s)
	case strings.Contains(s, "--"):
		t.Errorf("SlugFrom(%q) = %q has a doubled hyphen", title, s)
	}
	notAllowed := func(r rune) bool { return !slugRuneAllowed(r) }
	if i := strings.IndexFunc(s, notAllowed); i >= 0 {
		t.Errorf("SlugFrom(%q) = %q contains disallowed %q", title, s, s[i])
	}
}

// slugRuneAllowed reports whether r may appear in a slug. Stated positively:
// the set is closed, and a reader should see the members rather than reconstruct
// them from a negation.
func slugRuneAllowed(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
}
