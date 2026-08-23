package bundle

import (
	"os"
	"time"

	"github.com/StevenACoffman/gnosis/internal/archive"
)

// FreshnessState is what the stale check needs about time, gathered by the caller.
//
// A value passed in rather than read inside Snapshot, for the reason IndexState is:
// both are I/O over paths rather than over the fs.FS a snapshot is built from, and
// keeping them parameters is what lets a check be tested from a literal.
type FreshnessState struct {
	// Checks maps an archive path to when this user last verified that source
	// version. An absent key means never, which §14.3 calls unknown.
	Checks map[string]time.Time

	// StalenessDays is the declared window from standards/. Zero disables it.
	StalenessDays int
}

// LoadFreshness gathers what the stale check needs.
//
// Requires: bundleDir is a bundle root, which need not have an archive.
// Ensures: a usable zero-ish value rather than an error when the standards do not
// load — the window simply does not apply, which is a defined state, and failing
// the whole lint because a threshold file is malformed would be a check refusing to
// run over a problem `doctor` already reports.
func LoadFreshness(bundleDir string) (FreshnessState, error) {
	checks, err := checksByArchivePath(bundleDir)
	if err != nil {
		return FreshnessState{}, err
	}
	out := FreshnessState{Checks: checks}
	if std, sErr := LoadArchiveStandards(bundleDir); sErr == nil {
		out.StalenessDays = std.StalenessDays.Value
	}
	return out, nil
}

// checksByArchivePath joins this user's check record to the archived files a
// document's claims actually name.
//
// Requires: bundleDir is a bundle root.
// Ensures: a map from archive path to the moment that source version was last
// verified, holding only paths that have been. Empty rather than nil, and never an
// error for a corpus with no archive or no checks.
//
// **The join exists because the two records are keyed differently, and both keys
// are right for their own purpose.** `checked.jsonl` keys on `(uri, source hash)`
// because a check is an observation about a source version. A claim names an
// archive path because that is the file its quotation was validated against. The
// fetch record is the only artifact holding both, which makes it the join — and
// makes this the shell's work rather than a check's, since the checks do no I/O.
//
// A record whose source has never been checked contributes nothing, so an absent
// key means never-verified rather than verified-at-the-zero-time. That distinction
// is §14.3's `unknown`, and collapsing it would report an unexamined source as
// having been examined in 1970.
func checksByArchivePath(bundleDir string) (map[string]time.Time, error) {
	const op = "bundle.checksByArchivePath"

	checks, err := LoadChecks(bundleDir)
	if err != nil {
		return nil, err
	}
	out := map[string]time.Time{}
	if len(checks) == 0 {
		return out, nil
	}

	err = walkRecords(op, os.DirFS(bundleDir), func(rec *archive.Record) {
		if rec.ArchivePath == "" {
			return
		}
		c, ok := checks[Check{URI: rec.URI, SourceSHA256: rec.SourceSHA256}.key()]
		if !ok {
			return
		}
		// A source fetched twice has two records and may have two archive paths;
		// each is keyed on its own version, so there is nothing to reconcile.
		out[rec.ArchivePath] = c.At
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
