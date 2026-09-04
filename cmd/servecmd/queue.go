package servecmd

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/gnosis/internal/scan"
	"github.com/StevenACoffman/gnosis/internal/web"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/finding"
)

// subjectContext is what §10.6.2 and §10.6.2.1 ask be shown beside a decision.
//
// A value gathered once per queue load rather than per item: both halves are corpus-wide
// reads, and doing them per conflict would open the ontology once for every pair.
type subjectContext struct {
	owners  map[gnosis.SubjectKey]string
	decided []gnosis.Adjudicated
}

// queue assembles what needs a person, in the shape §13 requires to decide with.
//
// **The assembly is here rather than in a handler**, because a handler that built this
// would be a handler that needed the corpus — and `internal/web` cannot import it. The
// constraint that looked like an obstacle keeps the fetching out of the presenting.
type queue struct {
	dir   string
	rules *scan.Ruleset
}

// Waiting is every draft, contradiction and open challenge, drafts first.
//
// Requires: nothing; a bundle with nothing waiting is the ordinary case.
// Ensures: a stable order — drafts by path, then findings in the order the checks
// reported them. §13 asks that items be cheap to dismiss, and a list that reorders itself
// between two loads costs the reviewer their place on every refresh.
//
// **A failure to read one source is not a failure to serve the queue.** The corpus and
// the lint snapshot are read separately, and a bundle with no index still shows its
// drafts — a review queue that went blank because a derived file was stale would send a
// reviewer to a terminal to find out why, which is the audience §13 exists to spare.
func (q *queue) Waiting(ctx context.Context) ([]web.Item, error) {
	const op = "servecmd.queue.Waiting"

	drafts, err := bundle.Review(q.dir, q.rules)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	// The drafts themselves, so an item can show what the document says rather than
	// what it is called. A failure to parse them is not a failure to list what is
	// waiting: a reviewer told "one draft, unreadable" can act, and an error page
	// tells them nothing about their queue.
	docs, _ := bundle.LoadQuarantined(q.dir)
	byPath := make(map[string]*bundle.Document, len(docs))
	for i := range docs {
		byPath[docs[i].Path] = &docs[i]
	}
	items := make([]web.Item, 0, len(drafts))
	for i := range drafts {
		items = append(items, draftItem(&drafts[i], byPath[drafts[i].Path]))
	}
	return append(items, q.decisions(ctx)...), nil
}

// draftItem presents one quarantined document.
//
// The gate's verdict travels as `Why`, which is the field that makes a queue useful
// rather than merely present: a list of paths says what is waiting and not why any of it
// is stuck, which is the actual question.
func draftItem(w *bundle.Waiting, doc *bundle.Document) web.Item {
	item := web.Item{
		Kind: web.ItemDraft, ID: w.Path, Summary: w.Path,
		Why:    draftWhy(w),
		Action: web.CommandPromote,
		Sides:  []web.Side{{Ref: w.Path, Path: w.Path, Title: w.Path}},
	}
	if doc == nil {
		// The gate judged a document this could not parse. Said plainly rather than
		// shown as an empty card: a reviewer looking at a blank item concludes the
		// queue is broken, and this one is telling them something true.
		item.Sides[0].Text = "this draft could not be parsed; the gate's verdict above" +
			" is what is known about it"
		return item
	}
	item.Summary = doc.Title
	item.Sides[0] = draftSide(w.Path, doc)
	return item
}

// draftSide is the draft as §13 requires it presented: what it says, what it rests on,
// and the derived signals — not a filename three times.
//
// **A hand run is what found this**, and it is the failure the section names in its own
// words: "if the queue shows enough, a non-expert correctly recognizes when to defer; if
// it shows too little, even an expert guesses". The first version showed the path as the
// id, the summary and the title, and nothing else. Every test passed, because they
// asserted the item existed.
func draftSide(path string, doc *bundle.Document) web.Side {
	side := web.Side{
		Ref: path, Path: path, Title: doc.Title,
		Text:  claimText(doc),
		Trust: gnosis.FoldTrust(verifiedActors(doc)),
	}
	for _, resource := range doc.Resources {
		side.Sources = append(side.Sources, web.Source{
			Resource: resource,
			// Archived is what separates "check it yourself" from "you cannot"
			// (§14.4). A draft's claims name the archive paths they rest on, so a
			// claim citing one is resting on bytes tier 0 holds.
			Archived: len(doc.SourceKeys) > 0,
		})
	}
	return side
}

// claimText is what the draft asserts, in as few words as say something.
//
// The first claim's lead when it has one and its anchor otherwise, because §17.4's lead
// is the conclusion stated first — which is exactly what a reviewer scanning a queue
// needs — and the anchor is the honest fallback for a draft nobody has extracted one
// from yet.
func claimText(doc *bundle.Document) string {
	for i := range doc.Claims {
		if lead := doc.Claims[i].Lead; lead != "" {
			return lead
		}
		if anchor := doc.Claims[i].Anchor; anchor != "" {
			return anchor
		}
	}
	return "this draft declares no claim"
}

// verifiedActors are the actors a draft's claims record, for §14.1's fold.
//
// Raw strings rather than parsed actors, which §14.1.1 requires: `gnosis.Actor` refuses
// two of OKF §7's three forms, and a draft carrying `verified: [{by:
// reference_agent/gemini-2.5-pro}]` is conformant OKF that must not be rejected for the
// shape of an optional family.
func verifiedActors(doc *bundle.Document) []string {
	var out []string
	for i := range doc.Claims {
		for _, v := range doc.Claims[i].Verified {
			out = append(out, v.By)
		}
	}
	return out
}

// draftWhy says what the gate concluded and which signals stand behind it.
func draftWhy(w *bundle.Waiting) string {
	why := w.Decision.String()
	if len(w.Failed) > 0 {
		why += "; failed: " + signalList(w.Failed)
	}
	if len(w.Unchecked) > 0 {
		why += "; could not run: " + signalList(w.Unchecked)
	}
	return why
}

// decisions are the contradictions and challenges the checks reported.
//
// **Read separately from the drafts and degraded rather than fatal**, for `Waiting`'s
// reason: a stale index must not empty the queue. A corpus this cannot examine yields no
// decision items and says so through the one item it does produce, rather than through
// an error page.
func (q *queue) decisions(ctx context.Context) []web.Item {
	idx, err := bundle.LoadIndex(ctx, q.dir)
	if err != nil {
		return []web.Item{unreadable("the index could not be read", err)}
	}
	fresh, err := bundle.LoadFreshness(q.dir)
	if err != nil {
		return []web.Item{unreadable("the freshness state could not be read", err)}
	}
	snap, err := bundle.Snapshot(os.DirFS(q.dir), idx, fresh)
	if err != nil {
		return []web.Item{unreadable("the corpus could not be examined", err)}
	}
	// The clock is read here rather than injected, which every other caller of
	// `lint.Checks` does too: the freshness checks compare against now, and a served
	// queue that pinned a time would report a corpus as fresh for as long as the
	// process ran.
	report := lint.Run(snap, lint.Checks(time.Now().UTC()))

	// What somebody has already decided to live with does not come back to the queue.
	// §13 asks that each item be cheap to dismiss; an item that reappeared after being
	// dismissed would make the queue a list of things a reviewer has to dismiss again,
	// which is worse than not having the action at all. `gnosis lint` still reports
	// every deferral, because §17.0 makes reviewing the deferred set a different
	// activity from reviewing the open set.
	deferred := deferredFindings(snap)

	var out []web.Item
	for i := range report.Diagnostics {
		if item, ok := decisionItem(&report.Diagnostics[i]); ok {
			out = append(out, item)
		}
	}
	return append(out, unseparatedItems(snap, deferred, q.context(snap))...)
}

// context gathers the ownership and the adjudication history.
func (q *queue) context(snap *lint.Snapshot) subjectContext {
	docs, err := bundle.Load(os.DirFS(q.dir))
	if err != nil {
		// The history is a display, so a corpus this cannot read yields none rather
		// than failing the queue. The items still carry the claims and their
		// evidence, which is what §13 requires to decide with.
		return subjectContext{owners: bundle.SubjectOwners(os.DirFS(q.dir))}
	}
	return subjectContext{
		owners:  bundle.SubjectOwners(os.DirFS(q.dir)),
		decided: bundle.Adjudications(docs, snap.Vocabulary.ResolvesSubject),
	}
}

// unseparatedItems presents the pairs §10.3 routes to a judge.
//
// **Built from the pairs rather than from the diagnostics**, because §13 requires both
// claims side by side and a diagnostic carries one path and a sentence. The pair carries
// two references, which is what a reviewer needs to see what they are choosing between.
func unseparatedItems(
	snap *lint.Snapshot, deferred map[string]bool, ctx subjectContext,
) []web.Item {
	pairs := lint.Unseparated(snap)
	out := make([]web.Item, 0, len(pairs))
	for i := range pairs {
		pair := &pairs[i]
		if deferred[pair.ID()] {
			continue
		}
		out = append(out, web.Item{
			Kind: web.ItemConflict, ID: pair.First.Path, Finding: pair.ID(),
			Summary: "two claims about " + string(pair.Subject) +
				" that no predicate can separate",
			Why:    "a judge decides this, or a deferral records living with it",
			Action: web.CommandAdjudicate,
			Sides: []web.Side{
				unseparatedSide(&pair.First), unseparatedSide(&pair.Second),
			},
			Domain: domainHistory(ctx.decided, pair.Subject),
			Owner:  ctx.owners[pair.Subject],
		})
	}
	return out
}

// domainHistory is who has adjudicated under this subject's domain before.
//
// **Shown and never enforced** (§10.6.2): a declared capability roster is a political
// artifact that rots, and a capability derived from behaviour is self-certifying — "you
// may adjudicate `db.*` because you have adjudicated `db.*`" entrenches whoever arrived
// first. The count grants nothing and cannot be gamed into anything, because there is
// nothing to acquire.
func domainHistory(decided []gnosis.Adjudicated, subject gnosis.SubjectKey) []web.DomainCount {
	folded := gnosis.FoldDomainHistory(decided, subject)
	if len(folded) == 0 {
		return nil
	}
	under := string(gnosis.DomainOf(subject)) + ".*"
	out := make([]web.DomainCount, 0, len(folded))
	for _, count := range folded {
		// The label travels with the number, as §10.6.2's own rendering does: a bare
		// figure beside a name reads as a score, and "14 prior adjudications under
		// retry.*" says what it is a count of.
		out = append(out, web.DomainCount{By: count.By, Count: count.Count, Under: under})
	}
	return out
}

// unseparatedSide is one claim of an unseparated pair, as the queue shows it.
func unseparatedSide(claim *lint.UnseparatedClaim) web.Side {
	return web.Side{
		Ref: claim.Ref(), Path: claim.Path, Title: claim.Path, Text: claim.Anchor,
	}
}

// deferredFindings is every conflict identity this corpus has recorded a deferral for.
//
// Requires: snap carries the parsed conflict edges.
// Ensures: only valid deferrals count, which is the safe direction: a half-written entry
// leaves the conflict in the queue rather than silently suppressing it. Pure.
func deferredFindings(snap *lint.Snapshot) map[string]bool {
	out := map[string]bool{}
	for i := range snap.Documents {
		edges := snap.Documents[i].Conflicts
		for j := range edges {
			if edges[j].Valid() {
				out[edges[j].Finding] = true
			}
		}
	}
	return out
}

// decisionItem presents one diagnostic that needs a person, and reports whether it does.
//
// **Only findings a person can resolve here reach the queue.** §13's review surface is
// "quarantined concepts, open conflict findings, adjudication, and promotion requests" —
// a lint finding about a filename is real and is not a decision, and a queue that carried
// every diagnostic would be a lint report with a worse interface.
func decisionItem(d *finding.Diagnostic) (web.Item, bool) {
	kind, action, ok := decisionKind(d.Category)
	if !ok {
		return web.Item{}, false
	}
	return web.Item{
		Kind: kind, ID: d.Path, Summary: d.Message, Why: d.Category,
		Action: action,
		Sides:  []web.Side{{Ref: d.Path, Path: d.Path, Title: d.Path}},
	}, true
}

// decisionKind classifies a diagnostic category as something a reviewer resolves.
func decisionKind(category string) (web.ItemKind, web.CommandKind, bool) {
	switch category {
	case "conflict":
		return web.ItemConflict, web.CommandAdjudicate, true
	case "unanswered-challenge":
		return web.ItemChallenge, web.CommandAdjudicate, true
	default:
		return "", "", false
	}
}

// unreadable is the item a reviewer sees when part of the corpus could not be examined.
//
// **An item rather than an error**, because the alternative is an empty queue: a reviewer
// shown nothing concludes there is nothing to do, and this says which half of the answer
// is missing and why. It carries no action, because the remedy is a terminal.
func unreadable(what string, err error) web.Item {
	return web.Item{
		Kind: web.ItemDraft, ID: "", Summary: what,
		Why: err.Error() + " — the drafts above are complete; contradictions and" +
			" challenges are not listed until this is repaired",
	}
}

// signalList renders gate signals for a person.
func signalList[T ~string](signals []T) string {
	var b strings.Builder
	for i, s := range signals {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(string(s))
	}
	return b.String()
}
