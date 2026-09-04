package bundle

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/skillet/errs"
)

// filedType is the OKF type a filed answer declares when the caller names none.
//
// `Reference` — "a recorded fact with no prescriptive force" — because a synthesized
// answer prescribes nothing until somebody decides it does, and the starter vocabulary
// marks it `normative = false`, which is what keeps §17.2's limitations check from
// firing on every filed answer. A caller who is writing a rule says so with --type.
const filedType = "Reference"

// FileOptions files a good answer back as a concept (§8.3).
type FileOptions struct {
	// Key is the ask prompt this answers, and Reply is the agent's whole response.
	Key   string
	Reply []byte

	// Type is the OKF type the filed concept declares. Empty takes filedType.
	Type string

	// Actor is who is filing, for the trail.
	Actor string

	// Warn is where a note that is not a failure goes, or nil to discard one.
	Warn io.Writer
}

// Filed is what one answer produced.
type Filed struct {
	Key string `json:"key"`

	// Path is where the draft landed, which is inside .gnosis/quarantine — a filed
	// answer is a candidate and not yet part of the corpus.
	Path string `json:"path"`

	Title string `json:"title"`

	// Cites are the claims the answer rests on, carried onto the draft as its
	// evidence.
	Cites []string `json:"cites"`

	// Unanswered is what the answer said it did not cover, or empty.
	Unanswered string `json:"unanswered,omitempty"`
}

// FileAnswer turns an answer into a quarantined concept.
//
// Requires: the writer holds the lock; opts.Key names an ask prompt this bundle emitted.
// Ensures: the draft is in quarantine and nothing in the corpus changed. A declination
// is refused, not filed.
//
// # It stops at quarantine, and that is the gate §8.3 asks for
//
// "Subject to the same admission gate as an ingested source, because a synthesized
// answer is exactly as capable of being wrong." The gate is `promote`, and routing this
// through the same quarantine every ingested draft passes through is what subjects it to
// the same one — rather than a second gate here, which would be one set of rules kept in
// two places and would diverge the first time either moved.
//
// # The evidence is the cited claims' own
//
// A filed answer's claims carry the quotations and archive paths of the corpus claims it
// cites. That is what makes the draft checkable by the same machinery: the promote gate
// validates quotations against archived text, and an answer whose evidence was a claim
// reference would have nothing for it to check. It also means a filed answer cannot
// introduce a quotation nobody ever archived, which is the property that stops this
// command being a way to write unsourced content into a sourced corpus.
func (w *Writer) FileAnswer(opts *FileOptions) (*Filed, error) {
	const op = "bundle.Writer.FileAnswer"

	if err := w.held(op); err != nil {
		return nil, err
	}
	meta, err := LoadPromptMeta(w.dir, opts.Key)
	if err != nil {
		return nil, err
	}
	if meta.Kind != PromptAsk {
		return nil, &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": key " + opts.Key + " is a " + string(meta.Kind) +
				" prompt; `gnosis file` takes an answer to `gnosis ask`",
		}
	}
	reply, err := relay.ParseAnswer(opts.Reply, meta.Cites)
	if err != nil {
		// Wrapped rather than returned bare so the operation is in the message. The
		// EINVALID survives the wrap — `errs.ErrorCode` walks the chain — which is
		// what lets the command report a malformed reply as the caller's to fix
		// rather than as a broken bundle.
		return nil, &errs.Error{Op: op, Err: err}
	}
	if !reply.Answered() {
		// A declination is a real answer and it is not a concept. Refusing it here
		// says so; filing it would put "the corpus does not say" into the corpus,
		// where the next reader would retrieve it as a claim.
		return nil, &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": this reply declined to answer, which is a real answer" +
				" and not a document: " + reply.Unanswered,
		}
	}

	docs, err := Load(os.DirFS(w.dir))
	if err != nil {
		return nil, err
	}
	sources, err := sourceVersions(os.DirFS(w.dir))
	if err != nil {
		return nil, err
	}
	claims, missing := citedClaims(docs, reply.Cites)
	if len(missing) > 0 {
		// Between the prompt and the reply somebody deleted a claim the answer rests
		// on. Refused rather than filed without it: a draft citing evidence that is
		// gone would pass the gate on the quotations that remain and assert the rest.
		return nil, &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": the corpus no longer holds " + strings.Join(missing, ", ") +
				"; re-run `gnosis ask` against the corpus as it is now",
		}
	}
	return w.fileDraft(op, opts, &reply, claims, sourceURIs(claims, sources))
}

// fileDraft renders the draft, quarantines it, and records the write.
func (w *Writer) fileDraft(
	op string, opts *FileOptions, reply *relay.AnswerReply, claims []conceptClaim,
	uris []string,
) (*Filed, error) {
	id, err := gnosis.NewID()
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	docType := opts.Type
	if docType == "" {
		docType = filedType
	}
	content := renderConcept(&conceptDoc{
		Type: docType, Title: reply.Title, ID: id,
		SourceURI:  uris,
		Claims:     claims,
		Paragraphs: []string{reply.Answer},
	})
	rel := "c/" + id.String() + "-" + gnosis.SlugFrom(reply.Title).String() + ".md"
	if _, qErr := w.Quarantine(rel, content); qErr != nil {
		return nil, &errs.Error{Op: op, Err: qErr}
	}
	// The trail row is best-effort and the draft is not, which is `spendCritic`'s
	// split: the document is in quarantine either way, and refusing to report a write
	// that happened would leave the caller unable to find what was written. §15 wants
	// a mutation to fail hard when its row is unreadable — that rule is about the
	// corpus, and this write is to the per-user quarantine, which a promotion audits
	// again when it lands.
	if aErr := w.Audit(&audit.Row{
		At: time.Now().UTC(), Op: audit.OpAdmit, Actor: opts.Actor,
		Paths:     []string{rel},
		HashAfter: hashOrEmpty(content),
		Outcome:   string(gnosis.StatusOK),
		Detail:    "filed from answer " + opts.Key,
	}); aErr != nil && opts.Warn != nil {
		_, _ = fmt.Fprintf(opts.Warn,
			"warning: the draft was written and the trail was not appended: %v\n", aErr)
	}
	w.spendAnswered(opts.Key, "answer", opts.Warn)
	return &Filed{
		Key: opts.Key, Path: rel, Title: reply.Title,
		Cites: reply.Cites, Unanswered: reply.Unanswered,
	}, nil
}

// sourceURIs is where the cited claims' evidence came from, deduplicated and in the
// order the claims cite it.
//
// Requires: claims came from citedClaims; sources maps an archive path to the source it
// holds, as sourceVersions builds it.
// Ensures: one entry per distinct source, and none for an archive path with no record.
// Pure.
//
// **The draft declares the sources its evidence rests on, and a hand run is what found
// that it did not.** The promote gate's provenance signal fails a document that declares
// none — "a document asserting claims and citing nothing is exactly what this corpus
// exists to refuse" — and a filed answer with no sources was failing it every time. The
// fix is not to exempt the draft: the sources are genuinely known, because they are the
// ones the cited claims already rest on, and inheriting them is what makes the draft's
// provenance true rather than waived.
func sourceURIs(claims []conceptClaim, sources map[string]lint.SourceVersion) []string {
	var (
		out  []string
		seen = map[string]bool{}
	)
	for i := range claims {
		for _, path := range claims[i].ArchivePaths {
			source, ok := sources[path]
			if !ok || source.URI == "" || seen[source.URI] {
				continue
			}
			seen[source.URI] = true
			out = append(out, source.URI)
		}
	}
	return out
}

// citedClaims collects the corpus claims an answer cites, and names the ones that are
// gone.
//
// Requires: docs came from Load; refs are claim references the prompt offered.
// Ensures: one entry per resolvable reference in the order cited, and the references
// that no longer resolve. Pure.
//
// **The claim's identifier travels rather than being minted fresh**, so a reader of the
// filed draft can see that its second claim is the same assertion as some other
// document's — and so a later `supersede` or `adjudicate` naming one of them is talking
// about a claim this draft did not silently fork.
func citedClaims(docs []Document, refs []string) (claims []conceptClaim, missing []string) {
	for _, ref := range refs {
		id, claimID, ok := gnosis.ParseClaimRef(ref)
		if !ok {
			missing = append(missing, ref)
			continue
		}
		claim := claimByID(docs, id, claimID)
		if claim == nil {
			missing = append(missing, ref)
			continue
		}
		claims = append(claims, conceptClaim{
			ID: claimID, Anchor: claim.Anchor, Lead: claim.Lead,
			Quotes: claim.Quotes, ArchivePaths: claim.ArchivePaths,
		})
	}
	return claims, missing
}

// claimByID finds one claim by the document and claim identifiers that address it.
func claimByID(docs []Document, id gnosis.ID, claimID string) *DocClaim {
	for i := range docs {
		if docs[i].ID != id {
			continue
		}
		return findClaim(&docs[i], claimID)
	}
	return nil
}
