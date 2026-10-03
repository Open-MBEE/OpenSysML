// Package lower provides AST → execution IR lowering.
// Converts declarative members (TransitionMember, EntryMember) to
// operational graphs (nodes + edges) that executors consume.
package lower

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ActionGraph is the execution IR for actions.
// Nodes represent control flow points, edges represent flow paths.
type ActionGraph struct {
	// Scope is the scope the action's body was declared in, in which every
	// expression written directly among its members resolves its names. A nested
	// node or a body-local block carries its own scope instead.
	Scope *symbols.Scope

	// Attributes are the attribute defaults the action declares, in order.
	Attributes []Attribute

	// Parameters are the directed parameters the action declares itself (`in
	// bread : Bread;`, `out toast : Toast;`, `return r;`), in declaration order:
	// the pins on its own frame. A usage's inherited ones are its type's.
	Parameters []Feature

	// Nodes in the graph (InitialNode, FinalNode, ExecutionNode, etc.)
	Nodes []ast.Node

	// Edges: source node → successions in declaration order.
	Edges map[ast.Node][]ActionEdge

	// Multiplicities are the declarations on action nodes themselves.
	Multiplicities map[ast.Node]*ast.Multiplicity

	// DataFlows: source node → list of object flows
	DataFlows map[ast.Node][]ObjectFlow

	// Bodies: node → the statements that node executes, in declaration order
	Bodies map[ast.Node][]Statement

	// footprints: node → what advancing a token through the node may read, write,
	// send, accept and converge on; computed on the first call of Footprints.
	footprints     map[ast.Node]Footprint
	footprintsOnce sync.Once

	// incoming: node → the successions into it; computed on the first call of Incoming.
	incoming     map[ast.Node][]ActionEdge
	incomingOnce sync.Once

	// Features: node → the parameters and attributes the node declares itself,
	// in declaration order; each performance of the node holds its own values.
	Features map[ast.Node][]Feature

	// Scopes: node → the namespace the node owns, which its features resolve in.
	Scopes map[ast.Node]*symbols.Scope

	// Bindings are the bindings with an end at a node's pin (`bind add.a = x;`).
	Bindings []PinBinding

	// ValueBindings are the bindings a node's pins state by their values naming a
	// feature (`in b = bread;`, `in t = heat.t;`, `out x :>> x = y;`), one per pin,
	// in node then declaration order, with the other end resolved as a Binding's
	// is: a view draws them. The executor reads a pin's value as the pin's own,
	// so only an `inout` pin's, which writes back, is in Bindings as well.
	ValueBindings []PinBinding

	// Accepts: node → the message that node waits for
	Accepts map[ast.Node]Accept

	// Subflows: node → the flow the node's own members state, present only for a
	// node that states one. Its subactions are subperformances of the node, so it
	// completes only when that flow does (action_subflow.go).
	Subflows map[ast.Node]*Subflow

	// Enclosing and EnclosingNode are the graph and node a nested flow runs under,
	// whose pins a write under the flow streams from; nil for the outermost flow.
	Enclosing     *ActionGraph
	EnclosingNode ast.Node

	// Initial is the node the flow's ordered part starts at: the one a `first`
	// names, else its one node no succession leads to; nil where neither exists.
	Initial ast.Node

	// Concurrent are the composite subactions no succession leads to, other than
	// Initial, in declaration order: each starts when a performance of the flow
	// does, unordered against Initial and against each other (StartFlow).
	Concurrent []ast.Node

	// Invalid is the error a stated body's flow failed to lower with,
	// reported at initialize().
	Invalid error

	// FinalNodes (may be multiple)
	Finals []ast.Node

	// Connections are the connectors declared in the action body, which is how
	// a `send ... via <port>` finds the ports it reaches.
	Connections []Connection

	// StatementRuns marks the nodes of a block's own flow that stand for a run of
	// statements rather than for an action node (block_graph.go). Such a node is
	// keyed by the first statement of the run, whose name names no step.
	StatementRuns map[ast.Node]bool

	// StatementOrders holds order metadata for body lists lowered as part of a
	// calculation block's own flow.
	StatementOrders map[ast.Node]*StatementOrder

	// UnstatedCaseFlow marks an unordered case body lifted into an action graph.
	UnstatedCaseFlow bool

	// BlockNodes lists, per node, the action nodes its body's blocks (an `if` branch,
	// a loop body) declare, in declaration order: subperformances reached by name from it.
	BlockNodes map[ast.Node][]ast.Node

	// resolver is the name-resolution tier's, by which lowering tells the metadata
	// it gives a meaning to (Probability) from any other; nil reads none.
	resolver *resolve.Resolver

	// inherited are the actions the action specializes, nearest general first.
	inherited []Inherited

	// declaredIn: inherited node, flow or binding declaration → the scope of the
	// general's body it was written in, which says which document declares it.
	declaredIn map[ast.Node]*symbols.Scope
}

// Incoming returns the successions into node, in the declaration order of the
// nodes they leave. The result is shared: callers must not modify it.
func (g *ActionGraph) Incoming(node ast.Node) []ActionEdge {
	g.incomingOnce.Do(func() {
		g.incoming = make(map[ast.Node][]ActionEdge)
		for _, source := range g.Nodes {
			for _, edge := range g.Edges[source] {
				g.incoming[edge.Target] = append(g.incoming[edge.Target], edge)
			}
		}
	})
	return g.incoming[node]
}

// recordDeclaredIn records the scope of the body an inherited declaration was written in.
func (g *ActionGraph) recordDeclaredIn(decl ast.Node, scope *symbols.Scope) {
	if decl == nil || scope == nil {
		return
	}
	if g.declaredIn == nil {
		g.declaredIn = make(map[ast.Node]*symbols.Scope)
	}
	g.declaredIn[decl] = scope
}

// DocOf is the document a declaration of the graph was written in: the general's
// where inherited, else the action's own; "" outside any document.
func (g *ActionGraph) DocOf(decl ast.Node) string {
	scope := g.Scope
	if decl != nil {
		if declared := g.declaredIn[decl]; declared != nil {
			scope = declared
		}
	}
	return symbols.DocNameOf(scope)
}

// Inherited lists the declarations the action's content came from besides its
// own — the actions it specializes, then those its subflows specialize — each once.
func (g *ActionGraph) Inherited() []Inherited {
	var out []Inherited
	seen := make(map[ast.Node]bool)
	var collect func(g *ActionGraph)
	collect = func(g *ActionGraph) {
		for _, in := range g.inherited {
			if !seen[in.Decl] {
				seen[in.Decl] = true
				out = append(out, in)
			}
		}
		for _, node := range g.Nodes {
			if sub := g.Subflows[node]; sub != nil && sub.Graph != nil {
				collect(sub.Graph)
			}
		}
	}
	collect(g)
	return out
}

// ActionEdge is one succession out of a node: the node it leaves, the target it reaches, the
// guard it carries and its declaration (nil when implicit). No two edges of a graph compare equal.
// Probability is the weight its `@Probability` states, nil for an unweighted succession.
// Name is the name the succession was declared with, "" for an anonymous one.
type ActionEdge struct {
	Source             ast.Node
	Target             ast.Node
	Guard              ast.Node
	Decl               ast.Node
	Probability        *Probability
	Name               string
	SourceMultiplicity *ast.Multiplicity
	TargetMultiplicity *ast.Multiplicity
	// Carries marks the succession of a succession flow, which delivers a value
	// as well as ordering its ends.
	Carries bool
	// Gate is the succession that leads to a succession flow, whose guard and name the
	// edge takes; nil when the flow follows its source unconditionally.
	Gate ast.Node
}

// Statement is one lowered statement in an action node's body. Statements are
// kept in declaration order so the executor never walks the node's members
// again to find them.
type Statement interface {
	statement()
}

// Send is a lowered send statement. Message stays an expression because its
// value is only known at execution time.
//
// Target is the name the send addressed, empty for a broadcast. IsVia records
// that the name is a port of the sender rather than a receiver, in which case it
// is the whole path the port was written as and the message goes to whatever the
// graph's Connections join that port to.
//
// TargetSym is the feature a via target resolves to where it was written, so
// routing matches the feature the name denotes rather than the name alone: a
// port a behavior declares under a connected port's name shadows that port.
// It is nil where the scope tree alone does not resolve the path.
type Send struct {
	Message   ast.Node
	Target    string
	TargetSym *symbols.Symbol
	Node      ast.Node
	// TargetPath records that Target is a feature chain (`a.b`) reaching through
	// the sender's features, rather than a name in a namespace (`R`, `P::R`).
	TargetPath bool
	// TargetExpr is the receiver expression of an addressed send as written; the
	// runtime evaluates it to objects where Target carries no name or path.
	TargetExpr ast.Node
	IsVia      bool
	// ViaSelf records a via path written from `this`, whose root is a feature of
	// the sender even where the behavior binds that name to another object.
	ViaSelf bool
	// Receiver is the name addressed by a routed send, empty when omitted.
	Receiver     string
	ReceiverPath bool
	// ReceiverExpr is the `to` expression of a routed send, likewise evaluated to
	// objects where Receiver carries no name the sender resolves.
	ReceiverExpr ast.Node
	Scope        *symbols.Scope // the scope the statement was declared in
}

func (Send) statement() { /* marker: closed Statement set */ }

// Assign is a lowered assignment: `assign <Target> := <Value>`. Target is the
// feature written — the target's last segment — empty when the target names no
// feature.
type Assign struct {
	Target string
	// Qualified marks a namespace-qualified target (`Scope::azimuth`): it names
	// the feature on the object its qualifier names, no host binding applies.
	Qualified bool
	// Owner is the qualifier's symbol (`Probe` in `Probe::count`) and Feature the
	// feature it names; both are set only when Qualified is.
	Owner   *symbols.Symbol
	Feature *symbols.Symbol
	// Chain is the chained target the assignment writes through (`s.reading`),
	// nil when the target was a plain name the body's host binds.
	Chain *AssignTarget
	Value ast.Node
	Node  ast.Node       // the statement itself, for diagnostics
	Scope *symbols.Scope // the scope the statement was declared in
}

func (Assign) statement() { /* marker: closed Statement set */ }

// AssignTarget is a chained assignment target: `assign a.b.c := v` walks `b`
// from `a` and writes `c` on the object it reaches.
type AssignTarget struct {
	// Base is the expression the chain starts from, evaluated in the statement's
	// own scope.
	Base ast.Node
	// Steps are the features walked from Base to the object written, in order.
	Steps []string
	// Text is the target as written (`a.b.c`), for diagnostics.
	Text string
}

// assignTarget reports the chained target an assignment states, flattening a
// nested chain into one walk. It reports false for a plain or
// namespace-qualified name, neither of which reaches through an object.
func assignTarget(node ast.Node) (*AssignTarget, string, bool) {
	chain, ok := node.(*ast.FeatureChainExpr)
	if !ok {
		return nil, "", false
	}
	base, segments := flattenChain(chain)
	text := FeaturePath(node)
	if base == nil || len(segments) == 0 || text == "" {
		return nil, "", false
	}
	return &AssignTarget{
		Base:  base,
		Steps: segments[:len(segments)-1],
		Text:  text,
	}, segments[len(segments)-1], true
}

// flattenChain returns the node a feature chain starts from and every feature
// segment walked from it: `a.b.c` is one walk from `a`, not a walk through the
// value of `a.b`. It returns no segments when one of them names nothing.
func flattenChain(chain *ast.FeatureChainExpr) (ast.Node, []string) {
	var segments []string
	for {
		if chain.Member == nil || len(chain.Member.Parts) == 0 {
			return nil, nil
		}
		names := make([]string, 0, len(chain.Member.Parts))
		for _, part := range chain.Member.Parts {
			if part.Text == "" {
				return nil, nil
			}
			names = append(names, part.Text)
		}
		segments = append(names, segments...)
		inner, nested := chain.Operand.(*ast.FeatureChainExpr)
		if !nested {
			return chain.Operand, segments
		}
		chain = inner
	}
}

// Declare is a lowered declaration in a body-local block: `attribute i = 0;`
// written inside a loop or an `if` branch. The name it declares is a member of
// that block, so the executor binds it in the block's own frame and discards it
// when the block exits. Value is nil when the declaration carried none.
type Declare struct {
	Name  string
	Value ast.Node
	Node  ast.Node       // the declaration itself, for diagnostics
	Scope *symbols.Scope // the scope the declaration was written in
}

func (Declare) statement() { /* marker: closed Statement set */ }

// DeclareUsage is a calc usage declared in a body-local block: `calc p : Pair {
// in k = h; }` written inside a loop or an `if` branch. It states no step of the
// computation; it declares the usage and marks where it becomes reachable, so
// the statements after it read its outputs from one evaluation of its body per
// execution of the block.
type DeclareUsage struct {
	Name  string
	Node  *ast.Usage     // the declaration itself, for diagnostics
	Scope *symbols.Scope // the scope the usage was declared in
}

func (DeclareUsage) statement() { /* marker: closed Statement set */ }

// Block is a lowered body-local statement list: the body of a loop or of one
// branch of a conditional. It is a namespace of its own (symbols/builder.go), so
// the names its Declare statements introduce do not leak out of it.
type Block struct {
	Statements []Statement
	Node       ast.Node // the loop or branch the block belongs to
	// Order is the lowered statement order for a calculation or constraint block.
	// Action blocks leave it nil and derive their order from the enclosing graph.
	Order *StatementOrder
	// Scope is the block's own scope, which its declarations, and a loop's
	// condition, resolve in.
	Scope *symbols.Scope
	// Graph is the block's own token flow, present where a member of the block is
	// an action node rather than a statement — a nested action declaration, a
	// `perform` — which only a flow of its own executes with the succession
	// semantics it has (block_graph.go). Statements is empty for such a block: the
	// statements are the bodies of the flow's nodes.
	Graph *ActionGraph
	// Own marks the flow a case body states of its own (case_body.go), which runs
	// in the body's frame rather than a block's so its results read what it left.
	Own bool
	// Stated marks Graph as the token flow the body's successions, control nodes
	// or accepts state; unset, Graph runs the body's steps in declaration order.
	Stated bool
}

// A block is a statement in its own right: the anonymous action usage a loop or
// branch body is written as (`loop action { … } until c;`) runs its statements
// in its own namespace.
func (Block) statement() { /* marker: closed Statement set */ }

// Loop is a lowered loop statement. Kind says when the condition is tested:
// before each iteration (`while`), after each iteration (`loop … until`), or
// not at all, iteration being driven by a collection (`for`).
//
// Condition and Collection stay expressions because their values are only known
// at execution time. The iteration count is bounded by the executor's step
// budget, so a loop that never terminates fails the run rather than hanging it.
type Loop struct {
	Kind      ast.LoopKind
	Condition ast.Node // nil for `for`, and for a `loop` written without `until`
	// Until is the condition a `while` loop's `until` clause tests after each
	// iteration (`while c { … } until d;`), nil when it carries none.
	Until      ast.Node
	Variable   string   // `for` only: the name each element is bound to
	Collection ast.Node // `for` only: the collection iterated over
	Body       Block
	Node       ast.Node // the loop itself, for diagnostics
	// Scope is the scope the loop was declared in, which its collection resolves
	// in; its condition resolves in Body.Scope, which the body declares into.
	Scope *symbols.Scope
}

func (Loop) statement() { /* marker: closed Statement set */ }

// If is a lowered conditional. The condition is evaluated in the enclosing
// body, outside both branches. Else is nil when the conditional declared none.
type If struct {
	Condition ast.Node
	Then      Block
	Else      *Block
	Node      ast.Node       // the conditional itself, for diagnostics
	Scope     *symbols.Scope // the scope the conditional, and so its condition, was declared in
}

func (If) statement() { /* marker: closed Statement set */ }

// Return is a lowered `return`: the value the enclosing behavior computes,
// possibly from inside a block. Value is nil when it named no expression.
type Return struct {
	Value ast.Node
	Node  ast.Node       // the return itself, for diagnostics
	Scope *symbols.Scope // the scope the returned expression was written in
}

func (Return) statement() { /* marker: closed Statement set */ }

// EffectKind names a statement that acts on the world outside the body it
// stands in.
type EffectKind int

const (
	EffectPerform EffectKind = iota
	EffectAccept
	EffectTerminate
	// EffectStart is `perform obj.beh.start;`: the behavior the chain names begins on
	// its object and runs on its own, the statement done once it has started.
	EffectStart
)

func (k EffectKind) String() string {
	switch k {
	case EffectPerform:
		return "perform"
	case EffectAccept:
		return "accept"
	case EffectTerminate:
		return "terminate"
	case EffectStart:
		return "start"
	default:
		return "effect"
	}
}

// Effect is a statement acting on the world outside the body — perform, accept,
// terminate, start — lowered so a host rejecting it (a calculation) can say so.
type Effect struct {
	Kind  EffectKind
	Node  ast.Node
	Scope *symbols.Scope // the scope the statement was declared in
	// Terminates says what a terminate ends; Target is the action node it names
	// (TerminateNode), TargetExpr the target as written, nil for none. For a
	// start, Target is the behavior started (`obj.beh`) and TargetExpr its `start`.
	Terminates TerminateTarget
	Target     ast.Node
	TargetExpr ast.Node
}

// performEffect lowers a `perform`: one naming the `start` of a behavior held by
// an object (`perform obj.beh.start`) starts it, any other performs what it names.
func performEffect(node ast.Node, scope *symbols.Scope) Effect {
	if started, ref, ok := startedBehavior(node, scope); ok {
		return Effect{Kind: EffectStart, Node: node, Scope: scope, Target: started, TargetExpr: ref}
	}
	return Effect{Kind: EffectPerform, Node: node, Scope: scope}
}

// startedBehavior reports the behavior a perform starts: the operand of a
// reference chain ending in the `start` shot every behavior has, the operand
// itself a behavior an object holds where the scope can say what it names.
func startedBehavior(node ast.Node, scope *symbols.Scope) (started, ref ast.Node, ok bool) {
	var target ast.Node
	switch n := node.(type) {
	case *ast.PerformActionNode:
		target = n.ActionRef
	case *ast.Usage:
		for _, rel := range n.Relationships {
			if rel != nil && rel.Kind == ast.RelReferences {
				target = rel.Target
				break
			}
		}
	}
	chain, isChain := target.(*ast.FeatureChainExpr)
	if !isChain || chain.Member == nil || len(chain.Member.Parts) != 1 || chain.Member.Parts[0].Text != ast.StartFeature {
		return nil, nil, false
	}
	switch chain.Operand.(type) {
	case *ast.FeatureChainExpr, *ast.QualifiedName, *ast.FeatureReference:
	default:
		return nil, nil, false
	}
	if !namesStartableBehavior(chain.Operand, scope) {
		return nil, nil, false
	}
	return chain.Operand, chain, true
}

// namesStartableBehavior reports whether the `start` of the feature a path names is
// the shot every behavior has, not a `start` the feature's type declares itself; a
// path the scope cannot follow is taken as written and left to the runtime to refuse.
func namesStartableBehavior(operand ast.Node, scope *symbols.Scope) bool {
	path := FeaturePath(operand)
	if scope == nil || path == "" {
		return true
	}
	segments := strings.Split(path, ".")
	sym, ok := resolve.FeatureSymbolInScope(scope, segments)
	if !ok || sym == nil || sym.Decl == nil {
		return true
	}
	if _, startable := StartableBehaviorOf(sym.Decl); startable {
		return true
	}
	_, declared := resolve.FeatureSymbolInScope(scope, append(segments, ast.StartFeature))
	return !declared
}

// TerminateTarget is what a terminate names, settled where it was written.
type TerminateTarget int

const (
	// TerminateContaining is a terminate naming nothing: the containing performance ends.
	TerminateContaining TerminateTarget = iota
	// TerminateEnclosing is a terminate action usage: the performance its node is a step of ends.
	TerminateEnclosing
	// TerminateNode names an action usage (Effect.Target): a node of an enclosing flow,
	// or else the action occurrence the name denotes, evaluated as TerminateOccurrence is.
	TerminateNode
	// TerminateOccurrence names an occurrence by an expression the executor evaluates.
	TerminateOccurrence
)

// terminateTarget settles what a terminate written in scope names: a name reaching an
// action usage is a node; anything else is an expression denoting an occurrence.
func terminateTarget(m *ast.TerminateStatement, scope *symbols.Scope) (ast.Node, TerminateTarget) {
	if m.Target == nil {
		return nil, TerminateContaining
	}
	qn := ast.AsQualifiedName(m.Target)
	if qn == nil || len(qn.Parts) == 0 {
		return nil, TerminateOccurrence
	}
	if node, _, found, _ := resolve.ActionNodeInScope(scope, qn); found {
		return node, TerminateNode
	}
	segments := make([]string, len(qn.Parts))
	for i, part := range qn.Parts {
		segments[i] = part.Text
	}
	if sym, ok := resolve.FeatureSymbolInScope(scope, segments); ok {
		if usage, isUsage := sym.Decl.(*ast.Usage); isUsage && usage.Kind == ast.UsageAction {
			return usage, TerminateNode
		}
	}
	return nil, TerminateOccurrence
}

func (Effect) statement() { /* marker: closed Statement set */ }

// Unsupported is a body member the lowering layer recognizes but cannot yet
// turn into an executable statement. It is lowered rather than dropped so that
// reaching it fails the execution with a diagnostic instead of silently
// producing a wrong answer. Description names the construct.
type Unsupported struct {
	Description string
	Node        ast.Node
	Scope       *symbols.Scope // the scope the member was declared in
}

func (Unsupported) statement() { /* marker: closed Statement set */ }

// Assert is an `assert constraint` a succession orders among a flow's steps: the
// flow reaching it checks the conditions Sym states.
type Assert struct {
	Node  *ast.Usage
	Sym   *symbols.Symbol // the assertion's symbol, nil where the scope does not declare it
	Scope *symbols.Scope  // the scope the assertion was declared in
}

func (Assert) statement() { /* marker: closed Statement set */ }

// Accept is a lowered accept parameter: `action r accept msg : Warning;`.
// SignalType is the parameter's declared type name as written, nil when it was
// declared without one, in which case the node accepts a message of any type.
//
// ViaPort is the port named by `accept msg : Warning via p`, empty when the
// accept named none. A port-routed message is only offered to an accept on the
// port it arrived at, so the two forms do not consume each other's messages.
//
// SubsetsEvent is the event feature the payload subsets (`accept :> shutDown`),
// as written, nil for none: the accept waits for that one event.
//
// Trigger is the time or change event of `accept at t` / `accept after d` /
// `accept when c`, nil when the accept waits for a message instead.
type Accept struct {
	ParamName    string
	SignalType   *ast.QualifiedName
	ViaPort      string
	ViaSelf      bool // ViaPort was written from `this`, as Send.ViaSelf
	SubsetsEvent ast.Node
	Trigger      ast.Node
	// Scope is the scope the accept was declared in, in which SignalType resolves.
	Scope *symbols.Scope
	// Keeper reports the accept keeps the signal for a state deferring it, as a
	// DeferredKeeper annotation on the node declares (`#MigrationMetadata::DeferredKeeper
	// action receive accept kept : Sig;`): it takes an occurrence no other accept
	// of the state's behavior is ready for.
	Keeper bool
}

// Attribute is a lowered attribute default written among a behavior's members
// (`attribute h : LengthValue = 500.0 [m];`), whose Value resolves in the
// graph's own scope.
type Attribute struct {
	Name string
	// Direction is the parameter direction written, DirNone for a plain attribute.
	Direction ast.FeatureDirection
	IsResult  bool // a `return` parameter, what the behavior yields
	// Type is the declared type as written (`Natural`, `Vehicle::Mode`), "" without one.
	Type  string
	Value ast.Node
	Node  ast.Node // the declaration itself, for diagnostics
	// Scope is the scope the declaration was written in, in which its default
	// resolves; nil where the owner's own scope resolves it.
	Scope *symbols.Scope
	// Optional reports an effective multiplicity with lower bound 0 (`x : Integer[0..1]`):
	// the feature may hold no value at all. The run resolves it, as it does Scope.
	Optional bool
}

// TypeText spells the type a usage declares with `:` as the notation writes it
// (each segment quoted when it must be), or "" without one.
func TypeText(u *ast.Usage) string {
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		segments := make([]string, 0, len(qn.Parts))
		for _, part := range qn.Parts {
			segments = append(segments, source.NameText(part.Text))
		}
		text := strings.Join(segments, "::")
		if qn.Global {
			text = "$::" + text
		}
		return text
	}
	return ""
}

// Output reports whether the feature is written back rather than read: an `out`
// or `return` parameter, which no caller and no witness may fix.
func (a Attribute) Output() bool {
	return a.Direction == ast.DirOut || a.IsResult
}

// Feature is one parameter or attribute an action node declares itself. Value
// is its declared value (nil when none), resolving in Scope, the node's scope.
type Feature struct {
	Name      string
	Direction ast.FeatureDirection
	IsResult  bool // a `return` parameter, what the node stands for read as a value
	Value     ast.Node
	Node      ast.Node // the declaration, for diagnostics
	Scope     *symbols.Scope
}

// Output reports whether the feature is written back rather than read: an `out`
// or `return` parameter.
func (f Feature) Output() bool {
	return f.Direction == ast.DirOut || f.IsResult
}

// PinBinding is a binding connector with an end at pin Pin of Node — or, where Path
// is set, of the node Path reaches under it through the flows each owns (`leg.inner.v`:
// Node leg, Path [inner], Pin v). Other is the other end as written; OtherNode,
// OtherPath and OtherPin name the node pin it addresses, if any.
type PinBinding struct {
	Node      ast.Node
	Path      []ast.Node
	Pin       string
	Other     ast.Node
	OtherNode ast.Node
	OtherPath []ast.Node
	OtherPin  string
	// OtherChain is the chain the other end walks to the object whose feature
	// OtherFeature it names (`holder.inner.mark`); nil for a node's pin or a plain name.
	OtherChain   *AssignTarget
	OtherFeature string
	// OtherOwner is the qualifier's symbol when the other end was qualified
	// (`Bench::level`), and OtherFeatureSym the feature it names.
	OtherOwner      *symbols.Symbol
	OtherFeatureSym *symbols.Symbol
	// OtherParameter is the parameter of the action itself the other end names
	// (`toast`, `ToastBread::bread`, one a usage inherits from its type), "" for
	// an end naming none; the action's own frame holds that pin.
	OtherParameter string
	Scope          *symbols.Scope // the scope the binding was written in
	Decl           *ast.Usage
	// FromValue marks the binding a pin's own value states (`inout n = ticks;`): the
	// value is the pin's initial value alone when no feature around the node holds it.
	FromValue bool
}

// FlowKind is how a data flow carries its values: streaming, as a `flow` is unless
// designated otherwise (Flows::Flow), or as a succession flow (Flows::SuccessionFlow).
type FlowKind int

const (
	// FlowStreaming delivers each value the source pin takes to the target's ongoing
	// performances, or ahead of its next one while none is under way.
	FlowStreaming FlowKind = iota
	// FlowSuccession delivers the value the source pin holds once the source
	// completes, and the target begins no sooner.
	FlowSuccession
)

// String names the kind as the notation spells it.
func (k FlowKind) String() string {
	if k == FlowSuccession {
		return "succession flow"
	}
	return "flow"
}

// ObjectFlow represents a data flow edge between pins.
type ObjectFlow struct {
	// Name is the flow's own name, when it was declared with one
	// (`flow generateToAmplify from a.out to b.in;`), and "" for the anonymous
	// form and for a flow the notation writes as an edge.
	Name      string
	SourcePin string
	TargetPin string
	Target    ast.Node
	// Kind is how the flow carries its values, as its declaration designated.
	Kind FlowKind
	// Decl is the declaration the flow was written as, for a consumer that
	// reports where it comes from.
	Decl ast.Node
	// Gate is the succession whose target is this succession flow
	// (`first a if g then f;`): the flow moves its value only when that succession is taken.
	Gate ast.Node
}

// ToActionGraph converts an action AST (Usage or Definition) to an ActionGraph.
// scope is the scope the action's body was declared in — the scope the action
// itself owns — which every expression the graph carries is evaluated in.
// Returns error if graph is malformed (e.g., no initial node, dangling edges).
// Without the name-resolution tier's resolver a Probability annotation cannot be
// told from any other metadata, so none is read; a caller holding one uses
// ToActionGraphWith.
func ToActionGraph(actionDecl ast.Node, scope *symbols.Scope) (*ActionGraph, error) {
	return ToActionGraphWith(actionDecl, scope, nil)
}

// ToActionGraphWith is ToActionGraph reading the metadata the resolver identifies:
// a succession's `@Probability { p = ...; }` becomes its edge's weight.
func ToActionGraphWith(actionDecl ast.Node, scope *symbols.Scope, resolver *resolve.Resolver) (*ActionGraph, error) {
	members, err := actionMembers(actionDecl)
	if err != nil {
		return nil, err
	}
	graph, err := lowerActionFlow(members, scope, resolver)
	if err != nil {
		return nil, err
	}
	return graph, nil
}

func lowerActionFlow(members []ast.Node, scope *symbols.Scope, resolver *resolve.Resolver) (*ActionGraph, error) {
	graph, err := collectActionNodes(members, scope, resolver)
	if err != nil {
		return graph, err
	}
	lowerValueBindings(graph)
	// The initial node is optional at graph construction time; the executor's
	// initialize() reports its absence.
	edges := &actionEdgeLowerer{graph: graph, scope: scope, weights: &probabilityReader{resolver: resolver, scope: scope}}
	for _, member := range members {
		if err := edges.member(unwrapMembership(member)); err != nil {
			return graph, err
		}
	}
	if err := lowerInheritedPinConnections(graph, scope); err != nil {
		return graph, err
	}
	if err := edges.resolveGates(); err != nil {
		return graph, err
	}
	if err := checkProbabilities(graph); err != nil {
		return graph, err
	}
	recordBlockNodes(graph)
	encloseBlockFlows(graph)
	return graph, nil
}

// actionEdgeLowerer lowers the members of an action body that connect its nodes,
// once the nodes themselves are collected.
type actionEdgeLowerer struct {
	graph   *ActionGraph
	scope   *symbols.Scope
	weights *probabilityReader
	// gates are the successions leading to a name no node answers to, resolved
	// against the succession flows once every flow is lowered.
	gates []pendingGate
}

// pendingGate is a succession whose target names no node: the edge it states,
// its source resolved, and the reference its target end wrote.
type pendingGate struct {
	edge   ActionEdge
	target ast.Node
}

func (l *actionEdgeLowerer) member(member ast.Node) error {
	switch n := member.(type) {
	case *ast.InitialNode:
		return l.initial(n)
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		return l.weights.refuseStrayIn(nil, ast.NodeBodyMembers(n))
	case *ast.SuccessionEdge:
		return l.successionEdge(n)
	case *ast.ControlFlowEdge:
		return l.controlFlowEdge(n)
	case *ast.TransitionMember:
		return l.transition(n)
	case *ast.ObjectFlowEdge:
		return l.objectFlowEdge(n)
	case *ast.Usage:
		return l.usage(n)
	case *ast.PrefixMetadata:
		return l.weights.refuseStray(n)
	}
	return nil
}

// initial lowers `first a then b;`, the succession a -> b, as `succession first a then b;` is.
func (l *actionEdgeLowerer) initial(n *ast.InitialNode) error {
	if n.Successor == nil {
		return l.weights.refuseStrayIn(nil, n.Members)
	}
	if !annotationsOnly(n.Members) {
		return fmt.Errorf("action succession has unsupported body")
	}
	weight, err := l.weights.read(n.Members)
	if err != nil {
		return err
	}
	return l.succession(n.First, n.Successor, ActionEdge{Guard: n.Guard, Decl: n, Probability: weight})
}

func (l *actionEdgeLowerer) successionEdge(n *ast.SuccessionEdge) error {
	sourceNode := resolveActionEndpointForEdge(l.graph, n.Source, n.SourceMember, true)
	targetNode := resolveActionEndpointForEdge(l.graph, n.Target, n.TargetMember, false)
	if sourceNode == nil {
		return fmt.Errorf("succession edge references undefined source node %s", edgeEnd(n.Source, n.SourceMember))
	}
	if targetNode == nil {
		return fmt.Errorf("succession edge references undefined target node %s", edgeEnd(n.Target, n.TargetMember))
	}
	weight, err := l.weights.read(n.Members)
	if err != nil {
		return err
	}
	l.graph.Edges[sourceNode] = append(l.graph.Edges[sourceNode], ActionEdge{
		Source:             sourceNode,
		Target:             targetNode,
		Decl:               n,
		Probability:        weight,
		SourceMultiplicity: n.SourceMultiplicity,
		TargetMultiplicity: n.TargetMultiplicity,
	})
	return nil
}

func (l *actionEdgeLowerer) controlFlowEdge(n *ast.ControlFlowEdge) error {
	sourceNode := resolveActionEndpointForEdge(l.graph, n.Source, n.SourceMember, true)
	targetNode := resolveActionEndpointForEdge(l.graph, n.Target, n.TargetMember, false)
	if sourceNode == nil {
		return fmt.Errorf("control flow edge references undefined source %s", edgeEnd(n.Source, n.SourceMember))
	}
	if targetNode == nil {
		return fmt.Errorf("control flow edge references undefined target %s", edgeEnd(n.Target, n.TargetMember))
	}
	l.graph.Edges[sourceNode] = append(l.graph.Edges[sourceNode], ActionEdge{
		Source: sourceNode,
		Target: targetNode,
		Guard:  n.Guard,
		Decl:   n,
	})
	return nil
}

func (l *actionEdgeLowerer) transition(n *ast.TransitionMember) error {
	sourceNode := resolveActionEndpoint(l.graph, n.Source, true)
	if sourceNode == nil {
		return fmt.Errorf("succession references undefined source node %s", edgeEndName(n.Source))
	}
	weight, err := l.weights.read(n.Members)
	if err != nil {
		return err
	}
	edge := ActionEdge{Source: sourceNode, Guard: n.Guard, Decl: n, Probability: weight, Name: n.Name}
	targetNode := resolveActionEndpoint(l.graph, n.Target, false)
	if targetNode == nil {
		if n.Target != nil && ast.SimpleName(n.Target) != "" {
			l.gates = append(l.gates, pendingGate{edge: edge, target: n.Target})
			return nil
		}
		return fmt.Errorf("succession references undefined target node %s", edgeEndName(n.Target))
	}
	edge.Target = targetNode
	l.graph.Edges[sourceNode] = append(l.graph.Edges[sourceNode], edge)
	return nil
}

func (l *actionEdgeLowerer) objectFlowEdge(n *ast.ObjectFlowEdge) error {
	sourceNode, sourcePin := parsePinReference(l.graph.Nodes, n.Source)
	targetNode, targetPin := parsePinReference(l.graph.Nodes, n.Target)
	if sourceNode == nil {
		return fmt.Errorf("object flow edge references undefined source %v", n.Source)
	}
	if targetNode == nil {
		return fmt.Errorf("object flow edge references undefined target %v", n.Target)
	}
	for _, end := range []*ast.QualifiedName{n.Source, n.Target} {
		if err := flowEndReaches(end); err != nil {
			return fmt.Errorf("object flow edge: %w", err)
		}
	}
	l.graph.DataFlows[sourceNode] = append(l.graph.DataFlows[sourceNode], ObjectFlow{
		SourcePin: sourcePin,
		TargetPin: targetPin,
		Target:    targetNode,
		Kind:      FlowStreaming,
		Decl:      n,
	})
	return nil
}

func (l *actionEdgeLowerer) usage(n *ast.Usage) error {
	switch n.Kind {
	case ast.UsageAction:
		return l.weights.refuseStrayIn(n.Prefixes, n.Members)
	case ast.UsageBinding:
		bindings, err := lowerPinBindings(l.graph, nodesNamed(l.graph.Nodes), n, l.scope)
		if err != nil {
			return err
		}
		l.graph.Bindings = append(l.graph.Bindings, bindings...)
	case ast.UsageSuccession:
		return l.successionUsage(n)
	case ast.UsageMetadata:
		return l.weights.refuseStray(n)
	case ast.UsageFlow:
		if n.FlowEnds == nil {
			return nil
		}
		source, flow, err := lowerFlow(nodesNamed(l.graph.Nodes), n)
		if err != nil {
			return err
		}
		l.graph.DataFlows[source] = append(l.graph.DataFlows[source], flow)
		succeedFlow(l.graph, source, flow)
	}
	return nil
}

func (l *actionEdgeLowerer) successionUsage(n *ast.Usage) error {
	if len(n.ConnectorEnds) != 2 {
		return fmt.Errorf("action succession must have exactly two connector ends, got %d", len(n.ConnectorEnds))
	}
	if n.Multiplicity != nil {
		return fmt.Errorf("action succession has unsupported multiplicity")
	}
	if !annotationsOnly(n.Members) {
		return fmt.Errorf("action succession has unsupported body")
	}
	for i, end := range n.ConnectorEnds {
		if end.Multiplicity != nil && !hasDeclaredNodeMultiplicity(l.graph, n.ConnectorEnds) {
			return fmt.Errorf("action succession end %d has unsupported multiplicity", i+1)
		}
	}
	weight, err := l.weights.read(n.Members)
	if err != nil {
		return err
	}
	sourceRef := connectorEndReference(n.ConnectorEnds[0])
	targetRef := connectorEndReference(n.ConnectorEnds[1])
	name, _ := ast.EffectiveName(n)
	return l.succession(sourceRef, targetRef, ActionEdge{
		Decl:               n,
		Probability:        weight,
		Name:               name,
		SourceMultiplicity: n.ConnectorEnds[0].Multiplicity,
		TargetMultiplicity: n.ConnectorEnds[1].Multiplicity,
	})
}

func hasDeclaredNodeMultiplicity(graph *ActionGraph, ends []*ast.ConnectorEnd) bool {
	for _, end := range ends {
		node := resolveActionEndpoint(graph, connectorEndReference(end), false)
		if graph.Multiplicities[node] != nil {
			return true
		}
	}
	return false
}

// lowerInheritedPinConnections lowers the bindings and flows the actions the
// action specializes wrote at pins of its nodes, nearest general first: a node
// the action inherits keeps the connections its declaring action stated at it.
func lowerInheritedPinConnections(graph *ActionGraph, scope *symbols.Scope) error {
	for _, body := range resolve.ActionGeneralBodies(scope) {
		graph.inherited = append(graph.inherited, Inherited{Decl: body.Node(), Body: body})
		nodes := inheritedNodeLookup(graph, body)
		for _, member := range ast.DeclMembers(body.Node()) {
			u, ok := unwrapMembership(member).(*ast.Usage)
			if !ok {
				continue
			}
			switch u.Kind {
			case ast.UsageBinding:
				if bindsReplacedNode(nodes, body, u) {
					continue
				}
				bindings, err := lowerPinBindings(graph, nodes, u, body)
				if err != nil {
					return err
				}
				graph.recordDeclaredIn(u, body)
				graph.Bindings = append(graph.Bindings, bindings...)
			case ast.UsageFlow:
				if u.FlowEnds == nil {
					continue
				}
				if from, _ := flowEnd(nodes, u.FlowEnds.From); from == nil {
					continue
				}
				if to, _ := flowEnd(nodes, u.FlowEnds.To); to == nil {
					continue
				}
				source, flow, err := lowerFlow(nodes, u)
				if err != nil {
					return err
				}
				graph.recordDeclaredIn(u, body)
				graph.DataFlows[source] = append(graph.DataFlows[source], flow)
				succeedFlow(graph, source, flow)
			}
		}
	}
	return nil
}

// nodeLookup returns the node of a graph a connector end names, nil for none.
type nodeLookup func(name string) ast.Node

// bindsReplacedNode reports whether an end of a general body's binding names a
// node of that body the graph has no node for, so the binding holds at no end.
func bindsReplacedNode(nodes nodeLookup, body *symbols.Scope, u *ast.Usage) bool {
	binding, ok := lowerBinding(u, body)
	if !ok {
		return false
	}
	for _, end := range binding.Ends {
		segments := endSegments(end.Expr)
		if len(segments) == 0 {
			continue
		}
		if _, isNode := resolve.ActionNodeOfBody(body, segments[0]); isNode && nodes(segments[0]) == nil {
			return true
		}
	}
	return false
}

// nodesNamed looks a connector end up among nodes by the names they answer to.
func nodesNamed(nodes []ast.Node) nodeLookup {
	return func(name string) ast.Node { return nodeAnswering(nodes, name) }
}

// inheritedNodeLookup resolves an end a general body's connector writes to the
// node of graph standing for the declaration that name has in that body — the
// declaration itself or a node redefining it — never to an unrelated node that
// took the name in the specializing action.
func inheritedNodeLookup(graph *ActionGraph, body *symbols.Scope) nodeLookup {
	return func(name string) ast.Node {
		decl, ok := resolve.ActionNodeOfBody(body, name)
		if !ok {
			return nil
		}
		for _, node := range graph.Nodes {
			if node == decl {
				return node
			}
		}
		for _, node := range graph.Nodes {
			u, ok := node.(*ast.Usage)
			if !ok {
				continue
			}
			if nodeScope := graph.Scopes[node]; nodeScope != nil && resolve.RedefinesActionNode(nodeScope.Parent(), u, decl) {
				return node
			}
		}
		return nil
	}
}

// resolveFirstNode reinterprets a one-ended `first a;` whose name is a node the
// body declares: a is the flow's first node, so the initial node it parsed as is
// not a node of the graph.
func resolveFirstNode(graph *ActionGraph) error {
	initial, ok := graph.Initial.(*ast.InitialNode)
	if !ok || initial.Name() == "" {
		return nil
	}
	var named ast.Node
	for _, node := range graph.Nodes {
		if node != ast.Node(initial) && nodeAnswersTo(node, initial.Name()) {
			named = node
			break
		}
	}
	if named == nil {
		return nil
	}
	if _, isFinal := named.(*ast.FinalNode); isFinal {
		// A flow cannot start where it ends: naming a final node would retire the
		// token before any succession out of it is taken.
		return fmt.Errorf("first names the final node %s, so the action would end before it started", initial.Name())
	}
	graph.Initial = named
	graph.Nodes = slices.DeleteFunc(graph.Nodes, func(node ast.Node) bool {
		return node == ast.Node(initial)
	})
	return nil
}

// succession adds the edge a succession states between the nodes its two ends resolve
// to; edge carries everything but the resolved Source and Target. A target naming no
// node is left for resolveGates, which reads it as a succession flow.
func (l *actionEdgeLowerer) succession(sourceRef, targetRef ast.Node, edge ActionEdge) error {
	sourceNode := resolveActionEndpoint(l.graph, sourceRef, true)
	if sourceNode == nil {
		return fmt.Errorf("action succession references undefined source node %s", successionEndText(sourceRef))
	}
	edge.Source = sourceNode
	targetNode := resolveActionEndpoint(l.graph, targetRef, false)
	if targetNode == nil {
		if ast.AsQualifiedName(targetRef) != nil && ast.SimpleName(targetRef) != "" {
			l.gates = append(l.gates, pendingGate{edge: edge, target: targetRef})
			return nil
		}
		return fmt.Errorf("action succession references undefined target node %s", successionEndText(targetRef))
	}
	edge.Target = targetNode
	l.graph.Edges[sourceNode] = append(l.graph.Edges[sourceNode], edge)
	return nil
}

// resolveGates reads each succession whose target named no node as leading to the
// succession flow of that name out of its source (`first a if g then f;`): the flow's
// own succession takes the gate's guard, name and weight, and its value moves only
// along it.
func (l *actionEdgeLowerer) resolveGates() error {
	for _, gate := range l.gates {
		source, name := gate.edge.Source, ast.SimpleName(gate.target)
		at := slices.IndexFunc(l.graph.DataFlows[source], func(flow ObjectFlow) bool { return flow.Name == name })
		if at < 0 {
			if from := flowSourceNamed(l.graph, name); from != nil {
				return fmt.Errorf("action succession leads to flow %s, which leaves %s, not %s",
					name, getNodeName(from), getNodeName(source))
			}
			return fmt.Errorf("action succession references undefined target node %s", successionEndText(gate.target))
		}
		flow := &l.graph.DataFlows[source][at]
		switch {
		case flow.Kind != FlowSuccession:
			return fmt.Errorf("action succession leads to flow %s, which is no succession flow", name)
		case flow.Gate != nil:
			return fmt.Errorf("succession flow %s follows more than one succession", name)
		}
		flow.Gate = gate.edge.Decl
		edges := l.graph.Edges[source]
		i := slices.IndexFunc(edges, func(e ActionEdge) bool { return e.Carries && e.Decl == flow.Decl })
		if i < 0 {
			return fmt.Errorf("succession flow %s states no succession to gate", name)
		}
		gated := gate.edge
		gated.Target, gated.Decl, gated.Carries, gated.Gate = flow.Target, flow.Decl, true, gate.edge.Decl
		edges[i] = gated
	}
	return nil
}

// flowSourceNamed returns the node the flow of that name leaves, nil for none.
func flowSourceNamed(graph *ActionGraph, name string) ast.Node {
	for _, node := range graph.Nodes {
		for _, flow := range graph.DataFlows[node] {
			if flow.Name == name {
				return node
			}
		}
	}
	return nil
}

// lowerPinBindings lowers a binding to one PinBinding per end that addresses a
// pin of a node nodes resolves; a binding addressing no node lowers to nothing.
func lowerPinBindings(graph *ActionGraph, nodes nodeLookup, u *ast.Usage, scope *symbols.Scope) ([]PinBinding, error) {
	binding, ok := lowerBinding(u, scope)
	if !ok {
		return nil, nil
	}
	var out []PinBinding
	for i, end := range binding.Ends {
		node, path, pin, err := pinPath(graph, nodes, end.Expr)
		if err != nil {
			return nil, err
		}
		if node == nil {
			continue
		}
		if pin == "" {
			return nil, fmt.Errorf("binding end %s names an action node but no pin of it", flowEndText(end.Expr))
		}
		other := binding.Ends[1-i].Expr
		otherNode, otherPath, otherPin, err := pinPath(graph, nodes, other)
		if err != nil {
			return nil, err
		}
		binding := PinBinding{
			Node:      node,
			Path:      path,
			Pin:       pin,
			Other:     other,
			OtherNode: otherNode,
			OtherPath: otherPath,
			OtherPin:  otherPin,
			Scope:     scope,
			Decl:      u,
		}
		if otherNode == nil {
			if chain, feature, ok := assignTarget(other); ok {
				binding.OtherChain, binding.OtherFeature = chain, feature
			}
			binding.OtherParameter = ownParameter(scope, graph.Scope, other)
		}
		out = append(out, binding)
	}
	return out, nil
}

// pinPath resolves a binding end to the node nodes resolves it to, the nodes it
// reaches through under that one, and the pin: `leg.inner.v` is leg, [inner], v. node
// is nil for an end addressing no node; a path reaching into a node that holds no
// such nested action is an error.
func pinPath(graph *ActionGraph, nodes nodeLookup, end ast.Node) (node ast.Node, path []ast.Node, pin string, err error) {
	segments := endSegments(end)
	if len(segments) == 0 {
		return nil, nil, "", nil
	}
	node = nodes(segments[0])
	if node == nil || len(segments) == 1 {
		return node, nil, "", nil
	}
	current, in := node, graph
	for _, name := range segments[1 : len(segments)-1] {
		next, flow := nestedNodeNamed(in, current, name)
		if next == nil {
			return nil, nil, "", fmt.Errorf("binding end %s reaches into %s, which %s; bind at a pin of %s itself",
				flowEndText(end), getNodeName(current), holdsNoNested(current, name), getNodeName(current))
		}
		path = append(path, next)
		current, in = next, flow
	}
	return node, path, segments[len(segments)-1], nil
}

// holdsNoNested says why a node has no nested action of this name to reach into: it
// performs another action, whose nodes are that action's own, or it declares none.
func holdsNoNested(node ast.Node, name string) string {
	if u, ok := node.(*ast.Usage); ok && (typingTarget(u) != nil || u.Value != nil) {
		return "performs an action of its own rather than declaring " + name
	}
	return "declares no nested action " + name
}

// endSegments returns the names a connector end chains, outermost first:
// `leg.inner.v` is [leg inner v]. Empty for an end that is not a name.
func endSegments(end ast.Node) []string {
	switch e := end.(type) {
	case *ast.QualifiedName:
		segments := make([]string, 0, len(e.Parts))
		for _, part := range e.Parts {
			segments = append(segments, part.Text)
		}
		return segments
	case *ast.FeatureChainExpr:
		if operand := endSegments(e.Operand); len(operand) > 0 {
			return append(operand, ast.SimpleName(e.Member))
		}
	case *ast.FeatureReference:
		return endSegments(e.Name)
	}
	return nil
}

// nodeAnswering returns the node among nodes that answers to name, if any.
func nodeAnswering(nodes []ast.Node, name string) ast.Node {
	for _, node := range nodes {
		if nodeAnswersTo(node, name) {
			return node
		}
	}
	return nil
}

// lowerFeatures records the parameters and attributes a node declares itself. An
// `inout` pin valued by a feature name is bound to that feature, as a feature value
// binds the feature to its result, so what the node leaves in the pin writes back.
func lowerFeatures(graph *ActionGraph, node *ast.Usage, scope *symbols.Scope) {
	if graph.Features == nil {
		graph.Features = make(map[ast.Node][]Feature)
	}
	recordNodeScope(graph, node, scope)
	var features []Feature
	for _, member := range node.Members {
		m, ok := unwrapMembership(member).(*ast.Usage)
		if !ok {
			continue
		}
		if m.IsAccept {
			// A message payload is the accept's output pin; `accept when/at/after` binds none.
			if m.Value == nil && m.Ident.Name != "" {
				features = append(features, Feature{Name: m.Ident.Name, Direction: ast.DirOut, Node: m, Scope: scope})
			}
			continue
		}
		feature, ok := declaredFeature(m, scope)
		if !ok {
			continue
		}
		features = append(features, feature)
		if binding, ok := inoutValueBinding(node, m, feature.Name, scope); ok {
			graph.Bindings = append(graph.Bindings, binding)
		}
	}
	graph.Features[node] = features
}

// declaredFeature is the parameter or attribute a member declares, false for a
// member declaring none or one with no name.
func declaredFeature(m *ast.Usage, scope *symbols.Scope) (Feature, bool) {
	if !DeclaresNodeFeature(m) {
		return Feature{}, false
	}
	name, _ := ast.EffectiveName(m)
	if name == "" {
		return Feature{}, false
	}
	return Feature{
		Name:      name,
		Direction: m.Direction,
		IsResult:  m.IsResult,
		Value:     m.Value,
		Node:      m,
		Scope:     scope,
	}, true
}

// lowerParameters is the directed parameters an action declares among its own
// members, in order: the pins on its frame.
func lowerParameters(members []ast.Node, scope *symbols.Scope) []Feature {
	var params []Feature
	for _, member := range members {
		m, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || m.IsAccept || m.Direction == ast.DirNone && !m.IsResult {
			continue
		}
		if feature, ok := declaredFeature(m, scope); ok {
			params = append(params, feature)
		}
	}
	return params
}

// lowerValueBindings records the binding each node's directed pin states by its
// value naming a feature, once every node is collected so a value naming another
// node's pin (`in t = heat.t;`) finds that node. A value reaching into a node
// that holds no such pin binds the pin to no node and is left to the executor.
func lowerValueBindings(graph *ActionGraph) {
	nodes := nodesNamed(graph.Nodes)
	for _, node := range graph.Nodes {
		u, ok := node.(*ast.Usage)
		if !ok {
			continue
		}
		for _, feature := range graph.Features[node] {
			pin, ok := feature.Node.(*ast.Usage)
			if !ok || pin.IsAccept {
				continue
			}
			binding, ok := valueBinding(u, pin, feature.Name, feature.Scope)
			if !ok {
				continue
			}
			if other, path, otherPin, err := pinPath(graph, nodes, binding.Other); err == nil && other != nil {
				if otherPin == "" {
					continue
				}
				binding.OtherNode, binding.OtherPath, binding.OtherPin = other, path, otherPin
			} else {
				binding.OtherParameter = ownParameter(feature.Scope, graph.Scope, binding.Other)
			}
			graph.ValueBindings = append(graph.ValueBindings, binding)
		}
	}
}

// ownParameter is the parameter of the action whose body is frame that a
// binding end written in scope names, "" for an end naming none: a bare name
// the frame declares as a parameter, or one of its generals declares (a usage's
// inherited parameter), or the frame's own parameter qualified
// (`ToastBread::bread`). A bare name a scope between the end and the frame
// declares or inherits — a node's own parameter sharing the frame's name — is
// that scope's, not the frame's. An end in a general's body (an inherited
// node's pin) names the frame's parameters as the frame's own body does.
func ownParameter(scope, frame *symbols.Scope, end ast.Node) string {
	segments := endSegments(end)
	if frame == nil || len(segments) == 0 {
		return ""
	}
	name := segments[len(segments)-1]
	if len(segments) > 1 {
		_, owner, sym, ok := qualifiedEndFeature(end, scope)
		if !ok || !isParameter(sym.Decl) || !declaresFrame(frame, owner.Decl) {
			return ""
		}
		return name
	}
	for s := scope; s != nil; s = s.Parent() {
		if declaresFrame(frame, s.Node()) {
			break
		}
		if _, ok := lookupActionLocal(s, name); ok {
			return ""
		}
	}
	if sym, ok := lookupActionLocal(frame, name); ok && isParameter(sym.Decl) {
		return name
	}
	return ""
}

// declaresFrame reports whether decl is the action whose body is frame or one
// of the generals it takes its members from.
func declaresFrame(frame *symbols.Scope, decl ast.Node) bool {
	if frame.Node() == decl {
		return true
	}
	for _, body := range resolve.ActionGeneralBodies(frame) {
		if body.Node() == decl {
			return true
		}
	}
	return false
}

// lookupActionLocal is the symbol a scope declares by name or, for an action's
// body, one of its generals declares: the names its members see before any
// enclosing scope's.
func lookupActionLocal(s *symbols.Scope, name string) (*symbols.Symbol, bool) {
	if sym, ok := s.LookupLocal(name); ok {
		return sym, true
	}
	if !isActionDecl(s.Node()) {
		return nil, false
	}
	for _, body := range resolve.ActionGeneralBodies(s) {
		if sym, ok := body.LookupLocal(name); ok {
			return sym, true
		}
	}
	return nil, false
}

// isParameter reports whether a declaration is a directed parameter or a result.
func isParameter(decl ast.Node) bool {
	u, ok := decl.(*ast.Usage)
	return ok && DeclaresNodeFeature(u) && (u.Direction != ast.DirNone || u.IsResult)
}

// isActionDecl reports whether a node declares an action definition or usage.
func isActionDecl(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.Definition:
		return n.Kind == ast.DefAction
	case *ast.Usage:
		return n.Kind == ast.UsageAction
	}
	return false
}

// inoutValueBinding lowers the value of a node's `inout` pin that names a feature
// (`inout n = ticks;`) to the binding between the two it states; a value that is
// an expression of another kind is the pin's initial value alone. Which of the two
// a name is (`ticks`, or the literal `Mode::idle`) is settled where the node performs.
func inoutValueBinding(node, pin *ast.Usage, name string, scope *symbols.Scope) (PinBinding, bool) {
	if pin.Direction != ast.DirInOut {
		return PinBinding{}, false
	}
	return valueBinding(node, pin, name, scope)
}

// valueBinding is the binding a directed pin's value states by naming a feature
// (`in b = bread;`, `inout n = ticks;`, `out x :>> x = y;`), as written; false
// for an undirected pin or a value that is an expression of another kind, which
// is the pin's initial value alone.
func valueBinding(node, pin *ast.Usage, name string, scope *symbols.Scope) (PinBinding, bool) {
	if pin.Direction == ast.DirNone && !pin.IsResult || pin.Value == nil || len(endSegments(pin.Value)) == 0 {
		return PinBinding{}, false
	}
	binding := PinBinding{Node: node, Pin: name, Other: pin.Value, Scope: scope, Decl: pin, FromValue: true}
	if chain, feature, ok := assignTarget(pin.Value); ok {
		binding.OtherChain, binding.OtherFeature = chain, feature
		return binding, true
	}
	if feature, owner, sym, ok := qualifiedEndFeature(pin.Value, scope); ok {
		binding.OtherFeature, binding.OtherOwner, binding.OtherFeatureSym = feature, owner, sym
	}
	return binding, true
}

// qualifiedEndFeature names the feature a qualified path ends in (`Bench::level`
// ends in `level`), when the path resolves to one where it was written: it binds
// the pin to that feature on the object the qualifier names, so the pin writes
// back. A qualified name of another kind (`Mode::idle`) holds a value, not a
// feature. The owner and feature symbols are the qualifier and what it names.
func qualifiedEndFeature(node ast.Node, scope *symbols.Scope) (string, *symbols.Symbol, *symbols.Symbol, bool) {
	if ref, ok := node.(*ast.FeatureReference); ok {
		node = ref.Name
	}
	qn, ok := node.(*ast.QualifiedName)
	if !ok || len(qn.Parts) < 2 {
		return "", nil, nil, false
	}
	segments := make([]string, 0, len(qn.Parts))
	for _, part := range qn.Parts {
		if part.Text == "" {
			return "", nil, nil, false
		}
		segments = append(segments, part.Text)
	}
	if sym, ok := resolve.FeatureSymbolInScope(scope, segments); ok && sym != nil {
		owner, named := resolve.FeatureSymbolInScope(scope, segments[:len(segments)-1])
		if named && owner != nil && owner.Kind != symbols.SymbolPackage && owner.Kind != symbols.SymbolNamespace {
			return segments[len(segments)-1], owner, sym, true
		}
	}
	return "", nil, nil, false
}

// DeclaresNodeFeature reports whether an action member is a parameter or attribute.
func DeclaresNodeFeature(m *ast.Usage) bool {
	if m.IsAccept || m.IsBodyParameter {
		return false
	}
	switch m.Kind {
	case ast.UsageAction, ast.UsageFlow, ast.UsageBinding, ast.UsageSuccession, ast.UsageConnection:
		return false
	}
	return m.Direction != ast.DirNone || m.Kind == ast.UsageAttribute
}

// lowerBody records a nested action node's statements and the message it waits
// for, so the executor reads them from the graph rather than walking the node's
// members again.
func lowerBody(graph *ActionGraph, node *ast.Usage, scope *symbols.Scope) {
	for _, member := range BodyStatementMembers(node.Members) {
		graph.Bodies[node] = append(graph.Bodies[node], lowerStatement(unwrapMembership(member), scope))
	}
	lowerAccept(graph, node, scope)
}

// lowerNodeBody records the statements the body of an action node declares, so
// a body the notation admits on a control node or a succession executes when a
// token reaches it rather than being dropped.
func lowerNodeBody(graph *ActionGraph, node ast.Node, members []ast.Node, scope *symbols.Scope) {
	body := childScope(scope, node)
	for _, member := range BodyStatementMembers(members) {
		graph.Bodies[node] = append(graph.Bodies[node], lowerStatement(unwrapMembership(member), body))
	}
}

// lowerControlFeatures records the directed features a fork, join or merge declares
// (`in ref inputObject1; out ref outputObject1 = inputObject1;`): the values the
// object flows through it carry in and out.
func lowerControlFeatures(graph *ActionGraph, node ast.Node, scope *symbols.Scope) {
	if !CarriesObjects(node) {
		return
	}
	body := childScope(scope, node)
	var features []Feature
	for _, member := range ast.NodeBodyMembers(node) {
		m, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || m.Direction != ast.DirIn && m.Direction != ast.DirOut {
			continue
		}
		if feature, ok := declaredFeature(m, body); ok {
			features = append(features, feature)
		}
	}
	if len(features) == 0 {
		return
	}
	if graph.Features == nil {
		graph.Features = make(map[ast.Node][]Feature)
	}
	recordNodeScope(graph, node, body)
	graph.Features[node] = features
}

// CarriesObjects reports whether node is a control node object flows pass
// through, by the features it declares: a fork, a join or a merge.
func CarriesObjects(node ast.Node) bool {
	switch node.(type) {
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode:
		return true
	}
	return false
}

// BodyStatementMembers returns the members of a node body that state work to
// perform, in declaration order: what a body declares (a parameter, a doc
// comment) is a feature of the node, not a step of the flow through it.
func BodyStatementMembers(members []ast.Node) []ast.Node {
	var stmts []ast.Node
	for _, member := range members {
		switch m := unwrapMembership(member).(type) {
		case *ast.SendStatement, *ast.AssignmentActionNode, *ast.WhileLoopActionNode,
			*ast.IfActionNode, *ast.TerminateStatement:
			stmts = append(stmts, member)
		case *ast.Usage:
			// A declared action is a feature of the node; only one naming the action
			// it performs is a step (`perform a;`).
			if m.Kind == ast.UsageAction && !m.IsBodyParameter && performsAction(m) {
				stmts = append(stmts, member)
			}
		}
	}
	return stmts
}

// lowerStatement lowers one executable body statement, in the scope it was
// written in. Every form it recognizes is lowered losslessly; a form it does not
// becomes Unsupported, so the executor reports it rather than skipping it.
func lowerStatement(member ast.Node, scope *symbols.Scope) Statement {
	switch m := member.(type) {
	case *ast.SendStatement:
		return lowerSend(m, scope)
	case *ast.AssignmentActionNode:
		return lowerAssignment(m, scope)
	case *ast.WhileLoopActionNode:
		variable, _ := m.Variable.DeclaredName()
		return Loop{
			Kind:       m.Kind,
			Condition:  m.Condition,
			Until:      m.Until,
			Variable:   variable,
			Collection: m.Collection,
			Body:       lowerBlock(m, m.Body, childScope(scope, m)),
			Node:       m,
			Scope:      scope,
		}
	case *ast.IfActionNode:
		lowered := If{Condition: m.Condition, Node: m, Scope: scope}
		if m.Then != nil {
			lowered.Then = lowerBlock(m.Then, m.Then.Body, childScope(scope, m.Then))
		}
		if m.Else != nil {
			block := lowerBlock(m.Else, m.Else.Body, childScope(scope, m.Else))
			lowered.Else = &block
		}
		return lowered
	case *ast.PerformActionNode:
		return performEffect(m, scope)
	case *ast.TerminateStatement:
		target, terminates := terminateTarget(m, scope)
		return Effect{Kind: EffectTerminate, Node: m, Scope: scope, Terminates: terminates, Target: target, TargetExpr: m.Target}
	case *ast.Usage:
		return lowerUsageStatement(m, scope)
	default:
		return Unsupported{Description: statedFlowKeyword(member), Node: member, Scope: scope}
	}
}

// lowerSend lowers a send. A target is either a chain through features
// (`alpha.inPort`) or a name in a namespace (`P::Driver`), which resolve
// differently. A `via` target names a port of the sender, rendered as connector
// ends are so the two match.
func lowerSend(m *ast.SendStatement, scope *symbols.Scope) Statement {
	target, isPath := SendTarget(m.Target)
	var targetSym *symbols.Symbol
	var viaSelf bool
	if m.IsVia {
		target, viaSelf = ViaPortPath(m.Target)
		isPath = true
		targetSym, _ = resolve.FeatureSymbolInScope(scope, strings.Split(FeaturePath(m.Target), "."))
	}
	message := m.Message
	if message == nil {
		message = SendPayload(m)
	}
	if message == nil {
		return Unsupported{
			Description: "a send declaring no message",
			Node:        m,
			Scope:       scope,
		}
	}
	// `self` names the sending object, which is where a send addressing no one
	// goes, so the two forms lower alike.
	selfTarget := !isPath && target == "self"
	if selfTarget {
		target = ""
	}
	var targetExpr ast.Node
	if !m.IsVia && !selfTarget {
		targetExpr = m.Target
	}
	receiver, receiverPath := SendTarget(m.Receiver)
	return Send{
		Message:      message,
		Target:       target,
		TargetSym:    targetSym,
		Node:         m,
		TargetPath:   isPath,
		TargetExpr:   targetExpr,
		IsVia:        m.IsVia,
		ViaSelf:      viaSelf,
		Receiver:     receiver,
		ReceiverPath: receiverPath,
		ReceiverExpr: m.Receiver,
		Scope:        scope,
	}
}

// lowerAssignment lowers an assignment. A chained target writes a feature of
// the object its chain reaches, so the whole walk is carried rather than
// truncated to the last segment; a namespace-qualified target names a feature
// of the object performing the body: `Scope::azimuth` writes feature azimuth on it.
func lowerAssignment(m *ast.AssignmentActionNode, scope *symbols.Scope) Statement {
	if chain, feature, ok := assignTarget(m.Target); ok {
		return Assign{Target: feature, Chain: chain, Value: m.Value, Node: m, Scope: scope}
	}
	if qname := ast.AsQualifiedName(m.Target); qname != nil && len(qname.Parts) > 1 {
		if feature, owner, sym, ok := qualifiedEndFeature(m.Target, scope); ok {
			return Assign{Target: feature, Qualified: true, Owner: owner, Feature: sym, Value: m.Value, Node: m, Scope: scope}
		}
		return Unsupported{
			Description: "assignment to a qualified target",
			Node:        m,
			Scope:       scope,
		}
	}
	return Assign{
		Target: ast.SimpleName(m.Target),
		Value:  m.Value,
		Node:   m,
		Scope:  scope,
	}
}

// lowerUsageStatement lowers a usage in statement position. The
// ActionBodyParameter a loop or branch body is written as is the block itself,
// so its members are the statements: a name it declares only scopes them
// (`loop action charging { … } until charging.done`). An action usage naming
// the action it performs is a performed action, which the host executes or
// rejects as its own purity demands.
func lowerUsageStatement(m *ast.Usage, scope *symbols.Scope) Statement {
	if m.IsTerminate {
		return Effect{Kind: EffectTerminate, Node: m, Scope: scope, Terminates: TerminateEnclosing}
	}
	if stmt, ok := usageStatement(m, scope); ok {
		return stmt
	}
	if m.Kind == ast.UsageAction && m.IsBodyParameter {
		return lowerBlock(m, m.Members, childScope(scope, m))
	}
	if m.Kind == ast.UsageAction && performsAction(m) {
		return performEffect(m, scope)
	}
	return Unsupported{Description: usageDescription(m), Node: m, Scope: scope}
}

// SendPayload returns the message a send with no argument carries: the value
// its body binds the payload parameter to (`send { in :>> payload = s; }`).
func SendPayload(m *ast.SendStatement) ast.Node {
	if payload := SendPayloadParameter(m); payload != nil {
		return payload.Value
	}
	return nil
}

// SendPayloadParameter returns the body feature redefining SendAction::payload:
// by name in a `:>>` clause, else by position as the first parameter of an argument-less send.
func SendPayloadParameter(m *ast.SendStatement) *ast.Usage {
	var byPosition *ast.Usage
	positional := m.Message == nil && m.Target == nil
	for _, member := range m.Members {
		u, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || u.Direction == ast.DirNone || u.IsResult {
			continue
		}
		redefined := redefinedNames(u)
		for _, target := range redefined {
			if target == "payload" {
				return u
			}
		}
		if positional && len(redefined) == 0 && u.Direction == ast.DirIn {
			byPosition = u
		}
		positional = false
	}
	return byPosition
}

// redefinedNames returns the last segment of every feature a usage redefines.
func redefinedNames(u *ast.Usage) []string {
	var out []string
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		if name, _ := ast.TargetName(rel.Target); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// lowerBlock lowers the body of a loop or of one branch of a conditional. owner
// is the node the block belongs to, which is the element that owns the block's
// body-local namespace, and scope is the namespace it owns.
func lowerBlock(owner ast.Node, members []ast.Node, scope *symbols.Scope) Block {
	if statesOwnFlow(members) {
		return lowerStatedBlock(owner, members, scope)
	}
	if blockNeedsFlow(members) {
		graph := lowerBlockFlow(members, scope, false)
		return Block{Node: owner, Scope: scope, Graph: graph, Stated: len(graph.Accepts) > 0}
	}
	block := Block{Node: owner, Scope: scope}
	for _, member := range members {
		actual := unwrapMembership(member)
		if actual == nil || isAnnotation(actual) {
			continue
		}
		block.Statements = append(block.Statements, lowerStatement(actual, scope))
	}
	return block
}

// isAnnotation reports whether a body member annotates the body rather than
// stating a step of it.
func isAnnotation(n ast.Node) bool {
	switch n.(type) {
	case *ast.Comment, *ast.Documentation, *ast.TextualRepresentation:
		return true
	}
	return false
}

// lowerAttributes returns every attribute declared among a behavior's members,
// in order. An unvalued attribute is still owned by the behavior even though it
// supplies no initial value. A redefinition names the attribute it overrides
// (`attribute :>> x = 5;`), so the effective name is the one bound.
func lowerAttributes(members []ast.Node) []Attribute {
	var attrs []Attribute
	for _, member := range members {
		usage, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || usage.Kind != ast.UsageAttribute {
			continue
		}
		name, _ := ast.EffectiveName(usage)
		if name == "" {
			continue
		}
		attrs = append(attrs, Attribute{Name: name, Direction: usage.Direction, IsResult: usage.IsResult, Type: TypeText(usage), Value: usage.Value, Node: usage})
	}
	return attrs
}

// usageDescription names a usage declared where a statement was expected, for
// the error the executor reports when it reaches it.
func usageDescription(u *ast.Usage) string {
	kind := u.Kind.String()
	if name := getNodeName(u); name != "" {
		return fmt.Sprintf("%s usage %q", kind, name)
	}
	return fmt.Sprintf("anonymous %s usage", kind)
}

// acceptPort returns the port an accept action routes through
// (`action r accept msg : T via p`), which the parser records as a reference
// relationship on the accept action, or "" when it named none. The port is the
// whole path it was written as, so a nested one is the port it names; self
// reports that it was written from `this`.
func acceptPort(node *ast.Usage) (port string, self bool) {
	for _, rel := range node.Relationships {
		if rel == nil || rel.Kind != ast.RelVia {
			continue
		}
		if name, self := ViaPortPath(rel.Target); name != "" {
			return name, self
		}
	}
	return "", false
}

// subsettingTarget is the feature a usage subsets (`:> e`, `:>> a.e`) as the
// path written, or "" for none.
func subsettingTarget(usage *ast.Usage) ast.Node {
	for _, rel := range usage.Relationships {
		if rel == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSubsets, ast.RelRedefines, ast.RelSpecializes, ast.RelReferences:
		default:
			continue
		}
		if FeaturePath(rel.Target) != "" {
			return rel.Target
		}
	}
	return nil
}

// typingTarget returns the name a usage was typed with (`: T`) as written,
// or nil when it was declared without a type.
func typingTarget(usage *ast.Usage) *ast.QualifiedName {
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		if qn := ast.AsQualifiedName(rel.Target); qn != nil && len(qn.Parts) > 0 {
			return qn
		}
	}
	return nil
}

// unwrapMembership extracts the actual member from a Membership wrapper.
// annotationsOnly reports whether a body declares nothing but annotations —
// metadata, comments, documentation — and so nothing the flow depends on.
func annotationsOnly(members []ast.Node) bool {
	for _, member := range members {
		switch n := unwrapMembership(member).(type) {
		case *ast.PrefixMetadata, *ast.Comment, *ast.Documentation:
		case *ast.Usage:
			if n.Kind != ast.UsageMetadata {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func unwrapMembership(node ast.Node) ast.Node {
	if membership, ok := node.(*ast.Membership); ok {
		return membership.Member
	}
	return node
}

// findNodeByName looks up a node by its qualified name.
// edgeEndName renders the name an edge end names, for a message about a node
// the body does not declare.
func edgeEndName(qname *ast.QualifiedName) string {
	if qname == nil || len(qname.Parts) == 0 {
		return "an unnamed node"
	}
	var parts []string
	for _, part := range qname.Parts {
		parts = append(parts, part.Text)
	}
	return strconv.Quote(strings.Join(parts, "::"))
}

// edgeEnd names one end of an edge for a message about a node the body does not
// declare: the name the end references, or the kind of the member the notation
// bound to it by position, which has no name to report.
func edgeEnd(qname *ast.QualifiedName, member ast.Node) string {
	if member != nil {
		return "written as " + statementKeyword(member)
	}
	return edgeEndName(qname)
}

// resolveEnd resolves one end of an edge: the node the end names, or the member
// the notation bound to that end by position (SuccessionEdge.SourceMember), which
// is how an action node member with no name of its own is sequenced.
func resolveEnd(nodes []ast.Node, qname *ast.QualifiedName, member ast.Node) ast.Node {
	if member != nil {
		for _, node := range nodes {
			if node == member {
				return node
			}
		}
		return nil
	}
	return findNodeByName(nodes, qname)
}

// resolveActionEndpointForEdge resolves an edge end, which the notation may have
// bound to a member by position rather than named.
func resolveActionEndpointForEdge(graph *ActionGraph, ref ast.Node, member ast.Node, source bool) ast.Node {
	if member != nil {
		return resolveEnd(graph.Nodes, nil, member)
	}
	return resolveActionEndpoint(graph, ref, source)
}

// resolveActionEndpoint resolves an action edge end, or the implied start/done
// node it names when the body declares no such node.
func resolveActionEndpoint(graph *ActionGraph, ref ast.Node, source bool) ast.Node {
	node := findNodeByReference(graph.Nodes, ref)
	if node != nil {
		return node
	}
	if node = ensureInheritedActionNode(graph, ref); node != nil {
		return node
	}

	name := ast.SimpleName(ref)
	if !impliedMarker(name, source, graph.Initial == nil) {
		return nil
	}
	if source {
		first := &ast.QualifiedName{}
		first.NodeSpan = ref.Span()
		first.SetSingleton(ast.NameSegment{Text: "start", Span: ref.Span()})
		initial := &ast.InitialNode{NodeBase: ast.NodeBase{NodeSpan: ref.Span()}, First: first}
		graph.Initial = initial
		graph.Nodes = append(graph.Nodes, initial)
		return initial
	}
	for _, final := range graph.Finals {
		if getNodeName(final) == "done" {
			return final
		}
	}
	final := &ast.FinalNode{NodeBase: ast.NodeBase{NodeSpan: ref.Span()}}
	graph.Finals = append(graph.Finals, final)
	graph.Nodes = append(graph.Nodes, final)
	return final
}

// findNodeByReference resolves a plain name or a chain to its graph-node root.
// A chain attaches to the node whose body contains that feature.
func findNodeByReference(nodes []ast.Node, ref ast.Node) ast.Node {
	if qname := ast.AsQualifiedName(ref); qname != nil {
		return findNodeByName(nodes, qname)
	}
	chain, ok := ref.(*ast.FeatureChainExpr)
	if !ok {
		return nil
	}
	for {
		operand := chain.Operand
		if nested, ok := operand.(*ast.FeatureChainExpr); ok {
			chain = nested
			continue
		}
		return findNodeByName(nodes, ast.AsQualifiedName(operand))
	}
}

// successionEndText formats an endpoint for an action-succession diagnostic.
func successionEndText(ref ast.Node) string {
	if text := FeaturePath(ref); text != "" {
		return strconv.Quote(text)
	}
	return "an unnamed node"
}

// sequencedMembers collects the members of a body that an edge binds to one of
// its ends by position, the members a `then` sequences without a name.
func sequencedMembers(members []ast.Node) map[ast.Node]bool {
	sequenced := make(map[ast.Node]bool)
	mark := func(ends ...ast.Node) {
		for _, end := range ends {
			if end != nil {
				sequenced[end] = true
			}
		}
	}
	for _, member := range members {
		switch n := unwrapMembership(member).(type) {
		case *ast.SuccessionEdge:
			mark(n.SourceMember, n.TargetMember)
		case *ast.ControlFlowEdge:
			mark(n.SourceMember, n.TargetMember)
		}
	}
	return sequenced
}

// orderedAssertions collects the `assert constraint` members of a body that one
// of its successions names or binds by position, each a step of the flow.
func orderedAssertions(members []ast.Node) map[*ast.Usage]bool {
	named := make(map[string]bool)
	name := func(ends ...*ast.QualifiedName) {
		for _, end := range ends {
			if end != nil && len(end.Parts) == 1 {
				named[end.Parts[0].Text] = true
			}
		}
	}
	for _, member := range members {
		switch n := unwrapMembership(member).(type) {
		case *ast.InitialNode:
			name(n.First, n.Successor)
		case *ast.SuccessionEdge:
			name(n.Source, n.Target)
		case *ast.ControlFlowEdge:
			name(n.Source, n.Target)
		case *ast.Usage:
			if n.Kind == ast.UsageSuccession {
				for _, end := range n.ConnectorEnds {
					if ref := connectorEndReference(end); ref != nil && ast.SimpleName(ref) != "" {
						named[ast.SimpleName(ref)] = true
					}
				}
			}
		}
	}
	sequenced := sequencedMembers(members)
	ordered := make(map[*ast.Usage]bool)
	for _, member := range members {
		u, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || !resolve.IsAssertion(u) {
			continue
		}
		if sequenced[u] || named[getNodeName(u)] || named[u.Ident.ShortName] {
			ordered[u] = true
		}
	}
	return ordered
}

func findNodeByName(nodes []ast.Node, qname *ast.QualifiedName) ast.Node {
	if qname == nil || len(qname.Parts) == 0 {
		return nil
	}

	targetName := qname.Parts[len(qname.Parts)-1].Text
	for _, node := range nodes {
		if nodeAnswersTo(node, targetName) {
			return node
		}
	}
	return nil
}

// nodeAnswersTo reports whether name is one of the keys a node is declared
// under: its effective name or, for a usage, its declared short name. A short
// name is a name of its own, so `action <s> :>> takePhoto;` is reachable as
// both `s` and `takePhoto`.
func nodeAnswersTo(node ast.Node, name string) bool {
	if name == "" {
		return false
	}
	if getNodeName(node) == name {
		return true
	}
	u, ok := node.(*ast.Usage)
	return ok && u.Ident.ShortName == name
}

// getNodeName extracts the name from a node.
func getNodeName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.InitialNode:
		return n.Name()
	case *ast.FinalNode:
		// The node declares no name of its own: a succession reaches it by the
		// name of the library feature it is, `done`.
		return "done"
	case *ast.ForkNode:
		return n.Name
	case *ast.JoinNode:
		return n.Name
	case *ast.MergeNode:
		return n.Name
	case *ast.DecisionNode:
		return n.Name
	case *ast.ActionExecutionNode:
		return n.Name
	case *ast.Usage:
		// An unnamed usage is named after the feature it references or redefines
		// (`perform increment;` is a node named increment).
		if name, _ := ast.EffectiveName(n); name != "" {
			return name
		}
		return n.Ident.ShortName
	}
	return ""
}

// lowerFlow lowers a flow declared among an action's members
// (`flow generateToAmplify from generateTorque.engineTorque to
// amplifyTorque.engineTorque;`) to the data flow the executor applies when a
// token leaves the source node: the value at the source's pin becomes the value
// at the target's pin. The flow's own name is carried so a diagnostic and the
// REPL can name it.
//
// Both ends must name action nodes of this graph, since a data flow moves a
// value from one node's output to another's input; an end naming anything else
// is reported rather than dropped.
func lowerFlow(nodes nodeLookup, flow *ast.Usage) (ast.Node, ObjectFlow, error) {
	name, _ := ast.EffectiveName(flow)
	sourceNode, sourcePin := flowEnd(nodes, flow.FlowEnds.From)
	targetNode, targetPin := flowEnd(nodes, flow.FlowEnds.To)

	if sourceNode == nil {
		return nil, ObjectFlow{}, fmt.Errorf(
			"flow %s: source %s does not name an action node of this action",
			orAnonymous(name), flowEndText(flow.FlowEnds.From),
		)
	}
	if targetNode == nil {
		return nil, ObjectFlow{}, fmt.Errorf(
			"flow %s: target %s does not name an action node of this action",
			orAnonymous(name), flowEndText(flow.FlowEnds.To),
		)
	}
	for _, end := range []ast.Node{flow.FlowEnds.From, flow.FlowEnds.To} {
		if err := flowEndReaches(end); err != nil {
			return nil, ObjectFlow{}, fmt.Errorf("flow %s: %w", orAnonymous(name), err)
		}
	}

	// `flow of engineTorque from a to b` names the feature that flows rather
	// than a pin of each end, so it names the pin at both ends.
	if payload := ast.SimpleName(flow.FlowEnds.Payload); payload != "" {
		if sourcePin == "" {
			sourcePin = payload
		}
		if targetPin == "" {
			targetPin = payload
		}
	}

	// A flow carries the value of a feature, so an end naming a node alone with
	// no payload to name its pin identifies nothing to move.
	if sourcePin == "" || targetPin == "" {
		return nil, ObjectFlow{}, fmt.Errorf(
			"flow %s: names no feature to carry; write `flow of <payload> from %s to %s` or name a pin at each end",
			orAnonymous(name), flowEndText(flow.FlowEnds.From), flowEndText(flow.FlowEnds.To),
		)
	}

	kind := FlowStreaming
	if flow.IsSuccessionFlow() {
		kind = FlowSuccession
	}
	return sourceNode, ObjectFlow{
		Name:      name,
		SourcePin: sourcePin,
		TargetPin: targetPin,
		Target:    targetNode,
		Kind:      kind,
		Decl:      flow,
	}, nil
}

// succeedFlow adds the succession a `succession flow` also states: the target
// starts once the source completes and the value has moved.
func succeedFlow(graph *ActionGraph, source ast.Node, flow ObjectFlow) {
	if flow.Kind != FlowSuccession {
		return
	}
	graph.Edges[source] = append(graph.Edges[source], ActionEdge{Source: source, Target: flow.Target, Decl: flow.Decl, Carries: true})
}

// flowEnd resolves one end of a flow to the node it belongs to and the pin it
// names. A chain (`generateTorque.engineTorque`) names a node and its pin; a
// bare name (`generateTorque`) names the node alone, and the pin is whatever
// the flow's payload names.
func flowEnd(nodes nodeLookup, end ast.Node) (ast.Node, string) {
	segments := endSegments(end)
	if len(segments) == 0 {
		return nil, ""
	}
	node := nodes(segments[0])
	if len(segments) == 1 {
		return node, ""
	}
	return node, segments[1]
}

// flowEndReaches reports a flow end reaching into a node's own flow (`leg.inner.v`):
// a flow joins pins of nodes of the one flow it is declared in.
func flowEndReaches(end ast.Node) error {
	if len(endSegments(end)) > 2 {
		return fmt.Errorf("end %s reaches into a node's own flow; a flow joins pins of the nodes of one flow, so declare it in that node or bind the pin instead",
			flowEndText(end))
	}
	return nil
}

// orAnonymous names a declaration that may have been written without a name.
func orAnonymous(name string) string {
	if name == "" {
		return "(anonymous)"
	}
	return name
}

// flowEndText renders one end of a flow for a diagnostic: the chain `a.out` for
// a feature chain, the qualified name for a reference, and a placeholder for an
// end the notation left out, so the message never reads as an empty position.
func flowEndText(end ast.Node) string {
	switch e := end.(type) {
	case *ast.QualifiedName:
		return edgeEndName(e)
	case *ast.FeatureChainExpr:
		base := strings.Trim(flowEndText(e.Operand), `"`)
		return strconv.Quote(base + "." + ast.SimpleName(e.Member))
	case *ast.FeatureReference:
		return edgeEndName(e.Name)
	}
	return "(nothing)"
}

// parsePinReference extracts node and pin name from a qualified reference.
// Format: "nodeName.pinName" or just "nodeName" (pin = "")
func parsePinReference(nodes []ast.Node, qname *ast.QualifiedName) (ast.Node, string) {
	if qname == nil {
		return nil, ""
	}
	return flowEnd(nodesNamed(nodes), qname)
}

// statementKeyword names a body statement for a diagnostic.
func statementKeyword(node ast.Node) string {
	switch n := node.(type) {
	case *ast.WhileLoopActionNode:
		return "a '" + n.Kind.String() + "' loop"
	case *ast.IfActionNode:
		return "an 'if' conditional"
	case *ast.AssignmentActionNode:
		return "an assignment"
	case *ast.SendStatement:
		return "a 'send'"
	case *ast.TerminateStatement:
		return "a 'terminate'"
	case *ast.PerformActionNode:
		return "a 'perform'"
	case *ast.Usage:
		// A member bound to an edge by position may be a declaration rather than a
		// statement (`then part { … }`), named by its kind.
		return usageDescription(n)
	default:
		return "a member the body declares"
	}
}
