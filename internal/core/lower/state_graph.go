package lower

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// StateGraph is the execution IR for state machines.
type StateGraph struct {
	// Scope is the scope the machine's own body was declared in, in which the
	// expressions written directly among its members resolve their names.
	Scope *symbols.Scope

	// Attributes are the attributes the machine declares, in declaration order.
	Attributes []Attribute

	// StateScopes: state → the scope that state's body was declared in, which is
	// what the names in its entry, do and exit behaviors resolve against.
	StateScopes map[*ast.StateNode]*symbols.Scope

	// Behaviors: state → its lowered entry, do and exit behaviors. The executor
	// runs these rather than the state's AST members, so an inline action body is
	// executable statements by the time it is reached.
	Behaviors map[*ast.StateNode]*StateBehaviors

	// HiddenStates are graph-only composite owners synthesized for parallel
	// regions. They execute behaviors but are not user-visible state visits.
	HiddenStates map[*ast.StateNode]bool

	// HiddenRegionOf: graph-only owner → the region it stands for, so a walk up
	// the parent chain crosses it without losing which region a state is in.
	HiddenRegionOf map[*ast.StateNode]*ast.StateRegion

	// RegionState: synthesized region → the graph-only owner standing for it,
	// which holds the attributes the region's body declares.
	RegionState map[*ast.StateRegion]*ast.StateNode

	// Machine is the graph-only root state for the machine's own entry, do and
	// exit behaviors. A parallel machine's regions are TopRegions instead.
	Machine *ast.StateNode

	// declOf: synthesized state → the declaration it was built from, since the
	// scope tree is keyed by what the scope builder saw rather than by the state
	// nodes lowering derives from it.
	declOf map[*ast.StateNode]ast.Node

	// regionDecl: synthesized region → the direct substate that supplies its
	// body, since the scope tree is keyed by that substate rather than the region.
	regionDecl map[*ast.StateRegion]ast.Node

	// vertexOf: the declaration an endpoint resolves to → the graph node standing
	// for it, the state node built from it or the pseudostate itself.
	vertexOf map[ast.Node]ast.Node

	// stateByDecl: the declaration a state was built from → that state, hidden
	// region owners included, which is how the body a transition is written in is
	// matched to the state owning it.
	stateByDecl map[ast.Node]*ast.StateNode

	// completing are the states whose entry completes the region they belong to,
	// and with it the machine once every top-level region has completed. Entering
	// `done` is what completes: the marker-free rule this replaces a marker with.
	completing map[*ast.StateNode]bool

	// completionOf: the state or region a body belongs to → the `done` vertex
	// synthesized for it, so several transitions entering `done` in one body reach
	// one vertex. The machine's own body is the nil key.
	completionOf map[ast.Node]*ast.StateNode

	// endpoints resolves what a transition endpoint names.
	endpoints EndpointResolver

	// States in the machine (flat list, includes nested)
	States []*ast.StateNode

	// Pseudostates of the machine in declaration order. Not keyed by name:
	// sibling regions may declare same-named pseudostates.
	Pseudostates []*ast.PseudostateNode

	// PseudostateOwner: pseudostate -> the composite state that declares it,
	// absent for one declared directly in the machine. A history pseudostate
	// restores the configuration of its owner, so the owner must survive lowering.
	PseudostateOwner map[*ast.PseudostateNode]*ast.StateNode

	// Transitions: source node (StateNode or PseudostateNode) → list of transitions
	Transitions map[ast.Node][]*Transition

	// CompositeStates: state → regions
	CompositeStates map[*ast.StateNode][]*ast.StateRegion

	// CompositeStateOrder preserves declaration order for deterministic runtime
	// selection of an active composite owner.
	CompositeStateOrder []*ast.StateNode

	// RegionInitials: region → initial state
	RegionInitials map[*ast.StateRegion]*ast.StateNode

	// ParentState: child → parent
	ParentState map[*ast.StateNode]*ast.StateNode

	// RegionOwner: region → owning composite state
	RegionOwner map[*ast.StateRegion]*ast.StateNode

	// Deferred: state → the triggers it defers while active, normalized the same
	// way transition triggers are.
	Deferred map[*ast.StateNode][]ast.Node

	// TopRegions are the machine's own orthogonal regions, in declaration order.
	// The order is observable: it is the order regions are entered and exited in.
	TopRegions []*ast.StateRegion

	// RegionOf: state → the region that declares it (fork/join need the region a
	// target belongs to)
	RegionOf map[*ast.StateNode]*ast.StateRegion

	// InitialState (required for simple machines, nil for multi-region)
	Initial *ast.StateNode

	// designatedInitials are the states a transition out of the body's entry
	// action names as the state the machine starts in. It is graph-local: the
	// parsed AST it is derived from stays as written.
	designatedInitials map[*ast.StateNode]bool

	// EntryTransitions: the body a transition out of the entry action is written
	// in (a state, a region, nil for the machine's own body) → those transitions in
	// declaration order; the first whose guard holds names the state it starts in.
	EntryTransitions map[ast.Node][]*EntryTransition

	// Connections are the connectors declared in the state machine body, which
	// is how a `send ... via <port>` in an entry/do/exit/effect action finds the
	// ports it reaches.
	Connections []Connection

	// StateAttributes: state → the attributes it owns, its own and those it
	// inherits from the definition typing it. Each state owns its values, so two
	// usages of one definition hold two sets of them.
	StateAttributes map[*ast.StateNode][]Attribute

	// instanceOf: state → the materialization of the content it inherits, which
	// the transition pass lowers in the same context the vertices were collected in.
	instanceOf map[*ast.StateNode]*stateInstance

	// cur is the materialization being collected, nil for the machine's own body.
	cur *stateInstance

	// materializing are the state definitions whose content is being materialized,
	// which is what makes a definition reaching itself a reportable error rather
	// than an unbounded expansion.
	materializing map[ast.Node]bool

	// scopeOf: state → the scope its declaration was written in, recorded where
	// the content came from a definition's body rather than the usage's.
	scopeOf map[*ast.StateNode]*symbols.Scope

	// regionScopeOf: region → the scope its declaration was written in, for a
	// region a usage inherits.
	regionScopeOf map[*ast.StateRegion]*symbols.Scope

	// behaviorScope: entry, do or exit action → the scope it was declared in,
	// recorded where a state runs a behavior another body declares.
	behaviorScope map[ast.Node]*symbols.Scope

	// attributeScope: attribute declaration → the scope its default value
	// resolves in.
	attributeScope map[ast.Node]*symbols.Scope

	// bodyOf: state → the members its content came from, inherited then its own.
	bodyOf map[*ast.StateNode][]inheritedMember

	// parallelState: state → whether its direct substates are orthogonal regions,
	// which the definition typing it may say as much as its own declaration.
	parallelState map[*ast.StateNode]bool
}

// Transition represents a state transition (lowered from TransitionEdge or TransitionMember).
type Transition struct {
	// Name is the transition's own name, when it was written with one
	// (`transition maintain first idle then busy`), and "" when it was not.
	Name string
	// Decl is the declaration the transition was written as, for a consumer that
	// reports where it comes from.
	Decl    ast.Node
	Source  ast.Node // *ast.StateNode or *ast.PseudostateNode
	Target  ast.Node // *ast.StateNode or *ast.PseudostateNode
	Trigger ast.Node // TimeEvent, ChangeEvent, SignalEvent, CallEvent, nil = completion
	Guard   ast.Node // guard expression, nil = no guard
	// Effect are the transition's effect behaviors, lowered the same way a state's
	// entry, do and exit behaviors are.
	Effect []StateBehavior
	// Via is the port the accepted occurrence must arrive at
	// (`accept Ping via commPort`), and "" when the trigger names no port, in
	// which case an occurrence reaching the machine by any route fires it.
	Via string

	// Scope is the scope the transition was declared in, in which the expressions
	// its trigger carries — a time event's duration, a change event's condition —
	// resolve their names.
	Scope *symbols.Scope

	// BodyScope is the scope the transition's guard and effect resolve in. It is
	// Scope, except for a call trigger, whose parameters are visible to the guard
	// and effect and nowhere else (`accept setSpeed(v) if v > 0`).
	BodyScope *symbols.Scope
}

// ToStateGraph converts a state machine AST (Usage or Definition) to a StateGraph.
// scope is the scope the machine's body was declared in — the scope the machine
// itself owns — from which the scope of every state and transition it carries is
// derived. Its endpoints resolve against the machine's own symbols; a caller
// holding the name-resolution tier's uses ToStateGraphWithEndpoints instead.
func ToStateGraph(stateMachineDecl ast.Node, scope *symbols.Scope) (*StateGraph, error) {
	return ToStateGraphWithEndpoints(stateMachineDecl, scope, nil)
}

// ToStateGraphWithEndpoints lowers a state machine, building its transitions
// from the endpoints the name-resolution tier already resolved.
func ToStateGraphWithEndpoints(stateMachineDecl ast.Node, scope *symbols.Scope, endpoints EndpointResolver) (*StateGraph, error) {
	// A machine no scope tree holds is indexed on its own; without the tier's
	// resolver, one that has a tree names its endpoints from that tree alone.
	switch {
	case scope == nil:
		endpoints, scope = localEndpoints(stateMachineDecl)
	case endpoints == nil:
		endpoints = scopeEndpoints{machine: scope}
	}
	graph := newStateGraph(scope, endpoints)

	members, err := machineMembers(stateMachineDecl)
	if err != nil {
		return nil, err
	}

	// The machine may itself be a usage typed by a state definition, whose content
	// is as much part of the machine as the members written in its own body.
	inherited, owners, err := graph.inheritedContent(stateMachineDecl, outerScope(scope, stateMachineDecl))
	if err != nil {
		return nil, err
	}
	body := append(append([]inheritedMember{}, inherited...), ownMembers(members, scope)...)

	graph.Connections = lowerConnections(members, OwnerBehavior, scope)
	graph.Attributes = keptAttributes(lowerStateAttributes(graph, inherited), lowerStateAttributes(graph, ownMembers(members, scope)))
	parallelMachine := stateMachineIsParallel(stateMachineDecl)
	for _, owner := range owners {
		parallelMachine = parallelMachine || stateMachineIsParallel(owner)
	}
	graph.Machine = graph.machineState(stateMachineDecl, inherited, members, scope)

	for _, group := range groupMembers(body) {
		if err := collectVertices(graph, group.nodes, group.scope, parallelMachine); err != nil {
			return nil, err
		}
	}

	// Second pass: identify composite states with regions AND handle top-level regions
	// Check for top-level regions (state machine itself has regions as members)
	hasTopLevelRegions := len(graph.TopRegions) > 0
	for _, member := range body {
		actualMember := unwrapMembership(member.node)
		if region, ok := actualMember.(*ast.StateRegion); ok {
			hasTopLevelRegions = true
			graph.TopRegions = append(graph.TopRegions, region)
		}
	}
	// Also handle states that have regions as sub-members
	for _, state := range graph.States {
		if len(state.Regions) > 0 {
			graph.recordCompositeState(state)
		}
	}

	// Record the triggers each state defers, once every state is collected.
	for _, state := range graph.States {
		if err := collectDeferred(graph, state); err != nil {
			return nil, err
		}
	}

	// Third pass: collect transitions
	for _, group := range groupMembers(body) {
		if err := collectGroupTransitions(graph, group, group.owner, nil, nil); err != nil {
			return nil, err
		}
	}
	for _, region := range graph.TopRegions {
		graph.RegionInitials[region] = graph.unconditionalStart(region)
		if len(graph.EntryTransitions[region]) == 0 {
			if graph.regionDecl[region] != nil {
				return nil, fmt.Errorf("region %s has no initial state; write `entry; then <state>;` inside the region", region.Name)
			}
			return nil, fmt.Errorf("top-level region %s has no initial state", region.Name)
		}
	}
	for state, regions := range graph.CompositeStates {
		for _, region := range regions {
			graph.RegionInitials[region] = graph.unconditionalStart(region)
			if len(graph.EntryTransitions[region]) == 0 {
				if graph.regionDecl[region] != nil {
					return nil, fmt.Errorf("region %s has no initial state; write `entry; then <state>;` inside the region", region.Name)
				}
				return nil, fmt.Errorf("region %s in state %s has no initial state", region.Name, state.Name)
			}
		}
	}

	// A machine without top-level regions starts where its own body's entry
	// transition says; a missing one is the executor's to report at initialize().
	if !hasTopLevelRegions {
		graph.Initial = graph.unconditionalStart(nil)
	}

	graph.ownTransitionEffects()

	return graph, nil
}

// ownTransitionEffects records, on every transition effect, the state its
// transition leaves, whose attributes the effect reads and writes.
func (g *StateGraph) ownTransitionEffects() {
	for source, transitions := range g.Transitions {
		state, ok := source.(*ast.StateNode)
		if !ok {
			continue
		}
		for _, trans := range transitions {
			for i := range trans.Effect {
				trans.Effect[i].Owner = state
			}
		}
	}
}

// lowerStateAttributes returns every attribute a machine declares, its own and
// those it inherits. An unvalued attribute is still owned by the machine even
// though it supplies no initial value.
func lowerStateAttributes(graph *StateGraph, members []inheritedMember) []Attribute {
	var attrs []Attribute
	for _, member := range members {
		usage, ok := unwrapMembership(member.node).(*ast.Usage)
		if !ok || usage.Kind != ast.UsageAttribute {
			continue
		}
		name, _ := ast.EffectiveName(usage)
		if name == "" {
			continue
		}
		graph.attributeScope[usage] = member.scope
		attrs = append(attrs, Attribute{Name: name, Value: usage.Value, Node: usage, Scope: member.scope})
	}
	return attrs
}

// memberGroup is the members one body contributes to a state, in declaration
// order: a name written in them resolves in that body's scope, and a succession
// out of an entry action names one of them.
type memberGroup struct {
	owner ast.Node
	scope *symbols.Scope
	nodes []ast.Node
}

// groupMembers gathers members by the body declaring them, keeping the order
// they contribute their content in.
func groupMembers(members []inheritedMember) []memberGroup {
	var groups []memberGroup
	for _, member := range members {
		if len(groups) > 0 {
			last := &groups[len(groups)-1]
			if last.owner == member.owner && last.scope == member.scope {
				last.nodes = append(last.nodes, member.node)
				continue
			}
		}
		groups = append(groups, memberGroup{owner: member.owner, scope: member.scope, nodes: []ast.Node{member.node}})
	}
	return groups
}

// ownMembers pairs the members written in a body with the scope of that body.
func ownMembers(members []ast.Node, scope *symbols.Scope) []inheritedMember {
	owned := make([]inheritedMember, 0, len(members))
	for _, member := range members {
		owned = append(owned, inheritedMember{node: member, scope: scope})
	}
	return owned
}

// AttributeScope is the scope an attribute's value resolves in, which is the
// body declaring it rather than the machine's when the machine inherits it.
func (g *StateGraph) AttributeScope(attr Attribute) *symbols.Scope {
	if scope := g.attributeScope[attr.Node]; scope != nil {
		return scope
	}
	return g.Scope
}

// lowerStateBehaviors lowers a state's entry, do and exit behaviors, each in the
// scope the body declaring it was written in.
func (g *StateGraph) lowerStateBehaviors(state *ast.StateNode, scope *symbols.Scope) *StateBehaviors {
	return &StateBehaviors{
		Entry: g.lowerBehaviorsFor(state, state.Entry, scope),
		Do:    g.lowerBehaviorsFor(state, state.Do, scope),
		Exit:  g.lowerBehaviorsFor(state, state.Exit, scope),
	}
}

// lowerBehaviorsFor lowers the behaviors of one state, in the scope each was
// declared in, and records which state runs them.
func (g *StateGraph) lowerBehaviorsFor(state *ast.StateNode, actions []ast.Node, scope *symbols.Scope) []StateBehavior {
	if len(actions) == 0 {
		return nil
	}
	behaviors := make([]StateBehavior, 0, len(actions))
	for _, action := range actions {
		actual := unwrapMembership(action)
		if actual == nil {
			continue
		}
		declared := scope
		if inherited := g.behaviorScope[actual]; inherited != nil {
			declared = inherited
		}
		behavior := lowerStateBehavior(actual, declared)
		behavior.Owner = state
		behaviors = append(behaviors, behavior)
	}
	return behaviors
}

// stateNodeFromUsage builds the state node for `state <name> : Def { … }`: the
// content the definition typing it declares, then the content its own body
// declares, which adds to that and redefines it. scope is the scope the usage
// itself was written in.
func stateNodeFromUsage(graph *StateGraph, usage *ast.Usage, scope *symbols.Scope) (*ast.StateNode, error) {
	name, _ := ast.EffectiveName(usage)
	state := &ast.StateNode{Name: name}
	state.NodeSpan = usage.NodeSpan
	graph.declOf[state] = usage
	bodyScope := childScope(scope, usage)
	graph.scopeOf[state] = bodyScope

	inherited, owners, err := graph.inheritedContent(usage, scope)
	if err != nil {
		return nil, err
	}
	parallel := usage.IsParallel
	for _, owner := range owners {
		parallel = parallel || stateMachineIsParallel(owner)
	}

	base := &stateContent{node: &ast.StateNode{Name: name}}
	if len(inherited) > 0 {
		for _, owner := range owners {
			graph.materializing[owner] = true
		}
		err := graph.addMembers(base, inherited, parallel)
		for _, owner := range owners {
			graph.materializing[owner] = false
		}
		if err != nil {
			return nil, err
		}
	}

	own := &stateContent{node: &ast.StateNode{Name: name}}
	if err := graph.addMembers(own, ownMembers(usage.Members, bodyScope), parallel); err != nil {
		return nil, err
	}

	replaced := redeclare(state, base.node, own.node)
	if attrs := keptAttributes(base.attrs, own.attrs); len(attrs) > 0 {
		graph.StateAttributes[state] = attrs
	}
	graph.bodyOf[state] = append(inherited, ownMembers(usage.Members, bodyScope)...)
	graph.parallelState[state] = parallel
	if len(inherited) > 0 {
		graph.newInstance(state, inherited, owners, replaced)
	}
	return state, nil
}

// newStateGraph is an empty graph of a machine whose body scope is scope and
// whose endpoints resolve through endpoints.
func newStateGraph(scope *symbols.Scope, endpoints EndpointResolver) *StateGraph {
	return &StateGraph{
		Scope:               scope,
		vertexOf:            make(map[ast.Node]ast.Node),
		stateByDecl:         make(map[ast.Node]*ast.StateNode),
		completing:          make(map[*ast.StateNode]bool),
		StateAttributes:     make(map[*ast.StateNode][]Attribute),
		instanceOf:          make(map[*ast.StateNode]*stateInstance),
		materializing:       make(map[ast.Node]bool),
		scopeOf:             make(map[*ast.StateNode]*symbols.Scope),
		regionScopeOf:       make(map[*ast.StateRegion]*symbols.Scope),
		behaviorScope:       make(map[ast.Node]*symbols.Scope),
		attributeScope:      make(map[ast.Node]*symbols.Scope),
		bodyOf:              make(map[*ast.StateNode][]inheritedMember),
		parallelState:       make(map[*ast.StateNode]bool),
		completionOf:        make(map[ast.Node]*ast.StateNode),
		endpoints:           endpoints,
		StateScopes:         make(map[*ast.StateNode]*symbols.Scope),
		Behaviors:           make(map[*ast.StateNode]*StateBehaviors),
		HiddenStates:        make(map[*ast.StateNode]bool),
		HiddenRegionOf:      make(map[*ast.StateNode]*ast.StateRegion),
		RegionState:         make(map[*ast.StateRegion]*ast.StateNode),
		declOf:              make(map[*ast.StateNode]ast.Node),
		States:              make([]*ast.StateNode, 0),
		Pseudostates:        make([]*ast.PseudostateNode, 0),
		PseudostateOwner:    make(map[*ast.PseudostateNode]*ast.StateNode),
		Transitions:         make(map[ast.Node][]*Transition),
		CompositeStates:     make(map[*ast.StateNode][]*ast.StateRegion),
		CompositeStateOrder: make([]*ast.StateNode, 0),
		RegionInitials:      make(map[*ast.StateRegion]*ast.StateNode),
		ParentState:         make(map[*ast.StateNode]*ast.StateNode),
		RegionOwner:         make(map[*ast.StateRegion]*ast.StateNode),
		RegionOf:            make(map[*ast.StateNode]*ast.StateRegion),
		Deferred:            make(map[*ast.StateNode][]ast.Node),
		regionDecl:          make(map[*ast.StateRegion]ast.Node),

		designatedInitials: make(map[*ast.StateNode]bool),
		EntryTransitions:   make(map[ast.Node][]*EntryTransition),
	}
}

// Completes reports whether entering state completes the region it belongs to:
// it is the `done` end shot of the state or machine whose body names it.
func (g *StateGraph) Completes(state *ast.StateNode) bool {
	return state != nil && g.completing[state]
}

// completion is the `done` vertex of the body owner owns, synthesized on first
// use and placed where a state declared in that body would be, so completion
// belongs to the same region a declared vertex there would.
func (g *StateGraph) completion(owner ast.Node, scope *symbols.Scope, span source.Span) *ast.StateNode {
	if vertex := g.findCompletion(owner); vertex != nil {
		return vertex
	}
	vertex := &ast.StateNode{NodeBase: ast.NodeBase{NodeSpan: span}, Name: ast.DoneFeature}
	switch o := owner.(type) {
	case *ast.StateRegion:
		g.RegionOf[vertex] = o
		if parent := g.RegionOwner[o]; parent != nil {
			g.ParentState[vertex] = parent
		}
	default:
		if parent := g.findStateDecl(owner); parent != nil {
			g.ParentState[vertex] = parent
			if region := g.HiddenRegionOf[parent]; region != nil {
				g.RegionOf[vertex] = region
			}
		}
	}
	g.States = append(g.States, vertex)
	g.addVertex(vertex)
	g.StateScopes[vertex] = scope
	g.Behaviors[vertex] = &StateBehaviors{}
	g.completing[vertex] = true
	g.putCompletion(owner, vertex)
	return vertex
}

// completionOwner is the body a `done` written among node's members completes:
// a state standing for an orthogonal region completes that region, any other
// state defers to the region or machine it is nested in.
func (g *StateGraph) completionOwner(node, outer ast.Node) ast.Node {
	if state := g.findStateDecl(node); state != nil && g.HiddenRegionOf[state] != nil {
		return node
	}
	return outer
}

// isDoneEndpoint reports whether an endpoint names the end shot every state
// inherits rather than a vertex of its own: the unqualified `done`.
func isDoneEndpoint(qn *ast.QualifiedName) bool {
	return qn != nil && len(qn.Parts) == 1 && qn.Parts[0].Text == ast.DoneFeature
}

// targetVertex is the vertex a transition ends at: the one its endpoint names,
// or the completion of the body it is written in when that endpoint is `done`
// and no vertex of the machine is declared under that name.
func (g *StateGraph) targetVertex(scope *symbols.Scope, qn *ast.QualifiedName, owner ast.Node) (ast.Node, error) {
	node, err := g.vertex(scope, qn)
	if node == nil && isDoneEndpoint(qn) {
		return g.completion(owner, scope, qn.Span()), nil
	}
	return node, err
}

// IsInitial reports whether the machine starts in state, which a transition out
// of the body's entry action designates.
func (g *StateGraph) IsInitial(state *ast.StateNode) bool {
	return state != nil && g.designatedInitials[state]
}

// designateInitial records state as one the machine starts in.
func (g *StateGraph) designateInitial(state *ast.StateNode) {
	g.designatedInitials[state] = true
}

// machineMembers is the body of a state machine declaration.
func machineMembers(stateMachineDecl ast.Node) ([]ast.Node, error) {
	switch n := stateMachineDecl.(type) {
	case *ast.Usage:
		return n.Members, nil
	case *ast.Definition:
		return n.Members, nil
	}
	return nil, fmt.Errorf("state machine must be Usage or Definition, got %T", stateMachineDecl)
}

// collectVertices records every state and pseudostate the machine body declares,
// which are the vertices its transitions may name (UML 2.5.1 §14.2.3.9).
func collectVertices(graph *StateGraph, members []ast.Node, scope *symbols.Scope, parallel bool) error {
	if parallel {
		regions, err := graph.parallelRegions(ownMembers(members, scope), nil)
		if err != nil {
			return err
		}
		graph.TopRegions = append(graph.TopRegions, regions...)
	}
	for _, member := range members {
		actual := unwrapMembership(member)
		if parallel && isParallelRegionMember(actual) {
			continue
		}
		switch n := actual.(type) {
		case *ast.StateNode:
			if err := collectStates(graph, n, nil, graph.stateScope(scope, n)); err != nil {
				return err
			}
		case *ast.Usage:
			// `state <name> { … }`, parsed as a usage rather than a state node.
			if n.Kind == ast.UsageState {
				// The state node records the usage it came from, so its scope is the
				// one that usage declares: build it before asking for the scope.
				state, err := stateNodeFromUsage(graph, n, scope)
				if err != nil {
					return err
				}
				if err := collectStates(graph, state, nil, graph.stateScope(scope, state)); err != nil {
					return err
				}
			}
		case *ast.SubstateMember:
			// `state <name>;`, which declares a state with no body.
			stateNode := &ast.StateNode{Name: n.Name}
			stateNode.NodeSpan = n.NodeSpan
			graph.declOf[stateNode] = n
			if err := collectStates(graph, stateNode, nil, graph.stateScope(scope, stateNode)); err != nil {
				return err
			}
		case *ast.StateRegion:
			// Top-level region: collect its states, which no state is the parent of
			if err := collectRegionStates(graph, n, nil, childScope(scope, n)); err != nil {
				return err
			}
		case *ast.PseudostateNode:
			graph.addPseudostate(n)
		case *ast.DeferMember:
			// The machine's own body has no state to defer for: an event deferred
			// there would be retained for the whole run and never redelivered.
			return fmt.Errorf("defer must be declared inside a state, not in the state machine body")
		}
	}
	return nil
}

// collectStates recursively collects states and builds parent relationships.
// scope is the scope the state's own body was declared in, from which the scope
// of each of its substates and regions is derived.
func collectStates(graph *StateGraph, state *ast.StateNode, parent *ast.StateNode, scope *symbols.Scope) error {
	graph.States = append(graph.States, state)
	graph.addVertex(state)
	graph.recordDecl(state)
	graph.StateScopes[state] = scope
	graph.Behaviors[state] = graph.lowerStateBehaviors(state, scope)
	if parent != nil {
		graph.ParentState[state] = parent
	}
	// The content a state inherits is this usage's own: it is collected inside the
	// state's materialization, so a second usage of the same definition collects a
	// second set of vertices rather than sharing these.
	if inst := graph.instanceOf[state]; inst != nil {
		graph.push(inst)
		defer graph.pop()
	}
	return collectStateContents(graph, state, scope)
}

// collectGraphOnlyState records a synthesized state's behavior and hierarchy
// without exposing the region owner as a user-visible graph vertex.
func collectGraphOnlyState(graph *StateGraph, state *ast.StateNode, parent *ast.StateNode, scope *symbols.Scope) error {
	graph.HiddenStates[state] = true
	graph.recordDecl(state)
	graph.StateScopes[state] = scope
	graph.Behaviors[state] = graph.lowerStateBehaviors(state, scope)
	if parent != nil {
		graph.ParentState[state] = parent
	}
	if err := collectDeferred(graph, state); err != nil {
		return err
	}
	if inst := graph.instanceOf[state]; inst != nil {
		graph.push(inst)
		defer graph.pop()
	}
	return collectStateContents(graph, state, scope)
}

func collectStateContents(graph *StateGraph, state *ast.StateNode, scope *symbols.Scope) error {
	if graph.parallelState[state] {
		regions, err := graph.parallelRegions(graph.bodyOf[state], state)
		if err != nil {
			return err
		}
		state.Regions = append(state.Regions, regions...)
	}
	if len(state.Regions) > 0 {
		graph.recordCompositeState(state)
	}

	// Recursively collect substates
	for _, substate := range state.Substates {
		switch child := unwrapMembership(substate).(type) {
		case *ast.StateNode:
			if err := collectStates(graph, child, state, graph.stateScope(scope, child)); err != nil {
				return err
			}
		case *ast.PseudostateNode:
			// A pseudostate declared inside a composite state belongs to it: that
			// ownership is what a history pseudostate restores from, and without it
			// a nested pseudostate is not part of the graph at all.
			graph.addPseudostate(child)
			graph.PseudostateOwner[child] = state
		}
	}

	// Collect states in orthogonal regions
	for _, region := range state.Regions {
		if graph.regionDecl[region] != nil {
			continue
		}
		if err := collectRegionStates(graph, region, state, graph.regionScope(scope, region)); err != nil {
			return err
		}
	}
	return nil
}

// recordCompositeState registers a state's regions while retaining their
// declaration order for deterministic consumers.
func (g *StateGraph) recordCompositeState(state *ast.StateNode) {
	if _, exists := g.CompositeStates[state]; !exists {
		g.CompositeStateOrder = append(g.CompositeStateOrder, state)
	}
	g.CompositeStates[state] = state.Regions
	for _, region := range state.Regions {
		g.RegionOwner[region] = state
	}
}

// stateScope returns the scope state's body was declared in, given the scope of
// the body that declares it. A state lowering synthesized from a usage or a bare
// substate declaration is keyed in the scope tree by that declaration.
func (g *StateGraph) stateScope(parent *symbols.Scope, state *ast.StateNode) *symbols.Scope {
	if scope := g.scopeOf[state]; scope != nil {
		return scope
	}
	decl := ast.Node(state)
	if origin, ok := g.declOf[state]; ok {
		decl = origin
	}
	return childScope(parent, decl)
}

// collectRegionStates collects the states an orthogonal region declares, records
// which region declares each of them and which one the region starts in, and
// assigns the region's pseudostates to the state that owns the region. parent is
// the state owning the region, nil for the machine's own regions.
//
// Region members reach here as a state node, a bare `state <name>;` substate or a
// state usage with a body, each of them possibly wrapped in a membership: a state
// missed here is a state no transition can name.
func collectRegionStates(graph *StateGraph, region *ast.StateRegion, parent *ast.StateNode, scope *symbols.Scope) error {
	for _, member := range region.States {
		var state *ast.StateNode
		switch n := unwrapMembership(member).(type) {
		case *ast.StateNode:
			state = n
		case *ast.SubstateMember:
			state = &ast.StateNode{Name: n.Name}
			state.NodeSpan = n.NodeSpan
			graph.declOf[state] = n
		case *ast.Usage:
			if n.Kind != ast.UsageState {
				continue
			}
			built, err := stateNodeFromUsage(graph, n, scope)
			if err != nil {
				return err
			}
			state = built
		case *ast.PseudostateNode:
			graph.addPseudostate(n)
			if parent != nil {
				graph.PseudostateOwner[n] = parent
			}
			continue
		case *ast.DeferMember:
			// A region is not a state: only a state can retain an event.
			return fmt.Errorf("defer must be declared inside a state, not in a region body")
		default:
			continue
		}

		if err := collectStates(graph, state, parent, graph.stateScope(scope, state)); err != nil {
			return err
		}
		graph.RegionOf[state] = region
		if graph.IsInitial(state) && graph.RegionInitials[region] == nil {
			graph.RegionInitials[region] = state
		}
	}
	return nil
}

// stateMachineIsParallel reports whether a state definition or usage marks its
// direct substates as orthogonal regions.
func stateMachineIsParallel(decl ast.Node) bool {
	switch n := decl.(type) {
	case *ast.Definition:
		return n.IsParallel
	case *ast.Usage:
		return n.IsParallel
	default:
		return false
	}
}

// isParallelRegionMember reports whether a direct parallel-body member supplies
// one region; each such substate contributes its own body as region contents.
func isParallelRegionMember(member ast.Node) bool {
	switch n := member.(type) {
	case *ast.StateNode, *ast.SubstateMember:
		return true
	case *ast.Usage:
		return n.Kind == ast.UsageState
	default:
		return false
	}
}

// parallelOwnedMember reports whether a parallel state may own a member itself
// rather than contribute it to a region: its behaviors, its deferred events, the
// pseudostates its regions branch through, the edges between them, and a
// definition written in its body, which declares a type rather than a region.
func parallelOwnedMember(member ast.Node) bool {
	switch n := member.(type) {
	case *ast.Comment, *ast.Documentation, *ast.TextualRepresentation,
		*ast.EntryMember, *ast.DoMember, *ast.ExitMember,
		*ast.PseudostateNode, *ast.DeferMember,
		*ast.SuccessionEdge, *ast.TransitionEdge, *ast.TransitionMember,
		*ast.Definition, *ast.Package, *ast.ErrorNode:
		return true
	case *ast.Usage:
		switch n.Kind {
		case ast.UsageAttribute, ast.UsagePort, ast.UsageSuccession:
			return true
		}
	}
	return false
}

// parallelRegions synthesizes the regions represented by direct substates of a
// parallel body and lowers each substate body into the existing region IR.
func (g *StateGraph) parallelRegions(members []inheritedMember, parent *ast.StateNode) ([]*ast.StateRegion, error) {
	regions := make([]*ast.StateRegion, 0)
	for _, member := range members {
		actual := unwrapMembership(member.node)
		// Only state substates become regions; the members a parallel state may own
		// itself are collected with the rest of its body.
		if !isParallelRegionMember(actual) {
			if !parallelOwnedMember(actual) {
				return nil, fmt.Errorf("%w: parallel state body contains unsupported member %s; a parallel state's direct substates are its orthogonal regions",
					ErrUnsupportedStateContent, DescribeMember(actual))
			}
			continue
		}

		wrapper, err := parallelRegionState(g, actual, member.scope)
		if err != nil {
			return nil, err
		}
		region := &ast.StateRegion{
			NodeBase: ast.NodeBase{NodeSpan: actual.Span()},
			Name:     wrapper.Name,
			States:   regionBody(g, wrapper, actual),
		}
		g.regionDecl[region] = actual
		g.HiddenRegionOf[wrapper] = region
		g.RegionState[region] = wrapper
		before := len(g.States)
		if err := collectGraphOnlyState(g, wrapper, parent, g.stateScope(member.scope, wrapper)); err != nil {
			return nil, err
		}
		for _, state := range g.States[before:] {
			if g.ParentState[state] == wrapper {
				g.RegionOf[state] = region
			}
		}
		regions = append(regions, region)
	}
	return regions, nil
}

// machineState is the graph-only root state carrying the machine's own entry,
// do and exit behaviors: those its body states, or else those it inherits.
func (g *StateGraph) machineState(decl ast.Node, inherited []inheritedMember, members []ast.Node, scope *symbols.Scope) *ast.StateNode {
	inheritedNodes := make([]ast.Node, 0, len(inherited))
	for _, member := range inherited {
		inheritedNodes = append(inheritedNodes, member.node)
		g.recordBehaviorScope(member.node, member.scope)
	}
	state := parallelMachineState(decl, members)
	_ = redeclare(state, parallelMachineState(decl, inheritedNodes), state)
	g.StateScopes[state] = scope
	g.Behaviors[state] = g.lowerStateBehaviors(state, scope)
	return state
}

// recordBehaviorScope records the scope an inherited entry, do or exit member's
// actions were declared in.
func (g *StateGraph) recordBehaviorScope(member ast.Node, scope *symbols.Scope) {
	switch m := unwrapMembership(member).(type) {
	case *ast.EntryMember:
		g.behaviorsIn(m.Actions, scope)
	case *ast.DoMember:
		g.behaviorsIn(m.Actions, scope)
	case *ast.ExitMember:
		g.behaviorsIn(m.Actions, scope)
	}
}

// parallelMachineState preserves behaviors owned by the parallel state itself.
func parallelMachineState(decl ast.Node, members []ast.Node) *ast.StateNode {
	state := &ast.StateNode{NodeBase: ast.NodeBase{NodeSpan: decl.Span()}}
	switch n := decl.(type) {
	case *ast.Usage:
		state.Name, _ = ast.EffectiveName(n)
	case *ast.Definition:
		state.Name = n.Ident.Name
	}
	for _, member := range members {
		switch n := unwrapMembership(member).(type) {
		case *ast.EntryMember:
			state.Entry = append(state.Entry, n.Actions...)
		case *ast.DoMember:
			state.Do = append(state.Do, n.Actions...)
		case *ast.ExitMember:
			state.Exit = append(state.Exit, n.Actions...)
		}
	}
	return state
}

// parallelRegionState creates the graph state for a direct substate that owns
// a synthesized region, preserving its behaviors, its deferred triggers and the
// content it inherits from the definition typing it.
func parallelRegionState(graph *StateGraph, member ast.Node, scope *symbols.Scope) (*ast.StateNode, error) {
	switch n := member.(type) {
	case *ast.Usage:
		return stateNodeFromUsage(graph, n, scope)
	case *ast.StateNode:
		return n, nil
	case *ast.SubstateMember:
		state := &ast.StateNode{
			NodeBase: ast.NodeBase{NodeSpan: n.NodeSpan},
			Name:     n.Name,
		}
		graph.declOf[state] = n
		return state, nil
	default:
		return &ast.StateNode{}, nil
	}
}

// regionBody is the body a direct substate contributes to its synthesized
// region: what it declares itself, and what it inherits.
func regionBody(graph *StateGraph, wrapper *ast.StateNode, member ast.Node) []ast.Node {
	if body, ok := graph.bodyOf[wrapper]; ok {
		nodes := make([]ast.Node, 0, len(body))
		for _, m := range body {
			nodes = append(nodes, m.node)
		}
		return nodes
	}
	_, body := parallelRegionBody(member)
	return body
}

// parallelRegionBody returns the name and body a direct substate contributes to
// its synthesized region.
func parallelRegionBody(member ast.Node) (string, []ast.Node) {
	switch n := member.(type) {
	case *ast.Usage:
		name, _ := ast.EffectiveName(n)
		return name, n.Members
	case *ast.StateNode:
		body := make([]ast.Node, 0, len(n.Entry)+len(n.Do)+len(n.Exit)+len(n.Defer)+len(n.Substates)+len(n.Regions))
		body = append(body, n.Entry...)
		body = append(body, n.Do...)
		body = append(body, n.Exit...)
		body = append(body, n.Defer...)
		body = append(body, n.Substates...)
		for _, region := range n.Regions {
			body = append(body, region)
		}
		return n.Name, body
	case *ast.SubstateMember:
		return n.Name, nil
	default:
		return "", nil
	}
}

// regionScope uses the source substate scope for synthesized regions and the
// region's own scope for regions written with the extension syntax.
func (g *StateGraph) regionScope(scope *symbols.Scope, region *ast.StateRegion) *symbols.Scope {
	if inherited := g.regionScopeOf[region]; inherited != nil {
		return inherited
	}
	if decl := g.regionDecl[region]; decl != nil {
		return childScope(scope, decl)
	}
	return childScope(scope, region)
}

// collectDeferred records the triggers a state defers, normalized the same way
// transition triggers are. Only a signal or a call can be deferred: a time or
// change event is not dispatched from the event pool, so retaining one has no
// meaning and is reported rather than silently ignored.
func collectDeferred(graph *StateGraph, state *ast.StateNode) error {
	for _, trigger := range state.Defer {
		if trigger == nil {
			return fmt.Errorf("state %s defers a nil trigger", state.Name)
		}
		switch typed := classifyTrigger(trigger).(type) {
		case *ast.AcceptEvent:
			if ast.SimpleName(typed.SignalType) == "" {
				return fmt.Errorf("state %s defers a signal trigger that names no signal", state.Name)
			}
			graph.Deferred[state] = append(graph.Deferred[state], typed)
		case *ast.CallEvent:
			graph.Deferred[state] = append(graph.Deferred[state], typed)
		default:
			return fmt.Errorf("state %s defers a %T trigger: only signal and call triggers can be deferred", state.Name, typed)
		}
	}
	return nil
}

// endpointVertex is the vertex a succession's or a marker's endpoint names, or
// nil when it names none: the same resolution a transition's endpoints get, so a
// nested or region-local vertex is reached and a same-named one elsewhere is not.
// A succession naming no vertex is left out of the graph, which the
// name-resolution tier reports about (UML 2.5.1 §14.2.3.9 leniency).
func (g *StateGraph) endpointVertex(scope *symbols.Scope, qn *ast.QualifiedName) ast.Node {
	node, err := g.vertex(scope, qn)
	if err != nil {
		return nil
	}
	return node
}

// endpointState is endpointVertex where only a state will do, as for the state a
// machine or a region starts in.
func (g *StateGraph) endpointState(scope *symbols.Scope, qn *ast.QualifiedName) *ast.StateNode {
	state, _ := g.endpointVertex(scope, qn).(*ast.StateNode)
	return state
}

// addVertex records the graph node standing for a state, and for the
// declaration it was built from, which is what an endpoint resolves to.
func (g *StateGraph) addVertex(state *ast.StateNode) {
	g.putVertex(state, state)
	if decl, ok := g.declOf[state]; ok {
		g.putVertex(decl, state)
	}
}

// recordDecl records which state a body declaration was lowered into, the state
// itself included, since a hand-built graph declares no separate node.
func (g *StateGraph) recordDecl(state *ast.StateNode) {
	g.putStateDecl(state, state)
	if decl, ok := g.declOf[state]; ok {
		g.putStateDecl(decl, state)
	}
}

// addPseudostate records a pseudostate as a vertex of the graph.
func (g *StateGraph) addPseudostate(ps *ast.PseudostateNode) {
	g.Pseudostates = append(g.Pseudostates, ps)
	g.putVertex(ps, ps)
}

// vertex is the graph node a transition endpoint names: name resolution says
// which declaration it reaches, and lowering collected a vertex from that. An
// endpoint naming no vertex yields a nil node and no error — the name-resolution
// tier reports it, and how strictly the machine then runs is the runtime's call.
func (g *StateGraph) vertex(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, error) {
	if qn == nil {
		return nil, fmt.Errorf("transition endpoint names nothing")
	}
	decl, ok := g.endpoints.Endpoint(scope, qn)
	if !ok {
		return nil, nil
	}
	node, ok := g.vertexFor(scope, qn, decl)
	if !ok {
		return nil, fmt.Errorf(NotAVertexFormat, endpointText(qn), VertexKind(decl))
	}
	return node, nil
}

// vertexFor is the graph node an endpoint names. A qualified endpoint reaching
// into content a state inherits (`nested.i1`) names that state's own copy of the
// declaration, which is held by its materialization rather than by the machine.
func (g *StateGraph) vertexFor(scope *symbols.Scope, qn *ast.QualifiedName, decl ast.Node) (ast.Node, bool) {
	if node, ok := g.findVertex(decl); ok {
		return node, true
	}
	if qn == nil || len(qn.Parts) < 2 {
		return nil, false
	}
	prefix := &ast.QualifiedName{Parts: qn.Parts[:len(qn.Parts)-1]}
	ownerDecl, ok := g.endpoints.Endpoint(scope, prefix)
	if !ok {
		return nil, false
	}
	ownerNode, ok := g.vertexFor(scope, prefix, ownerDecl)
	if !ok {
		return nil, false
	}
	ownerState, ok := ownerNode.(*ast.StateNode)
	if !ok {
		return nil, false
	}
	inst := g.instanceOf[ownerState]
	if inst == nil {
		return nil, false
	}
	node, ok := inst.vertexOf[decl]
	return node, ok
}

// NotAVertexFormat reports an endpoint naming an element outside the machine's
// vertices, shared by the check reporting it and the lowering backstopping it.
const NotAVertexFormat = "transition endpoint %s names a %s that is not a vertex of this state machine"

// VertexKind names what an endpoint reached in modelling terms, for a message a
// modeller reads.
func VertexKind(decl ast.Node) string {
	switch decl.(type) {
	case *ast.StateNode, *ast.SubstateMember, *ast.Usage:
		return "state"
	case *ast.PseudostateNode:
		return "pseudostate"
	case *ast.InitialNode:
		return "start marker"
	case *ast.FinalNode:
		return "end marker"
	}
	return "element"
}

// endpointText renders an endpoint name for an error message.
func endpointText(qn *ast.QualifiedName) string {
	parts := make([]string, 0, len(qn.Parts))
	for _, p := range qn.Parts {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, "::")
}

// lowerTransitionEdge converts a TransitionEdge (legacy) to a Transition. No
// document declares such an edge, so an endpoint naming no vertex reports here.
// scope is the scope the edge was declared in.
func lowerTransitionEdge(graph *StateGraph, edge *ast.TransitionEdge, owner ast.Node, scope *symbols.Scope) (*Transition, error) {
	source, err := graph.vertex(scope, edge.Source)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("transition edge references undefined source state %s", endpointText(edge.Source))
	}
	target, err := graph.targetVertex(scope, edge.Target, owner)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, fmt.Errorf("transition edge references undefined target state %s", endpointText(edge.Target))
	}

	return &Transition{
		Decl:      edge,
		Source:    source,
		Target:    target,
		Trigger:   edge.Trigger,
		Guard:     edge.Guard,
		Effect:    LowerBehaviors(edge.Effect, scope),
		Scope:     scope,
		BodyScope: scope,
	}, nil
}

// lowerTransitionMember converts a TransitionMember (parser output) to a Transition.
// body is the ordered body it was written in; a sourceless one leaves the member before it.
// scope is the scope the transition was declared in.
func lowerTransitionMember(graph *StateGraph, member *ast.TransitionMember, body []ast.Node, owner ast.Node, scope *symbols.Scope) (*Transition, error) {
	var source ast.Node
	if member.Source == nil {
		decl, err := ImplicitSource(body, member)
		if err != nil {
			return nil, err
		}
		vertex, ok := graph.findVertex(decl)
		if !ok || !IsStateSource(decl) {
			state := graph.findStateDecl(decl)
			return nil, &TransitionSourceError{Source: decl, Region: state != nil && graph.HiddenRegionOf[state] != nil}
		}
		source = vertex
	} else {
		vertex, err := graph.vertex(scope, member.Source)
		if err != nil || vertex == nil {
			return nil, err
		}
		source = vertex
	}

	// A transition the parser could not read a target from names no edge.
	if member.Target == nil {
		return nil, fmt.Errorf("transition %s names no target", orAnonymous(member.Name))
	}

	target, err := graph.targetVertex(scope, member.Target, owner)
	if err != nil || target == nil {
		return nil, err
	}

	// A trigger's parameters are members of a scope of the transition's own, which
	// its guard and effect resolve in (symbols/bodyscopes.go).
	bodyScope := symbols.TriggerScope(scope, member)
	return &Transition{
		Name:      member.Name,
		Decl:      member,
		Source:    source,
		Target:    target,
		Trigger:   classifyTrigger(member.Trigger),
		Guard:     member.Guard,
		Effect:    transitionEffects(member, bodyScope),
		Via:       FeaturePath(member.Via),
		Scope:     scope,
		BodyScope: bodyScope,
	}, nil
}

// transitionEffects are the behaviors a transition performs: those written with
// `do`, then the steps its body states (SysML.xtext:1863, where TransitionUsage
// ends in ActionBody).
func transitionEffects(member *ast.TransitionMember, scope *symbols.Scope) []StateBehavior {
	effects := LowerBehaviors(member.Effect, scope)
	return append(effects, LowerBehaviors(BodyStatementMembers(member.Members), scope)...)
}

// isEntrySubaction reports whether member is the entry subaction of the body a
// succession was written in.
func isEntrySubaction(member ast.Node) bool {
	_, ok := unwrapMembership(member).(*ast.EntryMember)
	return ok
}

// startsAt records the state a bare completion transition out of the body's entry
// action starts the machine in, as `entry; then off;` does, and reports whether
// it did. owner is the body a `done` target completes, entryOwner the body it
// starts, as entryOwner names it; guard is the condition a
// `transition initial if c then off;` chooses it under, nil otherwise.
func (g *StateGraph) startsAt(decl, guard ast.Node, members []ast.Node, containingState, owner, entryOwner ast.Node, scope *symbols.Scope, source, target *ast.QualifiedName) (bool, error) {
	if source == nil || target == nil {
		return false, nil
	}
	entry, ok := g.endpoints.Endpoint(scope, source)
	if !ok {
		return false, nil
	}
	if !ast.IsEntryAction(ast.EntryActions(members), entry) &&
		!ast.IsEntryAction(ast.StateEntryActions(containingState), entry) {
		return false, nil
	}
	vertex, err := g.targetVertex(scope, target, owner)
	if err != nil || vertex == nil {
		return false, err
	}
	start, ok := vertex.(*ast.StateNode)
	if !ok {
		return false, &EntryTransitionTargetError{Target: vertex}
	}
	g.addEntryTransition(entryOwner, &EntryTransition{Decl: decl, Guard: guard, Target: start, Scope: scope})
	return true, nil
}

// classifyTrigger converts a raw trigger expression into a typed TriggerEvent.
// Classification rules (syntactic/structural):
// - nil → nil (completion transition)
// - *ast.TimeEvent → keep as-is (already typed from hand-built tests)
// - *ast.ChangeEvent → keep as-is
// - *ast.AcceptEvent → keep as-is
// - *ast.CallEvent → keep as-is
// - QualifiedName (bare name) → AcceptEvent{SignalType: qname} (signal trigger)
// - Expression with operators → ChangeEvent{Condition: expr} (guard-like condition)
//
// This is a syntactic heuristic - full signal vs feature disambiguation
// requires type system integration. Adequate for current SysML v2 syntax.
func classifyTrigger(trigger ast.Node) ast.Node {
	if trigger == nil {
		return nil
	}

	// Already typed events pass through
	switch trigger.(type) {
	case *ast.TimeEvent, *ast.ChangeEvent, *ast.AcceptEvent, *ast.CallEvent:
		return trigger
	}

	// A payload parameter: the accept states the occurrence it takes as a
	// parameter declaration — by its type (`accept msg : Warning`), by the event
	// it subsets (`accept :> shutDown`) — or carries a time or change event the
	// parameter was parsed with (`accept when x > 1`).
	if payload, ok := trigger.(*ast.Usage); ok {
		if payload.Value != nil {
			return classifyTrigger(payload.Value)
		}
		return &ast.AcceptEvent{
			SignalType: relationshipTarget(payload, ast.RelTyping),
			Subsets:    relationshipTarget(payload, ast.RelSubsets, ast.RelRedefines, ast.RelSpecializes, ast.RelReferences),
			Payload:    payload,
		}
	}

	// FeatureReference (bare name) → extract QualifiedName and treat as signal trigger
	if featureRef, ok := trigger.(*ast.FeatureReference); ok {
		return &ast.AcceptEvent{
			SignalType: featureRef.Name,
		}
	}

	// QualifiedName (bare name, though parser usually wraps in FeatureReference) → signal trigger
	if qname, ok := trigger.(*ast.QualifiedName); ok {
		return &ast.AcceptEvent{
			SignalType: qname,
		}
	}

	// Any other expression → change event (guard-like condition)
	return &ast.ChangeEvent{
		Condition: trigger,
	}
}

// relationshipTarget returns the qualified name a usage's first relationship of
// one of the given kinds names, or nil when it declares none.
func relationshipTarget(usage *ast.Usage, kinds ...ast.RelationshipKind) *ast.QualifiedName {
	for _, rel := range usage.Relationships {
		if rel == nil {
			continue
		}
		for _, kind := range kinds {
			if rel.Kind != kind {
				continue
			}
			if qn, ok := rel.Target.(*ast.QualifiedName); ok {
				return qn
			}
		}
	}
	return nil
}

// collectStateTransitions collects the transitions a state usage carries: those
// its own body declares and those it inherits, each lowered in the scope of the
// body that wrote it and against this usage's own vertices.
func collectStateTransitions(graph *StateGraph, usage *ast.Usage, owner ast.Node) error {
	state := graph.findStateDecl(usage)
	if state == nil {
		return nil
	}
	if inst := graph.instanceOf[state]; inst != nil {
		graph.push(inst)
		defer graph.pop()
	}
	for _, group := range groupMembers(graph.bodyOf[state]) {
		containing := group.owner
		if containing == nil {
			containing = usage
		}
		if err := collectGroupTransitions(graph, group, containing,
			graph.completionOwner(containing, owner), graph.entryOwner(state)); err != nil {
			return err
		}
	}
	return nil
}

// collectGroupTransitions lowers the transitions one body contributes to a state.
// A group without an owner is the state's own body, whose entry transitions
// replace the inherited ones.
func collectGroupTransitions(graph *StateGraph, group memberGroup, containingState, owner, entryOwner ast.Node) error {
	collect := func() error {
		return collectTransitions(graph, group.nodes, containingState, owner, entryOwner, group.scope)
	}
	if group.owner != nil {
		return collect()
	}
	return graph.withOwnEntryTransitions(entryOwner, collect)
}

// collectTransitions recursively processes member lists to collect transitions.
// Handles top-level members and region members.
// memberList is one body in declaration order (a sourceless transition leaves the member
// before it); containingState is the state whose body it is, nil at the top level.
// scope is the scope the members were declared in, in which their endpoints name
// the vertices they reach.
// owner is the region whose body memberList belongs to, which a transition
// entering `done` completes; nil is the machine's own body.
// entryOwner is the body a transition out of the entry action starts, as
// entryOwner names it.
func collectTransitions(graph *StateGraph, memberList []ast.Node, containingState, owner, entryOwner ast.Node, scope *symbols.Scope) error {

	for _, member := range memberList {
		actualMember := unwrapMembership(member)

		switch n := actualMember.(type) {
		case *ast.ErrorNode:
			// Skip error nodes
			continue
		case *ast.Usage:
			// Handle succession usages: succession statements parsed as Usage with Kind=UsageSuccession
			if n.Kind == ast.UsageSuccession {

				if len(n.ConnectorEnds) == 2 {
					// Connector-end targets name the source and target states.

					sourceQName, _ := connectorEndReference(n.ConnectorEnds[0]).(*ast.QualifiedName)
					targetQName, _ := connectorEndReference(n.ConnectorEnds[1]).(*ast.QualifiedName)

					if sourceQName != nil && targetQName != nil {
						sourceVertex := graph.endpointVertex(scope, sourceQName)
						targetVertex, _ := graph.targetVertex(scope, targetQName, owner)

						// `succession first begin then off;` out of a named entry action names the
						// state the machine starts in, not an edge (SysML 7.19.3).
						if sourceVertex == nil {
							starts, err := graph.startsAt(n, nil, memberList, containingState, owner, entryOwner, scope, sourceQName, targetQName)
							if err != nil {
								return err
							}
							if starts {
								continue
							}
						}

						if sourceVertex != nil && targetVertex != nil {
							trans := &Transition{
								Decl:      n,
								Source:    sourceVertex,
								Target:    targetVertex,
								Trigger:   nil, // Completion transition
								Guard:     nil,
								Effect:    nil,
								Scope:     scope,
								BodyScope: scope,
							}
							graph.Transitions[sourceVertex] = append(graph.Transitions[sourceVertex], trans)
						}
					}
				}
			} else if n.Kind == ast.UsageState {
				// A state usage carries the transitions its own body declares and
				// those the definition typing it declares, each lowered in the body
				// that wrote it and against this usage's own vertices.
				if err := collectStateTransitions(graph, n, owner); err != nil {
					return err
				}
			}
		case *ast.InitialNode:
			// Handle `initial X then Y` syntax:
			// This means "initial pseudostate transitions to state Y"
			// Mark Y as the initial state (no intermediate state for the initial node)
			if n.Successor != nil {
				targetState := graph.endpointState(scope, n.Successor)
				if targetState != nil {
					graph.addEntryTransition(entryOwner, &EntryTransition{Decl: n, Target: targetState, Scope: scope})
				}
			}
		case *ast.SuccessionEdge:
			// Handle succession statements: `source then target;`
			// Create completion transition from source to target
			sourceVertex := graph.endpointVertex(scope, n.Source)
			targetVertex, _ := graph.targetVertex(scope, n.Target, owner)

			// `entry; then off;` — a succession out of the body's own entry
			// subaction names the state it starts in (SysML 7.19.3), the same as
			// a named entry action with a succession out of it does.
			if sourceVertex == nil && isEntrySubaction(n.SourceMember) && targetVertex != nil {
				target, ok := targetVertex.(*ast.StateNode)
				if !ok {
					return &EntryTransitionTargetError{Target: targetVertex}
				}
				graph.addEntryTransition(entryOwner, &EntryTransition{Decl: n, Target: target, Scope: scope})
				continue
			}

			// `succession first start then off;` out of a named entry action says the same.
			if sourceVertex == nil {
				starts, err := graph.startsAt(n, nil, memberList, containingState, owner, entryOwner, scope, n.Source, n.Target)
				if err != nil {
					return err
				}
				if starts {
					continue
				}
			}

			if sourceVertex != nil && targetVertex != nil {
				trans := &Transition{
					Decl:      n,
					Source:    sourceVertex,
					Target:    targetVertex,
					Trigger:   nil, // Completion transition
					Guard:     nil,
					Effect:    nil,
					Scope:     scope,
					BodyScope: scope,
				}
				graph.Transitions[sourceVertex] = append(graph.Transitions[sourceVertex], trans)
			}
		case *ast.TransitionEdge:
			// Legacy: explicit TransitionEdge nodes (from hand-built tests)
			trans, err := lowerTransitionEdge(graph, n, owner, scope)
			if err != nil {
				return err
			}
			graph.Transitions[trans.Source] = append(graph.Transitions[trans.Source], trans)
		case *ast.TransitionMember:
			// `transition initial then off;` out of the entry action names the
			// state the machine starts in, not an edge between two vertices.
			if n.Trigger == nil && len(n.Effect) == 0 {
				starts, err := graph.startsAt(n, n.Guard, memberList, containingState, owner, entryOwner, scope, n.Source, n.Target)
				if err != nil {
					return err
				}
				if starts {
					continue
				}
			}
			// `entry; if c then off;` out of the entry subaction chooses the state the
			// body starts in by its guard.
			if n.Source == nil {
				if source := ast.ImplicitTransitionSource(memberList, n); source != nil && IsEntryTransition(source) {
					if err := lowerEntryTransition(graph, n, owner, entryOwner, scope); err != nil {
						return err
					}
					continue
				}
			}
			trans, err := lowerTransitionMember(graph, n, memberList, owner, scope)
			if err != nil {
				return err
			}
			// An endpoint naming no vertex was reported by name resolution.
			if trans == nil {
				continue
			}
			graph.Transitions[trans.Source] = append(graph.Transitions[trans.Source], trans)
		case *ast.StateNode:
			// Recurse into state substates to collect transitions within the state.
			stateScope := graph.StateScopes[n]
			if stateScope == nil {
				stateScope = graph.stateScope(scope, n)
			}
			if err := collectTransitions(graph, n.Substates, n,
				graph.completionOwner(n, owner), graph.entryOwner(n), stateScope); err != nil {
				return err
			}
			// The state's own regions carry successions of their own.
			for _, region := range n.Regions {
				if err := collectTransitions(graph, []ast.Node{region}, nil, owner, nil, stateScope); err != nil {
					return err
				}
			}
		case *ast.StateRegion:
			// Regions are orthogonal: a transition in one names its vertices from
			// the region's own scope.
			if err := collectTransitions(graph, n.States, nil, n, n, childScope(scope, n)); err != nil {
				return err
			}
		}
	}
	return nil
}
