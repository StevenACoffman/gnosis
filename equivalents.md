# Standard Ecosystem Equivalents and Approximations

This document compiles the closest functional approximations and standard equivalents for the custom-built tools in this family, as verified directly from `/Users/steve/Documents/git/gnosis/manifesto.md`.

---

## 1. Knowledge Base & LLM-Wiki

### `kvt` (Nearest Gnosis Architecture Equivalent)
`kvt` is the closest system architecture match to `gnosis` in the wild, utilizing markdown as source of truth and a compiled SQLite index with FTS5 search.

<!-- CITATION file="/Users/steve/Documents/git/gnosis/manifesto.md" -->
> `kvt` is the closest thing to gnosis anyone else has built: markdown as the source
> of truth, a **derived SQLite index with FTS5, links, and frontmatter fields**,
> every write committed to git, exposed over REST and MCP.
<!-- /CITATION -->

### `mnemo_wiki` (Nearest OKF Command Sibling)
`mnemo_wiki` is the nearest Open Knowledge Format (OKF v0.2) command-line sibling, matching the command structure of `exegesis` Phase 1 and adhering strictly to a model-agnostic local design pattern.

<!-- CITATION file="/Users/steve/Documents/git/gnosis/manifesto.md" -->
> The closest sibling: an LLM wiki **stored in OKF**, with a command surface nearly
> identical to our Phase 1 — `init`, `new`, `lint`, `index`, `reindex`, `search`,
> `show`, `links`, `move`, `log`, `validate`. Its statement of the division is
> crisper than ours: **"It never calls a language model itself… The tool is the
> hands; the agent is the head."**
<!-- /CITATION -->

### `context` (Nearest Social Goal Equivalent)
`context` (by Matryer) is the closest equivalent to the social goal of the knowledge base, implementing PR-reviewed, git-versioned Markdown files integrated directly with developer IDEs.

<!-- CITATION file="/Users/steve/Documents/git/gnosis/manifesto.md" -->
> - The closest match to the manifesto's social goal: a team knowledge base where
>   people, projects, and updates are markdown in git, reviewed by PR, and
>   interrogated in whatever LLM IDE each person already uses. Non-engineers
>   contribute through the same workflow.
> - `/questions/` — saved prompts that produce *repeatable* output (weekly status,
>   project health). Determinism obtained by fixing the prompt, not the model.
> - A `GLOSSARY.md` explicitly tuned "so the agents speak your language" — shared
>   vocabulary as a first-class, maintained artifact.
<!-- /CITATION -->

---

## 2. Skill Quality, Scoring, & Rubric Gating

### `AgentLint` (Nearest Deterministic/AI Split Gate)
`AgentLint` maps perfectly to `skillsaw`'s architectural pattern of keeping deterministic checks in the CLI and only consulting prompts when determinism runs out. It externalizes the rubric as data and structures findings with clear actionability.

<!-- CITATION file="/Users/steve/Documents/git/gnosis/manifesto.md" -->
> - 51 deterministic checks + 7 opt-in AI checks, split so the model is only
>   consulted where determinism runs out — the same "mechanism in the CLI,
>   judgement in the prompt" split we chose.
> - `standards/weights.json`, `standards/reference-thresholds.json`,
>   `standards/evidence.json` externalize the rubric: weights, thresholds, and the
>   citation backing each one are data, not code. Rubric changes become reviewable
>   diffs with provenance.
> - Findings carry a fix class (`guided` / `assisted`) so the report says who acts,
>   not just what is wrong.
<!-- /CITATION -->

### `superpowers` (Nearest Skill Axis Peer)
`superpowers` (by Microsoft) is the closest peer on the skill axis, defining a test harness and methodology to verify whether skill changes actually produce behavioral differences in practice.

<!-- CITATION file="/Users/steve/Documents/git/gnosis/manifesto.md" -->
> #### `superpowers` — The Half of Skill Quality This Family Does Not Measure
> 
> Filed twice by the first pass, wrongly both times: once in *Read Shallowly* as an
> "agent harness or runtime" whose composable-skill methodology might matter to
> `steve-skill-market`, and once in *Surveyed and Set Aside* among the skill
> catalogues. It is neither. **It is the closest peer this family has on the skill
> axis, and it holds the one thing `skillsaw` cannot currently produce: a method for
> finding out whether a skill changes behaviour.**
<!-- /CITATION -->

---

## 3. Agent Loops, Orchestration, & Codebase Auditing

### `4x` (Nearest Role-Isolated Loop)
`4x` implements a multi-role pipeline with strict role-isolation to ensure objective, unbiased adversarial review. It enforces guardrails in code (Go) rather than in prompts.

<!-- CITATION file="/Users/steve/Documents/git/gnosis/manifesto.md" -->
> - Role isolation as the anti-self-review mechanism: Design → Code → Review →
>   Test → Deep Review → Accept, where the Reviewer is adversarial by construction
>   and never sees the Coder's reasoning. This is the closest thing we have found
>   to "unbiased peer review" for LLMs.
> - Guardrails (state machine, scope lock, baseline snapshot, evidence gate) are
>   enforced in Go, not by asking the model to please stay in scope.
<!-- /CITATION -->

### `stringer` (Nearest Deterministic Codebase Audit)
`stringer` runs deterministic codebase collector audits to discover latent debt, providing confidence-scored and age-boosted metrics wrapped with strict evaluation governance.

<!-- CITATION file="/Users/steve/Documents/git/gnosis/manifesto.md" -->
> - Fifteen deterministic collectors (TODOs, CVEs via OSV, dependency health,
>   lottery risk, churn, coverage gaps, coupling, doc staleness, config drift, API
>   contract drift) that surface debt already latent in the repo, so agents stop
>   burning tokens rediscovering it.
> - Confidence-scored signals with age-based boosts — a finding carries how much
>   to trust it.
> - One scan, many renderings (markdown / JSON / agent tasks / backlog JSONL):
>   the consumer picks the shape, the analysis is computed once.
> - Ships `eval/` and `GOVERNANCE.md` — the tool is itself evaluated and governed.
<!-- /CITATION -->
