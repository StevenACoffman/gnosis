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

#### 1.1.0 a Specimen, Because the Argument Above Is Otherwise Abstract

Everything in §1.1 is a position about testimony in general, and a reader is
entitled to ask what the expensive posture buys that the cheap one does not. One
recent artifact answers it precisely, and it is worth recording because it arrived
as a document *about this family's tooling* rather than as a source somebody tried
to ingest.

A commissioned review of these seven repositories carried a section of
recommendations, each supported by references of the form *"Line 123
(cloudstrategy_book.md), Line 124 (eip_book.md), … and 2 other articles"*. The
document is well-formed. It names its sources, gives line numbers, distinguishes
prior-art mappings from research findings, and reads as careful work. Under global
reductionism — *absent a warning sign, believe it* — it is admissible, because
there is no warning sign on its face.

Three things are true of it and none is visible without opening the cited lines:

- **The same five references support two unrelated recommendations** in two
  different files — parameterised rulesets in one, a lockfile of content hashes in
  the other — with an identical trailing *"and 2 other articles"*.
- **The line numbers and the filenames disagree.** Lines 123–124, given as a cloud
  strategy book and an enterprise-integration book, are extracts from Russell's *A
  History of Western Philosophy*: "the terror of cosmic loneliness", and a
  Pythagorean "ethic which praised the contemplative life". The lines given as
  agile-organisation posts are a CLI design guide.
- **A further claim cites 65 articles** for the proposition that backend and
  frontend systems have different performance profiles.

This is the failure mode the whole evidence apparatus below exists to make
impossible, and the reason it is recorded here rather than in a survey note is
that it is *not* a story about a careless reviewer. It is the ordinary output of
generating prose over a corpus, and it will be the ordinary input to any corpus
that accepts prose. The citations were almost certainly attached because a
citation was expected there, and no step between writing and delivery required
anybody to open one.

**What separates the two postures is exactly one operation: looking up the line.**
Global reductionism does not require it and would have admitted every claim above.
Local reductionism requires a specific positive reason for *this* source on *this*
topic, and the cheapest such reason is that the quoted passage is present in the
named source — which is §9.4's rule, mechanised, and the whole of what
`quotecheck` does. The specimen is not evidence that people are unreliable. It is
evidence that **a citation is not self-verifying**, that the gap between a
well-formed reference and a supporting one is invisible at review speed, and that
a corpus which cannot check the difference will accrete the difference.

Two limits, stated so this does not read as more than it is. One specimen is one
specimen, and it is drawn from a genre — machine-assisted survey — that is
unusually prone to this. And gnosis would not have caught this document, because
gnosis validates a quote against an *archived source*, and these references name a
line number in a file rather than quoting it. That is a gap worth naming: **a
citation that gives a location and no passage is unfalsifiable by construction**,
and §5.5's requirement of a verbatim passage is what forecloses it. The specimen
argues for that requirement rather than illustrating its sufficiency.

#### 1.1.1 the Other Posture Is the Field's Default, and It Works

This section long argued against an opponent it declined to name, which made the
position look easier to hold than it is. Surveying the field supplied the names.

Four working systems write durable knowledge with **no gate on the write path**.
One rewrites a page when a new source contradicts it — *"claims revised, stale
facts replaced"* — and resolves contradictions automatically. One distils an
agent's session into skill files with an unverified model pass. One saves every
lesson it teaches and revises itself from feedback. One merges related facts into
consolidated beliefs on a background loop and raises a **proof count** as sources
corroborate, which is source reliability inheriting to claims exactly as the rule
above forbids.

None of these is careless and each is used. What separates them from this
specification is not rigour but **audience and reversibility**. Three serve one
person's recall, where rewriting is right: you want the current answer, not the
history of your own wrongness, and the cost of a bad memory is that one person is
briefly confused. A shared corpus inverts both. A rewritten claim cannot be
distinguished from a fabricated one, the reader was not present for the revision,
and the cost of a bad claim is that somebody builds on it.

So the honest statement of this section's position is not that unverified
accretion is wrong. It is that **unverified accretion is correct for a memory and
wrong for a record**, that gnosis is the second, and that the gates below are the
price of the difference rather than evidence of superior care. A reader running a
personal vault should take the other design; this one is more expensive and the
expense buys something they do not need.

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
           └── fetch/<sha256[:2]>/<sha256>.json   one record per source version (see §4.3.1)

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
         sha256 of the fetched bytes       ← recorded in the fetch record (§4.3.1)
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
recorded in its fetch record (§4.3.1) and, derived from those, in
`sources_fetched.disposition`:

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

#### 4.3.1 One Ledger File per Source Version

A fetch record is a single immutable JSON file at
`evidence/fetch/<h[:2]>/<h>.json`, where `h` is the sha256 of the canonical
record: `{uri, source_sha256, byte_size, media_type, disposition, archive_path, extractor, extractor_version, extracted_from, reject_reason}`.

**It is one file rather than one append-only ledger because a shared ledger merges
silently.** A single `fetch.jsonl` in a git-merged tree conflicts on its final line
whenever two users fetch anything, and the available fix — git's built-in
`union` driver, which needs only a committed `.gitattributes` line — resolves by
keeping both sides' lines unconditionally. For a genuine append that is right. For
a line somebody edited it keeps both versions, leaving two records that assert
different things about one fetch with no marker that a merge happened.

That is tolerable wherever a check can catch it. It is not tolerable here, because
the `referenced` disposition archives nothing (§4.3) — so for exactly the fetches
whose integrity cannot be re-derived from any other artifact, the ledger is the
only record. One file per record makes append-only **structural**: a rewritten
record lands at a different path, so a careless edit is visible rather than
absorbed.

**And that is the whole of the claim, which is narrower than content-addressing
sounds.** The hash detects *accidental* corruption and an edit made without
recomputing it. It is not authentication. A local actor with the same user and
filesystem access can rewrite a record, recompute its hash, rename the file, and
leave nothing to notice — and can equally rewrite the archive, the git history, or
the binary that checks them. Without an external trust anchor no arrangement of
hashes changes that, so this specification claims tamper-*evidence* against mistakes
and states tamper-*resistance* against a same-user actor as an explicit non-goal.
Saying so matters more than it might seem: a reader who believed the stronger claim
would stop looking for the control that actually provides it, which is git history
on a remote nobody local can rewrite.

**The record carries no timestamp, and that is the load-bearing part.**
Content-addressing over the source bytes rather than the fetch event means tier 0
grows when the corpus learns something — a new source, or a changed one — and not
when somebody checks. A re-fetch finding unchanged bytes produces the same path,
finds the record already there, and writes nothing: a no-op on disk, which is what
§9.2 calls it.

Including the timestamp was considered and refused for two reasons. It makes growth
a function of *checking* rather than of knowledge — a weekly staleness sweep over
500 sources is some 26,000 permanent committed records a year, each identical to
its neighbours but for a timestamp, in the tier whose purpose is evidence. And the
history it buys has no reader: §14.3's `fresh`/`stale`/`unknown` distinction needs
only the *latest* check, and nothing in this specification consumes the sequence.

**So when did we last look?** That lives in `.gnosis/checked.jsonl` — per-user,
gitignored — and the reason is §10.7.4's rule rather than convenience: *decisions
are committed, observations are cached.* That upstream **changed** is a fact about
the corpus, produces new bytes and a new record, and must travel. That *I looked
and nothing had changed* is an observation made at a moment, and two users who
report different freshness at the same commit are both right, because one of them
looked. What this gives up is stated in §20.

`.gnosis/fetch.jsonl` remains as a **derived rollup**, rebuilt by
`index rebuild` from the committed records, so the ledger is still greppable
without being authoritative.

#### 4.3.2 the Payload Cap Is Not Relaxed, and the Refusal Says Why

`embedded_payload_cap` over-reports on prose about data URIs, and §9.3 stage 4 now
applies it to a candidate document as well as a fetched source — where the
consequence is worse, because a scan failure is `refused` and §9.5.1's human path
opens only for what could not be checked. So the question of relaxing it is settled
here rather than left as a note.

**It is not relaxed.** Both obvious relaxations are wrong:

- Exempting a data URI inside a fenced code block sounds principled and fails on the
  cap's own rationale. This is a **weight** bound — it exists because a payload above
  it "reintroduces exactly the binary weight the allowlist excludes, inside a file the
  allowlist admitted" — and nine kilobytes of base64 inside a fence weighs nine
  kilobytes.
- A frontmatter escape saying "this one is prose" is a bypass, and a gate that can be
  talked past is decorative (§9.5.1).

What was actually wrong is that the refusal was **unactionable**. An author was told
`embedded-payload` and not how large or against what, and the only move available from
there is to argue the threshold down. The refusal now carries the measurement — "an
embedded payload is 9,017 bytes against a declared cap of 8,192" — which is the same
information turned into a truncated example instead of an argument. `archive.Bound`
carries it and renders it once, for both the tier-0 refusal and the promote gate.

The residual cost is stated plainly: a source that genuinely needs a payload above the
cap falls to `referenced`, keeping its URI and hash and losing its archived text. That
is the documented trade (§4.3) and the reason the cap is a *weight* bound rather than a
safety one — nothing is unsafe about the payload, and the corpus declines to carry it.

______________________________________________________________________

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

**A rebuild that loses most of the corpus MUST refuse rather than succeed.** Being
regenerable makes the index safe to destroy and therefore easy to destroy by
accident: a wrong `--bundle`, a partial clone, a working tree with `c/` unstaged, a
walk that silently stopped. In each case the rebuild does exactly what it was asked
and writes an index describing almost nothing — over the only artifact that would
have shown what was there a moment ago. So a rebuild whose document count falls
below a share of the last verified count declared in `standards/` is a **refusal
with the two numbers named**, not a warning, and `--force` is what a caller uses
when the corpus really did shrink.

This is the one place the regenerable-cache argument turns around. Everywhere else
it means the index is cheap to lose; here it means nothing else will notice that it
was. The check costs one integer stored beside the schema version.

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

The last row used to be a defect: the ledger was a single append-only
`fetch.jsonl`, which conflicts on every concurrent append. §4.3.1 replaced it with
one content-addressed record per source version, so the ledger now merges for the
same reason the archive does — two users who fetch the same thing produce the same
bytes at the same path, and git sees one blob.

#### 4.6.2 Writes Are Commands, and the Command Is a Value

The split above is already command-query segregation, and naming it settles a
question that otherwise looks like a transport decision.

**Reads bypass the coordinator by requirement.** `lint`, `search`, `show`, and
`graph` open the index directly and MUST work with no writer running, because a
corpus has to be inspectable when nothing is serving. **Writes go through it.** So
the coordinator is a command bus, and what crosses it is a command.

A command is a **value**, not a verb: one type per write operation, carrying
everything needed to decide whether and how to execute.

```go
type Promote struct {
	Path      string
	Effect    Effect // preview or apply; see below
	Approver  Actor  // who authorised it
	Rationale string // why, when the tier requires one (§10.6.4)
}
```

Three consequences, and the second is the reason this is in §4 rather than left to
the implementation.

**Every transport inherits the gating fields for free.** The CLI populates them
from flags, a socket or HTTP or MCP transport from a payload, an internal caller
directly — and none of them can construct the command without them. Review-gating
is therefore a property of the **type**, not of the wire format, which is stronger
than a schema-validated protocol field because it also binds the callers a protocol
never sees. It is what makes the transport question small: a transport serialises a
command and nothing more.

**§9.4's diff guarantee becomes constructible rather than promised.** That section
requires the diff the gate approved to be the diff that lands. If preview and apply
were two commands, or two code paths, the guarantee would reduce to a claim that
two functions agree — which is exactly the kind of claim this design refuses
elsewhere. As **one command differing in one field**, they cannot diverge: the same
handler receives the same input, computes the same diff, and `Effect` decides only
whether the final write happens. The property follows from the data model.

**The premise is "the same input", and a remote caller can break it.** In process
the writer lock spans compute-and-write, so nothing changes underneath a preview
and the premise holds for free. A transport that lets a caller preview, receive a
diff, and then send a *second* command to apply has two round trips with a gap
between them, and the corpus can move in that gap — at which point the handler
recomputes honestly and lands a diff the gate never approved. Nothing above
prevents that, because it is not a property of the command type.

So the rule for any served coordinator: **preview and apply are one round trip, or
the apply carries the revision the preview was computed against and is refused when
it no longer matches.** A lock held across both calls is the third option and the
worst, because it lets one idle client block every writer. This is deliberately not
decided here — no out-of-process writer exists yet — but it is the prerequisite a
two-call protocol has to satisfy, and recording it is what stops the transport
choice from quietly costing §9.4 the guarantee this section just constructed.

**`Effect`'s zero value MUST NOT be "apply".** A `DryRun bool` has this backwards —
`false` means *really do it*, so a caller that forgot the field performs a live
write. That inverts the rule this design applies everywhere else: `quotecheck`'s
`Unchecked` is the zero value, `finding.Action` has no `ActionUnknown`, `claims.pos`
is nullable because `0` is a real position. Go cannot make a struct field
mandatory, so the enforcement is a constructor plus a zero value that **fails
closed**:

```go
type Effect int

const (
	EffectUnset   Effect = iota // zero value: rejected, never assumed
	EffectPreview               // run every gate, write nothing
	EffectApply                 // run every gate, then write what they approved
)
```

`EffectUnset` is rejected rather than treated as a preview. A preview is a
deliberate request for a report, and silently substituting one would let a caller
that meant to write believe it had.

##### 4.6.2.1 What the Payload May Assert, and What It May Not

The design above was measured against `akbp`, which puts `dry_run`, `approved` and
`approval_required` on every write call. The comparison is worth recording because
it looked like a debt and is the reverse.

| `akbp`              | This design                                                                                   | The difference that matters                                                                               |
| ------------------- | --------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| `dry_run` bool      | `Effect`, with `EffectUnset` rejected                                                         | A bool defaults to `false`, which means *write*. A forgotten field here refuses                           |
| `approved`          | `Approver Actor`, unset and malformed both rejected, `IsHuman` required on the escalated path | Typed, so §9.5's refusal of a self-granted approval is enforceable on the kind                            |
| `approval_required` | the gate's own `NeedsHuman`, computed from the report                                         | **The caller cannot assert it.** A payload field means the party being gated declares whether it is gated |

The third row is the whole argument. A protocol field binds callers that cross the
wire; a field on a value binds those *and* the ones no protocol ever sees. Nothing
here is weaker than a schema-validated protocol, and one thing is strictly stronger.

**The line the comparison exposes is not on the command; it is at the wire.** Two of
these fields are asserted by the caller, and in process that is sound because the
caller is a person at their own terminal typing a flag. Over a transport it is not.

- **`Approver` is supplied by the transport, never by the payload**, and a command
  carrying a caller-set approver is refused. A remote caller sending `human:alice`
  is otherwise unverified, which turns §9.5's no-self-granted-approval rule into an
  honour system precisely when a non-human caller arrives — the caller class a
  served coordinator exists for. The mechanism differs per transport and the rule
  does not: peer credentials on a Unix socket, the authenticated session over §13's
  HTTP. It is written down before either exists because deciding it afterwards means
  deciding it by default.
- **The confirmation phrase does not survive the wire.** Typing a document's path
  defeats muscle memory in a person and costs a program nothing. So until §13 has a
  review queue, **the escalated path is refusable only at a terminal**: a served
  coordinator may preview anything and may apply an *approved* promotion, and a
  promotion the gate escalated is not completable over a transport at all.

That interim rule costs nothing today, because no out-of-process writer exists. It
is stated so that the window between the first socket and the first authenticated
session cannot ship the honour system by accident. §13's review queue is what
replaces it, and the replacement is the two-call shape §4.6.2 already requires: the
escalation returns a token bound to the diff, and the apply carries the token and
the rationale. **That is the same object as the "revision the preview was computed
against" above** — so the two-round-trip question and this one have one answer, and
answering either separately is how they end up inconsistent.

##### 4.6.2.2 One Protocol, Two Listeners, and Apply Is One Call

The transport was left open above as a genuinely later question. It is now settled,
and mostly by reading §13 rather than by choosing: `gnosis serve` carries the write
coordinator and the viewer **in one process**, because two servers would be two
authorities over one bundle, and that server is authenticated `net/http` with
reverse-proxy auth as a first-class mode. HTTP is therefore not one candidate among
several — it is required, in the same process as the coordinator, before Phase 5 is
finished.

**The argument that had favoured a Unix socket reverses under §4.6.2.1.** Filesystem
permissions were its appeal: the authorization the bundle already uses. But peer
credentials identify a *process*, not a person, and cannot distinguish a user from an
agent runtime running as them — which is the exact distinction §9.5's refusal of a
self-granted approval turns on. Reverse-proxy auth yields an actor; a uid does not.

And the choice was never exclusive. `net/http` serves on any `net.Listener`, so:

> **One protocol carrying the §8.0 envelope, two listeners.** A Unix socket for the
> local single-user case, where filesystem permissions are the right guard and no
> port or credential configuration should be needed to replace a `flock`; a TCP
> listener behind the proxy for the shared case. The same handler, the same command
> values, the same envelope. MCP, if an agent runtime becomes the primary caller, is
> a third listener over the same seam and not a competitor.

The socket listener has only peer credentials for an actor, so the escalated path
stays terminal-only there — which §4.6.2.1 already requires and which is now a
consequence rather than a separate rule.

**Apply is one call, and the "one or two" question dissolves.** `EffectApply` runs
every gate and writes under a single hold of the writer lock, so §9.4's guarantee is
structural and needs no second round trip. A preview is therefore *advisory by
definition*: it takes the lock, computes the diff, and promises nothing about a later
apply, which re-gates honestly.

> A caller that needs its preview to be binding sends the revision it previewed
> against, and the apply is refused when the corpus has moved. **Required** on the
> escalated path, where a person is necessarily in the middle; available to any
> caller that would rather have a refusal than a surprise.

That is §4.6.2's disjunction with its ambiguity removed. There is no ordinary
two-call flow to design, because the ordinary flow was never two calls.

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

### 5.0.1 What the Corpus Declines to Hold

Everything else in §5 says what a document *is*. Nothing said what does not belong, and
the failure mode of that silence is the one which cannot be caught later: **a corpus
restating what the code already says drifts invisibly, because both halves stay
internally consistent.** Neither is wrong on its own terms; they merely stop agreeing,
and nothing in either one can tell.

A surveyed wiki scopes itself by refusing to become "a second source of truth for
product behavior". Four exclusions, each stated as the reason rather than as a list of
topics, because a topic list ages and a reason does not.

- **What the code already states.** A retry budget is `3` because a constant says so.
  A claim asserting it is a copy with a slower update path, and §14.3's freshness
  machinery cannot help: the source of truth is not a URI anybody fetches. What belongs
  here is the *decision* — why 3, what was traded away — which the code cannot hold.
- **What a single run produced.** A log line, a benchmark number, one query's output.
  These are observations, and §10.7.4 puts observations in the per-user tier for a
  reason. A corpus of them is a corpus that cannot be wrong, because nothing it says is
  general enough to contradict.
- **What belongs to one person's working memory.** A note whose only reader is its
  author in the next hour. §1.1's requirement that a claim name its witness is not
  satisfied by "me, just now": the point of a witness is that somebody else can go back
  to it.
- **What the corpus would have to re-derive to keep true.** Anything whose truth is a
  function of something that moves faster than a person will re-check it. §14.3.2 makes
  drift visible; it does not make it affordable, and a page that is wrong more often
  than it is read is worse than an absent page because it is trusted.

**This section is convention, and says so.** §12.1's inversion holds: anything absent
from that table is convention by definition, and nothing here is in it. Whether a claim
restates the code is a judgement, and §17's refusal to score means gnosis will not
pretend a checker could make it. What a scope rule buys is not enforcement — it is that
a reviewer declining a page has something to point at, which is the same argument §6.2
makes for a threshold's rationale.

______________________________________________________________________

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

**The corpus is assumed to be in English, and the FTS5 tokenizer encodes that
assumption** (§5.5). `porter` stems English and `unicode61` splits on word
boundaries — which is close to useless for a language that does not put spaces
between words. A corpus taking in Chinese, Japanese, or Korean sources needs a
`trigram` tokenizer instead, giving up stemming to get substring matching.
This is stated because it is otherwise invisible: nothing fails, search simply
stops finding the material, and the tokenizer is fixed at table creation so the
repair is a migration rather than a flag.

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

#### 5.3.1 What a Foreign Consumer May Rely On

There is no export step, and stating why is the point of this section. A bundle is
a directory of markdown in a git repository; another tool consumes it by cloning
one. Nothing has to be produced, converted, or served first, and the design choice
that makes this true is already made elsewhere — the derived state lives in a
gitignored `.gnosis/`, so what a clone carries is exactly the portable part.

What a consumer lacks is not a capability but a boundary. Three tiers, and the
distinction is what each promises rather than where each file sits:

| Part                                                | Promise                                                                                                                                                                                                                        |
| --------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Markdown bodies and OKF §4.1 / §5 / §10 frontmatter | **A contract.** OKF v0.2, and a change that breaks a conformant reader is a breaking change to this bundle format                                                                                                              |
| `gnosis_`-prefixed keys, `evidence/`, `standards/`  | **Readable and unowned.** Stable enough to parse, documented here, and not promised across versions. OKF §11 already obliges a foreign consumer to preserve them without understanding them, which is the correct relationship |
| `.gnosis/`, including `index.db`                    | **Not for reading.** A per-user cache, rebuildable from the files above by construction (§12.0.1). A consumer parsing it has read a derived artifact and will be wrong the moment it is stale                                  |

**Conformance stays a boolean, and the alternative is refused on §6.2.** `akbp`
publishes a conformance *level*, which is a producer's instrument for stating
partial conformance — and there is none here to state. Required frontmatter is
`type` alone, §18.5.1 pins OKF §11 including the negative requirements, and a level
published today would always read *full*: a dial with one position. A level
structure defined by the only implementation certifies itself, and calling that a
grade is how an invented threshold acquires a number nobody can argue with later.

**The direction that would make grading useful is the opposite one.** A grade
describes a corpus *somebody else* produced — how far a foreign bundle conforms,
which `lint`'s `conformance` check already computes for this one. So the trigger is
not gnosis exporting, since export requires nothing; it is gnosis **consuming** a
second producer's bundle and needing to say how far that end conformed. That is the
same shape §12's `Skip{Check, Reason}` records: this codebase is the sole holder,
and the artifact earns its generality when there are two.

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

**A table preceded by `-- not built:` is specified and has no migration**, with the reason
on the marker. The distinction is load-bearing: this block presented ten tables as *the*
schema while the code had six, and nothing said which were which — the same gap `coverage`
had when it was specified, listed as enforced, and unbuilt. The marker is checked in both
directions against a fresh database, so a table that gains a migration and keeps its
marker fails, and so does one that loses its migration quietly.

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
-- lead is NULL until extraction writes one; title and description are NULL and are
-- not asked for at all. See 5.5.3.

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
-- not built: no reader. An alias resolves through ontology.toml, and a second
-- home for the same mapping is a second place for it to be wrong.
entity_aliases(entity_key, alias)              -- many names for one thing
claim_subjects(claim_id, subject_key, op, value_norm, value_raw,
                 dimension, derived, pattern_id)  -- derived=1 unless pinned
verifications(claim_id, by, at)             -- OKF §5.2 list; one row per event
-- not built: no reader. Frontmatter carries tags authoritatively and nothing
-- queries them yet; the row would be an index of something nobody looks up.
tags(document_id, tag)
-- not built: overlaps sources_fetched, which records one row per fetch rather
-- than per claim. Which grain a reader wants is undecided, and building both
-- would make the two disagree.
sources(claim_id, source_id, resource, title, author,
        usage_count, last_modified, window_from, window_to)
-- `pos` here is an offset into the ARCHIVED SOURCE, not into the document — a
-- different coordinate space from claims.pos. See 5.5.2.
-- not built: frontmatter already holds the quotations authoritatively (§9.4) and
-- `durability` needs §14.4's classes, which do not exist. Indexing it now would
-- be a derived copy of the authority with a column nothing can fill.
evidence(claim_id, pos, quote, source_id, archive_path, fold_hash, durability)

-- A link keeps what the author wrote even when it resolves to nothing.
-- snippet_start/end are byte offsets into the document BODY, the same space as
-- claims.pos. See 5.5.2.
links(id PK, source_claim_id, target_document_id, href, title,
      rel, external, snippet, snippet_start, snippet_end)

claims_fts USING fts5(
  title, description, lead,
  content = claims, content_rowid = id,
  tokenize = "porter unicode61 remove_diacritics 1 tokenchars '''&/'")

-- One row per fetched source. source_sha256 is always recorded; archive_path is
-- NULL for `referenced`. extracted_from links an extraction to its origin.
-- One row per committed fetch record (4.3.1), keyed by the record's own hash.
-- Derived: `index rebuild` reproduces every row from evidence/fetch/.
sources_fetched(record_sha256 PK, uri, source_sha256, byte_size, media_type,
                disposition, archive_path, extractor, extractor_version,
                extracted_from, reject_reason)

-- When this user last verified a source against upstream. Per-user and NOT
-- committed, because it is an observation rather than a decision (4.3.1).
-- not built: and it never will be as a table. This is `.gnosis/checked.jsonl`
-- (4.3.1), and the row is a leftover from before that decision. Kept and marked
-- rather than deleted, because the state it names is real and a reader who goes
-- looking for it should be sent to the file rather than find nothing.
checked(uri, source_sha256, checked_at, PRIMARY KEY (uri, source_sha256))
-- not built: the reply cache is content-addressed on disk, keyed the way
-- internal/relay computes it. A table would be a second index of files whose
-- names already are the key.
llm_cache(cache_key PK, source_hash, prompt_hash, model, model_version,
          response, created_at)
-- not built: `lint` computes findings on every run and holds none, which is
-- 10.7.4's own conclusion arriving one table earlier. The rows that would need
-- durable state are challenges, and those live in frontmatter and travel.
findings(id PK, kind, severity, category, action, state,
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
  **It is keyed by claim, and that required a decision OKF does not make.** OKF puts
  `verified` at document level; this table is claim-keyed; and a writer that expanded
  one into the other would assert that somebody verified each claim when they verified
  a page. §5.5.1 refused exactly that inheritance for `subject` — an edited default
  silently re-subjects every claim that did not override — and the reason this is a
  table at all is that two kinds of sign-off must stay apart, which a page-level
  expansion destroys one level down. So `verified` is read **inside a `gnosis_claims`
  entry** and a document-level list is not expanded. A bare actor with no timestamp is
  kept: OKF §11 says tolerate it, and the actor is the half §14.1's fold reads, so
  dropping the event would lower a concept's tier because somebody omitted a date.
  **Four tables in this block have no migration**: `entity_aliases`, `tags`, `sources`
  and `evidence`. `verifications` was the fifth until 2026-08-27. They are specified and
  absent, which is recorded in `TODO.md` rather than left for a reader to discover by
  querying one.
- `entity_aliases` carries the surface-term-versus-canonical-thing problem, and
  it hangs off an entity rather than a claim. Many names for one *thing* is a
  property of the thing; "a claim with two names" is close to meaningless. `tags`
  hangs off the document for the same reason — a reader tags what they are
  browsing, not one assertion inside it.
- `evidence.pos` records where a quote sits **in the archived source**, so a
  reader can be sent to it. It does not share a coordinate space with
  `claims.pos`, which is why §5.5.2 states both.
- **`sources_fetched` is keyed by the record hash, not by the source hash.** A
  source hash key would permit one row per set of bytes, so the same document
  reached through two URIs — a mirror, a moved page — could not be recorded twice,
  and the uri-to-hash mapping is most of what the ledger is for. An earlier draft
  keyed it `(uri, sha256)` and said that was so "a changed source appends a
  version"; that reasoning rules out `uri` alone and settles nothing about the rest,
  which is how the key came to look decided while being underspecified.
- **`checked` is the only table here that is not reconstructible from the bundle**,
  and it is the exception §4.5 does not cover: it records what this user observed
  about the outside world, which no rebuild can re-derive because the observation
  is gone. It is therefore per-user by necessity rather than by choice, and losing
  it degrades freshness to `unknown` — which is the honest state for a clone that
  has never looked.
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
    subject: cache.default_enabled        # optional; a key from ontology.toml
  - id: 01932b7c-2a03-7b11-8e44-9f10c2d3e4f5
    anchor: it is not shared across sessions
    subject: cache.session_scope
```

##### `subject` Is Declared per Claim, and This Corrects an Earlier Table

An earlier draft of §5.4 listed `gnosis_subject` among the **document's** frontmatter
keys, glossed "what this claim is about". Everything downstream is per claim —
`claim_subjects`' primary key is `(claim_id, subject_key)`, §5.8.3's review signal is
phrased over a claim, and §10.2 pairs claims — so the document-level entry was a
drafting slip and is corrected here rather than preserved.

Three reasons the grain is the claim, in the order they decide it:

- **A document's claims may bound different subjects, and that is the ordinary case.**
  The example above is one sentence split into two claims; those two constrain different
  things. A single document-level key cannot express it, and the corpus's most common
  page — a reference covering several properties of one system — is exactly the shape
  that breaks.
- **A claim's meaning must be recoverable from the claim.** That is this section's own
  requirement for identity and address, and a subject is what a comparison is *about*;
  reading it from a key one level up would make the claim's meaning depend on context the
  claim does not carry.
- **An inherited default was considered and refused.** Allowing a document-level key that
  claims inherit unless they override is terser, and it fails in the way §5.8.2.1 is
  about: editing the document's key silently re-subjects every claim that did not
  override. That is definition drift arriving through a convenience, and it would be
  invisible at the point of use — a reader of the second claim would have to look
  upwards to learn what it is about.

The correction costs nothing to make now and more later: **nothing reads the key yet**,
so no bundle can be relying on the document-level form.

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

#### 5.5.1.1 Documents Record the Conventions They Were Written Under

A document carries `gnosis_schema_version`, an integer, and the index mirrors it.

This is a fourth kind of drift, distinct from the three already specified, and
naming it is the point: `stale_after` and archive-versus-upstream are the *source*
changing (§14.3), `index-drift` is the *cache* falling behind the bundle (§12), and
`schema-shape` is the *database* not matching its migrations (§12). This one is the
**corpus's own conventions changing underneath documents that already exist**.

It is not hypothetical, and the first instance is already scheduled. §5.5.1 requires
`gnosis_claims` frontmatter, and no document written before extraction lands
carries it — so on that day every existing document predates the format, and
without a version there is no way to tell those from documents that should have
claims and are missing them. The same applies to any later change in required
frontmatter or in what `ontology.toml` declares.

Three properties keep this from becoming a migration treadmill:

- **It reports; it does not rewrite.** `lint`'s `schema-version` check names
  documents written under an older version. What to do about one is a decision per
  change, because some convention changes are worth backfilling and most are not.
- **An absent version means "before versioning", not zero.** The distinction
  matters for exactly the documents this exists to find.
- **The version advances only when a change makes old documents wrong**, not on
  every edit to this specification. A new optional key does not; a new required one
  does.
- **The check is skipped until the corpus starts versioning.** Until some document
  declares a version, none do, and reporting the whole corpus on the day versioning
  is introduced would teach a reader to ignore the check before it ever said
  anything useful. It activates on the first versioned document and then finds
  exactly the ones left behind — the derived applicability of §12, applied to the
  one case where "nothing is versioned yet" and "everything is out of date" look
  identical.

#### 5.5.1.2 an Untyped Link Asserts Nothing, and Order Is Not Causality

`links` carries a `rel` column and Phase 1 populates it with nothing. That is the
correct starting state — an untyped link cannot lie about a relationship it does not
name — but the consequence has to be written down, because an empty column is exactly
the kind of thing a later reader fills in with an assumption.

**An empty `rel` means the corpus does not know what the connection is.** It does not
mean "relates to", "supports", or "see also". A document citing a source it disputes,
a document superseding another, a document listing another for contrast, and a
document that merely mentions another all produce the same row. Any check, any view,
and any prompt that treats an untyped link as endorsement is reading a claim that was
never made.

**Nor does arrangement carry a claim.** The order links appear in a body, the order
rows come back from a query, and the shape of the resulting graph are all *layout*.
They are not a causal, temporal, or dependency order, and they become one only when
something states it. This is why §20's deferred trail is a document whose prose says
why the order is what it is rather than an ordered table: the prose is the claim, and
without it the list is a list. The rule generalises — **causality is carried as a
claim, never inferred from arrangement** — and it is the reason `gnosis graph` reports
structure and never explanation.

Typing the vocabulary is Phase 3 work and is blocked on the same problem as §5.8's
subjects: a relation vocabulary admitted before the corpus can adjudicate over it is
a vocabulary that will be used inconsistently and then relied on. Until then the
column stays empty and this section is what an empty column means. Recorded in the
manifesto's `agent-green` survey, where three unrelated projects reached the same
conclusion from three directions.

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

#### 5.5.3 a Claim Row Is an Address; Its Summary Is Separate

`claims` holds two unlike things and the difference decides when a row may be
written. Five columns are an **address** — `id`, `document_id`, `anchor_hash`,
`pos`, `type` — and §5.5.1 requires every one to be recoverable from the document
alone. Three are a **summary**: `title`, `description` and `lead`, which extraction
supplies (§10.2.1). The address is derivable today; the summary is not.

**So the rows are written at index-rebuild time, with the summary NULL.** A row
whose address is complete asserts exactly what it can prove — where this claim is
and what type of document holds it — and nothing about what the claim says.

**NULL rather than the empty string, and this is not a preference.** `''` is a
value: it asserts that the claim *has* no lead, which is false, and §17.4's `lead`
check cannot distinguish it from an author who wrote an empty one. Under the empty
default that check fires on every claim in the corpus the first time it runs — a
signal at its loudest when there is least to say, which this specification has now
recorded four times in messages and would here be recording in a schema. `claims.pos`
is nullable for the identical reason one column over: zero is a real position, and
the empty string is a real lead.

**And `claims_fts` stays unpopulated until the summary exists.** It indexes those
three columns as external content, so populating it from rows whose summaries are
unwritten produces a searchable corpus of blanks — claim-level search returning
fewer results than there are claims, with nothing saying why. That is §12's
distinction between *examined and clean* and *not yet examinable*, arriving in
`search` rather than in `lint`: the claim ladder skips with a reason (§11.3) until
extraction has written something to match on.

**The change is a migration, and the paragraph that said otherwise was wrong.** An
earlier draft here claimed the shipped migration could be corrected in place because
`schema-shape` would report the mismatch. It would not: `DB.Objects` selects
`name FROM sqlite_master`, so that check compares object **names** and is blind to a
column definition. An edited migration leaves every existing index carrying the old
constraint, and the first NULL insert fails at runtime as a constraint error nothing
warned about.

So the change is appended, and it drops and recreates the table. That is safe here and
nowhere else: nothing has ever written `claims`, so there are no rows to preserve.
`claims_fts` is recreated with it, because an external-content index references its
content table by name and a dropped table leaves the index pointing at nothing.

**When extraction lands, the state inverts.** A NULL summary on an extracted claim
stops being the ordinary case and becomes a finding — the same inversion `Freshness`
performs the first time a source is checked.

**Only `lead` is asked for, and the other two are refused rather than deferred.**
Extraction supplies a lead as of 2026-08-27. `title` and `description` stay NULL and the
reply format does not request them, on three grounds:

- **Nothing reads them.** No query selects either column. §5.5.3's argument was that a
  NULL summary is honest where an empty string is not; a *populated* column nothing reads
  is worse than either, because it looks like a capability.
- **`lead` already is the retrieval unit**, by §17.4's own argument — "the unit of
  comparison in §10's conflict predicates and the unit of retrieval in §11". A claim
  carries its assertion in `anchor` and its conclusion in `lead`; `title` overlaps the
  second and `description` overlaps the first.
- **Asking costs a full re-extraction of every bundle** (§6.1.1), because the prompt is
  part of the cache key. Two more fields is a second one, paid by everybody, for two
  columns nothing selects.

§11.1's "title, description" ladder is about `index.md` metadata at *document* grain.
Copying it one level down is a pattern applied rather than a need identified.

**The trigger that would change this**: a claim-level search result a reader cannot
identify from its lead alone. That is observable, and the columns remain in the schema so
that adding them later is a migration nobody has to design.

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

#### 5.7.1 the Marker Contract, in Four Rules

"Hand-written sections are preserved between markers" is one clause above and four
rules in practice. A generated region is the text between
`<!-- gnosis:begin NAME -->` and `<!-- gnosis:end NAME -->`, and:

1. **A generated region is replaced.** The markers stay, so the next run finds it.
2. **Everything outside every marker is preserved byte for byte.** Not re-rendered,
   not re-wrapped, not normalised. A person's prose is not the generator's to tidy,
   and a formatter that "improved" it would be the silent rewrite this contract exists
   to prevent — while looking like an improvement in the diff.
3. **A file carrying no markers is never overwritten.** The generated text goes to
   `AGENTS.generated.md` instead. This is the fail-closed rule: a file predating the
   tool was not written under its contract, so treating its existence as consent is
   fail-open — and the ETH Zurich finding above is what that would cost.
4. **A marker that opens and never closes is a refusal.** Reading an unterminated
   marker as "everything to the end of the file" would let one typo hand a whole
   document to the generator. The extent of the region is unknown, so nothing is
   written and the diagnostic names the marker.

Two consequences worth stating because they are choices rather than mechanics.

**A region `gnosis` generates that a file does not carry is left absent, never
appended.** Where a region belongs in somebody's document is their decision, and a
generator that inserted one would do it on every run — so a person who deleted a
region they did not want would get it back.

**Only two regions exist: `vocabulary` and `commands`.** The restraint is the
feature. A `workflow` region explaining how to ingest would be exactly the prose the
ETH Zurich study measured, and the command list is read from the registered command
tree rather than maintained by hand, so it is the binary describing itself.

A deprecated type is rendered **as deprecated** rather than omitted. §5.8.1's
soft-deprecation is announce-then-enforce, and a vocabulary listing that silently
dropped a deprecated key would enforce without announcing: an author would find
documents rejected for using a word the schema had stopped mentioning.

##### Where the Contract Applies, and Where It Is Refused

§6.3 splits accretion from synthesis at the level of the *operation*. The finer split is
at the level of the **region** — a refresh rewrites what the machine wrote and leaves
what a person wrote alone — and what blocked it was a way to mark regions. This is that
way, and the question of which documents adopt it has one answer per kind.

**Reserved files at the bundle root adopt it, and there are two.** `AGENTS.md` carries
the vocabulary and the command list; `index.md` carries a listing of the corpus grouped
by type, beside the part a person curates — "a handful of paths through the corpus that a
newcomer actually needs". Both are maintained by `gnosis schema`, because the contract is
a property of the class rather than of either document, and a second command would
re-implement the fail-closed rule, the sibling file, the unterminated-marker refusal and
`--check`.

**The seeded prose used to argue against the listing, and it was half right.** It said
"keep it a map, not a mirror. A generated list of every document is available from
`gnosis search` and `gnosis graph`." That is right about what a *person* writes there and
wrong about where the derived list belongs: §5.7's whole argument is that an agent reads
**files**, so a listing reachable only by running a command is not reachable by the
reader this document exists for. OKF §8's progressive disclosure is the same argument.
The generated region is the mirror, so the curated prose does not have to be.

**`init` generates it rather than seeding it**, which changed once the region existed.
Seeding prose with no markers made every later `gnosis schema` report the file as
unmarked and exit with findings — on every bundle, from the day it was created — and a
scaffolded copy of generated text is stale the moment anybody adds a document.

**Concept documents do not adopt it, and that is now a decision rather than a
deferral.** The boundary is not editorial; it is measured. `bundle.Load` walks `c/` only
and skips reserved names, so a root-level file is never parsed as a concept, never
indexed for full text, never anchor-matched and never segmented. **A marker there costs
nothing.** In `c/` the same marker is read as prose by four things — the FTS body,
`Snippet`, `segment.Claims`, and the fold that `claim-anchor` searches — so adopting it
would mean teaching each of them to ignore a comment.

Three reasons, in the order that decides it:

- **It buys protection for the least defensible content in the document.** A concept
  body is rendered from an admitted reply: every paragraph is a quotation-backed claim
  with an id and an anchor. Hand-written prose inside one carries none of those, and
  §5.0.1 already says the corpus declines to hold what belongs to one person's working
  memory.
- **§5.3 says the bundle format is OKF, unextended where possible**, and frontmatter is
  where `gnosis` extends. A marker in a *body* extends the document format itself.
  `AGENTS.md` is not an OKF concept, so the contract there extends nothing.
- **§6.3's gate already protects the thing worth protecting**, and protects it better. A
  region preserves paragraphs; `synthesize` preserves *evidence* — every prior quotation
  still validating, no entry silently dropped, the diff reported before the write. If a
  refresh must not lose something, that something is a validated quotation.

**The trigger to revisit**, so this is not re-litigated on taste: a real `synthesize`
diff that a reviewer cannot read — one where a preserved paragraph is
indistinguishable from a rewritten one. That would be evidence regions are needed. The
prediction is that it will not arise, because a diff whose evidence must survive it is a
small diff.

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
episodic = false                                # default; see §5.8.3.1
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
owner = "team:privacy"                                  # optional; accountability, never authority (§10.6.2.1)
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

**A vocabulary entry MUST also record the aliases it refused, and why.** An
`aliases` list says what resolves; a `rejected` list says what was proposed,
considered, and declined, with one sentence of reason each:

```toml
[[subjects]]
key     = "retry.max_attempts"
aliases = ["retry budget", "retry cap"]

  [[subjects.rejected]]
  alias  = "retry policy"
  reason = "covers backoff and jitter too; a claim bounding one is not bounding the others"
```

This is the required-rationale discipline of §6.2 applied to vocabulary, and it
exists for the same reason. A refused alias is exactly the knowledge that gets
re-litigated: somebody proposes `retry policy` again in six months, the person who
knows it was refused for covering three distinct constraints is not in the room, and
the corpus has no way to say. Recording only what matched keeps the *conclusion* and
throws away the *reasoning*, which is the failure mode this whole specification is
organised against — and it is worse here than elsewhere, because §5.8.2.1 makes an
alias exclusive, so admitting one wrongly forecloses a key that another group needed.

Rejections are also what make §5.8.4's evolution honest. A vocabulary that only ever
grows records every decision to admit and none to decline, so its history reads as
though nothing was ever hard.

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
`subject`, is **reported for review** — never blocked, never assigned a
subject automatically. Blocking would make the corpus refuse ordinary knowledge;
guessing would put an inferred key underneath a comparison gate, which §10.3
refuses on principle. Reporting puts it in front of a person, which is where the
judgment belongs.

This is deliberately a *review* signal rather than a defect. Many claims of a
normative type legitimately constrain nothing, and the check earns its place by
being cheap to dismiss.

#### 5.8.3.2 the per-Subject Population, and What It Is For

`gnosis audit --subjects` reports, per declared key: how many claims rest on it, across
how many documents, and **which surface phrases authors actually wrote**.

It is an instrument rather than a report of problems, and it exists because the detector
it serves cannot be built yet. §5.8.2.1's collision rule fires only when somebody
*declares* a colliding alias; it cannot fire when two groups have been using one word
differently and neither declared it, which is the ordinary way the problem arises.
Detecting that needs a threshold — how bimodal, how many new surfaces — and §6.2 forbids
inventing one. This produces the population a threshold could be calibrated against.

**One column is a signal rather than a count.** A key is marked when two documents
carrying claims on it cite no archived file in common: two internally consistent halves
with nothing comparing them, which is the shape the silent-drift failure actually takes.
It is observable with no threshold, which is why it is the recorded trigger. It is not a
finding — two teams often read different documentation about one thing.

A document citing nothing is excluded from that comparison. Two empty evidence sets are
disjoint by the letter of it, and counting them would make the condition true of every
hand-written corpus, which is indistinguishable from never firing at all.

**No coverage figure and no score.** §17 forbids presenting a count as health, and a
population is the most tempting such count there is: it looks like coverage and it can
be raised by declaring subjects nobody uses. The report exits `ok` whatever it finds.

#### 5.8.3.1 a Type Declares What the Knowledge Is; Behaviour Is Derived

`normative` and `expects_subject` say what a kind of knowledge *is* — does it prescribe,
does it bound something — and each drives a check. `episodic` is the third of that kind
and the reason for stating the pattern: **its claims assert what happened at a moment,
not what holds in general.**

Three behaviours follow, and none of them is declared:

- **The staleness window does not apply** (§14.3). An episode's evidence is a commit
  hash, immutable by construction, so "its sources were last verified 40 days ago;
  re-run `gnosis fetch` on them" is advice that can never be satisfied. This one is live
  today, independently of §10 — and it exempts **half** the check. A declared
  `stale_after` still applies, because that date is the author's own statement about
  their claim and a person may legitimately ask for an episode to be revisited. The
  exemption is about evidence that cannot change, not about silencing whoever asked.
- **Its claims are ineligible for the interval predicate** (§10.2). *"We set the retry
  budget to 3 in March"* and *"we set it to 5 in June"* present to that predicate as one
  subject with disjoint values, and adjudicating that would be the corpus adjudicating
  its own history. Two reports of different moments cannot contradict. The exclusion is
  **per claim, not per pair**: an episode records what happened and a rule states what
  holds, so pairing them would ask a reader to adjudicate a fact against a policy.
  **It does not extend to evidence divergence** (§10.2's second built predicate), and the boundary is what this
  bullet argues: ineligibility is about *adjudication between claims*. An episode resting
  on two versions of one source still has evidence that moved underneath it, and
  suppressing that would let "this is history" excuse an unsupported claim.
- **Supersession therefore never fires** (§10.4) — not by prohibition, but because §10.4
  deprecates the loser of an *adjudicated conflict* and there is never a conflict to
  adjudicate. "An episode is not superseded by a later episode" is a consequence rather
  than a rule.

**A `supersedable` flag was the obvious alternative and is refused.** It would put
*policy* in a vocabulary file: the existing flags describe the knowledge, that one would
describe what the tool may do to it — a permission, which §14.1 refuses for trust tiers
and which fails here the same way. Worse, the ontology is per-corpus and editable, so a
bundle could mark `Rule` unsupersedable and disable §10.4's central mechanism by editing
a data file, with no check able to tell that from a legitimate vocabulary choice. That is
§6.2's concealed-loosening hazard arriving in the ontology instead of in `standards/`.

Deriving it from `normative` was also considered and does not work: `normative = false`
covers `Reference`, "a recorded fact with no prescriptive force", which is emphatically
supersedable because a fact can be corrected. The flag does not discriminate.

**Reported, never refused.** A hand-deprecated episode is reported and not undone,
following §5.8.3, §10.6.2 and §14.1: the corpus notices, it does not police.

**No `Episode` type ships in the starter vocabulary yet**, and that is §10.6's
attenuation argument rather than an oversight — "declining to admit a vocabulary the
corpus cannot yet adjudicate over is attenuation, not procrastination." The flag and its
staleness derivation are useful now; the type is worth adding when a corpus has an
episode to file, because a starter entry nothing uses is a dead knob in a different file.

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
recorded in `log.md` alongside the finding count before and after **where such a
count exists** — which, on inspection, is fewer thresholds than this section
originally assumed.

Tracing which values actually reach a finding, recounted 2026-08-23 because the
earlier count was five short and one of its categories had gone wrong. There are
**twelve declared values across three files**, of which nine can be reported as
loosened — the extractor pair has no direction and the draw seed is not a loosening
at all (§6.2.1). They fall in four categories:

- **A countable delta.** `corpus_budget` and `corpus_warn_fraction` feed the
  archive-budget diagnostic, and `staleness_days` feeds the `stale` check's window.
  Raising any of the three can silence a real finding and the delta is exact — the
  corpus is read once and the check run twice.
- **A gate verdict and no finding.** `per_file_cap` and `embedded_payload_cap`, since
  §9.3 stage 4 applies them to a candidate document. This category did not exist
  until stage 4 did, and its absence is what made the previous sentence about them
  half true.
- **Admission only.** The allowlist changes which sources archive and what any check
  reports is unaffected. So do `hedging_max` and `rebuild_floor_fraction`, one file
  over, for the promote gate and the rebuild floor.
- **Read by nothing.** `in_degree_cut`, which §6.5.1 requires to be *reported* as
  such rather than assumed.

`staleness_days` is the one worth naming, because the tool had it wrong. It was
classified as read by nothing — true when written, false once the `stale` check
gained its window — so **widening the staleness window silenced `stale` findings and
`standards check --log` recorded that it cost nothing.** That is the precise
reassurance this section exists to withhold, produced by the mechanism this section
asked for, and it survived because the classification lived in one switch and the
truth lived in another. The classifier is now cross-checked against
`standards.Unread` by a test, which is the function that already knows.

`gnosis standards check` states which case a loosening is in rather than printing a
zero delta — a zero reads as *this cost nothing*, when what happened is that nothing
measured it, and that is precisely the reassurance this section exists to withhold. This is not
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

#### 6.2.2 the Seed Is Not Scaffolded, and the Loosening That Escapes

`init` writes `ontology.toml`, `index.md`, `log.md` and `.gitignore`. It does not
write `standards/`, and an absent file falls back to the embedded seed. **That is
the answer and it is deliberate**: the seed is a live default, so a corrected
threshold reaches every existing corpus, where a scaffolded copy would freeze each
bundle at whatever the values were on the day it was created. §6.2's mechanism does
not depend on scaffolding either — a corpus's first edit is diffed against the seed
and reported like any other change.

So `standards/` is policy with a defensible default, not a knob a corpus is expected
to turn. One of its files is the exception and is not what this says: retrieval cases
(§11.0.2) are queries about a particular corpus's content, ship empty, and have no
default to improve.

**The fallback has one hole, and it is not the one scaffolding would fix.** A value's
prior reading comes from the file at a git revision, falling back to the seed when
the file was not there — and that fallback is the *running binary's* seed, which is
the only one it can reach. For a corpus that has never edited the file, both readings
are therefore the same values, and nothing can be reported. A release that loosens a
seed changes the effective gates in every such corpus with no report and no `log.md`
entry: §6.2's requirement, bypassed by an install rather than by a commit.

Two rules close it where the decision is actually made:

- **A seed loosening is a §6.2 event in `gnosis`'s own `log.md`**, with the finding
  counts, because `gnosis` is itself a bundle and that is where somebody chose the
  new number. Recording it per corpus would demand the count from readers who did not
  make the change.
- **`doctor` names the source of each gate set** — the bundle's file, or the seed and
  the version it came from — so a reader who wants that entry can find it.

Both are reporting, not stored state. A recorded fingerprint per bundle would detect
the change more precisely and would need a writer, and a field with no writer is the
failure §4.6.2.1 records.

**Revisit when a corpus edits three of the four files.** One edit is a corrected
default and says nothing; three is tuning having become an expectation, which is the
condition under which scaffolding starts being worth its cost.

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

#### 6.3.0 What Each Half Is Allowed to Change

The split is enforced by what each operation may touch, not by which command a person
reaches for.

**Accretion may not change a body, and that is checked.** A reply about an existing
concept has its quotations appended to the claims the document *already makes*; a claim
the document does not make gets no evidence, because it has no paragraph and adding one
would rewrite the body. Those claims are reported by name rather than counted — one may
be a paraphrase of something the page already says and another may be knowledge it does
not hold, and the remedies differ. After the merge the rendered body is compared against
the one that was read, so the invariant is verified rather than promised.

**Synthesis may change anything it keeps the evidence for.** The gate is a **set
comparison over quotations**, never a count: a rewrite that dropped one passage and
added another would balance, and the arithmetic would approve a document that lost the
passage its claim rested on. What went missing sits in frontmatter, where a reader
skimming a diff of improved prose will not see it.

**Both refuse a reply computed against bytes that have moved.** The document's hash is
recorded when the prompt is emitted and compared when the reply arrives. Between those
two moments somebody can edit the page or another reply can land, and applying an answer
to a document that changed underneath it is §9.4's approved-diff window one level up.

**A rewrite is checked against every source the document cites**, not one. A concept
rests on several where a source prompt rests on one, and checking a rewrite against only
the first would refuse evidence the corpus already holds — the gate reporting a loss it
caused itself.

#### 6.3.1 a Concept Body Is the Machine's; a Person Contributes Elsewhere

This is not a restriction being introduced. It is what `renderQuarantined`,
`claim-anchor` and §9.4 already enforce between them, never stated in one place — and
stating it is what turns "we gate the whole rewrite" from an admission into a design.

A concept document's body is rendered from an admitted reply: a title, and one
quotation-backed claim per paragraph, each with an identifier and an anchor. Prose typed
into it afterwards has no claim id, no anchor, and no evidence; if it displaces an
anchored span, `claim-anchor` reports the address as lost and never repairs it.

So a person's contribution has four sanctioned homes, and each holds something the claim
page cannot:

- **`gnosis_warrant.rationale`** — why a decision went the way it did (§10.6.4).
- **`log.md`** — what changed and the reasoning the diff does not show (§6.2, OKF §9).
- **A linked document**, typically a `Decision` — reasoning that is itself knowledge, and
  which then gets an identifier, a place in the link graph, and the same gate as anything
  else. This is the answer most often wanted, and it is what the corpus is *for*.
- **The relay** — a claim, with a quotation, through `admit`.

The rule is what makes §5.7.1's refusal of body markers coherent: there is no
human-owned region in a concept body to protect, because a body is not where a person
writes. What `synthesize` must not lose is evidence, which is why its gate is stated over
quotations rather than over paragraphs.

#### 6.1.1 a Prompt Change Costs Every Cached Reply

The cache key hashes the prompt body (§6.1), so **any change to the prompt text moves
every key in every bundle**. Every prompt is re-emitted and every reply is re-asked.

That is the mechanism working rather than failing: a changed prompt is a changed
question, and serving the old answer would be answering something nobody asked. But
§6.1's promise — "a second run over unchanged inputs makes no model calls" — is about
unchanged *inputs*, and the prompt is one. The cost is real and it is paid by everybody
who has a bundle, so it belongs beside the promise rather than in the commit that first
incurs it.

**Two consequences for anyone editing the prompt.** Batch the changes: three edits in
three releases cost three re-extractions, and three edits in one cost one. And say so in
`log.md`, because a corpus that suddenly wants to re-answer every prompt is otherwise
indistinguishable from a corpus whose cache is broken.

Adding §17.4's `lead` to the reply format on 2026-08-27 is the first change to have paid
this.

### 6.4 Miss Log

`virgil`'s `internal/router/misslog.go` is the mechanism that makes "minimize
the model" measurable rather than aspirational. Its `MissEntry` records
`{Signal, KeywordsFound, KeywordsNotFound, FallbackPipe, AIPlan, AIConfidence}`
— and `KeywordsNotFound` is the load-bearing field, because it says *why* the
deterministic layers missed.

This file, the derived `fetch.jsonl` rollup, and `audit.jsonl` are **tracers**:
instrumentation added
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

**Built 2026-09-03, and two things it settled.**

**The reason carries two classes, because "a recurring reason is a check waiting to be
written" is true of one of them and false of the other.** Extraction has no
deterministic path — §19 records why a deterministic phase cannot say where one claim
ends — so `no_deterministic_path` recurs for as long as the corpus ingests anything and
names no work. `no_deterministic_predicate` is the backlog: a path ran and decided
nothing, which today means the critic (§17.1's question is not one any check answers)
and will mean conflict adjudication with `checks_run` non-empty and `checks_fired`
empty. `MissReason.Actionable` is the property, so the distinction lives on the value
rather than in a rule each emitter remembers, and the actionable groups sort first — by
count alone the unactionable line tops every run.

**A count of rows is not a count of consultations**, which running it showed. Two
`ingest` runs over one unanswered source write the same prompt twice, because nothing is
cached to skip; the log rightly holds two rows and the report counts one *question*, by
prompt key. Both numbers are reported: the gap is the times somebody re-ran a prompt
nobody had answered, which is a fact about how the relay is used rather than about the
corpus. A cache hit writes no row at all — it asked nothing.

#### 6.4.1 a Miss Log Measures Coverage, Never Correctness

Stated here because the log is cited elsewhere as evidence for things it cannot
supply, and a tracer whose limits are unwritten gets over-read.

**A miss is a non-event that fired.** The log answers *how often did the deterministic
path decline to answer*, and that is a coverage measure. It cannot answer *how often
did the deterministic path answer wrongly*, because a wrong answer produces no
fallback, no row, and no trace. The two questions have opposite shapes: coverage is
observable from inside the path, and correctness is not observable without a ground
truth the path does not have.

The consequence is a rule about what may be concluded from a rising hit rate. *"Ninety
percent of queries were answered before step 5"* is true and is a statement about
**reach**. It is not a statement about accuracy, and the difference is not pedantic:
a retrieval path that confidently returns the wrong concept every time has a perfect
miss-log record. §11.0.2 specifies the labelled case set that measures the other half,
and until it exists the honest form of the determinism claim is *the model was
consulted this rarely*, never *the deterministic layers were this good*.

The same limit applies to `fetch.jsonl` and to `audit.jsonl`, and for the same reason:
all three are tracers over actions taken. **A tracer records what happened, and its
silence is not a claim that nothing should have.**

### 6.5 Standards as Data, Not Code

`AgentLint/standards/` is the model to copy: `weights.json`,
`reference-thresholds.json`, and `evidence.json` joined on a check ID, where each
of 58 checks carries `{dimension, name, scope, fix_type, evidence_sources, evidence_text}` over a source registry that grades its own citations
`primary-data` / `peer-reviewed` / `case-study` / `industry-practice` — and
annotates the weak one honestly ("n=1 case study, useful reference point not
universal benchmark").

`gnosis/standards/` carries the same content, **as TOML rather than JSON**, plus
`operators.toml` — the comparison-operator patterns used to derive constraints
from prose (§10.2.2), as data with their own test corpus rather than regexes in
Go. Operator inversions are the first cases in that corpus, because *"no fewer
than three"* and *"should not exceed three"* turn on a word naive patterns miss,
and a wrong operator produces a false conflict.

The format follows §5.2's rule and not AgentLint's example: these are files a
human edits to change behaviour, and `toml.Decode` reports the keys it did not
consume, so a mistyped threshold is caught by name rather than silently ignored.
A JSON config cannot distinguish a typo from a key it was not built to read, and
a threshold somebody believes they changed is the expensive failure here.

**Each value carries its rationale in the same structure as the value**, not in a
sibling file joined on a key. AgentLint's three-file join is the part deliberately
not copied: a rationale reachable only by joining is a rationale that can go
missing without anything failing, and this one is required at load. See §6.2.

The files carry: candidate rank cuts, hop limits, `MinPassageWords`, staleness
defaults, the archive gates, and the promote gate's signals. Two consequences:

- A threshold change is a reviewable diff carrying its own justification.
- The standards files are hash-pinned into every finding and every audit row, so
  a verdict is inseparable from the configuration that produced it.

#### 6.5.1 a Value Nothing Reads

A threshold declared, justified, validated, and read by no code is the failure
this section's own machinery makes easy to miss. Every guard here checks that a
value is *present* and *defended*; none checks that it *does* anything. Two of the
first eleven values sat that way through two phases — `staleness_days` and
`hedging_max` — each with a paragraph of reasoning behind a number that changed
nothing.

Nothing at runtime can discover which values are branched on, so gnosis records it
statically, in Go, and one test asserts the recorded set. Three states, and the
third is the one a two-state design gets wrong:

| State    | Meaning                                                                                                                                                                                          |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| consumed | some code path branches on the number                                                                                                                                                            |
| pinned   | the value must equal a constant compiled into gnosis; it is recorded onto every artifact produced under it, and editing it in a bundle changes nothing except which gnosis will load that bundle |
| unread   | nothing reads it                                                                                                                                                                                 |

`html_extractor` and `html_extractor_version` are pinned, and calling them either
of the other two would mislead: *consumed* tells a reader their edit takes effect,
*unread* invites deleting the provenance every extracted record carries.

**`doctor` reports the narrow case, not the general one.** The general fact —
gnosis declares a knob it does not read — is a property of the binary, identical
for every corpus, actionable by nobody holding one, and not even fixable by
deletion, since the loader then rejects the file for the missing rationale. A
diagnostic like that is noise, and the first implementation emitted it on every
freshly initialised bundle before a test caught it. What belongs to a corpus is
narrower and genuinely useful: **somebody edited a number here and got nothing for
it.** So `doctor` reports a value tuned off the seed that nothing reads, and a
value pinned to something this binary does not implement. The general fact belongs
in gnosis's own tests, where its only possible audience reads.

`in_degree_cut` is the current unread value, and it stays unread deliberately.
§14.4.1 wants it for the conjunction *unprovable AND load-bearing*, and
`unprovable` is Phase 3, so the cut has nothing to narrow yet. Giving it a reader
that classified bare centrality would be a different feature wearing the same
number, and would make the value look consumed while the thing it was declared for
stayed unbuilt.

Their thresholds file also states this repo's charter better than the manifesto
does, and its header is adopted verbatim as the first key:

> Reference values from empirical data. NOT enforced thresholds — measures and
> compares, users decide.

______________________________________________________________________

**The classification is checked against the source, and the check it replaced was
not evidence.** Recording what reads a value cannot be discovered at runtime, so it is
a switch in Go — a second place to remember, whose only defence was a test asserting
the set. That test compared the answer to a literal list, which is a second copy of the
same list: the two agreed by construction, so forgetting to classify a new value was
caught and *misclassifying* one was not.

Every real read of a standards value goes through `.<Field>.Value`, because the fields
are `Value[T]` and the number is behind that selector. So a scan of the module for that
selector, outside `internal/standards` and outside test files, is evidence about this
binary rather than a restatement of the map — and both directions are asserted:
classified consumed with no reader is a dead knob claimed live, which is the state
`staleness_days` sat in for two phases; classified unread with a reader is the false
alarm somebody has to chase.

The selector matters rather than the field name. `Allowlist` and `PerFileCap` are also
fields of `archive.Gates`, which the standards *flow into*, so matching the bare name
would count the destination as a reader of the source.

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

| Concern         | Library                                | Why this one                                                                                   |
| --------------- | -------------------------------------- | ---------------------------------------------------------------------------------------------- |
| YAML            | `github.com/goccy/go-yaml`             | exegesis and skillsaw agree; skillet pins `v1.19.2`                                            |
| Markdown AST    | `github.com/yuin/goldmark` (+GFM)      | via `skillet/markdown`; never parsed twice                                                     |
| SQLite          | `modernc.org/sqlite`                   | pure Go, no CGo, single binary; FTS5 present                                                   |
| Git             | `github.com/go-git/go-git/v6`          | `v6.0.0-alpha.5`; chosen deliberately over stable `v5` — see §20.6                             |
| CLI             | `github.com/peterbourgon/ff/v4`        | the scaffold; `climax` conventions                                                             |
| TOML config     | `github.com/BurntSushi/toml`           | adh's choice, per the constraint table                                                         |
| Secret scanning | `betterleaks`                          | **see below**                                                                                  |
| HTML → markdown | `JohannesKaufmann/html-to-markdown/v2` | pure Go, deterministic; the one pinned extractor. `v2.5.2`, pinned in `standards/archive.toml` |
| Units ↔ prose   | `github.com/dustin/go-humanize`        | bidirectional `ParseBytes`/`ParseSI`; keeps stored constraints legible (§10.2)                 |
| Spelled numbers | `github.com/rodaine/numwords`          | `ParseString` normalizes "three and a half" to 2.5-style digits; see §7.3                      |
| HTTP fetch      | stdlib `net/http`                      | no framework                                                                                   |
| Web server      | stdlib `net/http` + `html/template`    | leafwiki's posture: one binary, no Node                                                        |
| Terminal UI     | `charmbracelet/glamour` for rendering  | llmwiki's choice; read-only viewing                                                            |

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

**One verdict, two renderings — never two verdicts.** The human output and the
JSONL envelope differ in presentation and in nothing else: the same run over the
same corpus reaches the same findings, the same status, and the same exit code
whichever form is asked for. This is stated because it is the sort of thing a
reasonable person proposes relaxing. A surveyed system carries "disposition traits"
that shape how strictly it interprets, and reading it invites an obvious-seeming
adaptation: strict in CI, forgiving in a local loop. That would give a developer a
pass on their machine and a failure in the pipeline **for one corpus at one
commit**, which is the most demoralising property a checking tool can have and the
fastest way to teach a team that the tool is noise.

It is also the same guarantee as §4.6's *two users at the same commit hold the same
index*, one layer up: a verdict that depends on where it ran is a verdict about the
environment rather than about the corpus. Where strictness genuinely must vary —
a repository that has not adopted a convention yet — the mechanism is a declared
value in `standards/` that both runs read (§6.5), so the difference is a fact about
the corpus and is visible in a diff.

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
gnosis schema [--check] | link       maintain AGENTS.md and agent-file symlinks
```

### 8.2 Ingestion

```text
gnosis fetch <uri>...                archive bytes into tier 0; append (uri, sha256)
gnosis fetch --recheck               re-fetch recorded sources; resolve drift (§14.3.2)
gnosis ingest <uri|path>...          fetch → scan → emit extraction prompt(s)
gnosis ingest --resume               drain the crash-resumable queue
gnosis admit <reply-file>|--stdin    gate a reply; write to quarantine on pass
gnosis promote <slug>...             quarantine → bundle, behind the promote gate
```

`ingest` is a two-phase relay, not a single call: it emits prompts and suspends,
an agent supplies the reasoning, `admit` consumes the reply.

**The `--relay` chaining this used to promise was a misreading, and the correction
is smaller than the promise.** `adh run --relay` does not emit a prompt and block
reading a reply in one invocation: it emits and *stops*, and a second invocation
resumes with `--response <file>`, where `-` means stdin. gnosis already has that
shape in two commands, so the chaining was never missing — reading the reply from a
pipe was. `gnosis admit --stdin` closes the round trip: no temporary file.

It matters that the misreading was caught, because the other reading needed a wire
format. A caller cannot know a prompt has finished arriving on a pipe that is about
to block, so something would have to delimit it — and inventing a protocol for a
tool whose whole design is that it never speaks to a model is a large cost for one
round trip. Two invocations need no delimiter: the process exits, which is the only
end-of-message marker that cannot be misread.

It is a flag rather than adh's `-` because this CLI cannot spell `-`: `ff` reads a
bare dash as the end-of-flags terminator, so it never arrives as an argument. The
flag has one advantage over the convention — `gnosis admit --key K` with no reply
source fails immediately instead of blocking on a terminal — and naming both
alternatives in one error is what keeps a caller who gave two from having one
chosen for them silently.

**A prompt is removed when the reply that answers it is filed, and not before.**
`.gnosis/prompts/` used to accumulate one file per question and remove none, so a
reader listing the directory could not tell what was outstanding. The removal is
`admit`'s, and the trigger is deliberately the *filing* rather than the caching:
caching happens before the reply is even parsed, and an agent told "the YAML is
malformed, fix it" must still be able to submit another reply under the same key —
which `admit` can only accept while the prompt's metadata is there. Removing it
earlier would turn the diagnostic into advice nobody could take. A preview removes
nothing, structurally, because it never reaches the filing step.

The removal order is the reverse of the write order. `Prompts` writes the metadata
first, so a crash between the two leaves an inert meta describing a prompt that is
not there; removal takes the prompt first, so a crash between *those* leaves the
same inert state rather than its opposite — a prompt an agent can answer whose meta
`admit` would refuse. It is best-effort and never becomes the operation's failure:
the reply is cached and the document is filed, and telling a caller to retry that
because a file could not be unlinked would be a worse report than a note on stderr.

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
gnosis lint --check conflict         contradiction detection (§10) — see below
gnosis adjudicate --claim ID --by A --rationale R <path>  record a decision (§10.6)
gnosis challenge --class C --rationale R --by A <path>    contest a claim (§10.7)
gnosis challenge --list [--unanswered]  open challenges, oldest first
gnosis lint --check stale            freshness: the date half; drift is `fetch --recheck`
gnosis supersede --loser-claim ID --winner-claim ID <old-path> <new-path>  §10.4
gnosis log [--since]                 read/append log.md (OKF §9)
gnosis critic --model M [--path P] [--sample N]   emit cold-critic prompts (§10.5)
gnosis critic --key K --response FILE   file a critic's verdict
gnosis miss report                   aggregate the miss log (§6.4)
gnosis gate <findings-file>          block on error-severity findings; runs its self-test
gnosis audit [--since] [--reversed] [--outstanding] [--gained] …  read the trail (§15)
```

**Two rows here named commands that were never built, and the function shipped
elsewhere.** `conflict` and `stale` are `lint` checks: §10.2.0 records that conflict
implements the two decidable predicates as checks, and §12 splits `stale` between the
date comparison — a check — and the drift comparison, which needs the network and
belongs to `fetch --recheck`. The rows said otherwise for two phases, which is the same
blind spot §12.1 names one section over: a command absent from the registry and absent
from anybody's backlog agree with each other. Corrected 2026-09-03, when a diff of this
list against the registry found ten unbuilt names and `TODO.md` mentioned none of them.

### 8.5 Family Interop

```text
gnosis export --format okf|jsonl     bundle export
gnosis proof create --arc A --out P  proof packet binding corpus + tier-0 digests
```

**`gnosis manifest` was specified here and is not built, because the package it names
keys on the one thing this corpus refuses to key on.** `skillet/manifest.Diff` matches
entries **by location**, and documents why: a slug is not unique across the runtime roots
a skill tree spans. A gnosis concept's location is a *view* — §5.1.1 puts the slug in the
path and §5.4 requires identifiers, never paths — so a manifest over this corpus would
report every retitled concept as one removed and one added. That is not a mapping detail;
it is what `Diff` is built on, and it is the defect §10.4's edge shipped with for one day.

The two packages share a **shape** — a hashed inventory answering "what changed since a
baseline" — and a shape is followed rather than imported, which is exactly what
`ruleset/conflict` records one section over. Nothing was built, because for a committed
corpus `git diff --name-only` already answers the question, and a JSON file duplicating
it would be a second answer that can disagree with the first. The trigger is the first
downstream tool that asks gnosis what changed and cannot use git to find out.

**`export` and `proof create` share one predicate**, `bundle.Portable`, which names the
shareable half of a bundle: everything except `.gnosis/`. That directory holds the audit
trail, the prompt cache, the miss log and the coverage ledger — per-user and derived, so
two colleagues at one commit have different ones and are both right. An export carrying
it would publish a colleague's session history to whoever the bundle was shared with, and
a proof packet covering it would fail to verify for a reader who had done nothing but run
a query.

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
  source is a genuine no-op: the record already exists at its content-addressed
  path (§4.3.1) and nothing is written to the bundle. Only `.gnosis/checked.jsonl`
  advances, recording that this user looked. Re-fetching a *changed* source writes a
  new archive file and a new record, and marks every claim citing the old one
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

Ordered, all deterministic. **All four stages are built for a candidate document;
a fetched source gets stages 1 to 3**, and `internal/scan`'s `Coverage` reports
which ran so that a clean result is never read as "the scan passed". A caller that
cannot distinguish "no hidden characters" from "§9.3 satisfied" will eventually
claim the second on the strength of the first.

That reporting is consumed rather than merely available, which was not true of its
first form: a `Stages()` function returned the implemented stage list and nothing
ever called it — an honesty mechanism declared and read by nobody, the same failure
§6.5.1 describes one layer up. `Coverage` feeds the promote gate's `security`
signal, which reports `unchecked` when a stage did not run, which is what routes a
candidate to §9.5.1's human path.

**Coverage is composed from what was performed, not declared by what performs it.**
`scan.CoverageOf` takes the stages a caller actually ran and computes the rest from
this section's own list, and `bundle.scanCandidate` is its one production caller —
so the stages claimed and the code running them sit adjacent, each stage claimed
inside the branch that performs it. That replaced two functions which each answered
part of the question and neither of which could say that stage 4 ran. **A coverage
report cannot be checked by a compiler**; what it can be is arranged so the claim
and the act are one edit apart, and so a mistyped stage name makes coverage look
worse rather than better.

**Stage 4 was the last gap and it needed no new threshold.** `archive.Gates`
enforces `per_file_cap` and `embedded_payload_cap` when a fetched source is
admitted, which is this stage's bound in the place this section asks for it — and
what that did not cover was a *candidate document*, because the archive gate bounds
sources arriving from upstream and a document a model wrote is neither fetched nor
archived. The resolution is to apply **the same declared caps** to the candidate,
through `archive.Oversize`, rather than to invent a second bound: two artifacts, one
threshold, which is what §6.5 requires. The caps reach the scan on `gate.Limits`,
beside the two thresholds the gate already reads, because the `security` signal is
what needs them to say whether the stage ran.

A fetched source still reports stage 4 as covered by the archive's own admission
rather than by the scan — `admits` checks the file cap before the text test and the
payload cap after it, and the ordering is what makes a binary report as binary
whatever its size.

**Completing §9.3 did not remove the human path from the ordinary case**, and that
is worth stating because it was expected to. A clean candidate's `security` verdict
is now `pass` rather than `unchecked`, and promotion still requires a person because
`conflict` reports `unchecked` for the Phase 3 reasons §10 describes. What the
stages bought is coverage, not friction.

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
3. **Secrets** — vendor-documented credential formats plus explicit `<redacted>`
   markers. Refusal happens **before anything reaches disk**, which is `pantry`'s
   one clearly-right invariant, and refusal rather than redaction because §4.4
   forbids rewriting anybody's bytes: a source carrying a key falls through to
   `referenced`, so the URI and hash are recorded and nothing quotable is kept.
   **`betterleaks`, which this section named, does not exist** — not on the public
   module proxy under any casing, and not in `skillet`. What is implemented instead
   is the part that needs no dependency and no judgement: shapes their issuers
   publish, which is the same class of justification as stage 1's codepoint ranges.
   Deliberately no entropy or length heuristic, because that is a tuned number
   inside a blocking gate.
4. **Oversize / binary** — bounded, with the bound in `standards/`. One bound, two
   artifacts: `archive.Oversize` is what applies it to both.

**A missing scanner refuses.** `archive.Gates.ScanText` used to admit text when no
scanner was wired, on the grounds that refusing would make every non-scanning caller
carry a stub — so a single test stood between the shell and no §9.3 at all. That
stopped being defensible once the candidate path was built the other way, degrading
toward *more* blocking and reporting the stages it could not run. Two halves of one
stage failing in opposite directions is worse than either choice made twice, so a
nil now refuses with `unscanned` and a caller that means to skip says so by name.

Findings gate the ingest, land in the audit trail, and export as SARIF for any
code-scanning pipeline that wants them.

The scan runs **twice, over two different artifacts**, and conflating them was a
real gap rather than a hypothetical one. At admission to tier 0 it scans the
*fetched source*. At the promote gate it scans the *candidate document* — the whole
file including frontmatter, since a `subject` or a source URI is machine-read and a
zero-width character hides in a key exactly as well as in prose. The candidate is
the more dangerous of the two: it is the artifact filed into the corpus for an
agent to obey, and a model can reproduce an injected instruction out of source text
that was itself clean.

The tier-0 half runs at **admission**, not at lint time: hidden characters in a
fetched source must never reach disk, which is what "before any model sees the
content" means when the content is being archived. A source that fails falls through
to `referenced` (§4.3) — the URI and hash are still recorded, and nothing quotable
was kept, which is the same shape every other admission refusal takes.

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

**The gate approves an artifact, and that artifact is what lands.** Between
checking a candidate and committing it there is a window, and nothing above closes
it: the promote gate validates content, a write then happens, and no rule says the
bytes checked are the bytes written. A corpus whose gate can be raced is a corpus
whose gate is decorative.

So promotion is stated over a **diff** rather than over a document: the gate
receives the exact change to be applied, and the writer applies precisely the
change the gate approved or nothing at all. A re-read between the two is a defect,
not an optimisation.

**The guarantee comes from the command shape, not from care.** §4.6.2 makes a write
one command value with an `Effect` field, so a preview and an apply are the same
handler over the same input, differing only in whether the final write happens.
Two code paths could agree by inspection and drift by edit; one path cannot
disagree with itself. The coordinator owns the bundle so that "the approved diff"
and "the committed diff" are one object rather than two reads of a file, and the
command type is what makes them one computation.

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

#### 9.4.1 a Reason Refuses a Cut, and the List Is Data

The stand-alone rule above forbids cutting at a subordinating join. *"The cache is
cleared when the process restarts"* split at *when* yields *the cache is cleared*,
which is false as stated, and *the process restarts*, which asserts nothing the
sentence claimed. So the cut is refused, and `internal/segment` says so where its
coordinating list is declared.

**The rule is stated and not fully enforced, which is why the indicator words are
data rather than a paragraph.** Segmentation cuts at a coordinating join and then
asks whether the right clause stands alone, and it asks by looking for a copula
with something before it. That test passes on a clause a reason introduces:

```text
"The retry budget is three, and because the SLA is 400ms."
  → "The retry budget is three"
  → "Because the SLA is 400ms."      ← emitted as an independent claim
```

The second is a fragment whose main clause is in its sibling, and the invariant
`Claims` states — every returned claim stands alone — does not hold for it. A copula
test cannot close this, because the fragment has one. What closes it is knowing that
*because* introduces a reason, and that is lexical, closed, and language-specific:
exactly the shape §12's argument reserves for a data file with a test corpus rather
than a list compiled into Go. It is `standards/indicators.toml`, and it is **not**
`standards/operators.toml`: an operator pattern parses a *quantity* out of prose and
lands with conflict detection, while an indicator classifies a *clause* and its reader
exists today.

The words reach `segment` as a parameter rather than an import, for the reason §5.8's
vocabulary reaches `lint` the same way — a parser must not import another parser, so
the shell reads the file and passes the list. A file that will not load yields no
words, and segmentation then behaves exactly as it did before the list existed: coarser
only where the words would have helped, which is a coarser corpus rather than a wrong
one.

**The words gate the cut; they never make one.** A clause opening with a reason
marker is not a place to split — it is a signal that a split already proposed
nearby must be withdrawn. Stating it this way settles the failure mode the backlog
entry named, a *because* inside a quoted passage: a false positive refuses a cut and
leaves the sentence whole, so the claim is coarser and its evidence must cover more
of it. Under-segmentation is a claim harder to support. Over-segmentation is a claim
that was never made. Only one of those is recoverable, and the lexicon's errors fall
on that side by construction.

The file carries both roles — the words that introduce a reason and the words that
introduce a conclusion — because they are one closed class and authoring half of it
guarantees a second pass over the same evidence. Only the reason half has a reader
at this point; §17.4's check is the conclusion half's, and the file says so, so that
a later reader does not mistake an unused row for a dead one.

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

#### 9.5.1 What a Gate Does When It Cannot Check

Seven signals, and two of them read subsystems that do not exist yet: `conflict`
needs §10's adjudication, and `security` can run one of §9.3's four scan stages.
A signal that did not run reports `unchecked`, which is not a pass — "nobody
looked" is not the claim "this is fine".

Requiring every signal to pass therefore meant **nothing could ever be promoted**.
An earlier draft of this section said so and called it correct. It was correct and
it was half an answer: a permanently red gate with no sanctioned way through it is
a trap, and the trap's shape is familiar — `oh-my-agent` caps its reinforcement
loop at five iterations "so a permanently red gate can't trap you". The honest form
is **a bound with a recorded reason, not a bypass.**

So the gate reaches one of four decisions:

| Decision      | Condition                                            | Who may promote                         |
| ------------- | ---------------------------------------------------- | --------------------------------------- |
| `approved`    | the control held and every signal passed             | anybody; no confirmation                |
| `needs_human` | nothing failed and at least one signal could not run | a person, with a phrase and a rationale |
| `refused`     | at least one signal failed                           | **nobody**                              |
| `unavailable` | the planted-defect control did not hold              | nobody                                  |

**The load-bearing row is `refused`.** A failed evidence check is not a judgement
call: there is no confirmation phrase that makes a fabricated quotation acceptable
and no person senior enough to make one true. **The human path opens for what could
not be checked and stays shut for what was checked and failed.** That sentence is
the whole difference between an escalation and the `--yes` this specification
forbids two sections later, and it is the property most worth testing.

`refused` outranks `needs_human` when both apply. Offering somebody a signature
over a document with a known defect in it is worse than refusing it twice.

**The escape is a person, and deliberately not a counter.** The `oh-my-agent` cap
cited above was read more closely afterward, and its actual verdict rule is
`PASS: ALL criteria are PASS **or BLOCKED**`, where `BLOCKED` means a criterion failed
three consecutive times and will not be retried. It breaks the deadlock by **counting
an unresolved criterion as satisfied**. For a development loop that is a reasonable
trade — the work is in a branch and a human reads the summary. For an admission gate
it is the collapse this whole subsection exists to prevent, because the resulting
`PASS` is indistinguishable from one where everything actually passed, and the
distinction is gone by the time anybody cites the claim.

So the bound gnosis takes from that design is the *shape* — bounded, reasoned,
recorded — and not the mechanism. A counter cannot be the escape here, for two
reasons. A counter that expires into `approved` is a `--yes` with a delay. And a
counter measures how many times gnosis tried, which is a fact about gnosis; whether
an unchecked signal is acceptable for *this* document is a fact about the document,
and only a reader has it. `needs_human` puts the bound where the knowledge is.

Two details of that protocol *are* worth having and are recorded against the ratchet
work rather than here: a `PASS → FAIL` transition is a distinct status from a
first-time failure and should be reported as one; and a failure counter must count
*consecutive* failures and reset on success, because a counter that never resets lets
an intermittently flaky check accumulate to a permanent verdict on nothing but time.

Carrying an unchecked signal takes three things, each closing a different route
back to a bypass:

- **A person.** `human:` and not `agent:`. An agent authorising its own promotion
  makes the path decorative, and an agent is what produced the candidate.
- **The phrase.** Typing the document's path. Not "yes" — a confirmation
  suppliable from muscle memory confirms nothing, and naming the file is what
  makes somebody look at which one it is. There is deliberately **no flag** that
  supplies it: a `--confirm=<path>` lives in shell history, in scripts, and in CI.
- **A rationale.** §10.6.4's argument applies unchanged: a required rationale
  filters more bad adjudications than a permission check does, because the
  reviewer has to write it where colleagues will read it.

**The audit row records which signals were carried, and that is what makes this a
debt register rather than a bypass.** A trail saying only "a human approved it"
cannot answer the question that matters when §10 lands: *which claims in this
corpus were admitted with no conflict check?* A trail naming the signals can, and
every such document is then one query from re-examination. Without that field the
argument in this subsection collapses and the design is a `--force` with a longer
prompt.

A machine caller is never prompted. `--jsonl` returns `blocked` with the
requirement in the envelope, because a prompt on a pipe hangs — which is a failure
mode that presents as the tool having crashed.

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

**Built 2026-09-03, and two things above it are corrected by having built it.**

**`mine` reports and never files.** The bullet above says the Stop-hook companion "files
wiki-touching turns as candidate answers, subject to §8.3's `file` gate". It cannot: a
chat answer cites no archived source, and the promote gate fails a document declaring
none — "a document asserting claims and citing nothing is exactly what this corpus exists
to refuse" (§9.5). An automatic filer would fill quarantine with drafts that can never
promote, which is a queue that only grows and a reader who learns to ignore it. So the
companion hands the session over, `mine` reports the questions somebody had to ask more
than once, and writing one up means `fetch` and `ingest`, where evidence exists.

**Retry chains are detected by repetition rather than by feedback.** "A prompt re-asked
after negative feedback" needs a vocabulary of what disappointment sounds like; a
vocabulary is a `standards/` value with a rationale (§6.2) and nobody can write that
rationale from measurement yet. Re-asking is observable without any of it — it *is* what
re-asking means — and the same comparison at a wider scope gives the recurring-intent
half: a question asked in two sessions is one nobody wrote down. Labelling outcomes from
feedback signals stays unbuilt, and its trigger is a corpus with enough mined sessions to
write that vocabulary from measurement.

**The hook's configuration format is deliberately not specified here.** `gnosis mine
--session -` reads a transcript from stdin and that is the whole contract; a hook config
written into this specification would be another tool's format kept in a second place,
which is the rot this section's own "one normalizing seam" sentence is about.

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

- **Subjects are declared.** A claim's `subject` names a key from `ontology.toml`
  (§5.8). A subject key alone buys **candidate narrowing** — two claims sharing a
  key are a candidate pair, feeding the deterministic selection in §6.2. No
  verdict, no value, no dimension needed, and a wrong key costs a wasted
  comparison rather than a wrong answer.
- **Values are derived.** A constraint is a **cached parse of the prose, not a
  second assertion** — operator patterns from `standards/` plus the units
  libraries (§7.2, §7.3), written into `claim_subjects` and nowhere else.

#### 10.2.0 Which of the Six Are Built, and Why the Rest Are Not

The table above lists six predicates and `conflict` implements one. Saying which, in the
check's own comment and here, is the §12.0 requirement applied to itself: a checker
called `conflict` that quietly implemented a sixth of this section would be the
unwarrantedly-confident reading — a green run standing in for an examination nobody
performed.

- **evidence divergence** is built. Its decidable form is **byte identity**: the corpus
  holds one assertion in two places, and the two rest on snapshots of a single source
  that are not the same bytes. Whether the page changed in the passage that matters is
  exactly what nobody has asked — and if it did, one of the two claims is now unsupported
  while both still read as evidenced. Two sites on the *same* version are corroboration
  and say nothing; two different *sources* are corroboration across sources, which is
  what a corpus is for.
- **severity** and **level divergence** read `ruleset.Severity` and `ruleset.Level`, and
  the blocker moved on 2026-08-27 without the predicate becoming buildable. The kernel
  ships `ruleset` as of v0.23.0, with `MUST`/`SHOULD`/`CONSIDER` and
  `CODE`/`ARCH`/`METHOD` — so the types exist. **What does not exist is a claim that
  carries one.** Nothing in `gnosis_claims` declares a severity or a level, so there is
  still nothing to compare; the predicate applies to a corpus of *rule* documents, and
  this one holds claims. `skillet/ruleset/conflict.Find` operates on a `ruleset.Ruleset`
  rather than on claims, which is why it remains the wrong shape rather than the missing
  piece. Recorded this way because "no such package" was true for a day and would have
  read as permanent.
- **identity collision** is already reported, by `identity` and by the promote gate's
  duplication signal. A third reporter would make one problem read as three.
- **interval conflict** is built. Two claims on one subject key whose admissible ranges
  share no value cannot both hold. **Disjoint, not merely different**: `<= 3` and `<= 5`
  differ and are perfectly compatible, and a predicate firing on difference would report
  every corpus that states a bound twice — which is every well-specified corpus. It runs
  only where both sides parsed and only within one dimension, because comparing a count
  to a duration is a category error that would blame the claims for the comparison.
- **enumeration conflict is subsumed rather than absent.** With the operator set as it
  stands, two claims asserting `==` on one subject with different values are two disjoint
  intervals, so the predicate above reports them. Building a second that fired on the same
  pair would report one problem twice. It separates only if a pattern ever yields a
  set-valued operator, and none does — which is the condition to watch rather than a
  deferral.

**Candidate narrowing by subject is not built either, and that is deliberate.** §10.2
describes it for the two predicates that compare *values*; evidence divergence keys on
the claim text and needs none. Building the narrowing now would be a mechanism with no
reader, which is the mistake this specification has recorded four times.

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

##### One Writer, Both Halves, and Not Yet

`claim_subjects` has no writer today and that is a decision rather than an omission.

A row there carries two kinds of thing: the **declared** subject key, and the
**derived** operator, value and dimension — which is what `derived` and `pattern_id`
exist to distinguish. Populating only the declared half would leave every row with three
columns whose whole purpose is telling a parsed value from a pinned one, meaning
neither. A table with no writer is honestly empty; a table whose writer fills half of
each row is a table a reader will misread, for however long the operator patterns take.

So: **one writer, at index-rebuild time, writing both halves together**, landing when
`standards/`'s operator patterns and their test corpus exist. Until then the table stays
empty, and `Digest`'s guarantee over it is vacuous rather than wrong — it becomes
meaningful the moment the writer appears.

**What that does not block, and this is the point of separating the two questions.** The
subject key is declared frontmatter, so everything that needs only *which subject a claim
is about* reads the document and needs no table: §5.8.3's review signal, the
`subject-unknown` check, §5.8's `ontology` check, and §5.8.2.1's per-subject population
report. Those wait on one thing — the lint snapshot carrying the **vocabulary**, which it
does not today, which is why all three are unbuilt together.

**What stays behind this writer is narrower than an earlier draft of this paragraph
said.** The foreign key runs from `claim_subjects` to `claims`, not the reverse, so
`claims` is not coupled to the operator patterns at all. Claim-level **search** (§11)
waits on *extraction* to supply a claim's title, description and lead — frontmatter
carries none of those — and claim-level **link attribution** waits on the link extractor
reporting byte offsets, plus `claims.pos` from the anchors. Neither needs a parsed
constraint. Only conflict detection and the two constraint checks do.

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

**The pin path was specified for two phases and unbuildable for both**, which is worth
recording where the rule is stated rather than only in a backlog entry. Nothing parsed
`gnosis_constraint`, `DocClaim` carried no field for one, and the row builder hard-coded
`derived = 1` — so the two columns whose whole purpose is telling a parsed value from a
pinned one meant neither, and this section described a precedence nothing could exercise.
It reads as the mirror of stored state with no reader: a stored **distinction** no input
could produce. Built 2026-08-27, along with the check below.

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

**This half is specified and not built, and the reason is the section's own argument
turned on itself.** Reporting a bound that "spans the plausible range of its dimension"
needs a plausible range for `count`, `duration`, `bytes` and `ratio` — and nothing
declares one. Inventing it is exactly what §6.2 exists to prevent, and a vacuous-constraint
check resting on a vacuous constant would be the joke telling itself.

**The example also does not match the operator set.** *"The retry budget is between 1 and
100"* is a **range**, and the patterns yield one operator per claim — so that sentence is
two claims, and no *single* constraint under this set is vacuous: `<= 100` on a count is
contradicted by `>= 101`. Implementing the rule as written would not catch the case that
motivates it.

What would make it decidable: a declared plausible range per dimension, which is a
`standards/` value with the rationale §6.2 requires — and which nobody can write from
measurement yet, because no corpus has enough constraints to measure. Recorded rather
than built.

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
  rather than regexes in Go. Inversions are the first cases in the corpus, and
  §18.4.1 governs their *shape*: the cases are sentences because a claim's anchor is
  a sentence, which the first version of this corpus got wrong.
- **A finding derived from a parse says so, and shows the parse.** The
  adjudicator sees `no more than 3` → `{op: "<=", value: 3, dimension: count}`
  beside the claim text. A false conflict that shows its reasoning is dismissible
  in seconds; one that shows only a verdict erodes trust in the whole queue.

#### 10.2.2.1 When the Pattern Set Is Built, and Why Not Sooner

The patterns land **with §10.2's conflict detection**, which is their only consumer.
Building them earlier would leave a parser whose output nothing reads, and — worse — a
test corpus authored from somebody's imagination of how people phrase a constraint, which
is §11.0.2's warning about an instrument that cannot measure the thing claimed.

**The reason to wait is that the evidence survives waiting, and that is not true of every
artifact of this shape.** Two things in this specification are "data with a test corpus
authored from real failures", and they schedule oppositely:

- **Retrieval cases (§11.0.2) need the instrument first.** A disappointing query is
  ephemeral — it happens, it is noticed, and with nowhere to write it down it is gone by
  the next day. So the grader ships before the data, and the file ships empty.
- **Operator patterns need the consumer first.** A mis-parsed claim is *durable*: it is on
  disk, and §10.2.1's regenerability means an improved pattern set fixes every affected
  claim retroactively on the next reindex. Nothing is lost by building the parser later.

The question to ask of the next artifact like these is therefore not "is the data
authorable yet" but **"does the evidence survive waiting"**.

**§10.2.3's coverage loop cannot bootstrap this**, only maintain it. With no patterns,
coverage is zero on every key and the report cannot distinguish its own two causes — a
claim carrying no quantity from a quantity in a phrasing the patterns miss. So the trigger
is the start of conflict detection, not a coverage figure.

**Nothing is pre-positioned.** No `operators.toml` seed, and no direct import of the units
library — it is already in the module graph as an indirect dependency, so there is nothing
to reserve, and §Blocking's rule applies: an unused dependency is worse than a deferred
one. Operator inversions are still the first cases in the corpus when it is written; they
are language facts and will be as true then as now.

**One part must not be deferred with the rest**, because it is the piece most likely to be
dropped for looking cosmetic: §10.2.2's second mitigation, that a finding derived from a
parse shows the parse. It belongs in the same change as the first pattern. "A false
conflict that shows its reasoning is dismissible in seconds; one that shows only a verdict
erodes trust in the whole queue."

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
either claim. *Enforced by construction since 2026-09-02*: `relay.CriticClaim` carries
a text, a lead and its quotations, and `relay` imports nothing of gnosis's domain, so
there is no field a warrant could travel in. What a type cannot prevent is the
projection folding one into a field that does exist — a rationale appended to the claim
text — and that is where the test is. The reason is expectancy bias — a judge shown the conclusion a
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

**The edge names `<gnosis_id>#<claim-id>`, and this section and §5.4 looked as though
they disagreed.** This one says the unit is a *claim*; §5.4 says the key names
"**identifiers, never paths**… an edge that survives reorganization is the point". Both
hold, and the reference form is what reconciles them: the document's identifier
addresses the concept durably and the claim identifier addresses the assertion within
it. *Corrected 2026-09-03 — the first implementation wrote `<path>#<claim>`, which reads
better and breaks on the first retitle, because §5.1.1 puts the slug in the path.* A
reader still sees a path: §5.6 makes the presented form a view that resolves **to** the
canonical one, so the command prints the page and the frontmatter records the identifier.

**A losing document with no `gnosis_id` is refused rather than referenced some other
way.** §5.1.2 quarantines an unidentified document precisely because nothing durable can
point at one, and writing an edge to it would commit a reference that can never resolve.

Because this fires only on the loser of an **adjudicated conflict**, a claim of an
`episodic` type is never superseded — its claims are ineligible for conflict detection
(§5.8.3.1), so there is nothing to adjudicate. That is a consequence of the type rather
than a rule about it: two reports of different moments cannot contradict, and a corpus
adjudicating *"we set it to 3 in March"* against *"we set it to 5 in June"* would be
adjudicating its own history.

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
persisted to `.gnosis/coverage.jsonl` and fed into subsequent critic prompts. Each
unexamined entry names an **aspect and a reason**, and an entry carrying one without the
other is refused rather than half-recorded.

**Built 2026-09-02, and four things it settled.** The ledger **appends** rather than
upserting, unlike `checked.jsonl` and `retrieved.jsonl`, because here the sequence is
what is consumed: steering away from *exhausted* angles means the union across
critiques, and one row per claim would discard what an earlier critic covered. An angle
declared unexamined and later examined is **subtracted**, so the next prompt is not sent
back to finished ground. A verdict's severity is gnosis's and not the model's —
`relay.CriticFinding` has no severity field, so a reply has nowhere to ask for one — and
every category is namespaced `critic:`, before a collision rather than after the one
§12.1 records. And because the coverage goes into the prompt, and the prompt's hash into
the key, **a second critique is a different question with a different key**: §6.1's cache
cannot serve the first answer to it, which is the property that makes steering work at
all.

**This does not compromise cold-context independence**, and the reason matters: a
coverage record says what was *looked at*, never what was *concluded* or how the
concept was produced. Feeding prior coverage to a fresh critic biases it toward
unexamined ground — the opposite of contamination. The block is **advisory
only**: a critic that declares a gap must never thereby block, or it will learn
to declare none.

**The reason is the half that makes a gap actionable, and it was added 2026-09-03.** An
aspect alone tells a later critic nothing it can act on; "the excerpt does not include
it" tells it the ground is unreachable from this prompt, and steering a second critic
toward it would send it at a wall the first already described. The shape is
`finding.Unexamined`'s — the family type whose documented purpose is exactly this reply —
so the requirement is that type's `Valid()` rather than a second rule stated here. Rows
written before the change are read as an aspect whose reason says where it came from,
because failing the load of the record that exists to be matched against later is a worse
answer than an honest one.

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
a claim's `subject`, needing no roster — and **shown rather than enforced**. The
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

**This is a bet, and the opposite bet works elsewhere.** A live skill catalogue in
the surveyed field admits community contributions by quorum — a fixed review window,
reactions as votes, a count on the closing day — and it functions. The difference is
blast radius. A bad skill is uninstalled by whoever notices; a bad claim is cited,
and the citation outlives the noticing. Where the population of reviewers is large
and the cost of a wrong admission is low, counting is the cheaper instrument and
asking each voter for a written reason would simply reduce the number of voters.
gnosis has the opposite profile — few reviewers, claims that get built on — so it
buys depth rather than breadth. Stating the trade keeps this a choice rather than an
assumption, and it names the condition under which the choice would be wrong: if a
corpus ever has many reviewers and cheap reversals, the rationale requirement is
worth revisiting.

#### 10.6.2.1 Ownership Is a Property of the Subject, Never of the Warrant

A corpus can say *Priya adjudicated this* and cannot say *the platform team is
accountable for this rule*. Those are two questions and the obvious single answer — a
`role` on `gnosis_warrant` — answers the wrong one.

**Ownership belongs to the subject.** A platform engineer adjudicating a documentation
claim does not make it a platform rule, so recording the approver's role would answer
"who is accountable for this area" with a fact about whoever happened to be in the
review queue. Accountability attaches to the subject matter, which is why an optional
`owner` on a subject declaration is the right shape and the same per-subject grain
`requires_capability` already uses: the argument is about a handful of keys rather than
everyone's job title.

**A `role` on the warrant is refused for three reasons**, and the third is the one that
would be hard to undo.

- **It would be self-asserted.** The approver types their own role, so the field records
  a *claim* about authority rather than authority — a permission check that has given up
  on verification, which is exactly what §10.6.4's rationale bet exists to avoid.
- **It answers the wrong question**, as above, and answers it incorrectly in precisely
  the case §10.6.2's domain history exists to detect: somebody adjudicating outside their
  area.
- **Its non-gating guarantee would be a convention rather than a structure.**
  `gate.Candidate` already carries the parsed document, so a warrant field is inside what
  the gate reads and only a comment would keep it unread. The ontology is in neither
  `gate.Candidate` nor `gate.Corpus`, so reading a subject's `owner` would require
  visibly widening the gate's inputs — a change a reviewer has to argue for. §14.1's rule
  that a tier is a signal and never a permission is worth an enforcement mechanism, not
  a promise.

**`owner` is reported and never enforced**, on §10.6.2's terms: it appears in the review
queue beside the domain history, so a reader deciding whether to defer sees both who has
adjudicated here before and who is accountable for the area. It grants nothing.

**One cost, accepted rather than mitigated.** Ownership does not survive a
reorganisation: if it moves, past decisions read as owned by the new team. That is the
right trade, because ownership is a *current* question — "who do I ask about this now" —
and §6.2's precedent already puts the change itself in `log.md`, where the history
belongs. Recording ownership per warrant to preserve it would buy a historical fact
nobody asks for at the price of the three problems above.

**It waits for its reader.** §13's review queue is the first consumer, and adding the key
before something displays it would repeat a mistake this project has recorded three times
— a stored value nobody reads.

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
  **The announcement in `log.md` is the stored previous tier**, which is why no
  separate baseline exists. `adjudicate` writes the line for the move it causes,
  and `doctor` reports a derived tier that no line matches — the case a command
  cannot see, because the `verified` list was edited by hand. A line that cannot
  be read back counts as no announcement, so the failure is an over-report with an
  obvious remedy rather than a silence.
- **Escalation must never deadlock.** At `paired`, if the second signer is
  unavailable, a single-signer override is permitted with a recorded reason and a
  flag on the claim. A queue that can block indefinitely stops being used, and an
  unused queue admits nothing — which is a worse outcome than an override that
  leaves a trail. This is the same escalate-rather-than-stall shape as
  `canonizer`'s rework budget, which returns `needs-human` rather than blocking
  forever.

#### 10.6.4 Warrant Is the Real Gate

**Nothing implements this yet, and two other repositories already cite it as
though something does.** A grep for `gnosis_warrant` across `internal/` and `cmd/`
returns nothing: it is Phase 3, correctly, and it is specified here down to
`co_signed_by`, `override`, and `reverses`. The risk is not the gap. It is that
`skillet` and `canonizer` both point at this section as the family's mature model
of adjudication authority, so **the shape below is load-bearing outside this
repository before any of it is built** — a reshaping done for local convenience
would break something that is not in this repository to notice.

That is the same obligation §14.1.1 records for `Report.Skipped`, and it is
recorded for the same reason: the cost of discovering it later is a refactor that
looked free.

Recorded so it is not re-litigated: **`canonizer` will get a smaller warrant than
this one, deliberately.** `skillet` will carry `{By, At, Rationale}` on
`ruleset.Rule` and nothing more. Tiers, co-signers, and reversal links stay here,
because they belong to §10.6's authority model — which `canonizer` explicitly bet
against in its own equivalent of this section, holding that a required rationale
filters more bad adjudications than a permission check. Two warrants with
different obligations is the correct outcome and not drift, and `gnosis.Actor`
being a closed three-kind enum for the counting below is precisely why a shared
warrant would be weaker than this one rather than stronger.

The warrant carries no `role` and no team. Accountability is a property of the subject,
recorded there and reported rather than enforced — §10.6.2.1 gives the three reasons,
of which the sharpest is that a warrant field sits inside the document the gate already
parses, so keeping it out of a permission decision would be a convention rather than a
structure.

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

##### The Failure Mode of This Bet, Observed

The bet above has a known way of losing and it is not the one a reviewer expects. It
is not that somebody writes a bad reason — a bad reason is legible and arguable, which
is the mechanism working. It is that **the field gets satisfied without being used.**

A surveyed system requires a decision rationale, enforces non-empty by schema, and
then has to warn its own agents in prose: *"an audit log of identical boilerplate
strings records that decisions happened but not what they were."* Their template text
was being emitted verbatim into the required field. Non-empty is a check on length,
and the thing being defended against has length.

So `rationale` carries one more admission rule, which is cheap and deterministic:

- **A rationale that folds to the prompt's own template text is refused.** The emitted
  prompt and its example rationale are known to gnosis — the relay wrote them — so the
  comparison is against a value in hand, under `textnorm.Fold` so whitespace and
  typographic variation do not defeat it.
- **A rationale byte-identical, after folding, to one already recorded for the same
  `subject` is refused.** Two claims may legitimately be adjudicated for the same
  reason, and the honest way to say so is a reference to the first warrant, not a
  second copy of its prose. The refusal names the earlier warrant so writing that
  reference is the easy path.

Both are `EINVALID` with the matched text quoted, because a diagnostic that says
"rationale rejected" without showing what it matched is a diagnostic somebody works
around by adding a word.

##### What Implementing It Settled

`gnosis.UnusableRationale` is pure over (rationale, the phrases the tool showed the
author, prior rationales). Four things the paragraphs above do not say.

**It is applied to `command.Promote.Rationale`, not to `gnosis_warrant`.** The warrant
is Phase 3 and unimplemented; the promotion rationale is the field that carries
reasoning in this binary today, is required on §9.5's human path, and is the one that
survives into the trail. Phase 3's warrant inherits the function unchanged.

**It folds case, and every quotation guard deliberately does not.** `textnorm`
preserves case on the argument that "a quotation differing only in case is a different
quotation" — correct for evidence and wrong here, because a rationale is not evidence
and `State why you are promoting…` is the same boilerplate as `state why you are
promoting…`. Capitalising one letter is the cheapest evasion available, and the first
implementation was defeated by exactly that.

**Template text is matched by containment; a prior rationale by equality.** The
workaround for an equality check is adding a word, which the paragraph above names in
its own argument for quoting the match. Equality is right for the second refusal
though: a rationale that quotes the earlier one and then says why *this* case differs
is the most useful thing an author can write, and containment would refuse precisely
the person who did the extra work.

**Only rationales on promotions that landed count as prior**, and taking refusals into
account broke the command. `promote`'s confirmation flow previews, records the blocked
outcome carrying the rationale, and then applies — so counting refusals made the apply
find the preview's own row and call it a repeat. The correct reading is also the
faithful one: "already recorded" means a warrant, and a warrant is a decision that
landed.

The comparison is scoped **per path** rather than per subject, because a promotion has
no subject. That is narrower than this section asks for and is stated so no reader
believes the corpus-wide check exists.

Two limits, stated so the check is not over-trusted. It cannot detect a rationale that
is original prose and says nothing — no mechanical check can, and §17's refusal to
score means gnosis will not pretend otherwise. And it deliberately does **not** apply
to `override.reason`, where *"marcus on leave until 09-02"* is a complete and correct
answer that will legitimately recur. The check defends the field that carries
reasoning, not every free-text field.

##### A Decline Is a Decision; Abandonment Is Not

§10.7.4 makes a decision to decline a decision, and "declined" turned out to cover
three events that want three different records.

- **The gate refused.** Mechanical, and *recomputable from the document* — run the gate
  again and get the same answer. It stays in the per-user trail. Committing it would
  put a derived fact in the authoritative tier, which §12 already refuses for the
  index.
- **A person was asked and walked away.** Not recomputable, and not a decision: nobody
  decided anything. Nothing records it, and `gnosis audit --outstanding` is what
  surfaces it — a promotion that reached `needs_human`, whose draft is still in
  quarantine, never promoted and never discarded.
- **A person looked at the draft and dropped it.** Not recomputable from anything: the
  draft is gone and the reason existed only in their head until they typed it. That is
  a decision, and it is filed in `log.md`, following §6.2's precedent for a threshold
  change — a decision with a reason, in the tier a colleague reads.

`gnosis quarantine --discard` was already the decline verb and already required a
reason. What was missing is that the reason never reached the committed tier.

**An agent's discard still does not reach it**, deliberately. `Discard.By` may be an
agent because dropping a draft grants no authority, and an agent clearing a reply its
own gate refused is housekeeping. Committing every one of those would fill the corpus's
history with the noise §12 argues teaches a reader to skip a category. The agent's
reason is still in the trail.

The committed entry is written **before** the draft is removed. A crash between them
leaves an entry saying the draft was declined and the draft still in quarantine —
visible in `gnosis quarantine`, and recoverable by discarding again. The reverse leaves
the draft gone and no record of the decision, which is the state this section exists to
prevent.

##### What an Ingestion Does Not Authorize

A corpus recorded what a source supports and kept no trace of the opposite. A reply
asserting something the archived text does not contain was refused, reported once, and
forgotten — so the same assertion could be offered again, by the same model, and nothing
would say it had been tried. That is the asymmetry §5.8.2.1 already fixed one level
down, where an `aliases` list "keeps the conclusion and throws away the reasoning".

The refusal is recorded on the trail row with the archive path it was checked against,
and `gnosis audit --unsupported` reads it back. Three decisions:

- **Its own field, not `Findings`.** That one means the finding *ids* a write turned on,
  and a claim's text is not an id. Overloading it would make one field mean two things.
- **Only *unsupported* claims, never *unchecked* ones.** "Sought in the archive and not
  there" is a statement about the source; "nobody looked" is not. Recording the second
  would assert that a source contradicts a passage too short to check, which is the
  accusation §9.4 goes to some trouble not to make.
- **The claim's text, not the refusal's wording.** "claim 2: …" locates an entry in a
  reply that is on screen now and refers to nothing in a trail read next month, and a
  truncated preview is the wrong content for a durable record.

The **committed** record of what a source does not support belongs with §10.7.4's
challenge states, which are Phase 3. What exists is the per-user observation half, and
saying so is what stops a later reader taking it for a corpus assertion.

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
gnosis challenge --class coverage \
  --rationale "the quote is about connection retries; the claim is about request retries" \
  --by human:dana c/01932b7c-…-retry-budget.md --apply
```

#### 10.7.1 Four Classes, Ordered by What Settles Them

The class is not a severity and not a topic. It states **what kind of thing would
resolve the dispute**, which is what decides whether a person needs to be involved
at all.

| Class             | The reader is asserting                                               | Settled by                      |
| ----------------- | --------------------------------------------------------------------- | ------------------------------- |
| `replay`          | "I checked the archived source and the quote is not there"            | **re-running the check**        |
| `contradiction`   | "this claim conflicts with that one, and nothing noticed"             | §10.2 predicates, then a person |
| `coverage`        | "the evidence does not support the scope the claim asserts" (§17.3.1) | a person                        |
| `rung`            | "the claim is causal and its support is observational" (§17.3.1.1)    | a person                        |
| `dimension-drift` | "this subject's values changed dimension" (§5.8.2.1)                  | a person                        |
| `scope`           | "the stated limitations are incomplete" (§17.2)                       | a person                        |

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

**The operative form of the rule is reliance, and the committed/cached line is how
to recognise it.** The question that decides a new case is not *is this a decision*
but **does later work have to rely on this?** — for handoff, replay, authority,
automation, or evidence. The two agree everywhere this specification has already
applied them, and reliance settles the cases where "decision" is genuinely arguable.
A fetch record looks like an observation and is committed, because a quotation
relies on it; `checked.jsonl` also looks like an observation and is not, because
nothing relies on when somebody last looked. An audit row records a real decision
and is still cached, because it is *this machine's* account and no later work
depends on it — the decision it describes is already committed elsewhere, in the
document that landed.

So: ask what breaks if this is missing on a fresh clone. If the answer is "a claim
can no longer be checked", it is committed. If the answer is "somebody re-does a
cheap thing", it is cached. The formulation is borrowed from `haft`, an independent
implementation of the same problem, and is recorded in the manifesto's `agent-green`
survey.

#### 10.7.5 What This Does Not Add

Challenge is a route into the existing finding lifecycle, not a parallel one. It
resolves through `adjudicate`, appears in `audit`, and obeys the same tiers.
Nothing here is a new adjudication mechanism, a new severity, or a new state — the
additions are one frontmatter family, two cached columns (`challenge_class`,
`opened_by`), and one check.

## 11. Search

### 11.0.0 Retrieval Is Not Evidence

Stated first because everything below is about making things findable, and
findability is the property most often mistaken for a stronger one.

A search hit is not evidence that a claim is true. A document existing in the corpus
is not the corpus having checked it. A link resolving is not the target supporting
the source. A ranked list is not an argument, and its order is not a claim about
importance — it is a bm25 score with a title weight, and §6.2.1 already says the
selector it feeds is biased in a named direction.

The general form, borrowed from the First Principles Framework and worth the whole
sentence: *a publication carrier does not become its subject, and a readable view
does not become evidence, assurance, permission, decision, architecture, or work
without the corresponding exact relation and test.*

gnosis has exactly one relation that makes something evidence, and it is §9.4's:
a quotation appearing in archived text under `textnorm.Fold`, at or above
`MinPassageWords`. Everything else a read path returns is navigation. This matters
because an agent consuming `--jsonl` cannot tell the difference unless the envelope
does, and the envelope does not — a `search` result and an adjudicated claim arrive
in the same shape. Until §17.3's evidence rendering exists, the burden is on the
reader, and this section is where they are told so.

### 11.0 Against Enabling Semantic Search

The reranker below is optional, and the reason to expect it stays optional is worth
recording, because it is the only *measured* claim available about the scale these
systems run at.

From months of daily operation of a comparable wiki: *"At wiki scale, you do not
have a retrieval problem. A curated wiki is 50,000 to 100,000 tokens. That is small.
Grep plus read finds the right note faster and more predictably than an embedding
lookup, with no vector database to host, no index to keep fresh, and no 'why did it
return that chunk' debugging. Vector search earns its keep at hundreds of thousands
of documents."*

FTS5 is the baseline here for the same reason, and it already sits well past grep.
The bar for turning on embeddings is therefore not "it might help" but a measured
retrieval failure that FTS5 cannot fix — and the miss log (§6.4) is what would
supply that evidence, since it records the queries the deterministic path did not
answer. Enabling a reranker before the miss log says why is solving a problem this
corpus has not been shown to have.

**The evidence above is one-sided, and the other side has numbers.** A surveyed
agent-memory system reports 94% and 91% on LongMemEval — a benchmark for long-term
conversational recall — running four retrieval strategies in parallel (vector,
BM25, graph traversal, temporal) fused by reciprocal rank and reranked by a
cross-encoder. That is a measured claim that the architecture this section declines
works well at what it is built for.

Those figures are quoted as reported and **not as comparable to the benchmark's own
published results**, because the grader diverges from the paper's in ways the vendor
documents itself: the abstention category is not implemented, the fallback grader is
borrowed from a different benchmark and instructed to be generous, and the judge
returns reasoning-then-label where the paper forces a single token. None of that makes
the number dishonest and all of it makes the number local. It is recorded here for the
reason a one-sided section should not stand, not as a figure this specification would
put in a finding — which is itself the general rule: **a retrieval score is a function
of its grader, and the grader does not travel with the score.**

Three things make it not decisive here, and stating them is the point of naming it
at all rather than leaving the section to cite only what agrees with it.

The **task is different**: recalling what was said across a long conversation is not
adjudicating whether a claim is supported. A corpus's hard problem is not finding
the document, it is knowing whether to believe it, and no retrieval score addresses
that. The **scale is different**, per the measurement above. And the **failure modes
are not symmetric**: a memory system that returns the wrong passage answers a
question badly, while a corpus that returns the wrong passage under an
authoritative frame supplies a citation somebody will build on.

But the honest form of this section's position is *we accept a retrieval ceiling in
exchange for a retrieval path a reader can audit*, not *there is nothing to give
up*. If the miss log ever shows FTS5 failing on queries a fused reranker would have
answered, that is the trade coming due, and §11's optional reranker is where it
gets paid.

#### 11.0.1 Snippets Are Rendered, Not Excerpted

A search result's snippet is rendered from the document body at query time — code
blanked, headings and inline links reduced to their text, whitespace collapsed —
rather than taken from FTS5's `snippet()`.

The reason is a constraint that pulls both ways. FTS5 excerpts the *indexed* text,
which is the document as written, so a hit beside a link renders most of its width
as `[Timeout](/c/01932b7c-…-timeout-policy.md)` and the reader gets an identifier
instead of a sentence. The obvious repair — strip markdown before indexing — is
worse: someone searching for a slug should find it, and the index is the only thing
that can answer that.

So the body is **indexed as written** and the snippet is **re-derived for reading**.
That the two no longer share offsets is the point rather than a cost: an
offset-mapped snippet would have to hold rendered text and indexed text in
correspondence forever, and a re-derived one holds nothing. A comparable
implementation reached the same design from a different direction, having measured
`snippet()` as the slow part of its search path.

#### 11.0.2 the Trigger in §11.0 Names an Instrument That Cannot Fire It

§11.0 closes by committing to a condition: *"If the miss log ever shows FTS5 failing
on queries a fused reranker would have answered, that is the trade coming due."* The
commitment is right and the instrument is wrong, which is worth stating here rather
than quietly repairing there, because the mistake is the ordinary one.

**The miss log records queries the deterministic path did not answer** (§6.4). A query
answered *wrongly* — a confident, well-ranked, incorrect hit — is not a miss. Nothing
fell through, no fallback fired, and no row is written. So the miss log is blind to
false positives, and a false positive is the failure that matters here: §11.0.0
already says a ranked list is not an argument, and the case this section is defending
against is a reader citing the wrong passage under an authoritative frame. **The
trigger as written cannot fire for the failure mode it exists to catch.** The
instrument is also self-referential — what counts as answered is decided by the path
being measured — which is the same defect in a second form.

Measuring it needs a ground truth the corpus does not currently hold:

```text
standards/retrieval-cases.toml   query, expected concept ids, note
```

A small committed set of labelled queries with known-correct target concepts,
**including queries whose correct answer is that the corpus holds nothing** — the
abstention cases, which are exactly what the surveyed vendor's harness dropped. Graded
by exact concept-ID match: a pure predicate over a string, no judge, no model, exactly
reproducible, and gradeable by `gnosis search --jsonl` with no new subsystem. It
yields recall@k *and* an abstention rate, and both are properties of the corpus at a
commit rather than of whoever ran the query.

Three constraints, so this does not become the score §17 forbids:

- **It is a finding surface, not a gate.** No threshold promotes or blocks anything.
  A case that fails names the query and the concept it should have returned.
- **The cases are authored when a real query disappoints**, not invented up front. A
  fabricated case set measures the imagination of whoever wrote it.
- **It is Phase 4**, with §11.4's reranker, because it is the reranker's admission
  evidence and there is nothing to admit before then. Until it exists, this section's
  position rests on principle plus one testimonial, and §11.0's opening sentence
  should be read as the claim it is.

##### The Instrument, Once Built

`standards/retrieval-cases.toml` and `gnosis search --cases`. Four things this section
did not settle, three of which it got wrong.

**Cases assert on titles, not on concept identifiers.** Identifiers are assigned per
corpus, so a case file naming them is unportable: unreviewable by anyone reading it,
unliftable to another bundle, and a failing case becomes an archaeology exercise. A
title is what the person authoring the case was actually looking for. This is the
correction §12.53's competency-question entry supplied, and it is why the two merged
into one instrument rather than two — they are the same thing at two grains, and two
files to author cases in is one too many.

**Extra results are not a failure.** The grader requires every expected title and
tolerates anything else, because a corpus that grows a second relevant document has not
regressed — and a case that failed on that would train an author to delete the case
rather than read the result. This measures coverage and never precision.

**There is no pass rate, and that is not an omission.** §17 forbids presenting a count
as health, and a retrieval percentage is the most tempting such number in the system:
it looks like progress and it rises when a failing case is deleted. Cases hold or fail,
and the report says how many of each.

**A case requires a `why`.** §6.2's argument for a threshold's rationale applies here to
a different artifact: a case with no account of the disappointment that produced it is
one a later reader deletes when it fails, because they cannot tell a real expectation
from an invented one. The file ships empty, an empty suite reports that it examined
nothing rather than passing, and the reason to build the instrument before the data is
that a disappointing query is unrecoverable evidence — it happens, it is noticed, and
with nowhere to write it down it is gone by the next day.

### 11.1 Progressive Disclosure First

Karpathy's observation is that `index.md` alone works at ~100 sources and
hundreds of pages, and avoids embedding infrastructure entirely. OKF §8 makes
that a spec'd artifact. `gnosis` therefore treats the index as tier one of
retrieval, not a fallback: `gnosis search` consults `index.md`-equivalent
metadata (title, description, tags, type) before touching the body index.

#### 11.1.1 Two Grains, One Query Language

`gnosis search` answers at document grain by default and at claim grain under `--claims`,
which queries `claims_fts` — one row per claim that carries a lead (§5.5.3).

**A flag rather than a second command**, for the reason `--cases` is one: the same index,
the same FTS5 syntax, the same bound. Two commands would be two paths to one query, and
they could then disagree about what the corpus answers — including about whose mistake a
malformed expression is, which a reader must never have to learn per command.

**The lead is the whole result** (§17.4 makes it "the unit of retrieval in §11"), so a
claim hit is legible without opening its document. `claims.title` and `claims.description`
are not rendered, because nothing writes them and displaying an empty column advertises a
capability the extractor does not have (§5.5.3).

**A claim search reports what it could not reach.** Extraction fills leads a document at a
time, so a corpus part-way through holds claims that cannot match at any ranking. The
count of them travels with the results — in the struct, not beside it, so that a caller
cannot drop it — and it is computed whether or not anything matched: a shortfall that
appeared only alongside hits would go silent in the case that most needs it. "No results"
and "no results, and forty claims were never extracted" send a reader to opposite
conclusions.

It is **unconditional in JSON and conditional in prose**. An absent field and a zero one
are indistinguishable on the wire, so the number is always emitted; a person told "0
claims carry no lead" after every query of a fully extracted corpus learns to skip the
line, and its absence is what makes its presence mean something. It is not a health
figure either way (§17): it measures how much of the corpus a query could not see, and it
falls as extraction proceeds rather than as anything improves.

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

**Five of the six shipped 2026-09-03, and `--tag` did not.** §5.5 records `tags` as a
table with no migration and no reader — "frontmatter carries tags authoritatively and
nothing reads a projection of them" — so the filter would need a projection nothing else
wants yet, and building one for a flag would be the stored-state-with-no-reader mistake
this specification has recorded seven times. The other five are one value rather than
five branches: five of them need the corpus rather than the index, which holds no
document type, no trust fold and no freshness, so a query that names any filter reads the
bundle once and a query that names none reads nothing.

**The subtree property is checked rather than asserted.** It holds on path *segments*,
so `c/retry` does not admit `c/retry-budget.md` — a string prefix would return a sibling
whose name begins with the same letters, which is a concept outside the restriction
delivered by a query that claimed to be restricted to it.

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

| Check                  | Deterministic? | What it reports                                                                                                                        |
| ---------------------- | -------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| `conformance`          | yes            | OKF §11 violations only                                                                                                                |
| `evidence`             | yes            | a sourced claim whose quote no longer validates                                                                                        |
| `warrant`              | yes            | an adjudicated claim with no `gnosis_warrant`, or a warrant with an empty `rationale`                                                  |
| `co-sign`              | yes            | an escalated claim missing a required co-signer and carrying no recorded override (§10.6)                                              |
| `stale`                | yes            | `today ≥ stale_after`, or a source unchecked longer than `staleness_days` — **the drift half belongs to `fetch --recheck`; see below** |
| `orphan`               | yes            | no inbound links — **see the applicability note**                                                                                      |
| `newly-orphaned`       | yes            | had an inbound link at baseline, has none now                                                                                          |
| `broken-link`          | yes            | unresolved link, reported **as a gap, never an error**                                                                                 |
| `duplicate`            | yes            | `Fold`-equal title or evidence set across documents; the post-merge step of §4.6.1                                                     |
| `conflict`             | partly         | §10 predicates; residue to `critic`                                                                                                    |
| `index-drift`          | yes            | the derived index disagrees with the bundle (§5.1.2) — **not** `index.md`; see below                                                   |
| `log-format`           | yes            | `log.md` violates OKF §9 date-heading form                                                                                             |
| `command`              | yes            | a command named in the schema document that does not resolve                                                                           |
| `gap`                  | no             | concepts mentioned but lacking a page — prompt-emitting                                                                                |
| `durability`           | yes            | a **load-bearing** unprovable concept (§14.4.1); reports the peripheral count it suppressed                                            |
| `archive-orphan`       | yes            | an `evidence/` file no claim cites — a candidate for pruning                                                                           |
| `archive-budget`       | yes            | archive size over its warning threshold, largest files named                                                                           |
| `identity`             | yes            | the six reconciliation cases of §5.1.2; duplicate identifiers first                                                                    |
| `filename-drift`       | yes            | slug no longer matches the title; corrected on the next write                                                                          |
| `limitations`          | yes            | a normative concept carrying no `gnosis_limitations` (§17.2)                                                                           |
| `claim-anchor`         | yes            | a `gnosis_claims` anchor that no longer appears in its document (§5.5.1); `pos` goes NULL, never auto-repaired                         |
| `unanswered-challenge` | yes            | a reader-filed challenge older than the window in `standards/` (§10.7)                                                                 |
| `lead`                 | yes            | a normative claim whose `lead` restates background rather than the conclusion (§17.4)                                                  |
| `coverage`             | yes            | a claim asserting more strongly than its evidence supports (§17.3.1); warning, never a gate                                            |
| `rung`                 | yes            | a claim asserting intervention whose quotations only observe (§17.3.1.1); warning, never a gate                                        |
| `language`             | yes            | hedging, weasel words, meaningless comparisons, assuring expressions — lexical only                                                    |
| `subject-missing`      | yes            | a claim of a type whose `expects_subject` is true, carrying none (§5.8.2)                                                              |
| `subject-unknown`      | yes            | a claim's `subject` names a key absent from `ontology.toml`                                                                            |
| `dimension-drift`      | yes            | a subject whose claims are written in a unit its declaration does not describe (§5.8.2.1); warning, never a gate                       |
| `constraint-drift`     | yes            | a **pinned** `gnosis_constraint` the prose no longer supports (§10.2.1)                                                                |
| `constraint-coverage`  | yes            | per subject key: claims parsed and candidates lost; a backlog signal for the operator patterns                                         |
| `ontology`             | yes            | types no concept uses, undeclared types, deprecated keys still in use                                                                  |

Four design notes:

**`index-drift` is about the database, and an earlier row here said otherwise.** It
compares `gnosis.Reconcile(Observed(docs), idx.Rows)` — the derived index against the
bundle — which is what §12.1's enforced row has always said. The row above used to read
"`index.md` differs from what would be generated", describing an intent no check has and
sending a reader looking for a checker that does not exist. **The generated region of
`index.md` is `gnosis schema --check`'s business, not a lint check's**, for the reason
§5.7.1 gives: the marker contract belongs to the class of reserved root files, and one
command maintaining that class is what stops the fail-closed rule being implemented
twice.

**`stale` reports half of what it names, and the other half lives in the command
that has the network.** Comparing archived text to upstream requires a fetch, and
`lint` does no network — §4.6 argues that a reader must not require the writer, and
requiring the network is the same argument. So the check implements the date
comparison and the check-age window, and nothing more.

The drift half is `fetch --recheck` (§14.3.2), which already has the connection and
already writes `checked.jsonl`. That is not a workaround for the split; it is where
the work belongs, and the split is what keeps `lint` runnable on a train. The two
halves report in different vocabularies on purpose: `lint` answers "is this old",
`--recheck` answers "does upstream still say it", and a single word covering both
would be the collapse §14.3.2 exists to prevent.

A source whose upstream has not been compared is `unknown` (§14.3) to `lint` and
`drift-unchecked` to `--recheck` — a real answer rather than a gap either way, and
the same shape `scan` uses when it reports the stages it did not run.

**Regression-relative, not absolute.** `coherence` reports
`NewlyOrphanedEndpoints` *and* `NewlyCoveredEndpoints`, with `BaseAvailable`
distinguishing "no baseline" from "zero" — the same distinction
`skillet/timeseries` preserves as `Verdict.Compared`. `gnosis lint --since <ref>` gates on what this change made worse, which is the only gate that tolerates
a corpus that is imperfect but improving.

**Applicability is derived, not declared.** `coherence`'s `Convention bool` is
true only when the corpus demonstrably uses the pattern being checked, and it
skips promotion when false. `orphan` is meaningless in a corpus with no links
yet; `gnosis` derives that rather than asking for a flag.

One correction to the attribution, recorded 2026-08-22 because the citation is
doing work it cannot support: **`Convention` is a description of `coherence`'s
code and has never been built anywhere in this family.** `gnosis`'s
`internal/lint` is the only implementation of the idea, and it is a fuller one —
`Check.Applies` returns `(bool, string)` and every skip lands in
`Report.Skipped` as a `Skip{Check, Reason}`, so the *reason* is a first-class
output rather than an inference from a missing finding.

That turned out to matter beyond this section. `skillet` had an open decision on
whether to name a general `Applicability` type, counting `Convention` as one of
two members; counting properly resolved it as **a rule rather than a type**,
because the five real sites across `skillsaw`, `adh`, and `gnosis` each suppress
a different thing — a deduction, a whole check, or nothing at all — deliberately,
because their output shapes differ. The rule the family adopted is this
package's own sentence, quoted from `internal/lint`:

> A check that silently declines to run is indistinguishable from a check that
> found nothing.

with the corollary that **what** is suppressed is the consumer's choice and the
reason is not. `gnosis` is the origin and already conforms; the obligation is
recorded here so a later refactor does not treat `Report.Skipped` as optional
detail. Should a second tool ever emit a check report, `Skip{Check, Reason}` is
the unit that promotes to `skillet` — not an `Applicability` type.

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
`conflict` and `constraint-drift` both skipped for want of the operator patterns.
The three vocabulary checks named in an earlier draft of this sentence are built
now, and they skip on the same principle: a bundle with no `ontology.toml` is told
so by name rather than passing quietly.

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

### 12.0.1 a Reader Is Told When the Cache Is Behind

`show --body` reads the file and `search` reads the indexed copy. Both are right — the
file is the truth and the index is what was searched — and a document edited since the
last rebuild renders fresh text while a search still matches the old, with nothing
saying so.

`index.Detail` carries the `content_hash` the table already stored, and `show` compares
it against the file it just read. Two limits, both deliberate: it is reported only with
`--body`, because without the text on screen there is nothing for the divergence to be
about and a note attached to nothing is noise; and an absent hash reports nothing,
because a document indexed by an older build is not evidence of divergence and would
otherwise put a rebuild note on every document in the bundle.

______________________________________________________________________

### 12.1 What Is Actually Enforced

This specification carries **51 MUSTs and 12 MUST NOTs** and no SHOULDs. It has
eleven named lint checks and a handful of gates. So `convention` is the
overwhelming default and `code` is the exception, and the useful table is the
short one: the rules something actually checks, and what checks each. **Anything
absent from this table is convention by definition**, which keeps the unenforced
set countable without enumerating sixty-three rules.

**A `gate:` prefix marks the promote gate's signals**, which share a namespace with nothing. The prefix was added on 2026-08-27, when the `evidence` lint check took a name the gate's evidence signal already had and the table could not express both — `duplicate` and `gate:duplication` had been one letter apart for longer. Two mechanisms answering at two moments deserve two names.

**Fixable** is what a reader can do about a finding, from `finding.Action`: *guided*
means a tool could propose the fix and a person confirms it, *human* means only a
person can decide. A dash is a rule enforced by something that is not a lint check —
a gate, a loader, the type system — where the question does not apply because there is
no finding to fix.

The column is declared on each check and asserted in both directions: an action a check
emits and did not declare fails, and one it declares and never emits fails too. It is
last in the table on purpose, because the test that walks this table reads the enforcer
by column position, and inserting a column earlier would change what it walks while
still passing.

**A declared action is not a promise of a fixer.** There is no `--fix`. This column says
what a fixer *could* do, which is what a reader planning an afternoon needs; building
one is a much larger decision, and shipping it inside a documentation column would be
shipping it by accident.

That inversion is deliberate. Tagging every rule `[code]` or `[convention]`
inline was the first instinct and is the worse mechanism: sixty-three tags are a
larger maintenance burden *and* the one that rots invisibly, because a rule that
gains a checker keeps reading `[convention]` and nothing notices. A table of the
enforced rules puts the edit at the moment somebody is already editing both.

| Rule                                                                     | Enforced by            | Emits                                               | Fixable       |
| ------------------------------------------------------------------------ | ---------------------- | --------------------------------------------------- | ------------- |
| A document MUST declare a `type` (OKF §4.1)                              | `conformance`          | `conformance`                                       | guided        |
| One identifier MUST NOT be carried by two documents (§5.1.2)             | `identity`             | `identity`                                          | guided, human |
| The index MUST agree with the bundle (§5.1.2)                            | `index-drift`          | `index-drift`                                       | guided        |
| A link MUST resolve, or be reported as a gap (OKF §6.1)                  | `broken-link`          | `broken-link`                                       | human         |
| A document SHOULD have an inbound link (§12)                             | `orphan`               | `orphan`                                            | human         |
| `log.md` entries MUST use the date-heading form (OKF §9)                 | `log-format`           | `log-format`                                        | guided        |
| A document MUST record the conventions it was written under (§5.5.1.1)   | `schema-version`       | `schema-version`                                    | human         |
| A document MUST NOT ship placeholder markers (§12)                       | `placeholder`          | `placeholder`                                       | human         |
| A heading MUST NOT be followed by nothing (§12)                          | `empty-section`        | `empty-section`                                     | human         |
| A claim's `archive_paths` MUST exist in tier 0 (§5.5.1)                  | `archive-path`         | `archive-path`                                      | guided        |
| Tier 0's store and its ledger MUST account for each other (§4.3.1)       | `archive-closure`      | `archive-orphan`, `archive-unrecorded`              | human         |
| A claim's anchor MUST appear in its document, once (§5.5.1)              | `claim-anchor`         | `anchor-absent`, `anchor-collision`                 | human         |
| A claim of a type expecting a subject SHOULD name one (§5.8.3)           | `subject-missing`      | `subject-missing`                                   | human         |
| A claim's `subject` MUST resolve to a declared key (§5.8.2.1)            | `subject-unknown`      | `subject-unknown`                                   | human         |
| Evidence MUST support the scope a claim asserts (§17.3.1)                | `coverage`             | `coverage`                                          | human         |
| A causal claim MUST NOT rest on observational evidence alone (§17.3.1.1) | `rung`                 | `rung`                                              | human         |
| A subject's values MUST stay in the dimension it declares (§5.8.2.1)     | `dimension-drift`      | `dimension-drift`                                   | human         |
| A pinned `gnosis_constraint` MUST match its prose (§10.2.1)              | `constraint-drift`     | `constraint-drift`                                  | human         |
| A normative claim's lead MUST state its conclusion first (§17.4)         | `lead`                 | `lead`                                              | human         |
| A normative concept MUST declare what it does not cover (§17.2)          | `limitations`          | `limitations`                                       | human         |
| Two documents MUST NOT hold one subject after a merge (§4.6.1)           | `duplicate`            | `duplicate-title`, `duplicate-evidence`             | human         |
| A command named in AGENTS.md MUST resolve (§5.7)                         | `command`              | `command`                                           | human         |
| A claim's quotation MUST remain in the file it names (§9.4)              | `evidence`             | `evidence`                                          | human         |
| Prose SHOULD name what it compares and attributes (§10.3)                | `language`             | `language`                                          | human         |
| A concept filename's slug SHOULD match its title (§5.1.1)                | `filename-drift`       | `filename-drift`                                    | automatic     |
| One claim MUST NOT rest on two versions of one source unexamined (§10.2) | `conflict`             | `evidence-divergence`                               | human         |
| Two claims about one subject MUST NOT bound it disjointly (§10.2)        | `conflict`             | `conflict`                                          | human         |
| An open challenge MUST NOT age past its declared window (§10.7.3)        | `unanswered-challenge` | `unanswered-challenge`                              | human         |
| An adjudicated claim MUST record why it was decided (§10.6.4)            | `warrant`              | `warrant`                                           | human         |
| An escalated claim MUST be co-signed or record an override (§10.6)       | `co-sign`              | `co-sign`                                           | human         |
| An unprovable concept the corpus leans on MUST be reported (§14.4.1)     | `durability`           | `durability`, `durability-peripheral`               | human         |
| The operator patterns SHOULD read what claims state (§10.2.3)            | `constraint-coverage`  | `constraint-coverage`                               | human         |
| The vocabulary and the corpus MUST agree on types (§5.8)                 | `ontology`             | `type-undeclared`, `type-unused`, `type-deprecated` | human         |
| A dated claim MUST be revisited after `stale_after` (§14.3)              | `stale`                | `stale`                                             | guided        |
| The derived index MUST match what the migrations declare (§5.5)          | `schema-shape`         | `schema-shape`                                      | —             |
| Archived text MUST pass §9.3's scan                                      | `archive.Gates`        | archive `reject_reason`                             | —             |
| A candidate MUST pass §9.3's scan                                        | `gate:security`        | gate verdict                                        | —             |
| Every offered quotation MUST validate against tier 0 (§9.4)              | `gate:evidence`        | gate verdict                                        | —             |
| Every source MUST be followable or declare it is not (§4.3)              | `gate:provenance`      | gate verdict                                        | —             |
| A title MUST NOT already be held (§4.6.1)                                | `gate:duplication`     | gate verdict                                        | —             |
| A body MUST NOT exceed `hedging_max` softening phrases (§9.5)            | `gate:hedging`         | gate verdict                                        | —             |
| Every mutation MUST write a verified audit row (§15)                     | `Writer.Audit`         | ECONFLICT / EINVALID                                | —             |
| Every bundle write MUST hold the writer lock (§4.6)                      | `bundle.Writer`        | compile error                                       | —             |
| A `standards/` value MUST carry a rationale (§6.2)                       | `standards.Load*`      | EINVALID                                            | —             |
| A §9.3 pattern MUST discriminate (§9.3)                                  | `scan.LoadRules`       | EINVALID                                            | —             |

**What the table cannot catch, and what does.** It is checked in both directions against
the registry, so a row claiming a checker that was deleted fails and a checker nobody
documented fails. Neither direction sees a check that is **specified in §12's list above
and never built**: it is absent from the registry and absent from this table, and the two
absences agree. Counted on 2026-08-27, thirteen were in that state — `co-sign`, `command`,
`constraint-drift`, `duplicate`, `durability`, `evidence`, `filename-drift`, `gap`,
`language`, `lead`, `limitations`, `newly-orphaned`, `unanswered-challenge` — and
`coverage` was the fourteenth until it was found by accident while asking a different
question.

**Counting them was not the useful act; splitting them was.** Measured against what
`lint.Snapshot` carries, the twelve remaining after `coverage` divide into checks that
need one field, checks that need a subsystem, and one that is not deterministic at all —
and §12's table calls the last of those `no` in a column nobody had read against the
backlog. Six had been called "unbuilt" when three were unbuildable and one was
undecidable, which is a distinction this specification has now had to draw four separate
times.

`TODO.md` is where that gap is recorded, because it is a *backlog* fact rather than a
specification one: §12's list is correct about what the corpus should check, and the
absence is work nobody has scheduled. What was wrong was that nothing said so.

**One entry in that list is built and not registered.** `archive-budget` runs through
`doctor`'s environment rather than through `lint.Checks`, because it reads a measured
archive size rather than a Snapshot. It is enforced; it is simply enforced elsewhere, and
a reader counting the registry would conclude otherwise.

**The table is checked against the code, in the direction that drifts.** A test
walks `lint.Checks()` and asserts that every check named here exists and that
every check in the registry is named here. The first direction catches the failure
that matters — a table claiming enforcement that was deleted — and the second
catches a checker nobody documented. That buys the non-drift property of a
generated table at the cost of a hand-maintained one, without needing stable
identifiers for all sixty-three rules.

The same walk asserts the **Emits** column, and that is not cosmetic. `Category`
is set two ways in `internal/lint` — string literals inside a `Run` body, and the
derived `resolutionCategory(kind)` — so the emitted vocabulary is **not
enumerable by grep**, and this column was the only description of it. Each check
now declares its categories, the walk compares the declared set against this
table, and a check firing on a fixture is asserted to emit only what it declared.

______________________________________________________________________

### 12.2 What It Means for a Claim to Be Used

`orphan` reports a document nothing links to, which is reliance at the document grain.
The claim grain is the question a corpus wants answered — *did anybody ever use this?* —
and it has five candidate meanings. Recording which one is meant, and which are refused,
is what keeps somebody from building the expensive one by default.

**Citation is refused.** A link cannot name a claim: `links` carries
`source_claim_id` and no target, and §5.6 makes a presented path a view of a *document*.
Claim-level citation would need a schema column and a new href form, contradicting that
design for the sake of a report — which is the tail wagging the dog.

**Adjudication and verification are not use.** A warrant weighing a claim is the
strongest thing that could be meant, and it is rare by construction: almost no claim is
adjudicated, so the report would name nearly the whole corpus and teach its reader to
skip the category. Verification is about *truth* — a claim can be verified and never
consulted, or consulted constantly and never verified. `verifications(claim_id, by, at)`
having no writer is a separate gap and not this one.

**Retrieval is the meaning worth having, and it is a tracer.** §6.4's argument applies
unchanged: a claim returned to nobody leaves no trace, and a determinism claim about
usefulness is untestable while the misses are invisible. It measures **reach** rather
than reliance — a result returned and ignored counts — and saying so is the honest limit
rather than a reason to reach for a stronger signal that cannot be had.

Two properties follow and both constrain it. It is **per-user**, like `checked.jsonl`
(§4.3.1): two colleagues at one commit have retrieved different things and are both
right, so it is derived state and never committed. And it reports **a count over a
window, never a fraction** — §17 forbids presenting a count as health, and a
proportion-of-the-corpus-ever-retrieved figure is the most target-shaped number this
specification could produce.

**What the originating evidence actually motivated was throughput, and that is built.**
The surveyed loop shipped four of eighty proposals over three months and could not see
it. gnosis's equivalent is `audit --gained` and `audit --outstanding`: work that landed,
and work somebody was asked about and abandoned. That is the failure, at the grain the
evidence supports. The claim-level version is a further question rather than the same
one, and conflating them made an unbuilt ambition look like a demonstrated gap.

**The trigger fired on 2026-08-27.** Extraction writes `claims.lead`, `claims_fts` holds
rows, and `gnosis search --claims` returns claim-grain results (§11.1.1) — so a tracer now
has something to trace. What remains is the log and the report, under the two constraints
above: per-user and uncommitted, a count over a window and never a fraction.

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

#### 14.1.1 Two Populations of Actor, and Only One of Them Is Gnosis's

`gnosis.Actor` is a **closed** three-kind enum — `human:` / `agent:` / `check:` —
and `ParseActor` refuses anything else. OKF §7's grammar is
`<producer>/<version>`, `human:<id>`, and `process:<id>`. Two of those three do not
parse here, and `agent:`/`check:` are not OKF forms. Stating the divergence rather
than discovering it when §14.1 is built:

**The closed enum is correct and stays, for actors gnosis mints.** §10.6.4 counts
*distinct human actors* to decide whether a review tier amplified anything, and a
kind that could pass for a person makes that count wrong in the direction that
flatters the corpus. An open actor grammar cannot make that guarantee, so the strict
type is load-bearing exactly where it is strict.

**It is the wrong reader for frontmatter that arrived from somewhere else.** A
concept carrying `verified: [{by: reference_agent/gemini-2.5-pro}]` is OKF-valid and
`ParseActor` rejects it — and §11 forbids rejecting a concept for the shape of an
optional family. So the fold above does **not** run over `gnosis.Actor`. It runs over
the raw strings, and it asks one question, which is the only question OKF §7 says a
trust classifier needs:

> Consumers that classify trust (§5.3) key off the `human:` prefix, so producers
> MUST use it for hand-authored or human-confirmed content.

Everything that is not `human:`-prefixed is non-human for tier purposes, whether it
is `agent:ingest`, `process:finance-nightly`, or a producer string gnosis has never
seen. **An unrecognised actor is never an error and never promotes a tier.** That is
strictly more permissive than `ParseActor` and strictly less capable — it cannot say
*which* non-human wrote something — which is right, because the tier does not depend
on that and a reader who wants it can read the field.

The two must not be merged. Widening `Actor` to accept OKF's forms would give up the
property §10.6.4 depends on; narrowing frontmatter reads to `Actor` would reject
conformant documents. §18 carries the conformance test that pins both.

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

#### 14.3.0 Two Clocks, and Only One of Them Is the Claim's

`stale_after` and `staleness_days` look like the same knob at two grains, and
implementing them proved they are not. They measure different things, and
conflating them would reintroduce the read-time dependence the paragraph above
just argued against.

**`stale_after` governs the claim.** An author writes a date because the thing
being asserted has a horizon — a version that will ship, a policy under review, a
number that gets restated annually. It is a statement about the world, so it is
absolute, and it outranks having been checked: a source can be verified byte-identical
and still be past the date its author said to revisit it.

**`staleness_days` governs the check.** It asks how long ago *this user* last
compared an archived source against upstream. That is a statement about an
observation, and an observation is already per-user and already timestamped, so
measuring it relatively costs nothing: the answer does not depend on when the
document is read, only on when the fetch happened, which is recorded.

The distinction decides where the number may be applied. Applying
`staleness_days` to a *document* — treating an old document as stale because its
author set no date — would make a claim's status depend on when somebody read it,
which is precisely what an absolute `stale_after` exists to prevent. Applying it
to a *check* does not. So a document with no `stale_after` is never stale on its
own account; it is stale only when the sources under it have gone unverified
longer than the window.

A document is exactly as verified as its **least recently checked** source. Taking
the newest check would let one re-fetch vouch for three sources nobody has looked
at, which is the same collapse §14.3 avoids one level down.

**One exemption, derived rather than declared.** The window does not apply to a claim of
an `episodic` type (§5.8.3.1). Its evidence is a record of a moment — a commit hash —
which is immutable by construction, so "these sources were last verified 40 days ago;
re-run `gnosis fetch`" is advice nobody can act on and the finding would never clear. The
exemption is read off the type rather than declared per document, because a document that
could opt out of the window would be a document that could opt out of §14.3.

**And it is reported at both grains, because those answer different questions.**
The document line is still the weakest of its claims, and a reader deciding whether
to trust the page gets exactly the verdict they got before. What the page could not
say is *which sentence* rests on the unverified source — one stale source marked
everything, which is the right conservative answer and the wrong useful one. So
`show` reports each claim's own freshness beside it, joined through the claim's
`archive_paths`, and both answers are available rather than one replacing the other.

Two consequences the implementation settled:

- **One measurement, used twice.** The document's state and each claim's go through
  the same function over different path sets. Computing them separately would let
  the two disagree, and the direction that matters is the dangerous one: a page
  reading fresher than a sentence it is made of.
- **A claim says *which* sources support it, and never how many.** A claim resting on
  four independent sources and one resting on one were indistinguishable in
  frontmatter: both carry `archive_paths`, and nothing said what those resolved to. So
  the claim's distinct source URIs are reported beside its freshness, and nothing sums
  them. Two reasons, and the second is the one that matters: four archive paths may be
  four versions of one page, so a count of paths would report one source as four; and a
  count of *sources* would still be the inheritance §1.1's local reductionism refuses,
  where corroboration becomes a number to compare rather than a set to examine.
  A claim whose paths resolve to no record reports nothing rather than an empty list —
  "cites a source tier 0 has no record of" is `lint`'s `archive-unrecorded`, and showing
  it here too would report one defect as two.
- **A declared date reaches every claim under it.** `stale_after` is a statement
  about what the document asserts (§14.3.0), and a claim is one of those assertions,
  so the date governs each of them. A per-claim report that consulted only check
  times would show a verified claim as fresh under a document its author had already
  asked to be revisited — the read-time dependence §14.3.0 exists to prevent,
  reintroduced one level down. A claim whose sources were never checked stays
  `unknown` rather than becoming `stale`: nobody looked, and a date cannot turn that
  into a verdict about the source.

Neither half is a finding when nothing has been checked at all. "The sources under
this document have never been verified" is true of every document in a corpus that
has just started fetching, and a warning true of everything teaches a reader to
skip the category. It is a *state* — `unknown`, which `show` renders — and the
four-state vocabulary exists so that it can be reported without being alarming.

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

**The objection to reporting-without-pruning, which is not answered here.** The one
field report available on running an LLM wiki daily for months puts it plainly:
*"The point of a forgetting curve is not the math. It is that something deletes."*
A corpus that only ever reports its unreviewed claims accumulates them, and notes
that accumulate become a graveyard — the reader stops opening the report long
before the corpus stops growing.

The rule above stands, because deleting a claim nobody has looked at is deleting on
the basis of attention rather than truth, and this corpus carries a team's
authority rather than one person's memory. But the objection identifies a real
failure mode and the mitigation is not yet designed. The likely shape is that
`stale --unreviewed` gains a companion that proposes *deprecation* rather than
deletion — `status: deprecated` is already in the model (§5.4), it is reversible,
it is visible in review, and it removes a claim from default retrieval without
destroying it. Recorded rather than specified.

#### 14.3.2 Upstream Drift Is Two Findings, Not One

`stale` today means *the archived bytes no longer match upstream*, and that is one
signal standing for two facts with opposite consequences.

A source is re-fetched and its sha256 differs. Either the passages this corpus quoted
are **still present** in the new bytes — the document was extended, reformatted,
re-hosted, or edited elsewhere — or they are **gone**. In the first case every claim
resting on that source is still supported and only the archive is behind; the work is
a re-fetch. In the second, a claim in the authoritative corpus has lost its support
upstream, which is the strongest signal §10 can receive short of a contradiction, and
it should reach a person. Reporting both as `stale` puts the cheapest maintenance task
and the most serious evidentiary event in the same bucket, and the bucket is sized for
the cheap one.

The second signal costs nothing to compute, because the machinery exists: run
`quotecheck` over the *new* bytes with the passages already recorded for that source.
So a drifted source resolves to one of:

| State               | Condition                                                       | Response                                         |
| ------------------- | --------------------------------------------------------------- | ------------------------------------------------ |
| `drift-benign`      | bytes differ, every recorded passage still matches under `Fold` | re-archive; no finding                           |
| `drift-unsupported` | bytes differ, at least one recorded passage no longer matches   | a finding per affected claim, naming the passage |
| `drift-unchecked`   | bytes differ, passages could not be re-checked                  | neither of the above may be asserted             |

The vocabulary is `ruflo`'s witness manifest, which pairs a whole-file hash with a
semantic marker and reports *drift* — *"acceptable, the codebase advanced"* — as a
state distinct from *regressed*, on the reasoning that a hash-only check *"would flag
every benign whitespace change as a regression."* The recorded passage is gnosis's
marker, and it is a better one than that project could use, because it was chosen by
whoever made the claim rather than by whoever wrote the check.

Three consequences worth naming:

- **`drift-benign` is not a downgrade of trust.** The claim was supported when
  admitted and is supported now. Rendering it as a warning would train readers past
  the state that matters.
- **`drift-unsupported` never rewrites or retracts anything.** It opens a finding.
  §9.6's accretion rule is unaffected: the corpus records that support was withdrawn
  upstream, and what to do about it is §10's.
- **Neither replaces the corruption check.** A passage failing against the *archived*
  bytes is corruption and fails hard (§4.3.1). This section is only about the archive
  disagreeing with upstream, where the archive is by definition still intact.

#### 14.3.2.1 What Implementing It Added to the Three States

Three things the table above does not say, each of which the code needed.

**A fourth state, because "the bytes differ" is a precondition and a precondition
in prose has no failure mode.** The table describes a source whose bytes moved,
which reads as something the caller must establish before asking. `gnosis.Drift`
takes both hashes and answers `drift-none` when they match, which makes the
function total: there is no way to call it wrongly and no answer it cannot give.
`DriftUnchecked` is still the zero value, so a state nobody computed asserts the
least of the four.

**Two ways a network error would have manufactured a catastrophe.** A fetch that
returns nothing — a 404 body, a redirect to a login page, a truncated read —
hashes to something that differs from the archive, and every recorded passage is
then genuinely absent from it. Handed to `quotecheck`, that reports
`drift-unsupported` for every claim resting on the source: the most serious event
this system can report, produced by a failed connection. An empty upstream is
`drift-unchecked`. So is a comparison where either hash is missing, on §14.3's
rule that an absent observation means never-checked rather than checked-in-1970.

**Anything unchecked blocks benign, and "no passages" is not vacuous agreement.**
A quotation too short to split yields `quotecheck.Unchecked`, and calling the
source benign on the strength of its neighbours would claim support for a claim
nobody verified. A source no claim quotes has nothing that could be found or lost,
so nothing was checked — which is what it says. The three answers cost different
amounts to be wrong about, and the order of the tests is the order of increasing
willingness to assert.

**Where it runs.** `fetch --recheck` takes its source list from tier 0 instead of
the command line and compares each recorded version against what it just fetched.
A source fetched twice is two comparisons, because each recorded version carries
the hash that was current when somebody quoted from it, and collapsing to the
newest would compare against a version no claim was ever validated against. The
passage comparison uses the text a quotation was validated against — the
extraction, for an `extracted` source — while the hash comparison uses the source's
own bytes, which is what tier 0 recorded. Confusing those two would report every
claim resting on an extracted page as unsupported.

Running it turned up one reporting defect worth recording, because it is a
consequence of §4.1 rather than a slip: a re-check that finds changed bytes
archives them, so the next re-check sees a second record for that URI whose text no
claim cites yet. Those accumulate one per run. They are counted rather than listed
in the human report — the machine envelope still carries every row — because a
settled corpus would otherwise grow a page of lines meaning "nothing happened"
around the one line that matters.

#### 14.3.2.2 the Verdict Is Stored on the Observation, Never on the Record

A verdict printed once and kept nowhere is a verdict a reader cannot have. `show`
reported `fresh` a week after a re-check had found that the source no longer contained
the passage the claim rests on — correctly, since freshness is about when somebody
looked, and silently, since nothing carried the other half.

**`checked.jsonl` is the home, and it is a type argument rather than a convenience.**
A fetch record's name is the hash of its own content (§4.3.1), so a field that varies
with a comparison somebody ran would re-record unchanged bytes: tier 0 growing because
somebody checked, which that section forbids. An observation is the opposite shape —
per-user, already timestamped, and *about* what this user saw when they looked. A
drift verdict and a git revision are two more things they saw.

Absent means unknown on both, and the zero values already say so: an empty revision is
"none, or not recorded", and an empty verdict is `drift-unchecked`. Lines written before
the fields existed parse unchanged, and that is the test.

**A re-check writes an observation for the version it did not fetch.** The verdict is
about the *recorded* copy, so it belongs on that copy's row — and this closes a gap
that existed independently of drift: a re-check finding changed bytes used to record a
check for the new version only, so a claim resting on the old archive path still read
as last verified whenever it was first fetched, however recently its quotations had
been confirmed against upstream.

`show` prints both signals, adjacent and separate. That pairing is the point: a claim
can be freshly checked and have lost its support, and either line alone is a half-truth.

#### 14.3.2.3 What a Source Costs to Keep Current

The paragraph below predicted that a source churning benignly wants a shorter
`stale_after`, called it a derived default, and said the data would accrue "from the day
this section is implemented". It has, and the register is `gnosis audit --churn`.

**It needed no new field, which is why it looked like a missing feature.** A source
fetched twice has two records (§4.1), so the record count per source already *was* the
number of times its bytes moved. Nothing had asked the question.

Each row carries four counts and they are never summed: versions whose passages held,
versions that lost one, versions nobody has compared, and the one upstream still
matches. Six benign moves and one withdrawn passage are different events, and no number
of the first adds up to the second — which is §14.3.2's whole argument, applied to a
summary rather than to a verdict. `drift-none` is counted apart from `drift-unchecked`
for the same reason one level down: compared-and-unmoved is the opposite answer from
not-compared.

**A count, never a cost.** §17 forbids presenting a count as health, and "this source
moved six times" is the observation an effort estimate would rest on rather than the
estimate. A source that has not moved is not a row at all: a register with a 1 beside
every source would bury the handful that moved among the hundreds that did not.

And the report is `ok` rather than findings. A source that moves is doing what sources
do; withdrawn support is already a finding where it happens, at the re-check that found
it.

A source that produces `drift-benign` repeatedly is a source that churns without
changing what it says, and its claims want a shorter `stale_after` than a source that
never moves. That is a derived default rather than a declared one and it is Phase 4;
recorded here because the data to compute it accrues from the day this section is
implemented, and a signal nobody stores cannot be derived later.

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
blocks on centrality.

**The cut is the single boundary, and the median above is not a second one.** The
table describes peripheral as *below the median* and load-bearing as *at or above a
declared cut*, which is two numbers with a gap between them — and there are only two
treatments, reported or suppressed, so a document in the gap would belong to no
class. Deriving the median as a second threshold is the invented constant §6.2 exists
to refuse. So the declared cut decides both, the median is what the cut should be
*calibrated to*, and `standards/archive.toml` already records that as pending the
corpus's own distribution. Settled while building the check on 2026-09-02.

**A cut of zero declines rather than reporting**, which is not a detail of the
implementation: an in-degree of zero is at or above a cut of zero, so a corpus whose
standards did not load would otherwise have every unprovable document reported as
load-bearing. The check skips with a reason, and the reason says the file may declare
a cut and have failed to load — an incomplete `standards/archive.toml` is rejected
whole, so a reader can be looking at the value while the check sees none. A load-bearing weak claim is a prompt to go find a better
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
- **Audit every write.** One row per mutation in `.gnosis/audit.jsonl`: operation,
  actor, paths, content hashes before and after, standards hash, finding ids.
  `clu` records every write and can answer who did what when; so must this. An
  earlier draft named `skillet/auditlog` for this and that package is the wrong
  shape — it reads `results.tsv`, nine columns describing an optimization
  experiment. gnosis writes its own; the two share a word and nothing else.
- **Corruption and operational failure are different, and a reader must be told
  which.** Only malformed state, a checksum mismatch, or unreadable evidence is
  corruption. A failed read, a full disk, and a git subprocess that died are
  operational. Collapsing them sends somebody hunting for tampering when a volume
  unmounted, and — worse in the other direction — lets a genuinely corrupt record
  read as a transient failure worth retrying.

  The distinction is **legible rather than machine-checkable**, and saying so is
  more honest than the alternatives. `skillet/errs` carries five codes and none of
  them means "the bytes on disk are wrong"; adding a sixth for a single consumer
  is the kind of vocabulary growth skillet's own guidance argues against, and a
  second error type living in `internal/gnosis` would be a competing vocabulary
  for the same job. So corruption is `EINVALID` with a message that says
  corruption and names the offending line. That is not nothing: `EINVALID` already
  means *no retry of the same value will help*, which is the actionable half of
  the distinction, and the half a caller branches on. What it does not give is a
  programmatic test for tampering, and a corpus that needs one should get a sixth
  code at the point where the second consumer appears, not before.
- **Anything an agent can name is an execution surface.** A quarantined path
  arrives from a model's reply (§9.4) and is refused if it escapes the bundle; the
  general rule is that any string a reply supplies which later selects a file, a
  command, or a check must be validated against a closed set rather than used.
  Where a command is selected, the set is an allowlist and the default is refusal.
- **The audit trail is the one component nothing watches, so something must.** Every
  bullet above is enforced by a check; the trail that records the enforcement is
  written by the same process it is recording, and a write that silently fails leaves
  a corpus that looks correct and cannot show it. This is not hypothetical: a surveyed
  project's nightly ledger-append step *"silently failed for 5 consecutive nights with
  no alerting"* while every other stage of the same routine succeeded, and the gap was
  found weeks later by a person reading the file.

  Two mechanisms, both cheap, and neither of which is a second log:

  - **A mutation verifies its own row before reporting success.** The row is
    written, the tail is re-read, and a mutation whose row cannot be read back
    returns an error rather than an `Outcome` — the write happened and the record of
    it did not, which is a state a caller must be told about. Fail-soft here would
    reproduce the failure exactly.

    **This does not make a failed *append* fail its write, and the distinction is
    the whole of it.** The two look like one requirement and are two events. An
    append that returns an error is a known gap: nothing is hidden, the caller is
    told in three places, and failing the write would tell somebody to retry an
    operation that succeeded. An append that returns *success* with nothing on disk
    is the trail lying, and this is the only place it can be noticed. Fail-soft is
    right for the first and reproduces the failure for the second, which is why the
    code carries two fields and not one.

    It is stated over *mutations* rather than over the write coordinator on
    purpose. `init` and `index rebuild` append without going through it — they
    predate it — so verifying inside the coordinator alone would satisfy this
    sentence for two mutations out of four. The unverified append is not on the
    package's surface, so the compiler enforces the rest.
  - **`gnosis doctor` reports the trail's own health**: the count of malformed
    lines, named by line number.

    An earlier version of this bullet also asked for the newest row's timestamp
    against the newest commit touching the bundle, on the reasoning that a trail
    whose last row predates the last write is the observable form of the failure
    above. **Building it showed the comparison cannot mean that.** A person editing
    a document and committing it is the ordinary way a plain-text corpus is used,
    and it produces a commit newer than any audit row with nothing having gone
    wrong — so the check fires on the normal workflow, which is worse than not
    checking. Git commits are not gnosis's writes, and no comparison against them
    distinguishes a hand-edit from a lost row.

    Nothing is lost by dropping it, because the timestamp comparison was only ever
    a way to *infer* the failure after the fact, and the bullet above detects it
    directly at the moment of the write. Both timestamps are still reported as
    context, because `Environment` exists so that a report pasted into an issue is
    self-contained — but neither produces a finding.

  **A malformed line is counted and named, never skipped.** The obvious reader —
  parse each line, ignore what does not decode — makes a truncated or edited trail
  read as a shorter one, which is the direction that flatters. `bundle.AuditTrail`
  returns the rows it could parse *and* the count and line numbers it could not, and
  `--jsonl` carries both, so a consumer cannot accidentally treat a partial trail as
  whole. Per the corruption bullet above, that count is `EINVALID` territory when it
  is non-zero and a reader asks for the whole trail; it is a reported number when they
  ask for a range.

  The two halves are a **value and a method**, not a value and an error. Go's
  convention is that a non-nil error makes the returned value untrustworthy, and
  this requirement is precisely that the rows stay usable while the damage is
  known — so the error channel keeps its one meaning, *the file could not be read*,
  and the damage is a field. `Trail.Whole()` is how a reader asks for all of it and
  the only place the count becomes an error. Two intermediate designs failed here:
  one dropped the unparsable lines and reported a short trail as whole, and one
  errored on the first bad line and returned no rows at all, so a single bad byte
  made the other 3,999 rows unreadable.

  `LoadChecks` reads a file of the same shape and deliberately keeps the
  fail-whole rule. The asymmetry is the point: a partial read of the trail is an
  incomplete answer about *history*, and a partial read of the check record is a
  wrong answer about the *corpus* — a source reads as never-checked when the record
  exists, which §14.3's four states exist to prevent.
- **Absence of a required record is itself recordable.** The trail today holds writes
  that happened. Where a decision was required and none was made — a promote that
  reached `needs_human` and was abandoned, a challenge opened and never resolved —
  there is nothing to enumerate later, because nothing was written. A surveyed event
  specification reserves a kind for exactly this, carrying the checkpoint that was
  missed and the remediation. gnosis does not need a new log for it: the states above
  are already committed frontmatter (§10.7.4), and what is missing is the *report* —
  `gnosis audit --outstanding`, which enumerates them. Recorded as the shape; the
  command is Phase 3, with §10.
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

`finding.Diagnostic` is `{severity, category, path, message, action}` on stdout as
JSON. `canonizer gate` can block on `gnosis` findings and vice versa, because the
severity model is shared.

**`Action` already answers "who acts", and this section previously proposed a field
that duplicates it.** `finding.Action` is `automatic` / `guided` / `human` —
respectively *a tool can generate the fix without asking*, *a tool can propose it
and a person confirms*, and *closing it needs judgment no tool here has* — with the
zero value meaning **nobody classified this**, deliberately distinct from `human`.
An earlier draft of this section listed `Diagnostic` without `Action` and then
proposed `certainty` on the grounds that severity does not say who acts. Severity
does not; `Action` does, and its three values are the same three:

| `agentsys` certainty | meaning              | `finding.Action` |
| -------------------- | -------------------- | ---------------- |
| HIGH                 | safe to auto-fix     | `automatic`      |
| MEDIUM               | needs context        | `guided`         |
| LOW                  | needs human judgment | `human`          |

So **gnosis adds no field to `Diagnostic` for this.** It populates `Action`. The
`certainty` vocabulary is kept here for one thing the mapping above does not carry
and §17 depends on: the concept of **requisite uncertainty** — a confidence
calibrated to what the system actually allows one to know. A finding claiming
`automatic` about something the corpus cannot determine is the overclaim §17 exists
to prevent, one level down. That is a rule about *when* `Action` may be set, not a
second field.

**`fix_class` is not a `Diagnostic` field either**, and never was: `AgentLint`'s
`guided` / `assisted` is per *check*, and it lives in `standards/evidence.toml`
beside that check's evidence (§6.5). A check's fix class is configuration; a
finding's `Action` is the instance. Conflating them would put a per-check constant
on every row it produced.

The general rule, since this section got it wrong once: **before adding a
classification axis to a shared type, check the axes it already has.** Three
overlapping answers to "who acts" is worse than none, because a consumer then has to
decide which one wins.

### 16.2 Manifests and Proofs

`gnosis proof create` binds corpus and tier-0 digests into a `skillet/proof` packet, so
`adh` can close an arc that touched the knowledge base under `no-proof-no-close`. Built
2026-09-03; `proof.Create` fits exactly, because it hashes bytes and records
repo-relative paths, which is what "binds the corpus" means.

**`gnosis manifest` is not built, and §8.5 records why**: `manifest.Diff` matches on
location and this corpus's locations are views. The sentence that stood here — "so
downstream tools consult it instead of rediscovering the tree" — described a real need
and named the wrong instrument.

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

#### 17.0.1 Read Paths That Cannot Refuse Are Not Trustworthy

Everything above governs the write path: a claim is admitted or it is not, a
finding blocks or it does not. The read path has no equivalent, and that is a gap
rather than a simplification.

`ask` retrieves context and emits a prompt. It has no way to say **"the corpus does
not support an answer to this"** — so a question the corpus cannot answer produces
the same shape of output as one it can, and the caller cannot tell which they got.
That is the failure §17 spends its length preventing, moved one surface over.

So: **`ask` MUST be able to refuse**, and a refusal is an ordinary outcome rather
than an error. It carries `status: blocked`, `reason: needs_human`, and names what
was missing — no claim on the subject, claims found but none with evidence, or
claims that contradict each other with no adjudication. The distinction that makes
this worth building is between *the corpus is silent* and *the corpus is
unresolved*, and only the second is a `conflict` waiting to be filed.

The principle is worth stating in the form the field has converged on: **a system
that never refuses has not demonstrated that it is careful, it has demonstrated
that it is not checking.** A confident answer assembled from nothing is the most
expensive output this design can produce, because it carries the corpus's
authority.

**Built 2026-09-03 as four states, and the fold is what refuses.** `gnosis.Answerability`
is `silent`, `unevidenced`, `unresolved` and `ready`, folded from the claims retrieved for
a question: retrieval returns a ranking and a ranking cannot refuse, so turning a ranked
list into an answerable decision is a separate, pure step. Each refusal carries its own
remedy, which is the clearest argument that they had to be three — only `unresolved` is a
conflict waiting to be filed, and only `unevidenced` is fixed by ingesting a source. A
refusal exits **0**: a non-zero code would make "the corpus does not know" indistinguishable
from "the command broke", which is this section's failure one surface further out.

One thing running it found that no test had. A retrieved claim carrying **no** passage
was reaching the prompt under a heading promising evidence, in a set where some other
claim was evidenced — so the rules said "answer only from the claims below" and one of
them had nothing behind it. Unevidenced claims are now left out of the prompt and their
number is reported, because a silent omission is the other half of the same failure.

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

**Built 2026-09-03 as four states, not two.** A clean critic produces no findings, so
"no critic finding" and "no critic ran" are different facts and a flag would collapse
them into the flattering one: `structural-only`, `semantic-clean`, `semantic-findings`,
and `unknown` when the record could not be read. A critic verdict in hand outranks an
unreadable record — the finding is evidence of the act and the ledger is bookkeeping
about it — and none of the four words is "verified".

The `critic:` category prefix is what makes the state readable from the findings alone,
which is what that prefix was added for. It lives in the domain rather than in the
package that stamps it, because two packages agreeing on a marker by inspection is one
decision in two modules and the failure would be silent: a gate that stopped recognising
critic findings would report every corpus as structurally checked only.

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
  `may`. Those markers are a closed list held in `standards/strength.toml` as data,
  and a claim carrying a universal quantifier over a single supporting quote is a
  reportable mismatch — `coverage`, warning tier. **Both roles are declared**, because
  the check has to tell a claim that *hedged* from one that said nothing either way:
  those are different states, and only the second is silent. A claim carrying a
  universal *and* a hedge is a reading no lexical check can settle, and saying nothing
  is the honest answer rather than reporting whichever list was consulted first.
- **It is a finding and never a rejection.** The remedy is usually to weaken the
  claim rather than to find more evidence, and that is an author's decision. A gate
  that rejected the claim would push the author toward removing the qualifier
  rather than adding the caveat.
- **Normative claims are held higher, and that appears in the message rather than in
  a second threshold.** A universal assertion the corpus leans on is where being wrong
  costs most — the stakes rule §10.6 and §14.4.1 already apply. **The first
  implementation raised the bar by one for a prescribing type, which silently required
  three quotations**: a figure this section never states and §6.2 forbids inventing.
  There is one comparison here and the check implements exactly it; the stakes go where
  a reader triaging a queue can act on them. Same evidence, same truth, different
  attention, because the cost of error differs — a person who knows a door is locked when a
  colleague's jacket is inside will say they do not know it when an armed intruder
  is being hunted, and both answers are correct.

#### 17.3.1.1 the Causal Register Is the Second Axis, and Nothing Stores It

Strength has a second dimension, and Pearl's ladder names it: *association* (seeing),
*intervention* (doing), *counterfactual* (imagining). Claims on different rungs get
treated as interchangeable, and a claim whose wording is causal while its evidence is
observational is the same silent upgrade §9.4 refuses for quotations — asserted at one
strength, supported at another.

**The rung is not stored anywhere, and that is the decision rather than a deferral.**
Three homes were considered and each fails for its own reason:

- **A declared `rung` on the claim** is self-certification. An author who overclaims
  causally will declare `intervention` too, which is the defect §4.6.2.1 refused when
  it took `approval_required` off the payload: the party being measured must not
  declare the measurement.
- **On the constraint** (`claim_subjects`) is where a backlog entry proposed it, and a
  constraint is a *quantity*. "Restarting the pod clears the leak" carries no operator,
  so the rung would reach only the claims that happen to state a number — close to the
  opposite of the population that needs it.
- **On the link's `rel`** contradicts §5.5.1.2: causality is carried as a claim, never
  as a relation between documents. A rung is the standing of an assertion, not a
  property of a pointer.

**So both registers are read at check time and only the gap is reported.** The claim's
register comes from its wording and the evidence's from the archived passage this
corpus already validates verbatim — *causes*, *leads to*, *results in* against *is
associated with*, *correlates with*, *predicts*. Two closed lexical lists in
`standards/`, the same shape as the quantifier markers above, and the finding is the
mismatch rather than either reading alone. A single stored rung could not express a gap,
which is what the interesting case always is.

Warning tier and never a rejection, for the reason the quantifier axis gives: the remedy
is usually to weaken the wording, and that is an author's decision.

**The word list ships with `coverage` or not at all.** A closed lexical class with no
reader is the mistake this specification has recorded three times, and `coverage` is
this one's only consumer.

**Built 2026-08-27 as `lint`'s `rung` check**, with `standards/registers.toml` beside it.
Three details are decided here rather than in the code:

- **The evidence's rung comes from the quotation, never from the archived file.** A whole
  source almost certainly contains a causal word somewhere, so reading the file would
  clear nearly every claim while appearing to check it. The quotation is also the text the
  corpus validates verbatim, so it is the only evidence a finding can honestly attribute.
- **A quotation carrying no register word is silent, not observational.** Most carry none.
  Reading silence as observational would report nearly every causal claim in a corpus on
  the strength of evidence never examined — asserting from absence, which is the move
  §9.4's `Unchecked` outcome refuses one layer down.
- **There is no counterfactual role.** The third rung has no lexical class this check
  could read: *"would have"* appears in ordinary conditional prose far more often than in
  a counterfactual claim, and a role with no reliable marker reports the corpus. It is
  named in this section and not detected.

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

**Extraction now writes it, and the check is what remains.** As of 2026-08-27 the
relay's reply format carries a per-claim `lead`, `gnosis_claims` records it, and
`claims.lead` is populated — so the data this check needed exists. It is **optional in
the reply**: a claim with no lead gets a NULL one, because §5.8.3's argument one field
over settles that reporting is a review signal where refusing is a gate, and turning one
into the other would make the corpus decline knowledge over a summary.

`claims_fts` is populated only for a claim that has a lead, which is §5.5.3 applied
rather than repeated — and it makes claim-level search cover the *extracted* part of the
corpus, so that search has to say what it did not cover. **As of 2026-08-27 it does**:
`gnosis search --claims` queries the table and reports the claims carrying no lead beside
its results (§11.1.1). Until then the table had been written on every extraction and read
by nothing, which is this specification's most-repeated defect appearing in the change
that recorded the rule.

Building the check itself is deliberately a separate change: the field existing and the
field being checked are two decisions, and shipping them together would conflate them.

**And the gap must not be closed by deriving the lead from the claim text.** A rule
that picked the conclusion clause — the one no reason marker introduces — would
produce a lead for every claim immediately, and would make this check vacuous: it
would be testing a derivation against the rule that produced it, and it would pass
everywhere. Worse, it contradicts what the column is for. A lead is the author's
conclusion *in its own words*, which is a judgement about what the claim is
ultimately asserting; a clause is merely the part of the sentence that survived a
filter. The two coincide often enough to look like a working implementation and
diverge exactly where the check would have earned its keep.

______________________________________________________________________

### 17.5 a Count in a Finding Sits in a Noun Phrase

**A message must not put a count where a verb has to agree with it.** Three findings
shipped saying "1 document declare", "1 claim name" and "1 command that do not resolve",
each written by composing a number with a sentence built for the plural case.

The remedy is mechanical and already in the code: `noun(n, word)` renders "1 quotation"
and "2 quotations", so the count lands inside a noun phrase and the sentence around it
never has to inflect. A message that reads correctly at every count is one nobody has to
proofread twice.

**It is a rule rather than a style note because of how the three were found.** Every one
was caught by running the binary, and none by a test — a substring assertion looking for
`1 document` finds it and stops, which is exactly the case where a check reads as passing
while the output is wrong. Three occurrences is where a comment stops being the remedy
(§12's argument for `filename-drift`, one layer over).

**Enforced where every check already renders its messages.** The `lint` test helper that
turns a diagnostic into the line `gnosis lint` prints now scans it for a count of one
followed by a plural verb. That is the derived option: no list of checks to maintain, and
a check written tomorrow is covered by the tests it comes with.

The detector is a closed list of verb forms, and **which way it fails is the reason a
list is acceptable here**: a verb nobody listed means one defect slips through, never a
wrong message blessed. Verbs that are also ordinary nouns in this vocabulary — *state*,
*use*, *report*, *reference* — are deliberately absent, because an earlier version read
"no document is of 1 declared type Reference" as a disagreement and would have had
somebody "fix" a correct message by breaking it.

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

### 18.0 What Each Suite Adds

Nineteen test packages and no account of what each one is for, so the honest failure is
a new test landing where an existing one already covered the case. A surveyed harness
annotates every test with its coverage delta; this is the same idea at package grain,
which is the grain somebody choosing *where* to add a test is working at.

Named by the question each answers, because "the pure core" and "the binary" are
different questions and a reader has one of them in mind.

| Suite                    | The question it answers                               | What nothing else covers                                                                 |
| ------------------------ | ----------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `internal/gnosis`        | What does the domain guarantee with no corpus at all? | Zero values, identifiers, freshness and drift states, the sampling draw, rationale reuse |
| `internal/okf`, `okflog` | Does a document round-trip?                           | Frontmatter parse and render byte-exactness; `log.md`'s date-heading form                |
| `internal/segment`       | Where may a sentence be cut?                          | The subject-recovery rule that makes over-splitting safe                                 |
| `internal/ontology`      | Is the vocabulary loadable and unambiguous?           | Alias exclusivity, rejections, dimensions                                                |
| `internal/standards`     | Is a threshold declared with its reason?              | Load-time refusals, the unread classification, retrieval-case grading                    |
| `internal/scan`          | Does a §9.3 rule discriminate?                        | The ruleset's self-test — every rule's own must-flag and must-not-flag                   |
| `internal/archive`       | What disposition do these bytes get?                  | Gate decisions, record identity, the oversize bounds                                     |
| `internal/schema`        | Whose text is this?                                   | The marker contract's four rules (§5.7.1)                                                |
| `internal/index`         | Do two rebuilds agree?                                | The content digest, FTS behaviour, the rebuild floor                                     |
| `internal/lint`          | What does a check report, and when does it decline?   | Derived applicability, skips, the declared categories and actions                        |
| `internal/gate`          | Would this candidate be admitted?                     | Signal composition, the control, §18.2's both-directions mutation                        |
| `internal/command`       | Is this write well-formed on its own terms?           | Validation before the lock                                                               |
| `internal/audit`         | Is this row writable and readable back?               | Canonical encoding, the fields a row may not omit                                        |
| `internal/bundle`        | Does the shell hold the corpus together?              | The writer lock, tier 0, the relay, the four trail reports, every join                   |
| `cmd/*` (per command)    | Does this one command's own logic hold?               | Flag validation and report rendering close to the command                                |
| `cmd` (dispatcher)       | Does the binary do what a person typed?               | Every end-to-end path: fetch → ingest → admit → promote, and every exit code             |
| repository root          | Does the specification still describe the code?       | §12.1's table against the registry, the standards classification against the source      |

Two things this table is for beyond orientation. A test that would fit two rows belongs
in the **higher** one — a rule provable from values does not need a bundle — and that is
the same pressure §rules-of-thumb call "treat test difficulty as a locating device". And
a row with nothing in its third column would be a suite to delete.

______________________________________________________________________

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

#### 18.4.1 a Test Corpus Is Only as Good as Its Resemblance to the Artifact

Five artifacts in this specification are **data with a test corpus** rather than logic:
`standards/operators.toml` (§10.2.2), `standards/indicators.toml` (§9.4.1),
`standards/strength.toml` (§17.3.1), `standards/retrieval-cases.toml` (§11.0.2), and the
scan rules (§9.3). Each exists because the alternative was a closed list compiled into Go,
and each is only as trustworthy as the cases beside it. This section is the rule those
cases have to satisfy, and it was learned the expensive way.

**The cases MUST be shaped like the input the artifact actually receives.** The operator
patterns shipped with thirteen rules and eight passing cases while the *commonest* real
input failed silently: a claim's anchor is a **sentence**, and the pinned number library
does not convert a spelled-out number with punctuation attached, so
`"Retries must be no more than three."` parsed to nothing at all. Every case in the corpus
had been written without terminal punctuation. Thirteen patterns, eight green tests, and
the shape every real claim has was untested.

**That is §11.0.2's warning arriving inside the file written to obey it.** An instrument
authored from imagination measures the thing that was imagined. The corpus was authored
carefully, with inversions first and negative cases — and its cases were still *phrases*
where the artifact receives *sentences*.

Two consequences, and the second is the one that generalises past data files:

- **Take the cases from the artifact, not from the description of it.** A claim anchor, a
  search query somebody actually typed, a fetched page's real bytes. Where a real one is
  not available yet, say so in the file — which is what `retrieval-cases.toml` shipping
  empty is for.
- **A green corpus is not coverage of the input space.** Adding patterns would never have
  found this; running the tool over one real document found it in a single command. The
  suite tells you the cases you wrote pass. Only the artifact tells you which cases you
  did not write.

The fix then exposed a third defect immediately — separating punctuation split `99.9%`
into a bound of 99, which is §9.4's own `split(".")` cuts `2.5 seconds` failure one layer
down, already written on the wall two sections away. **A failure mode recorded elsewhere
in this document is not thereby avoided here.**

**And it happened again on 2026-08-27, in the same file, one token over.** A unit written
against its number — `400ms`, `5MB` — defeated the number reader entirely, so
`"The timeout must be under 400ms."` parsed to nothing. That left `duration` and `bytes`,
two of the four dimensions §10.2.1 declares, with no working ordinary form: nobody writes
"400 ms" in a latency budget. Every case in the corpus was spaced, so thirteen patterns
and a green suite said nothing about it, and it was found by building a *different* check
that needed to read the unit. **The paragraph above was already on the wall when this
shipped.** Reading a rule is not obeying it; the corpus is what tests the artifact.

##### 18.4.1.1 the Other Four Artifacts, Read Against This Rule

Measured 2026-08-27, because a rule learned from one artifact is a guess about the rest.

- **`standards/indicators.toml` and `standards/strength.toml` already satisfy it**, and
  the reason is worth stating: their cases are whole sentences with terminal punctuation
  — *"The retry budget is three, and because the SLA is 400ms."*, *"Retries are always
  capped."* — which is what a claim's text and a claim's anchor actually are.
  **Their cases live in the consumer's tests rather than in the file, and that is
  correct here.** A word list's behaviour is only observable through the thing that
  matches it: whether `because` refuses a cut is a fact about `segment`, and
  `internal/standards` may not import its own consumers. The scan rules can self-test at
  load because a regex is executable on its own; a word is not.
- **The scan rules had the gap, and it was in the input's *size*, not its wording.** Every
  `must_flag` and `must_not_flag` is one sentence, and `Ruleset.Patterns` receives a whole
  fetched document. Two tests now close it, using this repository's own long-form
  documents — about a megabyte of real markdown with headings, tables, fenced code, inline
  regexes and URLs, which is the nearest thing to a fetched page that exists here:
  no rule may fire on any of it, and every rule's own example must still be found when
  **buried inside** one of those pages. The second test is what keeps the first from
  passing vacuously: zero matches over a megabyte means nothing unless the rules can fire
  on text of that shape at all, and "no findings" quietly meaning "no scan" is the
  confusion §9.3's coverage type exists to prevent.
- **`standards/retrieval-cases.toml` cannot be wrong yet** and is left alone. It ships
  empty by §11.0.2's argument, which this rule endorses rather than overrides: where a
  real case is not available, the file says so.

**The general shape, for the next artifact of this kind**: the cases belong wherever the
artifact's behaviour is observable, at the size and shape the artifact receives — and
where neither is available yet, the file records that instead of inventing one.

### 18.5 Adversarial Fixtures

A source containing zero-width characters, a bidi override, a prompt-injection
string, and a plausible-looking fabricated quote. Each MUST be caught, and the
test MUST assert *which* check caught it, so a check silently ceasing to fire is
visible.

#### 18.5.1 an OKF Conformance Table, Written Before §14.1 Is Built

`gnosis` claims OKF conformance in §5, §11, and §14, and nothing currently checks
that claim against the specification. One table test, over OKF §7's three actor
forms plus the two `gnosis.Actor` adds, asserting for each: whether `ParseActor`
accepts it, and what tier §14.1's fold yields.

| Actor string                     | `ParseActor` | Tier contribution |
| -------------------------------- | ------------ | ----------------- |
| `human:priya`                    | accepts      | human-reviewed    |
| `process:finance-nightly`        | **rejects**  | machine-confirmed |
| `reference_agent/gemini-2.5-pro` | **rejects**  | machine-confirmed |
| `agent:ingest`                   | accepts      | machine-confirmed |
| `check:duplicate`                | accepts      | machine-confirmed |
| `priya` (unprefixed)             | rejects      | machine-confirmed |

The two `rejects`-with-a-tier rows are the point: they are the cases where the
mint-side type and the read-side fold **must** disagree (§14.1.1), and a test that
does not contain them will pass under the merge that breaks conformance.

Write it now rather than with §14.1. The divergence it pins already exists in a
shipped type, it was introduced without touching trust metadata at all, and the
cost of finding it later is a corpus whose tiers were computed by a parser that
refused half its inputs. This is the same reason `skillet` moved its own promotion
trigger from *stores trust metadata* to *classifies an actor*: storage is not the
event, classification is.

### 18.6 the Relay Test, and Which Kind of Real It Needs

Everything above is deterministic and none of it touches the one seam where gnosis
meets a model. `cmd/relay_test.go` hand-writes every reply, so nothing establishes
that an agent handed a **real emitted prompt** produces a reply `admit` will accept.
The relay was designed so that gnosis never calls a model, and that decision makes
most of its testing easy and this one case hard: the gap exists *because* of the
determinism, not in spite of it.

Three methods are available and they differ in what they hold fixed:

| Method                           | Runtime                  | Reasoning                                                            | Assertion                                             | Fits CI                                   |
| -------------------------------- | ------------------------ | -------------------------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------- |
| Hand-written replies (today)     | none                     | authored                                                             | direct                                                | yes — and proves nothing about the prompt |
| Scripted model                   | real binary, real prompt | replaced by a local server speaking the model protocol from a script | on the request *and* the reply                        | yes                                       |
| Real model, mechanical predicate | real binary, real prompt | real                                                                 | a pure predicate over the machine-readable transcript | no                                        |

**The second is the one to build, and the third is the one to have.** They answer
different questions. A scripted model proves the *contract* — that the prompt gnosis
emits carries what a replier needs, and that a well-formed reply survives `admit` —
and it is reproducible, free, and CI-safe. It cannot show that any real model
produces such a reply. The third can, and cannot go in a gate: it is slow, billed,
network-dependent, and non-deterministic, and a suite with those properties is
disabled within a month.

Two disciplines carry over from the projects that built each.

- **From the scripted-model method: assert on what the agent *sent*.** A fixture
  that only dictates replies is a playback, not a test. The fixture MUST fail when
  the request arrives without the fields the prompt was supposed to carry, or in the
  wrong order — the contract is checked in both directions.
**gnosis has no transcript to grep, and the translation is the write trail.** The
surveyed harness wraps the agent and emits its own events; gnosis does not wrap it —
the seam is prompt file → agent → reply file, and the agent runs outside this process.
What gnosis holds instead is the audit trail, which already records every mutation with
a time and an actor. So both assertions are predicates over rows that exist: *was a
reply admitted*, and *was anything written first that is not on the allowlist*. A
refused admission does not count as one, because the question is whether a real model
produced a reply this corpus would take.

- **From the real-model method: grade the transcript, never the reply's prose.** A
  surveyed harness runs the real agent under an isolated `HOME`, emits
  machine-readable transcript events, and asserts with a grep over them. Its second
  assertion is the instructive one: it checks not only that the required step
  happened but that **nothing else happened first**, against an explicit allowlist of
  actions that do not count. Ordering is a property a prose reply cannot be trusted
  to report about itself.

#### 18.6.1 the Scripted Model, Once Built

"A local server speaking the model protocol" needed translating, and the
translation is the design: **gnosis speaks no model protocol.** It writes a prompt
file and reads a reply file, because the relay was built so that gnosis never calls
a model. So the seam is *prompt file → agent → reply file*, and the local server is
a function.

The translation keeps the property that matters. The scripted agent can see **only
the prompt** — it parses the fenced source out of the emitted file and quotes from
that and nothing else — so it cannot quote text the prompt failed to carry. If the
renderer stops fencing the archived text, the agent has nothing quotable, `admit`
never runs, and the fixture fails. That is "assert on what the agent *sent*" in the
shape this architecture has: here the prompt **is** the request, so asserting on the
request means deriving the reply from it and nothing else.

Two details are deliberate and both are about not testing gnosis against itself.
The agent restates the six-word minimum rather than importing `quotecheck`'s
constant, and it splits sentences with a naive `". "` — the very splitter
`internal/segment` exists to replace. An agent reads the instructions it was given;
a fixture that shared gnosis's constants and gnosis's segmenter would agree with
gnosis by construction.

The adversarial half is what makes it a test rather than a playback, and it is
two-directional: a prompt with its source section cut, or with its fence emptied,
must leave the agent nothing quotable; and a reply quoting text absent from the
prompt must be refused by `admit`. Writing those found the ordinary fixture mistake
— a damage case named "the fence emptied" that deleted one phrase and left the rest
of the passage quotable, so the agent legitimately answered. The mutation, not the
agent, was what failed.

The third method is still not built, and this does not bring it closer.

The third method's output is not a gate and MUST NOT become one. It is evidence,
recorded like any other, and §18's own standard applies to it: a run that could not
be performed reports `unchecked`, and a suite that skips it says so.

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

Five decisions are deliberately left open. Each would change the work materially,
none blocks Phase 1, and each is cheaper to make against a real corpus than
against an imagined one. A sixth has since been settled and is kept below with its
reasoning, because the option chosen is the one carrying the visible cost and a
future reader deserves to know it was chosen rather than defaulted into.

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
5. **Settled: `fetch` clones with `go-git/v6`.** Recorded here rather than
   removed, because the reasoning is the part worth keeping and the option chosen
   is the one with the visible cost.

   §7.2 had named `go-git/v6` and credited the choice to skillet. Both halves were
   wrong — `v6` publishes only alphas, and skillet has no go-git dependency at all,
   so there was no existing consistency to preserve. Three real options remained:
   stable `v5`, the `v6` alpha, and shelling out to `git`.

   The argument for shelling out is stronger than §4.3's pdftotext precedent makes
   it sound, and is recorded so nobody has to reconstruct it: git is **not**
   pdftotext. A gnosis bundle *is* a git repository — §4.5 makes git the transport
   between users and §4.6 the merge mechanism — so no user can have a corpus
   without it, and the adapter would add no dependency that is not already
   required. What it adds is process invocation, credential prompts inherited from
   an interactive terminal, and version skew across machines.

   `v6` was chosen anyway. The cost is named rather than argued away: this pins an
   alpha in the code path that produces evidence. Two things bound it — the clone
   is shallow, single-branch, and immediately discarded; and the property that
   matters downstream is enforced by the *record*, not by the library, since a
   record is the hash of its own content whatever produced the bytes.

   **The blast radius was measured rather than assumed, and it is larger than "one
   function and its tests".** Three production files import go-git, not one:
   `git.go` clones (`PlainCloneContext`, `CloneOptions`, `Repository.Head`),
   `gitfile.go` reads a file at a revision out of the bundle's own repository
   (`PlainOpenWithOptions`, `ResolveRevision`, `plumbing.Revision`,
   `object.Commit`), and `headtime.go` reads HEAD's commit time
   (`plumbing.NewHash`, `CommitObject`, two sentinel errors). About a dozen API
   surfaces across three files.

   That correction changes the shape of the risk rather than its size. Only
   `git.go` is in the evidence path; the other two read the *user's own*
   repository, so an alpha bump that broke them would surface in `standards
   --since` and the audit trail's health check rather than in tier 0. The record's
   content-addressing still bounds the evidence half. Checked at the time of
   writing: `v6.0.0-alpha.5` is still the newest published version, so the trigger
   for revisiting — a `v6.0.0` release — has not fired.

   One consequence is worth stating because it is easy to get wrong: **the recorded
   URI carries no commit.** It is `<remote>#<path>`. Including the commit would
   make a record's identity depend on the repository's activity rather than the
   file's, so a single unrelated push would re-record every file in the tree —
   which is §4.3.1's argument against a timestamp, arriving by a different route.
   Which commit a text came from stays recoverable: the repository still holds the
   blob.

   **The commit is reported at fetch time, and stored nowhere.** A reader had no way
   to learn which revision a git-sourced record came from and nothing said so, which
   is the complaint that produced this paragraph. So `fetch` prints the commit it
   cloned, beside every candidate from that clone, marked as not recorded — if it
   matters for a claim, `log.md` is where the person who decided it mattered writes
   it down. It travels on the *candidate* and never reaches the record, and a test
   asserts the record's canonical bytes are identical with and without it, because
   putting it on the record is the obvious next commit and §4.3.1 is what it would
   break.

______________________________________________________________________

*Sources: [`llm_wiki_pattern.md`](./llm_wiki_pattern.md);
[`manifesto.md`](./manifesto.md) and the repositories it surveys under
`~/Documents/agent-red` and `~/Documents/agent-blue`; the Open Knowledge Format
v0.2 specification at `~/Documents/agent-blue/knowledge-catalog/okf/SPEC.md`;
and the family's own backlogs in `skillet`, `exegesis`, `skillsaw`,
`agentic-dev-harness`, and `canonizer`.*
