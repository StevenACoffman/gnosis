# Manifesto

## Why?

We work in a educational technology software engineering product team that is comprised of people with a diverse set of both technical and non-technical skills and knowledge, including software engineers, quality assurance engineers, project managers, product owners, and customer support staff.

All of us are using various agents to work towards our individual and collective goals, but we wish to use these agents as a collective endeavor, rather than have us each do so in isolation.

We wish to pool all of our relevant tribal knowledge and skills to our mutual benefit.

In our team, we also wanted to ensure that every corrective interaction we made to an agent was accretive rather than just arguing with a random number generator. Since we could only control two external levers (context and tools) we curated our agentic environment around those.

We wanted to more fully articulate our nonfunctional requirements: the quality attributes and constraints governing reliability, security, compatibility, maintainability, performance, operability, risk posture, and polish. We also wanted to maintain our local decisions about how to prioritize, trade off, and satisfy those requirements.

We started with a curated knowledge base of widely esteemed general software engineering, architecture, and collaboration texts. We distilled this into a set of internally consistent base rules, while maintaining provenance.

We used a few textual analysis techniques to make an alternative distillation of the knowledge base into focused (and also internally consistent) skills, while ensuring that there would be no conflicts with the base rules.

We improved those skills using Microsoft research’s SkillOpt/SkillLens, while also offloading anything mechanistic in them into CLI tools, leaving only judgement to an LLM via templated prompts.

These reified into a harness that is comprised of the artifacts of our knowledge base (source of truth), skills, CLIs. We used skills / CLIs to make the harness improve itself according to the same non-functional requirements.

The harness is currently called by Claude/Gemini/Qwen, but has a stub direct model interface so it could potentially dispense with those.

We wish to ensure that both our curated knowledge base and our harness continues to collectively be maintained and improve in as deterministic of a manner as possible.

Both humans and LLMs are prone to errors in judgement, but these can be mitigated through deterministic processes.

For humans, critical thinking skills applied with a close examination of evidence and reasoning with an unbiased peer review is the gold standard for mitigation of errors in human judgement.

For mitigating errors in judgement for LLMs combined with Agents, the ideal process is still being established.

We are in the process of surveying different projects, collecting and combining their processes or techniques into different tools to facilitate our efforts.

This document contains a record of individual projects we have gleaned useful techniques or processes from, and the tools we have incorporated them into.

## Tools We Have Created, and How We Use Them

The family is one shared kernel, four CLIs that consume it, and three agent skills
that drive those CLIs.

```text
book2skill ──▶ exegesis ──▶ skillsaw ──▶ merge-skills
 produce       gate           score        consolidate
                     │
   adh ─────────────┴───────────── canonizer
   (fellow kernel consumers, alongside)
```

### Shared Kernel

#### `skillet` — `v0.15.0`

- **Repo:** [github.com/StevenACoffman/skillet](https://github.com/StevenACoffman/skillet)
- **Local:** `~/Documents/git/skillet/`

The Go library every other tool imports, so two tools cannot disagree about one
definition. Holds `speclint` (agentskills.io frontmatter), `redlines` (book2skill's
mechanical Quality Red Lines), `skilllens` (the three SkillLens detectors),
`markdown`, `judge`, `testprompts`, `ratchet`, and `timeseries`, plus `finding`,
`ruleset`, `proof`, `provenance`, `identity`, `neutrality`, `stats`, and
`calibration`. **Promotion rule: a package moves here on its second consumer.**

### The Four CLIs

#### `exegesis` — Structural Gate

- **Repo:** [github.com/StevenACoffman/exegesis](https://github.com/StevenACoffman/exegesis)
- **Local:** `~/Documents/agent-orange/exegesis/`

Gates a skill tree on the way in: `lint`, `verify`, `tests`, `scaffold`, `link`,
`index`, `merge-index`. Owns the related-skill edge graph and `INDEX.md`. Proves
a tree is well-formed; says nothing about quality. Also owns `quotecheck`, the
fabrication guard that reports quotations appearing in none of the source texts.

#### `skillsaw` — Quality Score and Ratchet

- **Repo:** [github.com/StevenACoffman/skillsaw](https://github.com/StevenACoffman/skillsaw)
- **Local:** `~/Documents/agent-orange/skillsaw/`

Scores skill quality on the 9-dimension rubric and runs the keep-or-revert ratchet:
`eval`, `diagnose`, `judge`, `gate`, `preflight`, `activation`, `scores`,
`calibrate`. **Deterministic only — never calls a model.** The fusion of
SkillLens + SkillOpt exists here and nowhere else.

#### `agentic-dev-harness` (`adh`) — Five-Stage Arc Loop

- **Repo:** [github.com/StevenACoffman/agentic-dev-harness](https://github.com/StevenACoffman/agentic-dev-harness)
- **Local:** `~/Documents/git/agentic-dev-harness/`

A different consumer of the same kernel: drives a change through a five-stage arc
loop (strategy → execution → critic → evaluation → ops) with its own 5-dimension
rubric. Being the second consumer of `skilllens` and `markdown` is what justified
promoting them.

#### `canonizer` — Ruleset Grader

- **Repo:** [github.com/StevenACoffman/canonizer](https://github.com/StevenACoffman/canonizer)
- **Local:** `~/Documents/git/canonizer/`

Grades rulesets rather than skills: `verify.Executable`, `verify.Provenance`,
`verify.Specificity`, plus a cold-critic prompt. **Findings-based by design — never
a weighted score that could become a ship threshold.** 0 open.

### The Agent Skills — `steve-skill-market`

All three live in [github.com/StevenACoffman/steve-skill-market](https://github.com/StevenACoffman/steve-skill-market), under
`~/Documents/agent-orange/steve-skill-market/skills/`.

| Skill             | Role         | What it does                                                                                 | Open |
| ----------------- | ------------ | -------------------------------------------------------------------------------------------- | ---- |
| `book2skill/`     | producer     | Distills a book into RIA-TV++ skills (R, I, A1, A2, E, B) via exegesis.                      | 0    |
| `skillsaw-skill/` | optimizer    | Drives the skillsaw CLI through a hill-climbing loop: baseline → diagnose → one edit → gate. | —    |
| `merge-skills/`   | consolidator | Detects convergence across book2skill outputs and builds one merged skill.                   | 0    |

**Pipeline:** book2skill produces → exegesis gates structure → skillsaw scores
quality → merge-skills consolidates. `adh` and `canonizer` sit alongside as fellow
kernel consumers.

## Tools We Were Inspired By, and What We Learned from Them

Each entry below is a project we read closely. The bullets are the specific
technique we want, and *Feeds* names the tool of ours it belongs in. Nothing
here is an endorsement of the whole project — only of the part we are stealing.

### Harness Quality: Scoring, Gating, and Drift

**AgentLint** — `github.com/0xmariowu/AgentLint` (`AgentLint/`)

- 51 deterministic checks + 7 opt-in AI checks, split so the model is only
  consulted where determinism runs out — the same "mechanism in the CLI,
  judgement in the prompt" split we chose.
- `standards/weights.json`, `standards/reference-thresholds.json`,
  `standards/evidence.json` externalize the rubric: weights, thresholds, and the
  citation backing each one are data, not code. Rubric changes become reviewable
  diffs with provenance.
- Findings carry a fix class (`guided` / `assisted`) so the report says who acts,
  not just what is wrong.
- Its README documents evidence *against* the practice (auto-generated context
  files reduced success in 5 of 8 settings). A tool that publishes its own
  counter-evidence is the honesty standard we want.
- *Feeds:* skillsaw (rubric-as-data, evidence provenance), canonizer (fix class
  on findings).

**coherence** — `github.com/fireharp/coherence` (`coherence/`)

- Names the failure our NFRs care about: tests pass, the repo still drifts.
  Checks whether code, docs, ADRs, tests, metrics, generated files, and
  endpoints still support each other after an agent edit.
- `ontology.yml` declares the repo's own artifact graph, so the check is
  local-decision-driven rather than universal.
- Emits `safe_to_commit` / `review_recommended` as *separate* verdicts plus a
  `recommended_next_command` — machine-actionable without collapsing to one
  pass/fail number.
- Drift *regressions* (baseline vs. now) rather than absolute drift: the ratchet
  pattern, applied to repo consistency.
- *Feeds:* a new drift gate alongside exegesis; skillsaw's ratchet
  (regression-relative gating); adh's ops stage.

**stringer** — `github.com/davetashner/stringer` (`stringer/`)

- Fifteen deterministic collectors (TODOs, CVEs via OSV, dependency health,
  lottery risk, churn, coverage gaps, coupling, doc staleness, config drift, API
  contract drift) that surface debt already latent in the repo, so agents stop
  burning tokens rediscovering it.
- Confidence-scored signals with age-based boosts — a finding carries how much
  to trust it.
- One scan, many renderings (markdown / JSON / agent tasks / backlog JSONL):
  the consumer picks the shape, the analysis is computed once.
- Ships `eval/` and `GOVERNANCE.md` — the tool is itself evaluated and governed.
- *Feeds:* our NFR articulation (its collectors are a concrete checklist of
  maintainability and security attributes); canonizer's findings model.

**4x** — `github.com/ggwhite/4x` (`4x/`)

- Role isolation as the anti-self-review mechanism: Design → Code → Review →
  Test → Deep Review → Accept, where the Reviewer is adversarial by construction
  and never sees the Coder's reasoning. This is the closest thing we have found
  to "unbiased peer review" for LLMs.
- Guardrails (state machine, scope lock, baseline snapshot, evidence gate) are
  enforced in Go, not by asking the model to please stay in scope.
- A file protocol (`.4x/`) rather than an SDK, so roles can be assigned to
  different models — exactly the property our stub direct-model interface wants.
- `4x evolve`: self-improvement behind a **value gate with anti-hack** and a
  **self-modification scope guard**. Self-improving harnesses need a gate the
  harness cannot rewrite.
- Publishes an explicit Trade-offs section (3–10x token cost, wrong for
  one-line fixes) and a `THREAT_MODEL.md`.
- *Feeds:* adh's five-stage arc (role isolation, adversarial critic); the
  anti-hack value gate for any self-improvement loop we build.

**agentsys** — `github.com/agent-sh/agentsys` (`agentsys/`)

- States the doctrine plainly: *code does code work, AI does AI work* —
  regex/AST/static analysis for detection, LLM only for synthesis and judgement.
  Measured 77% token reduction on `/drift-detect` versus multi-agent prompting.
- **Certainty levels on every finding** (HIGH = safe to auto-fix, MEDIUM = needs
  context, LOW = needs a human), derived from testing across 1,000+ repos. This
  is the missing dimension in a bare findings list.
- Benchmark showing Sonnet + harness ≈ Opus + harness at 40% lower cost: the
  empirical argument that harness quality substitutes for model tier.
- Phase gates agents cannot skip; state persists across session boundaries.
- *Feeds:* certainty grading in canonizer and skillsaw output; the
  harness-vs-model-tier argument in our own benchmarking.

### Skills and Context as a Governed Supply Chain

**qvr (quiver)** — `github.com/astra-sh/qvr` (`qvr/`)

- Treats a skill as a **dependency** with a full lifecycle: resolve → lock →
  install (immutable, SHA-keyed) → scan → symlink → reproduce → observe.
- `qvr.toml` (declared intent) separated from `qvr.lock` (resolved proof,
  including subtree hash, scan verdict, and commit author). The lock *is* the
  audit trail — provenance we currently maintain by convention.
- A 15-category scan taxonomy (prompt injection, exfiltration, secrets, MCP
  tool poisoning, invisible Unicode, OSV) gating every install, exported as
  SARIF.
- `qvr sync` hides anything not in the lock from the agent: the lockfile is the
  only source of truth for what loads. Deterministic context, enforced.
- Every command has `--output json`, data on stdout, diagnostics on stderr,
  meaningful exit codes — the agent-callable CLI contract.
- Its own skills (`optimize-skill-loop`, `create-skill-eval`,
  `trace-skill-activity`, `verify-skill-supply-chain`) mirror our
  skillsaw-skill loop.
- *Feeds:* a lock/provenance layer for our skill tree (exegesis); supply-chain
  scanning before any third-party skill enters the harness.

**mdm** — `github.com/sethcarney/mdm` (`mdm/`)

- `mdm rules link` makes `AGENTS.md` canonical and symlinks all 45 agents'
  expected filenames to it — one source of truth for instructions, zero drift
  between Claude/Gemini/Qwen surfaces.
- `skills-lock.json` committed to the repo so a new teammate onboards with one
  command, whatever agent they prefer. Directly serves "pool our tribal
  knowledge" for a mixed-skill team.
- Deterministic local scan for hidden characters and prompt-smuggling on every
  install; `skills audit` for updates and OSV advisories.
- Distributed as a Dev Container Feature — the harness travels with the
  environment.
- *Feeds:* single-source instruction files across our three model frontends;
  lockfile-based team onboarding.

**ailloy** — `github.com/nimble-giant/ailloy` (`ailloy/`)

- Helm's model applied to AI instructions: templated *blanks* + *flux* values
  with layered precedence, so the same mold renders differently per project and
  reproducibly. This is how a shared kernel of rules survives local
  prioritization decisions without forking.
- Clean separation of render (`forge`), install (`cast`), package (`smelt`),
  validate (`temper`), lint (`assay`) — dry-run before write is a first-class
  verb.
- Molds resolve straight from git with semver ranges; no registry service.
- *Feeds:* parameterized distribution of our base rules to teams with different
  trade-off profiles, without losing a common upstream.

**skillex** — `github.com/atheory-ai/skillex` (`skillex/`)

- Reframes skill selection as a **retrieval** problem, not an authoring problem:
  move scope resolution out of prompt assembly and into deterministic indexing +
  structured query.
- Version-correct and scope-aware — a query for `packages/app-a/**` can never
  return app-b's skills, and the right skill for `@acme/foo@2` is never the v1
  one.
- Public vs. private skill visibility (consumer-facing vs. contributor-facing),
  enforced automatically.
- Detector-driven activation via `pack.yaml`, with a small built-in detector set
  and the rest community-defined — the extensibility shape we want for our
  activation rules.
- The index is a deterministic build artifact: same repo state, same index.
- *Feeds:* exegesis's index/link commands (deterministic index as artifact);
  scoped activation for skillsaw's activation scoring.

**katalyst** — `github.com/abegong/katalyst` (`katalyst/`)

- A *content consistency layer* for knowledge bases curated by sloppy humans
  and sloppy agents alike — the exact governance problem our knowledge base has.
- Declarative rules over markdown structure, filenames, directory shape, and
  full JSON Schema validation of frontmatter — plus explicit migration verbs for
  when the rules change (`migrate-schema`, `migrate-storage`).
- `katalyst inspect` profiles an existing corpus and reports its de facto
  conventions **as evidence**, so rules are derived from what is there rather
  than imposed.
- Design principles that match ours almost word for word: fast enough to run on
  every write, discoverable by the agent unaided, readable by both audiences,
  extensible.
- *Feeds:* schema validation and migration for our knowledge-base frontmatter;
  `inspect`-style convention discovery before we author new rules.

### Knowledge Base, Provenance, and Grounding

**llmwiki** — `github.com/mritunjaysharma394/llmwiki` (`llmwiki/`) — **fork target**

Read the schema, not just the README — it corrects the first impression.

- The trust property is real and worth taking whole: **every page carries an
  evidence quote that is a byte-exact substring of its source**, and the
  validator runs after every LLM call on every code path (`ingest`, `promote`,
  `mcp.write_page`). A proposed body whose quotes fail substring-match is
  dropped and the page stays at its previous version — never silently
  downgraded. A cheaper model yields a *sparser* wiki, never a *wronger* one.
- **But it is a synthesizer, not an archive.** `sources` is
  `(uri UNIQUE, content_hash, ingested_at)`; `source_files` is
  `(relative_path, content_hash, byte_size, line_count)`; `chunks` is
  `(chunk_hash, file_paths)` — hashes and sizes, never the bytes
  (`internal/db/db.go:34`, `:120`, `:149`). Meanwhile `pages.body` holds the
  synthesized text. It persists the derivative and merely fingerprints the
  original, and substring validation runs against the *live* source. A moved
  PDF or a 404'd URL leaves the quote on disk and the proof gone. The trust
  property is genuine and **not durable**.
- `uri` is UNIQUE, so one URI is one row — there is no version history of a
  source. An archival tier needs `(uri, content_hash)` as the key, append-only.
  That is a schema change, not a config change.
- The reconciliation machinery we assumed we would have to build already exists:
  cross-page updates fold a new source's claims into every page they "refine,
  qualify, contradict, or extend," plus explicit contradiction calls and
  retro-linking. Every candidate considered appends to `page_update_log` with an
  outcome and a reason — an audit trail over the synthesis itself.
- Its gates are already model-free: the substring validator, and the four-signal
  auto-promote heuristic (cited pages, evidence quotes, length, no hedging, no
  near-duplicate). Neither calls a model.
- Cost and nondeterminism are concentrated in one place. A 50-page ingest runs
  roughly 5–10 ingest calls **+ up to 50 cross-page update calls** + up to 5
  contradiction calls, bounded by `update_existing_max_candidates_per_source`
  (20), `update_existing_max_candidates_total` (50), and
  `update_existing_quote_floor` (2). Candidate *selection* is the largest
  LLM-driven step and it is already parameterized — which is what makes it
  replaceable.
- Crash-resumable SQLite ingest queue; Obsidian-native plain markdown out; no
  proprietary store. Apache-2.0 and Go, so we can fork it on our terms.
- **We do not need its validator — we already have a better one.**
  `exegesis/internal/quotecheck` is the same fabrication guard ("does this run of
  words appear in the source at all"), and it compares through
  `exegesis/internal/textnorm.Fold`, which folds whitespace runs and typographic
  variants first. llmwiki's match is byte-exact against the *live* source, so a
  curly apostrophe or a rewrapped line fails it — as that package's own comment
  puts it, a guard that fired on every curly apostrophe would not get run.
  `quotecheck` also drops passages under `MinPassageWords`, and shares `textnorm`
  with `a2check` so two guards cannot disagree.
- *Feeds:* the ingest-and-grounding tier of the knowledge base. What we actually
  want from llmwiki is its **ingest adapters** (URL, PDF, YouTube, RSS, sitemap,
  repo), its **crash-resumable queue**, and its **cross-page reconciliation** —
  not its trust property, and not its storage. See *What we intend to build from
  this*, below.

**compozy-kb** — `github.com/compozy/kb` (`compozy-kb/`)

- Draws the line we drew: the CLI owns the non-LLM workflow (scaffolding,
  ingestion, structural linting, indexing, search) and **LLM compilation stays
  in the agent layer**.
- Multi-source ingestion (URLs, files, YouTube, codebases, bookmarks) into one
  topic-shaped vault, with `lint` as a structural gate and `promote` as the
  curation step into a checked bundle.
- `kb inspect complexity` / `dead-code` — codebase analysis as an ingestible
  knowledge source, not just prose.
- OKF (Open Knowledge Format) bundles with `okf check --strict` — an existing
  interchange format for agent knowledge, worth conforming to rather than
  inventing.
- *Feeds:* our knowledge-base ingestion path; OKF as a candidate export format
  for sharing the kernel outside the team.

**context** — `github.com/matryer/context` (`context/`)

- The closest match to the manifesto's social goal: a team knowledge base where
  people, projects, and updates are markdown in git, reviewed by PR, and
  interrogated in whatever LLM IDE each person already uses. Non-engineers
  contribute through the same workflow.
- `/questions/` — saved prompts that produce *repeatable* output (weekly status,
  project health). Determinism obtained by fixing the prompt, not the model.
- A `GLOSSARY.md` explicitly tuned "so the agents speak your language" — shared
  vocabulary as a first-class, maintained artifact.
- `/ingest` accepts raw meeting notes, screenshots, and status updates and files
  them into structure — the low-friction on-ramp our QA, PM, and support
  colleagues need.
- *Feeds:* the team-facing surface of our knowledge base; saved-question
  templates; a maintained glossary as a base-rules dependency.

**leona-kb** — `github.com/leona/kb` (`leona-kb/`)

- Externalizes per-project `CLAUDE.md` into a central versioned KB and leaves
  behind an `@import` pointer, so edits land in the KB rather than the repo copy.
  Kills the duplicated-context problem across repos.
- Every write auto-commits with a descriptive message — free provenance and
  history on a corpus that agents mutate.
- Ref vs. inline modes: large reference docs stay out of the session and are
  fetched on demand via MCP; small ones are embedded at session start. Explicit
  control over context budget.
- *Feeds:* central-KB-with-pointers layout; on-demand vs. always-loaded as a
  declared property of each knowledge artifact.

**Graft** — `github.com/NanoNets/Graft` (`Graft/`)

- A code graph built from the repo and injected per-turn, with a published,
  reproducible benchmark: −46% tool calls, −42% tokens, −60% time, and
  correctness 54% → 66% on SWE-bench Verified.
- The methodology matters more than the numbers: a 162-run controlled benchmark
  where *only the context differs*. That is the experimental design we need to
  justify our own context curation.
- The graph is a regenerable local cache (gitignored); what gets committed is
  the wiring. Right split between shared configuration and derived artifact.
- *Feeds:* our benchmark methodology; treating context indices as regenerable
  artifacts rather than reviewed source.

### Durable State, Memory, and Coordination

**mnemon** — `github.com/mnemon-dev/mnemon` (`mnemon/`)

- Names the architecture we independently arrived at: **LLM-supervised**. The
  binary does deterministic work (storage, indexing, search, decay); the host
  LLM makes judgement calls. Contrasted explicitly against LLM-embedded (Mem0,
  Letta), file-injection, and plain MCP-server designs.
- Intent-native primitives — `remember`, `link`, `recall` — named in the model's
  cognitive vocabulary rather than SQL's, with structured JSON output and signal
  transparency.
- Hooks drive the memory lifecycle deterministically instead of hoping the model
  remembers to call the tool.
- The compound-interest argument: engines churn, skills are cheap to rewrite,
  accumulated memory is the asset worth investing in.
- *Feeds:* vocabulary and framing for our direct-model interface; hook-driven
  determinism so corrective interactions are captured without relying on the
  model's cooperation.

**Yul-lu** — `github.com/tomanagle/Yullu` (`Yul-lu/`)

- The mechanism that makes corrections *accretive*: a **Stop hook** fires
  `record_messages` after every turn, so capture is deterministic rather than
  dependent on the model choosing to save something. (Their own README notes
  Codex, lacking hooks, degrades to hoping the model complies.)
- "Dreaming" — a background pass over recorded turns that extracts durable
  memories after the fact, so the user never has to decide in the moment what
  was worth keeping.
- Team sync via an event log committed to `.yullu/` in the repo: shared memory
  over ordinary git, no service.
- Memories scoped by canonicalized `origin` URL so SSH and HTTPS clones resolve
  to the same project.
- A similarity threshold that *drops* weak matches instead of injecting them as
  noise, plus per-recall relevance ratings — retrieval quality as a measured,
  tunable property.
- Reasoning via MCP sampling: the client's own subscription pays for the LLM
  call, so the tool needs no API key of its own.
- *Feeds:* Stop-hook capture of every corrective interaction; retro-extraction
  of durable rules; measured retrieval relevance.

**pantry** — `github.com/mobydeck/pantry` (`pantry/`)

- A minimal, cross-agent note store where one agent's notes are searchable by
  all — the smallest useful shared-memory surface.
- **Three-layer secret redaction before anything hits disk.** Any capture
  mechanism we build inherits this requirement.
- Zero idle cost: no daemon, no background process; the MCP server exists only
  while the agent runs it.
- A structured note schema (`what` / `why` / `impact` / `category` / `details`)
  with `details` explicitly written for "a future agent with no prior
  knowledge" — a good template for our decision records.
- Hybrid retrieval: FTS5 keyword search works with no configuration, embeddings
  are strictly optional. Graceful degradation to zero dependencies.
- *Feeds:* redaction requirements for any transcript capture; the what/why/
  impact note schema for recording local trade-off decisions.

**beadwork** — `github.com/jallum/beadwork` (`beadwork/`)

- States the problem our knowledge base exists to solve: compaction, session
  boundaries, and crashes silently kill in-context plans.
- Durable state on a git **orphan branch**, manipulated directly in the object
  database — never touches the working tree, never conflicts with code.
- Every operation is an atomic commit; sync is fetch-rebase-push with **intent
  replay** on conflict. Two agents on the same repo never contend for a file.
- `bw onboard` prints the snippet to paste into the instructions file, and
  `bw prime` re-injects workflow context at session start. Bootstrap is a
  command, not documentation.
- *Feeds:* orphan-branch storage for harness state; intent replay as a
  multi-writer conflict model.

**clu** — `github.com/arjia-labs/clu` (`clu/`)

- Atomic claim via `UPDATE … RETURNING` so racing agents provably get different
  work — coordination as a database property, not a prompt instruction.
- **Human-approval checkpoints inside the workflow graph**, so risky steps gate
  on a person by construction. Directly serves our risk-posture requirement.
- Context inheritance: `clu claim <id> --context` prints the upstream chain of
  notes and comments, so an agent inherits what prior agents learned.
- Capability routing — agents declare capabilities, `cap:` labels route work to
  the ones that match. The mechanism for a team with heterogeneous skills.
- Full audit log on every write; `--json` on every command; single binary, no
  network, no telemetry.
- *Feeds:* approval checkpoints and audit logging in adh's ops stage;
  capability routing for assigning work across our mixed-skill team.

**goalx** — `github.com/vonbai/goalx` (`goalx/`)

- Decomposes one objective into **typed obligations** with an immutable
  `objective-contract`, a mutable `obligation-model`, an `assurance-plan`, and an
  `evidence-log`. This is the closest thing we have found to a machine-checkable
  encoding of non-functional requirements and their satisfaction argument.
- Draws the line we want: `goalx verify` **records assurance evidence, it does
  not issue a completion verdict** — the master agent still owns judgement.
  Evidence is deterministic; the verdict is not.
- `goalx schema <surface>` — inspect the authoring contract *before* writing
  machine-consumed durable state. Schema-first writes for agents.
- `freshness-state` tracks whether evidence or cognition has gone stale, so
  conclusions expire rather than silently rotting.
- Worktree-isolated parallelism with explicit `keep` merge boundaries; runs
  survive restarts via journals, leases, and saved artifacts.
- Refuses unsafe execution under resource pressure explicitly rather than
  silently shrinking fan-out — degradation is visible, never quiet.
- *Feeds:* obligation/assurance/evidence modelling for our NFR articulation;
  evidence-not-verdict as a design rule for skillsaw and canonizer; staleness
  as a first-class state.

**rebar** — `github.com/willackerly/rebar` (`rebar/`)

- Architecture **contracts** as named behavioral specifications that outlive
  team turnover, plus a hard-won naming lesson: descriptive names, never
  numbers, because numbers collide on merge and mean nothing across repos.
  Their `CONTRACT-BLOBSTORE.2.1` post-mortem is worth reading before we name
  anything.
- Role-based workflow boundaries (architect / product / eng lead / developer)
  and testing tiers T0–T5 — an information-organization model for a team that
  looks like ours.
- Persistent per-role agent sessions exposed as MCP tools (`ask_<repo>_<role>`),
  so a role can be consulted without paying full sub-agent context cost, and
  findings propagate cross-role, cross-repo, cross-session, and cross-failure.
- Explicitly tracks failure modes as swarm knowledge — failures become shared
  assets rather than repeated lessons.
- Tiered adoption profiles (solo ~15 min → department ~2 hours) over a plain
  text substrate, with binaries strictly opt-in.
- *Feeds:* naming and provenance conventions for our base rules; role boundaries
  for adh; capturing failure modes as first-class knowledge-base entries.

### Harness Internals and Model-Agnosticism

**byo-coding-agent** — `github.com/betta-tech/byo-coding-agent`
(`byo-coding-agent/`)

- The reference explanation of harness engineering, and the layering we should
  adopt in our own docs: **build** the loop, **extend** it with tools,
  **configure** it with files. Most practitioners only ever touch the top layer.
- A ~600-line Go agent with three orthogonal extension points behind small
  interfaces — `Provider`, `Tool` (self-registering), `CompactionStrategy`
  (with a `WithLogging` decorator). Swapping any one is a single line.
- Live provider/model switching mid-session proves the abstraction: the harness
  is unchanged when the model changes.
- *Feeds:* the design of our stub direct-model interface; compaction strategy as
  a pluggable, observable component rather than a hidden behavior.

**agentx** — `github.com/sageox/agentx` (`agentx/`)

- Names the discipline — **Agent Experience (AX)**: CLIs designed for agent
  callers, not merely tolerant of them. This is the missing vocabulary for what
  our four CLIs are doing.
- A registry of 14 agents with their config paths, context filenames,
  capabilities (hooks / custom commands / MCP), and installation detection —
  the compatibility matrix our harness needs to install itself anywhere.
- `AGENT_ENV` propagation: detect once, and every downstream tool and child
  process reads one variable instead of reimplementing detection.
- Capability probing before use, so a tool degrades knowingly on an agent
  without hooks rather than failing.
- *Feeds:* promotion into skillet as the agent-detection package our CLIs share;
  capability-aware installation across Claude, Gemini, and Qwen.

______________________________________________________________________

The projects above are the `~/Documents/agent-red` survey — tools we read as
peers. Those below are `~/Documents/agent-blue`: the sources the practice itself
came from. Several are already absorbed, and saying *where* is as useful as
saying what.

### The Practice, and Where It Already Landed

**harness-engineering** — `agent-blue/harness-engineering`

- **This is the manifesto's own source text.** "Improves the two external
  levers — context and tools — and curates the environment around them," and a
  harness whose central purpose is "to carry an organization's nonfunctional
  requirements: the quality attributes and constraints governing reliability,
  security, compatibility, maintainability, performance, operability, risk
  posture, and polish," plus "local decisions about how to prioritize, trade off,
  and satisfy those requirements." Sections 2 and 3 of this document are that
  paragraph, applied to us.
- What we have not yet taken from it: *make coherence cumulative* — lessons from
  accepted work, corrections, failures, and user responses becoming context,
  boundaries, tools, examples, and checks that shape later trajectories. That is
  the accretion requirement stated as a design goal rather than a wish.
- Also ships `playbooks/`, `evals/`, and `sources/` — the same
  knowledge-base-plus-evaluation shape we built, arrived at independently.
- *Feeds:* the vocabulary we already use; the cumulative-coherence framing for
  the knowledge-base work.

**SkillLens** — `microsoft/SkillLens` · **SkillOpt** — `microsoft/SkillOpt`

- Already absorbed, and the fusion lives in `skillsaw` and nowhere else:
  SkillLens's three skill-quality detectors (failure-mode encoding, actionable
  specificity, risk-action blacklist) are `skillet/skilllens` and rubric dims
  3/5/9; SkillOpt's validation-gate ratchet (`sha256[:16]` identity, strict-`>`
  accept/reject) and rule-judge operators are `skillet/ratchet` and
  `skillet/judge`.
- Still unharvested: SkillLens's **lifecycle framing** — experience generation →
  skill extraction → skill consumption — and its two metrics, *Extraction
  Efficacy* and *Target Evolvability*. We score the artifact; neither of those
  scores the artifact. They score whether extraction captured what the trajectory
  contained, and whether the consuming model got better. That is the axis
  `skillsaw calibrate` is reaching for.
- SkillOpt's epochs / batch-size / learning-rate framing is the honest name for
  what `skillsaw-skill`'s hill-climbing loop is doing by hand.

**nfrs-guide** — `agent-blue/nfrs-guide`

- The source of the **Planguage** discipline already implemented in adh's
  `internal/nfr` (Scale, Meter, Fail/Goal/Stretch, Direction). Confirms that
  package is grounded rather than invented.
- The distinction worth adopting explicitly: **validation** (are we building the
  right thing — requirements examined *for conflicts* and assurance they meet
  need) versus **verification** (are we building it right — quality gates). Our
  tooling is almost entirely verification. Contradiction detection is the first
  validation-side check we will own, and this is the vocabulary for it.
- Patterns and templates for implementing NFRs, and a taxonomy for classifying
  them — the input to any NFR articulation beyond adh's current specs.

**cc-thinking-skills** — `tjboudreaux/cc-thinking-skills`

- Already surveyed; `skillsaw` and `exegesis` carry the *Eval-methodology
  adoptions* section from it (Wilson intervals on activation, objective
  answer-scorers, the measured-axis / disposition-axis split in `gate`).
- The part worth restating: its **Elevate-or-Kill scorecard publishes that zero
  of its 28 skills hold a replicated ELEVATE verdict**, rather than hiding it.
  Replication-gating and publishing the negative result is the standard our own
  scorecards should meet.

**unified-thinking** — `quanticsoul4772/unified-thinking`

- Already surveyed across all five TODOs. Yielded `skillet/calibration` (Brier /
  ECE / MCE); its keyword bias, fallacy, and blind-spot detectors were
  deliberately rejected family-wide as uncalibrated heuristics.
- `skillet/bandit` (Thompson sampling) remains deferred there pending a second
  consumer.

**evals-differential-oracle** — `egnaro9/evals-differential-oracle`

- The source of adh's **differential oracle self-test**. Its thesis is the one we
  keep re-deriving: *agreement between two independently-built implementations is
  a far stronger signal than either passing its own tests*.
- Its planted-defect negative control (`impl_buggy.py`, with a test proving both
  nets catch it) is already ours twice over — adh's `oracle selftest` and
  canonizer's `gate.SelfTest`, which explicitly mirrors it.
- **Two independent nets, not one**, is the part we took only half of.
  `src/invariants.py` — "the rules any correct implementation must obey, checked
  against a board independently of how the result was produced" — needs no second
  implementation and holds even when both are wrong in the same way. We have the
  differential half; the invariant half exists for rulesets (`verify.Executable`,
  `verify.Provenance`) and nowhere else.

**modelith** — `stacklok/modelith`

- Already a consumer relationship: `skillet.modelith.yaml` + the rendered `.md`
  are authored with it, and `skillet/provenance` is generalized from its vendored
  header.
- The pattern we and it converged on independently: **CI regenerates the output
  and fails on drift** — `modelith render --check` ("non-zero exit on drift"),
  matched by exegesis's `index --check` and `normalize --check`. A generated-doc
  check treated exactly like a generated-code check.
- Where we have *not* converged: **canonizer never calls `ruleset.Render`**, only
  `Parse`, and has no `--check`. A stored ruleset can be parseable yet
  non-canonical, so `Render(Parse(x)) != x` goes unnoticed — which matters
  because contradiction detection compares normalized rule text.

**darwinian_evolver** — `imbue/darwinian-evolver`

- The population-based framing behind `darwin-skill`, which `skillsaw`
  reimplements deterministically in Go.
- The property worth noting for our ratchet: it is **resilient to a noisy
  evaluator and an unreliable mutator** — a mutator that improves only 20% of the
  time still drives progress, because selection does the work. Relevant to how
  hard `skillsaw-skill` should try to make each single edit good.

### The Knowledge-Base Gap — the Highest-Value Find

**knowledge-catalog / OKF** — `GoogleCloudPlatform/knowledge-catalog` (`okf/`)

**The Open Knowledge Format v0.2 specifies, as a published standard, the four
things this document proposed to invent.** Its motivation section names them
directly: provenance ("what was this created from, and how was it verified?"),
trust ("how much should I trust it?"), freshness ("is it still true?"),
lifecycle ("is it the current version?"), and attestation ("was this number
produced the way we said it must be?") — all made first-class because "a
knowledge corpus is not authored once and then read: it is continuously written
and maintained by agents."

- **It is already our storage format**: a directory of markdown files with YAML
  frontmatter, no schema registry, no central authority, no required tooling.
  Conforming costs no migration.
- **Trust tiers, derived not declared** (§5.3): no `verified` key ⇒
  *unverified*; verified by non-human actors only ⇒ *machine-confirmed*;
  verified by a `human:<id>` actor ⇒ *human-reviewed*. That is exactly the
  provenance tier the knowledge base needs once ingestion is automatic.
- **`generated` and `verified` are separate fields** (§5.2) because "who *wrote*
  a concept need not be who *confirmed* it" — canonizer's entire thesis, as a
  data field. `verified` is a *list* of independent checks (a human sign-off
  *and* a nightly process), and is independent of `generated.at`: content can
  change without re-confirmation, and facts can be re-confirmed without
  regeneration.
- **`stale_after` is an absolute date, not a relative TTL** — chosen so the
  staleness decision "is a plain date comparison with no reference to when the
  concept was read." That is a determinism argument, and it is the right one:
  a TTL is read-time-dependent, a date is a pure function.
- **`status: draft | stable | deprecated`**, where deprecated is "kept for links
  and history; no longer current" — supersession without deletion.
- **Attested computations** (§10): a computation is its own concept with contract
  fields, and §10.6 separates *verification* from *attestation* — the distinction
  our `proof` packets gesture at.
- **Graceful adoption is specified**: a concept with no trust frontmatter is
  still consumable and consumers MUST NOT reject it (§11). No big-bang migration.
- *Feeds:* adopt as the knowledge base's frontmatter contract instead of
  inventing one. It supplies the *sourced* provenance class directly; the
  *adjudicated* class maps onto `verified` by a `human:` actor with no
  `sources`.

**leafwiki** — `perber/leafwiki`

- Single Go binary, SQLite plus **Markdown on disk**, no Node/Redis/Postgres —
  the operational floor for serving a knowledge base to non-engineers without
  taking on a platform. Git backup is a first-class (if experimental) mode.
- Reverse-proxy authentication and a plain data directory mean the corpus stays
  a set of files we own rather than rows in someone's schema.
- *Feeds:* a candidate read/edit surface for QA, PM, and support colleagues over
  the same git-backed markdown the CLIs gate.

**NFRLocator** — `agent-blue/NFRLocator`

- Finds and categorizes **non-functional requirements in unconstrained natural
  language** — the exact extraction step between "we ingested a document" and
  "we have typed obligations."
- The genuinely scarce asset is the data, not the Java: **labeled NFR corpora**
  (PromiseData, iTrust, OpenEMR, CCHIT ambulatory requirements, RFPs, CFRs,
  DUAs) plus an NFR category listing. A labeled ground truth is what any
  classifier — deterministic, model-driven, or hybrid — must be measured
  against, and we have none.
- *Feeds:* an evaluation set for NFR extraction, and a category taxonomy to
  reconcile against adh's `nfr` tags.

### Self-Evolution Loops, and What They Cost

**hermes-agent-self-evolution** — `NousResearch/hermes-agent-self-evolution`

- GEPA (Genetic-Pareto Prompt Evolution) over SKILL.md files, notable for one
  reason: it **reads execution traces to understand *why* something failed, not
  merely that it did**, then proposes targeted improvements. `skillsaw diagnose`
  answers "what is weakest"; nothing answers "why did it fail in use."
- **Constraint gates (tests, size limits, benchmarks) sit between candidate and
  merge**, and the output is a PR — evolution that terminates in human review.
- Can evolve against real session history rather than synthetic evals.

**hermes-dojo** — `Yonkoo11/hermes-dojo`

- Closes the loop we have left open: **measure → identify weakness → evolve →
  measure again → report.** Its premise is our premise, stated as a complaint:
  "You correct it, it forgets next session… Self-evolution exists but nobody uses
  it because there's no signal about WHAT to evolve."
- The signal is mined from session logs — tool errors, retry loops, **user
  corrections ("no, I meant…")**, explicit complaints — per skill. That is the
  concrete mechanism for making a corrective interaction accretive, and it needs
  no cooperation from the model.
- Stores daily metrics and shows a learning curve, so improvement is evidenced
  rather than asserted.
- *Feeds:* mining our own session logs for corrections as the input to
  `skillsaw-skill`'s edit selection.

**hermes-skill-factory** — `agent-blue/hermes-skill-factory`

- Detects a repeated workflow in session history and **proposes a new skill**,
  with an explicit accept/decline prompt rather than silent creation. The
  producer-side complement to dojo's optimizer, and the missing front end to
  `book2skill` (which needs a book; this needs only that you did something
  twice).

**super-hermes** — `agent-blue/super-hermes`

- The one idea here we have nothing like: a **constraint report** appended to
  every analysis — "this analysis maximized structural depth; it did not examine
  temporal degradation or security surfaces." **Declaring what was *not* examined
  turns an unknown into a recorded gap.** Directly applicable to critic and
  cold-critic output, where silence currently reads as coverage.
- Also: deriving structural impossibility ("this is a structural impossibility,
  not a code flaw") rather than naming a pattern — a higher bar for what a
  finding may claim.

**PolyBrain** — `mosesman831/PolyBrain`

- Multi-model orchestration whose output contract is **verified claims** —
  parallel research, then synthesis, with claims checked rather than merged on
  trust. The multi-model half of what our Claude/Gemini/Qwen split could be if
  the models cross-checked each other instead of taking turns.

### Judgment Gates and Deterministic Routing

**agentic-harness-bootstrap** — `agent-blue/agentic-harness-bootstrap`

- "Give agents the map, not the manual." Generates the whole harness surface in
  four phases (discover → analyze → generate → verify): instruction files for
  three agents, `ARCHITECTURE.md`, lint config, pre-commit hooks, ADR directory,
  CI, and — the part that matters — **`scripts/verify-harness.sh`, persistent
  harness integrity checks**. The generated harness checks itself thereafter.
- ADRs scaffolded from the start, so architectural decisions have a home before
  anyone needs one.
- *Feeds:* onboarding a new repository into the practice; a harness-integrity
  check comparable to adh's `doctor`.

**mycellium-harness** — `haabe/mycelium`

- The judgment gate we lack: it makes the agent **earn the right to start** —
  four questions (what problem, who has it, riskiest assumption, smallest test)
  before an editor opens. "What the agent won't do is silently skip past missing
  evidence and call the work done."
- **Depth is negotiable, skipping is not**: a weekend hack gets lighter prompts
  than a team product, and you may decline depth at any step — but not silently.
  That is the right shape for a gate a mixed-skill team has to live with daily.
- *Feeds:* a pre-strategy gate for adh's arc loop, where work currently begins at
  planning rather than at justification.

**mycellium-io** — `mycelium-io/mycelium`

- Coordination for agents as **peers** — no orchestrator, no supervisor — with
  shared rooms, persistent memory, and **semantic negotiation**. Names the
  failure mode precisely: without coordination you get "AI theatre — agents that
  talk over each other, repeat work already done, **fail to recognise
  disagreement**, and fail to negotiate trade-offs."
- Reports that alignment pays off at 3+ agents and is often decisive at 4+.
  Useful calibration on when coordination machinery is worth its cost.
- *Feeds:* recognising disagreement is contradiction detection at the agent
  layer rather than the document layer — the same predicate, different corpus.

**virgil** — `agent-blue/virgil`

- **The determinism ladder we want, already built:** exact match → keyword index
  → category narrowing → **AI fallback**, with 80%+ of queries never reaching a
  model — and every fallback **logged as a miss so the deterministic layers can
  learn.** The AI surface area shrinks with use. That is the operational answer
  to "minimize the model," and it is measurable rather than aspirational.
- **Memory, not chat history**: every invocation is stateless, context assembled
  fresh per call from memory rather than accumulated until it overflows and is
  lossily compacted. No context wall by construction.
- Pipes and pipelines with a standard envelope contract, composable
  recursively — a pipeline looks like a pipe from outside.
- Per-pipe KPIs derived from how output is received, driving proposed (or
  auto-applied) configuration changes.
- *Feeds:* the routing architecture for the harness's direct-model interface;
  log-the-miss as the mechanism that makes determinism increase over time.

______________________________________________________________________

Below is the `~/Documents/agent-fuschia` survey. Where agent-red gave us peer
tools and agent-blue the sources of the practice, this set is narrower and
sharper: it is about **what makes a claim checkable**, and about the vocabulary
layer without which no two claims can be compared at all.

### Verifiable Claims, and Named Refusals

**vac-protocol** — `egnaro9/vac-protocol` (`vac-protocol/`)

- A **Capability Evidence Bundle**: a manifest, artifacts pinned by sha256,
  declared numbers a verifier **recomputes from those artifacts offline**, and
  the exact commands to re-earn every verdict from the issuer's own
  deterministic grader. "Do not trust us, run it."
- **The split we keep half-making, stated cleanly** (§4): *structural
  verification* and *semantic replay* are "two distinct acts, never to be
  conflated." Structural means zero network and zero issuer code — schema valid,
  artifacts hash-identical, bundle closed, limitations stated, every declared
  number recomputed. Semantic replay clones the issuer at a pinned commit and
  re-earns the verdicts. **"A structural PASS means the bundle is *internally
  honest*, not that the issuer's grader agrees."** And the case worth designing
  for: a bundle that passes structure and fails replay "is a precise,
  reproducible accusation against the issuer."
- **`limitations` is REQUIRED and non-empty**; a bundle without explicit
  non-claims is invalid (`empty-limitations`). "A capability statement that will
  not say what it does not cover is an advertisement, and VAC does not carry
  advertisements." This is super-hermes' constraint footer promoted from a
  convention to a *format requirement*.
- **A closed vocabulary of named failure reasons** — nineteen of them
  (`sha256-mismatch`, `unlisted-file`, `summary-outruns-checks`,
  `stamp-mismatch`, …) — one named reason per failure, never free prose.
- **Registry rejection rules** (§5), of which the third is the sharpest:
  grading that names an LLM judge, human scoring, or anything wall-clock or
  sampling dependent without a pinned seed is refused outright — "if replay
  cannot reproduce it byte-for-byte, it is not evidence." And "the verifier's
  exit code is the floor, not the bar."
- **A challenge protocol** (§6): any reader may challenge an accepted bundle,
  in three classes — *replay* ("I ran your commands and got something else",
  the strongest, because it ships a counter-recipe), *coverage* ("the evidence
  does not support the stated scope"), and *scope* ("the limitations are
  incomplete").
- **§7 is titled "Explicitly refused in v0.1", with the best one-line defence of
  a practice this family already keeps informally:** *"Named refusals, so their
  absence reads as a decision rather than an oversight."* The refusals earn it —
  signatures are refused because "a signature proves who spoke, not that they
  spoke the truth… signing an unreplayable bundle would launder it"; Docker
  images because "shipping opaque filesystem images as 'reproducibility' hides
  exactly the drift this protocol exists to surface."
- *Feeds:* the structural/semantic split for canonizer's `verify` vs `critic`;
  mandatory limitations everywhere a claim is made; a closed reason vocabulary
  for `finding.Category`; the challenge protocol as adjudication's front door.

**vac-gate** — `egnaro9/vac-gate` (`vac-gate/`)

- States our own §12 rule better than we did: "Every PASS states what ran and
  what deliberately did not — **a gate that cannot say what it skipped is worse
  than no gate.**"
- Two absence-is-a-distinct-state rules, arrived at independently of ours:
  `binding-unrecorded` — "unrecorded is not matching"; and, best of all,
  **"'cannot regrade' is not 'regraded'"** — the regrader's honest stale-code
  refusal *fails* the gate rather than passing it.
- *Feeds:* adh's evaluation disposition; the not-applicable outcome now
  required of `quotecheck`.

**gradecore** — `egnaro9/gradecore` (`gradecore/`)

- "Every grade is a pure predicate over a string, so it reproduces exactly…
  **No second model grades the first**; there is nothing here you can't rerun
  and get the same answer." Zero dependencies.
- **`suite_hash`** — a `sha256[:12]` over each task's identity string, so two
  independent implementations can be *shown* to agree rather than asserted to.
  Its own docstring names the weakness in the scheme it is compatible with: the
  `id:prompt` form "misses an edited answer-key", so fold the grader id and
  expected value into each identity.
- **The README corrects itself in public**: *"'Shared by' was the older wording
  here and it was false in code"* — replaced with the checkable claim (same
  `suite_hash`, all 35 graders lift, 0 of 175 verdicts differing). That is the
  standard for how this family should phrase a compatibility claim.
- *Feeds:* a set-hash beside `identity.Hash`; the phrasing discipline for any
  "these two tools agree" claim.

**evalmut** — `egnaro9/evalmut` (`evalmut/`)

- Mutation testing for **eval suites**: "Your eval suite passes. Does it
  actually check anything?" Injects a known defect into an output the grader
  passed and reruns it; a grader that still passes has a **hole**.
- The defects are **mined from documented real-world eval failures, not
  invented** — which is what separates it from an arbitrary fault battery.
- Two-sided by construction: a MISSED mutation is a defect class the eval is
  blind to; a FLAGGED one is a correct-output class it wrongly rejects. Holes
  are classified `blind` vs `coverage-gap`.
- *Feeds:* skillsaw's rubric checks and gnosis's promote gate are both evals,
  and neither can currently demonstrate it would catch a defect it lacks a
  fixture for.

**agent-certlab** · **eval-history** — `egnaro9/*` (`agent-certlab/`, `eval-history/`)

- certlab: run an agent against tasks with **seeded, known defects**, and
  "grade only the artifacts it leaves on disk… **never the agent's own account
  of its success.**" The artifacts-only rule is sharper than our cold-critic
  framing, which withholds reasoning but still reads a reply.
- eval-history: "An eval score tells you how the system did *today*. It can't
  tell you that yesterday's change made things worse — and 'worse' is the only
  thing you actually need to be told." `skillet/timeseries` as a service, and
  independent confirmation of regression-relative gating.

### Claim Granularity — the Defect We Had Not Seen

**claim-segmenter-kit** — `rajatslakhina/claim-segmenter-kit` (`claim-segmenter-kit/`)

- Deterministic claim segmentation: **no model call, no network, no
  dependencies** — and it names a hole in any quote-validated corpus, ours
  included. **Sentence granularity conflates supported and unsupported claims
  inside one sentence:**

  > The cache is enabled by default, but it is not shared across sessions.

  One sentence, two assertions, and a verifier can return only one verdict for
  it. A quote can validate while half of what it appears to support is
  unsourced.

- The guarantee is the discipline: **"Every emitted claim stands on its own, or
  the cut is not made."** Splitting at the comma would leave *it is not shared
  across sessions*, whose subject sits in the discarded half — so the subject is
  recovered and substituted, or no cut happens.

- It is also honest about the field: every published alternative — FActScore
  atomic decomposition, decomposer/verifier alignment, molecular-facts
  decontextualization — needs a model. "This package is the deterministic corner
  none of them occupy."

- Measured, not asserted: `split(".")` yields 12 fragments, a sentence splitter
  8, this 5, over the same paragraph.

- Swift, so not importable — the algorithm and the guarantee are what transfer.
  Its named siblings are worth reading before we design conflict detection:
  **SourceConflictKit** (conflict between sources), ClaimConsistencyKit,
  GroundingKit.

- *Feeds:* the evidence invariant's grain; the claim-segmentation step that must
  precede quote validation.

### The Vocabulary Layer

**lexicon** — `jedi-knights/lexicon` (`lexicon/`)

- A markdown-native requirements DSL, and the reason it exists is our problem
  stated in other words: "Dev, QA, and Product routinely read the same
  requirement and walk away with three different mental models — not from
  carelessness, but because prose leaves three things implicit."
- **The mechanism worth taking: `Keyword` and `Role` are separate and both are
  kept.** A step records the surface keyword the author chose *and* the resolved
  semantic `StepRole` (`precondition`/`action`/`outcome`); two dialects resolve
  to the same Role, `And`/`But` inherit the preceding step's Role, and "Role is
  what a consuming human or LLM should key off of"
  (`internal/domain/step.go`).
- **This dissolves the cross-functional vocabulary problem rather than
  adjudicating it.** Each function writes in its own words; the resolution table
  is data. Go, MIT, hexagonal (`domain`/`ports`/`adapters`), and it compiles to
  Gauge, Gherkin, or schema-stable JSON "built for LLM consumption."
- *Feeds:* the `type` vocabulary — surface term plus resolved concept, both
  retained, resolution as data rather than debate.

**hubi** — `mvcds/hubi` (`hubi/`)

- "Like a database migration tool but for **ubiquitous language**": declare the
  domain in YAML, generate code, schema, and documents from it, across
  repositories that must not drift apart.
- Carries a **soft-deprecation path** on an attribute
  (`deprecated: [{message: …}, {error: false}]`) — announce, then enforce. That
  is the answer to "how does a vocabulary change once links already exist,"
  which is the standing objection to committing to a taxonomy early.
- *Feeds:* vocabulary migration; the answer to the deferred bundle-layout
  decision.

**termageddon** · **Glossary** — (`termageddon/`, `Glossary/`)

- Both solve the case `lexicon` does not: two groups that genuinely *disagree*
  rather than merely using different words. termageddon offers
  **perspective-based organization** with GitHub-style approval workflows;
  `Glossary` (C-Accel) keeps **multiple definitions per term, keyed by which
  team uses them**, explicitly "to enable teams to interoperate without
  enforcing a single artificial definition."
- *Feeds:* the bounded-context escape hatch when one canonical definition is the
  wrong answer.

**glossary-18F** — `18F/glossary` (`glossary-18F/`)

- A small accessible panel that resolves `data-term` attributes to definitions
  inline, as shipped on FEC.gov. Makes the vocabulary visible **where a term is
  used** rather than filed away in a glossary nobody opens.
- *Feeds:* the web interface's reading surface.

### Capture, and the Rest

**engineering-notebook** (`engineering-notebook/`) ingests Claude Code *and*
Codex session transcripts into daily summaries and a browsable journal — the
Stop-hook capture path already built, and a useful reference for the transcript
adapters. **jargon-v1** (`jargon-v1-main/`) is an AI-managed zettelkasten
parsing articles, papers, and videos into index-card-sized ideas — the closest
sibling to what we are building, and its card grain is an argument about claim
granularity.

Surveyed and off-axis: `agent-graph`, `crashkit`, `rag-eval-lab`, and
`aidetector` are model-facing, and our tools do not call models;
`effect-domain`'s "transports are projections of the model" is a good principle
on the wrong stack; `universal-translator` is i18n we need only if the web
interface is localized; `BugHive`, `metaphorically`, and the remaining glossary
web applications (`devterms`, `jargons.dev`, `glossary-kit`, `web-jargon`,
`yourjargon`) are products rather than components.

**One warning on vac-protocol.** Its claims are *about AI systems* — "this agent
handles X under conditions Y." Ours are *domain* claims — "the retry budget is
3." The bundle mechanics, the invalidity rules, and the structural/semantic
split transfer cleanly; the schema and its semantics do not. Take the
discipline, not the format.

______________________________________________________________________

### Prior Art for the Wiki Itself

The `~/Documents/agent-magenta` survey: seven personal-knowledge-management
systems. This is the most direct prior art we have — each is a markdown (or
org) corpus with a derived index, a link graph, and a CLI or editor surface,
which is gnosis's problem exactly. Several arrived at the same answers
independently, and that convergence is itself evidence.

**Read the designs, not the code.** `zk`, `zettel`, and `org-roam` are GPLv3;
`zettelstore` is EUPL-1.2. All four are copyleft and **none can be vendored into
an MIT- or Apache-licensed Go binary.** The temptation is real with `zk` in
particular — same language, same problem, good code — so the constraint is worth
stating before the survey rather than after. A table layout is a fact about the
problem; the implementation is not ours to take.

**zk** — `zk-org/zk` (GPLv3, Go)

- **A broken link is a first-class row, and it keeps what the author wrote.**
  `links.target_id` is nullable with `ON DELETE SET NULL` while `href` is always
  retained, so deleting a note *degrades* its inbound links to unresolved rather
  than erasing them. Our `links(from_path, to_path, resolved BOOL)` throws away
  the href when the target is missing — which is the one case you most need it.
  This is OKF §6.1's "not-yet-written knowledge" implemented rather than
  asserted.
- **The link's surrounding prose is stored**: `snippet`, plus `snippet_start`
  and `snippet_end` offsets. Since OKF §6.1 says a relationship's kind "is
  conveyed by the surrounding prose, not by the link itself", storing the snippet
  is storing the untyped relation's evidence — and it is what lets a reader see
  *why* A links to B without reopening A.
- **Typed relations live in the derived index, not the source format.**
  `links.rels` and `links.external` exist even though markdown links carry no
  type. That is a concrete answer to the ontology question: derive the type,
  index it, and leave the markdown plain.
- `notes.checksum` is stored and indexed — change detection without reparsing —
  and `notes.lead` holds the first paragraph separately from the body, which is
  progressive disclosure's snippet already materialized.
- **The FTS5 configuration is worth copying wholesale**: an external-content
  table (`content = notes, content_rowid = id`) so bodies are not duplicated,
  with `tokenize = "porter unicode61 remove_diacritics 1 tokenchars '''&/'"`.
  The custom tokenchars are the detail — an apostrophe and a slash in the token
  set are what make `don't` and `foo/bar` searchable in technical prose.
- Schema migrations run off `PRAGMA user_version` with numbered steps — the same
  mechanism `skillet` uses, arrived at separately.
- `collections` plus `notes_collections`, keyed by a `kind` column, carries tags
  and any other grouping in one generic pair of tables rather than a table per
  kind.
- *Feeds:* the link table's shape; the FTS5 tokenizer; snippet storage as
  relationship evidence.

**org-roam** — `org-roam/org-roam` (GPLv3, Emacs Lisp)

- **`files` and `nodes` are separate tables: one file holds many addressable
  nodes.** That is the single most important structural idea in this survey for
  us, because our `concepts(path PK)` assumes one concept per file and therefore
  cannot address a claim inside one — which is exactly the granularity defect
  that `claim-segmenter-kit` exposed. org-roam shows the index shape that
  supports sub-document addressing **without splitting the files**.
- `aliases(node-id, alias)` — many names for one node. The surface-term versus
  canonical-concept problem again, solved at the entity level rather than the
  vocabulary level.
- `citations(node-id, cite-key, pos, properties)` — citations are first-class
  rows **carrying their position**, so a reader can be sent to the exact spot.
  Our `evidence` table records the quote but not where it sits.
- `links(pos, source, dest, type, properties)` — typed, positioned, and with an
  open properties bag. Every child table cascades on delete.
- *Feeds:* the file-versus-node split as the answer to claim addressing;
  positions on evidence rows.

**dendron** — `dendronhq/dendron`

- Ships the artifact we have been circling: **the ontology layer as a committed,
  checkable file.** A `*.schema.yml` declares `id`, `desc`, `title`, `parent`,
  and `children` — which types may contain which — plus `namespace: true` for
  nodes that accept arbitrary children, and `template: {id, type}` binding a
  scaffold to a type.
- **Schemas compose**: `imports: [person]`, after which a child may reference an
  imported id (`person.public_persona`). That is what keeps a vocabulary from
  becoming one unmaintainable file, and it is the piece `hubi`'s migration story
  needs to be useful at scale.
- The result is MECE-checkable against the actual tree, which turns "are our
  categories mutually exclusive and exhaustive" from an aspiration into a lint.
- *Feeds:* the `type` vocabulary as data; per-type templates; the composition
  mechanism.

**zettelstore** — `zettelstore.de/z` (EUPL-1.2, Go)

- Design goals stated as goals: "longevity of stored notes, ease of installation
  and operation, **security by default**, and support for multiple user
  interfaces" — the last realized through "an application programming interface
  that offers a broader range of operations than the standard web-based user
  interface." That ordering is the right one for us: the API is primary and the
  web UI is a client of it, not the other way round.
- Internally hexagonal, and the project offers itself as a teaching example of
  the architecture.
- **The identifier tension, which this survey makes concrete.** Zettelstore uses
  an opaque 14-digit timestamp (`YYYYMMDDhhmmss`), and "the only restriction on
  zettel identifiers is that they consist of 14 digits" — immutable, meaningless,
  and therefore stable across any retitling. Dendron uses a hierarchical dotted
  path, which is legible but renames every descendant when a parent moves.
  `rebar` (agent-red) insists on descriptive names and never numbers, because
  numbers collide on merge and mean nothing across repositories. OKF recommends
  bundle-absolute paths as "stable when documents are moved within their
  subdirectory" — stable under a move, not under a rename.
  **Only the opaque-identifier answer survives both a move and a rename, and it
  pays for that with human legibility.** We have not chosen; the choice belongs
  with the deferred bundle-layout decision, and it is more consequential than it
  looks because links are what break.

**zettel** · **foam** · **depth=1**

- `hackstream/zettel` (GPLv3, Go) is the small end of the same idea: plain
  markdown in, static site and a graph UI out, with `yourbasic/graph` for the
  graph and goldmark for the markdown — the same parser `skillet` standardized
  on.
- `foam` (MIT, TypeScript) is the editor-resident surface — wikilinks, backlinks,
  and a graph inside VS Code. The only permissively-licensed project here, and
  the least liftable, being a VS Code extension.
- `depth=1` is a curated reading list on digital gardening rather than a tool.
  Its framing is a useful counterweight: a garden holds "half-finished thoughts
  that will grow and evolve", which is the opposite instinct to a corpus that
  gates everything at admission. Both are right for different material, and the
  quarantine tier is where that tension is supposed to live.

### Numbers Written as Words

Two small, maintained, MIT-licensed Go packages, both solving the same narrow
problem: a claim that says "three" where a stored value says `3`. That gap is
where a structured constraint and the prose it came from silently drift apart, so
closing it deterministically is what makes the drift check mechanical rather than
a thing a reviewer is asked to notice.

**numwords** — `rodaine/numwords`

- `ParseString` normalizes in place — "I've got three apples and two and a half
  bananas" becomes "…3 apples and 2.5 bananas" — which is the right primitive
  here. Normalizing the prose once catches every spelling variant at once, where
  rendering a value and searching for it catches only the spelling you happened
  to generate.
- Handles **floats and fractions**: "two and a half" → 2.5, "eight and three
  quarters" → 8.75, "a half" → 0.5. Also ordinals, and "nineteen eighty-eight" →
  1988\.
- No runtime dependencies at all; `testify` is test-only.
- One wart worth knowing: `IncludeSecond` is a package-level mutable toggle for
  whether "second" reads as a number. A global that two callers can disagree
  about is the defect class a shared kernel exists to prevent — set it once at
  initialization, and set it off, since "the second retry" becoming "the 2nd
  retry" is a false-match risk in exactly the text being checked.

**numberconverter** — `will-lol/numberconverter`

- Covers **both directions** — `Etoi` and `Itoe` — where numwords parses only.
- `FindAllEnglishNumberIndex` returns **positions**, which pairs naturally with
  evidence rows that record where a quote sits.
- Ships fuzz tests for both directions, and is explicit that there is "no
  prescribed style": "three hundred, fourty two million" parses, typo included.
- Two things keep it from being the pinned choice today: it is `int64` only, and
  timeouts, percentages, and ratios are the quantities a technical corpus
  actually carries; and its README still states "pre-release… some methods may not
  be correct", which is the author's own caution and a reason to wait for it to be
  withdrawn rather than to design around. Worth revisiting when it grows float
  support — the position-returning finders are genuinely better than what we
  chose.
- *Feeds:* the prose-versus-value agreement check; positions on evidence rows if
  it becomes the pinned package later.

## Methodologies and Documents We Were Inspired By

Everything above is code we read. This section is for the things that have no
repository: the organizational schemes, the two founding designs for a personal
knowledge system, and a body of systems-governance writing that turned out to be
about our problem under another name.

They earn a section because the code surveys kept arriving at the same three
questions — *what is the unit, where does it go, and who decides* — and none of
those questions is answered by a library.

### The Organizational Schemes

Three widely-used schemes, recorded because each answers a *different* question
and it is worth being explicit that we adopt none of them wholesale.

**PARA** — Projects, Areas, Resources, Archives. Tiago Forte's scheme from
*Building a Second Brain*. Its insight is not the four folders but the axis they
sit on: **organize by actionability, not by topic.** "We don't think in tags. We
think in context." A Project has a finish line, an Area is maintained
indefinitely and never completes, a Resource might matter later, an Archive is
finished but kept rather than deleted.

- *What transfers:* the Area/Project distinction is a real one we lack a word
  for, and Forte's observation that "a lot of stress comes from treating these
  like projects — they don't end" is the same shape as our `stale_after`
  question: a claim that is *maintained* is governed differently from a claim
  that was *concluded*. And **Archive-not-delete** is already our tier 0.
- *What does not:* PARA is a single-owner scheme keyed to one person's current
  attention. A shared corpus cannot place a document by what its author is doing
  this month, and the four categories are exclusive where our subjects must be
  many-to-many.

**CODE** — Capture, Organize, Distill, Express. Forte's companion *process* to
PARA's *structure*, and the closer analogue to what gnosis does: ingest, place,
compress to the reusable core, then produce something. Our pipeline is the same
arc with gates welded into each seam — `fetch` and `ingest` are Capture, the
ontology and the derived index are Organize, extraction and the promote gate are
Distill, and `ask`/`file` are Express.

- *What transfers:* naming Distill as a separate stage from Organize. Our
  quarantine tier exists because those two are not the same act, and having a
  name for the difference is worth something.
- *What does not:* CODE is a personal practice with no admission control, because
  its only contributor is trusted by construction. Ours is the opposite problem.

**LATCH** — Location, Alphabet, Time, Category, Hierarchy. Richard Saul Wurman's
claim that there are only five ways to organize information, and everything else
is a composite. It is the most useful of the three for us precisely because it is
a *closed enumeration*: it says that a knowledge base's presented navigation must
be one of five things, so the question "how should this be browsed" has finitely
many answers rather than infinitely many.

- *What transfers:* it is the checklist for SPEC §5.6's presented hierarchy. The
  storage path is opaque by design; the *view* has to pick from these five, and
  LATCH is why that choice is a small decision rather than an open one.
- *What does not:* LATCH is about presentation and says nothing about identity,
  provenance, or conflict. Treating it as a storage scheme is the mistake §5.6
  exists to avoid.

### Vannevar Bush's Memex — *As We May Think* (1945)

**Local copy:** `as_we_may_think_by_vannevar_bush.md`

§6 is a direct indictment of the thing every filing system does:

> "Our ineptitude in getting at the record is largely caused by the artificiality
> of systems of indexing. When data of any sort are placed in storage, they are
> filed alphabetically or numerically… **It can be in only one place, unless
> duplicates are used**; one has to have rules as to which path will locate it,
> and the rules are cumbersome. Having found one item, moreover, one has to
> emerge from the system and re-enter on a new path."

- **The many-to-many requirement is the founding complaint of the field**, not a
  preference of ours. A document belongs under several subjects; hierarchical
  placement forces a lie or a duplicate. Our answer — opaque identifiers, no
  meaningful path, subject association as a join — is the same answer, and this
  is where it comes from.
- **"One has to emerge from the system and re-enter on a new path"** is a
  requirement for `show` and `search`: a result must render its resolved outbound
  links inline, so following one does not mean going back and re-querying.
- **The trail is a first-class object we do not have.** Bush's associative trail
  is *named* ("when the user is building a trail, he names it"), *ordered*
  ("reviewed in turn… exactly as though the physical items had been gathered
  together to form a new book"), non-exclusive ("any item can be joined into
  numerous trails"), and *transferable* — he "photographs the whole trail out,
  and passes it to his friend for insertion in his own memex." We have pairwise
  links and nothing that names or orders a path through them. §8 makes that
  scaffolding the point of the whole essay: "a new profession of trail blazers…
  the inheritance from the master becomes, not only his additions to the world's
  record, but for his disciples the entire scaffolding by which they were
  erected." That is a fair description of what tribal knowledge actually is.
- **His link endpoint is the whole item, never a span.** The code spaces sit "at
  the bottom of each item," and his answer to *I want to say something about one
  specific point* is to make a new item and link it — "he inserts a page of
  longhand analysis of his own." He has marginal annotation and deliberately does
  not make it a link endpoint.
- **"The privilege of forgetting… with some assurance that he can find them again
  if they prove important"** (§8) is the argument for archive-and-retrieve over
  deletion, which is tier 0.

### Niklas Luhmann's Zettelkasten — *Communication with Zettelkastens* (1981)

**Local copy:**
`improved_translation_of_communications_with_zettelkastens_by_niklas_luhmann.md`
(Sascha's translation, 2023)

- **Content-free, never-reassigned identity is the load-bearing decision, and he
  says so.** "It is enough to assign a number to each note, place it so that it's
  easy to see, and never change it, and thus never change the note's place… This
  structural decision is exactly that reduction of complexity of possible
  arrangements that unlocks the creation of high complexity." A content-based
  order "would mean that you would have to adhere to a single structure forever
  (decades in advance!)." This is our opaque UUIDv7 and the fixed `/c/` prefix,
  and it is why content-addressed identity is the wrong instinct: an identifier
  derived from text cannot survive a typo fix.
- **He names the same problem Bush does, and gives the same answer.**
  "References allow solving the **multiple storage problem** without significant
  investment of labor or paper… you can solve the problem by placing the note
  wherever you want and create references to capture other possible contexts."
  Place it anywhere; link. Two independent inventors, one diagnosis.
- **Sub-note addressability, inscribed in the artifact.** He connects "anywhere,
  even to single words within a text," and the mechanism matters: "In the note
  itself, I use red letters or numbers to mark the place of connection." The
  address lives *in the slip*, not in the register — his register indexes notes.
  Neither founder supports an addressability that exists only in a regenerable
  cache, which is what a byte offset in a derived index is.
- **Atomicity is not his doctrine.** His slips run long and continue across
  57/12, 57/13; the one-idea-per-note rule is a later gloss. What he insists on
  is fixed identity for the *note* plus arbitrary-granularity connection points
  *within* it. That is the document/claim split, and it is an argument against
  collapsing them in either direction.
- **The orphan check has a better justification than tidiness.** "Each note is
  just an element that gets its value from being a part of a network of
  references… A note that is not connected to this network will get lost in the
  Zettelkasten, and will be forgotten by the Zettelkasten." He also rejects
  privileged notes outright — "you must give up the assumption that there are
  privileged places, notes of special and knowledge-ensuring quality" — which
  argues against any canonical-document-per-subject model.
- **Read versus merely collected is his own distinction.** "Books, articles, etc.
  that you actually have read should each get individual notes… This allows you
  in the longer run to distinguish what source you have actually read and what
  source you have just collected for later use." Our `sources_fetched.disposition`
  is the same distinction mechanized.
- **A register is mandatory, not optional.** "You have to put a search mechanism
  in place because you cannot rely on your numerical memory… it is therefore
  necessary to maintain a keyword index." Opaque identity *requires* a derived
  index; the two decisions are one decision.

Both scraped copies are also unintentional evidence for the ingest pipeline: the
Luhmann page ends in raw JavaScript, and the Bush page carries navigation chrome,
a newsletter form, and a comment widget. Boilerplate stripping is not a polish
item.

### Systems Governance — the `pmresearcher` Corpus

**Local copy:** `~/Documents/agent-orange/substack/pmresearcher` — 219 documents,
indexed by `index.md`

A project-management research archive that turns out to be about our problem in
another vocabulary. It contains no PARA-style scheme of its own and never
mentions Memex or Zettelkasten; what it supplies is the governance half — the
*who decides, on what signal, with what authority* that the PKM literature omits
entirely.

**Findings, not scores — independently derived.** *Schedule Variance as a Signal,
Not a Score* makes the argument SPEC §17 makes, from a different direction and
more sharply:

> "A control system that receives a feedback signal and takes no corrective
> action is not a control system. It is a measurement system wearing a control
> system's name badge."

- Its diagnosis of why metrics get filed rather than acted on is **corrective
  permission, not corrective capacity**: "The corrective options exist. What is
  absent is corrective permission… The PMO records the number because recording
  the number satisfies the compliance requirement and does not require anyone to
  make a difficult decision." That is precisely what an unadjudicated
  contradiction finding becomes if nothing forces it closed.
- **Trend, not level.** "A single reading in a single period tells you almost
  nothing about system behavior… You can tell from three to five consecutive
  periods." A worsening count means the corrective loop is broken; a *stable*
  count is the dangerous case, "deviation normalized rather than corrected." This
  is a real gap: gnosis reports findings at a moment and never reports whether
  the corpus is closing them.

**Why "findings are not failures" is load-bearing and not a nicety.** *The Wise
Governor* explains the mechanism: "They learn what the governance system rewards
and punishes. They route their signals accordingly." A tool whose honest report
of a problem is indistinguishable from its own breakage teaches contributors to
avoid running it. That is the argument for exit code 3 versus 1, and it is
stronger than the convenience argument we had.

- **"A framework makes judgment more precise. It does not replace it."** The
  cleanest statement of why gnosis minimizes the model and keeps human
  adjudication authoritative.
- **Requisite uncertainty** — "a confidence calibrated to what the system
  actually allows you to know" — is the name for what `findings.certainty` is for.

**Ashby's Law of Requisite Variety is the principle behind adjudication tiers.**
*The Logic of Institutional Failure* and *Designing for Variety* supply the
frame: a controller must have at least as much variety as the system it governs,
and the deficit closes by **amplifying** the governor's variety, **attenuating**
the governed system's, or both. Adding process is the wrong lever — "variety
amplification by volume, and it rarely works."

- SPEC §10.6 — adjudication authority scaling with the number of adjudicators —
  *is* variety matching, and now has a name and a literature.
- "**A deficit is not a failure. It is a design specification.**" The best
  one-line defence of reporting a gap rather than blocking on it.
- The 2008 case names the failure mode we should fear most: not a wrong model but
  **unwarranted confidence in the model** — "That epistemic confidence is
  structurally more dangerous because it forecloses the adaptive response before
  the problem is visible enough to trigger one." Applied here: a corpus that lints
  clean because its checks do not apply yet. Our derived applicability with
  mandatory skip reporting is the mitigation, and this is why it matters.

**Terminology is a governance problem, not a style problem.** *Words Matter*:
"people toss around terms like 'initiative,' 'milestone,' or 'epic' as if
everyone shares the same definitions. Most don't. And that gap? It breaks
communication, delays decisions, and undermines good planning." That is the
justification for `ontology.toml` being a committed, reviewed, checkable
artifact rather than a wiki page.

**Orientation over accumulation.** *On Being Oriented* and *Knowing What Belongs
at the Center*: "Being oriented has little to do with how much information
someone possesses… Information gains meaning only when it is placed correctly."
The unit of value is placement, not volume — which is the argument for
`index.md` being a curated map rather than a generated list, and for `search`
ranking rather than dumping.

**BLUF as information architecture.** *A Field Note on BLUF*: "BLUF is not just a
writing trick. It is an information architecture for decision making." Bottom
line up front — conclusion, then reasoning. This is what the `lead` column on a
claim is for, and it should be a lint check on normative types rather than a
convention.

**The reporting-to-learning distinction, with its own warning attached.** *From
Reporting to Learning* describes turning accumulated history into a pattern
library, and is the one piece here we should mostly *decline*: its architecture
is predictive scoring, which §17 rules out. What transfers is its structural
insight — "**Without defined decision pathways, predictive outputs remain
isolated metrics**" — and its list of what a decision layer must define: trigger
thresholds, accountable authorities, escalation pathways, documentation
standards, **override mechanisms**, and feedback capture. We have most of those;
override mechanism and feedback capture are the two we have not named.

### Measurement — Hubbard, *How to Measure Anything*

Douglas W. Hubbard, 2nd edition. **Local copy:**
`~/Documents/agent-orange/go-advice/Sources/data_science/HowToMeasureAnythingEd2DouglasWHubbard_book.md`

The book supplies what nothing else surveyed here does: a working definition of
measurement, a procedure for making a vague thing measurable, and — most
usefully — an account of why organizations reliably measure the wrong things.
Three of its ideas are load-bearing for us and one is an indictment we should
accept.

**The definition, and why intervals are not pedantry.**

> "Measurement: A quantitatively expressed reduction of uncertainty based on one
> or more observations."

A mere reduction, not elimination, counts. This is exactly what a finding is: a
`lint` run does not establish that a corpus is correct, it reduces uncertainty
about where it is wrong. But the sharper contribution is the corollary, which is
a better justification for §17's interval rule than the one we had:

> "The lack of reported error — implying the number is exact — can be an
> indication that empirical methods, such as sampling and experiments, were not
> used (i.e., it's not really a measurement at all)."

A bare number is *evidence that no measurement happened*. That reframes
`stats.Wilson` from a nicety into a tell: a corpus statistic reported without an
interval should be read as a count of something convenient rather than a
measurement of something that matters.

**The Clarification Chain is the missing procedure for admitting a subject.**

> 1. If it matters at all, it is detectable/observable.
> 2. If it is detectable, it can be detected as an amount (or range of possible
>    amounts).
> 3. If it can be detected as a range of possible amounts, it can be measured.

SPEC §20 leaves open "which subject keys the corpus tracks" and says the corpus
will nominate and the team will ratify — but states no test for ratification.
This is the test, and it retroactively justifies a requirement we had already
imposed for a weaker reason: **every subject in `ontology.toml` must declare a
`dimension`.** A proposed subject with no dimension has failed step 2, which
means it is not a subject at all — it is a topic, and topics belong to tags.

The two clarification-workshop questions are the same gate applied by hand:
**"What do you mean, exactly?"** and **"Why do you care?"** Hubbard's observation
about what happens next is the useful part — "it is interesting how often people
further refine their use of the term in a way that almost answers the measurement
question by itself." The one about mentorship ends with the participant saying "I
don't think I know," which is the honest outcome a promote gate should be willing
to produce.

**Measurement Inversion — the indictment we should accept.**

> "In a business case, the economic value of measuring a variable is usually
> inversely proportional to how much measurement attention it usually gets."

His explanation is the part that stings, because it describes our check registry:

> "First people measure what they know how to measure or what they believe is easy
> to measure. You probably know the old joke about the drunk looking for his watch
> in the well-lit street, even though he knows he lost it in the dark alley."

SPEC §12 lists roughly two dozen checks. The overwhelming majority are cheap and
deterministic — conformance, broken links, orphans, log format, filename drift,
archive orphans — and every one of them was chosen partly *because* it was
mechanizable. The checks that would actually change what a reader believes are the
expensive ones: does this claim contradict that one, is this claim still true, is
the evidence adequate to the scope claimed. A health report dominated by the cheap
checks is measurement inversion with a JSON schema, and the mitigation is not to
delete the cheap checks but to stop letting their count stand in for corpus
health.

His second reason lands too: "managers might tend to measure things that are more
likely to produce good news… Don't let managers be the only ones responsible for
measuring their own performance." That is the argument for the cold critic and for
`vac-protocol`'s reader-initiated challenge classes being someone *other than the
author's* instrument.

**Systemic error does not average out, so a bigger corpus does not fix a biased
gate.** Hubbard's account of Kinsey is the cleanest statement of this, via Tukey:

> "A random selection of three people would have been better than a group of 300
> chosen by Mr. Kinsey."

And the trap he names is one we are already in:

> "In business, people often choose precision with unknown systemic error over a
> highly imprecise measurement with random error."

An exact count of conformance violations is precise and systematically blind to
everything it does not check. A sampled critic estimate with a stated interval is
imprecise and unbiased. We currently report the first and not the second, and the
book's point is that the preference is backwards. **The Rule of Five** — "there is
a 93.75% chance that the median of a population is between the smallest and
largest values in any random sample of five" — is the cheap instrument that makes
the second option affordable: five randomly chosen claims, seeded so the sample is
reproducible, tells us something real about a corpus we cannot afford to critique
exhaustively.

**All three of his observation biases apply to us by name.**

- **Expectancy bias** — "seeing what we want to see." A critic handed a claim
  *together with the verdict the corpus already reached* is not an independent
  observation. Blinding it is the same move as a double-blind trial and costs
  nothing but prompt discipline.
- **Selection bias** — §10's conflict-candidate selection draws from claims that
  share a source, a link, or a tag-plus-rank-cut. That is a deliberately
  non-random sample, and it will systematically miss the contradiction between two
  claims that share nothing — which is precisely where a *surprising* conflict
  lives. Tukey's point is that enlarging a biased sample does not help.
- **Observer bias** — "the act of observing them causes them both to change
  behavior." Contributors who know which checks run will write to pass them. This
  is the same mechanism the governance corpus named independently ("they route
  their signals accordingly"), and two unrelated sources arriving at it makes it a
  structural property of the design rather than a worry about people.

**Confirmations worth recording, because they were arrived at without the book.**
Hubbard's four basic methods of observation are "follow its trail like a clever
detective… if it hasn't left any trail so far, add a *tracer* to it so it starts
leaving a trail." `fetch.jsonl`, `audit.jsonl`, and especially `miss.jsonl` — "why
the deterministic path did not decide" — are tracers added to make an otherwise
invisible thing countable. His Assumption 2, "you have far more data than you
think… the things you care about measuring tend to leave tracks," is the same bet
those files represent. And "the information value curve is usually steepest at the
beginning" is the argument for phasing: the first fifty documents will teach us
more about the data model than the next five hundred.

### Epistemology — *Knowledge: a Very Short Introduction*

**Local copy:** `~/Documents/agent-orange/go-advice/Sources/misc/knowledge_book.md`

The most directly load-bearing document surveyed, because it is about the thing
this project is: knowledge that arrives second-hand.

**The corpus is testimony, and the book poses our exact question.** "In the realm
of knowledge, many of our prized possessions come to us second-hand… What should
we think about resources like Wikipedia, where most articles have multiple and
anonymous authors?" Every claim in the corpus is something somebody said. Nothing
in it is perceived.

**gnosis is a local reductionist, and that is a commitment rather than a default.**
The book lays out three positions. *Global reductionism* says testimony is
generally reliable, so absent a warning sign you have standing reason to believe —
a corpus where admission is presumed and findings are exceptions. *Local
reductionism* demands specific positive reasons for **this** informant on **this**
topic: "Is this person an expert? Has she told you the truth in the past? How
plausible is her story now?" *The direct view* treats testimony as a basic source
needing no such support.

Every gate in this design is local-reductionist: provenance attaches per claim,
not per corpus; a credible source does not make its claims admissible; a warrant
is required per adjudication. The book also supplies the honest objection — "local
reductionism can sound very calculating: in practice, we don't often weigh the
reasons to trust someone" — and the reply that matters for us: it "is not a
descriptive theory about how we actually form our beliefs. It's a theory about the
conditions under which those beliefs deserve to count as knowledge." A corpus is
exactly the artifact for which the calculating version is appropriate, because it
outlives the conversation that produced it.

**The bucket brigade names the provenance chain, and "spills" name the failure.**
Lackey: "in order to give you a full bucket of water, I must have a full bucket of
water to pass to you… spills aside." A spill is a quote that no longer validates,
an extraction that dropped a qualifier, a link whose target moved. `quotecheck`
is spill detection.

**A source can transmit knowledge it does not itself hold, and this justifies the
relay.** Lackey's counterexample is a creationist teacher who diligently teaches
natural selection: "someone with less than a full bucket manages to pass on more
knowledge than she herself possesses." Applied here, this is the principled
defence of an extraction pipeline whose model believes nothing. What must be
checked of a relay is not sincerity or understanding but **spillage** — did the
text arrive intact — which is precisely what byte-exact quote validation checks
and precisely what a model cannot fake.

**The Wikipedia passage is this project's thesis, stated as epistemology.**

> "Over time, the entry as a whole has been vetted by so many people that the line
> about those mountain ranges is by now well-secured by the whole community of
> editors. This group may have succeeded in filling the bucket together, jointly
> generating an entry that is now able to provide the reader with knowledge… If the
> reliability of an informant is what counts, groups working together under the
> right conditions can outperform single authors."

That is collective accretion producing knowledge no contributor individually had.
And the condition is named in the same breath — a group entry supplies knowledge
"when its internal systems of quality control are working well." **gnosis is the
internal system of quality control.** This is the clearest statement of purpose
anything surveyed here has produced.

**Gettier is why a structural pass may never be called "verified."** A man reads a
stopped clock at the one moment it happens to be right. His belief is true and his
evidence is ordinary and reasonable, and yet: "It's not enough to add some
justification to true belief if the justification and the truth of the belief
aren't properly related to each other."

The corpus's central invariant — every quote appears byte-exact in an archived
source — is a **justification** check. It establishes that the claim is supported
in the way it says it is. It cannot establish that the quote *bears on* the claim,
and a claim can therefore be quote-valid, true, and still not knowledge for
exactly Gettier's reason. That is the "structurally valid, semantically wrong"
state, and this is why it is a principled gap rather than an implementation
shortfall: no strengthening of `quotecheck` closes it, because the gap is between
justification and support, not between weak and strong justification.

**Interest-relative invariantism is the stakes rule, arrived at independently.**
Lee locked the supply-room door half an hour ago. Asked by a colleague who left a
jacket inside, he says he knows it is locked. Asked by police hunting an armed
gunman, he says he does not know. Same evidence, same truth, same memory — and
both answers sound correct. What changed is the cost of being wrong.

This converges with two other sources surveyed here from different directions:
Hubbard's value of information scales with the decision, and Haskins holds that
"the sufficiency of evidence should be in proportion to the strength to which the
conclusion is being asserted." Three independent traditions arriving at *the
evidentiary standard is a function of the stakes* makes the load-bearing versus
peripheral distinction a principle rather than a convenience.

**Craig: the concept of knowledge exists to mark good informants.** "It's
imperative that we have a way of sorting out good informants, who can serve as our
eyes and ears, from bad informants, who are likely to lead us astray. Good
informants are identified as knowers." That is what a trust tier is for. The book
also supplies the limit — "knowers can sometimes be bad informants; knowers can be
secretive or deceptive" — which is why a tier is a signal and never access control.

### Critical Thinking — Haskins, *A Practical Guide to Critical Thinking*

**Local copy:**
`~/Documents/agent-orange/go-advice/Sources/soft_skills/a_practical_guide_to_critical_thinking_book.md`

Short, and the most directly *mechanizable* document here.

**Argument = Reason + Conclusion.** That is the claim structure already in the
schema: `lead` is the conclusion, `gnosis_evidence` and `sources` are the reasons.
Naming it that way makes the `lead` check obviously right rather than stylistic.

**Indicator words are a deterministic instrument, and we did not have one.** The
paper lists them: *since, because, for, for the reason that, as indicated by*
introduce a reason; *therefore, thus, so, hence, it follows that* introduce a
conclusion. These are lexical, closed, and language-specific — exactly the shape
of an operator pattern held as data with a test corpus rather than a regex in Go.
This gives claim segmentation and the `lead` check something concrete to work
from, without a model.

**The three-part argument evaluation is a gate we have two-thirds of.**

1. **Are the assumptions warranted?** A warranted assumption is "known to be true"
   or "reasonable to accept without requiring another argument to support it." Our
   `gnosis_warrant` field turns out to be named after exactly this.
2. **Is the reasoning relevant *and* sufficient?** "Relevance is the quality of the
   reasoning, sufficiency the quantity." And the rule that matters:
   **"the sufficiency of evidence should be in proportion to the strength to which
   the conclusion is being asserted."** *John definitely bought the painting* and
   *John may have bought the painting* need different evidence for the same
   photograph. This is the **coverage** check we have recorded as missing — the
   principled basis for asking whether a claim's evidence supports the scope it
   claims, rather than merely existing.
3. **Has relevant information been omitted?** "A cogent argument is one that is
   complete, in that it presents all relevant reasoning, not just evidence that
   supports the argument." The remedy given is "seek opposing arguments on the
   subject," which is the cold critic's job and a second argument for the
   random-sample conflict pass: a selector that only surfaces related claims cannot
   surface an omission.

**Source evaluation as disqualifiers, not a score.** Four questions: does the
source have the qualifications; does it have a reputation for accuracy; does it
have a motive to be inaccurate; is there reason to question its integrity. "If any
of the answers are 'no' to the first two or 'yes' to the last two, the critical
thinker should be hesitant." A conjunction of disqualifiers, never a weighted
composite — which is how credibility signals should read.

**"Perhaps the most important question the critical thinker should ask of any
statistical result is: were the samples taken representative of the entire target
population?"** A third independent arrival at the sampling point.

**Degrees of certainty, and "I don't know" as a legitimate answer.** Intellectual
humility means "adhering tentatively to recently acquired opinions" and thinking in
"degrees of certainty or shades of grey" rather than right and wrong — "sometimes
'I don't know' can be the wisest position to take on an issue." That is `status: draft`, `requisite uncertainty`, and the `deferred` finding state, each of which
now has a reason beyond convenience.

**Inductive arguments never prove.** "No matter how strong the evidence in support
of an inductive argument, it will never prove its conclusion by following with
necessity." Nearly every claim a corpus holds is inductive, which is the general
form of the rule that a clean pass licenses far less than it appears to.

**The hindrance tables draw the deterministic line for us.** Four tables list
roughly forty named hindrances, and they split cleanly on whether a machine can
find them:

- **Table 2, Use of Language** — ambiguity, vagueness, hedging and weasel words,
  meaningless comparisons, assuring expressions, doublespeak jargon, gobbledygook.
  These are **lexically detectable**. A word list plus a test corpus finds them,
  which makes them candidate `lint` checks in the same family as a Unicode
  confusable check.
- **Tables 1, 3, and 4** — confirmation bias, post hoc, begging the question, false
  dilemma, appeal to authority, sunk cost, positive outcome bias. These are
  **reasoning** failures and are not deterministically detectable. They belong to
  the critic or to nothing, and a lexical check that claimed to find them would be
  the model-based bias detector this family already refused.

**On tone:** "Thinking critically is not thinking negatively with a predisposition
to find fault or flaws. It is a neutral and unbiased process for evaluating
claims." Worth keeping in front of whoever writes the critic prompt.

### Three Documents with Less to Offer, and Why

Recorded so their absence from the design reads as a decision.

**`Second_Brain_Setup_Guide.md`** operationalizes PARA into four folders and a
maintenance cadence. The folders add nothing beyond the PARA discussion above, but
the cadence is a gap it exposes: "review Projects weekly and move completed ones to
Archives; check Areas monthly." **Every review trigger in our design is
event-driven** — an ingest, a conflict, a pull request — and none is periodic. A
corpus with no scheduled review accumulates claims nobody has looked at since
admission, and `stale_after` only catches the ones somebody thought to date. Its
other rule, "avoid over-tagging; PARA thrives on simplicity," is an argument for
the vocabulary starting empty.

**`problem_solving_tools_book.md`** (Nickols) catalogues 24 tools in three
families: visualizing problem structure, displaying data, and problem-solving
technique. Almost all are for *generating* and *presenting* information rather than
admitting it, so there is nothing to lift wholesale. Three are worth naming:
**Affinity Diagram** is bottom-up emergent categorization, the honest counterpart
to nominating subjects from collisions rather than declaring a taxonomy;
**Five Whys** is the depth a warrant `rationale` should reach, since a first
answer is usually a restatement; **Pareto** is measurement inversion's cousin —
most findings come from few causes, which is worth knowing before adding checks.

**`critical_thinking_in_world_book.md`** is the weakest of the five for our
purposes. It is a popular treatment of individual cognitive bias — interoception,
bystander effect, heuristics, confirmation bias, halo effect, priming, social
proof — and where it overlaps Haskins's tables it adds narrative rather than
mechanism. It is also, unintentionally, a small argument for admission gates: its
chapter on bad science asserts that pseudo-science is "usually spearheaded by big
pharmaceutical corporations," which is exactly the kind of unsourced causal claim a
corpus should hold to a warrant. Noted, not adopted.

### Instrumentation — Hamming, *The Art of Doing Science and Engineering*

**Local copy:**
`~/Documents/agent-orange/go-advice/Sources/misc/Hamming-TheArtOfDoingScienceAndEngineering_book.md`

Chapters 27 (*Unreliable Data*) and 29 (*You Get What You Measure*) are the most
directly useful things read in this whole survey, because they are about the
instrument rather than the subject — and gnosis **is** an instrument.

**The question to ask of gnosis itself.** Hamming, shown the rig for life-testing
vacuum tubes destined for a submarine cable:

> "Why do you believe the test equipment is as reliable as what is being tested?"
> The answer I got convinced me he had not really thought about it.

gnosis is test equipment for a corpus. Its checks are less reliable than their
determinism advertises, and a planted-defect self-test is the only answer to
Hamming's question we have.

**The low-variance trap, which is a live hazard for us.** His account of how
laboratory accuracy gets overstated:

> "You fine tune the equipment. How? By adjusting it so you get consistent runs!
> In simple words, you adjust for low variance; what else can you do? But it is
> this low variance data you turn over to the statistician… you supply the low
> variance data, and you get from the statistician the high reliability you want to
> claim!"

Every threshold in this design — rank cuts, hop limits, `MinPassageWords`,
staleness defaults — lives in `standards/` so that runs are reproducible. Nothing
prevents those knobs being turned until the corpus reports quietly, and a corpus
tuned to lint clean is the low-variance rig exactly. Determinism guarantees the
same answer twice; it says nothing about whether the answer is about anything.
**A threshold changed in a direction that reduces findings has to be recorded as
such**, or the standards file becomes a place where inconvenient checks go to die.

**Eddington's fishermen are the best statement of selection bias we found.**

> "They examined the size of the fish they caught and concluded there was a minimum
> size to the fish in the sea. The instrument you use clearly affects what you
> see."

The conflict-candidate selector determines which contradictions exist *as far as
the corpus knows*. That is the net, and its mesh is "shares a source, a link, or a
tag."

**Accuracy versus relevance — measurement inversion, stated better than Hubbard
states it.**

> "Accuracy of measurement tends to get confused with relevance of measurement,
> much more than most people believe. That a measurement is accurate, reproducible,
> and easy to make does not mean it should be done; instead a much poorer one which
> is more closely related to your goals may be much preferable… in school it is
> easy to measure training and hard to measure education."

Substitute *conformance* for training and *whether the claim is true* for
education and the sentence is about §12 without alteration.

**The IQ artifact, which is a warning about a mechanism this project just
adopted.** Describing how intelligence testing manufactures its own distribution:

> "Those questions which show an internal correlation with others are kept and
> those which do not correlate well are dropped… As a result it is observed
> intelligence has a normal distribution in the population! Of course it has, it was
> made to be that way!"

We have just specified a `--check-value` report intended to identify checks worth
retiring. Retiring the checks that *disagree with the other checks* is precisely
this construction, and it would manufacture the appearance of a coherent registry.
The only admissible retirement criterion is that **nobody acts on the findings** —
never that a check dissents.

**Careful estimates combined with wild guesses.**

> "Much of the reliability of the engineering guesses was transferred to the sum,
> and the uncertainty of the salesman's guesses was ignored… Careful estimates are
> combined with wild guesses, and the reliability of the whole is taken to be the
> reliability of the engineering part."

This is the two-provenance-class problem with its failure mode named. A claim
resting on one byte-exact quote and one adjudicated assumption will be read at the
credibility of the quote. **A composite must be reported at its weakest link, not
its strongest.**

**Definitions drift silently, which is why deprecation must be announced.**

> "The definition of what is being measured is constantly changing… Definitions have
> a habit of changing over time without any formal statement of this fact."

A subject key whose meaning shifts quietly invalidates every comparison ever made
under the old meaning, and nothing in the corpus would show it. He also names the
counter-pressure honestly — "better to have an irrelevant indicator than an
inconsistent one, so they claim" — which is the real tension a soft-deprecation
path exists to manage.

**Averages over heterogeneous populations, which is a better argument than the one
we were using.**

> "Averages are meaningful for homogeneous groups… but for diverse groups averages
> are often meaningless."

Our stated reason for refusing a corpus-quality score was that a score is
subjective and goes stale. The stronger reason is arithmetic: a corpus spans types,
subjects, sources, and provenance classes, so a number averaged across it describes
no part of it. It is not a bad summary — it is a summary of nothing.

**Two more, briefly.** "Small samples carefully taken are better than large samples
poorly done" is a third independent arrival at the sampling point. And the dynamic
range argument from information theory — "you have the most information when all
the grades are used equally" — implies something checkable: **if nearly every
finding is a warning, severity carries almost no information.** A severity
vocabulary used unevenly is a low-entropy channel.

**Finally, the rating-system dynamic, which is about contributors rather than
data.** "If in a rating system everyone starts out at 95% then there is clearly
little a person can do to raise their rating but much which will lower the rating;
hence the obvious strategy of the personnel is to play things safe." A corpus whose
only visible signal is *problems found* rewards contributing less and claiming
less. Reporting what the corpus gained alongside what it got wrong is not
cheerleading; it is the counterweight that keeps the incentive from inverting.

### Analysis Discipline — *The Art of Data Science*

**Local copy:**
`~/Documents/agent-orange/go-advice/Sources/data_science/art_of_data_science_book.md`

Peng and Matsui. Two ideas transfer cleanly.

**The epicycle: set expectations, collect, compare — and when they disagree, know
which of two things to fix.** "Either your expectations were wrong and need to be
revised, or the check was wrong and contains an error." The `--check` idiom
throughout this design is expectation-comparison, and reconciliation (§5.1.2)
already enumerates which side to fix per case. The framing is a good check on
whether a new check knows what it expected before it looked.

**The sharp hypothesis, which gives us a real constraint on constraints.**

> "The expectation of a $30 meal is… a sharp hypothesis because it states something
> very specific that can be verified with the data. If our original expectation was
> that the meal would be between $0 and $1,000, then it's true that our data fall
> into that range, but it's not clear how much more we've learned."

**A constraint so wide it cannot fail is not a constraint.** `retry.max_attempts is between 1 and 100` is well-formed, parseable, indexable, and worthless — and it
would pass every check the design currently specifies. This applies to derived
constraints and to the strength/sufficiency check alike.

**Five characteristics of a good question** — of interest to the audience; not
already answered; stems from a plausible framework; answerable; specific — are the
gate for `ask`, and the second is what search-before-ask is *for*. The third
carries a warning we need: their counterexample is asking whether pepperoni sales
correlate with yogurt sales, where "if you do find they are correlated, many
questions are raised about the result itself." A randomly paired conflict candidate
is a pepperoni-yogurt pair by construction, so the random-sample pass needs a
plausibility filter or it will manufacture coincidences at a steady rate.

Their specificity example — from "is eating a healthier diet better for you?" to
"does eating at least 5 servings per day of fresh fruits and vegetables lead to
fewer colds?" — is the clarification chain arriving from a third direction.

### Presentation — Knaflic, *Storytelling with Data*

**Local copy:**
`~/Documents/agent-orange/go-advice/Sources/data_science/storytelling_with_data_book.md`

One distinction earns its place: **exploratory versus explanatory**.

> "Exploratory analysis is what you do to understand the data… like hunting for
> pearls in oysters. We might have to open 100 oysters to find perhaps two pearls…
> Too often, people err and think it's OK to show exploratory analysis (simply
> present the data, all 100 oysters) when they should be showing explanatory… You
> are making your audience reopen all of the oysters!"

That is the `index.md` versus `search` split exactly. `search`, `graph`, and
`critic` are exploratory instruments; `index.md` and a `show` rendering are
explanatory artifacts, and the temptation she names — showing everything "as
evidence of all of the work you did" — is precisely the failure mode of a health
report that lists two dozen checks.

### Two Documents with Nothing to Lift

**`sqlite_tutorial_book.md`** is an introductory SQL reference: installation, DDL
and DML syntax, joins, basic clauses. Across 32,000 words it mentions FTS5,
`WITHOUT ROWID`, `PRAGMA user_version`, and `EXPLAIN QUERY PLAN` ten times
combined, and the derived-index design already depends on all four in ways the
tutorial does not cover. Two small things are worth knowing: the `sqlite_master`
table is a way for `doctor` to verify schema shape against what the migrations
should have produced, and its limitations list is a reminder that `ALTER TABLE`
cannot drop a column — which is why migrations here are append-only.

**`mathematics_for_machine_learning_book.md`** (130,000 words of linear algebra,
analytic geometry, matrix decomposition, vector calculus, probability, and
optimization) has no application to a tool that runs no model and computes no
gradient. Its probability chapters cover ground `skillet/stats` already occupies
via the specific estimators we need. Recorded so its absence is a decision rather
than an oversight; if a semantic reranker is ever enabled (§11, optional), the
inner-product and projection material becomes relevant and not before.

### The LLM-Wiki Field — the `agent-purple` Survey

**Local copies:** `~/Documents/agent-purple` — 19 implementations and 9 documents.

This is the first survey where gnosis is not alone. A dozen projects here implement
Karpathy's pattern, several arrived at the same architecture independently, and two
are close enough to be siblings. The convergences are worth more than the
differences, because a design four strangers reached separately is a design the
problem forces.

**What the field converged on, with no coordination.** Markdown plus git as the
substrate. A deterministic CLI beside an agent, not inside it. Lint as a
first-class operation. The index as a derived cache. Search over browsing. Every
one of those is in this specification for reasons argued from first principles,
and every one shows up in projects that never read it.

#### `canopy`, and a Principle Taxonomy Worth Stealing

A Go binary for markdown wikis, built on *"판단은 LLM이, 불변식은 코드가"* —
**judgment to the LLM, invariants to code**, which is this project's thesis in nine
words. Its `docs/philosophy.md` lists eleven principles, and several are ours
verbatim: *derivatives are never hand-edited*, *distinguish source, state, and
cache* (its three tiers to our four), *code generates candidates and the LLM
judges* (§6.2 plus §10.3, exactly), *distinguish assertion from conjecture*.

The contribution is not any principle. It is that **each one is tagged `[code]`,
`[convention]`, or `[code+convention]`** — a stated axis for *what enforces this*.
Our specification is full of MUSTs and says nowhere which are machine-checked and
which rest on people behaving. Two examples make the gap concrete: canopy marks
*raw/ is immutable* as `[convention]`, where §4.1 makes tier 0 append-only and
content-addressed precisely because "immutable is an assertion nothing currently
enforces" — we are stricter and could say so. Conversely several of our own rules
are convention wearing a MUST, and a reader cannot tell which.

Two mechanisms we lack outright: **`bridge`**, which surfaces similar-but-unlinked
pages as candidate connections, and a **rediscovery loop** (`resurface` with
👍/👎/😴 feedback feeding later candidate selection) that answers §14.3.1's
periodic-review gap with something better than a date comparison. And its gap log —
"searches that found no answer accumulate, becoming page-creation candidates" — is
`miss.jsonl` and the `gap` check, arrived at independently.

#### `mnemo_wiki`, Our Nearest Sibling in OKF

The closest sibling: an LLM wiki **stored in OKF**, with a command surface nearly
identical to our Phase 1 — `init`, `new`, `lint`, `index`, `reindex`, `search`,
`show`, `links`, `move`, `log`, `validate`. Its statement of the division is
crisper than ours: **"It never calls a language model itself… The tool is the
hands; the agent is the head."**

Where it differs is instructive. mnemo_wiki puts the agent's routines in a
`CLAUDE.md` inside each wiki — the rules, the frontmatter each page type needs, the
step-by-step for adding a source. That is our §5.7 schema document, and theirs is
load-bearing where ours is generated. The comparison worth making later is whether
a corpus's conventions belong in a file the agent reads or in a tool that refuses
the write.

#### The Deterministic Half, Twice: `kb-lint` and `wiki-compiler`

`kb-lint` is a Python linter for markdown knowledge bases with seven checks:
`links`, `frontmatter`, `orphans`, `structure`, `content`, `index`, `consistency`.
Its check table carries a column ours does not: **`Auto-fixable?`**. We have that
axis — `finding.Action` is `automatic`/`guided`/`human` — and §12's table does not
show it, so a reader cannot see at a glance which findings a tool can close. That
is a free improvement.

Two of its content checks we lack: **`{{PLACEHOLDERS}}`** left in a page, and
**empty sections**. Both are lexical, both are the kind of thing an agent leaves
behind, and neither needs a model. Its `index` check is a deliberate divergence
rather than a gap — it validates `_index.md` against the file tree and auto-fixes
it, where §5.6 makes `index.md` a curated map precisely so it is *not* a generated
listing.

`wiki-compiler` is the thesis stated as a pipeline: `Raw Notes → Extractor → Graph → Rewriter → Linter → Compiled Wiki`, pure Python, "no LLM calls, no embeddings, no
dependencies", published under the title *LLM Wikis Are Over-Engineered*. It is
what remains when the model is removed entirely, and it is a useful floor: anything
gnosis asks a model to do that this does deterministically is a place we have not
tried hard enough.

#### `tome`, and an Admission Test We Lack

An agent-memory vault with constraints "enforced by the CLI and `tome lint` rather
than trusted to prose" — again the same instinct. Its contribution is the criterion
for what is worth writing down at all: **durable, non-obvious, not trivially
derivable.**

gnosis gates on evidence, conformance, and identity, and has no test for whether a
claim is *worth having*. A well-sourced triviality passes every check we specify.
That is a real hole, and it is the kind that fills a corpus with true, cited,
useless pages until search stops being worth running.

#### `LLM Wiki V3: Segmentation`, and Its Librarian Pattern

A concept document in the V1/V2 lineage, and one idea in it is directly applicable.
When an agent searches the corpus for its own context, "as the agent searches… its
context fills. By the time it returns, it may have drifted from the original
objective… The team inherits the agent's drift."

The **librarian pattern** separates retrieval into its own context: the caller
states an objective and waits, the librarian searches and returns pre-scoped
material, and the caller reads it fresh alongside the objective. `gnosis ask`
already emits a prompt with retrieved context, so we have the mechanism — what we
lack is the *reason*, stated. It is not token economy. It is bias prevention, and
that framing changes what the command should and should not include.

Its general claim is also worth recording: the failure modes of a growing wiki "are
not data problems, they are segmentation failures… the answer is not a stronger
foundation, it is more foundations — each one narrow."

#### Shirky's *Ontology Is Overrated*, Our Honest Counter-Argument

Everything above agrees with us. This does not, and it is the strongest available
objection to §5.8's controlled vocabulary, so it belongs here rather than in a
footnote.

Shirky's case is that categorization schemes impose a single true home on things
that have none — *"there is no shelf"* — and his exhibit is Yahoo's directory,
where *Books and Literature* appears under *Entertainment* with an `@` marking it
as filed "for your convenience" while it "really" lives elsewhere. To which, he
says, one can only respond: "What's real?" His alternative is the link and the tag:
free-form labelling, no categorical constraint, value extracted from big messy data
sets.

**Note what this shares with Bush and Luhmann.** Three writers across sixty years
naming the same defect — a thing can be in only one place — and Shirky draws the
opposite conclusion from ours: abandon the controlled vocabulary rather than
enforce it.

The reconciliation is in the conditions, and Shirky supplies them himself. His
argument holds where the corpus is large, open, uncoordinated, and its users
share no purpose — the Web. `ontology.toml` governs the opposite case: a small
corpus, a coordinated team, and a purpose that *is* comparison. A subject key
exists so §10 can decide whether two claims disagree, and free-form tags cannot do
that at any scale. So the vocabulary stays enforced (§5.8.2.1), and Shirky is the
reason it stays **small** — subjects start empty, accrete only on a real collision,
and anything that cannot name a dimension is a tag rather than a subject. He is
also the reason tags exist beside subjects at all: the free-form layer is where
everything that is not a comparison lives.

#### *What to Keep, What to Skip*, Our Only Field Evidence

**Local copy:** `what_to_keep.md` (Eugeniu Ghelbur, *The AI Operator*)

Everything else surveyed here is a design. This is a **report from months of daily
use**, and it is the only document in the field that says which of the proposed
features died in practice. It deserves more weight than its length suggests, and
some of what it says is uncomfortable for this specification.

**The split it found.** "v2" clusters into governance features — confidence,
supersession, forgetting — and infrastructure — hybrid search, hooks, quality
scoring, tiered memory. His verdict after months: *"the split between them is
exactly the split between what works and what is overkill."* Governance earned its
place; infrastructure did not.

**Kept, and it validates four of our decisions:**

- **Confidence as a marker, not a score.** `stated` / `high` / `low`, one word per
  fact. *"This is the single highest-value v2 idea and it requires zero
  infrastructure."* And the sharp part: **"a `stated` claim gets treated as a
  quote, not a truth."** That is exactly what our sourced-versus-adjudicated split
  is for, and it is a better sentence than any in §10.4. v2 wanted decaying numeric
  confidence (0.85, reinforced on access); we refused numeric scores in §17 and
  kept `certainty` as HIGH/MEDIUM/LOW, which is his version, arrived at
  independently.
- **Supersession as reconciliation, not a data structure.** *"You do not need a
  formal supersession chain. You need the AI to actually resolve the conflict
  instead of hoarding it."* We have `gnosis_supersedes` as a formal edge — worth
  keeping, because a team corpus needs the audit trail a single-user vault does
  not, but his point stands: the structure is not the value, the resolution is.
- **Lifecycle hooks as scheduled agents.** *"You do not need an event-bus
  abstraction. You need cron and a rules file."* That is §14.3.1's periodic review,
  and it names the mechanism.
- **Quality-scoring engines are weight.** Independent agreement with §17.

**Skipped, and this is the challenge:**

> "At wiki scale, you do not have a retrieval problem. A curated wiki is 50,000 to
> 100,000 tokens. That is small. Grep plus read finds the right note faster and more
> predictably than an embedding lookup… Bolting embeddings onto a 200-note vault is
> solving a problem you do not have."

We already keep the semantic reranker optional (§11), so this is not a
contradiction. But it is the only *measured* statement in the survey about the
scale these systems actually run at, and it points at something larger than
embeddings: **gnosis may be over-engineered for the corpus it will hold.** At 200
notes, a SQLite index with FTS5, four tiers, claim-level addressing, and three
adjudication tiers is a great deal of machinery.

The honest reply is that his evidence comes from a single-person vault, and every
mechanism here that looks heavy exists for something a single-person vault does not
have: contradictory sources admitted by different people, contributors of mixed
skill, and a corpus that carries the team's authority rather than one person's
memory. But that is a *reason to expect* the machinery to pay, not evidence that it
does — and his caveat cuts both ways, because he is equally clear that scale is
what decides. It belongs on the record as the strongest available argument that
this design is too big, to be answered with a real corpus rather than with prose.

**One thing he says that we should simply adopt:** *"The point of a forgetting curve
is not the math. It is that something deletes."* §14.3.1 reports unreviewed claims
and explicitly never invalidates, on the grounds that an old claim is not a wrong
claim. He is describing the failure that rule permits — notes that accumulate become
a graveyard — and he is right that reporting is not the same as pruning.

#### `stigmergy` — Gates, Refusal, and Minting

**Local copy:** `stigmergy/`

The most operationally serious project surveyed, and the only one designed for a
team from the start. Three ideas worth taking.

**"A model writes; code decides."** An agent drafts a page; **eight deterministic
gates** run over the resulting *diff* — zone, binary-page, body-rewrite, secrets,
pii, frontmatter, contract, anchoring — and *"the diff those gates approved is
provably the diff that lands."* Their summary is the sharpest statement of this
family's thesis: **"A model can be argued with. A gate cannot."**

The property in that sentence is one we do not state. Our promote gate checks
content and then a write happens; nothing says the checked artifact and the
committed artifact are the same bytes. That is a time-of-check-to-time-of-use gap,
and it is exactly the gap a gate exists to close.

Note also `body-rewrite` as a *gate*: §6.3 separates mechanical accretion from
gated synthesis as two operations, and stigmergy enforces the same split as a check
on the diff, which is harder to route around.

**"An honest refusal beats a confident guess."** Their read path — a single MCP
server — *"answers questions with sources, and refuses when it cannot support an
answer,"* and: **"A system that never refuses is the failure, not the success."**
gnosis has `blocked` and `needs_human` on the *write* path and nothing equivalent on
the read path. `ask` emits a prompt with retrieved context and never declines.

**Identity minting is human-gated.** *"A name the registry does not know makes it
ask ONCE and park until a steward answers."* The capture waits rather than guessing.
We take the opposite line — UUIDv7 assigned automatically at admission (§5.1) —
and it is worth being clear that these are different problems: they gate *which
entity this is*, we gate *whether this claim is supported*. Their approach would
catch the duplicate-concept case §4.6.1 leaves to a post-merge check.

#### `akbp` — Protocol Discipline and Graded Conformance

**Local copy:** `akbp/` — rohitg00's own implementation of the v2 gist.

Two mechanisms worth stealing.

**Review-gated writes as a protocol, not a workflow.** `dry_run: true`,
`approved: true`, `approval_required` — an agent previews a decision, the same write
is *rejected without approval*, and applied after it. The core rule: **"agents can
propose memory, but durable writes are review-gated."** Our promote gate is a
command; theirs is a property of every write call, which is harder to bypass.

**Conformance has levels.** Their demo *"runs level 3 conformance"* — a graded
suite rather than a boolean. Our §11 treats OKF conformance as pass/fail, and a
graded ladder would let a producer state how far it conforms and let a consumer
require a level. Also worth noting: `export-check`, `import-check`, and
`import-apply` as distinct verbs, which makes bundle round-tripping testable.

#### `wenlan` — Which Parts of a Page the Machine May Rewrite

**Local copy:** `wenlan/`

Its refresh model makes a distinction we do not: *"Wenlan rebuilds it from current
Sources and Memories, records the revision, and **stages changes to human writing
for review**."*

So a page has machine-owned regions and human-owned regions, and a refresh may
rewrite the first while the second must be staged. §6.3 splits accretion from
synthesis at the level of the *operation*; wenlan splits at the level of the
*region*. That is finer and it is the version that survives an agent refreshing a
page a person has edited — which is the common case and one our design currently
resolves by gating the whole rewrite.

Its two linked lifecycles — Sources and Memories, "linked without collapsing them
into one layer" — are our tier 0 and the corpus, and the vocabulary is clearer.

#### `piekbs` — Four Concrete Findings

**Local copy:** `piekbs/` — 87 Go files, `modernc.org/sqlite`, FTS5, MCP.

The closest implementation to ours in language and stack, and the one that
produced specific findings rather than principles.

**It solved the snippet problem the way we guessed we would have to.** Their commit
*"perf(search): avoid slow FTS5 snippet generation"* drops FTS5's `snippet()`
entirely and computes an excerpt in Go: parse the markdown, take
`ParseMarkdown(content).Content`, collapse whitespace, find the first keyword, and
window 120 characters around it. Our own note on this recorded a tension — strip
link syntax at index time and slugs become unsearchable, strip at render time and
FTS5's offsets no longer match what is shown — and concluded that re-deriving the
snippet rather than offset-mapping it was "probably right". They did exactly that,
and had a second reason we did not have: FTS5's `snippet()` was measurably slow.
Independent confirmation of an untested guess is worth more than the guess.

**Per-document `schema_version` — a gap we have.** Their `documents` table carries
one, and `FindOutdatedNotes` reports every document written under a version older
than the current one. That is not staleness against an upstream source, and it is
not index drift. It is *a document written under an older version of the corpus's
own conventions*, and we have no way to express it.

The gap is about to bite. §5.5.1 introduces `gnosis_claims` frontmatter that no
document written in Phase 1 carries, so on the day extraction lands, every existing
document predates the format — and nothing records which. The same applies to any
future change in required frontmatter or in `ontology.toml`'s shape.

**Their tokenizer is `trigram`; ours is `porter unicode61`.** Theirs is a Chinese
team with a Lark importer, and the choice follows: porter stems English and
unicode61 splits on word boundaries, which is close to useless for a language that
does not put spaces between words. Trigram handles CJK and substring matching and
gives up stemming.

Neither choice is wrong, but ours is an *assumption* rather than a decision — §5.5
justifies the `tokenchars` and says nothing about the corpus being English. It
should say so, because the day someone ingests a Chinese source into a corpus
tokenized with porter, search will quietly stop finding it.

**Two divergences where we are on the better side, and one where they are.**
Their `links.target_doc_id` is `NOT NULL` with a foreign key, so a link to a page
that does not exist yet cannot be stored — OKF §6.1's "not-yet-written knowledge"
is unrepresentable, and our nullable target plus retained `href` is the more useful
model for a corpus that grows. Their `links.confidence REAL DEFAULT 1.0` is the
per-edge score §17 refuses, and refuses for reasons that apply here too.

Against that: their frontmatter validator distinguishes an **absent** field from an
**explicitly empty** one — *"`sources: []` is considered present; it means 'no
sources' rather than 'field missing'"* — which is our own absent-versus-empty
discipline (`Unchecked` versus `Missing`, `HasLog` versus `LogLines`) applied
somewhere we have not yet applied it. Phase 1 reads only scalar frontmatter, so
this is guidance for when `gnosis_evidence` and `sources` arrive rather than a
present defect.

Finally, a false alarm worth recording because checking it was cheap: their whole
`kb` package is gated behind `//go:build fts5` and their CI wants
`CGO_ENABLED=1`. We use FTS5 with no build tag. `modernc.org/sqlite` v1.57 has no
`fts5` tag at all — FTS5 is compiled in unconditionally — so their constraint looks
like a convention carried over from a CGo driver, and our pure-Go, no-tag posture
is correct.

#### Nearest Architecture (`kvt`), and a Library That Exists (`okf`)

`kvt` is the closest thing to gnosis anyone else has built: markdown as the source
of truth, a **derived SQLite index with FTS5, links, and frontmatter fields**,
every write committed to git, exposed over REST and MCP. Its one visible divergence
is that it *regenerates* `index.md` as a service-owned file where §5.6 keeps it a
curated map — the same disagreement `kb-lint` has with us, now twice.

`okf` (skosovsky) is a Go OKF toolkit with `bundle`, `validator`, `graph`, `store`,
and `store/fs` packages and transactional mutations. It is a genuine build-versus-
adopt question for `internal/okf`, and the reason to keep ours is narrow but real:
our `Parse`/`Render` retains the frontmatter block **verbatim** so a round trip is
byte-exact, which a general-purpose library that re-encodes YAML cannot offer. Worth
re-checking if that assumption ever stops holding.

#### Noted, Not Adopted

`chroma` and `milvus` are vector databases; relevant only if §11's optional
reranker is ever enabled, and both are far heavier than a corpus of this size
justifies — a judgement the practitioner report above turns from an intuition into
a measured one. `synthadoc` and `swarmvault` compile multi-format sources into
markdown wikis with linting, and are ingestion prior art to read when Phase 2 needs
adapters rather than now. `expo-llm-wiki` is a second OKF implementation, useful as
a conformance cross-check because two independent readings of one spec is how the
ambiguities get found. `cq-gitstore` is the most interesting of the small ones: it
makes a git repository of OKF markdown a `Store` adapter for mozilla-ai's `cq` SDK,
which is evidence OKF is becoming an interchange format with consumers that are not
wikis at all. `mnemos` is agent memory with citations, in the same family as
`memory-os`. `logseq` and `Zettlr` are end-user PKM applications — the graph view and
the outliner are worth looking at when §13's viewer is built, and neither
contributes to the model. `memory-os`, `jarvis-vault`, and `LLM-wiki-dev` are agent
*session* memory rather than a curated corpus: the distinction is that they capture
what an agent learned, where gnosis admits what a team has checked. `open-knowledge`
is an editor. `llm-wiki-skill`, `karpathy-llm-wiki`, and `wiki-gen-skill.md` are
skill packages that instruct an agent rather than constrain it, which is the
posture §1.1 argues against for an artifact that outlives its conversation.

### Governance, Memory, and the Opposite Choice — the `agent-green` Survey

**Local copies:** `~/Documents/agent-green` — 39 repositories.

Where `agent-purple` was a field of LLM-wiki implementations, this is a field of
*governance layers*: things that sit between a person, an agent, and a repository
and decide what may be relied on. Two findings dominate. One repository pair — FPF
and `haft` — supplies vocabulary this specification has been circling without
naming. And one repository, `obsidian-second-brain`, implements Karpathy's pattern
and chooses the **opposite** of gnosis on every axis, which is worth more than any
agreement here.

#### FPF and `haft` — "Reliance-Bearing Memory"

The [First Principles Framework](https://github.com/ailev/FPF) (Anatoly Levenchuk)
is a 105,000-line pattern language for keeping meanings, claims, evidence, and
decisions coherent across people, tools, and time. `haft` is its Go implementation:
"a local governance layer for AI coding agents ... what problem is being solved,
which options were compared, which decisions the human made, what evidence supports
them, and what has gone stale."

That sentence is most of §4 through §14 in one line, arrived at independently. What
it adds is a **better test than the one §10.7.4 states**:

> Ordinary local reasoning may stay in chat; records enter the project graph for
> handoff, replay, authority, automation, **evidence**, or another explicit
> downstream reliance.

§10.7.4 says *decisions are committed, observations are cached*, and that rule is
sound but hard to apply at the margin — is a fetch record a decision? `haft`'s
criterion is **downstream reliance**: does later work need to depend on this? Under
that test the fetch record is obviously committed (a quotation relies on it) and
`checked.jsonl` obviously is not (nothing relies on when you looked). The two rules
agree everywhere §4.3.1 and §4.6 already reasoned; reliance is the one that decides
the next case without a fresh argument. **Recorded as the sharper formulation of
the same rule, not as a replacement.**

Four more things transfer directly:

- **"Kernel gates, not prompt-only discipline."** Skills carry the procedure; the
  MCP kernel validates required fields, parity gaps, missing evidence, and
  authority boundaries *server-side*. This is exactly the family's own division —
  everything measurable is a CLI, the agent is reserved for what a deterministic
  check cannot decide — and finding it independently in a project built on a
  different framework is the strongest evidence yet that the split is forced by the
  problem rather than by taste.
- **"Evidence decays."** Old proof is not treated as forever current. §14.3.
- **"Human authority stays explicit."** Agents may frame, compare, verify, and
  *prepare* records; binding decisions require the human principal. §9.5, and
  §4.6.2's `Approver` is the field that makes it a property of the type.
- **"Retrieval is not application, evidence, approval, or performed work."**
  gnosis half-states this. A `search` hit is not evidence that a claim is true, and
  a document existing in the corpus is not the corpus having checked it. FPF's
  longer form is sharper still: *"a publication carrier does not become its
  subject, and a readable view does not become evidence, assurance, permission,
  decision, architecture, or work without the corresponding exact relation and
  test."* Worth a line in §11 and §17.

#### The Idea gnosis Does Not Have: Order Is Not Causality

FPF is **relation-first**, and states the consequence plainly:

> The order of text, graph edges, cards, skills, or a demonstrative walkthrough
> does not by itself prescribe causal, temporal, method, or performed-work order.
> Such order exists only when an explicit, separately governed causal claim states
> it. This does not make FPF acausal. Causality must be carried as a claim rather
> than inferred from layout.

gnosis's `links` table is **untyped**: a link is a link, and §5.5 records `href` and
direction and nothing about what the connection *asserts*. That is defensible for
Phase 1 — an untyped link cannot lie about a relationship it does not name — but it
means the corpus cannot distinguish "A cites B", "A supersedes B", "A causes B", and
"A is filed near B". §20's deferred trails decision assumes an ordered list of links
is a path; FPF's point is that an ordered list is a *layout* until something claims
it is a path.

Two other repositories arrived at the same place from different directions. FPF's
CAUSAL-USE card exists because "association, intervention, and counterfactual claims
are being treated as interchangeable" — Pearl's rungs, as an admission gate.
`systems-thinking` names **Factor Listing** as its first anti-pattern: *"listing
multiple causes without showing how they connect ... the word 'and' connects causes
that should have causal arrows."* Three independent sources, one conclusion: **an
unlabelled connection is not a causal claim, and a list is not an explanation.**
Recorded as a TODO against §5.5 and §20 rather than acted on — typing the link graph
is a Phase 3 decision and it needs a vocabulary, which is §5.8's problem.

#### `obsidian-second-brain` — The Same Pattern, Every Choice Inverted

This is the most useful repository in the survey and it agrees with almost nothing
here. It implements Karpathy's LLM Wiki and publishes a table of how it *extends*
it. Set beside gnosis:

| | `obsidian-second-brain` | gnosis |
| --- | --- | --- |
| A new source contradicts a page | **Rewrite the page.** "Claims revised, stale facts replaced." | Append a version; never rewrite (§4.1, §9.6) |
| Contradictions | Resolved **automatically** | Adjudicated, with a warrant carrying a required rationale (§10.6) |
| Patterns across pages | Synthesized **unprompted** into new pages | A synthesis is a claim and needs its own evidence (§17) |
| Cadence | Four scheduled agents: morning, nightly, weekly, health | "Nothing here is periodic, and one thing should be" (§14.3.1) |
| Note format | **AI-first**, "for LLM retrieval, not human review" | Human-readable OKF; the reader is the point (§11) |
| Thesis | "A knowledge base that maintains itself" | A corpus that will not maintain itself without a warrant |

Every one of those is a real position honestly held, and the differences are not
carelessness — they follow from a different goal. That project serves **one person's
recall**, where gnosis serves **a team's shared account of what it has checked**.
For one person's recall, rewriting is right: you want the current answer, not the
history of your own wrongness. For a shared account it is fatal, because a rewritten
claim cannot be told from a fabricated one, and §9.6's whole argument is that a
correction has to accrete rather than overwrite.

What it does supply, and gnosis should steal, is the **per-fact recency marker**:
its research dossiers write every key fact as `(as of YYYY-MM, source.com)`, inline.
gnosis puts freshness in the index and in `checked.jsonl`; a reader looking at a
rendered claim sees none of it. §14.3's staleness is currently a fact about the
corpus that the corpus does not show a reader.

The `--jsonl` envelope is the counter-example to the "AI-first" format: an artifact
can be machine-legible without ceasing to be human-legible, and the family's whole
posture is that a document nobody can read is a document nobody can review.

#### `oh-my-agent` — Verification, Not Narration

Directly applicable to `agentic-dev-harness` and `skillsaw`. Its opening claim is
the harness thesis stated better than adh states it: *"Each mechanism is mechanical:
a command exits 0 or it doesn't, a file is on disk or it isn't. No LLM is asked
whether the work 'looks correct.'"* Five mechanisms are worth lifting:

- **The Anti-Circumvention Gate** checks four artifacts a shortcut cannot fake —
  phase records, the plan, a *distinct* QA agent's result file, a *distinct*
  refactor agent's result file. *"Missing artifacts mean the phase did not run,
  whatever the narration says."* This is `proof`'s no-proof-no-close generalised
  from artifacts to **stages**, and the distinctness requirement is the part adh
  does not have: a QA result written by the implementer is not a QA result.
- **The independent judge is briefed on the criteria only, never on what the
  implementer claims it fixed**, and **re-verifies every criterion each iteration,
  including prior passes** — *"because fixing C2 is how C1 silently regresses."*
  That is a direct finding against `skillsaw`'s ratchet, which should be checked:
  a ratchet that re-scores only the failed dimensions cannot see a regression its
  own fix caused.
- **An allowlist of executable commands.** Only `typecheck`, `test`, and `lint` may
  run; *"an agent that writes anything else into the state file gets it ignored,
  never run."* A state file an agent can write is an execution surface, and gnosis's
  §9.3 should say so about anything a reply can name.
- **A cap on reinforcement** — five, *"so a permanently red gate can't trap you."*
  gnosis's promote gate currently blocks unconditionally while two signals are
  unimplementable, with no cap and no escape. That is the correct behaviour and it
  is also a deadlock; the honest form is a bound with a recorded reason, not a
  bypass.
- **`oma skills eval` measures utility lift on held-out tasks, treatment versus
  baseline, "instead of assuming a skill helps."** This is a stronger claim than
  `skillsaw`'s rubric produces. A rubric score says a skill is well-formed; a
  measured lift says it works. They are different claims and the second is the one
  a user cares about.

#### `agents-md` — The Marker Contract, Which Answers an Open TODO

gnosis has `gnosis schema [link|--check]` for maintaining `AGENTS.md`, and an open
TODO about machine-owned versus human-owned regions. `agents-md` has the whole
answer and it is three rules:

1. Generated sections are wrapped in HTML-comment markers.
2. Everything outside the markers is preserved forever.
3. **A file with no markers is never overwritten** — a `AGENTS.generated.md` is
   written beside it instead.

Rule 3 is the one a naive implementation misses and the one that matters: a file
that predates the tool was not written under its contract, so the tool may not claim
it. That is the same fail-closed direction as `EffectUnset` and `VerdictUnchecked`,
applied to a file format. It also runs both a skill and a CLI against **one shared
marker contract**, which is skillet's thesis with a different noun.

#### The Gentleman Programming Ecosystem — a Family Beside Ours

**Local copies:** `gentle-ai`, `engram`, `gentle-wiki`, `Gentleman-Skills`.

Four repositories from one author covering roughly the concerns this family covers:
a Go CLI that gates work, a Go memory layer, a documentation corpus, and a skill
catalogue. It is the only assembly in either survey that can be compared to ours as
an assembly rather than as a component, and the comparison is worth more than any
single mechanism in it.

**The shape matches almost exactly.** Two Go binaries with `internal/` and `cmd/`,
SQLite as the store, FTS5 as the search, `openspec/` for specification-driven
development, one binary per concern, and a skill catalogue one layer up that drives
the CLIs. `engram`'s own comparison document lists why it is not the popular
TypeScript alternative, and every divergence it names is a choice gnosis made
independently: Go single binary over Node plus Python plus a vector database; SQLite
FTS5 over ChromaDB; one database file over two storage systems; and — the
load-bearing one — **"Agent-curated summaries only"** over **"captures all tool
calls then compresses,"** which it states as *"Agent decides what matters"* rather
than auto-capture. That is a fifth independent derivation of the architecture
`agent-purple` found four of.

##### `AI_POLICY.md` — the Document This Family Does Not Have

`gentle-ai` ships a contribution policy for AI-assisted work, and it is the single
most directly liftable artefact in the survey. Four of its rules matter here.

- **"Review is based on observable submission quality, not on whether text or code
  appears to be AI-generated."** It refuses AI-detection as a review criterion
  outright. That is the correct position and it is the one `aidetector` — set aside
  in the `agent-purple` survey — got wrong.
- **"They may reject work that the contributor cannot explain, verify, or defend."**
  The gate is not on authorship but on defensibility. §10.6.4 argues that a required
  rationale filters more bad adjudications than a permission check; this is the same
  bet, stated as a review policy instead of a schema field.
- **"AI tools must not receive human attribution, including `Co-Authored-By`,
  `Reviewed-by`, `Tested-by`, `Signed-off-by`, approval, or equivalent credit. An
  optional `Assisted-by` trailer may be accepted."** This is `gnosis.Actor`'s
  `human:` / `agent:` / `check:` split, in git trailers, for the identical reason:
  §10.6.4 counts distinct *human* actors, so an agent that could pass for a person
  makes the count wrong in the direction that flatters the work. Two projects
  reached the same three-way distinction from opposite ends — one from a review
  policy, one from a type.
- **The disclosure has three required fields**: the tool or model, the material
  scope of the assistance, and **the verification the contributor performed.** The
  third is the interesting one. It is not "an AI helped" but "here is what I
  checked" — an audit row for a contribution, with the same shape as
  `audit.Row`'s actor plus outcome plus detail.

Recorded as a gap rather than a convergence: **this family has no AI-assistance
policy at all**, in any of its repositories, while every one of them is built this
way. §1.1 argues at length that every claim in a corpus is testimony and must name
its witness; the repositories that argue it do not name theirs.

##### Deterministic Agent Testing — Directly Applicable to `adh`

`gentle-ai/docs/testing-agents-deterministically.md` solves a problem `adh` has and
states it better than `adh` does: *"A model asserting 'I verified it, it passes' is
prose, not proof."* The product invariant it needs to test is that **a real agent
does real work and the deterministic CLI correctly decides whether that work is
acceptable** — which cannot be tested with unit tests and cannot be tested against a
live model, because a test that is non-deterministic, billed per token,
network-dependent, and holding an API key *"gets disabled within a month."*

The technique is to **keep the agent real and replace only its reasoning**: the real
binary at a pinned version, the real shipped prompt, real Git, real filesystem, real
TLS with a self-signed certificate — and a local server speaking the model protocol
from a script. Their five-point generalisation is worth quoting because it is a
method and not a trick:

1. Keep the runtime real — same binary, version pin, shipped prompt, permissions.
2. Replace only the reasoning.
3. **Make the fixture adversarial.** Assert on the incoming request, not just the
   outgoing response. Fail when evidence arrives out of order.
4. Fake nothing else. Anything that can be deterministic should stay real.
5. Keep the non-deterministic part out of the gate.

Point three is the one that makes it a test rather than a playback, and their code
shows it: the fixture refuses to dictate the next step if the agent asked to advance
before producing commit evidence. *"The contract is checked in both directions."*

Point five is a scoping principle for `skillsaw` as much as for `adh`: *"Whether a
prompt reliably steers a live model is a product question, answered by usage, not by
CI."* And the honest-limits section is the discipline this specification practices in
prose and not in its suite — **"Not proved. That a live model, given the shipped
prompt, produces the same tool calls the fixture scripts."**

For gnosis the application is specific. The relay was designed so that gnosis never
calls a model, which makes its own tests easy — and `cmd/relay_test.go` consequently
hand-writes every reply. Nothing tests that an agent handed a real emitted prompt
produces a reply `admit` will accept. That is the one place gnosis's determinism
makes a gap rather than closing one.

##### `review-authority-threat-model.md` — the Sentence gnosis Needs

Ninety-eight lines, and the first paragraph is one gnosis should copy the posture of:

> It does not claim to authenticate state against a malicious local actor with the
> same user and filesystem access: without an external trust anchor, that actor can
> rewrite the state, receipt, Git repository, or binary.

And then, among the retained controls: **"Checksums only where useful for detecting
accidental corruption; they are not authentication."**

gnosis's tier-0 record is content-addressed so that *"a rewritten record lands at a
different path, which makes append-only structural rather than conventional."* That
is true and it is narrower than it sounds: it detects **accidental** corruption and a
careless edit, and a local actor who recomputes the hash and renames the file defeats
it entirely. §4.3.1 says tampering is *"visible rather than absorbed"* and that
over-claims by exactly the amount Gentleman's sentence marks off. The threat model
also names three controls gnosis lacks:

- **A lock plus an expected revision**, so a stale writer is rejected rather than
  merely queued. gnosis holds its lock across compute-and-write, which is sufficient
  today; a served coordinator holding the lock across many commands would need the
  revision.
- **Corruption distinguished from operational failure** — *"Only malformed state,
  checksum, graph, or receipt evidence is corruption; operational Git and filesystem
  failures remain [operational]."* `bundle.AuditTrail` treats a malformed line as an
  error and cannot say whether the disk failed or somebody edited the file.
- **A declined candidate recorded as a first-class authorization.** gnosis records a
  refused promotion in the audit trail; Gentleman records the *decline itself* as a
  canonical authorization, atomically, without creating a review lineage. The
  distinction is that a refusal in a log is an observation and a recorded decline is
  a decision, which is §10.7.4's own line applied one level up.

##### `engram` — Where It Agrees, and the One Place It Diverges

Beyond the architecture convergence, three mechanisms:

- **A review lifecycle that is deliberately local.** A memory carries a computed
  `state` of `active` or `needs_review` plus a `review_after` date, and
  *"`review_after` is intentionally not part of sync payloads."* Marking something
  reviewed is local-only while the memory itself is shared. That is `checked.jsonl`
  exactly — §4.3.1's one documented exception to §4.5 — reached independently, which
  is the strongest evidence yet that the per-user/committed line falls where gnosis
  put it.
- **Provenance of why a record exists, with an opt-out for machine writes.**
  `mem_save` takes `capture_prompt`, best-effort records the prompt that produced an
  observation, and the guidance is that *"automated saves such as SDD artifacts
  should pass `capture_prompt=false`."* gnosis stamps a quarantined document with the
  cache key of the reply that produced it, which is the same idea with a stronger
  handle: the key resolves to the exact prompt, model, and source version.
- **The audit boundary is fail-soft and says so out loud.** A store that does not
  implement `InsertAuditEntry` *"silently skips the audit with a log warning — no
  panic, no 5xx"*, and the insert is deliberately synchronous because buffering
  *"would complicate recovery, lose entries on process restart."* gnosis reached the
  same synchronous conclusion, and its fail-soft path is **weaker**: the audit
  failure lands in the outcome's message where no machine reads it, while engram's
  lands in a log where an operator does.

Where it diverges is the same axis `obsidian-second-brain` diverges on, and for the
same reason. A topic upsert *"increments `revision_count` so evolving decisions stay
in one memory"*; exact duplicates *"update metadata (`duplicate_count`,
`last_seen_at`) instead of creating new rows"*; deletion is a soft `deleted_at` that
search ignores. One record that counts its own revisions, rather than a version per
revision. For one person's working memory that is right and cheaper. For a shared
account it loses the ability to say what the claim used to be, which §9.6 will not
give up. Worth noting all the same: **`revision_count` answers "has this been
churning?" for free**, and gnosis can only answer it by walking git.

##### The Skill Catalogue, and a Convention That Verifiably Held

`Gentleman-Skills` splits `curated/` from `community/` and the split is an admission
*procedure*, not a label. Curated skills are *"personally crafted and
battle-tested"* by the owner. Community skills *"go through a democratic voting
process"* — a seven-day review, GitHub reactions as votes, maintainers counting on
day eight, plus automated structural validation.

That is a permission-and-quorum model, and gnosis explicitly bet against it: §10.6.4
holds that a required rationale filters more bad adjudications than a permission
check ever will. Here is a live system that chose the other way. The bet is not
symmetric and the difference is the cost of being wrong — a bad skill in a catalogue
is uninstalled, and a bad claim in a corpus is cited. A vote counts *how many* people
approve; a rationale records *why one* did. For a catalogue with many reviewers and
low blast radius, counting is the cheaper instrument. Recorded because it is the
first time in either survey that §10.6.4's position has been contested by something
that works.

The more useful finding is in `gentle-ai`'s own `AGENTS.md`, which is not project
instructions but a **skills index**: a `Skill | Trigger | Path` table with *"load the
relevant skill(s) BEFORE writing any code."* It declares a portability convention —
*"`gentle-ai-*` skills are repo-specific workflow skills. Unprefixed skills are
portable writing or work-unit skills and intentionally keep their canonical
names"* — and the convention **verifiably held** when checked: `cognitive-doc-design`
is unprefixed and is **byte-identical** across `gentle-ai` and `gentle-wiki`, while
`branch-pr` is exposed as `gentle-ai-branch-pr` and has legitimately diverged.

That is the promote-on-second-consumer moment, observed from outside. Two repositories
hold one skill at identical bytes, by hand, with nothing enforcing it — and the
convention that says which files ought to match is written in prose in a table.
It holds today because one person maintains both. **A cross-repository check that
same-named unprefixed skills hash alike is a `skillsaw` command that does not exist
and should**, and it is the one mechanism in this ecosystem that this family could
supply back.

`gentle-ai` also splits skills by *whose* work they govern: `internal/assets/skills/`
is embedded in the binary and ships to users, `skills/` is repo-local and governs
work on the tool itself. Neither `skillet` nor `steve-skill-market` marks that
difference, and it is a real one — a skill that ships is a published artefact under
`speclint`'s rules, and a skill that governs the repository is closer to a
`CONTRIBUTING.md`.

#### Convergences Worth Recording

- **`Acontext`** — "skill memory": agent memory as plain markdown skill files, and
  explicitly *"progressive disclosure, not search ... no embeddings ... Git, grep."*
  Independent agreement with §11.0's refusal of semantic search, reached for the
  same reason — a retrieval path nobody can inspect is a retrieval path nobody can
  correct. Where it parts company is the write path: an LLM distillation pass infers
  "what worked and what failed" and writes it to a skill file with no quotation and
  no gate, which is precisely the admission this specification exists to refuse.
- **FPF's "unknowns propagate (never coerce to zero")** for constraint-fit. The
  same rule as `quotecheck.Unchecked`, `VerdictUnchecked`, and `EffectUnset`, stated
  as a measurement principle. Three unrelated derivations of one discipline.
- **FPF's edition pinning** — `DescriptorMapRef.edition`, `policy-id`, and a
  `PathSliceId` recorded with every parity run, because results without them are
  *"refresh-unsafe."* §6.5's standards-hash-in-every-finding, independently.
- **`haft`'s four-valued refresh verdict** — `no_change`, `apply_ready`,
  `review_ready`, `candidate_rejected` — where `review_ready` is *"an auditable
  semantic-delta classification, not a veto"*: the candidate is adopted, a prominent
  warning prints, and every finding is retained for later review. gnosis's gate has
  pass / fail / unchecked and no equivalent of "admitted, with the concerns
  recorded." Whether §9.5 wants one is a real question and it is now in TODO.
- **`haft`'s rebuild sanity guard** — a refresh is hard-rejected when the derived
  source-unit projection falls *below 50% of the preceding verified count*. gnosis's
  `index rebuild` has no such guard: a rebuild that finds three documents where
  there were five hundred is a corrupted bundle, and gnosis would write it without
  comment. A cheap, high-value check, and now a TODO.
- **`gentle-wiki`'s scope rule** — a wiki *"complements product repositories by
  explaining practices and concepts without becoming a second source of truth for
  product behavior."* gnosis says at length what a document **is** and nowhere what
  a corpus should **decline to hold**. A corpus that restates what the code already
  says is a second source of truth that drifts, and the drift is invisible because
  both halves are internally consistent.
- **`systems-thinking`'s anti-pattern catalogue** — named failures with *mechanical*
  detection criteria (phrase lists, structural absences) and a separate fix. That is
  `skilllens` in a different domain, and it is the right shape: the detector is
  deterministic and the repair is guidance. Worth mining for `canonizer`.
- **`agent-sop`** — `.sop.md`, natural-language parameterised workflows, from the
  Strands project. A sibling format to `SKILL.md` with a published specification;
  relevant to `steve-skill-market` as an interchange question rather than a
  competitor.

#### Read Shallowly — Warranting Deeper Exploration

Judged by README, layout, and targeted greps only. Each could repay a closer look:

- **`haft` internals** (Go, ~40 packages: `authority`, `autonomyenvelope`,
  `decisionbinding`, `contextgraph`, `graphrank`, `governance`, `p13acceptance`).
  This is the single highest-value item in the survey and only its README was read.
  It is a Go implementation of the same problem gnosis solves, with a worked model
  of authority and decision-binding that §10.6 would benefit from.
- **`FPF-Spec.md` beyond the practical-use cards** — 105,000 lines. The TIME,
  CAUSAL-USE, DESCRIPTION-USE, NAMING, and WORDING cards were read; the pattern
  bodies they point into (C.27, C.28, E.17, F.18, E.10) were not. NAMING's
  `NameCard` and WORDING's `KindRestorationCheck` both look directly applicable to
  §5.8's vocabulary problem.
- **`ruflo`** (Rust, 623M) — "self-learning / self-optimizing agent architecture."
  Relevant to adh's optimization loop and `skillsaw`'s ratchet; not opened.
- **`hindsight`** — claims state-of-the-art on LongMemEval with "biomimetic data
  structures" rather than vector search or a knowledge graph. gnosis will not adopt
  a learned memory, but the *benchmark* is interesting: it is a measured claim about
  retrieval quality, and §11 currently argues from principle alone.
- **`oh-my-agent`'s `judge-protocol.md` and `event-spec.md`** — read only as
  summarised in the README table; the protocols themselves are the artefact.
- **`Context-Engineering`** (86M, course material) — surveyed at the top level only.
- **`evals`** (Strands Evals SDK) and **`scientific-agents`** (503 expert profiles) —
  identified as relevant to `skillsaw` in principle, not examined.
- **`ECC`, `gstack`, `opengap`, `superpowers`, `acpx`, `hankweave-runtime`** — agent
  harnesses and runtimes, read at README level. `superpowers`' composable-skill
  methodology is the most likely of these to matter to `steve-skill-market`.

#### Surveyed and Set Aside

`camel`, `semantic-kernel`, `trpc-agent-go`, `nexent`, `deer-flow`, `Archon`,
`hive`, `MMCTAgent`, `oh-my-openagent`, `gentle-ai`, `cascadeflow` — agent
frameworks, orchestration platforms, and model-routing layers. gnosis calls no
model and builds no agent; these solve a problem it does not have.
Five repositories are skill catalogues — the collection *is* the product — and they
are worth grading with `skillsaw` rather than mining for mechanism. Counted by hand,
because a `find -name SKILL.md` is misleading here: `thinking-skills` (24, with a
template and an authoring guide), `Gentleman-Skills` (24), `ai-agent-skills` (10,
with a CI workflow that gates its own catalogue), `superpowers` (14, though it calls
the skills the substrate of a methodology rather than the product), and `agent-sop`
(5 `.sop.md` plus one skill for authoring them, in a different format entirely).

Two of those have something a catalogue owner should see. **`Gentleman-Skills` splits
`curated/` from `community/`** — a trust tier inside one repository, which is §14.1's
distinction already load-bearing in a live catalogue and something
`steve-skill-market` does not currently make. **`ai-agent-skills` runs a
`validate-skills.yml` workflow over its own contents**, which is what `skillsaw`
exists to do; it is a real integration target rather than a hypothetical one.

Three repositories were miscounted on a first pass and the corrections matter more
than the count. `agent-thinking-skills` holds **two** skills, exactly as its README
says — it was filed by its name rather than its contents. `gstack` is not a tooling
opinion piece but a full stack with 54 top-level skill directories and 443 source
files beside them. And the very large SKILL.md counts — `ECC` at 898, `ruflo` at 352,
`oh-my-agent` at 94 — are **one skill tree multiplied by host**: the same skills
installed into `.agents/skills`, `.claude/skills`, `.cursor/skills`, and
`.kiro/skills`, plus test fixtures and benchmark runs. `trpc-agent-go`'s 74 all sit
inside `examples/`. Those are harnesses that ship skills, and counting files would
have made each of them look like the largest catalogue in the survey.

`agents.md` is the AGENTS.md standard's own website. `principles` generates agent
networks from first-principles decomposition and is an experiment its author labels
as such.

`doceo` and `systems-thinking` are single skills, one `SKILL.md` each. `doceo` was
grouped with the catalogues on a first pass and does not belong there.
It is a single skill — an AI tutor that answers in one screen: one plain-language
answer, one diagram, one analogy, a self-quiz. What makes it relevant is the part
that is not tutoring: **every lesson is saved to the user's notes, the next lesson
reads what was already learned, and the skill revises itself from feedback.** That
is a self-accreting corpus with a feedback loop, and it is the third project in this
survey — after `obsidian-second-brain` and `Acontext` — that writes durable
knowledge with no evidence gate on the write path. Three independent projects making
the same choice is worth recording as the field's default rather than as one
author's oversight, and it is the choice §1.1 exists to argue against.

One more pattern is visible only across repositories. `gentle-ai` (runtime),
`gentle-wiki` (documentation), `engram` (memory), and `Gentleman-Skills` (skills)
are four repositories from one ecosystem, covering roughly the concerns this family
covers. Whether that assembly agrees or disagrees with ours is unexamined and is the
most interesting unread thing in the survey.

## What We Intend to Build from This

Our tools cover skills. The knowledge base is still a hand-curated repository of
markdown, and the gap is ingestion: taking in outside sources and accreting
domain knowledge collectively without polluting the corpus with contradictory or
low-quality material.

Two observations reorder everything above. First, **every project here detects
similarity; none adjudicate conflict.** llmwiki surfaces contradictions, mnemon
deduplicates, coherence finds broken support links — but nothing decides which
of two conflicting claims is authoritative, or records why. That is ours to
build, and we already own the right primitive: `merge-skills` detects
convergence across independently-derived skills, and contradiction detection is
its dual — the same comparison, kept when it comes back negative. The verdict
belongs in `canonizer`, which is findings-based by design and refuses to emit a
weighted score. Admission is a findings problem, not a threshold problem; the
moment it becomes a number, someone will ship against it.

Second, **the invariant that makes the knowledge base trustworthy is that a
person read every byte.** Automatic ingestion ends that, and nothing in the repo
currently records which parts still hold it. Provenance tier becomes explicit
per artifact.

### Four Tiers

`gnosis` (`~/Documents/git/gnosis`) is the tool this section used to describe as a
fork of llmwiki. It is not a fork — llmwiki is prior art we read closely and
departed from, chiefly because it stores hashes rather than bytes and asks a model
per candidate page. **The authoritative design is [`SPEC.md`](./SPEC.md); what
follows is the shape, not the specification**, and where the two disagree the spec
is right.

Karpathy names three layers. `gnosis` splits his first in two, because "immutable"
is an assertion nothing was enforcing:

- **Tier 0 — `evidence/`**, append-only, committed. Content-addressed archived
  *text* (`.md`, `.txt`, `.svg`), plus a fetch ledger recording `(uri, sha256)` for
  everything fetched including what was not archived. This is the piece llmwiki
  lacks: it computes hashes and validates against a live fetch, so its byte-exact
  guarantee silently weakens the day a source changes.
- **Tier 1 — `.gnosis/quarantine/`**, mechanically admitted, not authoritative,
  **not in the bundle**. Trust: unverified. This is where an ingested claim waits
  for the promote gate.
- **Tier 2 — the bundle**, an OKF knowledge base in git: `index.md`, `log.md`, and
  `c/<uuid7>-<slug>.md`. The only source of truth, and the only thing shared
  between people.
- **Tier 3 — `.gnosis/`**, derived, regenerable, gitignored, **per-user**: the
  SQLite index, the response cache, the miss log, the audit trail.

Tier 0 also splits a signal llmwiki conflates, and the two demand opposite
responses: a quote that no longer matches the *archived* bytes is corruption — fail
hard; archived bytes that no longer match *upstream* are staleness — flag the
derived claims, do not fail. `goalx`'s `freshness-state` is the model for the
second, and its `unknown` versus `not_applicable` distinction is what keeps "we
never looked" from reading as "it is fine".

### Three Kinds of Knowledge, Not Two

Reconciliation produces knowledge present in no source. It cannot carry a
byte-exact quote, so **the most valuable artifact a team produces fails llmwiki's
trust property by construction.** Our first pass at this named two classes; the
corpus actually holds three, and the middle one is the largest:

- **Sourced** — quote-validated, tier-0 backed, machine-admissible.
- **Adjudicated over sources** — the weighing. *"We weighed Gilb's Planguage
  formulation against our existing SLO practice and adopted X."* Both inputs are
  citable; the decision appears in neither. Explicit in the world, tacit in the
  team.
- **Genuinely tacit** — adjudicated with no sources at all. Real, and the smallest
  of the three.

Only the third is source-free, which means a warrant and a citation are not
alternatives and the schema must let them co-occur freely. This is where the
commitment to maintaining local decisions about how to prioritize and trade off
requirements actually lives.

### Decisions Since Settled

Recorded here because each changed the shape above, and because a reader of this
document should not have to reconstruct them from the spec's section numbering.

- **A claim is not a document.** OKF defines "Concept ID" as *the path of the
  concept's file*, so in the format we conform to, a concept **is** a document. The
  addressable assertion *inside* a document needed a word nobody had spent, and it
  is `claim`. The collision had already produced one wrong recommendation before it
  was caught.
- **A claim's address lives in the document, not the index.** Identity is assigned
  and content-free; the address is a fold-normalized anchor carried in frontmatter.
  A byte offset is a location, not an address — it does not survive reflowing a
  paragraph, and an index that alone knew a claim's identity would lose it on
  rebuild.
- **Local reductionism about testimony.** Every claim here is something somebody
  said. Provenance attaches per claim rather than per corpus, and **a source's
  reliability is never inherited by its claims.**
- **A reader may challenge an accepted claim.** Adjudication previously started
  only when a check noticed something, which left the most capable informant — a
  person who already knows a claim is wrong — with no way in. Four classes ordered
  by what settles them, and the strongest is the one gnosis can verify itself.
- **One writer per user, git between users.** No shared database, no locking
  protocol. The index is per-user and derived, so two people at one commit hold the
  same index and a disagreement between them is about the corpus. The cost: two
  people documenting one subject produce two identifiers and git merges both
  *cleanly*, which makes duplicate detection a post-merge reconciliation step
  rather than a hygiene check.
- **One surface phrase resolves to one key, enforced.** Perspective-keyed
  vocabularies are coherent for a glossary, which records what people mean, and
  incoherent for a comparison substrate, which decides whether two claims disagree.
  The rule forces a conversation; that is the intent.

### Minimizing the Model

The goal is a pipeline that is deterministic wherever determinism is available.
llmwiki's validator and auto-promote heuristic already call no model, which is the
proof the rest is reachable; the work is pushing everything else in that
direction.

1. **Content-addressed response cache**, keyed on
   `(source content_hash, prompt hash, model + version)`. A second run over
   unchanged inputs makes
   *no model calls at all* and reproduces byte-identically. Cheapest and largest
   determinism win available. `qvr`'s lock — resolved SHA, subtree hash, verdict
   — is the record shape.
2. **Deterministic candidate selection.** Replace the LLM scan over ~47 pages
   with an index in the manner of `skillex` (same corpus state always produces
   the same index) plus a declared artifact graph in the manner of `coherence`'s
   `ontology.yml`. This is up to 50 of the ~65 calls per ingest.
3. **Separate accretion from synthesis.** Appending evidence to a page is
   mechanical and needs no model; rewriting a page body is a gated, rare,
   deliberately-triggered event. llmwiki's own `body_only` update outcome shows
   the two are already separable — and accretion is what we actually asked for.
   Rewriting is replacement.
4. **Deterministic pre-filters ahead of any contradiction call.** Glossary
   normalization (`context`'s maintained `GLOSSARY.md` — two claims cannot be
   compared until the same word means the same thing), direct computation of
   numeric, threshold, and enumeration conflicts, JSON Schema over frontmatter
   (`katalyst`), and the injection, hidden-character, and secret scans from
   `qvr` and `mdm` at the admission gate. The model sees only the residue. This
   is `agentsys`'s doctrine and `AgentLint`'s 51-deterministic/7-AI split.
5. **Scoring and gating move to tools we already own.** `skillsaw` is
   deterministic by charter and never calls a model; candidate pages score
   there. Conflict verdicts land in `canonizer` as findings.
6. **Pin anything statistical.** If embeddings generate conflict candidates,
   note that an embedding is a deterministic function of a pinned model, unlike
   sampled generation — pin model and version in the lock and neighbor sets
   reproduce exactly. `mnemon`'s graph is the candidate generator.

What remains irreducibly judgmental is prose-to-claim extraction and semantic
conflict between claims that survive the pre-filters. We do not pretend
otherwise; we bound it. Every call records `(prompt, model, version, output hash)` so any run is replayable and auditable. An adversarial refuter that never
saw the proposer's reasoning checks the output, in the manner of `4x`'s role
isolation. And human PR review is the last gate. The model proposes;
deterministic gates and people dispose — nondeterminism upstream of a
deterministic gate is bounded, and with the cache, a repeat run is not
nondeterministic at all.

One cost to state plainly: four tiers create link-integrity surfaces that did not
exist before — a claim citing an archived file, a quarantined document referencing
a bundle concept, an index row pointing at a path. That is precisely the drift
class `coherence` detects, so it is gated in CI rather than discovered later, and
it is why every foreign key is an identifier and nothing joins on a path.

## Techniques Worth Adding to the Tools We Already Have

Separate from the knowledge-base work: a pass over `exegesis`, `skillsaw`, `adh`,
and `canonizer` against the survey, with each claim checked against both
codebases rather than against a README. Each tool's own `TODO.md` carries the
full reasoning; this is the summary.

**The finding that outranks the rest is a live inconsistency between two of our
own tools.** `exegesis/internal/textnorm.Fold` folds whitespace runs *and*
typographic characters — curly quotes, dashes, non-breaking spaces — before
comparing a quotation to its source, because a guard that fired on every curly
apostrophe would not get run. `canonizer/internal/verify.normalize` is
`strings.Join(strings.Fields(s), " ")`: whitespace only. So the two tools answer
the same question — *does this quotation appear in the source?* — differently,
and a rule anchored to a passage containing a curly apostrophe fails
`verify.Provenance` while passing `quotecheck`. This is exactly the drift skillet
exists to prevent, and it is the strongest argument for promoting `textnorm`.

### `exegesis` — Structure Gate

- **Hidden-character scanning belongs in `lint`.** `lint` gates frontmatter, body
  links, and runtime neutrality — nothing adversarial. `qvr`'s
  `internal/security/unicode.go` is pure, dependency-light, and detects
  zero-width characters, bidi overrides (Trojan Source), and Unicode tag
  characters. Its constants are **codepoint ranges from the Unicode standard, not
  tuned thresholds**, which is what separates it from the heuristics the family
  rejects. It also reasons explicitly about false positives (mixed-script
  confusables are a warning, since multilingual prose is legitimate).
- **`coherence`'s derived applicability gate.** Its `OrphanEndpoints` meter
  carries `Convention bool` — true only if the repo demonstrably uses the
  test→source pattern — and skips promotion when false. That is a *fifth* way to
  answer "does this check apply here", and unlike the four the family already
  uses, it is **derived from the corpus rather than declared, judged, or opted
  into**. Directly applicable to the related-skill edge graph, where
  `NewlyOrphaned` / `NewlyCovered` is the shape a link-integrity gate wants.
- **`qvr`'s lock as a manifest model.** `skillet/manifest.Skill` records
  `{slug, dir, sha256, test_prompts}` — identity, but no origin and no verdict.
  A `qvr.lock` entry adds resolved commit, subtree hash, scan decision, and
  commit author. The gap only bites for skills that come from outside, which is
  precisely the ingestion case.

### `skillsaw` — Quality Score and Ratchet

- **The rubric should be data, not a Go table.** `rubric.Dimensions()` hardcodes
  the nine weights, and their provenance is a code comment ("the reconciled table
  from spec D1"). `AgentLint/standards/` splits the same job across three files
  joined on a check ID: `weights.json` (dimension + per-check weights),
  `reference-thresholds.json`, and `evidence.json` (58 checks, each with
  `dimension`, `scope`, `fix_type`, `evidence_sources`, `evidence_text`) over a
  typed source registry that grades its own citations `primary-data` /
  `peer-reviewed` / `case-study` / `industry-practice` — and annotates the weak
  one honestly ("n=1 case study, useful reference point not universal
  benchmark"). This is the manifesto's own requirement — maintain local decisions
  about how to prioritize and trade off, **with provenance** — implemented.
- **Its thresholds file states our charter better than we do:** *"Reference
  values from empirical data. NOT enforced thresholds — AgentLint measures and
  compares, users decide."* Every reference carries the empirical basis that
  produced it (`"Anthropic 265 versions: 12→4"`).
- **Externalizing weights finds bugs.** AgentLint's `weights.json` note records
  that its dimension weights deliberately sum to 1.10 because several checks were
  being emitted while **no dimension owned them, so they were silently dropped**.
  That defect was visible because the weights were data. Ours sum to exactly 100
  and the invariant is asserted in a comment.

### `adh` — Five-Stage Arc Loop

- **Evidence has no staleness state.** `internal/evidence` is an append-only log
  with a timestamp; a grep for `stale` across `internal/` returns nothing. When
  the thing an evidence record measured changes, the record stays as valid as the
  day it was written. `goalx/cli/freshness_state.go` carries the vocabulary worth
  copying: four states — `fresh`, `stale`, `unknown`, `not_applicable` — with
  `LatestRevision` vs `CurrentRevision` and a reason. The discipline matches
  `skillet/timeseries`, where absence of history is a distinct state rather than
  a zero.
- **`clu`'s capability routing.** Agents declare capabilities; `cap:*` labels
  route unassigned work to those that match, with a shared pool for work carrying
  no `cap:` label. adh's autonomy ladder governs *how much* an agent may do, not
  *which* agent should take it — the missing axis for a team with genuinely
  different skills.
- **Correction: adh's redaction is already better than `pantry`'s**, and the
  earlier suggestion to adopt it was wrong. `internal/redact` offloads detection
  to `betterleaks`' maintained ruleset and is a thin wrapper — which is what
  skillet's *Preserve Mature Libraries* constraint asks for. `pantry` hardcodes
  ten regexes, including a `password\s*[:=]\s*["']?.+` that runs to end of line.
  One idea survives: pantry's first redaction layer honours explicit
  `<redacted>…</redacted>` tags, letting an author declare a secret the patterns
  would miss.
- Likewise, adh's `internal/nfr` is **Planguage-quantified** (Scale, Meter,
  Fail/Goal/Stretch, Direction, a validated taxonomy tag, and rejection of a goal
  ordered worse than its fail threshold) and is already more rigorous than
  `goalx`'s obligation model. The lesson from goalx is narrowly freshness.

### `canonizer` — Ruleset Grader

- **`verify.Provenance` conflates two failures under one category.** A rule whose
  `↦` anchor is not found in the source emits `anchor-absent` and blocks —
  whether the anchor was **fabricated** (a real defect) or the **source drifted**
  under it (an edition change or a reformat, where the rule may still be sound
  and only the anchor needs refreshing). llmwiki makes the same conflation with
  `evidence_invalid`. Once an immutable evidence archive exists, the two become
  distinguishable, and they warrant opposite responses.
- **A fix class on findings.** A `finding.Diagnostic` is
  `{severity, category, path, message}` — it says what is wrong but not who acts.
  AgentLint carries `fix_type` (`guided` / `assisted`) per check.
- **Contradiction detection lands here first.** canonizer is consumer #1 of the
  `ruleset/conflict` work recorded in skillet's TODO: certifying that a ruleset
  is internally consistent is the base rules' entire claim, and nothing checks it
  today.
