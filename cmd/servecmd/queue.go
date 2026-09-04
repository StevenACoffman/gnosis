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

	var out []web.Item
	for i := range report.Diagnostics {
		if item, ok := decisionItem(&report.Diagnostics[i]); ok {
			out = append(out, item)
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
