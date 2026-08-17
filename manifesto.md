# manifesto

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

## Tools we have created, and how we use them

The family is one shared kernel, four CLIs that consume it, and three agent skills
that drive those CLIs.

```text
book2skill ──▶ exegesis ──▶ skillsaw ──▶ merge-skills
 produce       gate           score        consolidate
                     │
   adh ─────────────┴───────────── canonizer
   (fellow kernel consumers, alongside)
```

### Shared kernel

#### [skillet](https://github.com/StevenACoffman/skillet) — `v0.15.0`

- **Repo:** `git@github.com:StevenACoffman/skillet.git`
- **Local:** [`~/Documents/git/skillet/`](file:///Users/steve/Documents/git/skillet/)

The Go library every other tool imports, so two tools cannot disagree about one
definition. Holds `speclint` (agentskills.io frontmatter), `redlines` (book2skill's
mechanical Quality Red Lines), `skilllens` (the three SkillLens detectors),
`markdown`, `judge`, `testprompts`, `ratchet`, and `timeseries`, plus `finding`,
`ruleset`, `proof`, `provenance`, `identity`, `neutrality`, `stats`, and
`calibration`. **Promotion rule: a package moves here on its second consumer.**

### The four CLIs

#### [exegesis](https://github.com/StevenACoffman/exegesis) — structural gate

- **Repo:** `git@github.com:StevenACoffman/exegesis.git`
- **Local:** [`~/Documents/agent-orange/exegesis/`](file:///Users/steve/Documents/agent-orange/exegesis/)

Gates a skill tree on the way in: `lint`, `verify`, `tests`, `scaffold`, `link`,
`index`, `merge-index`. Owns the related-skill edge graph and `INDEX.md`. Proves
a tree is well-formed; says nothing about quality. Also owns `quotecheck`, the
fabrication guard that reports quotations appearing in none of the source texts.

#### [skillsaw](https://github.com/StevenACoffman/skillsaw) — quality score and ratchet

- **Repo:** `git@github.com:StevenACoffman/skillsaw.git`
- **Local:** [`~/Documents/agent-orange/skillsaw/`](file:///Users/steve/Documents/agent-orange/skillsaw/)

Scores skill quality on the 9-dimension rubric and runs the keep-or-revert ratchet:
`eval`, `diagnose`, `judge`, `gate`, `preflight`, `activation`, `scores`,
`calibrate`. **Deterministic only — never calls a model.** The fusion of
SkillLens + SkillOpt exists here and nowhere else.

#### [agentic-dev-harness](https://github.com/StevenACoffman/agentic-dev-harness) (`adh`) — five-stage arc loop

- **Repo:** `git@github.com:StevenACoffman/agentic-dev-harness.git`
- **Local:** [`~/Documents/git/agentic-dev-harness/`](file:///Users/steve/Documents/git/agentic-dev-harness/)

A different consumer of the same kernel: drives a change through a five-stage arc
loop (strategy → execution → critic → evaluation → ops) with its own 5-dimension
rubric. Being the second consumer of `skilllens` and `markdown` is what justified
promoting them.

#### [canonizer](https://github.com/StevenACoffman/canonizer) — ruleset grader

- **Repo:** `git@github.com:StevenACoffman/canonizer.git`
- **Local:** [`~/Documents/git/canonizer/`](file:///Users/steve/Documents/git/canonizer/)

Grades rulesets rather than skills: `verify.Executable`, `verify.Provenance`,
`verify.Specificity`, plus a cold-critic prompt. **Findings-based by design — never
a weighted score that could become a ship threshold.** 0 open.

### The agent skills — [steve-skill-market](https://github.com/StevenACoffman/steve-skill-market)

All three live in `git@github.com:StevenACoffman/steve-skill-market.git`, under
[`~/Documents/agent-orange/steve-skill-market/skills/`](file:///Users/steve/Documents/agent-orange/steve-skill-market/skills/).

| Skill | Role | What it does | Open |
| --- | --- | --- | --- |
| [`book2skill/`](file:///Users/steve/Documents/agent-orange/steve-skill-market/skills/book2skill/) | producer | Distills a book into RIA-TV++ skills (R, I, A1, A2, E, B) via exegesis. | 0 |
| [`skillsaw-skill/`](file:///Users/steve/Documents/agent-orange/steve-skill-market/skills/skillsaw-skill/) | optimizer | Drives the skillsaw CLI through a hill-climbing loop: baseline → diagnose → one edit → gate. | — |
| [`merge-skills/`](file:///Users/steve/Documents/agent-orange/steve-skill-market/skills/merge-skills/) | consolidator | Detects convergence across book2skill outputs and builds one merged skill. | 0 |

**Pipeline:** book2skill produces → exegesis gates structure → skillsaw scores
quality → merge-skills consolidates. `adh` and `canonizer` sit alongside as fellow
kernel consumers.

## Tools we were inspired by, and what we learned from them

Each entry below is a project we read closely. The bullets are the specific
technique we want, and *Feeds* names the tool of ours it belongs in. Nothing
here is an endorsement of the whole project — only of the part we are stealing.

### Harness quality: scoring, gating, and drift

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

### Skills and context as a governed supply chain

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

### Knowledge base, provenance, and grounding

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

### Durable state, memory, and coordination

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

### Harness internals and model-agnosticism

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

### The practice, and where it already landed

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

### The knowledge-base gap — the highest-value find

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

### Self-evolution loops, and what they cost

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

### Judgment gates and deterministic routing

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

## What we intend to build from this

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

### Three tiers

- **Tier 0 — evidence archive.** Content-addressed immutable snapshots of source
  bytes, keyed `(uri, content_hash)`, append-only. This is the piece llmwiki
  lacks and it is small: it already computes the hashes, so we are adding "keep
  the bytes" and repointing the validator at the snapshot instead of the live
  fetch.
- **Tier 1 — staging synthesis.** Forked llmwiki, cross-page updates on.
  Reconciliation is *attempted* here. Deliberately not authoritative.
- **Tier 2 — the curated git knowledge base**, unchanged in kind. Promotion is
  the seam, and `llmwiki promote` already fails closed on invalid evidence; we
  wrap it in the ordinary PR review that non-engineers on the team can operate.

Tier 0 also splits a signal llmwiki currently conflates, and the two demand
opposite responses: a quote that no longer matches the *archived* bytes is
corruption — fail hard; archived bytes that no longer match *upstream* are
staleness — flag the derived claims for re-review, do not fail. `goalx`'s
`freshness-state` is the model for the second.

### Two provenance classes

Reconciliation produces knowledge present in no source. It cannot carry a
byte-exact quote, so **our most valuable artifact fails llmwiki's trust property
by construction.** The corpus therefore holds two kinds of claim:

- **sourced** — quote-validated, tier-0 backed, machine-admissible.
- **adjudicated** — warranted by the PR, the participants, the date, and the
  recorded reasoning; supersedes rather than deletes.

This is where the manifesto's commitment to maintaining our local decisions
about how to prioritize and trade off requirements actually lives, and it gives
contradiction detection something to be authoritative against.

### Minimizing the model

We fork llmwiki to make the pipeline deterministic while preserving its
behavior. The validator and the auto-promote heuristic already call no model;
the work is pushing everything else in that direction.

1. **Content-addressed response cache**, keyed on `(source content_hash, prompt
   hash, model + version)`. A second run over unchanged inputs makes *no model
   calls at all* and reproduces byte-identically. Cheapest and largest
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
otherwise; we bound it. Every call records `(prompt, model, version, output
hash)` so any run is replayable and auditable. An adversarial refuter that never
saw the proposer's reasoning checks the output, in the manner of `4x`'s role
isolation. And human PR review is the last gate. The model proposes;
deterministic gates and people dispose — nondeterminism upstream of a
deterministic gate is bounded, and with the cache, a repeat run is not
nondeterministic at all.

One cost to state plainly: two stores create a link-integrity surface between
tiers that did not exist before. That is precisely the drift class `coherence`
detects, so we gate it in CI rather than discovering it later.

## Techniques worth adding to the tools we already have

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

### exegesis — structure gate

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

### skillsaw — quality score and ratchet

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

### adh — five-stage arc loop

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

### canonizer — ruleset grader

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
