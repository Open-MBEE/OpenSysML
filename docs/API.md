# API Documentation

Complete API reference for Systemica packages.

## Overview

Systemica is organized into core packages under `internal/core/`, with frontends in `internal/lsp/` and `internal/repl/`.

**Package Organization:**

```
github.com/Open-MBEE/Systemica
├── internal/core/          # Core language implementation
│   ├── source/             # Source files and position tracking
│   ├── lexer/              # Tokenization
│   ├── parser/             # Parsing to AST
│   ├── ast/                # Abstract Syntax Tree
│   ├── symbols/            # Symbol tables and scopes
│   ├── resolve/            # Name resolution
│   ├── semantics/          # Type system and semantic queries
│   ├── passes/             # Validation passes
│   ├── runtime/            # Execution runtime
│   ├── model/              # Workspace and document management
│   ├── libs/               # Standard library handling
│   └── deps/               # Dependency resolution
├── internal/lsp/           # Language Server Protocol
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
idx.Register("example.sysml", root) // root is *ast.RootNamespace
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
  - `Run(ctx Context, docName string, root *ast.RootNamespace) []Diagnostic`

- **`Context`** — Provides access to resolver and model
  - `Resolver() *resolve.Resolver`
  - `Model() *semantics.Model`

- **`Diagnostic`** — Error/warning
  - `Level DiagnosticLevel` — Error, Warning, Info
  - `Span source.Span`
  - `Message string`

**Usage:**
```go
reg := passes.DefaultRegistry()
diagnostics := passes.Analyze(ctx, reg, "example.sysml", root)
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
  - `Slots map[*symbols.Symbol]Value` — Feature values

**Execution Context:**

- **`Context`** — Runtime execution context
  - `Instantiate(sym *symbols.Symbol) (*Instance, error)` — Create instance
  - `Eval(expr ast.Node, env map[*symbols.Symbol]Value) (Value, error)` — Evaluate expression
  - `GetSlot(inst *Instance, feature *symbols.Symbol) (Value, bool)` — Read slot
  - `SetSlot(inst *Instance, feature *symbols.Symbol, val Value) error` — Write slot
  - `InvokeCalc(sym *symbols.Symbol, args []Value, scope *symbols.Scope) (Value, error)` — Invoke calculation
  - `EvaluateConstraint(sym *symbols.Symbol, scope *symbols.Scope) (bool, error)` — Evaluate constraint
  - `EvaluateRequirement(sym *symbols.Symbol, scope *symbols.Scope) (bool, error)` — Evaluate requirement
  - **`ExecuteAction(sym *symbols.Symbol) (map[string]Value, error)`** — Execute action to completion
  - **`ExecuteState(sym *symbols.Symbol) (map[string]Value, error)`** — Execute state machine until final/suspended
  - **`CreateActionExecutor(sym *symbols.Symbol) (*ActionExecutor, error)`** — Create action executor for debugging
  - **`CreateStateExecutor(sym *symbols.Symbol) (*StateExecutor, error)`** — Create state executor for debugging

**Behavioral Execution (Tier 5):**

- **`Token`** — Control/data token for action execution
  - `ID int64` — Unique token ID
  - `Location ast.Node` — Current node (InitialNode, ActionExecutionNode, etc.)
  - `Data map[string]Value` — Token data (pin values)

- **`ActionExecutor`** — Petri-net token-flow execution engine
  - `Step() error` — Advance all tokens one step
  - `RunToCompletion() error` — Execute until StateCompleted (max 10k steps)
  - `Tokens() []Token` — Get active tokens (copy)
  - `State() ExecutionState` — Current execution state (Ready/Running/Completed/Suspended)
  - `Results() map[string]Value` — Get results after completion
  - `SetBreakpoint(nodeName string)` — Set breakpoint on node
  - `ClearBreakpoints()` — Clear all breakpoints
  - `ActionSymbol() *symbols.Symbol` — Get action symbol

- **`StateExecutor`** — Event-driven state machine execution
  - `ProcessNextEvent() error` — Process next event from queue
  - `CurrentState() ast.Node` — Get current StateNode
  - `StateStack() []*ast.StateNode` — Get active configuration (hierarchical states)
  - `StateData() map[string]Value` — Get state machine variables
  - `EventQueue() *EventQueue` — Get event queue
  - `CurrentTime() float64` — Get simulation time
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
ctx := runtime.NewContext(model, resolver, 100000)
inst, _ := ctx.Instantiate(wheelSym)
val, _ := ctx.GetSlot(inst, diameterSym)
result, _ := ctx.InvokeCalc(addSym, []Value{v1, v2}, scope)
```

Tier 5 (Actions):
```go
// Execute action to completion
results, err := ctx.ExecuteAction(myActionSym)
if err != nil { /* handle error */ }
result := results["result"]

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

### `internal/core/deps`

Dependency resolution (local and git sources).

**Status:** 🚧 In progress

---

## Frontend Packages

### `internal/lsp`

Language Server Protocol implementation.

**Key Type:**

- **`Server`** — LSP server
  - `Run(ctx context.Context, conn io.ReadWriteCloser) error`

**Current Capabilities:**
- Document synchronization (open/change/close)
- Diagnostics (syntax errors)

**Planned:**
- Hover (type info)
- Go to definition
- Completion
- Workspace symbols
- References

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
- `%instantiate <name>`, `%slots <name>`, `%instances`
- `%eval <expr>`
- `%calc <name> [args...]` — Invoke calculation with arguments
- `%constraint <name>` — Evaluate constraint
- `%requirement <name>` — Evaluate requirement

**Usage:**
```go
session := repl.NewSession()
repl.Loop(reader, os.Stdout, session)
```

---

## Usage Examples

### Parse a File

```go
import (
    "github.com/Open-MBEE/Systemica/internal/core/source"
    "github.com/Open-MBEE/Systemica/internal/core/parser"
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
    "github.com/Open-MBEE/Systemica/internal/core/symbols"
)

idx := symbols.NewIndex()
idx.Register("example.sysml", root)
scope := idx.DocumentRoot("example.sysml")
sym, ok := scope.LookupLocal("Wheel")
```

### Resolve Names

```go
import (
    "github.com/Open-MBEE/Systemica/internal/core/resolve"
)

res := resolve.New(idx)
sym, ok := res.ResolveQualified(scope, qualifiedName)
```

### Type Queries

```go
import (
    "github.com/Open-MBEE/Systemica/internal/core/semantics"
)

model := semantics.NewModel(res)
members := model.MembersOf(wheelSym)
conforms := model.Conforms(wheelSym, vehiclePartSym)
```

### Run Validation

```go
import (
    "github.com/Open-MBEE/Systemica/internal/core/passes"
)

ctx := passes.NewContext(res, model)
reg := passes.DefaultRegistry()
diagnostics := passes.Analyze(ctx, reg, "example.sysml", root)
```

### Execute Runtime

```go
import (
    "github.com/Open-MBEE/Systemica/internal/core/runtime"
)

rtCtx := runtime.New(model)
inst, _ := rtCtx.Instantiate(wheelSym)
diameterVal, _ := rtCtx.GetSlot(inst, diameterFeatureSym)
fmt.Println(diameterVal) // Value{Kind: ValConst, Real: 16.0}
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

- **[ARCHITECTURE.md](ARCHITECTURE.md)** — System architecture and design decisions
- **[QUICKSTART.md](QUICKSTART.md)** — Getting started guide
- **[OMG SysML v2.0 Spec](https://www.omg.org/spec/SysML/2.0)** — Language specification
