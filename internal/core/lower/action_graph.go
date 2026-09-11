// Package lower provides AST → execution IR lowering.
// Converts declarative members (TransitionMember, EntryMember) to
// operational graphs (nodes + edges) that executors consume.
package lower

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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

	// Nodes in the graph (InitialNode, FinalNode, ExecutionNode, etc.)
	Nodes []ast.Node

	// Edges: source node → successions in declaration order.
	Edges map[ast.Node][]ActionEdge

	// DataFlows: source node → list of object flows
	DataFlows map[ast.Node][]ObjectFlow

	// Bodies: node → the statements that node executes, in declaration order
	Bodies map[ast.Node][]Statement

	// Footprints: node → what advancing a token through the node may read, write,
	// send, accept and converge on, for the model checker's independence relation.
	Footprints map[ast.Node]Footprint

	// Features: node → the parameters and attributes the node declares itself,
	// in declaration order; each performance of the node holds its own values.
	Features map[ast.Node][]Feature

	// Scopes: node → the namespace the node owns, which its features resolve in.
	Scopes map[ast.Node]*symbols.Scope

	// Bindings are the bindings with an end at a node's pin (`bind add.a = x;`).
	Bindings []PinBinding

	// Accepts: node → the message that node waits for
	Accepts map[ast.Node]Accept

	// Subflows: node → the flow the node's own members state, present only for a
	// node that states one. Its subactions are subperformances of the node, so it
	// completes only when that flow does (action_subflow.go).
	Subflows map[ast.Node]*Subflow

	// InitialNode (required)
	Initial ast.Node

	// FinalNodes (may be multiple)
	Finals []ast.Node

	// Connections are the connectors declared in the action body, which is how
	// a `send ... via <port>` finds the ports it reaches.
	Connections []Connection

	// StatementRuns marks the nodes of a block's own flow that stand for a run of
	// statements rather than for an action node (block_graph.go). Such a node is
	// keyed by the first statement of the run, whose name names no step.
	StatementRuns map[ast.Node]bool

	// BlockNodes lists, per node, the action nodes its body's blocks (an `if` branch,
	// a loop body) declare, in declaration order: subperformances reached by name from it.
	BlockNodes map[ast.Node][]ast.Node
}

// ActionEdge is one succession out of a node: the node it leaves, the target it reaches, the
// guard it carries and its declaration (nil when implicit). No two edges of a graph compare equal.
type ActionEdge struct {
	Source ast.Node
	Target ast.Node
	Guard  ast.Node
	Decl   ast.Node
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
	// TargetPath records that Target is a feature chain (`a.b`) reaching through
	// the sender's features, rather than a name in a namespace (`R`, `P::R`).
	TargetPath bool
	IsVia      bool
	// Receiver is the name addressed by a routed send, empty when omitted.
	Receiver     string
	ReceiverPath bool
	Scope        *symbols.Scope // the scope the statement was declared in
}

func (Send) statement() { /* marker: closed Statement set */ }

// Assign is a lowered assignment: `assign <Target> := <Value>`. Target is the
// feature written — the target's last segment — empty when the target names no
// feature.
type Assign struct {
	Target string
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
	// Stated marks Graph as the token flow the body's successions and control
	// nodes state; unset, Graph runs the body's steps in declaration order.
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
)

func (k EffectKind) String() string {
	switch k {
	case EffectPerform:
		return "perform"
	case EffectAccept:
		return "accept"
	case EffectTerminate:
		return "terminate"
	default:
		return "effect"
	}
}

// Effect is a statement acting on the world outside the body — perform, accept,
// terminate — lowered so a host rejecting it (a calculation) can say so.
type Effect struct {
	Kind  EffectKind
	Node  ast.Node
	Scope *symbols.Scope // the scope the statement was declared in
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
	SubsetsEvent ast.Node
	Trigger      ast.Node
	// Scope is the scope the accept was declared in, in which SignalType resolves.
	Scope *symbols.Scope
}

// Attribute is a lowered attribute default written among a behavior's members
// (`attribute h : LengthValue = 500.0 [m];`), whose Value resolves in the
// graph's own scope.
type Attribute struct {
	Name  string
	Value ast.Node
	Node  ast.Node // the declaration itself, for diagnostics
	// Scope is the scope the declaration was written in, in which its default
	// resolves; nil where the owner's own scope resolves it.
	Scope *symbols.Scope
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
	Scope        *symbols.Scope // the scope the binding was written in
	Decl         *ast.Usage
	// FromValue marks the binding a pin's own value states (`inout n = ticks;`): the
	// value is the pin's initial value alone when no feature around the node holds it.
	FromValue bool
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
	// Decl is the declaration the flow was written as, for a consumer that
	// reports where it comes from.
	Decl ast.Node
}

// ToActionGraph converts an action AST (Usage or Definition) to an ActionGraph.
// scope is the scope the action's body was declared in — the scope the action
// itself owns — which every expression the graph carries is evaluated in.
// Returns error if graph is malformed (e.g., no initial node, dangling edges).
func ToActionGraph(actionDecl ast.Node, scope *symbols.Scope) (*ActionGraph, error) {
	graph, members, firstNode, err := collectActionNodes(actionDecl, scope)
	if err != nil {
		return nil, err
	}

	// Note: Initial node is optional at graph construction time.
	// The executor's initialize() will validate and return the error if missing.

	// Second pass: build edges
	for _, member := range members {
		actualMember := unwrapMembership(member)

		switch n := actualMember.(type) {
		case *ast.InitialNode:
			// Handle implicit successor from `first X then Y` syntax
			if n.Successor != nil {
				sourceNode := ast.Node(n)
				if named, ok := firstNode[n]; ok {
					sourceNode = named
				}
				targetNode := resolveActionEndpoint(graph, n.Successor, false)
				if targetNode == nil {
					return nil, fmt.Errorf("initial node %s successor references undefined target %s", n.Name, edgeEndName(n.Successor))
				}
				graph.Edges[sourceNode] = append(graph.Edges[sourceNode], ActionEdge{
					Source: sourceNode,
					Target: targetNode,
					Guard:  n.Guard,
					Decl:   n,
				})
			}
		case *ast.SuccessionEdge:
			sourceNode := resolveActionEndpointForEdge(graph, n.Source, n.SourceMember, true)
			targetNode := resolveActionEndpointForEdge(graph, n.Target, n.TargetMember, false)

			if sourceNode == nil {
				return nil, fmt.Errorf("succession edge references undefined source node %s", edgeEnd(n.Source, n.SourceMember))
			}
			if targetNode == nil {
				return nil, fmt.Errorf("succession edge references undefined target node %s", edgeEnd(n.Target, n.TargetMember))
			}
			graph.Edges[sourceNode] = append(graph.Edges[sourceNode], ActionEdge{
				Source: sourceNode,
				Target: targetNode,
				Decl:   n,
			})
		case *ast.ControlFlowEdge:
			sourceNode := resolveActionEndpointForEdge(graph, n.Source, n.SourceMember, true)
			targetNode := resolveActionEndpointForEdge(graph, n.Target, n.TargetMember, false)

			if sourceNode == nil {
				return nil, fmt.Errorf("control flow edge references undefined source %s", edgeEnd(n.Source, n.SourceMember))
			}
			if targetNode == nil {
				return nil, fmt.Errorf("control flow edge references undefined target %s", edgeEnd(n.Target, n.TargetMember))
			}
			graph.Edges[sourceNode] = append(graph.Edges[sourceNode], ActionEdge{
				Source: sourceNode,
				Target: targetNode,
				Guard:  n.Guard,
				Decl:   n,
			})
		case *ast.TransitionMember:
			sourceNode := resolveActionEndpoint(graph, n.Source, true)
			targetNode := resolveActionEndpoint(graph, n.Target, false)
			if sourceNode == nil {
				return nil, fmt.Errorf("succession references undefined source node %s", edgeEndName(n.Source))
			}
			if targetNode == nil {
				return nil, fmt.Errorf("succession references undefined target node %s", edgeEndName(n.Target))
			}
			graph.Edges[sourceNode] = append(graph.Edges[sourceNode], ActionEdge{
				Source: sourceNode,
				Target: targetNode,
				Guard:  n.Guard,
				Decl:   n,
			})
		case *ast.ObjectFlowEdge:
			sourceNode, sourcePin := parsePinReference(graph.Nodes, n.Source)
			targetNode, targetPin := parsePinReference(graph.Nodes, n.Target)

			if sourceNode == nil {
				return nil, fmt.Errorf("object flow edge references undefined source %v", n.Source)
			}
			if targetNode == nil {
				return nil, fmt.Errorf("object flow edge references undefined target %v", n.Target)
			}
			for _, end := range []*ast.QualifiedName{n.Source, n.Target} {
				if err := flowEndReaches(end); err != nil {
					return nil, fmt.Errorf("object flow edge: %w", err)
				}
			}

			graph.DataFlows[sourceNode] = append(graph.DataFlows[sourceNode], ObjectFlow{
				SourcePin: sourcePin,
				TargetPin: targetPin,
				Target:    targetNode,
				Decl:      n,
			})
		case *ast.Usage:
			if n.Kind == ast.UsageBinding {
				bindings, err := lowerPinBindings(graph, nodesNamed(graph.Nodes), n, scope)
				if err != nil {
					return nil, err
				}
				graph.Bindings = append(graph.Bindings, bindings...)
				continue
			}
			if n.Kind == ast.UsageSuccession {
				if len(n.ConnectorEnds) != 2 {
					return nil, fmt.Errorf("action succession must have exactly two connector ends, got %d", len(n.ConnectorEnds))
				}
				if n.Multiplicity != nil {
					return nil, fmt.Errorf("action succession has unsupported multiplicity")
				}
				if n.HasBody || len(n.Members) != 0 {
					return nil, fmt.Errorf("action succession has unsupported body")
				}
				for i, end := range n.ConnectorEnds {
					if end.Multiplicity != nil {
						return nil, fmt.Errorf("action succession end %d has unsupported multiplicity", i+1)
					}
				}
				sourceRef := connectorEndReference(n.ConnectorEnds[0])
				targetRef := connectorEndReference(n.ConnectorEnds[1])
				sourceNode := resolveActionEndpoint(graph, sourceRef, true)
				targetNode := resolveActionEndpoint(graph, targetRef, false)
				if sourceNode == nil {
					return nil, fmt.Errorf("action succession references undefined source node %s", successionEndText(sourceRef))
				}
				if targetNode == nil {
					return nil, fmt.Errorf("action succession references undefined target node %s", successionEndText(targetRef))
				}
				graph.Edges[sourceNode] = append(graph.Edges[sourceNode], ActionEdge{
					Source: sourceNode,
					Target: targetNode,
					Decl:   n,
				})
				continue
			}
			if n.Kind != ast.UsageFlow || n.FlowEnds == nil {
				continue
			}
			source, flow, err := lowerFlow(nodesNamed(graph.Nodes), n)
			if err != nil {
				return nil, err
			}
			graph.DataFlows[source] = append(graph.DataFlows[source], flow)
		}
	}

	if err := lowerInheritedPinConnections(graph, scope); err != nil {
		return nil, err
	}
	recordBlockNodes(graph)
	lowerFootprints(graph)
	return graph, nil
}

// lowerInheritedPinConnections lowers the bindings and flows the actions the
// action specializes wrote at pins of its nodes, nearest general first: a node
// the action inherits keeps the connections its declaring action stated at it.
func lowerInheritedPinConnections(graph *ActionGraph, scope *symbols.Scope) error {
	for _, body := range resolve.ActionGeneralBodies(scope) {
		nodes := inheritedNodeLookup(graph, body)
		for _, member := range declMembers(body.Node()) {
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
				graph.DataFlows[source] = append(graph.DataFlows[source], flow)
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

// resolveFirstNode reinterprets a `first a …;` whose name is a node the body
// declares: a is the flow's first node, so the initial node it parsed as is not
// a node of the graph. Returns the named node each such initial node stands for.
func resolveFirstNode(graph *ActionGraph) (map[*ast.InitialNode]ast.Node, error) {
	initial, ok := graph.Initial.(*ast.InitialNode)
	if !ok || initial.Name == "" {
		return nil, nil
	}
	var named ast.Node
	for _, node := range graph.Nodes {
		if node != ast.Node(initial) && nodeAnswersTo(node, initial.Name) {
			named = node
			break
		}
	}
	if named == nil {
		return nil, nil
	}
	if _, isFinal := named.(*ast.FinalNode); isFinal {
		// A flow cannot start where it ends: naming a final node would retire the
		// token before any succession out of it is taken.
		return nil, fmt.Errorf("first names the final node %s, so the action would end before it started", initial.Name)
	}
	graph.Initial = named
	graph.Nodes = slices.DeleteFunc(graph.Nodes, func(node ast.Node) bool {
		return node == ast.Node(initial)
	})
	return map[*ast.InitialNode]ast.Node{initial: named}, nil
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
		if !ok || !DeclaresNodeFeature(m) {
			continue
		}
		name, _ := ast.EffectiveName(m)
		if name == "" {
			continue
		}
		features = append(features, Feature{
			Name:      name,
			Direction: m.Direction,
			IsResult:  m.IsResult,
			Value:     m.Value,
			Node:      m,
			Scope:     scope,
		})
		if binding, ok := inoutValueBinding(node, m, name, scope); ok {
			graph.Bindings = append(graph.Bindings, binding)
		}
	}
	graph.Features[node] = features
}

// inoutValueBinding lowers the value of a node's `inout` pin that names a feature
// (`inout n = ticks;`) to the binding between the two it states; a value that is
// an expression of another kind is the pin's initial value alone. Which of the two
// a name is (`ticks`, or the literal `Mode::idle`) is settled where the node performs.
func inoutValueBinding(node, pin *ast.Usage, name string, scope *symbols.Scope) (PinBinding, bool) {
	if pin.Direction != ast.DirInOut || pin.Value == nil || len(endSegments(pin.Value)) == 0 {
		return PinBinding{}, false
	}
	binding := PinBinding{Node: node, Pin: name, Other: pin.Value, Scope: scope, Decl: pin, FromValue: true}
	if chain, feature, ok := assignTarget(pin.Value); ok {
		binding.OtherChain, binding.OtherFeature = chain, feature
	}
	return binding, true
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
	for _, member := range node.Members {
		switch m := unwrapMembership(member).(type) {
		case *ast.SendStatement, *ast.AssignmentActionNode, *ast.WhileLoopActionNode, *ast.IfActionNode:
			graph.Bodies[node] = append(graph.Bodies[node], lowerStatement(m, scope))
		}
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
		// A target is either a chain through features (`alpha.inPort`) or a name in
		// a namespace (`P::Driver`), which resolve differently. A `via` target names
		// a port of the sender, rendered as connector ends are so the two match.
		target, isPath := SendTarget(m.Target)
		var targetSym *symbols.Symbol
		if m.IsVia {
			target, isPath = FeaturePath(m.Target), true
			targetSym, _ = resolve.FeatureSymbolInScope(scope, strings.Split(target, "."))
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
		receiver, receiverPath := SendTarget(m.Receiver)
		return Send{
			Message:      message,
			Target:       target,
			TargetSym:    targetSym,
			TargetPath:   isPath,
			IsVia:        m.IsVia,
			Receiver:     receiver,
			ReceiverPath: receiverPath,
			Scope:        scope,
		}
	case *ast.AssignmentActionNode:
		// A chained target writes a feature of the object its chain reaches, so the
		// whole walk is carried rather than truncated to the last segment.
		if chain, feature, ok := assignTarget(m.Target); ok {
			return Assign{Target: feature, Chain: chain, Value: m.Value, Node: m, Scope: scope}
		}
		// A namespace-qualified target names no object to write on: an assignment
		// writes a feature of its target occurrence (Actions::AssignmentAction).
		if qname := ast.AsQualifiedName(m.Target); qname != nil && len(qname.Parts) > 1 {
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
	case *ast.WhileLoopActionNode:
		return Loop{
			Kind:       m.Kind,
			Condition:  m.Condition,
			Until:      m.Until,
			Variable:   m.Variable.Name,
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
		return Effect{Kind: EffectPerform, Node: m, Scope: scope}
	case *ast.TerminateStatement:
		return Effect{Kind: EffectTerminate, Node: m, Scope: scope}
	case *ast.Usage:
		if stmt, ok := usageStatement(m, scope); ok {
			return stmt
		}
		// The ActionBodyParameter a loop or branch body is written as is the block
		// itself, so its members are the statements: a name it declares only scopes
		// them (`loop action charging { … } until charging.done`).
		if m.Kind == ast.UsageAction && m.IsBodyParameter {
			return lowerBlock(m, m.Members, childScope(scope, m))
		}
		// An action usage naming the action it performs is a performed action, which
		// the host executes or rejects as its own purity demands.
		if m.Kind == ast.UsageAction && performsAction(m) {
			return Effect{Kind: EffectPerform, Node: m, Scope: scope}
		}
		return Unsupported{Description: usageDescription(m), Node: m, Scope: scope}
	default:
		return Unsupported{Description: fmt.Sprintf("%T", member), Node: member, Scope: scope}
	}
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
	if blockNeedsFlow(members) {
		return Block{Node: owner, Scope: scope, Graph: lowerBlockFlow(members, scope, false)}
	}
	block := Block{Node: owner, Scope: scope}
	for _, member := range members {
		actual := unwrapMembership(member)
		if actual == nil {
			continue
		}
		block.Statements = append(block.Statements, lowerStatement(actual, scope))
	}
	return block
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
		attrs = append(attrs, Attribute{Name: name, Value: usage.Value, Node: usage})
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
// whole path it was written as, so a nested one is the port it names.
func acceptPort(node *ast.Usage) string {
	for _, rel := range node.Relationships {
		if rel == nil || rel.Kind != ast.RelVia {
			continue
		}
		if name := FeaturePath(rel.Target); name != "" {
			return name
		}
	}
	return ""
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
		initial := &ast.InitialNode{NodeBase: ast.NodeBase{NodeSpan: ref.Span()}, Name: "start"}
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
		return n.Name
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

	return sourceNode, ObjectFlow{
		Name:      name,
		SourcePin: sourcePin,
		TargetPin: targetPin,
		Target:    targetNode,
		Decl:      flow,
	}, nil
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
