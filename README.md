# gnosis

A collaborative, version-controlled knowledge base and validation harness designed to pool team-wide tribal knowledge, articulate non-functional requirements (NFRs), and systematically improve LLM-agent behaviors through deterministic verification.

---

## Why Gnosis?

- **Collaborative Synthesis:** Pools domain expertise from software engineers, QA, PMs, and support staff into a unified, version-controlled repository using git and Markdown.
- **Accretive Edits:** Ensures agent updates are additive and stable, treating modifications as evidence-backed and verified changes rather than arguing with stochastic models.
- **NFR Articulation:** Explicitly maps out constraints for reliability, security, compatibility, maintainability, and performance, establishing local trade-offs.
- **Deterministic Mitigation:** Standardizes critical-thinking-driven peer reviews and automated checks to mitigate judgement errors in both humans and LLMs.

---

## System Architecture & Pipeline

The system is powered by a shared kernel (`skillet`), four specialized command-line tools, and three agent skills.

```text
book2skill ──▶ exegesis ──▶ skillsaw ──▶ merge-skills
 produce       gate           score        consolidate
                     │
    adh ─────────────┴───────────── canonizer
    (fellow kernel consumers, alongside)
```

### 1. Shared Kernel

*   **`skillet`** (Go library): The unified domain core. Holds common definitions, `speclint` frontmatter schemas, `redlines` quality rules, `skilllens` detectors, `testprompts`, `ratchet`, and `calibration` metrics. Ensures all downstream consumers have a single source of truth.

This library powers the four specialized command-line tools, and those power the three agent skills.

```text
book2skill ──▶ exegesis ──▶ skillsaw ──▶ merge-skills
 produce       gate           score        consolidate
                     │
    adh ─────────────┴───────────── canonizer
    (fellow kernel consumers, alongside)
```

### 2. The Four CLIs

*   **`exegesis`** (Structural Gate): Lints, validates, and gates incoming skill trees. Proves structural validity (graph links, index consistency) and verifies quotation authenticity via `quotecheck`.
*   **`skillsaw`** (Quality & Ratchet): Scores skill quality using a 9-dimension rubric and runs a deterministic keep-or-revert ratchet. It fuses `SkillLens` and `SkillOpt` without ever calling an LLM.
*   **`agentic-dev-harness` (`adh`)** (Arc Loop): Guides codebase modifications through a 5-stage pipeline (Strategy → Execution → Critic → Evaluation → Ops) utilizing its own 5-dimension rubric.
*   **`canonizer`** (Ruleset Grader): Evaluates non-functional and behavioral rulesets against executable specificity, provenance, and specificity, acting as a findings-based grader.

### 3. The Agent Skills (`steve-skill-market`)

| Skill | Role | Responsibility |
| :--- | :--- | :--- |
| **`book2skill`** | Producer | Distills text corpora into structured, compliant skills. |
| **`skillsaw-skill`** | Optimizer | Drives the `skillsaw` CLI through an iterative hill-climbing loop. |
| **`merge-skills`** | Consolidator | Identifies overlaps across raw outputs and builds a unified skill. |

---

## Pipeline Flow

1. **`book2skill`** extracts raw structured guidelines.
2. **`exegesis`** enforces schema and structure.
3. **`skillsaw`** rates qualitative compliance and guides refinement.
4. **`merge-skills`** consolidates redundant knowledge into production-ready skills.
