package view

import "fmt"

// placement is what a positioned rendering has a place for: the nodes a Layout
// positions, the clusters round a placed member, and the nodes a route meets.
// Every graph-shaped form draws by it, so a drawing shows the same nodes and
// edges whichever form writes it.
type placement struct {
	placed map[string]bool // node ID -> has a place
	extent map[string]bool // node ID -> its place has an extent, not a corner alone
	nodes  int             // nodes in the rendering, and how many have a place
	count  int
}

// placeRendering classifies every node of r. A rendering no Layout or Route
// positions places nothing, and every form draws all of it.
func placeRendering(r *Rendering) *placement {
	p := &placement{placed: map[string]bool{}, extent: map[string]bool{}}
	routed := map[string]bool{}
	for _, edge := range r.Edges {
		if len(edge.Route) > 1 {
			routed[edge.From], routed[edge.To] = true, true
		}
	}
	for _, root := range r.Roots {
		p.place(root, r.Kind == KindTree, routed)
	}
	return p
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
	case node.Geometry != nil:
		p.extent[node.ID] = !cluster || node.Geometry.HasSize || members
	case cluster && members, routed[node.ID]:
		p.extent[node.ID] = true
	default:
		return
	}
	p.placed[node.ID] = true
	p.count++
}

// partial reports whether the rendering places some nodes and not others: the
// case a form settles as its Options.Unplaced asks.
func (p *placement) partial() bool { return p.count > 0 && p.count < p.nodes }

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
// positioned rendering: the placed nodes alone by default, every node under
// UnplacedStrip, each noticed. A rendering placing all or none is drawn whole.
func (r *Rendering) settleUnplaced(unplaced Unplaced, form Form) *Rendering {
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
