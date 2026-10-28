package lint

import (
	"cmp"
	"slices"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// durabilityCheck reports a load-bearing unprovable concept (§14.4.1).
//
// Not every claim is equally provable offline, because the archive holds text and not
// sources (§4.2): a `referenced` source is a hash and a URI, so a quotation resting on
// it cannot be re-checked without the network — and possibly not then. §14.4 makes that
// difference visible per concept rather than averaged into a corpus-level number.
//
// **Weakness alone is not the finding, and reporting it would be the mistake.**
// `referenced` sources are admitted (§4.3) on the reasoning that weakly trusting a
// reliable external authority is fine *when the claim is not central*. That reasoning
// has a mechanical consequence — the risk is the product of weakness and centrality — so
// a check that listed every unprovable page would flood a reader with exactly the
// peripheral cases the admission policy already decided were acceptable.
//
// So two classes are reported and one is counted:
//
//   - **load-bearing-weak**: unprovable, with in-degree at or above the declared cut.
//   - **cited-by-provable**: unprovable, with provable work resting on it — which is
//     worth a reader's attention wherever it sits in the graph.
//   - **peripheral-weak**: counted and never listed, because §14.4.1 requires the
//     suppressed count to be reported. A check that silently drops most of its findings
//     reads as coverage.
//
// **Warning, never a gate**, and §14.4.1 says why in one sentence: a load-bearing weak
// claim is a prompt to go and find a better source, which is human work and not a build
// failure. The cut is a reference value, and nothing blocks on centrality.
func durabilityCheck() Check {
	return Check{
		Name:       "durability",
		Categories: []string{"durability", "durability-peripheral"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someDocumentRestsOnASource,
		Run:        DurabilityFindings,
	}
}

// someDocumentRestsOnASource reports whether there is anything to be provable against.
//
// Requires: nothing.
// Ensures: a reason whenever it declines, naming the missing thing rather than the
// check. Pure.
//
// **A cut of zero declines rather than reporting**, and the direction matters: an
// in-degree of zero is at or above a cut of zero, so a corpus whose standards did not
// load would otherwise have every unprovable document reported as load-bearing — the
// loudest possible reading of a missing threshold.
func someDocumentRestsOnASource(snap *Snapshot) (bool, string) {
	if snap.InDegreeCut <= 0 {
		// **It does not say the file declares none**, because it may declare one and
		// have failed to load — an incomplete `standards/archive.toml` is rejected
		// whole, and the first draft of this sentence told a reader with the value in
		// front of them that they had not written it. `doctor` is where the two are
		// told apart; this check only knows there is no cut in force.
		return false, "no in_degree_cut is in force: standards/archive.toml either" +
			" declares none or did not load, and `gnosis doctor` says which. Without a" +
			" cut every document is at or above it"
	}
	for i := range snap.Documents {
		if len(snap.Documents[i].Evidence) > 0 {
			return true, ""
		}
	}
	return false, "no document rests on a source tier 0 holds a record of, so there is" +
		" nothing to be provable against"
}

// DurabilityFindings reports the unprovable documents the corpus leans on.
//
// Requires: snap.InDegreeCut is positive.
// Ensures: one diagnostic per reported document, sorted by path, plus one carrying the
// peripheral count whenever any was suppressed. Pure.
//
// Exported for one caller, and the caller is why it exists rather than a convenience.
// §6.2 requires a loosening to be recorded with the finding count before and after, and
// `in_degree_cut` is a threshold this check reads — so `bundle.durabilityFindingDelta`
// runs it twice over one corpus with the two cuts. The registry calls it too, so there
// is one implementation rather than a check and a copy of it.
//
// **Applicability is deliberately not included**, for the reason StaleFindings gives:
// a caller asking for the count under two cuts wants the same population both times,
// and skipping would make the delta a comparison of two different questions.
func DurabilityFindings(snap *Snapshot) []finding.Diagnostic {
	durability := durabilityByDocument(snap)
	inDegree, citedByProvable := centrality(snap, durability)

	out := make([]finding.Diagnostic, 0)
	peripheral := 0
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		class := gnosis.ClassifyWeakness(
			durability[doc.ID], inDegree[doc.ID], snap.InDegreeCut,
			citedByProvable[doc.ID])
		switch class {
		case gnosis.WeaknessPeripheral:
			peripheral++
		case gnosis.WeaknessLoadBearing, gnosis.WeaknessCitedByProvable:
			out = append(out, weakFinding(doc, class, inDegree[doc.ID],
				citedByProvable[doc.ID]))
		case gnosis.WeaknessNotWeak:
			// Provable, partly provable, or resting on nothing.
		}
	}
	slices.SortFunc(out, func(a, b finding.Diagnostic) int {
		return cmp.Compare(a.Path, b.Path)
	})
	if peripheral > 0 {
		out = append(out, suppressedFinding(peripheral))
	}
	return out
}

// durabilityByDocument folds each document's evidence into §14.4's signal.
//
// Requires: nothing.
// Ensures: an entry for every document carrying an identifier. Pure.
func durabilityByDocument(snap *Snapshot) map[gnosis.ID]gnosis.Durability {
	out := make(map[gnosis.ID]gnosis.Durability, len(snap.Documents))
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		support := make([]gnosis.Support, 0, len(doc.Evidence))
		for _, e := range doc.Evidence {
			support = append(support, e.Support)
		}
		out[doc.ID] = gnosis.FoldDurability(support)
	}
	return out
}

// centrality is how many documents link to each document, and which unprovable ones
// carry provable work.
//
// Requires: durability holds an entry per document.
// Ensures: two maps keyed by document identifier; an absent key means zero and false.
// Pure.
//
// The two are computed together because they are one walk over the same links, and
// separating them would walk the graph twice to answer two questions about the same
// edge. Self-links are excluded from both: a document citing itself has not been relied
// on by anybody, and counting it would let a page make itself load-bearing.
func centrality(
	snap *Snapshot, durability map[gnosis.ID]gnosis.Durability,
) (inDegree map[gnosis.ID]int, citedByProvable map[gnosis.ID]bool) {
	inDegree = map[gnosis.ID]int{}
	citedByProvable = map[gnosis.ID]bool{}
	for _, l := range snap.Links {
		if l.External || l.ToID == "" || l.ToID == l.FromID {
			continue
		}
		inDegree[l.ToID]++
		if durability[l.FromID] == gnosis.DurabilityProvable {
			citedByProvable[l.ToID] = true
		}
	}
	return inDegree, citedByProvable
}

// weakFinding is one reported document.
//
// **Both reasons are stated when both hold**, rather than only the class name. The
// classifier picks one class by precedence because a finding has one category, and a
// reader deciding whether to go and find a better source wants to know that the page is
// central *and* that provable work rests on it. Dropping the second fact to keep the
// message short would make the finding weaker than the evidence for it.
func weakFinding(
	doc *Document, class gnosis.Weakness, inDegree int, citedByProvable bool,
) finding.Diagnostic {
	why := []string{"every source it rests on is `referenced`, so nothing it says can" +
		" be checked offline"}
	if inDegree > 0 {
		// "cited by 1 document" rather than "1 document links here", because §17.5
		// wants the count inside a noun phrase where no verb has to agree with it.
		why = append(why, "cited by "+Noun(inDegree, "document"))
	}
	if citedByProvable {
		why = append(why, "and work that *is* provable rests on it")
	}
	return finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "durability",
		Path:     doc.Path,
		Message: class.String() + ": " + strings.Join(why, ", ") +
			" — find a source whose text can be archived, or move what depends on this" +
			" onto one that can",
		Action: finding.ActionHuman,
	}
}

// suppressedFinding reports the peripheral count, which §14.4.1 requires.
//
// **One diagnostic rather than a line per document**, and it is emitted only when the
// count is non-zero. The requirement is that the suppression be visible, not that every
// peripheral page be listed — listing them is the flooding the class exists to prevent,
// and a permanent "0 suppressed" line is the kind a reader learns to skip, which would
// make its presence mean nothing on the day it mattered.
func suppressedFinding(peripheral int) finding.Diagnostic {
	return finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "durability-peripheral",
		Message: "not listed above: " + Noun(peripheral, "unprovable document") +
			" whose in-degree is below the declared cut, which §4.3 admits" +
			" deliberately — weakly trusting an external authority is acceptable where" +
			" the claim is not central. Reported so the suppression is visible rather" +
			" than read as coverage",
		Action: finding.ActionHuman,
	}
}
