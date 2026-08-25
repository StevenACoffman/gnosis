# Gnosis Implementation Plan

Implements [`SPEC.md`](./SPEC.md); the backlog is [`TODO.md`](./TODO.md).
Reconciled against both on 2026-08-20 — see §6.4 for what the specification
changed under this plan. Governed by
`~/Documents/agent-orange/go-advice/summary_rules.md`, referenced below as
**§rules N**.

The plan is deliberately phased to match SPEC §19, and each step ends in a
committable state: contract comments written, pure core tested, shell wired,
`golangci-lint run ./...` clean without relaxing any rule.

______________________________________________________________________

## 0. Decisions This Plan Resolves Before Any Code

### 0.1 Package Layout, and the Tension in It

**§rules 1** prescribes: domain types in the root package, `main` under
`cmd/<name>/`, adapter subpackages named after what they wrap, and *no
subpackage importing another subpackage*. Its signal test is "if `go doc .` would
show domain types and service interfaces, use `cmd/<name>/main.go`."

gnosis has substantial domain — documents, concepts, evidence, subjects, links —
so that test points at restructuring the scaffold. **This plan does not do that,
and the reason is recorded here rather than left implicit.**

- `skillet` is already the family's root domain package, and the other four
  consumers (`exegesis`, `skillsaw`, `adh`, `canonizer`) all use `main.go` at
  root with `internal/*` beneath. **§rules 15** requires following conventions
  already present, and a fifth consumer inventing a sixth shape is the "variant
  pattern for personal preference" that rule prohibits.
- The prohibition on subpackage-to-subpackage imports exists to prevent circular
  dependencies. A strictly layered `internal/adapter → internal/gnosis` import is
  acyclic by construction, and the layering is enforced by a lint rule (0.4).
- gnosis's domain types are **promotion candidates, not shared code**: under the
  family's promote-on-second-consumer rule they cannot move to `skillet` until a
  second tool needs them. Until then `internal/` is the honest home, because
  `internal/` says "not yet API" in a way a root package cannot.

Resulting layout:

```text
gnosis/
  main.go                  package main — signal handling + exit translation only
  cmd/                     command infrastructure (climax); one package per command
    cmd.go                 dispatcher
    root/                  shared Config, ExitError
    <command>/             one per SPEC §8 command
  internal/
    gnosis/                THE DOMAIN. Plain types, service interfaces, no I/O imports
    okf/                   OKF parse/render/conformance over skillet/frontmatter+markdown
    ontology/              types, subjects, aliases, dimensions (SPEC §5.7)
    index/                 SQLite: schema, migrations, queries (SPEC §5.4)
    archive/               tier-0 evidence store (SPEC §4.2–4.4)
    lint/                  the check registry (SPEC §12)
    web/                   authenticated viewer (SPEC §13) — Phase 5
```

`internal/gnosis` imports nothing from `internal/*`. Every other `internal/*`
package imports `internal/gnosis` and never each other.

### 0.2 Testing Posture

**§rules 9** requires calibrating investment rather than applying a uniform
standard. gnosis is **tended** (a human can revert a commit), **long-lived**, and
**moderate risk** — it cannot hurt anyone, but it can silently corrupt a corpus
the team relies on. That places it at *"TDD for design; regression for changes"*,
with one deviation upward:

**The admission gates get untended-grade treatment.** Quote validation,
conformance, identity reconciliation, and the promote gate are the checks that
decide what enters the corpus. A silent false pass there is not recoverable by
reverting, because the corrupt claim outlives the commit. These get property
tests and two-sided mutation tests (SPEC §18.2) regardless of what the table
above would suggest.

### 0.3 What Gets a Contract Comment Before a Body

**§rules 4 and 9** require writing the interface comment first, and treating a
long tangled comment as evidence the interface is wrong. Applied here: every
exported function in `internal/gnosis`, `internal/okf`, `internal/ontology`, and
`internal/archive` gets `// Requires:` / `// Ensures:` before its body exists.
If `Requires` needs more than one sentence, the function is split before it is
written.

### 0.4 Layering Enforced by Lint, Not Vigilance

**§rules 15** says to enforce conventions with linters. Before any domain code
lands, add an `importas`/`depguard` rule to `.golangci.yaml` forbidding
`internal/gnosis` from importing `internal/*`, and forbidding
`internal/*` from importing each other. **This adds a rule; it relaxes none.**

______________________________________________________________________

## 1. Step 0 — Clean the Baseline

`golangci-lint run ./...` currently reports four issues in the scaffold:

| File                  | Linter   | Issue                                                         |
| --------------------- | -------- | ------------------------------------------------------------- |
| `cmd/root/root.go:21` | decorder | `type` after `func`; required order is const, var, type, func |
| `cmd/cmd.go:21`       | gci      | import grouping                                               |
| `main.go:12`          | gci      | import grouping                                               |
| `main.go:14`          | gofumpt  | formatting                                                    |

**Exit criteria:** `golangci-lint run ./...` reports zero issues; `go build ./...`
and `go vet ./...` clean; `main.go`'s `run()` still matches **§rules 7 Shape B**
verbatim, including the preserved comment.

No behaviour changes in this step.

______________________________________________________________________

## 2. Phase 1 — Read

SPEC §19 Phase 1: parse and render OKF, seed an ontology, build and verify the
derived index, and read the corpus. No ingestion, no model, no network.

### Step 1.1 — Domain Types (`internal/gnosis`)

Plain structs and service interfaces only. No `database/sql`, no `net/http`, no
third-party import beyond `skillet`.

```go
type Document struct {
	ID     ID     // UUIDv7; immutable (SPEC §5.1)
	Path   string // current location; may change
	Slug   string // advisory, derived from title
	Hash   string // skillet/identity.Hash of the file bytes
	Claims []*Claim
}

// Claim is one addressable assertion inside a document (SPEC §5.0).
// Phase 1 constructs none of these: identifying claims needs extraction, which
// is Phase 2. The type is defined now because the index schema references it and
// because getting AnchorHash right is what makes Phase 2 non-destructive.
type Claim struct {
	ID          ID
	DocumentID  ID
	AnchorHash  string // fold hash of the anchoring text — the address (§5.5.1)
	Pos         *int   // bytes from start of BODY; nil = anchor not located (§5.5.2)
	Type        TypeKey
	Title       string
	Description string
	Lead        string // the conclusion, stated first (§17.4)
	Status      Status // draft | stable | deprecated
	StaleAfter  *Date  // nil means never declared
	Evidence    []Evidence
	Subject     *SubjectKey
}
```

Interfaces per **§rules 2**: `DocumentService`, `ClaimService`, each with godoc
naming the error codes it returns, filter structs with pointer fields,
`FindByID` never returning `(nil, nil)`.

`Error` is **not** redefined — gnosis uses `skillet/errs` with the family's five
codes, per the shared-kernel rule.

Modelling note per **§rules 4**: `ID`, `TypeKey`, `SubjectKey`, and `Status` are
named types with validated constructors, not bare strings. A `Status` that can
only hold three values needs no validation at its use sites.

**Tests:** constructors reject invalid input; `Status`/`TypeKey` round-trip.
Nothing that merely re-tests the type system (**§rules 4**, last prohibition).

### Step 1.2 — OKF Parse and Render (`internal/okf`)

Pure functions over bytes. `skillet/frontmatter.Split` separates YAML from body;
`skillet/markdown.Parse` yields the body; `goccy/go-yaml` decodes frontmatter.

```go
// Requires: src is a UTF-8 OKF concept document.
// Ensures:  returns the parsed document with unknown frontmatter keys preserved;
//
//	returns ECONFORMANCE only for the three OKF §11 conditions.
func Parse(src []byte) (*gnosis.Document, error)
```

The conformance checker is where OKF's **negative** requirements live, and they
are the part most likely to be got wrong by someone reading only the happy path:
unknown `type` values, unknown extra keys, broken links, and a bare `verified`
mapping must all be **accepted**.

**Tests (property-based, per §rules 9):**

- `Render(Parse(x)) == x` for every fixture — round-trip is the invariant that
  makes rewriting safe.
- Unknown keys survive a round trip.
- Each of OKF §11's "MUST NOT reject" conditions has a fixture asserting
  acceptance. These are the tests that stop a future contributor from
  "tightening" conformance into non-conformance.

### Step 1.3 — Ontology (`internal/ontology`)

Load and validate `ontology.toml` (SPEC §5.8): types with
`normative`/`expects_subject`/`template`/`aliases`, subjects with
`dimension`/`aliases`.

The behavioural-identity rule is a function, not prose:

```go
// Requires: a, b are declared types.
// Ensures:  reports whether a and b are behaviourally identical and should
//
//	therefore be one type with two aliases (SPEC §5.8.1).
func Identical(a, b Type) bool
```

`init` seeds the five starter types and **zero** subjects.

**Tests:** alias resolution is deterministic and case-folded via
`skillet/textnorm`; `Identical` has a table covering each flag differing alone.
A duplicate alias across two keys is an error **and the diagnostic names the
remedy** — §5.8.2.1 requires both branches (merge, or two distinct keys), because
the obvious repair is deleting an alias, which makes the file load and leaves the
ambiguity in place.

### Step 1.4 — Index (`internal/index`)

SQLite via `modernc.org/sqlite`. Migrations are numbered steps applied off
`PRAGMA user_version` (SPEC §5.5), append-only because SQLite cannot
`DROP COLUMN`.

**§rules 8** governs everything here:

- Service methods own the transaction boundary; unexported helpers take `*Tx`.
- `defer tx.Rollback()` immediately after `Begin`.
- `make([]*T, 0)`, never `var`.
- `rows.Err()` after every loop, `defer rows.Close()` after every query.
- Dynamic `WHERE` built from a `[]string{"1 = 1"}` seed with bound args.
- Sort order from a closed `switch`, never an interpolated string.
- No transaction in any exported signature.
- **`claims.pos` is nullable and modelled as `*int`.** SPEC §5.5.2: `0` is a valid
  position — the first byte of the body — so it cannot double as "anchor not
  located", and a caller that read it that way would send readers to the top of
  the document. This is the one column where the zero value is a real answer.

The FTS5 configuration is copied verbatim from SPEC §5.5 including the tokenizer,
because the custom `tokenchars` are load-bearing for technical prose. **The
tokenizer string is one Go constant**, shared by `documents_fts` and `claims_fts`:
two copies drift, and the corpus would then search differently depending on which
table answered.

**Tests:** a real temporary database, not a mock — **§rules 10** prefers the real
thing where it is cheap, and an embedded SQLite file is cheap. Round-trip every
table; assert `rebuild` twice yields identical content hashes (SPEC §18.3).

Positions get a property test rather than an example, because both halves of the
obvious one look right: assert a recomputed `pos` points at text whose fold hash
equals the stored `anchor_hash`, not merely that rebuild produced some offset. The
failure that catches is a search matching the wrong occurrence of a short anchor —
a plausible number pointing at the wrong paragraph, with no error anywhere.

That byte-identical property is load-bearing for a reason the plan originally did
not know: the index is **per-user** (§4.6), so two people at one commit must hold
the same index or a disagreement between them could be about their caches rather
than about the corpus.

### Step 1.5 — Reconciliation (`internal/index`, Pure)

The six cases of SPEC §5.1.2 as one pure function over observed state. This is
the clearest FCIS boundary in the whole tool: scanning the filesystem is shell,
deciding what each discrepancy means is core.

```go
// Requires: observed lists every document found on disk with its frontmatter id;
//	indexed lists every row currently in the index.
//
// Ensures:  returns one Resolution per discrepancy, and never more than one per
//
//	(path, id) pair; DuplicateID resolutions carry both paths and no winner.
func Reconcile(observed []Observed, indexed []Indexed) []Resolution
```

**Tests:** table-driven over all six cases plus the benign slug-drift case. The
duplicate-ID case asserts **no winner is chosen** — that is the property most
likely to be "helpfully" broken later.

### Step 1.6 — Lint Registry (`internal/lint`)

Each check is a pure function returning `[]finding.Diagnostic`. Phase 1 ships
**conformance, identity, index-drift, broken-link, orphan, log-format**, and
`schema-shape`.

Per SPEC §12, two properties are structural rather than per-check: applicability
is **derived** (an orphan check is meaningless in a corpus with no links yet), and
every run reports what it **skipped and why** — mandatory, because derived
applicability makes it possible to lint clean by not applying, and that state is
indistinguishable from health in any output that omits the skips.

This package holds **two** passes, not one, and keeping them apart is the point: a
`Snapshot` describes the knowledge and its findings say the corpus is wrong; an
`Environment` describes the apparatus and its findings say gnosis *cannot judge*
whether the corpus is wrong. A vocabulary that will not load belongs to the second.

**Tests:** each check gets a fixture that fires it and a fixture that must not.
`broken-link` asserts a missing target is reported as a gap and **never** as an
error.

### Step 1.7 — Commands

`init`, `doctor`, `index rebuild [--check]`, `show`, `search`, `graph`, `lint`.

Per **§rules 7 Shape B** and the climax conventions already in `cmd/`: one
package per command, a `Config` embedding `*root.Config`,
`ff.NewFlagSet(name).SetParent(parent.Flags)`, registration at the
`// climax:imports` marker. Every command supports `--jsonl`, keeps data on
stdout and diagnostics on stderr, and returns `root.ExitError` for a specific
code.

Commands are **shell** (**§rules 5**): a flat sequence of load, call core, write.
No command contains a conditional that decides domain meaning.

**Tests:** `cmd.Run` end-to-end with injected I/O per **§rules 10**, asserting
exit codes and stdout/stderr separation. The envelope shape is asserted, not just
the exit code: a corpus with problems must decode as `status: findings, code: 3`
and never as an error, because that difference is the whole reason the vocabulary
exists.

______________________________________________________________________

## 3. Phase 2 — Ingest with Proof

SPEC §19 Phase 2: tier 0, `fetch`, the scan stage, the ingest/admit relay with the
response cache, quarantine, the promote gate, `log.md`, the audit trail. The corpus
starts accumulating and every claim is traceable from the first one.

Ordered so each step unblocks the next and each ends committable.

### Step 2.1 — Claim Segmentation (`internal/segment`)

The blocking item, and the one that needs no decision: §9.4 commits to the
algorithm and to the guarantee.

> **Every emitted claim stands on its own, or the cut is not made.**

Pure, deterministic, no model. The reference implementation is Swift, so the
algorithm and the guarantee transfer and the code does not.

```go
// Requires: text is one document body or one paragraph of it.
// Ensures:  every returned claim is independently verifiable — no claim's subject
//
//	sits in a discarded sibling; concatenating the claims loses no assertion;
//	it is pure.
func Claims(text string) []Claim
```

The failure cases are known and are the whole difficulty. `split(".")` cuts
`2.5 seconds`; an abbreviation list still cuts `e.g.`, `README.md`, `foo.bar()`,
`https://example.com/a.html`, and `A. Turing`; splitting on newlines cuts every
hard-wrapped paragraph. The stand-alone rule is what makes over-splitting *safe*
rather than merely tolerable: a fragment whose subject was discarded fails the rule
and the cut is refused, so the sentence stays whole.

**Tests are untended-grade per §0.2**, because a wrong cut is a silent false pass in
the check the corpus most depends on. Property tests: concatenation preserves every
assertion; no emitted claim is a strict prefix of a discarded subject; the six named
splitter traps each have a fixture; and a two-assertion sentence
(*"The cache is enabled by default, but it is not shared across sessions"*) splits
into two claims each carrying its own subject, which is §5.5's worked example.

### Step 2.2 — Standards (`internal/standards`)

TOML under `standards/`, loaded with `md.Undecoded()` strictness like the ontology
(§5.2). Phase 2 needs `archive.toml`: the extension allowlist, the size cap, the
pinned HTML extractor and its version, the staleness window.

Every value carries a `rationale` (§6.2), and a value moved in the
finding-reducing direction records that it was — the mechanism that keeps
`standards/` from becoming the place inconvenient checks go to die.

### Step 2.3 — Tier 0 (`internal/archive`)

The content-addressed store and the ledger, both now fully specified (§4.2–4.3.1).

- `text/<sha256[:2]>/<sha256>.<ext>` for archived text.
- `fetch/<h[:2]>/<h>.json`, one immutable record per source version, `h` over the
  canonical record with **no timestamp** (§4.3.1).
- Three dispositions decided by §4.3's rules, never by a caller.
- Sanitization refuses and never repairs (§4.4); SVG is active content.

**Tests:** a re-fetch of unchanged bytes writes nothing and is observably a no-op;
a changed source produces a second record and leaves the first untouched; a
rejected file records its `reject_reason` and falls through to `referenced`; two
independent writes of one source produce byte-identical records, which is what
makes the ledger merge (§4.6.1).

### Step 2.4 — Fetch (`cmd/fetchcmd`)

Shell over the archive. Four adapters and no more (§9.2): local file, directory,
URL, git repository. The hash is recorded for every fetch including `referenced`.
The HTML extractor **strips boilerplate**, and its identity is recorded with the
record so a re-extraction by a different stripper is visible rather than silent.

### Step 2.5 — Command Types, Then a Coordinator

Before the second writer exists, per §4.6.2. `Promote`, `Effect` with a zero value
that fails closed, and `Execute(ctx, Command) (Outcome, error)` where `Outcome` is
§8.0's envelope. Serialisation and transport come when a caller needs them; the
type comes first because §9.4's guarantee derives from it.

Serialisation of writes starts as an advisory `flock` on `.gnosis/writer.lock` —
correct for `init` and `index rebuild`, and explicitly a step whose ceiling is
known: a lock carries no command.

### Step 2.6 — Quarantine and the Promote Gate

Tier 1 under `.gnosis/quarantine/`, trust `unverified`, not in the bundle. The gate
runs over a **diff** and the writer applies exactly what the gate approved (§9.4),
which the command type from 2.5 is what makes possible.

### Step 2.7 — Relay: `ingest` and `admit`

Two-phase: `ingest` emits prompts and suspends, an agent supplies the reasoning,
`admit` consumes the reply. The response cache is keyed
`(source content_hash, prompt hash, model + version)`, so a second run over
unchanged inputs makes no model calls and reproduces byte-identically — the
cheapest determinism win available (§6.1). `--cache-only` refuses to emit and exits
non-zero listing what is missing; CI uses it.

`quotecheck` wires in here, with the `Unchecked` outcome mattering for the first
time: a claim whose passages were never checked must not read as clean.

### Step 2.8 — `log.md` and the Audit Trail

OKF §9 date headings for the log; `.gnosis/audit.jsonl` for every write, per-user.
The `deferred` finding state and reader challenges are committed frontmatter
(§10.7.4), not audit rows — decisions are committed, observations are cached.

______________________________________________________________________

## 4. Later Phases — Scope Only

Recorded so the Phase 1 interfaces are shaped for them, not built now.

| Phase      | Adds                                                                                       | Key constraint from the rules                                                                                                                                       |
| ---------- | ------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 2 — Ingest | `archive`, `fetch`, scan, relay, quarantine, promote gate, **claims**, `challenge`         | Fetch is shell; every gate is pure. Four adapters, one normalizing seam. Segmentation precedes the evidence invariant                                               |
| 3 — Curate | `conflict`, `critic`, `gate`, `adjudicate`, `supersede`, subjects accrete, **trust tiers** | Predicates are gnosis's own — `skillet/ruleset/conflict` exists and does not fit (§Blocking). The tier fold reads raw actor strings, never `gnosis.Actor` (§14.1.1) |
| 4 — Scale  | derived constraints, operator patterns, optional rerank                                    | Operator patterns are data with a test corpus, never regexes in Go                                                                                                  |
| 5 — Serve  | authenticated web viewer, review queue                                                     | `NewServer(...) http.Handler`; `addRoutes` never errors; explicit NotFound and healthz                                                                              |

Two Phase 2 items are easy to under-scope because Phase 1 does not need them and
both are expensive to retrofit:

- **The write coordinator** (§4.6). One writer per user, and it owns the *bundle*
  rather than merely the database — serializing SQLite writes while leaving markdown
  writes unserialized coordinates the cache and not the corpus. Readers stay
  independent of it by requirement, which is what keeps Phase 1 unaffected.
  Build the **command type before the transport** (§4.6.2). Writes are values with
  their own gating fields, so review-gating binds every caller including the
  internal ones, and §9.4's approved-diff-is-the-committed-diff property follows
  from preview and apply being one handler rather than two. An advisory lock is a
  fine first step for serialisation and can never supply that property, because a
  lock carries no command.
  **Status 2026-08-22: the command type is built and the transport is deferred with a
  trigger.** `internal/command` carries `Command`, `Promote`, `Admit`, and a fail-closed
  `Effect`, so the "type before transport" instruction above is satisfied and the advisory
  lock is now the *correct* answer rather than a first step — every writer is in this
  process. The trigger is the first writer that is not: §13's served viewer, or an agent
  runtime calling `admit` directly.
  One thing the sentence above understates, now recorded in §4.6.2: preview-and-apply being
  one handler gives the property **only while both see the same input**. In process the
  lock spans compute-and-write and that is free; across two round trips the corpus can move
  between them, so a served coordinator needs one round trip or an expected revision on the
  apply. That is the prerequisite a transport has to meet, not a detail of which one.
- **Claim anchors before any claim row is written** (§5.5.1). A claim's identity
  and address live in the document. Writing claims first and adding anchors later
  means every claim written in between has an identity no rebuild can recover.

A third item is cheap now and expensive later, and unlike those two it is not a
Phase 2 dependency but a Phase 3 one that must be paid in Phase 2:

- **The OKF conformance table test** (§18.5.1). **Done, and it needed the fold with
  it.** `gnosis.Actor` rejects two of OKF §7's three actor forms, and §14.1's tier fold
  must therefore read raw strings rather than the parsed type (§14.1.1) — so asserting
  "what tier the fold yields" meant building `gnosis.FoldTrust`, a pure function over
  `[]string`, before its consumer exists. That is what §18.5.1 asks for and the reason
  it gives: the divergence is already in a shipped type, and the merge that breaks
  conformance is the natural thing to write. It is also what `skillet`'s revised
  promotion trigger watches, and a function with no receiver lifts unchanged.
  What is *not* done is §14.1 itself — deriving and reporting a tier for a document.
  That is Phase 3, and the fold now waiting for it is the point rather than a gap.

**Nothing is blocked, and all three recorded blockers resolved differently than this
paragraph predicted** — kept rather than deleted, because two of the three were
resolved by being *wrong* and that is worth more than the prediction was.

- `quotecheck` with the checked/unchecked third outcome **shipped** in `skillet`
  v0.18.0; `Unchecked` is the zero value. Only the comparison was promoted —
  `Segment` and the quotation extraction stayed in exegesis, because where a
  quotation begins is precisely what a shared package must not know.
- The **claim segmenter exists** (Step 2.1, `internal/segment`). The Swift reference
  supplied the guarantee and none of the code, as expected.
- **`skillet/ruleset/conflict` was never a blocker.** The package already existed, and
  it reads four fields off a `ruleset.Rule` that a gnosis claim does not have. The
  overlap is zero; what the two share is a shape, and a shape is followed rather than
  imported. Phase 3 writes its own predicates.

The live Phase 3 dependency was not a promotion at all: it was the OKF conformance
test above, which had to land before §14.1 rather than with it. **It has, so Phase 3
now begins with no arrears** — and the fold it required is sitting there waiting for
§14.1 to call it, which is the state that paragraph was written to produce.

### 4.1 Small Items the `agent-green` Survey Added

Each is specified now and none is large. They are listed together because they came
from one source and would otherwise be scattered across four phases.

- **The rebuild floor** (§4.5). **Done.** It cost less than budgeted: the previously
  indexed count is `len(indexed)`, which `indexcmd` already loads to compute drift,
  so the planned meta row was unnecessary.
- **`standards/promote.toml`** (§9.5). **Done**, holding both the hedging limit and
  the rebuild floor.
- **Corruption versus operational failure** (§15). **Done.** `AuditTrail` and
  `LoadChecks` name a malformed line as corruption and carry its line number.
  `StoreEvidence` already drew the line for the case that matters most — differing
  bytes at a content-addressed path is ECONFLICT rather than a quiet no-op. §15 now
  records the limit: `errs` has no corruption code, so this is legible rather than
  machine-checkable, and a sixth code belongs at the second consumer.
- **Freshness at the point of reading** (§14.3). **Done, and now at both grains.**
  `lint`'s `stale` check and `show`'s freshness line both land, joined by
  `bundle.LoadFreshness`. The §14.3.0 distinction fell out of building it —
  `stale_after` governs the claim, `staleness_days` governs the check — as did the
  decision that never-checked is a state rather than a finding. Per claim as well as
  per document since §6.19: one measurement used twice, with the document line still
  the weakest of its claims, so the conservative answer was added to rather than
  replaced.
- **A relay test with a scripted model** (§18.6). **The scripted one is built; the
  real-model run is not.** §18.6 specifies all three methods and what each is for: the
  scripted model proves the contract and belongs in CI; a real-model run graded by a
  pure predicate over the transcript proves a live model can satisfy it and must stay
  out of the gate. §18.6.1 records what building it settled — "a local server speaking
  the model protocol" needed translating, because gnosis speaks no model protocol, so
  the seam is prompt file → agent → reply file and the server is a function that can
  see only the prompt. The real-model run stays unbuilt and stays out of the gate.
- **An `AI_POLICY.md`** for this repository. Not a code change and not a spec change
  — a repository is a corpus, §1.1 says a claim must name its witness, and this one
  does not.

______________________________________________________________________

### 4.1.1 What the `agent-green` Deep Reads Added (2026-08-22)

Three repositories the survey had filed as read-shallowly were opened:
`oh-my-agent`'s judge protocol and event specification, `ruflo`'s optimisation logs,
and `hindsight`'s benchmark harness. Six items, sequenced by what they block rather
than by size, because two of them are cheap now and expensive later.

**Do with Phase 2, because the write path is being touched anyway:**

- **A mutation verifies its own audit row** (§15). **Done.** It was not one
  function: `init` and `index rebuild` append outside the coordinator, so the
  verifying append is the exported one and the bare append is unexported, which
  makes the compiler enforce what a source-scanning test was briefly asserting.
- **`bundle.AuditTrail` counts malformed lines rather than skipping them** (§15).
  **Done**, as a `Trail` value with a `Whole()` method rather than a value plus an
  error — Go's convention makes a value untrustworthy beside a non-nil error, and
  the whole requirement is that the rows stay usable while the damage is known.
- **`gnosis doctor` reports the trail's health** — malformed-line count. **Done,
  and half of it is withdrawn:** the timestamp comparison fires on the ordinary
  hand-edit-and-commit workflow, because a git commit is not a gnosis write. §15 is
  corrected rather than the check being weakened.

**Phase 2 or 3, once `quotecheck` is wired and the passages are stored:**

- **Upstream drift resolves to three states** (§14.3.2). **Done in §6.19, as four
  states.** `gnosis.Drift` is pure over (archived hash, upstream hash, upstream text,
  recorded quotations) and `fetch --recheck` supplies them; `drift-unsupported` opens
  one finding per affected claim. Two things this bullet did not anticipate, both now
  in §14.3.2.1: the table's "bytes differ" was a precondition in prose, so the hash
  comparison moved inside the function and `drift-none` is its answer; and an empty
  upstream or a hash nobody computed is `drift-unchecked`, because one 404 body would
  otherwise report withdrawn support for every claim resting on the source. The
  corruption path is untouched, as this bullet required — a passage failing against
  the *archived* bytes is still a hard failure.

**Phase 3, with §10 — both now done ahead of it:**

- **`rationale` gains the fold-and-compare refusal** (§10.6.4). **Done in §6.20**, and
  three of this bullet's details were wrong. It is applied to
  `command.Promote.Rationale` rather than to `gnosis_warrant`, which is still
  unimplemented — that is the field carrying reasoning in this binary, and Phase 3's
  warrant inherits the function. The comparison is per **path**, not per `subject`,
  because a promotion has no subject. And it is not "two `EINVALID` cases and a
  comparison against a value already in hand": the fold has to include **case**, which
  every quotation guard here deliberately excludes, and the prior rationales are read
  from the trail rather than being in hand, so scoping them to promotions that *landed*
  is what stopped `promote` refusing its own second half.
- **`gnosis audit --outstanding`** (§15). **Done in §6.20.** This bullet's premise was
  wrong in a way worth keeping: the states are **not** already committed frontmatter.
  A promotion that reached `needs_human` is in the per-user trail and the draft it
  concerns is in quarantine, so the report is a subtraction over two uncommitted
  sources — asked, not answered, draft still present. The frontmatter half belongs to
  §10.7.4's challenge states, which are Phase 3 and unbuilt, and `--outstanding` will
  gain them when they exist.

**Phase 4, and it is the one worth naming early:**

- **`standards/retrieval-cases.toml`** (§11.0.2). **Done in §6.21**, and two details
  here were wrong. Cases assert on **titles**, not concept ids: identifiers are assigned
  per corpus, so a file naming them is unportable, unreviewable, and turns a failing
  case into archaeology — the correction came from the competency-question entry, which
  merged into this one because they are the same instrument at two grains.
  And the reason to build it was not "nothing to admit before §11.4": that was about the
  *reranker* the evidence is for. The instrument is threshold-free, and a disappointing
  query is unrecoverable evidence — it happens, it is noticed, and with nowhere to write
  it down it is gone by the next day. The file ships empty and an empty suite reports
  that it examined nothing.
  What this bullet got right is the part worth keeping: §11.0 said the miss log would
  supply the evidence for enabling a reranker, and the miss log records only queries the
  deterministic path *declined*, never ones it answered wrongly. **The instrument named
  cannot measure the thing claimed.**

One item is explicitly *not* taken, recorded so its absence reads as a decision. The
surveyed judge breaks a permanently-red gate by counting an unresolved criterion as
passed (`PASS: ALL criteria are PASS or BLOCKED`). §9.5.1 now names that collapse and
refuses it: gnosis's escape from a red gate is `needs_human`, a person, and a counter
that expires into `approved` would be a `--yes` with a delay.

The remaining findings are against `skillsaw`, `canonizer`, and `adh` rather than
gnosis, and live in `TODO.md` — the ratchet's regression status and consecutive-reset
counter, the count-shaped rubric dimensions, the rule that loosening a gate may not
score as improving it, the evaluator noise floor, `verify.Provenance`'s two-signal
cross, and recording a critic that ran with reduced independence.

______________________________________________________________________

### 4.2 What the Second Tier 1 Pass Turned Up

Three items that looked unrelated were one: something built and not connected.
Two of them were the same work, since joining freshness to a command is what gives
`staleness_days` a reader.

- **A finding nobody can act on is noise, and the test said so.** `doctor`'s first
  unread-value check reported gnosis's own dead knob on every freshly initialised
  bundle. Its owner could not build the reader, and could not delete the value
  either, because the loader then rejects the file for a missing rationale. The
  fix was to narrow the claim from *this knob is dead* to *you edited this knob and
  got nothing*, which is per-corpus, actionable, and silent on a fresh bundle.
- **Two states were not enough.** `html_extractor` and its version are read only by
  a test that pins them to Go constants. Calling them consumed tells a reader their
  edit takes effect; calling them unread invites deleting the provenance every
  extracted record carries. *Pinned* is the third state and it had to be added.
- **`hasUpstream` and `everChecked` are different questions.** `lint.FreshnessOf`
  passed the second for the first, which reported a document nobody had checked as
  `not_applicable` — "there is nothing to check" instead of "nobody looked". That
  is the exact collapse the four-state vocabulary exists to prevent, inside the
  function written to prevent it, and only its own test caught it.
- **Declining to build a reader is a decision worth writing down.** `in_degree_cut`
  could have been given one in an afternoon by labelling bare centrality. It would
  have been a different feature wearing the same number, and would have made the
  value look consumed while §14.4.1's actual requirement stayed unbuilt.

______________________________________________________________________

### 4.3 What the Third Tier 1 Pass Turned Up

The three items were one: the gate was permanently red, there was no way to drive
it from a terminal, and there was no sanctioned way through it. A corpus that can
ingest and never promote has a full inbox and an empty shelf.

- **"Correct" was half an answer.** The gate refused everything and the package
  comment said so and called that right. It *was* right and it was not sufficient,
  and the difference took writing down the third option to see: a bound with a
  recorded reason is neither the lie (pass a partial check) nor the bypass
  (`--force`). The test of which one you have built is whether the corpus can
  enumerate the debt afterwards.
- **A comment claiming a package does not exist.** `security` returned a literal
  saying §9.3's scan "is not built", months after `internal/scan` landed. Dead-wrong
  comments are worse than missing ones, and this one hid a real gap: the scan ran
  over fetched sources and never over the candidate document, which is the more
  dangerous artifact.
- **An honesty mechanism nobody called.** `scan.Stages()` existed so a caller could
  report which stages ran, and had zero callers — the exact failure §6.5.1 was
  written about one layer up, in the same week, without either being noticed from
  the other.
- **Implemented, proven, and still unchecked.** Adding a planted defect for
  `security` broke an equivalence the self-test's own tests relied on. *Unproven* is
  a fact about a signal; *unchecked* is a fact about one candidate. They coincided
  only while every unchecked signal was an unbuilt one.
- **Running it by hand found what tests did not, for the third time.** A preview
  with no `--approver` told the caller their promotion "cannot be self-granted by an
  agent" — an accusation about an action they had not taken — and wrote an audit row
  for a read. Neither was reachable from the test suite as written, because both
  tests supplied an approver.

______________________________________________________________________

### 4.4 What the Third Audit-Trail Pass Turned Up

- **Two spec sentences that looked contradictory were about different events.**
  §15 wants a mutation to fail hard when its row is unreadable; `Audit`'s comment
  wants an audit failure never to fail its write. Both are right: a failed *append*
  is a known gap, and a successful append with nothing on disk is the trail lying.
  Reading either as the general rule would have produced a wrong design, and the
  resolution is two fields rather than a compromise.
- **"A mutation" was four mutations, not one function.** §15 names `Execute`, and
  `init` and `index rebuild` append outside it. Implementing the sentence as
  written would have satisfied it for half its subjects — the shape of half-truth
  the same section is about.
- **A requirement that fires on the normal workflow is worse than no requirement.**
  §15's timestamp comparison assumed a commit implies a gnosis write. People edit
  markdown by hand and commit; that is the point of a plain-text corpus. Found by
  running the command, which is now the fourth time.
- **A test of source text is a signal to change the code.** The guard on "every
  mutation verifies" was briefly a test grepping call sites for `bundle.Audit(`.
  Unexporting the unverified append made the compiler do it instead.
- **A wrong severity is invisible to tests.** `diagnoseStandards` blocked while its
  own comment said it did not, and nothing failed — the symptom is a red CI on a
  corpus with nothing wrong. Found by reading, and the contract above it had drifted
  the same way, stating a count of blocking conditions that had been true once.

______________________________________________________________________

### 4.5 Two Commissioned Reviews, and What They Cost to Check

Seven-repository gap reviews arrived in two rounds on 2026-08-22. **No work item came from
either**, and the accounting is worth one paragraph here because the same offer will be
made again.

Round one proposed roughly seventy findings; `skillet/TODO.md` records that nothing landed
and why. Round two added a *Code-Reality Verification* step — the improvement the first
round's assessment explicitly asked for — and produced four findings, three of which are
restatements of existing backlog entries **including the corrections those entries had
made to round one's own proposals**. Its verification line reads *"Confirmed via `git diff
HEAD` and `TODO.md`"*, and the second clause is the mechanism: a report that verifies
against the backlog converges on the backlog.

The planning consequence is the one worth carrying forward. **A survey of a corpus this
family has already absorbed cannot produce a gap, because the absorption is what the
backlog is.** What can produce one is a reading of the *code* — the four items this plan
has taken from running the binary by hand, and none of the seventy-four from the two
reviews. Budget accordingly: the reviews cost several hours to check and returned one
reusable method note apiece, which is a fair price for the note and not for the findings.

The one thing round two supplied that nothing else has is a **specimen**: its own citations
do not survive a lookup, in a document reviewing the tool that exists to catch exactly
that. Recorded in `SPEC.md` §1.1.0 and the manifesto, where it does more work than any of
its recommendations would have.

______________________________________________________________________

## 5. Per-Step Exit Criteria

Every step, without exception:

1. Contract comments written before bodies (**§rules 4**).
2. `go build ./...` and `go vet ./...` clean.
3. `golangci-lint run --fix ./...` then `golangci-lint run ./...` reporting
   **zero** issues, with **no rule relaxed, disabled, or `nolint`-suppressed**.
   A lint finding is a design signal first: prefer changing the code.
4. New pure functions have tests; new shell code has tests only where it has
   branches (**§rules 5** — the threshold is fear, not coverage).
5. A rules review recorded in the commit body naming which sections applied.

______________________________________________________________________

## 6. Progress

| Step                    | State    | Notes                                                                                                                              |
| ----------------------- | -------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| 0 — clean baseline      | **done** | 4 scaffold issues to 0; `ShortHelp` placeholder replaced                                                                           |
| 0.4 — layering lint     | **done** | depguard rules added. The first version was too broad and failed the domain's own external test package; scoped with `!$test`      |
| 1.1 — domain identity   | **done** | `ID` (UUIDv7) and `Slug`, split one-per-file because decorder orders types before funcs at file scope                              |
| 1.2 — OKF parse/render  | **done** | Verbatim-block round trip; CRLF normalisation pinned as a documented exception                                                     |
| 1.3 — ontology          | **done** | TOML rather than YAML — see the format finding below. `Dimension` split to its own file for the same decorder reason as 1.1        |
| 1.4 — index             | **done** | Real temporary SQLite in tests, not a mock. FK pragma, cascade direction, and link degradation each pinned                         |
| 1.5 — reconciliation    | **done** | Placed in `internal/gnosis`, not `internal/index` — see the deviation below                                                        |
| 1.6 — lint registry     | **done** | 5 checks; applicability derived, skips always reported with a reason                                                               |
| 1.6b — bundle loader    | **done** | `fs.FS`-based, so only its own tests touch a filesystem                                                                            |
| 1.7 — commands          | **done** | All seven wired end to end. The row read "in progress" for three phases after `show`, `search`, and `graph` landed                 |
| 1.8 — `documents_fts`   | **new**  | Phase 1 search is document-scoped (§19); one tokenizer constant shared with `claims_fts`                                           |
| 1.9 — `schema-shape`    | **new**  | `sqlite_master` against what the migrations declare; catches a partially applied migration the version check cannot see            |
| 2.1 — segmentation      | **done** | `Claims` cuts only when the fragment's subject can be recovered; `Anchor` locates it, `Text` verifies it — see §6.8                |
| 2.2 — standards         | **done** | `Value[T]` makes the rationale structural; the loosening direction lives in Go, not the file — see §6.8                            |
| 2.3 — tier 0 (pure)     | **done** | `Decide` is a pure function of (candidate, gates); a record's sha256 is its own filename. Writing is 2.4's shell                   |
| 2.4 — `fetch`           | **done** | Four adapters, the pinned HTML extractor, `--dry-run` as a command field; a re-fetch of unchanged bytes is a verified no-op        |
| 2.5 — command + lock    | **done** | `Effect` fails closed, `Promote` validates itself, one writer per bundle. Found three readers that were writing — see §6.9         |
| 2.6 — gate + quarantine | **done** | Five signals run, two report `unchecked` and block; the gate proves it can fail on every invocation — see §6.10                    |
| 2.7 — ingest/admit      | **done** | Two-phase relay, content-addressed cache, `--cache-only`; segment-then-check wired end to end — see §6.11                          |
| 2.8 — log + audit       | **done** | Two records, not one rendering of one: `log.md` committed, `audit.jsonl` per-user. Clock injected — see §6.12                      |
| 2.9 — writer as a type  | **done** | The lock is a value the writers require. It found a caller that was not taking it — see §6.13                                      |
| 2.10 — scan 2 and 3     | **done** | A self-testing pattern ruleset; `betterleaks` does not exist. The item's stated payoff was wrong — see §6.14                       |
| 2.11 — debt + discard   | **done** | `gnosis debt` reads the trail; `quarantine --discard` is the refused candidate's route. A refusal no longer prompts — §6.15        |
| 2.12 — Phase 3 arrears  | **done** | The trust fold, the OKF conformance table, the seeded sampler, the index digest, `claim-anchor`, §12.1's table — see §6.16         |
| 2.13 — tier-0 closure   | **done** | §9.3 completed for a candidate; the scan fails closed; `fetch` is audited; the store and the ledger account for each other — §6.17 |
| 2.14 — accounting fixes | **done** | The loosening classifier called a consumed threshold unread; the fixtures are hermetic; the payload refusal is actionable — §6.18  |

Three findings from the per-step reviews changed the design rather than the code
around it:

- **Re-encoding YAML cannot round-trip.** Every encoder normalises quoting and
  key order, and comments do not survive a decode. `okf` therefore retains the
  frontmatter block verbatim and re-emits it, which also removed the need for an
  encoder in Phase 1 entirely — nothing mutates yet.
- **`frontmatter.Split` normalises CRLF**, so byte-exactness cannot hold for a
  Windows-authored file. That is now a stated exception with a test, rather than
  a surprise in a future diff.
- **YAML was measured rather than assumed, and the vocabulary moved to TOML.**
  Probing `goccy/go-yaml` showed YAML 1.2 is better than its reputation — `no`,
  `yes`, `on`, and `12:30` all stay strings, and duplicate keys already error —
  but two coercions survive: `0755` becomes an integer and `1.20` becomes `1.2`.
  TOML has neither. The decisive difference was elsewhere: `toml.Decode` reports
  `MetaData.Undecoded()`, so a mistyped `normatve = true` is caught, where
  decoding YAML into a map cannot distinguish a typo from a producer extension.
  OKF frontmatter stays YAML because OKF §4 mandates it; SPEC §5.2 now states the
  three-format rule so the question is not re-litigated per file.
- **That probe found a defect in code already written.** `okf.Parse` reported a
  coerced `type: 1.20` as a *missing* key, sending a reader to hunt for something
  plainly present. The diagnostic now names the decoded type and says to quote it.
- **Reconciliation moved out of `internal/index`.** The plan placed it there;
  it is pure domain logic over domain values with no SQL, and SPEC §5.1.2 is a
  Data Model section. Putting it in `internal/gnosis` keeps the index package
  about SQLite and makes the six-case core testable without opening a database.
- **`export_test.go` rather than a production seam.** The schema properties worth
  testing — that the foreign-key pragma took, that the cascade runs the right
  way, that an unresolved link keeps its `href` — need arbitrary SQL. rules.md §9
  forbids test-only seams in production code, so the helpers live in a file
  compiled only under `go test`, adding no exported surface a consumer can see.
- **decorder shapes file layout, and that is worth knowing up front.** It orders
  declarations by kind at *file* scope, so constants must precede the type they
  belong to and all types precede all functions. Three files hit this before the
  habit stuck. It pushes toward the rules' own one-concept-per-file guidance.
- **A cognitive-complexity finding was a real design signal**, not noise: the
  slug property test was doing four assertions in nested loops. Extracted to a
  `t.Helper()` assertion helper per §rules 10, which shortened it and made the
  failure messages name their input.

### 6.1 Layer the Depguard Rule Had Not Named

Writing the loader tripped this plan's own §0.4 rule: `internal/bundle` must
import `internal/okf` to parse, and the rule forbade sibling imports outright.

The rule was incomplete rather than wrong — there are four layers, not two:

```text
internal/gnosis     domain    imports nothing internal
internal/{okf,ontology,index,archive,lint}
                    parsers   import the domain only, never each other
internal/bundle     shell     imports the domain and the parsers; composes them
cmd/*               commands  import anything internal
```

`bundle` is a real layer rather than an exemption carved to fit: nothing imports
it but commands, and it imports nothing but the domain and the parsers. The
depguard rules now say so in both directions — the parsers are forbidden from
importing `bundle`, which is what keeps the layering honest rather than
aspirational.

### 6.2 Gap This Plan Missed

Steps 1.1 to 1.6 build the pure core; step 1.7 builds the commands. **Nothing
builds the shell between them** — the loader that walks the bundle, parses each
document, resolves links, and assembles a `lint.Snapshot`. Every command needs
it, and no step owns it.

Recorded as **step 1.6b — bundle loader (`internal/bundle`)**: the imperative
shell, a flat sequence of directory walk, parse, and assemble, with no branch
that decides domain meaning. It is the one package in Phase 1 whose tests are
filesystem-backed, using `t.TempDir()`.

That the gap appeared only when the pure core was finished is itself the
FCIS point: the core was specifiable in advance, and the shell's shape was not
knowable until it had something to wrap.

### 6.3 What Step 1.7 Turned Up

Four commands are done — `init`, `doctor`, `index rebuild [--check]`, `lint` —
and building them changed three things the earlier steps had settled wrongly.

**The identity check was two checks wearing one name.** Wiring `lint` to the real
index made every document on a fresh clone report "not in the index", which is
precisely the noise `internal/lint`'s own applicability rule exists to prevent.
The split is by what an outcome is *about*: a duplicate identifier is a fact
about the files and stays true if the index is deleted, while "not in the index"
says nothing about the documents at all. So `identity` always runs and reports
duplicates and unidentified documents, and `index-drift` runs only when the
bundle has an index. This is the `index-drift` check step 1.6 listed and did not
build; it turned out to be a slice of an existing check rather than a new one.

**Reading must not create.** `lint` needs index rows, and the obvious way to get
them creates a database as a side effect of having looked. `bundle.LoadIndex`
therefore reports `Present: false` for an absent index and writes nothing, while
`bundle.OpenIndex` — used by `init` and `rebuild` — creates. Both live in
`internal/bundle` under the promote-on-second-consumer rule: the path constants
and the open logic were duplicated in `indexcmd` the moment `lint` needed them.

**`doctor` and `lint` answer different questions, and the split is load-bearing.**
`lint` examines the knowledge; its findings say the corpus is wrong. `doctor`
examines the apparatus; its findings say gnosis *cannot judge* whether the corpus
is wrong. A vocabulary that does not load belongs to the second, because until it
loads no document can be classified at all. Both are pure functions in
`internal/lint` — one over a `Snapshot`, one over an `Environment` — with the
filesystem gathering in `internal/bundle`.

The linter earned its keep three more times, each a real defect rather than a
style objection:

- **`musttag` found that `lint.Environment` had no JSON tags**, so
  `doctor --jsonl` was emitting `OntologyPresent` beside the envelope's
  `ontology_present`. A caller would have had to special-case one command.
- **`unparam` found `init`'s error helper always passed the same reason.** Every
  way `init` can fail is the same way, so the reason is now fixed in one place
  rather than repeated at eight call sites.
- **`gocritic`'s hugeParam found `Environment` and `Result` copied by value** on
  every call. Both are now pointers, matching `lint.Run(*Snapshot, ...)`.

**And the SPEC was the stale document, not the code.** §8 still promised
`--output json` and never described the envelope the `--jsonl` decision
produced. It now has §8.0 stating the status and reason vocabularies, and why
`status` has four values while there are five exit codes: a bad invocation is a
tool failure whose *repair* differs, so it shares `status: error` and takes its
own code. `root.Usage` makes exit code 2 real; before this it was a declared
constant nothing emitted.

### 6.4 What the Specification Changed Under This Plan

Between step 1.7 and here, `SPEC.md` absorbed a long survey of prior art and
several decisions, and this plan was written against the earlier version. Recorded
because a reader comparing the two would otherwise conclude one of them is wrong.

#### What Changed for Code Already Written

- **`concept` → `claim`.** OKF defines "Concept ID" as *the path of the concept's
  file*, so in the format gnosis conforms to a concept **is** a document. The
  addressable assertion inside one is a `claim`. Step 1.1's `Concept` type and
  `ConceptService` are renamed above; the schema rename is done.
- **A claim's address is an anchor, not an offset.** Step 1.1 had `Pos int` as the
  claim's location and said nothing about identity surviving an edit. §5.5.1 now
  requires both to be recoverable from the document: an assigned id and a
  fold-normalized anchor in frontmatter, with `pos` demoted to a cached
  convenience. This is the correction with the longest reach, because a claim
  written before it would have an identity no rebuild could recover.
- **Position conventions were absent, and three columns disagreed** (§5.5.2).
  Units were stated (bytes); origin and relativity were not, and `claims.pos`,
  `links.snippet_start/end`, and `evidence.pos` do not share a coordinate space —
  the first two are offsets into the document *body*, the third into the *archived
  source*. Body-relative rather than file-relative is the load-bearing part: claim
  identities live in frontmatter, so a file-relative offset would shift every
  position in a document whenever a claim was added, though the prose had not
  moved. `Pos` is now `*int`, since `0` is a valid position and cannot also mean
  "not located".
- **`ontology.yml` → `ontology.toml`.** Recorded in §5.2's three-format rule and
  in the finding below it; the plan still said YAML.
- **`--output json` → `--jsonl`, with an envelope.** Status, code, reason, message,
  data (§8.0). Findings exit 3 and tool failures exit 1, and the tests assert the
  decoded envelope rather than the exit code alone.
- **The alias-collision diagnostic must name its remedy** (§5.8.2.1). The loader
  already errored; erroring was never the hard part.

#### What Changed for Unstarted Work

- **Phase 1 is document-scoped and the `claims` table stays empty** (§19).
  Identifying claims needs extraction, which needs a model, which is Phase 2. So
  `search` queries `documents_fts` — hence new step 1.8 — and no claim identity is
  issued for something gnosis had to guess at.
- **The index is per-user, and git is the only inter-user channel** (§4.6). Step
  1.4's byte-identical rebuild requirement stops being a determinism nicety and
  becomes the property that makes per-user caches safe. It also makes `duplicate` a
  post-merge reconciliation step rather than a hygiene check, because two people
  documenting one subject produce two identifiers and git merges both cleanly.
- **`internal/lint` holds two passes, not one** — `Snapshot` over the knowledge and
  `Environment` over the apparatus. This was discovered building `doctor` and is
  now in the spec rather than only in this plan.
- **Several checks were added**: `schema-shape` (new step 1.9), `claim-anchor`,
  `coverage`, `language`, `lead`, `unanswered-challenge`. Only the first is Phase 1.

**One thing this plan got right and the spec now says out loud.** §0.4's layering
rule — enforced by depguard, not vigilance — turned out to need a fourth layer
(§5.1) and then held without further amendment through every subsequent change.
Nothing in the surveys or the decisions since has required loosening it.

### 6.5 What Building Phase 1 Turned Up

Four things the plan did not anticipate, each found by running the thing rather
than by reading it.

- **`links.source_claim_id` made the link graph unrepresentable in Phase 1.**
  Every link referenced a claim, and Phase 1 has no claims — so `graph`, `show`,
  and `search`'s inline links would all have had nothing to read. A link has two
  sources and only one is always known: it is *in* a document from the first
  commit, and *within* a claim only once extraction identifies one. So
  `source_document_id` is NOT NULL and `source_claim_id` is nullable.
- **`documents_fts` could not be external-content**, as §5.5 had it. External
  content reads column values back out of the content table, which would have
  meant storing every document body in `documents` purely to satisfy FTS. It is
  self-contained instead, and the difference is now a comment in the migration
  because the two sibling tables genuinely differ.
- **A tool failure was printing usage help.** The dispatcher prints command help
  for any error it does not recognise as an `ExitError`, and `root.Fail` returned
  the bare cause in human mode — so `show <absent-id>` answered with the full flag
  list and buried the one sentence that explained it. `Fail` now writes to stderr
  and returns an `ExitError`, symmetric with `Usage`. Pinned by a test.
- **Inbound links rendered the wrong end.** `Inbound` returned the link's `href`,
  which on an inbound link points back at the document being shown — the reader is
  already there. It now returns the *source* document's path and title.

Two smaller notes. `gosec`'s G101 flags any identifier containing "token"
assigned a long string literal, so the shared tokenizer constant is named
`ftsAnalyzer`; renaming was the honest fix rather than a `nolint` on a rule that
is usually right. And the seventh decorder finding split `internal/index/find.go`
out of `documents.go`, which is the pressure toward one concept per file the plan
noted at 1.1 and has now paid off four times.

### 6.6 What the Field Survey Changed

A survey of 29 LLM-wiki implementations and 10 design documents
(`~/Documents/agent-purple`, recorded in `manifesto.md`). Most of it confirmed
decisions already made — markdown plus git, a deterministic CLI beside the agent,
lint as first-class, the index as a derived cache, all arrived at independently by
projects that never read this. Four things changed the plan.

- **A read path that cannot refuse** (§17.0.1). Every gate in this design is on the
  write path. `ask` retrieves and emits, and has no way to say the corpus does not
  support an answer — so an unanswerable question produces the same shape of output
  as an answerable one. Phase 3 work, because it needs the conflict machinery to
  distinguish *silent* from *unresolved*, but it belongs in the interface now.
- **The gate must approve a diff, not a document** (§9.4). Between checking a
  candidate and committing it there is a window nothing closed. This is a
  requirement on the Phase 2 promote path and on the write coordinator, and it is
  cheap to build in and awkward to retrofit — a gate that can be raced is
  decorative.
- **The semantic reranker now has a stated bar** (§11.0). The only measured claim
  in the field says a curated wiki is 50–100k tokens and grep beats embeddings at
  that size. The reranker stays optional, and the trigger for enabling it is a miss
  log that shows FTS5 failing — not a hunch.
- **Pruning is an unanswered objection** (§14.3.1). The one practitioner report
  available says the point of a forgetting curve is that *something deletes*, and
  our periodic review only ever reports. The rule stands; the mitigation
  (deprecation rather than deletion) is recorded and not designed.

**The uncomfortable one, which is not a plan change but should be visible here.**
The same report — months of daily use — argues that governance features earn their
place and infrastructure does not, at the scale these systems actually run. gnosis
is mostly infrastructure. The reply is that a team corpus has contradictory sources,
mixed-skill contributors, and borrowed authority in a way a personal vault does not,
so the machinery should pay — but that is a *prediction*, and the honest test is a
real corpus at Phase 2, not more specification.

### 6.7 Unblocked Work Taken Ahead of Phase 2

Phase 2's core — `fetch`, the archive, the ingest relay — is blocked on the claim
segmenter, the `fetch.jsonl` layout decision, and the write-coordinator API. Four
specified, unblocked items were taken instead, and two of them turned up something.

- **`gnosis_schema_version`** (§5.5.1.1) with `okf.Int` and `okf.Has`. Built ahead
  of Phase 2 deliberately: the first convention change is already scheduled, and
  backfilling a version onto documents whose conventions are already unknown is
  guesswork. The check **skips until the corpus starts versioning** — without that
  it would report every document on the day versioning arrives, which is the
  derived-applicability failure §12 exists to prevent, in the one case where
  "nothing is versioned yet" and "everything is out of date" look identical.
- **Snippets rendered rather than excerpted** (§11.0.1). The tension recorded
  earlier — strip markdown at index time and slugs become unsearchable, strip at
  render time and FTS5's offsets stop matching — dissolves once the snippet is
  re-derived instead of offset-mapped, because then nothing has to stay in
  correspondence.
- **`placeholder` and `empty-section`.** Both catch a page that reads as finished
  to every other check: it conforms, it has a type, its links resolve, and it
  answers nothing.

**Two things the linters and tests caught, both real:**

`gocritic` flagged five range-copies the moment `Document` grew one pointer field
past its 128-byte threshold. Indexing rather than copying is the fix, and it is the
pattern the index package already used — the type had been sitting just under the
line.

More usefully, `empty-section`'s own test caught the implementation contradicting
its documented contract. The comment said a following heading ends a section
without emptying it; the code reported every parent heading whose first child was a
subheading. The rule it should have stated is about **level**: a deeper successor
means the section's content *is* its subsections, and only a same-or-shallower one
leaves it empty. Writing the contract first is what made the disagreement visible.

### 6.8 What Building Steps 2.1–2.3 Turned Up

**The segmenter needs two strings per claim, not one.** §5.5.1 asks a claim to
carry an *anchor* locating it in the document, and §9.4 asks the emitted claim to
stand on its own. Those are the same string only when no subject was recovered.
"it is not shared across sessions" is what the document says and is therefore the
only thing findable in it; "The cache is not shared across sessions" is what a
verifier can check and appears nowhere. `Claim` carries both plus a `Substituted`
flag, because a reader adjudicating a finding needs to know the text they are
judging is not the text the author wrote.

**The stand-alone rule is what makes over-splitting safe, and it fires often.**
A clause whose subject cannot be recovered leaves the sentence whole, so
*"Deploy on Friday, but it rarely ends well"* stays one coarse claim rather than
becoming one honest claim and one that validates against anything. Refusing the
cut is the conservative direction and it needs no confidence estimate to choose.

**A property test caught its own harness, not the code.** "Concatenating the
claims loses no assertion" was asserted by rejoining the anchors — with no
separator, which fused `default` and `it` into `defaultit` and reported two words
lost. The cut consumes the separator it cut on, so the rejoin has to restore one.
Worth recording because the failure looked exactly like a segmenter bug.

**A structural rationale beats a conventional one.** `standards.Value[T]` pairs a
threshold with its justification in one type, so there is no way to express a
value without a reason; a `rationale` that were merely conventional would be the
first field dropped by whoever was in a hurry. The loader walks by reflection for
the `justified` interface rather than down a list of fields, because a list is a
second place to remember and the failure it permits — a threshold added to the
file and the struct but not the list — is precisely the unjustified value the
check exists to prevent.

**The loosening direction belongs in Go, and this was not obvious.** The first
design put a `looser = "higher"` field beside each value, which is self-documenting
and wrong: concealing a loosening would then take nothing more than flipping that
field in the same commit. `CompareArchive` states each direction in code, so hiding
one means editing Go, which is a different diff read by different reviewers.

**Adapters cannot import each other, and tier 0 needed gates from `standards`.**
Rather than relax §0.1, `archive.Gates` states what the policy needs and the shell
joins the two. The duplication is three fields and it keeps the layering claim
true; the same shape `lint.Snapshot` already uses.

**`archive` scans SVG with the XML tokeniser, not with patterns.** A `<script`
match is defeated by a namespace prefix (`<s:script>`), by case, and by a newline
inside the tag — all three are in the test table, and all three are already
resolved by the time the decoder names an element. Malformed XML is refused rather
than best-effort parsed, because a document two parsers disagree about is one whose
rendered form is not the form that was scanned.

**Omitting the timestamp is now a test, not a comment.** §4.3.1's decision is the
kind a later contributor undoes helpfully, so `TestNoTimestampField` fails on any
encoding containing one. The property it protects — that a re-fetch of unchanged
bytes lands at the same path and writes nothing — is tested directly beside it.

### 6.9 What Building Step 2.5 Turned Up

**Three readers were writing, and the failure was worse than layering.** §4.6 names
`lint`, `search`, `show`, and `graph` as readers that must not require the writer,
and §4.5 says nothing read-only creates state. `search`, `show`, and `graph` all
called `bundle.OpenIndex`, which creates `.gnosis/` and migrates. On a fresh clone
`gnosis search cache` therefore built an empty index and answered **zero hits** —
which a caller cannot distinguish from *no matches*. The corpus appeared to contain
nothing rather than to be unbuilt. `OpenIndexForRead` refuses instead and names the
repair. Migration of an index that *does* exist is kept, and the asymmetry is
argued in place: the index is a derived cache, so moving its schema forward loses
nothing, while failing every read until someone rebuilds would make an upgrade feel
like a breakage.

**The envelope had to move down, and the rule that says so is the family's own.**
§4.6.2 specifies `Execute(ctx, Command) (Outcome, error)`, `internal/*` cannot
import `cmd/*`, and the envelope lived in `cmd/root`. That is
promote-on-second-consumer applied inside one repository: the value moved to
`internal/gnosis`, the *emitters* stayed in `cmd/root` because they are I/O, and
the vocabulary is re-exported so no call site changed.

**Typing `Status` and `Code` immediately caught call sites.** They had been untyped
string and int constants, and the compiler found nine comparisons that had been
passing only because everything was a string. None was a live bug, but a `Reason`
where a `Status` belongs is precisely the mistake a machine contract cannot afford.

**`Code` has no safe zero value, and that had to be designed around rather than
declared away.** Every other enumeration here gives the zero value a name that
asserts nothing — `EffectUnset`, `ActorUnset`, `DispositionUnset`, `quotecheck.Unchecked`.
`Code(0)` is `CodeOK`, a real and successful value, so the same trick is
unavailable. The resolution is to make an `Outcome` constructible only through five
functions that set status and code together — the only five pairings §8.0 defines —
so nothing in the package can produce a mismatched pair, and `Valid` reports one
built by hand.

**`golangci-lint --fix` silently deleted the `Command` interface.** It had no
implementor referencing it yet, so `unused` removed it and the next build failed on
a type that had been written minutes earlier. The fix is the compile-time assertion
`var _ Command = (*Promote)(nil)`, which is worth having anyway: without it the
interface is satisfied by accident and a renamed method surfaces at the coordinator
rather than at the declaration.

**A concurrency test that cannot fail proves nothing.** The first version
incremented a counter under the lock and checked the peak — with a critical section
so short that the peak stays at one whether or not the lock works. The rewritten
test does real file work inside the lock and asserts the enter/exit log is strictly
nested, and it was **verified by disabling the lock**: it fails with the writers
interleaved and passes when the lock is restored.

**Validation runs before the lock, and the ordering is a design decision.** A
malformed command must not queue behind a well-formed one. It is also where "no
transport can skip validation" becomes true, since every transport arrives at
`Execute`.

**A preview takes the lock too.** That looks unnecessary and is not: a preview
computes the diff the apply will use, and a preview racing a concurrent write would
report a diff against a bundle that no longer exists — which is exactly the window
§9.4 closes.

### 6.10 What Building Step 2.6 Turned Up

**Two of seven signals have nothing to read, and that is a design question rather
than a gap.** `security` needs §9.3's admission scan and `conflict` needs §10's
adjudication. Omitting them would be a silent pass on evidence nobody examined;
failing them would be a lie, because the signal did not fail, it did not run. They
report `VerdictUnchecked`, and **unchecked blocks**. That is `quotecheck.Unchecked`
one level up and §17.0.1's rule applied — a read path that cannot refuse is not
trustworthy. The consequence is stated rather than buried: **until those subsystems
exist, no promotion succeeds.** A test asserts exactly that, so it is not later
mistaken for a defect.

`Withheld` returns the failures and the unchecked signals **separately**, because
the two call for opposite responses: a failure is something the author fixes, an
unchecked signal is something this build cannot do, and a caller told only
"blocked" would go hunting for a defect that is not in their document.

**The self-test caught two of my own signals not discriminating**, on its first
run, before any test existed. `duplication` and `evidence` both failed their
controls, and the root cause was one fact: `textnorm.Fold` deliberately does not
lower-case, because case carries meaning in a *quotation*. It carries none in a
*title*, so the duplication signal wanted `gnosis.Surface.Fold` — which lower-cases
on top of the same folding, and which the ontology already uses for the same
reason. The evidence failure was the fixture's own case mismatch, which is the
signal behaving correctly.

**The self-test was then verified against a deliberately decorative signal.**
Wiring `duplication` to always pass makes `TestControlHolds` fail and name it.
That check matters more here than anywhere else in the codebase: the whole claim of
§9.5 is that the gate can be shown to fail, and a self-test nobody has seen fail is
in exactly the position the gate would be without it.

**A self-test must also report what it did not exercise.** `SelfTest` derives its
unproven set by difference from the full signal list rather than listing it, so a
signal implemented later without a planted defect shows up as unproven instead of
quietly counting as proven.

**`unparam` and `nilerr` together found a dishonest signature.** `candidate`
returned `(*gate.Candidate, error)` and the error was always nil, because a
document that will not parse is deliberately not an error — it is a candidate whose
conformance signal fails. Two linters saying so from different directions is the
signal that the return type was claiming a failure mode the function does not have.

**gosec's TOCTOU warning had a real fix, not a suppression.** Walking `evidence/`
with `filepath.WalkDir` lets a symlink lead the reader out of the bundle. Rooting
the walk at `os.DirFS` closes it, costs nothing, and is the posture `bundle.Load`
already took.

**Quarantine's traversal check is not defensive habit.** A quarantined document's
path arrives from a model's reply, so `../../etc/whatever` is an input this
function will actually receive — and tier 1 exists precisely to keep untrusted
content out of the working tree (§3083).

### 6.11 What Building Step 2.7 Turned Up

**`--cache-only` was wrong on the first pass, and a test caught it.** The flag was
checked in the report, after `PromptsFor` had already written the prompts to disk —
so a caller learned which replies were missing and found the prompts emitted
anyway. §6.1 says the flag "refuses to emit", and refusing has to happen where the
writing happens. The fix moved it into `PromptOptions`, which is also the shape a
third option would have wanted.

**The model belongs in the cache key, and this is worth defending.** A reply is a
claim about what a *particular* model said about a *particular* text. Keying on the
text alone would serve one model's answer to another model's question, which is not
a cache hit — it is a substitution nobody was told about. The cost is that changing
models re-asks the whole corpus, and that cost is the honest one.

**The key needs a separator, and the reason is a collision nobody would find.**
Concatenating the components bare makes model `gpt` version `4o` hash identically
to model `gpt4` version `o`. A cache collision here means one source's reply
answering for another's, which would be discovered — if ever — as a claim citing a
document it has nothing to do with.

**A reply is cached before it is parsed, deliberately.** The model call is already
spent. Caching only replies that turned out to be usable would make a caller pay
again to receive the same unusable answer, and §6.1's promise is that a second run
over unchanged inputs makes no model calls — not that it makes none when the first
run went well.

**Three fields on a reply are the caller's, not the model's**, and each would be an
attack if it were not. `SourceURI` — a model that could name its own source could
cite one it never read. `Claim.ID` — an identifier a reply chose could collide with
one in the corpus, or be reused to make two claims look like one. `ArchivePaths` —
a reply nominating its own archive could choose the file its quotations happen to
appear in, which is the check answering to the thing it checks. The last is derived
from the check's own findings, so the document records where evidence *was found*.

**`Unchecked` finally does the work it was built for.** A quotation too short to be
evidence, or one with no archived text to check against, is not a fabrication:
`quotecheck` reports Unchecked and `admit` reports it under its own heading.
Collapsing it into "missing" would accuse an agent of inventing a quotation that
may well be accurate, and the two need different fixes — a longer quotation versus
an archived source to check against.

**`Admit` was the second command, and the interface held.** Nothing in it restates
how a write is gated: `Effect`, `Validate`, the coordinator's lock, and the
envelope all applied with no new plumbing. That is the first real evidence that
§4.6.2's shape was worth building before there was a second writer.

**There was no ID generator.** §5.1 says an identifier is assigned "once, at
admission", and admission is exactly this step — the domain package had a parser
and no constructor. `gnosis.NewID` is now the one impure function in that package,
and its comment says so, because everything else there is a value operation and a
reader is entitled to know which one reads a clock.

### 6.12 What Building Step 2.8 Turned Up

**§15 names `skillet/auditlog` and that package is the wrong shape.** It reads and
writes `results.tsv`: nine tab-separated columns describing a
baseline/keep/revert/error *optimization experiment*. §15's row is a mutation
record with paths and content hashes. They share a word and nothing else. This is
the third SPEC reference to a library that does not fit — after `go-git/v6` and the
extractor name — and the pattern is worth naming: a specification written before
the code cites what sounds right, and only building against it finds out.

**The audit row carries a timestamp and a fetch record does not, and that is
consistent rather than contradictory.** §4.3.1 refused a timestamp because a fetch
record is content-addressed, so one would make tier 0 grow when somebody *checks*
rather than when the corpus *learns*. An audit row is a record of an event and
"when" is half the question it answers. §10.7.4 reconciles them: a fetch record
states a fact about the corpus and must travel; an audit row states what this
user's process did and must not.

**`log.md` and `audit.jsonl` are two mechanisms, not one with two renderings.** A
colleague pulling the repository needs to know the per-file cap was raised and why;
they do not need to know this laptop rebuilt its index eleven times. Merging the
second into git would conflict on every pull and tell nobody anything.

**A refused promotion is recorded.** "We declined to promote this eleven times" is
a fact about the corpus that a successful-writes-only trail would not hold, and it
is the fact most worth having when somebody asks why a document never landed.

**An audit failure must not fail the write it describes.** If the document landed
and the append failed, returning an error would tell a caller to retry something
that succeeded — the more dangerous of the two wrong answers. The failure is folded
into the outcome's message instead. This is a real weakness rather than a tidy
design and it is in TODO as one: a trail with silent gaps cannot answer the
question it exists for.

**The clock is a field on the coordinator, and that is a genuine dependency rather
than a test-only seam.** An audit row's whole value is the time on it, and a value
the tests cannot pin is a value the tests do not check.

**Running the thing found two defects the tests did not.** A demo corpus put
through fetch → ingest → admit produced a document with `resource: ""` — the source
URI was documented as caller-set and never set, so the promote gate's provenance
signal would have failed every admitted document. The fix is a `PromptMeta` sidecar
written when a prompt is emitted, which also closed a TODO item: `admit` now refuses
a key that names no emitted prompt, where before it would cache a reply to a
question nobody asked. And quotations are now checked against **the one archived
file the prompt was built from** rather than the whole archive — checking against
everything would let a reply about one source pass on a phrase that happens to
appear in another.

That is the argument for exercising a build by hand even with a passing suite. Both
defects were in the seams between components that each had good tests.

## 7. Rules Review of This Plan

A pass over the plan against `summary_rules.md`, recording what it changed.

| Rule                           | Finding                                            | Change made                                                                                                                                  |
| ------------------------------ | -------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| §1 layout                      | The plan originally accepted the scaffold silently | Added §0.1 naming the conflict and justifying `internal/gnosis` explicitly, rather than leaving a reader to assume the rule was overlooked   |
| §1 subpackage imports          | Layering was asserted in prose                     | Added §0.4 — enforced by a lint rule, since §15 requires linters over vigilance                                                              |
| §4 interface comments          | Contract comments were implied                     | Added §0.3 making them a precondition of writing a body, with the split-if-too-long trigger                                                  |
| §5 FCIS                        | Reconciliation was described as a command          | Made §1.5 a pure function with the filesystem scan as its shell caller, which is where the boundary actually is                              |
| §9 calibration                 | Plan assumed uniform test investment               | Added §0.2 with an explicit upward deviation for admission gates, and the reason: a corrupt claim outlives the revert                        |
| §2 error type                  | Plan would have defined a local `Error`            | Corrected to use `skillet/errs` — a fifth copy of the family's error type is precisely the drift the kernel exists to prevent                |
| §4 domain constraints in types | `Status`, `TypeKey` were plain strings             | Named types with validated constructors, so use sites need no validation                                                                     |
| §8 SQL                         | Plan said "use SQLite"                             | Enumerated the six invariants that actually get broken (rollback, `make`, `rows.Err`, bound args, closed sort switch, no `Tx` in signatures) |
| §10 mocks                      | Index tests would have mocked the database         | Real temporary SQLite file — cheap, and a mock here would test the mock                                                                      |
| §15 consistency                | —                                                  | Confirmed: `main.go` at root matches all four sibling tools                                                                                  |

______________________________________________________________________

### 6.13 What Making the Writer Lock a Type Turned Up

**The predicted defect was shipped, and it was worse than the entry guessed.**
`TODO.md` recorded that seven functions carried `Requires: the writer lock is held`
with nothing behind the sentence, and called a second in-process writer forgetting
it "a defect available today". It was not available — it was present. `gnosis fetch`
wrote tier 0 **and rewrote `.gnosis/checked.jsonl` whole**, a read-modify-write over
the entire file, under no lock at all. Two concurrent fetches could lose one user's
observations outright, and nothing would report it.

That is the whole argument for the change. The compiler now refuses a caller that
never took the lock, which does not make the guarantee stronger for the six callers
that already complied — it makes the seventh impossible.

**The free functions were deleted rather than wrapped.** A method whose body forwards
to a package function with the same arguments is the pass-through §rules 4 prohibits,
and it would have left the unguarded entry point exactly where it was. The body moved
and `w.dir` replaced the parameter.

**One write path stays prose, and the reason is layering.**
`index.DB.ReplaceSources` is in a parser package, and PLAN §0.4 forbids a parser
importing this shell. Relaxing that depguard rule to enforce this precondition would
trade a checked architectural claim for a checked precondition, which is the worse
bargain. The comment stays and now says so.

**The test helpers had to name how long permission is held, and that is the mechanism
working.** A fixture that acquires a `Writer` for the whole test deadlocks against
`Coordinator.Execute`, which takes the lock itself. So there are two helpers —
`writerFor` for a test that only writes, `withWriter` for a fixture that writes and
then hands the bundle to something else — and every fixture had to say which it meant.
Under the old design the duration was invisible because there was nothing to hold.

**A helper inside a loop deadlocks against itself.** `AcquireWriter` waits on a
context, and in a test that context outlives the test's own timeout, so a second
acquisition in one test hangs rather than failing. Three call sites had to hoist. That
is a real ergonomic cost of the design and it is worth stating plainly: the failure is
a hang, not an error.

**The linter caught the shell absorbing logic, again.** `fetchcmd.exec` crossed the
cognitive-complexity cap once the lock was added, and the finding was right: it had
come to hold argument validation, standards loading, lock acquisition, a nested loop,
and a second write, with **two mutable accumulators** — `looked` and `result` — filled
in the same loop and able to disagree about what happened. Every field `looked` needed
was already on a `Source`, so one of them was redundant. `checksFor` is now a pure
projection and `exec` is a flat sequence again.

### 6.14 What Building Stages 2 and 3 of §9.3 Turned Up

**`betterleaks` does not exist.** §9.3 names it for stage 3; it is not on the public
module proxy under any casing and not in `skillet`. That is the **fourth** library this
specification has cited that turned out not to fit or not to be there — after
`go-git/v6`, the extractor name, and `skillet/auditlog` — and §6.12 already named the
pattern: a specification written before the code cites what sounds right, and only
building against it finds out. Worth adding one observation to that pattern: all four
were discovered at the moment of use, and none of the four cost anything beyond the
hour it took to find out, because the citation was load-bearing for a decision rather
than for a design.

What is there instead is the part needing no dependency: vendor-documented credential
formats — a PEM armour header, AWS's four-character key prefix, GitHub's `gh?_`
family. Those are the same class of justification as §9.3's Unicode ranges: checkable
against a published standard without running anything. Deliberately **no entropy or
length heuristic**, which is what a general secret scanner adds and what would put a
tuned number inside a blocking gate — precisely the thing §9.3's own argument for why
these constants may block rules out.

**A pattern is arguable where a codepoint is not, so it earns its standing another
way.** Every rule carries a case it must flag and a case it must not, and `LoadRules`
refuses the entire ruleset if any rule fails either. That is the promote gate's
planted-defect argument applied to a rule table, and it is at load rather than in a
test on purpose: a test catches the same defect one commit later and only for whoever
ran it. It paid for itself on the first run by catching a Google-API-key example two
characters too long to match its own pattern.

Cross-rule false positives are asked by a test rather than at load, and the difference
matters: whether rule A's negative example is caught by rule B is the real
false-positive question, and the answer may legitimately be yes — the fix would then
be a better example, not a refused ruleset. A refusal at load would make that
judgement for everybody.

**The entry's stated payoff was wrong, and the test now says so.** `TODO.md` held that
until these stages landed "every promotion in every corpus routes through §9.5.1's
human path". True, and building them does not change it: `conflict` reports
`unchecked` for Phase 3 reasons and withholds automatic approval on its own. What the
stages buy is coverage, not friction — an injected directive or a committed credential
now fails rather than passing unexamined.

**The ruleset is data in the binary, not in the bundle**, which inverts this project's
usual rule that thresholds are data a corpus edits. A per-corpus injection ruleset is
a way to switch the gate off, and §9.3's argument for why these constants may block is
that they are not arguable. A corpus needing a different rule needs a different gnosis.

### 6.15 What `debt` and `discard` Turned Up

**A refusal was reported with the escalation's reason token.** A candidate whose
`security` signal *failed* came back as `needs_human` — the same token an *unchecked*
signal produces — so the CLI prompted for the confirmation phrase and then declined
anyway. `authorise` enforced §9.5.1 correctly; the reported reason erased it. That is
worse than noise: prompting on a refusal teaches somebody that typing the path is what
unlocks one, which is the belief that turns the whole escalation into a `--yes`.
`gnosis.ReasonRefused` now exists and the message names the route out.

**A test had asserted the collapse.** `TestAFabricatedQuotationFailsRatherThanBeingUnchecked`
checked for `needs_human` under a comment reading "a real failure, not an unbuilt
check" — it named the distinction it existed for and asserted the value that erased it,
and passed because both cases shared one token. A test can encode the bug it was
written to prevent.

**Running it found a legibility defect the tests could not.** `debt` printed
per-signal counts above the entries on stdout, where `conflict\t1` has the same shape
as `conflict\tc/a.md\thuman:priya`. A reader could not tell a total from a row, and
neither could `cut`. Data on stdout, summary on stderr — the convention this project
already had, not followed. That is the fifth defect found by running a command by hand.

**The discard decision was the easy half.** Refusing `--edit` took one paragraph; what
took the thought was that a refused candidate needed *somewhere to go*, and that
`audit.OpDiscard` had been declared since Step 2.8 with no writer. A verb that gives a
dead enumeration value a reader is a better answer than one that adds a field.

### 6.16 What Paying the Phase 3 Arrears Turned Up

Six items whose common property is that they were cheap now and expensive later. Three
notes worth keeping.

**§18.5.1's table needed the fold, and building the fold before its consumer is the
point.** The table asserts what `ParseActor` does *and* what tier §14.1's fold yields,
so the fold had to exist. It is a pure function over `[]string` with no consumer in
this build — which §rules 15 would ordinarily call dead code, and the answer is that
§18.5.1 asks for it before §14.1 for a stated reason: the divergence is already in a
shipped type, and the cost of finding it later is a corpus whose tiers were computed
by a parser that refused half its inputs.

**The sampler's home was decided by layering, not preference.** Three future callers —
`critic`, `stale`, and §6.2.1's random pass — live in packages that are siblings of
each other, and siblings may not import each other. `internal/gnosis` is the only
place all three can reach. And the draw is a keyed hash rather than a seeded shuffle,
because §18.3's reproducibility must not depend on `math/rand`'s consumption pattern
staying put across Go releases, and because a hash key is also independent of the
population's input order — which a shuffle is not, and which §18.3 lists separately as
a hazard.

**A digest of a SQLite file would have been the wrong test.** §18.3 asks for
"byte-identical `index.db` content hashes" and a SQLite file is not byte-stable, so a
byte comparison fails on a database that is correct. A determinism test that fails on
correct output gets turned off, after which the property is unmeasured — which is
worse than the weaker property. The digest is over content, and the negative cases are
what stop one that hashes a constant from passing.

**§12.1's table is checked in the direction that drifts, and the check was verified by
breaking it.** All three failure modes — a check deleted from the table, a table naming
a check that does not exist, a category emitted and undocumented — were confirmed to
fire. That matters more here than for most tests: the whole argument for a
hand-maintained table over sixty-three inline tags is that this walk keeps it honest,
and a walk nobody has seen fail is in the same position the tags would be.

______________________________________________________________________

### 6.17 What Closing Tier 0 Turned Up

Six backlog items, and **five pieces of work** — the first thing the pass found was
that two of the entries were one mechanism.

**Two entries reasoning from different failure stories described one predicate.** One
asked for "an archived file that no `fetch.jsonl` row records", arguing from bundle
closure and citing VAC and `qvr sync`. The other asked whether `evidence/text` has
orphans, arguing from a crash between the content write and the record write. Those
are the same file in the same state, and they sat as separate items for months because
**the story differed, not the check**. Worth carrying forward as a shape to look for:
when two entries cite different prior art for the same predicate, the prior art is
what is being compared and not the corpus.

The pair also had complementary halves. The closure entry had the *other direction* —
a record naming an absent file — and the orphan entry had the *cost*, that an orphan
counts against the corpus budget and nothing collects it. Neither alone would have
produced a check with two severities, and the severities are the whole reading: an
orphan is untidy, and a ledger claiming evidence tier 0 does not hold is a corpus that
can no longer fail honestly.

**A fail-open default became decidable only once its other half existed.** The nil
`ScanText` admitting unexamined text had been documented as deliberate and defended
on real grounds: refusing would make every non-scanning caller carry a stub. What
changed is that the candidate path was later built the opposite way — a nil ruleset
there degrades toward *more* blocking and reports the stages it could not run. Two
halves of one security stage failing in opposite directions is a sharper argument than
either half could make alone, and it was not available when the entry was written.
The general form: a wart defended on a local trade-off is worth re-reading each time
its sibling is built.

**Stage 4 needed no new threshold, which is why it was the last one left.** The bound
already existed for a fetched source; what was missing was applying it to the second
artifact. Inventing a candidate-specific cap would have been quick and is what §6.5
forbids, so the work was to make one declared threshold reach two callers —
`archive.Oversize`, with the caps travelling on `gate.Limits` beside the two the gate
already reads. The entry had this right and it is worth noting that *the correct answer
made the item look harder than a wrong one would have*.

**And it did not buy what the backlog predicted.** Completing §9.3 moved a clean
candidate's `security` verdict from `unchecked` to `pass` and did not remove the human
path from the ordinary case, because `conflict` is unchecked for Phase 3 reasons. That
is the second time this specific prediction has been wrong — the same claim was
corrected when stages 2 and 3 landed — and the reason is worth stating once: **an
`unchecked` signal blocks whatever the others do**, so no amount of completing one
subsystem moves the gate while another is unbuilt.

**Coverage cannot be type-checked, so it was arranged instead.** `scan.CoverageOf`
takes the stages a caller performed, and the risk is a caller naming one it did not.
There is no signature that prevents that. What there is: one production caller, each
stage claimed inside the branch that runs it, and a stage name that is not §9.3's
failing to reduce `Missing` — so a typo makes coverage look *worse*. When a property
cannot be enforced, the next best thing is arranging the code so the claim and the act
are one edit apart.

**A test that hangs depending on whose machine it runs on is not a test.**
`go test ./...` began failing after two minutes with a GPG timeout: two fixtures shell
out to `git commit`, and the developer's *global* `commit.gpgsign` applies inside a
temporary repository. Setting it false locally took the run from 120 seconds to 1.6.
Signing was only the setting that happened to bite — `core.hooksPath`, a global
pre-commit hook, and `gpg.format` are all in the same position — so the general fix is
filed: a fixture that reads ambient configuration is testing the machine.

**Two smaller notes.** `gocritic` flagged a range-copy the moment `Source` grew one
slice field past its 128-byte threshold, which is the third time that has happened in
this repository and always for the same reason. And `hasOversizePayload` compares
against the limit it is given, so at zero it flags *every* data URI — a disabled cap
has to disable its check, and the guard is not symmetry for its own sake.

______________________________________________________________________

### 6.18 What the Accounting Pass Turned Up

Four backlog items, two of which had already been overtaken and one of which was a
documentation note hiding a live defect. The pattern across all three findings is the
same and it is worth naming once: **a fact recorded in two places, where only one of
them was maintained.**

**A classifier said a consumed threshold was read by nothing.**
`bundle.describeLoosening` had `staleness_days` in the category "nothing reads this
threshold yet, so moving it changes no finding". That was true when written and stopped
being true the day the `stale` check gained its window — so widening the window
silenced `stale` findings while `standards check --log` recorded that it cost nothing.
§6.2 exists precisely to stop a threshold being loosened without its cost being
visible, and the mechanism §6.2 asked for was the thing reporting the cost as zero.

What made it survivable is what makes it instructive: `standards.Unread` had the right
answer the whole time. Two static lists about one fact, and the one nobody thought of
as a list drifted. The fix is not the delta computation, it is the test that makes the
second list answer to the first — and it was verified by re-introducing the wrong
classification and watching it fail.

**A test fixture's diagnosis was a plausible story fitted to a symptom.** TODO carried
"the git-adapter fixture is fragile under concurrent load", reading
`fatal: failed to write commit object` under two simultaneous runs as filesystem and
subprocess contention in a `t.Parallel()` test. That is the line git prints when
*signing* fails: concurrent load was the trigger only because two runs meant two
requests to the GPG agent. The tell was there in the entry — the same error string
appeared in the newer signing entry — and nobody had put them side by side.

The consequence of believing it would have been removing `t.Parallel()` from tests
that are fine, which is a change with no evidence behind it and a slower suite. The
fixtures are hermetic now (`GIT_CONFIG_GLOBAL` and `GIT_CONFIG_SYSTEM` nulled, identity
in the environment, three subprocesses where there were six) and three concurrent runs
pass in 3–4 seconds with the parallelism untouched. Verified against a deliberately
hostile global config that sets every setting the entry's siblings named.

**A "revisit if it fires" note had the right reading and the wrong trigger.**
`hasOversizePayload` over-reporting on prose about data URIs was filed to be revisited
if it ever fired on a real source. What made it pressing was not a source at all: §9.3
stage 4 applied the same cap to a *candidate document*, where a scan failure is
`refused` with no §9.5.1 human path, so the same over-report went from a lost archive
to an unpromotable document.

The decision is not to relax the cap, and writing down *why* was most of the work —
both obvious relaxations fail on the cap's own rationale, which is weight rather than
safety. What was actually wrong is that the refusal was unactionable: `embedded-payload`
with no measurement leaves an author nothing to do but argue the threshold down.
`archive.Bound` carries the measurement and renders it once for both callers.

**Two smaller notes, both from running the command.** The first draft of `Bound.Detail`
began with the reason token, and `fetch` prints the reason on its own line with the
finding indented beneath it — so the token appeared twice in three lines. And the
per-value invariants of a `Bound` had been bolted onto a table whose cases vary one
thing, which the complexity linter caught and was right about: a property that holds
for every value belongs in one place that says so.

**Sibling backlogs, recorded 2026-08-23.** Fourteen entries in this file were filed
against `skillsaw`, `canonizer`, `adh` and `skillet`. Nine were already in their real
homes and **five of those were already closed there** — `skillsaw/TODO.md` had itself
noted that gnosis was "the wrong home" and moved them, and nothing updated the copies
here. The five genuinely-missing items are now recorded in those repositories and this
file holds pointers with the status each backlog reports. The rule is the one this
project already applies to knowledge: one home, and a pointer from everywhere else. A
mirror goes stale in the direction that flatters, because the only person who reads it
is deciding what is left to do.

### 6.19 What Paying Down the §14 Backlog Turned Up

Seven items, and five of them contained a factual error about their own subject. That
is a higher rate than the previous passes and the reason is structural: these entries
were written from the specification rather than from the code, and a specification
records what a thing should do while an entry has to say what it *does*.

**Three entries mis-stated the mechanism they were about.**

`--relay` was the largest. The entry read `adh run --relay` as one invocation emitting
a prompt on stdout and blocking on stdin for the reply, and the plan for it went as far
as arguing about a delimiter. adh does no such thing: it emits and *stops*, and a second
invocation resumes with `--response <file>`, where `-` is stdin. gnosis already had that
shape in two commands, so the chaining was never missing — reading the reply from a pipe
was, and that is one flag. The cost of believing the entry would have been a wire format
for a tool designed never to speak one.

"Prompts are never cleaned up" named the right defect and the wrong trigger: it said to
remove the prompt "once the reply is cached". Caching happens before the reply is even
parsed, so that would delete the metadata an agent needs to submit a corrected reply
under the same key — turning "the YAML is malformed, fix it" into advice nobody could
take. The removal belongs at the *filing*, which is the one outcome after which nothing
more will be admitted under that key.

`go-git/v6`'s entry claimed "the exposure is one function and its tests". Measured: three
production files and about a dozen API surfaces. The correction changes the shape of the
risk rather than its size — only one of the three is in the evidence path, and the other
two read the user's own repository, so an alpha bump breaking them surfaces in
`standards --since` and the trail-health check rather than in tier 0. Worth recording
because the next reader would otherwise re-derive it, and would find the entry
reassuring.

**A specified table left a precondition in prose, and prose has no failure mode.**
§14.3.2's three drift states all begin "bytes differ", which reads as something the
caller establishes first. Implementing it that way would have made every caller
responsible for a comparison the function could do itself, so `Drift` takes both hashes
and answers `drift-none` when they match — total, with no way to call it wrongly. This
is the same correction the writer lock got: a precondition carried in a doc comment was
the one thing seven functions asserted and one caller did not do.

**The dangerous direction was in the inputs, not the logic.** A fetch that returns
nothing — a 404 body, a login redirect, a truncated read — differs from the archive by
hash, and every recorded passage is genuinely absent from empty text. Handed straight to
`quotecheck`, one network error reports `drift-unsupported` for every claim resting on
that source: the most serious signal this system has, manufactured by a failed
connection. Guarded inside the pure function rather than in the caller, because it is a
property of the inputs and the caller cannot see the consequence.

**Running the command found the reporting defect the fixtures could not.** A re-check
that finds changed bytes archives them, so the *next* re-check sees a second record for
that URI whose text no claim cites yet. Those accumulate one per run, and the first
report listed each of them by URI with nothing to distinguish the versions — so a settled
corpus would grow a page of lines meaning "nothing happened" around the one line that
mattered. Fixed by counting them and printing the archive path beside the rows that
remain, and by a test that reproduces three versions rather than one. This is the third
pass in a row where the defect that mattered came from running the binary and not from
the suite, which is now a pattern rather than an anecdote: fixtures assert what was
thought of.

**Two entries were correct and cheap, which is worth saying.** The remote-clone gap and
the per-claim freshness gap were both accurate, and both took one file. `git daemon` on a
kernel-assigned loopback port exercises a real capability advertisement and a real
shallow negotiation, and a bare listener that accepts and hangs up asserts the failure
is an *error* rather than an empty candidate set — a source that silently archives
nothing is the outcome worth a test. Per-claim freshness is one measurement used at two
grains, deliberately: the document line stays the weakest of its claims, so the
conservative answer a reader already had is not replaced by the useful one.

**Authentication is still untested, and the test that says so is a skip.** A fixture that
fabricated a credential store would assert that go-git reads credentials the fixture
handed it — true of any library, and silent about whether a real clone against a private
remote works. It is a `t.Skip` with the reasoning in its comment rather than a line in
this file, so it appears in every run and cannot be deleted quietly along with the thing
it is about.

### 6.20 What Paying Down Tiers 2 and 3 Turned Up

Ten items. Two were decisions not to build, and both of those started as entries that
sounded cheap — which is the pattern worth naming from this pass: **an entry that
describes its own cost is describing a guess, and the guess was wrong in the expensive
direction twice.**

**`revision_count` was "nearly free" and cannot be built at all as described.**
`engram`'s increment-on-upsert has no meaning here, because the index is *rebuilt* from
the corpus — a counter resets to one every rebuild and reports a churning document as
new. Deriving it from git instead breaks a property the index has and the entry did not
consider: `Digest`'s contract is that two colleagues at one commit hold indexes that
answer the same questions, and history is not part of the corpus. gnosis's own git
adapter clones with `Depth: 1`, so a bundle fetched from a remote would report 1 for
every document. The column would make the index depend on how somebody cloned. The
question is still worth asking and `git log --oneline -- <path>` already answers it.

**The definition-drift detector is not built, and the reason is §6.2.** Every signal
the entry names — bimodal values, a dimension inconsistent with its declaration, a
cluster of new aliases — needs a threshold, and there is no corpus of claims to
calibrate one against. Shipping invented numbers is precisely the value nobody can
argue with a year later. What the pass did instead was narrow the entry: the
*structural* half turns out to be closed already (`ontology.indexSubjects` indexes each
subject key with its aliases, so a key colliding with an alias is rejected at load),
and the entry now carries a trigger that can fire — the first subject key carrying
claims from two documents with disjoint evidence sets.

**Three entries were wrong about their own mechanism, and one of the errors would have
shipped.** `rationale`'s fold-and-compare had to fold *case*, which every quotation
guard in this family deliberately does not: `textnorm` preserves case because a
quotation differing in case is a different quotation, and a rationale differing in case
is the same boilerplate. The first implementation was case-sensitive and its own test
caught it. Then the first *scoping* was wrong in the other direction: counting refused
promotions as prior rationales made `promote` refuse its own second half, because the
confirmation flow previews, records the blocked outcome carrying the rationale, and then
applies. Two CLI tests caught that one. The correct reading — "already recorded" means a
decision that landed — is also the faithful one.

**"A declined promotion" was three events wearing one word**, and separating them is
the whole of the answer: a gate refusal is recomputable and stays per-user; a person who
walked away decided nothing and is what `audit --outstanding` exists to surface; a
person who dropped the draft made a decision no re-computation recovers, and that goes
in `log.md`. An agent's discard deliberately does not, because dropping a draft grants
no authority and committing every one would fill the history with what §12 calls the
category a reader learns to skip.

**Running the command found two defects the suite could not, again.** Writing the
decline to `log.md` surfaced a bug in `okflog.Parse` that had been latent since the
seed log was written: `log.md`'s preamble explains its own format by *showing* it, in an
indented code block, and the parser treated a four-space-indented `## 2026-01-31` as a real heading — so
the first command ever to write to a fresh log re-emitted it at column zero and turned
the file's explanation of itself into a fabricated January entry. Nothing caught it
because nothing had written to `log.md` before. And `audit --outstanding` printed
`asked 2 times` then `asked 2026-08-23`, the word twice in one line, which reads as a
formatting bug rather than a report.

**Two fixtures passed for the wrong reason and the linter found one of them.**
`Outstanding`'s "asked, then promoted" and "asked, then discarded" cases were written
with no draft present, so the absent draft alone cleared them and the two terms they
were named for were never exercised. Separately, `unparam` reported that a test helper
always received one path — which was true, and the missing case was "two abandoned
decisions appear, in order".

**The one item whose fix was purely a strengthening.** `standards.Unread`'s
classification was defended by a test comparing the answer to a literal list, which is
two copies of one fact agreeing by construction. It is now scanned out of the source:
every read of a standards value goes through `.<Field>.Value`, so the compiler's own
symbols are the evidence, and both directions fail — a dead knob claimed live, and a
live knob recorded as dead. Verified by misclassifying `in_degree_cut` and watching it
fail for the right reason.

**A markdown rule was disabled, and measuring is what justified it.** MD063 (heading
title case) was enabled in `.rumdl.toml` and never followed; the entry recorded that
"all of it is formatting `rumdl fmt` would fix in one pass". That is true of MD060 and
was never true of MD063: every heading here is §-numbered, MD063 counts the number as
the first word, and it therefore asks for "1.1.0 a Specimen, Because…" — running the
formatter would degrade eleven headings. The 138 remaining MD060 table-alignment issues
still want their own commit, for the reason they always did.

### 6.21 What Paying Down Tiers 2 and 3 Turned Up

Eleven items, four built and six recorded as decisions, one merged into another. **The
work was mostly measurement**: six of the eleven could not be built, and three of those
had premises I could name as false before writing a line.

**Three things nothing writes, and two entries resting on them.** `grep -rn "INSERT
INTO claims"` returns nothing. The `claims` and `claim_subjects` tables are created by
migration and read only by `Digest`; claims live in document frontmatter, and `claimsOf`
does not parse a `subject` at all. So the claim-level index is scaffolding for Phase 3 —
which blocks the definition-drift report outright, and not for want of calibration but
because *the join does not exist*. It also blocks "was this claim ever used", because
`links.source_claim_id` is filled only when extraction identifies the claim containing a
link. Both entries now say so; the previous revision of the drift entry had promised a
report that had nothing to report.

**`lint --since` does not exist.** The entry asking for a "what the corpus gained"
column says it "already reports what a change made worse". The only `--since` flags in
the tool are on `standards check` and `log`. So that item is a baseline mechanism plus a
column, which is a different size of job, and Hamming's argument for it is untouched.

**A fourth gate verdict was refused because gnosis already has one.** `haft`'s
`review_ready` adopts the candidate, prints a warning, and retains the findings — which
is exactly what a promotion carried over unrun signals does: `apply` records
`Signals: carried`, the envelope carries them, and `debt` reports every document
admitted that way. What the surveyed verdict adds is the removal of the person, and
§9.5 exists to require them. The entry was right that it was a genuine question; the
answer is that the feature exists and the difference is the part not to adopt.

**Two entries merged, and the junior one supplied the correction.** §11.0.2's
retrieval-case file proposed asserting on concept identifiers. The
competency-question entry pointed out that identifiers are assigned per corpus — so a
case file naming them is unportable, unreviewable, and turns a failing case into
archaeology — and proposed titles. They are one instrument at two grains, so there is
one file, and it asserts on titles. Two files to author cases in is one too many.

**A `default` branch nearly launched a collapse this codebase refuses everywhere.**
Splitting `Churned` for the complexity linter, I replaced an explicit
`case DriftUnchecked, ""` with `default` — which silently swallowed `drift-none` into
the unchecked count, under a comment asserting the two "mean the same thing to a
reader". They mean opposite things: compared-and-unmoved against not-compared. Caught by
re-reading the comment I had just written, and fixed with a fourth count rather than a
reworded comment.

**Two fields got a reader in the same pass that created them.** Last pass recorded a
lesson about a stored value nobody reads being the other half of the
reported-but-not-stored mistake. So `audit.Row.Unsupported` shipped with
`audit --unsupported`, and the drift verdicts stored last pass got their second reader
in `audit --churn`. `audit` now carries three reports over what the tool recorded, and
the help text outgrew `New` — which the length linter reported and was right about for
the wrong reason: registering a command is not the same job as explaining it, so the
prose is a constant.

**One deletion worth naming.** `checksByArchivePath` became a one-caller wrapper hiding
a second return value, and `nilnil` flagged the wrapper rather than the design. Deleting
it was the right answer to the lint, and the replacement records why the two maps must
stay two: `verified` reads a *missing* key as §14.3's `unknown`, so a single map with an
entry for every path would make every source read as checked-at-1970 — the collapse the
four-state vocabulary exists to prevent, arriving as a refactor that looked like tidying.

### 6.22 What Building `gnosis schema` Turned Up

Two backlog entries, one job, and the entry that claimed to close the other was right
about the mechanism and optimistic about the scope.

**`gnosis schema` did not exist.** §5.7 specifies it — generated `AGENTS.md`, per-agent
symlinks, `--check` for drift, "hand-written sections preserved between markers" — and
`ls cmd/` had no `schemacmd`. Measured before planning, which turned an adoption into a
build.

**"Hand-written sections are preserved between markers" is one clause and four rules.**
Three are in the surveyed tool: replace the region, preserve everything else, never
overwrite an unmarked file. The fourth the code needed and nobody had written down — a
marker that opens and never closes is a *refusal*, because reading it as "everything to
the end of the file" lets one typo hand a whole document to a generator. It is now
§5.7.1.

**Three defects, and each was found by a different thing.**

A test found that the merge **added a blank line per run**: a rendered region carried
its own trailing newline and the replaced span already included the file's, so
`--check` would have reported drift against a file it had just written. The fix is that
`marked` renders without a trailing newline and `Render` supplies the separators —
which makes the rendered form its own fixed point, and that property now has a test of
its own.

A test found that **a truncated marker was not refused.** `unclosed` returned a region
name, and "no name" was also how it said "no problem" — so a half-written marker, which
is precisely the file nobody should write over, was waved through. The exported form is
a comma-ok pair now, and the empty name is a real answer.

Running the command found that **the command list came back empty.** `c.Command` in a
command package resolves to that package's own field and shadows the embedded root's,
so `c.Command.Subcommands` was the schema command's own subcommands: none. It compiled,
ran, and wrote "No commands were reported, which is a defect in gnosis" — a message
that turned out to be describing itself. This is the same shadowing that made
`admitcmd`'s flag `FromStdin`, and the second time it has bitten.

**`link`'s two refusals are the marker contract's argument one level up.** It will not
replace a regular file, because somebody wrote it and a symlink would delete it. It
*will* repoint a symlink, because that is what the command is for. Distinguishing them
needs `Lstat` rather than `Stat`: `Stat` follows the link, so after the first run it
would report the command's own work as a regular file and the second run would refuse
to do what the first one did.

**Where the region question actually lands.** The entry claiming to close the
machine-versus-human-owned-regions item supplied the mechanism and not the scope. The
blocker — "needs a way to mark regions" — is gone, and `AGENTS.md` is the first document
using it. Concept documents are a separate decision, because a marker inside one
interacts with anchors, with quotation validation, and with the index: a §5.5 change
wearing a §6.3 hat. It has a trigger now rather than a phase label.

**One thing noticed and filed rather than built.** Nothing reports a missing
`AGENTS.md`: `init` does not scaffold one and `doctor` — which reports every other
absent apparatus file — is silent about it. The fix is one entry in
`diagnoseBundleFiles`, and it is filed because scaffolding it in `init` is the
alternative and the two are mutually exclusive.

### 6.23 What Clearing the Unblocked Backlog Turned Up

Eleven items. Two were not what my own priority listing had said, one decision I had
already made turned out to be wrong, and a test is what found it.

**My listing was wrong twice, and both are corrections to my own work rather than to the
entries.** `:785` (indicator words as data) was listed as unblocked; it has no consumer —
§17.4's `lead` check does not exist and `internal/segment` splits on coordination, not on
reason-giving — so the data file would have shipped with nothing reading it, which is the
mistake this project has now recorded three times. And `:883` (initial `standards/`
values) was stale: the four files exist, seeded, each value carrying its rationale.
Nothing closed it because nothing was looking at it.

**A test overturned a decision I had reasoned my way to.** I decided `doctor` should
report a missing `AGENTS.md` and `init` should *not* scaffold one, on the argument that a
scaffolded copy would be stale from the first vocabulary edit.
`TestInitialisedBundleIsHealthy` then failed: my change made every freshly initialised
bundle unhealthy on creation. The resolution dissolves the objection rather than splitting
it — `init` **generates** the document, which is not a scaffolded copy but what
`gnosis schema` would write that second, exactly as `init` already opens the index rather
than shipping a database. The check stays for the case it is actually for.

**And that forced a restructure that fixed a defect I had shipped an hour earlier.**
`init` needed the same generator, so the plan-and-write moved into
`internal/bundle/schemadoc.go` — and writing it there made obvious that `gnosis schema`
had been writing `AGENTS.md`, a committed file at the bundle root, **without the writer
lock**. `Writer.Log` already takes it for the one other committed root file a command
writes. Two processes running `schema` would have interleaved. The lock is a
`(*Writer)` method now and the compiler asks.

**The Fixable column is the fourth thing in this codebase that is declared and checked
in both directions.** `lint.Check.Actions` joins `Categories` for the same reason — an
action is a field inside a `Run` body, so nothing can enumerate it by inspection — and
the column is *last* in §12.1's table because `spec_test.go` reads the enforcer by
position: a column inserted earlier would change what that test walks while still
passing. Verified by mis-stating one cell and watching both halves of the assertion fail.

**The gains report needed the entry's premise corrected to become cheap.** `:791` says
"`lint --since` already reports what a change made worse"; there is no `lint --since`.
But the gains were already in the trail three reports read, so `--gained` is a fourth
pure fold rather than a baseline mechanism. It is `ok` always — exiting non-zero on good
news would be the asymmetry Hamming's argument is about, arriving through the exit code —
and it takes a window, because a total since the beginning only grows and therefore says
nothing.

**One library evaluated and declined with a measurement.** `github.com/skosovsky/okf`
v0.2.1 exists and is a real toolkit. It implements OKF **v0.1** where this bundle
conforms to v0.2; it **preserves CRLF** where gnosis normalises it, which is not a
preference but a change to what counts as the same words in the quotation check the
corpus rests on; and its `LoadBundle(root string)` does its own I/O where `okf.Parse`
takes bytes. No dependency was added, and the reasons are recorded so nobody re-derives
them.

**One outward change, and checking first changed the method.**
`~/Documents/agent-red` is **not under version control**. A 1,059-line replacement there
would have been unrecoverable, so the superseded content moved to
`manifesto.superseded.md` and the pointer names which sections were *rewritten* rather
than moved — because a bare redirect leaves somebody who remembers the old text unable
to find the new.

### 6.24 the Subject Decision, and Why It Was Two Questions

Recorded because it was the backlog's most consequential open decision, and because
separating it into two is what made one half cheap.

**Three of the four things one might argue about were already settled**, and finding that
out was most of the work. §5.8.3: a subject is declared, never inferred, and its absence
is reported rather than blocked. §10.2: a subject key buys candidate narrowing and
nothing more, so a wrong key costs a wasted comparison rather than a wrong answer.
§10.2.1: `claim_subjects` is a cache of a parse, regenerable, so improving the parser is
a reindex rather than a corpus rewrite. What was open was narrower than "does a claim
carry a subject".

**Question one: at what grain?** §5.4 listed `gnosis_subject` among the *document's* keys,
glossed "what this **claim** is about" — and everything downstream is per claim:
`claim_subjects`' primary key, §5.8.3's wording, §10.2's pairing. Decided per claim, and
§5.4 corrected. The interesting refusal is the middle option: a document-level key that
claims *inherit* unless they override is terser and fails in the way §5.8.2.1 is about —
editing it silently re-subjects every claim that did not override, which is definition
drift arriving through a convenience, invisible at the point of use. The correction cost
nothing to make because nothing reads the key yet, and that cost only rises.

**Question two: what writes `claim_subjects`, and when?** Decided: one writer, at
index-rebuild time, writing the declared and derived halves **together**, when the
operator patterns and their test corpus exist. The option rejected was the tempting one —
populate the declared half now, since it is available — and the reason is what `derived`
and `pattern_id` are for. Those columns exist to tell a parsed value from a pinned one,
and rows where they mean neither would make the table misleading for however long Phase 4
takes. A table with no writer is honestly empty.

**Separating the questions is what unblocked half the work.** The subject key is declared
frontmatter, so everything needing only *which subject a claim is about* reads the
document and needs no table. That turned out to be three specified checks and a report,
all blocked on **one field**: `lint.Snapshot` does not carry the vocabulary, which is why
`subject-missing`, `subject-unknown` and `ontology` are unbuilt together rather than for
three separate reasons. `bundle.inspectOntology` already loads it for `doctor`.

What stays behind the writer is what genuinely needs the `claims` table: claim-level
search (§11) and claim-level link attribution. Two consequences worth stating rather than
discovering: `Digest`'s guarantee over `claims` and `claim_subjects` is **vacuous** until
the writer lands — not wrong, and meaningful the moment it appears — and the
definition-drift *detector* still waits on §6.2, because the population a threshold would
be calibrated against is exactly what the report produces. The report comes first for
that reason, not merely because it is easier.

### 6.25 the Warrant's Shape, and a Decision Withdrawn

The question was "what shape does `gnosis_warrant` take", and the answer turned out to be
"the one §10.6.4 already specifies" — with one field refused and an earlier decision of
mine withdrawn.

**Most of it was settled and the work was finding that out.** §10.6.4 specifies the
warrant to the field; `:1857` had already recorded, on 2026-08-23, that it ships whole or
not at all, because `skillet` and `canonizer` both cite that section and a subset under
the same name is the "reshaping done for local convenience" it warns against. Tiers,
co-signing and `override` are one mechanism. The admission rules — the fold-and-compare
refusal — are already built on `command.Promote.Rationale`, which the warrant inherits.

So the only open field was the approver's role, and **§10.6.2 had already decided how
authority enters this system**: not by roster, which "is a political artifact… it rots,
and it hard-blocks whenever the sole holder is unavailable"; not by behaviour, which is
self-certifying and "mistakes activity for competence"; but by domain history *computed*
from the warrants and shown rather than enforced, with `requires_capability` on a single
subject key as the narrow escape hatch.

**Reading that properly reversed my own earlier decision.** On 2026-08-23 I had recorded
"the role belongs on the warrant, as an attribute of the decision". It does not. The
entry asks for two things and they want different answers: *who is accountable for this
area* is a property of the **subject**, and *was this adjudicated by anyone who works on
it* is already answered by the computed history. Recording the approver's role answers
the second question with a fact about whoever happened to be in the review queue — and
answers it wrongly in exactly the case the history exists to detect.

**The sharpest of the three reasons is structural rather than editorial.**
`gate.Candidate` carries `Path`, `Before`, `After`, `Scan` and the parsed `Doc`; `Corpus`
carries archived text, fetched URIs and folded titles. A warrant field lives inside
`Doc`, which the gate already parses — so "the gate must not read the role" would be a
comment. The ontology is in neither input, so reading a subject's `owner` would require
widening the gate's inputs, which is a change a reviewer must argue for. §14.1's rule
that a tier is a signal and never a permission deserves an enforcement mechanism rather
than a promise, and this is the one place the two options differ in kind.

**One cost accepted rather than mitigated:** ownership does not survive a reorganisation.
That is right, because ownership is a current question — who to ask now — and §6.2's
precedent puts the change itself in `log.md`. Preserving it per warrant would buy a
historical fact nobody asks for at the price of the three problems above.

**And the item moved rather than closing.** `owner` waits on §13's review queue as its
first reader, not on the warrant. A key added before something displays it is the mistake
recorded three times already.

### 6.26 Where the Marker Contract Stops

The question was whether concept documents adopt the marker contract. Answered no, with
a measured boundary rather than a preference — and answering it surfaced a rule that had
been enforced by three mechanisms and stated by none.

**The boundary is `c/`.** `bundle.Load` walks the concept directory and skips reserved
names, so `AGENTS.md`, `index.md` and `log.md` are never parsed as concepts, never
indexed for full text, never anchor-matched, never segmented. A marker at the root costs
nothing. Inside `c/` the same marker is read as prose by four readers — the FTS body,
`Snippet`, `segment.Claims`, and the fold `claim-anchor` searches — because **nothing in
this codebase strips HTML comments**. Checking that was the work; the decision followed
from it.

**Three reasons, and the first is the one the entry did not weigh.** A concept body is
rendered from an admitted reply: every paragraph is a quotation-backed claim with an
identifier and an anchor. So regions would protect the least defensible content in the
document — prose with no claim, no anchor and no evidence, which §5.0.1 had just finished
saying the corpus declines to hold. Second, §5.3 makes frontmatter gnosis's extension
point and a marker in a body extends the document format itself; `AGENTS.md` is not an
OKF concept, so the contract there extends nothing. Third, §6.3's `synthesize` gate
already protects the thing worth protecting and protects it better: a region preserves
paragraphs, the gate preserves evidence.

**The imported design assumed the opposite shape of document.** The surveyed tool splits
regions because its pages are hand-written with machine sections. gnosis's are the
inverse — machine-rendered, with a person's edits being the exception. That inversion is
why the mechanism transfers to the apparatus files and not to the corpus, and it is the
kind of thing only visible once the mechanism exists.

**§6.3.1 is the by-product and may be the more useful half.** A concept body is the
machine's; a person's contribution goes to `gnosis_warrant.rationale`, to `log.md`, to a
linked `Decision` document, or through the relay as a claim. Every one of those was
already true — `renderQuarantined` writes the body, `claim-anchor` reports a displaced
anchor, §9.4 requires a quotation — and none of it was said in one place. Stating it turns
"we gate the whole rewrite" from an admission into a design, and it is what makes the
refusal above coherent rather than merely convenient.

**`index.md` is where the contract goes next**, and the need is already specified: §12's
`index-drift` says it "differs from what would be generated", nothing generates it, and
its seeded content is explicitly the curated part — "a handful of paths through the corpus
that a newcomer actually needs" — beside a list that could be derived.

**The revisit trigger is stated so this is not re-argued on taste:** a real `synthesize`
diff a reviewer cannot read. Prediction recorded with it — that it will not arise,
because a diff whose evidence must survive it is small.

### 6.27 What an Episode Needed, Which Was Not Accretion Rules

The question was whether the type vocabulary carries per-type accretion rules. Answered
no — and the way to the answer was working out what an `Episode` type would actually
require, which the entry had recorded as "different accretion rules" without going the
next step.

**Three of its four requirements were already met.** `normative = false` covers
prescribing nothing, `expects_subject` covers being about something, and "a commit hash
as evidence" is tier 0's git adapter. Listing them is what isolated the fourth.

**The fourth is not accretion; it is conflict eligibility.** *"We set the retry budget to
3 in March"* and *"we set it to 5 in June"* present to §10.2's interval detector as one
subject with disjoint values. Adjudicating that would deprecate one of them, which is the
corpus adjudicating its own history. Two reports of different moments cannot contradict —
and once that is stated, "an episode is not superseded by a later episode" stops needing
a rule, because §10.4 supersedes only the loser of an adjudicated conflict and there is
never one to adjudicate.

**So the vocabulary gains a fact, not a permission.** `normative` and `expects_subject`
say what the knowledge *is*; `episodic` is the third of that kind, and the three
behaviours — the staleness exemption, conflict ineligibility, supersession never firing —
are each derived at a place the corpus already asks "does this apply?" That is §12's
derived applicability and §10.6's "derived from the facts, not configured against them",
applied to the ontology.

**The refused alternative is worth recording because it is the obvious one.** A
`supersedable = false` flag would put policy in a vocabulary file, and the ontology is
per-corpus and editable — so a bundle could mark `Rule` unsupersedable and switch off
§10.4's central mechanism by editing data, with no check able to distinguish that from a
legitimate vocabulary choice. That is §6.2's concealed-loosening hazard arriving in the
ontology rather than in `standards/`. Deriving it from `normative` fails on `Reference`,
which prescribes nothing and is emphatically supersedable.

**One derivation is available before Phase 3, and that is what makes the flag earn its
keep.** `unverified` in `stale.go` reports a document whose sources were last verified
past the window; an episode's evidence is immutable, so the advice can never be satisfied
and the finding never clears. Checking that the consequence was live — rather than
assuming the whole item was Phase 3 — turned a blocked entry into two steps with the
cheap one first.

**And the type does not ship yet**, on §10.6's attenuation argument. A starter vocabulary
entry nothing uses is the dead knob this repository has now recorded four times, in a
different file.

### 6.28 the Operator Patterns' Schedule, and a Coupling I Invented

Framed as a scheduling question and it was mostly a correction: the patterns block less
than my own entries claimed, and the reason to wait is a property of the evidence rather
than of the calendar.

**The coupling was mine, not the schema's.** Yesterday's entries said the `claims` table
"lands with the `claim_subjects` writer". The foreign key runs from `claim_subjects` to
`claims`, so `claims` is not coupled to the parser at all. Checking what each blocked
thing actually needs: claim-level **search** waits on *extraction* to supply a claim's
title, description and lead, which frontmatter does not carry; claim-level **link
attribution** waits on the link extractor reporting byte offsets — `LinkRow` has none —
plus `claims.pos` from the anchors. Neither wants a parsed constraint. The patterns block
conflict detection, `constraint-coverage`, `constraint-drift`, and the drift detector.
Nothing else.

**The scheduling argument turned on an asymmetry worth reusing.** Two artifacts here are
"data with a test corpus authored from real failures", and they schedule oppositely.
Retrieval cases needed the instrument first, because a disappointing query is ephemeral —
that was §11.0.2's whole argument for shipping the grader with an empty file. Operator
patterns need the *consumer* first, because a mis-parsed claim is durable: it stays on
disk and §10.2.1's regenerability fixes it retroactively on the next reindex. So the
question to ask of the next artifact of this shape is not "is the data authorable yet" but
**"does the evidence survive waiting"** — and that reframing is the transferable part.

**One appealing option was circular.** Triggering the work off `constraint-coverage` reads
well — build patterns when the corpus shows they are missing — and cannot bootstrap:
with no patterns, coverage is zero everywhere and cannot distinguish "no quantity present"
from "a phrasing we miss", which are §10.2.3's own two causes. It is the maintenance
mechanism, and the trigger is conflict detection beginning.

**Nothing is pre-positioned**, including the units library, which turns out to be in the
module graph already as an indirect dependency — so there was nothing to reserve, and the
unused-dependency rule settles it.

**And one part is explicitly not deferred with the rest**: §10.2.2's rule that a finding
derived from a parse shows the parse. It ships with the first pattern. It looks cosmetic,
it is the difference between a false conflict dismissible in seconds and one that erodes
trust in the queue, and it is exactly the sort of thing that gets dropped from a large
change for looking like polish.

### 6.29 the Indicator Words, and a Choice That Was Not One

Posed as *does `lead` get built, or does segmentation cut at a reason?* Both halves were
wrong, and the second time in two days that framing a decision was mostly discovering the
question had already been answered somewhere I had not read.

**The segmentation half was decided in code, with a reason, before I asked.**
`internal/segment`'s coordinating-join list carries a comment saying subordinating joins
are excluded because cutting there produces a claim whose truth conditions changed. So
"cuts at a reason" was never an option to weigh; it is the rule the package exists to
enforce.

**The `lead` half had its dependency backwards.** The lexicon does not unblock the check —
`claims.lead` has no writer, frontmatter carries none of the three fields extraction
supplies, and §10.2.1 already said so. The check waits on extraction, and building a word
list does not advance it by a day.

**What the probe found was a third thing, and it is the useful part.** Running
`segment.Claims` over sentences whose right clause opens with a reason marker:
`"The retry budget is three, and because the SLA is 400ms."` cuts into two, and the second
is `"Because the SLA is 400ms."` — a fragment asserted as an independent claim. The
package's stated invariant does not hold. `standsAlone` accepts it because it looks for a
copula with something before it, and the fragment has one. **A copula test cannot close
this; only knowing what *because* does can.** So the lexicon has a consumer today, and it
is a repair of a violated invariant rather than a new feature.

**The verb was wrong throughout, and fixing it settles the risk the entry had already
named.** The words do not tell segmentation where to cut. They tell it when to withdraw a
cut proposed nearby. That inverts the failure mode: a *because* inside a quotation now
causes a refused cut, leaving a coarser claim whose evidence must cover more of it.
Under-segmentation is a claim harder to support; over-segmentation is a claim that was
never made. Only one is recoverable, and the errors fall on that side by construction —
which is a better property than a more accurate word list would have bought.

**One appealing option was circular, again.** Deriving `lead` from the claim text with the
same lexicon — the conclusion is the clause no reason marker introduces — produces a lead
for every claim immediately and makes §17.4's check vacuous, since it would test a
derivation against the rule that produced it. Recording the rejection in the spec matters
more than the rejection: without it, the cheap way to "unblock" §17.4 later is to build
exactly the thing that empties it.

**And the file ships whole rather than halved.** Only the reason rows have a reader; the
conclusion rows wait on extraction. Shipping half a closed lexical class to satisfy the
data-needs-a-reader rule would buy that rule a second authoring pass over the same
evidence. The compromise is that the file states which rows are live — the distinction
this project keeps needing, between *nothing yet* and *nobody looked*.

### 6.30 Review Gating, Where an Envied Design Was Already Behind Us

The entry read `akbp`'s `dry_run` / `approved` / `approval_required` on every write call
and concluded "theirs is harder to bypass". Measuring it field by field against
`internal/command` gave the opposite answer, and the third row is the one that settles
it: `approval_required` is a payload field, so **the party being gated declares whether
it is gated**. Here the requirement is the gate's `NeedsHuman`, computed by the
coordinator from its own report, and a caller cannot assert it at all.

**The dichotomy the entry posed had a third answer that was already built.** Gating is a
property of the command *value* rather than of the wire format or of a command somebody
remembers to run — which binds every caller, including the in-process ones a protocol
never sees. The package doc had said so for some time. This is the second entry in two
days closed by reading the code the entry was filed against.

**Checking it found a defect worth more than the decision.** `Promote.RequiresRationale`
had no writer: `Validate` checked it, the tests set it, and no caller ever did, because
`authorisedBy` derives the requirement inside the coordinator where the gate report is.
So the field's only effect was to make the type look like it enforced something it did
not — worse than its absence, and the fourth instance of stored state nobody writes. The
comment that replaced it says what the type can and cannot be wrong about on its own,
which is the general form of the mistake.

**The real question was one level down, and the entry's own trigger was pointing at it.**
Two gating fields are caller-asserted. In process that is sound, because the caller is a
person at their own terminal typing a flag. Over a wire it is not: `Approver: human:alice`
from a socket is unverified, and typing a document's path defeats muscle memory in a
person while costing a program nothing. So §4.6.2.1 fixes the rule before the transport
exists — the transport supplies the actor, the payload never does — and holds the
escalated path at a terminal until §13 has a review queue. Both are free today, which is
exactly why they had to be written now: the alternative was deciding them by default in
the window between the first socket and the first authenticated session.

**And two entries turned out to be one decision.** The token that lets §13 approve
remotely must be bound to the diff, and the apply must carry it back — which is the
"revision the preview was computed against" that §4.6.2 already required of a two-call
protocol. Filed months apart and phrased differently, they are one object; answering
either alone is how they end up inconsistent.

### 6.31 Graded Conformance, and a Condition Already Satisfied

The entry made graded OKF conformance conditional: *worth defining if gnosis ever
exports bundles other tools consume.* Checking the condition answered the entry. A
bundle **is** a directory of markdown in a git repository, and another tool consumes it
by cloning one — no export step exists to build or to decline, and the choice that made
it so was made long ago, when the derived state went into a gitignored `.gnosis/`. The
condition was satisfied before it was written.

**So the question became whether a grade is worth having, and it is not.** Grading is a
producer's instrument for stating *partial* conformance. There is none here: required
frontmatter is `type` alone, §18.5.1 pins OKF §11 with its negative requirements, and a
published level would always read full. A dial with one position. Worse, a level
structure defined by the only implementation certifies itself — §6.2's invented
threshold, arriving with a number instead of a rationale, which is the form that is
hardest to argue with later.

**The useful direction is the reverse of the one the entry assumed.** A grade describes
a corpus somebody else produced. `lint`'s `conformance` check already computes that for
this one, so the trigger is gnosis becoming a **consumer** of a second producer's
bundle, not gnosis exporting. Recording the inversion matters more than the decline: the
entry's own trigger could never have fired, because it named an event that requires no
work and therefore never announces itself.

**What replaced it was smaller and answerable today.** A foreign consumer's real gap is
not a number but a boundary — which files are a promise, which are readable but unowned,
which are cache and must not be parsed. Every part of that was already decided
elsewhere; §5.3.1 only writes it down. That is the third decision this week whose honest
resolution was to record what the code had already settled rather than to build
something.

### 6.32 the Seed Stays Live, and the Loosening That Escapes It

*Do `standards/` values become something people tune per corpus?* No — and the entry's
own condition turned out to be testable rather than speculative: no corpus has edited
any of the three policy files. The seed is a live default, so a corrected threshold
reaches every existing bundle; a scaffolded copy would freeze each one at its birth
values. §6.2's mechanism never depended on scaffolding, because a corpus's first edit is
diffed against the seed like any other change.

**One of the four files is the exception, and noticing that split was most of the
work.** Retrieval cases are queries about a particular corpus's content, ship empty, and
have no default to improve. The entry treated `standards/` as one thing; it is policy
with defaults, plus one file that is per-corpus by construction.

**Checking the fallback found a hole that scaffolding would not fix.** `archiveAtRef`
falls back to the *running binary's* seed — the only one it can reach — so for a corpus
that never edited the file, both readings are identical and no loosening can be reported.
A release that loosens a seed changes the effective gates everywhere, with no report and
no `log.md` entry: §6.2 bypassed by an install rather than by a commit. Scaffolding
inverts it rather than closing it, and makes the inversion permanent — a *tightened*
seed would leave every old bundle loose forever.

**So the fix goes where the decision is made, not where it is felt.** A seed loosening is
a §6.2 event in gnosis's own `log.md`, because that is where somebody chose the number;
`doctor` names the source of each gate set so a reader can find that entry. Both are
reporting. The tempting alternative — a recorded seed fingerprint per bundle — detects
the change more precisely and needs a writer, and this week already deleted one field
that had none.

### 6.33 the Transport, Decided by Reading §13

*Which transport, and is the ordinary path one call or two?* Both halves answered from
sections already written, which is now the pattern rather than the exception.

**§13 had already committed to HTTP in the coordinator's own process.** `gnosis serve`
carries the coordinator and the viewer together because two servers would be two
authorities over one bundle, and it is authenticated `net/http` with reverse-proxy auth
first-class. So the transport was never a free choice among peers — HTTP is required
before Phase 5 finishes, in the same process.

**The socket preference inverted under a rule adopted three decisions earlier.**
Filesystem permissions were its whole appeal, and §4.6.2.1 had just made the transport
responsible for supplying the actor. Peer credentials identify a process, not a person,
and cannot distinguish a user from an agent runtime running as them — which is exactly
what §9.5's refusal of a self-granted approval turns on. An argument that read as decisive
in one entry became the argument against, from a rule written in a different one.

**And the choice was never exclusive.** `net/http` serves on any `net.Listener`, so one
protocol with two listeners gets both properties: the socket keeps filesystem permissions
for the local single-user case, TCP behind the proxy gets real identity for the shared
one. This is the third dichotomy this week that dissolved on inspection — protocol *or*
command, `lead` *or* segmentation, socket *or* HTTP — and the shape is the same each
time: two options that compose, posed as alternatives because they were filed as
alternatives.

**The one-or-two question dissolved differently: it had a false premise.** `EffectApply`
already gates and writes under one hold of the lock, so §9.4's guarantee is structural
and there is no ordinary two-call flow to design. What a two-call flow needs — the
revision the preview was computed against — is required only where a person sits in the
middle, which is the escalated path, which already has it as §4.6.2.1's token. The entry
had been holding open a design question about a flow that does not exist.

### 6.34 Tier 1 Built, and Three Noise Defects the Suite Could Not See

Seven entries planned, five built, two corrected into honesty. The pattern worth keeping
is not in any of the features.

**Three separate findings would have made a check nobody reads**, and all three were
found by running the binary over a real corpus rather than by a test. The starter
vocabulary ships five types and a fresh bundle uses one, so `type-unused` reported per
type would have been the loudest check in the tool on the day a corpus is created. The
starter declares *no* subjects, only a commented example, so `subject-unknown` would have
fired on the first claim anybody wrote a subject on — teaching exactly the wrong lesson.
And `doctor`'s new gate-source line printed four identical sentences, because on a fresh
corpus every standards file falls back to the seed.

They are one defect in three costumes: **a signal that fires hardest when there is least
to say**. A test asserting "the finding is emitted" passes in every one of those cases.
What catches them is the question *what does this look like on day one*, and the answer
each time was grouping plus a derived-applicability guard. The third instance happened an
hour after the second was fixed, which is an argument for writing the rule down rather
than for trusting the reviewer.

**Two subject-verb disagreements shipped into messages** — "1 document declare", "1 claim
name" — for the same reason: a substring assertion sees `"1 document"` and stops. The fix
was to make the helper carry the verb, or to phrase so no verb has to agree.

**A layering rule decided the shape of the main feature.** `internal/lint` had exactly one
internal import. Rather than let it import `internal/ontology`, the shell flattens the
vocabulary into the value the checks compare against — and the same rule then decided how
the indicator words reach `segment`. One rule, applied twice on the same afternoon,
produced two designs that look alike; a package that imported what it needed each time
would have produced two that do not.

**Two entries were corrected rather than built, and both corrections were to my own
earlier corrections.** `:1114` said it needed "three small things"; nothing writes the
`claims` table, so `claims.pos` presupposed rows that do not exist and the writer was a
fourth thing nobody had named. And the machine-owned-regions entry had been holding an
`index.md` generator — a pure fold, buildable today — behind `synthesize`, which needs a
model. Splitting it was worth more than either half.

**One instruction in a backlog entry was wrong and was not followed.** `:790` asked for
`subject` in both claim readers. One of them builds the promote gate's shape, and giving
the gate a subject hands it a field §5.8.3 forbids it to act on. The entry's own adjacent
comment already said why — sharing a type gives a check the fields to start judging.

### 6.35 One Buildable Item in Four, and the Fourth Instance of One Defect

Tier 1 had four entries. Measuring before planning found **one** buildable, and the tier
assignment was mine from the day before — which is the second time this week a priority
listing of mine has been the thing that needed correcting rather than the entries.

**"Unblocked" and "one step done" are not the same claim.** The definition-drift entry's
report half shipped, so I listed the detector Tier 1; it is blocked on §6.2 for want of
a corpus to calibrate against, and making its trigger *observable* did not make it
*fire*. The episode entry's staleness exemption shipped, so I listed its second step
Tier 1; conflict ineligibility lands with a detector that is behind
`standards/operators.toml`. Both corrections are the same shape: progress inside an
entry does not move the entry.

**The buildable item's stated premise was false, and the false part was a spec row.**
`:1734` said `index-drift` "compares against nothing". It reconciles the derived database
against the bundle and works — §12.1's enforced row has always said so. The stale row was
§12's specified list, still describing an `index.md` check nobody built. Two rows, one
name, two subjects, and a reader following the wrong one would have built a checker that
already exists under another name.

**A seeded file argued against the feature, and reading it carefully was the design
work.** `init`'s `index.md` said: *"Keep it a map, not a mirror. A generated list of every
document is available from `gnosis search` and `gnosis graph`."* That is right about what
a person writes and wrong about where the derived list belongs — §5.7's argument is that
an agent reads **files**, so a listing reachable only by running a command is not
reachable by the reader that document exists for. The resolution was not to overrule the
prose but to split it: the generated region is the mirror, so the curated prose does not
have to be.

**And the fourth instance of one defect.** `init` seeded `index.md` unmarked, so the new
`gnosis schema` reported it as unmarked and exited non-zero — on every bundle, from the
day it was created. The suite caught this one, which is the difference from the three
last pass; the reason it caught it is that the fixtures *are* fresh bundles. The lesson
compounds rather than repeating: **the day-one question is answerable by building the
fixture out of `init`**, and where a test cannot, running the binary is what remains.

The other defect that pass needed the binary. The type grouping compared each document's
type against the previous one starting from `""`, so the group whose type *is* `""` — the
untyped documents, the ones `conformance` reports and a reader most needs labelled — was
rendered with no heading. An example-based test asserting "the entry appears" passes.

**The linter found dead code I wrote speculatively.** `planIndexDocFS` was an interface
for a caller I imagined `init` would need and it never did. Deleted. It is worth noting
because it is the failure mode the plan's own §rules 15 citation warns about, committed
in the same pass that cited it.
