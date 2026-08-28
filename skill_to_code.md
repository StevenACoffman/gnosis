# End Toil: Resolving the Skill-to-Code Pipeline Through Resilient Reconciliation

This document adapts the SRE principles of **Do-Nothing Scripting**, the **Theory of Constraints (TOC)**, and **Kubernetes-style Reconciliation Loops** to the task of programmatically converting plain-text, declarative markdown `SKILL.md` files into executable, type-safe Go/Python codebases.

---

## Part 1: The 6-Stage SRE Evolution of Skill-to-Code

Boilerplate code generation is a classic developer "toil"—repetitive, manual, tactical, and $O(n)$ with repository scale. We dissolve this toil through your 6-stage SRE model:

```
  [Stage 1: Do-Nothing] ──────► [Stage 3: Do-Something] ──────► [Stage 5 & 6: Unattended]
   Codified checklists           Interactive code gen          Background watch-loops &
   waiting for manual writes     & manual test tweaks          self-healing LLM patch runs
```

### Stage 1: The Do-Nothing Compiler Script
Before automating, encapsulate the logical steps of skill compilation into an interactive script that guides the developer but does *none* of the actual writing.
*   **The Run:** A developer runs `exegesis compile --do-nothing skill/my-new-skill`.
*   **The Steps:** The script prints precise instructions and waits:
    1.  *Prompt:* "Create `internal/skill/my_new_skill.go` and define the input/output Go structs matching the frontmatter schema. Press Enter when done, and I will run `go vet`."
    2.  *Prompt:* "Convert the prompt guidelines into Go system prompt variables. Press Enter, and I will check your link alignment."
    3.  *Prompt:* "Write three test cases inside `test-prompts.json`. Press Enter, and I'll verify they conform to the schema."
*   **The Value:** Encapsulates the compilation slog into distinct Go functions inside the compiler package. You have defined the *seams* of the compilation process before writing any automation.

### Stage 2: Maintain a Localized Skill Runbook
Document the exact translation conventions and common compiler failure modes in your localized `GEMINI.md` and repository standards. Every manual override of a generated struct is recorded as a schema rule, transforming unstructured translation into a predictable, standard runbook.

### Stage 3: The Interactive Do-Something Task Runner
Replace individual manual steps inside your Do-Nothing compiler with automated, scriptable actions—written strictly in your **ubiquitous language (Go)** to preserve static analysis and testability.
*   **The Run:** Instead of waiting for the developer to write the Go structs, Stage 3 inlines an AST parser (`go/ast`) that automatically reads the `SKILL.md` frontmatter schema and generates the exact Go type declarations (`type Input struct`, `type Output struct`).
*   **The Integration:** It then invokes a local LLM or template to draft the system prompt wrapper, pauses, and prompts the user: *"Generated skeleton written. Inspect `my_new_skill.go`. Press Enter to proceed to automated test execution."* You have automated the low-hanging boilerplate while keeping human inspection at the boundaries.

### Stage 4: Fire-and-Forget (Observability-Driven)
Remove human babysitting. The compiler runs completely unattended, outputting a durable ledger of outcomes and triggering notifications only when compile-time assertions are breached.
*   **The Run:** Running `exegesis compile` executes the entire generation, writes the code, compiles it, and executes the unit test suites silently in the background, logging outputs to `compile.log`. If compilation or tests fail, it writes a structured JSON error trace to `diagnostics.json` and alerts the developer via CLI status alerts.

### Stage 5: Event-Driven Triggers
Hook the compiler directly to filesystem events instead of manual execution.
*   **The Run:** A background filesystem watcher (using `fsnotify` in Go) monitors your skills directory. The moment a developer or agent edits a `SKILL.md` file, the event triggers your Fire-and-Forget compiler pipeline instantly.

### Stage 6: Remove from Runbook (Total Dissolution)
The conversion process is no longer a task. You do not "compile" skills anymore; writing a `SKILL.md` is mathematically and procedurally equivalent to writing code.

---

## Part 2: Applying the Principle of Reconciliation to Skill-to-Code

Traditional code generation is highly fragile: it is **edge-triggered**. An event (like a file save) triggers a generator. If the generator outputs buggy code, or if a prompt changes and breaks a test, the pipeline crashes, state drifts, and compilation breaks.

Adapting your article's core thesis means building a **Level-Based Skill Reconciler Loop**:

```
      ┌────────────────────────────────────────────────────────┐
      │                                                        │
      ▼                                                        │
   [ WATCH ] ──► Observe SKILL.md Frontmatter & Prose          │
      │                                                        │
      ▼                                                        │
   [COMPARE] ──► Compare Desired Specification against         │ (Idempotent
      │          Actual Go Code AST & Test Execution State     │  Reconciliation
      ▼                                                        │  Loop)
   [  ACT  ] ──► Regenerate Structs / Auto-heal Code           │
      │          via Self-Repairing LLM Test Loops             │
      │                                                        │
      └────────────────────────────────────────────────────────┘
```

### The Desired State vs. The Actual State
Every change to your skill graph creates a tension between:
*   **The Desired State (Declarative Spec):** The plain-text markdown `SKILL.md` (defining schema, constraints, and prompt-guidelines).
*   **The Actual State (Imperative Reality):** The generated Go source files (`skill.go`), the test assertions (`test-prompts.json`), and the execution status of `go test` (must pass 100%).

### The Reconciler Feedback Loop
The compiler acts as a continuous background reconciler:
1.  **Watch:** Continuously read `/skills` directory specifications and current generated Go codebases.
2.  **Compare (The Delta):** Compute the delta between desired and actual:
    - *Has the frontmatter schema changed?* (Delta = Go structs do not match YAML/TOML specifications).
    - *Has a prompt guideline been added?* (Delta = Code prompt templates are out of sync).
    - *Do the tests pass?* (Delta = Compilation fails or `go test` exits with non-zero code).
3.  **Act (Idempotent Healing):** Safe, repeatable steps to close the gap:
    - If structs are out of sync, regenerate the Go AST files.
    - If `go test` fails due to compilation errors or failing test assertions, Gnosis/Exegesis invokes a **Self-Repairing LLM Loop**: it feeds the `go test` compiler error back into a localized, code-restricted patch prompt, generates a targeted fix, overwrites the code, and re-runs the tests.
    - It repeats this loop autonomously until the actual code behavior matches the desired skill spec (tests pass).
4.  **Repeat:** Forever.

---

## Part 3: The Theory of Constraints (TOC) Perspective

What is the ultimate bottleneck (the constraint) of scaling your agentic architecture? 

It is the **cognitive load and toil of writing safe integration layers, prompt structures, and typing wrappers** for every new domain capability you add to the system. 

By applying Step 2 of the TOC (*"Decide how to exploit the constraint"*):
*   You subordinate the entire codebase generation to the declarative `SKILL.md` specification.
*   Developers and agents **only write Markdown**. They never write the boilerplate Go structs or prompt-wiring code.
*   The system treats prose and frontmatter schemas as the *only* source of truth, and the reconciler automates the rest, elevating your development throughput.

---

## Part 4: Comparative Analysis of Tool Placement

To decide where this Skill-to-Code compiler and reconciler functionality should be hosted, we evaluate the pros and cons across each of the 6 core systems in your tool suite:

### 1. `skillet` (The Shared Go Kernel)
*   **Pros:**
    - Shared AST generators, frontmatter parsers, and YAML validators instantly become available to all other 6 tools as zero-dependency library utilities.
    - Prevents any schema parsing or serialization contract drift at the shared data-structure level.
*   **Cons:**
    - Violates `skillet`'s core design charter: skillet is a collection of pure, lightweight, "deep" library modules.
    - Introducing file-system watch loops, LLM-based self-healing pipelines, and external test execution introduces heavy operational dependencies and external system calls (running `go test`), which would turn the kernel into an interactive, heavy shell runtime.

### 2. `canonizer` (Independent Ruleset Grader)
*   **Pros:**
    - Canonizer already understands ruleset evaluation and operates on standard constraints.
*   **Cons:**
    - Complete mismatch with `canonizer`'s design charter: Canonizer's single responsibility is to grade and report code compliance statically against rulesets. It is a read-only judge, not a writer.
    - Hosting code-generation loops or compilation triggers inside a static grader creates a circular dependency where the judge is also responsible for authoring the code it grades.

### 3. `agentic-dev-harness` (`adh` — Change Harness)
*   **Pros:**
    - Excellent operational alignment: `adh` is built specifically to manage execution and validation loops. It has the harness infrastructure to run code, parse compilation errors, intercept test results, and invoke self-repairing LLM patch loops.
    - Matches the SRE "reconciliation" philosophy, as `adh` is the master loop runner managing code change transitions and test gates.
*   **Cons:**
    - `adh` is a generic execution harness meant to run over *any* codebase (it takes a workspace, runs steps, checks gates). Putting skill-specific markdown compilers and Go AST generators inside `adh` would tightly couple a generic harness to the specific skill-card formatting of `exegesis` and `steve-skill-market`.

### 4. `exegesis` (Skill Structure Gater & Distiller)
*   **Pros:**
    - Perfect domain alignment: `exegesis` already parses, validates, distills, and topologically sorts your skill card trees and schemas. It is the compiler layer of your skill graph.
    - Exegesis is the direct interface to the skill market; it natively understands `SKILL.md` files, validation schemas, and frontmatter constraints.
*   **Cons:**
    - Currently designed as a pure, lightweight Go CLI. Adding LLM integrations (for self-healing) and background watch deamons (`fsnotify`) introduces heavier, stateful runtime dependencies.

### 5. `skillsaw` (Skill Quality Scorer & Optimizer)
*   **Pros:**
    - High thematic alignment: `skillsaw`'s purpose is to score and optimize skill quality, managing local keep/revert evaluation loops.
*   **Cons:**
    - Designed strictly as an evaluation engine (scoring prose and test composition against a discrete 9-dimension rubric). Moving into imperative code generation, AST file writing, and Go compiler execution diverges from its scoring charter.

### 6. `gnosis` (Developer Knowledge Ingestion & Curation)
*   **Pros:**
    - Gnosis already manages plain-text developer knowledge ingestion, accretion, and curation.
    - Gnosis owns the background accretion daemons and ingestion queues, allowing it to easily trigger compilation updates on filesystem changes.
*   **Cons:**
    - Gnosis operates on general developer knowledge and general plain-text, storing them as raw facts in a database. It does not natively understand compiled skill-market dependencies, topological sorting of skill cards, or Go compilation targets (which are specifically `exegesis`'s domains).

---

## Part 5: The Recommended Synthesis (Clean Architecture)

To enforce **Exception Reduction** and **Deep Modules** (minimizing system complexity), we should not dump this entire feature set into a single repo. Instead, we distribute the labor based on functional seams:

| Tool | Role in Skill-to-Code Pipeline | Why |
| --- | --- | --- |
| **`skillet`** | **The Library Primitives** (`skillet/skill`, `skillet/frontmatter`) | Exposes AST models, schema verification, and parsing helpers as a pure, static kernel. |
| **`exegesis`** | **The Compiler Core** (`exegesis compile` engine) | Reads the markdown skill, generates the Go AST type structs and prompt variables, and compiles them. |
| **`agentic-dev-harness` (`adh`)** | **The Loop Reconciler & Self-Healer** (`adh reconcile`) | Establishes the `watch` trigger, invokes the compiler core (`exegesis compile`), executes tests, catches errors, and manages the self-healing LLM patch loop. |

---

## Part 6: Externalizing to a New Focused Tool vs. Core Monoliths

Rather than hosting the reconciler inside your 7 existing tools, we evaluate the architectural trade-offs of creating a brand new, highly focused single-purpose tool (e.g., `skillc` or `skillect`), sharing only parsing primitives in `skillet`.

### The Pros of a Dedicated Tool
1.  **Absolute Single Responsibility Principle (SRP):**
    A dedicated compiler tool has one job: translate `SKILL.md` to code and reconcile its test state. It separates the **Compiler** domain from the **Gating** domain (`exegesis`) and the **Execution** domain (`adh`), preventing feature creep in other CLI tools.
2.  **High Cohesion and "Deep Modules":**
    It can expose a single, simple CLI interface (e.g., `skillc reconcile --watch ./skills`) while wrapping the entire complex implementation of file-watching, code-writing, test runners, and LLM self-patching behind the scenes.
3.  **No Dependency Bloat in Core Repos:**
    Downstream tools (`exegesis`, `skillsaw`) stay lightweight, compiled Go binaries. Only the new compiler tool takes dependencies on filesystem watchers (`fsnotify`), LLM client SDKs, and OS shell commands.
4.  **Independent Release Cadences:**
    Changing a prompt-generation template or updating a self-healing LLM logic block does not require a kernel-level `skillet` release or a coordinated rebuild of `canonizer` or `skillsaw`. You isolate development churn.

### The Cons of a Dedicated Tool
1.  **Toolbox Fatigue (Cognitive Load):**
    Adding an 8th tool increases the cognitive overhead for onboarding human developers and AI agents, expanding the surface area of tool configurations and environment variables.
2.  **Release-Lag Bottleneck:**
    Sharing common code in `skillet` means that any change to the shared `frontmatter` or `skill` models requires a traditional "two-commit evolution" (releasing `skillet` first, then updating `go.mod` in the new tool) which slows down initial experimental prototyping.
3.  **Duplicate Boilerplate:**
    Every Go CLI tool requires its own command structure (e.g., `climax` or `ff/v4`), flag declarations, slog logs, and environment configurations, leading to duplicated CLI skeletons across your workspace.

### The Recommendation: The SRE "Extract-Refactor-Promote" Compromise
*   **Phase 1 (In-Tree Prototype):** Write the compiler logic directly inside `exegesis` first. It is the direct manager of `SKILL.md` documents, allowing you to iterate on Go AST code generation rapidly in a single repository without upstream `skillet` release lags.
*   **Phase 2 (Promote and Extract):** Once the code-generation, prompt-templating, and self-repairing loops are stable and mature:
    - Push the stable AST and parsing data structures to the `skillet/skill` library.
    - Spin out the background watchdog and LLM reconciler into a dedicated single-purpose tool (`skillc`), keeping the other downstream tools pure and decoupled.
