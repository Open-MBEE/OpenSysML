# API Documentation

Complete API reference for OpenSysML packages.

## The public Go API

`github.com/Open-MBEE/OpenSysML/client/opensysml` is what a Go program outside this repository
imports: parse, symbol lookup, evaluation and instantiation, answered in process by the engine the
importing binary already links, or over Connect against a service someone else runs. It is the
only package covered by a compatibility commitment; everything under `internal/` below is
documented for contributors and may change without notice.

```sh
go get github.com/Open-MBEE/OpenSysML@latest
```

```go
client, err := opensysml.New()          // in process; Dial(address) for a remote service
defer client.Close()

model, err := client.ParseFile(ctx, "vehicle.sysml")
mass, err := client.Evaluate(ctx, model, "mass", opensysml.WithSubject("Demo::sedan"))
```

Nothing else is installed — the standard library is embedded in the module and the v1 operations
never shell out — and a `Client` is safe to share across goroutines, so one per process is the
intended shape. `ParseFile` and `ParseSource` each parse one document; `ParseFiles` and
`ParseDocuments` parse several as one model, each keeping its own name, so an import between them
resolves and a diagnostic locates itself in the file it came from.

`ExecuteAction`, `ExecuteState` and `RunAnalysis` answer one run, under the scheduling policy
`WithSchedule`/`Schedule` names (`declared`, `reverse`, `seed:<n>`). `ExploreAction`,
`ExploreState` and `ExploreAnalysis` answer every run: they take the `explore` policy — the
default when none is given, or `explore:runs=N,depth=D` to set its budget — and report an
`Exploration`, one `Outcome` per distinct result with the number of linearizations that reached
it and one run's choices as its `Witness`, plus whether the search was `Complete` or which
`BudgetsHit` ended it (`Status()` renders it as the `sysml` command does). A run that fails under
some order is an `Outcome` whose `Error` is set, not a failure of the call. The two families
refuse each other's policies with `CodeInvalidArgument`, and exploring requires the
`schedule_explore` capability alongside `schedule`.

```go
exploration, err := client.ExploreAction(ctx, model, "Demo::race", nil)
for _, outcome := range exploration.Outcomes {
	fmt.Println(outcome.Outputs["winner"], outcome.Linearizations, outcome.Witness)
}
fmt.Println(exploration.Status()) // complete (6 runs)
```

Its errors, ownership rules, capability negotiation and v1 boundary are in
[client/opensysml/README.md](../../client/opensysml/README.md), and the other client languages are on
[client libraries](clients.md). A program with no client library that posts JSON to the service
by hand decodes the answers by [the wire contract](wire-contract.md).

## Python authoring

`Editor.add_member(owner, kind, name, type=None, multiplicity=None, value=None,
specializes=None)` and its typed `add_*` helpers create declarations while
preserving untouched source bytes. `Editor.delete(target, cascade=False)`
removes declarations transactionally. `opensysml.loads(content, language=None,
strict=False)` loads inline SysML or KerML for this workflow.

## Overview

OpenSysML is organized into core packages under `internal/core/`, with frontends in `internal/lsp/` and `internal/repl/`.

**Package Organization:**

```
github.com/Open-MBEE/OpenSysML
├── internal/core/          # Core language implementation
│   ├── source/             # Source files and position tracking
│   ├── lexer/              # Tokenization
│   ├── parser/             # Parsing to AST
│   ├── ast/                # Abstract Syntax Tree
│   ├── symbols/            # Symbol tables and scopes
│   ├── resolve/            # Name resolution
│   ├── semantics/          # Type system and semantic queries
│   ├── passes/             # Validation passes
│   ├── lower/              # AST → execution IR (ActionGraph/StateGraph)
│   ├── runtime/            # Execution runtime
│   ├── model/              # Workspace and document management
│   └── libs/               # Standard library handling
├── internal/lsp/           # Language Server Protocol
├── internal/grpc/          # gRPC service implementation
└── internal/repl/          # Interactive REPL
```

---

## Core Packages

### `internal/core/source`

Source file management and position tracking.

**Key Types:**

- **`SourceFile`** — Represents a source file with content and line indexing
  - `Name() string` — File path
  - `Content() []byte` — Raw bytes
  - `LineCount() int` — Number of lines
  - `Line(n int) string` — Get line content

- **`Span`** — Position range in source (offset-based)
  - `Start, End int` — Byte offsets
  - `Contains(pos int) bool`
  - `Overlaps(other Span) bool`

**Usage:**
```go
src := source.New("example.sysml", []byte("part Wheel;"))
span := source.Span{Start: 0, End: 4} // "part"
```

---

### `internal/core/lexer`

Tokenization of SysML v2 textual notation.

**Key Types:**

- **`Lexer`** — Scanner for SysML v2 tokens
  - `Next() Token` — Get next token
  - `Peek() Token` — Look ahead without consuming

- **`Token`** — Single token with position
  - `Kind TokenKind` — Token type (keyword, identifier, literal, operator)
  - `Span source.Span` — Position in source
  - `Text string` — Raw text

- **`TokenKind`** — Enum of all token types
  - Keywords: `KwPackage`, `KwPart`, `KwAttribute`, `KwAction`, etc. (~200 keywords)
  - Literals: `LitInteger`, `LitReal`, `LitString`, `LitBool`
  - Operators: `OpPlus`, `OpMinus`, `OpEq`, etc.
  - Structure: `LBrace`, `RBrace`, `Semicolon`, `Comma`, etc.

**Usage:**
```go
lex := lexer.New(source.New("test", []byte("part Wheel { }")))
for tok := lex.Next(); tok.Kind != lexer.EOF; tok = lex.Next() {
    fmt.Println(tok.Kind, tok.Text)
}
```

---

### `internal/core/parser`

Recursive-descent parser producing AST.

**Entry Points:**

- **`New(src *source.SourceFile) *Parser`** — Create parser
- **`(*Parser).ParseFile() *ast.RootNamespace`** — Parse complete file

**Key Functions:**

- `parseDefinition()` — Parse def (part, attribute, action, etc.)
- `parseUsage()` — Parse usage
- `parseExpression()` — Parse expressions
- `parseActionBody()` — Parse behavioral action body (Phase C3)

**Error Recovery:**

Parser always produces a complete tree. Errors result in `ast.ErrorNode` placeholders.

**Usage:**
```go
p := parser.New(source.New("test.sysml", content))
root := p.ParseFile()
// root is always non-nil, check root.Errors for diagnostics
```

---

### `internal/core/ast`

Abstract Syntax Tree nodes (syntax-only, immutable).

**Node Interface:**

All AST nodes implement:
```go
type Node interface {
    Span() source.Span
    LeadingTrivia() []Trivia
    TrailingTrivia() []Trivia
}
```

**Key Types:**

**Namespace & Elements:**
- `RootNamespace` — Top-level (one per file)
- `Package` — Package declaration
- `Import` — Import statement

**Definitions & Usages:**
- `Definition` — Base for all defs (part, attribute, action, etc.)
  - `.Kind DefinitionKind` — Type of definition
  - `.Ident *Identifier` — Name
  - `.Members []Node` — Body contents
  - `.Specializations []*QualifiedName` — Generalization edges
  - `.Visibility VisibilityKind`
  
- `Usage` — Base for all usages
  - `.Kind UsageKind`
  - `.Ident *Identifier`
  - `.Members []Node`
  - `.Multiplicity *MultiplicityExpr`
  - `.Value Node` — Default value expression

**Expressions:**
- `LiteralInteger`, `LiteralReal`, `LiteralBool`, `LiteralString`
- `FeatureReference` — Reference to feature by name
- `FeatureChainExpr` — Dot notation (`x.y.z`)
- `UnaryExpr`, `BinaryExpr`, `ConditionalExpr`
- `InvocationExpr` — Function/calc invocation
- `SequenceExpr`, `CollectExpr`, `SelectExpr` — Collection operations

**Behavioral Nodes (Phase C3):**
- `InitialNode`, `FinalNode` — Control flow start/end
- `ForkNode`, `JoinNode` — Concurrency
- `MergeNode`, `DecisionNode` — Branching
- `ActionExecutionNode` — Action invocation
- `SuccessionEdge` — Flow between nodes
  - `.Source, .Target *QualifiedName`
  - `.Guard Node` — Optional guard expression

**Architecture Rule:** AST is immutable after parsing. All semantic information lives in side tables.

---

### `internal/core/symbols`

Symbol tables and scope trees.

**Key Types:**

- **`Symbol`** — Represents a declared name
  - `Name string` — Identifier
  - `Kind SymbolKind` — Type of symbol
  - `Decl ast.Node` — AST node that declares it
  - `Scope *Scope` — Child scope (if compound)
  - `OwnerScope *Scope` — Parent scope
  - `Visibility ast.VisibilityKind`

- **`Scope`** — Lexical scope
  - `Parent() *Scope`
  - `Node() ast.Node`
  - `Children() []*Scope`
  - `LookupLocal(name string) (*Symbol, bool)` — Local lookup only
  - `LookupLocalAll(name string) []*Symbol` — All with that name (short+primary)
  - `MemberNames() []string` — All declared names in order

- **`Index`** — Global symbol index
  - `DocumentRoot(name string) *Scope` — Get document root scope

**Usage:**
```go
idx := symbols.NewIndex()
idx.AddDocument("example.sysml", root) // root is *ast.RootNamespace
scope := idx.DocumentRoot("example.sysml")
sym, ok := scope.LookupLocal("Wheel")
```

**Important:** Short name + primary name alias the same `*Symbol`. Dedupe by pointer when iterating.

---

### `internal/core/resolve`

Name resolution (lazy, memoized).

**Key Types:**

- **`Resolver`** — Name resolver
  - `ResolveQualified(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool)`
  - `ResolveUnqualified(scope *symbols.Scope, name string) (*symbols.Symbol, bool)`
  - `ResolveImport(importNode *ast.Import) []*symbols.Symbol`

**Usage:**
```go
res := resolve.New(index)
sym, ok := res.ResolveQualified(scope, qname)
```

Results are memoized internally.

---

### `internal/core/semantics`

Type system, conformance, semantic queries.

**Key Type: `Model`**

Central semantic query engine. Built from resolver:
```go
model := semantics.NewModel(resolver)
```

**Methods:**

**Type relationships:**
- `DirectSupertypes(sym *symbols.Symbol) []*symbols.Symbol` — Immediate generalizations
- `AllSupertypes(sym *symbols.Symbol) []*symbols.Symbol` — Transitive closure
- `Conforms(a, b *symbols.Symbol) bool` — Type conformance check
- `HasSpecializationCycle(sym *symbols.Symbol) bool` — Cycle detection

**Members:**
- `MembersOf(sym *symbols.Symbol) []*symbols.Symbol` — All members (local + inherited with masking)
- `LookupMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool)`

**Multiplicity:**
- `MultiplicityOf(sym *symbols.Symbol) (Range, bool)` — Extract multiplicity bounds
  - `Range{Lower, Upper Bound}`
  - `Bound{Value int64, Infinite bool, Known bool}`

**Constant evaluation:**
- `Eval(n ast.Node) (Value, bool)` — Constant-folder for literals and operators
  - `Value{Kind ValueKind, Int int64, Real float64, Bool bool}`
  - `ValueKind` ∈ {ValInt, ValReal, ValBool, ValInfinity, ValInvalid}

**Note:** `Eval()` is a **constant-folder only**. For full runtime evaluation, see `internal/core/runtime`.

---

### `internal/core/passes`

Pluggable validation passes.

**Architecture:**

Validation runs in tiers:
1. **Syntax** — Checks for ErrorNodes
2. **Name Resolution** — Validates all names resolve
3. **Type Checking** — Type conformance
4. **Constraints** — Deep semantic rules

Higher tiers skip if lower tier fails.

**Key Types:**

- **`Pass`** — Validation pass interface
  - `Level() PassLevel` — Which tier
  - `Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic`

- **`Context`** — Provides access to resolver and model
  - `Resolver() *resolve.Resolver`
  - `Model() *semantics.Model`

- **`Diagnostic`** — Error/warning
  - `Severity Severity` — Error, Warning, Info
  - `Span source.Span`
  - `Message string`
  - `Code string` — stable identifier of what was found (`syntax`, a pass or rule code,
    `choice-point`, `guard-unevaluable`); carried as-is on the wire as `Diagnostic.code`

**Usage:**
```go
// Analyze runs the default pass registry internally.
diagnostics := passes.Analyze("example.sysml", root, parseDiags, idx)
```

---

### `internal/core/runtime`

Execution runtime (Tiers 1-5: instances, expressions, behaviors).

**Key Types:**

**Value System:**

- **`Value`** — Runtime value
  - `Kind ValueKind` — Type tag
  - `Const semantics.Value` — Integer/real/bool (for ValConst)
  - `Str string`, `Instance int64`, `Sequence *Sequence`, `Set *Set`

- **`ValueKind`** — Enum
  - `ValConst` — Integer/real/bool (stored in Const field)
  - `ValNull`, `ValString`, `ValInstance`, `ValSequence`, `ValSet`

**Instance Model:**

- **`Instance`** — Runtime instance
  - `ID int64` — Unique instance ID
  - `Type *symbols.Symbol` — Type symbol
  - `FeatureValues map[string]*FeatureValue` — Feature values by feature name

**Execution Context:**

- **`Context`** — Runtime execution context
  - `Instantiate(sym *symbols.Symbol) (*Instance, error)` — Create instance
  - `Eval(expr ast.Node, env map[*symbols.Symbol]Value) (Value, error)` — Evaluate expression
  - `InvokeCalc(sym *symbols.Symbol, args []Value, scope *symbols.Scope) (Value, error)` — Invoke calculation
  - `EvaluateConstraint(sym *symbols.Symbol, scope *symbols.Scope) (bool, error)` — Evaluate constraint
  - `EvaluateRequirement(sym *symbols.Symbol, scope *symbols.Scope) (bool, error)` — Evaluate requirement
  - **`ExecuteAction(sym *symbols.Symbol) (map[string]Value, error)`** — Execute action to completion
  - **`ExecuteState(sym *symbols.Symbol) (map[string]Value, error)`** — Execute state machine until final/suspended
  - **`CreateActionExecutor(sym *symbols.Symbol) (*ActionExecutor, error)`** — Create action executor for debugging
  - **`CreateStateExecutor(sym *symbols.Symbol) (*StateExecutor, error)`** — Create state executor for debugging
  - `SetSchedule(policy SchedulePolicy) error` — Set the scheduling policy the runs started from
    now on resolve their choice points under; a run already under way keeps the one it started
    with. `explore` is refused with `ErrExploreUndriven`: an exploration replays whole runs over
    fresh contexts, so `Explore` drives it rather than one context running under it
  - `Schedule() SchedulePolicy` — The policy the next run resolves its choice points under
  - `Clock() *Clock` — The simulation clock every executor of the context reads and waits on:
    `Now()` its current instant in `SI::s`, `Waits()` every state timer and action `accept
    after`/`accept at` registered on it in due order, `NextDue()` the earliest
  - **`Advance(duration float64) (AdvanceReport, error)`** — Move the clock by `duration` seconds,
    running every state event, action token, change-condition poll and do round that comes due
    on the way, instant by instant, within the run budgets; executors due at one instant are
    ordered by the scheduling policy and recorded as a `ChoiceDueOrder` choice point. A negative
    or NaN duration is `ErrNegativeDuration`; zero dispatches what is already due. The report
    counts the events, do steps and action steps run, the signals dropped and the notes recorded
  - `StateOutcomeWithEvents(sym *symbols.Symbol, events []string) (Outcome, error)` — Run a
    state machine on the events and answer its `Outcome`: the state it rests in, the states it
    entered and the values it holds

- **`SchedulePolicy`** — How the executors resolve a run's choice points (several steppable
  tokens in one step, several holding guards at a decision, several enabled transitions for one
  event, several regions reacting to one event, several executors due at one instant of the
  clock; which same-step write to one feature
  stands follows from the token order). The zero
  value and `DefaultSchedulePolicy` are `reverse`, what every run did before policies were
  selectable; every `.expected.json` and `.trace.golden` recorded under it still holds
  - `ParseSchedulePolicy(spelling string) (SchedulePolicy, error)` — Read `declared`, `reverse`,
    `seed:<n>` (n a non-negative decimal integer) or `explore[:runs=<n>,depth=<d>]` (n at least 1,
    d at least 0, in either order, each at most once; `DefaultExploreBudget`, 1024 runs and 64
    choice points deep, for one not given); the empty spelling is the default. Any other
    spelling — an unknown name, `seed` or `seed:` without a number, `seed:-1`, `seed:abc`,
    `explore:` with nothing after the colon, `explore:runs=0` — is a `*SchedulePolicyError`
    (`Spelling`, `Reason`) matching `ErrInvalidSchedulePolicy` under `errors.Is`, so every surface
    refuses it before anything runs
  - `ExplorePolicy(budget ExploreBudget) (SchedulePolicy, error)` — The `explore` policy with the
    budget given, refusing one outside the bounds above
  - `Exploration() (ExploreBudget, bool)` — The budget when the policy is `explore`
  - `String() string` — The spelling `ParseSchedulePolicy` reads the policy back from
  - `IsDefault() bool` — Whether the policy is `reverse`
  - `SchedulePolicyNames` — The accepted spellings, for usage text
  - `declared` steps tokens in the order they were spawned and takes the first holding guard
    and first enabled transition in declaration order; `seed:<n>` draws every resolution from
    a pseudo-random sequence the seed fixes, the same on every platform, so one seed replays
    one run and two seeds may take two linearizations. A policy changes which alternative each
    choice point takes, never whether it is reported: the `Taken` of each `ChoicePoint` is what
    the policy took

- **`Explore(stop context.Context, policy SchedulePolicy, fresh func() (*Context, error), run func(*Context) (Outcome, error)) (*Exploration, error)`**
  — Run a behavior under `explore` once per linearization within the budget: each run starts
  from the `Context` `fresh` builds over the model and lowering they all share, records the
  alternative taken at every choice point, and the next run replays that prefix up to its
  frontier and takes the first untried alternative there, depth-first over the tree of choice
  sequences. `run` performs one run and answers its `Outcome`; an error it returns is the
  `Outcome.Err` of an outcome of its own, so a run some orders fail is reported rather than
  ending the search. A `stop` that ends between runs ends the exploration with its error before
  the next context is built. A policy other than `explore` is `ErrNotExploring`; a replay that
  does not meet the choice points its prefix recorded is `ErrExplorationDiverged`, since the
  model's runs are then not a function of their choices
  - **`Outcome`** — What one run came to, in the observables a conformance case compares:
    `Outputs`, and for a state machine `FinalState` and `StateVisits`; `Err` for a run that
    failed. `String()` renders it in a fixed order; `Explore` tells outcomes apart by a stricter
    identity that quotes every name, so two runs agreeing on their observables are one outcome
    and no output name can spell another outcome's rendering. An object a run holds is spelled
    by its type and feature values, not by the id the run gave it, so two runs building the same
    object are one outcome whatever their ids.
    `(*Context).ActionOutcome(outputs)`, `(*StateExecutor).Outcome()`,
    `(*Context).AnalysisOutcome(result)` and `(*Context).VerifiedOutcome(result, verdicts)` build
    one over the run's context; a case's verdicts are values named `objective <name>`,
    `assertion <name>` and `verdict <case>`
  - **`Exploration`** — `Budget`, `Runs`, the distinct `Outcomes` in canonical order and
    `BudgetsHit`, `runs` before `depth`, empty when `Complete()`. `Status()` renders
    `complete (N runs)` or `incomplete: <budget> budget <limit> hit after N runs`
  - **`ExploredOutcome`** — One `Outcome` with the `Linearizations` that reached it, the
    `Witness` (one run's `ChoiceTaken` sequence, `FormatChoices` renders it) and `WitnessRun`
  - **`ChoiceTaken`** — One resolved choice point: its `Kind`, `Step`, `Where`, the
    `Alternatives` and `Among` it had and the `Taken`/`Took` it resolved to

**Behavioral Execution (Tier 5):**

- **`Token`** — Control token for action execution, carrying no values of its own
  - `ID int64` — Unique token ID
  - `Location ast.Node` — Current node (InitialNode, ActionExecutionNode, etc.)

- **`ActionExecutor`** — Petri-net token-flow execution engine
  - `Step() error` — Advance all tokens one step; a breakpoint met inside a token's body
    ends the step there, with no other token stepped, and the next step steps the other
    tokens before resuming the paused one
  - `RunToCompletion() error` — Execute until StateCompleted (max 10k steps)
  - `Tokens() []Token` — Get active tokens (copy)
  - `State() ExecutionState` — Current execution state (Ready/Running/Completed/Suspended)
  - `Results() map[string]Value` — The values the action's features hold, and under
    `node.pin` (`p.v`, `leg.inner.v`) those the latest performance of each nested
    action node holds; complete once execution has completed
  - `Data() map[string]Value` — The live feature space of the action's own performance,
    which every token in its flow shares. A nested action node performs in a frame of
    its own, so its pins are not here; read them from `Results()`
  - `SetBreakpoint(nodeName string)` — Set breakpoint on node: a run pauses when a token
    reaches it, or before a body performs it when it is a node an `if` branch or a loop body
    declares; `NodeNames()` lists every node a breakpoint may name
  - `ClearBreakpoints()` — Clear all breakpoints
  - `Release()` — End the run for good, ending the paused work of every token a breakpoint
    left mid-body; a later `Step()` or `RunToCompletion()` returns `ErrExecutorReleased`,
    whether the run had completed or not. Call it when abandoning an executor that may be
    suspended
  - `ActionSymbol() *symbols.Symbol` — Get action symbol

- **`StateExecutor`** — Event-driven state machine execution
  - `ProcessNextEvent() error` — Process next event from queue
  - `CurrentState() ast.Node` — Get current StateNode
  - `StateStack() []*ast.StateNode` — Get active configuration (hierarchical states)
  - `StateData() map[string]Value` — Get state machine variables
  - `EventQueue() *EventQueue` — Get event queue
  - `CurrentTime() float64` — Read the context's shared simulation clock (`Context.Clock().Now()`)
  - `State() ExecutionState` — Get execution state
  - `StateMachineSymbol() *symbols.Symbol` — Get state machine symbol

- **`ExecutionState`** — Enum
  - `StateReady` — Initialized, not started
  - `StateRunning` — Executing
  - `StateCompleted` — Finished (final node/state reached)
  - `StateSuspended` — Paused (waiting for events)

- **`Event`** — State machine event
  - `ID int64` — Unique event ID
  - `Type EventType` — Time/Change/Accept/Call
  - `Timestamp float64` — Event timestamp (for TimeEvent)
  - `Payload map[string]Value` — Event data

**Built-in Functions:**

Registered KerML builtins:
- Arithmetic: `+`, `-`, `*`, `/`, `%`, `**`
- Comparison: `==`, `!=`, `<`, `>`, `<=`, `>=`
- Boolean: `and`, `or`, `xor`, `not`, `implies`
- Collections: `size`, `isEmpty`, `->select`, `->collect`
- String: `+` (concat), `size`, `substring`

**Usage:**

Tier 1-3 (Instances & Expressions):
```go
// Honour the OPENSYSML_MAX_* budgets instead of the defaults with:
//   budgets, err := runtime.BudgetsFromEnv()
//   err = ctx.SetBudgets(budgets)
ctx := runtime.NewContext(model, resolver, runtime.DefaultMaxSteps)
inst, _ := ctx.Instantiate(wheelSym)
fv, _ := inst.GetFeatureValue(ctx, "diameter")
result, _ := ctx.InvokeCalc(addSym, []Value{v1, v2}, scope)
```

Tier 5 (Actions):
```go
// Execute action to completion
results, err := ctx.ExecuteAction(myActionSym)
if err != nil { /* handle error */ }
result := results["result"]

// Or under another scheduling policy, refusing a spelling that names none
policy, err := runtime.ParseSchedulePolicy("seed:7")
if err != nil { /* *runtime.SchedulePolicyError */ }
if err := ctx.SetSchedule(policy); err != nil { /* runtime.ErrExploreUndriven for explore */ }
results, err = ctx.ExecuteAction(myActionSym) // the same run on every call

// Or every run: one fresh context per linearization over the shared model
explore, _ := runtime.ParseSchedulePolicy("explore:runs=64")
exploration, err := runtime.Explore(context.Background(), explore,
    func() (*runtime.Context, error) { return runtime.NewContext(model, resolver, runtime.DefaultMaxSteps), nil },
    func(c *runtime.Context) (runtime.Outcome, error) {
        outputs, err := c.ExecuteAction(myActionSym)
        return c.ActionOutcome(outputs), err
    })
for _, o := range exploration.Outcomes {
    fmt.Println(o.Outcome, o.Linearizations, runtime.FormatChoices(o.Witness))
}
fmt.Println(exploration.Status()) // complete (6 runs), or the budget hit

// Or debug step-by-step
exec, _ := ctx.CreateActionExecutor(myActionSym)
exec.Initialize()
for exec.State() != StateCompleted {
    exec.Step()
    tokens := exec.Tokens()
    // inspect tokens
}
```

Tier 5 (State Machines):
```go
// Execute state machine
stateData, err := ctx.ExecuteState(stateMachineSym)
if err != nil { /* handle error */ }

// Or debug with events
exec, _ := ctx.CreateStateExecutor(stateMachineSym)
exec.Initialize()
for exec.State() != StateCompleted {
    exec.ProcessNextEvent()
    fmt.Printf("State: %s, Time: %f\n", exec.CurrentState(), exec.CurrentTime())
}
```

---

### `internal/core/model`

Workspace and document management.

**Key Types:**

- **`Workspace`** — Multi-file workspace
  - `AddDocument(name string, src *source.SourceFile) *Document`
  - `GetDocument(name string) (*Document, bool)`
  - `RemoveDocument(name string)`
  - `Index() *symbols.Index` — Global symbol index
  - `Diagnostics(name string) []passes.Diagnostic`

- **`Document`** — Single source file
  - `Name() string`
  - `Source() *source.SourceFile`
  - `Root() *ast.RootNamespace`
  - `Scope() *symbols.Scope`
  - `Version() int` — Increments on update

**Usage:**
```go
ws := model.NewWorkspace()
doc := ws.AddDocument("example.sysml", src)
diagnostics := ws.Diagnostics("example.sysml")
```

---

### `internal/core/libs`

Standard library bundling and caching.

**Key Functions:**

- `Load(name string) (*source.SourceFile, error)` — Load stdlib file
- `ListFiles() []string` — All stdlib files

Standard library is embedded in the binary using Go `embed.FS`.

---

## Frontend Packages

### `internal/lsp`

Language Server Protocol implementation.

**Key Type:**

- **`Server`** — LSP server
  - `Run(ctx context.Context, conn io.ReadWriteCloser) error`

**Transport:** stdio only. `sysml-lsp` also accepts `--stdio`/`-stdio` as an explicit no-op, because
standard language clients (including the bundled VS Code extension) name the transport on the command
line. Any other unknown flag is still rejected with exit status 2.

**Lifecycle (LSP 3.17):** `shutdown` is answered, after which every request other than `exit` is answered
`InvalidRequest` (`-32600`) and non-`exit` notifications are dropped. `exit` makes `Run` return and the
process terminate — status 0 after a preceding `shutdown`, 1 otherwise.

**Current Capabilities:**
- Document synchronization (open/change/close)
- Diagnostics (syntax + semantic errors)
- Hover (type info)
- Go to definition
- References
- Document symbols
- Workspace symbols
- Completion

**Usage:**
```go
ws := model.NewWorkspace()
srv := lsp.NewServer(ws)
srv.Run(ctx, stdio{}) // stdio implements io.ReadWriteCloser
```

---

### `internal/repl`

Interactive REPL implementation.

**Key Types:**

- **`Session`** — REPL session state
  - Accumulates declarations across inputs
  - Tracks runtime context and instances

**Entry Point:**

- `Loop(reader LineReader, out io.Writer, session *Session) error`

**LineReader Interface:**
```go
type LineReader interface {
    ReadLine(prompt string) (string, error)
}
```

**Meta Commands:**
- `%help`, `%list`, `%clear`, `%load <file>`
- `%search <substring>` — List the declared and library symbols whose qualified name contains the substring, with the kind of each
- `%builtins` — List the library functions the runtime implements directly, each with the package an import must name for its bare name to resolve
- `%instantiate <name>`, `%features <name>`, `%instances`
- `%eval <expr>`
- `%calc <name> [args...]` — Invoke calculation with arguments
- `%constraint <name>` — Evaluate constraint
- `%requirement <name>` — Evaluate requirement
- `%satisfy [name]` — Evaluate satisfaction assertions (`assert satisfy <requirement> by <part>;`), every one in the model or the ones a named element states

**Usage:**
```go
session := repl.NewSession()
repl.Loop(reader, os.Stdout, session)
```

---

## SysML v2 API & Services `Query`

This section describes the structured API Query surface. OpenSysML also accepts
OSLC Query text for element identification; see [OSLC Query text](oslc-query.md).
The two surfaces intentionally differ: structured queries support `or`, while
OSLC compound terms support only `and`, so neither surface subsumes the other.

The gRPC service implements the query surface the **SysML v2 API & Services**
standard defines, so a client that speaks that API — the
[`SysML-v2-API-Java-Client`](https://github.com/Systems-Modeling/SysML-v2-API-Java-Client),
the SysML v2 API Cookbook notebooks, MATLAB System Composer's `executeQuery` —
can filter a model OpenSysML parsed. The standard's schema is authoritative:
`api/openapi.yaml` in the Java client, components `Query`, `Constraint`,
`PrimitiveConstraint`, `CompositeConstraint`.

**Implementation:** `internal/grpc/query.go` (`Service.Query`), reported from
`GetServerInfo` as the `query` capability. Python: `model.query(...)`
(`clients/python/opensysml/query.py`). The JSON a hand-written client sends and
receives for this call is shown, captured, on
[the wire contract](wire-contract.md#query).

### The query model

```
Query          scope[]    elements to consider; empty is the whole loaded model
               select[]   properties to report; empty reports every one
               where      one Constraint; absent selects the whole scope

Constraint     PrimitiveConstraint | CompositeConstraint      (a protobuf oneof)

PrimitiveConstraint   property, operator (= > <), value[], inverse
CompositeConstraint   operator (and | or), constraint[]        (nests arbitrarily)
```

The RPC takes the same shape, with the standard's JSON names preserved, so
translation from the standard's JSON is mechanical:

```proto
rpc Query(QueryRequest) returns (QueryResponse);   // api/proto/sysml.proto
```

A cookbook payload, sent verbatim through the Python client:

```python
model = opensysml.load("examples/vehicle.sysml")
model.query({"@type": "Query", "where": {
    "@type": "PrimitiveConstraint",
    "operator": "=", "property": "@type", "value": ["PartUsage"]}})
```

Each answered element is `@id` (its qualified name), `@type` and the selected
properties it has. A property an element does not have is **absent**, not empty.

An element with no qualified identity — an unnamed `doc`, an anonymous usage, an
anonymous `connect` — is **not** answered: its qualified name has an empty
segment (`Demo::`), so it is neither unique nor a name a `scope` could use. The
standard identifies an element by `@id`, and such an element has none
(`TestQueryOmitsElementsWithNoQualifiedIdentity`).

Neither is one declared inside an action body — a branch of an `if`, a loop body —
since the body is owned by no element and so names its declarations only locally
(`step`, not `Demo::Drive::step`): that name identifies no element and could not
be used as a `scope` (`TestQueryOmitsBodyLocalDeclarations`). An answered `@id` is
always a qualified name that the model resolves back to that element.

### Queryable properties

The set is closed and is the single source of truth in
`queryProperties` (`internal/grpc/query.go`). A property outside it is an
`INVALID_ARGUMENT` error listing the ones that exist — never a silently empty
answer.

| Property | Reports | Ordered |
|---|---|---|
| `@id` | The element's qualified name, which is also how `scope` names it | |
| `qualifiedName` | Same as `@id` | |
| `@type` | The element's metamodel type (table below) | |
| `name` | The element's own name, the last segment of its qualified name | |
| `declaredName` | `name`, absent when the name is an effective name borrowed from a referenced feature | |
| `shortName` | The element's effective short name (`<'HLR-R001'>` declares `HLR-R001`); absent when it has none | |
| `declaredShortName` | `shortName`, absent when the short name is borrowed from a redefined or subsetted feature | |
| `documentation` | The body text of the element's `doc` comment, delimiters and indentation removed; absent when undocumented. This single-valued record reports the first body of an element declaring several — a document query's `Project` carries every body | |
| `owner` | Qualified name of the owning element; absent for a top-level element, whose owner is the document root | |
| `isAbstract` | `true`/`false` for a definition or usage; absent for anything else, and for a standard-library element restored from cache, which carries no declaration | |
| `type` | Qualified name of the resolved type of a typed feature; absent when untyped or unresolved | |
| `multiplicityLower` | Declared lower bound | ✅ |
| `multiplicityUpper` | Declared upper bound, `*` when unbounded | ✅ |

### `@type` — symbol kind → metamodel type

Mapping OpenSysML's symbol kinds onto the standard's metamodel type names is the
substantive design decision here; `metamodelTypeNames`
(`internal/grpc/query.go`) is the single source of truth, refined per element by
`MetamodelTypeNameOf` for the kinds one kind spans several metaclasses for, and
`TestMetamodelTypeNameCoversEveryKind` keeps it total over every kind a parsed
declaration can have. A standard-library element restored from cache may carry no
kind at all, and then reports **no** `@type`: it is answered, but never matches a
`@type =` comparison (and is kept by the inverse of one).

| OpenSysML kind | `@type` |
|---|---|
| `package`, `namespace` | `Package`, `Namespace` |
| `partDef` / `partUsage` | `PartDefinition` / `PartUsage` |
| `attributeDef` / `attributeUsage` | `AttributeDefinition` / `AttributeUsage` |
| `itemDef` / `itemUsage` | `ItemDefinition` / `ItemUsage` |
| `occurrenceDef` / `occurrenceUsage` | `OccurrenceDefinition` / `OccurrenceUsage` |
| `portDef` / `portUsage` | `PortDefinition` / `PortUsage` |
| `interfaceDef` / `interfaceUsage` | `InterfaceDefinition` / `InterfaceUsage` |
| `connectionDef` / `connectionUsage` | `ConnectionDefinition` / `ConnectionUsage` |
| `flowDef` / `flowUsage`, `allocationDef` / `allocationUsage` | `FlowDefinition` / `FlowUsage`, `AllocationDefinition` / `AllocationUsage` |
| `actionDef` / `actionUsage`, `stateDef` / `stateUsage` | `ActionDefinition` / `ActionUsage`, `StateDefinition` / `StateUsage` |
| `calcDef` / `calcUsage` | `CalculationDefinition` / `CalculationUsage` |
| `constraintDef` / `constraintUsage`, `requirementDef` / `requirementUsage` | `ConstraintDefinition` / `ConstraintUsage`, `RequirementDefinition` / `RequirementUsage` |
| `caseDef` / `caseUsage` and the analysis / verification / use-case forms | `CaseDefinition` / `CaseUsage`, `AnalysisCase…`, `VerificationCase…`, `UseCase…` |
| `viewDef`, `viewpointDef`, `renderingDef`, `concernDef` and their usages | `ViewDefinition`, `ViewpointDefinition`, `RenderingDefinition`, `ConcernDefinition` and `…Usage` |
| `enumerationDef` / `enumerationUsage`, `metadataDef` / `metadataUsage`, `metaclass` | `EnumerationDefinition` / `EnumerationUsage`, `MetadataDefinition` / `MetadataUsage`, `Metaclass` |
| `comment`, `documentation`, `textualRepresentation`, `dependency` | `Comment`, `Documentation`, `TextualRepresentation`, `Dependency` |

Three kinds spell a metamodel type the notation's keyword does not name
directly. Each is the metaclass the pinned grammar's own production returns, so
none of the three is an approximation:

| OpenSysML kind | `@type` | Why |
|---|---|---|
| `individualDef` / `individualUsage` | `OccurrenceDefinition` / `OccurrenceUsage` | `IndividualDefinition returns SysML::OccurrenceDefinition` (`SysML.xtext`): an individual *is* an occurrence, with `isIndividual` set. The metamodel has no individual class to report |
| `connectorEnd` | `ReferenceUsage`, or `PortUsage` on an interface | `ConnectorEnd returns SysML::ReferenceUsage` and `InterfaceEnd returns SysML::PortUsage` (`SysML.xtext`), so the end is named by the connector that declares it — `MetamodelTypeNameOf` reads the owning declaration to tell the two apart |
| `alias` | `Membership` | `AliasMember returns SysML::Membership`: an alias is a named membership, not an element of its own |

One kind reports a type only when the model being queried declares it.
`kerMLType` covers KerML's `class`, `struct`, `assoc`, `behavior`, `predicate`
and `interaction`, which are distinct metaclasses the symbol kind does not
record; `MetamodelTypeNameOf` recovers the metaclass from the declaration's
keyword. A `kerMLType` restored from the library cache carries no declaration and
so reports no `@type`, like any cached element whose kind was not retained.

### Comparison semantics

Where the standard is vague, these are the choices this implementation makes:

- `=` compares the property's value as text, and matches if it equals **any** of
  the listed values. The standard writes one value and its clients write a list
  for `@type`; one value is the degenerate case of the same rule.
- `>` and `<` compare numbers, and require exactly one operand and an ordered
  property (the `multiplicity*` pair). A non-ordered property, more than one
  operand, or an operand that is not a number is an `INVALID_ARGUMENT` error, not
  a false verdict. `*` parses as infinity, so `multiplicityUpper > 1` holds for
  an unbounded feature.
- An element that *has no value* for the property fails the comparison; this is a
  fact about the element, not a fault in the query.
- `inverse` negates the verdict of its own constraint, so a constraint and its
  inverse partition the scope.
- `and`/`or` combine nested verdicts, short-circuiting once one is decisive.
- The whole `where` tree is judged **before** any element is read, so a fault in
  it — an unknown property, a missing operator, an empty composite, `>` on an
  unordered property — is reported whatever the scope holds, including a model
  that declares nothing (`TestQueryFaultIsReportedWithNoElementsToConsider`), and
  wherever it sits, including under an already-decisive sibling.
- `scope` considers each named element **and everything nested inside it**, in
  declaration order, parents first; a name the model does not have is an error.
  An empty scope enumerates the parsed document (not the standard library, which
  is reachable by naming a library element as a scope).

### Not supported — by design of the standard

The standard's query model is deliberately weak, and this is an interop surface,
not OpenSysML's expressive query story:

- **No graph traversal and no transitive closure.** There is no "all elements
  under X", no "everything that specializes Y", no path expressions and no joins.
  Containment is expressible only as a `scope`; specialization is not expressible
  at all, even though `semantics.Model` can answer it.
- No `owningProject` / `@id` on the query resource (single-model service, no
  project or commit store), no derived or computed properties, and no ordering or
  paging of results — elements come back in declaration order.

---

## Native document queries and rendering over gRPC

The native document pipeline — document queries (`calc def` specializing
`DocumentQueries::Query`) and documents (`part def` specializing
`DocumentQueries::Document`) — is served by two RPCs, so a CI pipeline or an
external tool can run what `%run-query` and `%render-document` run without a
REPL or a script:

```proto
rpc RunDocumentQuery(RunDocumentQueryRequest) returns (RunDocumentQueryResponse);
rpc RenderDocument(RenderDocumentRequest) returns (RenderDocumentResponse);
```

**Implementation:** `internal/grpc/docquery.go` (`Service.RunDocumentQuery`,
`Service.RenderDocument`), reported from `GetServerInfo` as the
`document_query` and `render_document` capabilities. Python:
`model.run_document_query(...)` and `model.render_document(...)`
(`clients/python/opensysml/document.py`). The JSON shape of a binding and of the
result table, captured, is on [the wire contract](wire-contract.md#rundocumentquery).

Both name a loaded model by its hash and a definition by qualified name, and
compose the engine packages the REPL and CLI compose — `queryplan` →
`queryexec` for a query, `docplan` → `docir` → `docrender` for a document — so
the rows and the Markdown are the ones those surfaces produce, in the same
deterministic order.

`RunDocumentQuery` takes typed parameter bindings — an element by qualified
name, a string, an integer, a real, a boolean, or a list of these for a
multi-valued parameter — and answers the projected column names and typed rows:
each row's element and each cell's values, an element carried as its qualified
name plus its metamodel type (the `@type` mapping above). `RenderDocument`
takes no bindings, because a document binds its queries' parameters in the
model; it answers the rendered CommonMark Markdown, byte-identical to
`-render-document` on the same model.

Failures keep the engine's message, append the declaring document where the
failure carries provenance, and map onto status codes by whose fault they are:
an unknown model or an unknown query/document is `NOT_FOUND`; a symbol of the
wrong kind, a malformed request, or a wrong binding (unknown, missing,
mistyped, wrong multiplicity, or naming an element the model does not have) is
`INVALID_ARGUMENT`; an exhausted visit or invocation budget is
`RESOURCE_EXHAUSTED`; an operation the engine does not execute — a fault in the
model's own definitions rather than in the request — is `FAILED_PRECONDITION`;
and a request the service lacks the capability for is `UNIMPLEMENTED`, naming
it.

Known limitations: the response carries no provenance for *successful* rows
(only failures name their source), and the document IR is not exposed in a
structured form — Markdown is the only document form served, as it is the only
form the build writes.

---

## Usage Examples

### Parse a File

```go
import (
    "github.com/Open-MBEE/OpenSysML/internal/core/source"
    "github.com/Open-MBEE/OpenSysML/internal/core/parser"
)

src := source.New("example.sysml", []byte(`
    part Wheel {
        attribute diameter : Real;
    }
`))

p := parser.New(src)
root := p.ParseFile()
// root is always non-nil, check root.Errors for parse errors
```

### Build Symbol Table

```go
import (
    "github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

idx := symbols.NewIndex()
idx.AddDocument("example.sysml", root)
scope := idx.DocumentRoot("example.sysml")
sym, ok := scope.LookupLocal("Wheel")
```

### Resolve Names

```go
import (
    "github.com/Open-MBEE/OpenSysML/internal/core/resolve"
)

res := resolve.New(idx)
sym, ok := res.ResolveQualified(scope, qualifiedName)
```

### Type Queries

```go
import (
    "github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

model := semantics.NewModel(res)
members := model.MembersOf(wheelSym)
conforms := model.Conforms(wheelSym, vehiclePartSym)
```

### Run Validation

```go
import (
    "github.com/Open-MBEE/OpenSysML/internal/core/passes"
)

// Analyze wires up the default pass registry and context internally.
diagnostics := passes.Analyze("example.sysml", root, parseDiags, idx)
```

### Execute Runtime

```go
import (
    "github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

rtCtx := runtime.NewContext(model, resolver, runtime.DefaultMaxSteps)
inst, _ := rtCtx.Instantiate(wheelSym)
diameter, _ := inst.GetFeatureValue(rtCtx, "diameter")
fmt.Println(diameter.Value) // Value{Kind: ValConst, Real: 16.0}
```

---

## Architecture Principles

### 1. Immutable AST

AST nodes are syntax-only and immutable after parsing. All semantic information (types, resolved references, instance values) lives in **side tables** keyed by `ast.Node` or `*symbols.Symbol`.

### 2. Lazy & Memoized

- Name resolution: computed on-demand, cached
- Semantic queries: computed on-demand, cached
- Passes: run only when diagnostics requested

### 3. Incremental Analysis

Documents can be updated individually. Symbol index and caches invalidate only affected documents.

### 4. Separation of Concerns

```
Source → Lexer → Parser → AST
                          ↓
                     Symbols (side table)
                          ↓
                     Resolve (side table)
                          ↓
                    Semantics (side table)
                          ↓
                       Passes (diagnostics)
                          ↓
                      Runtime (values, instances)
```

Each layer is independent and testable.

---

## Testing

All packages have comprehensive test coverage:

```bash
go test ./internal/core/parser    # Parser tests
go test ./internal/core/symbols   # Symbol table tests
go test ./internal/core/semantics # Semantic tests
go test ./internal/core/runtime   # Runtime tests
go test ./...                     # All tests
```

Test fixtures in `testdata/*.sysml`.

---

## Further Reading

- **[ARCHITECTURE.md](../internals/architecture.md)** — System architecture and design decisions
- **[the guide](../guide/)** — Getting started guide
- **[Wire contract](wire-contract.md)** — Connect + JSON field by field, for a client with no generated library
- **[OMG SysML v2.1 Beta 1 Spec](https://www.omg.org/spec/SysML/2.0)** — Language specification (2026-07 release)
