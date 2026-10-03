package view

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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
		graph, err := lower.ToStateGraphWithEndpoints(elem.Decl, declScope(elem), lower.NewLibraryStateTypes(r.resolver))
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
	root := &Node{ID: ids.take(), Kind: declKind(machine), Name: r.notationName(machine), NameSynthesized: r.model.NameSynthesized(machine),
		Type: declType(machine), Origin: symbolOrigin(machine), Inherited: inheritedOrigins(graph.Inherited()), Geometry: r.geometryOf(view, machine, out)}
	r.dress(view, machine, root, out)
	nodes := map[ast.Node]*Node{}
	regions := map[*ast.StateRegion]*Node{}
	place := func(node *Node, owner, decl ast.Node, name string) *Node {
		node.Geometry = r.nodeGeometryOf(view, machine, r.bodySymbol(machine, graph, owner), decl, name, out)
		node.NameSynthesized = node.NameSynthesized || r.declaredNameSynthesized(machine, decl)
		return r.declaredDress(view, machine, decl, node, out)
	}

	// Regions first: a state of an orthogonal region is nested in that region,
	// and the region order is the order the machine enters and exits them in.
	for _, region := range graph.TopRegions {
		regions[region] = place(r.regionNode(region, graph, machine, ids), nil, region, region.Name)
		root.Children = append(root.Children, regions[region])
	}
	for _, state := range graph.States {
		node := place(r.stateNode(state, graph, machine, ids), bodyOwning(graph, state), graph.DeclOf(state), state.Name)
		if graph.Completes(state) {
			node.Geometry = r.memberGeometryOf(view, r.bodySymbol(machine, graph, bodyOwning(graph, state)), ast.DoneFeature, out)
		}
		nodes[state] = node
		for _, region := range graph.CompositeStates[state] {
			regions[region] = place(r.regionNode(region, graph, machine, ids), state, region, region.Name)
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
		node := place(&Node{ID: ids.take(), Kind: pseudo.Kind.String(), Name: nameText(pseudo.Name),
			Origin: nodeOrigin(docOf(graph, pseudo, machine.DocName), pseudo)}, graph.PseudostateOwner[pseudo], pseudo, pseudo.Name)
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
		r.entryEdges(view, machine, graph, body, nodes, ids, out)
	}

	sources := make([]ast.Node, 0, len(graph.States)+len(graph.Pseudostates))
	for _, state := range graph.States {
		sources = append(sources, state)
	}
	for _, pseudo := range graph.Pseudostates {
		sources = append(sources, pseudo)
	}
	for _, src := range sources {
		r.transitionEdges(view, machine, graph, src, nodes, out)
	}
	if len(root.Children) == 0 {
		root.Detail = detailWith(root.Detail, "declares no states")
	}
	return root
}

// entryEdges gives a body that says where it starts a start node, and one edge
// per entry transition, in the order the guards are tried in.
func (r *Renderer) entryEdges(view, machine *symbols.Symbol, graph *lower.StateGraph, body stateBody,
	nodes map[ast.Node]*Node, ids *nodeIDs, out *Rendering) {
	entries := graph.StartOf(body.owner)
	if len(entries) == 0 {
		return
	}
	start := &Node{ID: ids.take(), Kind: startKind,
		Geometry: r.memberGeometryOf(view, r.bodySymbol(machine, graph, body.owner), ast.StartFeature, out)}
	body.node.Children = append([]*Node{start}, body.node.Children...)
	for _, entry := range entries {
		entryTarget := ast.Node(entry.Target)
		if entry.Via != nil {
			entryTarget = entry.Via
		}
		target, ok := nodes[entryTarget]
		if !ok {
			out.Notices = append(out.Notices, fmt.Sprintf("entry transition to %s of %s leaves the machine's own states; no edge is drawn",
				behaviorNodeName(entryTarget), r.notationName(machine)))
			continue
		}
		doc := docOf(graph, entry.Decl, machine.DocName)
		label := r.guardLabel(doc, entry.Guard)
		if len(entry.Effect) > 0 {
			effect := r.behaviorNames(doc, entry.Effect)
			if label == "" {
				label = "/" + effect
			} else {
				label += " / " + effect
			}
		}
		out.Edges = append(out.Edges, Edge{
			From: start.ID, To: target.ID, Label: label, Kind: EdgeTransition,
			Origin: nodeOrigin(doc, entry.Decl), Route: r.declaredRouteOf(view, machine, entry.Decl, out),
			Style: r.declaredEdgeDress(view, machine, entry.Decl, start.ID, target.ID, out),
		})
	}
}

// transitionEdges adds an edge for each transition leaving src, reporting one
// whose target the machine's own states do not include.
func (r *Renderer) transitionEdges(view, machine *symbols.Symbol, graph *lower.StateGraph, src ast.Node,
	nodes map[ast.Node]*Node, out *Rendering) {
	for _, transition := range graph.Transitions[src] {
		target, ok := nodes[transition.Target]
		if !ok {
			out.Notices = append(out.Notices, fmt.Sprintf("transition %s of %s leaves the machine's own states; no edge is drawn",
				transitionName(transition), r.notationName(machine)))
			continue
		}
		doc := docOf(graph, transition.Decl, machine.DocName)
		out.Edges = append(out.Edges, Edge{
			From: nodes[src].ID, To: target.ID, Label: r.transitionLabel(doc, transition, r.declaredNameSynthesized(machine, transition.Decl)),
			Kind: EdgeTransition, Origin: nodeOrigin(doc, transition.Decl), Route: r.declaredRouteOf(view, machine, transition.Decl, out),
			Style: r.declaredEdgeDress(view, machine, transition.Decl, nodes[src].ID, target.ID, out),
		})
	}
}

// terminateKind is the Kind of a terminate action usage (`action final terminate;`),
// the activity's final node, drawn as the final symbol when a box places it.
const terminateKind = "terminate action"

// startKind is the Kind of the node a body's entry transitions leave, which a
// state diagram draws as its start marker.
const startKind = "start"

// stateBody is a body whose entry transitions say where it starts — the machine
// (owner nil), a composite state or a region — and the node rendering it.
type stateBody struct {
	owner ast.Node
	node  *Node
}

// bodySymbol is the element whose body owner is, as StateGraph.StartOf names
// bodies: the machine itself for nil, else the state or region declared under
// it; nil when owner declares no element.
func (r *Renderer) bodySymbol(machine *symbols.Symbol, graph *lower.StateGraph, owner ast.Node) *symbols.Symbol {
	if owner == nil {
		return machine
	}
	decl := owner
	if state, ok := owner.(*ast.StateNode); ok {
		decl = graph.DeclOf(state)
	}
	return r.declaredSymbol(machine, decl)
}

// docOfer is a lowered graph that knows which document each of its
// declarations was written in.
type docOfer interface {
	DocOf(decl ast.Node) string
}

// docOf is the document a graph declaration was written in, else the document of
// the behavior drawn, for a graph lowered outside any document.
func docOf(graph docOfer, decl ast.Node, fallback string) string {
	if doc := graph.DocOf(decl); doc != "" {
		return doc
	}
	return fallback
}

// regionNode renders one orthogonal region, which holds the states declared in
// it.
func (r *Renderer) regionNode(region *ast.StateRegion, graph *lower.StateGraph, machine *symbols.Symbol, ids *nodeIDs) *Node {
	return &Node{ID: ids.take(), Kind: "region", Name: nameText(region.Name), Origin: nodeOrigin(docOf(graph, region, machine.DocName), region)}
}

// stateNode renders one state with what the machine says about it: whether its
// body unconditionally starts in it, what it runs as `do / Activity`, what it
// defers. The `done` vertex a body's completion synthesizes is its final node.
func (r *Renderer) stateNode(state *ast.StateNode, graph *lower.StateGraph, machine *symbols.Symbol, ids *nodeIDs) *Node {
	node := &Node{ID: ids.take(), Kind: "state", Name: nameText(state.Name), Type: nodeType(graph.DeclOf(state)),
		Origin: nodeOrigin(docOf(graph, state, machine.DocName), state)}
	if graph.Completes(state) {
		node.Kind, node.Name, node.Type, node.NameSynthesized = "final", ast.DoneFeature, "", true
		return node
	}
	var detail []string
	if graph.UnconditionalStart(bodyOwning(graph, state)) == state {
		detail = append(detail, "initial")
	}
	if behaviors := graph.Behaviors[state]; behaviors != nil {
		for _, part := range []struct {
			name string
			list []lower.StateBehavior
		}{{"entry", behaviors.Entry}, {"do", behaviors.Do}, {"exit", behaviors.Exit}} {
			if len(part.list) > 0 {
				detail = append(detail, r.stateBehaviorLabel(node.Origin.Doc, part.name, part.list))
			}
		}
	}
	node.Detail = strings.Join(detail, ", ")
	return node
}

const acceptPrefix = "accept "

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

// transitionLabel is a transition edge's trigger, guard and effect, `after 5 [ok] /
// effect`; a transition with none of those is labelled by its name, unless synthesized.
func (r *Renderer) transitionLabel(doc string, transition *lower.Transition, synthesized bool) string {
	var parts []string
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
		parts = append(parts, "/ "+r.behaviorNames(doc, transition.Effect))
	}
	return edgeLabel(transition.Name, strings.Join(parts, " "), synthesized)
}

// edgeLabel is text when the edge has any of its own, else its name: a name
// only labels an edge that nothing else describes, and a name a migration made
// up (synthesized) labels nothing, as the unnamed source edge it stands for.
func edgeLabel(name, text string, synthesized bool) string {
	if text != "" || name == "" || synthesized {
		return text
	}
	return nameText(name)
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
		return nameText(transition.Name)
	}
	return fmt.Sprintf("first %s then %s", behaviorNodeName(transition.Source), behaviorNodeName(transition.Target))
}

// behaviorNames names the behaviors an effect runs, so an edge says what it
// does without carrying the statements themselves.
func (r *Renderer) behaviorNames(doc string, behaviors []lower.StateBehavior) string {
	names := make([]string, 0, len(behaviors))
	for _, behavior := range behaviors {
		if name := r.behaviorText(doc, behavior); name != "" {
			names = append(names, name)
			continue
		}
		names = append(names, "effect")
	}
	return strings.Join(names, ", ")
}

// behaviorText is what a state behavior is drawn as: its name, else the
// activity it performs by type (`do action : Reset` reads `Reset`), else what
// its anonymous body does — the message it sends, the one assignment it makes —
// as written in the document declaring it, else "".
func (r *Renderer) behaviorText(doc string, behavior lower.StateBehavior) string {
	if behavior.Name != "" {
		return nameText(behavior.Name)
	}
	if typ := nodeType(behavior.Node); typ != "" {
		return source.ReferenceEndNames(typ)
	}
	if declared := symbols.DocNameOf(behavior.Scope); declared != "" {
		doc = declared
	}
	body := behavior.Body
	if len(body) == 1 {
		if block, ok := body[0].(lower.Block); ok {
			body = block.Statements
		}
	}
	if text := r.sendText(doc, body); text != "" {
		return text
	}
	return r.assignmentText(doc, body)
}

// stateBehaviorLabel is a state's compartment line for one kind of behavior, as
// UML writes it: `do / Activity`, or the kind alone when none is named.
func (r *Renderer) stateBehaviorLabel(doc, kind string, behaviors []lower.StateBehavior) string {
	var names []string
	for _, behavior := range behaviors {
		if name := r.behaviorText(doc, behavior); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return kind
	}
	return kind + " / " + strings.Join(names, ", ")
}

// triggerLabel is the event a transition waits for: an accepted signal or called
// operation by the name it ends in, as a type is; a time or change event as
// written when the source is at hand; else what kind of event it is.
func (r *Renderer) triggerLabel(doc string, trigger ast.Node) string {
	if trigger == nil {
		return ""
	}
	switch event := trigger.(type) {
	case *ast.AcceptEvent:
		if event.SignalType != nil {
			return joinNonEmpty(joinNonEmpty("accept", payloadHead(event.Payload)), endName(event.SignalType))
		}
	case *ast.CallEvent:
		if event.Operation != nil {
			return acceptPrefix + endName(event.Operation) + callParameters(event.Parameters)
		}
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
		return "accept event"
	case *ast.CallEvent:
		return "call event"
	}
	return "event"
}

// payloadHead is the payload parameter an accept declares, `msg :` for
// `accept msg : Warning`, and "" when the accept names none.
func payloadHead(payload *ast.Usage) string {
	if payload == nil || payload.Ident.Name == "" {
		return ""
	}
	return nameText(payload.Ident.Name) + " :"
}

// endName is the last segment of a qualified reference, quoted as the notation does.
func endName(name *ast.QualifiedName) string {
	if name == nil || len(name.Parts) == 0 {
		return ""
	}
	return nameText(name.Parts[len(name.Parts)-1].Text)
}

// callParameters writes a call trigger's argument list, `(speed)`, `()` for none.
func callParameters(parameters []ast.NameSegment) string {
	names := make([]string, len(parameters))
	for i, parameter := range parameters {
		names[i] = nameText(parameter.Text)
	}
	return "(" + strings.Join(names, ", ") + ")"
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
		subject := actionSubject{decl: elem.Decl, kind: declKind(elem), name: r.notationName(elem), typ: declType(elem),
			scope: declScope(elem), doc: elem.DocName, view: view, elem: elem}
		node, ok := r.actionNode(subject, ids, out, map[ast.Node]bool{}, map[ast.Node]*Node{}, 0)
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
	typ   string
	scope *symbols.Scope
	doc   string
	// view is the view the action is drawn in, nil outside any view; elem is
	// the exposed action the subject is rendered under.
	view *symbols.Symbol
	elem *symbols.Symbol
	// frame is the node already standing for the action in the enclosing
	// rendering, whose pins its bindings attach to; nil for a rendered action.
	frame *Node
}

// actionNode renders one lowered action: its nodes as nested nodes, its
// successions, object flows and bindings as edges. A nested action declaring a
// body of its own is lowered in turn, so the rendering shows the flow within it
// as well; the node standing for it in the caller is its frame, carrying its
// nodes, geometry and notes, and its pins are the ones its bindings attach to.
// drawn collects the node drawn for each lowered node across the nesting, so a
// binding reaching into a nested flow finds its pin.
func (r *Renderer) actionNode(subject actionSubject, ids *nodeIDs, out *Rendering,
	lowered map[ast.Node]bool, drawn map[ast.Node]*Node, depth int) (*Node, bool) {
	decl, kind, name, scope, doc := subject.decl, subject.kind, subject.name, subject.scope, subject.doc
	// A node performing statements holds no flow of its own to render.
	if usage, ok := decl.(*ast.Usage); ok && depth > 0 && lower.PerformsLeafStatements(usage.Members) {
		return nil, false
	}
	graph, err := lower.ToActionGraphWith(decl, scope, r.resolver)
	if err != nil {
		out.Notices = append(out.Notices, fmt.Sprintf("%s %s does not lower to an action graph: %v", kind, name, err))
		return nil, false
	}
	root := subject.frame
	if root == nil {
		root = &Node{ID: ids.take(), Kind: kind, Name: name, NameSynthesized: r.declaredNameSynthesized(subject.elem, decl), Type: subject.typ,
			Origin: nodeOrigin(doc, decl), Inherited: inheritedOrigins(graph.Inherited())}
		root.Ports = r.inheritedPorts(subject.elem, decl, root.ID, actionPorts(root.ID, graph.Parameters, doc))
		root.Geometry = r.declaredGeometryOf(subject.view, subject.elem, decl, out)
		r.declaredDress(subject.view, subject.elem, decl, root, out)
	}
	lowered[decl] = true
	nodes := map[ast.Node]*Node{}
	owner := r.declaredSymbol(subject.elem, decl)
	for _, node := range graph.Nodes {
		nodeDoc := docOf(graph, node, doc)
		child := &Node{ID: ids.take(), Kind: actionNodeKind(node, graph), Name: nameText(behaviorNodeName(node)),
			NameSynthesized: languageNamed(node) || r.declaredNameSynthesized(subject.elem, node), StandIn: r.declaredStandIn(subject.elem, node),
			Type: nodeType(node), Origin: nodeOrigin(nodeDoc, node)}
		if languageNamed(node) {
			child.Geometry = r.memberGeometryOf(subject.view, owner, behaviorNodeName(node), out)
		} else {
			child.Geometry = r.nodeGeometryOf(subject.view, subject.elem, owner, node, behaviorNodeName(node), out)
		}
		r.declaredDress(subject.view, subject.elem, node, child, out)
		child.Ports = r.inheritedPorts(subject.elem, node, child.ID, actionPorts(child.ID, graph.Features[node], nodeDoc))
		if child.NameSynthesized || child.Name == "" {
			child.Text = r.actionText(graph, node, nodeDoc)
		}
		nodes[node], drawn[node] = child, child
		root.Children = append(root.Children, child)
		if nested, ok := nestedAction(node); ok && depth < maxBehaviorDepth && !lowered[node] {
			nestedScope := graph.Scopes[node]
			if nestedScope == nil {
				nestedScope = actionScope(scope, nested)
			}
			nestedSubject := actionSubject{decl: nested, kind: child.Kind, name: child.Name, typ: child.Type,
				scope: nestedScope, doc: nodeDoc, view: subject.view, elem: subject.elem, frame: child}
			// The nested flow's own edges belong to the nested nodes, which the
			// sub-rendering adds to out.Edges; a flow with no nodes leaves nothing to show.
			if _, ok := r.actionNode(nestedSubject, ids, out, lowered, drawn, depth+1); ok && len(child.Children) > 0 {
				child.Detail = detailWith(child.Detail, "own flow")
			}
		}
	}
	r.actionEdges(subject, graph, nodes, out)
	r.bindingEdges(subject, graph, root, nodes, drawn, out)
	if subject.frame == nil && len(root.Children) == 0 {
		root.Detail = detailWith(root.Detail, "declares no nodes")
	}
	return root, true
}

// actionEdges draws the action's successions and object flows between its own
// nodes; one leaving them is noticed instead.
func (r *Renderer) actionEdges(subject actionSubject, graph *lower.ActionGraph, nodes map[ast.Node]*Node, out *Rendering) {
	name, doc := subject.name, subject.doc
	for _, src := range graph.Nodes {
		for _, edge := range graph.Edges[src] {
			to, ok := nodes[edge.Target]
			if !ok {
				out.Notices = append(out.Notices, fmt.Sprintf("succession from %s in %s leaves the action's own nodes; no edge is drawn",
					nameText(behaviorNodeName(src)), name))
				continue
			}
			edgeDoc := docOf(graph, edge.Decl, doc)
			label := r.successionLabel(edge, edgeDoc, doc, r.declaredNameSynthesized(subject.elem, edge.Decl))
			out.Edges = append(out.Edges, Edge{From: nodes[src].ID, To: to.ID, Label: label,
				Kind: EdgeSuccession, Origin: nodeOrigin(edgeDoc, edge.Decl), Route: r.declaredRouteOf(subject.view, subject.elem, edge.Decl, out),
				Style: r.declaredEdgeDress(subject.view, subject.elem, edge.Decl, nodes[src].ID, to.ID, out)})
		}
		for _, flow := range graph.DataFlows[src] {
			to, ok := nodes[flow.Target]
			if !ok {
				out.Notices = append(out.Notices, fmt.Sprintf("flow from %s in %s leaves the action's own nodes; no edge is drawn",
					nameText(behaviorNodeName(src)), name))
				continue
			}
			synthesized := r.declaredNameSynthesized(subject.elem, flow.Decl)
			out.Edges = append(out.Edges, Edge{From: nodes[src].ID, To: to.ID, Label: flowLabel(flow, synthesized), Name: edgeName(flow.Name, synthesized),
				FromPort: portNamed(nodes[src], flow.SourcePin, PortOut), ToPort: portNamed(to, flow.TargetPin, PortIn),
				Kind: EdgeFlow, Origin: nodeOrigin(docOf(graph, flow.Decl, doc), flow.Decl), Route: r.declaredRouteOf(subject.view, subject.elem, flow.Decl, out),
				Style: r.declaredEdgeDress(subject.view, subject.elem, flow.Decl, nodes[src].ID, to.ID, out)})
		}
	}
}

// bindingEdges draws the action's parameter bindings as binding edges between
// the pins they join: a node's pin valued by one of the action's own parameters
// (`in b = bread;`, `in b = ToastBread::bread;`, `out x :>> x = y;`), an explicit
// bind with an end at a node's pin (`bind pack.boxed = toast;`), and one between
// two nodes' pins (`bind heat.t = pack.t;`, which is no flow). The edge runs the
// way the values go, from the frame's input or a node's output to the pin that
// takes them. A binding whose other end is no drawn pin — a literal, an
// expression, an attribute — draws nothing: it states a value, not a wire.
func (r *Renderer) bindingEdges(subject actionSubject, graph *lower.ActionGraph, root *Node, nodes map[ast.Node]*Node,
	drawn map[ast.Node]*Node, out *Rendering) {
	// An explicit bind between two nodes' pins lowers to an entry at each end;
	// an inout pin's value binding is in Bindings as well as ValueBindings.
	seen := map[*ast.Usage]bool{}
	for _, binding := range slices.Concat(graph.ValueBindings, graph.Bindings) {
		if seen[binding.Decl] {
			continue
		}
		seen[binding.Decl] = true
		at, atPin := bindingPin(nodes, drawn, binding.Node, binding.Path, binding.Pin, root, "")
		other, otherPin := bindingPin(nodes, drawn, binding.OtherNode, binding.OtherPath, binding.OtherPin, root, binding.OtherParameter)
		if at == nil || other == nil {
			continue
		}
		from, fromPin, to, toPin := at, atPin, other, otherPin
		if pinGives(other, otherPin, root) && !pinGives(at, atPin, root) {
			from, fromPin, to, toPin = other, otherPin, at, atPin
		}
		synthesized := r.declaredNameSynthesized(subject.elem, binding.Decl)
		var name string
		if binding.Decl.Kind == ast.UsageBinding {
			name = binding.Decl.Ident.Name
		}
		out.Edges = append(out.Edges, Edge{From: from.ID, To: to.ID, FromPort: fromPin, ToPort: toPin,
			Label: edgeLabel(name, portLabelOf(from, fromPin)+" = "+portLabelOf(to, toPin), synthesized), Name: edgeName(name, synthesized),
			Kind: EdgeBinding, Origin: nodeOrigin(docOf(graph, binding.Decl, subject.doc), binding.Decl),
			Route: r.declaredRouteOf(subject.view, subject.elem, binding.Decl, out),
			Style: r.declaredEdgeDress(subject.view, subject.elem, binding.Decl, from.ID, to.ID, out)})
	}
}

// bindingPin is the drawn node and pin a binding end is at: the node the end
// names, reached through path where it reaches into a nested flow, else the
// frame's pin named parameter; nil for an end at no drawn pin, which no edge
// can attach to.
func bindingPin(nodes, drawn map[ast.Node]*Node, at ast.Node, path []ast.Node, pin string, root *Node, parameter string) (*Node, string) {
	node := root
	if at != nil {
		var ok bool
		if node, ok = nodes[at]; !ok {
			return nil, ""
		}
		for _, step := range path {
			if node, ok = drawn[step]; !ok {
				return nil, ""
			}
		}
		parameter = pin
	}
	if id := drawnPort(node, parameter); id != "" {
		return node, id
	}
	return nil, ""
}

// drawnPort is the ID of a node's port by name, "" for a name of none.
func drawnPort(node *Node, name string) string {
	for _, port := range node.Ports {
		if port.Name == nameText(name) {
			return port.ID
		}
	}
	return ""
}

// pinGives reports whether a pin is where a binding's values come from: the
// frame's input, which the action is given, or a node's output, which it yields.
func pinGives(node *Node, id string, root *Node) bool {
	direction := portDirectionOf(node, id)
	if node == root {
		return direction != PortOut
	}
	return direction == PortOut
}

// portDirectionOf is the direction of a node's port, by ID.
func portDirectionOf(node *Node, id string) PortDirection {
	for _, port := range node.Ports {
		if port.ID == id {
			return port.Direction
		}
	}
	return PortUndirected
}

// portLabelOf is the name of a node's port, by ID.
func portLabelOf(node *Node, id string) string {
	for _, port := range node.Ports {
		if port.ID == id {
			return port.Name
		}
	}
	return ""
}

// flowLabel is what an object flow carries: the pins it joins; a flow naming no
// pins is labelled by its name, unless synthesized. A writer drawing the pins
// themselves names them there instead, and labels the flow by its Name.
func flowLabel(flow lower.ObjectFlow, synthesized bool) string {
	label := flow.SourcePin
	if flow.TargetPin != "" {
		label = strings.TrimPrefix(label+" to "+flow.TargetPin, " to ")
	}
	return edgeLabel(flow.Name, label, synthesized)
}

// edgeName is the name an edge was declared with, "" for one declared anonymous
// or named by a migration (synthesized), which stands for an unnamed edge.
func edgeName(name string, synthesized bool) string {
	if synthesized {
		return ""
	}
	return nameText(name)
}

// actionPorts are the pins an action node declares itself: its parameters, and
// the result its value binds. An attribute with no direction is no pin.
func actionPorts(id string, features []lower.Feature, doc string) []Port {
	var ports []Port
	for _, feature := range features {
		if feature.Direction == ast.DirNone && !feature.IsResult {
			continue
		}
		ports = append(ports, Port{ID: portID(id, len(ports)), Name: nameText(feature.Name),
			Direction: portDirection(feature), Origin: nodeOrigin(doc, feature.Node)})
	}
	return ports
}

// inheritedPorts adds to a node's declared pins the directed parameters it takes
// from its type (`action provide : Provide` has Provide's `in`s and `out`s),
// each not already declared by name, in the type's member order.
func (r *Renderer) inheritedPorts(elem *symbols.Symbol, node ast.Node, id string, ports []Port) []Port {
	sym, ok := r.model.SymbolDeclaring(documentScope(elem), node)
	if !ok {
		return ports
	}
	for _, member := range r.model.MembersOf(sym) {
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || !lower.DeclaresNodeFeature(usage) || usage.Direction == ast.DirNone && !usage.IsResult {
			continue
		}
		name := nameText(member.Name)
		if slices.ContainsFunc(ports, func(port Port) bool { return port.Name == name }) {
			continue
		}
		ports = append(ports, Port{ID: portID(id, len(ports)), Name: name,
			Direction: portDirection(lower.Feature{Direction: usage.Direction, IsResult: usage.IsResult}), Origin: member.Origin()})
	}
	return ports
}

// portID identifies the i-th port of a node, under the node's own ID.
func portID(node string, i int) string {
	return node + "." + strconv.Itoa(i)
}

// portDirection is which way a pin's values flow: a result is an output.
func portDirection(feature lower.Feature) PortDirection {
	switch {
	case feature.Direction == ast.DirInOut:
		return PortInOut
	case feature.Output():
		return PortOut
	}
	return PortIn
}

// portNamed is the ID of the node's port a flow end names, "" for an end naming
// none. A pin the node takes from its type rather than declaring itself is added
// to the node as the flow finds it, in the direction the flow uses it.
func portNamed(node *Node, name string, direction PortDirection) string {
	if name == "" {
		return ""
	}
	name = nameText(name)
	for _, port := range node.Ports {
		if port.Name == name {
			return port.ID
		}
	}
	port := Port{ID: portID(node.ID, len(node.Ports)), Name: name, Direction: direction}
	node.Ports = append(node.Ports, port)
	return port.ID
}

// actionText is what heads an action node whose name the model did not give,
// which is what it does: the event an accept waits for, the message a send
// sends, the one assignment its body makes, the literal a value specification's
// result is bound to. It is "" for a node its kind heads, and for one whose type
// names what it calls.
func (r *Renderer) actionText(graph *lower.ActionGraph, node ast.Node, doc string) string {
	if accept, ok := graph.Accepts[node]; ok {
		return r.acceptText(doc, accept)
	}
	if text := r.sendText(doc, graph.Bodies[node]); text != "" {
		return text
	}
	if nodeType(node) != "" {
		return ""
	}
	if body := graph.Bodies[node]; len(body) > 0 {
		return r.assignmentText(doc, body)
	}
	if sub := graph.Subflows[node]; sub != nil && sub.Graph != nil && len(sub.Graph.Nodes) > 0 {
		return ""
	}
	return r.valueSpecification(doc, graph.Features[node])
}

// acceptText is the event an accept waits for: the signal by the name it ends
// in, else the trigger as the transition label writes it, without the keyword.
func (r *Renderer) acceptText(doc string, accept lower.Accept) string {
	if accept.SignalType != nil {
		return endName(accept.SignalType)
	}
	return strings.TrimPrefix(r.triggerLabel(doc, accept.Trigger), acceptPrefix)
}

// sendText is the message a body sends when the send is all the body does, and
// "" for a body doing anything else.
func (r *Renderer) sendText(doc string, body []lower.Statement) string {
	if len(body) != 1 {
		return ""
	}
	send, ok := body[0].(lower.Send)
	if !ok {
		return ""
	}
	return r.messageText(doc, send.Message)
}

// messageText is the message a send sends: the type it constructs by the name it
// ends in, else the expression as written.
func (r *Renderer) messageText(doc string, message ast.Node) string {
	switch m := message.(type) {
	case *ast.ConstructorExpr:
		return endName(m.Type)
	case *ast.InvocationExpr:
		if m.Operand == nil {
			return endName(m.Type)
		}
	}
	return r.nodeText(doc, message)
}

// assignmentText is the one assignment a body makes, target and value as
// written (`this.i := 1`), and "" for a body doing anything else.
func (r *Renderer) assignmentText(doc string, body []lower.Statement) string {
	if len(body) != 1 {
		return ""
	}
	assign, ok := body[0].(lower.Assign)
	if !ok {
		return ""
	}
	stmt, ok := assign.Node.(*ast.AssignmentActionNode)
	if !ok {
		return ""
	}
	target, value := r.nodeText(doc, stmt.Target), r.nodeText(doc, stmt.Value)
	if target == "" || value == "" {
		return ""
	}
	return target + " := " + value
}

// valueSpecification is the value a node standing for one binds its one output
// to: a literal as written, a reference by the bare name it ends in, else the
// output's type as a call shows its target (`: StarCoordinates`) when it binds
// none, and "" for a node with any other pins.
func (r *Renderer) valueSpecification(doc string, features []lower.Feature) string {
	var output *lower.Feature
	for i := range features {
		feature := &features[i]
		if feature.Direction == ast.DirNone && !feature.IsResult {
			continue
		}
		if !feature.Output() || output != nil {
			return ""
		}
		output = feature
	}
	if output == nil {
		return ""
	}
	value := output.Value
	if value == nil {
		if typ := nodeType(output.Node); typ != "" {
			return ": " + source.ReferenceEndNames(typ)
		}
		return ""
	}
	if qn := ast.AsQualifiedName(value); qn != nil && len(qn.Parts) > 0 {
		return qn.Parts[len(qn.Parts)-1].Text
	}
	if text := r.nodeText(doc, value); text != "" {
		return text
	}
	return literalText(value)
}

// literalText writes a literal expression as the notation does, and "" for an
// expression that is no literal.
func literalText(expr ast.Node) string {
	switch e := expr.(type) {
	case *ast.LiteralBool:
		return strconv.FormatBool(e.Value)
	case *ast.LiteralString:
		return e.Value
	case *ast.LiteralInteger:
		return e.Value
	case *ast.LiteralReal:
		return e.Value
	case *ast.LiteralInfinity:
		return "*"
	case *ast.NullExpr:
		return "null"
	case *ast.FeatureReference:
		return notationName(qualifiedText(e.Name))
	}
	return ""
}

// successionLabel is the succession's guard in brackets, then its probability;
// a succession with neither is labelled by its name, unless synthesized.
func (r *Renderer) successionLabel(edge lower.ActionEdge, edgeDoc, doc string, synthesized bool) string {
	label := ""
	if guard := edge.Guard; guard != nil {
		if text := r.nodeText(edgeDoc, guard); text != "" {
			label = "[" + text + "]"
		} else {
			label = "[guard]"
		}
	}
	if weight := edge.Probability; weight != nil {
		label = strings.TrimSpace(label + " p = " + r.nodeText(doc, weight.Expr))
	}
	return edgeLabel(edge.Name, label, synthesized)
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
		if lower.IsTerminateUsage(n) {
			return terminateKind
		}
		return n.Kind.String()
	case *ast.Definition:
		return n.Kind.String() + " def"
	}
	return "node"
}

// languageNamed reports whether a graph node's name is the language's, not the
// body's: the `start` an action's flow leaves and the `done` it ends at.
func languageNamed(node ast.Node) bool {
	switch node.(type) {
	case *ast.InitialNode, *ast.FinalNode:
		return true
	}
	return false
}

// behaviorNodeName is the name a state or action graph node was declared with.
func behaviorNodeName(node ast.Node) string {
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
