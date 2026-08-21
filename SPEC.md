# Gnosis — Specification

`gnosis` is the tool that makes the LLM Wiki pattern
([`llm_wiki_pattern.md`](./llm_wiki_pattern.md)) operable by a mixed-skill team
under the constraints in [`manifesto.md`](./manifesto.md): pooled tribal
knowledge, corrections that accrete rather than evaporate, and a process as
deterministic as the problem allows.

The knowledge base is a **git repository of Open Knowledge Format (OKF) markdown
documents** with a **SQLite derived index**. `gnosis` owns its ingestion,
curation, maintenance, search, and viewing — primarily as a CLI, secondarily
through an authenticated web interface.

> **Reference convention.** A bare `§N` is a section of *this* document. A
> reference to the Open Knowledge Format specification is always written
> `OKF §N`. The two numbering schemes overlap, so the prefix is load-bearing.

______________________________________________________________________

## 1. Purpose

Karpathy's pattern names the problem precisely: the tedious part of maintaining a
knowledge base is not the reading or the thinking, it is the bookkeeping, and
humans abandon wikis because maintenance grows faster than value. His answer is
to let the LLM do the bookkeeping.

The manifesto adds a constraint his document does not address: **an LLM doing
bookkeeping unsupervised will pollute the corpus.** It will file a claim that
contradicts an accepted one, refresh a page against a source that moved, and
restate as fact something a single low-quality source asserted once. A wiki that
accumulates faster than it is checked is worse than no wiki, because it carries
the team's authority.

`gnosis` exists to sit between those two facts. It does the mechanical work
deterministically, gates admission on checks a model cannot argue with, routes
the irreducibly judgmental remainder to a model through a recorded prompt, and
gives a human the last word on anything contested.

### 1.1 Governing Split

> Everything that can be decided without a model is decided in Go. A model is
> asked only what a deterministic check cannot decide, through a prompt `gnosis`
> emits and does not run. What comes back is gated deterministically before it
> touches the corpus.

This is `canonizer`'s prompt-filler posture and `adh`'s relay posture, applied to
a knowledge base. It is not an aesthetic preference — it is what makes a
correction accretive. A deterministic gate produces the same verdict on the same
input forever, so arguing with it changes the rule rather than the roll of the
dice.

### 1.1 Every Claim Here Is Testimony, and the Posture Toward It Is Deliberate

Nothing in this corpus is perceived. Every claim is something somebody said, and
the design's whole subject is when that is good enough to keep. There are three
available postures toward testimony and this specification adopts the middle one
throughout, so it is worth naming rather than leaving to be inferred from two
dozen gates.

| Posture                 | Says                                                                 | Would produce                                       |
| ----------------------- | -------------------------------------------------------------------- | --------------------------------------------------- |
| Testimony never knows   | second-hand reports are at best highly probable                      | a corpus that cannot claim to hold knowledge at all |
| **Global reductionism** | testimony is generally reliable; absent a warning sign, believe it   | admission presumed, findings as exceptions          |
| **Local reductionism**  | you need specific positive reasons for *this* source on *this* topic | **what is specified here**                          |
| The direct view         | testimony is a basic source needing no supporting reasons            | admission by assertion                              |

**`gnosis` is local-reductionist, and every gate follows from that.** Provenance
attaches per claim rather than per corpus (§14). A credible source does not make
its claims admissible; the quote still has to validate (§9.4). An adjudication
requires its own written warrant even from the person whose corpus it is (§10.6.4).
A trust tier is a signal about an actor, never a permission (§14.1).

The standard objection is that this is uncomfortably calculating — people do not in
practice weigh reasons before believing a colleague, and a tool that insists on it
models nobody's behaviour. The reply is that local reductionism is not a
description of how beliefs get formed but an account of **when a belief deserves to
count as knowledge**, and a corpus is precisely the artifact for which that
distinction earns its cost: it outlives the conversation that produced it, is read
by people who were not there, and is consulted by agents that cannot ask a
follow-up question. The calculation the reader would have performed is the thing
being stored.

The consequence worth stating plainly, because it constrains everything downstream:
**a source's reliability is never inherited by its claims.** No amount of standing
admits an unquoted assertion, and no disqualification of a source retroactively
invalidates a claim whose quote still validates.

______________________________________________________________________

## 2. Non-Goals

- **`gnosis` never calls a model.** No provider SDK, no API key, no network call
  to an inference endpoint. It fills prompts and consumes replies. A
  `direct-model` seam is specified in §16.4 as a stub only, per the manifesto's
  note that the harness "has a stub direct model interface so it could
  potentially dispense with" Claude/Gemini/Qwen.
- **Not a RAG service.** No embedding pipeline is required for conformance.
  Vector search is an optional, explicitly-degradable capability (§11.4).
- **Not an authoring tool for humans.** Humans curate sources, ask questions, and
  adjudicate. The corpus body text is model-written by design.
- **Not a replacement for Obsidian.** The bundle stays plain markdown with
  wikilink-compatible cross-references so Obsidian, `leafwiki`, or any markdown
  reader keeps working. The web UI (§13) is an authenticated convenience, not the
  canonical reader.
- **Not a general wiki engine.** Bundle shape is OKF; anything OKF forbids,
  `gnosis` forbids.
- **No score that could become a ship threshold.** See §17.

______________________________________________________________________

## 3. Position in the Tool Family

`gnosis` is the fifth CLI consumer of `skillet`, alongside `exegesis`,
`skillsaw`, `adh`, and `canonizer`. It is the *knowledge* tool where the others
are *artifact* tools.

| Tool        | Governs              | Meets `gnosis` at                                                         |
| ----------- | -------------------- | ------------------------------------------------------------------------- |
| `skillet`   | shared kernel        | every shared type; `gnosis` adds no private copy of anything skillet owns |
| `exegesis`  | skill-tree structure | `quotecheck` / `textnorm` (§7.2); the `--check` drift idiom               |
| `skillsaw`  | skill quality        | nothing directly; both consume `skillet/ratchet`, `stats`, `calibration`  |
| `canonizer` | ruleset quality      | `finding.Diagnostic`; the cold-critic loop; `ruleset` as a claim carrier  |
| `adh`       | change execution     | `proof` packets; the miss-log discipline (§6.4)                           |

Two promotions to `skillet` are triggered by `gnosis` existing, both on the
family's promote-on-second-consumer rule:

1. **`quotecheck`** — currently `exegesis/internal/quotecheck`. `gnosis` is its
   second consumer and its most important one: it is the fabrication guard over
   ingested sources. Recorded in `skillet/TODO.md`.
2. **`ruleset/conflict`** — contradiction detection. `canonizer` is consumer one
   (a ruleset's internal consistency), `gnosis` is consumer two (a corpus's).
   Recorded in `skillet/TODO.md` under *Contradiction Detection*.

`textnorm` already landed in `skillet` and MUST be the only text-folding
implementation `gnosis` uses. A second normalizer is the exact defect that
promotion fixed.

______________________________________________________________________

## 4. Architecture — Four Tiers

Karpathy names three layers: raw sources (immutable), the wiki (model-owned), the
schema (co-evolved configuration). `gnosis` splits his first layer in two,
because "immutable" is an assertion nothing currently enforces.

```text
Tier 0   evidence/            append-only, never rewritten, committed to git
           ├── text/<sha256[:2]>/<sha256>.<ext>   archived text: .md .txt .svg (see §4.3)
           └── fetch.jsonl                        every fetch: uri, hashes, kind, decision

Tier 1   .gnosis/quarantine/  admitted mechanically, not authoritative, NOT in the bundle
           └── <slug>.md                          trust: unverified

Tier 2   <bundle root>/       the authoritative corpus — an OKF bundle in git
           ├── index.md                           OKF §8 / Karpathy's index.md
           ├── log.md                             OKF §9 / Karpathy's log.md
           └── c/<uuid7>-<slug>.md                every concept; see §5.2

Tier 3   .gnosis/             derived, regenerable, gitignored
           ├── index.db                           SQLite: FTS5 + graph + caches
           ├── cache/                             content-addressed prompt/response cache
           ├── miss.jsonl                         why the deterministic path did not decide
           └── audit.jsonl                        every write, who and when
```

### 4.1 Why Tier 0 Exists

`llmwiki` enforces a byte-exact quote property but stores only hashes:
`sources` is `(uri UNIQUE, content_hash, ingested_at)`, `source_files` is
`(relative_path, content_hash, byte_size, line_count)`. It validates against the
**live** source. A moved PDF or a 404'd URL leaves the quote on disk and the
proof gone; the property is genuine and not durable.

Tier 0 fixes that: keep what the quote was validated against, key it
`(uri, content_hash)` so a changed source appends a version rather than
overwriting one, and point the validator at the archive.

The payoff is that one conflated failure becomes two with opposite responses:

| Condition                        | Meaning                   | Response                    |
| -------------------------------- | ------------------------- | --------------------------- |
| quote ∉ archived text            | fabrication or corruption | **block**, always           |
| archived text ≠ current upstream | the source moved under us | **flag stale**, never block |

`llmwiki` reports both as `evidence_invalid`; `canonizer` reports both as
`anchor-absent`. Neither can tell them apart, and they are not the same event.

### 4.2 Archive Holds Text, Not Sources

**No binary blobs, and no git-lfs.** The archive is committed to the same git
repository as the bundle, so it MUST stay small and diffable.

This costs less than it appears, because of what the guard actually compares.
`quotecheck` validates a quote against *text*. A PDF is never the thing a quote
is checked against — the text extracted from it is. So `gnosis` archives the
**normalized text extraction**, and records the original as provenance:

```text
source:  https://example.org/paper.pdf     ← never stored
         sha256 of the fetched bytes       ← recorded in fetch.jsonl
extract: evidence/text/ab/abc123….md       ← stored, committed, quote-validated
```

The honest consequence, stated rather than buried: the proof becomes *"this quote
appears in the text we extracted from a source whose bytes hashed to X."* If the
original vanishes, the extraction can no longer be re-derived from it — but the
quote remains verifiable against what was actually read, which is the property
that stops fabrication. Extraction fidelity becomes a trusted step, so the
extractor and its version are recorded per archived file.

### 4.3 Archive Admission Policy

Every fetched source gets exactly one of three deterministic dispositions,
recorded in `fetch.jsonl` and in `sources_fetched.disposition`:

| Disposition  | When                                                 | Evidence durability                                  |
| ------------ | ---------------------------------------------------- | ---------------------------------------------------- |
| `archived`   | text-like, passes the allowlist and size cap         | **durable** — quote validates offline, forever       |
| `extracted`  | binary or oversize, but a text extraction passes     | **durable** — quote validates against the extraction |
| `referenced` | neither the source nor an extraction can be archived | **weak** — hash and URI only; no offline proof       |

The gates, all in `standards/archive.toml`, none hardcoded:

- **Extension allowlist**: `.md`, `.txt`, `.svg`. Nothing else is archived
  directly. Extraction targets are always `.md` or `.txt`.
- **Text test, not extension trust**: a file is text only if it is valid UTF-8
  and contains no NUL byte. A `.txt` that is really a binary is rejected as
  binary, whatever it is named.
- **Per-file cap** (default 256 KiB) and a **per-corpus budget** with a warning
  threshold, so the repository cannot grow without someone being told.
- **No embedded payloads**: a data URI inside an archived file above a small
  threshold is rejected — otherwise a base64 raster in an SVG or markdown file
  reintroduces exactly the binary weight this policy excludes.

**Exactly one extractor is pinned, and it is not for PDFs.** HTML → markdown is
the common case and has a deterministic pure-Go answer, so it is pinned
(§7.2). **There is deliberately no PDF extractor**: the pure-Go options extract
poorly, and `pdftotext` would buy a non-Go external dependency that no other tool
in the family carries — for a format nobody has yet needed to quote. A PDF
therefore falls to `referenced`, which is a supported outcome rather than a
failure. Revisit on a real need, not in advance.

`referenced` is a first-class, non-failing outcome. OKF already contemplates a
source a consumer cannot dereference: OKF §5.1 permits `sources[].resource` to
name "a population or scope descriptor it cannot" follow. What `gnosis` adds is
that the *weakness is visible per claim* rather than averaged away — see §14.4.

**Policy: `referenced` claims are admitted to the authoritative corpus.** It is
reasonable to weakly trust a reliable external authority for a claim that is not
central to the domain — a standards document, a vendor's published limit, a
regulation cited once in passing. Excluding them would push real knowledge out of
the corpus to protect a property those claims were never going to have.

What makes that safe is not the admission rule but the visibility rule: the
weakness is permanent, per-claim, and queryable, and it is **weighed against how
load-bearing the claim is** rather than reported flat (§14.4). Weak evidence
under a peripheral claim is ordinary; weak evidence under a claim the rest of the
corpus leans on is the thing worth surfacing, and centrality is derivable from
the link graph rather than declared.

### 4.4 SVG Is Active Content

SVG is on the allowlist because diagrams are genuinely useful evidence and are
diffable text. It is also XML, which makes it the one allowed format that can
attack a reader. Before any SVG is archived it MUST be sanitized, and
sanitization is a **rejection**, not a rewrite — a file that needs stripping is
refused, so what is committed is what was fetched:

- no `<script>`, no event-handler attributes (`on*`)
- no external references: `xlink:href` / `href` to any non-fragment target, no
  `<use>` across documents, no `<image>` with a remote URI
- no `<!DOCTYPE>`, no entity declarations (XXE, billion laughs)
- no `<foreignObject>`
- text content is subject to the same hidden-character scan as any other source
  (§9.3) — an SVG can carry invisible text as easily as markdown can

The web interface (§13) MUST serve archived SVG with
`Content-Security-Policy: default-src 'none'` and
`Content-Type: image/svg+xml`, from a path that never shares an origin with an
authenticated session. An SVG that renders is still a stored-XSS surface; the
sanitizer is the first defence and the CSP is the second.

### 4.5 Tier 2 Is the Only Source of Truth

The SQLite database is a **regenerable cache**, not a store. `graft` draws this
line well: the graph is gitignored, what gets committed is the wiring. `gnosis`
follows it — `gnosis index rebuild` MUST reproduce `index.db` byte-identically
from the bundle plus tier 0, and CI MUST verify that (§18.3). Anything that
exists only in SQLite is a bug.

### 4.6 Concurrency: One Writer per User, Git Between Users

There are two independent concurrency problems here and conflating them produces
the wrong design for both.

```text
      user A                              user B
 ┌──────────────────────┐            ┌──────────────────────┐
 │ CLI  agent  viewer   │            │ CLI  agent  viewer   │
 │   └────┬────┘        │            │   └────┬────┘        │
 │        │ local API   │            │        │ local API   │
 │   ┌────▼─────┐       │            │   ┌────▼─────┐       │
 │   │  writer  │       │            │   │  writer  │       │
 │   └────┬─────┘       │            │   └────┬─────┘       │
 │  bundle + index.db   │            │  bundle + index.db   │
 └──────────┬───────────┘            └───────────┬──────────┘
            └───────────►  git  ◄────────────────┘
                    (markdown + tier 0 only)
```

**Within one user: many clients, one writer, coordinated by a local API.** A
person runs several tool instances at once — a CLI invocation, one or more agents,
a viewer — and all of them reach the corpus through a single writing process. The
requirement this creates is easy to state too narrowly, so it is stated in full:

> **The writer owns the bundle, not merely the database.** Serializing SQLite
> writes and leaving markdown writes unserialized would coordinate the cache and
> not the corpus. Two agents promoting a claim concurrently is a bundle problem,
> and SQLite's locking has nothing to say about it.

Readers do not need the writer and MUST NOT require it: `lint`, `search`, `show`,
and `graph` open the index directly, which is why `busy_timeout` is set (§5.5) and
why nothing read-only creates state (§4.5). A corpus must be inspectable when no
daemon is running.

This gives `gnosis serve` a second role, and earlier than §13's viewer. One
process, two jobs: **write coordinator** and **viewer**. Two servers would mean two
authorities over one bundle, which is the problem being solved rather than a
solution to it.

**Between users: nothing is shared but git, and the index is never a merge
target.** Every user has their own `.gnosis/index.db`, gitignored and derived, so
there is no distributed database, no locking protocol, and no synchronization
story. The shared artifact is the bundle plus tier 0 — markdown and archived text,
merged as text by the tool built for it.

This is what §4.5's byte-identical requirement is actually for. Stated per-user it
sounds like a determinism nicety; stated across users it is the load-bearing
property: **two users at the same commit hold the same index**, so a disagreement
between them is a disagreement about the corpus and never about their caches. It
also makes `index rebuild` the routine consequence of `git pull` rather than a
repair.

#### 4.6.1 What Git Merges Well, and the One Case It Merges Too Well

Text merging handles most of this correctly, and it is worth being explicit about
the exception because it produces no conflict marker at all.

| Situation                                           | Git result                     | Then what                          |
| --------------------------------------------------- | ------------------------------ | ---------------------------------- |
| Two users edit one document                         | ordinary text conflict         | a person resolves it; correct      |
| Two users add different claims to one document      | frontmatter conflict, or clean | ids are UUIDv7, so no collision    |
| **Two users independently write about one subject** | **clean merge, two documents** | **`duplicate` (§12) — post-merge** |
| Two users append to `evidence/fetch.jsonl`          | conflict on the final line     | see the caveat below               |

The third row is the one that matters. Because identity is assigned rather than
derived from content (§5.1.3), two people documenting the same thing produce two
different identifiers, and git has no reason to object — it merges both files
cleanly and the corpus quietly contains the same knowledge twice. Nothing is
broken, no check fails at write time, and the condition is invisible until someone
looks.

So `duplicate` is not a hygiene check about careless copying. It is **the merge
reconciliation step for a distributed corpus**, it runs after `git pull` rather
than before commit, and `Fold`-equal titles across documents is exactly the signal
it needs. This is the cost of assigned identity, and it is the right cost: the
alternative — deriving identity from content so that duplicates collide — would
mean every typo correction changed a document's identity (§5.1.3).

The `fetch.jsonl` row is a real defect rather than a design: a single append-only
file in a merged tree conflicts on every concurrent append. The archive itself is
content-addressed and merges perfectly; only the ledger has the problem, and the
fix is recorded in `TODO.md`.

______________________________________________________________________

## 5. Data Model

### 5.0 Three Words, Defined Once

Three things are easy to conflate and this specification depends on keeping them
apart. An earlier draft did not, and the cost was a data-model recommendation
that contradicted §5.5's own worked example.

| Term        | Is                                                                                                                                                        | Granularity          |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------- |
| **concept** | an OKF document: one file under `c/`. OKF's word, not ours — OKF defines "Concept ID" as *the path of the concept's file*, so a concept **is** a document | one file             |
| **claim**   | one addressable assertion *inside* a document                                                                                                             | a span within a file |
| **subject** | the thing claims are *about*, and can be compared on                                                                                                      | a vocabulary key     |

- **`concept` and `document` are the same thing** and both words appear here:
  `concept` where OKF's vocabulary or the `c/` directory is in view, `document`
  where the index is. Nothing turns on which is used.
- **`claim` is this project's own word**, chosen because every neighbouring
  tradition had already spent the alternatives on the *document*: Bush's unit is
  an "item", Luhmann's is a "note", OKF's is a "concept". A sub-span needs a word
  nobody has used for the whole.
- **A document reaches a subject only through its claims.** One document carries
  claims about several subjects; one subject is bounded by claims in several
  documents. Both directions are many-to-many and there is no direct
  document-to-subject relation (§5.5).

### 5.1 Identity Is Opaque, Immutable, and Recorded Twice

Every concept carries a **UUIDv7** assigned once, at admission, and never
rewritten — not on move, not on retitle, not on supersession, not when its body
is replaced wholesale. A superseded concept keeps the identifier it was born
with, which is what makes "what did we believe in March, and why did it change"
an answerable question.

v7 rather than v4 or a timestamp: it is time-ordered, so it sorts chronologically
and gives the index natural locality, and it is collision-free, which a
second-resolution timestamp is not. Legibility is not a criterion, because no
human ever types one — navigation is by presented path (§5.6).

**The identifier is recorded in two independent places:**

1. `gnosis_id` in the document's own YAML frontmatter.
2. The `documents` row in `.gnosis/index.db`.

That redundancy is not belt-and-braces; it buys two specific properties.

- **The database stays a derived cache.** If identity lived only in SQLite, that
  database would be the sole record of it — undeletable, unrebuildable,
  necessarily committed, and a binary merge conflict waiting to happen. Because
  every document carries its own identifier, `gnosis index rebuild` is total:
  delete the database, rebuild from the bundle and the archive, get a
  byte-identical result. §4.5 depends on this.
- **Disagreement is detectable.** Two independent records of the same fact can be
  compared. One cannot.

A knowledge base therefore always *has* a SQLite database and never *commits*
one. `.gnosis/` is gitignored in its entirety.

#### 5.1.1 Filenames Are Composite: `<uuid7>-<slug>.md`

Concepts live at `/c/<uuid7>-<slug>.md`. The identifier prefix is immutable; the
slug is advisory, derived from the title, and rewritten whenever the title
changes.

This shape is chosen against three constraints that pull in different
directions:

| Constraint                                              | Why a naive answer fails                                                                                         |
| ------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| Links must survive moves and retitles                   | A purely human path breaks on every rename                                                                       |
| OKF §6.1 links are *paths*, and consumers traverse them | A virtual `/c/<uuid>` route that maps to no file leaves every link broken to every consumer that is not `gnosis` |
| Review happens by pull request, often by non-engineers  | A directory of bare UUIDs makes `git log` and a PR diff unreadable, and review is how knowledge is admitted      |

A composite name satisfies all three. The path is real, so `grep`, Obsidian, a
static-site generator, and any other OKF consumer resolve links without
bespoke tooling. The diff says `c/01932b7c-…-retry-budget.md`, which a reviewer
can read. And the immutable prefix means a **stale link still resolves**: the
resolver parses the identifier out of the path and matches on it, so a link
written before a retitle keeps working with no mapping table and no lookup —
just a prefix match.

Retitling rewrites the filename and every inbound link in one atomic commit.
`gnosis` owns these files, so this is cheap and needs no coordination.

#### 5.1.2 Reconciliation

Nothing in this design assumes external tools behave. Git merges, reverts,
another agent, or someone fixing a typo in an editor can all disturb the
correspondence between filename, frontmatter, and index. `gnosis doctor` and
every write path check it, and there are six distinct outcomes, not one:

| Observed                                                 | Meaning                    | Response                                                |
| -------------------------------------------------------- | -------------------------- | ------------------------------------------------------- |
| `gnosis_id` present, no index row                        | new file, or index stale   | index it                                                |
| Index row, file absent, identifier found at another path | moved or renamed           | update the index path; no content change                |
| Index row, identifier found nowhere                      | deleted outside `gnosis`   | tombstone, and report — never silently drop             |
| **Two files bearing one identifier**                     | a copy, or a bad merge     | **flag both; neither wins; a human decides**            |
| File with no `gnosis_id`                                 | created outside `gnosis`   | quarantine it (§4); never assign an identifier silently |
| Frontmatter identifier ≠ index row for that path         | both changed independently | conflict; report with both values                       |

The duplicate case is the one that matters. An automatic winner there discards
someone's work silently, which is the failure this whole design exists to
prevent.

Filename drift is a seventh, benign case: a filename whose slug no longer
matches the title is corrected on the next write, and reported by `lint` in
between. The identifier prefix, not the slug, is authoritative.

#### 5.1.3 Content-Addressed Identity Is a Stated Non-Goal

No identifier in this design is derived from the content it names — not a
document's, not a claim's (§5.5.1). The temptation is real, because a content
hash reconciles effortlessly: two extractions of the same text produce the same
identifier with no bookkeeping.

It is the wrong trade, and the reason is structural rather than aesthetic. A
content-addressed identifier changes when the content changes, which means
correcting a typo, tightening a sentence, or resolving an ambiguity orphans every
verdict, evidence row, verification event, supersession edge, and open finding
attached to that text. The corpus would systematically lose its accumulated
judgment at exactly the moments a claim was being improved.

Luhmann is explicit that this is the load-bearing decision, not a detail: an
order based on content "would mean that you would have to adhere to a single
structure forever (decades in advance!)", whereas a fixed content-free number is
"exactly that reduction of complexity of possible arrangements that unlocks the
creation of high complexity."

Content hashing therefore has one job here and it is not identity: **matching**.
`anchor_hash` matches a re-extracted claim to an existing one, and
`sources_fetched.source_sha256` matches fetched bytes to an archived file.
Matching answers *is this the same text?*; identity answers *is this the same
thing?*, and those questions have different answers every time someone edits a
sentence without changing what it asserts.

#### 5.1.4 OKF Conformance

Adding `gnosis_id` is conformant without argument. OKF §4.1 permits
producer-defined keys; OKF §11 requires consumers to preserve unknown keys and
forbids rejecting a document for carrying them. A foreign consumer sees an extra
frontmatter field and a slightly odd filename, and everything else works.

### 5.2 One Format Rule, Three Formats

Three serialisation formats appear in a knowledge base, and which one applies is
decided by a rule rather than per file:

| Format | Used for                                                                             | Why                                                            |
| ------ | ------------------------------------------------------------------------------------ | -------------------------------------------------------------- |
| YAML   | OKF concept frontmatter                                                              | **Mandated by OKF §4** — not gnosis's choice                   |
| TOML   | configuration a human edits to change behaviour: `ontology.toml`, `standards/*.toml` | unambiguous scalars, and a mistyped key is *detectable*        |
| JSON   | machine-generated artifacts and interchange: `--jsonl` output, manifests, SARIF      | the consumer is a program, and the format is often fixed by it |

The middle row is the one worth justifying, because YAML would have been the
lazy choice for consistency with frontmatter. Measured against `goccy/go-yaml`,
YAML 1.2 is better than its reputation — `no`, `yes`, `on`, and `12:30` all stay
strings, and duplicate keys are already an error — but two coercions survive:
`0755` becomes an integer and `1.20` becomes `1.2`, silently losing the trailing
zero. TOML has neither, because a bareword is a syntax error with a line number
rather than a guess.

The decisive difference is not coercion, though. It is that a **mistyped key in
TOML is reportable**: `toml.Decode` returns `MetaData.Undecoded()`, so
`normatve = true` is caught. Decoding YAML into a map cannot distinguish a typo
from a producer-defined extension. For a vocabulary file that a mixed group edits
during review, a silently ignored flag is the expensive failure.

Frontmatter cannot benefit from that, because OKF *requires* unknown keys to be
preserved rather than rejected (OKF §11) — so the strictness that helps
configuration would be non-conformant there. Hence two formats rather than one,
for a stated reason.

### 5.3 Bundle Format Is OKF, Unextended Where Possible

The bundle conforms to **OKF v0.2**. Reference:
`~/Documents/agent-blue/knowledge-catalog/okf/SPEC.md` (Apache-2.0).

OKF is already the format the manifesto's knowledge base is stored in — a
directory of markdown with YAML frontmatter, no schema registry, no required
tooling — so conformance costs no migration. More importantly, **it already
specifies the four things the manifesto proposed to invent**: provenance
(`sources`), trust (`generated` / `verified`), lifecycle (`status`), and freshness
(`stale_after`).

The convergence with Karpathy is exact and not coincidental: OKF §3.1 reserves
`index.md` and `log.md` at any level of the hierarchy, with OKF §8 defining
index-as-progressive-disclosure and OKF §9 defining a log of `## YYYY-MM-DD`
headings newest-first. Those are Karpathy's two special files, including his
grep-the-prefix trick.

`gnosis` MUST implement OKF conformance (OKF §11) as written, including the negative
requirements, which are load-bearing for incremental adoption:

- MUST NOT reject a concept for a missing optional family.
- MUST treat a bare `verified` mapping as a one-element list.
- MUST tolerate unknown `type` values, unknown extra keys, and **broken
  cross-links** — a link to a not-yet-written page is knowledge about a gap, not
  a defect.
- MUST preserve unknown keys when round-tripping.

Required frontmatter is `type` alone. A concept carrying only `type` is fully
conformant, and `gnosis lint` MUST NOT say otherwise.

### 5.4 Frontmatter `gnosis` Reads

All OKF §4.1, OKF §5, and OKF §10 fields, plus these `gnosis`-namespaced
extensions.
Extensions are prefixed `gnosis_` so a foreign OKF consumer ignores them
cleanly per OKF §4.1.

| Key                  | Type       | Meaning                                                                   |
| -------------------- | ---------- | ------------------------------------------------------------------------- |
| `gnosis_id`          | UUIDv7     | **required.** Immutable identity; the redundant half of §5.1              |
| `gnosis_evidence`    | list       | per-claim evidence — see below                                            |
| `gnosis_supersedes`  | list of id | concepts this one replaces; pairs with `status: deprecated` on the target |
| `gnosis_challenges`  | list       | reader-filed challenges and their state (§10.7.4)                         |
| `gnosis_warrant`     | mapping    | adjudication warrant; `rationale` is required — see §10.4, §10.6          |
| `gnosis_conflicts`   | list       | open contradiction findings, each naming a concept id and a finding id    |
| `gnosis_limitations` | list       | what this concept does **not** cover; required on normative types (§17)   |
| `gnosis_subject`     | string     | a subject key from `ontology.toml` (§5.8); what this claim is about       |
| `gnosis_constraint`  | mapping    | optional; pins an ambiguous reading. Normally derived (§10.2.1)           |

`gnosis_supersedes` and `gnosis_conflicts` name **identifiers, never paths**,
for the reason in §5.4: an edge that survives reorganization is the point.

An evidence entry names the archived text it was validated against, not the
original source:

```yaml
gnosis_evidence:
  - quote: defer is commonly used to close a file
    source_id: gopl                       # keys into OKF `sources`
    archive: text/ab/abc123….md           # bundle-relative, under evidence/
    durability: archived                  # archived | extracted | referenced
```

`gnosis_evidence` carries the corpus's one hard invariant, and its scope is
exactly as strong as the archive allows:

> For `durability: archived` or `extracted`, the quote MUST be present in the
> named `evidence/` file under `textnorm.Fold` normalization. Checked on every
> write path (§9.4); a failure drops the concept and leaves the prior version
> standing.
>
> For `durability: referenced`, there is no local text to check against, so
> `archive` is absent and **no quote invariant applies**. The claim is
> admissible and permanently marked weak (§14.4). `gnosis` MUST NOT silently
> validate a referenced claim against a live re-fetch and present the result as
> equivalent — that is precisely the durability illusion §4.1 exists to remove.

### 5.5 Derived Index

Pure Go SQLite via `modernc.org/sqlite` — no CGo, matching the family's
single-binary posture (`clu` and `qvr` both made this call). FTS5 is available in
that driver and is the baseline search (§11). Schema migrations are numbered
steps applied off `PRAGMA user_version`, the same mechanism `skillet` and `zk`
both use.

Every table below is reconstructible from the bundle plus the archive.

```sql
-- A document. `id` is the UUIDv7 from gnosis_id; `path` is where it currently
-- sits and may change. Nothing outside this table joins on path.
documents(id PK, path UNIQUE, slug, content_hash, byte_size,
          created_at, modified_at)

-- An addressable assertion inside a document. A document holds one or more.
-- `anchor_hash` is the textnorm.Fold hash of the anchoring text and is the
-- claim's address; `pos` is a byte offset into the document BODY, cached for
-- "send the reader here", NULL when the anchor cannot be located, and never
-- identity. See 5.5.1 and 5.5.2.
claims(id PK, document_id, anchor_hash, pos, type, title, description,
       status, stale_after, generated_by, generated_at, lead)

-- Full text over documents. Phase 1 searches this; claims_fts arrives with
-- extraction in Phase 2 (19). Both tokenizers come from one Go constant, so
-- they cannot drift apart.
documents_fts USING fts5(
  title, body,
  content = documents, content_rowid = rowid,
  tokenize = "porter unicode61 remove_diacritics 1 tokenchars '''&/'")

-- The ontology.toml registry, indexed. Both are derived from the artifact.
subjects(key PK, dimension, description, deprecated)
subject_aliases(key, alias)                    -- surface phrase -> resolved key
entity_aliases(entity_key, alias)              -- many names for one thing
claim_subjects(claim_id, subject_key, op, value_norm, value_raw,
                 dimension, derived, pattern_id)  -- derived=1 unless pinned
verifications(claim_id, by, at)             -- OKF §5.2 list; one row per event
tags(document_id, tag)
sources(claim_id, source_id, resource, title, author,
        usage_count, last_modified, window_from, window_to)
-- `pos` here is an offset into the ARCHIVED SOURCE, not into the document — a
-- different coordinate space from claims.pos. See 5.5.2.
evidence(claim_id, pos, quote, source_id, archive_path, fold_hash, durability)

-- A link keeps what the author wrote even when it resolves to nothing.
-- snippet_start/end are byte offsets into the document BODY, the same space as
-- claims.pos. See 5.5.2.
links(id PK, source_claim_id, target_document_id, href, title,
      rel, external, snippet, snippet_start, snippet_end)

claims_fts USING fts5(
  title, description, body,
  content = claims, content_rowid = id,
  tokenize = "porter unicode61 remove_diacritics 1 tokenchars '''&/'")

-- One row per fetched source. source_sha256 is always recorded; archive_path is
-- NULL for `referenced`. extracted_from links an extraction to its origin.
sources_fetched(source_sha256 PK, uri, fetched_at, byte_size, media_type,
                disposition, archive_path, extractor, extractor_version,
                extracted_from, reject_reason)
fetch_history(uri, source_sha256, fetched_at)  -- (uri, sha256); append-only
llm_cache(cache_key PK, source_hash, prompt_hash, model, model_version,
          response, created_at)
findings(id PK, kind, severity, category, certainty, fix_class, state,
         opened_by, challenge_class,
         claim_id, other_claim_id, message, opened_at, closed_at)
```

Notes on shape, each of which is load-bearing:

- **`documents` and `claims` are separate, and a document holds many claims.** A
  single sentence can carry two assertions — *"The cache is enabled by default,
  but it is not shared across sessions"* is one sentence and two claims — so a
  one-claim-per-file index cannot attach a verdict to the right half, and a quote
  could validate while the other half goes unsourced. Splitting the table is what
  makes a claim addressable **without splitting the file**.
- **A document is associated with a subject *through its claims*, never
  directly.** That is what makes the association many-to-many in both directions:
  one document carries claims about several subjects, and one subject is bounded
  by claims in several documents. There is deliberately no
  `document_subjects` table. Hierarchical placement — one document, one location,
  one topic — is the failure both progenitors of this design named independently:
  Bush's "it can be in only one place, unless duplicates are used" and Luhmann's
  "multiple storage problem." Their shared answer is to place it anywhere and
  link, which is what an opaque path plus a join is.
- **Nothing joins on `path`.** Every foreign key is an identifier. A
  reorganization therefore cannot break a supersession edge, orphan an open
  finding, or invalidate an evidence row.
- **`links.target_document_id` is nullable and `href` is always retained.** A
  link to a document that does not exist is a legal row, not an error — OKF §6.1
  calls that "not-yet-written knowledge" — and deleting a document *degrades* its
  inbound links rather than erasing them. Keeping `href` matters most in exactly
  the case where the target is missing, because the href is then the only
  surviving record of what the author meant.
- **`links.snippet` with offsets stores the prose surrounding the link.** OKF
  §6.1 says a relationship's kind "is conveyed by the surrounding prose, not by
  the link itself", so the snippet *is* the untyped relation's evidence. It also
  lets a reader see why A links to B without reopening A.
- **`links.rel` types the relation in the index, never in the markdown.** The
  source format stays plain; the type is derived.
- `verifications` is a table rather than a column because OKF §5.2 makes
  `verified` a list of independent events, specifically so a human sign-off and
  an automated pass stay distinguishable. Collapsing it destroys the distinction
  that makes trust tiers meaningful.
- `entity_aliases` carries the surface-term-versus-canonical-thing problem, and
  it hangs off an entity rather than a claim. Many names for one *thing* is a
  property of the thing; "a claim with two names" is close to meaningless. `tags`
  hangs off the document for the same reason — a reader tags what they are
  browsing, not one assertion inside it.
- `evidence.pos` records where a quote sits **in the archived source**, so a
  reader can be sent to it. It does not share a coordinate space with
  `claims.pos`, which is why §5.5.2 states both.
- `fetch_history` is keyed `(uri, sha256)` so a changed source appends a version.
  A `uri UNIQUE` key would permit exactly one version per source and therefore no
  source history at all.
- `findings.state` is open, closed, or **deferred**. It is not a score. See §17.
- `findings.opened_by` names a check or a person, because "who says so" is the
  first thing a reader asks of a finding and a check name is as much an answer as
  an actor is. `challenge_class` is set only on a reader-filed challenge (§10.7)
  and is null for everything a check raised.

#### 5.5.1 Claim Addresses Live in the Document, Not in the Index

Every table above is reconstructible from the bundle plus the archive, and the
`claims` table is the one where that guarantee is hardest to keep. It is stated
here as a requirement rather than left to be inferred, because an earlier draft of
this section violated it.

A claim has an assigned identifier and an address. Both MUST be recoverable from
the document alone:

```yaml
gnosis_claims:
  - id: 01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d
    anchor: The cache is enabled by default
  - id: 01932b7c-2a03-7b11-8e44-9f10c2d3e4f5
    anchor: it is not shared across sessions
```

- **`id` is assigned once and never derived from content.** Deriving it from the
  text would mean that fixing a typo orphans every verdict, every evidence row,
  and every conflict finding attached to that claim. This is Luhmann's central
  structural decision — the number is content-free precisely so it survives
  content change — and it applies one level down from the document.
- **`anchor` is the claim's address, matched under `textnorm.Fold`.**
  `claims.anchor_hash` caches the fold hash so a match is an index lookup. Folding
  is what makes the address survive rewrapping, a curly-quote substitution, or a
  whitespace change, none of which alter what the claim says.
- **`pos` is a cached byte offset and is NOT identity.** Reflowing one paragraph
  invalidates every offset below it with nothing to say which claim moved where. A
  byte offset is a location; `anchor_hash` is an address. `index rebuild`
  recomputes `pos` from the anchor, never the reverse. Its units and origin are
  §5.5.2, and they are not obvious.

Two consequences follow, and both are requirements:

- **Re-extraction reconciles; it does not replace.** A second extraction pass —
  including one by a different model — matches emitted claims against existing
  `anchor_hash` values and preserves the identifiers of those that match. Only an
  unmatched claim receives a new identifier. Without this, every re-run churns
  identity and orphans every verdict earned by the previous one.
- **An anchor that no longer appears in its document is a finding, not a
  repair.** `lint`'s `claim-anchor` check reports it and stops. The claim may have
  been rewritten (identity should survive), deleted (a tombstone), or split (two
  claims where there was one), and nothing deterministic distinguishes those three
  from the outside. Guessing would silently reattach a verdict to a claim nobody
  verified.

This is the mechanism Luhmann implemented with ink: "In the note itself, I use red
letters or numbers to mark the place of connection." The address is inscribed in
the artifact, and the register indexes it. An address that lives only in a
regenerable cache is not an address.

#### 5.5.2 Position Conventions

Three columns hold positions — `claims.pos`, `evidence.pos`, and
`links.snippet_start`/`snippet_end` — and they do **not** all mean the same thing.
Stating the convention once is cheaper than discovering the discrepancy from a
reader sent to the wrong paragraph.

| Column                    | Unit  | Measured from                         |
| ------------------------- | ----- | ------------------------------------- |
| `claims.pos`              | bytes | start of the document **body**        |
| `links.snippet_start/end` | bytes | start of the document **body**        |
| `evidence.pos`            | bytes | start of the **archived source file** |

**Bytes, not runes, and not line:column.** `strings.Index` returns bytes, `textnorm`
operates on strings, and every producer and consumer of these values is Go — a rune
offset would mean a conversion at each boundary and would still not be what a
person wants. What a person wants is `line:column`, and that is *derived* at render
time from the offset plus the text, never stored: two representations of one
location drift, and the derived one is the one that can be recomputed.

**From the start of the body, not the start of the file.** This is the part that
looks arbitrary and is not. §5.5.1 puts claim identities in `gnosis_claims`
frontmatter, so **adding a claim changes the frontmatter's length** — and every
file-relative offset in that document would shift, for every claim, because a
different claim was added. The prose did not move. Body-relative offsets are
invariant under frontmatter edits, and `okf.Parse` already separates the two, so
the body is both the correct origin and the one already at hand.

**Absolute within its space, never relative to another claim.** A position measured
from the preceding claim's anchor would chain: inserting one claim renumbers every
claim after it, and one unlocatable anchor corrupts every position downstream of
it. Since each `pos` is recomputed independently by searching for its own anchor,
chaining would add coupling and buy nothing. Each position is independently
correct or independently wrong.

**`evidence.pos` is in a different coordinate space, and this is not an
inconsistency to be cleaned up.** A claim's anchor lives in a bundle document; an
evidence quote lives in an archived source file under `evidence/text/`. Those are
different files with different lifetimes — the archived source is immutable
(§4.1), so an offset into it is *stable* rather than cached, which is the opposite
of `claims.pos`. Sharing a column name across two spaces is the hazard; the two
spaces are inherent.

**NULL means "the anchor could not be located."** Not zero — zero is a valid
position, the first byte of the body. §5.5.1 makes an unlocatable anchor a finding
that is never auto-repaired, so the schema needs a value for "this claim's text is
not where its anchor says it is," and a caller reading `pos = 0` as that state
would send readers to the top of the document.

### 5.6 Presented Paths Are a View, Not the Storage

The path a document is stored at and the path a human navigates by are different
things, and conflating them is what makes a layout decision expensive.

- **Canonical** — `/c/<uuid7>-<slug>.md`. What is written in document bodies,
  what OKF consumers traverse, what the index records.
- **Presented** — whatever hierarchy is ergonomic for a reader: by type, by tag,
  by team, by recency, several at once. Computed from frontmatter and the index,
  never stored, and free to change.

The web interface (§13) and the CLI navigator both render presented paths.
Neither writes one into a document.

**The hazard this creates, and the rule that contains it:** people copy URLs out
of the address bar and paste them into concepts. If a presented path is what the
browser shows, that is what gets pasted, and a fragile reference is back. So the
canonical form is authoritative and presented forms resolve *to* it, never the
reverse — the same rule every content system with a permalink/slug split arrives
at. Three mechanisms, because one is not enough:

1. the canonical form is reachable and copyable in the viewer;
2. an explicit "copy canonical link" affordance;
3. **normalization at admission** — a presented-path URL found in a submitted
   body is rewritten to canonical before the document is written. Only the third
   catches a paste from someone else's browser.

Because presented paths are views, choosing one is not a schema decision and can
be made after the corpus exists (§20).

**Explanatory and exploratory are different jobs, and one artifact must not do
both.** `search`, `graph`, and `critic` are exploratory: they exist to find what
might be worth attention, and their output is properly voluminous. `index.md` and a
`show` rendering are explanatory: they exist to convey something specific to a
reader who arrived with a question. Opening a hundred oysters to find two pearls is
the exploratory job; handing someone the hundred oysters is a failure to do the
explanatory one, and it is a tempting failure because the hundred are evidence of
work performed. So `index.md` is a curated map rather than a generated listing
(§4.5 makes generated listings free from `search`), and `doctor` and `lint` lead
with what a reader must act on rather than with everything they examined.

**The choice is bounded, which is why deferring it is safe.** Wurman's LATCH —
Location, Alphabet, Time, Category, Hierarchy — claims these are the only ways to
organise information and everything else is a composite of them. Taken as a
closed enumeration it makes this a choice among five rather than an open design
problem: a presented hierarchy is *by team or origin* (Location), *alphabetical*,
*by recency or `stale_after`* (Time), *by type, subject, or tag* (Category), or
*by containment* (Hierarchy). Several may be offered at once, since none is
stored. What LATCH does not address is identity, provenance, or conflict, and
treating it as a storage scheme rather than a presentation scheme is precisely the
conflation this section forbids.

### 5.7 Schema Document

Karpathy's third layer. `gnosis` generates and maintains `AGENTS.md` at the
bundle root, describing the corpus's conventions, its `type` vocabulary, the
ingest and query workflows, and the commands an agent should use.

Two rules, both borrowed:

- **One canonical file, symlinked.** `mdm rules link` makes `AGENTS.md`
  canonical and symlinks each agent's expected filename to it. `gnosis schema link` does the same, so Claude, Gemini, and Qwen read one file and cannot
  drift.
- **Generated sections are drift-checked.** The `type` vocabulary and command
  list are derived from the corpus and the binary; `gnosis schema --check` exits
  non-zero when the committed file is stale, exactly as `modelith render --check`
  and `exegesis index --check` do. Hand-written sections are preserved between
  markers.

An important caution recorded rather than ignored: an ETH Zurich study found
*auto-generated* context files reduced agent success in five of eight settings.
So `gnosis` generates the mechanical parts only — vocabulary, commands, paths —
and never writes the prose that tells an agent how to think.

______________________________________________________________________

### 5.8 Ontology: Types, Subjects, and Aliases

A taxonomy says where a document sits. An ontology says what its terms *mean* and
how they relate — and without one, no two claims can be compared, because
comparison requires knowing they are about the same thing. `gnosis` therefore
carries one committed, checkable artifact declaring both vocabularies:
`ontology.toml` at the bundle root.

```toml
version = 1
imports = ["platform-slo"] # composable; an imported key is referenceable

[[types]]
key = "Reference"
desc = "a recorded fact with no prescriptive force"
normative = false
expects_subject = false
aliases = ["Note", "Background"]

[[types]]
key = "Rule"
desc = "prescribes what must or should be done"
normative = true                                # requires gnosis_limitations (§17.2)
expects_subject = true                          # usually bounds something; a missing subject is flagged
aliases = ["Guideline", "Standard"]

[[types]]
key = "Procedure"
desc = "prescribes ordered steps; bounds nothing"
normative = true
expects_subject = false
template = "templates/procedure.md"
aliases = ["Runbook", "Playbook"]

[[types]]
key = "Decision"
desc = "a recorded trade-off and the reasoning behind it"
normative = true
expects_subject = false
template = "templates/decision.md"

[[types]]
key = "Glossary"
desc = "defines a term; feeds this vocabulary"
normative = false
expects_subject = false

[[subjects]]
key = "retry.max_attempts"
dimension = "count"
desc = "attempts made before the operation is abandoned"
aliases = ["retry budget", "retry cap", "maximum retries"]

[[subjects]]
key = "request.timeout"
dimension = "duration"
desc = "wall-clock limit on a single outbound request"
aliases = ["request deadline", "per-request timeout"]

[[subjects]]
key = "data.retention_period"
dimension = "duration"
desc = "how long personal data is kept before deletion"
aliases = ["retention window"]
requires_capability = true                              # rare; see §10.6.2 for both required conditions
```

**Subjects and types live in one artifact deliberately.** They are the same
governance problem — a controlled vocabulary the whole team has to share — and
splitting them would mean maintaining two namespaces, two review processes, and
two migration paths for one concern.

#### 5.8.1 Types and Subjects Are Populated Differently

They look alike and their urgency is opposite, which is why one is seeded and the
other accretes.

|             | Types                            | Subjects                              |
| ----------- | -------------------------------- | ------------------------------------- |
| Required by | OKF §4.1, on **every** document  | nothing; used only by §10.2 narrowing |
| Count       | few, structural                  | many, domain-specific                 |
| Contention  | low — a procedure is a procedure | high — "retry budget" or "retry cap"? |
| Needed by   | **Phase 1**                      | Phase 3                               |

**A type is a behavioural distinction, not a semantic label.** In this design a
type drives exactly three things: `normative`, `expects_subject`, and `template`.
So the test for whether a proposed type is real is mechanical:

> Two proposed types sharing all three flags and the same template are **one type
> with two aliases**.

That converts an unbounded question — what should we call things — into a bounded
one, and it gives a non-political way to settle a disagreement: not "your word is
wrong" but "these behave identically, so they are aliases." The seed above is
five types because five combinations of those flags earn distinct behaviour;
`Runbook` and `Playbook` do not, so they are aliases of `Procedure`.

`Decision` is the one worth arguing for hardest. It holds precisely the knowledge
this corpus exists to pool: a recorded trade-off, with its reasoning, present in
no source.

**Subjects start empty.** A key is added the first time two claims need comparing
and cannot be — the second-consumer rule applied to vocabulary. Every entry then
arrives with a real conflict as its justification, nothing is speculative, and the
registry grows only where comparison is actually wanted. Subjects therefore never
block Phase 1.

**The admission test is the clarification chain**, which turns "is this a
subject?" from a matter of taste into three questions with answers:

1. If it matters at all, it is detectable.
2. If it is detectable, it is detectable as an **amount** — or a range of amounts.
3. If it is detectable as an amount, it can be measured.

The second step is the one that decides, and it is why `dimension` is a required
field rather than a helpful one: **a proposed subject that cannot name a dimension
has failed step 2 and is not a subject — it is a tag.** `security` is a tag;
`session.idle_timeout` is a subject, because there is an amount of it. Claims can
be compared on the second and not the first, and comparison is the entire reason
the registry exists.

Applied by hand at review, the chain is two questions: *what do you mean,
exactly?* and *why do you care?* The useful outcome is often not a subject: asked
what they mean, people frequently refine the term into something already
measurable, or discover they cannot say — and "I cannot say what I mean by this
yet" is a better result than a vocabulary key nobody can apply.

#### 5.8.2 Aliases Are the Mechanism, Not a Convenience

Types and subjects both declare the surface phrases that resolve to them. This is
what lets each function write in its own words without anyone having to lose an
argument: engineering says "retry budget", support says "retry cap", and both
resolve to `retry.max_attempts`; one team writes `Runbook` and another `Playbook`,
and both resolve to `Procedure`. The **key is the resolved concept; the aliases
are the surface terms**, and both are retained — the alias is what a reader sees,
the key is what comparison and gating use.

Aliases on types carry a second job: they are how the merge in §5.8.1 is applied
without touching a single document. Deciding that two types behave identically is
one registry line, not a rename across the corpus.

Where two groups mean genuinely *different* things by one phrase, an alias is the
wrong tool. That is a bounded-context problem, and the answer is two subject keys
with distinct descriptions, not one key pretending to cover both.

##### 5.8.2.1 One Surface Phrase Resolves to One Key, and This Is Enforced

A phrase claimed by two keys is an **error**, and `ontology.toml` does not load.
Not a warning, not a last-writer-wins, not a resolution by declaration order.

The alternative was considered and refused. Perspective-based vocabularies —
several definitions per term, each keyed by the team that uses it — exist and
solve a real problem: they let teams interoperate without anyone losing an
argument about whose "incident" is the real one. The reason gnosis cannot adopt
one is narrower than a preference:

> **Comparison requires knowing that two claims are about the same thing.** A
> subject key is what makes two claims comparable at all (§5.8), and §10's
> predicates run on it. If one key can mean two things depending on who wrote the
> claim, then every comparison across that key is either a contradiction that
> isn't one or a silence where a real conflict lives — and the corpus cannot tell
> which without asking a person, which is the work it exists to avoid.

A perspective-keyed vocabulary is coherent for a glossary, whose job is to *record*
what people mean. It is incoherent for a comparison substrate, whose job is to
*decide* whether two claims disagree.

**The remedy is two keys, and the diagnostic MUST name it.** Reporting the
collision without the fix leaves an author to guess, and the guess is usually to
delete an alias, which loses the surface term rather than resolving the ambiguity.
`incident` meaning a customer-visible outage and `incident` meaning a
security event become `ops.incident` and `security.incident`, each with its own
description and its own aliases — and the phrase "incident" then belongs to
neither until somebody decides, which is the correct state for a word two groups
use differently.

**The cost is honest and it is a conversation.** This rule forces a discussion
that a perspective-keyed vocabulary would let a team avoid indefinitely. That is
the intent: the discussion happens once, at the moment the ambiguity is
discovered, rather than never — and §5.8.1's soft-deprecation path
(`deprecated: {message, error: false}`) is what makes the resulting rename
survivable once documents already reference the old key. Announce, then enforce.

**The limit of this rule is worth stating, because it is where the real risk
lives.** The error fires when someone *declares* a colliding alias. It cannot fire
when two groups have been using one word differently and neither has declared it —
which is the ordinary way the problem arises, and in that state `ontology.toml` is
perfectly valid and the corpus is quietly ambiguous. Nothing here detects that; the
signal it would need (a subject whose claim population is bimodal or dimensionally
inconsistent) is recorded in `TODO.md` as an open gap, not solved by this rule.

#### 5.8.3 Claims Without a Subject Are Flagged, Never Rejected

A claim of a type whose `expects_subject` is true, carrying no
`gnosis_subject`, is **reported for review** — never blocked, never assigned a
subject automatically. Blocking would make the corpus refuse ordinary knowledge;
guessing would put an inferred key underneath a comparison gate, which §10.3
refuses on principle. Reporting puts it in front of a person, which is where the
judgment belongs.

This is deliberately a *review* signal rather than a defect. Many claims of a
normative type legitimately constrain nothing, and the check earns its place by
being cheap to dismiss.

#### 5.8.4 Evolution

The vocabulary will be wrong at first, and the migration path is what makes that
survivable:

- **`imports`** keeps the artifact from becoming one unmaintainable file, and
  lets a shared vocabulary be reused across bundles.
- **Soft deprecation** — a key marked `deprecated: {message, error: false}`
  announces before it enforces. `lint` reports uses; nothing breaks until the
  flag flips.
- **Coverage checks** apply because the vocabulary is data: types no concept
  uses, concepts whose type is undeclared, subjects with no claims, and claims
  whose subject key is not in the registry.

## 6. The Determinism Contract

The manifesto's goal is a process as deterministic as possible. Determinism here
is achieved in four ways, in descending order of payoff.

### 6.1 Content-Addressed Response Cache

Every prompt `gnosis` emits is keyed
`sha256(source_content_hash ‖ prompt_hash ‖ model ‖ model_version)`. When a reply
is fed back, it is stored under that key. A second run over unchanged inputs
**makes no model calls and reproduces byte-identically.**

This is the single largest determinism win available and it is cheap. `qvr`'s
lock is the record shape: resolved identity plus verdict, so the artifact is its
own audit trail. It also collapses the cost of re-ingestion, which is what makes
a full corpus rebuild affordable.

Replay is explicit: `--cache-only` refuses to emit any prompt whose reply is not
already cached, and exits non-zero listing what is missing. CI uses it.

### 6.2 Deterministic Candidate Selection

`llmwiki` scans ~47 candidate pages per ingest and asks a model, per candidate,
whether it needs updating — up to 50 of roughly 65 calls. `gnosis` MUST NOT do
this. Candidates come from the SQLite index:

1. claims sharing a `source_id` with the incoming source;
2. claims in documents linked to or from those (one hop, `links`);
3. claims whose document shares a tag *and* an FTS5 term above a declared rank cut;
4. claims whose `gnosis_conflicts` names the incoming source.

The rule is `skillex`'s: same corpus state produces the same candidate set,
every time, because the index is a deterministic build artifact. Selection is a
query, not a judgment.

Every rank cut and hop limit is declared in `standards/` (§6.5), never a literal
in Go.

**A threshold moved in the direction that reduces findings MUST record that it
was.** `standards/` exists so that two runs over one corpus agree, and that
property is silent about whether the thresholds are any good. The failure it
enables is the one laboratory practice is notorious for: tune the instrument until
the runs are consistent, then report the consistency as reliability. A corpus can
be made to lint clean by widening a rank cut, lowering `MinPassageWords`, or
pushing a staleness default out, and every run afterwards will be perfectly
reproducible and perfectly quiet.

So each `standards/` value carries a `rationale`, and a change that loosens one is
recorded in `log.md` alongside the finding count before and after. This is not
version control — git already has the diff. It is the distinction between a
threshold that was wrong and a threshold that was inconvenient, which the diff
cannot show and which nobody reconstructs a year later.

#### 6.2.1 This Selector Is Biased, and the Bias Is Named

All four rules above draw from claims that already share something with the
incoming source: a source, a link, a tag, a term. That makes the candidate set a
**non-random sample**, and it is blind in a specific and unfortunate direction:
it cannot surface a contradiction between two claims that share no source, no
link, and no vocabulary. Those are exactly the *surprising* conflicts — the ones a
reader could not have found unaided, and therefore the only ones worth the cost
of a machine looking.

This is selection bias in the ordinary empirical sense, and the important
property of selection bias is that **it does not average out**. Raising the rank
cut or adding a second hop enlarges the sample without reducing its bias; a
larger biased sample is still biased. Tukey's verdict on Kinsey's 18,000
non-random interviews — that a random sample of 400 would have been better —
applies unchanged.

Two requirements follow:

- **A seeded random-sample pass runs alongside the selector.** A fixed number of
  claim pairs is drawn from *outside* the candidate set, with the seed recorded in
  `standards/` so the draw is reproducible under §18.3. Its purpose is not
  coverage — it cannot be — but measurement: it estimates what the selector is
  missing, which is otherwise unknowable.
- **The two paths are reported separately.** A conflict found by the selector and
  one found by the random pass carry different information about the corpus, and
  collapsing them hides the only signal that says whether the selector is
  adequate.
- **Random pairs need a plausibility guard, or the pass manufactures coincidences.**
  A pair drawn at random shares nothing by construction, which is the point — and
  it is also why an apparent conflict between them is more likely to be a collision
  of vocabulary than a contradiction. Asking whether pepperoni sales correlate with
  yogurt sales will eventually return yes, and the answer raises more questions
  about the method than about the groceries. The guard is the one already
  available: a random pair is only reported when both claims resolve to the **same
  subject key** (§5.8). That keeps the sample random with respect to source, link,
  and vocabulary overlap — the three axes the selector is biased on — while
  requiring the one thing that makes a contradiction meaningful at all.

The alternative — asserting the selector is sufficient because it is
deterministic — confuses precision for accuracy. The selector is perfectly
reproducible and systematically incomplete, and reproducibility is not evidence
of coverage.

### 6.3 Accretion Is Mechanical; Synthesis Is Gated

The manifesto asks for accretion. Appending evidence to a concept is accretion
and needs no model: the quote validates or it does not. Rewriting a concept's
body is *replacement*, and it is where a model silently drops what it did not
think important.

`gnosis` separates them:

- `gnosis ingest` appends `gnosis_evidence` entries and updates `sources`
  mechanically. No model, no body rewrite. This alone keeps the corpus current
  on facts.
- `gnosis synthesize <path>` is a separate, explicit, rarer operation that emits
  a rewrite prompt for one concept, and gates the reply on: every prior evidence
  quote still validating, no evidence entry silently dropped, and the diff being
  reported before the write.

`llmwiki`'s own update log has a `body_only` outcome, which shows the two are
already separable there.

### 6.4 Miss Log

`virgil`'s `internal/router/misslog.go` is the mechanism that makes "minimize
the model" measurable rather than aspirational. Its `MissEntry` records
`{Signal, KeywordsFound, KeywordsNotFound, FallbackPipe, AIPlan, AIConfidence}`
— and `KeywordsNotFound` is the load-bearing field, because it says *why* the
deterministic layers missed.

This file, `fetch.jsonl`, and `audit.jsonl` are **tracers**: instrumentation added
so that something which otherwise leaves no trace begins leaving one. A
determinism claim is untestable if the misses are invisible, and a miss is a
non-event — nothing happened, deterministically, and then a model was asked. The
log is what converts that into a countable observation, which is the whole reason
"minimize the model" can be a measured property rather than an intention.

`gnosis` writes `.gnosis/miss.jsonl` on every prompt emission:

```json
{
  "op": "conflict-adjudicate",
  "reason": "no_deterministic_predicate",
  "checks_run": [
    "severity_divergence",
    "interval",
    "enumeration"
  ],
  "checks_fired": [],
  "candidate": "/c/01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d-retry-budget.md",
  "other": "/c/01932b7c-2a03-7b11-8e44-9f10c2d3e4f5-timeout-policy.md",
  "at": "2026-08-17T14:02:11Z"
}
```

`gnosis miss report` aggregates by `reason` and `checks_run`. A reason that
recurs is a deterministic check waiting to be written; that is the backlog that
shrinks the model's surface area over time.

### 6.5 Standards as Data, Not Code

`AgentLint/standards/` is the model to copy: `weights.json`,
`reference-thresholds.json`, and `evidence.json` joined on a check ID, where each
of 58 checks carries `{dimension, name, scope, fix_type, evidence_sources, evidence_text}` over a source registry that grades its own citations
`primary-data` / `peer-reviewed` / `case-study` / `industry-practice` — and
annotates the weak one honestly ("n=1 case study, useful reference point not
universal benchmark").

`gnosis/standards/` carries the same three files for every threshold in the tool,
plus `operators.json` — the comparison-operator patterns used to derive
constraints from prose (§10.2.2), as data with their own test corpus rather than
regexes in Go. Operator inversions are the first cases in that corpus, because
*"no fewer than three"* and *"should not exceed three"* turn on a word naive
patterns miss, and a wrong operator produces a false conflict.

The three shared files carry:
candidate rank cuts, hop limits, `MinPassageWords`, staleness defaults, the
promote gate's signals. Two consequences:

- A threshold change is a reviewable diff carrying its own justification.
- The standards files are hash-pinned into every finding and every audit row, so
  a verdict is inseparable from the configuration that produced it.

Their thresholds file also states this repo's charter better than the manifesto
does, and its header is adopted verbatim as the first key:

> Reference values from empirical data. NOT enforced thresholds — measures and
> compares, users decide.

______________________________________________________________________

## 7. Library Offload

Per `skillet/TODO.md`'s *Preserve Mature Libraries (Hard Constraint)*: where the
family already offloads a job, keep that library; resolve only genuine
conflicts. `gnosis` adds no new library for a job skillet already answers.

### 7.1 Packages from `skillet`

Requires **skillet v0.17.0 or later** (the scaffold already pins v0.17.0).

| Package            | Used for                                                                              |
| ------------------ | ------------------------------------------------------------------------------------- |
| `frontmatter`      | `Split` — separating YAML from body. The only splitter.                               |
| `markdown`         | `Parse` → `Doc` (goldmark) for sections, links, `HasCodeBlock`. The only body parser. |
| `textnorm`         | `Fold` — the only text normalization on any comparison path                           |
| `finding`          | `Diagnostic` / `Result` / `Severity` — the only finding shape                         |
| `identity`         | `Hash` — content hashes, byte-identical with every family tool                        |
| `proof`            | `Packet` over archived evidence and admitted concepts (`no-proof-no-close`)           |
| `provenance`       | vendored-header stamp for any file `gnosis` copies in                                 |
| `judge`            | `Check` / `Score` — deterministic operators for the promote gate                      |
| `ratchet`          | `Evaluate` / `SelectScore` — corpus-health regression gating                          |
| `timeseries`       | `Detect` — "is the corpus worse than it was", with `Compared` distinct from zero      |
| `stats`            | `Wilson` / `McNemar` — intervals on any proportion `gnosis` reports                   |
| `calibration`      | `Compute` — do stated confidences match observed outcomes                             |
| `auditlog`         | `Append` / `Read` — the audit trail                                                   |
| `atomicfile`       | `WriteFile` / `Rename` — every corpus write                                           |
| `fsutil`           | `SubdirsContaining` — bundle discovery                                                |
| `errs` / `toerr`   | classification and wrapping, per the family split                                     |
| `naming`           | `Title`, `TitleFromMarkdown` — slug and title derivation                              |
| `neutrality`       | `Scan` — runtime-binding wording in the schema document                               |
| `ruleset`          | claim carrier where corpus knowledge is normative                                     |
| `ruleset/conflict` | contradiction predicates (§10) — **to be built**                                      |
| `quotecheck`       | fabrication guard — **to be promoted from exegesis**                                  |

### 7.2 Third-Party, Matched to the Family's Existing Choices

| Concern         | Library                               | Why this one                                                                   |
| --------------- | ------------------------------------- | ------------------------------------------------------------------------------ |
| YAML            | `github.com/goccy/go-yaml`            | exegesis and skillsaw agree; skillet pins `v1.19.2`                            |
| Markdown AST    | `github.com/yuin/goldmark` (+GFM)     | via `skillet/markdown`; never parsed twice                                     |
| SQLite          | `modernc.org/sqlite`                  | pure Go, no CGo, single binary; FTS5 present                                   |
| Git             | `github.com/go-git/go-git/v6`         | skillet's declared choice for provenance                                       |
| CLI             | `github.com/peterbourgon/ff/v4`       | the scaffold; `climax` conventions                                             |
| TOML config     | `github.com/BurntSushi/toml`          | adh's choice, per the constraint table                                         |
| Secret scanning | `betterleaks`                         | **see below**                                                                  |
| HTML → markdown | `JohannesKaufmann/html-to-markdown`   | pure Go, deterministic; the one pinned extractor                               |
| Units ↔ prose   | `github.com/dustin/go-humanize`       | bidirectional `ParseBytes`/`ParseSI`; keeps stored constraints legible (§10.2) |
| Spelled numbers | `github.com/rodaine/numwords`         | `ParseString` normalizes "three and a half" to 2.5-style digits; see §7.3      |
| HTTP fetch      | stdlib `net/http`                     | no framework                                                                   |
| Web server      | stdlib `net/http` + `html/template`   | leafwiki's posture: one binary, no Node                                        |
| Terminal UI     | `charmbracelet/glamour` for rendering | llmwiki's choice; read-only viewing                                            |

On secrets: `pantry` advertises three-layer redaction, but hardcodes ten regexes
including `password\s*[:=]\s*["']?.+`, greedy to end of line. `adh/internal/redact`
is the better design and the one to copy — it offloads detection to `betterleaks`'
maintained ruleset and is a thin wrapper. `gnosis` does the same, and takes from
`pantry` only its first layer: honour explicit `<redacted>…</redacted>` markers so
an author can declare a secret no pattern would catch.

### 7.3 Spelled-Out Numbers

The prose-versus-constraint check in §10.2 has to cope with a claim that says
"three" where the constraint says `3`. Two Go packages solve this; `numwords` is
the one pinned, and the reason is narrow and decisive.

|                      | `rodaine/numwords` (chosen)                                                                 | `will-lol/numberconverter`                                                     |
| -------------------- | ------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| Direction            | English → number, plus in-place `ParseString`                                               | both, `Etoi` and `Itoe`                                                        |
| Numeric range        | ints, **floats, and fractions** — "two and a half" → 2.5, "eight and three quarters" → 8.75 | **`int64` only**                                                               |
| Positions            | no                                                                                          | yes — `FindAllEnglishNumberIndex`                                              |
| Runtime dependencies | **none** (testify is test-only)                                                             | five indirect, including a pretty-printer                                      |
| Stability            | stable API; maintained                                                                      | maintained; README still states "pre-release… some methods may not be correct" |
| License              | MIT                                                                                         | MIT                                                                            |

**Decimals and fractions decide it.** The quantities this corpus will carry are
timeouts, percentages, and ratios — "2.5 seconds", "99.9% availability", "half
the budget". An `int64`-only converter fails on the most common case, and no
amount of API polish compensates.

Both packages are maintained. The secondary consideration is that
`numberconverter`'s README still carries the author's own caution — "pre-release…
some methods may not be correct" — and these checks sit under a review gate,
where a stated caveat about correctness is a reason to wait for it to be
withdrawn rather than to design around it.

`numberconverter`'s advantages are real and worth revisiting when it grows float
support: `Etoi`/`Itoe` cover both directions, it returns positions rather than
only values, and it ships fuzz tests for both directions.

One correction to an earlier reading of this: those positions were described here
as pairing naturally with `evidence.pos`. They do not. A number found in a claim's
prose is located in the **document body**, and `evidence.pos` is an offset into the
**archived source** (§5.5.2) — two coordinate spaces. Positions from a prose scan
belong beside `claims.pos`, and the prose-versus-value agreement check compares a
number in the body against a value in `claim_subjects`, never against an archive
offset.

One caveat on the chosen package, mattering more than it looks:
**`IncludeSecond` is package-level mutable state.** A global toggle means two
callers can disagree about whether "second" is a number, which is precisely the
class of defect `skillet` exists to prevent. Set it once at initialization, never
at call time — and set it **off**, because "the second retry" turning into "the
2nd retry" is a false-match risk in exactly the text being checked.

Dependency footprint is the remaining difference: `numwords` has no runtime
dependencies (testify is test-only), where `numberconverter` pulls five indirect,
one of them a pretty-printer. For a package sitting behind a deterministic check
the smaller surface is worth something.

### 7.4 Deliberately Not Adopted

- **Embedding providers as a dependency.** Optional and pluggable (§11.4); FTS5
  works with no configuration, as in `pantry`. Graceful degradation to zero
  dependencies is a requirement, not a nicety.
- **`qmd`** — Karpathy's suggestion, and a good tool, but shelling out to a
  Node-installed binary for the primary search path breaks the single-binary
  posture. Supported as an *optional* external reranker behind the same interface
  as embeddings.
- **A vector store.** `sqlite-vec` requires CGo. If semantic search is enabled,
  vectors live in a plain table and brute-force cosine is acceptable at this
  corpus scale; revisit on a measurement, not on speculation.
- **`llmwiki`'s storage and validator.** The validator is weaker than
  `quotecheck` (byte-exact against a live source, so a curly apostrophe or a
  rewrapped line fails it). What is worth taking is its ingest adapters, its
  crash-resumable queue, and its cross-page reconciliation *shape*.

______________________________________________________________________

## 8. Command Surface

`climax` conventions throughout: one package per command under `cmd/<name>/`, a
`Config` embedding `*root.Config`, `ff.NewFlagSet(name).SetParent(parent.Flags)`,
registration in `cmd/cmd.go` at the `// climax:imports` marker, `GNOSIS_`
env-var prefix, `root.ExitError` for specific exit codes.

Every command MUST support `--jsonl`, keep structured data on stdout and
diagnostics on stderr, and exit with meaningful codes — `qvr`'s agent-callable
contract. An agent is a first-class caller.

### 8.0 Machine Output Envelope

Under `--jsonl` a command writes one JSON record per line to stdout, and nothing
else. The envelope is shared with `adh`, so a caller that already parses one tool
parses both:

```json
{
  "status": "findings",
  "code": 3,
  "reason": "duplicate_identity",
  "message": "the corpus has blocking findings",
  "data": {}
}
```

`status` and `reason` are both closed vocabularies, and the reason for having
both is worth stating: `status` says what kind of outcome this is, `reason` says
which one, and an agent branches on a token rather than matching prose.

| `status`   | `code` | Means                                                                  |
| ---------- | ------ | ---------------------------------------------------------------------- |
| `ok`       | 0      | completed; nothing blocking to report                                  |
| `error`    | 1      | the tool itself failed                                                 |
| `error`    | 2      | with `reason: usage` — the invocation was wrong; nothing was attempted |
| `findings` | 3      | completed, **and** reported a blocking finding                         |
| `blocked`  | 4      | could not complete because a person must act (§10.6, §17)              |

`status` has four values, not five: a bad invocation is a tool-level failure, and
`reason` already says which kind. The exit code separates them because the repair
differs — code 2 means "call me differently", code 1 means "something is wrong
that changing the arguments will not fix" — and an agent that cannot tell those
apart retries the same wrong call.

`findings` is the distinction the vocabulary exists for. A corpus with problems
is not a broken tool, and a CI job needs to tell those apart: `code 3` means
gnosis worked and the corpus did not, `code 1` means gnosis did not work and the
corpus was never judged. Conflating them is how a green build comes to mean
nothing.

`code` is carried inside the record as well as returned as the process exit
status. That is deliberate redundancy: a caller reading a captured `.jsonl` log
after the fact no longer has the exit status, and a record that cannot be
interpreted without out-of-band context is not self-describing.

`reason` is a closed set of snake_case tokens — `duplicate_identity`,
`identity_conflict`, `index_drift`, `unparsable`, `vocabulary_invalid`,
`no_bundle`, `needs_human`, `usage`, `challenge_unanswered`. Adding a token is a compatible change;
changing what one means is not. `message` is for a person and MUST NOT be
parsed. `data` carries the command-specific payload and is absent when there is
none.

### 8.1 Corpus Lifecycle

```text
gnosis init [--bundle DIR]           scaffold an OKF bundle, .gnosis/, AGENTS.md, hooks
gnosis doctor                        validate bundle, index, standards, hooks, tier-0 integrity
gnosis index rebuild [--check]       regenerate index.db; --check exits 1 on any divergence
gnosis schema [link|--check]         maintain AGENTS.md and agent-file symlinks
```

### 8.2 Ingestion

```text
gnosis fetch <uri>...                archive bytes into tier 0; append (uri, sha256)
gnosis ingest <uri|path>...          fetch → scan → emit extraction prompt(s)
gnosis ingest --resume               drain the crash-resumable queue
gnosis admit <reply-file>            gate a reply; write to quarantine on pass
gnosis promote <slug>...             quarantine → bundle, behind the promote gate
```

`ingest` is a two-phase relay, not a single call: it emits prompts and suspends,
an agent supplies the reasoning, `admit` consumes the reply. `--relay` chains
emit/resume to cut round-trips, as `adh run --relay` does.

### 8.3 Query and Search

```text
gnosis search <query>                FTS5 (+ optional semantic), ranked, --jsonl
gnosis ask <question>                emit an answer prompt with retrieved context
gnosis file <answer-file>            file a good answer back as a concept (promote gate)
gnosis show <path>                   render a document, resolved links, evidence, trust tier
gnosis graph [--orphans] [--dot]     link graph; orphan and hub reporting
gnosis serve [--addr] [--auth]       the web interface (§13)
```

`gnosis file` is Karpathy's key insight — good answers are valuable and should
not disappear into chat history — subject to the same admission gate as an
ingested source, because a synthesized answer is exactly as capable of being
wrong.

**`show` and `search` MUST render resolved outbound links inline**, with each
target's title and identifier, not merely the hrefs. This is a requirement rather
than an ergonomic preference. *As We May Think* §6 indicts conventional indexing
in two parts, and the second is usually forgotten: not only that an item "can be
in only one place", but that "having found one item, moreover, one has to emerge
from the system and re-enter on a new path." A result that forces a fresh query to
follow a link reproduces the exact defect the memex was proposed to fix, and the
cost of avoiding it is one join against `links` and `documents`.

### 8.4 Curation and Maintenance

```text
gnosis lint [--check <name>] [--check-value]  the deterministic health pass (§12)
gnosis conflict [--path P]           contradiction detection (§10)
gnosis adjudicate <finding-id>       record a human decision; write the warrant
gnosis challenge <path> --class C    a reader contests an accepted claim (§10.7)
gnosis challenge --list [--unanswered]  open challenges, oldest first
gnosis stale [--refresh]             freshness: compare archived text to upstream
gnosis supersede <old> <new>         status: deprecated + gnosis_supersedes edge
gnosis log [--since]                 read/append log.md (OKF §9)
gnosis critic [--path P] [--sample N]  cold-critic prompt over a claim, or a sample
gnosis gate <findings-file>          block on error-severity findings; runs its self-test
gnosis miss report                   aggregate the miss log (§6.4)
gnosis audit [--since] [--who] [--reversed]   read the audit trail (§10.6.5)
```

### 8.5 Family Interop

```text
gnosis export --format okf|jsonl     bundle export
gnosis manifest [--check]            emit skillet/manifest over the corpus
gnosis proof create --out P          proof packet binding corpus + tier-0 digests
```

______________________________________________________________________

## 9. Ingestion

### 9.1 Pipeline

```text
fetch → archive → scan → extract → validate → quarantine → gate → promote
 (det)   (det)     (det)  (model)   (det)      (det)        (det)  (human)
```

Only one stage calls for a model, and it is the only stage that cannot be
decided mechanically: turning prose into claims.

### 9.2 Fetch and Archive

Source adapters. Phase 2 ships four — **local file, directory, URL, git
repository** — and each adapter's only job is to produce bytes plus a URI and a
media type. Nothing is archived until the disposition in §4.3 is decided.

RSS/Atom and sitemap adapters are `llmwiki`'s breadth and are **not** in scope
here; they are enumerated as a later item rather than described as shipped,
because a specification that lists unbuilt adapters teaches a reader to discount
the rest of it.

```text
fetch bytes ──▶ hash (always recorded)
                  │
                  ├─ text-like, allowlisted, under cap ──────────▶ archived
                  ├─ else: extractor produces text under cap ───▶ extracted
                  └─ else ─────────────────────────────────────▶ referenced
```

Rules:

- **The hash is always recorded, even for `referenced`.** That is what makes a
  later upstream change detectable regardless of disposition.
- **Extraction is deterministic or it is not used.** An extractor MUST be pinned
  by name and version in `standards/archive.toml`, and its output MUST be
  reproducible for a given input; a non-deterministic extractor would make the
  archive un-rebuildable, breaking §18.3. Extraction runs locally; a
  model-driven extraction is not an extractor, it is an ingest prompt.
  **One extractor is pinned: HTML → markdown, and it MUST strip boilerplate.**
  This is not a polish concern. The two web pages archived in this repository as
  reference material demonstrate the failure directly: the Luhmann translation
  ends in raw JavaScript (`var vanilla_forum_url = …`), and the Bush essay carries
  navigation chrome, an author bio, a newsletter signup, and a comment widget. Text
  archived with that intact means a quote can validate against a cookie notice —
  the corpus's central invariant (§9.4) satisfied by furniture. The stripper is
  part of the pinned extractor identity, so `sources_fetched.extractor` and
  `extractor_version` record which one produced a given tier-0 file and a
  re-extraction with a different stripper is visible rather than silent.
  PDF, image, audio, and video
  sources take the `referenced` path by design (§4.3) — no extractor, no
  external binary, no offline proof, and the weakness recorded rather than
  papered over.
- **Archiving is append-only and content-addressed.** Re-fetching an unchanged
  source is a no-op that records a `fetch_history` row. Re-fetching a *changed*
  source appends a new archive file and marks every claim citing the old one
  `stale`. Nothing is ever rewritten in place.
- **Sanitization refuses, never repairs** (§4.4). A rejected file is recorded with
  its `reject_reason` and the source falls through to `referenced`, so a refusal
  is visible rather than looking like an absent source.
- **The corpus budget is reported, not enforced silently.** When the archive
  crosses its warning threshold, `gnosis doctor` and `ingest` say so, and name
  the largest files. Growth that nobody was told about is the failure mode.

The ingest queue is SQLite-backed and crash-resumable, as `llmwiki`'s is; a
killed process resumes rather than restarting.

### 9.3 Scan — Admission Security

Ingested text is text an agent will obey. A poisoned upstream page filed into
the corpus is a durable prompt injection carrying the team's own authority. This
stage is not optional and runs before any model sees the content.

Ordered, all deterministic:

1. **Hidden characters** — `qvr/internal/security/unicode.go` is the liftable
   reference: zero-width (`0x200B`, `0x200C`, `0x200D`, `0x2060`, `0xFEFF`), bidi
   overrides (`0x202A–0x202E`, `0x2066–0x2069`, the Trojan Source class), and
   Unicode tag characters (`0xE0001–0xE007F`). **Its constants are codepoint
   ranges from the Unicode standard, not tuned thresholds**, which is what makes
   it eligible for a blocking gate. Mixed-script confusables are a warning, not
   an error, because multilingual prose is legitimate.
2. **Prompt-injection and exfiltration patterns** — `qvr`'s taxonomy names the
   categories worth carrying here: `prompt_injection`, `data_exfiltration`,
   `memory_poisoning` (named for exactly this scenario), `tool_misuse`.
3. **Secrets** — `betterleaks` ruleset plus explicit `<redacted>` markers.
   Redaction happens **before anything reaches disk**, which is `pantry`'s one
   clearly-right invariant.
4. **Oversize / binary** — bounded, with the bound in `standards/`.

Findings gate the ingest, land in the audit trail, and export as SARIF for any
code-scanning pipeline that wants them.

### 9.4 Segment, Then Validate

The extraction prompt asks for OKF concepts, each claim carrying a verbatim quote
and the `source_id` it came from. Before any quote is checked, the reply is
**segmented into claims**, and this ordering is not incidental.

A sentence is the wrong unit. *"The cache is enabled by default, but it is not
shared across sessions"* is one sentence carrying two assertions, and a verifier
that attaches one verdict to it will report the whole sentence supported when a
quote validates only the first half. That is a silent false pass in the one check
the corpus most depends on.

Segmentation is deterministic — no model, no network — and carries one rule:

> **Every emitted claim stands on its own, or the cut is not made.**

Splitting the example at its comma would leave *it is not shared across
sessions*, whose subject sits in the discarded half; verified alone it is
meaningless, and verified against a source it is a coin flip. So the subject is
recovered and substituted, or no cut happens and the sentence stays one claim
whose evidence must cover all of it.

Naive splitting is not an option and the failure cases are known: `split(".")`
cuts `2.5 seconds` in half, and an abbreviation list still cuts `e.g.`,
`README.md`, `foo.bar()`, `https://example.com/a.html`, and `A. Turing`.
Splitting on newlines additionally cuts every hard-wrapped paragraph.

Each emitted claim becomes one `claims` row carrying the fold hash of its
anchoring text (§5.5.1), which is why a document holds many claims rather than
one. The byte offset is cached alongside it and is not the address.

With the claim grain established, validation is deterministic, and this is the
corpus's central invariant:

> Every `gnosis_evidence` quote MUST appear in the named archived file under
> `textnorm.Fold` normalization, with the passage at or above
> `MinPassageWords`.

**The invariant is stated per claim, and that is only sound because claims are
segmented first.** Bound to a sentence it would be unsound: *"The cache is enabled
by default, but it is not shared across sessions"* is one sentence and two
assertions, so a single quote could validate while half of what it appears to
support went unsourced — a silent false pass in the check the corpus most depends
on. The stand-alone rule above is what closes that: a cut is made only when each
side survives alone, so every claim the invariant binds is a claim a quote can
wholly support or wholly fail to.

`quotecheck` is the implementation. Its normalization choice is the reason it
works on a real corpus: folding whitespace runs *and* typographic characters
before comparing, because a book, a plain-text extraction, and a markdown file
each spell curly quotes and dashes their own way — as that package puts it, a
guard that fired on every curly apostrophe would not get run. Case is preserved.

A document whose claims' quotes fail validation is **dropped, and the prior version
stands**. Never partially written, never silently downgraded. The consequence is
the property worth having: a cheaper model yields a *sparser* corpus, never a
*wronger* one. Degradation is in coverage, not correctness.

### 9.5 Promote Gate

Quarantine → bundle. Deterministic signals only, each declared in `standards/`,
following `llmwiki`'s four-signal promote heuristic and extending it:

| Signal      | Check                                                                 |
| ----------- | --------------------------------------------------------------------- |
| evidence    | ≥1 validating quote per enforced claim                                |
| provenance  | every `sources[].resource` resolvable or a declared scope descriptor  |
| conformance | OKF §11 satisfied; `type` non-empty                                   |
| duplication | no near-identical concept by `Fold`-normalized title and evidence set |
| hedging     | body free of `skilllens.SofteningPhrases` beyond a declared count     |
| conflict    | no open error-severity finding from §10                               |
| security    | scan clean, or every finding accepted with a recorded warrant         |

The gate runs a **planted-defect self-test on every invocation** and refuses to
gate if the control fails — `canonizer/internal/gate.SelfTest`, which mirrors
`adh`'s `oracle selftest`, which comes from
`evals-differential-oracle`'s `impl_buggy.py`. A gate nobody has proven can fail
is not a gate.

Promotion of a concept whose scan produced findings, or which contradicts an
accepted concept, requires a human — `clu`'s approval checkpoints in the workflow
graph, with the phrase-confirmation discipline `adh` uses for irreversible
actions. No `--yes`, no environment variable, no self-granted approval.

### 9.6 Corrections Accrete Without Cooperation

The manifesto's requirement that every corrective interaction be accretive cannot
depend on a model choosing to record it. Two mechanisms, neither requiring
cooperation:

- **Hook-driven capture.** `Yul-lu` fires `record_messages` from a Stop hook so
  capture is deterministic; its own README notes that Codex, lacking hooks,
  degrades to hoping the model complies. `gnosis` ships a Stop-hook companion
  that pipes session JSON in and files wiki-touching turns as candidate answers,
  subject to §8.3's `file` gate.
- **Deterministic mining.** `SkillOpt/skillopt_sleep/mine.py`'s `heuristic_mine`
  is the reference: it detects retry chains — a prompt re-asked after negative
  feedback implies the earlier attempt failed — extracts recurring intents, and
  labels outcomes from feedback signals, with **no API and fully offline**. An
  optional `llm_mine` enriches and falls back to heuristic on error. `gnosis mine` implements the heuristic tier; the LLM tier is a prompt like every other.

Session-store adapters follow `skillopt_sleep`'s shape — one normalizing seam so
a foreign format change is one file — since reading another tool's on-disk format
will rot.

______________________________________________________________________

## 10. Curation and Contradiction

Every tool surveyed detects *similarity*. None adjudicate *conflict*. This
section is the part `gnosis` must build, and it is the reason the corpus can be
called authoritative.

### 10.1 Shared Half Lives in `skillet`

`ruleset/conflict`, recorded in `skillet/TODO.md`. `canonizer` is consumer one,
`gnosis` consumer two. `merge-skills` detects convergence across independently
derived artifacts; **contradiction detection is its dual — the same comparison,
kept when it comes back negative.**

### 10.2 What Is Exactly Decidable

Only these, and each is exact with no tunable constant:

| Predicate            | Fires when                                            |
| -------------------- | ----------------------------------------------------- |
| severity divergence  | `Fold`-equal claim text, differing `ruleset.Severity` |
| level divergence     | `Fold`-equal claim text, differing `ruleset.Level`    |
| interval conflict    | same named quantity, disjoint admissible intervals    |
| enumeration conflict | same slot, disjoint required value sets               |
| identity collision   | two concepts claiming one `§`/slug after a merge      |
| evidence divergence  | `Fold`-equal claim, archived texts that disagree      |

Interval and enumeration conflicts need two things prose does not directly
supply: a canonical **subject**, so two claims can be known to be about the same
thing, and a typed **value**, so their assertions can be compared. The two arrive
by different routes, and keeping them separate is what makes the second one
cheap.

- **Subjects are declared.** `gnosis_subject` names a key from `ontology.toml`
  (§5.8). A subject key alone buys **candidate narrowing** — two claims sharing a
  key are a candidate pair, feeding the deterministic selection in §6.2. No
  verdict, no value, no dimension needed, and a wrong key costs a wasted
  comparison rather than a wrong answer.
- **Values are derived.** A constraint is a **cached parse of the prose, not a
  second assertion** — operator patterns from `standards/` plus the units
  libraries (§7.2, §7.3), written into `claim_subjects` and nowhere else.

#### 10.2.1 Prose Is Authoritative; the Constraint Indexes It

This is the load-bearing decision in this section. A constraint never redefines
what a claim says. It is a derived reading of the claim, regenerable from it, and
three properties follow:

- **No authoring burden.** Nothing has to be written by hand or proposed by a
  model. Coverage grows as the parser improves.
- **No migration.** Because the value lives only in the index, adding or
  improving parsing is a reindex, not a rewrite of the corpus.
- **Drift is impossible by construction.** A second *representation* can disagree
  with the first; a cache cannot, because regenerating it is the repair. This is
  the same reasoning §4.5 applies to the index as a whole.

`gnosis_constraint` remains available in frontmatter, and **is never required**.
Its purpose is narrow and worth stating positively: the case where a precise
value exists but not in parseable prose — a number in a table, a code fence, or a
figure caption the body parser cannot reach but a human can see. There a pin
carries information the parser genuinely cannot derive. That is something a
person notices while reviewing, not something a type-level flag can predict,
which is why it stays opt-in.

**Requiring a pin would invert this section.** A required pin means a model must
produce a value for the claim to be admitted, and a pinned value outranks the
prose for comparison — so the requirement would hand a model-generated number
authority over the human-readable text, which is exactly what making the prose
authoritative was for.

When a pin is present it takes precedence over the derived value, and *then* the
two representations can disagree — so a pinned constraint is checked against the
prose and reported when it drifts. Because the units libraries are bidirectional, that check is
mechanical: normalize spelled-out numbers in the claim text (§7.3), render the
constraint through the same library, and look for it under `textnorm.Fold`. It
will not catch a paraphrase. It is a warning, never a gate.

#### 10.2.1.1 Constraints Too Wide to Fail Are Not Constraints

A constraint states a **sharp** expectation or it states nothing. *The retry budget
is 3* can be contradicted; *the retry budget is between 1 and 100* cannot, and it
would pass every check specified here — it parses, it indexes, it carries a
dimension, it conflicts with nothing ever. An expectation that admits every
observation has not been narrowed by the evidence and cannot be narrowed by
anything.

`constraint-coverage` therefore reports a pinned constraint whose bound spans the
plausible range of its dimension, as a warning. The remedy is usually that the
author knows something sharper and hedged, which is worth asking about; the second
possibility is that the subject genuinely is not constrained here, and the honest
form of that is no constraint rather than a vacuous one.

#### 10.2.2 Derived Values Generate Findings, Never Verdicts

A pattern-derived operator is a heuristic, and this specification refuses
heuristics underneath blocking gates. It does not refuse them for **candidate
generation**, which is what this is: the interval and enumeration predicates run
where both sides parsed, and they emit a conflict *finding* that a person
adjudicates (§10.6). Nothing blocks on a parse.

That narrows the win, honestly stated: derived constraints buy **reliable
detection** of numeric conflicts, not reproducible verdicts. Detection is the half
that matters, because the failure mode with teeth is a numeric contradiction going
*undetected* — not one being adjudicated by a person.

Two mitigations, both required, because a mis-parse produces a **false conflict**
and noise in the queue is what gets a check switched off:

- **The operator pattern set lives in `standards/` with its own test corpus.**
  `"no fewer than three retries"` and `"retries should not exceed three"` invert
  the operator on a word naive patterns miss, so the patterns are data with tests
  rather than regexes in Go. Inversions are the first cases in the corpus.
- **A finding derived from a parse says so, and shows the parse.** The
  adjudicator sees `no more than 3` → `{op: "<=", value: 3, dimension: count}`
  beside the claim text. A false conflict that shows its reasoning is dismissible
  in seconds; one that shows only a verdict erodes trust in the whole queue.

#### 10.2.3 Coverage Is Reported Continuously

For each subject key, `gnosis lint` reports how many claims parsed to a value and
how many conflict candidates fell through for lack of one.

**This is a parser-improvement signal, and its consumer is
`standards/operators.toml`.** Poor coverage on a key has exactly two causes, and
neither is answered by asking authors for more:

- **The claims carry no quantity** — "a few retries", "as many as needed". There
  is nothing to pin, and demanding one would manufacture precision the source
  never had.
- **A quantity is present in a phrasing the patterns miss.** That is a gap in the
  operator set, and closing it improves every affected claim retroactively on the
  next reindex, rather than one claim at a time.

So a lopsided key is a backlog item for the patterns and their test corpus, not
evidence that the corpus needs a stricter authoring rule.

**Units are an offload, not a build.** The obvious objection to structured
claims is that comparing `≤ 5s` with `≥ 5000ms` needs dimensional handling, and
that hand-rolling one is where the cost hides. It does not have to be built:

| Dimension                   | Parse ↔ format                                                | Canonical base    |
| --------------------------- | ------------------------------------------------------------- | ----------------- |
| bytes (SI and binary)       | `humanize.ParseBytes` / `Bytes`, `ParseBigBytes` / `IBytes`   | bytes             |
| SI-prefixed quantities      | `humanize.ParseSI` / `SI` — returns value **and** unit suffix | base SI unit      |
| durations                   | stdlib `time.ParseDuration` / `Duration.String`               | nanoseconds       |
| counts, ratios, percentages | stdlib `strconv`; `humanize.Comma`/`Ftoa` to render           | the number itself |

So the only thing `gnosis` owns is a **`dimension` field on each subject key**
declaring which parser applies and therefore what base unit to normalize to.
Comparison is then integer or float arithmetic on normalized values, and the
human-readable rendering round-trips back through the same library — which is
what keeps a stored constraint legible in a markdown body instead of becoming an
opaque canonical integer.

`github.com/dustin/go-humanize` is the pinned library for that boundary
(§7.2). Note what it is *not*: a dimensional algebra. It will not tell you that
seconds and bytes are incomparable, and it will not catch a subject key whose
declared dimension is wrong. Those stay the schema's job, which is why the
`dimension` declaration belongs with the subject key rather than being inferred
per claim.

### 10.3 What Is Refused, and Where the Line Falls

**No near-duplicate or semantic-similarity detector with a threshold.** "These
two claims are 0.87 similar, probably in conflict" requires a constant nobody has
calibrated, over an embedding, gating admission. That is the defect the family
rejected in `unified-thinking`'s bias detectors and it is refused here for the
same reason.

**The line runs between language and reasoning, and it is worth drawing
precisely** — because a catalogue of named thinking hindrances is tempting to
mechanize wholesale, and only part of it can be:

| Class                                                                                                            | Detectable?        | Where it goes          |
| ---------------------------------------------------------------------------------------------------------------- | ------------------ | ---------------------- |
| **Language** — hedging, weasel words, vagueness, meaningless comparisons, assuring expressions, gobbledygook     | **yes**, lexically | `language` check (§12) |
| **Reasoning** — post hoc, begging the question, false dilemma, appeal to authority, sunk cost, confirmation bias | **no**             | the critic, or nowhere |

A word list plus a test corpus finds "industry-leading", "significantly better"
with no comparison class, and "studies show" with no citation. Nothing lexical
finds a post hoc inference, and a check that claimed to would be a classifier
wearing a rule's name badge — the thing refused two paragraphs above. The
distinction is not about difficulty; it is that one class is a property of the
words and the other is a property of the inference, and only the first is present
in the artifact.

Semantic contradiction between claims that survive the decidable predicates is
**judge work**, routed through `gnosis critic` — a cold-critic prompt that sees
the two claims and their sources but never the reasoning that produced either.
Verdicts return as `finding.Diagnostic`.

**The critic is blinded, and this is a requirement.** The prompt MUST NOT include
the existing adjudication, warrant, status, trust tier, or verification history of
either claim. The reason is expectancy bias — a judge shown the conclusion a
corpus already reached tends to find support for it, and its agreement then
carries no information beyond the fact that it was told. That is the failure a
double-blind trial is designed against, and here it costs nothing but prompt
discipline to avoid.

The cost of getting this wrong is specific: an adjudicated claim would accumulate
critic agreements that merely echo the original decision, and §10.6.5's reversal
record would be the only surviving signal that the decision had ever been
questioned. Blinding is what makes a second look a second observation.

### 10.4 Adjudication Is a Distinct Artifact

When two claims conflict and a person decides, the decision is knowledge present
in neither source. **It can carry no quote, so it fails the evidence invariant by
construction** — the highest-value artifact the team produces, rejected by the
check that exists to protect quality.

The knowledge a team pools spans three kinds, and only one of them is what the
evidence invariant was designed for:

| Kind                                     | Example                                                                                                       | How it enters                    |
| ---------------------------------------- | ------------------------------------------------------------------------------------------------------------- | -------------------------------- |
| explicit and citable                     | the published research, a standard, a vendor's stated limit                                                   | **sourced** — a validating quote |
| explicit in the world, tacit in the team | *which* of that literature this team treats as authoritative, and how it is weighed against local constraints | **adjudicated, citing sources**  |
| genuinely tacit                          | hard-won judgment with no documentary referent                                                                | **adjudicated, no sources**      |

The middle kind is the largest for a group of experienced practitioners, and it
is neither folklore nor a sourced claim — it is adjudication *over* sources. So
the corpus carries two provenance classes, and **`sources` belongs to both**:

| Class           | Warrant                                         | Frontmatter                                                                                  |
| --------------- | ----------------------------------------------- | -------------------------------------------------------------------------------------------- |
| **sourced**     | a validating quote in archived text             | `gnosis_evidence`, `sources`                                                                 |
| **adjudicated** | the review, the people, the date, the rationale | `gnosis_warrant`, `verified: [{by: human:…}]`, and `sources` where the decision weighed them |

An adjudicated claim is *sourced differently*, not *unsourced*. A decision that
weighed two published positions names both, even though the decision appears in
neither. The evidence invariant is scoped to sourced claims; `lint` requires a
warrant on an adjudicated claim and says nothing about whether it also cites
sources. OKF supports this without extension at the trust layer: OKF §5.2
keeps `generated` and `verified` separate precisely because "who *wrote* a
concept need not be who *confirmed* it".

Supersession, never deletion: the losing claim gets `status: deprecated`
(OKF §5.4 — "kept for links and history; no longer current") and the winner gets
a `gnosis_supersedes` edge. The corpus can always answer what we believed in
March and why we changed.

### 10.5 Constraint Reporting

**`--sample N` critiques a random sample instead of everything**, and the reason
to have it is that the alternative to an affordable estimate is no estimate. A
corpus too large to critique exhaustively otherwise gets its quality judged by the
deterministic checks alone — which is to say by the cheap ones (§12).

The sample is seeded from `standards/` so the draw is reproducible (§18.3), and
the default is five. Five is not arbitrary: **the median of a population lies
between the smallest and largest of any five random samples with 93.75%
probability**, because the sample misses the median only if all five fall on one
side of it, which is five coin flips landing alike. That is enough to say something
real about a corpus at the cost of five prompts.

What this buys is the trade §17 is otherwise on the wrong side of: an exhaustive
deterministic pass is precise and systematically blind, while a small random
critic sample is imprecise and unbiased. Preferring the first because it produces
an exact number is choosing precision with unknown systemic error over imprecision
with quantifiable error, and the exact number is exact about the wrong thing.
Both are reported; neither is a score.

`super-hermes` closes a hole both cold critics have: a reply with no finding in an
area is indistinguishable from that area not having been examined, and the gate
ships on that silence. Its `prism-scan` appends a **constraint footer** — "this
analysis maximized X; it did not examine Y" — and `prism-reflect` persists a
constraint report that later runs read to steer away from exhausted angles.

`gnosis critic` replies carry an additive `examined` / `not_examined` block,
persisted to `.gnosis/coverage.jsonl` and fed into subsequent critic prompts.

**This does not compromise cold-context independence**, and the reason matters: a
coverage record says what was *looked at*, never what was *concluded* or how the
concept was produced. Feeding prior coverage to a fresh critic biases it toward
unexamined ground — the opposite of contamination. The block is **advisory
only**: a critic that declares a gap must never thereby block, or it will learn
to declare none.

______________________________________________________________________

### 10.6 Adjudication Authority Scales with the Adjudicators

Adjudication is the corpus's one path for content that cannot be checked. Every
other route is gated deterministically; an adjudicated claim carries no quote by
construction, so its only warrant is that a person said so. The question of who
may do that therefore has real weight — and it has a different right answer for a
corpus with one curator than for one with a dozen.

So the policy is **derived from the adjudicators the corpus actually has**, not
configured against them. This is the same discipline as trust tiers (§14.1),
credibility (§14.2), durability (§14.4), and check applicability (§12): a fold
over recorded facts rather than a flag someone must remember to change.

**This is Ashby's Law of Requisite Variety, and naming it constrains the design.**
A controller must have at least as much variety as the system it governs; a
corpus whose contradictions outrun its adjudicators has a *variety deficit*, and
scaling authority with population is variety matching rather than a convenience.
Three properties of the law bear directly on the tiers below:

- **A deficit is a design specification, not a failure.** A corpus with more
  open conflicts than its adjudicators can close is reporting a real fact about
  itself. §17's insistence that a finding is not a failure is the same claim.
- **There are three levers, and only two are legitimate here.** Variety closes by
  *amplifying* the governor — more distinct responses, faster — or *attenuating*
  the governed system: narrowing what the corpus admits, sequencing what it takes
  in, scoping subjects deliberately. The third instinct, adding process steps, is
  amplification by volume and does not increase capacity; a spec of this size
  drifts toward it by default, so it is named here as the thing not to do. A tier
  that adds a required reviewer without adding a *different* kind of reviewer has
  amplified nothing.
- **Attenuation is why §5.8 ships no subjects.** Declining to admit a vocabulary
  the corpus cannot yet adjudicate over is attenuation, not procrastination.

The population is the count of distinct `human:<id>` actors appearing in
`gnosis_warrant` and in `verified` within a declared window — data the corpus
already holds. Nothing new is maintained, and the count moves in both directions
on its own.

#### 10.6.1 Tiers

There are three, and none of them requires anything to be declared.

| Tier     | Derived when    | Ordinary conflict   | Load-bearing or normative conflict                                                           |
| -------- | --------------- | ------------------- | -------------------------------------------------------------------------------------------- |
| `sole`   | one adjudicator | that person decides | decides, and the claim records `sole_arbiter`; the escalation is logged, not blocked         |
| `paired` | two or three    | anyone              | a second signer is required, with a single-signer override permitted and its reason recorded |
| `quorum` | four or more    | anyone              | a second signer is required; no override                                                     |

"Load-bearing or normative" is already computable: centrality from the link graph
(§14.4.1) and `normative: true` from the type registry (§5.8). No new judgment is
needed, and `quorum` is the ceiling.

#### 10.6.2 Domain Expertise Is Surfaced, Never Enforced

At `quorum`, any second signer satisfies the requirement — which at scale means a
product manager could co-sign a transaction-isolation conflict. The obvious answer
is to require a co-signer with matching domain expertise. `gnosis` deliberately
does not do that, and the reasoning is worth recording because the obvious answer
has two failure modes and neither is obvious.

**A declared capability roster is a political artifact.** Someone must write down,
about colleagues, who is and is not qualified for a domain, then maintain it as
people change focus. It rots, and it hard-blocks whenever the sole holder is
unavailable — at which point the override that unblocks it has undone the reason
for requiring a specialist.

**A capability derived from behaviour is self-certifying.** "You may adjudicate
`db.*` because you have adjudicated `db.*`" makes whoever arrives first into the
domain's authority by fiat, which is backwards. It entrenches incumbents against
better-qualified newcomers, is gameable by adjudicating easy claims to acquire
standing over hard ones, and mistakes activity for competence.

So domain history is computed — it is a query over `gnosis_warrant` and
`gnosis_subject`, needing no roster — and **shown rather than enforced**. The
review queue displays, for the subject under adjudication, how many claims each
person has previously adjudicated under it:

```text
Conflict on retry.max_attempts
  you (human:priya)   0 prior adjudications under retry.*
  human:sarah        14 prior adjudications under retry.*
```

That grants no authority and cannot be gamed into any, because there is nothing
to acquire. It is the §13 principle applied to the co-signer rather than the
claim: if the queue shows enough, a non-expert recognises when to defer; if it
shows too little, even an expert guesses.

**The escalation path, if one domain ever genuinely needs a gate.** A subject key
in `ontology.toml` may carry `requires_capability: true`, which restricts
co-signing on that key alone to declared holders. This is deliberately
per-subject rather than global: most subjects need nothing, so the argument is
about three keys instead of everyone's competence, and the roster shrinks to
match. Enabling it requires **both** conditions, because either alone fails:

1. a wrong adjudication on that subject carries real external consequence —
   regulatory, security, data loss; **and**
2. at least two qualified holders exist.

Condition 1 without condition 2 is a bus factor wearing a gate. Condition 2
without condition 1 is ceremony.

**This may never be needed, and that is the expected outcome.** The two
interventions already specified are stronger than any routing rule. A required
`rationale` (§10.6.4) filters more bad adjudications than a permission check ever
will, because someone who cannot articulate a reason usually stops before
finishing the sentence. A sufficient queue (§13) changes what a person is capable
of deciding well. If those work, routing is unnecessary; if they fail, routing
will not rescue them, because a roster records who was *permitted* to be wrong
rather than whether the decision was sound.

#### 10.6.3 Four Properties the Tiers Must Have

- **A single-curator corpus is a supported configuration, not a degenerate one.**
  At `sole` there is no second signer to require, so escalation cannot mean
  blocking. It means recording: the claim is marked as decided by a sole arbiter,
  which is a fact a later reader — or a later, larger team — can act on.
- **Scaling down never invalidates what was already decided.** The tier governs
  admission at the time of the decision, not validity afterwards. An adjudication
  made under `quorum` stays exactly as valid when the team shrinks to one. The
  warrant records the tier in force when it was written, so provenance is not
  retroactively rewritten by a change in headcount.
- **A tier change is announced, never silent.** When the derived tier moves,
  `gnosis doctor` and `log.md` say so and say why. A gate that tightens or
  loosens without telling anyone is the "no silent caps" failure this
  specification refuses elsewhere.
- **Escalation must never deadlock.** At `paired`, if the second signer is
  unavailable, a single-signer override is permitted with a recorded reason and a
  flag on the claim. A queue that can block indefinitely stops being used, and an
  unused queue admits nothing — which is a worse outcome than an override that
  leaves a trail. This is the same escalate-rather-than-stall shape as
  `canonizer`'s rework budget, which returns `needs-human` rather than blocking
  forever.

#### 10.6.4 Warrant Is the Real Gate

`gnosis_warrant` requires `rationale`, non-empty, at every tier including `sole`.

This matters more than the authorization rule, and it is worth being explicit
about why. A permission bit asks whether someone is allowed to decide. A required
rationale asks them to write down *why*, in a commit, in front of colleagues —
and someone who cannot articulate a reason usually stops before finishing the
sentence. It costs one required field and will prevent more bad adjudications than
any roster.

At `sole` it still works: the reader you are writing for is yourself in six
months, and that reader has no other way to reconstruct the decision.

```yaml
gnosis_warrant:
  by: human:sarah
  at: 2026-08-19T14:02:11Z
  tier: paired                    # in force when decided
  review: https://…/pull/412      # where it was discussed
  rationale: |                    # required, non-empty
    Both sources are credible. Chose the vendor's published limit over the
    2024 blog post because the post predates the 3.0 rewrite.
  co_signed_by: human:marcus      # required at paired/quorum for escalated claims
  override:                       # present only when a required co-signer was waived
    reason: marcus on leave until 09-02; conflict blocking the incident writeup
```

The `override` block is the recorded escape hatch, and recording it is the whole
mechanism: a waived co-signature that leaves no trace is indistinguishable from a
tier that was never in force. `lint`'s `co-sign` check passes an escalated claim
with an override present and a reason non-empty, and `audit` can enumerate them.
A gate with no override is a gate people route around; a gate whose overrides are
countable is a gate.

#### 10.6.5 Reversals Are Recorded, Because They Are the Only Feedback

Everything above governs how a decision is *made*. Nothing so far records whether
it was any good, and without that the corpus accumulates decisions but never
learns anything from having made them.

So when an adjudicated claim is superseded, or a finding closed by adjudication is
reopened, the new warrant names the one it reversed:

```yaml
gnosis_warrant:
  by: human:marcus
  at: 2026-11-03T09:14:00Z
  reverses: 01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d   # the warrant, not the claim
  rationale: |
    The vendor limit we chose in August was per-connection, not per-client. The
    2024 post was describing the behaviour we actually see.
```

- **It is a link, not a judgement.** No score attaches to a reversed warrant and
  no reputation attaches to its author. Reversal is the ordinary consequence of
  deciding under incomplete information, and a corpus that made none would be one
  where nobody was deciding anything contestable.
- **`gnosis audit --reversed` is the only report it feeds.** A reader assembling
  an argument, or a team deciding how much weight to give an adjudication tier,
  can see which reasoning did not hold. This is deliberately a *retrieval*
  surface, not an analytic one: §17 forbids scoring, and inferring reliability
  from reversal counts would be scoring with extra steps.
- **The reason to spend a field on this** is that the alternative is a corpus
  where a decision reversed three times looks exactly like one never questioned.
  The reversal is the most informative fact available about a claim that cannot be
  checked, and it is free to record at the moment it happens and impossible to
  reconstruct later.

### 10.7 Challenge Is Adjudication's Front Door

Everything above starts the same way: a check notices something, and a person then
decides. That ordering has a gap in it, and the gap is the most capable informant
the corpus has — **a reader who already knows a claim is wrong.** Until now such a
person had no way in. They could open a pull request against a file, which routes a
knowledge dispute through a diff review, or say something in chat, which the corpus
never hears.

So a challenge is a first-class operation. Any reader may contest an accepted
claim, and doing so opens a finding.

```text
gnosis challenge /c/01932b7c-…-retry-budget.md --class coverage \
  --rationale "the quote is about connection retries; the claim is about request retries"
```

#### 10.7.1 Four Classes, Ordered by What Settles Them

The class is not a severity and not a topic. It states **what kind of thing would
resolve the dispute**, which is what decides whether a person needs to be involved
at all.

| Class           | The reader is asserting                                               | Settled by                      |
| --------------- | --------------------------------------------------------------------- | ------------------------------- |
| `replay`        | "I checked the archived source and the quote is not there"            | **re-running the check**        |
| `contradiction` | "this claim conflicts with that one, and nothing noticed"             | §10.2 predicates, then a person |
| `coverage`      | "the evidence does not support the scope the claim asserts" (§17.3.1) | a person                        |
| `scope`         | "the stated limitations are incomplete" (§17.2)                       | a person                        |

`replay` is the strongest and the cheapest, and it is worth being precise about
why: it is the only class gnosis can **adjudicate itself**. The challenger is
asserting something mechanically decidable, so the response is not a judgement but
a re-run — and if the challenger is right, the resulting finding is an ordinary
error-severity evidence failure that would have blocked anyway. The corpus does not
need to trust the challenger at all; it needs to check.

`contradiction` is the class that closes a known hole. §6.2.1 records that the
candidate selector is systematically blind to conflicts between claims sharing no
source, no link, and no vocabulary — and that a larger biased sample does not help.
A reader who noticed such a conflict has done, for free, the thing the selector
provably cannot. This is the human-powered complement to the seeded random pass,
and it is likely to be the higher-yield of the two.

#### 10.7.2 Challenges Are Themselves Testimony

Per §1.1, everything in this corpus is something somebody said, and a challenge is
no exception. It therefore carries the same obligation an adjudication does:
**`--rationale` is required and non-empty.** A reader who cannot say why a claim is
wrong has not filed a challenge, they have filed a doubt, and the corpus has no way
to act on a doubt.

This is deliberately the same discipline as §10.6.4's warrant, for the same reason:
the requirement costs one field and screens out most of what should never have been
filed, because someone who cannot articulate the objection usually stops before
finishing the sentence.

#### 10.7.3 Five Properties, Each of Which Is Load-Bearing

- **An unverified challenge does not block.** Only a `replay` challenge that gnosis
  has itself verified becomes error-severity, because at that point it is an
  evidence failure and not an assertion. Judgement-class challenges are warnings.
  The alternative — any challenge blocking on assertion alone — makes the front
  door a denial of service, and the first person to discover that changes what the
  command is for.
- **But silence is visible, and ages.** The complement of *challenges do not block*
  must be that they cannot be ignored quietly, or the class collapses into a
  suggestion box. An open challenge appears in `doctor`, `challenge --unanswered`
  lists them oldest first, and `lint`'s `unanswered-challenge` check reports any
  older than the window in `standards/`. This is §17.0's point about corrective
  permission: the corrective option existing is not the same as anyone being
  obliged to take it.
- **A rejected challenge is recorded, never deleted.** It closes with a warrant
  explaining why the claim stands. A claim challenged three times and upheld three
  times is a different artifact from one never questioned, and only one of them has
  evidence that anyone looked. This is the same reasoning as §10.6.5's reversal
  record, pointed the other way.
- **Being wrong costs the challenger nothing.** No count of rejected challenges
  attaches to an actor, and none feeds a tier or a credibility signal. If
  challenging carries risk, the people best placed to challenge — the ones with
  most at stake in the claim being right — are the ones who stop. That is the
  rating-system dynamic §12 already warns about, aimed at contribution: when the
  only visible signal is fault, the safe strategy is silence.
- **The claim's author cannot dispose of the challenge.** Resolution follows the
  §10.6 tier, and at `paired` or `quorum` the author is not the decider. At `sole`
  the arbiter and the author are the same person, which is a real limitation and is
  stated rather than papered over: at that tier a challenge is a note to yourself,
  and it is still worth having for exactly the reason §10.6.4 gives — the reader
  you are writing for is yourself in six months, and that reader has no other record
  that the question was raised.

#### 10.7.4 Challenges Are Committed, Not Cached

`findings` lives in `.gnosis/index.db`, which is derived, gitignored, and
**per-user** (§4.6). A challenge recorded only there would be invisible to
everyone else, would not survive `index rebuild`, and would fail §4.5's rule that
nothing exists only in SQLite. It would be a private note that looked like a
corpus artifact.

So a challenge is written to the challenged document's frontmatter, exactly as a
warrant is (§10.4) and a claim anchor is (§5.5.1):

```yaml
gnosis_challenges:
  - id: 01932c04-8b21-7f03-a5e1-3d92f7c04a1b
    class: coverage
    by: human:dana
    at: 2026-08-20T11:42:09Z
    rationale: |
      The quote is about connection retries; the claim is about request retries.
    state: open            # open | closed | deferred
```

Three properties follow, and they are the reason this is the right home rather
than a convenient one:

- **It travels.** A challenge filed by one user reaches every other user through
  the same `git pull` that carries the claim, with no service to run and no
  protocol to agree on.
- **It is reconstructible.** `index rebuild` recovers every challenge from the
  bundle, so the `findings` rows for challenges are a cache like everything else.
- **It is reviewable.** A challenge arrives as a diff on the document it contests,
  which is where a reviewer is already looking.

The same reasoning applies to any finding state that records **a human decision**
rather than a machine observation. A `deferred` state (§17.0) says *a person saw
this and is not acting yet*, which no rebuild can re-derive; a check finding says
*the corpus is currently shaped this way*, which every rebuild re-derives. The
first is committed, the second is cached. That line — **decisions are committed,
observations are cached** — is the general rule, and the challenge is its clearest
instance.

#### 10.7.5 What This Does Not Add

Challenge is a route into the existing finding lifecycle, not a parallel one. It
resolves through `adjudicate`, appears in `audit`, and obeys the same tiers.
Nothing here is a new adjudication mechanism, a new severity, or a new state — the
additions are one frontmatter family, two cached columns (`challenge_class`,
`opened_by`), and one check.

## 11. Search

### 11.1 Progressive Disclosure First

Karpathy's observation is that `index.md` alone works at ~100 sources and
hundreds of pages, and avoids embedding infrastructure entirely. OKF §8 makes
that a spec'd artifact. `gnosis` therefore treats the index as tier one of
retrieval, not a fallback: `gnosis search` consults `index.md`-equivalent
metadata (title, description, tags, type) before touching the body index.

### 11.2 Determinism Ladder

`virgil`'s router is the design: exact match → keyword index → category
narrowing → model fallback, with 80%+ never reaching a model, and every fallback
logged as a miss so the deterministic layers grow. `gnosis search` follows it:

1. exact path or title match;
2. `type` / `tag` / frontmatter-field filter;
3. FTS5 over title, description, body;
4. optional semantic rerank (§11.4);
5. model-assisted query reformulation — **prompt-emitting, logged as a miss.**

The measured claim `gnosis` must be able to make: what fraction of queries were
answered before step 5, over time. `gnosis miss report` is where that lives.

### 11.3 Scoping

`skillex` moves scope resolution out of prompt assembly and into deterministic
indexing plus structured query. `gnosis search` supports the same: `--type`,
`--tag`, `--under <path>`, `--status`, `--trust <tier>`, `--fresh`. A query
restricted to a subtree can never return a concept outside it.

### 11.4 Semantic Search Is Optional and Degradable

FTS5 works with no configuration and no keys. If an embedding provider is
configured, vectors are stored in SQLite and used to rerank FTS5 candidates —
never to replace them.

One determinism note worth recording: **an embedding is a deterministic function
of a pinned model**, unlike sampled generation. Pin model and version in
`standards/` and the reranked order reproduces exactly. This is why reranking is
acceptable in a deterministic pipeline and generation is not.

______________________________________________________________________

## 12. Lint and Maintenance

Karpathy's lint list, made mechanical wherever possible. Each check is named,
independently runnable via `--check <name>`, and emits `finding.Diagnostic`.

| Check                  | Deterministic? | What it reports                                                                                                |
| ---------------------- | -------------- | -------------------------------------------------------------------------------------------------------------- |
| `conformance`          | yes            | OKF §11 violations only                                                                                        |
| `evidence`             | yes            | a sourced claim whose quote no longer validates                                                                |
| `warrant`              | yes            | an adjudicated claim with no `gnosis_warrant`, or a warrant with an empty `rationale`                          |
| `co-sign`              | yes            | an escalated claim missing a required co-signer and carrying no recorded override (§10.6)                      |
| `stale`                | yes            | archived text ≠ upstream, or `today ≥ stale_after`                                                             |
| `orphan`               | yes            | no inbound links — **see the applicability note**                                                              |
| `newly-orphaned`       | yes            | had an inbound link at baseline, has none now                                                                  |
| `broken-link`          | yes            | unresolved link, reported **as a gap, never an error**                                                         |
| `duplicate`            | yes            | `Fold`-equal title or evidence set across documents; the post-merge step of §4.6.1                             |
| `conflict`             | partly         | §10 predicates; residue to `critic`                                                                            |
| `index-drift`          | yes            | `index.md` differs from what would be generated                                                                |
| `log-format`           | yes            | `log.md` violates OKF §9 date-heading form                                                                     |
| `command`              | yes            | a command named in the schema document that does not resolve                                                   |
| `gap`                  | no             | concepts mentioned but lacking a page — prompt-emitting                                                        |
| `durability`           | yes            | a **load-bearing** unprovable concept (§14.4.1); reports the peripheral count it suppressed                    |
| `archive-orphan`       | yes            | an `evidence/` file no claim cites — a candidate for pruning                                                   |
| `archive-budget`       | yes            | archive size over its warning threshold, largest files named                                                   |
| `identity`             | yes            | the six reconciliation cases of §5.1.2; duplicate identifiers first                                            |
| `filename-drift`       | yes            | slug no longer matches the title; corrected on the next write                                                  |
| `limitations`          | yes            | a normative concept carrying no `gnosis_limitations` (§17.2)                                                   |
| `claim-anchor`         | yes            | a `gnosis_claims` anchor that no longer appears in its document (§5.5.1); `pos` goes NULL, never auto-repaired |
| `unanswered-challenge` | yes            | a reader-filed challenge older than the window in `standards/` (§10.7)                                         |
| `lead`                 | yes            | a normative claim whose `lead` restates background rather than the conclusion (§17.4)                          |
| `coverage`             | yes            | a claim asserting more strongly than its evidence supports (§17.3.1); warning, never a gate                    |
| `language`             | yes            | hedging, weasel words, meaningless comparisons, assuring expressions — lexical only                            |
| `subject-missing`      | yes            | a claim of a type whose `expects_subject` is true, carrying none (§5.8.2)                                      |
| `subject-unknown`      | yes            | `gnosis_subject` names a key absent from `ontology.toml`                                                       |
| `constraint-drift`     | yes            | a **pinned** `gnosis_constraint` the prose no longer supports (§10.2.1)                                        |
| `constraint-coverage`  | yes            | per subject key: claims parsed and candidates lost; a backlog signal for the operator patterns                 |
| `ontology`             | yes            | types no concept uses, undeclared types, deprecated keys still in use                                          |

Three design notes:

**Regression-relative, not absolute.** `coherence` reports
`NewlyOrphanedEndpoints` *and* `NewlyCoveredEndpoints`, with `BaseAvailable`
distinguishing "no baseline" from "zero" — the same distinction
`skillet/timeseries` preserves as `Verdict.Compared`. `gnosis lint --since <ref>` gates on what this change made worse, which is the only gate that tolerates
a corpus that is imperfect but improving.

**Applicability is derived, not declared.** `coherence`'s `Convention bool` is
true only when the corpus demonstrably uses the pattern being checked, and it
skips promotion when false. `orphan` is meaningless in a corpus with no links
yet; `gnosis` derives that rather than asking for a flag.

**"Every instruction must be backed by a working command"** is Principle 1 of
`agentic-harness-bootstrap`, whose `verify-harness.sh` parses the module table out
of the document itself and checks each path exists. The `command` check applies
it to the schema document: a command named in `AGENTS.md` is a checkable claim.
Warning tier, since resolving an executable is environment-dependent in a way a
link is not.

`gnosis lint` MUST log what it skipped and why. A bounded check that reports
nothing reads as coverage.

**These checks are cheap, and that is a bias, not a virtue.** Nearly every check
above is deterministic, and each was chosen partly *because* it was mechanizable.
The checks that would most change what a reader believes are the three that are
not: `conflict` (partly), `gap` (no), and the adequacy of a claim's evidence to
the scope it claims (unlisted, because nothing deterministic decides it). This is
measurement inversion — the value of measuring something tends to run inverse to
how much measurement attention it gets, because people measure what they know how
to measure.

Naming it produces one rule rather than a redesign:

> **A finding count MUST NOT be presented as corpus health.** Not in the human
> summary line, not in the web interface, not in a badge.

A count of findings is a measurement of the *registry* — of how many cheap checks
ran and what they saw. It is not a measurement of whether the corpus is right, and
the two are easy to confuse precisely because the first is available and the
second is not. `doctor` and `lint` report counts because counts are useful; they
report them as counts.

**Why the skip report is mandatory rather than a courtesy.** The two mechanisms
above — derived applicability and regression-relative gating — together make it
possible for a corpus to lint clean because most of its checks did not apply. That
state is indistinguishable from a healthy corpus in any output that omits the
skips, and it is the more dangerous of the two failure modes available here.

The distinction is worth stating in full because governance literature has a
precise account of it. A model that is *wrong* eventually collides with reality
and gets corrected. A model that is *unwarrantedly confident* does not: it
"forecloses the adaptive response before the problem is visible enough to trigger
one." The 2008 financial system is the reference case — the regulators were not
choosing inadequate tools over adequate ones, they believed the tools were
adequate — and the corpus analogue is a green `lint` run on a bundle where
`conflict`, `constraint-drift`, and `subject-missing` all skipped for want of a
vocabulary.

So the skip list is part of the result, not a diagnostic detail: it goes in the
`--jsonl` payload, it is printed in the human form, and `doctor` reports the
count. A reader must be able to tell *examined and clean* from *not yet
examinable* without asking.

**`schema-shape` catches the one failure the version check cannot see.**
`PRAGMA user_version` records how far migration got, and each migration commits in
its own transaction — so a run interrupted between two of them leaves a database
reporting a version whose schema is not fully present. Comparing `sqlite_master`
against the tables, indexes, and virtual tables the migrations declare is a direct
check of what exists rather than of what was recorded, and it costs one query.
This is the same reason the corpus records identity twice (§5.1): a system asked to
report on itself will report what it believes.

**A severity vocabulary used unevenly carries almost no information.** If nearly
every finding is a warning, the severity field has stopped distinguishing anything
and a reader is right to ignore it — a channel transmits most when its symbols are
used comparably often. `--check-value` therefore reports the severity distribution
alongside the per-check counts. This is a fact about the registry, not a target:
the remedy for a lopsided distribution is examining whether the severities are
assigned correctly, never redistributing them to look balanced.

**A check that has never changed anything is costing attention.** `lint --check-value` reports, per check, findings raised against findings acted on —
closed by a fix, closed by adjudication, or deferred with a reason. A check with a
long raised column and an empty acted column is either measuring something nobody
cares about or reporting it in a form nobody can act on, and both are worth
knowing before adding the next check.

This does not contravene §17. Scoring the **corpus** is forbidden; scoring the
**instrument** is what a planted-defect self-test already does, and declining to
compute it is how a registry accumulates checks that produce noise. The report is
a table, never a composite, and no check is retired automatically.

**The retirement criterion is "nobody acts on it", never "it disagrees with the
others."** This restriction matters more than the report does, because the
tempting version of it manufactures its own result. IQ testing built its normal
distribution by keeping the questions that correlated with the others and dropping
the ones that did not, and then discovered that intelligence is normally
distributed — which it was made to be. A registry pruned for internal agreement
would likewise come to look coherent, and the first checks dropped would be the
ones surfacing something no other check can see. Dissent is the property a check
is *for*.

______________________________________________________________________

## 13. The Web Interface

Secondary to the CLI, and deliberately modest. `leafwiki`'s posture is the
target: a single Go binary, markdown on disk, no Node, no Redis, no Postgres —
because a knowledge base a team cannot operate is a knowledge base a team stops
using.

`gnosis serve` carries two roles in one process — the **write coordinator** of
§4.6 and the viewer below. They are the same binary and the same lifetime because
two servers would mean two authorities over one bundle. The coordinator is needed
from the first phase in which more than one tool instance writes; the viewer
arrives in Phase 5.

`gnosis serve` provides:

- **Read**: rendered concepts with resolved links, evidence with quote and source
  attribution, trust tier, freshness, and open conflicts shown inline.
- **Browse**: the OKF hierarchy, tag and type views synthesized at read time
  (OKF §3.1 deliberately specifies no tag file format), and the link graph
  including orphans and hubs.
- **Search**: the same ladder as the CLI, same scoping flags.
- **Review**: the queue non-engineers actually need — quarantined concepts,
  open conflict findings, adjudication, and promotion requests. This is where a
  QA engineer, PM, or support colleague participates without a terminal.
- **The queue MUST present enough to decide with, and this is the higher-leverage
  investment than any authorization rule.** For a conflict, that means both
  claims side by side, each one's sources with their OKF §5.1 credibility signals
  (`author`, `usage_count`, `last_modified`), the durability class of each
  (§14.4), the centrality class (§14.4.1), and the required `rationale` field.
  If the queue shows enough, a non-expert correctly recognizes when to defer; if
  it shows too little, even an expert guesses. Contributors are scarce, so each
  item must also be cheap to dismiss — batch actions and defaults, not a form per
  finding.
- **No prose editing.** The corpus body is model-written by design. The web UI
  writes warrants, adjudications, and approvals — never concept bodies.

Requirements:

- **Authenticated**, with reverse-proxy auth supported as a first-class mode
  (`leafwiki` again), so it drops behind existing SSO instead of owning
  credentials.
- **Every mutation is an atomic git commit** with a descriptive message and an
  audit row, exactly as the CLI's are. There is no web-only write path.
- Embedded assets; `stdlib net/http` and `html/template`. No SPA build in the
  release path.
- Read-only mode by flag, for a shared instance.

______________________________________________________________________

## 14. Provenance, Trust, and Freshness

All three are OKF's, unextended. `gnosis` computes, never stores, anything
derivable.

### 14.1 Trust Tiers Are Derived

OKF §5.3, a pure fold over the `verified` actor list:

- no `verified` key ⇒ **unverified**
- `verified` by non-`human:` actors only ⇒ **machine-confirmed**
- `verified` by a `human:<id>` actor ⇒ **human-reviewed**

Tiers are advisory signals, not access control (OKF §5.3), and a concept with no
trust frontmatter MUST remain consumable (OKF §11). This is exactly the
provenance-tier problem the manifesto identified — the invariant that a person
read every byte dies the moment ingestion is automatic, and nothing recorded
which parts still held it. OKF records it.

### 14.2 Credibility Is Inferred, Never Scored

OKF §5.1 carries objective per-source signals — `author`, `usage_count`,
`last_modified`, framed by `usage_window` — and explicitly refuses to store a
credibility score, because "a score is subjective, unportable across consumers,
and goes stale." `gnosis` adopts that reasoning wholesale. It surfaces signals
and lets a reader judge; it does not compute a number.

`usage_count` is documented there as coarse — comparable at the alive-versus-dead
and order-of-magnitude level, not as a precise cross-kind ranking. `gnosis` MUST
present it as liveness and trend.

**The signals combine as disqualifiers, never as a sum.** The four questions a
reader is actually asking of a source are whether it has the standing to make the
claim, whether it has a reputation for accuracy, whether it has a motive to be
inaccurate, and whether there is reason to doubt its integrity — and the rule is
*hesitate if either of the first two is negative or either of the last two is
positive*. That is a conjunction of vetoes, and it does not reduce to a number: a
source with excellent standing and an obvious motive to mislead is not the average
of the two.

This matters because the alternative is available and wrong. Weighting the signals
would produce a single figure that ranks sources, and that figure is a credibility
score with a different name — refused above, and refused again for the reason §17
gives. `gnosis` therefore surfaces each signal and, where a disqualifier is
present, says which one.

### 14.3 Freshness

`stale_after` is an absolute date, not a relative TTL, and OKF's rationale is a
determinism argument: it "keeps the staleness decision a plain date comparison
with no reference to when the concept was read." A TTL is read-time-dependent; a
date is a pure function.

Beyond the date, `gnosis` tracks archive-versus-upstream drift, and reports state
with `goalx/cli/freshness_state.go`'s four-value vocabulary — `fresh`, `stale`,
`unknown`, `not_applicable` — because `unknown` (never checked) and
`not_applicable` (no upstream to compare) are genuinely distinct from `stale`,
and collapsing them turns "we never looked" into "it is fine".

#### 14.3.1 Nothing Here Is Periodic, and One Thing Should Be

Every trigger in this specification is an event: an ingest, a conflict, a pull
request, a `stale_after` date somebody chose to set. Nothing revisits a claim
because time has passed.

The consequence is quiet and cumulative. A claim admitted correctly, never cited,
never conflicted, and carrying no `stale_after` is never looked at again — and it
is indistinguishable in every report from a claim reviewed last week. `stale`
catches only what an author thought to date, which is the subset least likely to
need catching, since an author who anticipated expiry was already paying
attention.

So one report is time-based:

```text
gnosis stale --unreviewed <duration>   claims whose last verified event predates a window
```

Three properties keep this from becoming an expiry mechanism:

- **It reports; it never invalidates.** An old claim is not a wrong claim, and
  marking one stale for the sole reason that nobody has looked at it would spend
  the corpus's credibility on the passage of time.
- **The window is a parameter, not a default policy.** Different corpora and
  different subjects age differently, and a single built-in interval would be
  wrong for most of them.
- **It pairs with the seeded sampler (§10.5).** A corpus too large to re-review
  exhaustively gets a reproducible sample of unreviewed claims rather than a
  backlog nobody starts.

The prompt for this was a personal-knowledge-management maintenance cadence —
review active projects weekly, ongoing responsibilities monthly — which is a
discipline event-driven systems reliably lack. The point is not the interval. It is
that "nobody has examined this since it was admitted" is a fact about the corpus,
and no existing check reports it.

### 14.4 Evidence Durability Is a Fourth Derived Signal

Because the archive holds text and not sources (§4.2), not every claim is
equally provable offline. That difference MUST be visible per claim rather than
averaged into a corpus-level number.

Derived, never stored, from the `durability` of a claim's evidence entries:

| Signal              | Condition                                                             |
| ------------------- | --------------------------------------------------------------------- |
| **provable**        | every sourced claim cites `archived` or `extracted` text              |
| **partly provable** | at least one claim cites archived text, at least one is `referenced`  |
| **unprovable**      | every sourced claim is `referenced` — nothing can be checked offline  |
| **not applicable**  | the claim is adjudicated (§10.4), so it carries a warrant, not quotes |

This is the same derive-don't-store discipline as OKF's trust tiers (OKF §5.3)
and for the same reason: a stored score goes stale and is unportable, while a
fold over recorded facts is neither.

Three consequences that keep it honest:

- **Durability is orthogonal to trust.** A human-reviewed claim resting on
  `referenced` sources is well-attested and still unprovable; a
  machine-confirmed claim over archived text is weakly attested and fully
  provable. Neither ordering dominates, so `gnosis` reports both axes and
  composes neither.
- **`gnosis search --provable` exists** so a reader assembling an argument can
  restrict to claims that can still be checked. That is the query the
  distinction is for; without it the signal is decoration.
- **Weakness is weighed against centrality, not reported flat.** See below.

#### 14.4.1 Unprovable Is Only Interesting Where the Corpus Leans on It

`referenced` claims are admitted (§4.3) on the reasoning that weakly trusting a
reliable external authority is fine when the claim is not central. That reasoning
has a mechanical consequence: **the risk is the product of weakness and
centrality**, so reporting weakness alone floods the reader with the peripheral
cases that were never a problem.

Centrality is derived from the link graph, which `gnosis` already maintains, and
is a plain count rather than a judgment:

| Class                 | Condition                                                | Treatment                                                |
| --------------------- | -------------------------------------------------------- | -------------------------------------------------------- |
| peripheral-weak       | `unprovable`, in-degree below the corpus median          | informational; not listed by default                     |
| **load-bearing-weak** | `unprovable`, in-degree at or above a declared cut       | **reported by `lint`, every run**                        |
| cited-by-provable     | `unprovable`, but a `provable` claim cites it as support | reported — provable work is resting on unprovable ground |

The in-degree cut lives in `standards/archive.toml` with the empirical basis for
whatever value is chosen, and it is a **reference value, not a gate**: nothing
blocks on centrality. A load-bearing weak claim is a prompt to go find a better
source, which is human work, not a build failure.

Two guards against this becoming a score: the classes are named states, never a
number; and `lint` MUST report the count it suppressed as peripheral, because a
check that silently drops most of its findings reads as coverage.

______________________________________________________________________

## 15. Security and Safety

- **Injection scanning at admission**, before a model sees content (§9.3).
- **Secret redaction before disk**, via a maintained ruleset (§7.2).
- **Human approval for irreversible or contested actions**, with phrase
  confirmation, no `--yes` bypass, no environment-variable override.
- **Audit every write.** `skillet/auditlog`, one row per mutation: operation,
  actor, paths, content hashes before and after, standards hash, finding ids.
  `clu` records every write and can answer who did what when; so must this.
- **Atomic commits.** One operation, one commit, via `atomicfile` plus go-git.
  `beadwork`'s intent-replay-on-conflict model is the reference if concurrent
  writers appear; not built until they do.
- **Quarantine lives outside the bundle**, at `.gnosis/quarantine/`, and is
  gitignored. This is a decided constraint, not a default: unvetted text is text
  an agent will obey, and a coding agent browsing the repository does not know
  about `--include-quarantine`. Putting unvetted content beside vetted content in the
  working tree would undercut the whole of §9.3. Tier 1 is reachable only through
  `gnosis` commands, never by a filesystem walk of the bundle.
- **Archived SVG is sanitized by refusal** (§4.4) and served under a
  `default-src 'none'` CSP from a non-session origin. It is the one allowed
  format that can attack a reader.
- **No network in the default path** except explicit `fetch`. No telemetry.

______________________________________________________________________

## 16. Interop

### 16.1 Findings Are the Family's

`finding.Diagnostic` `{severity, category, path, message}` on stdout as JSON.
`canonizer gate` can block on `gnosis` findings and vice versa, because the
severity model is shared.

Two additive fields, both borrowed and both classification rather than
measurement:

- **`certainty`** — `agentsys`'s HIGH / MEDIUM / LOW meaning safe-to-auto-fix /
  needs-context / needs-human-judgment. The concept it encodes has a name worth
  using: **requisite uncertainty**, a confidence calibrated to what the system
  actually allows one to know. A finding asserting HIGH certainty about something
  the corpus cannot determine is the overclaim §17 exists to prevent, one level
  down.
- **`fix_class`** — `AgentLint`'s `guided` / `assisted`, per check, stored in
  `standards/evidence.toml` beside the check's own evidence.

Both say *who acts*, which a severity does not.

### 16.2 Manifests and Proofs

`gnosis manifest` emits `skillet/manifest` over the corpus so downstream tools
consult it instead of rediscovering the tree, and `manifest.Diff` answers which
concepts changed since a baseline. `gnosis proof create` binds corpus and tier-0
digests into a `skillet/proof` packet, so `adh` can close an arc that touched the
knowledge base under `no-proof-no-close`.

### 16.3 Knowledge Flows Both Ways

- **In**: `book2skill` distils a book into skills; `gnosis` ingests the same book
  as sourced concepts. The two are complementary, and both must cite the same
  passages — which is why `quotecheck` belongs in `skillet` rather than in either
  tool.
- **Out**: corpus knowledge that is normative renders as `skillet/ruleset`
  canonical form, so `canonizer` can grade it and `skillsaw` can score skills
  distilled from it. `gnosis export --format okf` produces a portable bundle for
  sharing outside the team.

### 16.4 Direct-Model Stub

An interface with one in-tree implementation that **refuses every call** and
returns a diagnostic naming the prompt file to run manually. It exists so the
seam is visible and testable, and so no code path accidentally grows a real
provider call. Wiring an actual provider is out of scope; the manifesto wants the
option, not the dependency.

______________________________________________________________________

## 17. Findings, Not Scores

`gnosis` MUST NOT emit a weighted corpus-quality score.

The usual objection to a score is that it is subjective, unportable, and goes
stale. The stronger objection is arithmetic: **an average over a heterogeneous
population describes no member of it.** A corpus spans types, subjects, sources,
provenance classes, and ages, and a figure averaged across all of them is not a
rough summary of the corpus — it is a summary of nothing, in the way that the
average adult has one breast and one testicle. Statistics are meaningful over
groups that are homogeneous with respect to the action that might be taken next,
and no corpus-wide grouping is.

`canonizer` is findings-based by design so no threshold can become a ship gate,
and a "corpus health score" would be that threshold wearing a different name.
Admission is a findings problem: this claim conflicts with that one, both cite
sources, a human decides.

Where `gnosis` reports numbers they are **measurements with intervals** —
proportions via `stats.Wilson`, regressions via `timeseries.Detect`, confidence
reliability via `calibration.Compute` — never a single composite.

The interval is not decoration, and the reason is worth stating as a diagnostic a
reader can apply: **a number reported without error is evidence that no
measurement was made.** A measurement is a reduction of uncertainty based on
observation, so it arrives with the uncertainty that remains; an exact figure with
no interval is either a complete count of something small — the documents in a
bundle, the findings in a run — or a calculation dressed as an observation. Both
appear in this corpus and they must be distinguishable. So: complete counts are
reported bare and labelled as counts, and anything estimated from a sample or a
model carries its interval or is not reported.

**A mixed-provenance claim is reported at its weakest link.** A claim resting on
one byte-exact quote and one adjudicated assumption is not "mostly sourced": its
trust tier, its durability, and its `certainty` all take the value of the weaker
component. The failure this prevents is well documented outside software — careful
engineering estimates get combined with a salesman's guesses, and the reliability
of the sum is reported as the reliability of the engineering part. A reader
skimming a claim whose evidence block contains one exact quote will extend that
exactness to the whole claim unless the artifact refuses to let them.

**Accuracy is not relevance, and the two are easy to confuse.** A measurement that
is exact, reproducible, and cheap to make is not thereby worth making; a poorer one
closer to the question may be worth far more. It is easy to measure conformance and
hard to measure whether a claim is true, in the same way that it is easy to measure
training and hard to measure education — and a report will drift toward whichever
it can compute. §12 states the consequence for the check registry.

**Observer bias is a design constraint here, not a caveat.** The act of observing
changes what is observed: contributors who know which checks run will write to
pass them, and contributors learn what a system rewards and punishes and route
their signals accordingly. Both effects are documented independently — one in
empirical measurement, one in governance practice — which makes this structural
rather than a worry about particular people. It is the deepest reason the exit
codes separate *the corpus has findings* from *gnosis broke*: a tool whose honest
report of a problem is indistinguishable from its own failure teaches people to
avoid running it, and a check nobody runs measures nothing.

### 17.0 Trends Are Not Scores, and the Distinction Is Required

This section is easy to over-read as forbidding all reporting over time. It does
not, and the difference matters enough to state before anything else.

A **score** compresses a corpus into one number that stands in for judgment, and
whatever threshold it acquires becomes a gate nobody chose. A **trend** is the
count of open findings across successive runs, and it answers a different question
entirely: *is anything being closed?*

The reason to require it is that a findings tool with no trend is a detector with
no control loop, and detection without response is the failure mode this whole
section is guarding against from the other side. A report that lands, is noted,
and changes nothing has the same practical value as no report. Three readings are
distinguishable and each means something different:

| Open findings over successive runs | Meaning                                                 |
| ---------------------------------- | ------------------------------------------------------- |
| falling                            | the corrective loop is working                          |
| **stable**                         | **deviation has been normalised rather than corrected** |
| rising                             | the loop is absent, too slow, or being routed around    |

The stable case is the dangerous one and the one a snapshot cannot show. A single
run reports the same count either way.

Two constraints keep this from becoming a score by accident:

- **It is a count of findings, never a quality figure.** No weighting across
  categories, no ratio against corpus size, no composite. `skillet/ratchet` is
  the mechanism; a ratchet on a count is not a threshold on quality.
- **A rising count MUST NOT block on its own.** Severity blocks; a trend informs.
  A corpus that has just started running a check it previously skipped (§12) will
  see its count rise for a good reason, and blocking on that punishes the act of
  looking more closely.

Reader-filed challenges (§10.7) are counted here alongside check findings, and
they are the part of the trend most worth watching: a check finding going
unanswered means a tool was ignored, while a challenge going unanswered means a
*person* was.

**`findings.state` therefore has three values, not two: open, closed, and
deferred.** A deferred finding records who saw it, when, and why they are not
acting yet — and because that is a human decision rather than a machine
observation, it is committed to the bundle rather than cached in the index, per
§10.7.4. The reason to spend a state on this: the common failure of a findings
system is not that problems go undetected but that detected problems go
unanswered, and silence is indistinguishable from nobody having looked. Reviewing
the deferred set is a different activity from reviewing the open set, and it is
the one that tells a team what it has decided to live with.

### 17.1 Structural Verification Is Not Semantic Agreement

Two acts, never conflated:

- **Structural verification** — zero network, zero model. Conformance, quote
  validation against archived text, closure, identity reconciliation, stated
  limitations, every derived signal recomputed from the artifacts themselves.
- **Semantic review** — the cold critic reads the claim and its sources and
  judges whether the claim is supported.

**The gap between them is Gettier's, and no amount of engineering closes it.** The
quote invariant is a *justification* check: it establishes that a claim is
supported in the way it says it is. It cannot establish that the quote **bears on**
the claim. A justified true belief whose justification and truth are not properly
related is the stopped-clock case — a man reads a clock that stopped two days ago,
at the one moment it happens to be right — and it is not knowledge despite being
both true and reasonably held.

The corpus can produce exactly that state: a claim whose quote validates
byte-exact, whose assertion is true, and whose quote does not support it. This is
worth stating because it changes what strengthening `quotecheck` could ever buy.
Tightening normalization, raising `MinPassageWords`, or requiring more quotes all
make the justification *stronger*; none of them make it *related*. Semantic review
is not a more thorough version of structural verification, it is the only thing
that addresses a different question.

A structural pass means the corpus is **internally honest**, not that anyone
agrees with it. `gate` MUST report which act ran: a `semantic_review` field in
the findings result, and a sentence in the human output. A structural pass
reported as "verified" is exactly the overclaim this section exists to prevent.

The interesting case is a claim that passes structure and fails review. That is
not a malformed artifact; it is a well-formed wrong one, and it is the state the
corpus most needs a name for.

### 17.2 Claims Must State Their Limits

`gnosis_limitations` is **required and non-empty on normative concepts** — the
ones asserting what should be done, as opposed to recording what is. A concept
that will not say what it does not cover cannot be well-formed, and `lint`
refuses an empty one.

This is deliberately a property of the artifact rather than a discipline asked of
a reviewer. A convention to "state your scope" is forgotten; a document that
cannot be written without one is not.

### 17.3 Governance of Claims About the Corpus

Two rules, both about what the tool licenses a person to say:

- **The data is the authority.** Findings JSON and the standards hash are
  canonical; if a narrative disagrees with the JSON, the JSON wins.
- **Enumerate the claims not authorized.** That repo states plainly that no
  public "proven / validated / improves" claim is authorized and that inferences
  are labeled — and publishes that zero of its 28 skills hold a replicated
  ELEVATE verdict rather than hiding it. `gnosis`'s README MUST state what a
  clean `lint` and `gate` do and do not license anyone to say: that no
  deterministic check objected and, where a critic ran, one critic agreed.

### 17.3.1 Evidence Sufficiency Scales with the Strength of the Claim

Relevance is the *quality* of a claim's evidence; sufficiency is its *quantity*.
The corpus checks the first — a quote that validates is on-topic by construction —
and has no account of the second. This is the gap: nothing asks whether a claim's
evidence supports the **scope it claims**, only whether evidence exists.

The rule is that sufficiency is proportional to the strength of the assertion. One
photograph of someone at an art shop on the day a painting sold may be sufficient
for *John may have bought the painting* and is plainly insufficient for *John
definitely bought the painting*. The evidence did not change; the claim did.

Three consequences, and the first is the one that makes this checkable:

- **The strength of a claim is partly lexical.** `always`, `never`, `all`, `every`,
  `guarantees`, `MUST` assert more than `typically`, `usually`, `in most cases`,
  `may`. Those markers are a closed list held in `standards/` as data, and a claim
  carrying a universal quantifier over a single supporting quote is a reportable
  mismatch — `coverage`, warning tier.
- **It is a finding and never a rejection.** The remedy is usually to weaken the
  claim rather than to find more evidence, and that is an author's decision. A gate
  that rejected the claim would push the author toward removing the qualifier
  rather than adding the caveat.
- **Normative claims are held higher, per §14.4.1.** A universal assertion the
  corpus leans on is where being wrong costs most, which is the same stakes rule
  §10.6 and §14.4.1 already apply. Same evidence, same truth, different standard,
  because the cost of error differs — a person who knows a door is locked when a
  colleague's jacket is inside will say they do not know it when an armed intruder
  is being hunted, and both answers are correct.

**Omission is the third question and belongs to the critic.** Evidence that
supports a claim and omits what tells against it can look stronger than it is, and
no deterministic check finds an absence. The remedy is to seek opposing material,
which is why the critic exists and why §6.2.1's random-sample pass matters: a
selector that surfaces only related claims cannot surface what was left out.

### 17.4 Conclusion First — `lead` Is a Checked Property

`claims.lead` holds the claim's conclusion, stated first, in its own words. On a
normative claim it is checked, not merely conventional, and `lint`'s `lead` check
reports a lead that restates background instead.

The rule is BLUF — bottom line up front, the military briefing convention: say
the conclusion, then explain it. Its usual justification is courtesy toward busy
readers, but the sharper framing is that **it is an information architecture for
decision making** rather than a writing style. A document that buries its
conclusion under its derivation cannot be excerpted, and both of this corpus's
readers need excerpts:

- **An agent retrieving under a context budget** takes the first *n* tokens of a
  ranked result. If those tokens are background, retrieval returns something
  true, relevant, and useless, and the agent has no way to know that the part it
  needed was three paragraphs down.
- **A person scanning a conflict queue** is comparing two claims, not reading two
  documents. Conclusion-first is what makes a queue scannable at all.

This is why `lead` is a column rather than derived at render time: it is the unit
of comparison in §10's conflict predicates and the unit of retrieval in §11, and
in both cases the alternative is guessing which sentence the author meant.

______________________________________________________________________

## 18. Testing Requirements

Per the family's standard, and stricter in the places where being wrong is
expensive.

**One invariant here is easy to leave untested because both halves look obviously
right.** `index rebuild` recomputes every `claims.pos` by searching for its anchor,
so the property to assert is that the recomputed offset **points at text whose fold
hash equals the stored `anchor_hash`** — not merely that a rebuild produces some
offset. The failure it catches is a search that finds the wrong occurrence of a
short anchor, which yields a plausible number pointing at the wrong paragraph and
no error anywhere. Pair it with the round trip: reflow a document's whitespace and
assert every `pos` moves while every `anchor_hash` and every claim id does not.

The question this section answers is Hamming's, asked of a rig built to life-test
components destined for a submarine cable: **"Why do you believe the test equipment
is as reliable as what is being tested?"** `gnosis` is test equipment for a corpus,
its determinism is a claim about repeatability rather than about correctness, and
the planted-defect self-test below is the only honest answer available. A gate that
has never been shown to catch a defect it was built to catch is an assertion.

### 18.1 Purity and Property Tests

Every pure function — quote validation, conflict predicates, trust-tier
derivation, staleness, candidate selection, index rendering — gets property tests.
`textnorm.Fold` idempotence, `Render`/`Parse` round-trip, and
`identity.Hash` stability are inherited invariants and MUST be asserted here too.

### 18.2 Mutation Tests on the Gates, in Both Directions

Any check that can block MUST have a mutation test proving it fails when the
defect it guards against is planted. The promote gate's self-test runs on every
invocation; `gnosis gate --selftest` runs it alone.

A gate is an eval, and an eval that has never been shown to reject a bad input
has not been shown to check anything. So the battery is **two-sided**:

- a **blind spot** is a planted defect the check still passes;
- a **coverage gap** is a correct input the check wrongly rejects.

Only the first is usually tested, and the second is what makes a gate get
switched off. Both are reported per check.

**Mutations are mined from defects the corpus has actually produced**, not
invented. An invented battery measures imagination; a mined one measures
exposure. The classes to seed it with are the ones a string predicate is weakest
against — a quote that validates against the wrong half of a sentence, an
evidence entry whose archive reference is right but whose `source_id` is not, a
negated claim whose keywords all appear.

### 18.3 Determinism Tests

- `index rebuild` twice from the same bundle produces byte-identical `index.db`
  content hashes.
- A full ingest replayed with `--cache-only` produces a byte-identical corpus.
- `lint`, `search`, and `conflict` outputs are stable across runs and independent
  of filesystem iteration order.

### 18.4 Corpus Tests

A committed fixture bundle exercising: every OKF optional family present and
absent; a broken link; an unknown `type`; unknown extra keys; a bare `verified`
mapping; a deprecated concept with a supersedes edge; an adjudicated claim with
no quote. Each asserts the **negative** conformance requirements — that `gnosis`
does not reject what OKF forbids rejecting.

### 18.5 Adversarial Fixtures

A source containing zero-width characters, a bidi override, a prompt-injection
string, and a plausible-looking fabricated quote. Each MUST be caught, and the
test MUST assert *which* check caught it, so a check silently ceasing to fire is
visible.

______________________________________________________________________

## 19. Phasing

Each phase ends with a working tool. No phase depends on a decision that has not
been made.

**Phase 1 — Read.** OKF parse/render via `frontmatter` + `markdown`; `init`,
`doctor`, `index rebuild --check`, `show`, `search` (FTS5), `graph`, `lint`
(conformance, identity, index-drift, orphan, broken-link, log-format). No
ingestion, no model. Proves the data model and the derived-index discipline.

**Phase 1 is document-scoped, and the `claims` table stays empty.** Identifying
claims requires extraction, which is Phase 2 and needs a model; a deterministic
phase cannot honestly say where one claim ends and the next begins, and §11
demonstrates why naive splitting is not an alternative. So `search` queries
`documents_fts`, `show` renders a document, and `graph` walks document-to-document
links.

The reason this is a scope decision and not a shortcut: a claim identifier issued
in Phase 1 would name something gnosis had guessed at, and every verdict later
attached to it would inherit that guess. Nothing is cheaper to defer than an
identifier for a thing you cannot yet delimit. When extraction lands, §5.5.1's
anchors are what let those claims arrive without disturbing anything Phase 1
wrote.

`init` seeds `ontology.toml` with the starter types of §5.8 and **no subjects**.
Types are needed here because OKF requires one on every document; subjects are
not needed until Phase 3, and starting them empty is what keeps this phase free
of a vocabulary negotiation.

**Phase 2 — Ingest with proof.** Tier 0 archive; `fetch`; the scan stage;
`ingest`/`admit` relay with the response cache; `quotecheck` promoted to
`skillet`; quarantine and the promote gate; `log.md` maintenance; audit trail.
The corpus starts accumulating, and every claim is already traceable.

**Phase 3 — Curate.** `ruleset/conflict` in `skillet`; `conflict`, `critic`,
`gate`, `adjudicate`, `supersede`; the two provenance classes; constraint
reporting; `stale`; `miss report`. The corpus becomes authoritative because
contradictions have somewhere to go.

**Phase 4 — Structure and scale.** Structured claim subjects (the §10.2
prerequisite, its own decision); interval and enumeration conflicts; optional
semantic rerank; `mine` and the Stop-hook companion; family export paths.

**Phase 5 — Serve.** The authenticated web interface, review queue first.

______________________________________________________________________

## 20. Deferred Decisions

Four decisions are deliberately left open. Each would change the work materially,
none blocks Phase 1, and each is cheaper to make against a real corpus than
against an imagined one.

1. **Presented hierarchy.** Deferred *by design* rather than by postponement.
   Because identity is opaque and presented paths are computed views (§5.6),
   choosing a hierarchy commits nothing: no link targets change, no schema
   changes, and several views can coexist. A team corpus spanning engineering,
   QA, product, and support could organize by department, by concept type, or by
   product area, and the honest way to find out which is to look at ~50 real
   concepts and see how people actually reach for them — the `katalyst inspect`
   move of deriving conventions from evidence rather than declaring them.
2. **Which subject keys the corpus tracks.** The vocabulary mechanism is settled
   (§5.8) and so is how each half is populated: types are seeded at `init` from
   the starter set, with a mechanical test for whether a proposed type is real —
   two types sharing `normative`, `expects_subject`, and `template` are one type
   with two aliases. Subjects start empty and accrete on the second collision,
   each entry justified by a conflict that actually occurred. What remains open is
   therefore not a design question but an accumulating one: the domain keys
   themselves, which the corpus will nominate and the team will ratify. Nothing
   blocks until §10's comparison predicates run in Phase 3.
3. **Whether any single subject ever needs a hard capability gate.** The
   adjudication model is settled (§10.6): three derived tiers with `quorum` as the
   ceiling, domain expertise surfaced in the queue rather than enforced, and a
   per-subject `requires_capability` escape hatch for the rare key where a wrong
   decision carries external consequence. Enabling that hatch needs both a
   real-consequence subject and two qualified holders, and the expected outcome is
   that neither the hatch nor a roster is ever needed — the required rationale and
   a sufficient queue are the stronger interventions, and if they fail a roster
   will not rescue them.
4. **Whether a trail is a first-class object.** Bush's associative trail is
   *named*, *ordered*, non-exclusive, and **transferable** — he "photographs the
   whole trail out, and passes it to his friend for insertion in his own memex."
   `links` gives us pairwise edges with a derived `rel` and nothing that names or
   orders a path through them, so the one artifact *As We May Think* treats as the
   point of the exercise has no representation here. §8 of that essay is explicit
   that the trail, not the archive, is what gets inherited: "the inheritance from
   the master becomes, not only his additions to the world's record, but for his
   disciples the entire scaffolding by which they were erected." That is a fair
   description of what this project means by tribal knowledge, which is why the
   omission is recorded rather than dismissed.
   Deferred rather than designed because the shape has an obvious cheap answer
   that costs nothing to postpone: **a trail is a document of a reserved type**,
   whose body is an ordered list of links and whose prose says why the order is
   what it is. It then inherits identity, provenance, review, supersession, and
   transfer-by-git for free, and needs no table. The alternative — a `trails`
   table with membership rows — buys ordering ergonomics and gives up every one of
   those properties. What is genuinely unknown is whether anyone builds trails
   deliberately or only in retrospect, and that is answerable only from a corpus
   with a real link graph in it.

______________________________________________________________________

*Sources: [`llm_wiki_pattern.md`](./llm_wiki_pattern.md);
[`manifesto.md`](./manifesto.md) and the repositories it surveys under
`~/Documents/agent-red` and `~/Documents/agent-blue`; the Open Knowledge Format
v0.2 specification at `~/Documents/agent-blue/knowledge-catalog/okf/SPEC.md`;
and the family's own backlogs in `skillet`, `exegesis`, `skillsaw`,
`agentic-dev-harness`, and `canonizer`.*
