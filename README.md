# Open Source SysML v2 Implementation

[![CircleCI](https://dl.circleci.com/status-badge/img/gh/Open-MBEE/OpenSysML/tree/main.svg?style=shield)](https://dl.circleci.com/status-badge/redirect/gh/Open-MBEE/OpenSysML/tree/main)
[![Quality gate](https://sonarcloud.io/api/project_badges/measure?project=Open-MBEE_OpenSysML&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=Open-MBEE_OpenSysML)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=Open-MBEE_OpenSysML&metric=coverage)](https://sonarcloud.io/component_measures?id=Open-MBEE_OpenSysML&metric=coverage)
[![Maintainability](https://sonarcloud.io/api/project_badges/measure?project=Open-MBEE_OpenSysML&metric=sqale_rating)](https://sonarcloud.io/component_measures?id=Open-MBEE_OpenSysML&metric=Maintainability)
[![Reliability](https://sonarcloud.io/api/project_badges/measure?project=Open-MBEE_OpenSysML&metric=reliability_rating)](https://sonarcloud.io/component_measures?id=Open-MBEE_OpenSysML&metric=Reliability)
[![Security](https://sonarcloud.io/api/project_badges/measure?project=Open-MBEE_OpenSysML&metric=security_rating)](https://sonarcloud.io/component_measures?id=Open-MBEE_OpenSysML&metric=Security)

[![Release](https://img.shields.io/github/v/release/Open-MBEE/OpenSysML?label=release)](https://github.com/Open-MBEE/OpenSysML/releases/latest)
[![Go reference](https://pkg.go.dev/badge/github.com/Open-MBEE/OpenSysML.svg)](https://pkg.go.dev/github.com/Open-MBEE/OpenSysML)
[![PyPI](https://img.shields.io/pypi/v/opensysml?label=pypi)](https://pypi.org/project/opensysml/)
[![Python versions](https://img.shields.io/pypi/pyversions/opensysml)](https://pypi.org/project/opensysml/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-opensysml.org-blue)](https://opensysml.org/)

OpenSysML is a SysML v2 and KerML 1.1 implementation in Go. It provides a language server, an
interactive REPL, an execution runtime, an embeddable Go API, and Python, Node/TypeScript, Java
and Rust client libraries, covering the lifecycle from authoring through execution with the
integrated tooling systems engineers expect from a modern language ecosystem.

It is fast. The public [Apollo 11 SysML v2 model](https://github.com/airbus/apollo-11-sysml-v2)
— 28 files, 7,200 lines — parses in **8 ms** (42 MB/s on one core) and loads, resolves and
validates against the full standard library in **0.43 s**. The measurement and how to repeat
it are in [performance](docs/internals/performance.md#a-real-model-apollo-11).

The basis for these claims, and their limits, are documented in
[spec compliance](docs/project/spec-compliance.md) and the
[pilot differential](docs/project/pilot-differential.md): every diagnostic is compared against the
pinned OMG pilot implementation over that implementation's own corpora, and no conformance
certification is claimed.

## Quick start

**Introductory material:** [the guide](docs/guide/) and the
[document generation manual](docs/manual/README.md)

**Complete searchable documentation:** <https://opensysml.org/> — the same pages as
[docs/](docs/), rendered from `main`.

### Install

**Download pre-built binaries:**
```bash
# Linux x64 (use opensysml-linux-arm64.tar.gz on arm64)
wget https://github.com/Open-MBEE/OpenSysML/releases/latest/download/opensysml-linux-amd64.tar.gz
tar xzf opensysml-linux-amd64.tar.gz && sudo mv sysml sysml-lsp /usr/local/bin/

# macOS (Intel or Apple Silicon) — see the note below
brew install Open-MBEE/tap/opensysml
```

**With a Go toolchain (no download, never quarantined):**
```bash
go install github.com/Open-MBEE/OpenSysML/cmd/sysml@latest
go install github.com/Open-MBEE/OpenSysML/cmd/sysml-lsp@latest
```

**Or build from source:**
```bash
make build
./bin/sysml
```

> **macOS — use Homebrew.** The released binaries are not Developer ID signed or notarized,
> so a tarball downloaded *in a browser* carries `com.apple.quarantine` and Gatekeeper shows
> "cannot be opened because the developer cannot be verified". Homebrew downloads with
> `curl`, which never sets that attribute, so `brew install` avoids the prompt entirely.
> When the tarball is downloaded directly (`curl -fL ... opensysml-darwin-arm64.tar.gz`, followed
> by `xattr -d com.apple.quarantine`), see
> [the guide](docs/guide/01-install.md#macos-gatekeeper). Signing and notarization are the
> intended long-term resolution —
> [docs/project/macos-distribution.md](docs/project/macos-distribution.md).
>
> Install by the **fully-qualified** name. Homebrew 6 requires third-party taps to be trusted
> before their Ruby is loaded, and `brew install Open-MBEE/tap/opensysml` trusts just that
> formula. `brew tap Open-MBEE/tap && brew install opensysml` needs
> `brew trust --formula Open-MBEE/tap/opensysml` in between.

### Examples

**Interactive modeling:**
```bash
$ sysml
sysml> part def Wheel { attribute diameter = 16.0; }
✓ part def Wheel

sysml> %instantiate Wheel
✓ Created instance of Wheel
  ID: 1
  Use %features Wheel to inspect

sysml> %features Wheel
Instance: Wheel (ID: 1)
Features:
  diameter = 16.0
  ownedPorts = []
  performedActions = []
  ownedActions = []
  exhibitedStates = []
  ownedStates = []
  shape = []
  envelopingShapes = []
  boundingShapes = []
  voids = []
  isSolid = true
  subitems = []
  subparts = []
  checkedConstraints = []
```

**Behavioral execution:**
```bash
sysml> calc add { in x; in y; x + y }
✓ calc add

sysml> %calc add 10 20
✓ add(10, 20)
  = 30

sysml> constraint ValidSpeed { 65 <= 120 }
✓ constraint ValidSpeed

sysml> %constraint ValidSpeed
✓ Constraint ValidSpeed passed
```

**Action & state debugging:**
```bash
sysml> action MyWorkflow { attribute result = 0; first start; then action compute { assign result := 42; } then done; }
✓ action MyWorkflow

sysml> %action MyWorkflow
✓ Started action executor for "MyWorkflow"
  State: Running
  Tokens: 1

Use %step to advance, %tokens to inspect, %continue to run to completion

sysml> %break compute
✓ Breakpoint set at node "compute"
  %continue runs until a token reaches it

sysml> %continue
⏸ Paused at breakpoint "compute"
  State: Suspended
  Tokens: 1

Use %tokens to inspect, %step or %continue to resume

sysml> %tokens
Active tokens (1):
  Token 1 @ compute
  Values:
    result = 0

sysml> %continue
✓ Action completed
  Final state: Completed
  Results:
    result = 42

sysml> state TrafficLight { entry; then red; state red; state green; transition first red then green; }
✓ state TrafficLight

sysml> %state TrafficLight
✓ Started state machine executor for "TrafficLight"
  Current state: red
  Time: 0.0
  Events: 1

Use %events to see queue, %current for state, %advance <time> to step

sysml> %advance 30
✓ Advanced to 30.0 (1 event(s) processed)
  Current state: green
  Last event at: 0.0
  Remaining events: 0
  Action state: Completed
  Tokens: 0
  The action runs in a context of its own, whose clock advanced from 0.0 to 30.0

✓ Action completed
  Results:
    result = 42
```

**Further demonstrations are available in
[examples/repl-behavioral-demo.sysml](examples/repl-behavioral-demo.sysml).**

---

## Overview

The project provides the tooling familiar from the Python, Rust and Go ecosystems, applied to
SysML v2:

- **Language Server** — A standard LSP server (`sysml-lsp`) with live diagnostics, semantic hover, go-to-definition, find references, completion, workspace-wide symbol search, formatting, rename, semantic tokens and quick fixes. A VS Code extension with TextMate grammars for `.sysml` and `.kerml` ships in [editors/vscode](editors/vscode), and any editor with a generic LSP client can drive the server directly — [guide chapter 8](docs/guide/08-editors.md) walks through both. *Not yet:* the extension is built from source rather than published to a marketplace, and the server answers no semantic token delta requests or signature help.
- **Interactive REPL** — An exploratory modeling environment: define models incrementally, evaluate expressions interactively, instantiate parts, run calculations and inspect runtime state, comparable to IPython or Jupyter for systems engineering.
- **Constraint Solving** *(experimental)* — In addition to evaluating what holds of an object, an external SMT solver determines whether a constraint, requirement or satisfaction assertion *can* hold, which conditions conflict when it cannot, which values would satisfy it, which variants a model permits, and what optimizes an `analysis def`'s objectives. The solver is optional and discovered at runtime. [The REPL command reference](docs/reference/repl-commands.md) documents each command, and [installing a solver](docs/guide/01-install.md#installing-a-solver-optional) describes how to obtain one. The design follows OpenMBEE's [HMF](https://github.com/hivecore-dev/hmf) (see [Acknowledgements](#acknowledgements)).
- **Execution Runtime** — More than a validator: instantiate parts, evaluate constraints against concrete values and execute calc and analysis cases. Action and state executor infrastructure is complete (activity fork/join parallelism, decision guards, hierarchical/orthogonal states, choice/junction pseudostates, TimeEvent/ChangeEvent/AcceptEvent, sourceless transitions). See [spec compliance](docs/project/spec-compliance.md) for measured behavioral coverage.
- **Embeddable Go API** — `client/opensysml` is the public Go surface: parse, look up symbols, evaluate expressions and instantiate parts from Go code, answered in process by the engine the calling binary already links (no port, no child process and no serialization round trip), or over the Connect protocol against an externally hosted service. See [client/opensysml/README.md](client/opensysml/README.md).
- **Python Client Library** — gRPC-based Python bindings for programmatic access: parse models, resolve symbols, evaluate expressions, instantiate parts, execute actions/state machines. Includes IPython display hooks for Jupyter notebooks and pandas DataFrame integration. Constraint, requirement, satisfaction and calc verdicts are available as RPCs (`verify_constraint`, `verify_requirement`, `verify_satisfaction`, `calc`).
- **Node/TypeScript Client Library** — `@opensysml/client` for Node and the browser, over the Connect protocol with protobuf bodies: parse, evaluate, look up symbols and instantiate, with values as discriminated unions. No native addon and nothing downloaded at install time ([clients/node/README.md](clients/node/README.md)).
- **Java Client Library** — `org.openmbee:opensysml-client` for a JVM host application it does not own, on the JDK's own `java.net.http.HttpClient`, so no gRPC, Netty or `tcnative` reaches the host ([clients/java/README.md](clients/java/README.md)).
- **Rust Client Library** — A blocking client for the local `sysml-grpc` service, with no asynchronous runtime in its default dependency tree, available from the [Rust crate documentation](clients/rust/README.md).

Guidance on selecting a client, the coverage of the four newer clients, and the functionality they intentionally defer to a future version is provided in [docs/reference/clients.md](docs/reference/clients.md).
- **Modern Toolchain** — Incremental compilation, a bundled standard library and persistent semantic caches. A model is a set of files, named on the command line or opened by the editor.

## Goals

- **Performance:** sub-millisecond parsing, a single static binary, and no JVM or Eclipse runtime
- **Completeness:** SysML v2 textual notation support (98 of 98 standard library files parse cleanly: 94 vendored OMG files and 4 OpenSysML extensions)
- **Executable models:** instantiate, evaluate and simulate, turning specifications into running systems
- **Practical ergonomics:** multi-file workspaces, incremental analysis and detailed diagnostics

## Status

The project is under active development, with the core infrastructure operational:

| Component | Status |
|-----------|--------|
| Lexer/Parser (structural + behavioral grammar) | ✅ Operational (98/98 stdlib clean - see [conformance gate](internal/core/libs/stdlib_conformance_test.go)) |
| Symbol resolution & type system | ✅ Complete |
| Semantic layer (operators, builtins, validation) | ✅ Complete |
| Feature chain resolution (member access) | ✅ Complete |
| Validation passes (typing conformance, redefinition) | ✅ Complete |
| Native document generation | ✅ Document queries (`%run-query`, `-run-query`), document definitions with sections, paragraphs, inline runs, tables (including grouped, with computed columns), lists and embedded diagrams, rendered to Markdown (`-render-document`, `%render-document`), semantic HTML with an overridable stylesheet (`-doc-form html`), a linked multi-document set (`-render-documents`) with cross-document references, or PDF (`-doc-form pdf`) — see [the document generation manual](docs/manual/README.md) |
| Expression evaluator & instance model (runtime Tiers 1-3) | ✅ Complete |
| Runtime operators (equality, logical, negation) | ✅ Complete |
| Workspace/reindex/file watching | ✅ Complete |
| Behavioral parser (unified grammar with graceful fallback) | ✅ Complete (195 golden ASTs, 249 negative tests) |
| Calc invocation, constraint & requirement evaluation | ✅ Complete (conformance gate: 185 calc/constraint/requirement/satisfy cases passing) |
| Action execution engine (Tier 5) | ✅ Complete (129 conformance cases passing) |
| State machine runtime (Tier 5) | ✅ Complete (104 conformance cases: transitions, accept events, sourceless) |
| REPL debugging commands | ✅ Complete — `%constraint`, `%requirement`, `%satisfy` and `%calc` also answer from the command line (`-constraint`, `-requirement`, `-satisfy`, `-calc`) and over gRPC, on one evaluation |
| Model save to notation (`%save model.sysml`, `sysml -convert sysml`) | ✅ Complete — writes the source through the formatter, so comments and spacing survive |
| SysML ↔ RDF Turtle conversion (`%save model.ttl`, `sysml -convert ttl`) | 🧪 **Experimental** — packages, definitions, usages, ports, connections, values, documentation, and the nodes an action or state body states (every one of the 346 models under `examples/` converts and round-trips; what is not mapped is refused with the construct named), but the vocabulary may change without a compatibility path. Every run says so; see [the RDF mapping's status](docs/reference/rdf-mapping.md#status-experimental) and [worked example](examples/rdf-interop-demo.sysml) |
| SysML v1 → v2 migration (`sysml Model.xmi -convert sysml`, `.mdzip` too) | 🧪 **Experimental** — Cameo/MagicDraw XMI is read and written as v2 notation or RDF: packages, blocks, value types, properties, ports and connectors, requirements with satisfy/verify/derive, constraint blocks, instances and allocations, with a per-element migration report (mapped, approximated, unmapped, skipped). Behaviors, operations and units are not migrated yet, and the mapping may change without a compatibility path. Every run says so; see [the migration's status](docs/reference/sysml-v1-migration.md#status-experimental) |
| View rendering (`%render <view>`, `sysml -render`) | ✅ Complete for the kinds produced — containment tree, interconnection diagram, state machine, action flow, sequence diagram and table, as indented text or in the kind's machine-readable form (Mermaid, Markdown). State and action renderings read the graph the runtime executes; the notation itself is tool-defined ([SysML v2 §10.2](docs/project/spec-compliance.md)) |
| Constraint solving (`%check`, `%explain`, `%solve`, `%configure`, `%optimize`) | 🧪 **Experimental** — an external SMT-LIB 2 solver decides whether conditions *can* be satisfied, explains an `unsat` with a minimal unsat core, synthesises satisfying values, enumerates the variant selections a model permits and optimizes an `analysis def`'s objectives (optimization needs z3, which implements it). The solver is optional and discovered on `PATH` or through `OPENSYSML_SMT`; a build with none reports that rather than a verdict — see [installing a solver](docs/guide/01-install.md#installing-a-solver-optional) |
| Source-preserving model edits (`ApplyEdits`, `model.edit()`) | ✅ Complete for four operations — set a feature's value, rename a declaration, add a member, and delete a declaration — rewriting the bytes of the model's own source so every untouched byte is identical. A rename rewrites the references to the renamed element too, and a non-cascade deletion of a referenced element is refused rather than approximated |
| Standard library bundling | ✅ Complete |
| LSP server implementation | ✅ Diagnostics, hover, go-to-definition, references, symbols, completion, formatting, rename, semantic tokens (full + range), code actions (quick fixes) — semantic token deltas and signature help not implemented |
| gRPC service layer | ✅ Complete (parse, symbols, diagnostics, runtime, verification, conversion, edit and Query RPCs), served as gRPC, gRPC-Web and the Connect protocol on one port |
| Public Go API (`client/opensysml`) | ✅ Complete for its v1 scope: parse, diagnostics, symbols, evaluation, instantiation and capability negotiation, answered in process or over Connect, with the edit API, conversion, verification, behaviour execution and Query out of scope ([client/opensysml/README.md](client/opensysml/README.md)) |
| Python client library | ✅ Complete for the RPCs that exist (connection lifecycle, parse/symbols/eval/instantiate/execute, constraint/requirement/satisfaction/calc verification, conversion, edits, Query, IPython hooks, DataFrame) |
| Rust client library | 🚧 Blocking v1 client for parse, diagnostics, symbols, evaluation and instantiation; see the [Rust client README](clients/rust/README.md) |
| Java client library | ✅ Complete for its v1 scope, with the remaining scope stated explicitly: connection lifecycle, parse/symbols/eval/instantiate and capability negotiation, with the edit API, conversion, verification, behaviour execution and Query out of scope. Connect protocol over the JDK's own HTTP client, so no gRPC or Netty reaches a host application ([clients/java/README.md](clients/java/README.md)) |
| Node/TypeScript client library | ✅ Complete for the same v1 scope, in Node and the browser, over the Connect protocol with protobuf bodies and no native addon; values arrive as discriminated unions ([clients/node/README.md](clients/node/README.md)) |

<!-- doc-counts:begin refereed-figures -->
**Measured against the pinned reference** (`PILOT_TAG=2026-07`, artifact `0.61.0`). Every number below is generated by `make docs-counts` from the committed baselines and gated; none of them is typed in by hand.

- **Corpus agreement:** 338 of 368 files agree diagnostic-by-diagnostic; 21 diagnostics are ours alone and 596 the reference's alone, and the first number must be read by root: our diagnostics against the reference's own corpora fell while our non-standard-notation warnings on our own example models rose ([differential](docs/project/pilot-differential.md), `go run ./cmd/pilot-diff`).
- **Declared-diagnostic silence:** of the 511 declared `errors` rows in the reference's own Xpect suites, we report nothing for 0. 244 we report word-for-word; 248 wording-only and 7 location-only differences are agreement in substance and are not counted as gaps; 0 more we report as a warning and 2 elsewhere in the file ([Xpect oracle](docs/project/pilot-xpect.md), `go run ./cmd/pilot-xpect`).
- **Scope agreement:** 230 of 230 declared scope assertions match exactly (same source).
- **Permissiveness gaps:** of 285 invalid models we wrote ourselves, the reference rejects 3 that we accept by default, and 273 both reject; 3 further cases agree only when we are asked strictly. We authored every one of these cases ourselves, so the denominator measures the reach of our own corpus and not our conformance; agreement reached only under an opt-in strict mode is weaker evidence than agreement by default ([rejection oracle](docs/project/pilot-rejection.md), `go run ./cmd/pilot-reject`).
- **Declared errata:** the registry declares 3 defect(s) in the published reference material — 1 with a specification-derived correction, 2 documented without one, since no intended reading can be inferred ([OMG issues](docs/project/omg-issues.md), `internal/errata`). Every figure above is as published and stays the conformance statement; running the same oracles over the corrected text instead reports 339 of 368 files agreeing, 20 diagnostics ours alone and 596 the reference's alone, 0 declared rows we are silent on, and 0 of 285 authored cases the reference alone rejects. The corrected figures are diagnostic only: an erratum never reclassifies a divergence category, and the published corpus is never edited.
- **Self-assessed surface:** the action, state-machine and classifier-behavior rows have no external referee at all — the four refereed figures above cannot see them, because the pinned artifact evaluates expressions but executes neither actions nor state machines. [Spec compliance](docs/project/spec-compliance.md) counts them.

What these numbers cannot show: the OMG corpora are demonstrations rather than an official conformance suite; the differential is one-directional, comparing the diagnostics the two implementations report on the same files; the Xpect suites are the pilot authors' test intent rather than a certification oracle; and none of these is a percentage of the specification — no global compliance figure is claimed anywhere.

**Row bookkeeping:** the ✅/⚠️/❌/⛔ status of each tracked rule stays in [spec compliance](docs/project/spec-compliance.md) as a census of our own row list, counted when the documentation site is built rather than committed. It moves when rows are rewritten and does not move when an oracle does, so it is not the progress measure.
<!-- doc-counts:end refereed-figures -->

**Current commit:** All tests pass (`go test -race ./...`), builds clean (`go build ./...`).
**Test coverage:** 15,139 tests and subtests (15,122 pass, 17 skip — 3 skip themselves, 14 gate on a PDF toolchain, a pinned pilot artifact, a locale, a case-insensitive filesystem or a live Flexo stack; 6,570 top-level `Test` functions; counted with the OMG corpora downloaded and an SMT solver installed, without which 80 more skip) covering parsers, semantics, runtime (actions, states, instances, operators, validation). Behavioral robustness: 195 golden ASTs, 249 negatives, 671 conformance cases, 140 golden traces, 336 runtime robustness cases, 15 gRPC conformance cases and 8 gRPC robustness cases.
**Parser coverage:** 98/98 bundled library files parse cleanly — the 94 official SysML v2 standard library files and the non-normative `OpenSysML Libraries/OpenSysMLMathFunctions.kerml`, `OpenSysML Libraries/DocumentQueries.sysml`, `OpenSysML Libraries/IdentityMetadata.sysml` and `OpenSysML Libraries/OOSEM.sysml` extensions. Conformance verified by [stdlib_conformance_test.go](internal/core/libs/stdlib_conformance_test.go). Grammar reference: [OMG Xtext grammar](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/master/org.omg.kerml.xtext/src/org/omg/kerml/xtext).
**Behavioral execution:** Calc/constraint/requirement/satisfy functional. Action/state executors handle nested invocation, control flow keywords, loop and conditional statements and the send statement (671/671 conformance cases passing). Coverage is self-assessed against the specification text and the normative library: the pinned OMG pilot implementation evaluates expressions but does not execute actions or state machines headlessly, so no external implementation currently adjudicates these rows. See [spec compliance](docs/project/spec-compliance.md).
**Reference differential:** 368 files compared diagnostic-by-diagnostic against the pinned OMG pilot implementation (`2026-07`), 338 in full agreement; every divergence is enumerated and adjudicated in [the differential](docs/project/pilot-differential.md), reproducible with `go run ./cmd/pilot-diff`.
**Rejection oracle:** the reverse direction — do we reject what the reference rejects? 285 hand-written invalid models validated by both implementations, 276 rejected by both, 0 the pinned pilot rejects and we accept; the remainder only we reject — the control-node succession rules the pinned pilot leaves unimplemented and a non-Boolean succession guard it accepts once the standard library types it — and every permissiveness gap is enumerated with a reproducer and likely root cause in [the rejection oracle](docs/project/pilot-rejection.md), reproducible with `go run ./cmd/pilot-reject`. We wrote every case, so the count measures our coverage of the rejection surface, not our conformance — a sample, not a proof.
**Training examples:** 100/100 files clean, gated by `internal/core/model/testdata/training_examples_expected.txt`. Download with `./scripts/download-training-examples.sh` (from the [OMG training directory](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/master/sysml/src/training)). See [training examples](docs/project/training-examples.md) for analysis.
**Semantic layer:** a complete implementation of runtime operators, feature chains and validation rules. See [examples/semantic-layer/](examples/semantic-layer/) for a full demonstration.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Frontends: LSP Server │ Interactive REPL               │
├─────────────────────────────────────────────────────────┤
│  Workspace: Multi-file documents, incremental reindex   │
├─────────────────────────────────────────────────────────┤
│  Semantic Engine: Types, resolution, validation         │
├─────────────────────────────────────────────────────────┤
│  Execution Runtime: Expressions, instances, behaviors   │
├─────────────────────────────────────────────────────────┤
│  Parser/Lexer: Hand-written recursive descent           │
├─────────────────────────────────────────────────────────┤
│  AST: Syntax-only, immutable (semantics in side tables) │
└─────────────────────────────────────────────────────────┘
```

**Key design principles:**
- **Incremental and lazy:** parse immediately and resolve semantics on demand, following the precedent set by gopls and rust-analyzer
- **Immutable AST:** all semantic state resides in side tables keyed by node or symbol
- **Pluggable validation:** tiered passes (syntax → names → types → constraints)
- **Separated concerns:** the static analysis pipeline feeds the execution runtime

## Module structure

```
github.com/Open-MBEE/OpenSysML
├── cmd/
│   ├── sysml-lsp/          # LSP server binary
│   ├── sysml-grpc/         # gRPC server binary (Python bindings)
│   └── sysml/              # Interactive REPL binary
├── internal/core/
│   ├── source/             # Source files, spans, line indexing
│   ├── lexer/              # Hand-written scanner
│   ├── parser/             # Recursive-descent parser
│   ├── ast/                # Syntax tree nodes
│   ├── symbols/            # Symbol tables, scope trees
│   ├── resolve/            # Name resolution (lazy, memoized)
│   ├── semantics/          # Type system, conformance, multiplicity
│   ├── passes/             # Validation passes (syntax → constraints)
│   ├── lower/              # AST → execution IR (ActionGraph/StateGraph)
│   ├── runtime/            # Execution engine (eval, instances, builtins)
│   ├── model/              # Workspace, document management
│   └── libs/               # Standard library bundling & caching
├── internal/lsp/           # LSP protocol implementation
├── internal/grpc/          # gRPC service implementation
├── internal/repl/          # REPL loop implementation
├── client/opensysml/       # The public Go API (in-process and remote)
├── clients/java/           # Java client (org.openmbee:opensysml-client)
├── clients/node/           # Node/TypeScript client (@opensysml/client)
├── clients/python/         # Python client bindings (opensysml)
├── clients/rust/           # Rust client (opensysml) and its conformance runner
├── docs/                   # Design specs, architecture docs
└── testdata/               # Test fixtures (.sysml, .kerml)
```

## Technology

- **Language:** Go 1.25 or later (goroutines for concurrency, a single static binary, and an established record in language servers)
- **Parser:** hand-written recursive descent (no framework overhead, full error recovery, sub-millisecond parses)
- **Grammar source:** OMG pilot Xtext grammars (`SysML.xtext` and `KerMLExpressions`)
- **Spec compliance:** [OMG SysML v2.1 Beta 1 / KerML 1.1](https://www.omg.org/spec/SysML/2.0) (2026-07 release)
- **Standard library:** 94 files from [SysML v2 Pilot Implementation 2026-07](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/releases/tag/2026-07), byte-identical, plus the non-normative `OpenSysML Libraries/OpenSysMLMathFunctions.kerml` extension
- **CI/CD:** GitHub Actions checks pull requests; CircleCI builds and tests `main` and publishes releases from tags

## Releases

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases page](https://github.com/Open-MBEE/OpenSysML/releases).

**Supported platforms:**
- Linux (x64, ARM64)
- macOS (Intel, Apple Silicon)
- Windows (x64)

**Release process:**
- Every commit: build and test
- Tagged releases (`v*`): the suite runs again on the tagged commit, then multi-platform
  binaries are published to GitHub Releases. Maintainer procedure:
  [docs/project/releasing.md](docs/project/releasing.md); what changed per release:
  [CHANGELOG.md](CHANGELOG.md)
- The Python client is released on its own tag (`opensysml-v*`), which uploads `opensysml` to
  PyPI — its version is not coupled to the core's, since it resolves a `sysml-grpc` binary
  at runtime from whichever release the caller names
- The Java client is not yet published: consume it with `mvn -f clients/java/pom.xml install`. The
  prerequisites a maintainer must obtain for a first Maven Central upload are listed in
  [docs/project/releasing.md](docs/project/releasing.md)
- The Node client is released the same way on `client-node-v*`, which publishes
  `@opensysml/client` and the five per-platform packages that carry the service binary
- The Rust client is not yet published to crates.io: use a path or Git dependency, and see
  [clients/rust/README.md](clients/rust/README.md) and
  [docs/project/releasing.md](docs/project/releasing.md) for the requirements of a first publish
- `client/opensysml`, the public Go API, requires no release of its own. It is part of this
  module, so a Go program pins it with `go get github.com/Open-MBEE/OpenSysML@v0.3.0`

**Release artifacts:** per-binary archives (`sysml-<os>-<arch>.tar.gz`,
`sysml-lsp-<os>-<arch>.tar.gz`), `opensysml-<os>-<arch>.tar.gz` bundles containing both
binaries, and `SHA256SUMS.txt`. Windows also gets an installer,
`opensysml-<x.y.z>-windows-amd64.msi` (`sysml`, `sysml-lsp`, `sysml-grpc`, optional bundled Z3
solver; see [packaging/msi](packaging/msi/README.md)), built by the GitHub Actions release
workflow after CircleCI publishes the release, with its digest in `SHA256SUMS-windows-msi.txt`.
macOS binaries are not Developer ID signed or notarized; see
[docs/project/macos-distribution.md](docs/project/macos-distribution.md). Windows binaries and
the MSI are Authenticode signed through [SignPath Foundation](https://signpath.org) once the
project's application is approved; the signed files are published as separate `*-signed*`
assets beside the unsigned ones that `SHA256SUMS.txt` covers. See the
[Code signing policy](#code-signing-policy).

## Code signing policy

Free code signing provided by [SignPath.io](https://about.signpath.io), certificate by
[SignPath Foundation](https://signpath.org).

The Windows binaries (`sysml.exe`, `sysml-lsp.exe`, `sysml-grpc.exe`) of a tagged release are
built by the GitHub Actions workflow
[`.github/workflows/release-windows.yml`](.github/workflows/release-windows.yml) from the
tagged commit of this repository and submitted to SignPath for signing from that workflow, so
every signed file traces back to a public commit and a public build log. The signed files are
published on the GitHub release as `sysml-windows-amd64-signed.zip`,
`sysml-lsp-windows-amd64-signed.zip`, `sysml-grpc-windows-amd64-signed.exe`,
`opensysml-windows-amd64-signed.zip` and the installer `opensysml-<x.y.z>-windows-amd64-signed.msi`
(built from the signed executables and itself signed), with their digests in
`SHA256SUMS-windows-signed.txt`. Every signed executable carries `ProductName` `OpenSysML` and
the release tag as `ProductVersion`/`FileVersion`. The Z3 solver the MSI optionally bundles is
upstream open-source software and is never signed with the Foundation certificate; the
installer carries it unsigned, as the SignPath Foundation terms allow. Signing is not in effect
until the project's SignPath application is approved; releases made before that carry only
unsigned Windows assets (including the unsigned MSI).

**Team roles**

- **Authors** may commit to this repository without additional review:
  [`@Open-MBEE/opensysml-authors`](https://github.com/orgs/Open-MBEE/teams/opensysml-authors).
- **Reviewers** review all pull requests from non-committers before they are merged:
  [`@Open-MBEE/opensysml-reviewers`](https://github.com/orgs/Open-MBEE/teams/opensysml-reviewers).
- **Approvers** approve each signing request in SignPath before a certificate is applied; a
  release ships no signed Windows binaries without that manual approval:
  [`@Open-MBEE/opensysml-approvers`](https://github.com/orgs/Open-MBEE/teams/opensysml-approvers)
  and the [owners of the Open-MBEE organization](https://github.com/orgs/Open-MBEE/people?query=role%3Aowner).

All Authors, Reviewers and Approvers must have multi-factor authentication enabled on their
GitHub accounts and on their SignPath accounts.

**Privacy policy**

This program will not transfer any information to other networked systems unless specifically
requested by the user or the person installing or operating it.

In particular, none of `sysml`, `sysml-lsp` and `sysml-grpc` checks for updates, collects
telemetry or downloads anything on its own. The only network activity any of them performs is
what the operator asks for by name: `sysml-lsp` speaks only over its standard input and output,
`sysml-grpc` serves the address it is started with and answers only the clients that connect to
it, and `sysml -sync-diff` / `sysml -sync-apply` contact a SysML v2 API / Flexo MMS repository
only when the operator names that endpoint's `http://` or `https://` URL on the command line.
The OMG pilot corpora and reference tools used by the test suite are fetched by developer
scripts under `scripts/`, which are not part of the shipped binaries.

## Building

```bash
# Build all binaries
go build ./...

# Run tests
go test ./...

# Build LSP server
go build -o bin/sysml-lsp ./cmd/sysml-lsp

# Build REPL
go build -o bin/sysml ./cmd/sysml

# Build gRPC service
go build -o bin/sysml-grpc ./cmd/sysml-grpc
```

## Test and conformance reports

Both cross-implementation harnesses write their results in machine-readable form as well as prose,
so a CI system can render them without parsing text:

```bash
make conformance      # bin/conformance-report.json + bin/conformance-report.xml
go run ./cmd/conformance -report - -junit bin/conformance-report.xml
go run ./cmd/pilot-diff   # build/pilot-diff/pilot-diff.{txt,json,xml,sarif}
```

- **JSON** stays the source of truth for both: aggregate totals, and per scenario or per file every
  outcome, status, duration and mismatch. The other renderings are derived from the same run.
- **JUnit XML** — `bin/conformance-report.xml` holds one suite per configuration and protocol and
  one case per scenario, so CI shows per-scenario results in its own test report;
  `build/pilot-diff/pilot-diff.xml` holds one suite per corpus root and one case per file that drew
  a diagnostic. CI stores the conformance XML as test results on every run, pass or fail.
- **SARIF 2.1.0** — `build/pilot-diff/pilot-diff.sarif` carries one result per disagreeing
  diagnostic group, located on the compared model file, for a code-scanning viewer.

The suites themselves are described in [conformance/README.md](conformance/README.md) and
[the pilot differential](docs/project/pilot-differential.md); nothing in the differential gates CI.

## Clients

Five surfaces reach the same engine: the Go API, used in the calling process, and four clients of
the `sysml-grpc` service. [Client libraries](docs/reference/clients.md) states what each covers and
how to choose; [guide chapter 9](docs/guide/09-clients.md) works through each one.

| Surface | Reaches the engine by | Published | API reference |
|---|---|---|---|
| Go, `client/opensysml` | in process, or Connect to a service | with the core (`v*` tags) | [Go packages](docs/reference/api.md) |
| Python, `opensysml` | gRPC, to a private child service or a named one | PyPI, on `opensysml-v*` tags | [Python API](docs/reference/python-api.md) |
| Node/TypeScript, `@opensysml/client` | Connect, from Node or a browser page | not yet | [Node API](docs/reference/node-api.md) |
| Java, `org.openmbee:opensysml-client` | Connect, over the JDK's own HTTP client | not yet | [Java API](docs/reference/java-api.md) |
| Rust, `opensysml` | Connect, blocking, no async runtime | not yet | [Rust API](docs/reference/rust-api.md) |

The Go and Python clients cover every RPC the service serves; Node, Java and Rust cover a v1
subset — connection lifecycle, capability negotiation, parsing, diagnostics, symbol lookup,
evaluation and instantiation — enumerated in the client libraries page.

### Python

**opensysml** is a Python client library providing programmatic access to OpenSysML's parsing and runtime capabilities over gRPC.

**Installation:**
```bash
pip install opensysml          # from PyPI

# Or from a checkout, in development mode
pip install -e clients/python/
```

**Quick example:**
```python
import opensysml

# Load and parse a SysML model
model = opensysml.load("vehicle.sysml")

# Evaluate expressions
result = model.eval("2 + 2")
print(result)  # 4

# Instantiate parts
instance = opensysml.instantiate("Vehicle", model_hash=model.hash)
print(instance.slots["mass"])
```

**Features:**
- Jupyter notebook integration with rich HTML displays
- pandas DataFrame integration for model analysis
- automatic service lifecycle management
- full runtime API access (evaluation, instantiation, action and state execution)

Detailed installation and usage instructions are in
[clients/python/INSTALL.md](clients/python/INSTALL.md), and the API in
[docs/reference/python-api.md](docs/reference/python-api.md).

### Node/TypeScript

**@opensysml/client** provides the same access over the Connect protocol, for Node and the
browser. It includes no native addon: installation is a standard registry fetch, and the service
binary is supplied by a per-platform optional dependency.

```ts
import { loads } from "@opensysml/client";

await using model = await loads("part def Wheel { attribute radius : ScalarValues::Real = 0.3; }");
const radius = await model.eval("0.3 * 2");
```

Version 1 covers loading, evaluation, symbol lookup and instantiation, and negotiates against the
capabilities the service advertises. It is not yet published. See the
[Node API](docs/reference/node-api.md) and [clients/node/README.md](clients/node/README.md) for the
two lifecycle modes (a private child of the calling process, or an externally hosted service), the
capabilities and limitations of the browser entry point, and the functionality version 1 omits.

### Go, Java and Rust

```go
client, err := opensysml.New()   // in process; opensysml.Dial(addr) for a running service
defer client.Close()
model, err := client.ParseFile(ctx, "vehicle.sysml")
mass, err := client.Evaluate(ctx, model, "mass", opensysml.WithSubject("Demo::sedan"))
```

The Go API is documented type by type in [Go packages](docs/reference/api.md) and
[client/opensysml/README.md](client/opensysml/README.md). The Java client is a `try`-with-resources
`Connection` over the JDK's HTTP client ([Java API](docs/reference/java-api.md),
[clients/java/README.md](clients/java/README.md)); the Rust client is blocking, with no async
runtime in its default dependency tree ([Rust API](docs/reference/rust-api.md),
[clients/rust/README.md](clients/rust/README.md)). Neither is published yet.

## Documentation

- **[The guide](docs/guide/)** — install, first model, CLI, REPL, checks, behavior, saving, editors, and [driving it from your own program](docs/guide/09-clients.md)
- **[Client libraries](docs/reference/clients.md)** — the Go, Python, Node, Java and Rust surfaces, and how to choose between them
- **[Reference](docs/reference/)** — CLI flags, REPL commands, environment, each client's API, service transports, RDF mapping
- **[Internals](docs/internals/architecture.md)** — the pipeline, the tiers, testing and performance
- **[Project status](docs/project/spec-compliance.md)** — spec compliance, roadmap and releasing
- **[Examples](examples/)** — runtime demonstrations and behavioral model examples

A complete index is available in [docs/README.md](docs/README.md).

## License

Apache 2.0

## Acknowledgements

The constraint-solving capability set — satisfiability checking, conflict explanation
through unsat cores, value synthesis and objective optimization through an SMT solver —
follows the design of the `ConstraintSolverService` in OpenMBEE's
[HMF (Hivecore Model Framework)](https://github.com/hivecore-dev/hmf) (Apache 2.0). The
implementation in `internal/core/solve` is independent: it translates conditions to
SMT-LIB 2 for an external `z3`/`cvc5` process rather than binding to Z3 in-process.

## Contributing

The project is under active development. See [CONTRIBUTING.md](CONTRIBUTING.md) for build, test and contribution guidelines.

## Contact

[Contact information to be added]
