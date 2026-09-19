# An execution-owned IR

A design for giving the lowered execution graphs their own identity, so that the runtime no
longer depends on the lifetime or the shape of the syntax tree. Status: **proposal**; nothing
below is implemented. The change it describes is the
[roadmap](../project/roadmap.md)'s "an execution-owned IR" item under package layering; the
layering it completes is in [the architecture](architecture.md#architecture-layers), and the
executor it changes underneath is described in [the action executor](design/action-executor.md).

Every count below was taken on `develop` at the commit this document was first written against,
over non-test Go files, with the commands quoted beside it; re-run them before relying on a
figure, since the codebase moves.

## Contents

1. [The problem](#the-problem)
2. [Inventory](#inventory)
3. [What the runtime needs from each reference](#what-the-runtime-needs-from-each-reference)
4. [The proposed IR](#the-proposed-ir)
5. [Migration as a stack of pull requests](#migration-as-a-stack-of-pull-requests)
6. [Risks and open decisions](#risks-and-open-decisions)

## The problem

The package layering is in place: `internal/` is `syntax/ semantic/ ir/ check/ exec/ translate/
doc/ workspace/ frontend/`, a package imports only the layers below it, and
`tests/hygiene/layering_test.go` fails the build on a violation. Under that layering the runtime
already consumes lowered graphs (`internal/ir/lower`'s `ActionGraph` and `StateGraph`) rather than
`symbols.Symbol.Decl`, and it imports neither `internal/syntax/parser` nor `internal/check/passes`
(both are explicit forbidden edges in the layering test; the parser reaches the runtime only
through the caller-installed `runtime.ExpressionParser` seam, `internal/exec/runtime/model.go`,
and argument typing through `semantics.ArgumentTyper`, `internal/semantic/semantics/invocation.go`).

What is missing is a layer that *owns* the execution structure. The lowered graphs are side
tables keyed by AST nodes: an action node's identity is its `ast.Node` pointer
(`ActionGraph.Nodes []ast.Node`, `Edges map[ast.Node][]ActionEdge`), a state's identity is its
`*ast.StateNode` (`StateGraph.States []*ast.StateNode`, `Behaviors map[*ast.StateNode]…`),
and the executors' *own* state — the active configuration, history, the token queue, breakpoints,
snapshots and held images — is keyed the same way (`StateConfiguration.simpleState
*ast.StateNode`, `actionFrame.subactions map[ast.Node]*actionFrame`). Expressions are not lowered
at all: `eval.go` interprets `ast.OperatorExpr`, `ast.InvocationExpr` and `ast.FeatureChainExpr`
directly and `compile.go` compiles the same tree into closures, and the calc shape
(`calcShape`, `runtime/invoke_calc.go`) is lowered inside the runtime rather than in `lower`.

This is the immutable-AST, side-table design `AGENTS.md` §4 requires, and it is not a defect. Its
costs are:

- **Lifetime.** A running executor, a held image or a debug session pins the syntax tree of every
  document it was lowered from. After an edit the workspace builds a fresh tree and scope for that
  document (`internal/workspace/model/document.go`), so a live session's pointers now name nodes
  no resolver knows; there is no way to re-anchor it except to start over.
- **Shape.** A change to an AST node type reaches `lower`, `check/passes`, `exec/runtime`,
  `exec/smt`, `exec/analysis/modelform`, `doc/queryexec` and `frontend/*` at once, because each
  of them type-switches on the node behind a graph key (`internal/frontend/grpc/session.go`
  asserts `t.Edge.Decl.(*ast.ControlFlowEdge)` to read `IsElse`, for one).
- **Testability and serialization.** The runtime cannot be exercised, snapshotted or compared
  without real trees; a graph has no canonical form, so nothing can be written beside a run or
  diffed between two versions of a model. The embedded target's
  [closed behavior IR](design/embedded-target.md#the-closed-behavior-ir) needs exactly the layer
  this document introduces.
- **Hot-path cost.** Every per-node lookup is a pointer-keyed map (`map[*ast.StateNode]…`,
  `map[ast.Node]…`), where a dense index into a slice would do.

The target is a lowered structure that owns its identity — an opaque id per node, edge and
expression, allocated by lowering — holds no `*ast.*` pointer on any path the executors take, and
keeps the way back to the syntax tree and the symbol table as an *origin* handle (document, span,
qualified name, element id) for diagnostics and for the LSP, REPL and gRPC back-references.

## Inventory

Method: a `go/ast` scanner over `internal/ir/lower` and `internal/exec/...` that lists every
struct field whose type mentions `ast.` and every function whose signature does, plus `rg` for
textual occurrence counts and `go list -deps` for the import closure.

```sh
go list -deps ./internal/exec/runtime | rg '^github.com/Open-MBEE/OpenSysML/' | sort -u
rg -o 'ast\.' internal/ir/lower/*.go internal/exec/runtime/*.go --glob '!*_test.go' | wc -l
rg -o 'ast\.StateNode' internal/exec/runtime/state_executor.go | wc -l   # 218
rg -o 'ast\.Node' internal/exec/runtime/action_executor.go | wc -l       # 33
```

### Package figures

| package | files | imports `syntax/ast` | `ast.` mentions | struct fields typed with `ast.` | functions with `ast.` in signature |
|---|---:|---:|---:|---:|---:|
| `internal/ir/lower` | 25 | 25 | 1383 | 148 | 292 (56 exported) |
| `internal/exec/runtime` | 139 | 85 | 1943 | 108 | 553 |
| `internal/exec/smt` | 11 | 5 | 115 | 15 | 29 |
| `internal/exec/solve` | 20 | 4 | 98 | 0 | 33 |
| `internal/exec/analysis/modelform` | 5 | 3 | 77 | 2 | 15 |
| `internal/exec/analysis` | 30 | 0 | 1 | 0 | 0 |
| `internal/exec/objref`, `engines`, `enginewire` | 4 | 0 | 0 | 0 | 0 |

Neither `lower` nor any `exec` package imports `internal/syntax/parser` or `internal/check/passes`.
`internal/exec/runtime` resolves names through `internal/semantic/symbols` (85 files) and
`internal/semantic/resolve` (6 files).

### The graphs (`internal/ir/lower`)

`ActionGraph` (`action_graph.go`) has 14 AST-typed or AST-keyed fields; `StateGraph`
(`state_graph.go`) has 44. Grouped by what the AST reference *is*:

| role | `ActionGraph` | `StateGraph` |
|---|---|---|
| node identity | `Nodes []ast.Node`, `Initial ast.Node`, `Finals []ast.Node` | `Machine`, `Initial *ast.StateNode`, `States []*ast.StateNode`, `Pseudostates []*ast.PseudostateNode`, `Terminates []*ast.Usage`, `TopRegions []*ast.StateRegion`, `CompositeStateOrder` |
| node-keyed side tables | `Edges`, `DataFlows`, `Bodies`, `footprints`, `Features`, `Scopes`, `Accepts`, `Subflows`, `StatementRuns`, `BlockNodes`, `declaredIn` (11 maps) | `StateScopes`, `Behaviors`, `HiddenStates`, `HiddenRegionOf`, `RegionState`, `PseudostateOwner`, `TerminateOwner`, `Transitions`, `behaviorFootprints`, `CompositeStates`, `RegionInitials`, `ParentState`, `RegionOwner`, `Deferred`, `RegionOf`, `EntryTransitions`, `ForkPlans`, `ForkEntered`, `JoinPlans`, `StateAttributes`, `designatedInitials`, `completing`, `parallelState` and the private inheritance/materialization maps (`declOf`, `regionDecl`, `vertexOf`, `stateByDecl`, `completionOf`, `instanceOf`, `materializing`, `scopeOf`, `regionScopeOf`, `declaredIn`, `copiedFrom`, `behaviorScope`, `attributeScope`, `bodyOf`) |

The edge and statement records repeat the pattern. `ActionEdge{Source, Target, Guard, Decl
ast.Node}`, `ObjectFlow{Target, Decl}`, `Transition{Decl, Source, Target, Trigger, Guard
ast.Node}`, `EntryTransition{Decl, Guard ast.Node; Target *ast.StateNode}`; every `Statement`
variant carries the declaring node (`Assign.Node`, `Declare.Node`, `Block.Node`, `Loop.Node`,
`If.Node`, `Return.Node`, `Effect.Node`, `Unsupported.Node`) and its expressions (`Assign.Value`,
`Loop.Condition/Until/Collection`, `If.Condition`, `Return.Value`, `Send.Message`,
`Accept.Trigger`, `Attribute.Value`, `Feature.Value`, `Probability.Expr`, `ZeroCrossing.Guard`,
`BindingEnd.Expr`). `PinBinding` holds two node paths (`Path`, `OtherPath []ast.Node`). The plan
records (`ForkPlan`, `JoinPlan`, `StateBehavior`, `Place`, `Footprint.Control`) name states and
regions by pointer. Of the 148 fields, by the scanner's heuristic, 16 are node identities, 16
are expression payloads, 9 are declaration handles (`Decl`), and the remaining 107 are
node-keyed tables, `ast` value enums, or the private state of the lowering itself.

Who writes them: only `lower` (constructors `ToActionGraph`, `ToActionGraphWith`,
`ToActionInterface`, `ToStateGraph`, `ToStateGraphWithEndpoints`, `CalcBody`, `ToBindings`,
`ToObjectConnections`, `LowerBehaviors`). Who reads them — textual `\.Field\b` matches, so an
upper bound that includes writes and same-named fields elsewhere:

| reader | matches | entry point |
|---|---:|---|
| `internal/exec/runtime` | 842 | `newActionExecutor` → `lower.ToActionGraphWith` / `ToActionInterface` (`action_executor.go`); `newStateExecutorForOccurrence` → `lower.ToStateGraphWithEndpoints` (`state_executor.go:208`) |
| `internal/check/passes` | 345 | behavior passes over `Transition.{Source,Target,Decl}` |
| `internal/frontend` (`lsp`, `grpc`, `repl`) | 213 | `lsp/debug.go:546` walks `Nodes`/`Subflows`; `grpc/session.go:622` reads `Edges`/`Decl` |
| `tools/` (referee, doc counts) | 212 | `tools/referee/fuml/validate.go:31`, `tools/referee/pssm/validate.go:31` lower directly |
| `internal/exec/analysis`, `modelform` | 162 | `modelform/graphs.go:108–151` lowers and builds model-form graphs |
| `internal/exec/smt` | 155 | `support.go`, `encode.go` carry `*lower.ActionGraph`; `engine.go:185` passes `exec.Graph()` |
| `internal/ir/view` | 128 | diagram views over `Edges`, `States`, `Transitions` |
| `internal/doc/queryexec` | 58 | `states.go:201,227` takes `*lower.StateGraph` |
| `internal/exec/solve` | 10 | via runtime interfaces only; no direct lowering call |
| `internal/translate/export` | 0 | RDF/XMI export does not read the execution graphs |

Materialization (`runtime/materialize.go`) walks instances and features and never obtains a
graph. No cache of lowered graphs exists anywhere: every entry point above re-lowers.

### The runtime's own state (`internal/exec/runtime`)

108 AST-typed fields across 32 files; the ones on the executors' paths:

| file | fields | what they are |
|---|---:|---|
| `state_executor.go` | 19 | `StateConfiguration{simpleState *ast.StateNode; regionStates map[*ast.StateRegion]*ast.StateNode}`, `StateExecutor{stateAttrs, stateStack, history, breakpointNodes, breakpointHit, pausedAt, leftAhead, enteredAhead}`, `historyRecord`, `FiredTransition{Decl, Source, Target, Owner ast.Node}`, `dispatchCandidate` |
| `held_image_behavior.go` | 9 | held-image copies of the frame and state tables above |
| `action_frame.go` | 8 | `actionFrame{node; subactions, pending, nested map[ast.Node]…; nodes []ast.Node}`, `nestedDelivery.path` |
| `state_route.go` | 8 | `route{target, choice, crossed, terminate}`, `junctionDraw.at`, `routeEnds` |
| `state_region_entry.go` | 7 | `regionEntry`, `lazyEntry` |
| `invoke_action.go` | 7 | `actionInvocation{target *ast.QualifiedName; args []ast.Node; named []ast.NamedArg; chain; expr *ast.InvocationExpr; referrer}` |
| `snapshot.go` | 5 | `stateCapture` (the serializable snapshot keeps `*ast.StateNode`) |
| `action_executor.go` | 4 | `NodeBreakpoint{Within []ast.Node; Node ast.Node}`, `Traversal.Within`, `breakpointVisit` |
| `state_unit_front.go` | 4 | `unitHead`, `firingScope` |
| `invoke_calc.go` | 3 | `calcShape{Nodes []ast.Node; ResultExpr ast.Node}`, `calcParameter.Default` |
| `model.go` | 3 | `Model{integerLiterals map[*ast.LiteralInteger]int64; realLiterals map[*ast.LiteralReal]float64; declared map[ast.Node]*symbols.Symbol}` |
| `condition.go`, `signal.go`, `action_body_run.go` | 3 each | expression payloads; `messageArgs`; work items keyed by node |
| `context.go` | 2 | `Context.bindingOwners map[featureValueRef]*ast.Usage`, `scopedMember.node` |
| `eval.go` | 2 | `invocationKey.node *ast.InvocationExpr` (cache key), `writtenArgument.expr` |
| `executor_common.go` | 1 | `Token.Location ast.Node` |
| `shape.go` | 1 | `EffectiveFeature.DefaultValue ast.Node` |
| `errors.go` | 1 | `NoValueError.Ref *ast.QualifiedName` |

Pointer identity is compared directly at `action_executor.go:137` (breakpoints),
`state_executor.go:2815,2918` (transition source matching), `check_moves.go:110`,
`lower/state_footprint.go:328` and `lower/join_check.go:37`.

Expression evaluation is two type switches over the tree: `EvalContext.eval` (`eval.go:302`,
18 cases: the literals, `MetadataAccessExpr`, `NullExpr`, `FeatureReference`, `QualifiedName`,
`FeatureChainExpr`, `OperatorExpr`, `SequenceExpr`, `CollectExpr`, `SelectExpr`,
`InvocationExpr`, `IndexExpr`, `ConstructorExpr`, `BodyExpr`) and `calcCompiler.compileNode`
(`compile.go:414`, 6 cases). Two further switches serve names (`subjectName`, `eval.go:1838`;
`chainText`, `eval.go:3064`). The 553 AST-bearing runtime signatures are dominated by these two
files and the state executor (`state_executor.go` 123, `eval.go` 81, `state_route.go` 28,
`state_region_transition.go` 27, `action_executor.go` 24).

### Where a lowered graph goes after an edit

`reindexLocked` (`internal/workspace/model/workspace.go:317`) reparses only the edited document,
builds a fresh AST and scope tree, replaces the document in the shared index
(`symbols.Index.AddDocumentScope`, `internal/semantic/symbols/index.go:357`, which shares the
caller's scope tree so that "a symbol found through either is the same"), then `invalidateLocked`
(`workspace.go:339`) bumps the generation, hands the index changes to `resolver.Invalidate` and
propagates to dependent documents. Unchanged documents keep their trees, so a graph lowered from
them keeps valid pointers; a graph lowered from the edited document, or from anything that
resolved into it, is silently stale. Nothing detects that today because nothing caches graphs —
and nothing can cache them until they have an identity that is not the tree.

## What the runtime needs from each reference

Each AST reference in the inventory serves one of a small number of needs. The classification
below is the design's contract: **id** means the reference is only compared, used as a key or
followed to a neighbour, and an opaque id does the same job; **origin** means the reference is
only rendered — a name, a kind word, a span — for a diagnostic, a trace line, a hover or a
wire message, and an origin handle does the same job; **smell** means the runtime reads the
declaration's *content* through the reference, which is reconstruction from declarations and has
to move into lowering.

| reference | need | class | replacement |
|---|---|---|---|
| `ActionGraph.Nodes/Initial/Finals`, `ActionEdge.Source/Target`, `ObjectFlow.Target`, `Effect.Target`, `Subflows`, `BlockNodes`, `PinBinding.Path/OtherPath` | traverse, compare, key | id | `NodeID`; paths become `[]NodeID` |
| `StateGraph.States/Pseudostates/Terminates/TopRegions/Machine/Initial`, every `map[*ast.StateNode]`, `map[*ast.StateRegion]`, `map[*ast.PseudostateNode]` | traverse, compare, key | id | `StateID`, `RegionID`, `PseudoID`, dense slices indexed by id |
| `Transition.Source/Target`, `EntryTransition.Target`, `ForkPlan.Branches`, `JoinPlan.Regions` | route between vertices | id | `VertexRef{Kind, Index}` |
| node kind checks (`node.(*ast.InitialNode)`, `*ast.DecisionNode`, `*ast.ForkNode`, …) in runtime, smt, grpc, lsp | dispatch on kind | id | `NodeKind` enum on the node record |
| `ast.LoopKind`, `ast.FeatureDirection` values | enum | id | keep — value types from `ast` with no pointer; re-export from the IR package so consumers need not import `ast` |
| `*.Decl` on edges, transitions, bindings, `Inherited`, `FiredTransition.Decl/Owner`, `Token.Location`, `NoValueError.Ref`, `nodeDescription(node)`, `lower.DocOf`, `VertexKind`, `EndpointText` | name, kind word, span, doc string for messages and traces | origin | `Origin` handle; `nodeDescription` becomes `Origin.String()`; the doc string is lowered into the record once |
| `NodeBreakpoint.Node/Within`, `breakpointNodes`, `pausedAt`, `breakpointHit`, LSP `debugNode`, gRPC session nodes | identify a location across the process boundary and across edits | origin + id | `(GraphID, NodeID)` on the wire; re-anchoring by `Origin.Path` after an edit |
| `grpc/session.go`: `t.Edge.Decl.(*ast.ControlFlowEdge).IsElse` | a flag the declaration carries | smell (mild) | `ActionEdge.Else bool`, lowered |
| `ActionGraph.Scopes/declaredIn`, `StateGraph.StateScopes/scopeOf/…`, `actionFrame.scope` | resolve a name written in the body at run time | smell | with expression lowering, names resolve during lowering into `SymbolRef`/slot indices and the scope leaves the hot path; until then, `Scopes []*symbols.Scope` indexed by id (a semantic-layer handle, allowed) |
| `Model.declared map[ast.Node]*symbols.Symbol` | find the symbol of a declaration the runtime is holding | smell | the runtime holds symbols and ids, never declarations; drop |
| `Context.bindingOwners map[…]*ast.Usage` | the usage that owns a binding, for error text and redefinition | smell | `Origin` for text; the redefinition relation is lowered into `Binding` |
| `EffectiveFeature.DefaultValue ast.Node` | evaluate the default | expression | `ExprID` |
| `Model.integerLiterals/realLiterals` keyed by literal pointer | memoized literal parse | smell | literals are values in the lowered expression; drop |
| `invocationKey.node *ast.InvocationExpr` | cache the resolved callee per call site | id | key by `ExprID` |
| `calcShape{Nodes, ResultExpr}` and its lowering in `invoke_calc.go` | the calc's parameters, steps and result | smell (lowering in the wrong layer; named by the roadmap) | `lower.CalcBody` grows into `lower.ToCalc` returning an IR `Calc`; the runtime memoizes by symbol as now |
| every expression payload (44 fields in `lower`, `Condition.Expr`, `Objective.Value`, `actionInvocation.args/named/expr`, `messageArgs`, `valueSource.expr`, `writtenArgument.expr`, `SatisfyAssertion.SubjectChain`, `passedReference.ref`) | evaluate | expression | `ExprID` into the graph's expression table; `eval` and `compile` switch on `ExprKind` |
| `ExpressionParser func(origin, text string) (ast.Node, bool)` | parse notation text handed to a run | seam | the seam returns a lowered expression: `func(origin, text string) (*program.Expr, bool)`; the installer (frontend/workspace) parses *and* lowers, so parsing and lowering both stay out of `exec` |
| `lower`'s 56 exported AST-signature helpers (`DeclaresNodeFeature`, `SendPayload`, `IsCaseNode`, `ClassifierBehaviorOf`, `StartShot`, …) | used by `check/passes`, `ir/view`, `tools` on declarations | not on the runtime path | unchanged; `lower` stays the one package that reads both the tree and the IR |

The runtime never traverses AST *children* itself except in the two expression evaluators and in
`invoke_action.go`/`signal.go`, which read `InvocationExpr` arguments; all three are covered by
expression lowering. No runtime code asks a `*ast.*` for a type: type lookup goes through
`symbols`/`semantics` with a symbol in hand.

## The proposed IR

### Package and ownership

A new package `internal/ir/program` (the name is an open decision, below) holds the types. It
imports `internal/syntax/source` for spans and `internal/semantic/symbols` for symbol handles,
and **not** `internal/syntax/ast`. `internal/ir/lower` keeps importing `ast` — it is the one
place that reads the tree and writes the IR — and returns `program` values. `exec/*` imports
`program` and, at the end of the migration, no longer `ast`.

Allocation is **per graph**: one `Action` or `StateMachine` value owns dense slices for its
nodes, edges, statements and expressions, and every id is an index into a slice of that value.
No process-wide arena: graphs are rebuilt per lowering, held by an executor for the life of a
run, and dropped with it, so a per-graph arena has exactly the lifetime the data needs and needs
no locking; a global arena would need generation stamps to keep ids from dangling and would pin
every graph ever lowered. Ids are therefore **not** unique across graphs; a location that must
name a graph too carries `GraphID` (a content digest of the canonical form, see below).

```go
package program

// Ids are indices into the owning graph's tables; zero is "none".
type NodeID uint32   // Action.Nodes
type EdgeID uint32   // Action.Edges
type StmtID uint32   // Action.Stmts
type ExprID uint32   // Exprs (shared by Action, StateMachine, Calc)
type StateID uint32  // StateMachine.States
type RegionID uint32 // StateMachine.Regions
type PseudoID uint32 // StateMachine.Pseudostates
type TransID uint32  // StateMachine.Transitions

// GraphID names one lowered graph: a digest of its canonical serialization,
// so two lowerings of the same text yield the same id and an edit that
// changes the graph changes it.
type GraphID [16]byte

// Origin is the way back from an IR record to the source and the symbol table.
// It is the only thing a diagnostic, a trace line or a wire message needs, and
// it is plain data: no pointer into the tree.
type Origin struct {
	Doc     string      // document name, as the workspace keys it
	Span    source.Span // the declaring construct; zero for synthesized records
	Path    string      // qualified name of the nearest named owner, then
	                    // `#<kind>[<ordinal>]` for anonymous members
	Element string      // identity.Info.EffectiveID of the nearest element; "" if none
	Kind    string      // the kind word messages use today ("action", "decide", ...)
	Name    string      // declared name or "" (nodeDescription's input)
	Docs    string      // lowered documentation comment, for hovers
}
```

`Origin.Path` is the stable key: it survives a reparse of the same text and, for named members,
an edit elsewhere in the document. `Span` does not survive an edit above it and is for rendering
only. `Element` is the element id the identity table already derives (annotated, normative or
derived from the qualified name), so an origin can be joined to RDF/XMI output without the tree.

### Actions

```go
type NodeKind uint8 // Initial, Final, Fork, Join, Merge, Decision, Execution,
                    // Accept, Send, Assign, Terminate, If, Loop, Block, ...

type Node struct {
	Kind     NodeKind
	Origin   Origin
	Out      []EdgeID    // successions in declaration order
	Flows    []FlowID    // object flows out of this node
	Body     []StmtID    // what advancing a token through the node runs
	Features []Feature   // parameters and attributes the node declares itself
	Scope    *symbols.Scope // semantic handle; removed once expressions are lowered
	Accept   *Accept
	Subflow  *Action     // nested flow, when the node's own members state one
	Block    []NodeID    // nodes of a body-local block
	RunsAsStatement bool
}

type Edge struct {
	Source, Target NodeID
	Guard  ExprID // 0 when unguarded
	Else   bool
	Origin Origin
}

type Flow struct { Source, Target NodeID; SourcePin, TargetPin string; Origin Origin }

type Action struct {
	ID         GraphID
	Origin     Origin
	Scope      *symbols.Scope // the body's declaring scope (semantic handle)
	Attributes []Feature
	Nodes      []Node   // index 0 unused
	Edges      []Edge
	Flows      []Flow
	Stmts      []Stmt
	Exprs      []Expr
	Initial    NodeID
	Finals     []NodeID
	Bindings   []PinBinding
	Footprints []Footprint // computed lazily today; indexed by NodeID
}
```

`Stmt` keeps the current statement variants (`Assign`, `Declare`, `Block`, `Loop`, `If`,
`Return`, `Send`, `Effect`, `Unsupported`, …) with `Origin` in place of `Node` and `ExprID` in
place of every `ast.Node` expression. `Unsupported` keeps its role: a construct lowering cannot
express becomes a typed record, and the runtime fails on it where it fails today.

### State machines

```go
type VertexKind uint8 // State, Pseudostate, Terminate, Final

type VertexRef struct { Kind VertexKind; Index uint32 } // StateID / PseudoID / usage ordinal

type State struct {
	Origin     Origin
	Parent     StateID   // 0 for a top state
	Region     RegionID  // the region that owns it
	Regions    []RegionID // orthogonal regions, in order; empty for a simple state
	Entry, Do, Exit []BehaviorID
	Deferred   []ExprID  // deferred triggers
	Attributes []Feature
	Transitions []TransID // outgoing, in declaration order
	Hidden, Completing, DesignatedInitial, Parallel bool
	Scope      *symbols.Scope
}

type Region struct { Origin Origin; Owner StateID; Initial StateID; Entered []PseudoID }

type Pseudostate struct { Kind PseudoKind; Origin Origin; Owner StateID; Fork *ForkPlan; Join *JoinPlan }

type Transition struct {
	Origin  Origin
	Source, Target VertexRef
	Trigger ExprID
	Guard   ExprID
	Effects []StmtID
	Probability ExprID
}

type StateMachine struct {
	ID          GraphID
	Origin      Origin
	Machine     StateID
	Initial     StateID
	States      []State
	Regions     []Region
	Pseudostates []Pseudostate
	Transitions []Transition
	EntryTransitions []EntryTransition
	Behaviors   []Behavior // each a *Action lowered from an entry/do/exit body
	Stmts       []Stmt
	Exprs       []Expr
	TopRegions  []RegionID
	CompositeOrder []StateID
}
```

The private inheritance and materialization maps of today's `StateGraph` (`declOf`,
`vertexOf`, `stateByDecl`, `copiedFrom`, `instanceOf`, …) are the lowering's own working state,
keyed by declaration because inheritance copies declarations. They move into the lowering
*builder* and do not appear on the result; the result records only what the runtime consumes,
plus one back-map.

### Expressions

```go
type ExprKind uint8 // Int, Real, Bool, String, Infinity, Null, Ref, Chain, Op,
                    // Sequence, Collect, Select, Invoke, Index, Construct, Body, Metadata

type Expr struct {
	Kind   ExprKind
	Origin Origin
	Op     string        // operator token for Op
	Args   []ExprID      // operands, sequence elements, invocation arguments
	Named  []NamedArg    // {Name string; Value ExprID}
	Ref    SymbolRef     // resolved feature for Ref/Chain heads; see below
	Chain  []SymbolRef   // remaining steps of a Chain
	Int    *big.Int; Real float64; Bool bool; Str string // literal payloads
	Body   *Action       // Body expressions lower to a nested flow
}

// SymbolRef is what name resolution produced at lowering time: the symbol, or
// the written name when resolution deferred to run time (a chain through a
// value whose type is only known when it is held).
type SymbolRef struct { Sym *symbols.Symbol; Name string; Span source.Span }
```

Lowering an expression runs today's resolver once, at lowering, and records the result; the
evaluator switches on `ExprKind` and reads `Ref.Sym` instead of resolving `FeatureReference` in a
scope. Where the runtime today resolves against a *held* value's type — chains through
polymorphic features, `meta` casts — the `SymbolRef` carries the written name and the runtime
resolves as it does now, against `semantics`, not against the tree. This is the boundary the
roadmap names ("lower expressions once rather than interpreting the tree in two evaluators");
`compile.go`'s closures compile from the same `Expr` table. The typed IR of
`internal/translate/codegen/ir.go` is a *lower* level (typed, monomorphic, for emission) and is
not merged with this one; `codegen` will later consume `program.Expr` instead of the tree.

### The back-map

```go
package lower

// Lowered is what a lowering call returns: the graph and its provenance.
type Lowered[G any] struct {
	Graph   G
	Origins Provenance
}

// Provenance answers the syntax tree and symbol for an IR record. It is a
// side table the frontends and the checkers use; the runtime never receives it.
type Provenance struct {
	nodes   []ast.Node          // by NodeID
	states  []*ast.StateNode    // by StateID
	regions []*ast.StateRegion
	pseudos []*ast.PseudostateNode
	trans   []ast.Node          // by TransID
	exprs   []ast.Node          // by ExprID
	byNode  map[ast.Node]NodeID // the inverse, for "which node is this declaration"
}
```

The runtime constructors take `*program.Action` / `*program.StateMachine`; the frontends that
lower (LSP debug, gRPC session, REPL, `queryexec`, `modelform`, the referee tools) keep the
`Provenance` beside the graph and translate in both directions: a breakpoint set on a span
becomes `Provenance.NodeAt(span)` → `NodeID`; a `FiredTransition{Source, Target VertexRef}`
becomes an AST node again for a hover through `Provenance.Node(ref)`. This keeps the
`ast`-import on the frontend side of the seam, where it already is, and makes the runtime's wire
types (`Token`, `FiredTransition`, `Traversal`, `NodeBreakpoint`, `RunNote`s) carry `(GraphID,
NodeID)` and `Origin` only.

### Canonical form

Every id is an index and every table is in declaration order, so a graph has a canonical text
form for free: encode the tables in field order, with ids as integers and origins as
`(Path, Kind, Name)`. `GraphID` is the digest of that encoding without spans and documentation.
Two lowerings of one model produce byte-identical forms; a diff of two model versions can be read
at the IR level; and the [closed behavior IR](design/embedded-target.md#the-closed-behavior-ir)
the embedded target needs is this form with the `*symbols.Scope` handles removed, which is why
those handles are the one field this design leaves on a hot record and marks for removal.

### Incremental re-lowering with the persistent resolver

Graphs gain a cache in the workspace, not in `lower` or `exec`:

```go
package model // internal/workspace/model

type graphKey struct{ owner string /* Origin.Path */; kind program.GraphKind }

type graphEntry struct {
	generation uint64              // workspace generation it was lowered under
	docs       map[string]struct{} // documents the lowering resolved into
	action     *lower.Lowered[*program.Action]
	state      *lower.Lowered[*program.StateMachine]
}
```

- Lowering records, through the resolver it is given, the set of documents whose scopes it
  looked into (`docs`); the resolver already knows this per query, and the shared scope tree
  (`symbols.Index.AddDocumentScope`) means those are the same scope objects the index holds.
- `invalidateLocked` already computes the edited document and its dependents; it additionally
  drops every `graphEntry` whose `docs` intersects that set. Unchanged, independent graphs
  survive an edit with their pointers valid *and* their `GraphID` unchanged.
- A live executor keeps its graph regardless — it owns a `*program.Action` and no tree. A debug
  session that must continue across an edit asks the workspace for the graph at the same
  `Origin.Path`; if the new `GraphID` equals the old, nothing changed and the session continues
  on the new value; if not, the session re-anchors each breakpoint by `Origin.Path` (falls back
  to `Kind` + ordinal inside the owner) and reports the ones it could not place, typed, instead
  of silently pointing at a dead tree as today.
- Snapshots and held images record `GraphID` beside each `NodeID`/`StateID`; restoring against a
  graph with a different id is a typed error (`ErrSnapshotGraphMismatch`), which is a behavior
  the runtime cannot express at all today.

## Migration as a stack of pull requests

Each step keeps the whole gate green: `go build ./...`, `go vet ./...`, `gofmt -l .` empty, the
full suite with `OPENSYSML_REQUIRE_TRAINING_CORPUS=1 OPENSYSML_REQUIRE_PILOT_CORPORA=1
OPENSYSML_REQUIRE_PILOT_LIBRARY_XMI=1`, the `tools` module, `make docs-counts`, and
`TestPackageLayering`. The baselines that must not move at **any** step: the pilot Xpect,
differential and rejection ratchets (`tests/corpus`, `TestPilotCorpora`), the RDF round trip
(`TestCorpusRoundTrip`), the fUML and PSSM referee records under `tools/referee`, the execution
conformance suite (194 fixture files under `internal/exec/runtime/testdata/conformance`,
`TestExecutionConformance` and `…UnderPolicies`), the golden traces (`TestExecutionTrace`,
which print names, not pointers, and so are the right pin for an identity change), every
`TestRuntimeRobustness*`, `TestGRPCConformance` and the LSP debug tests. Export goldens are
untouched throughout since `translate/export` does not read the graphs.

Files-touched figures are estimates from the inventory; "pins" names the tests that would catch a
regression in that step specifically, on top of the always-on gate.

| # | change | files (est.) | pins | layering |
|---|---|---:|---|---|
| 1 | `internal/ir/program`: id types, `Origin`, `GraphID`, `NodeKind`; `lower` assigns a `NodeID`/`StateID`/… to every record it already builds and returns `Provenance` beside the graph (additive: `ActionGraph.ID map[ast.Node]NodeID`, `StateGraph.ID map[*ast.StateNode]StateID`, inverse slices). No consumer changes. | 8–10 | new `lower` unit tests: ids dense, in declaration order, deterministic across two lowerings, `Origin.Path` unique per graph; `TestGolden` untouched | `program` registered in the `ir` layer; forbidden `program → syntax/ast` |
| 2 | Action executor state keyed by id: `actionFrame.{subactions,pending,nested,nodes}`, `Token.Location`, `NodeBreakpoint`, `Traversal`, `breakpointVisit`, held-image and snapshot copies, `check_reduce.futureKey`. The graph still carries AST; the executor indexes `Nodes[id]`. | 12–16 | conformance, traces, `TestRuntimeRobustness*`, `TestRuntimeRobustnessReplay` (snapshots), LSP debug tests, `TestGRPCConformance` | — |
| 3 | State executor state keyed by id: `StateConfiguration`, `stateAttrs/stateStack/history`, `leftAhead/enteredAhead`, `route`, `regionEntry/lazyEntry`, `unitHead/firingScope`, `dispatchCandidate`, `FiredTransition` (`VertexRef` + `Origin`), held image and snapshot. The 19 fields in `state_executor.go` and its 123 signatures are the bulk. | 20–28 | conformance (state cases), traces with region-order policies, `TestRuntimeRobustnessRegionOrder`, `…Terminate`, `…ResumableInlineDoBody`, PSSM referee record | — |
| 4 | Origins replace declaration handles for text: `nodeDescription`, `lower.DocOf/VertexKind/EndpointText` results lowered into `Origin`; `NoValueError.Ref`, `Context.bindingOwners`, `Model.declared`, `ActionEdge.Else` lowered; gRPC `session.go` and `queryexec/states.go` read `Origin` and `Provenance`. | 15–20 | conformance `.expected.json` (error text), `TestGRPCConformance`, `conformance_docquery_test`, `tests/grpc` robustness | forbidden `exec/runtime → syntax/ast` **not yet** — expressions remain |
| 5 | Calc lowering moves to `lower`: `calcShape` becomes `program.Calc` built by `lower.ToCalc`; `compileBatch.callees` keyed by `*program.Calc`; the runtime memoizes by symbol as now. | 8–10 | conformance (calc cases), `testdata/compiled` (4 files), `tests/perf` calc benchmarks, pilot differential | — |
| 6a | Expression IR, first half: literals, `Ref`, `Chain`, `Op`, `Sequence`, `Index`, `Null`, `Infinity`; `lower.LowerExpr` resolves names once; `eval`/`compile` switch on `ExprKind` for these kinds and fall back to the tree for the rest; `Model.integerLiterals/realLiterals` dropped. | 20–30 | conformance, `testdata/compiled`, `TestGolden` (unchanged), pilot differential and Xpect (evaluation results), exact-rational tests | — |
| 6b | Expression IR, second half: `Invoke` (`actionInvocation`, `messageArgs`, `invocationKey`), `Collect`, `Select`, `Construct`, `Body`, `Metadata`; `ExpressionParser` seam returns `*program.Expr`; every `lower` expression payload becomes `ExprID`; `EffectiveFeature.DefaultValue` becomes `ExprID`. | 30–40 | all of 6a plus `TestRuntimeRobustnessSendTarget`, `…PerformOnPart`, signal/accept conformance, REPL tests for the seam | forbidden `exec/runtime → syntax/ast` **added**, allowing only value types re-exported by `program` |
| 7 | Consumers off the tree: `exec/smt` (`Flow`, `Encode` over `program.Action`), `exec/analysis/modelform`, `exec/solve`, `ir/view`, `doc/queryexec`, `frontend/lsp/debug.go`, `frontend/grpc/session.go`, `frontend/repl`, `tools/referee/{fuml,pssm}` use ids and `Provenance`. | 35–45 | SMT engine tests, analysis framework tests, view goldens, doc-query conformance, LSP debug tests, referee records, `tools` module tests | forbidden `exec/smt`, `exec/solve`, `exec/analysis/… → syntax/ast` |
| 8 | Remove the AST fields from `ActionGraph`/`StateGraph`, rename to `program.Action`/`program.StateMachine` (type aliases first, then the move), delete the parallel maps from step 1, move the private inheritance maps into the builder. | 40–60 (mechanical) | the whole gate; `go vet` for dead code; `tests/testutil/graphcmp` comparisons in `lower` tests updated to compare ids | forbidden `ir/program → syntax/ast` re-checked; `check/passes` may still import both |
| 9 | Canonical form and `GraphID`: encoder, `TestCanonicalFormDeterministic` over the conformance corpus, `GraphID` on snapshots and held images with `ErrSnapshotGraphMismatch`. | 6–8 | new determinism test; `TestRuntimeRobustnessReplay`; snapshot goldens | — |
| 10 | Workspace graph cache and re-anchoring: `graphEntry` keyed by `Origin.Path`, invalidated with the resolver's dependent set; LSP debug re-anchors breakpoints by `Origin.Path` after an edit and reports the unplaceable ones. | 8–12 | new workspace tests: edit an unrelated document → same `GraphID` and same pointer; edit the owner → new entry; LSP debug edit-during-session test | — |

Steps 2–5 are independent of one another once 1 has landed and can be reviewed in parallel; 6a
depends on 5 (the calc compiler is the first consumer of `Expr`), 6b on 6a, 7 on 6b, 8 on 7, and
9–10 on 8. Each of 2, 3, 6a, 6b and 7 is feature-sized under `AGENTS.md` §8 — complete, no
bridge that copies IR back into the legacy fields — and each can be split further along the
file groups its row lists if review size demands it.

`docs/project/spec-compliance.md` does not move: no semantic rule changes. The documentation
census (`make docs-counts`) is unaffected until step 8 renames the graph types, when
`architecture.md` §"Runtime consumes lowered IR" and `design/action-executor.md` are updated in
the same pull request.

## Risks and open decisions

Each needs a human decision; the recommendation is the one the design above assumes.

1. **Package name and location.** `internal/ir/program` (this document), `internal/ir/exec`
   (reads well but collides with the `exec` layer name in every grep and in the layering table)
   or `internal/ir/behavior` (collides with `check/passes/behavior`). *Recommend `program`.*

2. **Expression lowering in scope, or identity only.** Steps 1–5 and 8–10 deliver the identity
   and lifetime goals without touching the evaluators; the `exec/runtime → syntax/ast` forbidden
   edge, however, is only reachable through 6a/6b, which are the largest and riskiest steps
   (`eval.go` has 81 AST-bearing signatures and the pilot differential pins evaluation results).
   *Recommend keeping 6a/6b in scope but landing 1–5 first and re-estimating 6 against the
   inventory at that point; the forbidden edge is the exit criterion, not a mid-point.*

3. **Where name resolution happens for expressions.** Resolving at lowering time (this design)
   makes lowering depend on the resolver's state and means an edit that changes resolution
   invalidates the graph, which step 10 handles; resolving at run time keeps today's behavior but
   keeps `*symbols.Scope` on hot records forever and blocks the closed form. *Recommend lowering-
   time resolution with `SymbolRef.Name` as the run-time fallback for type-dependent chains.*

4. **Ids per graph or process-wide.** Per-graph indices (this design) are dense, cheap and need no
   synchronization, but are not unique across graphs, so every wire type must carry `GraphID`.
   *Recommend per-graph; a global arena buys nothing the digest does not, and pins memory.*

5. **`GraphID` as a content digest versus a workspace-assigned serial.** A digest lets two
   sessions and two processes agree without coordination and makes snapshot compatibility a
   local check; a serial is cheaper to compute. *Recommend the digest; the canonical encoder is
   needed anyway for the embedded target.*

6. **Whether `Origin` carries the span.** Spans go stale on the first edit above them; carrying
   them means a diagnostic from a long-running session can point at the wrong line. The
   alternative is `Path` only and a lookup through `Provenance` for the span at render time.
   *Recommend carrying the span and the document name: diagnostics are rendered at once in almost
   every path, `Path` is there for the late ones, and the frontends already re-resolve spans on
   the current tree for hovers.*

7. **What the debugger promises across an edit.** Re-anchoring by `Origin.Path` recovers named
   members; anonymous nodes (`then`, `first start`, unnamed `decide`) re-anchor by kind and
   ordinal within their owner, which is wrong when a sibling was inserted before them. *Recommend
   re-anchoring with a typed report of what moved or was lost, and documenting that anonymous
   breakpoints may shift, rather than restarting the session or pretending the old tree lives.*

8. **`check/passes` and `ir/view` as readers of the graphs.** Today the behavior passes read
   `Transition.{Source,Target,Decl}` from the tree side; after step 8 they read ids and use
   `Provenance` for the node to attach a diagnostic to. That keeps `check` importing both
   `ast` and `program`, which the layering permits. *Recommend no new forbidden edge for `check`;
   it is a checker and legitimately reads declarations.*

9. **`lower`'s exported declaration helpers.** The 56 exported AST-signature functions
   (`DeclaresNodeFeature`, `SendPayload`, `IsCaseNode`, `ClassifierBehaviorOf`, …) are used by
   passes, views and tools and are not on the runtime path. Leaving them makes `lower` both the
   lowering and a declaration utility library. *Recommend leaving them for this change and
   revisiting once step 8 shows which are still called from outside `lower`.*

10. **Snapshot compatibility.** Snapshots written before step 9 have no `GraphID`; a snapshot
    format bump is a user-visible change for anyone persisting them. *Recommend accepting a
    one-time format version bump at step 9 with a changelog entry, refusing old snapshots with a
    typed error rather than guessing.*

11. **Baseline drift through error text.** Steps 4 and 6 replace `nodeDescription(node)` and
    the chain/subject name helpers with lowered `Origin` fields; a one-character difference moves
    conformance `.expected.json` files, gRPC fixtures and possibly pilot rejection records.
    *Recommend pinning the current strings with a table test in step 1 (`Origin.String()` equals
    `nodeDescription(node)` over the conformance corpus) so step 4 cannot drift silently.*

12. **The `ExpressionParser` seam's owner.** Returning a lowered expression from the seam means
    every installer (REPL, gRPC service, tools) must call `lower.LowerExpr` with a scope, which
    moves resolver access to the installer. *Recommend it: the seam exists to keep parsing out of
    `exec`, and lowering belongs on the same side.*
