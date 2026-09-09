# AGENTS.md — Guide for AI Agents Working on OpenSysML

> **Prime directive:** If I wanted hacks, I'd write it myself. **Don't ever choose hacky over correct.**
> Fix root causes upstream, not symptoms. Never weaken, skip, or delete tests to make them pass.

OpenSysML is a production-grade SysML v2 implementation in **Go 1.23+** (module `github.com/Open-MBEE/OpenSysML`).
It provides a hand-written lexer/parser, semantic engine, execution runtime, LSP server (`sysml-lsp`), and REPL (`sysml`).

---

## 1. Golden Rules

1. **Correctness over expedience.** No shortcuts, no stubs left behind, no lossy conversions. If a proper fix is large, do it properly or stop and flag it.
   - **For features specifically: do not minimize code changes or dodge complexity.** Implement the feature fully and correctly even if it touches many files, adds new types, or requires refactoring. Completeness beats diff size. See §8.
2. **Root-cause first.** Before editing, confirm *why* something fails (read the code, add a temporary debug print, write a focused test). Then make the minimal correct change.
3. **Never regress.** `develop` is green. Any test passing on `develop` must still pass on your branch. Diff against `develop` if unsure: `git stash && git checkout develop && go test ./... ; git checkout - && git stash pop`.
4. **Respect the architecture invariants** (see §4). The AST is immutable; semantics live in side tables; execution consumes lowered IR — do not bypass these.
5. **Tests are the contract.** Existing tests encode intended behavior (including *when* and *where* errors surface). Make code satisfy tests, not the reverse — unless the test is provably wrong, in which case explain before changing it.
6. **Leave no dead code.** Remove superseded helpers/structs. Run `go vet ./...` to catch it.

---

## 2. Build, Test & Verify

Use the Makefile (preferred) or raw `go` commands. **Never `cd`** — run from repo root.

```bash
make build            # build bin/sysml and bin/sysml-lsp (with version ldflags)
make build-sysml      # REPL binary only
make build-lsp        # LSP binary only
make test             # full suite: go test -race -coverprofile ... ./...
make test-short       # faster, no race detector
make clean            # remove build artifacts
```

Raw equivalents / targeted runs:

```bash
go build ./...                                   # must always be clean
go vet ./...                                     # must be clean (catches unused/dead code)
go test ./...                                    # all tests
go test ./internal/core/runtime/...             # one package tree
go test -run TestExecutionConformance ./internal/core/runtime
go test -race ./...                             # race detector (CI runs this)
gofmt -l .                                      # must print nothing (CI enforces gofmt)
```

**Definition of done for any change:** `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), and `go test ./...` all pass. Paste the results.

The OMG training-corpus gate is part of that suite but skips while the corpus is absent, so
fetch it once with `./scripts/download-training-examples.sh` and re-run
`go test -count=1 ./internal/core/model -run TestTrainingExamples`. CI downloads the corpus
too and sets `OPENSYSML_REQUIRE_TRAINING_CORPUS=1`, so there an absent corpus fails rather
than skips.

The three OMG pilot corpora are gated the same way: fetch them with
`./scripts/download-pilot-corpora.sh` and run
`go test -count=1 ./internal/core/model -run TestPilotCorpora`. CI sets
`OPENSYSML_REQUIRE_PILOT_CORPORA=1`. See `docs/project/pilot-corpora.md`.

All four roots share one mechanism (`internal/core/model/corpus_gate_test.go`) but two
policies, and the difference is deliberate: the training corpus is **asserted** clean, so its
expectation file holds no per-file counts and `-update-training` refuses to record one, while
the other three are a **per-file ratchet** whose every movement must be adjudicated. Do not
turn the assertion into a ratchet.

The RDF mapping has a per-file ratchet of its own over every model under `examples/`, the
downloaded corpora included: `TestCorpusRoundTrip` in `internal/core/export` converts each file
notation → Turtle → notation → Turtle and pins the verdict. Run it with both require variables
set after any change to `internal/core/export`, adjudicate every movement, then regenerate with
`-update-corpus-roundtrip`. See `docs/project/rdf-corpus-roundtrip.md`.

---

## 3. Repository Map

```
cmd/
  sysml/                 REPL binary
  sysml-lsp/             LSP server binary
internal/core/
  source/                source files, spans, line indexing
  lexer/                 hand-written scanner (~200 keywords)
  parser/                recursive-descent parser (never panics; emits ErrorNodes)
  ast/                   syntax tree nodes — IMMUTABLE after parse
  symbols/               symbol tables, scope trees
  resolve/               lazy, memoized name resolution
  semantics/             type system, conformance, multiplicity, const-folding eval
  passes/                tiered validation (syntax → nameres → type → constraint)
  lower/                 AST → execution IR (ActionGraph, StateGraph) for the runtime
  runtime/               execution engine (eval, instances, action/state executors)
  model/                 workspace / document management
  libs/                  stdlib bundling + conformance gate
internal/lsp/            LSP protocol implementation
internal/repl/           REPL loop
testdata/                shared fixtures (.sysml, .kerml, .golden)
examples/                example models and demos
docs/                    guide/ (handbook), reference/, internals/, project/ (status)
```

Read `docs/internals/architecture.md` before non-trivial work — it documents the pipeline, tiers, and test contracts in depth.

---

## 4. Architecture Invariants (do not violate)

- **Immutable AST.** `internal/core/ast` is syntax-only and is never mutated after parsing. All derived/semantic data lives in **side tables keyed by node/symbol**.
- **Parser never fails.** `parser.New(src).ParseFile()` always returns a tree; malformed input yields `ErrorNode`s + diagnostics, never a panic.
- **Lazy + memoized semantics.** Name resolution and type queries compute on demand and cache. Don't force eager work.
- **Tiered passes.** Higher validation tiers are skipped when a lower tier errors, unless a pass declares `passes.ElementScoped` and gates itself per subject via `Context.DownstreamOfFailure`. Keep passes independent and level-scoped.
- **Runtime consumes lowered IR.** Executors should operate on `internal/core/lower` graphs (`ActionGraph`/`StateGraph`) as the single source of truth — do **not** re-parse `symbol.Decl` inside executors, and do not build parallel/duplicate structures that can drift. Lowering must be lossless (carry guards, triggers, effects, pseudostate edges).
- **Error timing is part of the contract.** Constructors (`newActionExecutor`, `newStateExecutor`) succeed on structurally-empty inputs; "no initial node/state" errors surface at `initialize()`. Don't move error points without updating the corresponding tests intentionally.

---

## 5. Testing Strategy

### 5.1 Parser features — four-layer contract
When touching the lexer/parser or adding grammar:
1. **Conformance gate:** `go test -run TestStdlibConformance ./internal/core/libs` — all official stdlib files must still parse clean (no regressions).
2. **Golden ASTs:** `go test -run TestGolden ./internal/core/parser`. Add a representative fixture under `internal/core/parser/testdata/parse/*.sysml`.
3. **Negative tests:** `go test -run TestNegative ./internal/core/parser` — malformed input must produce diagnostics without panicking.
4. **Update goldens only after intentional changes:** `go test -run TestGolden -update ./internal/core/parser`, then review the diff carefully.

### 5.2 Behavioral features (actions/states/calc/constraints/requirements) — four-layer contract
1. **Golden AST fixture** locking parse structure (`internal/core/parser/testdata/parse/`).
2. **Execution conformance:** add `.sysml` + `.expected.json` under `internal/core/runtime/testdata/conformance/`; run `go test -run TestExecutionConformance ./internal/core/runtime`. Schema is documented in that dir's `README.md`.
3. **Golden execution traces** for ordering-sensitive behavior (fork/join, transitions): `go test -run TestExecutionTrace ./internal/core/runtime` (update flag: `-update-traces`).
4. **Robustness:** add a failure-mode case to `robustness_test.go` (deadlock, unbound params, missing refs, dangling transitions, step budget). Must return typed errors, never panic or hang.

Then update `docs/project/spec-compliance.md` mapping: semantic rule → implementation (file:function) → test → status (✅ faithful / ⚠️ approximate / ❌ not implemented / 🚧 known failure).

### 5.3 General
- Unit tests live beside code as `*_test.go`, one concern per test.
- Design/adjust tests **before or alongside** implementation; don't retrofit weak tests afterward.
- Prefer real SysML models in `testdata/` over hand-built ASTs when exercising end-to-end behavior; hand-built ASTs are fine for targeted unit tests.

---

## 6. Development Workflow

1. **Understand first.** Grep/read the relevant package and its tests. Diff the branch against `develop` to see what changed and why.
2. **Reproduce.** Run the failing test(s) and read the exact error before changing anything.
3. **Locate the root cause** in the correct layer (lexer vs parser vs lower vs runtime). Bugs in specialized layers are often upstream of where they surface.
4. **Implement the correct fix.** For bug fixes, keep edits minimal and scoped. For features, implement completely (see §8) — "minimal" means *no unrelated changes*, never *under-built*. Match existing style; keep imports at the top.
5. **Add/adjust tests** to lock in the fix and cover the failure mode.
6. **Verify** with the full gate in §2. Remove any temporary debug code and dead code.
7. **Commit** using Conventional Commits (see §7). Keep PRs focused (one feature/fix each).

---

## 7. Code Style & Commits

- **Formatting:** `gofmt` is mandatory (CI-enforced). Follow *Effective Go*. Document exported types/functions.
- **Changelog: add a fragment, never edit `CHANGELOG.md`.** Write the entry to
  `changes/unreleased/<slug>.<section>.md` (`<section>` is `added`, `changed`, `fixed`, …; body is
  the list item(s) only — see the README there). Concurrent PRs then cannot conflict on the changelog;
  the release procedure folds fragments in. `python3 scripts/changelog.py check` validates them.
- **Comments:** don't add or remove comments/docs unrelated to your change.
- **Commit messages — Conventional Commits:** `<type>(<scope>): <description>`
  - types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`
  - e.g. `fix(runtime): preserve transition effects when lowering state graph`
- **Never write internal work-item labels into anything a user reads.** Waves and slices
  (`wave 12A`, `W8G`), follow-up rows (`F4`, `F84–F95`), adjudication probes (`P1`) and diagnostic
  classes (`K5`, `S10`) are this project's private bookkeeping — a reader has nothing to resolve them
  against. Write what the change *did* instead: not "F3 unreserved these four", but "these four are
  unreserved by file kind".
  - This applies to `CHANGELOG.md` and `changes/unreleased/`, `README.md`, `docs/guide/`, `docs/reference/`, `docs/internals/`,
    doc comments, diagnostic messages, **and equally to PR titles/bodies, release notes and GitHub
    comments** — CI only guards the files (`python3 scripts/check-doc-ids.py`, or `make docs-check`
    with the link checker), so the prose you write around them is on you.
  - The conformance records under `docs/project/` are the one exception: they cross-reference each
    other by these labels, so each opens with a **Labels** note defining them. If you add a record
    that uses them, add that note too.
  - Real keyboard shortcuts (`F2` to rename, `F5` in VS Code) are not internal labels — spell them
    `<kbd>F2</kbd>` so their meaning is unambiguous.
  - Identifiers in code (an `errata.Entry.ID`, a test name) may keep their labels; when code points
    at documentation, point at the section's title, not at the label (`errata.Entry.Heading`).

---

## 8. Implementing Features & Complex Tasks

Bug-fix discipline (small, scoped diffs) does **not** apply to feature work. For features, the goal is a **complete, correct, production-grade implementation** — not the smallest possible change.

**Do not:**
- Minimize the diff at the expense of correctness or completeness.
- Avoid touching many files, adding new types, or refactoring when the feature genuinely needs it.
- Stub, fake, hardcode, or special-case a path to make a demo/test pass while leaving the general case unhandled.
- Bridge/adapter around the real design (e.g. copying IR back into legacy fields) to avoid a proper migration — this drifts and loses data.
- Silently narrow scope. If you implement only part of a feature, that is a **known limitation** that must be called out, not hidden.

**Do:**
- **Implement the whole feature**, including the hard cases (nesting, hierarchy, orthogonal regions, error/edge paths), not just the happy path.
- **Follow the layering.** Put logic in the correct layer (lexer → parser → lower → runtime). If a feature needs new IR, extend `internal/core/lower` losslessly rather than re-deriving data downstream.
- **Refactor when the design requires it.** If the clean implementation needs a new type, an interface change, or migrating existing callers, do that — and migrate *all* callers, deleting the superseded code.
- **Prefer completeness over diff size** every time the two conflict.

**Workflow for a complex/multi-part feature:**
1. **Plan and decompose.** Break the feature into ordered, independently-verifiable steps. State the plan before large edits. For long-horizon work, keep a short scratch note (e.g. `progress.txt`) of steps done / remaining — but don't leave it in the final PR.
   - **When writing plans or specs:** Write the outline first (structure, task headers, DoD), then fill in each section completely, one at a time. Never write placeholder TODOs or "fill in later" — complete each section before moving to the next.
2. **Design the data first.** Decide the AST/IR/side-table shape before wiring behavior. Getting the representation right avoids downstream shortcuts.
3. **Write the test contract up front** (§5). Add the conformance/golden/robustness cases that define "done" *before or alongside* the code, including the hard cases — so you can't accidentally under-build.
4. **Implement layer by layer**, keeping `go build ./...` and `go vet ./...` green at each step.
5. **Handle all cases explicitly.** Every unsupported path returns a typed error with a clear message — never a silent no-op, panic, or wrong result.
6. **Remove interim scaffolding** (bridges, temporary fields, debug prints, dead code) before finishing.
7. **Verify the full gate** (§2) and **update `docs/project/spec-compliance.md`** with honest status flags.

**When a feature is genuinely too large to finish correctly in one pass:** stop and flag it. Deliver a correct, complete *subset* with the remaining scope explicitly documented as known limitations + failing/`t.Skip`-with-reason tests or a `known_failures` entry. Never fake completeness.

---

## 9. Common Pitfalls

- **Bridge/adapter shims that copy IR back into legacy fields** — these drift and lose data (e.g. dropping a transition's `Effect`). Migrate consumers to the IR instead.
- **Re-parsing `symbol.Decl` inside executors** — bypasses the lowering layer; use the graph.
- **Hard-failing in `ToActionGraph`/`ToStateGraph` on missing initial** — breaks the constructor-succeeds/`initialize()`-errors contract.
- **Forgetting `-update`/`-update-traces`** after an intentional golden change, or running them blindly and masking a real regression — always review the golden diff.
- **Leaving unused structs/functions** after a refactor — `go vet ./...` and clean them up.
- **Assuming where states/members live** (Members vs Substates vs Regions) — verify against actual parser output.

See `CONTRIBUTING.md` and `docs/internals/architecture.md` for the authoritative, detailed references.
