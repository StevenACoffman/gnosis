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

- [x] **`quotecheck` is available and no longer blocks anything.** *Released in
  `skillet v0.18.0` on 2026-08-21; gnosis is on that version.* The package carries
  the three-value `Status` (`Unchecked`/`Found`/`Missing`, unchecked as the zero
  value). Wiring it into a `lint` check waits for Phase 2, when there is evidence
  to run it against — an unused dependency is worse than a deferred one.
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
- [x] **Claim segmentation implemented in Go.** *Built in Step 2.1; `internal/segment`.
  `Claim` carries both an anchor and a substituted text, because §5.5.1 wants the
  passage locatable and §9.4 wants it standing alone, and those differ whenever a
  subject is recovered.* Original entry: SPEC §9.4 commits
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
- [x] **`evidence/fetch.jsonl` conflicts on every concurrent append.** *Decided
  and specified as §4.3.1: one immutable content-addressed record per source
  version at `evidence/fetch/<h[:2]>/<h>.json`, with `.gnosis/fetch.jsonl` demoted
  to a derived rollup.*
  Two corrections came out of deciding it. First, my claim that a git merge driver
  "will not survive a fresh checkout" was **wrong**: `merge=union` is git's
  built-in driver and a committed `.gitattributes` line is sufficient, verified
  directly. The argument for one-file-per-record is therefore narrower than stated —
  union merge resolves *silently*, keeping both versions of an edited line, and the
  `referenced` disposition archives nothing, so for exactly the fetches whose
  integrity cannot be re-derived the ledger is the only record.
  Second, **the record carries no timestamp**, reversing my initial recommendation.
  Including it makes tier 0 grow with *checking* rather than with knowledge — a
  weekly sweep over 500 sources is ~26,000 permanent records a year — and the check
  history it buys has no reader, since §14.3 needs only the latest check. Last-checked
  moved to `.gnosis/checked.jsonl` under §10.7.4's rule: decisions are committed,
  observations are cached. Recorded as §20 decision 5.
  Original finding: A single append-only file in a git-merged tree (§4.6)
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
- [ ] **The write-coordinator transport is undecided, and is now the smaller half
  of that question.** §4.6.2 settles the shape: writes are command *values*, one
  type per operation, carrying their own gating fields, and a transport serialises
  one. That removed the part that would have been expensive to get wrong —
  review-gating is a property of the type, so every transport inherits it, and
  §9.4's diff guarantee follows from preview and apply being one command rather
  than two code paths.
  What remains open: **which transport**, and it is deliberately not urgent. A Unix
  socket carrying the §8.0 envelope is the likely answer — the path is its own
  discovery, filesystem permissions are the right authorization for a local daemon,
  and the seam is an `io.ReadWriter`. HTTP is not a competitor but a second
  transport over the same interface, needed anyway for §13's viewer; MCP likewise,
  when an agent runtime is the primary caller. What genuinely cannot be predicted
  from here is whether preview and apply are one call or two, and Phase 2's real
  writers will say.
  **The interim step has a visible ceiling.** An advisory `flock` on
  `.gnosis/writer.lock` satisfies everything `init` and `index rebuild` need and
  commits to no protocol — but a lock cannot carry a command, so it can never
  provide §9.4's guarantee. The command type should therefore exist before the
  second writer does, even if the transport does not.

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
- [x] ~~Finish the sweep.~~ *Superseded by the entry above.* The schema, §5.0,
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

## Noticed While Building Phase 2

- [ ] **`AuditTrail` still has no production caller.** `Trail` and `Whole()` exist so
  a reader can tell a partial trail from a whole one, and the only readers are tests
  and `doctor`'s row count. `gnosis log --audit` or the `gnosis debt` verb already
  filed above is where it lands. Recorded because a careful API with no consumer is
  the same trap §6.5.1 is about, one layer up.

- [ ] **§9.3 stages 2 and 3 are still unbuilt, and now cost a signature.** Injection
  and exfiltration patterns need a pattern corpus with its own test set; secrets need
  a `betterleaks` dependency. Until they land, every promotion in every corpus routes
  through §9.5.1's human path, which is correct and is also friction on the ordinary
  case. These are the highest-value remaining Phase 2 items precisely because their
  absence is now measurable — the audit trail counts it.
- [ ] **Nothing reports the accumulated debt.** `audit.Row.Signals` records which
  checks each promotion was carried over, and no command reads it. The obvious verb
  is `gnosis debt` or a `log --carried` flag: *these 34 documents were admitted with
  no conflict check*. Without a reader the field is a promise rather than a
  mechanism, which is the same trap §6.5.1 is about.
- [ ] **A `refused` candidate has no route to a fix.** `promote` reports which
  signal failed and the author must re-run the whole relay to correct it. There is no
  `gnosis quarantine --edit` and arguably should not be — editing quarantined content
  by hand is how unvetted text acquires a human's authority without review. Worth a
  decision either way rather than an absence.

- [ ] **Freshness is per document, not per claim.** `show` reports the oldest check
  across every source the document cites, so one unverified source marks the whole
  page. That is the right *conservative* answer and the wrong *useful* one: a reader
  wants to know which sentence rests on the stale source. `gnosis_claims` anchors
  already tie a claim to its evidence, so the join exists; what is missing is
  carrying it through `lint.Document`.
- [ ] **The `stale` check cannot compare archived text to upstream.** Half of §12's
  own row for it. `lint` does no network by design (§4.6), so the drift half needs a
  different home — plausibly `fetch --recheck`, which already has the connection and
  already writes `checked.jsonl`. Recorded in §12 rather than dropped from the table.
- [ ] **`standards.Unread`'s classification is a maintained list.** Recording what
  reads each value cannot be discovered at runtime, so it is a switch in Go with a
  test asserting the set. The failure direction is safe — a newly consumed value not
  recorded is reported as unread, which is a false alarm somebody chases — but it is
  still a second place to remember, and the honest note is that the test is what
  makes it survivable.

- [x] **`staleness_days` and `in_degree_cut` are read by nothing.** `staleness_days`
  now drives the `stale` check's window. `in_degree_cut` stays unread deliberately:
  §14.4.1 wants it for *unprovable AND load-bearing*, `unprovable` is Phase 3, and a
  reader that classified bare centrality would be a different feature wearing the
  same number. Instead the deadness is now *reported* — `standards.Unread` records
  what reads each value, a test asserts the set, and `doctor` reports a value tuned
  off the seed that nothing reads. §6.5.1. A third state fell out of implementing
  it: `html_extractor`/`html_extractor_version` are *pinned*, not consumed, and
  calling them either of the other two misleads in opposite directions.
- [ ] **§6.2 assumes every threshold affects the finding count and two of seven
  do.** `corpus_budget` and `corpus_warn_fraction` feed `doctor`'s budget
  diagnostic and their delta is exact. The allowlist and caps govern admission;
  `hedging_max` and `rebuild_floor_fraction` govern the gate and the rebuild;
  the other two are read by nothing. `standards check` says which case each
  loosening is in rather than printing a zero, but §6.2's own wording should be
  revised to ask for the count *where one exists*.
- [x] **Freshness is computed and nothing calls it.** `lint`'s `stale` check and
  `show`'s freshness line both land now, joined by `bundle.LoadFreshness` — which is
  the shell's work because `checked.jsonl` keys `(uri, hash)`, claims key archive
  paths, and the fetch record is the only artifact holding both. Two things surfaced
  in the doing: `stale_after` governs the *claim* and `staleness_days` governs the
  *check* (§14.3.0), and never-checked is deliberately **not** a finding, because it
  is true of every document in a corpus that has just started fetching.
- [ ] **`init` does not scaffold `standards/`.** Deliberate for now — an absent
  file falls back to the embedded seed, so a seed improvement reaches every
  existing bundle, which scaffolding would prevent. Worth revisiting if the values
  become something people are expected to tune per corpus.
- [ ] **Nothing renders a scan's full finding set.** `archive` reduces it to one
  `RejectReason`, which is right for a disposition and loses the detail: a source
  carrying three classes reports one. A `doctor` or `fetch --explain` view wants
  `scan.Hidden`'s findings with their offsets.
- [ ] **`archive.Gates.ScanText` fails open on nil.** Documented and tested as
  deliberate, because the alternative makes every caller carry a stub. It means the
  wiring is a property one test asserts rather than one the type guarantees, and if
  a second shell ever builds Gates that test is what stands between it and no scan.
- [x] **`rebuild_floor_fraction` moved to `standards/promote.toml`.**
- [ ] **No `--resume` and no crash-resumable queue.** §9.2 wants the ingest queue
  SQLite-backed so a killed process resumes rather than restarting. Prompts are
  currently emitted in one pass and a crash halfway through leaves some written and
  some not — recoverable by re-running, since emission is idempotent, but not what
  the spec describes. Worth building when ingestion is bulk; it is not yet.
- [ ] **No `--relay` chaining.** §8.2 offers it to cut round-trips, the way
  `adh run --relay` does. Two commands came first deliberately.
- [ ] **Prompts are never cleaned up.** `.gnosis/prompts/` accumulates one file per
  unanswered question and nothing removes an answered one. Gitignored, so it costs
  only disk, but a reader listing the directory cannot tell what is outstanding.
- [x] **`admit` verifies the key names an emitted prompt.** *Fixed in Step 2.8 by
  the `PromptMeta` sidecar: a key with no meta is refused before the reply is
  cached.*
- [x] **An audit gap is now visible in three places.** *`audit_failed` in Data, the
  envelope message, and a Warn writer the commands point at stderr. Still not a
  status: the write happened.* Original:
  silent.** `Coordinator.record` is best-effort by design — reporting the failure
  as the operation's would tell a caller to retry something that succeeded — and
  the note lands in the outcome's message where a machine will not see it. A trail
  with silent gaps cannot answer the question it exists for. Wants either a
  `doctor` check that the trail's last row matches the corpus's last change, or an
  explicit field on the envelope.
- [x] **`gnosis standards check --log` files the entry.** *Reads the previous values
  from git. The finding count is reported only for the two thresholds that produce
  findings — see the new item below.* Original: §6.2 requires
  a loosened `standards/` value to be recorded there with the finding count before
  and after. `standards.CompareArchive` can detect the loosening and `okflog.Add`
  can file the note; nothing joins them, so the requirement is currently a
  convention a person has to remember — which is the thing §6.2 says not to rely on.
- [x] **`init` and `index rebuild` emit audit rows.** *Actor is `check:init` /
  `check:index-rebuild`, because the tool caused the write.* Original: §15 says every mutation.
  They write scaffold and derived state rather than corpus content, which is an
  argument for a different row rather than for none.
- [x] **`standards/promote.toml` exists.** *`hedging_max` moved out of Go with a
  rationale that admits it was guessed; `rebuild_floor_fraction` moved out of
  `archive.toml`.* Original: §9.5 requires every gate signal
  to be declared in `standards/`, and `gate.Limits` is currently built from a
  literal `HedgingMax: 3` in `bundle.gateInputs`. That is exactly the hardcoded
  threshold §6.5 forbids, and it has no rationale attached. `MinPassageWords` is
  fine — it comes from `quotecheck`, so the gate and the guard cannot disagree.
- [x] **The gate has no human-approval path.** Built, and generalised past what
  this entry asked for: the path opens for any signal that *could not run*, not only
  for a scan finding or a conflict. Three requirements, each closing a route back to
  a bypass — a `human:` approver, the document's path typed as the phrase, and a
  rationale. §9.5.1. The audit row records which signals were carried, which is the
  field the whole argument rests on.
- [x] **No `gnosis promote` or `gnosis quarantine` verb.** Both exist. `quarantine`
  reports each waiting document's gate decision rather than only its path, because a
  list of paths says what is stuck and not why. `promote` previews by default. The
  ingest cycle now completes end to end for the first time.
- [x] **`gnosis_claims` archive paths are checked by lint.** *The `archive-path`
  check, which covers the whole corpus rather than only newly admitted documents —
  a better placement than the Quarantine hook this entry proposed.* Original: The gate
  reads `archive_paths` from frontmatter and checks they exist in tier 0, which
  catches the case at promotion. Nothing checks them when a document is
  *quarantined*, so an author learns about a typo one step later than they could.
- [x] **`StoreEvidence` reports a path holding the wrong bytes.** *ECONFLICT naming
  the path, and the existing bytes survive to be looked at.* Original:
  have.** A record path is the hash of its content, so differing bytes at an
  existing path is corruption or tampering rather than an update. The writer
  correctly declines to replace it — and currently says nothing, reporting the
  same "unchanged" a genuine no-op reports. `doctor` should re-hash tier 0 and
  report a mismatch. Until it does, the tamper-evidence is a property of the
  layout that nothing actually checks.
- [x] **The corpus budget is reported by `doctor`.** *Warns at the threshold, errors
  past the budget, names the five largest files, and refuses nothing.* Original:
  `corpus_budget` and `corpus_warn_fraction` are validated at load and no code
  consults them, so the repository can still grow without anyone being told —
  which is the thing they exist to prevent. Wants a `doctor` check that sums
  `evidence/` against the budget.
- [ ] **`hasOversizePayload` over-reports on prose about data URIs.** Documented
  as deliberate — the failure direction is a lost archive rather than a committed
  raster — but a document *about* data URIs is a plausible corpus member, and the
  refusal gives no way to say "this one is prose". Revisit if it ever fires on a
  real source.
- [ ] **Nothing checks that `evidence/text` has no orphans.** `StoreEvidence`
  writes content before the record so a crash leaves inert orphaned text rather
  than a record pointing at nothing. Inert is not free: the orphans count against
  the corpus budget and never get collected.
- [x] **`.gnosis/checked.jsonl` exists.** *Per-user, upsert rather than append
  because §4.3.1 says nothing consumes the sequence; a check is keyed on a source
  *version*, not a URI. `gnosis.FreshnessOf` is the four-state vocabulary.*
  Original: §9.2 says a re-fetch of an
  unchanged source advances it, recording that this user looked, and §4.3.1 makes
  it the documented exception to §4.5 — the one per-user observation that is
  cached rather than committed. Without it a no-op fetch records nothing at all,
  so `fresh`/`stale`/`unknown` (§14.3) cannot distinguish "checked and unchanged"
  from "never checked". This is the piece that makes omitting the record timestamp
  affordable, so it is the natural companion to Step 2.4 rather than a later
  nicety.
- [x] **`sources_fetched` is derived on rebuild.** *Keyed on the record hash so both
  versions of a twice-fetched source survive; replaced rather than merged. The
  tier-0 walks stay, because readers must work with no index.* Original: §9.2 has
  `sources_fetched.extractor` and `.extractor_version` answering which stripper
  produced a tier-0 file; §4.3.1 has `.gnosis/fetch.jsonl` as a greppable rollup
  rebuilt by `index rebuild`. Neither exists, so tier 0 is currently writable and
  unqueryable — every question about it means walking `evidence/fetch/`.
- [x] **The fetch adapters run §9.3's hidden-character scan.** *`internal/scan`,
  wired through `archive.Gates.ScanText`. Stage one of four; `scan.Coverage` reports
  which ran, so a clean scan is not read as "§9.3 passed".* Original: §4.4 requires archived
  text be subject to §9.3's scan, and §9.3 is unbuilt. Currently an SVG — or any
  source — can carry invisible text into the archive.
- [ ] **`go-git/v6` is an alpha in the evidence path.** Decided in §20.6 with the
  cost named. Revisit when `v6.0.0` ships, or sooner if an alpha bump breaks the
  clone: the exposure is one function and its tests, because a record's identity
  comes from its own content and not from what produced the bytes.
- [ ] **A git fetch records no commit, by design, and nothing says so to the
  user.** The URI is `<remote>#<path>` for the reason in §20.6. A reader looking
  at a record has no way to learn which revision it came from short of searching
  the repository for the blob, and `show` does not offer to.
- [ ] **The git-adapter test fixture is fragile under concurrent load.** `originRepo`
  shells out to `git init/add/commit`, and two `go test ./...` runs at once produced
  `fatal: failed to write commit object` and a 120-second timeout. Passes reliably
  when run alone, including under `-race`. A test that fails under load is a test
  that will fail in CI on a busy runner, and the honest reading is that the fixture
  is doing real filesystem and subprocess work in a `t.Parallel()` test.
- [ ] **The git adapter is not exercised against a remote.** Its tests clone a
  local repository built by `git` itself, which covers the walk, the URI rewrite,
  and the cleanup — and not authentication, shallow-clone negotiation, or a
  server that hangs up.

## Noticed While Building Phase 1

Not blocking, not urgent; each is a real rough edge found by running the thing.

- [x] **Search snippets carry raw markdown.** *Fixed: `index.Snippet` renders the
  excerpt from the body at query time — code blanked, headings and inline links
  reduced to their text — replacing FTS5's `snippet()`. Specified as §11.0.1.*
  Original finding: A hit shows the whole
  `[Timeout](/c/01932b7c-…-timeout-policy.md)` inline, most of the width spent on a
  UUID. The body is indexed as written, which is right for matching — someone
  searching for a slug should find it — so the fix is at render time.
  `piekbs` did exactly this and had a second reason we lacked: FTS5's `snippet()`
  was measurably slow, so they dropped it entirely. Their shape: parse the markdown,
  take the plain content, collapse whitespace, find the first keyword, window ~120
  characters around it. That resolves the tension the original note recorded — the
  snippet is **re-derived**, so FTS5's offsets never need to line up with what is
  shown.
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

## Deep Reads — `oh-my-agent`, `ruflo`, `hindsight`, `superpowers` (2026-08-22)

Four repositories the `agent-green` survey filed as *read shallowly*, opened. Written
up in `manifesto.md`; six findings went into `SPEC.md` (§6.4.1, §9.5.1, §10.6.4,
§11.0.2, §14.3.2, §15, §18.6). What is left to build is here.

Three of the four repaid the read in a direction the survey did not predict. `ruflo`
is valuable as a **negative control** — a self-optimising loop that logged its own
failures faithfully enough to be evidence — rather than as an optimisation design.
`hindsight`'s benchmark turned out to be the claim needing checking rather than the
evidence that settles §11. And `superpowers` was filed as a harness and again as a
catalogue when it is a **measurement discipline for skills** — which is why almost
none of it lands here. Its findings went to `skillet`, `skillsaw`, `exegesis`, and
`adh`, and are recorded in each of their backlogs; only the relay-test item below is
gnosis's.

- [ ] **A miss log cannot show that FTS5 was wrong, and §11.0 says it will.** *Stated
  in §11.0.2 and §6.4.1; what remains is the instrument.* `standards/retrieval-cases.toml`:
  labelled queries with expected concept ids, **including cases whose correct answer
  is that the corpus holds nothing**, graded by exact id match against
  `gnosis search --jsonl`. No judge, no model, no threshold — a finding surface, not a
  gate (§17). Cases authored when a real query disappoints, never invented up front.
  Phase 4, with the reranker whose admission evidence it is.
- [x] **A mutation does not verify that its audit row was written.** `AuditVerified`
  appends and re-reads the tail; the unverified append is unexported so the compiler
  enforces it. Two things the entry did not anticipate. §15 and `Audit`'s own comment
  looked contradictory and are about *different events* — a failed append is a known
  gap and stays fail-soft, a successful append with nothing on disk is the trail lying
  and fails hard — so the coordinator carries two fields. And "a mutation" is four
  mutations: `init` and `index rebuild` append outside the coordinator, so verifying
  only in `Execute` would have satisfied §15 for half its subjects.
- [x] **`bundle.AuditTrail` skips malformed lines instead of counting them.**
  *The premise was already stale when written: it had stopped skipping and started
  erroring on the first bad line, returning no rows at all — worse than either
  option, and my doing.* Now returns a `Trail` carrying the rows and the failed line
  numbers, with `Whole()` as the only place the damage becomes an error. A value and
  a method rather than a value and an error, because Go's convention is that a
  non-nil error makes the value untrustworthy and this requirement is that the rows
  stay usable. `LoadChecks` keeps fail-whole: a partial trail is an incomplete
  answer about history, a partial check record is a wrong answer about the corpus.
- [x] **`gnosis doctor` should report the trail's own health.** Malformed-line count,
  named by line, as a warning — a damaged trail leaves the corpus checkable and makes
  its history unrecountable, so blocking would fail `doctor` on a corpus with nothing
  wrong with it. **The timestamp comparison is not built and §15 is corrected.**
  Running it showed a commit newer than the last row is the *ordinary* state: people
  edit markdown by hand and commit, and git commits are not gnosis's writes. The
  check would have fired on the normal workflow. The failure it was inferring is now
  caught directly at the append.
- [ ] **Upstream drift resolves to three states, and `quotecheck` already computes
  the discriminator.** §14.3.2 specifies `drift-benign` / `drift-unsupported` /
  `drift-unchecked` — re-run the recorded passages against the *new* bytes. Today both
  the cheap maintenance case and the loss of upstream support report as `stale`. The
  code is a loop over passages already stored; the work is the finding and the
  reporting.
- [ ] **`rationale` admission needs the fold-and-compare refusal.** §10.6.4 specifies
  it: refuse a rationale that folds to the emitted prompt's own template text, and one
  byte-identical under `Fold` to a rationale already recorded for the same `subject`,
  naming the earlier warrant in the diagnostic. Not applied to `override.reason`. This
  is the observed failure mode of §10.6.4's central bet, not a hypothetical one — a
  surveyed system with the same required field had to warn its agents in prose that
  they were emitting the template verbatim.
- [ ] **`gnosis audit --outstanding`.** Enumerate required decisions that were never
  made — a promote that reached `needs_human` and was abandoned, a challenge opened
  and unresolved. The states are already committed frontmatter (§10.7.4); the report
  is missing, and absence is the one thing an append-only log of writes cannot show.
  Phase 3, with §10.
- [ ] **`skillsaw`: a `PASS → FAIL` transition is its own status.** Emitted once on
  the transition, not incrementing the failure counter, and carrying the diff between
  the last passing run and now so the next attempt is a diagnosis rather than a
  reimplementation. Distinct from the re-verification item above, which is its
  precondition.
- [ ] **`skillsaw`: the failure counter must be consecutive and reset on success.**
  Otherwise a flaky dimension accumulates to a terminal verdict on elapsed time. The
  reasoning is worth keeping with the code: recurring flakiness should surface as
  repeated regressions, which is a signal about the *check*, not a permanent verdict
  about the skill.
- [ ] **`skillsaw`: audit the rubric for count-shaped dimensions.** `ruflo`'s loop
  moved a harness score 40 → 55 with no capability change, by adding files a
  presence-counting dimension rewarded — and one of those artefacts, a bare symlink,
  is still in its repository root. **Any dimension scored by counting artefacts is
  gameable by creating artefacts.** Check `rubric.Dimensions()` against exactly that
  predicate.
- [ ] **`skillsaw`: a change that loosens a gate may not score as improving it.**
  gnosis's `standards.Value[T]` records the loosening direction, and `ruflo`'s
  optimiser proves recording is not sufficient — it relaxed a promotion predicate from
  `AND` to `OR`, doubled the promotion rate, logged it as a win, and committed. The
  ratchet has to *read* the direction, not merely store it.
- [ ] **`skillsaw`: estimate the evaluator's noise floor before accepting a delta.**
  `ruflo` accepted `+0.0028` and rejected `−0.0004`, an order of magnitude apart, with
  no variance estimate and no re-measurement of the champion. `skillet/stats` and
  `timeseries` are already there. A ratchet that accepts on one noisy measurement and
  never re-measures drifts upward on noise alone.
- [ ] **`canonizer`: `verify.Provenance` wants the two-signal cross.** `ruflo`'s
  witness manifest pairs a whole-file hash with a semantic marker and reports
  `Pass` / `Drift` / `Regressed` / `Missing`, because *"a SHA-256-only check would flag
  every benign whitespace change as a regression."* That is the resolution for
  `anchor-absent` conflating a fabricated anchor with a drifted source — the same
  finding as §14.3.2, one repository over.
- [ ] **`canonizer` and `adh`: record a critic that ran with reduced independence.**
  The surveyed judge spawns a fresh-context subagent and, when it cannot, runs inline
  **and emits an event recording the downgrade**. Neither `checked` nor `unchecked`
  covers *checked under reduced independence*, and a gate that silently degrades its
  own isolation reports a verdict it did not earn.
- [ ] **`adh`: guard invariants belong in the hypothesis, frozen before the run.** The
  same organisation whose optimiser gamed its rubric later rejected two changes with
  large favourable primary metrics because a *named* guard moved — a density invariant
  and a recall floor — under a hypothesis marked *"frozen before evaluation began; not
  modified after seeing results."* An objective without guards is hill-climbed by
  trading away everything unmeasured.
- [ ] **A corpus does not know whether an admitted claim was ever used.** `ruflo`'s
  nightly loop shipped 4 of 80 proposals over three months and could not see it,
  because each run checked only whether it was repeating itself. gnosis's promote gate
  decides admission and nothing asks about downstream reliance — the test the survey
  already adopted from `haft`, now with a measured cost for omitting it. The link
  graph holds the answer; the report does not exist. Phase 4.
- [ ] **The relay test now has three methods and gnosis has the useless one.**
  *§18.6 specifies all three.* Build the scripted-model fixture — real binary, real
  emitted prompt, reasoning replaced by a local server, and the fixture asserting on
  what the agent **sent** and not only on what it received. Record a real-model run
  graded by a pure predicate over the transcript as evidence, never as a gate:
  `superpowers` runs exactly that shape under an isolated `HOME`, and its second
  assertion — that **nothing else happened before** the required step, against an
  explicit allowlist — is one prose replies cannot be trusted to make about
  themselves. Phase 2 for the fixture; the real-model run whenever it is wanted.
- [ ] **No map says what each of this family's suites adds over the others.**
  `superpowers`' `docs/testing.md` annotates every test with its coverage delta
  against the other harness — *"drill covers the YAGNI subset; bash adds commit-count,
  task-tracking, and token telemetry assertions"*, and *"tests description-recall, not
  behavior."* gnosis has property tests, mutation tests, corpus fixtures, adversarial
  fixtures, and soon a relay fixture, and nothing states what a second suite covers
  that the first does not. Cheap, and it is the artefact that makes a redundant-looking
  test defensible instead of deletable.

______________________________________________________________________

## Applicability — This Repository Is the Family's Reference (2026-08-22)

`skillet` resolved its long-open decision on naming a general `Applicability` type. The
answer is **no type, a rule** — and the rule is this repository's sentence, lifted verbatim
from `internal/lint`'s package doc. Recorded here because being the reference implementation
is a constraint, not a compliment: three other repos now cite this design, and a refactor
that treated `Report.Skipped` as cosmetic would break something outside gnosis.

Two corrections landed in §12 with it: `coherence`'s `Convention bool` is **not built
anywhere** — it was a description cited as prior art and counted as a family member — and
`internal/lint` is the only implementation of the idea in the family, with the part
`Convention` lacks, which is the reason as a first-class output.

- [x] **§12's attribution corrected and the obligation stated.** *`Convention` named as
  unbuilt; `Check.Applies (bool, string)` + `Skip{Check, Reason}` named as the family
  reference; the corollary recorded — what gets suppressed is the consumer's choice, the
  reason is not.*
- [ ] **`Report.Skipped` is now load-bearing outside this repo; give it a test that says
  so.** Today the guarantee lives in prose (*"every check that did not run appears in
  Skipped with a reason"*) and in `Run`'s six lines. A test that constructs a registry with
  one always-inapplicable check and asserts the skip is reported **with a non-empty
  reason** pins the property that three other backlogs now reference. Cheap, and it is the
  difference between a documented intent and a checked one — which is the distinction this
  specification spends §18 on.
- [ ] **If a second tool emits a check report, `Skip{Check, Reason}` promotes.** That is
  the trigger `skillet` recorded, and gnosis is the current sole holder. Nothing to do now;
  noted so the type is not casually reshaped in a way that makes lifting it awkward — it is
  two strings and should stay two strings.

## Commissioned Gap Report — Checked, and One Correction (2026-08-22)

Source: `~/Documents/agent-green/FPF/gnosis_topten.md`, one of seven such files. Checked
against the code rather than filed. **Nothing from the gnosis file lands** — its ten gaps
describe `llmwiki`'s design defects, which gnosis was built to avoid and did. Gap 1 wants
`sources` re-keyed off `UNIQUE uri` in `internal/db/db.go`: that path is `llmwiki`'s and
does not exist here, and §4.3.1's fetch ledger has been one immutable record per source
version since Step 2.3. Gap 2 wants raw bytes stored durably; `evidence/text/<sha[:2]>/…`
has held them since the same step. Gap 8 wants sentence-level claim segmentation;
`internal/segment.Claims` shipped in Step 2.1. Gap 3 wants a background consolidation
worker, which is §14.3.1's *nothing here is periodic* and the `obsidian-second-brain`
unattended merge this survey already refused. The reasoning generalises across all seven
files and is recorded once, in `skillet/TODO.md`.

**One item was worth chasing, and it found a defect in this specification rather than in
the report.** The report proposed adding `certainty` to `finding.Diagnostic`; §16.1 had
proposed the same thing, and both are wrong for the same reason.

- [x] **§16.1 proposed a field that duplicates `finding.Action`.** *Fixed.* §16.1 described
  `Diagnostic` as `{severity, category, path, message}` — omitting `Action`, which exists —
  and then argued for `certainty` on the grounds that severity does not say who acts.
  Severity does not; **`Action` does**, and `agentsys`'s HIGH/MEDIUM/LOW maps one-to-one
  onto `automatic`/`guided`/`human`. §16.1 now carries that mapping, keeps `certainty` only
  as a rule about *when* `Action` may be set (requisite uncertainty, which §17 depends on),
  states that `fix_class` is per-check configuration in `standards/evidence.toml` and never
  a `Diagnostic` field, and closes with the general rule: **before adding a classification
  axis to a shared type, check the axes it already has.** §5.4's `findings` table drops the
  `certainty` and `fix_class` columns for `action`.
  Recorded as done rather than deleted because the same proposal arrived twice from
  different directions, which is what a duplicated axis does — it looks like a gap from
  every angle except the one that lists the existing fields.

## OKF Conformance — the Actor Divergence (2026-08-22)

`skillet` re-reviewed its *OKF trust fields* decision and the review turned up
something in this repository rather than in the kernel: **`gnosis.Actor` is already
built and it does not accept two of the three actor forms OKF §7 defines**, while
§14.1 states the trust fold implements OKF §5.3 verbatim.

Nothing is broken today — `Actor` carries audit rows, warrants, and approvers, and
the fold is specified but unbuilt. The point of recording it now is that the type
shipped *without touching trust metadata at all*, which is why `skillet` moved its
own promotion trigger from **stores trust metadata** to **classifies an actor**. The
same reasoning applies here: the cost of finding this after §14.1 is a corpus whose
tiers were computed by a parser that refused half its inputs.

- [ ] **Write the OKF conformance table test before §14.1 is built.** *Specified as
  §18.5.1.* Six rows over OKF §7's three forms plus `Actor`'s two additions,
  asserting for each what `ParseActor` does **and** what tier the fold yields. The
  two rows where those disagree — `process:finance-nightly` and
  `reference_agent/gemini-2.5-pro`, both rejected by the parser and both
  machine-confirmed for tier purposes — are the whole test; one that omits them
  passes under exactly the merge that breaks conformance. Cheap, and it belongs
  before the code it constrains rather than after.
- [ ] **§14.1's fold reads raw strings, not `gnosis.Actor`.** *Stated as §14.1.1;
  this is the implementation note.* Two populations, two treatments: the closed enum
  stays for actors gnosis mints, because §10.6.4 counts distinct humans and a kind
  that could pass for a person makes that count wrong in the flattering direction;
  frontmatter that arrived from elsewhere gets a permissive read asking only *is this
  `human:`?*, which is the sole question OKF §7 says a trust classifier needs. An
  unrecognised actor is never an error and never promotes a tier. Do not widen
  `Actor` and do not narrow the fold to it — §11 forbids rejecting a conformant
  concept, and §10.6.4 forbids an open mint-side grammar.
- [ ] **`gnosis` is the trigger the kernel is now watching.** `skillet` will promote
  §5.3's **fold** — a pure function over actor strings — when a *second* repo
  classifies an actor or derives a tier, and explicitly not the `generated`/`verified`
  record types, because `okf` keeps frontmatter verbatim and a struct would be
  decode-only. Build the fold locally, keep it a pure function over `[]string`, and it
  lifts unchanged if a second consumer appears. Phase 3, with §14.

## Reviewed Summaries — `hindsight` (2026-08-22)

*Superseded in part by the deep read above: this section reviewed a commissioned
summary, and the repository's own benchmark harness was read afterward. The
LongMemEval figures are quoted as reported and not as comparable to the benchmark's
published results — the grader diverges from the paper's, and §11.0 now says so.*

- [x] **§11.0 now names what refusing semantic search costs.** *The section was
  better than this entry credited — it already cited a measurement and named the
  condition for enabling embeddings. The real gap was that it cited only favourable
  evidence. It now states the LongMemEval numbers, the three reasons they are not
  decisive here, and the honest form of the position: a retrieval ceiling accepted
  in exchange for an auditable path.* Original: `hindsight`
  reports 94% and 91% on LongMemEval for an architecture gnosis declines on
  inspectability grounds. The task is not gnosis's and the number is the vendor's,
  but "we decline this, and here is roughly what declining costs on an adjacent
  benchmark" is a stronger and more honest section than the one there now, which
  argues entirely from principle. Revise §11.0 to state the trade.
- [ ] **Nothing holds an *experience*.** A standard and an episode are different
  knowledge with different evidence: "React 19 prohibits X" versus "the team applied
  this here and approved it." The second is arguably the tribal knowledge this
  project is named for and there is no document type for it. It belongs in tier 2
  with a commit hash as evidence — **not** tier 3, where the summary put it, because
  a session trace cannot be re-derived and §4.5 forbids anything existing only in
  SQLite. Needs a type in §5.8 first, so Phase 3.
- [ ] **Multi-source corroboration is not recorded as a structure.** A claim
  supported by four independent sources and one supported by one look alike in the
  frontmatter. The salvageable half of `hindsight`'s proof count: record *which*
  sources support a claim without collapsing them into a number, since §1.1's local
  reductionism refuses the inheritance a count implies.
- [x] **§8.0 states that one verdict may have two renderings and never two
  verdicts.** *With the strict-in-CI proposal named as the thing it forbids.*
  Original:
  The disposition-traits proposal — strict in CI, lenient locally — would give a
  developer a pass locally and a failure in CI for one corpus at one commit. §4.6
  implies this is forbidden and no section says it outright, which is why a
  reasonable reviewer proposed it.

## Reviewed Summaries — `scientific-agents` and FPF (2026-08-22)

Both summaries are in `manifesto.md` with what survived review. These are the items
worth doing; the contested ones are recorded there rather than here, because a
position argued against is not a task.

- [x] **The ontology records rejections.** *§5.8.2 requires them, `ontology.Rejection`
  carries the phrase and a required reason, the loader refuses a reason-less
  rejection and a phrase that is both admitted and refused, and the seed carries two
  worked examples.* Original: FPF's `F.18 NameCard`
  carries the candidate set, the chosen name, **and the rejected candidates with the
  reason each was rejected**. gnosis keeps only what matched. A rejected alias is
  precisely the knowledge that gets re-litigated — somebody proposes `runbook` for
  `Playbook`, the person who knows why it was refused is not in the room, and the
  corpus cannot say. One list and one sentence per entry; it is the required-
  rationale discipline applied to vocabulary. Best item from either summary.
- [ ] **Nothing records what an ingestion does *not* authorize.** NSTD.1's blocked
  overread. gnosis records what a source supports and keeps no trace of what it was
  explicitly held not to support — the same asymmetry as the rejections gap, one
  level up. Natural home is the fetch record or the quarantined document.
- [ ] **A corpus-level competency-question suite.** Natural-language questions the
  corpus must answer, frozen as tests. gnosis has dispatcher-level read tests and
  nothing asking whether the corpus still answers what it was built to answer. The
  proposed mechanism — asserting specific UUIDs — is wrong for gnosis, since
  identifiers are assigned per corpus; assert on titles or on claim text.
- [ ] **A known-answer soundness test per rule, in `canonizer` and `skillsaw`.** The
  gate's planted-defect self-test generalised: every rule ships a case it must flag
  and a case it must not. Trust is more sensitive to false alarms than to misses, so
  soundness is the property to prove first.
- [ ] **Name the object/metalanguage split in `skillet/ruleset/conflict`.** It is a
  metalanguage check — rules about rules — and calling it that would keep a future
  contributor from adding object-level checks to it.
- [ ] **`skillsaw` and `canonizer` cache keys should carry the rubric edition.**
  gnosis's relay key does not need it: a `standards/` change cannot stale a reply,
  because the rubric never enters the prompt. Where the rubric *is* what is being
  applied, a key without it serves yesterday's grade under today's rules.
- [ ] **Record what a claim costs to keep current.** FPF's C.27 carries an `Effort`
  field beside its validity window. gnosis knows when a claim goes stale and not
  what re-checking it costs, so it cannot distinguish knowledge that is expensive
  from knowledge that is merely old. Its sibling in that pattern — decay curves — is
  rejected in the manifesto, and this field is separable from it.

## Field Survey — `agent-green` (2026-08-21)

Findings from the governance/memory survey recorded in `manifesto.md`. Ordered by
how cheap they are relative to what they buy.

- [x] **`index rebuild` refuses a collapse.** *§4.5, `index.FloorBreached`, and
  `rebuild_floor_fraction` in `archive.toml`. The previously indexed count is
  `len(indexed)`, already loaded to compute drift, so the plan's meta row was
  unnecessary. `--force` overrides; `--check` is exempt because it writes nothing.* `haft` hard-
  rejects a refresh whose derived unit count falls below 50% of the last verified
  one. A gnosis rebuild that finds three documents where there were five hundred is
  a corrupted bundle or a bad `--bundle`, and it currently writes that index without
  comment — destroying the only copy of the state that would have shown the problem.
  Cheapest high-value item in the survey.
- [x] **A reader cannot see a claim's freshness.** `show` renders the state and why.
  Still per *document* rather than per *claim* — `obsidian-second-brain`'s inline
  `(as of YYYY-MM, source.com)` is finer-grained than this, and reaching it needs the
  claim-level source join that §5.5.1 anchors would support. Filed below.
- [ ] **`skillsaw`'s ratchet may not re-verify prior passes.** `oh-my-agent`'s judge
  re-checks every criterion each iteration "because fixing C2 is how C1 silently
  regresses." Needs checking in skillsaw; if the ratchet re-scores only failed
  dimensions it cannot see a regression its own fix caused. *Sharpened by the
  2026-08-22 protocol read: re-verification is the precondition and not the finding.
  A `PASS → FAIL` transition wants its own status, emitted once, routed differently
  from a first-time failure — and the failure counter must count consecutive failures
  and reset on success, or a flaky check accumulates to a permanent verdict on nothing
  but elapsed time. Both below.*
- [ ] **Nothing says what the corpus should decline to hold.** `gentle-wiki` scopes
  itself by refusing to become "a second source of truth for product behavior."
  §5 says at length what a document is and nowhere what does not belong. A corpus
  restating what the code already says drifts invisibly, because both halves stay
  internally consistent.
- [x] **The promote gate can deadlock with no cap and no recorded reason.** Resolved
  as the entry framed it: a bound with a recorded reason. `gate.Decision` separates
  *could not check* from *checked and failed*, and only the first is carryable. The
  reason is recorded in the audit row's new `signals` field, so the debt is
  enumerable rather than merely permitted.
- [x] **The link graph is untyped and nothing said what that means.** *Fixed:
  §5.5.1.2 states that an empty `rel` asserts nothing, that arrangement is not
  causality, and why typing the vocabulary waits on §5.8. §20's trail entry already
  required prose stating why the order is what it is, which was better than the
  finding credited.* Remaining work — populating `rel` — is Phase 3. Original: FPF is
  relation-first: order is layout until a claim says it is a path. `systems-thinking`
  names Factor Listing as an anti-pattern for the same reason. gnosis cannot
  distinguish "cites", "supersedes", "causes", and "is filed near". Typing the graph
  needs a relation vocabulary, which is §5.8's problem, so this is Phase 3 at the
  earliest — but §20's trails entry should stop assuming an ordered list is a path.
- [ ] **No claim carries a causal rung.** FPF's CAUSAL-USE exists because
  association, intervention, and counterfactual claims get treated as
  interchangeable. §10.2's constraint extraction is the natural place for a rung, and
  a claim whose wording is causal but whose evidence is associational is exactly the
  silent upgrade §9.4 guards against for quotations.
- [ ] **`gnosis schema` should adopt the marker contract.** `agents-md`: generated
  regions in HTML-comment markers, everything outside preserved, and **a file with
  no markers is never overwritten** — write `AGENTS.generated.md` beside it. Rule
  three is the fail-closed one: a file predating the tool was not written under its
  contract. Closes the open machine-vs-human-owned-regions item.
- [ ] **Consider a fourth gate verdict: admitted-with-findings.** `haft`'s
  `review_ready` is "an auditable semantic-delta classification, not a veto" — the
  candidate is adopted, a prominent warning prints, every finding is retained. gnosis
  has pass/fail/unchecked and no way to say "this landed and here is what we noticed."
  Whether §9.5 wants one is a genuine question, not an obvious yes: the whole point
  of `unchecked` blocking is that a gate which can be talked past is decorative.
- [x] **"Retrieval is not evidence" is now stated where a reader hits it.** *Added
  as §11.0.0, ahead of everything about making things findable, and it names the one
  relation that does make something evidence.* Original:
  FPF: "a publication carrier does not become its subject, and a readable view does
  not become evidence, assurance, permission, decision, architecture, or work
  without the corresponding exact relation and test." §11 and §17 both imply it;
  neither says it.
- [x] **§10.7.4's rule has a sharper formulation, and now uses it.** *Reliance is
  the operative test — does later work have to rely on this? — with
  committed/observed kept as how to recognise it, plus the audit row as the case that
  needed the sharper rule to settle.* Original: `haft`: records become durable
  when later work must **rely** on them — handoff, replay, authority, automation,
  evidence. "Decisions are committed, observations are cached" agrees everywhere it
  has been applied; reliance is the version that decides the next case without a
  fresh argument. Worth adopting as the stated test with the current wording kept as
  its gloss.
- [x] **§1.1.1 names the field's default and why it is right for its audience.**
  *Four systems now, including hindsight's proof counts. The section's honest form
  is that unverified accretion is correct for a memory and wrong for a record.*
  Original: — `obsidian-second-brain` (rewrites pages), `Acontext` (an LLM distillation
  pass writes skill files), `doceo` (saves lessons and self-revises from feedback).
  Not an action item for gnosis so much as a calibration one: §1.1's posture is
  contested by the field's default, not merely by an imagined opponent, and the
  specification currently argues against a position nobody in it is named as holding.
- [x] **The Gentleman Programming ecosystem examined as an ecosystem.** *Recorded in
  `manifesto.md`; the items below are what it produced.*
- [x] **§4.3.1 over-claimed what content-addressing detects.** *Fixed: §4.3.1 now
  says a careless edit is visible, states tamper-resistance against a same-user local
  actor as an explicit non-goal, and names git-on-a-remote as the control that does
  provide it.* Original finding: It says a rewritten
  record makes tampering "visible rather than absorbed." True for a careless edit and
  false for a local actor who recomputes the hash and renames the file.
  `gentle-ai/docs/review-authority-threat-model.md` has the sentence to adopt:
  "checksums only where useful for detecting accidental corruption; they are not
  authentication" — together with an explicitly stated non-goal, that no
  tamper-resistance is claimed against a same-user local actor without an external
  trust anchor. gnosis should state the same non-goal rather than let a reader infer
  a stronger one.
- [ ] **No AI-assistance policy in any repository of this family.** *Listed in PLAN
  §4.1; it is a repository file rather than a spec change, so it stays open here.* Every one of them
  is built this way and none says so, while §1.1 argues that a claim must name its
  witness. `gentle-ai/AI_POLICY.md` is the model, and three of its rules are directly
  adoptable: review on observable quality rather than on whether output looks
  AI-generated; reject what the contributor cannot explain or defend; and no human
  attribution trailers for tools, with an optional `Assisted-by`. The third is
  `gnosis.Actor`'s human/agent/check split expressed in git.
- [ ] **Nothing tests that an agent's real reply is one `admit` accepts.**
  `cmd/relay_test.go` hand-writes every reply, which the relay's design made easy and
  which leaves the seam untested. `gentle-ai`'s method applies directly: keep the
  runtime real, replace only the reasoning with a scripted local model server, and
  **make the fixture adversarial** — assert on what the agent sent, not only on what
  it received. Their own honest-limits note bounds the claim: it does not prove a live
  model produces the same calls, and that belongs to usage rather than to CI.
- [x] **`bundle.AuditTrail` cannot distinguish corruption from operational failure.**
  `AuditTrail` and `LoadChecks` now report a malformed line as corruption *with its
  line number*, distinct from a read failure. The honest limit is recorded in §15 and
  in the code: `errs` has five codes and none means "the bytes on disk are wrong", so
  this is `EINVALID` with a message that says corruption — **legible rather than
  machine-checkable**. A sixth code belongs at the second consumer, not the first.
- [ ] **A declined promotion is logged as an observation, not recorded as a decision.**
  gnosis writes a refusal to `audit.jsonl`, which is per-user and gitignored.
  Gentleman records the decline itself as a canonical authorization, atomically. By
  §10.7.4's own rule a decision to decline is a decision, so it arguably belongs in
  the committed tier — which is a §9.5 question and not an implementation one.
- [ ] **`revision_count` is nearly free and answers a question git cannot cheaply.**
  `engram` increments a counter on upsert so an evolving record says how many times it
  has changed. gnosis keeps full history, which is strictly more information and
  strictly more expensive to query: "has this claim been churning?" currently requires
  walking git. A derived count in the index would answer it in a query.
- [ ] **`skillsaw` has no cross-repository skill-identity check, and should.**
  `Gentleman-Skills` and `gentle-ai` declare a portability convention in prose —
  unprefixed names are portable and keep their canonical names, `<tool>-*` names are
  repo-specific — and it verifiably holds: `cognitive-doc-design` is byte-identical
  across two repositories and `gentle-ai-branch-pr` has legitimately diverged. It
  holds because one person maintains both. A check that same-named unprefixed skills
  hash alike across a set of repositories is the mechanism this family could supply
  back, and `identity.Hash` already does the hard part.
- [ ] **`skillet`/`steve-skill-market` do not distinguish a shipped skill from a
  repo-governing one.** `gentle-ai` keeps `internal/assets/skills/` (embedded, ships
  to users) apart from `skills/` (repo-local, governs work on the tool). A shipped
  skill is a published artefact under `speclint`'s rules; a repo-governing one is
  closer to a `CONTRIBUTING.md`, and grading them against one rubric will misjudge
  both.
- [x] **§10.6.4's bet acknowledges the case against it.** *Added: quorum works where
  reviewers are many and reversals cheap, gnosis has the opposite profile, and the
  condition under which the rationale requirement should be revisited is named.*
  Original: `Gentleman-Skills`
  admits community skills by seven-day review and reaction quorum — a
  permission-and-quorum model, where §10.6.4 holds that a required rationale filters
  more bad adjudications than a permission check. The asymmetry is blast radius: a bad
  skill is uninstalled, a bad claim is cited. Worth a sentence in §10.6.4
  acknowledging the case where counting is the cheaper instrument, so the position
  reads as a choice rather than as the only option.
- [x] **§9.3's execution-surface rule is stated.** *Added to §15: any string a reply
  supplies which later selects a file, a command, or a check is validated against a
  closed set, and where it selects a command the set is an allowlist with refusal as
  the default.* Original:
  `oh-my-agent` allowlists exactly three executable commands "so an agent that writes
  anything else into the state file gets it ignored, never run." gnosis already
  refuses traversal on a quarantined path; the general rule is the one to write down.

## Field Survey (2026-08-21)

Source: `~/Documents/agent-purple` — 29 implementations, 10 documents. Recorded in
`manifesto.md`; four findings went into `SPEC.md` (§17.0.1, §9.4, §11.0, §14.3.1)
and `PLAN.md` §5.6. What remains open is here.

- [ ] **Mark each specification rule by what enforces it.** `canopy`'s philosophy
  tags every principle `[code]`, `[convention]`, or `[code+convention]`. This spec
  is full of MUSTs and says nowhere which are machine-checked and which rest on
  people behaving — a reader cannot tell a guarantee from an intention. Two
  concrete cases: §4.1's append-only tier 0 is enforced and reads like a
  convention, while several rules phrased as MUST are conventions with no checker.
  Cheap to add as a column or a marker; the value is that it makes the unenforced
  ones visible enough to either enforce or downgrade.

- [ ] **§12's check table should carry an auto-fixable column.** `kb-lint`'s does.
  We have the axis already — `finding.Action` is `automatic`/`guided`/`human` — and
  the table does not show it, so a reader cannot see at a glance which findings a
  tool can close for them.

- [x] **Two lexical checks, now built** (`placeholder`, `empty-section`). The
  empty-section rule needed a level distinction the first implementation got wrong:
  a heading followed by a *deeper* one is a parent whose content is its
  subsections, and only a same-or-shallower successor leaves it empty. Caught by
  its own test. Original finding: `{{PLACEHOLDER}}` markers left in
  a document, and empty sections. Both are what an agent leaves behind when it runs
  out of material, both are pure text, and neither needs a model. From `kb-lint`.

- [ ] **Machine-owned versus human-owned regions of a document.** §6.3 splits
  accretion from synthesis at the level of the *operation*; `wenlan` splits at the
  level of the *region* — a refresh rewrites machine-written prose and **stages
  human-written prose for review**. That is the finer and more survivable version,
  because the common case is an agent refreshing a page a person has edited, and we
  currently resolve it by gating the whole rewrite. Needs a way to mark regions,
  which is a frontmatter or fence-marker design question. Phase 3.

- [ ] **Graded conformance rather than a boolean.** `akbp` runs "level 3
  conformance". §11's OKF conformance is pass/fail, so a producer cannot state how
  far it conforms and a consumer cannot require a level. Worth defining if gnosis
  ever exports bundles other tools consume — and `cq-gitstore` and `expo-llm-wiki`
  suggest that is where OKF is heading.

- [ ] **Review-gated writes as a protocol property, not a command.** `akbp` puts
  `dry_run` / `approved` / `approval_required` on every write call; our promote gate
  is a command someone runs. Theirs is harder to bypass. Relevant when §4.6's write
  coordinator gets its API — that is the moment to decide, and after it the answer
  is baked in.

- [ ] **Consider `okf` (skosovsky) for `internal/okf`.** A Go OKF toolkit with
  `bundle`, `validator`, `graph`, `store` packages and transactional mutations. The
  reason to keep ours is narrow and real — `Parse`/`Render` retains the frontmatter
  block verbatim so a round trip is byte-exact, which a library that re-encodes YAML
  cannot offer. Re-check if that stops being true.

- Noted, no action: `stigmergy` gates *identity* on a human steward (an unknown name
  parks the capture until someone decides) where we auto-assign UUIDv7 at admission.
  Different problems — they gate *which entity this is*, we gate *whether this claim
  is supported* — but their approach catches the duplicate-concept case §4.6.1
  leaves to a post-merge check.

- Noted, no action: `kvt` and `kb-lint` both **regenerate** `index.md` as a derived
  file; §5.6 keeps it a curated map. Two independent projects disagreeing with us on
  the same point is worth knowing, and the disagreement is real rather than an
  oversight: a generated listing is available from `search` and `graph`, and what
  `index.md` is for is the handful of paths a newcomer actually needs.

- [x] **`gnosis_schema_version` and its check are built.** *`okf.Int`/`okf.Has`
  accessors, a nullable `SchemaVersion` on the document model, `gnosis.SchemaVersion = 1`, and a `schema-version` check that skips until the corpus starts versioning.*
  Original finding: A document records the corpus conventions it was written under; `lint`
  reports documents older than the current version and never rewrites them.
  Not urgent, but the first instance is scheduled rather than hypothetical: no
  Phase 1 document carries the `gnosis_claims` frontmatter §5.5.1 requires, so on
  the day extraction lands every existing document predates the format and nothing
  distinguishes those from documents that *should* have claims and lack them.
  Cheapest if the field exists before that, since backfilling a version onto
  documents whose conventions are already unknown is guesswork.

- [x] **A self-contradiction in the spec, found by questioning the timestamp.**
  §9.2 said re-fetching an unchanged source "records a `fetch_history` row" while
  §5.5 said that table was keyed `(uri, sha256)` — under which the second unchanged
  re-fetch cannot append, only overwrite. The key's stated justification ruled out
  `uri` alone and settled nothing about `fetched_at`, so it read as decided while
  being underspecified. Both sections now agree, and `fetch_history` is replaced by
  `sources_fetched` (keyed by record hash, derived) plus `checked` (per-user, not
  reconstructible — the one documented exception to §4.5).

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

## Commissioned Gap Report, Round Two — Nothing Lands (2026-08-22)

Source: `~/Documents/agent-green/FPF/gnosis_todo.md`. Its verdict is *"no genuinely
unimplemented, valuable, and relevant gaps found"*, and read as *"this corpus surfaced
nothing new"* that is correct. **Full reasoning for the family is in `skillet/TODO.md`
under "Round Two, and What Asking for Code-Reality Verification Actually Bought."** The
citation failure it exhibits is recorded as evidence rather than as a complaint — §1.1.0
and the manifesto's closing section — because it is the best specimen anyone has supplied
of the failure this corpus exists to prevent.

Two addendum items were checked against the backlog rather than dismissed:

- **hindsight's dual-pathway storage** (world facts beside execution traces) is already
  here as *"Nothing holds an experience"*, which states it better: a standard and an
  episode are different kinds of thing, and the entry above works out why.
- **katalyst frontmatter schemas plus a `migrate-schema` pass** is covered on the
  validation half by OKF conformance and `gnosis.SchemaVersion`. The migration half — a
  programmatic rewrite of legacy cards when the schema changes — is genuinely unrecorded
  and genuinely premature: no corpus has a legacy card, and the shape of the migration
  depends on what the first breaking change turns out to be.

One is filed on its own merits below, because the idea is sound and the report supplies no
evidence for it beyond a corpus reference.

- [ ] **A claim records who approved it and not what authority they held.** `Actor` is
  `human:<id>` and `gnosis_warrant` carries a rationale, so a corpus can say *Priya
  adjudicated this* and cannot say *the platform team owns this rule*. `haft`'s decision
  records carry a role rather than only an identity, which makes two things possible that
  are not possible now: filtering a corpus to the rules a given team is accountable for,
  and noticing that a rule about deployment was adjudicated by nobody who works on it.
  **The reason to hesitate is §14.1's rule that a trust tier is a signal, never a
  permission.** A role field is one edit away from becoming a permission — *platform-team
  claims skip review* — which is the exact collapse that section forbids, and the pressure
  to make it one will come from whoever maintains the largest set of rules. If it is built,
  the role must be recorded on the warrant as an attribute of the decision and must not be
  readable by the gate. Worth doing only alongside a reader that uses it for reporting, so
  the first consumer establishes the non-gating precedent.
