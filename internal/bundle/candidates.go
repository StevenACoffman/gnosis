package bundle

import (
	"context"
	"os"
	"sort"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/finding"
)

// candidateLimit bounds a selection.
//
// Twenty-five, which is under the ~47 candidates §6.2 opens by refusing to scan and well
// over what a source usually touches. It is a display and cost bound rather than a
// judgement about relevance: the rules decide what is a candidate, and this decides how
// many a person is asked to read at once.
const candidateLimit = 25

// Selection is what §6.2's rules found, and what they could not run.
//
// **The gap travels with the answer**, which is `finding.Unexamined`'s discipline applied
// to a query: a candidate set that silently omitted a rule would present a partial
// selection as a whole one, and the reader's only clue would be a concept that never
// appeared.
type Selection struct {
	// Candidates are the claims an incoming source might update, best rule first.
	Candidates []index.Candidate `json:"candidates"`

	// NotRun names the rules that did not execute and why not.
	NotRun []finding.Unexamined `json:"not_run,omitempty"`
}

// Candidates selects the claims an incoming source might update (§6.2).
//
// Requires: bundleDir is a bundle with an index; archivePaths are the tier-0 files the
// incoming source produced; concept is the document the source is becoming or updating,
// or empty.
// Ensures: a deterministic set — the same corpus state produces the same candidates,
// because every rule is a query over a build artifact or over committed frontmatter.
// Rules that cannot run are reported rather than dropped.
//
// # Selection is a query, not a judgment
//
// §6.2 opens by refusing `llmwiki`'s design: "scans ~47 candidate pages per ingest and
// asks a model, per candidate, whether it needs updating — up to 50 of roughly 65 calls.
// `gnosis` MUST NOT do this." Nothing here consults a model, and each candidate names the
// rule that found it so a reader can check the selection rather than trust it.
//
// # It selects and never binds
//
// §6.2 chooses candidates; §6.3 accretes into a document a person named. A selector that
// picked one would be making the judgement this section removes — and the failure would
// be silent, because a wrong choice looks exactly like a right one until somebody reads
// the diff.
func Candidates(
	ctx context.Context, bundleDir string, archivePaths []string, concept gnosis.ID,
) (*Selection, error) {
	const op = "bundle.Candidates"

	db, err := OpenIndexForRead(ctx, bundleDir)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = db.Close() }()

	found, err := db.CandidatesFor(ctx, archivePaths, candidateLimit)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	out := &Selection{Candidates: found, NotRun: []finding.Unexamined{tagRuleGap()}}
	if concept == "" {
		return out, nil
	}
	edges, err := conflictCandidates(bundleDir, concept, found)
	if err != nil {
		return nil, err
	}
	out.Candidates = append(out.Candidates, edges...)
	return out, nil
}

// conflictCandidates is §6.2's fourth rule: claims whose deferred conflicts name the
// incoming concept.
//
// Requires: concept is the identifier the incoming source is becoming or updating.
// Ensures: one candidate per claim in a document deferring a conflict with that concept,
// excluding claims the earlier rules already found.
//
// **Read from the corpus rather than from the index**, because `gnosis_conflicts` is
// committed frontmatter and no projection holds it. §6.2's determinism is unaffected: the
// same corpus state produces the same edges, which is what that section requires of a
// rule rather than that it be SQL.
func conflictCandidates(
	bundleDir string, concept gnosis.ID, already []index.Candidate,
) ([]index.Candidate, error) {
	const op = "bundle.conflictCandidates"

	docs, err := Load(os.DirFS(bundleDir))
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	seen := make(map[string]bool, len(already))
	for _, c := range already {
		seen[c.ClaimID] = true
	}

	var out []index.Candidate
	for i := range docs {
		doc := &docs[i]
		if !defersConflictWith(doc, concept) {
			continue
		}
		for j := range doc.Claims {
			if seen[doc.Claims[j].ID] {
				continue
			}
			out = append(out, index.Candidate{
				ClaimID: doc.Claims[j].ID, DocumentID: doc.ID.String(),
				Path: doc.Path, Title: doc.Title, Lead: doc.Claims[j].Lead,
				Rule: index.RuleConflictEdge,
			})
		}
	}
	return out, nil
}

// defersConflictWith reports whether a document defers a conflict naming this concept.
//
// Only a valid edge counts, which is `Defers`'s rule and the safe direction: a
// half-written entry selects nothing rather than pulling a document into every selection
// on the strength of a record nobody finished.
func defersConflictWith(doc *Document, concept gnosis.ID) bool {
	for i := range doc.Conflicts {
		if doc.Conflicts[i].Valid() && doc.Conflicts[i].Concept == concept {
			return true
		}
	}
	return false
}

// tagRuleGap is §6.2's third rule, named as not run.
//
// **Two blockers, and neither is one this can remove.** The rule needs a *tag*
// projection — §5.5 records `tags` as a table with no migration and no reader, which is
// why §11.3's `--tag` filter was refused — and a *declared rank cut*, which §6.2 puts in
// `standards/` and forbids as a literal in Go. Nobody can write that value's rationale
// from measurement until a corpus has enough tagged claims to measure.
//
// Reported rather than omitted, because a selection that quietly ran three rules of four
// would look complete. The reason is the half that makes the gap actionable, which is
// what `finding.Unexamined` requires of every entry.
func tagRuleGap() finding.Unexamined {
	return finding.Unexamined{
		Aspect: "§6.2's third rule: claims sharing a tag and an FTS5 term above a" +
			" declared rank cut",
		Reason: "`tags` has no projection to query (§5.5) and the rank cut is a" +
			" standards value nobody can write from measurement yet (§6.5), which" +
			" §6.2 forbids inventing as a literal",
	}
}

// ArchivePathsFor is the tier-0 text these source URIs produced.
//
// Requires: bundleDir is a bundle; uris are source URIs as a caller named them.
// Ensures: the archive paths of the durable records for those URIs, deduplicated and
// sorted, so a selection over them is stable. Empty for sources nothing has fetched.
//
// **It reads the committed records rather than the index projection**, which is the same
// choice `recordsByURI` makes one file over and for its reason: tier 0 is the authority
// and `sources_fetched` is a rollup, so a selection computed before the next rebuild
// would otherwise miss a source fetched a minute ago.
func ArchivePathsFor(bundleDir string, uris []string) ([]string, error) {
	const op = "bundle.ArchivePathsFor"

	records, err := recordsByURI(op, bundleDir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(uris))
	for _, uri := range uris {
		rec, ok := records[uri]
		if !ok || rec.ArchivePath == "" || seen[rec.ArchivePath] {
			continue
		}
		seen[rec.ArchivePath] = true
		out = append(out, rec.ArchivePath)
	}
	sort.Strings(out)
	return out, nil
}
