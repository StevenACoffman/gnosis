package bundle

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// askDefaultLimit is how many claims a question retrieves when the caller names no
// bound.
//
// Ten rather than five: this is not §10.5's sample, where the number is a cost control
// on model calls, but the width of the context one answer is assembled from — and a
// question whose answer needs six claims should not be refused for a limit chosen to
// make a different command cheap. It is a flag, so a caller who disagrees says so.
const askDefaultLimit = 10

// AskOptions asks the corpus a question (§8.3).
type AskOptions struct {
	// Question is the caller's, verbatim. It is fenced into the prompt and it is
	// **not** the index query: see AskQuery.
	Question string

	// Model is what will answer, and is part of the key (§6.1).
	Model relay.Model

	// Limit is how many claims to retrieve. Zero takes askDefaultLimit.
	Limit int

	// Warn is where a note that is not a failure goes, or nil to discard one.
	Warn io.Writer
}

// askTarget is one retrieved claim, and what the fold needs to know about it that the
// prompt must not.
//
// **The contested flag stops here and does not travel into `relay.AskClaim`.** A
// contested claim never reaches a prompt — the fold refuses the question first — so a
// field for it on the request type would exist only to be always false, and a type that
// cannot express a state is this codebase's preferred way of guaranteeing it, which
// `CriticClaim` records at length.
type askTarget struct {
	Claim     relay.AskClaim
	Contested bool
}

// Asked is what one question produced: a prompt, or the reason there is none.
//
// **The refusal is a field rather than an error**, which is §17.0.1's whole point. A
// question the corpus cannot answer is an ordinary outcome — the caller gets a state, a
// remedy and the counts behind them, and only a broken bundle produces an error.
type Asked struct {
	Question string `json:"question"`

	// Answerability is the fold over what was retrieved, and Remedy is what a person
	// can do about it. Both travel, because a state names what happened and a remedy
	// names what to do, and a caller handed only the first has to look the second up.
	Answerability gnosis.Answerability `json:"answerability"`
	Remedy        string               `json:"remedy"`

	// Prompt is the emitted prompt, or nil when the corpus refused.
	Prompt *Pending `json:"prompt,omitempty"`

	// Retrieved is how many claims the query matched, and Cites is their references —
	// which are exactly what an answer is permitted to cite.
	Retrieved int      `json:"retrieved"`
	Cites     []string `json:"cites,omitempty"`

	// Unevidenced is how many retrieved claims were left out of the prompt because
	// they offer no passage.
	//
	// **Left out rather than carried**, and the count is what keeps that from being a
	// silent cap. A claim with no evidence under a heading that promises some is an
	// invitation to assert it: the prompt says "answer only from the claims below",
	// so anything below it is licensed. The fold still counts them — a set where
	// *none* is evidenced refuses the question outright — and this is the mixed case,
	// where the corpus can answer and part of what it retrieved cannot support one.
	Unevidenced int `json:"unevidenced"`

	// Unextracted is how many claims this index holds that carry no lead, and are
	// therefore invisible to the query at any ranking (§5.5.3).
	//
	// **Part of the answer rather than a statistic beside it.** A corpus mid-extraction
	// can be silent on a question it holds the material for, and a refusal that did
	// not say so would send a reader to write a concept that is already there.
	Unextracted int `json:"unextracted"`

	// Deferred is how many retrieved claims sit under a challenge somebody saw and
	// chose not to act on yet (§10.7.4).
	//
	// It does not refuse the question — a deferred challenge is a recorded decision to
	// live with something, which is different from an open one nobody has looked at —
	// but it is reported, because answering silently from a document under an
	// unresolved contest is the shape §17.0.1 exists to make visible.
	Deferred int `json:"deferred"`
}

// AskQuery turns a question into an FTS5 query.
//
// Requires: nothing; any string may be offered.
// Ensures: an OR of the question's quoted word tokens, or the empty string when it holds
// none. Pure.
//
// # Why the question is not passed through
//
// `search` takes FTS5 syntax and this takes a sentence. "How many times does the service
// retry?" is not a valid FTS5 query — the punctuation is syntax there — so passing it
// through would turn every natural question into the EINVALID that `search` correctly
// reports for a malformed one, and the caller has made no mistake.
//
// **There is no stopword list, and its absence is a decision.** Dropping "how", "many"
// and "does" would need a list, a list is a `standards/` value with a rationale (§6.2),
// and nobody can write that rationale from measurement yet. BM25 already does this
// work principledly: a term appearing in every document carries almost no weight in the
// ranking, which is what a stopword list is a hand-made approximation of.
func AskQuery(question string) string {
	var terms []string
	for _, field := range strings.FieldsFunc(question, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		// Quoted, so a token that happens to be an FTS5 keyword — AND, OR, NOT, NEAR —
		// is matched as a word rather than executed as syntax by a question that
		// contained it by accident.
		terms = append(terms, `"`+field+`"`)
	}
	return strings.Join(terms, " OR ")
}

// AskPrompt retrieves claims for a question and emits an answer prompt, or refuses.
//
// Requires: the writer holds the lock, because emitting a prompt writes one; the index
// exists and is current.
// Ensures: an Asked whose Prompt is nil exactly when Answerability is not answerable.
// A refusal is not an error.
//
// **The retrieval and the refusal are separate steps on purpose.** What comes back from
// the index is a ranking, and a ranking cannot refuse: it returns its best matches for
// any question, including one the corpus holds nothing about. The fold is what turns a
// ranked list into an answerable/not-answerable decision, and it is pure, so the rule can
// be read and tested without a corpus.
func (w *Writer) AskPrompt(ctx context.Context, opts *AskOptions) (*Asked, error) {
	const op = "bundle.Writer.AskPrompt"

	if err := w.held(op); err != nil {
		return nil, err
	}
	query := AskQuery(opts.Question)
	if query == "" {
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": the question holds no word to search for",
		}
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = askDefaultLimit
	}

	db, err := OpenIndexForRead(ctx, w.dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	results, err := db.SearchClaims(ctx, query, limit)
	if err != nil {
		// Wrapped rather than returned bare: a malformed FTS5 query reaching here is
		// AskQuery's defect and not the caller's, so the operation that built it has
		// to be in the message.
		return nil, &errs.Error{Op: op, Err: err}
	}
	docs, err := Load(os.DirFS(w.dir))
	if err != nil {
		return nil, err
	}

	targets, deferred := askClaims(docs, results.Hits)
	out := &Asked{
		Question:    opts.Question,
		Retrieved:   len(targets),
		Unextracted: results.Unextracted,
		Deferred:    deferred,
	}
	out.Answerability = gnosis.FoldAnswerability(answerabilityOf(targets))
	out.Remedy = gnosis.Remedy(out.Answerability)
	if !out.Answerability.Answerable() {
		return out, nil
	}
	claims, unevidenced := promptClaims(targets)
	out.Unevidenced = unevidenced
	out.Cites = refsOf(claims)
	pending, err := w.askPrompt(op, opts, claims)
	if err != nil {
		return nil, err
	}
	out.Prompt = &pending
	return out, nil
}

// askPrompt renders and stores one answer prompt.
func (w *Writer) askPrompt(
	op string, opts *AskOptions, claims []relay.AskClaim,
) (Pending, error) {
	prompt := relay.RenderAsk(&relay.AskRequest{
		Question: opts.Question, Claims: claims, Model: opts.Model,
	})
	_, cached, err := LoadCached(w.dir, prompt.Key)
	if err != nil {
		return Pending{}, err
	}
	if cached {
		return Pending{Key: prompt.Key, Cached: true}, nil
	}
	meta := PromptMeta{
		Key: prompt.Key, Kind: PromptAsk, Model: opts.Model.Name,
		Cites: refsOf(claims),
	}
	// Metadata before the prompt, as ingest and critic do: a crash between them
	// leaves a meta file describing a prompt that is not there, which is inert. The
	// reverse leaves a prompt an agent can answer and nothing can accept.
	if mErr := w.StorePromptMeta(&meta); mErr != nil {
		return Pending{}, mErr
	}
	rel := promptPath(prompt.Key)
	full := filepath.Join(w.dir, filepath.FromSlash(rel))
	if mkErr := os.MkdirAll(filepath.Dir(full), 0o750); mkErr != nil {
		return Pending{}, &errs.Error{Op: op, Err: mkErr}
	}
	if wErr := atomicfile.WriteFile(full, []byte(prompt.Text), 0o640); wErr != nil {
		return Pending{}, &errs.Error{Op: op, Err: wErr}
	}
	// §6.4's miss, and this one is the reason the log exists. Retrieval reached step 5
	// of §11.2's ladder: the deterministic layers found claims and could not compose an
	// answer from them, which is not a defect and is exactly the count a corpus needs
	// to make its determinism claim honestly.
	w.noteMiss(&Miss{
		Op: "ask", Reason: gnosis.MissNoPredicate, Key: prompt.Key,
		ChecksRun: askChecksRun(), Candidate: opts.Question, At: time.Now().UTC(),
	}, opts.Warn)
	return Pending{Key: prompt.Key, Path: rel}, nil
}

// refsOf is the claim references a prompt offered, which is the set a reply may cite.
func refsOf(claims []relay.AskClaim) []string {
	out := make([]string, 0, len(claims))
	for i := range claims {
		out = append(out, claims[i].Ref)
	}
	return out
}

// askClaims joins ranked hits back to the documents that hold them.
//
// Requires: docs came from Load; hits came from SearchClaims, best first.
// Ensures: one AskClaim per hit that still resolves, in the ranking's order, and a count
// of the retrieved claims sitting under a deferred challenge. Pure.
//
// **A hit that no longer resolves is dropped rather than reported.** The index is
// derived and can lag the corpus by exactly one edit; a claim deleted since the last
// rebuild is not evidence of anything a reader can act on, and `doctor` is what reports
// index drift. Dropping it here means the answer is assembled from claims that exist.
func askClaims(docs []Document, hits []index.ClaimHit) (targets []askTarget, deferred int) {
	byPath := make(map[string]*Document, len(docs))
	for i := range docs {
		byPath[docs[i].Path] = &docs[i]
	}
	for _, hit := range hits {
		doc, ok := byPath[hit.Path]
		if !ok {
			continue
		}
		claim := findClaim(doc, hit.ID)
		if claim == nil {
			continue
		}
		if hasChallenge(doc, gnosis.ChallengeDeferred) {
			deferred++
		}
		targets = append(targets, askTarget{
			Claim: relay.AskClaim{
				Ref:    gnosis.ClaimRef(doc.ID, claim.ID),
				Text:   claim.Anchor,
				Lead:   claim.Lead,
				Quotes: claim.Quotes,
				Title:  doc.Title,
				Path:   doc.Path,
			},
			Contested: hasChallenge(doc, gnosis.ChallengeOpen),
		})
	}
	return targets, deferred
}

// promptClaims is what the answerer sees: the evidenced claims, without the contested
// flag the fold needed and the prompt must not carry.
//
// Requires: targets came from askClaims.
// Ensures: every returned claim offers at least one passage, and a count of those left
// out. Pure.
//
// **A claim with no passage is dropped rather than shown**, and the discovery was a hand
// run rather than a test: the prompt rendered a heading, "Passages it rests on:", and
// nothing under it. The rules directly above tell the model to answer from the claims
// below, so an unevidenced one is a licensed assertion with nothing behind it — the
// failure §17.0.1 calls the most expensive output this design can produce, arriving
// through the one door built to keep it out.
func promptClaims(targets []askTarget) (claims []relay.AskClaim, unevidenced int) {
	claims = make([]relay.AskClaim, 0, len(targets))
	for i := range targets {
		if len(targets[i].Claim.Quotes) == 0 {
			unevidenced++
			continue
		}
		claims = append(claims, targets[i].Claim)
	}
	return claims, unevidenced
}

// answerabilityOf projects retrieved claims onto the three questions the fold asks.
//
// Contested is read from the *document's* open challenges, because §10.7.4 files a
// challenge against a document rather than against one claim. That is broader than it
// could be — a challenge about one claim contests its neighbours too — and the direction
// is the safe one: refusing a question the corpus could have answered costs a person a
// second query, and answering from a contested document costs them the belief that the
// answer was checked.
func answerabilityOf(targets []askTarget) []gnosis.Retrieved {
	out := make([]gnosis.Retrieved, 0, len(targets))
	for i := range targets {
		out = append(out, gnosis.Retrieved{
			Evidenced: len(targets[i].Claim.Quotes) > 0,
			Contested: targets[i].Contested,
		})
	}
	return out
}

// findClaim is the claim a hit names, or nil when the document no longer declares it.
func findClaim(doc *Document, claimID string) *DocClaim {
	for i := range doc.Claims {
		if doc.Claims[i].ID == claimID {
			return &doc.Claims[i]
		}
	}
	return nil
}

// hasChallenge reports whether a document carries a challenge in the given state.
func hasChallenge(doc *Document, state gnosis.ChallengeState) bool {
	for i := range doc.Challenges {
		if challengeState(&doc.Challenges[i]) == state {
			return true
		}
	}
	return false
}

// challengeState reads a challenge's state, defaulting the empty one.
//
// Empty reads as open, which is `ChallengeOpen`'s own documented default: a challenge
// nobody has recorded a disposition for is open, and the alternative would have an
// unpopulated value claim somebody dealt with it.
func challengeState(ch *gnosis.Challenge) gnosis.ChallengeState {
	if ch.State == "" {
		return gnosis.ChallengeOpen
	}
	return ch.State
}

// askChecksRun names what looked at this question before a model was asked.
//
// §6.4 wants the miss to say what already ran, because a recurring miss naming the
// checks that decided nothing is a backlog item and a bare count is not.
func askChecksRun() []string {
	return []string{"claims_fts", "answerability"}
}
