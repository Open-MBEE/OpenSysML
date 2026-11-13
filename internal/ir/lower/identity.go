package lower

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Provenance maps a graph's ids back to the declarations its records were
// lowered from; index zero of every table is "none".
type Provenance struct {
	nodes   []ast.Node
	states  []*ast.StateNode
	regions []*ast.StateRegion
	pseudos []*ast.PseudostateNode
	trans   []*Transition
}

// Node is the declaration behind a node id, nil for none.
func (p *Provenance) Node(id program.NodeID) ast.Node {
	if id == 0 || int(id) >= len(p.nodes) {
		return nil
	}
	return p.nodes[id]
}

// State is the declaration behind a state id, nil for none.
func (p *Provenance) State(id program.StateID) *ast.StateNode {
	if id == 0 || int(id) >= len(p.states) {
		return nil
	}
	return p.states[id]
}

// Region is the declaration behind a region id, nil for none.
func (p *Provenance) Region(id program.RegionID) *ast.StateRegion {
	if id == 0 || int(id) >= len(p.regions) {
		return nil
	}
	return p.regions[id]
}

// Pseudostate is the declaration behind a pseudostate id, nil for none.
func (p *Provenance) Pseudostate(id program.PseudoID) *ast.PseudostateNode {
	if id == 0 || int(id) >= len(p.pseudos) {
		return nil
	}
	return p.pseudos[id]
}

// Transition is the record behind a transition id, nil for none.
func (p *Provenance) Transition(id program.TransID) *Transition {
	if id == 0 || int(id) >= len(p.trans) {
		return nil
	}
	return p.trans[id]
}

// tableIndex is the id of the record about to be appended to a table of n entries.
func tableIndex(n int) uint32 {
	return uint32(n) // #nosec G115 -- a graph has far fewer than 2^32 records
}

// originPaths keeps the paths handed out within one graph unique.
type originPaths map[string]bool

// unique returns base, or base with the first `[k]` that no earlier origin took.
func (p originPaths) unique(base string) string {
	path := base
	for k := 1; p[path]; k++ {
		path = base + "[" + strconv.Itoa(k) + "]"
	}
	p[path] = true
	return path
}

// originPath is a declaration's path: its qualified name, else its owner's
// qualified name with its name or, unnamed, its kind word (`A::b`, `A#if`).
func originPath(scope *symbols.Scope, sym *symbols.Symbol, name, kind string) string {
	if sym != nil {
		if fqn := symbols.FQNOf(sym); fqn != "" {
			return fqn
		}
	}
	var owner string
	if scope != nil {
		owner = symbols.FQNOf(scope.Owner())
	}
	if name != "" {
		if owner == "" {
			return name
		}
		return owner + "::" + name
	}
	return owner + "#" + kind
}

// originName is the name a message calls a node by, "" when it has none.
func originName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.InitialNode:
		return n.Name()
	case *ast.FinalNode:
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
	case *ast.StateNode:
		return n.Name
	case *ast.Usage:
		if name, _ := ast.EffectiveName(n); name != "" {
			return name
		}
		return n.Ident.ShortName
	case *ast.Definition:
		if n.Ident.Name != "" {
			return n.Ident.Name
		}
		return n.Ident.ShortName
	}
	return ""
}

// nodeKind is what a node of the graph does when a token reaches it.
func (g *ActionGraph) nodeKind(node ast.Node) program.NodeKind {
	if g.StatementRuns[node] {
		return program.NodeStatement
	}
	switch n := node.(type) {
	case *ast.InitialNode:
		return program.NodeInitial
	case *ast.FinalNode:
		return program.NodeFinal
	case *ast.ForkNode:
		return program.NodeFork
	case *ast.JoinNode:
		return program.NodeJoin
	case *ast.MergeNode:
		return program.NodeMerge
	case *ast.DecisionNode:
		return program.NodeDecision
	case *ast.ActionExecutionNode:
		return program.NodeExecution
	case *ast.PerformActionNode:
		return program.NodePerform
	case *ast.AssignmentActionNode:
		return program.NodeAssign
	case *ast.WhileLoopActionNode:
		return program.NodeLoop
	case *ast.IfActionNode:
		return program.NodeIf
	case *ast.Usage:
		if _, ok := g.Accepts[node]; ok || n.IsAccept {
			return program.NodeAccept
		}
		if n.IsTerminate {
			return program.NodeTerminate
		}
		return program.NodeExecution
	}
	return program.NodeStatement
}

// assignIDs gives every node of the graph its id, dense from 1 in the order of
// Nodes, and the origin and kind beside it; a graph lowered twice gets the same ids.
func (g *ActionGraph) assignIDs() {
	n := len(g.Nodes) + 1
	g.ID = make(map[ast.Node]program.NodeID, len(g.Nodes))
	g.Origins = make([]program.Origin, 1, n)
	g.Kinds = make([]program.NodeKind, 1, n)
	g.Provenance.nodes = make([]ast.Node, 1, n)
	paths := originPaths{}
	for _, node := range g.Nodes {
		if _, seen := g.ID[node]; seen {
			continue
		}
		id := program.NodeID(tableIndex(len(g.Provenance.nodes)))
		g.ID[node] = id
		g.Provenance.nodes = append(g.Provenance.nodes, node)
		kind := g.nodeKind(node)
		g.Kinds = append(g.Kinds, kind)
		g.Origins = append(g.Origins, g.nodeOrigin(node, kind, paths))
	}
}

func (g *ActionGraph) nodeOrigin(node ast.Node, kind program.NodeKind, paths originPaths) program.Origin {
	scope := g.DeclScope(node)
	var sym *symbols.Symbol
	if scope != nil {
		sym = scope.MemberDeclaring(node)
	}
	name := originName(node)
	return program.Origin{
		Doc:  symbols.DocNameOf(scope),
		Span: node.Span(),
		Path: paths.unique(originPath(scope, sym, name, kind.String())),
		Kind: fmt.Sprintf("%T", node),
		Name: name,
	}
}

// assignIDs gives every state, region, pseudostate and transition of the graph
// its id, dense from 1 in declaration order, and the origin beside it.
func (g *StateGraph) assignIDs() {
	paths := originPaths{}
	g.assignStateIDs(paths)
	g.assignRegionIDs(paths)
	g.assignPseudoIDs(paths)
	g.assignTransIDs(paths)
}

// statesInOrder lists the graph's states once each: States in order, then the
// graph-only states of a parallel body's usages, which parent chains and region tables reach.
func (g *StateGraph) statesInOrder() []*ast.StateNode {
	seen := make(map[*ast.StateNode]bool, len(g.States))
	out := make([]*ast.StateNode, 0, len(g.States))
	add := func(state *ast.StateNode) {
		if state != nil && !seen[state] {
			seen[state] = true
			out = append(out, state)
		}
	}
	for _, state := range g.States {
		add(state)
	}
	for _, region := range g.TopRegions {
		add(g.RegionState[region])
	}
	for i := 0; i < len(out); i++ {
		for parent := g.ParentState[out[i]]; parent != nil && !seen[parent]; parent = g.ParentState[parent] {
			add(parent)
		}
		for _, region := range g.CompositeStates[out[i]] {
			add(g.RegionState[region])
		}
	}
	return out
}

func (g *StateGraph) assignStateIDs(paths originPaths) {
	states := g.statesInOrder()
	n := len(states) + 1
	g.ID = make(map[*ast.StateNode]program.StateID, len(states))
	g.Origins = make([]program.Origin, 1, n)
	g.Provenance.states = make([]*ast.StateNode, 1, n)
	for _, state := range states {
		if _, seen := g.ID[state]; seen {
			continue
		}
		g.ID[state] = program.StateID(tableIndex(len(g.Provenance.states)))
		g.Provenance.states = append(g.Provenance.states, state)
		scope := g.StateScopes[state]
		var sym *symbols.Symbol
		if scope != nil {
			sym = scope.Owner()
		}
		if sym == nil {
			if declared := g.declaredIn[state]; declared != nil {
				scope, sym = declared, declared.MemberDeclaring(state)
			}
		}
		g.Origins = append(g.Origins, g.vertexOrigin(state, scope, sym, state.Name, paths))
	}
}

// regionsInOrder lists the graph's regions once each: the machine's own, then
// each state's in state order.
func (g *StateGraph) regionsInOrder() []*ast.StateRegion {
	seen := make(map[*ast.StateRegion]bool)
	var out []*ast.StateRegion
	add := func(regions []*ast.StateRegion) {
		for _, region := range regions {
			if !seen[region] {
				seen[region] = true
				out = append(out, region)
			}
		}
	}
	add(g.TopRegions)
	for _, state := range g.statesInOrder() {
		add(g.CompositeStates[state])
	}
	return out
}

func (g *StateGraph) assignRegionIDs(paths originPaths) {
	regions := g.regionsInOrder()
	g.RegionID = make(map[*ast.StateRegion]program.RegionID, len(regions))
	g.RegionOrigins = make([]program.Origin, 1, len(regions)+1)
	g.Provenance.regions = make([]*ast.StateRegion, 1, len(regions)+1)
	for _, region := range regions {
		g.RegionID[region] = program.RegionID(tableIndex(len(g.Provenance.regions)))
		g.Provenance.regions = append(g.Provenance.regions, region)
		scope := g.regionScopeOf[region]
		if scope == nil {
			scope = g.declaredIn[region]
		}
		g.RegionOrigins = append(g.RegionOrigins, program.Origin{
			Doc:  symbols.DocNameOf(scope),
			Span: region.Span(),
			Path: paths.unique(originPath(scope, nil, region.Name, "region")),
			Kind: "region",
			Name: region.Name,
		})
	}
}

func (g *StateGraph) assignPseudoIDs(paths originPaths) {
	n := len(g.Pseudostates) + 1
	g.PseudoID = make(map[*ast.PseudostateNode]program.PseudoID, len(g.Pseudostates))
	g.PseudoOrigins = make([]program.Origin, 1, n)
	g.Provenance.pseudos = make([]*ast.PseudostateNode, 1, n)
	for _, ps := range g.Pseudostates {
		if _, seen := g.PseudoID[ps]; seen {
			continue
		}
		g.PseudoID[ps] = program.PseudoID(tableIndex(len(g.Provenance.pseudos)))
		g.Provenance.pseudos = append(g.Provenance.pseudos, ps)
		scope := g.declaredIn[ps]
		var sym *symbols.Symbol
		if scope != nil {
			sym = scope.MemberDeclaring(ps)
		}
		g.PseudoOrigins = append(g.PseudoOrigins, g.vertexOrigin(ps, scope, sym, ps.Name, paths))
	}
}

func (g *StateGraph) vertexOrigin(decl ast.Node, scope *symbols.Scope, sym *symbols.Symbol, name string, paths originPaths) program.Origin {
	kind := VertexKind(decl)
	return program.Origin{
		Doc:  g.DocOf(decl),
		Span: decl.Span(),
		Path: paths.unique(originPath(scope, sym, name, kind)),
		Kind: kind,
		Name: name,
	}
}

// transitionsInOrder lists the graph's transitions once each: those leaving each
// state in state order, then each pseudostate's, then any other source's by position.
func (g *StateGraph) transitionsInOrder() []*Transition {
	var out []*Transition
	seen := make(map[ast.Node]bool)
	add := func(source ast.Node) {
		if seen[source] {
			return
		}
		seen[source] = true
		out = append(out, g.Transitions[source]...)
	}
	for _, state := range g.States {
		add(state)
	}
	for _, ps := range g.Pseudostates {
		add(ps)
	}
	var rest []ast.Node
	for source := range g.Transitions {
		if !seen[source] {
			rest = append(rest, source)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i].Span().Offset < rest[j].Span().Offset })
	for _, source := range rest {
		add(source)
	}
	return out
}

func (g *StateGraph) assignTransIDs(paths originPaths) {
	trans := g.transitionsInOrder()
	g.TransID = make(map[*Transition]program.TransID, len(trans))
	g.TransOrigins = make([]program.Origin, 1, len(trans)+1)
	g.Provenance.trans = make([]*Transition, 1, len(trans)+1)
	for _, t := range trans {
		if _, seen := g.TransID[t]; seen {
			continue
		}
		g.TransID[t] = program.TransID(tableIndex(len(g.Provenance.trans)))
		g.Provenance.trans = append(g.Provenance.trans, t)
		var span source.Span
		var sym *symbols.Symbol
		if t.Decl != nil {
			span = t.Decl.Span()
			if t.Scope != nil {
				sym = t.Scope.MemberDeclaring(t.Decl)
			}
		} else if t.Source != nil {
			span = t.Source.Span()
		}
		g.TransOrigins = append(g.TransOrigins, program.Origin{
			Doc:  g.DocOf(t.Decl),
			Span: span,
			Path: paths.unique(originPath(t.Scope, sym, t.Name, "transition")),
			Kind: "transition",
			Name: t.Name,
		})
	}
}
