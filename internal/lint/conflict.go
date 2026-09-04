package lint

import (
	"sort"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/textnorm"
)

// SourceVersion identifies which version of which source an archived file holds.
//
// Gathered by the shell from tier 0's records, like every other Snapshot field. It is
// keyed by archive path because that is what a claim names.
type SourceVersion struct {
	// URI is the source the bytes came from.
	URI string

	// SHA256 is the content hash of those bytes — the version.
	SHA256 string
}

// claimSite is one place a claim is asserted, and the version its evidence rests on.
type claimSite struct {
	Path    string
	ClaimID string
	SHA256  string
}

// conflictCheck reports the §10.2 predicates that are exactly decidable today.
//
// §10.2 lists six and this implements one. Naming which, and why the others are absent,
// is the point of this comment: a check called `conflict` that quietly implemented a
// sixth of what §10.2 describes would be the "unwarrantedly confident" reading §12.0
// warns about — a green run standing in for an examination nobody performed.
//
//   - **evidence divergence** — implemented below.
//   - **severity** and **level divergence** read `ruleset.Severity` and `ruleset.Level`.
//     The kernel ships those as of skillet v0.23.0, and the predicate is still not
//     buildable: nothing in `gnosis_claims` declares a severity or a level, so there is
//     nothing to compare. It applies to a corpus of *rule* documents and this one holds
//     claims — which is also why `ruleset/conflict.Find` is the wrong shape rather than
//     the missing piece, since it takes a `ruleset.Ruleset`.
//   - **identity collision** is already reported, by `identity` and by the promote
//     gate's duplication signal. Adding a third reporter would make one problem read as
//     three.
//   - **interval conflict** is implemented, in interval.go.
//   - **enumeration conflict** is *subsumed* by it rather than absent: with the operator
//     set as it stands, two claims asserting `==` on one subject with different values
//     are two disjoint intervals, and the interval predicate reports them. A second
//     predicate firing on the same pair would report one problem twice. It separates
//     only if a pattern ever yields a set-valued operator, and none does.
//
// **Findings, never verdicts** (§10.2.2). Nothing blocks on a conflict; §10.6
// adjudicates, and §10.3 routes what survives the decidable predicates to the critic
// rather than to a threshold.
func conflictCheck() Check {
	return Check{
		Name:       "conflict",
		Categories: []string{"evidence-divergence", "conflict"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    somethingToCompare,
		Run: func(snap *Snapshot) []finding.Diagnostic {
			return append(divergentEvidence(snap), intervalConflicts(snap)...)
		},
	}
}

// somethingToCompare reports whether either predicate has inputs.
//
// Requires: nothing.
// Ensures: a reason whenever it declines, naming what both predicates would have needed.
// Pure.
//
// **It asks for *either*, and the first version asked only for the first.** This check
// began with evidence divergence and gated on archived sources; adding the interval
// predicate made that too narrow, so a corpus stating two contradictory bounds and having
// fetched nothing would have been told there was nothing to examine. Derived
// applicability has to track what the check actually does, which is the failure mode §12
// warns about pointed at itself.
func somethingToCompare(snap *Snapshot) (bool, string) {
	if traceableEvidence(snap) || len(snap.Bounds) > 0 {
		return true, ""
	}
	return false, "no claim's evidence can be traced to a source version, and no claim's" +
		" prose parses to a bound — there is nothing two claims could disagree about"
}

// traceableEvidence reports whether any claim's evidence can be traced to a version.
func traceableEvidence(snap *Snapshot) bool {
	if len(snap.Sources) == 0 {
		return false
	}
	for i := range snap.Documents {
		claims := snap.Documents[i].Claims
		for j := range claims {
			if len(claims[j].ArchivePaths) > 0 {
				return true
			}
		}
	}
	return false
}

// divergentEvidence reports one claim asserted twice on different versions of one source.
//
// Requires: snap.Sources maps archive paths to their source and version.
// Ensures: one diagnostic per (claim text, source URI) pair that diverges, sorted, and
// each showing its reasoning. Pure.
//
// **This is what "archived texts that disagree" is decidable as.** The corpus holds the
// same assertion in two places, and the two rest on snapshots of one page that are not
// the same bytes. Whether the page changed in the passage that matters is exactly the
// question nobody has asked — and if it did, one of the two claims is now unsupported
// while both still read as evidenced.
//
// It is not a semantic comparison and cannot become one. §10.3 refuses a similarity
// threshold over an embedding and routes real contradiction to the critic; what is left
// for a deterministic predicate is byte identity, which this is.
func divergentEvidence(snap *Snapshot) []finding.Diagnostic {
	byClaim := sitesByClaimAndSource(snap)

	out := make([]finding.Diagnostic, 0)
	for text, byURI := range byClaim {
		for uri, sites := range byURI {
			if d := divergence(text, uri, sites); d != nil {
				out = append(out, *d)
			}
		}
	}
	// Sorted because the grouping came out of maps, and `--check` rests on two runs
	// over one corpus reporting alike.
	sort.Slice(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	return out
}

// divergence reports whether these sites rest on differing bytes, or nil.
//
// Requires: sites all carry one claim text and one source URI.
// Ensures: nil unless at least two distinct version hashes appear. Two sites on the
// *same* hash are corroboration and say nothing. Pure.
func divergence(text, uri string, sites []claimSite) *finding.Diagnostic {
	versions := map[string][]claimSite{}
	for _, s := range sites {
		versions[s.SHA256] = append(versions[s.SHA256], s)
	}
	if len(versions) < 2 {
		return nil
	}
	hashes := make([]string, 0, len(versions))
	for h := range versions {
		hashes = append(hashes, h)
	}
	sort.Strings(hashes)

	// The reasoning, shown rather than summarised (§10.2.2). A finding a reader can
	// dismiss in seconds needs the two versions and where each is asserted; one that
	// shows only a verdict erodes trust in the whole queue.
	var b strings.Builder
	b.WriteString("the same claim rests on ")
	b.WriteString(Noun(len(versions), "version"))
	b.WriteString(" of ")
	b.WriteString(uri)
	b.WriteString(": ")
	for i, h := range hashes {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(h[:min(12, len(h))])
		b.WriteString(" in ")
		b.WriteString(strings.Join(pathsOf(versions[h]), ", "))
	}
	b.WriteString(" — ")
	b.WriteString(excerpt(text))
	b.WriteString(". Nothing has compared the two, so if the passage changed, one of" +
		" them is unsupported while both still read as evidenced;" +
		" `gnosis fetch --recheck` answers it")

	return &finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "evidence-divergence",
		Path:     pathsOf(sites)[0],
		Message:  b.String(),
		Action:   finding.ActionHuman,
	}
}

// pathsOf lists the documents these sites are in, sorted and deduplicated.
func pathsOf(sites []claimSite) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(sites))
	for _, s := range sites {
		if !seen[s.Path] {
			seen[s.Path] = true
			out = append(out, s.Path)
		}
	}
	sort.Strings(out)
	return out
}

// sitesByClaimAndSource groups every assertion by its claim text and its source.
//
// Requires: snap.Sources maps archive paths to their source and version.
// Ensures: keyed by folded claim text, then by source URI. A claim naming an archive
// path with no fetch record contributes nothing — its evidence cannot be traced to a
// version, so there is nothing to compare it against. Pure.
func sitesByClaimAndSource(snap *Snapshot) map[string]map[string][]claimSite {
	out := map[string]map[string][]claimSite{}
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		for j := range doc.Claims {
			claim := &doc.Claims[j]
			text := textnorm.Fold(strings.TrimSpace(claim.Anchor))
			if text == "" {
				continue
			}
			if out[text] == nil {
				out[text] = map[string][]claimSite{}
			}
			addSites(out[text], snap, doc.Path, claim)
		}
	}
	return out
}

// addSites records one claim's assertions under each source it rests on.
func addSites(byURI map[string][]claimSite, snap *Snapshot, path string, claim *Claim) {
	for _, rel := range claim.ArchivePaths {
		src, known := snap.Sources[rel]
		if !known || src.URI == "" || src.SHA256 == "" {
			continue
		}
		byURI[src.URI] = append(byURI[src.URI],
			claimSite{Path: path, ClaimID: claim.ID, SHA256: src.SHA256})
	}
}
