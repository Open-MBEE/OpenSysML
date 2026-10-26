package view

import (
	"fmt"
	"strings"
)

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
	// A Route about a usage steers the edge its owner draws to the usage's
	// definitions once the usage folds into a row: it places those ends, not
	// the usage's node, which a Layout alone places.
	nesting := r.nestingEdges()
	typed := map[string][]string{}
	for _, edge := range r.Edges {
		if edge.Kind == EdgeTyping {
			typed[edge.From] = append(typed[edge.From], edge.To)
		}
	}
	routed := map[string]bool{}
	for _, edge := range r.Edges {
		switch {
		case len(edge.Route) <= 1:
		case nesting[[2]string{edge.From, edge.To}]:
			for _, to := range typed[edge.To] {
				routed[edge.From], routed[to] = true, true
			}
		default:
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
	kept, dropped := p.keptEdges(p.foldOmitted(r.Roots, r.Edges, r.nestingEdges()))
	out.Edges = kept
	out.Notices = append(append([]string(nil), r.Notices...), p.omitNotice(dropped))
	return &out
}

// foldCompartments is a tree showing compartments with members folded into
// compartment rows of the node above them: under CompartmentsPositioned the
// unplaced members of a placed node, with the edges and notes at them dropped;
// under an automatic folding the members the mode names, with the edges and
// notes at them moved to the owner. Any other rendering is returned as it is.
func foldCompartments(r *Rendering, mode Compartments) *Rendering {
	if r.Kind != KindTree || !r.Shown.Has(ElementCompartments) {
		return r
	}
	p := placeRendering(r)
	if !mode.automatic() && !p.partial() {
		return r
	}
	f := &folder{placement: p, mode: mode, owners: map[string]string{}, rows: map[string]*Node{}, nesting: r.nestingEdges()}
	out := *r
	out.Roots = cloneNodes(r.Roots)
	f.foldNodes(out.Roots)
	if f.drawn+f.hidden == 0 {
		return r
	}
	notice := f.notice()
	var moved, dropped int
	out.Edges, moved, dropped = f.foldEdges(r.Edges)
	if mode.automatic() {
		if moved > 0 {
			notice += fmt.Sprintf(", and %d edge(s) at them moved to their owner", moved)
		}
		if dropped > 0 {
			notice += fmt.Sprintf(", and %d edge(s) within one owner dropped", dropped)
		}
		out.Notes, moved = f.reanchorNotes(r.Notes)
		if moved > 0 {
			notice += fmt.Sprintf(", and %d note(s) at them moved to their owner", moved)
		}
	} else {
		if dropped > 0 {
			notice += fmt.Sprintf(", and %d edge(s) at them", dropped)
		}
		out.Notes = make([]Note, 0, len(r.Notes))
		for _, note := range r.Notes {
			if f.folded(note.Anchor) || f.folded(note.EdgeFrom) || f.folded(note.EdgeTo) {
				continue
			}
			out.Notes = append(out.Notes, note)
		}
		if dropped := len(r.Notes) - len(out.Notes); dropped > 0 {
			notice += fmt.Sprintf(", and %d note(s) at them", dropped)
		}
	}
	out.Notices = append(append([]string(nil), r.Notices...), notice)
	return &out
}

// folder folds the members of a tree into compartment rows, counting the rows
// drawn, the rows a Layout hides and the nodes under the rows left undrawn.
type folder struct {
	*placement
	mode                  Compartments
	owners                map[string]string  // folded node ID -> the ID of the node whose row it is
	rows                  map[string]*Node   // folded node ID -> the node, a row or one left undrawn under a row
	nesting               map[[2]string]bool // the membership edges drawn from a node to one nested in it
	drawn, hidden, nested int
}

func (f *folder) folded(id string) bool { _, ok := f.owners[id]; return ok }

// notice words what the folding did, by whether the members folded for want of a
// position or because the mode names them, and counts the nodes left undrawn under the rows.
func (f *folder) notice() string {
	if f.mode.automatic() {
		notice := fmt.Sprintf("%d member(s) drawn as compartment rows of their owner", f.drawn)
		if f.hidden > 0 {
			notice = fmt.Sprintf("%d member(s) folded into compartments of their owner: %d drawn as rows, %d in compartments its Layout hides left undrawn", f.drawn+f.hidden, f.drawn, f.hidden)
		}
		if f.nested > 0 {
			notice += fmt.Sprintf(", and %d node(s) under them left undrawn", f.nested)
		}
		return notice
	}
	notice := fmt.Sprintf("%d node(s) without a position, drawn as compartment rows of the node above them", f.drawn)
	if f.hidden > 0 {
		notice = fmt.Sprintf("%d node(s) without a position: %d drawn as compartment rows of the node above them, %d in compartments its Layout hides left undrawn", f.drawn+f.hidden, f.drawn, f.hidden)
	}
	if f.nested > 0 {
		notice += fmt.Sprintf(", and %d node(s) under them left undrawn", f.nested)
	}
	return notice
}

// foldNodes folds the members of every owner among nodes and under them,
// rewriting each owner's Children and Compartments in place.
func (f *folder) foldNodes(nodes []*Node) {
	for _, node := range nodes {
		if f.owns(node) {
			var folded, listed []*Node
			node.Children, folded = f.rowsOf(node)
			for _, row := range folded {
				f.record(row, node.ID)
				if hiddenRow(node, row) {
					f.hidden++
					f.nested += f.undrawnUnder(row, nil)
					continue
				}
				rows := f.rowsUnder(row)
				f.drawn += len(rows)
				drawn := make(map[string]bool, len(rows))
				for _, sub := range rows[1:] {
					drawn[sub.ID] = true
				}
				f.nested += f.undrawnUnder(row, drawn)
				listed = append(listed, rows...)
			}
			node.Compartments = compartmentsOf(listed)
		}
		f.foldNodes(node.Children)
	}
}

// undrawnUnder counts the nodes under a row that no row among drawn shows. An automatic
// folding leaves out what the row has from its type: that is the type's own, not left undrawn.
func (f *folder) undrawnUnder(row *Node, drawn map[string]bool) int {
	n := 0
	for _, child := range row.Children {
		if f.mode.automatic() && child.FromType {
			continue
		}
		if !drawn[child.ID] {
			n++
		}
		n += f.undrawnUnder(child, drawn)
	}
	return n
}

// owns reports a node whose members may fold into its compartments: a placed
// one under the positioned folding, any type under an automatic one. A view
// node's children are what it exposes, not members, so they never fold.
func (f *folder) owns(node *Node) bool {
	if !f.mode.automatic() {
		return f.placed[node.ID]
	}
	return !packageNode(node) && !viewNode(node) && !f.folded(node.ID)
}

func (f *folder) record(node *Node, owner string) {
	f.owners[node.ID] = owner
	f.rows[node.ID] = node
	for _, child := range node.Children {
		f.record(child, owner)
	}
}

// rowsOf splits a node's children into those kept as nodes and those folded
// into its compartment rows, by the folding's predicate.
func (f *folder) rowsOf(node *Node) (kept, rows []*Node) {
	for _, child := range node.Children {
		if f.row(child) {
			rows = append(rows, child)
			continue
		}
		kept = append(kept, child)
	}
	return kept, rows
}

// row is the fold predicate: a stand-in or a member with a place of its own
// or under it stays a node under every folding; the positioned folding folds
// every other member, the automatic ones a usage (COMPMOST) or a port
// (COMPTREE) with no definition or package under it.
func (f *folder) row(child *Node) bool {
	if child.StandIn || f.anyPlaced(child) {
		return false
	}
	switch f.mode {
	case CompartmentsMost:
		return usageNode(child) && !f.anyStructure(child)
	case CompartmentsPorts:
		return portNode(child) && !f.anyStructure(child)
	}
	return true
}

// anyStructure reports a definition or package node under node.
func (f *folder) anyStructure(node *Node) bool {
	for _, child := range node.Children {
		if !usageNode(child) || f.anyStructure(child) {
			return true
		}
	}
	return false
}

// rowsUnder is a folded member's rows: itself, then under an automatic folding
// every port nested under a port, named by its path from the row as the
// pilot's compartment tree lists it.
func (f *folder) rowsUnder(row *Node) []*Node {
	rows := []*Node{row}
	if !f.mode.automatic() {
		return rows
	}
	typeRow(row)
	if !portNode(row) {
		return rows
	}
	var walk func(node *Node, path string)
	walk = func(node *Node, path string) {
		for _, child := range node.Children {
			if !portNode(child) || child.StandIn {
				continue
			}
			sub := *child
			sub.Name, sub.Children, sub.NameSynthesized = path+"."+child.Name, nil, false
			typeRow(&sub)
			rows = append(rows, &sub)
			walk(child, sub.Name)
		}
	}
	walk(row, row.Name)
	return rows
}

// typeRow restores the typing a row's declaration line reads with, which the
// tree's boxes leave to their typing edges unless typings are shown.
func typeRow(row *Node) {
	if row.Type == "" && len(row.Typings) > 0 {
		row.Type = strings.Join(row.Typings, ", ")
	}
}

// foldMemberships folds a folded member's membership edge and its typing
// edges into one edge from the member's owner to each drawn type, the
// composition or reference labelled with the member's name and multiplicity —
// the property association a block definition diagram draws for a member
// listed in a compartment — in the membership edge's place. A member nested
// in another folded one folds to the owner under an automatic folding, as its
// other edges move there, counted in moved; under the positioned folding its
// nesting folds away, the nesting of an undrawn node, and its other edges are
// dropped with the rest at one. A nesting with no drawn type folds away, as
// does one drawn with a glyph of its own (a portion, an event), whose row
// names the member's type.
func (f *folder) foldMemberships(edges []Edge) (folded []Edge, moved int) {
	typings := map[string][]int{}
	for i, edge := range edges {
		if edge.Kind == EdgeTyping && f.folded(edge.From) {
			typings[edge.From] = append(typings[edge.From], i)
		}
	}
	folded = make([]Edge, 0, len(edges))
	consumed := map[int]bool{}
	for i, edge := range edges {
		if consumed[i] {
			continue
		}
		row, ok := f.rows[edge.To]
		if !ok || !f.nesting[[2]string{edge.From, edge.To}] {
			folded = append(folded, edge)
			continue
		}
		if f.folded(edge.From) && !f.mode.automatic() {
			continue
		}
		if edge.Kind != EdgeComposition && edge.Kind != EdgeReference {
			// A portion or event glyph names the usage, not its type: the
			// row's `name : Type` stands for the folded member.
			for _, j := range typings[edge.To] {
				consumed[j] = true
			}
			continue
		}
		for _, j := range typings[edge.To] {
			consumed[j] = true
			moved += boolToInt(f.folded(edge.From))
			route := edge.Route
			if f.folded(edge.From) {
				route = nil
			}
			folded = append(folded, Edge{From: f.owners[edge.To], To: edges[j].To, Kind: edge.Kind, ToLabel: shown(row) + edge.ToLabel,
				Origin: edge.Origin, Route: route, Style: edge.Style})
		}
	}
	return folded, moved
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// foldEdges is the edges of the folded tree: the memberships folded
// (foldMemberships), then under the automatic foldings every other edge end
// at a folded member moved to the member's owner, dropping the edges that
// then join an owner to itself and the duplicates, and under the positioned
// folding every other edge at one dropped.
func (f *folder) foldEdges(edges []Edge) (kept []Edge, moved, dropped int) {
	edges, moved = f.foldMemberships(edges)
	kept = make([]Edge, 0, len(edges))
	seen := map[string]bool{}
	for _, edge := range edges {
		fromOwner, fromRow := f.owners[edge.From]
		toOwner, toRow := f.owners[edge.To]
		if !fromRow && !toRow {
			kept = append(kept, edge)
			continue
		}
		if !f.mode.automatic() {
			dropped++
			continue
		}
		if fromRow {
			edge.From, edge.FromPort = fromOwner, ""
		}
		if toRow {
			edge.To, edge.ToPort = toOwner, ""
		}
		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s", edge.From, edge.To, edge.Kind, edge.Label, edge.FromLabel, edge.ToLabel, edge.Name)
		if edge.From == edge.To || seen[key] {
			dropped++
			continue
		}
		seen[key] = true
		edge.Route = nil
		kept = append(kept, edge)
		moved++
	}
	return kept, moved, dropped
}

// reanchorNotes moves every note at a folded member to the member's owner; a
// note on an edge that folding dropped inside one owner becomes a note on it.
func (f *folder) reanchorNotes(notes []Note) (kept []Note, moved int) {
	kept = make([]Note, 0, len(notes))
	for _, note := range notes {
		at := false
		for _, id := range []*string{&note.Anchor, &note.EdgeFrom, &note.EdgeTo} {
			if owner, ok := f.owners[*id]; ok {
				*id, at = owner, true
			}
		}
		if at && note.EdgeFrom != "" && note.EdgeFrom == note.EdgeTo {
			note.Anchor, note.EdgeFrom, note.EdgeTo = note.EdgeFrom, "", ""
		}
		if at {
			moved++
		}
		kept = append(kept, note)
	}
	return kept, moved
}

// usageNode reports a node drawn for a usage: not a definition, package or view.
func usageNode(node *Node) bool {
	_, keyword := splitRowPrefix(node.Kind)
	switch keyword {
	case "", "package", "library package", "view", "viewpoint", "rendering":
		return false
	}
	return keyword != "def" && !strings.HasSuffix(keyword, " def")
}

// portNode reports a node drawn for a port usage.
func portNode(node *Node) bool {
	_, keyword := splitRowPrefix(node.Kind)
	return keyword == "port"
}

// viewNode reports a node drawn for a view or viewpoint, under which the tree
// places what the view exposes rather than what it owns.
func viewNode(node *Node) bool {
	_, keyword := splitRowPrefix(node.Kind)
	return keyword == "view" || keyword == "viewpoint" || keyword == "rendering"
}

// packageNode reports a node drawn for a package, whose members are never rows.
func packageNode(node *Node) bool {
	_, keyword := splitRowPrefix(node.Kind)
	return keyword == "package" || keyword == "library package"
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

// foldOmitted folds the membership and typing edges of the unplaced members
// of a placed node, which a form leaves undrawn, into the owner's labelled
// edge to each drawn type — the relationship is shown so or not at all —
// before the edges at the undrawn nodes are dropped. An unplaced node folds
// under its nearest placed ancestor; a placed node under an unplaced one,
// which the form hoists, is drawn and keeps its own members' edges to fold.
func (p *placement) foldOmitted(roots []*Node, edges []Edge, nesting map[[2]string]bool) []Edge {
	f := &folder{placement: p, mode: CompartmentsPositioned, owners: map[string]string{}, rows: map[string]*Node{}, nesting: nesting}
	var walk func(node *Node, owner string)
	walk = func(node *Node, owner string) {
		for _, child := range node.Children {
			switch {
			case p.placed[child.ID]:
				walk(child, child.ID)
			default:
				if owner != "" {
					f.owners[child.ID], f.rows[child.ID] = owner, child
				}
				walk(child, owner)
			}
		}
	}
	for _, root := range roots {
		if p.placed[root.ID] {
			walk(root, root.ID)
		} else {
			walk(root, "")
		}
	}
	if len(f.rows) == 0 {
		return edges
	}
	folded, _ := f.foldMemberships(edges)
	return folded
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
func (r *Rendering) settleUnplaced(unplaced Unplaced, compartments Compartments, form Form) *Rendering {
	if unplaced != UnplacedStrip || compartments.automatic() {
		r = foldCompartments(withoutStandIns(r), compartments)
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
