# SysML v2 Execution Environment — Architecture

**Module:** `github.com/Open-MBEE/OpenSysML`  
**Language:** Go 1.23+

## Overview

A SysML v2 and KerML 1.1 implementation delivering the integrated tooling experience systems engineers expect from modern language ecosystems (Python, Rust, Go).

### Core Components

1. **Language Server (`sysml-lsp`)** — IDE support with live diagnostics, semantic hover, go-to-definition, intelligent completion, and workspace-wide symbol search
2. **Interactive REPL (`sysml`)** — Exploratory modeling: define models incrementally, evaluate expressions, instantiate parts, inspect runtime state
3. **Execution Runtime** — Instantiate parts, evaluate constraints, execute calc/analysis cases, simulate behavioral models
4. **Toolchain** — Workspace management, dependency resolution, incremental compilation, bundled stdlib, persistent caches

### Design Principles

- **Performance:** sub-millisecond parsing, a single static binary, and no JVM or Eclipse runtime
- **Completeness:** SysML v2 textual notation support (98 of 98 standard library files parse cleanly: 94 vendored OMG files and 4 OpenSysML extensions)
- **Executable models:** beyond validation, a runtime that instantiates, evaluates and simulates
- **Incremental and lazy:** parse immediately and resolve semantics on demand, following the precedent set by gopls and rust-analyzer
- **Immutable AST:** all semantic state resides in side tables keyed by node or symbol

This architecture is also written as a SysML v2 model of itself in
[examples/self-model/](../../examples/self-model/README.md), where the stages carry the Go
packages that implement them, the invariants below are requirements the tool evaluates, and
`make self-model` renders the diagrams.

---

## Architecture Layers

```
┌─────────────────────────────────────────────────────────┐
│  Frontends: LSP Server │ Interactive REPL               │
├─────────────────────────────────────────────────────────┤
│  Workspace: Multi-file projects, dependency management  │
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

---

## Module Structure

```
github.com/Open-MBEE/OpenSysML
├── cmd/
│   ├── sysml-lsp/          # LSP server binary
│   ├── sysml-grpc/         # gRPC server binary
│   └── sysml/              # Interactive REPL binary
├── internal/core/
│   ├── source/             # Source files, spans, line indexing
│   ├── lexer/              # Hand-written scanner (~200 keywords)
│   ├── parser/             # Recursive-descent parser
│   ├── ast/                # Syntax tree nodes (immutable)
│   ├── symbols/            # Symbol tables, scope trees
│   ├── resolve/            # Name resolution (lazy, memoized)
│   ├── semantics/          # Type system, conformance, multiplicity
│   ├── passes/             # Validation passes (syntax → constraints)
│   ├── queryplan/          # Document-query definitions → immutable query plans
│   ├── queryexec/          # Query plan execution → typed, ordered row sets
│   ├── docplan/            # Document definitions → immutable document plans
│   ├── docir/              # Document plan evaluation → backend-agnostic document tree
│   ├── lower/              # AST → execution IR (ActionGraph/StateGraph)
│   ├── runtime/            # Execution engine (eval, instances, builtins)
│   ├── model/              # Workspace, document management
│   └── libs/               # Standard library bundling & caching
├── internal/lsp/           # LSP protocol implementation
├── internal/repl/          # REPL loop implementation
├── internal/grpc/          # gRPC service implementation
├── clients/python/         # Python client bindings (opensysml)
├── clients/rust/           # Rust client (opensysml) and its conformance runner
├── api/proto/              # Protobuf service definitions
├── testdata/               # Test fixtures (.sysml, .kerml)
├── examples/               # Example models and demos
└── docs/                   # Documentation
```

---

## Core Pipeline

**Static Analysis Path:**

```
source → lexer → parser → AST → symbol index → resolve → passes
```

### 1. Source & Lexer (`internal/core/source`, `internal/core/lexer`)

- **SourceFile:** Input file (.sysml or .kerml) with byte content
- **Lexer:** Hand-written scanner producing tokens with full position tracking
- **Trivia:** Comments and whitespace tracked as leading/trailing trivia
- **Keywords:** ~200 SysML keywords (case-sensitive, pre-registered)

### 2. Parser (`internal/core/parser`)

- **Hand-written recursive descent** (chosen over ANTLR4/yacc/JNI bridge)
- **Rationale:** Zero overhead, full error recovery, sub-ms parses for keystroke-latency feedback
- **Entry:** `parser.New(source).ParseFile() → *ast.RootNamespace`
- **Always produces tree:** ErrorNodes on bad input, parsing never fails
- **Grammar source:** OMG pilot Xtext grammars (SysML.xtext + KerMLExpressions)

### 3. AST (`internal/core/ast`)

**Key architectural rule:** AST is syntax-only, **immutable after parse**

- **Node interface:** `{Span() source.Span; LeadingTrivia()/TrailingTrivia() []Trivia}`
- **NodeBase:** Embedded by all nodes
- **No semantic info in AST:** All derived data lives in **side tables keyed by node/symbol**
- **Expression AST:** Full SysML v2 expression grammar (literals, operators, feature refs, invocations, collections, lambdas)
- **Behavioral AST:** Action control-flow nodes (InitialNode, FinalNode, ForkNode, JoinNode, MergeNode, DecisionNode, ActionExecutionNode), succession edges with guards

### 4. Symbols & Resolution (`internal/core/symbols`, `internal/core/resolve`)

- **Symbol:** `{Name, Kind, Decl ast.Node, Visibility, Scope, OwnerScope}`
- **Scope:** `{Parent(), Node(), Children(), LookupLocal(name), MemberNames()}`
- **Index:** `DocumentRoot(name) *Scope` — global qualified-name index
- **Resolver:** Lazy name resolution, memoized, `ResolveQualified(scope, *ast.QualifiedName) (*Symbol, bool)`
- **Deduplication:** Short+primary names alias same `*Symbol` — dedupe by pointer when walking

### 5. Semantic Model (`internal/core/semantics`)

**Runtime's primary substrate. Built via `NewModel(*resolve.Resolver)`. All results memoized in side tables.**

- **`model.go`:**
  - `DirectSupertypes(sym)` — resolved generalization edges (specializes/subsets/redefines/typing)
  - `AllSupertypes(sym)` — transitive, cycle-safe
  - `Conforms(a, b) bool` — conformance checking
  - `HasSpecializationCycle(sym) bool`
- **`members.go`:**
  - `MembersOf(sym)` — local + inherited members with masking
  - `LookupMember(sym, name)` — member lookup
  - **Effective feature list per type** (substrate for runtime instantiation)
- **`multiplicity.go`:**
  - `MultiplicityOf(sym) (Range, bool)` — parse multiplicity bounds
  - `Range{Lower, Upper Bound}`; `Bound{Value int64, Infinite bool, Known bool}`
- **`eval.go`:**
  - `Eval(n ast.Node) (Value, bool)` — **constant-folder** (seed of runtime)
  - `Value{Kind ValueKind, Int, Real, Bool}` — int/real/bool/infinity only
  - Returns `ok=false` for feature refs, strings, null, invocations, collections
  - **Runtime Tier 3 extends this to full evaluator**

### 6. Validation Passes (`internal/core/passes`)

**Pluggable validation tiers:**

- **PassLevel:** `{LevelSyntax, LevelNameResolution, LevelType, LevelConstraint}`
- **Pass:** `{Level() PassLevel; Run(ctx, name, root) []Diagnostic}`
- **Context:** Exposes `Resolver()` + `Model()` (both lazy, memoized) and `DownstreamOfFailure(ref)` — did a lower tier report a blocking diagnostic inside this reference?
- **DefaultRegistry:** SyntaxPass, NameResolutionPass, TypeCheckPass, ConstraintPass
- **Tiered execution:** a document-scoped pass at a higher tier is skipped once a lower tier errors; a pass marked `ElementScoped` runs and gates itself per subject through `Context.DownstreamOfFailure` ([element-scoped tier gating](../project/element-scoped-tier-gating.md))
- **Quick fixes:** A `Diagnostic` carries the `quickfix.Fix` values (`internal/core/quickfix`) the layer reporting it attached, so an editor offers edits without parsing messages

### 6a. Highlighting (`internal/core/highlight`)

- **Semantic tokens:** `Tokens(content, root, scope, SegmentResolver)` — keywords, comments and literals from the lexer; declared names from the symbol table; reference segments from the resolver
- **Ordered and disjoint:** the result is sorted by offset with overlaps dropped, semantics winning, so a consumer encodes it directly
- **Vocabulary:** LSP token types and modifiers (`Classes()`, `Modifiers()` give legend order)

### 7. Workspace (`internal/core/model`)

- **Single source of truth:** Owns document set + global index + diagnostic cache
  + reverse reference index
- **Document:** `{source, AST, scope, version}`
- **One Workspace per session** (LSP/REPL)
- **Reverse reference index** (`refindex.go`): every name segment written in a
  workspace document, keyed by the element it denotes (`symbols.KeyOf`, i.e.
  declaring document + declaration span — stable across reindexing, unlike a
  `*Symbol`). Each segment is stored under two identities: the element it
  *reaches* (after invocation overload selection; a tied call reaches nothing)
  and the name it *writes* (an alias, where one was written), with the
  `resolve.Reference` it is a segment of. Find References matches either;
  Rename edits only the written name, and `RenameConflict` checks each
  occurrence for capture through `internal/core/rename` — a trial reading of the
  reference with that segment respelled (`Resolver.ProbeReading`, which keeps
  what each segment reached even where the whole name then fails), so a chain
  member is read in its operand's type, a redefinition target among the
  generals, and a qualifier respelled onto an element lacking the rest of the
  name is still seen — the check the batch edit API shares. Built lazily on the first
  query after a change, over all documents with one shared resolver and
  semantic model, under the workspace's write lock; never built on the
  `didChange` path. Any mutation (`reindexLocked`, `removeLocked`, a
  conformance-mode switch) drops the whole index, because an edit to one
  document can change what a name in another resolves to (a shadowing
  declaration, an import target, an alias, an overload that ties a call).
  Library documents are never enumerated; only workspace documents are.

### 8. Standard library (`internal/core/libs`)

- **Source of truth:** the 98 library files under `internal/core/libs/stdlib/`, embedded in
  the binary; `OPENSYSML_LIBRARY_PATH` substitutes a directory of files for them.
- **Shared base:** `libs.SharedBase()` builds one frozen `symbols.Index` of the library per
  process; every model is an overlay over it (`NewOverlay`), reading the library without copying
  it.
- **Snapshot:** `internal/core/libs/stdlib.snapshot` is that frozen index — syntax trees, scopes,
  symbols, wildcard-import expansion, facts — serialized at generation time
  (`go generate ./internal/core/libs`, `make stdlib-snapshot`) and embedded. A process decodes it
  instead of parsing, when its recorded digest of the library files and its format version match
  the files in hand and its CRC-32C over the stream holds; otherwise (an edited file, a
  library-path override, a stale or damaged blob) it parses the files as before. The snapshot is
  a derived artifact: never edit it, regenerate it, and `TestEmbeddedSnapshotIsCurrent` plus
  `make stdlib-snapshot-check` in CI fail when it lags the files.
- **Encoding:** `internal/core/pack` (varint scalars over a string table) and
  `internal/core/ast/astcodec` (a node table, every node type, index references in place of
  pointers); `symbols.WriteSnapshot`/`ReadSnapshot` number scopes and symbols the same way. No
  reflection or `encoding/gob`. Decoding reproduces the object graph a fresh load builds, sharing
  and all, which `TestSnapshotIndexMatchesFreshLoad` checks structurally.
- **Facts cache:** `$XDG_CACHE_HOME/sysml-ls/libs` still holds derived facts for library sets
  the snapshot does not cover, keyed by content digest and build.

---

## Execution Runtime Architecture

**Package:** `internal/core/runtime`  
**Not a Pass:** Execution is stateful/iterative/value-producing (different shape than diagnostic-emitting pass)

### Tier 1 — Feature Flattening ✅

Harden `MembersOf` into stable, ordered **effective-feature list** per type:
- Own + inherited − redefined/masked
- Each entry: type + multiplicity + default-value expression
- **Schema for instance materialization**

### Tier 2 — Instance Model ✅

- **Value:** Extends `semantics.Value` → `null`, strings, **instance references**, **collections** (sequences/sets)
- **Instance:** Typed object with one feature value per effective feature (Tier 1)
- **Instantiation:** Materialize instance graph from `part`/`item` usage
  - Recursively instantiate composite features
  - Multiplicity governs feature value cardinality
  - Lazy feature value materialization

### Tier 3 — Expression Evaluator ✅

Full evaluator with **user-defined calc invocation**, **constraint evaluation**, and **requirement evaluation**:
- Feature access `x.y.z` resolved against instance feature values
- KerML operator library (`->select`, `->collect`, `size`, string ops)
- **Calc invocation:** Resolve calc symbol → extract params/return → bind args to parameters → evaluate return expression
- **Function values** (`function_value.go`, `ValFunction`): a calc definition, a calc usage with an unsupplied input or an `in calc` parameter read as a value is the calc's lowered `calcShape` plus the environment it was read in — declaring scope, the object it was read off, and, for a calc declared inside a behavior body, the frames through the innermost active run of that behavior (`EvalContext.enclosingRun`, by `frame.runs`) — never a caller's frames, and none when no such run is active. Invoking one (`f(a)` through a calc-typed parameter, or `SampledFunctions::Sample` applying its `calculation`) takes the calc invocation path (`invokeCalcShapeIn`), never a closure over statements; `ValExpr` remains the distinct kind for an expression body a collection operation evaluates per element
- **Constraint evaluation:** Extract `assert`/`assume` members → evaluate boolean expressions → check satisfaction (with optional `not` negation)
- **Requirement evaluation:** Extract `subject`/`assume`/`require`/`actor` members → validate bindings → evaluate conditions
- **Scoped evaluation:** `EvalContext.scope` for name resolution, frame stack for parameter bindings
- **Membership unwrapping:** Runtime automatically unwraps AST Membership nodes when extracting members
- **Compiled calc tier** (`compile.go`, `compiled_ops.go`, `compiled_stmts.go`): a calc whose body is scalar — Integer/Real/Boolean literals, its effective `in` parameters (flattened through the specialization chain and redefinition exactly as `calcShape` lays them out for `bindCalcParameters`), the arithmetic, comparison, equality, identity, logical and conditional operators, body-local scalar declarations, `return` and `if`/`else` statements, invocations of other such calcs (cycles included, positional or by name), and the standard library's scalar functions and constants (`sqrt`, `ln`, `sin`, `TrigFunctions::pi`, … — dispatched through the resolved symbol to the same Go implementation the evaluator calls, never by bare name, so a model's own `sqrt` is an ordinary calc) — is compiled on its first invocation into a tree of Go closures over an unboxed scalar frame, held in a side table on the `Context`'s `calcShape` (the AST is untouched, and a new `Context` compiles afresh). Statements are compiled from the lowered `calcShape.Steps`: each declaration takes a fresh frame slot at compile time, masking an earlier binding of its name for the rest of its block, so shadowing and order behave as `stmtEngine` does, and a name read before its declaration is declined rather than guessed. It reproduces the evaluator's values, errors and per-node step charges exactly; the differential test (`compile_differential_test.go`) checks that over every calc in the fixture and example trees, and `compile_constructs_test.go` over focused fixtures in `testdata/compiled/`. Anything outside the subset — calc usages, `out` features, feature chains, `self`, collections and the library functions over them, quantities, strings, loops and assignments, a local without a value, a body that may run off its end, non-literal defaults — keeps the calc, and every calc calling it, on the evaluator.
  - **Fallback rule:** a traced `Context` (`ctx.trace != nil`), a non-scalar argument, an unbound parameter without a default, a receiver object where the body reads a library constant, and `OPENSYSML_CALC_COMPILE=0` run the whole invocation on the reference evaluator; the tier never falls back for a sub-expression. Argument checking (`calcShape.checkArgs`: arity and unknown names; the evaluator's refusal of a receiver beside named arguments) precedes the dispatch, so both tiers report those identically.
- **Unlocks:** Constraint checking against concrete values, `calc` execution, requirement validation, runtime behavioral verification

### Tier 4 — Behavioral AST ✅

Parse + model all behavioral bodies with unified fallback grammar:
- **Calc bodies** — `return` expressions + mixed parameter declarations (✅ **fully executable**)
- **Constraint bodies** — `assert`/`assume` with optional `not` negation (✅ **fully executable**)
- **Requirement bodies** — `subject`/`assume`/`require`/`actor` declarations (✅ **fully executable**)
- **Action bodies** — Control flow nodes (initial/final/fork/join/merge/decision) + action execution nodes + succession edges (✅ **parsed**, executor infrastructure complete)
- **State bodies** — Entry/do/exit behaviors, substates, transitions with triggers/guards/effects (✅ **parsed**, executor infrastructure complete)
- **Unified Grammar:** Body parsers use graceful fallback to general member grammar (no terminal keyword whitelists)
- **Status:** All parsers complete. Calc/constraint/requirement **fully executable**. Action/state **executors complete** with control flow keywords, nested invocation, send statement.

### Tier 5 — Behavioral Interpreter ✅ Complete

**Package:** `internal/core/runtime`  
**Status:** Complete. Conformance gate: every case passing (calc/constraint/requirement/satisfy/action/state all functional); count in [the measured counts](../project/spec-compliance.md).  
**Spec Alignment:** The governing reference is the SysML v2 metamodel or the bundled KerML semantic library (`internal/core/libs/stdlib/`); UML 2.5.1 is a fallback only where the SysML v2 notation has no production for a concept *and* the KerML library no performance for it (state-body `fork`/`join`, history, regions). The runtime is not a UML or fUML activity engine: "token" names the executor's bookkeeping for where each performance is along the successions of the lowered graph, an implementation device, not the semantic model. What the tokens realize is succession order: a succession is a KerML `HappensBefore` link (`Occurrences.kerml`), which orders occurrences in time and carries no values — a `SuccessionFlow` is the form that carries a payload (`KerML.kerml`: `Succession specializes Connector`, `SuccessionFlow specializes Succession, Flow`). State machine execution is `Occurrences::Occurrence::isRunToCompletion` over its `runToCompletionScope` ("determines whether transition performances might happen during state entry performances within the run to completion scope"), with event dispatch `isDispatch` / `dispatchScope`. See [SPEC_COMPLIANCE.md](../project/spec-compliance.md) for the detailed compliance mapping, and [the pilot differential](../project/pilot-differential.md) for what is checked against the reference implementation.

**Architecture:**

1. **ActionExecutor** — succession-ordered action execution, scheduled as a token queue over the lowered `ActionGraph`
   - Token-based control flow (initial → action → final, first/done keywords)
   - `fork`/`join` for parallelism, `decide`/`merge` for branching (`Actions::ForkAction`, `JoinAction`, `DecisionAction`, `MergeAction`)
   - Nested action invocation with attribute initialization
   - Send statement for message passing
   - ObjectFlow for pin-to-pin data routing
   - Deadlock detection via progress tracking
   - Golden trace recording with deterministic token ordering
   - APIs: `Step()`, `RunToCompletion()`, `Tokens()`, `SetBreakpoint()`, `SetTrace()`
   - Breakpoints and stepping observe action nodes, so a calc an action invokes is one step to them; only a `TraceRecorder` observes sub-expressions, and it keeps the calc on the evaluator (Tier 3's fallback rule)

2. **StateExecutor** — Event-driven state machine execution
   - Initial/final state keywords (initial/final)
   - Entry/exit/do behaviors (`do` runs while its state is active, one action per round, interleaved with the do behaviors of the states active alongside it)
   - TimeEvent waits registered on the `Context`'s `Clock`, which every executor of the context shares (`clock.go`, `advance.go`: `Context.Advance` runs all due work instant by instant; executors due together are a scheduler choice)
   - ChangeEvent condition polling
   - Guard evaluation for transitions
   - Transition effect actions
   - Hierarchical states with LCA-based entry/exit propagation
   - Orthogonal regions with multi-region event broadcasting; the order sibling regions react in is a scheduler choice, drawn per firing among the regions still active (`dispatchInOrder`), for change triggers as for queued events
   - Choice + Junction pseudostates
   - Golden trace recording for transitions/entry/exit
   - APIs: `ProcessNextEvent()`, `CurrentState()`, `EventQueue()`, `StateData()`, `SetTrace()`
   - Deferred events: an event no active transition handles is retained while a state deferring it is active, and delivered afterwards in arrival order
   - CallEvent matches the operation named by the trigger (`signal.go`, `state_executor.go`; `signal_test.go:TestCallEventMatchesOperationName`)

3. **Scheduler and choice points** — one resolution rule for what the library leaves unordered ([design note](design/scheduling.md))
   - Six `ChoiceKind`s (`choice.go`): token order within a step, decision branch, same-step write order, transition, region order, due order; each site resolves through the run's `scheduler` and then records a `ChoicePoint`, an informational `RunNote` (diagnostic code `choice-point`) that never alters the run
   - Every decision guard is evaluated so a second holding one is seen; a later guard that cannot be evaluated is an `UnevaluableGuard` note (`guard-unevaluable`), not a failure
   - `SchedulePolicy` (`scheduler.go`): `reverse` (default and zero value — exactly what every run did before policies existed), `declared`, `seed:<n>` (a PCG generator the run consumes, replayed by the seed), `replay:<file>` (a witness's choice lines followed move for move, then `reverse`; a move the run cannot make is a typed `ReplayError`, `replay.go`), `explore[:runs=N,depth=D]`
   - `Explore` (`explore.go`) replays whole runs from a fresh `Context` each, a recorded choice prefix then the first untried alternative, depth-first within `ExploreBudget` (default 1024 runs, 64 choice points); reports distinct outcomes by `Outcome.identity` with linearization counts and a witness, and `incomplete` when a bound stopped it
   - The scheduler lives in the run's `runState` beside the budget and notes; a run driven call by call (`beginExecutorRun`) keeps its own across interleaved runs, and a probe (`beginProbe`) restores the scheduler's position and notes nothing

4. **Context Integration** — Public runtime APIs
   - `InvokeCalc(symbol, args)` — Invoke calculation with arguments, return result
   - `EvaluateConstraint(symbol)` — Evaluate constraint, return satisfaction boolean (assert/assume)
   - `EvaluateRequirement(symbol)` — Evaluate requirement, return satisfaction boolean (require/subject/actor/assume/nested)
   - `ExecuteAction(symbol)` — Run action to completion, return results
   - `ExecuteState(symbol)` — Run state machine until final/suspended
   - `CreateActionExecutor(symbol)` — Create executor for debugging
   - `CreateStateExecutor(symbol)` — Create executor for debugging
   - `SetSchedule(policy)`, `Schedule()` — the policy runs started from now on resolve their choice points under (`explore` is refused: `Explore` drives it)
   - `Notes()`, `Choices()`, `UnevaluableGuards()` — what the last run recorded

**Implementation:**
- `context.go` (460 lines) — Public Execute/Invoke/Evaluate APIs, step budget enforcement
- `action_executor.go` (729 lines) — Token-flow engine with nested actions, send statement
- `state_executor.go` (1149 lines) — Event-driven state machine with do behaviors
- `executor_common.go` — Token, Event, EventQueue, ExecutionState
- `scheduler.go`, `choice.go`, `action_choice.go`, `explore.go` — scheduling policies, choice-point notes, bounded exploration
- `trace.go` (154 lines) — Deterministic execution trace recorder
- `eval.go` — Expression evaluation (binary/unary operators, literals, feature references, qualified names, type coercion)
- Lowering to execution IR lives in `internal/core/lower/` (`ToActionGraph`, `ToStateGraph`)

**Testing:**
- **Golden ASTs**: `internal/core/parser/testdata/parse/` — count in [the measured counts](../project/spec-compliance.md)
- **Negative tests**: `internal/core/parser/negative_test.go` — count in [the measured counts](../project/spec-compliance.md)
- **Unit tests**: `action_executor_test.go`, `state_executor_test.go` (action, state)
- **Conformance gate**: `.sysml` + `.expected.json` pairs, all passing - `conformance_test.go` — counts and per-category breakdown in [the measured counts](../project/spec-compliance.md); a case whose model admits several results lists them as `outcomes`, each cited to [the semantic oracle](../project/behavior-semantic-oracle.md), and is explored to prove every one reachable and nothing else; `TestExecutionConformanceUnderPolicies` re-runs the suite under `declared` and `seed:1`
- **Golden traces**: `.trace.golden` files - `trace_test.go` — count in [the measured counts](../project/spec-compliance.md); `.trace.order` files state the partial order a trace must respect (`a < b`), and a case with `outcomes` owns a `<case>.<policy>.trace.golden` per sweep policy
- **Exploration**: `explore_test.go` — every linearization reached once, determinism, each budget's incompleteness, an error as an outcome, transition, region and due order
- **Robustness**: failure-mode cases (deadlock, unbound params, missing features, dangling transitions, sourceless accept, step budget, pseudostate dead ends and cycles, history and defer misuse, send/accept misrouting, calc arity/recursion, `perform` reference failures) - `robustness_test.go`
- **Coverage**: All behavioral types fully functional. Action: 14/14 features ✅. State: 13/13 features ✅. Calc: 8/8 ✅. Constraint: 5/5 ✅. Requirement: 5/5 ✅. Evaluation: 7/7 ✅.

**Measured Compliance:** See [SPEC_COMPLIANCE.md](../project/spec-compliance.md) for semantic rule → implementation → test case mapping with status (✅ faithful / ⚠️ approximate / ❌ not yet implemented).

### Tier 6 — Analysis & Verification Drivers ⏳ (Future)

- Analysis case: subject → calc chain → result values
- Verification case: evaluate requirements → pass/fail
- Entry points: REPL/LSP commands (`%run`, `%verify`)
- Design: [the analysis framework](design/analysis-framework.md) — registered engines behind
  one contract, one scale for the strength of an answer, parallel isolated runs; and
  [bring your own engine](design/bring-your-own-engines.md) — how a user's engine, strategy
  or tool registers and what its answers are worth
- Landed: `internal/core/analysis` — the contract (`Question`, `Engine`, `Result`, `Claim`,
  `Strength`, `Budget`), a per-owner `Registry` with `auto` dispatch, and the `run`, `explore`,
  `sweep` and `solve` engines as adapters over the runtime and `internal/core/solve`; the REPL
  session and the gRPC service ask every check, run, exploration, sweep and solver question
  through it with no change to what they print or return

---

## LSP Server

**Package:** `internal/lsp`  
**Binary:** `cmd/sysml-lsp`  
**Status:** ✅ Complete (stdio protocol, 10 LSP features, tested end to end)

### Features

**Lifecycle:**
- `initialize` — Advertise server capabilities, record the session's folders
- `initialized` — Scan those folders and index every `.sysml`/`.kerml` file they
  hold, so cross-file names resolve without the editor opening each file
- `shutdown` / `exit` — Graceful termination

**Document Synchronization:**
- `textDocument/didOpen` — Track opened documents (the buffer becomes authoritative)
- `textDocument/didChange` — Incremental updates (UTF-8 byte offsets)
- `textDocument/didClose` — Revert to the file's on-disk content; the document
  stays indexed, since other documents resolve names through it, but its markers
  are withdrawn — only open documents carry diagnostics
- `textDocument/didSave` — Refresh diagnostics for every open document
- `workspace/didChangeWatchedFiles` — Reindex files created, edited or deleted
  outside the editor; a deletion leaves an open buffer alone
- `workspace/didChangeWorkspaceFolders` — Walk a folder added mid-session and
  unindex what a removed one contributed, open buffers aside

**Diagnostics:**
- Publish on document open/change; the edited document immediately, the other
  open ones on a coalesced sweep once the edit burst settles, since each sweep
  re-analyzes them
- Withdrawn (empty set) for a document the workspace no longer holds
- Syntax errors (parser)
- Semantic errors (name resolution, type checking, validation passes)
- Real-time feedback

**Hover (textDocument/hover):**
- Symbol info: name, kind, type, multiplicity
- Definition source location
- Documentation comments (future)

**Go-to-Definition (textDocument/definition):**
- Navigate to symbol declaration
- Follows qualified name chains
- Cross-document navigation

**Find References (textDocument/references):**
- Find all usages of symbol, in every workspace document, at whichever segment
  of a qualified name denotes it
- Include declaration option, reported in the declaring document
- Answered from the workspace's reverse reference index (a lookup, not a scan);
  Rename reads the same index

**Rename (textDocument/prepareRename, textDocument/rename):**
- Rewrites the name under the cursor — long or `<short>` — at its declaration
  and wherever a reference in any workspace document writes it
- Refused with an error naming the element the new name would mean when that
  name is already taken where the element is declared, or when a rewritten
  reference would afterwards read another element (`internal/core/rename`)

**Completion (textDocument/completion):**
- Trigger characters: `:`, `.`
- Symbol-based suggestions
- Future: keyword completion, snippet support

**Semantic Tokens (textDocument/semanticTokens/full, /range):**
- Legend advertised at `initialize`; tokens classified by `internal/core/highlight`
- Keywords, comments and literals from the token stream; names from the symbol
  table and the resolver, with declaration/definition/readonly/abstract modifiers
- Encoded relative to the previous token, split per line; no delta support

**Code Actions (textDocument/codeAction):**
- Quick fixes only, from the `quickfix.Fix` values parser and resolver
  diagnostics carry — spelling of an unresolved name, importing the namespace
  declaring it, inserting a semicolon the parser located exactly

**Document Symbols (textDocument/documentSymbol):**
- Outline view (packages, parts, attributes, actions, states)
- Hierarchical structure
- Navigate within file

**Workspace Symbols (workspace/symbol):**
- Global symbol search
- Fuzzy matching
- Aggregates across every indexed document, opened or not

### Implementation

**Architecture:**
- `server.go` — Server lifecycle, stdio transport
- `base.go` — Stub handlers for unimplemented LSP methods
- `handler.go` — Custom didChange with pointer-valued Range (full vs incremental edits)
- `sync.go` — Document synchronization (didOpen/didChange/didClose/didSave)
- `files.go` — Folder scan and watched-file events (the on-disk half of the workspace)
- `lifecycle.go` — Initialize capabilities advertisement
- `diagnostics.go` — Error publishing
- `hover.go`, `completion.go`, `definition.go`, `references.go`, `symbols.go` — Feature implementations
- `posmap.go` — UTF-8 offset ↔ LSP line/character conversion
- `semantictokens.go`, `codeaction.go` — Semantic tokens and quick fixes
- `walk.go` — Reference lookup over `resolve.References`

**Testing:**
- Tests covering every feature — count in [the measured counts](../project/spec-compliance.md)
- Integration tests with mock clients
- Incremental sync edge cases (astral plane characters, multi-change, offset-zero insertion)

**Usage:**
```bash
go build -o sysml-lsp ./cmd/sysml-lsp
./sysml-lsp  # stdio mode for editors
```

**Editor Setup:**
- VS Code: Generic LSP Client extension + workspace settings
- Neovim: nvim-lspconfig custom server
- Emacs: lsp-mode manual server registration

See [the guide](../guide/) for VS Code configuration.

---

## REPL Integration

**Package:** `internal/repl`  
**Binary:** `cmd/sysml`

### Commands

**Document management:**
- `%help` — Show help
- `%list` — List current session declarations
- `%clear` — Reset session
- `%load <file>` — Load .sysml file

**Runtime execution:**
- `%instantiate <name>` — Create instance from part def
- `%eval <expr>` — Evaluate expression (feature refs + literals)
- `%features <object> [all|depth <n>] [json]` — Show an object's features and their values, bounded unless asked for whole or to a depth; `json` writes the graph in the API's `InstantiateResponse` shape
- `%instances` — List all created instances

**Behavioral execution:**
- `%calc <name> [args...]` — Invoke calculation with literal arguments (e.g., `%calc add 10 20`)
- `%constraint <name>` — Evaluate constraint, check assert/assume satisfaction
- `%requirement <name>` — Evaluate requirement, validate subject/require/actor conditions
- `%satisfy [name]` — Evaluate satisfaction assertions, with the requirement's subject bound to the object `by` names
- `%schedule [policy]` — Show or set the policy the next run resolves its choice points under (`reverse`, `declared`, `seed:<n>`, `replay:<file>`); `explore` is refused, since a debugging session steps one run

**Action debugging:**
- `%action <name> [<object>]` — Start debugging action execution, optionally performed by an instantiated object
- `%step` — Advance all tokens one step
- `%continue` — Run action to completion, or to the first breakpoint hit
- `%tokens` — Show active tokens with location + data
- `%break <nodeName>` — Set breakpoint on node; `%continue` stops when a token reaches it

**State machine debugging:**
- `%state <name> [<object>]` — Start debugging state machine, optionally performed by an instantiated object
- `%events` — Show event queue length
- `%current` — Show current state, stack, stateData, time
- `%advance <time>` — Advance simulation time by `<time>` units, processing every event due
- `%stop` — Stop debugging session

### Implementation

- **Session:** Manages document + runtime context + instances + debugging sessions
- **getOrCreateRuntime():** Lazy init, builds index from current document
- **Runtime commands wire to:** 
  - `runtime.Context.Instantiate()`, `runtime.Context.Eval()`, `runtime.Context.InvokeCalc()`
  - `runtime.Context.EvaluateConstraint()`, `runtime.Context.EvaluateRequirement()`
  - `runtime.Context.ExecuteAction()`, `runtime.Context.ExecuteState()`
  - `runtime.Context.CreateActionExecutor()`, `runtime.Context.CreateStateExecutor()`
- **Argument parsing:** `%calc` parses literal args via wrapper parsing (`part { attribute arg = <expr>; }`) + Membership unwrapping
- **Debugging sessions:** Session tracks active ActionExecutor/StateExecutor for step-by-step control

---

## Technology Choices

### Why Go?

- **Goroutines:** Concurrent reindex/query handling
- **Single binary:** Cross-platform, no JVM/runtime dependencies
- **LSP track record:** gopls demonstrates Go's suitability for language servers
- **Performance:** Fast compilation, efficient memory model

### Why Hand-Written Parser?

**Alternatives rejected:** ANTLR4-Go, goyacc, JNI/gRPC bridge to pilot

**Rationale:**
- Zero runtime overhead
- Full control over error recovery
- Sub-millisecond parses (keystroke-latency diagnostics)
- **Trade-off accepted:** Manual grammar translation from Xtext

### Incremental & Lazy Analysis

**Precedents:** gopls, rust-analyzer

- Parse immediately (syntax errors visible instantly)
- Defer name resolution / type checking until requested
- Memoize all semantic queries
- **Result:** Interactive performance even on large workspaces

---

## Development Status

| Component | Status |
|-----------|--------|
| Lexer/Parser (structural + behavioral) | ✅ Operational (98/98 stdlib clean - see [conformance gate](../../internal/core/libs/stdlib_conformance_test.go)) |
| Symbol resolution & type system | ✅ Complete |
| Validation passes (syntax → constraints) | ✅ Complete |
| Expression evaluator & instance model (Tiers 1-3) | ✅ Complete |
| Workspace/reindex/file watching | ✅ Complete |
| Behavioral parser (all behavioral bodies) | ✅ Complete |
| Calc invocation & constraint evaluation | ✅ Complete |
| Action execution engine (Tier 5) | ✅ Complete |
| State machine runtime (Tier 5) | ✅ Complete |
| REPL debugging commands | ✅ Complete |
| REPL implementation | ✅ Complete |
| Standard library bundling | ✅ Complete |
| LSP server implementation | ✅ Complete |

**Parser coverage:** 98/98 bundled library files parse cleanly — the 94 official SysML v2 standard library files and four non-normative OpenSysML extensions: `OpenSysML Libraries/OpenSysMLMathFunctions.kerml`, `OpenSysML Libraries/DocumentQueries.sysml`, `OpenSysML Libraries/IdentityMetadata.sysml` and `OpenSysML Libraries/OOSEM.sysml`. Conformance verified by [stdlib_conformance_test.go](../../internal/core/libs/stdlib_conformance_test.go). Grammar reference available at [OMG Xtext grammar](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/master/org.omg.kerml.xtext/src/org/omg/kerml/xtext).

---

## Testing Strategy

### Parser Test Contract

New grammar features require a **four-layer test contract** to ensure correctness and prevent regressions:

#### 1. Conformance Gate
- **Purpose:** Ensure stdlib continues to parse cleanly
- **Location:** `internal/core/libs/stdlib_conformance_test.go`
- **Test:** `TestStdlibConformance` loads all 96 bundled library files
- **Acceptance:** 98/98 files parse without errors
- **Allowlist:** `testdata/stdlib_known_failures.txt` (currently empty)
- **Failure mode:** Regression breaks previously-working stdlib files

**Usage:**
```bash
go test -v -run TestStdlibConformance ./internal/core/libs
```

#### 2. Golden AST Snapshots
- **Purpose:** Verify AST structure matches expected output
- **Location:** `internal/core/parser/golden_test.go`
- **Fixtures:** `testdata/parse/*.sysml` and `*.kerml` (one representative file per construct)
- **Goldens:** `testdata/parse/*.golden` (AST dumps)
- **Acceptance:** Parse output matches golden file
- **Update flag:** `go test -run TestGolden -update` (regenerate goldens after intentional changes)

**Coverage:**
- Package/namespace declarations
- Part/attribute definitions and usages
- Connections and relationships
- Requirements and constraints
- State machines
- Calculations
- Enumerations
- Imports and aliases
- Metadata annotations

#### 3. Round-Trip Serialization
- **Status:** Explicitly deferred (no faithful SysML printer exists)
- **Rationale:** `ast.Dump()` is debug-only, not spec-compliant
- **Future work:** If SysML printer added, verify `parse(print(parse(input))) == parse(input)`

#### 4. Negative Test Suite
- **Purpose:** Verify parser rejects malformed input gracefully
- **Location:** `internal/core/parser/negative_test.go`
- **Test:** `TestNegative`, one subtest per malformed input
- **Acceptance:** each case produces diagnostics rather than panicking
- **Coverage:** Unclosed blocks, unexpected tokens, invalid syntax, incomplete behavioral members

**Example:**
```go
{
    name: "unclosed_package",
    input: "package Foo {",
    wantError: true,
},
```

---

### Behavioral Test Contract

New behavioral features (actions, states, calc, constraints, requirements) require a **four-layer test contract** to ensure execution correctness:

#### 1. Golden AST Fixtures
- **Purpose:** Lock in parse structure before execution changes
- **Location:** `internal/core/parser/testdata/parse/` (behavioral fixtures)
- **Coverage:** the behavioral fixtures (action, calc, constraint, requirement, state) among the whole set
- **Acceptance:** `TestGolden` passes, AST dumps match expectations
- **Update flag:** `go test -run TestGolden -update`

**Behavioral fixtures:**
- `action_control_flow.sysml`, `action_if_branch_body.sysml`, `action_mixed_params.sysml`, `action_send_port.sysml`
- `state.sysml`, `state_full.sysml`, `state_transition_variants.sysml`, `state_call_trigger.sysml`, `state_def_region_pseudostate.sysml`, `state_defer.sysml`, `state_fork_join.sysml`, `state_history.sysml`, `state_timed_triggers.sysml`
- `calc.sysml`, `calc_defaults_and_invocation.sysml`, `calc_return.sysml`, `calc_return_parameter.sysml`
- `constraint_assert_assume.sysml`
- `requirement.sysml`, `requirement_members.sysml`

#### 2. Execution Conformance Gate
- **Purpose:** Verify behavioral execution produces expected outcomes
- **Location:** `internal/core/runtime/conformance_test.go`
- **Test:** `TestExecutionConformance` runs `.sysml` + `.expected.json` pairs; `TestExecutionConformanceUnderPolicies` runs them again under `declared` and `seed:1`
- **Schema:** `internal/core/runtime/testdata/conformance/README.md` (outcome format for each behavioral type; `outcomes` with an `admissible` citation when the model admits several)
- **Allowlist:** `known_failures.txt` (currently empty — all cases pass)
- **Acceptance:** Expected outputs/satisfaction match actual execution results under every policy; a case listing `outcomes` is explored, and every listed outcome must be reached and no other ([design note](design/scheduling.md#the-conformance-contract))

**Coverage (by fixture prefix, all passing; counts in [the measured counts](../project/spec-compliance.md)):**
- Calc: parameter binding, return values, defaults, inherited parameters, unary operators, type coercion, qualified names, body-local usages, statement bodies, nested and from-constraint invocation
- Action: token flow, outputs, nested invocation, send/accept, port communication, `perform` reference and shorthand, accept...then, flows, loops and decisions
- State: simple, do behavior, concurrent do, transition effect, choice/junction/fork-join pseudostates, orthogonal regions and region pseudostates, shallow/deep history, deferred/undeferred events, call and timed triggers, signal discrimination/unmatched, self signal
- Requirement: require/subject/actor/assume satisfaction, nested
- Instance: derived feature values, constraint binding, inherited constraints, nested usage bodies
- Unit and quantity evaluation
- Constraint: assert/assume satisfaction, negation
- Satisfy assertions, variations, redefinitions, variants, feature chains, string operations, nested behaviors, element filters, the ball-and-chain model, and one each of attribute, connector, cubesat and view

**Usage:**
```bash
go test -v -run TestExecutionConformance ./internal/core/runtime
```

#### 3. Golden Execution Traces
- **Purpose:** verify *how* execution proceeds (ordering, scheduling), not only the final result
- **Location:** `internal/core/runtime/trace_test.go`
- **Test:** `TestExecutionTrace` compares executor traces against `.trace.golden`, and against the partial order a `.trace.order` states (`a < b` per line)
- **Determinism:** Token sorting by ID, fixed event queue tie-breaking; each `choice` the run made is a trace line, so a golden pins one linearization and a case with `outcomes` owns one golden per sweep policy (`<case>.<policy>.trace.golden`)
- **Acceptance:** Trace output matches golden file
- **Update flag:** `go test -run TestExecutionTrace -update-traces`
- **Coverage:** `.trace.golden` files for action, calc, state, constraint, accept and string execution

**Trace format:**
- Action: `step N: token T1@node1, token T2@node2` (sorted)
- State: `entry: StateName [hasEntryAction]`, `transition: From -> To [event]`, `exit: StateName [hasExitAction]`

#### 4. Runtime Robustness Tests
- **Purpose:** Verify malformed/pathological behaviors fail gracefully (typed errors, no panics/hangs)
- **Location:** `internal/core/runtime/robustness_test.go`
- **Test:** `TestRuntimeRobustness`, one subtest per failure mode
- **Acceptance:** All return typed errors, never panic, timeout guard (60s) prevents hangs

**Failure modes:**
- Deadlocked action (join starvation)
- Decision with no satisfied guard
- State machine with dangling transition
- Sourceless accept...then written first in its body or after a member that is not a state
- Calc with unbound parameter, surplus or unknown-named arguments, no result, non-calc target, direct or mutual recursion
- Constraint referencing missing feature
- Step budget exceeded
- Fork/join misuse (branches sharing a region, join with one incoming branch)
- Region pseudostate with no satisfied guard, or a cycle
- Non-numeric time trigger
- Send that reaches only its addressee, accept of an unsent type, send through an unconnected port
- History outside a composite state, or without a record or default
- Defer of a non-deferrable trigger
- Non-terminating do behavior
- Call of an unhandled operation, call argument of the wrong type
- `perform` of a missing action, `perform` reference cycle

**Usage:**
```bash
go test -v -run TestRuntimeRobustness -timeout 60s ./internal/core/runtime
```

---

### Declared errata (`internal/errata`)

The three pilot oracles read OMG-published material, which is sometimes wrong itself.
`internal/errata` is the registry of those defects: file, line, published bytes, the specification
clause violated, the derivation, and the corrected text where the intended reading is unambiguous.
The published corpus is never written to — corrections are applied to a copy under the oracle's
output directory — and an entry whose published text no longer matches the bytes on disk fails a
test rather than rotting. Each oracle reports both censuses; the as-published one stays the
conformance statement. See [the declared errata overlay](../project/errata-overlay.md).

---

### Behavioral Semantics Map

**See:** [SPEC_COMPLIANCE.md](../project/spec-compliance.md) for semantic rule → implementation → test case → status mapping.

Every behavioral feature must have:
- Semantic rule reference: the SysML v2 metamodel or the bundled KerML semantic library, and UML 2.5.1 only where neither has the concept
- Implementation location (file:function)
- Test case(s) exercising the feature
- Status: ✅ Faithful / ⚠️ Approximate / ❌ Not Yet Implemented / ⛔ Deliberate Divergence / 🚧 Known Failure

<!-- doc-counts:begin refereed-figures -->
**Measured against the pinned reference** (`PILOT_TAG=2026-08`, artifact `0.62.0`). Every number below is generated by `make docs-counts` from the committed baselines and gated; none of them is typed in by hand.

- **Corpus agreement:** 344 of 375 files agree diagnostic-by-diagnostic; 38 diagnostics are ours alone and 1113 the reference's alone, and the first number must be read by root: our diagnostics against the reference's own corpora fell while our non-standard-notation warnings on our own example models rose ([differential](../project/pilot-differential.md), `go run ./cmd/pilot-diff`).
- **Declared-diagnostic silence:** of the 512 declared `errors` rows in the reference's own Xpect suites, we report nothing for 0. 245 we report word-for-word; 248 wording-only and 7 location-only differences are agreement in substance and are not counted as gaps; 0 more we report as a warning and 2 elsewhere in the file ([Xpect oracle](../project/pilot-xpect.md), `go run ./cmd/pilot-xpect`).
- **Scope agreement:** 230 of 230 declared scope assertions match exactly (same source).
- **Permissiveness gaps:** of 285 invalid models we wrote ourselves, the reference rejects 3 that we accept by default, and 273 both reject; 3 further cases agree only when we are asked strictly. We authored every one of these cases ourselves, so the denominator measures the reach of our own corpus and not our conformance; agreement reached only under an opt-in strict mode is weaker evidence than agreement by default ([rejection oracle](../project/pilot-rejection.md), `go run ./cmd/pilot-reject`).
- **Declared errata:** the registry declares 3 defect(s) in the published reference material — 1 with a specification-derived correction, 2 documented without one, since no intended reading can be inferred ([OMG issues](../project/omg-issues.md), `internal/errata`). Every figure above is as published and stays the conformance statement; running the same oracles over the corrected text instead reports 345 of 375 files agreeing, 37 diagnostics ours alone and 1113 the reference's alone, 0 declared rows we are silent on, and 0 of 285 authored cases the reference alone rejects. The corrected figures are diagnostic only: an erratum never reclassifies a divergence category, and the published corpus is never edited.
- **Self-assessed surface:** the action, state-machine and classifier-behavior rows have no external referee at all — the four refereed figures above cannot see them, because the pinned artifact evaluates expressions but executes neither actions nor state machines. [Spec compliance](../project/spec-compliance.md) counts them.

What these numbers cannot show: the OMG corpora are demonstrations rather than an official conformance suite; the differential is one-directional, comparing the diagnostics the two implementations report on the same files; the Xpect suites are the pilot authors' test intent rather than a certification oracle; and none of these is a percentage of the specification — no global compliance figure is claimed anywhere.

**Row bookkeeping:** the ✅/⚠️/❌/⛔ status of each tracked rule stays in [spec compliance](../project/spec-compliance.md) as a census of our own row list, counted when the documentation site is built rather than committed. It moves when rows are rewritten and does not move when an oracle does, so it is not the progress measure.
<!-- doc-counts:end refereed-figures -->

Calc/constraint/requirement functional. Action/state executor infrastructure complete (fork/join/decision, TimeEvent/ChangeEvent, guards, hierarchy, orthogonal regions all tested); every conformance case passes. Fork/join, shallow/deep history and deferred events are implemented and reachable from source text — see docs/project/spec-compliance.md and docs/reference/grammar/README.md.

---

### Unit & Integration Tests
- **Unit tests:** Per-package test coverage (lexer, parser, semantics, runtime)
- **Integration tests:** End-to-end REPL/runtime scenarios
- **Test fixtures:** `testdata/*.sysml`, `testdata/*.kerml`
- **Golden files:** Expected parse/resolve/diagnostic outputs
- **Verification:** `go test ./...` (all tests pass), `go build ./...` (clean build)

### Contributing New Grammar Features

When adding parser support for new SysML v2 constructs:

1. ✅ Add representative example to `testdata/parse/*.sysml`
2. ✅ Run `go test -run TestGolden -update` to generate golden
3. ✅ Verify `TestStdlibConformance` still passes (no regressions)
4. ✅ Add negative test case if construct has error conditions

### Contributing New Behavioral Features

When adding execution support for behavioral constructs (actions, states, calc, constraints, requirements):

1. ✅ Add golden AST fixture to `internal/core/parser/testdata/parse/` (if not already covered)
2. ✅ Implement semantics in `internal/core/runtime/` (executor or evaluator)
3. ✅ Add conformance case: `.sysml` + `.expected.json` in `internal/core/runtime/testdata/conformance/`
4. ✅ Add golden trace case: `.trace.golden` for ordering-sensitive features (fork/join, transitions)
5. ✅ Add robustness test for failure modes (deadlock, unbound params, missing refs)
6. ✅ Update `docs/project/spec-compliance.md` with semantic rule → implementation → test → status
7. ✅ Verify all tests pass: `go test ./internal/core/parser/ ./internal/core/runtime/`

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for full contribution guidelines.

---

## References

- **OMG SysML v2.1 Beta 1 Spec:** [https://www.omg.org/spec/SysML/2.0](https://www.omg.org/spec/SysML/2.0) (2026-08 release)
- **Pilot Implementation:** [SysML-v2-Pilot-Implementation 2026-08](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/releases/tag/2026-08)
- **Pilot Xtext Grammar:** `SysML.xtext` + `KerMLExpressions` (OMG reference implementation)
- **Metamodel:** OMG SysML v2 metamodel (semantic foundation)
- **Precedents:** gopls (Go LSP), rust-analyzer (Rust LSP), IPython/Jupyter (REPL design)
