package gate

import (
	"strings"

	"github.com/StevenACoffman/skillet/skilllens"
)

const archivedControlText = "The archived sentence about caching behaviour appears here in full."

// control is one signal's pair of fixtures and the closure that evaluates them.
type control struct {
	signal  Signal
	defect  fixture
	control fixture
	run     func(*fixture) Verdict
}

// fixture is a candidate and the corpus it is judged against, together, because a
// defect in the evidence signal lives in the relationship between the two rather
// than in either alone.
type fixture struct {
	candidate Candidate
	corpus    Corpus
	limits    Limits
}

// controls returns the battery.
//
// The fixtures are deliberately minimal and deliberately *not* shared with the
// package's own tests. A control that reused a test fixture would drift with it,
// and the property being asserted here is that this signal discriminates at all —
// not that it handles the cases somebody thought to write down.
func controls() []control {
	return []control{
		{
			signal: SignalEvidence,
			// Defect: a claim whose quotation is nowhere in the archived text. The
			// control's case matches the source, because quotecheck folds
			// whitespace and typography and deliberately not case.
			defect:  evidenceFixture("a sentence that was never written"),
			control: evidenceFixture("The archived sentence about caching behaviour"),
			run: func(f *fixture) Verdict {
				return evidence(&f.candidate, &f.corpus, f.limits).Verdict
			},
		},
		{
			signal:  SignalProvenance,
			defect:  provenanceFixture("https://example.org/never-fetched"),
			control: provenanceFixture("https://example.org/fetched"),
			run:     func(f *fixture) Verdict { return provenance(&f.candidate, &f.corpus).Verdict },
		},
		{
			signal:  SignalConformance,
			defect:  conformanceFixture(""),
			control: conformanceFixture("Reference"),
			run:     func(f *fixture) Verdict { return conformance(&f.candidate).Verdict },
		},
		{
			signal:  SignalDuplication,
			defect:  duplicationFixture("Cache Policy"),
			control: duplicationFixture("Something Else Entirely"),
			run:     func(f *fixture) Verdict { return duplication(&f.candidate, &f.corpus).Verdict },
		},
		{
			signal:  SignalHedging,
			defect:  hedgingFixture(true),
			control: hedgingFixture(false),
			run:     func(f *fixture) Verdict { return hedging(&f.candidate, f.limits).Verdict },
		},
	}
}

func evidenceFixture(quote string) fixture {
	const path = "evidence/text/aa/aaaa.md"
	return fixture{
		candidate: Candidate{
			Path: "c/x.md",
			Doc: Document{
				Claims: []Claim{{
					ID: "control", Enforced: true,
					Quotes: []string{quote}, ArchivePaths: []string{path},
				}},
			},
		},
		corpus: Corpus{ArchivedText: map[string]string{path: archivedControlText}},
		limits: Limits{MinPassageWords: 6},
	}
}

func provenanceFixture(resource string) fixture {
	return fixture{
		candidate: Candidate{
			Path: "c/x.md",
			Doc:  Document{Sources: []Source{{Resource: resource}}},
		},
		corpus: Corpus{FetchedURIs: map[string]bool{"https://example.org/fetched": true}},
	}
}

func conformanceFixture(docType string) fixture {
	return fixture{
		candidate: Candidate{
			Path: "c/x.md",
			Doc:  Document{Type: docType, Title: "A Title", Body: "A body."},
		},
	}
}

func duplicationFixture(title string) fixture {
	return fixture{
		candidate: Candidate{Path: "c/x.md", Doc: Document{Title: title}},
		corpus: Corpus{
			TitlesByFold: map[string][]string{"cache policy": {"c/other.md"}},
		},
	}
}

func hedgingFixture(hedged bool) fixture {
	body := "The cache is disabled after ten minutes."
	if hedged {
		body = strings.Join(hedgeTerms(), " ") + " the cache is disabled."
	}
	return fixture{
		candidate: Candidate{Path: "c/x.md", Doc: Document{Body: body}},
		limits:    Limits{HedgingMax: 1},
	}
}

// hedgeTerms takes two softening terms from the shared list rather than writing
// literals, so the defect stays a defect if skilllens revises its vocabulary. Two,
// because the fixture's limit is one.
//
// A vocabulary too short to build the fixture returns nothing, and the control
// then fails rather than silently exercising a body with no hedging in it.
func hedgeTerms() []string {
	terms := skilllens.SofteningTerms()
	if len(terms) < 2 {
		return nil
	}
	return terms[:2]
}
