# Surface parity: the REPL, the editor, the public package and the wire

*A proposal. Nothing here is implemented; the note exists to be reviewed before code is
written.*

The repository ships one engine behind four surfaces, and they do not offer the same operations.
`sysml` (the CLI and REPL) and `sysml-lsp` call `internal/core/*` directly. `sysml-grpc` hosts
`internal/grpc.Service` over gRPC, Connect and stdio. The public Go package `client/opensysml`
wraps that same `Service` in-process (`New`) or dials it (`Dial`), so the package and the wire
already agree with each other by construction. The REPL and the editor agree with neither, except
where a test pins them to (`internal/grpc/oslc_query_repl_test.go` holds `Query` to the REPL's
`%query`; `internal/lsp/library_invocation_test.go` holds the editor's diagnostics to the REPL's
evaluation).

This note inventories the mismatch, sorts every difference into one of five classes — shared
already, missing and worth adding, interactive, protocol-bound, local-only — and stages the work
that closes the differences worth closing. The goal is not one protocol for everything. It is that
a capability the engine has is reachable from the package and the wire *when it makes sense
outside an interactive session*, that the REPL and the editor compute what they show through the
same code the service does, and that a conformance scenario proves the agreement.

## 1. Why the binaries do not go through the service

Three reasons, and the roadmap keeps all three.

**Same process.** A Go binary that imports the engine has it linked. Calling `Service` in-process
costs a method call; calling it over a port costs a serialization round trip. The
[transport evaluation](transport-evaluation.md) §5 measured that trip at 0.1–1 ms p50 for a small
call on any of the three protocols — noise for a coarse client, a tax for a REPL loop issuing
thousands of fine-grained calls or an editor answering at keystroke latency — and at 6–7 ms
(protobuf) against 38–40 ms (JSON) for a 468 KB `Query` answer. Nothing in this note routes
`sysml` or `sysml-lsp` through a socket.

**Precision.** The wire flattens a value into a `Value` message and a symbol into a
`SymbolInfo`. The REPL prints an instance from `runtime.Instance`; the editor reads `ast` nodes
and side tables to place a diagnostic at a range or offer a completion. Forcing them through the
proto shapes would be the lossy conversion AGENTS.md forbids.

**Statefulness.** A REPL session accumulates declarations, supersedes earlier ones by name,
carries a debugger across unrelated submissions and ends it only when the behavior it steps is
gone (`dropStaleDebugSessions` in `internal/repl/session.go`). The editor holds open documents
and edits them incrementally. `Service` is a cache of immutable models keyed by hash. These are
different lifecycles, and the mismatch they cause is intentional.

## 2. Inventory

Every operation the REPL, CLI and editor expose, against what the package and the wire expose
today. "Same code" means the surfaces reach one `internal/core` entry point; "own code" means the
surface reimplements the assembly around the core call.

### 2.1 Shared already

| Operation | REPL / CLI | Editor | `Service` RPC | Go package | Same code? |
|---|---|---|---|---|---|
| Parse, resolve, diagnose | `%load`, `-validate`, submit | `didOpen`/`didChange` → diagnostics | `ParseFile`, `ParseSources`, `GetDiagnostics` | `ParseFile`, `ParseSource(s)`, `ParseDocuments`, `Diagnostics` | yes — `model`, `passes` |
| Symbol lookup | `%print`, `%features` | hover, definition, symbols | `GetSymbol` | `LookupSymbol` | yes — `symbols`, `resolve`; the REPL's qualified-name suggestions (`qualsuggest.go`) are its own |
| Expression evaluation | `%eval`, `-eval`, `-e` | — | `Evaluate` | `Evaluate` | yes — `runtime` |
| Instantiate | `%instantiate`, `-instantiate` | — | `Instantiate` | `Instantiate` | yes; the REPL's instance graph is drawn through `grpc.InstanceGraphToProtoWithin` (`internal/repl/features.go`) |
| Run an action / state machine to completion | `%action`, `%state` without stepping; `-action`, `-state`, `-schedule`, `-seed` | — | `ExecuteAction`, `ExecuteState` with `schedule` | `ExecuteAction`, `ExecuteState`, `ExploreAction`, `ExploreState` | yes — `lower`, `runtime`, `analysis` |
| Calc evaluation | `%calc`, `-calc` | — | `EvaluateCalc` | `EvaluateCalc`, `Calculate` | yes |
| Constraint / requirement / satisfy | `%constraint`, `%requirement`, `%satisfy`, and the flags | — | `VerifyConstraint`, `VerifyRequirement`, `VerifySatisfaction` | `Verify*` | yes — `runtime` verdicts |
| Analysis case, trade study, sweep | `%analysis`, `%sweep`, `%samples` | — | `RunAnalysis`, `RunSweep` | `RunAnalysis`, `ExploreAnalysis` | yes — `analysis` questions `Evaluate`, `Outcomes`, `Sweep` |
| Engine listing and selection | `%engines`, `%engine`, `-engines`, `-engine` | — | `ListEngines`, the `engine` field | `ListEngines`, the option | yes — `engines.Default` |
| Model query | `%query`, `-query` (OSLC); `%run-query`, `-run-query` (document query) | — | `Query`, `RunDocumentQuery` | `Query`, `QueryOSLC`, `RunDocumentQuery` | yes — `query`, `queryplan`, `queryexec` |
| Document rendering | `%render-document`, `-render-document(s)`, `-doc-form` | `opensysml/renderDocument` | `RenderDocument` | `RenderDocument` | yes — `docir`, `docplan`, `docrender` |
| Convert (notation, RDF, element JSON) | `%save` (`.ttl`), `-convert`, `-from`, `-to` | — | `Convert` | `Convert`, `ConvertFile`, `ConvertSource` | yes — `export` |
| Model edits | — | `opensysml/applyModelEdit`, rename, code actions | `ApplyEdits` | `ApplyEdits` | yes — `edit`; the editor's rename and quick fixes are their own assembly over it |
| Strict conformance | `%strict`, `-strict` | editor setting | `strict_conformance` request field | option | yes — `conformance` |
| Server / build identity | `-version` | `initialize` result | `GetServerInfo` | `ServerInfo` | yes |

The shared rows are where parity holds today, and the risk in them is drift in the *assembly*:
the REPL's `%constraint` and the service's `VerifyConstraint` each build the invocation, choose
the engine, run, and shape a verdict, in two files. A fix to one has to be repeated in the other,
and the only thing that catches an omission is a test somebody thought to write across the two.
Stage 1 below takes that duplication out.

### 2.2 Missing from the package and the wire, and worth adding

Operations with a stateless meaning — a model in, an answer out — that only the REPL or the CLI
can perform today.

| Operation | Where it lives | What the wire lacks | Stage |
|---|---|---|---|
| Satisfiability: `%check`, `%solve`, `%explain`, `%optimize`, `%configure` | `internal/repl/check.go`, `explain.go`, `optimize.go` over `internal/core/solve`; the `solve` engine answers `analysis.Satisfiable` | No RPC asks a `Satisfiable` question. `ListEngines` advertises `solve`, but `RunAnalysis`, `Verify*` and the rest only pose `Evaluate`, `Outcomes`, `Holds`, `Sensitive` and `Sweep`. A remote caller can see the engine and cannot use it. (The `smt` engine, which proves `Holds`, *is* reachable: `VerifyConstraint` with `engine: smt`.) | 2 |
| Model-checker configuration: `%check-diverge`, `%check-property`, `%check-input`, `%check-assume`, `%check-witness`, `%check-bounds`, and the `-check-*` flags | REPL session state, read when a check runs | No fields. The `check` engine runs on the wire with defaults only; a caller cannot name the properties, assumptions, bounds or witness directory | 2 |
| Replay a recorded schedule: `%replay`, `-schedule replay:` | REPL and CLI | `schedule` is a string field, so `replay:<path>` would name a server-side path. The wire needs the witness *inline* | 2 |
| View rendering (diagrams): `%view`, `%render`, `-render`, `-render-all`, `-render-form`, `-render-palette` | `internal/core/view`; the editor's `opensysml/render`, `opensysml/views`, `opensysml/renderChanged` | No RPC. `RenderDocument` renders documents, not views. The editor's diagram protocol is the most complete client of `view` and is reachable only from an LSP client | 2 |
| Library search: `%search`, `%builtins` | `internal/core/suggest`, `libs` | No RPC. `GetSymbol` needs a name; there is no way to ask "what declares something like this" | 2 |
| XMI migration report: `-convert -from xmi -migration-report` | `cmd/sysml` over `export` | `Convert` from XMI answers with the model; the element-by-element report of what the conversion kept, renamed and dropped is written only to a local file | 3 |
| Code generation: `-compile`, `-target` | `internal/core/codegen` | No RPC, no package method. The output is a file tree, which fits `Convert`'s shape (a format, bytes out) once the format list is open | 3 |
| Conformance runs: `internal/repl/conformance.go` | REPL | Not an API; the REPL exposes it for the harness | — |

### 2.3 Interactive: a session, not a call

| Operation | Why it is not an RPC as the wire stands |
|---|---|
| Action debugger: `%step`, `%continue`, `%tokens`, `%break`, `%stop` | Steps an executor that lives between calls; breakpoints are set on it; the trace it prints is incremental |
| State debugger: `%send`, `%events`, `%current`, `%advance` | Same, with an event queue and a clock |
| Incremental declaration: submit, `%list`, `%clear`, `%print` of the session model | A session supersedes declarations by name and preserves object identity across the rebuilt runtime context (`keepIdentitiesOf`) |
| Runtime population: `%instances`, `%invoke` | Objects materialized by earlier calls, addressed by identity |
| Settings: `%verbosity`, `%trace`, `%schedule`, `%budget`, `%jobs`, `%engine` | Defaults for the calls that follow; `%trace` in particular is a stream, not a result |

`ExecuteAction` and `ExecuteState` already return the *result* of a run, and under `explore` the
set of outcomes. What they cannot do is stop in the middle. Stage 4 designs a session API for that,
separately, because the questions it raises — who owns the executor, what ends it, how a
breakpoint is addressed, how a trace streams — have no analogue in the stateless calls and must
not leak into them.

### 2.4 Protocol-bound: the editor

`textDocument/completion`, `hover`, `definition`, `references`, `rename`, `documentSymbol`,
`codeAction`, `rangeFormatting`, `semanticTokens/full` and `/range`, and `publishDiagnostics` are
the Language Server Protocol. Their inputs are documents and positions; their outputs are
protocol shapes. The service should not grow a `Complete(position)` RPC: a non-editor client has
no cursor, and an editor already has the protocol.

What the editor *computes* is another matter. Hover shows a symbol's type and documentation;
`GetSymbol` returns the same facts. Completion ranks candidates from `symbols` and `suggest`;
`%search` would use the same ranking. Code actions apply `internal/core/edit` operations;
`ApplyEdits` applies the same. Stage 1 gives these one implementation each; the editor's
`internal/lsp` files then translate positions in and protocol shapes out, and nothing else.

The editor's custom methods (`opensysml/render`, `opensysml/views`, `opensysml/renderChanged`,
`opensysml/renderDocument`, `opensysml/applyModelEdit`, `opensysml/documents`,
`opensysml/stdlibContent`) are not protocol; they are this project's diagram and authoring
surface, delivered over the LSP connection because that is the connection VS Code has. Stage 2
gives their model-level operations RPCs, and the editor keeps its methods as thin translations.

### 2.5 Local-only

| Operation | Why it stays local |
|---|---|
| `%load <path>`, `%save <file>`, `-o`, `-output` | Paths on the caller's disk. The wire takes and returns bytes (`ParseSources`, `Convert`) and that is the right shape |
| `-sync-*` (project sync with a repository), `internal/core/project`, `internal/interop/reposync` | Operates on a checked-out project and its remote; a workspace model with identity and history. Out of scope for a model-in/answer-out service until a workspace API is designed on its own terms |
| `-cpuprofile`, `-memprofile`, `-memstats`, `-debug` | Process introspection |
| `%help`, `%verbosity`, tab completion, unknown-command suggestion | Presentation |
| `-html-*`, `-pdf-*`, `-doc-*` | Renderer options. `RenderDocument` takes a form; the options that shape a form should travel with it (stage 2) rather than stay flags, but the *files* the renderer writes (assets, a PDF) are local |

## 3. Stages

Each stage is a pull request into `develop` that leaves the gate green. The first stage is a
refactor with no visible change and every later stage depends on it; the rest can go in any
order, and 4 is independent of 2 and 3.

### Stage 1 — one assembly per operation

For each shared row in §2.1, one function in `internal/core` (or a new `internal/core/ops`
package if no existing package owns it) that takes a resolved model plus a plain-Go request and
returns a plain-Go result, and that `internal/repl`, `internal/lsp` and `internal/grpc` all call.
The REPL's job becomes: parse the meta-command, call, print. The service's job becomes: decode the
proto, call, encode. The editor's: translate the position, call, shape the protocol reply.

Concretely, for verification: `internal/grpc/verify.go` and `internal/repl/verification.go` each
resolve the constraint and its subject, choose an engine through `analysis`, run it and grade the
outcome into a verdict. They become one `VerifyConstraint(model, Request) Result` with a `Request`
of symbol, subject, engine and budget, and a `Result` of verdict, standing, plan and diagnostics.
The proto `VerifyConstraintResponse` and the REPL's `Verdict` are both projections of `Result`.

The same for `Instantiate`, `Evaluate`, `EvaluateCalc`, `RunAnalysis`, `RunSweep`,
`ExecuteAction`/`ExecuteState` (the run-to-completion path), `Query`, `RunDocumentQuery`,
`RenderDocument`, `Convert`, `ApplyEdits`, and for the editor's hover and definition against
`GetSymbol`.

Rules for the shared layer:

- Plain Go in, plain Go out. No proto types, no REPL types. `Value` stays `runtime.Value`;
  the wire's `Value` message is `internal/grpc`'s projection of it, as now.
- No I/O. The REPL passes bytes it read; the service passes bytes it received.
- Every failure is a typed error or an in-band diagnostic; the caller decides whether that is a
  Connect code, a REPL notice or an LSP diagnostic. The classification the wire contract
  documents (`docs/reference/wire-contract.md`, "Three places a failure can be") is already the
  right split; the REPL adopts it.
- The `Service` cache and the REPL session stay where they are: they are the two lifecycles that
  own a model; the shared layer is handed a model and owns nothing.

Definition of done: `internal/repl` and `internal/lsp` import nothing from `internal/grpc`
(the instance-graph helper in `features.go` moves down), and no operation in §2.1 is assembled
in more than one package. The existing cross-surface tests (`oslc_query_repl_test.go`,
`library_invocation_test.go`) keep passing; they become regression guards for the refactor rather
than the only thing holding the surfaces together.

### Stage 2 — the missing stateless operations

Each row of §2.2 gains, in order: the shared function (stage 1's shape), a proto RPC or request
fields, a `Service` method, a capability string, a Go package method, a wire-contract section, a
conformance scenario, and the REPL and CLI rewritten to call the shared function. Every one is
gated by a capability so an older service refuses it with `CodeUnimplemented`, as the package
already does for an unavailable capability, rather than failing on an unknown method.

1. **Satisfiability.** `Solve(model_hash, symbol_id, subject, ask, engine)` posing
   `analysis.Satisfiable`, with `ask` selecting check (is it satisfiable), solve (an
   assignment, keeping what is fixed), explain (the conflicting conditions), configure (the
   variants the conditions permit) or optimize (the best values an analysis case's objectives
   admit). The response is the REPL's `SolveReport` on the wire: status, the assignment as
   `Value`s, unfixed features, the queries asked. The five meta-commands become five asks of one
   call. Capability `solve`.
2. **Checker configuration.** A `CheckOptions` message — properties, assumptions, inputs,
   divergence features, bounds (depth, states, unroll, timeout) — accepted by `RunAnalysis`,
   `Verify*` and `Execute*` where `engine` is accepted today. The REPL's `%check-*` commands set
   the session's default `CheckOptions`; the CLI's `-check-*` flags fill one. Witnesses come
   back in the response rather than being written to a directory; `%check-witness <dir>` stays
   REPL-side and writes what the response carries. Capability `check_options`.
3. **Replay.** `schedule: replay` with the witness inline as a repeated message, in the shape the
   `check` and `smt` engines already produce for their witnesses (see
   [the analysis framework](analysis-framework.md)). `%replay <path>` reads the file and sends
   the content. Capability `replay`.
4. **Views.** `ListViews(model_hash)` and `RenderView(model_hash, view, form, palette)` returning
   the artifact plus the nodes and edges the editor's `renderResult` already carries, each
   located in source. The editor's `opensysml/render` and `opensysml/views` become translations
   of these; `%view`, `%render` and `-render*` call the shared function. Capability `views`.
5. **Search.** `SearchSymbols(model_hash, text, kinds, limit)` over `suggest` and the frozen
   standard-library index every model already shares (`internal/grpc/libindex.go`). `%search` and
   `%builtins` call it; editor completion draws its candidates from the same ranking. Capability
   `search`.
6. **Renderer options.** The `-html-*`, `-pdf-*` and `-doc-*` flags as fields of
   `RenderDocumentRequest`, so a remote client renders the same document the CLI does.

### Stage 3 — open the `Convert` format list

`Convert` takes a format name; codegen (`-compile -target <target>`) produces bytes for a target.
Register codegen targets as `Convert` formats, one capability per target family, and drop
`-compile`'s private path in favor of the shared `Convert`. The public package's `Convert` then
reaches them without a new method. In the same stage, `ConvertResponse` gains the XMI migration
report as a repeated message when the source format is XMI, and `-migration-report` writes what
the response carries. This is the smallest stage and depends only on stage 1's `Convert`
assembly.

### Stage 4 — a session API for the debuggers

The action and state debuggers need a protocol of their own. Design questions, with the
positions this note takes:

- **Lifecycle.** A `Session` is opened over a model hash and a behavior, stepped, and closed;
  the server bounds how many are open and how long an idle one lives (the same bounds the model
  cache has). The REPL's rule — a session ends when the behavior it steps or the object it
  materialized is superseded — becomes `OpenSession` failing on a model hash the cache has
  evicted, and the REPL's own bookkeeping staying where it is.
- **Shape.** Bidirectional streaming (`StepSession` as a stream of commands in and events out) is
  the natural shape for `%step`/`%tokens`/`%events`, and the [transport evaluation](transport-evaluation.md)
  §8 already lays out what streaming costs each protocol: gRPC, Connect over HTTP/2 and stdio
  carry bidirectional streams, Connect over HTTP/1.1 does not, and a browser can consume only
  server-streaming. Unary `Step(session_id)` returning the state after the step is the fallback
  that every protocol serves; design the messages so both are the same messages.
- **Addressing.** Breakpoints name a lowered node or state by its symbol id and the executor's
  own node identity, which the trace already prints; the session returns them so a client can
  set a breakpoint on what it saw.
- **Trace.** `%trace` becomes the event stream; the REPL prints it, a client consumes it.
- **Not in scope.** The REPL's incremental declaration model (`%list`, `%clear`, superseding by
  name) is not a service concept; a client that wants to change the model re-parses and opens a
  new session.

The design goes in its own note before code is written, as the analysis framework's did.

### Stage 5 — parity as a test

Extend `cmd/conformance` so that every scenario runs on a third protocol beside `pkg` and
`pkg-connect`: `repl`, which submits the scenario's model to a REPL session and issues the
meta-command that corresponds to the RPC, then compares the REPL's machine-readable output
(`-json` where the CLI has it; a `SolveReport`, a `Verdict`) to the scenario's expected response
through the same projection the service uses. A scenario that has no REPL counterpart declares it
(`repl: none`), and the runner reports the count, so the residual mismatch is a number in the
conformance report and not a claim in this note.

This is the stage that makes stages 1–4 stay done. It depends on stage 1 (there has to be one
result type to project) and on the shared conformance fixtures of the roadmap's Track I.

## 4. What this does not change

- **The binaries keep calling the engine in-process.** No stage puts a socket between `sysml` or
  `sysml-lsp` and `internal/core`. Stage 1 makes them call the same functions the service calls;
  it does not make them call the service.
- **The public package's contract.** `New` and `Dial` keep the same interface and the same
  semantics; new methods arrive with capabilities and are refused with `CodeUnimplemented`
  against an older service, as today. Nothing existing is renamed.
- **The wire contract.** Every existing field keeps its meaning. New RPCs and fields are additive
  and documented in `docs/reference/wire-contract.md` as they land; the Buf breaking-change check
  stays the gate.
- **The editor's protocol.** `internal/lsp` keeps speaking LSP and its custom methods to VS Code.
  What changes is what it calls underneath.
- **The REPL's language.** Meta-command names, arguments and printed forms are unchanged; the
  changelog records any output that a stage makes agree with the wire where it did not before.
- **Performance.** The transport numbers in the [transport evaluation](transport-evaluation.md)
  are for remote callers and stay as they are; in-process callers gain a function-call boundary
  and lose nothing. Any stage that changes what the REPL allocates per command is measured
  against `develop` before it merges.

## 5. Order and size

Stage 1 first; it is the largest — one pull request per operation family (verification,
execution, analysis, query, documents, edits, symbols) is the right grain, each leaving the
suites green — and everything else is a projection of it. Stages 2 and 3 follow in the order
listed, each item its own pull request, each landing with its conformance scenario. Stage 4 is a
design note before it is code, and can be written while stages 2 and 3 are in progress. Stage 5
lands its runner after the first stage and gains a `repl` row for each scenario as the
corresponding stage rewires the REPL.
