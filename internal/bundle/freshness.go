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
	//
	// Only the time, because a check does no network (§4.6) and the drift verdict on
	// the same observation would be a signal `lint` cannot compute and must not
	// appear to. §12's row for `stale` says as much.
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
	checks, _, err := archiveIndex(bundleDir)
	if err != nil {
		return FreshnessState{}, err
	}
	out := FreshnessState{Checks: whenChecked(checks)}
	if std, sErr := LoadArchiveStandards(bundleDir); sErr == nil {
		out.StalenessDays = std.StalenessDays.Value
	}
	return out, nil
}

// archiveIndex reads tier 0 once and returns the two things a reader of a claim's
// evidence needs: when each archived file was last verified, and which source it came
// from.
//
// **The join exists because the two records are keyed differently, and both keys are
// right for their own purpose.** `checked.jsonl` keys on `(uri, source hash)` because a
// check is an observation about a source version. A claim names an archive path because
// that is the file its quotation was validated against. The fetch record is the only
// artifact holding both, which makes it the join — and makes this the shell's work
// rather than a check's, since the checks do no I/O.
//
// Requires: bundleDir is a bundle root.
// Ensures: `checks` holds only paths that have been verified; `uris` holds every
// archive path a record names, whether verified or not. Both empty rather than nil.
//
// **They are two maps and must stay two**, which is the whole reason this function
// exists rather than one richer map. `verified` reads a *missing* key as §14.3's
// `unknown`, so a map with an entry for every path — carrying a zero timestamp for the
// unverified ones — would make every source read as checked-at-1970. That is precisely
// the collapse the four-state vocabulary exists to prevent, and it would arrive as a
// refactor that looked like tidying.
//
// One walk rather than two because the records are the join for both questions:
// `checked.jsonl` keys on `(uri, source hash)` because a check is an observation about
// a version, and a claim names an archive path because that is the file its quotation
// was validated against. The fetch record is the only artifact holding all three.
func archiveIndex(bundleDir string) (
	checks map[string]Check, uris map[string]string, err error,
) {
	const op = "bundle.archiveIndex"

	recorded, err := LoadChecks(bundleDir)
	if err != nil {
		return nil, nil, err
	}
	checks, uris = map[string]Check{}, map[string]string{}

	err = walkRecords(op, os.DirFS(bundleDir), func(rec *archive.Record) {
		if rec.ArchivePath == "" {
			return
		}
		uris[rec.ArchivePath] = rec.URI

		key := Check{URI: rec.URI, SourceSHA256: rec.SourceSHA256}
		if c, ok := recorded[key.key()]; ok {
			// A source fetched twice has two records and may have two archive
			// paths; each is keyed on its own version, so there is nothing to
			// reconcile.
			checks[rec.ArchivePath] = c
		}
	})
	if err != nil {
		return nil, nil, err
	}
	return checks, uris, nil
}

// whenChecked projects observations down to their timestamps.
//
// Requires: checks came from archiveIndex.
// Ensures: one entry per input, never nil. Pure.
//
// It exists so the `stale` check receives only what it may act on. A checker handed
// the drift verdict could branch on a network result, which §4.6 forbids it from
// having — and the narrower type is what makes that a compile error rather than a
// convention.
func whenChecked(checks map[string]Check) map[string]time.Time {
	out := make(map[string]time.Time, len(checks))
	for path, c := range checks {
		out[path] = c.At
	}
	return out
}
