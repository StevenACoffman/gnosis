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
  **REVIEWED 2026-08-22: the deferral is right and two of this entry's own claims are
  stale. Kept open, with a trigger that can fire.**
  **The command type now exists** — `internal/command` has the `Command` interface
  (`Op`/`Effect`/`Validate`), `Promote`, `Admit`, and a compile-time assertion — so the
  "should exist before the second writer" clause is satisfied. The interface comment
  carries the property that makes the transport small: *a transport that deserialises into
  a Command still has to hand it to something that calls Validate.*
  **And the stated unknown is half-answered by §4.6.2, which this entry cites.** That
  section already settles preview-versus-apply *as a type question*: **one command
  differing in one field**, so *"the same handler receives the same input, computes the
  same diff, and `Effect` decides only whether the final write happens."* `Effect` is built
  and fails closed. What remains open is the **protocol** question, which is different and
  smaller: whether a remote caller previews, receives a diff, and then sends a *second*
  command to apply.
  **That distinction matters because §4.6.2's argument has a premise a two-round-trip
  transport can break.** "They cannot diverge" holds because the handler receives *the same
  input*. Across two round trips the bundle can change in between, and then the diff the
  gate approved is not the diff that lands — precisely what §9.4 forbids. In-process today
  the writer lock spans compute-and-write so the premise holds for free. A served
  coordinator would need what the manifesto already names: **a lock plus an expected
  revision, so a stale writer is rejected rather than merely queued.** That is the actual
  prerequisite for a two-call protocol, and it is not recorded anywhere as an item.
  **Replacement trigger, because "Phase 2's real writers will say" has expired — Phase 2 is
  complete and every writer is in-process:** the first writer that is **not in this
  process**. Concretely, §13's served viewer (Phase 5) or an agent runtime calling `admit`
  directly. Until one exists the `flock` is not a compromise, it is the correct answer, and
  this entry should read as a deferral rather than as debt.
  **Preference when it fires, unchanged and recorded so it is not re-derived:** a Unix
  socket carrying the §8.0 envelope — the path is its own discovery, filesystem permissions
  are the authorization the bundle already uses, and the seam is an `io.ReadWriter` that a
  pipe can test. HTTP arrives anyway with §13 and is a second transport over that seam, not
  a competitor; MCP likewise if an agent runtime becomes the primary caller. The choice is
  reversible precisely because the gating lives in the type.
- [x] **The writer lock's contract is a type now, and one caller was not honouring it.**
  *`bundle.Writer` is obtainable only by taking the lock, and every write is a method
  on it — `Audit`, `StoreEvidence`, `StoreCached`, `RecordChecks`, `Prompts`,
  `Quarantine`, `Discard`, `StorePromptMeta`. The free functions were deleted rather
  than wrapped, so the unguarded entry point stopped existing.*
  **The predicted defect was real and shipped.** `gnosis fetch` wrote tier 0 **and
  rewrote `.gnosis/checked.jsonl` whole** — a read-modify-write over the entire
  file — under no lock at all, so two concurrent fetches could lose one user's
  observations outright. Seven functions carried the precondition in prose and this
  caller was the one that did not do it; nothing reported that, because a prose
  precondition has no failure mode. It just stops being true.
  Three things the fix turned up. The test helpers had to distinguish *held for the
  test* from *scoped to one write*, because a fixture holding the lock deadlocks
  against `Coordinator.Execute` — which is the mechanism working, not a wart: how long
  write permission is held is now visible rather than assumed. `index.ReplaceSources`
  keeps its prose precondition, because depguard forbids a parser importing the shell
  and relaxing that would trade a checked architectural claim for a checked
  precondition. And the one hole a type cannot close — a caller keeping a `Writer`
  past its `defer Release()` — is a runtime check returning EINTERNAL, with a test
  per method.
  Original: Seven call sites
  carry `Requires: the writer lock is held` in their doc comments — `bundle.Audit`,
  `AuditVerified`, `ingest`, and others — with no runtime assertion behind any of them. A
  second in-process writer that forgets to take it is a defect available **today**, where
  the transport question is one available in Phase 5.
  Same shape as `adh`'s `Critic.Deny`, found the same day: a guarantee that lives in a
  comment. Cheapest fix is for the lock holder to be a value the writing functions require
  rather than a precondition they document — a `*bundle.Writer` that can only be obtained
  by taking the lock, so the compiler enforces what the comment currently asks for.
  Recorded separately from the transport because it does not wait on it.

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
- [x] **One seeded sampler, and a first caller so it is not dead.**
  *`gnosis.Sample(seed, n, population)` in the domain package — the only place all
  three future callers can reach it, since `lint` and the Phase 3 curate packages are
  siblings and siblings may not import each other. Ordered by a keyed hash of
  `(seed, item)` rather than by a stdlib shuffle: §18.3's reproducibility must not
  depend on `math/rand`'s internals staying put across releases, and a hash key also
  makes the draw independent of the population's input order, which a shuffle is not.
  `standards/sample.toml` holds the seed, consumed today by `gnosis debt --sample`,
  so it is `ReadingConsumed` honestly rather than a fourth entry in `standards.Unread`.
  No `critic_default`: a default sample size is `critic`'s business in Phase 3, and
  declaring it now would be the dead value this was avoiding.*
  There is deliberately no `CompareSample`. A changed seed draws a different sample of
  the same size from the same population, so it is neither a loosening nor a
  tightening — a comparison returning nil would be a mechanism asserting there was
  something to compare. What a change *does* mean is that results before and after are
  not comparable, which belongs in log.md for its own reason.
  Original: §10.5's `critic --sample N`, §14.3.1's
  `stale --unreviewed`, and §6.2.1's random conflict pass all need reproducible
  draws. They must share one sampler reading its seed from `standards/`, or the
  three drift and none is reproducible under §18.3. Currently each is specified
  independently. Cheap now, awkward after three call sites exist.

______________________________________________________________________

## Noticed While Building Phase 2

- [x] **The git-shelling fixtures are hermetic, and this closed the "fragile under
  concurrent load" entry below as the same root cause.** *`GIT_CONFIG_GLOBAL` and
  `GIT_CONFIG_SYSTEM` are nulled and the identity moved into the environment, so both
  fixtures need no `git config` step at all — three subprocesses where there were six.
  Verified against a deliberately hostile global config setting `commit.gpgsign`,
  `commit.template`, `core.hooksPath`, `gpg.format` and `init.defaultBranch`: both
  packages pass with it in place.*
  **And the other entry's diagnosis was wrong.** "Fragile under concurrent load" read
  `fatal: failed to write commit object` as filesystem and subprocess contention in a
  `t.Parallel()` test. That is the line git prints when *signing* fails, and concurrent
  load was the trigger only because two runs meant two signing requests and the agent
  timed out. Three concurrent runs now pass in 3–4 seconds, so the parallelism was
  never the problem and is left alone — a change with no evidence behind it would have
  been the worse outcome than the entry.
  Worth keeping as a shape: **two backlog entries, one root cause, and the older one's
  explanation was a plausible story fitted to a symptom.** The tell was that the
  quoted error was the same string in both. Original: Found
  when `go test ./...` began hanging for two minutes and failing: `standardscmd`'s and
  `internal/bundle`'s fixtures shell out to `git commit` in a temporary repository, and
  because the *global* `commit.gpgsign` applies there, git blocks on the GPG agent. On a
  machine that signs commits the test hangs; on one that does not it passes. Both
  fixtures now set `commit.gpgsign=false` locally, and the run went from 120 seconds to
  1.6.
  Recorded rather than closed because the general form is unfixed: **these fixtures
  inherit whatever the reader's git configuration says**, and signing was only the
  setting that happened to bite. `core.hooksPath`, a global `pre-commit`, a templated
  `commit.template`, and `gpg.format=ssh` are all in the same position. The durable fix
  is one helper that runs git with a hermetic environment — `-c` for the identity,
  `GIT_CONFIG_GLOBAL=/dev/null`, no hooks — used by both, which is also the shape the
  already-filed concurrency fragility wants. Cheap, and it is the same lesson as
  `configuration-hermeticity`: a test that reads ambient configuration is testing the
  machine.
- [x] **`fetch` records an audit row.** *One row per invocation, actor `check:fetch`,
  `Paths` naming the records this run wrote and `Detail` counting the dispositions.
  None under `--dry-run`. Written even when nothing reached tier 0, following `init`:
  "we re-fetched and everything was already there" is a fact about this machine, and
  `checked.jsonl` records that the sources were looked at and cannot say that a fetch
  ran.* Original: Found while making the
  writer lock a type: `audit.OpFetch` is declared and appears only in tests, so the
  one operation that puts durable evidence into the corpus is the one mutation §15
  does not see. `init` and `index rebuild` were fixed for exactly this reason — a
  claim that holds for some of its subjects is the half-truth that section is about —
  and fetch was missed because it was not going through the coordinator *or* taking
  the lock. It takes the lock now, so the row is three lines and a decision about the
  actor: `check:fetch`, by §5.5's reasoning for `findings.opened_by`.
- [x] **§9.3 stage 4 over a candidate document is built, and it needed no new
  threshold.** *`archive.Oversize` applies the archive's own `per_file_cap` and
  `embedded_payload_cap` to the candidate; the caps reach it on `gate.Limits`, beside
  the two thresholds the gate already reads, because `security` is what needs them to
  say whether the stage ran. `scan.CoverageOf` replaced two functions that each
  answered part of the coverage question and neither of which could say stage 4 ran —
  coverage is now composed from what was performed, in the one place that performs it.*
  **The correction this entry itself predicted, now measured:** completing §9.3 moved a
  clean candidate's `security` verdict from `unchecked` to `pass` and did **not** remove
  the human path from the ordinary case, because `conflict` is unchecked for Phase 3
  reasons and withholds automatic approval on its own. A test pins it.
  Original: `archive.Gates` bounds a *fetched source* with `per_file_cap` and
  `embedded_payload_cap`; a document a model wrote is neither fetched nor archived, so
  the bound exists for one input and not the other and `Coverage` reports the stage
  missing. The resolution is to apply the **same declared caps** to the candidate
  rather than to invent a second bound — which is what §6.5 forbids and what the
  earlier note about this correctly refused. Until it lands, `security` reports
  `unchecked` on a clean candidate, which is why every promotion still needs a
  signature even with stages 2 and 3 built.
- [x] **`AuditTrail` and `Trail.Whole` have a production caller.**
  *`gnosis debt`. `Whole()` decides one sentence — whether the count is a total or a
  floor — which is exactly the distinction the method was built for.*
  Original: `Trail` and `Whole()` exist so
  a reader can tell a partial trail from a whole one, and the only readers are tests
  and `doctor`'s row count. `gnosis log --audit` or the `gnosis debt` verb already
  filed above is where it lands. Recorded because a careful API with no consumer is
  the same trap §6.5.1 is about, one layer up.
- [x] **§9.3 stages 2 and 3 are built, and this entry's stated payoff was wrong.**
  *`internal/scan/rules.toml` holds twenty rules across `prompt_injection`,
  `data_exfiltration`, `memory_poisoning`, `tool_misuse`, and `secret`; `LoadRules`
  compiles them and **refuses the whole ruleset if any rule fails its own
  `must_flag`/`must_not_flag` case**. Wired into both call sites — `archive.Gates`
  for a fetched source, `scanCandidate` for a candidate document — with distinct
  reject reasons, because an injected instruction means somebody is attacking the
  corpus and a leaked credential means somebody has a key to rotate.*
  **The correction: building these did not remove the human path from the ordinary
  case, which is what this entry predicted.** `conflict` reports `unchecked` for
  Phase 3 reasons and withholds automatic approval on its own, so every promotion
  needed a signature before and still does. What the stages buy is coverage — an
  injected directive or a committed credential now *fails* rather than passing
  unexamined. `TestAdmissibleCandidateAsksForAHuman` carries the note.
  **`betterleaks` does not exist.** Not on the public proxy under any casing, not in
  skillet. That is the fourth library this specification has cited that turned out
  not to fit or not to be there, after `go-git/v6`, the extractor name, and
  `skillet/auditlog` — PLAN §6.12 already named the pattern. What is there instead is
  the part needing no dependency: vendor-documented credential formats, which are the
  same class of justification as §9.3's Unicode ranges. Deliberately **no entropy or
  length heuristic**, because that would put a tuned number inside a blocking gate.
  The ruleset's own self-test caught one of my patterns on its first run — the Google
  API key example was two characters too long to match — which is the argument for
  self-testing at load rather than in a test.
  Stage 4 over a *candidate* is now §9.3's only gap, and it needs no new threshold:
  `per_file_cap` and `embedded_payload_cap` are declared and already justified for
  prose. Filed, not built.
  Original: Injection
  and exfiltration patterns need a pattern corpus with its own test set; secrets need
  a `betterleaks` dependency. Until they land, every promotion in every corpus routes
  through §9.5.1's human path, which is correct and is also friction on the ordinary
  case. These are the highest-value remaining Phase 2 items precisely because their
  absence is now measurable — the audit trail counts it.
- [x] **`gnosis debt` reports the accumulated debt.**
  *`bundle.Owed` is a pure fold over the trail; `cmd/debtcmd` renders it. Per-signal
  counts, the paths, who carried each, and the rationale — and the **denominator**,
  because 34 means something different against 40 promotions than against 4000.
  `--sample N` draws a reproducible subset. Read-only; takes no lock.*
  Two things came out of running it. A damaged trail makes the count a **floor**
  rather than a total, which is what `Trail.Whole` is for and is the only direction
  this report must not get wrong. And the first version printed the per-signal summary
  above the entries on stdout, where `conflict\t1` reads as the same shape as
  `conflict\tc/a.md\thuman:priya` — a reader could not tell a total from a row, and
  neither could `cut`. Data on stdout, summary on stderr, with a test pinning it.
  Original: `audit.Row.Signals` records which
  checks each promotion was carried over, and no command reads it. The obvious verb
  is `gnosis debt` or a `log --carried` flag: *these 34 documents were admitted with
  no conflict check*. Without a reader the field is a promise rather than a
  mechanism, which is the same trap §6.5.1 is about.
- [x] **A refused candidate has a route, and it is not an edit.**
  *Decided: **no `quarantine --edit`, ever.** Hand-editing quarantined content is how
  unvetted text acquires a human's authority without review — a person who fixes the
  sentence the gate objected to and promotes the result has produced a document that
  passed the gate and was checked against nothing, because they made the quotation
  validate. The sanctioned route is discard and re-admit: fix the input, run the relay
  again. `command.Discard` + a coordinator handler + `gnosis quarantine --discard`,
  requiring `--by` and `--reason` and previewing by default. It gives `audit.OpDiscard`
  its first reader.*
  **Running it found a worse problem than the missing verb.** A *refused* candidate
  reported `needs_human` — the same reason token an *unchecked* signal produces — so
  the CLI prompted for the confirmation phrase and then declined anyway. §9.5.1's
  policy is that the human path opens for what could not be checked and stays shut for
  what was checked and failed; `authorise` enforced that correctly and the *reported
  reason* erased it, which teaches somebody that typing the path is what unlocks a
  refusal. `gnosis.ReasonRefused` now exists and the message names the route.
  The test for the fabricated-quotation case had asserted `want needs_human` under a
  comment reading "a real failure, not an unbuilt check" — it named the distinction it
  existed for and asserted the value that collapsed it, and passed because both cases
  shared one token.
  Original: `promote` reports which
  signal failed and the author must re-run the whole relay to correct it. There is no
  `gnosis quarantine --edit` and arguably should not be — editing quarantined content
  by hand is how unvetted text acquires a human's authority without review. Worth a
  decision either way rather than an absence.
- [x] **Freshness is reported per claim as well as per document.** *§14.3.
  `bundle.ClaimFreshness` joins each claim's `archive_paths` to `checked.jsonl` and
  `show` prints it beside the anchor. The document line is still the weakest of its
  claims, so the conservative answer is added to rather than replaced. One
  measurement used at both grains, because computing them separately would let a page
  read fresher than a sentence it is made of; and the document's `stale_after` governs
  every claim under it, since a date is a statement about what the document asserts
  (§14.3.0) and a claim is one of those assertions.*
- [x] **A drift finding is stored on the observation that produced it.**
  *`checked.jsonl` gained `drift` and `revision`, and `show` prints the drift beside
  the freshness. The home is a type argument rather than a convenience: a fetch
  record's name is the hash of its own content (§4.3.1), so a field varying with a
  comparison somebody ran would re-record unchanged bytes — but an observation is
  per-user, already timestamped, and its whole subject is what this user saw when they
  looked. Absent means unknown on both, which the zero values already say, so there is
  no migration.*
  **A re-check now also writes an observation for the version it did not fetch**, which
  is the part this entry did not see: the verdict is about the *recorded* copy, so
  without that row a claim resting on the old archive path still read as last verified
  whenever it was first fetched, however recently its quotations had been confirmed
  upstream. Original: `fetch --recheck` opens a `drift-unsupported` diagnostic per affected claim and
  prints it; nothing persists it, so a reader who runs `show` on the claim a week later
  sees `fresh` — correctly, since freshness is about when a source was checked and drift
  is about what it now says, but the reader is not told the second thing at all. §14.3.2
  is explicit that drift "never rewrites or retracts anything", so the answer is not a
  frontmatter mutation. The plausible home is the same one the git revision wants:
  `checked.jsonl`, per-user and already timestamped. Both should land together or
  neither, because one reader is what justifies the field.
- [x] **`drift-unsupported` is enumerable, in the second of two checked places.**
  *`bundle.CategoryDriftUnsupported` is the one definition and a root test asserts
  §14.3.2.1 names it, bounded to that section so a mention elsewhere cannot satisfy
  it. The tempting option was refused: registering a no-network "check" so the
  category appeared in `lint.Checks()` would buy enumerability by putting a row in
  §12.1's table for something `lint` cannot run — a false statement in the document
  the other test exists to keep true. The registry is no longer the whole answer, and
  that is the part worth having written down.* Original: `spec_test.go` walks the check registry against §12's
  table in both directions, which is what keeps that table honest — and a category
  emitted by `internal/bundle` rather than by a registered check is invisible to it.
  Two ways out and the choice is not obvious: register a check that does no network
  and exists only to declare the category, or widen the test to a second source of
  categories and accept that the registry is no longer the whole answer. Filed rather
  than guessed, because the wrong one makes the table's guarantee weaker while looking
  like it strengthened it.
- [x] **The `stale` check's drift half has a home.** *`fetch --recheck`, as this entry
  guessed. §12's row and its design note now say so rather than saying "not
  implemented". The split is deliberate and stays: `lint` answers "is this old" with
  no network, `--recheck` answers "does upstream still say it", and one word covering
  both is the collapse §14.3.2 exists to prevent.*
- [x] **`standards.Unread`'s classification is checked against the source.** *The
  entry's own honest note was that the test made it survivable; that test compared
  `Unread()` to a literal list, which is a second copy of the same list, so the two
  agreed by construction and neither was evidence. Forgetting to classify a value was
  caught; misclassifying one was not.*
  *`standards.Declarations()` exports each tunable's Go field, and a root test scans
  the module for `.<Field>.Value` outside `internal/standards`. Both directions fail:
  classified consumed with no reader is the dangerous one — the state `staleness_days`
  sat in for two phases — and classified unread with a reader is the false alarm this
  entry was filed about. The selector rather than the bare field name is what makes it
  precise: `.Allowlist` and `.PerFileCap` are also `archive.Gates` fields, which the
  standards flow into, and matching the name alone would count the destination as a
  reader. Verified by misclassifying `in_degree_cut` and watching it fail.*
  Original: recording what
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
- [x] **§6.2's threshold accounting was five short and one category was wrong.**
  *Recounted in §6.2: twelve declared values across three files, nine of them
  comparable, in four categories rather than two. The classifier in
  `bundle.describeLoosening` is fixed and now cross-checked against
  `standards.Unread` by a test — the function that already knows.*
  **The recount found a live defect rather than a stale sentence.**
  `staleness_days` was classified as "nothing reads this threshold yet", which was true
  when written and stopped being true once the `stale` check gained its window. So
  **widening the staleness window silenced `stale` findings while `standards check
  --log` recorded that it cost nothing** — the precise reassurance §6.2 exists to
  withhold, produced by the mechanism §6.2 asked for. It survived because the
  classification lived in one switch and the truth lived in another, which is the
  general form worth remembering: two static lists about the same fact, and only one of
  them was maintained.
  A third category also had to be added — `per_file_cap` and `embedded_payload_cap`
  now reach a promote-gate verdict as well as tier-0 admission — and the four wordings
  are named constants in one place, so a fifth is a visible addition rather than a
  sentence written from memory. Original: `corpus_budget` and `corpus_warn_fraction` feed `doctor`'s budget
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
- [x] **A refused source reports every finding.** *`bundle.ScanFindings` renders the
  whole set and `fetch` carries it on the payload and in the human output, for the three
  reasons the scan produces — an extension refusal is not a scan finding and re-scanning
  it would say nothing.*
  Two decisions worth keeping. **No `--explain` flag**, which this entry suggested: the
  set is bounded by construction, one entry per character class and one per matching
  rule, so there is nothing long enough to gate and a knob nobody needs is what §6.5 is
  about. And it renders through `scan.Describe`, **the same renderer the candidate scan
  uses** — a second one would let `fetch` and the promote gate describe one problem two
  ways, so an author seeing a finding from each would have to work out they were the
  same. Original: `archive` reduces it to one
  `RejectReason`, which is right for a disposition and loses the detail: a source
  carrying three classes reports one. A `doctor` or `fetch --explain` view wants
  `scan.Hidden`'s findings with their offsets.
- [x] **`archive.Gates.ScanText` fails closed on nil.** *A nil now refuses with
  `ReasonUnscanned`; a caller that means not to scan says so with `archive.NoScan`,
  which is grep-able where a nil was invisible. Five `Gates` literals each had to
  declare which they meant, and that churn was the point.*
  **What made it decidable was the candidate path being built the other way.** A nil
  ruleset there degrades toward *more* blocking, reports the stages it could not run,
  and routes the document to a person. Two halves of one security stage failing in
  opposite directions is worse than either choice made twice — which is a sharper
  argument than the one this entry had, and it only became available once the second
  half existed. Original: Documented and tested as
  deliberate, because the alternative makes every caller carry a stub. It means the
  wiring is a property one test asserts rather than one the type guarantees, and if
  a second shell ever builds Gates that test is what stands between it and no scan.
- [x] **`rebuild_floor_fraction` moved to `standards/promote.toml`.**
- [ ] **No `--resume` and no crash-resumable queue.** §9.2 wants the ingest queue
  SQLite-backed so a killed process resumes rather than restarting. Prompts are
  currently emitted in one pass and a crash halfway through leaves some written and
  some not — recoverable by re-running, since emission is idempotent, but not what
  the spec describes. **The trigger, so this stops being re-litigable:** the first
  `ingest` over more than one source that somebody actually interrupts. Until then
  re-running is idempotent and cheaper than a queue, and §9.2's SQLite queue is state
  with its own crash story to write.
- [x] **The relay's round trip closes with `admit --stdin`, and this entry misread
  what was missing.** *`adh run --relay` does not emit and block on stdin in one
  invocation: it emits and stops, and a second invocation resumes from
  `--response <file>` where `-` is stdin. gnosis already had that shape in two
  commands, so the chaining was never absent — reading the reply from a pipe was.
  The misreading mattered: the other reading needs a wire format, because a caller
  cannot know a prompt has finished arriving on a pipe about to block. It is a flag
  rather than `-` because `ff` consumes a bare dash as the end-of-flags terminator.*
- [x] **Prompts are cleaned up when the reply is filed.** *`Writer.SpendPrompt`,
  called from `admit` after the document is quarantined. **This entry's own trigger
  was wrong**: it said "once the reply is cached", and caching happens before the
  reply is even parsed — so removing the metadata then would leave an agent told "the
  YAML is malformed, fix it" unable to submit a corrected reply under the same key.
  The removal order is the reverse of the write order, so a crash mid-removal leaves
  the same inert state a crash mid-write does. Best-effort: the reply is cached and
  the document is filed, so a failure to unlink is a note on stderr and not the
  operation's.*
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
- [x] **`hasOversizePayload` over-reports on prose about data URIs, and the decision
  is not to relax it.** *Recorded as SPEC §4.3.2. Both obvious relaxations are wrong:
  exempting a data URI in a fenced code block fails on the cap's own rationale, since
  this is a **weight** bound and nine kilobytes of base64 in a fence weighs nine
  kilobytes; a frontmatter escape is a bypass.*
  **What was actually wrong is that the refusal was unactionable**, and that is fixed.
  An author was told `embedded-payload` and not how large or against what, so the only
  move available was to argue the threshold down. `archive.Bound` now carries the
  measurement and renders it once for both the tier-0 refusal and the promote gate: "an
  embedded payload is 9,017 bytes against a declared cap of 8,192" is the same
  information turned into a truncated example instead of an argument.
  The entry said "revisit if it ever fires on a real source". It became more pressing
  than that when §9.3 stage 4 applied the same cap to a *candidate*, where a scan
  failure is `refused` with no §9.5.1 human path — so the reading was right and the
  trigger it named was the wrong one. Original: Documented
  as deliberate — the failure direction is a lost archive rather than a committed
  raster — but a document *about* data URIs is a plausible corpus member, and the
  refusal gives no way to say "this one is prose". Revisit if it ever fires on a
  real source.
- [x] **`evidence/text` orphans are checked, by the same mechanism as the entry
  below.** *See `archive-closure`. This entry and "Bundle closure is half-specified"
  described the same file in the same state from opposite directions — one reasoning
  from a crash between the content write and the record write, one from bundle
  closure — and one check closes both. This is where the cost is stated, which is the
  half the other entry did not have.* Original: `StoreEvidence`
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
  cost named. Revisit when `v6.0.0` ships — checked 2026-08-23: `v6.0.0-alpha.5` is
  still the newest published version, so the trigger has not fired.
  **This entry's "the exposure is one function and its tests" was wrong, and the
  measurement is now in §20.6.** Three production files import go-git — `git.go`
  clones, `gitfile.go` reads a file at a revision, `headtime.go` reads HEAD's commit
  time — across about a dozen API surfaces. Only `git.go` is in the evidence path;
  the other two read the *user's own* repository, so an alpha bump breaking them
  surfaces in `standards --since` and the trail-health check rather than in tier 0. A
  record's identity still comes from its own content, which bounds the evidence half.
- [x] **A git fetch reports its commit and still records none.** *§20.6. `fetch`
  prints the revision beside every candidate from the clone, marked as not recorded,
  and points at `log.md` for anyone who decides it matters for a claim. It travels on
  the candidate and never reaches the record; a test asserts the record's canonical
  bytes are identical with and without it, because putting it on the record is the
  obvious next commit and §4.3.1 is what it would break — a field varying with the
  repository's activity means one unrelated push re-records every file in the tree.*
  Still open, and smaller: a reader looking at an **existing** record months later
  still cannot learn its revision. The only home that would not violate §4.3.1 is
  `checked.jsonl`, which is per-user and already timestamped — worth doing when
  something reads it, and not before, since a field nobody reads is the other half of
  the same mistake.
- [x] **The git-adapter test fixture was not fragile under concurrent load — it was
  signing.** *Closed by the hermetic-fixture work above, and the diagnosis here was
  wrong. `fatal: failed to write commit object` is what git prints when signing fails;
  two simultaneous runs meant two requests to the GPG agent and a timeout, and the
  120-second figure was `go test`'s own timeout rather than a contention symptom.
  Three concurrent runs now pass in 3–4 seconds with the parallelism untouched.*
  Original: `originRepo`
  shells out to `git init/add/commit`, and two `go test ./...` runs at once produced
  `fatal: failed to write commit object` and a 120-second timeout. Passes reliably
  when run alone, including under `-race`. A test that fails under load is a test
  that will fail in CI on a busy runner, and the honest reading is that the fixture
  is doing real filesystem and subprocess work in a `t.Parallel()` test.
- [x] **The git adapter is exercised against a remote.** *`git daemon` on a
  kernel-assigned loopback port — a real capability advertisement and a real shallow
  single-branch negotiation, neither of which a local path performs — plus a bare
  listener that accepts and hangs up, asserting the failure is an *error* rather than
  an empty candidate set, because a source that silently archives nothing is the
  outcome worth a test. **Authentication is still untested and says so in the run**:
  it needs a credential store, and a fixture that fabricated one would assert that
  go-git reads what the fixture handed it. `TestAuthenticationIsNotTested` is a skip
  carrying that reasoning, so the gap cannot be deleted quietly.*

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
- [x] **`show --body` says when the index is behind.** *`index.Detail` carries the
  `content_hash` the table already stored, and `show` compares it against
  `identity.Hash` of the file it just read. Only with `--body`: without the text on
  screen there is nothing for the divergence to be about, and a note attached to
  nothing is noise. An absent hash — a document indexed by an older build — is not
  evidence of divergence and reports nothing, which is what keeps the note off every
  document in an old bundle.*
- [x] **Two rebuilds are tested to agree, over content rather than bytes.**
  *`index.DB.Digest` is a canonical dump of all seven content tables, each with an
  explicit `ORDER BY`, hashed. Tested at the package level and at the dispatcher, in
  both directions — the negative cases are what stop a digest that hashes a constant
  from passing. `DigestedTables` plus a walk over `Objects` makes a table added by a
  later migration and forgotten here a loud failure rather than a silent gap.*
  The entry guessed right that this compares a canonical dump. Worth stating why:
  a SQLite file is not byte-stable, so a byte comparison fails on a database that is
  correct — and a determinism test that fails on correct output is one somebody turns
  off, after which the property is unmeasured. The digest is reported in `index
  rebuild`'s envelope, which gives the export a production reader and makes §4.6's
  "two colleagues at one commit hold the same index" checkable instead of asserted.
  Original: SPEC
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
  **The structural half is already closed, and knowing that narrows this.**
  `ontology.indexSubjects` indexes each subject key together with its aliases, so a key
  colliding with another key's alias is rejected at load. What is open is only the
  population signal.
  **Not built, and the reason is §6.2 rather than effort.** Every signal named above
  needs a threshold — how bimodal, how many new aliases — and there is no corpus of
  claims to calibrate one against. A detector shipped now would carry invented numbers,
  which is exactly the value nobody can argue with later. §11.0.2 made the same mistake
  in the other direction once: an instrument was named that could not measure the thing
  claimed, and the correction was a data file authored from real disappointments rather
  than a cleverer metric.
  **The blocker is harder than calibration, measured 2026-08-23.** Claims carry no
  `subject` in frontmatter — `claimsOf` reads id, anchor, evidence and archive paths and
  nothing else — and **nothing writes the `claims` or `claim_subjects` tables**: a grep
  for `INSERT INTO claims` returns nothing, and only `digest.go` reads them, for the
  digest. The claim-level index is scaffolding for Phase 3. So the *report* the previous
  revision of this entry promised — claims per subject with their sources — has no join
  to make either. Two things must exist before any of this: a `subject` on a claim, and
  a writer for `claim_subjects`.
  **The trigger, so this stops being re-litigable:** the first subject key carrying
  claims from two documents whose evidence sets are disjoint. That is observable with no
  threshold, it is the shape the failure actually takes, and it is the population a
  detector would have to be calibrated against. Until one exists the instrument to build
  is the *report* — claims per subject with their sources — and not the detector.
- [ ] **Indicator words as an operator pattern.** *since, because, for, for the
  reason that, as indicated by* introduce a reason; *therefore, thus, so, hence, it
  follows that* introduce a conclusion. Lexical, closed, language-specific — held as
  data with a test corpus, never a regex in Go. Gives the `lead` check (§17.4) and
  segmentation something concrete without a model. Needs the test corpus before the
  code; the failure mode is a "because" inside a quotation.
  **Blocked: it has no consumer, and a previous priority listing of mine called it
  unblocked without checking.** `grep -rn '"lead"' internal/lint` returns nothing —
  §12.1 lists eleven enforced checks and `lead` is not among them — and
  `internal/segment` has its own `conjunctions()` list and splits on coordination, not
  on reason-giving. So the data file would ship with nothing reading it, which is the
  mistake this project has now recorded twice (a stored revision nobody read, a stored
  drift verdict nobody read).
  **What must exist first:** §17.4's `lead` check, or a segmentation rule that cuts at a
  reason rather than at a conjunction. Either one gives the data a reader; until then the
  test corpus would be authored against nothing.
- [x] **`gnosis audit --gained` reports what the corpus gained.** *`bundle.Gained` is a
  pure fold over the trail — the fourth in that family — counting documents promoted,
  replies admitted, sources archived, and drafts a reader declined.*
  **The entry's premise was wrong and the correction made it cheaper.** There is no
  `lint --since`, so this was never a second column on an existing comparison; the gains
  were already in the trail three reports read.
  **`ok`, always.** Exiting non-zero on good news would be the asymmetry this corrects,
  arriving through the exit code. **A window, and no rate**: a total since the beginning
  only grows and says nothing, and a rate invites a target which invites the padding this
  exists to stop rewarding. **A declined draft counts as a gain** — the corpus holds a
  judgement it did not hold before, and counting only additions would make
  deciding-against invisible. Original: Hamming's
  rating dynamic: "if everyone starts out at 95% there is little a person can do to
  raise their rating but much which will lower it; hence the obvious strategy is to
  play things safe." A corpus whose only visible signal is *problems found* rewards
  contributing less and claiming less. `lint --since` already reports what a change
  made worse; it should equally report claims admitted, evidence added, and
  conflicts closed. A second column, not a score.
  **The premise is wrong: there is no `lint --since`.** Measured 2026-08-23 — the only
  `--since` flags in the tool are on `standards check` and `log`. So this is not a
  second column on an existing comparison; it needs the baseline mechanism first, which
  is what `standards check --since REV` already does for thresholds and nothing does for
  findings. Hamming's argument stands and the cost is larger than the entry implies.
- [x] **`claim-anchor` is built, with the collision half.**
  *Two findings: `anchor-absent` — §12's own row for this check — and
  `anchor-collision`, two claims in one document with `Fold`-equal anchors. Building
  only the second would have left the check misnamed against the table §12.1 now makes
  self-checking.*
  Three decisions worth keeping. Comparison uses `textnorm.Fold` and **not**
  `Surface.Fold`, so case is preserved: an anchor locates a quotation, where case
  carries meaning, which is the opposite of the choice `duplication` makes for a title.
  Collisions are within a document only — two documents quoting one sentence is
  ordinary, and only the scope of the comparison tells them apart. And one finding per
  colliding group rather than per claim, so a report about three claims on one passage
  is about the passage.
  `anchor-absent` cannot yet tell a fabricated anchor from a drifted source, and the
  finding **says so** rather than implying it knows. That is TODO:596's two-signal
  cross and stays filed. Original: Claim ids are
  UUIDv7 so they never collide, but two users adding claims to one document can
  independently anchor different ids to the same text, and the merge is clean.
  Detectable as an `anchor_hash` collision within a document; a cheap addition to
  `claim-anchor` (§12). Phase 2, since nothing writes claims yet.
- [x] **Bundle closure is checked, in both directions.** *`archive-closure`:
  `archive-orphan` for archived text no record names, a **warning** because nothing is
  lost and the corpus is merely carrying weight it cannot account for;
  `archive-unrecorded` for a record naming an absent file, an **error** because the
  ledger claims evidence tier 0 does not hold and §9.4's invariant then has nothing to
  check a quotation against.*
  **This entry and "Nothing checks that `evidence/text` has no orphans" were one
  mechanism filed twice.** One argued from bundle closure — VAC fails a bundle holding
  a file its manifest does not list — and one from a crash leaving inert text; they are
  the same file in the same state. Worth recording as a shape: two entries reasoning
  from different failure stories to one predicate read as two items for months, and the
  thing that separated them was the story rather than the check.
  Distinct from `archive-path`, which reports a *claim* naming a missing file — that is
  a claim that cannot be verified, where this is the store and the ledger disagreeing
  about what tier 0 holds. Original: §12's `archive-orphan` reports an
  `evidence/` file no claim cites. VAC's `unlisted-file` also fails a bundle
  containing a file the manifest does not list, and `qvr sync` does the same from
  the other end — anything in an agent directory not in the lock is hidden from the
  agent. The missing half here: an archived file that **no `fetch.jsonl` row
  records** is unaccounted for regardless of whether anything cites it.
- [x] **`skillet/finding.Category` is an untyped string.** *Settled in skillet
  2026-08-22: it stays untyped, and the shared question was the wrong one.* Original:
  `Severity` and `Action`
  are typed while `Category` is a bare `string` with `omitempty`, so nothing stops
  two gnosis checks spelling one failure differently. VAC enumerates nineteen named
  reasons and never free prose; §8.0's `reason` vocabulary does this for the
  envelope but not for findings. Cross-repo — recorded against `skillet` as the
  shared question.
  **What the measurement found.** Across the family's thirty category values there is
  **not one same-word-different-meaning collision** — the risk this entry names has
  zero instances. The one real defect is its opposite: exegesis and canonizer spelled
  the *same* failure two ways (`skilllens-softening` versus `softening`) from the same
  `skilllens.SofteningPhrases` call. A closed enum was refused as a union of private
  vocabularies that would make every new check a kernel release; a registration seam was
  refused because it would not have caught that defect, both spellings being validly
  registered in their own repos.
  **The rule instead: where the kernel owns the detector, the kernel owns the name.**
  `skilllens` now exports its three category constants, unprefixed. Nothing changes for
  gnosis — its eighteen categories are its own domain vocabulary, which is exactly the
  case the rule leaves alone, and `reasonFor`'s switch on `identity` / `index-drift` /
  `conformance` is unaffected.
  **One thing worth taking from it here.** gnosis sets `Category` two ways — string
  literals and the derived `resolutionCategory(kind)` — and a grep for literals misses
  the two derived values. That is a live wrinkle for §12's check table and for anyone
  auditing the vocabulary: **it is not enumerable by inspection.** A test that walks the
  registry and asserts the emitted set matches what §12 documents would close it, and is
  the gnosis-side analogue of what skillet solved with constants.
- [x] **Both transcript adapters are read.** *`engineering-notebook` surveyed in
  manifesto.md ("Transcript Adapters"). What transfers is one file: a single parser
  for both formats that discriminates from the first record rather than from a
  caller-supplied flag, and an exclusion list — thinking blocks, `tool_use`,
  `tool_result`, compact summaries, Codex's `AGENTS.md` preamble — that is the actual
  interface to a session transcript. What does not: it skips malformed lines
  silently, which is right for a journal and is the failure this project keeps
  refusing for evidence; it truncates every message to its first line, useless as a
  source of quotations; and its summarizer calls a model from inside the tool. It and
  `skillopt_sleep` agree on the seam and disagree on the output — outcomes versus
  narrative — which is what §9.6 already decided.*
- [ ] **Surface definitions where terms are used.** `glossary-18F` is a small
  accessible panel resolving `data-term` attributes inline, as shipped on FEC.gov. A
  glossary nobody opens is not an ontology. Phase 5, with the viewer.
- [x] **Initial `standards/` values are written, and this entry was stale.**
  *Measured 2026-08-23: `standards/archive.toml`, `promote.toml`, `sample.toml` and
  `retrieval-cases.toml` all exist as embedded seeds, every value carrying the rationale
  §6.2 requires — the loader refuses a file without one. The entry was true when written
  and stopped being true as each file landed; nothing closed it because nothing was
  looking at it.*

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

- [x] **The retrieval-case instrument exists, and the competency-question entry merged
  into it.** *`standards/retrieval-cases.toml` plus `gnosis search --cases`: labelled
  queries, the titles that must come back, and cases whose correct answer is that the
  corpus holds nothing. No judge, no model, no threshold, and no pass rate — §17 forbids
  presenting a count as health, and a retrieval percentage is the most tempting such
  number there is, because it looks like progress and rises when a failing case is
  deleted.*
  **Titles rather than concept ids, and this entry's own proposal was wrong about
  that.** Identifiers are assigned per corpus, so a case file naming them is
  unportable — unreviewable by anyone reading it, unliftable to another bundle, and a
  failing case becomes archaeology. A title is what the person authoring the case was
  looking for.
  **Built now rather than with the reranker**, correcting PLAN's reason for waiting: the
  wait was about the *evidence for enabling a reranker*, and the instrument is
  threshold-free. A disappointing query is unrecoverable evidence — it happens, it is
  noticed, and with nowhere to write it down it is gone by the next day. The file ships
  empty and an empty suite reports that it examined nothing.
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
- [x] **Upstream drift resolves to three states.** *§14.3.2.1. `gnosis.Drift` is pure
  over (archived hash, upstream hash, upstream text, recorded quotations) and
  `fetch --recheck` supplies them. Four states, not three: the table's "bytes differ"
  was a precondition in prose, so the hash comparison moved inside and `drift-none` is
  its answer, which makes the function total. Two guards the table did not name —
  empty upstream text and a hash nobody computed are both `drift-unchecked`, because
  one 404 body would otherwise report `drift-unsupported` for every claim resting on
  the source.*
- [x] **`rationale` gained the fold-and-compare refusal.** *§10.6.4.
  `gnosis.UnusableRationale` is pure over (rationale, the phrases the tool showed the
  author, prior rationales) and `promote` supplies them. Applied to
  `command.Promote.Rationale` rather than to `gnosis_warrant`, which is Phase 3 and
  unimplemented: that is the field carrying reasoning in this binary, and Phase 3's
  warrant inherits the function. Four things building it settled.*
  **It folds case, unlike every quotation guard.** `textnorm` preserves case because
  "a quotation differing only in case is a different quotation" — right for evidence,
  wrong here, since capitalising one letter is the cheapest evasion there is. The
  first version was case-sensitive and its own test caught it.
  **Template text matches by containment, a prior rationale by equality.** The
  workaround for equality is adding a word, which §10.6.4 names in its own argument
  for quoting the match; but a rationale that quotes an earlier one and then says why
  this case differs is what should be encouraged, so containment there would punish
  the author who did the extra work.
  **Only successful promotions count as prior**, and the first draft had it backwards.
  Counting refusals made `promote` refuse its own second half: the confirmation flow
  previews, records the blocked outcome with the rationale, then applies. §10.6.4's
  "already recorded" means a warrant, which is a decision that landed.
  **The comparison is per path, not per subject**, because a promotion has no subject.
  Narrower than §10.6.4 and stated as such.
  Original: §10.6.4 specifies
  it: refuse a rationale that folds to the emitted prompt's own template text, and one
  byte-identical under `Fold` to a rationale already recorded for the same `subject`,
  naming the earlier warrant in the diagnostic. Not applied to `override.reason`. This
  is the observed failure mode of §10.6.4's central bet, not a hypothetical one — a
  surveyed system with the same required field had to warn its agents in prose that
  they were emitting the template verbatim.
- [x] **`gnosis audit --outstanding` exists.** *`bundle.Outstanding` is a pure fold
  over (trail, drafts) modelled on `Owed`, and `cmd/auditcmd` reports it as findings
  rather than as an error — the examination completed and it found something (§17).
  The definition is a subtraction and each term removes a different way of listing a
  decision already taken: asked, then no successful promotion, no discard, and the
  draft still in quarantine. Two of the first fixtures passed for the wrong reason —
  written with no draft present, so the absent draft alone cleared them and the
  promotion and discard terms were never exercised.*
  **A draft nobody ever ran `promote` against is deliberately not listed.** It is
  unexamined rather than abandoned, `quarantine` already lists it, and reporting a
  fresh corpus as a pile of neglected decisions on day one is the
  warning-true-of-everything §12 argues against.
  Original: enumerate required decisions that were never
  made — a promote that reached `needs_human` and was abandoned, a challenge opened
  and unresolved. The states are already committed frontmatter (§10.7.4); the report
  is missing, and absence is the one thing an append-only log of writes cannot show.
  Phase 3, with §10.
- [x] **Six sibling-repo items moved to their real homes.** *`skillsaw`'s five and
  `canonizer`'s and `adh`'s were transcribed here when the deep read produced them, and
  they belong in those repositories' backlogs. Recorded there on 2026-08-23; the status
  below is what those backlogs say, not what this one guessed.*
  **Five of the six were already done, and this file still had them open.** That is the
  finding, not the transfer: `skillsaw/TODO.md` had itself noted that gnosis was "the
  wrong home — a backlog for this tool belongs here", moved the items, closed
  `re-verify prior passes`, `the consecutive failure counter`, `count-shaped
  dimensions`, `loosens a gate`, and `the evaluator's noise floor` — and nothing
  updated the copies here. **A backlog that mirrors another repository's work goes
  stale in the direction that flatters**, because the mirror is only ever read by
  somebody deciding what is left to do.
  **Where each lives, and deliberately not what state it is in.** The first version of
  this entry listed each item's status in its home repository, which made it a mirror
  of state rather than of content — the same failure one level up, and it was wrong
  within hours: `adh`'s backlog was written by another process the same day. A pointer
  that records a status has to be maintained; one that records a location does not.
  - `skillsaw`: the `PASS → FAIL` status, re-verifying prior passes, the consecutive
    failure counter, count-shaped rubric dimensions, a gate loosening scoring as an
    improvement, and the evaluator's noise floor.
  - `canonizer`: `verify.Provenance`'s two-signal cross.
  - `canonizer` and `adh`: recording a critic that ran with reduced independence.
  - `adh`: guard invariants frozen in the hypothesis before the run.
  Read those files for state. The rule this file should have been following is the one
  it applies to knowledge: one home, and a pointer from everywhere else.
  What stays here is what gnosis owes. The two-signal cross is cited by §14.3.2 as the
  resolution for `anchor-absent` conflating a fabricated anchor with a drifted source —
  that is an obligation to *read* canonizer's answer when it lands, not to hold the
  item.
- [ ] **A corpus does not know whether an admitted claim was ever used.** `ruflo`'s
  nightly loop shipped 4 of 80 proposals over three months and could not see it,
  because each run checked only whether it was repeating itself. gnosis's promote gate
  decides admission and nothing asks about downstream reliance — the test the survey
  already adopted from `haft`, now with a measured cost for omitting it. Phase 4.
  **The blocker, measured 2026-08-23: the link graph does not hold the answer yet.**
  `links.source_claim_id` is nullable and filled only when extraction identifies the
  claim containing a link, which is Phase 3 — and nothing writes the `claims` table at
  all. Document-level reliance already exists and is called `orphan`; what is missing is
  claim-level, and it needs a claim-level link writer first.
- [x] **The scripted-model fixture is built; the real-model run is not.** *§18.6.1.
  "A local server speaking the model protocol" needed translating, because gnosis
  speaks no model protocol: it writes a prompt file and reads a reply file, so the
  seam is prompt file → agent → reply file and the local server is a function. The
  fixture's agent can see only the prompt, so it cannot quote text the prompt failed
  to carry — which is "assert on what the agent sent" in this architecture's shape.
  Adversarial in both directions: a prompt with its source cut or its fence emptied
  must leave nothing quotable, and a reply quoting absent text must be refused.*
  Still open: the real-model run graded by a pure predicate over the transcript,
  recorded as evidence and never as a gate — `superpowers` runs that shape under an
  isolated `HOME`, and its second assertion, that **nothing else happened before**
  the required step against an explicit allowlist, is one prose replies cannot be
  trusted to make about themselves. Whenever it is wanted.
- [x] **§18.0 maps what each suite adds.** *Nineteen test packages, one row each, named
  by the question the suite answers rather than by its path — because "what does the pure
  core guarantee" and "what does the binary do" are different questions and a reader
  choosing where to add a test has one of them in mind. Two rules travel with it: a test
  that would fit two rows belongs in the higher one, and a row with an empty third column
  would be a suite to delete.* Original:
  `superpowers`' `docs/testing.md` annotates every test with its coverage delta
  against the other harness — *"drill covers the YAGNI subset; bash adds commit-count,
  task-tracking, and token telemetry assertions"*, and *"tests description-recall, not
  behavior."* gnosis has property tests, mutation tests, corpus fixtures, adversarial
  fixtures, and soon a relay fixture, and nothing states what a second suite covers
  that the first does not. Cheap, and it is the artefact that makes a redundant-looking
  test defensible instead of deletable.

______________________________________________________________________

## Adjudication — This Repository Holds the Reference Model

`skillet` and `canonizer` each carry an entry saying an adjudicated artifact has no home
and fails a provenance check by construction, and both hold it pending "a second consumer".
There are three specifications of the idea, gnosis's is the fullest, and none of the three
had counted the others. Recorded here because the consequence is a constraint on this
repository rather than a task for it.

- [x] **§10's two provenance classes are the family's reference, and the count difference
  with `manifesto.md` is explained.** *The manifesto classifies three kinds of knowledge,
  §10 classifies two kinds of provenance, and genuinely-tacit is the adjudicated class with
  empty `sources`. Noted in the manifesto so a reader comparing them does not have to
  re-derive it.*
- [x] **§10.6.4 now says the warrant is unbuilt and load-bearing anyway.**
  *Stated at the top of the section, where a reader deciding to reshape it is
  standing, with the §14.1.1 parallel and the recorded non-task about canonizer's
  smaller warrant moved in beside it.* Original: `gnosis_warrant` is specified in §10.6.4 down to `co_signed_by`, `override`, and
  `reverses`, and a grep for it across `internal/` and `cmd/` returns **nothing** — it is
  Phase 3, correctly. The risk is not the gap, it is that `skillet` and `canonizer` both
  now point at this specification as the mature model, so the shape here is load-bearing
  outside this repository before any of it is built. Worth one line in §10.6.4 saying so,
  the way §14.1.1 records that `Report.Skipped` became load-bearing elsewhere.
- Not a task, recorded so it is not re-litigated: **canonizer will get a smaller warrant
  than this one, deliberately.** `skillet` will carry `{By, At, Rationale}` on
  `ruleset.Rule` and nothing more. Tiers, co-signers, and reversal links stay here, because
  they belong to §10.6's authority model — which canonizer explicitly bet against in its
  own §10.6.4-equivalent, holding that a required rationale filters more bad adjudications
  than a permission check. Two warrants with different obligations is the correct outcome,
  not drift, and gnosis's `Actor` being a closed three-kind enum for §10.6.4's counting is
  precisely why a shared warrant would be weaker than this one rather than stronger.

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
- [x] **`Report.Skipped` has the test that says so.**
  *A registry with one always-inapplicable check, asserting the skip is reported
  **with a non-empty reason**; the same property over the shipped registry on a corpus
  designed to make nearly everything decline; and that neither slice is ever nil. The
  failure it catches is `Applies` returning `(false, "")`, which compiles, runs, and
  silently suppresses a check — and which no other test in the package would notice.*
  Original: Today the guarantee lives in prose (*"every check that did not run appears in
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

- [x] **The OKF conformance table test is written, and the fold it needs with it.**
  *§18.5.1's six rows, each asserting what `ParseActor` does **and** what tier the fold
  yields. `gnosis.FoldTrust` is a pure function over `[]string` — the shape `skillet`
  will lift unchanged when a second repo classifies an actor — asking only §14.1.1's
  one question, is this `human:`-prefixed. `TierUnverified` is the zero value, so a
  value nobody populated cannot claim the strongest tier in the set.*
  The two disagreeing rows are the test, as §18.5.1 says. A second property test
  states the relation rather than the rows: every actor the parser accepts the fold
  also classifies, and the fold classifies some the parser refuses — if that ever
  inverts, one population has been narrowed to the other. Original: *Specified as
  §18.5.1.* Six rows over OKF §7's three forms plus `Actor`'s two additions,
  asserting for each what `ParseActor` does **and** what tier the fold yields. The
  two rows where those disagree — `process:finance-nightly` and
  `reference_agent/gemini-2.5-pro`, both rejected by the parser and both
  machine-confirmed for tier purposes — are the whole test; one that omits them
  passes under exactly the merge that breaks conformance. Cheap, and it belongs
  before the code it constrains rather than after.
- [x] **§14.1's fold reads raw strings, not `gnosis.Actor`.** *Built as
  `gnosis.FoldTrust([]string) Tier` — permissive read, `Actor` unwidened, an
  unrecognised actor never promoting a tier, exactly as this entry instructs. Its
  §18.5.1 table test is what required it, since asserting "what tier the fold yields"
  needs a fold.*
  **What remains is a different item from the one written here.** The fold exists and
  nothing calls it: deriving and reporting a document's tier is §14.1, which is Phase 3.
  That is the state §18.5.1 asked for rather than a gap — the instruction was to build
  the fold before its consumer so the consumer cannot be written the wrong way — and it
  is filed under §14 rather than reopened here.
  Original instruction, kept because it is the design and not a task: Two populations,
  two treatments: the closed enum
  stays for actors gnosis mints, because §10.6.4 counts distinct humans and a kind
  that could pass for a person makes that count wrong in the flattering direction;
  frontmatter that arrived from elsewhere gets a permissive read asking only *is this
  `human:`?*, which is the sole question OKF §7 says a trust classifier needs. An
  unrecognised actor is never an error and never promotes a tier. Do not widen
  `Actor` and do not narrow the fold to it — §11 forbids rejecting a conformant
  concept, and §10.6.4 forbids an open mint-side grammar.
- [x] **`gnosis` is the trigger the kernel is now watching, and gnosis has now
  tripped it.** *`gnosis.FoldTrust` is a pure function over `[]string`, which is the
  shape this entry asked for: it lifts unchanged if a second consumer appears. Nothing
  further is gnosis's to do — the promotion is `skillet`'s decision when a second repo
  classifies an actor, and this is now the first.* Original: `skillet` will promote
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
- [ ] **Nothing holds an *experience*.** *Shape recorded 2026-08-23: it is a `type` in
  the ontology with different accretion rules, not a field. A standard is superseded by
  a better standard; an episode is not superseded by a later episode — both happened —
  so what differs is how the two **age**, which is §10's supersession machinery rather
  than a frontmatter key. That makes it Phase 3, alongside the rest of §10, and it needs
  the type vocabulary to carry per-type accretion rules, which today it does not.*
  Original: a standard and an episode are different
  knowledge with different evidence: "React 19 prohibits X" versus "the team applied
  this here and approved it." The second is arguably the tribal knowledge this
  project is named for and there is no document type for it. It belongs in tier 2
  with a commit hash as evidence — **not** tier 3, where the summary put it, because
  a session trace cannot be re-derived and §4.5 forbids anything existing only in
  SQLite. Needs a type in §5.8 first, so Phase 3.
- [x] **A claim reports which sources support it.** *`bundle.ClaimFreshness.Sources`
  resolves each claim's `archive_paths` through the fetch records to the distinct source
  URIs, and `show` lists them beside the claim's freshness and drift. No new structure
  was needed — the join is the one `archiveIndex` already makes, and the data was on
  disk.*
  **Distinct by URI, and never a count.** Four archive paths may be four versions of
  one page, so a count of paths would report one source as four; and a count of sources
  would still be the inheritance §1.1's local reductionism refuses, where corroboration
  is a number to compare rather than a set to examine. A claim whose paths resolve to no
  record reports nothing rather than an empty list, because "cites a source tier 0 has
  no record of" is `lint`'s `archive-unrecorded` and showing it twice would be one
  defect reported as two.
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
- [x] **What an ingestion does not authorize is recorded.** *`audit.Row.Unsupported`
  carries the claims a reply asserted and the archived source did not support, with the
  archive path in `Paths` so the row says *which* source refused them.
  `gnosis audit --unsupported` reads it back. A refused reply used to be reported once
  and forgotten, so the same assertion could be offered again by the same model with
  nothing saying it had been tried.*
  Three decisions worth keeping. **Its own field, not `Findings`** — that one means the
  finding *ids* a write turned on, and a claim's text is not an id. **Only
  *unsupported* claims, never *unchecked* ones**, because "sought in the archive and not
  there" is a statement about the source and "nobody looked" is not; recording the
  second would assert that a source contradicts a passage too short to check. **The
  claim's text, not `describe`'s form** — "claim 2:" locates an entry in a reply that is
  on screen now and refers to nothing in a trail read next month, and truncation is
  wrong in a durable record for the same reason.
  Still open, and named rather than implied: the **committed** record of what a source
  does not support belongs with §10.7.4's challenge states, which are Phase 3. What
  exists is the per-user observation half.
- [x] **A corpus-level competency-question suite — merged into the retrieval cases
  above.** *They are one instrument at two grains: questions the corpus must answer,
  frozen as data, graded by a pure predicate. Two files to author cases in is one too
  many, and this entry supplied the design decision the other one had wrong — assert on
  titles, not on identifiers, because identifiers are assigned per corpus.*
- [x] **A known-answer soundness test per rule — recorded in `canonizer` and
  `skillsaw`, and *built here*.** *`scan.LoadRules` runs every rule's must-flag and
  must-not-flag case at load and refuses the whole ruleset if any fails, which is this
  item's shape one repository over. It caught a pattern whose own positive example did
  not match, on the first run.* The two siblings' copies live in their own backlogs. Original: The
  gate's planted-defect self-test generalised: every rule ships a case it must flag
  and a case it must not. Trust is more sensitive to false alarms than to misses, so
  soundness is the property to prove first.
- [x] **Name the object/metalanguage split in `skillet/ruleset/conflict`.**
  *Lives in `skillet/TODO.md`, which is where a change to that package gets made.* Original: It is a
  metalanguage check — rules about rules — and calling it that would keep a future
  contributor from adding object-level checks to it.
- [x] **`skillsaw` and `canonizer` cache keys should carry the rubric edition.**
  *Lives in both backlogs. The half that is gnosis's — that its own
  relay key does **not** need it — is already stated in §6.1 and in `relay/key.go`,
  and is the reason this was worth writing down at all: the same omission is correct
  here and a defect there.* Original:
  gnosis's relay key does not need it: a `standards/` change cannot stale a reply,
  because the rubric never enters the prompt. Where the rubric *is* what is being
  applied, a key without it serves yesterday's grade under today's rules.
- [x] **What a source costs to keep current is reported.** *`gnosis audit --churn`:
  per source, how many versions tier 0 holds and what each move cost — passages kept,
  passages lost, versions nobody has compared, and the one upstream still matches. It
  needed no new field, because a source fetched twice has two records (§4.1), so the
  record count per source already **was** the number of times the bytes moved; nothing
  had asked the question.*
  *A count and never a cost: §17 forbids presenting a count as health, and "this source
  moved six times" is the observation an estimate would rest on rather than the
  estimate. §14.4 is where it is weighed. The four outcomes are never summed, because
  six benign moves and one withdrawn passage are different events and no number of the
  first adds up to the second — and `drift-none` is counted apart from `unchecked`,
  which a `default` branch briefly collapsed while a comment claimed they meant the
  same thing.* Original: FPF's C.27 carries an `Effort`
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
- [x] **`skillsaw`'s ratchet may not re-verify prior passes.** *Checked in skillsaw
  and closed there: `Evaluate` iterates the whole dimension table with no skip path,
  and `TestEveryDimensionIsScoredEveryTime` now pins it. This copy stayed open here for
  a day after that.* Original: `oh-my-agent`'s judge
  re-checks every criterion each iteration "because fixing C2 is how C1 silently
  regresses." Needs checking in skillsaw; if the ratchet re-scores only failed
  dimensions it cannot see a regression its own fix caused. *Sharpened by the
  2026-08-22 protocol read: re-verification is the precondition and not the finding.
  A `PASS → FAIL` transition wants its own status, emitted once, routed differently
  from a first-time failure — and the failure counter must count consecutive failures
  and reset on success, or a flaky check accumulates to a permanent verdict on nothing
  but elapsed time. Both below.*
- [x] **§5.0.1 says what the corpus declines to hold.** *Four exclusions, each stated as
  the reason rather than as a topic list, because a topic list ages and a reason does
  not: what the code already states, what a single run produced, what belongs to one
  person's working memory, and what the corpus would have to re-derive to keep true.*
  *It states its own limit, which is the part that keeps it honest. Whether a claim
  restates the code is a judgement, §17 refuses to score, and §12.1's inversion already
  makes anything absent from its table convention by definition. What a scope rule buys
  is not enforcement — it is that a reviewer declining a page has something to point at,
  which is §6.2's argument for a threshold's rationale applied to editorial judgement.*
  Original: `gentle-wiki` scopes
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
- [x] **`gnosis schema` exists and carries the marker contract.** *§5.7.1.
  `internal/schema` is pure over (existing text, generated regions) and `cmd/schemacmd`
  does the I/O. The command did not exist — §5.7 specified it and `ls cmd/` had no
  `schemacmd` — so this was a build rather than an adoption.*
  **A fourth rule the entry does not state, and the code needed it:** a marker that
  opens and never closes is a refusal. Reading it as "everything to end of file" would
  let one typo hand a whole document to the generator.
  Three defects worth recording, all found by tests or by running it. **The merge added
  a blank line per run**, because a rendered region carried its own trailing newline and
  the replacement span already included the file's — so `--check` reported drift against
  a file it had just written. **A truncated marker was not refused**, because "no name"
  and "no problem" were both the empty string, so the refusal was skipped exactly where
  the file was most damaged; `Unclosed` returns a comma-ok pair now. **The command list
  came back empty**, because `c.Command` resolves to this command's own field and
  shadows the embedded root's — the same shadowing that made `admitcmd`'s flag
  `FromStdin`.
  Two refusals in `link` too: it will not replace a **regular file** somebody wrote,
  and it repoints an existing symlink, which `Lstat` rather than `Stat` is what makes
  possible — `Stat` follows the link and would report the second run's own work as a
  regular file.
- [x] **A fourth gate verdict is refused, because gnosis already has one.**
  *Checked rather than argued: a promotion carried over unrun signals **is**
  admitted-with-findings. `apply` records `Signals: carried` on the successful row, the
  envelope carries `"carried"`, and `gnosis debt` reports every document admitted that
  way — the candidate is adopted, the findings are retained, and they stay queryable
  when the subsystem behind a signal lands.*
  *What `haft`'s `review_ready` adds over that is the removal of the person. §9.5's
  human path requires an approver who is not an agent, a typed confirmation, and a
  rationale; a verdict that admitted with findings and no signature would be that path
  with the signature deleted, which is the `--yes` with extra steps §15 forbids. The
  entry was right that this is a genuine question — the answer is that the feature
  exists and the difference is the part not to adopt.*
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
- [x] **`AI_POLICY.md` exists in this repository.** *The three rules the entry named,
  and the third is `gnosis.Actor`'s human/agent/check split expressed in git: a person can
  be asked why, an agent can be re-run, a tool can be read, and collapsing them in the
  commit log while enforcing them in the corpus would be enforcing a rule the repository
  does not follow.*
  *It sets no percentage and no disclosure threshold, and says why: a threshold needs a
  rationale under §6.2's own discipline, and nobody knows what fraction of a change being
  model-written predicts a defect. Inventing a number is what §6.2 exists to prevent.*
  *The other repositories in the family are still without one; this closes gnosis's half.*
  Original: *listed in PLAN
  §4.1; it is a repository file rather than a spec change, so it stays open here.* Every one of them
  is built this way and none says so, while §1.1 argues that a claim must name its
  witness. `gentle-ai/AI_POLICY.md` is the model, and three of its rules are directly
  adoptable: review on observable quality rather than on whether output looks
  AI-generated; reject what the contributor cannot explain or defend; and no human
  attribution trailers for tools, with an optional `Assisted-by`. The third is
  `gnosis.Actor`'s human/agent/check split expressed in git.
- [x] **A scripted agent's reply is admitted, and a fabricated one is refused.**
  *The same gap as the §18.6 entry above, filed twice from two surveys; closed by
  `cmd/scriptedagent_test.go`. `gentle-ai`'s honest-limits note still bounds the
  claim: it does not prove a live model produces such a reply, which is what the
  unbuilt third method is for.*
- [x] **`bundle.AuditTrail` cannot distinguish corruption from operational failure.**
  `AuditTrail` and `LoadChecks` now report a malformed line as corruption *with its
  line number*, distinct from a read failure. The honest limit is recorded in §15 and
  in the code: `errs` has five codes and none means "the bytes on disk are wrong", so
  this is `EINVALID` with a message that says corruption — **legible rather than
  machine-checkable**. A sixth code belongs at the second consumer, not the first.
- [x] **A declined promotion is recorded as a decision, and the word covered three
  events.** *Separating them answered the §9.5 question this entry left open. **The
  gate refused** — mechanical and recomputable, so it stays in the per-user trail;
  committing it would put a derived fact in the authoritative tier, which §12 already
  refuses for the index. **A person was asked and walked away** — not a decision at
  all, and `audit --outstanding` is what surfaces it. **A person looked at the draft
  and dropped it** — not recomputable from anything, because the reason existed only
  in their head until they typed it. That one goes in `log.md`, following §6.2's
  precedent for a threshold change.*
  *`discard` was already the decline verb and already required a reason; what was
  missing is that the reason never reached the committed tier. **An agent's discard
  still does not**, deliberately: `Discard.By` may be an agent because dropping a
  draft grants no authority, and committing every agent's housekeeping would fill the
  corpus's history with the noise that teaches a reader to skip it.*
  Original: a declined promotion is logged as an observation.
  gnosis writes a refusal to `audit.jsonl`, which is per-user and gitignored.
  Gentleman records the decline itself as a canonical authorization, atomically. By
  §10.7.4's own rule a decision to decline is a decision, so it arguably belongs in
  the committed tier — which is a §9.5 question and not an implementation one.
- [x] **`revision_count` is not built, and "nearly free" was wrong twice.** *Measured
  rather than argued. `engram`'s method — increment on upsert — cannot work here at
  all: the index is **rebuilt** from the corpus (§12), so a counter would reset to one
  on every rebuild and report a churning document as new.*
  *Deriving it honestly means reading git history at rebuild time, and that breaks a
  property the index has. `Digest`'s contract is that "two colleagues at one commit
  hold indexes that answer the same questions"; history is not part of the corpus, so
  a shallow clone would produce different counts from a full one — and gnosis's own git
  adapter clones with `Depth: 1`, so a bundle fetched from a remote would report 1 for
  every document. The column would make the index depend on how somebody cloned.*
  *The question is worth answering and git already answers it:
  `git log --oneline -- <path>`. Wrapping one git command in a gnosis command is the
  knob §6.5 is about; if a report ever wants it, it is computed on demand and is not a
  column.* Original:
  `engram` increments a counter on upsert so an evolving record says how many times it
  has changed. gnosis keeps full history, which is strictly more information and
  strictly more expensive to query: "has this claim been churning?" currently requires
  walking git. A derived count in the index would answer it in a query.
- [x] **`skillsaw` has no cross-repository skill-identity check, and should.**
  *Lives in `skillsaw/TODO.md`, with the design question this entry did not have: the check is only as good as the naming convention it rests on, so a
  skill that should have been prefixed and was not reports as a portability violation
  rather than a naming one.* Original:
  `Gentleman-Skills` and `gentle-ai` declare a portability convention in prose —
  unprefixed names are portable and keep their canonical names, `<tool>-*` names are
  repo-specific — and it verifiably holds: `cognitive-doc-design` is byte-identical
  across two repositories and `gentle-ai-branch-pr` has legitimately diverged. It
  holds because one person maintains both. A check that same-named unprefixed skills
  hash alike across a set of repositories is the mechanism this family could supply
  back, and `identity.Hash` already does the hard part.
- [x] **`skillet`/`steve-skill-market` do not distinguish a shipped skill from a
  repo-governing one.** *Lives in `skillet/TODO.md`, with the cheap mechanism named: a declared kind on `manifest.Skill` that the rubric reads, which is
  the same shape as `Check.Applies` — state which convention applies rather than
  applying all of them.* Original: `gentle-ai` keeps `internal/assets/skills/` (embedded, ships
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

- [x] **Mark each specification rule by what enforces it.** `canopy`'s philosophy
  tags every principle `[code]`, `[convention]`, or `[code+convention]`. This spec
  is full of MUSTs and says nowhere which are machine-checked and which rest on
  people behaving — a reader cannot tell a guarantee from an intention. Two
  concrete cases: §4.1's append-only tier 0 is enforced and reads like a
  convention, while several rules phrased as MUST are conventions with no checker.
  Cheap to add as a column or a marker; the value is that it makes the unenforced
  ones visible enough to either enforce or downgrade.
  **DONE 2026-08-22 as decided: §12.1 is the short list, and it is self-checking.**
  *Twenty-three rows naming the rule, what enforces it, and what it emits. A test at
  the repository root parses the table out of `SPEC.md` and walks `lint.Checks()`:
  every check named exists, every check in the registry is named, and every declared
  category is documented. `lint.Check` gained a `Categories` field, which is what makes
  the third assertion possible at all — two categories come out of
  `resolutionCategory` and a grep for literals finds neither. All three failure
  directions were verified by breaking the table and watching each fire.*
  The reasoning below is the decision as recorded, kept because the sizing is what
  chose the shape.
  **DECIDED 2026-08-22: maintain the short list, not the long one, and make it
  self-checking in the direction that drifts.**
  Sized first, because the shape of the answer follows from it: the specification
  carries **51 MUSTs and 12 MUST NOTs** — 63 rules, no SHOULDs — against **11 named
  lint checks** plus a handful of gates. So `convention` is the overwhelming default
  and `code` is the exception, perhaps twenty rules at most.
  That rules out annotating every rule, which was the entry's first instinct. Sixty-three
  inline `[code]`/`[convention]` tags is the larger maintenance burden *and* the one that
  rots invisibly: a rule that gains a checker keeps reading `[convention]` and nothing
  notices. Inverting it — **a table in §12 listing only the enforced rules, with the check
  that enforces each** — is roughly twenty rows, and it puts the edit at the moment you are
  already editing both, which is when a checker is added or removed. Anything absent from
  the table is convention by definition, so the unenforced set stays countable without
  being enumerated.
  **The part that makes it more than a second place to drift:** a test that walks
  `lint.Checks()` and asserts the table names only checks that exist, and that every check
  appears. The first direction catches the failure that matters — the table claiming
  enforcement that was deleted — and the second catches a checker nobody documented. That
  buys the non-drift property of a generated table at the cost of a hand-maintained one,
  without needing stable identifiers for all 63 rules.
  **Fold in the adjacent wrinkle rather than writing a second walk.** §12's categories are
  set two ways — string literals and the derived `resolutionCategory(kind)` — so the emitted
  vocabulary is not enumerable by grep, which is recorded against the settled
  `finding.Category` entry above. The same registry walk can assert the emitted category set
  matches what §12 documents. One test, two properties, and both are currently invisible to
  inspection.

- [x] **A missing `AGENTS.md` is both reported and prevented, and my stated decision was
  wrong.** *I decided `doctor` should report it and `init` should not scaffold it,
  arguing that a scaffolded copy would be stale from the first vocabulary edit. A test
  found the flaw: `TestInitialisedBundleIsHealthy` asserts a fresh bundle produces no
  findings, and my change made every new bundle unhealthy on creation.*
  *The resolution dissolves the objection rather than splitting the difference: `init`
  **generates** the document — it is not a scaffolded copy, it is what `gnosis schema`
  would write that second — exactly as `init` already opens the index rather than
  shipping a database. And `doctor` keeps the check, for the case it is actually for: a
  bundle cloned from before the command existed, or a file somebody deleted.*
  Original: noticed building `gnosis schema`:
  §5.7 says gnosis "generates and maintains" the schema document, `init` does not
  scaffold one, and `doctor` — which reports every other absent apparatus file — says
  nothing about it. So a corpus can run for months with no schema document and no
  signal. The fix is one `diagnoseBundleFiles` entry, and the reason it is filed rather
  than done is that `init` scaffolding it is the other candidate and the two are
  alternatives: a scaffolded file would make the check dead, and a check makes the
  scaffold unnecessary. Deciding that is a §5.7 question about whether a corpus is
  expected to have one from day one.

- [x] **§12.1's table carries a Fixable column.** *`lint.Check` gained `Actions`,
  declared exactly as `Categories` is and for the identical reason — an action is a field
  set inside a `Run` body, so it is not enumerable by inspection. Asserted in both
  directions: an action emitted and not declared fails, and a table cell naming one the
  check does not declare fails. Verified by mis-stating one cell and watching both halves
  fail.*
  **The column is last, and that is not cosmetic.** `spec_test.go` reads the enforcer by
  column position, so a column inserted earlier would change what that test walks while
  still passing.
  **A declared action is not a promise of a fixer.** There is no `--fix`; the column says
  what a fixer *could* do. Building one is a much larger decision, and shipping it inside
  a documentation column would be shipping it by accident.
  Original: `kb-lint`'s does.
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
  currently resolve it by gating the whole rewrite.
  **The blocker is gone: the marker contract is the way to mark regions** (§5.7.1), and
  `AGENTS.md` is the first document using it — a `schema` run rewrites what the machine
  wrote and preserves a person's prose byte for byte.
  **Concept documents do not adopt it yet, and that is a separate decision rather than
  the rest of this one.** A marker inside a concept document interacts with anchors
  (§5.5.1), with quotation validation, and with the index, so it is a §5.5 change
  wearing a §6.3 hat. **The trigger:** the first agent refresh of a document a person
  has edited — which needs the refresh path to exist, and that is Phase 3.

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

- [x] **`okf` (skosovsky) evaluated and declined, with the measurement.** *Fetched
  `v0.2.1` and read its `bundle` package against `internal/okf`. Three reasons, and the
  second is the one that settles it.*
  **It implements OKF v0.1; this bundle conforms to v0.2** (§5.3). Its own package
  comment says so — "the Open Knowledge Format (OKF) v0.1 data model" — so adopting it
  would mean conforming to the version gnosis deliberately moved past.
  **It preserves CRLF and gnosis normalises it, deliberately.** Its
  `TestParseDocument_PreservesCRLFBody` against this repository's
  `TestCRLFIsNormalisedNotPreserved`. That is not a preference: a quotation is validated
  by comparing passages against archived text, so two documents differing only in line
  endings would fail the check the whole corpus rests on. Swapping the parser would
  change what counts as the same words.
  **It does its own I/O and gnosis's parser is pure.** `LoadBundle(root string)` reads a
  directory; `okf.Parse(src []byte)` takes bytes and `bundle.Load(fsys)` does the
  reading. That split is the one §4.6 and the functional-core rule require, and a
  library that loads a bundle, resolves links, computes backlinks and offers a mutation
  store would put a second implementation of four things gnosis already layers.
  *No dependency was added. The evaluation is recorded so the next reader does not
  re-derive it.* Original: a Go OKF toolkit with
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

- [x] **The two manifestos no longer diverge: the older one is a pointer.**
  *`~/Documents/agent-red/manifesto.md` is now 31 lines naming this file as
  authoritative, with a table of which sections were superseded by what — because two
  were *rewritten* rather than moved (`Three tiers` → `Four Tiers`,
  `Two provenance classes` → `Three Kinds of Knowledge, Not Two`) and a bare redirect
  would leave somebody who remembers the old text unable to find the new.*
  **Nothing was deleted.** The previous 1,059 lines are beside it in
  `manifesto.superseded.md`, because that directory is **not under version control** —
  checked before touching it, and it is what changed the method: a replacement there
  would have been unrecoverable.
- [x] **The markdown formatting is done, and `mdformat` is refused with a reason.**
  *`rumdl fmt` run over the five documents this repository **authors** — `SPEC.md`,
  `PLAN.md`, `TODO.md`, `manifesto.md`, `llm_wiki_pattern.md`. 192 issues fixed, and
  `rumdl check` now reports none across all five. The suite passes afterwards, which
  mattered: `spec_test.go` parses §12.1's table by column position and a reflow is
  exactly what could have broken it.*
  **The imported reference texts were deliberately left alone.** `as_we_may_think`, the
  Luhmann translation, the zettelkasten documents, `skill_to_code`,
  `dissolving-toil`, `taxonomies-ontologies`: 465 of the repository's issues are in
  files somebody else wrote. Running a formatter over an imported source changes text
  this repository is holding rather than authoring, which is the marker contract's
  argument one level up.
  **`mdformat --check` still fails and will keep failing, because the two tools
  disagree in a way that is not a preference.** Measured on a copy: `mdformat` unwraps
  every paragraph — undoing the wrapping a 5,000-line specification is readable at — and
  escapes `**` as `\*\*` where a paragraph's continuation line begins with it — two
  places in `TODO.md`, none in `SPEC.md` — which **changes what renders**, turning bold
  into literal asterisks. The unwrapping alone is disqualifying; the escaping is the
  part that would be a silent content change, and it is rare rather than pervasive. So this repository formats with `rumdl` and the invocation is
  `rumdl fmt --disable MD063 <authored files>`; `mdformat` is not part of the toolchain
  and the earlier entries naming it were describing an intention rather than a decision.
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
  **Decided 2026-08-23 on the warrant itself, since this entry waits on it: build it whole,
  under that name, or not at all.** §10.6.4 is specified to the field and already carries the
  two admission rules; the one open question was whether to ship a subset first.
  **A partial `gnosis_warrant` is refused, and the spec's own warning is why.** It records
  that *"the shape below is load-bearing outside this repository before any of it is built"*
  — `skillet` and `canonizer` both cite the section — so shipping `{by, at, rationale}` under
  that name is precisely the *"reshaping done for local convenience"* it warns against: two
  repositories would read `gnosis_warrant` and find fewer fields than the section promises.
  If a subset is ever wanted, it must carry a different name and pay the migration.
  **Tiers, co-signing and `override` are one mechanism and ship together.** The section is
  explicit: *"a gate with no override is a gate people route around; a gate whose overrides
  are countable is a gate."* Half of that is not a gate.
  **The admission rules were the part worth doing early, and they are done** — see the
  fold-and-compare refusal above, already shipped on `command.Promote.Rationale`, which
  Phase 3's warrant inherits. So the remaining work is the artifact, not its enforcement.
  **Decided 2026-08-23: it waits for `gnosis_warrant`, and the reason is not effort.**
  The role belongs on the warrant, as an attribute of the decision. Putting it anywhere
  reachable today — a flag on `promote`, a field on the audit row — creates a second
  home that has to be migrated when the warrant lands, and §10.6.4 already records that
  the warrant's shape is load-bearing outside this repository. A field invented in the
  wrong place first is the cost that section warns about, arriving through convenience.
