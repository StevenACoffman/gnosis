package bundle

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/okf"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/quotecheck"
)

// CategoryDriftUnsupported is the finding category a re-check opens when a source no
// longer contains a passage this corpus quotes (§14.3.2).
//
// **It is a constant because it is a finding category `lint.Checks()` cannot hold.**
// `spec_test.go` walks that registry against §12.1's table in both directions, which
// is what keeps the table honest — and this category is emitted by a command that does
// network I/O, which §4.6 forbids a check from doing. So the walk cannot see it.
//
// The tempting fix was to register a no-network "check" so it appeared in the
// registry. That is worse while looking better: §12.1's table is what `lint` runs, and
// a row for something `lint` cannot run buys enumerability by putting a false
// statement in the document the test exists to keep true.
//
// So the honest form is this constant and a second, smaller assertion: §14.3.2.1 names
// the category, and a test checks that it does. The registry stops being the whole
// answer, which is the part worth writing down — a reader auditing the vocabulary now
// has two places to look, and both are checked.
const CategoryDriftUnsupported = "drift-unsupported"

// Quoted is one claim's quotations of one archived source.
//
// It carries the document path because a finding has to land somewhere a person can
// open, and the claim id because §14.3.2 asks for "a finding per affected claim": the
// source is what drifted, but the claim is what lost its support.
type Quoted struct {
	Path    string
	ClaimID string
	Quotes  []string
}

// RecheckTarget is one recorded source and what rests on it.
type RecheckTarget struct {
	URI          string
	SourceSHA256 string

	// ArchivePath is where the text this corpus validated against lives, or empty
	// for a `referenced` source that kept none. Empty is not a defect: §4.3 makes
	// `referenced` a supported outcome, and such a source can still be re-fetched
	// and its hash compared — there is simply no local text for the passages to
	// have come from, so the passages are whatever the claims recorded.
	ArchivePath string

	// Resting is every claim quoting this source, gathered from the documents
	// rather than from the index, for the reason claimsOf reads frontmatter: the
	// index is a derived cache, and a check resting on it would be checking
	// something rebuildable rather than what is committed.
	Resting []Quoted
}

// Rechecked is what one source's re-fetch established.
type Rechecked struct {
	URI string

	// SourceSHA256 is the *recorded* version this verdict is about, not the bytes
	// just fetched. The verdict answers "do the passages taken from this archived
	// copy still appear upstream", so the observation it belongs on is the one for
	// this version — which is also the version a claim's archive path resolves to.
	SourceSHA256 string

	ArchivePath string
	Drift       gnosis.Drifted

	// Findings is empty for every state but drift-unsupported. §14.3.2's first
	// consequence is that `drift-benign` "is not a downgrade of trust" and
	// "rendering it as a warning would train readers past the state that matters",
	// so the benign case deliberately produces nothing to read.
	Findings []finding.Diagnostic

	// Resting is how many claims quote this source version, and it is carried
	// because it is the difference between the two ways a re-check reports
	// `drift-unchecked`.
	//
	// Zero means there was nothing to check: no claim rests on this version, so no
	// passage could have been found or lost. That is the ordinary state of every
	// version a re-check itself archives — a changed source becomes a new record
	// (§4.1) whose text no document cites yet — and those accumulate one per
	// re-check. Without this a reporter cannot tell them from a version whose
	// passages genuinely could not be re-checked, and the report of a settled corpus
	// fills up with the case that means nothing happened.
	Resting int
}

// Quotes is every quotation recorded against this source, in document order.
//
// Requires: nothing.
// Ensures: a flat list, never nil, with duplicates kept — two claims quoting the
// same sentence are two claims, and collapsing them here would lose the second
// finding. Pure.
func (t *RecheckTarget) Quotes() []string {
	out := make([]string, 0, len(t.Resting))
	for i := range t.Resting {
		out = append(out, t.Resting[i].Quotes...)
	}
	return out
}

// RecheckTargets gathers every recorded source and the claims resting on it.
//
// Requires: bundleDir is a bundle root, which need not have an archive.
// Ensures: one target per recorded source version, in the order the record walk
// found them; a corpus that has fetched nothing yields none rather than an error.
//
// **A source fetched twice is two targets, and that is correct.** Records are keyed
// by their own content, so both versions are in tier 0 (§4.1), and each carries the
// hash that was current when somebody quoted from it. Collapsing them to the newest
// would compare this run against a version no claim was ever validated against.
//
// The claims are joined by archive path, which is the address a claim actually
// records — `checked.jsonl` keys observations by `(uri, hash)` and a claim names a
// file, and the fetch record is the only artifact holding both. That is the same join
// `archiveIndex` makes and for the same reason.
func RecheckTargets(bundleDir string) ([]RecheckTarget, error) {
	const op = "bundle.RecheckTargets"

	fsys := os.DirFS(bundleDir)
	resting, err := quotedBySource(op, fsys)
	if err != nil {
		return nil, err
	}

	var out []RecheckTarget
	err = walkRecords(op, fsys, func(rec *archive.Record) {
		if rec.URI == "" {
			return
		}
		out = append(out, RecheckTarget{
			URI:          rec.URI,
			SourceSHA256: rec.SourceSHA256,
			ArchivePath:  rec.ArchivePath,
			Resting:      resting[rec.ArchivePath],
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Recheck decides one source's drift and opens the findings it calls for.
//
// Requires: target came from RecheckTargets; upstreamSHA256 is the hash of the bytes
// just fetched and upstream the text those bytes yielded, either empty when the fetch
// could not establish them.
// Ensures: exactly one finding per affected claim when the state is
// drift-unsupported, and none otherwise. Pure — the decision is `gnosis.Drift`'s and
// the fetching is the caller's (§4.6).
//
// **The text and the hash are deliberately about different bytes.** The hash question
// is "did the source move", so it compares the source's own bytes, which is what tier
// 0 recorded. The passage question is "does the archived text still appear upstream",
// and for an `extracted` source the archived text is the extractor's output — so the
// caller extracts before passing `upstream`. Comparing passages against raw HTML
// would report every claim resting on an extracted page as unsupported, which is the
// same manufactured catastrophe `gnosis.Drift` guards its empty case against.
func Recheck(target *RecheckTarget, upstreamSHA256, upstream string) Rechecked {
	drifted := gnosis.Drift(
		target.SourceSHA256, upstreamSHA256, upstream, target.Quotes())
	return Rechecked{
		URI:          target.URI,
		SourceSHA256: target.SourceSHA256,
		ArchivePath:  target.ArchivePath,
		Drift:        drifted,
		Findings:     withdrawn(target, &drifted),
		Resting:      len(target.Resting),
	}
}

// withdrawn is one finding per claim that lost a passage.
//
// Requires: drifted came from Drift over target's quotations.
// Ensures: nothing for any state but drift-unsupported; otherwise one diagnostic per
// (claim, lost passage), in document order. Pure.
//
// A claim is affected when one of *its* passages is among the missing, rather than
// when anything about the source changed. That distinction is the whole value of the
// three states: a page that dropped one paragraph should reach the one claim that
// quoted the paragraph, not every claim that ever cited the page.
//
// The severity is an error and the action is a human's. §14.3.2 calls this "the
// strongest signal §10 can receive short of a contradiction", and §14.3.2's second
// consequence is that it "never rewrites or retracts anything" — so there is nothing
// a guided fix could do, and offering one would imply the tool could restore support
// it cannot.
func withdrawn(target *RecheckTarget, drifted *gnosis.Drifted) []finding.Diagnostic {
	if drifted.State != gnosis.DriftUnsupported {
		return nil
	}
	lost := make(map[string]bool, len(drifted.Missing))
	for _, p := range drifted.Missing {
		lost[p] = true
	}

	var out []finding.Diagnostic
	for i := range target.Resting {
		q := &target.Resting[i]
		for _, passage := range quotecheck.Passages(strings.Join(q.Quotes, " ")) {
			if !lost[passage] {
				continue
			}
			out = append(out, finding.Diagnostic{
				Severity: finding.SeverityError,
				Category: CategoryDriftUnsupported,
				Path:     q.Path,
				Message: "claim " + q.ClaimID + " quotes a passage " + target.URI +
					" no longer contains: " + strconv.Quote(passage) +
					"; the archived copy is intact, so support was withdrawn " +
					"upstream rather than corrupted here",
				Action: finding.ActionHuman,
			})
		}
	}
	return out
}

// quotedBySource reads every document's claims and files their quotations under the
// archived text each names.
//
// Requires: fsys is rooted at a bundle.
// Ensures: a map from archive path to the claims quoting it, empty rather than nil. A
// document that fails to parse contributes nothing rather than failing the walk: a
// re-check is a maintenance pass over the archive, and refusing to run it because one
// unrelated document is malformed would make the corpus's worst state the one where
// its evidence goes unexamined. `lint` is what reports the malformed document.
func quotedBySource(op string, fsys fs.FS) (map[string][]Quoted, error) {
	out := map[string][]Quoted{}
	err := fs.WalkDir(fsys, conceptDir, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir(), !strings.HasSuffix(path, ".md"):
			return nil
		case isReserved(filepath.Base(path)):
			return nil
		}
		raw, rErr := fs.ReadFile(fsys, path)
		if rErr != nil {
			return &errs.Error{Op: op, Err: rErr}
		}
		// A document that will not parse contributes nothing and does not stop the
		// walk. Written as a positive test rather than as an early return on the
		// error, so nothing here looks like an error being swallowed: there is no
		// failure to report, because `lint` is what reports an unparsable document
		// and a re-check refusing to run over the archive would make the corpus's
		// worst state the one where its evidence goes unexamined.
		if parsed, pErr := okf.Parse(raw); pErr == nil {
			fileQuotes(out, path, parsed)
		}
		return nil
	})
	// An absent concept directory is an empty corpus, following Load: a bundle that
	// has fetched sources and written no documents yet is an ordinary state, and
	// every source in it re-checks to unchecked because nothing quotes it.
	if err != nil && !isNotExist(err) {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}

// fileQuotes adds one document's claims to the index of quotations by archive path.
//
// It reads through claimsOf rather than parsing `gnosis_claims` again, because two
// readers of one format agree by inspection and drift by edit — the argument §12's
// generated table makes, applied to the frontmatter the gate and this both depend on.
func fileQuotes(into map[string][]Quoted, path string, doc *okf.Document) {
	for _, claim := range claimsOf(doc) {
		if len(claim.Quotes) == 0 {
			continue
		}
		for _, archivePath := range claim.ArchivePaths {
			into[archivePath] = append(into[archivePath], Quoted{
				Path:    path,
				ClaimID: claim.ID,
				Quotes:  claim.Quotes,
			})
		}
	}
}
