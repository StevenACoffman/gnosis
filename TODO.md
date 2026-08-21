# Gnosis TODO

`github.com/StevenACoffman/gnosis` — the LLM Wiki tool. The design lives in
[`SPEC.md`](./SPEC.md) and the build order in [`PLAN.md`](./PLAN.md); this is the
backlog against them.

Scope boundary, same as the rest of the family: **CLI scaffolding and the
machine-output envelope belong to `climax`**, shared domain code belongs to
`skillet`, and gnosis owns only what is specific to a knowledge corpus.

**Reconciled against `SPEC.md` on 2026-08-20.** Items the spec has absorbed are
recorded in *Settled* as one line and a section reference — the reasoning lives in
the spec now, and keeping two copies of it is how they diverge. Anything above that
line is genuinely outstanding.

______________________________________________________________________

## Blocking a Phase

- [ ] **`quotecheck` — written and adopted; blocked on a skillet release.**
  `skillet/quotecheck` exists with the three-value `Status`
  (`Unchecked`/`Found`/`Missing`, unchecked as the zero value), and exegesis has
  adopted it. **What remains is not code**: skillet must be committed and tagged
  before exegesis can drop its `replace` directive and before gnosis can depend on
  the package at all. That is a release decision, not an implementation one.
  Only the *comparison* was promoted. `Segment` and the `redlines.Quotes`
  extraction stayed in exegesis, because where a quotation begins is precisely
  what the shared package must not know: exegesis says blockquote-in-R, gnosis
  says `gnosis_evidence` frontmatter, and a kernel carrying either would have the
  other routing around it.
- [x] **`ruleset/conflict` was never a blocker, and the entry was wrong.**
  *Corrected 2026-08-20 after reading the source.* The package **already exists**
  at `skillet/ruleset/conflict`, complete and tested — so there was nothing to
  promote. More to the point it does not do what gnosis needs: `conflict.Find`
  reads exactly four fields off a `ruleset.Rule` (`Statement`, `Severity`,
  `Level`, `Section`) and reports severity divergence, level divergence, and
  section collision. A gnosis claim has none of those; §10.2's decidable
  predicates are numeric, threshold, and enumeration comparisons over a subject
  key, an operator, and a value. The overlap is zero.
  What the two share is a **shape** — pure, deterministic, `finding.Diagnostic`
  out, no score, fold-normalised equality — and a shape is followed, not imported.
  Generalising `Find` over both domains would produce the special-general mixture
  the guidelines name as a red flag: a general mechanism with `if isRule … else if isClaim` inside it. **Phase 3 is not blocked.** gnosis writes its own predicates
  when Phase 3 arrives, and the promotion question reopens honestly if a third
  consumer ever wants the same comparison.
- [ ] **Claim segmentation implemented in Go. Blocks Phase 2.** SPEC §9.4 commits
  to deterministic segmentation with `claim-segmenter-kit`'s guarantee stated
  verbatim: **"Every emitted claim stands on its own, or the cut is not made."** The
  reference implementation is Swift, so nothing is importable — the algorithm and
  the guarantee transfer, not the code. Budget for the failure cases it names:
  `split(".")` cuts `2.5 seconds`, and an abbreviation list still cuts `e.g.`,
  `README.md`, `foo.bar()`, `https://example.com/a.html`, and `A. Turing`;
  newline-splitting cuts every hard-wrapped paragraph.
  Read its named siblings first — **SourceConflictKit** (conflict detection
  *between sources*), ClaimConsistencyKit, GroundingKit. Lives in `internal/`
  unless canonizer needs the same thing for rule bodies; one consumer is not
  evidence for promotion.
- [ ] **`evidence/fetch.jsonl` conflicts on every concurrent append. Decide before
  tier 0 accumulates.** A single append-only file in a git-merged tree (§4.6)
  conflicts on its final line whenever two users fetch anything. The archive beside
  it merges perfectly because it is content-addressed; only the ledger has the
  problem. **Preferred fix: one file per fetch** under the same content-addressed
  layout, with `fetch.jsonl` demoted to a derived rollup in `.gnosis/` — it then
  inherits the archive's merge behaviour and needs no per-clone git configuration.
  The alternative, a merge driver that concatenates and re-sorts, works but will not
  survive a fresh checkout. Urgent only in that tier 0 is committed and append-only,
  so the layout is expensive to change once real evidence exists. `log.md` has the
  same shape but is expected to conflict and is human-resolvable, so it stays as
  OKF §9 specifies.
- [ ] **The local API surface is unspecified. Needed when more than one instance
  writes.** §4.6 states the coordinator's responsibility and `gnosis serve` carries
  the role, but nothing says what the protocol is, how a client discovers it, what
  happens when it is absent for a write, or whether a write without it fails or
  starts one. Bounded by readers being explicitly independent of it. Phase 2 at the
  earliest — the only writers today are `init` and `index rebuild`.

______________________________________________________________________

## Phase 1 Remainder

**Phase 1 is complete.** `init`, `doctor`, `index rebuild`, `lint`, `search`,
`show`, `graph`, all integration-tested at the dispatcher. What remains below is
Phase 2 and later.

- [x] **Commands `show`, `search`, `graph`** — done, with resolved links inline.
- [x] **`documents_fts` with one analyzer constant** — done; self-contained rather
  than external-content, and the shared clause is asserted against
  `sqlite_master` rather than against the Go source.
- [x] **`schema-shape` check** — done; the expectation is derived by migrating a
  scratch in-memory database, so no hand-maintained list can drift from it.
- [x] **The `concept` → `claim` prose sweep is done.** Each remaining use was
  decided by whether a *file* or an *assertion* was meant; uses that correctly mean
  the OKF document were left, which §5.0 permits. It also turned up a real error:
  the §10 conflict-payload example used `/concepts/retry-budget.md`, a path prefix
  that has never existed.
- [ ] ~~Finish the sweep.~~ *Superseded by the entry above.* The schema, §5.0,
  §5.5, and everything touched by the rename pass are done. Roughly twenty prose
  uses remain and they split three ways: some correctly mean the OKF document and
  stay (§5.0 permits both words for it), some mean the claim and must change (§10's
  candidate-selection list, §14's trust and durability prose, §16's proof wording),
  and a few are genuinely ambiguous in the original — which is the defect that
  motivated the rename. One pass with §5.0 open, deciding each by whether a *file*
  or an *assertion* is meant. Not search-and-replace.
- [ ] **One seeded sampler, three callers.** §10.5's `critic --sample N`, §14.3.1's
  `stale --unreviewed`, and §6.2.1's random conflict pass all need reproducible
  draws. They must share one sampler reading its seed from `standards/`, or the
  three drift and none is reproducible under §18.3. Currently each is specified
  independently. Cheap now, awkward after three call sites exist.

______________________________________________________________________

## Noticed While Building Phase 1

Not blocking, not urgent; each is a real rough edge found by running the thing.

- [ ] **Search snippets carry raw markdown.** A hit on a document containing a
  link shows the whole `[Timeout](/c/01932b7c-…-timeout-policy.md)` inline, which
  is most of the snippet's width spent on a UUID. The body is indexed as written,
  which is right for matching — someone searching for a slug should find it — but
  the *rendering* should reduce link syntax to its text. Note the tension: strip it
  at index time and the slug becomes unsearchable; strip it at render time and the
  offsets FTS5 returns no longer line up with what is shown. Render-time, with the
  snippet re-derived rather than offset-mapped, is probably right.
- [ ] **`index rebuild` cannot repair a schema-shape failure.** `schema-shape`
  reports a missing table and advises deleting the file, which is correct —
  `rebuild` opens the existing database, and migration skips every statement
  because `user_version` is already current, so it would fail on the missing table
  rather than recreate it. Auto-deleting on a shape mismatch is the obvious fix and
  deliberately not taken: the `Unexpected` half of the same check exists because
  people do put things in that database, and a repair that silently drops them is
  worse than a manual step. If it is ever added it should be `--force` and say what
  it is about to destroy.
- [ ] **`show --body` reads the file; `search` reads the index copy.** Both are
  defensible — the file is the truth, the index is what was searched — but a
  document edited since the last rebuild will show fresh text with a stale snippet,
  and nothing says so. Cheapest fix is for `show` to compare the file's hash
  against `documents.content_hash` and note the divergence rather than hide it.
- [ ] **Nothing tests that two rebuilds produce byte-identical databases.** SPEC
  §18.3 requires it and §4.6 leans on it hard — it is the property that makes
  per-user indexes safe. `TestGraphIsDeterministic` covers one surface's output,
  which is weaker. SQLite files are not naturally byte-stable, so this likely
  compares a canonical dump rather than the file.

______________________________________________________________________

## Recorded for Later Phases

- [ ] **Silent definition drift has no detector — the largest hole in the
  vocabulary layer.** §5.8.2.1's alias-collision rule fires only when someone
  *declares* a colliding alias. It cannot fire when two groups have been using one
  word differently and neither has declared it, which is the ordinary way the
  problem arises; in that state `ontology.toml` is perfectly valid and the corpus is
  quietly ambiguous. Hamming states the general case: "definitions have a habit of
  changing over time without any formal statement of this fact," and a subject key
  whose meaning shifts invalidates every comparison ever made under the old meaning.
  The signal it would need is a subject whose claim population changes character —
  bimodal values, a dimension inconsistent with its declaration, a cluster of new
  aliases. Needs claims to observe, so Phase 3. The remedy once detected already
  exists: soft-deprecation, announce then enforce (§5.8.1).
- [ ] **Indicator words as an operator pattern.** *since, because, for, for the
  reason that, as indicated by* introduce a reason; *therefore, thus, so, hence, it
  follows that* introduce a conclusion. Lexical, closed, language-specific — held as
  data with a test corpus, never a regex in Go. Gives the `lead` check (§17.4) and
  segmentation something concrete without a model. Needs the test corpus before the
  code; the failure mode is a "because" inside a quotation.
- [ ] **Report what the corpus gained, not only what it got wrong.** Hamming's
  rating dynamic: "if everyone starts out at 95% there is little a person can do to
  raise their rating but much which will lower it; hence the obvious strategy is to
  play things safe." A corpus whose only visible signal is *problems found* rewards
  contributing less and claiming less. `lint --since` already reports what a change
  made worse; it should equally report claims admitted, evidence added, and
  conflicts closed. A second column, not a score.
- [ ] **Two claims may anchor to the same passage after a merge.** Claim ids are
  UUIDv7 so they never collide, but two users adding claims to one document can
  independently anchor different ids to the same text, and the merge is clean.
  Detectable as an `anchor_hash` collision within a document; a cheap addition to
  `claim-anchor` (§12). Phase 2, since nothing writes claims yet.
- [ ] **Bundle closure is half-specified.** §12's `archive-orphan` reports an
  `evidence/` file no claim cites. VAC's `unlisted-file` also fails a bundle
  containing a file the manifest does not list, and `qvr sync` does the same from
  the other end — anything in an agent directory not in the lock is hidden from the
  agent. The missing half here: an archived file that **no `fetch.jsonl` row
  records** is unaccounted for regardless of whether anything cites it.
- [ ] **`skillet/finding.Category` is an untyped string.** `Severity` and `Action`
  are typed while `Category` is a bare `string` with `omitempty`, so nothing stops
  two gnosis checks spelling one failure differently. VAC enumerates nineteen named
  reasons and never free prose; §8.0's `reason` vocabulary does this for the
  envelope but not for findings. Cross-repo — recorded against `skillet` as the
  shared question.
- [ ] **Read both transcript adapters before writing ours.** `engineering-notebook`
  ingests Claude Code *and* Codex session transcripts into daily summaries and a
  browsable journal — the §9.6 Stop-hook path already built, and the second
  reference implementation after `SkillOpt/skillopt_sleep`. Two independent readers
  of the same on-disk formats are worth more than one. Phase 2.
- [ ] **Surface definitions where terms are used.** `glossary-18F` is a small
  accessible panel resolving `data-term` attributes inline, as shipped on FEC.gov. A
  glossary nobody opens is not an ontology. Phase 5, with the viewer.
- [ ] **Initial `standards/` values are unwritten.** Referenced from §5.2's
  three-format rule and §6.2's threshold discipline, including the `rationale` field
  each value now carries. Phase 3, but the file layout is load-bearing earlier.

______________________________________________________________________

## Housekeeping

- [ ] **The two manifestos have diverged.** `~/Documents/agent-red/manifesto.md`
  and [`manifesto.md`](./manifesto.md) were the same document and are no longer.
  This one is authoritative — it is what the spec was written against, and it now
  carries the survey sections and the settled-decisions record. Make the other a
  pointer rather than a stale copy.
- [ ] **`llm_wiki_pattern.md` fails both rumdl and mdformat.** `SPEC.md`,
  `PLAN.md`, `TODO.md`, and `manifesto.md` all pass both.

______________________________________________________________________

## Settled

Each of these was an open item; each is now specified. The reasoning is in the
spec, not here.

### Data Model and Identity

- Claim identity and address are recorded in the document, not the index — §5.5.1
- `pos` is a cached location, never an address — §5.5.1
- Re-extraction reconciles by anchor rather than replacing — §5.5.1
- Content-addressed identity is a stated non-goal; hashing matches, never
  identifies — §5.1.3
- `concept` → `claim` in the schema, with document / claim / subject defined once —
  §5.0
- `entity_aliases` hangs off an entity, `tags` off a document — §5.5
- A document reaches a subject only through its claims; no `document_subjects` —
  §5.5

### Testimony, Provenance, and Trust

- Local reductionism named as the posture; a source's reliability is never
  inherited by its claims — §1.1
- Three kinds of knowledge, not two; `sources` belongs to both provenance classes —
  §10.4
- The evidence invariant is sound at claim grain only because claims are segmented
  first — §9.4
- Credibility signals combine as disqualifiers, never as a sum — §14.2
- A mixed-provenance claim is reported at its weakest link — §17
- The Gettier gap: quote validation is a justification check and cannot establish
  support — §17.1
- Evidence sufficiency scales with the strength of the claim — §17.3.1

### Curation and Adjudication

- A reader may challenge an accepted claim; four classes ordered by what settles
  them — §10.7
- Challenges are committed frontmatter, not cached rows — decisions are committed,
  observations are cached — §10.7.4
- Reversals recorded via `reverses`, surfaced only by `audit --reversed` — §10.6.5
- Adjudication authority scales with the adjudicators, and that is Ashby's law —
  §10.6
- The critic is blinded to existing adjudication, status, and tier — §10.3
- The language/reasoning line: lexical hindrances are checkable, reasoning
  fallacies are not — §10.3
- A constraint too wide to fail is not a constraint — §10.2.1.1
- Trails recorded as a deferred decision with the cheap answer named — §20

### Vocabulary

- One surface phrase resolves to one key, enforced, with the remedy named in the
  diagnostic — §5.8.2.1
- The clarification chain is the subject-admission test; no dimension means it is a
  tag — §5.8.1
- Soft-deprecation is what makes a forced rename survivable — §5.8.1
- Surface term and resolved key are both retained — §5.8.2

### Measurement and Reporting

- A finding count is not corpus health; §12 exhibits measurement inversion and says
  so — §12
- A trend is not a score, and `findings.state` gains `deferred` — §17.0
- A number without an interval is evidence no measurement occurred — §17
- No composite score, for an arithmetic reason: an average over a heterogeneous
  population describes no member of it — §17
- Observer bias is a design constraint, and the deepest reason findings ≠ failure —
  §17
- Accuracy is not relevance — §17
- `lint --check-value`, with retirement only for *nobody acts on it*, never for
  dissent — §12
- Severity distribution reported; a lopsided vocabulary carries no information —
  §12
- Structural verification is not semantic agreement, and `gate` says which ran —
  §17.1
- `gnosis_limitations` required and non-empty on normative claims — §17.2
- `lead` is a checked property, not a convention — §17.4
- `requisite uncertainty` names what `findings.certainty` encodes — §16.1
- The skip report is mandatory, and why — §12
- Tracers named as such: `fetch.jsonl`, `audit.jsonl`, `miss.jsonl` — §6.4
- §18 opens with Hamming's question about the test equipment — §18

### Architecture and Process

- One writer per user behind a local API; git between users; index never a merge
  target — §4.6
- The writer owns the bundle, not merely the database — §4.6
- `duplicate` is the merge reconciliation step, not a hygiene check — §4.6.1
- Thresholds moved in the finding-reducing direction must record that they were —
  §6.2
- The candidate selector is biased, named as such, with a seeded random pass and a
  plausibility guard — §6.2.1
- Periodic review exists: `stale --unreviewed` reports, never invalidates — §14.3.1
- Phase 1 is document-scoped and the `claims` table stays empty — §19
- Machine-output envelope: status, code, reason, message, data — §8.0
- Ingest scoped to four adapters; boilerplate stripping required of the pinned
  extractor — §9.2
- `show` and `search` render resolved links inline — §8.3
- Explanatory and exploratory are different jobs — §5.6
- LATCH bounds the presented-hierarchy choice to five — §5.6

______________________________________________________________________

## Deliberately Not Adopted

Named refusals, so their absence reads as a decision rather than an oversight —
the framing is VAC §7's, and adopting the practice is worth more than adopting the
format.

- **vac-protocol wholesale.** Its claims are *about AI systems* ("this agent
  handles X under conditions Y"); ours are *domain* claims ("the retry budget is
  3"). The mechanics, invalidity rules, and structural/semantic split transfer; the
  schema and its semantics do not.
- **Perspective-keyed vocabularies** (`Glossary`, `termageddon`). Coherent for a
  glossary, which records what people mean; incoherent for a comparison substrate,
  which decides whether two claims disagree. §5.8.2.1.
- **Predictive scoring over corpus history** (*From Reporting to Learning*'s
  architecture). §17 forbids it. The decision-layer checklist and the
  concept-drift discipline transfer; the model does not.
- **Model-based bias and fallacy detectors** (`unified-thinking`). A classifier
  wearing a rule's name badge. §10.3.
- `agent-graph`, `crashkit`, `rag-eval-lab`, `aidetector` — model-facing, and
  gnosis never calls a model.
- `effect-domain` — "transports are projections of the model" is the right
  principle on the wrong stack (TypeScript/Effect).
- `universal-translator` — i18n; only if the web interface is localized.
- `devterms`, `jargons.dev`, `glossary-kit`, `web-jargon`, `yourjargon`, `BugHive`,
  `metaphorically` — products rather than components.
- `sqlite_tutorial_book.md` — introductory SQL the derived-index design already
  goes past. One keeper: SQLite cannot `DROP COLUMN`, which is why migrations are
  append-only.
- `mathematics_for_machine_learning_book.md` — no application to a tool that runs
  no model and computes no gradient. Relevant only if the optional semantic
  reranker (§11) is ever enabled.
- `critical_thinking_in_world_book.md` — popular treatment of individual cognitive
  bias, overlapping Haskins's tables without adding a mechanism.
- Note, not an adoption: `jargon-v1` is an AI-managed zettelkasten parsing sources
  into index-card-sized ideas. Its granularity choice was once cited here as
  independent support for atomic claims; Luhmann's own practice supersedes that —
  his slips run long and continue across `57/12`, `57/13`, and it is the
  document/claim split his method actually supports.
