package fetchcmd

import (
	"errors"
	"sort"
	"strings"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// rechecks is the recorded sources a --recheck run compares against, keyed by URI.
//
// A URI maps to a *list* because a source fetched twice has two records and each
// carries the hash that was current when somebody quoted from it (§4.1). Both are
// compared against the new bytes: the older version's claims are exactly the ones
// most likely to have lost their support, so keeping only the newest would silence
// the case worth reporting.
//
// A nil *rechecks is a run with no --recheck, and every method is written for it.
// That is what keeps the ordinary fetch path free of a flag test.
type rechecks struct {
	byURI map[string][]bundle.RecheckTarget
}

// recheckPlan works out what a --recheck run fetches, or leaves an ordinary fetch
// alone.
//
// Requires: args is what the caller named.
// Ensures: a nil rechecks and args unchanged without --recheck; otherwise the
// recorded URIs to fetch and the targets behind them. Any error returned is already
// adapted for this command's output form, so exec returns it as it stands.
//
// **The targets are gathered with the writer lock already held**, for the reason
// --dry-run takes it: a re-check that read tier 0 while another fetch was writing it
// would compare against a record set that no longer describes the archive.
//
// The two failures are reported differently because they ask different things of the
// caller. An unreadable archive is a tool failure and may be worth retrying; a URI
// tier 0 has never seen is a mistake in the invocation and retrying it will not help.
func (c *Config) recheckPlan(args []string) (*rechecks, []string, error) {
	if !c.Recheck {
		return nil, args, nil
	}
	targets, err := bundle.RecheckTargets(c.Bundle)
	if err != nil {
		return nil, nil, c.fail(root.ReasonFetchFailed, err)
	}
	rk, uris, err := selectRechecks(targets, args)
	if err != nil {
		return nil, nil, c.usage(err)
	}
	return rk, uris, nil
}

// selectRechecks decides which recorded sources this run re-fetches.
//
// Requires: targets came from bundle.RecheckTargets; only is the URIs the caller
// named, or empty for all of them.
// Ensures: the URIs to fetch, deduplicated and in record order, and a rechecks
// holding the targets behind them. An error when the caller named a URI tier 0 does
// not record, and an error when there is nothing recorded to compare — asking to
// re-check something never fetched is a mistake worth reporting rather than a run
// that quietly does nothing. Pure.
func selectRechecks(targets []bundle.RecheckTarget, only []string) (
	*rechecks, []string, error,
) {
	keep := map[string]bool{}
	for _, uri := range only {
		keep[uri] = false
	}

	r := rechecks{byURI: map[string][]bundle.RecheckTarget{}}
	var uris []string
	for i := range targets {
		uri := targets[i].URI
		if len(only) > 0 {
			if _, wanted := keep[uri]; !wanted {
				continue
			}
			keep[uri] = true
		}
		if _, seen := r.byURI[uri]; !seen {
			uris = append(uris, uri)
		}
		r.byURI[uri] = append(r.byURI[uri], targets[i])
	}

	if missing := unrecorded(keep); len(missing) > 0 {
		return nil, nil, errors.New(
			"--recheck names sources this bundle has never fetched: " +
				strings.Join(missing, ", "))
	}
	if len(uris) == 0 {
		return nil, nil, errors.New(
			"--recheck found no fetched sources to compare; run `gnosis fetch <URI>` first")
	}
	return &r, uris, nil
}

// unrecorded names the requested URIs no record matched.
//
// Sorted, because a map's iteration order is not one: an error message that listed
// the same two URIs in a different order each run would be a diagnostic nobody could
// diff or test against.
func unrecorded(keep map[string]bool) []string {
	var out []string
	for uri, found := range keep {
		if !found {
			out = append(out, uri)
		}
	}
	sort.Strings(out)
	return out
}

// compare re-runs this source's recorded passages against what was just fetched.
//
// Requires: cand is the candidate as admit left it, so its extraction is populated;
// result is this run's accumulating report.
// Ensures: one drift row per recorded version of this URI, appended to result.
// Nothing at all for a nil receiver, which is every run without --recheck.
func (r *rechecks) compare(cand *archive.Candidate, result *Result) {
	if r == nil {
		return
	}
	hash := archive.SourceHash(cand.Bytes)
	text := string(candidateText(cand))
	for i := range r.byURI[cand.URI] {
		target := &r.byURI[cand.URI][i]
		checked := bundle.Recheck(target, hash, text)
		result.addDrift(&checked)
	}
}

// candidateText is the text a quotation would have been validated against.
//
// Requires: cand is a fetched candidate, extraction attempted.
// Ensures: the extraction when the source had one, the source's own bytes
// otherwise. Pure.
//
// The extraction, not the source, and the distinction is load-bearing twice over:
// a refusal is explained from the text the reason is about, and a re-check compares
// passages against the text they were taken from. For an HTML page those are the
// extractor's output, and comparing against raw markup would report every claim
// resting on it as unsupported.
func candidateText(cand *archive.Candidate) []byte {
	if cand.Extraction != nil {
		return cand.Extraction.Text
	}
	return cand.Bytes
}
