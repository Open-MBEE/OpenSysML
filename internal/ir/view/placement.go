package view

import "fmt"

// placement is what a positioned rendering has a place for: positioned nodes,
// their clusters, routed nodes and pictures; every graph-shaped form draws by it.
type placement struct {
	placed   map[string]bool // node ID -> has a place
	extent   map[string]bool // node ID -> its place has an extent, not a corner alone
	derived  map[string]bool // node ID -> placed beside the placed nodes its edges reach
	nodes    int             // nodes in the rendering, and how many have a place
	count    int
	pictures int     // pictures drawn, each at stated bounds
	ends     []*Node // the unplaced start and final nodes, candidates for a derived place
}

// placeRendering classifies every node of r. A rendering no Layout or Route
// positions places nothing, and every form draws all of it. A start or final
// node nothing positions takes its place beside the placed nodes its edges reach.
func placeRendering(r *Rendering) *placement {
	p := &placement{placed: map[string]bool{}, extent: map[string]bool{}, derived: map[string]bool{}, pictures: len(r.Pictures)}
	routed := map[string]bool{}
	for _, edge := range r.Edges {
		if len(edge.Route) > 1 {
			routed[edge.From], routed[edge.To] = true, true
		}
	}
	for _, root := range r.Roots {
		p.place(root, r.Kind == KindTree, routed)
	}
	if p.count > 0 {
		p.deriveEnds(r.Edges)
	}
	return p
}

// deriveEnds places every unplaced start or final node whose edges all reach a
// node placed in its own right.
func (p *placement) deriveEnds(edges []Edge) {
	for _, node := range p.ends {
		found := false
		for _, edge := range edges {
			other, ok := edgeOtherEnd(edge, node.ID)
			if !ok {
				continue
			}
			if !p.placed[other] || p.derived[other] {
				found = false
				break
			}
			found = true
		}
		if found {
			p.placed[node.ID], p.extent[node.ID], p.derived[node.ID] = true, true, true
			p.count++
		}
	}
}

// edgeOtherEnd is the node at the other end of edge from id, if id is an end.
func edgeOtherEnd(edge Edge, id string) (string, bool) {
	switch id {
	case edge.From:
		return edge.To, true
	case edge.To:
		return edge.From, true
	}
	return "", false
}

// endKind reports whether a node is where a flow begins or ends — a start,
// initial, final or terminate pseudostate — drawn beside its neighbour when
// nothing positions it.
func endKind(kind string) bool {
	switch kind {
	case startKind, "initial", "final", terminateKind:
		return true
	}
	return false
}

// place classifies node and the nodes under it, members first so a cluster
// can take its place from them. A cluster a Layout gives a corner but no size
// has a place without an extent while no member has one.
func (p *placement) place(node *Node, tree bool, routed map[string]bool) {
	p.nodes++
	members := false
	for _, child := range node.Children {
		p.place(child, tree, routed)
		members = members || p.extent[child.ID]
	}
	cluster := len(node.Children) > 0 && !tree
	switch {
	case node.Geometry.placed():
		p.extent[node.ID] = !cluster || node.Geometry.HasSize || members
	case cluster && members, routed[node.ID]:
		p.extent[node.ID] = true
	default:
		if endKind(node.Kind) && !cluster {
			p.ends = append(p.ends, node)
		}
		return
	}
	p.placed[node.ID] = true
	p.count++
}

// positioned reports whether anything in the rendering has a stated place, a
// node or a picture, so the drawing is pinned where the diagram states.
func (p *placement) positioned() bool { return p.count > 0 || p.pictures > 0 }

// partial reports whether the rendering places some nodes and not others: the
// case a form settles as its Options.Unplaced asks. Pictures place no node.
func (p *placement) partial() bool { return p.count > 0 && p.count < p.nodes }

// picturedOnly reports whether pictures alone pin the drawing while none of
// its nodes has a place, so the nodes are drawn in a strip below the pictures.
func (p *placement) picturedOnly() bool { return p.count == 0 && p.pictures > 0 && p.nodes > 0 }

// unplaced is how many nodes have no place.
func (p *placement) unplaced() int { return p.nodes - p.count }

// omit is r without the nodes it leaves unplaced and the edges at them, with a
// notice of what was left undrawn. In a tree an omitted node's placed members
// become roots of their own, detached from the node above; a cluster round a
// placed member has a place itself, so a clustered kind omits whole subtrees.
func (p *placement) omit(r *Rendering) *Rendering {
	out := *r
	var hoisted []*Node
	out.Roots = append(p.keep(r.Roots, &hoisted), hoisted...)
	kept, dropped := p.keptEdges(r.Edges)
	out.Edges = kept
	out.Notices = append(append([]string(nil), r.Notices...), p.omitNotice(dropped))
	return &out
}

// foldCompartments is a positioned tree showing compartments with every
// unplaced member of a placed node folded into a compartment row of it, the
// edges and notes at those members dropped and noticed: what every form
// draws of such a tree before it settles the nodes still unplaced. Any other
// rendering is returned as it is.
func foldCompartments(r *Rendering) *Rendering {
	if r.Kind != KindTree || !r.Shown.Has(ElementCompartments) {
		return r
	}
	p := placeRendering(r)
	if !p.partial() {
		return r
	}
	out := *r
	out.Roots = cloneNodes(r.Roots)
	rows := map[string]bool{}
	folded := p.foldNodes(out.Roots, rows)
	if folded.drawn+folded.hidden == 0 {
		return r
	}
	notice := fmt.Sprintf("%d node(s) without a position, drawn as compartment rows of the node above them", folded.drawn)
	if folded.hidden > 0 {
		notice = fmt.Sprintf("%d node(s) without a position: %d drawn as compartment rows of the node above them, %d in compartments its Layout hides left undrawn", folded.drawn+folded.hidden, folded.drawn, folded.hidden)
	}
	if folded.nested > 0 {
		notice += fmt.Sprintf(", and %d node(s) under them left undrawn", folded.nested)
	}
	out.Edges = make([]Edge, 0, len(r.Edges))
	for _, edge := range r.Edges {
		if rows[edge.From] || rows[edge.To] {
			continue
		}
		out.Edges = append(out.Edges, edge)
	}
	if dropped := len(r.Edges) - len(out.Edges); dropped > 0 {
		notice += fmt.Sprintf(", and %d edge(s) at them", dropped)
	}
	out.Notes = make([]Note, 0, len(r.Notes))
	for _, note := range r.Notes {
		if rows[note.Anchor] || rows[note.EdgeFrom] || rows[note.EdgeTo] {
			continue
		}
		out.Notes = append(out.Notes, note)
	}
	if dropped := len(r.Notes) - len(out.Notes); dropped > 0 {
		notice += fmt.Sprintf(", and %d note(s) at them", dropped)
	}
	out.Notices = append(append([]string(nil), r.Notices...), notice)
	return &out
}

// folding counts what folding drew as compartment rows, what it left undrawn
// because the node's Layout hides the row's compartment, and the nodes under
// the rows, which a row does not show.
type folding struct {
	drawn, hidden, nested int
}

// foldNodes folds the unplaced members of each placed node under nodes into
// its compartments, in place, less the rows of the compartments the node's
// Layout hides, recording the IDs of the folded members and of every node
// under them in rows.
func (p *placement) foldNodes(nodes []*Node, rows map[string]bool) folding {
	var f folding
	for _, node := range nodes {
		if p.placed[node.ID] {
			var folded, listed []*Node
			node.Children, folded = p.rowsOf(node)
			for _, row := range folded {
				before := len(rows)
				recordIDs(row, rows)
				f.nested += len(rows) - before - 1
				if hiddenRow(node, row) {
					f.hidden++
					continue
				}
				f.drawn++
				listed = append(listed, row)
			}
			node.Compartments = compartmentsOf(listed)
		}
		sub := p.foldNodes(node.Children, rows)
		f.drawn, f.hidden, f.nested = f.drawn+sub.drawn, f.hidden+sub.hidden, f.nested+sub.nested
	}
	return f
}

// recordIDs records the ID of node and of every node under it in ids.
func recordIDs(node *Node, ids map[string]bool) {
	ids[node.ID] = true
	for _, child := range node.Children {
		recordIDs(child, ids)
	}
}

// rowsOf splits a placed tree node's members into the ones kept as nodes and
// the ones drawn as compartment rows of it: an unplaced member with no placed
// node anywhere under it, which a stand-in never is.
func (p *placement) rowsOf(node *Node) (kept, rows []*Node) {
	for _, child := range node.Children {
		if child.StandIn || p.anyPlaced(child) {
			kept = append(kept, child)
			continue
		}
		rows = append(rows, child)
	}
	return kept, rows
}

// anyPlaced reports whether node or a node under it has a place.
func (p *placement) anyPlaced(node *Node) bool {
	if p.placed[node.ID] {
		return true
	}
	for _, child := range node.Children {
		if p.anyPlaced(child) {
			return true
		}
	}
	return false
}

// keptEdges is edges without those at an unplaced node, and how many those were.
func (p *placement) keptEdges(edges []Edge) (kept []Edge, dropped int) {
	kept = make([]Edge, 0, len(edges))
	for _, edge := range edges {
		if p.placed[edge.From] && p.placed[edge.To] {
			kept = append(kept, edge)
		}
	}
	return kept, len(edges) - len(kept)
}

// omitNotice accounts for the unplaced nodes left undrawn, and the edges at them.
func (p *placement) omitNotice(dropped int) string {
	notice := fmt.Sprintf("%d node(s) without a position, left undrawn", p.unplaced())
	if dropped > 0 {
		notice += fmt.Sprintf(", and %d edge(s) at them", dropped)
	}
	return notice
}

// keep is nodes without the unplaced ones, collecting the kept members of an
// omitted node in hoisted.
func (p *placement) keep(nodes []*Node, hoisted *[]*Node) []*Node {
	var kept []*Node
	for _, node := range nodes {
		members := p.keep(node.Children, hoisted)
		if !p.placed[node.ID] {
			*hoisted = append(*hoisted, members...)
			continue
		}
		copied := *node
		copied.Children = members
		kept = append(kept, &copied)
	}
	return kept
}

// settleUnplaced is what a form that lays nodes out itself draws of a
// positioned rendering: the placed nodes alone by default — the stand-ins a
// migration made up elided first, as DOT elides them — every node under
// UnplacedStrip, each noticed. A rendering placing all or none is drawn whole.
func (r *Rendering) settleUnplaced(unplaced Unplaced, form Form) *Rendering {
	if unplaced != UnplacedStrip {
		r = foldCompartments(withoutStandIns(r))
	}
	p := placeRendering(r)
	if !p.partial() {
		return r
	}
	if unplaced != UnplacedStrip {
		return p.omit(r)
	}
	out := *r
	out.Notices = append(append([]string(nil), r.Notices...),
		fmt.Sprintf("%d node(s) without a position, drawn among the placed ones; the %s form lays every node out itself", p.unplaced(), form))
	return &out
}
