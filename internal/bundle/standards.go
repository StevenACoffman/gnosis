package bundle

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/scan"
	"github.com/StevenACoffman/gnosis/internal/standards"
	"github.com/StevenACoffman/skillet/errs"
)

// LoadArchiveStandards reads a bundle's archive gates, falling back to the seed.
//
// Requires: bundleDir is a path, which need not exist.
// Ensures: a bundle with no standards file gets the embedded default rather than
// an error. That is not leniency: the seed is the corpus's starting policy, and a
// bundle cloned before `standards/` was introduced would otherwise be unfetchable
// until someone copied a file in. A file that *is* present and is malformed is a
// hard error, because that is somebody's edit and guessing what they meant would
// silently apply gates they did not write.
func LoadArchiveStandards(bundleDir string) (*standards.Archive, error) {
	const op = "bundle.LoadArchiveStandards"

	src, err := os.ReadFile(filepath.Join(bundleDir,
		filepath.FromSlash(standards.ArchiveFileName)))
	if errors.Is(err, fs.ErrNotExist) {
		src = standards.DefaultArchive()
	} else if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}

	a, err := standards.LoadArchive(src)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return a, nil
}

// ArchiveGates projects the loaded standards onto what the archive policy needs.
//
// This is the join §0.1's layering requires: adapters do not import each other,
// so `archive` states its dependency as a value and this shell — which may import
// both — is the one place that knows they correspond. A gate added to `standards`
// and not wired through here is inert, which is why the two are tested together.
func ArchiveGates(a *standards.Archive) archive.Gates {
	return archive.Gates{
		Allowlist:          a.Allowlist.Value,
		PerFileCap:         a.PerFileCap.Value,
		EmbeddedPayloadCap: a.EmbeddedPayloadCap.Value,
		ScanText:           scanText,
	}
}

// scanText is §9.3's admission scan, as the archive's policy needs it.
//
// It reports only the first class found. The archive's disposition is one reason
// and a source rejected for hidden characters is rejected whichever kind it
// carries; the full finding set belongs in a report, and a TODO records that
// nothing renders one yet.
func scanText(text string) archive.RejectReason {
	if len(scan.Hidden(text)) == 0 {
		return archive.ReasonNone
	}
	return archive.ReasonHiddenCharacters
}
