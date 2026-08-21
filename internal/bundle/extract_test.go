package bundle_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/standards"
)

// TestExtractorMatchesTheStandardsFile is the join the layering makes possible to
// get wrong. `standards/archive.toml` declares which extractor produced a stored
// text and `bundle` is what actually runs one; nothing forces them to agree, and a
// disagreement would stamp every extracted record with a provenance that was never
// true. This is the test the seed's own rationale promises.
func TestExtractorMatchesTheStandardsFile(t *testing.T) {
	t.Parallel()
	a, err := standards.LoadArchive(standards.DefaultArchive())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if a.HTMLExtractor.Value != bundle.ExtractorName {
		t.Errorf("standards names %q, bundle runs %q",
			a.HTMLExtractor.Value, bundle.ExtractorName)
	}
	if a.HTMLExtractorVersion.Value != bundle.ExtractorVersion {
		t.Errorf("standards pins %q, bundle runs %q",
			a.HTMLExtractorVersion.Value, bundle.ExtractorVersion)
	}
}

// TestExtractorVersionMatchesGoMod: the recorded version is a claim about which
// code ran, and go.mod is the only thing that decides that. Bumping the module
// without bumping the constant would silently backdate every new record.
func TestExtractorVersionMatchesGoMod(t *testing.T) {
	t.Parallel()
	const module = "github.com/JohannesKaufmann/html-to-markdown/v2 "

	src, err := goMod()
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	i := strings.Index(src, module)
	if i < 0 {
		t.Fatalf("go.mod does not require %s", module)
	}
	line := src[i+len(module):]
	if end := strings.IndexAny(line, " \n"); end >= 0 {
		line = line[:end]
	}
	if line != bundle.ExtractorVersion {
		t.Errorf("go.mod has %s, bundle.ExtractorVersion is %s", line, bundle.ExtractorVersion)
	}
}
