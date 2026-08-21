# Gnosis Implementation Plan

Implements [`SPEC.md`](./SPEC.md); the backlog is [`TODO.md`](./TODO.md).
Reconciled against both on 2026-08-20 — see §5.4 for what the specification
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

## 3. Later Phases — Scope Only

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
  rather than merely the database — serializing SQLite writes while leaving
  markdown writes unserialized coordinates the cache and not the corpus. Readers
  stay independent of it by requirement, which is what keeps Phase 1 unaffected.
- **Claim anchors before any claim row is written** (§5.5.1). A claim's identity
  and address live in the document. Writing claims first and adding anchors later
  means every claim written in between has an identity no rebuild can recover.

**Phase 2 is blocked** on `skillet` promoting `quotecheck` with the
checked/unchecked third outcome, and on the claim segmenter existing in Go — the
reference implementation is Swift, so the algorithm transfers and the code does
not. **Phase 3 is blocked** on `skillet/ruleset/conflict`. Neither blocks Phase 1.

______________________________________________________________________

## 4. Per-Step Exit Criteria

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

## 5. Progress

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

### 5.1 Layer the Depguard Rule Had Not Named

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

### 5.2 Gap This Plan Missed

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

### 5.3 What Step 1.7 Turned Up

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

### 5.4 What the Specification Changed Under This Plan

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

### 5.5 What Building Phase 1 Turned Up

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

## 6. Rules Review of This Plan

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
