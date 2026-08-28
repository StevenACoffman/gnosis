package searchcmd

import (
	"context"
	"fmt"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/gnosis/internal/standards"
)

// Graded is one case and what the corpus did with it.
type Graded struct {
	Query   string            `json:"query"`
	Verdict standards.Verdict `json:"verdict"`

	// Missing are the expected titles that did not come back, for a case that
	// expected titles. Empty for one that held and for a `nothing` case, where the
	// verdict is the whole report.
	Missing []string `json:"missing,omitempty"`

	// Got are the titles the search returned, so a failing case is diagnosable
	// without re-running it by hand.
	Got []string `json:"got,omitempty"`

	// Why is the disappointment that produced the case, carried through because a
	// reader looking at a failure needs to know what it was for.
	Why string `json:"why"`
}

// CaseResult is the payload for --cases.
type CaseResult struct {
	Cases []Graded `json:"cases"`

	// Held and Failed are counted, and there is deliberately no rate. §17 forbids
	// presenting a count as health, and a retrieval percentage is the most tempting
	// such number there is — it looks like progress and rises when a failing case is
	// deleted.
	Held   int `json:"held"`
	Failed int `json:"failed"`
}

// cases grades the corpus's retrieval suite.
//
// Requires: the index is readable.
// Ensures: every case graded, in file order; a suite with no cases reports that it
// examined nothing rather than reporting success.
//
// The shell reads and the judgement is `Case.Grade`'s, which is pure — so the
// interesting cases (a title that came back with different capitalisation, a
// `nothing` case that matched something) are tested from literals rather than from a
// corpus arranged to produce them.
func (c *Config) cases(ctx context.Context) error {
	suite, err := bundle.LoadRetrievalCases(c.Bundle)
	if err != nil {
		return c.fail(root.ReasonStandardsInvalid, err)
	}
	db, err := bundle.OpenIndexForRead(ctx, c.Bundle)
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	defer func() { _ = db.Close() }()

	result := CaseResult{Cases: make([]Graded, 0, len(suite.Cases))}
	for i := range suite.Cases {
		kase := &suite.Cases[i]
		hits, sErr := db.Search(ctx, kase.Query, c.Limit)
		if sErr != nil {
			return c.fail(root.ReasonIndexDrift, sErr)
		}
		result.add(kase, titlesOf(hits))
	}
	return c.reportCases(&result)
}

// add grades one case and counts it.
func (r *CaseResult) add(kase *standards.Case, got []string) {
	verdict := kase.Grade(got)
	r.Cases = append(r.Cases, Graded{
		Query: kase.Query, Verdict: verdict,
		Missing: kase.Missing(got), Got: got, Why: kase.Why,
	})
	if verdict == standards.VerdictHeld {
		r.Held++
		return
	}
	r.Failed++
}

// titlesOf projects search hits down to what a case asserts on.
//
// Titles rather than identifiers, because §11.0.2's own proposal to assert on ids was
// wrong: identifiers are assigned per corpus, so a case file naming them is unportable
// and a failing case becomes an archaeology exercise.
func titlesOf(hits []index.Hit) []string {
	out := make([]string, 0, len(hits))
	for i := range hits {
		out = append(out, hits[i].Title)
	}
	return out
}

// reportCases renders the suite.
//
// **An empty suite is `ok` and says it examined nothing.** That is not a technicality:
// §11.0.2 says cases are authored when a real query disappoints, so a corpus that has
// authored none is in the ordinary state, and reporting it as a pass would be the
// silence `scan.Coverage` exists to break one subsystem over.
//
// A failing case is a **finding**: the examination completed and found something (§17),
// so a CI job has a code to branch on and a broken index is still distinguishable from
// a corpus that stopped answering.
func (c *Config) reportCases(result *CaseResult) error {
	if c.JSONL {
		return c.emitCases(result)
	}

	for i := range result.Cases {
		g := &result.Cases[i]
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\n", g.Verdict, g.Query)
		if g.Verdict == standards.VerdictHeld {
			continue
		}
		// The reason the case exists, because a reader looking at a failure needs to
		// know what somebody was looking for when they wrote it.
		_, _ = fmt.Fprintf(c.Stderr, "  %s\n", g.Why)
		for _, m := range g.Missing {
			_, _ = fmt.Fprintf(c.Stderr, "  missing: %s\n", m)
		}
	}

	if len(result.Cases) == 0 {
		_, _ = fmt.Fprintf(c.Stderr,
			"no retrieval cases; %s holds none, and §11.0.2 says to write one when a "+
				"real query disappoints\n", standards.RetrievalFileName)
		return nil
	}
	_, _ = fmt.Fprintf(c.Stderr, "%d held, %d failed\n", result.Held, result.Failed)
	if result.Failed == 0 {
		return nil
	}
	return root.ExitError(root.CodeFindings)
}

// emitCases writes the machine envelope for --cases.
func (c *Config) emitCases(result *CaseResult) error {
	if result.Failed == 0 {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("search: %w", err)
		}
		return nil
	}
	message := fmt.Sprintf("%d retrieval case(s) failed", result.Failed)
	if err := c.EmitFindings(root.ReasonNeedsHuman, message, result); err != nil {
		return fmt.Errorf("search: %w", err)
	}
	return root.ExitError(root.CodeFindings)
}
