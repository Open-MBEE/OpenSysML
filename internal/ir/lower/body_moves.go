package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A leaf body's performance encloses its start shot, where its initial values and
// inputs are read, and one assignment or other statement performance per statement.
// Another performance may interleave between two of them; BodyDivides says where
// that may change an outcome (docs/internals/design/scheduling.md).

// ReadsAtStart reports whether a performance of node, beginning, evaluates initial
// values at its start shot before its body runs.
func ReadsAtStart(graph *ActionGraph, node ast.Node) bool {
	if len(graph.Bodies[node]) == 0 {
		return false
	}
	for _, feature := range graph.Features[node] {
		if feature.Value != nil {
			return true
		}
	}
	return false
}

// BodyDivides reports whether another performance interleaving inside node's
// performance may change an outcome: two or more of the body's moves depend on a
// move another performance may make concurrently. With one such move at most, every
// interleaving reorders only independent moves, so the body runs as one move.
func BodyDivides(graph *ActionGraph, node ast.Node) bool {
	moves := bodyMoves(graph, node)
	if len(moves) < 2 {
		return false
	}
	root := graph
	for root.Enclosing != nil {
		root = root.Enclosing
	}
	if !runsConcurrently(root) {
		return false
	}
	others := concurrentFootprints(root, graph, node)
	dependent := 0
	for _, move := range moves {
		for _, other := range others {
			if move.footprint.Dependent(other) {
				dependent += move.times
				break
			}
		}
		if dependent > 1 {
			return true
		}
	}
	return false
}

// BodySharesMoves reports whether two or more of node's moves may touch what another
// performance does: alongside moves outside its flow, such a body may be divided by one.
func BodySharesMoves(graph *ActionGraph, node ast.Node) bool {
	shared := 0
	for _, move := range bodyMoves(graph, node) {
		if touchesShared(move.footprint) {
			shared += move.times
		}
		if shared > 1 {
			return true
		}
	}
	return false
}

// FlowSharesMoves reports whether two or more moves of a performance of graph's flow,
// its start shot and its subflows' moves included, may touch what another performance does;
// the features the performance holds its own values of are not shared.
func FlowSharesMoves(graph *ActionGraph) bool {
	own := ownFeatures(graph)
	touches := func(f Footprint) bool { return touchesShared(withoutPlaces(f, own)) }
	shared := 0
	b := &footprintBuilder{graph: graph, scope: graph.Scope, declared: declaredFeatures(graph)}
	for _, attr := range graph.Attributes {
		scope := attr.Scope
		if scope == nil {
			scope = graph.Scope
		}
		b.reads(scope, attr.Value)
	}
	if touches(b.footprint) {
		shared++
	}
	var walk func(g *ActionGraph) bool
	walk = func(g *ActionGraph) bool {
		for _, n := range g.Nodes {
			if touches(g.Footprints()[n]) {
				shared++
				if g.Multiplicities[n] != nil {
					shared++
				}
			}
			if shared > 1 {
				return true
			}
			if sub := g.Subflows[n]; sub != nil && sub.Graph != nil && walk(sub.Graph) {
				return true
			}
		}
		return false
	}
	return walk(graph)
}

// ownFeatures are the symbols of the parameters and attributes graph's action declares.
func ownFeatures(graph *ActionGraph) map[*symbols.Symbol]bool {
	own := make(map[*symbols.Symbol]bool)
	for _, attr := range graph.Attributes {
		if sym := featureSymbol(graph.Scope, Feature{Name: attr.Name, Node: attr.Node}); sym != nil {
			own[sym] = true
		}
	}
	return own
}

// withoutPlaces drops from f the places resolving to one of syms.
func withoutPlaces(f Footprint, syms map[*symbols.Symbol]bool) Footprint {
	keep := func(places []Place) []Place {
		var out []Place
		for _, p := range places {
			if p.Sym == nil || !syms[p.Sym] {
				out = append(out, p)
			}
		}
		return out
	}
	f.Reads, f.Writes = keep(f.Reads), keep(f.Writes)
	return f
}

// touchesShared reports whether a move may touch what another performance does: a
// feature it holds no pin of, the bus, or a target unresolved.
func touchesShared(f Footprint) bool {
	s := sharedOnly(f)
	return f.Dynamic || f.messages() || len(s.Reads)+len(s.Writes) > 0
}

type bodyMove struct {
	footprint Footprint
	// times is how many moves it stands for: two for a loop, which may iterate.
	times int
}

// bodyMoves lists the moves a performance of node makes: its start shot, where it
// evaluates initial values, then each statement, a block's or conditional's by its
// own. The start shot's footprint leaves out the pins it seeds that no flow reads.
func bodyMoves(graph *ActionGraph, node ast.Node) []bodyMove {
	declared := declaredFeatures(graph)
	builder := func() *footprintBuilder {
		return &footprintBuilder{graph: graph, node: node, scope: nodeScopeOf(graph, node), declared: declared}
	}
	var moves []bodyMove
	if ReadsAtStart(graph, node) {
		b := builder()
		for _, f := range graph.Features[node] {
			b.reads(f.Scope, f.Value)
			if f.Direction == ast.DirOut || f.Direction == ast.DirInOut || f.IsResult {
				b.write(Place{Sym: featureSymbol(b.scope, f), Name: f.Name, Local: true})
			}
		}
		moves = append(moves, bodyMove{footprint: b.footprint, times: 1})
	}
	var walk func(stmts []Statement)
	walk = func(stmts []Statement) {
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case Block:
				if s.Graph == nil {
					walk(s.Statements)
					continue
				}
			case If:
				b := builder()
				b.reads(s.Scope, s.Condition)
				moves = append(moves, bodyMove{footprint: b.footprint, times: 1})
				walk(s.Then.Statements)
				if s.Else != nil {
					walk(s.Else.Statements)
				}
				continue
			}
			b := builder()
			b.statement(stmt)
			times := 1
			if _, loops := stmt.(Loop); loops {
				times = 2
			}
			moves = append(moves, bodyMove{footprint: b.footprint, times: times})
		}
	}
	walk(graph.Bodies[node])
	return moves
}

// concurrentFootprints lists the footprints of every move of the flow node is in
// that may run concurrently with node's: those of every node of the outermost flow
// and of each flow nested in it, node's own only where it may be performed
// concurrently with itself, held pins aside.
func concurrentFootprints(root, graph *ActionGraph, node ast.Node) []Footprint {
	var out []Footprint
	var walk func(g *ActionGraph)
	walk = func(g *ActionGraph) {
		for _, n := range g.Nodes {
			footprint := g.Footprints()[n]
			if g == graph && n == node {
				if !selfConcurrent(graph, node) {
					continue
				}
				footprint = sharedOnly(footprint)
			}
			out = append(out, footprint)
			if sub := g.Subflows[n]; sub != nil && sub.Graph != nil {
				walk(sub.Graph)
			}
		}
	}
	walk(root)
	return out
}

// selfConcurrent reports whether two performances of node may overlap: it declares
// a multiplicity, or a succession path leads from it back to it.
func selfConcurrent(graph *ActionGraph, node ast.Node) bool {
	if graph.Multiplicities[node] != nil {
		return true
	}
	seen := map[ast.Node]bool{}
	stack := []ast.Node{node}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, edge := range graph.Edges[n] {
			if edge.Target == node {
				return true
			}
			if !seen[edge.Target] {
				seen[edge.Target] = true
				stack = append(stack, edge.Target)
			}
		}
	}
	return false
}

// sharedOnly drops from a footprint the pins each performance holds its own of.
func sharedOnly(f Footprint) Footprint {
	keep := func(places []Place) []Place {
		var out []Place
		for _, p := range places {
			if !p.Local {
				out = append(out, p)
			}
		}
		return out
	}
	f.Reads, f.Writes = keep(f.Reads), keep(f.Writes)
	return f
}

// runsConcurrently reports whether any flow under root may hold two tokens at once:
// it forks, starts subactions together, repeats a step or leaves a node by two
// successions other than a decision's.
func runsConcurrently(root *ActionGraph) bool {
	if len(root.Concurrent) > 0 {
		return true
	}
	for _, m := range root.Multiplicities {
		if m != nil {
			return true
		}
	}
	for _, n := range root.Nodes {
		switch n.(type) {
		case *ast.ForkNode:
			return true
		case *ast.DecisionNode:
		default:
			if len(root.Edges[n]) > 1 {
				return true
			}
		}
		if sub := root.Subflows[n]; sub != nil && sub.Graph != nil && runsConcurrently(sub.Graph) {
			return true
		}
	}
	return false
}
