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

| Phase      | Adds                                                                               | Key constraint from the rules                                                                                         |
| ---------- | ---------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| 2 — Ingest | `archive`, `fetch`, scan, relay, quarantine, promote gate, **claims**, `challenge` | Fetch is shell; every gate is pure. Four adapters, one normalizing seam. Segmentation precedes the evidence invariant |
| 3 — Curate | `conflict`, `critic`, `gate`, `adjudicate`, `supersede`, subjects accrete          | `ruleset/conflict` must land in skillet first                                                                         |
| 4 — Scale  | derived constraints, operator patterns, optional rerank                            | Operator patterns are data with a test corpus, never regexes in Go                                                    |
| 5 — Serve  | authenticated web viewer, review queue                                             | `NewServer(...) http.Handler`; `addRoutes` never errors; explicit NotFound and healthz                                |

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
- **Claim anchors before any claim row is written** (§5.5.1). A claim's identity
  and address live in the document. Writing claims first and adding anchors later
  means every claim written in between has an identity no rebuild can recover.

**Phase 2 is blocked** on `skillet` promoting `quotecheck` with the
checked/unchecked third outcome, and on the claim segmenter existing in Go — the
reference implementation is Swift, so the algorithm transfers and the code does
not. **Phase 3 is blocked** on `skillet/ruleset/conflict`. Neither blocks Phase 1.

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
- **Freshness at the point of reading** (§14.3). **Done.** `lint`'s `stale` check
  and `show`'s freshness line both land, joined by `bundle.LoadFreshness`. The
  §14.3.0 distinction fell out of building it — `stale_after` governs the claim,
  `staleness_days` governs the check — as did the decision that never-checked is a
  state rather than a finding. Still per document rather than per claim; filed.
- **A relay test with a scripted model** (§18). `cmd/relay_test.go` hand-writes every
  reply, so nothing checks that a real agent handed a real emitted prompt produces
  one `admit` accepts. The method is in the manifesto: keep the runtime real, replace
  only the reasoning, and make the fixture assert on what the agent *sent*.
- **An `AI_POLICY.md`** for this repository. Not a code change and not a spec change
  — a repository is a corpus, §1.1 says a claim must name its witness, and this one
  does not.

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

| Step                   | State       | Notes                                                                                                                         |
| ---------------------- | ----------- | ----------------------------------------------------------------------------------------------------------------------------- |
| 0 — clean baseline     | **done**    | 4 scaffold issues to 0; `ShortHelp` placeholder replaced                                                                      |
| 0.4 — layering lint    | **done**    | depguard rules added. The first version was too broad and failed the domain's own external test package; scoped with `!$test` |
| 1.1 — domain identity  | **done**    | `ID` (UUIDv7) and `Slug`, split one-per-file because decorder orders types before funcs at file scope                         |
| 1.2 — OKF parse/render | **done**    | Verbatim-block round trip; CRLF normalisation pinned as a documented exception                                                |
| 1.3 — ontology         | **done**    | TOML rather than YAML — see the format finding below. `Dimension` split to its own file for the same decorder reason as 1.1   |
| 1.4 — index            | **done**    | Real temporary SQLite in tests, not a mock. FK pragma, cascade direction, and link degradation each pinned                    |
| 1.5 — reconciliation   | **done**    | Placed in `internal/gnosis`, not `internal/index` — see the deviation below                                                   |
| 1.6 — lint registry    | **done**    | 5 checks; applicability derived, skips always reported with a reason                                                          |
| 1.6b — bundle loader   | **done**    | `fs.FS`-based, so only its own tests touch a filesystem                                                                       |
| 1.7 — commands         | in progress | `lint`, `index rebuild`, `init`, `doctor` done and wired end to end; `show`, `search`, `graph` remain — see §5.3 and §5.4     |
| 1.8 — `documents_fts`  | **new**     | Phase 1 search is document-scoped (§19); one tokenizer constant shared with `claims_fts`                                      |
| 1.9 — `schema-shape`   | **new**     | `sqlite_master` against what the migrations declare; catches a partially applied migration the version check cannot see       |
| 2.1 — segmentation     | **done**    | `Claims` cuts only when the fragment's subject can be recovered; `Anchor` locates it, `Text` verifies it — see §6.8           |
| 2.2 — standards        | **done**    | `Value[T]` makes the rationale structural; the loosening direction lives in Go, not the file — see §6.8                       |
| 2.3 — tier 0 (pure)    | **done**    | `Decide` is a pure function of (candidate, gates); a record's sha256 is its own filename. Writing is 2.4's shell              |
| 2.4 — `fetch`          | **done**    | Four adapters, the pinned HTML extractor, `--dry-run` as a command field; a re-fetch of unchanged bytes is a verified no-op   |
| 2.5 — command + lock   | **done**    | `Effect` fails closed, `Promote` validates itself, one writer per bundle. Found three readers that were writing — see §6.9    |
| 2.6 — gate + quarantine | **done**   | Five signals run, two report `unchecked` and block; the gate proves it can fail on every invocation — see §6.10              |
| 2.7 — ingest/admit     | **done**    | Two-phase relay, content-addressed cache, `--cache-only`; segment-then-check wired end to end — see §6.11                    |
| 2.8 — log + audit      | **done**    | Two records, not one rendering of one: `log.md` committed, `audit.jsonl` per-user. Clock injected — see §6.12                |

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
