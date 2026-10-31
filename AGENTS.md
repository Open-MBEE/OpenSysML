# AGENTS.md — Guide for AI Agents Working on OpenSysML

> **Prime directive:** If I wanted hacks, I'd write it myself. **Don't ever choose hacky over correct.**
> Fix root causes upstream, not symptoms. Never weaken, skip, or delete tests to make them pass.

The OpenSysML MDK (Runtime Environment and Development Kit) is a production-grade SysML v2 implementation in **Go 1.26+** (module `github.com/Open-MBEE/OpenSysML`).
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
make test-shard SHARD=runtime   # one CI shard of the race suite (runtime|runtime-corpus|model|export|graph|codegen|tools|rest|migrate|cli)
make test-short       # faster, no race detector
make clean            # remove build artifacts
```

Raw equivalents / targeted runs:

```bash
go build ./...                                   # must always be clean
go vet ./...                                     # must be clean (catches unused/dead code)
go test ./...                                    # all tests
go test ./internal/exec/runtime/...             # one package tree
go test -run TestExecutionConformance ./internal/exec/runtime
go test -race ./...                             # race detector (CI runs this)
gofmt -l .                                      # must print nothing (CI enforces gofmt)
```

**Definition of done for any change:** `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), and `go test ./...` all pass. Paste the results.

The OMG training-corpus gate is part of that suite but skips while the corpus is absent, so
fetch it once with `./scripts/download-training-examples.sh` and re-run
`go test -count=1 ./tests/corpus -run TestTrainingExamples`. CI downloads the corpus
too and sets `OPENSYSML_REQUIRE_TRAINING_CORPUS=1`, so there an absent corpus fails rather
than skips.

The three OMG pilot corpora are gated the same way: fetch them with
`./scripts/download-pilot-corpora.sh` and run
`go test -count=1 ./tests/corpus -run TestPilotCorpora`. CI sets
`OPENSYSML_REQUIRE_PILOT_CORPORA=1`. See `docs/project/pilot-corpora.md`.

So are the textual-notation examples of the SysML and KerML specifications themselves:
`./scripts/download-spec-examples.sh` fetches the two PDFs at the same pin, renders them with
`pdftotext -layout` (poppler-utils) and extracts one fixture per example into
`examples/spec-examples/`; run `go test -count=1 ./tests/corpus -run TestSpecExamples`. CI sets
`OPENSYSML_REQUIRE_SPEC_EXAMPLES=1`. See `docs/project/spec-examples.md`.

So is the pilot's XMI of the standard library, which the identity gate reads: fetch it with
`./scripts/download-pilot-library-xmi.sh` and run
`go test -count=1 ./tests/identity -run TestPilotLibraryXMI`. CI sets
`OPENSYSML_REQUIRE_PILOT_LIBRARY_XMI=1`. So is the OMG PSSM test suite, which the SysML v1
migrator is gated over: fetch it with `./scripts/download-pssm-suite.sh` and run
`go test -count=1 ./tests/corpus -run TestPSSMSuiteMigration`. CI sets
`OPENSYSML_REQUIRE_PSSM_SUITE=1`. See `docs/project/pssm-migration.md`. Whatever sets a require
variable must run the matching download script first; the scripts are idempotent, and none
reports success over an empty corpus.

The Modelica Reference-FMUs gate the FMI integration the same way: fetch them with
`./scripts/download-reference-fmus.sh` (into `examples/reference-fmus/`, gitignored), install
the runner's simulator with `pip install fmpy`, and run
`go test -count=1 ./tests/fmi -run TestReferenceFMUs`. CI sets
`OPENSYSML_REQUIRE_REFERENCE_FMUS=1`.

The training, pilot and specification-example roots share one mechanism
(`tests/corpus/corpus_gate_test.go`) but two policies, and the difference is deliberate: the
training corpus is **asserted** clean, so its expectation file holds no per-file counts and
`-update-training` refuses to record one, while the others are a **per-file ratchet** whose
every movement must be adjudicated. Do not
turn the assertion into a ratchet.

`sysml -render-document -doc-form pdf` drives external converters (WeasyPrint, pandoc, mermaid-cli)
that nothing else needs. `./scripts/download-doc-pdf-toolchain.sh` provisions pinned copies under
`build/doc-pdf/`; point `OPENSYSML_WEASYPRINT`, `OPENSYSML_PANDOC`, `OPENSYSML_MMDC` and
`OPENSYSML_MMDC_PUPPETEER` at them to run the PDF integration tests. Those tests skip rather than
fail when a converter is absent, so a passing run proves nothing until the converters were found.
The same script installs Asciidoctor.js under `build/doc-pdf/asciidoctor/` for `rep language "asciidoc"`
prose bodies, which `scripts/asciidoc/asciidoctor-json.js` parses into JSON for the document planner;
point `OPENSYSML_ASCIIDOCTOR` at the installed copy. The planner's tests run against recorded converter
answers under `internal/ir/docplan/markup/testdata/`, and the `Installed` test there checks the recordings
against the pinned converter — re-record them (`asciidoctor-json.js < x.adoc > x.json`) when either moves.

The RDF mapping has a per-file ratchet of its own over every model under `examples/`, the
downloaded corpora included: `TestCorpusRoundTrip` in `tests/corpus` converts each file
notation → Turtle → notation → Turtle and pins the verdict. Run it with both require variables
set after any change to `internal/translate/export`, adjudicate every movement, then regenerate with
`-update-corpus-roundtrip`. See `docs/project/rdf-corpus-roundtrip.md`.

---

## 3. Repository Map

```
cmd/
  sysml/                 REPL binary
  sysml-lsp/             LSP server binary
internal/                one directory per layer; a package imports only the layers below it
  syntax/                source files and spans, diagnostics, lexer, parser, AST, pack, format
  semantic/              symbol tables, name resolution, suggestions, semantics, identity, highlight, query
  ir/                    lowered execution IR (ActionGraph, StateGraph), query/doc plans, views
  check/                 tiered validation passes (syntax → nameres → type → constraint), workspace edits
  exec/                  execution engine, SMT solving, analysis framework and engines
  translate/             RDF, XMI and notation conversion, code generation, interop
  doc/                   query execution, document IR, Markdown/HTML/PDF backends
  workspace/             workspace and document management, stdlib bundling, project files, env vars
  frontend/              LSP, REPL, gRPC and stdio transports, protobuf conversion, usage
tests/                   black-box suites and their fixtures
  hygiene/               module-wide checks (no production code imports testing)
  perf/                  benchmark harness (go test ./tests/perf -run '^$' -bench .)
  testutil/              gobuild (build a command under test), graphcmp (pointer-graph comparison)
  parser/                golden ASTs (TestGolden, -update) and negative cases, with testdata/parse
  grpc/                  gRPC conformance cases (TestGRPCConformance) driven over the RPC surface
  export/, resolve/, …   external-package (package x_test) suites, each beside its own testdata
  testdata/              shared fixtures (.sysml, .kerml, .golden)
examples/                example models and demos
docs/                    guide/ (handbook), reference/, internals/, project/ (status)
```

Read `docs/internals/architecture.md` before non-trivial work — it documents the pipeline, tiers, and test contracts in depth.

---

## 4. Architecture Invariants (do not violate)

- **Immutable AST.** `internal/syntax/ast` is syntax-only and is never mutated after parsing. All derived/semantic data lives in **side tables keyed by node/symbol**.
- **Parser never fails.** `parser.New(src).ParseFile()` always returns a tree; malformed input yields `ErrorNode`s + diagnostics, never a panic.
- **Lazy + memoized semantics.** Name resolution and type queries compute on demand and cache. Don't force eager work.
- **Tiered passes.** Higher validation tiers are skipped when a lower tier errors, unless a pass declares `passes.ElementScoped` and gates itself per subject via `Context.DownstreamOfFailure`. Keep passes independent and level-scoped.
- **Runtime consumes lowered IR.** Executors should operate on `internal/ir/lower` graphs (`ActionGraph`/`StateGraph`) as the single source of truth — do **not** re-parse `symbol.Decl` inside executors, and do not build parallel/duplicate structures that can drift. Lowering must be lossless (carry guards, triggers, effects, pseudostate edges).
- **Error timing is part of the contract.** Constructors (`newActionExecutor`, `newStateExecutor`) succeed on structurally-empty inputs; "no initial node/state" errors surface at `initialize()`. Don't move error points without updating the corresponding tests intentionally.

---

## 5. Testing Strategy

### 5.1 Parser features — four-layer contract
When touching the lexer/parser or adding grammar:
1. **Conformance gate:** `go test -run TestStdlibConformance ./internal/workspace/libs` — all official stdlib files must still parse clean (no regressions).
2. **Golden ASTs:** `go test -run TestGolden ./tests/parser`. Add a representative fixture under `tests/parser/testdata/parse/*.sysml`.
3. **Negative tests:** `go test -run TestNegative ./tests/parser ./internal/syntax/parser` — malformed input must produce diagnostics without panicking.
4. **Update goldens only after intentional changes:** `go test -run TestGolden -update ./tests/parser`, then review the diff carefully.

### 5.2 Behavioral features (actions/states/calc/constraints/requirements) — four-layer contract
1. **Golden AST fixture** locking parse structure (`tests/parser/testdata/parse/`).
2. **Execution conformance:** add `.sysml` + `.expected.json` under `internal/exec/runtime/testdata/conformance/`; run `go test -run TestExecutionConformance ./internal/exec/runtime`. Schema is documented in that dir's `README.md`.
3. **Golden execution traces** for ordering-sensitive behavior (fork/join, transitions): `go test -run TestExecutionTrace ./internal/exec/runtime` (update flag: `-update-traces`).
4. **Robustness:** add a failure-mode case (deadlock, unbound params, missing refs, dangling transitions, step budget) as a subtest of a `TestRuntimeRobustness<Feature>` function in `internal/exec/runtime/robustness_<feature>_test.go` — a new file for a new feature, so branches never edit one shared registry; `robustness_test.go` holds only the shared helpers and is not where new ones go. Must return typed errors, never panic or hang. The suite counters read every `TestRuntimeRobustness*` function, and gRPC cases follow the same pattern with `TestGRPCRobustness*`.

Then update `docs/project/spec-compliance.md` mapping: semantic rule → implementation (file:function) → test → status (✅ faithful / ⚠️ approximate / ❌ not implemented / 🚧 known failure).

### 5.3 General
- Unit tests live beside code as `*_test.go`, one concern per test.
- Design/adjust tests **before or alongside** implementation; don't retrofit weak tests afterward.
- Prefer real SysML models in `tests/testdata/` over hand-built ASTs when exercising end-to-end behavior; hand-built ASTs are fine for targeted unit tests.
- **Size caps.** `tests/hygiene/size_test.go` fails on a `_test.go` file over 2,000 lines or a fixture (under `testdata/` or named `*.golden.*`) over 200 KiB. Split the test file per §5.2 or shrink/digest the fixture; the pre-existing offenders are allowlisted there at their current size and may not grow — remove the entry once the file is back under the cap, and justify any new one in its reason.

---

## 6. Development Workflow

1. **Branch from `develop`, target `develop`.** `develop` is the default and integration branch; `main` is release-only (`release/x.y.z` and `hotfix/` PRs, cut by a maintainer). Cut ordinary development branches from `origin/develop` and open their pull requests against `develop` — never `main`. Only `release/x.y.z` (cut from `develop`) and `hotfix/` (cut from `main`) branches target `main`; see `docs/project/releasing.md`. Branch names are git-flow style, `<type>/<short-slug>` (see CONTRIBUTING.md § Branches).
2. **Understand first.** Grep/read the relevant package and its tests. Diff the branch against `develop` to see what changed and why.
3. **Reproduce.** Run the failing test(s) and read the exact error before changing anything.
4. **Locate the root cause** in the correct layer (lexer vs parser vs lower vs runtime). Bugs in specialized layers are often upstream of where they surface.
5. **Implement the correct fix.** For bug fixes, keep edits minimal and scoped. For features, implement completely (see §8) — "minimal" means *no unrelated changes*, never *under-built*. Match existing style; keep imports at the top.
6. **Add/adjust tests** to lock in the fix and cover the failure mode.
7. **Verify** with the full gate in §2. Remove any temporary debug code and dead code.
8. **Commit** using Conventional Commits (see §7). Keep PRs focused (one feature/fix each).
9. **Open the PR ready for review** against `develop`. The full CI tier runs on every push and must be
   green. Address review findings as they arrive rather than waiting for CI. Merging is a maintainer's
   decision. A draft is only for work you do not yet want reviewed — it runs the cheaper quick tier —
   and it has to be marked ready before it can be considered.

---

### 6.1 Bumping the extension-library pin

The vendored `OpenSysML Libraries` (`scripts/extension-libraries-pin.sh`, synced by
`scripts/sync-extension-libraries.sh`) are an input to gates far from `internal/workspace/libs`, and
the full CI tier is expensive, so run them locally before the PR, every time the pin moves:

1. `go generate ./internal/workspace/libs` — `stdlib.snapshot` embeds the libraries; commit it.
2. `make extension-libraries-pin-check` — Java-free, about a minute: the sync check, the snapshot,
   `tests/hygiene TestEngineContractCoversEveryBoundName` (a new binding in
   `internal/ir/queryplan/compiler.go` needs an `engine-contract.json` entry upstream, in the same
   library PR as the declaration), the provenance guards of both pilot-differential baselines, and
   the REPL/query tests (a new library name enters the REPL's "did you mean" suggestions and can
   collide with a fixture).
3. If a provenance guard fails, the baselines record the previous library digest: run
   `go run -C tools ./cmd/pilot-diff` and `./cmd/pilot-apijson-diff` (the pilot toolchain is in the
   blueprint), adjudicate every moved entry against the committed baseline, write the adjudication
   into `docs/project/pilot-differential.md` / `api-json-differential.md`, then re-record with
   `-update` and run `make docs-counts`. A library change that adds declarations always moves
   `pilotLibraryElements`; anything else moving is an implementation difference to explain.
### 6.2 Changing `internal/translate/export`: the api-json writer provenance

The api-json baseline's `api-json-writer` provenance digests **every non-test `.go` file of
`internal/translate/export`** — the RDF/api-json writer *and* the reverse converter
(`rdf_in*.go`), element ids, names, addresses — so any commit touching that package, however
small, moves the digest and turns the `rest` shard red on `TestCommittedBaselineStatesThisRepositorysProvenance`.
Before pushing such a commit (including a review fix pushed after a re-record): run
`go run -C tools ./cmd/pilot-apijson-diff -update` over the real corpus directories, confirm that
only `sourceDigest` moved (account for anything else in `docs/project/api-json-differential.md`),
and run `go test -C tools -count=1 ./referee/apijson` as the pre-push check.

### 6.3 Keeping the agent skills current

The skills that teach a coding agent to *use* OpenSysML live in their own repository,
[`Open-MBEE/OpenSysML-MDK-Skills`](https://github.com/Open-MBEE/OpenSysML-MDK-Skills): one
`skills/<name>/SKILL.md` per surface (install, CLI, REPL, authoring, checking, analysis engines,
service, LSP, MCP, Jupyter, WebAssembly, documents, draw.io, RDF conversion, v1 migration,
troubleshooting, modeling practice, and one per client library). Each is written against this tree
and states the `sysml -version` it was verified with, so like the extension-library pin it drifts
the moment a surface here moves. A change an agent would read about is not done until the matching
skill says the same thing:

1. **What moves a skill.** A `sysml` flag, REPL `%` command, exit status or diagnostic class added,
   renamed or removed; an MCP tool (a row in `internal/frontend/mcp/catalog`, the `sysml-mcp` man
   page and `docs/reference/sysml-mcp.md` move with it, and so does `skills/opensysml-mcp`); a gRPC
   RPC or a client method — each of the seven client skills, not just the one you tested; a rendering,
   diagram or document form; an environment variable, install option, package or release asset;
   migrator behaviour the modeling skill describes. If a change touched `docs/reference/` or
   `docs/guide/`, assume a skill has to follow and check.
2. **Verify, don't transcribe.** Build the binaries from the OpenSysML branch and check every
   claim against them — `sysml -help`, `-man`, the REPL `%help`, `sysml-mcp`'s `tool_catalog`, a
   probe model run through the command — and regenerate tables from the tool's own output rather
   than by hand. Stamp the `sysml -version` string the skill was verified with. Anything not in a
   tagged release is worded "in nightly builds and the release after `<latest tag>`"; never name a
   version that does not exist yet. A gap between the documentation and the binary is an OpenSysML
   fix, not something the skill papers over.
3. **Open the skills PR with the OpenSysML PR.** The skills repository has only `main`: branch
   `<type>/<slug>` from `origin/main`, open against `main`, and reference the OpenSysML PR in the
   body; its checks are `python3 scripts/validate-skills.py` and `python3 scripts/check-links.py`.
   It lands after the OpenSysML change is on `develop`, so what it describes is what merged.
4. **Rev the skills at every release.** When a release is cut, re-verify the skills against the
   release binaries, replace "nightly builds and the release after `<previous>`" with the tag for
   what shipped, and update the version stamps. The in-tree `packaging/agent-skills/` set is the
   smaller copy that ships with the tree and carries no verification stamps; its one version
   string, the Java skill's `<dependency>` snippet, is held to `client/java/pom.xml` by
   `client/python/tests/test_check_version.py` with the other in-repo references, so the release
   commit's version bump moves it.

## 7. Code Style & Commits

- **Formatting:** `gofmt` is mandatory (CI-enforced). Follow *Effective Go*. Document exported types/functions.
- **Changelog: add a fragment, never edit `CHANGELOG.md`.** Write the entry to
  `changes/unreleased/<slug>.<section>.md` (`<section>` is `added`, `changed`, `fixed`, …; body is
  the list item(s) only — see the README there). Concurrent PRs then cannot conflict on the changelog;
  the release procedure folds fragments in. `python3 scripts/changelog.py check` validates them.
- **Comments:** don't add or remove comments/docs unrelated to your change. Keep a code comment to
  one or two lines; no paragraphs.
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
  - Nor anything only the session that produced the change could resolve: no references to a
    conversation ("as discussed", "per your request"), no links to agent sessions, and no narration
    of your own process ("after reviewing", "I installed"). Write for a reader who has only the tree.
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
- **Follow the layering.** Put logic in the correct layer (lexer → parser → lower → runtime). If a feature needs new IR, extend `internal/ir/lower` losslessly rather than re-deriving data downstream.
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
7. **Verify the full gate** (§2), **update `docs/project/spec-compliance.md`** with honest status flags, and **update the agent skills** the feature's surface is described in (§6.3).

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
