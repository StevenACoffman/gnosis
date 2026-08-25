package bundle

import (
	"os"
	"sort"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// Churning is one source and how much work it has cost to keep current.
type Churning struct {
	// URI is the source.
	URI string `json:"uri"`

	// Versions is how many distinct versions of its bytes tier 0 holds.
	//
	// **This is the measurement, and it needed no new field.** A source fetched twice
	// has two records (§4.1), so the record count per URI *is* how many times the
	// bytes changed. Nothing had asked the question, which is why it looked like a
	// missing feature.
	Versions int `json:"versions"`

	// Benign and Unsupported are how those changes turned out for the passages this
	// corpus quotes: bytes that moved and kept them, and bytes that moved and lost
	// them.
	//
	// The split is the whole point. Six versions that all kept their passages is a
	// source that churns without changing what it says — cheap, and §14.3.2 already
	// says its claims want a shorter `stale_after`. One version that lost a passage is
	// a different event entirely and no amount of the first adds up to it.
	Benign      int `json:"benign"`
	Unsupported int `json:"unsupported"`

	// Current is how many of its versions a re-check found upstream still matches.
	//
	// In practice this is the newest one, and it is counted rather than folded into
	// Unchecked because the two are opposites: `drift-none` means somebody compared
	// and the bytes had not moved, and calling that "not compared" would be the
	// checked/unchecked collapse this codebase refuses everywhere else — arriving,
	// as it nearly did here, through a `default` branch that swallowed a case.
	Current int `json:"current"`

	// Unchecked is how many of its versions no comparison has been made against.
	// Carried because a source with six versions and six unchecked ones has told us
	// nothing yet, and a report that omitted it would read as six cheap changes.
	//
	// The four counts sum to Versions, which is what makes the row readable: a reader
	// can see that every move is accounted for.
	Unchecked int `json:"unchecked"`
}

// Churn is what the corpus's sources have cost to keep current.
//
// FPF's `Effort` field asks what a claim costs to keep current, and this is the
// computable half: how often its sources move. **It is a count and never a cost.** §17
// forbids presenting a count as health, and "this source moved six times" is not an
// effort estimate — it is the observation an estimate would rest on. What to do about
// it is §14.4's question, weighed against how load-bearing the claims resting on it
// turn out to be.
type Churn struct {
	// Sources is every source tier 0 holds more than one version of, worst first.
	// A source fetched once has not churned and is not a row here.
	Sources []Churning `json:"sources"`

	// Recorded is how many sources tier 0 holds at all, which is the denominator: six
	// churning sources mean something different against seven than against seven
	// hundred.
	Recorded int `json:"recorded"`
}

// Churned computes the churn register.
//
// Requires: versions maps each source URI to its recorded version hashes; observed is
// this user's check record, in any order.
// Ensures: one row per source with more than one version, sorted by withdrawn support
// first, then by version count, then by URI; empty rather than nil for a corpus whose
// sources have all held still. Pure.
//
// **Sorted by what it costs rather than by how much it moved.** A source that lost a
// passage belongs at the top whatever its version count, because that is the one row
// where the answer is not "re-archive" — the ordering is `weakestDrift`'s argument
// applied to a list rather than to a claim.
//
// A source with one version is not a row. It has not churned, and a register listing
// every source with a 1 beside it would bury the six that moved among the four hundred
// that did not — which is §12's argument about a warning true of everything, applied to
// a count.
func Churned(versions map[string][]string, observed []Check) *Churn {
	out := &Churn{Sources: []Churning{}, Recorded: len(versions)}

	// Keyed here rather than by the caller, so nothing outside this package needs to
	// know what an observation's identity is spelled like — a test that built the key
	// itself would be asserting against a format rather than against a source version.
	verdicts := make(map[string]string, len(observed))
	for i := range observed {
		verdicts[observed[i].key()] = observed[i].Drift
	}

	for uri, hashes := range versions {
		if len(hashes) < 2 {
			continue
		}
		out.Sources = append(out.Sources, outcomes(uri, hashes, verdicts))
	}

	sort.Slice(out.Sources, func(i, j int) bool {
		a, b := &out.Sources[i], &out.Sources[j]
		switch {
		case a.Unsupported != b.Unsupported:
			return a.Unsupported > b.Unsupported
		case a.Versions != b.Versions:
			return a.Versions > b.Versions
		default:
			return a.URI < b.URI
		}
	})
	return out
}

// outcomes counts what one source's moves did to the passages resting on it.
//
// Requires: hashes are the source's recorded versions; verdicts is keyed as an
// observation is.
// Ensures: the four counts sum to len(hashes). Pure.
//
// Split out of Churned because the linter reported its complexity and was right: one
// function was building a lookup, filtering, classifying, and sorting. The split is by
// question — Churned decides which sources are rows and in what order, this one counts
// what a row cost.
func outcomes(uri string, hashes []string, verdicts map[string]string) Churning {
	row := Churning{URI: uri, Versions: len(hashes)}
	for _, hash := range hashes {
		key := Check{URI: uri, SourceSHA256: hash}
		switch verdicts[key.key()] {
		case gnosis.DriftBenign.String():
			row.Benign++
		case gnosis.DriftUnsupported.String():
			row.Unsupported++
		case gnosis.DriftNone.String():
			row.Current++
		default:
			// An absent verdict and an explicit `drift-unchecked` mean the same thing
			// to a reader asking what this version cost: nobody has compared it, so
			// it has told them nothing. `drift-none` is deliberately *not* here — it
			// means somebody compared and the bytes had not moved, which is the
			// opposite answer.
			row.Unchecked++
		}
	}
	return row
}

// LoadChurn gathers what Churned needs.
//
// Requires: bundleDir is a bundle root, which need not have an archive.
// Ensures: a corpus that has fetched nothing yields an empty register rather than an
// error.
//
// Both halves come from records this user already has: the versions from tier 0, which
// is authoritative and committed, and the verdicts from `checked.jsonl`, which is this
// user's own observation. That split is why the report is honest about what it does not
// know — two colleagues at one commit see the same versions and may see different
// verdicts, because one of them has run the re-check.
func LoadChurn(bundleDir string) (*Churn, error) {
	const op = "bundle.LoadChurn"

	versions := map[string][]string{}
	seen := map[string]bool{}
	err := walkRecords(op, os.DirFS(bundleDir), func(rec *archive.Record) {
		if rec.URI == "" {
			return
		}
		// A record per version, and a URI may have several — but `extracted` writes
		// a second record for the same source bytes, so the version is the source
		// hash rather than the record.
		key := rec.URI + "\x00" + rec.SourceSHA256
		if seen[key] {
			return
		}
		seen[key] = true
		versions[rec.URI] = append(versions[rec.URI], rec.SourceSHA256)
	})
	if err != nil {
		return nil, err
	}

	checks, err := LoadChecks(bundleDir)
	if err != nil {
		return nil, err
	}
	observed := make([]Check, 0, len(checks))
	for _, c := range checks {
		observed = append(observed, c)
	}
	return Churned(versions, observed), nil
}
