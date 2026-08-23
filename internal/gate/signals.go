package gate

import (
	"strconv"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/quotecheck"
	"github.com/StevenACoffman/skillet/skilllens"
)

// evidence requires at least one validating quotation per enforced claim.
//
// The corpus's central invariant (§9.4): every offered quote MUST appear in the
// named archived file under textnorm.Fold normalization, at or above
// MinPassageWords. Validation is against **tier 0**, never against the live
// upstream — a quote checked against a URL is a proof that expires the moment the
// page moves (§4.1).
//
// A claim naming no archive path fails rather than passing vacuously. That is the
// `referenced` disposition arriving here: a source with no offline text cannot
// support an enforced claim, however reliable its publisher, and §4.3 admits such
// sources precisely on the understanding that the weakness stays visible.
//
// A document with no enforced claims passes, and the detail says so. "Nothing to
// check" and "everything checked out" are different passes, and a reader deciding
// how much the verdict is worth needs to know which one they have.
func evidence(c *Candidate, corpus *Corpus, limits Limits) Result {
	res := Result{Signal: SignalEvidence}

	var enforced, unsupported int
	var reasons []string
	for i := range c.Doc.Claims {
		claim := &c.Doc.Claims[i]
		if !claim.Enforced {
			continue
		}
		enforced++
		if why := unsupportedBecause(claim, corpus, limits); why != "" {
			unsupported++
			reasons = append(reasons, claim.ID+": "+why)
		}
	}

	switch {
	case enforced == 0:
		res.Verdict = VerdictPass
		res.Detail = "no enforced claims to check"
	case unsupported == 0:
		res.Verdict = VerdictPass
		res.Detail = strconv.Itoa(enforced) + " enforced claims, all supported by archived text"
	default:
		res.Verdict = VerdictFail
		res.Detail = strconv.Itoa(unsupported) + " of " + strconv.Itoa(enforced) +
			" enforced claims unsupported — " + strings.Join(reasons, "; ")
	}
	return res
}

// unsupportedBecause says why a claim's evidence does not hold, or "" when it
// does.
func unsupportedBecause(claim *Claim, corpus *Corpus, limits Limits) string {
	if len(claim.Quotes) == 0 {
		return "no quotation offered"
	}
	if len(claim.ArchivePaths) == 0 {
		return "no archived text to check against; the source is referenced only"
	}

	sources := make([]quotecheck.Source, 0, len(claim.ArchivePaths))
	for _, p := range claim.ArchivePaths {
		text, ok := corpus.ArchivedText[p]
		if !ok {
			return "archived text " + p + " is missing from tier 0"
		}
		sources = append(sources, quotecheck.Source{Name: p, Text: text})
	}

	// One validating quotation is enough, per §9.5. Requiring all of them would
	// make an author's extra citation a liability, which teaches them to cite
	// less.
	found := quotecheck.Support(quotecheck.Check(claim.Quotes, sources))
	if found > 0 {
		return ""
	}
	if limits.MinPassageWords > 0 {
		return "no quotation of at least " + strconv.Itoa(limits.MinPassageWords) +
			" words was found in the archived text"
	}
	return "no quotation was found in the archived text"
}

// provenance requires every source to be followable or to declare that it is not.
//
// OKF §5.1 permits a `sources[].resource` to name "a population or scope
// descriptor" a consumer cannot dereference, and such a source passes: it has said
// what it is. A URI that merely happens to be absent from tier 0 has not — it
// looks followable and is not, which is the case this signal exists to catch.
func provenance(c *Candidate, corpus *Corpus) Result {
	res := Result{Signal: SignalProvenance}

	var missing []string
	for _, s := range c.Doc.Sources {
		switch {
		case s.Scope:
			continue
		case strings.TrimSpace(s.Resource) == "":
			missing = append(missing, "(empty resource)")
		case !corpus.FetchedURIs[s.Resource]:
			missing = append(missing, s.Resource)
		}
	}

	switch {
	case len(c.Doc.Sources) == 0:
		// Not a pass. A document asserting claims and citing nothing is exactly
		// what this corpus exists to refuse, and reporting "no sources to check"
		// would let it through on a technicality.
		res.Verdict = VerdictFail
		res.Detail = "no sources declared"
	case len(missing) == 0:
		res.Verdict = VerdictPass
		res.Detail = strconv.Itoa(len(c.Doc.Sources)) + " sources, all accounted for in tier 0"
	default:
		res.Verdict = VerdictFail
		res.Detail = "sources with no fetch record: " + strings.Join(missing, ", ") +
			" — run `gnosis fetch` on them, or declare them as scope descriptors"
	}
	return res
}

// conformance requires OKF §11's structure and a non-empty type.
//
// It is the cheapest signal and the one most worth having early: a document with
// no type cannot be routed, compared, or reviewed, and every later signal would be
// answering questions about a thing the corpus has no name for.
func conformance(c *Candidate) Result {
	res := Result{Signal: SignalConformance}

	var missing []string
	if strings.TrimSpace(c.Doc.Type) == "" {
		missing = append(missing, "type")
	}
	if strings.TrimSpace(c.Doc.Title) == "" {
		missing = append(missing, "title")
	}
	if strings.TrimSpace(c.Doc.Body) == "" {
		missing = append(missing, "body")
	}
	if len(missing) > 0 {
		res.Verdict = VerdictFail
		res.Detail = "missing: " + strings.Join(missing, ", ")
		return res
	}
	res.Verdict = VerdictPass
	res.Detail = "type, title, and body present"
	return res
}

// duplication refuses a document whose title already exists in the corpus.
//
// Fold-normalised, because §4.6.1 makes this the merge reconciliation step for a
// distributed corpus rather than a hygiene check about careless copying: identity
// is assigned rather than derived (§5.1.3), so two people documenting one subject
// produce two identifiers and git merges both cleanly. Fold-equal titles is
// exactly the signal that condition leaves behind.
//
// A document that already occupies its own path is not a duplicate of itself. That
// case is a revision, and Before being non-nil is what distinguishes them.
//
// The normalizer is gnosis.Surface.Fold, not textnorm.Fold. The difference is
// case: textnorm.Fold deliberately preserves it, because case carries meaning in a
// *quotation*, and Surface.Fold lower-cases on top of it because case carries
// none in a *title*. "Cache Policy" and "cache policy" are one subject documented
// twice, which is the condition §4.6.1 asks this signal to find. The gate's own
// self-test caught this: with the quotation folder the planted duplicate was not
// detected as one.
func duplication(c *Candidate, corpus *Corpus) Result {
	res := Result{Signal: SignalDuplication}

	folded := gnosis.Surface(c.Doc.Title).Fold()
	var others []string
	for _, path := range corpus.TitlesByFold[folded] {
		if path != c.Path {
			others = append(others, path)
		}
	}
	if len(others) > 0 {
		res.Verdict = VerdictFail
		res.Detail = "title already used by " + strings.Join(others, ", ") +
			" — merge them, or retitle so the difference is stated"
		return res
	}
	res.Verdict = VerdictPass
	res.Detail = "title is not held by another document"
	return res
}

// hedging refuses a body carrying more softening phrases than the declared count.
//
// The terms come from skillet's skilllens, shared so no two tools in the family
// disagree about what hedging is. The count is a threshold rather than a
// prohibition because prose legitimately hedges — a claim that is *genuinely*
// uncertain should say so, and a corpus that banned the words would get the same
// uncertainty asserted flatly, which is worse.
func hedging(c *Candidate, limits Limits) Result {
	res := Result{Signal: SignalHedging}

	body := strings.ToLower(c.Doc.Body)
	var found []string
	for _, term := range skilllens.SofteningTerms() {
		if n := strings.Count(body, strings.ToLower(term)); n > 0 {
			found = append(found, term)
		}
	}
	if len(found) > limits.HedgingMax {
		res.Verdict = VerdictFail
		res.Detail = strconv.Itoa(len(found)) + " softening phrases (limit " +
			strconv.Itoa(limits.HedgingMax) + "): " + strings.Join(found, ", ")
		return res
	}
	res.Verdict = VerdictPass
	res.Detail = strconv.Itoa(len(found)) + " softening phrases, at or under the limit of " +
		strconv.Itoa(limits.HedgingMax)
	return res
}

// conflict would refuse a document contradicting an accepted one.
//
// It cannot run: §10's adjudication — findings, severities, and the record of what
// was accepted — is Phase 3, so there is nothing to query. Reporting Unchecked is
// the honest outcome: this build cannot tell whether the candidate contradicts the
// corpus, and saying so is different from saying it does not.
//
// What Unchecked costs has changed. It used to mean nothing could be promoted at
// all; it now means a promotion carries a person's name and a rationale, and the
// audit row records that this signal did not run. When §10 lands, every claim
// admitted on that basis can be found and re-examined, which is the property that
// makes an unrun check a debt rather than a hole.
func conflict() Result {
	return Result{
		Signal:  SignalConflict,
		Verdict: VerdictUnchecked,
		Detail:  "§10 adjudication is not built; no finding store to query",
	}
}

// security refuses a document whose admission scan produced findings.
//
// It scans the **candidate**, not the sources it was built from, and that is the
// gap this signal closes. `archive.Gates.ScanText` already scans a fetched source
// on the way into tier 0, so upstream text is covered. Nothing scanned the
// document a model then wrote *out* of that text — which is the more dangerous
// artifact, because it is the one filed into the corpus for an agent to obey, and
// a model can reproduce an injected instruction out of source that was itself
// clean.
//
// Three outcomes, and the third is the one that earns its keep:
//
//   - findings present → Fail. Hidden characters in a document about to be
//     committed are not a judgement call.
//   - clean and every §9.3 stage ran → Pass.
//   - clean and some stage did not run → Unchecked. "No hidden characters" and
//     "§9.3 passed" are different claims, and only the first is available while
//     injection patterns and secret scanning are unbuilt. Unchecked routes the
//     candidate to DecisionNeedsHuman, so it can still be promoted — by a named
//     person, on the record, with the unrun stages in the audit row.
//
// A prior version of this returned a literal saying the scan "is not built". That
// stopped being true when `internal/scan` landed, and a comment asserting the
// absence of a package that exists is worse than no comment.
func security(c *Candidate) Result {
	res := Result{Signal: SignalSecurity}

	switch {
	case len(c.Scan.Findings) > 0:
		res.Verdict = VerdictFail
		res.Detail = strconv.Itoa(len(c.Scan.Findings)) +
			" admission scan finding(s): " + strings.Join(c.Scan.Findings, "; ")
	case len(c.Scan.StagesMissing) > 0:
		res.Verdict = VerdictUnchecked
		res.Detail = "no findings from " + stagesOrNone(c.Scan.StagesRun) +
			", but §9.3 stages not run: " + strings.Join(c.Scan.StagesMissing, ", ")
	case len(c.Scan.StagesRun) == 0:
		// Neither run nor missing: nobody populated the field. Reading that as a
		// clean scan is precisely the mistake this signal exists to prevent.
		res.Verdict = VerdictUnchecked
		res.Detail = "the candidate was not scanned"
	default:
		res.Verdict = VerdictPass
		res.Detail = "§9.3 scan clean across " + strings.Join(c.Scan.StagesRun, ", ")
	}
	return res
}

// stagesOrNone renders a stage list, naming the empty case rather than producing
// a sentence with a hole in it.
func stagesOrNone(stages []string) string {
	if len(stages) == 0 {
		return "no stages"
	}
	return strings.Join(stages, ", ")
}
