package view

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// maxBehaviorDepth bounds how deep a nested action usage is lowered, so a
// behavior performing itself still renders.
const maxBehaviorDepth = 8

// renderStates renders the states and transitions of the exposed behaviors, from
// the lowered StateGraph of each. An exposed element that is no state machine is
// reported, and a machine that does not lower is reported with the reason.
// States and transitions are placed by the Layout and Route annotations
// positioning their declarations in view, nil for a rendering outside any view.
func (r *Renderer) renderStates(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	ids := &nodeIDs{}
	for _, elem := range exposed {
		if elem.Kind != symbols.SymbolStateDef && elem.Kind != symbols.SymbolStateUsage {
			out.Notices = append(out.Notices, fmt.Sprintf("%s %s is no state machine; a state rendering does not show it",
				declKind(elem), r.notationName(elem)))
			continue
		}
		graph, err := lower.ToStateGraph(elem.Decl, declScope(elem))
		if err != nil {
			out.Notices = append(out.Notices, fmt.Sprintf("%s %s does not lower to a state graph: %v",
				declKind(elem), r.notationName(elem), err))
			continue
		}
		out.Roots = append(out.Roots, r.stateMachineNode(view, elem, graph, ids, out))
	}
}

// stateMachineNode renders one lowered state machine: its regions and states as
// nested nodes, the start of each body with the entry transitions out of it, and
// its transitions as edges.
func (r *Renderer) stateMachineNode(view, machine *symbols.Symbol, graph *lower.StateGraph, ids *nodeIDs, out *Rendering) *Node {
	root := &Node{ID: ids.take(), Kind: declKind(machine), Name: r.notationName(machine), Detail: declType(machine),
		Origin: symbolOrigin(machine), Geometry: r.geometryOf(view, machine, out)}
	doc := machine.DocName
	nodes := map[ast.Node]*Node{}
	regions := map[*ast.StateRegion]*Node{}
	place := func(node *Node, decl ast.Node) *Node {
		node.Geometry = r.declaredGeometryOf(view, machine, decl, out)
		return node
	}

	// Regions first: a state of an orthogonal region is nested in that region,
	// and the region order is the order the machine enters and exits them in.
	for _, region := range graph.TopRegions {
		regions[region] = place(r.regionNode(region, doc, ids), region)
		root.Children = append(root.Children, regions[region])
	}
	for _, state := range graph.States {
		node := place(r.stateNode(state, graph, doc, ids), graph.DeclOf(state))
		nodes[state] = node
		for _, region := range graph.CompositeStates[state] {
			regions[region] = place(r.regionNode(region, doc, ids), region)
			node.Children = append(node.Children, regions[region])
		}
	}
	// Then placement, so that a nested state reaches the node of its owner.
	for _, state := range graph.States {
		parent := root
		if region := graph.RegionOf[state]; region != nil && regions[region] != nil {
			parent = regions[region]
		} else if owner := graph.ParentState[state]; owner != nil && nodes[owner] != nil {
			parent = nodes[owner]
		}
		parent.Children = append(parent.Children, nodes[state])
	}
	for _, pseudo := range graph.Pseudostates {
		node := place(&Node{ID: ids.take(), Kind: pseudo.Kind.String(), Name: notationName(pseudo.Name),
			Origin: nodeOrigin(doc, pseudo)}, pseudo)
		nodes[pseudo] = node
		parent := root
		if owner := graph.PseudostateOwner[pseudo]; owner != nil && nodes[owner] != nil {
			parent = nodes[owner]
		}
		parent.Children = append(parent.Children, node)
	}

	// Each body that says where it starts gets a start node, and one edge per
	// entry transition, in the order the guards are tried in.
	bodies := []stateBody{{nil, root}}
	for _, region := range graph.TopRegions {
		bodies = append(bodies, stateBody{region, regions[region]})
	}
	for _, state := range graph.States {
		bodies = append(bodies, stateBody{state, nodes[state]})
		for _, region := range graph.CompositeStates[state] {
			bodies = append(bodies, stateBody{region, regions[region]})
		}
	}
	for _, body := range bodies {
		entries := graph.StartOf(body.owner)
		if len(entries) == 0 {
			continue
		}
		start := &Node{ID: ids.take(), Kind: startKind}
		body.node.Children = append([]*Node{start}, body.node.Children...)
		for _, entry := range entries {
			target, ok := nodes[entry.Target]
			if !ok {
				out.Notices = append(out.Notices, fmt.Sprintf("entry transition to %s of %s leaves the machine's own states; no edge is drawn",
					behaviorNodeName(entry.Target), r.notationName(machine)))
				continue
			}
			out.Edges = append(out.Edges, Edge{
				From: start.ID, To: target.ID, Label: r.guardLabel(doc, entry.Guard), Kind: EdgeTransition,
				Origin: nodeOrigin(doc, entry.Decl), Route: r.declaredRouteOf(view, machine, entry.Decl, out),
			})
		}
	}

	sources := make([]ast.Node, 0, len(graph.States)+len(graph.Pseudostates))
	for _, state := range graph.States {
		sources = append(sources, state)
	}
	for _, pseudo := range graph.Pseudostates {
		sources = append(sources, pseudo)
	}
	for _, src := range sources {
		for _, transition := range graph.Transitions[src] {
			target, ok := nodes[transition.Target]
			if !ok {
				out.Notices = append(out.Notices, fmt.Sprintf("transition %s of %s leaves the machine's own states; no edge is drawn",
					transitionName(transition), r.notationName(machine)))
				continue
			}
			out.Edges = append(out.Edges, Edge{
				From: nodes[src].ID, To: target.ID, Label: r.transitionLabel(doc, transition), Kind: EdgeTransition,
				Origin: nodeOrigin(doc, transition.Decl), Route: r.declaredRouteOf(view, machine, transition.Decl, out),
			})
		}
	}
	if len(root.Children) == 0 {
		root.Detail = detailWith(root.Detail, "declares no states")
	}
	return root
}

// startKind is the Kind of the node a body's entry transitions leave, which a
// state diagram draws as its start marker.
const startKind = "start"

// stateBody is a body whose entry transitions say where it starts — the machine
// (owner nil), a composite state or a region — and the node rendering it.
type stateBody struct {
	owner ast.Node
	node  *Node
}

// regionNode renders one orthogonal region, which holds the states declared in
// it.
func (r *Renderer) regionNode(region *ast.StateRegion, doc string, ids *nodeIDs) *Node {
	return &Node{ID: ids.take(), Kind: "region", Name: notationName(region.Name), Origin: nodeOrigin(doc, region)}
}

// stateNode renders one state with what the machine says about it: whether its
// body unconditionally starts in it, whether entering it completes, what it runs.
func (r *Renderer) stateNode(state *ast.StateNode, graph *lower.StateGraph, doc string, ids *nodeIDs) *Node {
	node := &Node{ID: ids.take(), Kind: "state", Name: notationName(state.Name), Origin: nodeOrigin(doc, state)}
	var detail []string
	if graph.UnconditionalStart(bodyOwning(graph, state)) == state {
		detail = append(detail, "initial")
	}
	if graph.Completes(state) {
		detail = append(detail, "completes")
	}
	if behaviors := graph.Behaviors[state]; behaviors != nil {
		for _, part := range []struct {
			name string
			list []lower.StateBehavior
		}{{"entry", behaviors.Entry}, {"do", behaviors.Do}, {"exit", behaviors.Exit}} {
			if len(part.list) > 0 {
				detail = append(detail, part.name)
			}
		}
	}
	if len(graph.Deferred[state]) > 0 {
		detail = append(detail, "defers")
	}
	node.Detail = strings.Join(detail, ", ")
	return node
}

// bodyOwning is the body a state is declared in, as StateGraph.StartOf names it:
// its region, else its parent state, else nil for the machine's own body.
func bodyOwning(graph *lower.StateGraph, state *ast.StateNode) ast.Node {
	if region := graph.RegionOf[state]; region != nil {
		return region
	}
	if parent := graph.ParentState[state]; parent != nil {
		return parent
	}
	return nil
}

// transitionLabel is what a transition edge carries: its name, the trigger it
// waits for, the guard it is subject to, and whether it runs an effect —
// `maintain: after 5 [ok] / effect`.
func (r *Renderer) transitionLabel(doc string, transition *lower.Transition) string {
	var parts []string
	if transition.Name != "" {
		parts = append(parts, notationName(transition.Name)+":")
	}
	if trigger := r.triggerLabel(doc, transition.Trigger); trigger != "" {
		parts = append(parts, trigger)
	}
	if transition.Via != "" {
		parts = append(parts, "via "+notationName(transition.Via))
	}
	if guard := r.guardLabel(doc, transition.Guard); guard != "" {
		parts = append(parts, guard)
	}
	if len(transition.Effect) > 0 {
		parts = append(parts, "/ "+behaviorNames(transition.Effect))
	}
	return strings.Join(parts, " ")
}

// guardLabel is the guard an edge carries, `[guard]` as written when the source
// is at hand, and "" for an unguarded one.
func (r *Renderer) guardLabel(doc string, guard ast.Node) string {
	if guard == nil {
		return ""
	}
	if text := r.nodeText(doc, guard); text != "" {
		return "[" + text + "]"
	}
	return "[guard]"
}

// transitionName names a transition in a notice: its own name, else the states
// it was written between.
func transitionName(transition *lower.Transition) string {
	if transition.Name != "" {
		return notationName(transition.Name)
	}
	return fmt.Sprintf("first %s then %s", behaviorNodeName(transition.Source), behaviorNodeName(transition.Target))
}

// behaviorNames names the behaviors an effect runs, so an edge says what it
// does without carrying the statements themselves.
func behaviorNames(behaviors []lower.StateBehavior) string {
	names := make([]string, 0, len(behaviors))
	for _, behavior := range behaviors {
		if behavior.Name != "" {
			names = append(names, notationName(behavior.Name))
			continue
		}
		names = append(names, "effect")
	}
	return strings.Join(names, ", ")
}

// triggerLabel is the event a transition waits for, as written when the source
// is at hand, else what kind of event it is.
func (r *Renderer) triggerLabel(doc string, trigger ast.Node) string {
	if trigger == nil {
		return ""
	}
	if text := r.nodeText(doc, trigger); text != "" {
		return text
	}
	switch event := trigger.(type) {
	case *ast.TimeEvent:
		keyword := "after"
		if event.Absolute {
			keyword = "at"
		}
		return joinNonEmpty(keyword, r.nodeText(doc, event.Duration))
	case *ast.ChangeEvent:
		return joinNonEmpty("when", r.nodeText(doc, event.Condition))
	case *ast.AcceptEvent:
		if event.Subsets != nil {
			return "accept :> " + notationName(qualifiedText(event.Subsets))
		}
		if event.SignalType != nil {
			return "accept " + notationName(qualifiedText(event.SignalType))
		}
		return "accept event"
	case *ast.CallEvent:
		if event.Operation != nil {
			return "accept " + notationName(qualifiedText(event.Operation))
		}
		return "call event"
	}
	return "event"
}

// nodeText is the notation a node was written in, collapsed to one line, and ""
// when the rendering holds no source for it.
func (r *Renderer) nodeText(doc string, node ast.Node) string {
	if r.text == nil || node == nil || doc == "" {
		return ""
	}
	span := node.Span()
	if span.Len <= 0 {
		return ""
	}
	return collapseSpace(r.text(doc, span))
}

// joinNonEmpty joins a keyword and what follows it, dropping an empty tail.
func joinNonEmpty(keyword, tail string) string {
	if tail == "" {
		return keyword
	}
	return keyword + " " + tail
}

// collapseSpace folds the runs of whitespace in a label written across lines
// into single spaces.
func collapseSpace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// renderActions renders the nodes and successions of the exposed behaviors, from
// the lowered ActionGraph of each. Nodes and edges are placed by the Layout and
// Route annotations positioning their declarations in view, nil for a rendering
// outside any view.
func (r *Renderer) renderActions(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	ids := &nodeIDs{}
	for _, elem := range exposed {
		if elem.Kind != symbols.SymbolActionDef && elem.Kind != symbols.SymbolActionUsage {
			out.Notices = append(out.Notices, fmt.Sprintf("%s %s is no action; an action rendering does not show it",
				declKind(elem), r.notationName(elem)))
			continue
		}
		subject := actionSubject{decl: elem.Decl, kind: declKind(elem), name: r.notationName(elem),
			scope: declScope(elem), doc: elem.DocName, view: view, elem: elem}
		node, ok := r.actionNode(subject, ids, out, map[ast.Node]bool{}, 0)
		if ok {
			out.Roots = append(out.Roots, node)
		}
	}
}

// actionSubject is the action being rendered and the naming context it is
// rendered in.
type actionSubject struct {
	decl  ast.Node
	kind  string
	name  string
	scope *symbols.Scope
	doc   string
	// view is the view the action is drawn in, nil outside any view; elem is
	// the exposed action the subject is rendered under.
	view *symbols.Symbol
	elem *symbols.Symbol
}

// actionNode renders one lowered action: its nodes as nested nodes, its
// successions and object flows as edges. A nested action declaring a body of its
// own is lowered in turn, so the rendering shows the flow within it as well.
func (r *Renderer) actionNode(subject actionSubject, ids *nodeIDs, out *Rendering,
	lowered map[ast.Node]bool, depth int) (*Node, bool) {
	decl, kind, name, scope, doc := subject.decl, subject.kind, subject.name, subject.scope, subject.doc
	graph, err := lower.ToActionGraph(decl, scope)
	if err != nil {
		// A node performing statements holds no flow of its own to render, which is
		// no shortcoming of the rendering; only an exposed action is reported.
		if depth > 0 && errors.Is(err, lower.ErrStatementOutsideFlow) {
			return nil, false
		}
		out.Notices = append(out.Notices, fmt.Sprintf("%s %s does not lower to an action graph: %v", kind, name, err))
		return nil, false
	}
	root := &Node{ID: ids.take(), Kind: kind, Name: name, Origin: nodeOrigin(doc, decl),
		Geometry: r.declaredGeometryOf(subject.view, subject.elem, decl, out)}
	lowered[decl] = true
	nodes := map[ast.Node]*Node{}
	for _, node := range graph.Nodes {
		child := &Node{ID: ids.take(), Kind: actionNodeKind(node, graph), Name: notationName(behaviorNodeName(node)),
			Origin: nodeOrigin(doc, node), Geometry: r.declaredGeometryOf(subject.view, subject.elem, node, out)}
		nodes[node] = child
		root.Children = append(root.Children, child)
		if nested, ok := nestedAction(node); ok && depth < maxBehaviorDepth && !lowered[node] {
			nestedSubject := actionSubject{decl: nested, kind: child.Kind, name: child.Name,
				scope: actionScope(scope, nested), doc: doc, view: subject.view, elem: subject.elem}
			sub, ok := r.actionNode(nestedSubject, ids, out, lowered, depth+1)
			if ok {
				child.Children, child.Detail = sub.Children, detailWith(child.Detail, "own flow")
				// The nested flow's own edges belong to the nested nodes, which the
				// sub-rendering already added to out.Edges.
			}
		}
	}
	for _, src := range graph.Nodes {
		for _, edge := range graph.Edges[src] {
			to, ok := nodes[edge.Target]
			if !ok {
				out.Notices = append(out.Notices, fmt.Sprintf("succession from %s in %s leaves the action's own nodes; no edge is drawn",
					notationName(behaviorNodeName(src)), name))
				continue
			}
			label := ""
			if guard := edge.Guard; guard != nil {
				if text := r.nodeText(doc, guard); text != "" {
					label = "[" + text + "]"
				} else {
					label = "[guard]"
				}
			}
			out.Edges = append(out.Edges, Edge{From: nodes[src].ID, To: to.ID, Label: label, Kind: EdgeSuccession,
				Origin: nodeOrigin(doc, edge.Decl), Route: r.declaredRouteOf(subject.view, subject.elem, edge.Decl, out)})
		}
		for _, flow := range graph.DataFlows[src] {
			to, ok := nodes[flow.Target]
			if !ok {
				out.Notices = append(out.Notices, fmt.Sprintf("flow from %s in %s leaves the action's own nodes; no edge is drawn",
					notationName(behaviorNodeName(src)), name))
				continue
			}
			out.Edges = append(out.Edges, Edge{From: nodes[src].ID, To: to.ID, Label: flowLabel(flow), Kind: EdgeFlow,
				Origin: nodeOrigin(doc, flow.Decl), Route: r.declaredRouteOf(subject.view, subject.elem, flow.Decl, out)})
		}
	}
	if len(root.Children) == 0 {
		root.Detail = detailWith(root.Detail, "declares no nodes")
	}
	return root, true
}

// flowLabel is what an object flow carries: the pins it joins, named by the flow
// when it has a name of its own.
func flowLabel(flow lower.ObjectFlow) string {
	label := flow.SourcePin
	if flow.TargetPin != "" {
		label = strings.TrimPrefix(label+" to "+flow.TargetPin, " to ")
	}
	if flow.Name != "" {
		return notationName(flow.Name) + ": " + label
	}
	return label
}

// nestedAction is the declaration of an action node that performs a body of its
// own, which is lowered as an action in turn.
func nestedAction(node ast.Node) (ast.Node, bool) {
	switch n := node.(type) {
	case *ast.Usage:
		if n.Kind == ast.UsageAction && n.HasBody {
			return n, true
		}
	case *ast.Definition:
		if n.Kind == ast.DefAction {
			return n, true
		}
	}
	return nil, false
}

// actionScope is the scope a nested action's body resolves in: the child scope
// the enclosing scope holds for it, else the enclosing scope itself.
func actionScope(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	if scope == nil {
		return nil
	}
	if child := scope.ChildFor(decl); child != nil {
		return child
	}
	return scope
}

// actionNodeKind names an action graph node the way the notation declares it, so
// a rendering says "fork" and "action" rather than a Go type name.
func actionNodeKind(node ast.Node, graph *lower.ActionGraph) string {
	if graph != nil && graph.StatementRuns[node] {
		return "statements"
	}
	switch n := node.(type) {
	case *ast.InitialNode:
		return "initial"
	case *ast.FinalNode:
		return "final"
	case *ast.ForkNode:
		return "fork"
	case *ast.JoinNode:
		return "join"
	case *ast.MergeNode:
		return "merge"
	case *ast.DecisionNode:
		return "decision"
	case *ast.ActionExecutionNode:
		return "perform"
	case *ast.StateNode:
		return "state"
	case *ast.Usage:
		return n.Kind.String()
	case *ast.Definition:
		return n.Kind.String() + " def"
	}
	return "node"
}

// behaviorNodeName is the name a state or action graph node was declared with.
func behaviorNodeName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.InitialNode:
		return n.Name
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
	case *ast.PseudostateNode:
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

// declScope is the scope a declaration's own members resolve in: the scope it
// owns, else the scope it was declared in.
func declScope(sym *symbols.Symbol) *symbols.Scope {
	if sym == nil {
		return nil
	}
	if sym.Scope != nil {
		return sym.Scope
	}
	return sym.OwnerScope
}
