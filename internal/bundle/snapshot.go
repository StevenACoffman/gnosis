package bundle

import (
	"errors"
	"io/fs"
	"sort"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/constraint"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/gnosis/internal/ontology"
	"github.com/StevenACoffman/gnosis/internal/standards"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/markdown"
)

// conceptPrefix is the bundle-absolute prefix OKF §6.1 recommends for links
// between concepts. A link that does not start with it is treated as external.
const conceptPrefix = "/" + conceptDir + "/"

// Snapshot assembles everything the checks need.
//
// This is the composition the shell exists for: read the bundle, project it into
// the shapes the pure core consumes, and hand over. It decides no severity and
// reports no finding — those belong to internal/lint, which is why this function
// returns a Snapshot rather than a Report.
//
// Requires: fsys is rooted at the bundle; idx describes the same bundle, and its
// zero value states that the bundle has no index.
// Ensures: a bundle with no concepts yields a valid empty Snapshot rather than an
// error, because every command must work against a freshly initialised bundle.
func Snapshot(fsys fs.FS, idx IndexState, fresh FreshnessState) (*lint.Snapshot, error) {
	const op = "bundle.Snapshot"

	docs, err := Load(fsys)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	logLines, hasLog, err := LoadLog(fsys)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}

	archived, err := archivedPaths(fsys)
	if err != nil {
		return nil, err
	}
	recorded, err := recordedPaths(fsys)
	if err != nil {
		return nil, err
	}
	sources, err := sourceVersions(fsys)
	if err != nil {
		return nil, err
	}
	support, err := sourceSupport(fsys)
	if err != nil {
		return nil, err
	}

	return &lint.Snapshot{
		Documents:       documents(docs, support),
		ArchivedText:    archived,
		RecordedText:    recorded,
		SourceChecks:    fresh.Checks,
		StalenessDays:   fresh.StalenessDays,
		Links:           links(docs),
		Vocabulary:      vocabulary(fsys),
		Strength:        strengthMarkers(fsys),
		Registers:       registerWords(fsys),
		Indicators:      indicatorWords(fsys),
		SchemaDoc:       schemaDocText(fsys),
		LanguageMarkers: languageMarkers(fsys),
		ArchiveText:     citedArchiveText(fsys, docs),
		Bounds:          claimBounds(fsys, docs),
		Sources:         sources,
		Authority:       authorityOf(docs),
		InDegreeCut:     inDegreeCut(fsys),
		ChallengeDays:   challengeWindow(fsys),
		Resolutions:     gnosis.Reconcile(Observed(docs), idx.Rows),
		SchemaVersion:   gnosis.SchemaVersion,
		LogLines:        logLines,
		HasLog:          hasLog,
		HasIndex:        idx.Present,
	}, nil
}

// documents projects loaded files into the check-facing shape.
//
// support maps a source — by archive path and by URI — to what it buys, so the
// projection can say what each document rests on without a check reading a record.
func documents(docs []Document, support map[string]lint.Evidence) []lint.Document {
	out := make([]lint.Document, 0, len(docs))
	for i := range docs {
		d := &docs[i]
		out = append(out, lint.Document{
			ID: d.ID, Path: d.Path, Type: d.Type, Title: d.Title,
			Body: d.Body, SchemaVersion: d.SchemaVersion,
			Claims:     claimRefs(d.Claims),
			StaleAfter: d.StaleAfter, SourceKeys: d.SourceKeys,
			Limitations: d.Limitations,
			Challenges:  d.Challenges,
			Conflicts:   d.Conflicts,
			Evidence:    evidenceOf(d, support),
		})
	}
	return out
}

// evidenceOf is what one document's assertions rest on (§14.4).
//
// Requires: support maps an archive path and a source URI to what each buys.
// Ensures: one entry per distinct source the document names, sorted, and nil for a
// document naming none. Pure.
//
// **Two routes in, because the two dispositions arrive by different keys.** A claim
// names an `archive_paths` entry, which exists only for a source whose text was
// stored; a `referenced` source has no archived text and can therefore only be named
// in the document's OKF `sources` list. Reading one route would make the signal
// always `provable`, and reading the other would miss every claim-level citation.
//
// Deduplicated by URI, because a source fetched twice has two records and a document
// citing both cites one source — the same reason `citedSources` deduplicates, and the
// same failure if it did not: one page reading as corroboration.
func evidenceOf(doc *Document, support map[string]lint.Evidence) []lint.Evidence {
	seen := map[string]gnosis.Support{}
	cite := func(key string) {
		// A key tier 0 has no record of buys nothing. That is not this signal's
		// finding: `gate:provenance` decides whether a source is followable and
		// `archive-closure` reports an address naming no record, so counting it
		// here would name one gap twice.
		e, ok := support[key]
		if ok && e.Support > seen[e.URI] {
			seen[e.URI] = e.Support
		}
	}
	for i := range doc.Claims {
		for _, p := range doc.Claims[i].ArchivePaths {
			cite(p)
		}
	}
	for _, res := range doc.Resources {
		cite(res)
	}
	if len(seen) == 0 {
		return nil
	}
	uris := make([]string, 0, len(seen))
	for uri := range seen {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	out := make([]lint.Evidence, 0, len(uris))
	for _, uri := range uris {
		out = append(out, lint.Evidence{URI: uri, Support: seen[uri]})
	}
	return out
}

// sourceSupport maps every key a document can cite to the source it names and what
// that source buys (§14.4).
//
// Requires: fsys is rooted at the bundle.
// Ensures: an entry per archive path **and** per source URI tier 0 holds a record
// for; empty rather than nil for a corpus that has fetched nothing.
//
// **Two keys per record, because a document can cite either.** A claim names an
// archive path; the OKF `sources` list names the URI. One map with both keys is
// smaller than two maps and cannot fall out of agreement with itself.
//
// **The strongest record for a URI wins.** A source fetched twice may be archived
// once and referenced once — it grew past the per-file cap, or the extractor stopped
// applying — and a quotation checked against the archived version is still checkable.
// Taking the weakest would report a claim as unprovable while its evidence sits in
// tier 0.
//
// Which dispositions are durable is `archive.Disposition.Durable()`'s decision and is
// deliberately not re-stated here: that package owns the dispositions, and a second
// list of the durable ones is the information leakage a shared decision in two modules
// always becomes.
func sourceSupport(fsys fs.FS) (map[string]lint.Evidence, error) {
	const op = "bundle.sourceSupport"

	out := map[string]lint.Evidence{}
	put := func(key, uri string, s gnosis.Support) {
		if key == "" {
			return
		}
		if prev, ok := out[key]; ok && prev.Support >= s {
			return
		}
		out[key] = lint.Evidence{URI: uri, Support: s}
	}
	err := walkRecords(op, fsys, func(rec *archive.Record) {
		s := gnosis.SupportWeak
		if rec.Disposition.Durable() {
			s = gnosis.SupportDurable
		}
		put(rec.URI, rec.URI, s)
		put(rec.ArchivePath, rec.URI, s)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// authorityOf derives the corpus's adjudication authority from its adjudicators
// (§10.6.1).
//
// Requires: docs came from Load.
// Ensures: the authority for the count of distinct `human:` actors in the corpus's
// warrants and verification lists. Never an error: a corpus with no adjudicators is
// `sole`, which requires nothing.
//
// **Co-signers count.** Somebody who signed a decision is an adjudicator of it, and
// counting only the first signer would let a corpus with three active reviewers fold
// to `sole` because one person happened to open every adjudication.
//
// **Only `human:` actors**, on §10.6.4's own terms: that section counts *distinct human
// actors* to decide whether an authority amplified anything, and a kind that could pass
// for a person makes the count wrong in the direction that flatters the corpus. This is
// the one place `gnosis.Actor`'s closed enum is load-bearing, and `IsHumanActor` is the
// permissive read that answers the same question of a string somebody else wrote.
//
// **The declared window of §10.6 is not implemented, and the consequence is stated
// rather than papered over.** §10.6 counts actors *within a declared window*, and
// nothing declares one — inventing a number is what §6.2 exists to prevent, and this
// one would decide whether a colleague who left last year still holds the corpus at
// `paired`. So the count is over the whole history today, which means the authority can
// **rise and never fall**. The trigger for building the window is observable and needs
// no threshold: the first corpus whose authority requires a co-signer nobody active can
// supply. Until then the override of §10.6.3 is the escape hatch, and it leaves the
// trail that would make the case.
func authorityOf(docs []Document) gnosis.Authority {
	return gnosis.FoldAuthority(len(adjudicators(docs)))
}

// adjudicators counts the distinct human actors a corpus has adjudicated with.
//
// Requires: docs came from Load.
// Ensures: the number of distinct `human:`-prefixed actors in the corpus's warrants
// and verification lists. Pure.
//
// Separate from the fold so `doctor` can report the count beside the authority: a
// reader told the corpus is at `paired` cannot tell whether that is two people or
// three, and only one of those is one departure away from `sole`.
//
// **The set rather than the count**, because `adjudicate` needs to know what the
// authority will be *after* a decision it is about to write: adding the signers to this
// and folding again answers that without reading the corpus a second time, and reading
// it twice would let the two answers disagree about a document written in between.
func adjudicators(docs []Document) map[string]bool {
	people := map[string]bool{}
	note := func(by string) {
		if gnosis.IsHumanActor(by) {
			people[by] = true
		}
	}
	for i := range docs {
		for j := range docs[i].Claims {
			claim := &docs[i].Claims[j]
			note(claim.Warrant.By)
			note(claim.Warrant.CoSignedBy)
			for _, v := range claim.Verified {
				note(v.By)
			}
		}
	}
	return people
}

// challengeWindow reads §10.7.3's unanswered window, falling back to the seed.
//
// Zero when the file will not load, which **skips** the check: a window of zero would
// report every challenge the moment it was filed, which is indistinguishable from
// having no window at all.
func challengeWindow(fsys fs.FS) int {
	raw, err := fs.ReadFile(fsys, standards.ChallengeFileName)
	if err != nil {
		raw = standards.DefaultChallenge()
	}
	in, err := standards.LoadChallenge(raw)
	if err != nil {
		return 0
	}
	return in.UnansweredDays.Value
}

// inDegreeCut reads §14.4.1's centrality cut, falling back to the seed.
//
// Zero when the file will not load, which **skips** the durability check: an
// in-degree of zero is at or above a cut of zero, so a missing threshold would
// otherwise report every document in the corpus as load-bearing.
func inDegreeCut(fsys fs.FS) int {
	raw, err := fs.ReadFile(fsys, standards.ArchiveFileName)
	if err != nil {
		raw = standards.DefaultArchive()
	}
	in, err := standards.LoadArchive(raw)
	if err != nil {
		return 0
	}
	return in.InDegreeCut.Value
}

// claimRefs projects a document's claims into the check-facing shape.
func claimRefs(claims []DocClaim) []lint.Claim {
	if len(claims) == 0 {
		return nil
	}
	out := make([]lint.Claim, 0, len(claims))
	for i := range claims {
		out = append(out, lint.Claim{
			ID:           claims[i].ID,
			Anchor:       claims[i].Anchor,
			Subject:      claims[i].Subject,
			Lead:         claims[i].Lead,
			Quotes:       claims[i].Quotes,
			Warrant:      claims[i].Warrant,
			Supersedes:   claims[i].Supersedes,
			ArchivePaths: claims[i].ArchivePaths,
		})
	}
	return out
}

// archivedPaths is the set of archived text files present, for the archive-path
// check to resolve claim addresses against.
//
// Walked through the same fs.FS as the documents, so a check never touches a disk
// and a caller testing with an fstest.MapFS gets the same answers as one reading a
// bundle. An absent archive is an empty set rather than an error: a corpus that has
// fetched nothing has no dangling paths, only claims that will report as dangling
// the moment they name one.
func archivedPaths(fsys fs.FS) (map[string]bool, error) {
	const op = "bundle.archivedPaths"

	out := map[string]bool{}
	err := fs.WalkDir(fsys, archive.TextDir, func(name string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return fs.SkipAll
		case err != nil:
			return err
		case d.IsDir():
			return nil
		}
		out[name] = true
		return nil
	})
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}

// recordedPaths is the set of archive paths tier 0's fetch records name.
//
// Requires: fsys is rooted at the bundle.
// Ensures: one entry per distinct archive path any record names, and an empty set
// for a bundle with no records rather than an error — a corpus that has fetched
// nothing has nothing to account for.
//
// A `referenced` record names no archive path and contributes nothing, which is
// correct: it stored no text, so there is no file for it to account for. That is why
// this reads the records rather than counting them.
//
// Walked through the same fs.FS as the documents, so the closure check never touches
// a disk and a caller testing with an fstest.MapFS gets the same answers as one
// reading a bundle.
func recordedPaths(fsys fs.FS) (map[string]bool, error) {
	const op = "bundle.recordedPaths"

	out := map[string]bool{}
	err := walkRecords(op, fsys, func(rec *archive.Record) {
		if rec.ArchivePath != "" {
			out[rec.ArchivePath] = true
		}
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// claimBounds parses every claim's prose into the reading the predicates compare.
//
// Requires: fsys is rooted at the bundle; docs came from Load.
// Ensures: empty when the bundle declares no vocabulary or its operator patterns will
// not load — in which case every claim parses to nothing and the predicates skip, rather
// than comparing readings nobody can trust.
//
// **Read through the fs.FS like the vocabulary and the strength markers**, so a caller
// testing with an fstest.MapFS gets the same answers as one reading a disk.
func claimBounds(fsys fs.FS, docs []Document) map[string]*lint.Bound {
	out := map[string]*lint.Bound{}
	o := ontologyFrom(fsys)
	if o == nil {
		return out
	}
	for id, cs := range ClaimConstraints(docs, o, operatorPatternsFrom(fsys)) {
		b := &lint.Bound{
			SubjectKey: cs.SubjectKey,
			Dimension:  cs.Dimension,
			Pinned:     cs.Pinned,
		}
		if effective, ok := cs.Effective(); ok {
			b.Op, b.Value = string(effective.Op), effective.Value
			b.Raw, b.PatternID = effective.Raw, effective.PatternID
			if written, ok := ontology.DimensionWritten(effective.Raw); ok {
				b.Written = string(written)
			}
		}
		// Only a pinned claim's text is normalised: it is the only text anything
		// compares a value against, and normalising the corpus would put every
		// claim's rewritten wording in the snapshot beside the author's own.
		if b.Pinned {
			b.ProseText = constraint.NormalizeNumbers(anchorOf(docs, id))
		}
		out[id] = b
	}
	return out
}

// ontologyFrom loads the vocabulary through a filesystem, or nil.
func ontologyFrom(fsys fs.FS) *ontology.Ontology {
	raw, err := fs.ReadFile(fsys, ontology.FileName)
	if err != nil {
		return nil
	}
	o, err := ontology.Load(raw)
	if err != nil {
		return nil
	}
	return o
}

// operatorPatternsFrom loads the operator patterns through a filesystem, falling back to
// the seed and yielding nothing when a present file will not parse.
func operatorPatternsFrom(fsys fs.FS) []constraint.Pattern {
	raw, err := fs.ReadFile(fsys, standards.OperatorsFileName)
	if err != nil {
		raw = standards.DefaultOperators()
	}
	in, err := standards.LoadOperators(raw)
	if err != nil {
		return nil
	}
	out := make([]constraint.Pattern, 0, len(in.Pattern))
	for _, p := range in.Pattern {
		out = append(out, constraint.Pattern{
			ID: p.ID, Phrase: p.Phrase, Op: constraint.OpKind(p.Op),
		})
	}
	return out
}

// citedArchiveText reads the archived files claims name, and only those.
//
// Requires: fsys is rooted at the bundle; docs came from Load.
// Ensures: one entry per readable cited path. A path no claim names is not read, and a
// cited path that will not open is simply absent — `archive-path` reports that, and
// failing here would report one fault twice.
//
// **Bounded by what is cited rather than by what is archived**, because the archive is
// the largest thing a bundle owns and this needs a handful of files out of it.
func citedArchiveText(fsys fs.FS, docs []Document) map[string]string {
	wanted := map[string]bool{}
	for i := range docs {
		for j := range docs[i].Claims {
			claim := &docs[i].Claims[j]
			if len(claim.Quotes) == 0 {
				continue
			}
			for _, p := range claim.ArchivePaths {
				wanted[p] = true
			}
		}
	}
	out := make(map[string]string, len(wanted))
	for p := range wanted {
		if raw, err := fs.ReadFile(fsys, p); err == nil {
			out[p] = string(raw)
		}
	}
	return out
}

// schemaDocText reads AGENTS.md, treating absence as empty.
//
// Requires: fsys is rooted at the bundle.
// Ensures: "" when the document is absent, which skips the command check with a reason
// rather than reporting a bundle that has not run `gnosis schema` yet.
func schemaDocText(fsys fs.FS) string {
	raw, err := fs.ReadFile(fsys, SchemaFile)
	if err != nil {
		return ""
	}
	return string(raw)
}

// sourceVersions maps each archived file to the source and version it holds.
//
// Requires: fsys is rooted at the bundle.
// Ensures: one entry per archive path any record names, and an empty map for a corpus
// that has fetched nothing rather than an error.
//
// A `referenced` record names no archive path and contributes nothing, which is correct:
// it stored no text, so no claim can rest on it. That is why this reads the records
// rather than walking `evidence/text/` — the files say what they contain and only the
// records say where they came from.
func sourceVersions(fsys fs.FS) (map[string]lint.SourceVersion, error) {
	const op = "bundle.sourceVersions"

	out := map[string]lint.SourceVersion{}
	err := walkRecords(op, fsys, func(rec *archive.Record) {
		if rec.ArchivePath == "" {
			return
		}
		out[rec.ArchivePath] = lint.SourceVersion{
			URI: rec.URI, SHA256: rec.SourceSHA256,
		}
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// links extracts every link from every body and resolves each against the
// documents present.
//
// Resolution is by identifier parsed out of the href's filename, not by path
// lookup: a link written before a retitle still points at the old slug, and the
// immutable prefix is what lets it keep working (SPEC §5.1.1). A href naming an
// identifier no document carries resolves to nothing, which is legal.
func links(docs []Document) []lint.Link {
	present := make(map[gnosis.ID]bool, len(docs))
	for i := range docs {
		if docs[i].ID != "" {
			present[docs[i].ID] = true
		}
	}

	out := make([]lint.Link, 0)
	for i := range docs {
		d := &docs[i]
		if d.ID == "" {
			continue
		}
		for _, href := range markdown.Parse(d.Body).Links {
			link := lint.Link{FromID: d.ID, Href: href}
			if target, ok := targetID(href); ok {
				link.External = false
				if present[target] {
					link.ToID = target
				}
			} else {
				link.External = true
			}
			out = append(out, link)
		}
	}
	return out
}

// targetID recovers the identifier a concept link points at.
//
// Requires: nothing.
// Ensures: reports false for any href that is not a bundle-absolute concept
// path, which includes every external URL and every relative link. A concept
// filename is "<uuid>-<slug>.md", so the identifier is the leading segment.
func targetID(href string) (gnosis.ID, bool) {
	if !strings.HasPrefix(href, conceptPrefix) {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(href, conceptPrefix), ".md")
	// The identifier is 36 characters; a slug follows it after a hyphen.
	if len(name) < 36 {
		return "", false
	}
	id, err := gnosis.ParseID(name[:36])
	if err != nil {
		return "", false
	}
	return id, true
}

// LinkRows projects the resolved link graph into index rows.
//
// Requires: docs came from Load.
// Ensures: one row per link found in an identified document's body, with the
// target filled in only where it resolves. Links out of an unidentified document
// are omitted for the same reason its document row is (§5.1.2): it is
// quarantined, not indexed, and indexing its links would give it a presence in
// the graph it has not earned.
func LinkRows(docs []Document) []index.LinkRow {
	out := make([]index.LinkRow, 0)
	for _, l := range links(docs) {
		out = append(out, index.LinkRow{
			SourceID: l.FromID,
			TargetID: l.ToID,
			Href:     l.Href,
			External: l.External,
		})
	}
	return out
}

// Reconciled compares loaded documents against index rows.
//
// Requires: docs came from Load.
// Ensures: the outcome of gnosis.Reconcile over the projection, so a caller need
// not know how an observation is shaped.
func Reconciled(docs []Document, indexed []gnosis.Indexed) []gnosis.Resolution {
	return gnosis.Reconcile(Observed(docs), indexed)
}

// ClaimRefsForTest exposes the claim projection so a test can assert that every declared
// field reaches the snapshot.
//
// **A test-only export, which this codebase otherwise refuses**, and the reason it earns
// an exception is the failure it exists to catch: the `lead` check was correct and
// silently examined an empty field for a day because this projection never copied it.
// The alternative — asserting through `Snapshot`, which needs a bundle on disk — would
// make the test slower and no more revealing, since what is being checked is one
// assignment list.
func ClaimRefsForTest(claims []DocClaim) []lint.Claim { return claimRefs(claims) }

// DocumentsForTest exposes the document projection, for the same reason ClaimRefsForTest
// does: this seam is where a declared field goes missing silently, and asserting through
// Snapshot would need a bundle on disk to check one assignment list.
//
// It takes the support map because `Evidence` is derived from it rather than copied off
// a Document field, so a test that could not supply one could not see that projection
// at all — which is the failure this seam exists to catch.
func DocumentsForTest(docs []Document, support map[string]lint.Evidence) []lint.Document {
	return documents(docs, support)
}

// anchorOf returns one claim's anchor, or empty when the claim is gone.
//
// A linear walk rather than an index: it runs once per *pinned* claim, and §10.2.1 keeps
// pins opt-in precisely so most claims never carry one. Building a map to serve a handful
// of lookups would be the more expensive answer on every corpus that has none.
func anchorOf(docs []Document, claimID string) string {
	for i := range docs {
		for j := range docs[i].Claims {
			if docs[i].Claims[j].ID == claimID {
				return docs[i].Claims[j].Anchor
			}
		}
	}
	return ""
}
