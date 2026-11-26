package view

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
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
	drawn := map[symbols.ElementKey]*Node{}
	var others []*symbols.Symbol
	for _, elem := range exposed {
		if elem.Kind != symbols.SymbolStateDef && elem.Kind != symbols.SymbolStateUsage {
			others = append(others, elem)
			continue
		}
		if node := r.renderState(view, elem, ids, out, recordDrawn(drawn)); node != nil {
			out.Roots = append(out.Roots, node)
		}
	}
	used := r.exposedRelationships(view, exposed, ids, drawn, out)
	for _, elem := range others {
		if !used[elem] {
			out.Notices = append(out.Notices, notShown(r, elem, "state machine", "state"))
		}
	}
}

// recordDrawn records, in drawn, the first node drawn for each element.
func recordDrawn(drawn map[symbols.ElementKey]*Node) func(*symbols.Symbol, *Node) {
	return func(sym *symbols.Symbol, node *Node) {
		if sym == nil || node == nil {
			return
		}
		if _, ok := drawn[symbols.KeyOf(sym)]; !ok {
			drawn[symbols.KeyOf(sym)] = node
		}
	}
}

// notShown is the notice for an exposed element a rendering of a kind does not draw.
func notShown(r *Renderer, elem *symbols.Symbol, what, rendering string) string {
	return fmt.Sprintf("%s %s is no %s; %s %s rendering does not show it", declKind(elem), r.notationName(elem), what, diag.IndefiniteArticle(rendering), rendering)
}

// rootsByOwner groups the roots whose kind isRoot by the nearest owner (kind isOwner) declaring
// them, keeping only undrawn owners of two or more such roots, whose body the roots are drawn apart from.
func rootsByOwner(roots map[*symbols.Symbol]*Node, isRoot, isOwner func(symbols.SymbolKind) bool) map[*symbols.Symbol]map[*symbols.Symbol]*Node {
	out := map[*symbols.Symbol]map[*symbols.Symbol]*Node{}
	for sym, node := range roots {
		if !isRoot(sym.Kind) {
			continue
		}
		owner := behaviorOwnerWhere(sym, isOwner)
		if owner == nil || roots[owner] != nil {
			continue
		}
		if out[owner] == nil {
			out[owner] = map[*symbols.Symbol]*Node{}
		}
		out[owner][sym] = node
	}
	for owner, group := range out {
		if len(group) < 2 {
			delete(out, owner)
		}
	}
	return out
}

// renderState builds one state machine or state node with shared IDs.
func (r *Renderer) renderState(view, elem *symbols.Symbol, ids *nodeIDs, out *Rendering,
	record func(*symbols.Symbol, *Node)) *Node {
	if elem.Kind != symbols.SymbolStateDef && elem.Kind != symbols.SymbolStateUsage {
		out.Notices = append(out.Notices, notShown(r, elem, "state machine", "state"))
		return nil
	}
	graph, err := lower.ToStateGraphWithEndpoints(elem.Decl, declScope(elem), lower.NewLibraryStateTypes(r.resolver))
	if err != nil {
		out.Notices = append(out.Notices, fmt.Sprintf("%s %s does not lower to a state graph: %v",
			declKind(elem), r.notationName(elem), err))
		return nil
	}
	return r.stateMachineNode(view, elem, graph, ids, out, record)
}

// stateMachineNode renders one lowered state machine: its regions and states as
// nested nodes, the start of each body with the entry transitions out of it, and
// its transitions as edges.
// stateMachineNode lowers one state graph into nodes and transition edges.
func (r *Renderer) stateMachineNode(view, machine *symbols.Symbol, graph *lower.StateGraph, ids *nodeIDs, out *Rendering,
	record func(*symbols.Symbol, *Node)) *Node {
	root := &Node{ID: ids.take(), Kind: declKind(machine), Name: r.notationName(machine), NameSynthesized: r.model.NameSynthesized(machine),
		Type: r.headType(machine), Origin: symbolOrigin(machine), InheritedFrom: inheritedOrigins(graph.Inherited()), Geometry: r.geometryOf(view, machine, out),
		Parallel: graph.MachineParallel}
	r.dress(view, machine, root, out)
	root.Compartments = append(root.Compartments, r.stateActionCompartments(machine.DocName, graph.BehaviorsOf(graph.MachineID()), ids)...)
	root.Children = append(root.Children, r.stateActionNodes(machine, machine.DocName, graph, graph.BehaviorsOf(graph.MachineID()), ids)...)
	if record != nil {
		record(machine, root)
	}
	nodes := map[program.VertexRef]*Node{}
	regions := map[program.RegionID]*Node{}
	place := func(node *Node, owner stateBodyRef, decl ast.Node, name string) *Node {
		body := r.bodySymbol(machine, graph, owner)
		node.Geometry = r.nodeGeometryOf(view, machine, body, decl, name, out)
		node.NameSynthesized = node.NameSynthesized || r.declaredNameSynthesized(machine, decl)
		node.Inherited = inheritedMember(r.declaredSymbol(machine, decl), body)
		return r.declaredDress(view, machine, decl, node, out)
	}
	placeRegion := func(id program.RegionID, owner program.StateID) *Node {
		decl := graph.RegionDecl(id)
		node := place(r.regionNode(id, graph, machine, ids), stateBodyRef{state: owner}, decl, graph.Provenance.Region(id).Name)
		if record != nil {
			record(r.declaredSymbol(machine, decl), node)
		}
		regions[id] = node
		return node
	}

	// Regions first: a state of an orthogonal region is nested in that region,
	// and the region order is the order the machine enters and exits them in.
	for _, region := range graph.TopRegionIDs() {
		root.Children = append(root.Children, placeRegion(region, 0))
	}
	states := graph.StateIDs()
	for _, state := range states {
		decl := graph.StateDecl(state)
		node := place(r.stateNode(state, graph, machine, ids), bodyOwning(graph, state), decl, graph.Provenance.State(state).Name)
		if record != nil {
			record(r.declaredSymbol(machine, decl), node)
		}
		if graph.Completing(state) {
			body := r.bodySymbol(machine, graph, bodyOwning(graph, state))
			node.Member = ast.DoneFeature
			node.Geometry = r.memberGeometryOf(view, body, ast.DoneFeature, out)
			r.memberDress(view, body, ast.DoneFeature, node, out)
		}
		nodes[program.StateVertex(state)] = node
		for _, region := range graph.RegionsOf(state) {
			node.Children = append(node.Children, placeRegion(region, state))
		}
	}
	// Then placement, so that a nested state reaches the node of its owner.
	for _, state := range states {
		parent := root
		if region := regions[graph.StateRegion(state)]; region != nil {
			parent = region
		} else if owner := nodes[program.StateVertex(graph.ParentOf(state))]; owner != nil {
			parent = owner
		}
		parent.Children = append(parent.Children, nodes[program.StateVertex(state)])
	}
	pseudos := graph.PseudoIDs()
	for _, id := range pseudos {
		pseudo, owner := graph.Provenance.Pseudostate(id), graph.PseudostateOwnerOf(id)
		ref := program.PseudoVertex(id)
		node := place(&Node{ID: ids.take(), Kind: pseudo.Kind.String(), Name: nameText(pseudo.Name),
			Origin: nodeOrigin(docOr(graph.VertexDoc(ref), machine.DocName), pseudo)}, stateBodyRef{state: owner, unowned: owner == 0}, pseudo, pseudo.Name)
		if record != nil {
			record(r.declaredSymbol(machine, pseudo), node)
		}
		nodes[ref] = node
		parent := root
		if owner := nodes[program.StateVertex(owner)]; owner != nil {
			parent = owner
		}
		parent.Children = append(parent.Children, node)
	}

	// Each body that says where it starts gets a start node, and one edge per
	// entry transition, in the order the guards are tried in.
	bodies := []stateBody{{stateBodyRef{}, root}}
	for _, region := range graph.TopRegionIDs() {
		bodies = append(bodies, stateBody{stateBodyRef{region: region}, regions[region]})
	}
	for _, state := range states {
		bodies = append(bodies, stateBody{stateBodyRef{state: state}, nodes[program.StateVertex(state)]})
		for _, region := range graph.RegionsOf(state) {
			bodies = append(bodies, stateBody{stateBodyRef{region: region}, regions[region]})
		}
	}
	for _, body := range bodies {
		r.entryEdges(view, machine, graph, body, nodes, ids, out)
	}
	// A composite state — one with substates, pseudostates or a start — draws its
	// entry, do and exit actions as nodes under the compartment heading its view.
	for _, state := range states {
		if node := nodes[program.StateVertex(state)]; len(node.Children) > 0 {
			node.Children = append(r.stateActionNodes(machine, node.Origin.Doc, graph, graph.BehaviorsOf(state), ids), node.Children...)
			node.Compartments = append(node.Compartments, Compartment{Name: stateTransitionCompartment})
		}
	}
	if len(root.Children) > 0 {
		root.Compartments = append(root.Compartments, Compartment{Name: stateTransitionCompartment})
	}

	for _, state := range states {
		r.transitionEdges(view, machine, graph, program.StateVertex(state), nodes, out)
	}
	for _, pseudo := range pseudos {
		r.transitionEdges(view, machine, graph, program.PseudoVertex(pseudo), nodes, out)
	}
	if len(root.Children) == 0 {
		root.Detail = detailWith(root.Detail, "declares no states")
	}
	out.folds = append(out.folds, root)
	return root
}

// entryEdges gives a body that says where it starts a start node, and one edge
// per entry transition, in the order the guards are tried in.
func (r *Renderer) entryEdges(view, machine *symbols.Symbol, graph *lower.StateGraph, body stateBody,
	nodes map[program.VertexRef]*Node, ids *nodeIDs, out *Rendering) {
	entries := graph.StartOfState(body.owner.state)
	if body.owner.region != 0 {
		entries = graph.RegionEntryTransitions(body.owner.region)
	}
	if len(entries) == 0 {
		return
	}
	bodySym := r.bodySymbol(machine, graph, body.owner)
	start := &Node{ID: ids.take(), Kind: startKind, Member: ast.StartFeature, Geometry: r.memberGeometryOf(view, bodySym, ast.StartFeature, out)}
	r.memberDress(view, bodySym, ast.StartFeature, start, out)
	body.node.Children = append([]*Node{start}, body.node.Children...)
	for _, entry := range entries {
		ref, name := entry.TargetVertex()
		target, ok := nodes[ref]
		if !ok {
			out.Notices = append(out.Notices, fmt.Sprintf("entry transition to %s of %s leaves the machine's own states; no edge is drawn",
				name, r.notationName(machine)))
			continue
		}
		doc, decl := docOr(graph.EntryDoc(entry), machine.DocName), entry.Declared()
		label := r.guardLabel(doc, entry.GuardAt())
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
			Origin: nodeOrigin(doc, decl), Route: r.declaredRouteOf(view, machine, decl, out),
			Style: r.declaredEdgeDress(view, machine, decl, start.ID, target.ID, out),
		})
	}
}

// transitionEdges adds an edge for each transition leaving src, reporting one
// whose target the machine's own states do not include.
func (r *Renderer) transitionEdges(view, machine *symbols.Symbol, graph *lower.StateGraph, src program.VertexRef,
	nodes map[program.VertexRef]*Node, out *Rendering) {
	for _, transition := range graph.TransitionsOf(src) {
		target, ok := nodes[transition.TargetRef]
		if !ok {
			out.Notices = append(out.Notices, fmt.Sprintf("transition %s of %s leaves the machine's own states; no edge is drawn",
				transitionName(transition), r.notationName(machine)))
			continue
		}
		doc, decl := docOr(graph.TransitionDoc(transition), machine.DocName), transition.Declared()
		out.Edges = append(out.Edges, Edge{
			From: nodes[src].ID, To: target.ID, Label: r.transitionLabel(doc, transition, r.declaredNameSynthesized(machine, decl)),
			Kind: EdgeTransition, Origin: nodeOrigin(doc, decl), Route: r.declaredRouteOf(view, machine, decl, out),
			Style: r.declaredEdgeDress(view, machine, decl, nodes[src].ID, target.ID, out),
		})
	}
}

// terminateKind is the Kind of a terminate action usage (`action final terminate;`),
// the activity's final node, drawn as the final symbol when a box places it.
const terminateKind = "terminate action"

// startKind is the Kind of the node a body's entry transitions leave, which a
// state diagram draws as its start marker.
const startKind = "start"

// stateBodyRef names a body by id: a region, else a state, else the machine's own.
type stateBodyRef struct {
	state  program.StateID
	region program.RegionID
	// unowned marks a pseudostate of the machine's own body, which is placed
	// under no element rather than under the machine.
	unowned bool
}

// stateBody is a body whose entry transitions say where it starts — the machine,
// a composite state or a region — and the node rendering it.
type stateBody struct {
	owner stateBodyRef
	node  *Node
}

// bodySymbol is the element whose body owner names: the machine itself, else the
// state or region declared under it; nil when owner declares no element.
func (r *Renderer) bodySymbol(machine *symbols.Symbol, graph *lower.StateGraph, owner stateBodyRef) *symbols.Symbol {
	switch {
	case owner.region != 0:
		return r.declaredSymbol(machine, graph.RegionDecl(owner.region))
	case owner.state != 0:
		return r.declaredSymbol(machine, graph.StateDecl(owner.state))
	case owner.unowned:
		return nil
	}
	return machine
}

// docOfer is a lowered graph that knows which document each of its
// declarations was written in.
type docOfer interface {
	DocOf(decl ast.Node) string
}

// docOf is the document a graph declaration was written in, else the document of
// the behavior drawn, for a graph lowered outside any document.
func docOf(graph docOfer, decl ast.Node, fallback string) string {
	return docOr(graph.DocOf(decl), fallback)
}

// docOr is doc, else the document of the behavior drawn, for a graph lowered
// outside any document.
func docOr(doc, fallback string) string {
	if doc != "" {
		return doc
	}
	return fallback
}

// regionNode renders one orthogonal region, which holds the states declared in
// it.
func (r *Renderer) regionNode(id program.RegionID, graph *lower.StateGraph, machine *symbols.Symbol, ids *nodeIDs) *Node {
	region := graph.Provenance.Region(id)
	return &Node{ID: ids.take(), Kind: "region", Name: nameText(region.Name),
		Origin: nodeOrigin(docOr(graph.RegionDoc(id), machine.DocName), region)}
}

// stateNode renders one state with what the machine says about it: whether its
// body unconditionally starts in it, what it runs as `do / Activity`, what it
// defers. The `done` vertex a body's completion synthesizes is its final node.
func (r *Renderer) stateNode(id program.StateID, graph *lower.StateGraph, machine *symbols.Symbol, ids *nodeIDs) *Node {
	state := graph.Provenance.State(id)
	node := &Node{ID: ids.take(), Kind: "state", Name: nameText(state.Name), Type: nodeType(graph.StateDecl(id)),
		Origin: nodeOrigin(docOr(graph.VertexDoc(program.StateVertex(id)), machine.DocName), state)}
	if graph.Completing(id) {
		node.Kind, node.Name, node.Type, node.NameSynthesized = "final", ast.DoneFeature, "", true
		return node
	}
	node.Parallel = graph.ParallelState(id)
	if graph.StartsBody(id) {
		node.Detail = "initial"
	}
	node.Compartments = r.stateActionCompartments(node.Origin.Doc, graph.BehaviorsOf(id), ids)
	return node
}

// The kinds of a state's entry, do and exit action nodes (§8.2.3.18
// entry-action, do-action, exit-action), as their name compartments key them.
const (
	entryActionKind = "entry action"
	doActionKind    = "do action"
	exitActionKind  = "exit action"
)

// stateActionKinds is the node kind of each state action compartment's actions.
var stateActionKinds = map[string]string{"entry": entryActionKind, "do": doActionKind, "exit": exitActionKind}

// stateActionParts are a state's behaviors by compartment, in the figure's
// order: `entry`, `do`, `exit`.
func stateActionParts(behaviors *lower.StateBehaviors) []struct {
	name string
	list []lower.StateBehavior
} {
	if behaviors == nil {
		return nil
	}
	return []struct {
		name string
		list []lower.StateBehavior
	}{{"entry", behaviors.Entry}, {"do", behaviors.Do}, {"exit", behaviors.Exit}}
}

// stateActionCompartments are a state's `entry`, `do` and `exit` compartments
// (§8.2.3.18 state-entry-action, state-do-action, state-exit-action), each
// listing the behaviors it runs as written, one a row; none for a state with none.
func (r *Renderer) stateActionCompartments(doc string, behaviors *lower.StateBehaviors, ids *nodeIDs) []Compartment {
	var compartments []Compartment
	for _, part := range stateActionParts(behaviors) {
		if len(part.list) == 0 {
			continue
		}
		var rows []*Node
		for _, behavior := range part.list {
			rows = append(rows, &Node{ID: ids.take(), Kind: statementRowKind, Name: nameText(behavior.Name), Type: nodeType(behavior.Declared()),
				Text: r.stateActionText(doc, behavior), Origin: nodeOrigin(doc, behavior.Declared())})
		}
		compartments = append(compartments, Compartment{Name: part.name, Rows: rows})
	}
	return compartments
}

// stateActionText is a state behavior as its compartment lists it: the action
// usage it declares (`action a : Reset`), else what its body does, else the
// declaration as written.
func (r *Renderer) stateActionText(doc string, behavior lower.StateBehavior) string {
	if usage, ok := behavior.Declared().(*ast.Usage); ok && (behavior.Name != "" || nodeType(behavior.Declared()) != "") {
		text := "action"
		if behavior.Name != "" {
			text += " " + nameText(behavior.Name)
		}
		if typ := nodeType(usage); typ != "" {
			text += " : " + source.ReferenceEndNames(typ)
		}
		return text
	}
	if text := r.behaviorText(doc, behavior); text != "" {
		return text
	}
	return strings.TrimSuffix(strings.TrimSpace(r.nodeText(doc, behavior.Declared())), ";")
}

// stateActionNodes are the entry, do and exit action nodes a composite state
// draws among its substates: each behavior declared as a named or bodied
// action usage, headed «entry action» and so on, with its pins.
func (r *Renderer) stateActionNodes(machine *symbols.Symbol, doc string, graph *lower.StateGraph, behaviors *lower.StateBehaviors, ids *nodeIDs) []*Node {
	var nodes []*Node
	for _, part := range stateActionParts(behaviors) {
		for _, behavior := range part.list {
			usage, ok := behavior.Declared().(*ast.Usage)
			if !ok || behavior.Name == "" && !usage.HasBody {
				continue
			}
			node := &Node{ID: ids.take(), Kind: stateActionKinds[part.name], Name: nameText(behavior.Name), Type: nodeType(usage),
				Origin: nodeOrigin(doc, behavior.Declared())}
			if node.Name == "" {
				node.Text = r.behaviorText(doc, behavior)
			}
			node.Ports = r.inheritedPorts(machine, usage, node.ID, actionPorts(node.ID, graph.BehaviorFeatures(behavior), doc), declaresResult(graph.BehaviorFeatures(behavior)))
			nodes = append(nodes, node)
		}
	}
	return nodes
}

const acceptPrefix = "accept "

// bodyOwning is the body a state is declared in, as StateGraph.StartOf names it:
// its region, else its parent state, else nil for the machine's own body.
func bodyOwning(graph *lower.StateGraph, state program.StateID) stateBodyRef {
	if region := graph.StateRegion(state); region != 0 {
		return stateBodyRef{region: region}
	}
	return stateBodyRef{state: graph.ParentOf(state)}
}

// transitionLabel is a transition edge's trigger, guard and effect, `after 5 [ok] /
// effect`; a transition with none of those is labelled by its name, unless synthesized.
func (r *Renderer) transitionLabel(doc string, transition *lower.Transition, synthesized bool) string {
	var parts []string
	if trigger := r.triggerLabel(doc, transition.TriggerWritten); trigger != "" {
		parts = append(parts, trigger)
	}
	if transition.Via != "" {
		parts = append(parts, "via "+notationName(transition.Via))
	}
	if guard := r.guardLabel(doc, transition.GuardAt()); guard != "" {
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
func (r *Renderer) guardLabel(doc string, guard lower.Extent) string {
	if !guard.Written {
		return ""
	}
	if text := r.extentText(doc, guard); text != "" {
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
	source, target := transition.EndNames()
	return fmt.Sprintf("first %s then %s", source, target)
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
	if typ := nodeType(behavior.Declared()); typ != "" {
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

// triggerLabel is the event a transition waits for: an accepted signal or called
// operation by the name it ends in, as a type is; a time or change event as
// written when the source is at hand; else what kind of event it is.
func (r *Renderer) triggerLabel(doc string, written func() (lower.Trigger, bool)) string {
	trigger, ok := written()
	if !ok {
		return ""
	}
	switch {
	case trigger.Kind == "accept" && trigger.Typed:
		return joinNonEmpty(joinNonEmpty("accept", payloadHead(trigger.PayloadName)), segmentText(trigger.Signal))
	case trigger.Kind == "call" && trigger.Calls:
		return acceptPrefix + segmentText(trigger.Called) + callParameters(trigger.Parameters)
	}
	if text := r.extentText(doc, lower.Extent{Span: trigger.Span, Written: true}); text != "" {
		return text
	}
	switch trigger.Kind {
	case "time":
		keyword := "after"
		if trigger.Absolute {
			keyword = "at"
		}
		return joinNonEmpty(keyword, r.extentText(doc, trigger.Expr))
	case "change":
		return joinNonEmpty("when", r.extentText(doc, trigger.Expr))
	case "accept":
		if trigger.Subsetting {
			return "accept :> " + trigger.Subset
		}
		return "accept event"
	case "call":
		return "call event"
	}
	return "event"
}

// payloadHead is the `name :` an accept trigger declares its payload by.
func payloadHead(name string) string {
	if name == "" {
		return ""
	}
	return nameText(name) + " :"
}

// endName is the last segment of a qualified name, as a label shows it.
func endName(name *ast.QualifiedName) string {
	if name == nil || len(name.Parts) == 0 {
		return ""
	}
	return nameText(name.Parts[len(name.Parts)-1].Text)
}

// segmentText is the last segment of a name as a label shows it, "" for none.
func segmentText(segment string) string {
	if segment == "" {
		return ""
	}
	return nameText(segment)
}

// callParameters are a call trigger's parameter names, parenthesized.
func callParameters(parameters []string) string {
	names := make([]string, len(parameters))
	for i, parameter := range parameters {
		names[i] = nameText(parameter)
	}
	return "(" + strings.Join(names, ", ") + ")"
}

// extentText is the collapsed source text of an extent, "" for none.
func (r *Renderer) extentText(doc string, at lower.Extent) string {
	if r.text == nil || !at.Written || doc == "" || at.Span.Len <= 0 {
		return ""
	}
	return collapseSpace(r.text(doc, at.Span))
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
	var b strings.Builder
	var quote rune
	space := false
	for i, r := range text {
		switch {
		case quote != 0:
			if r == quote && !escaped(text, i) {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case unicode.IsSpace(r):
			space = b.Len() > 0
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// escaped reports whether the character at i in text is preceded by an odd run
// of backslashes, which escapes it.
func escaped(text string, i int) bool {
	n := 0
	for i--; i >= 0 && text[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

// renderActions renders the nodes and successions of the exposed behaviors, from
// the lowered ActionGraph of each. Nodes and edges are placed by the Layout and
// Route annotations positioning their declarations in view, nil for a rendering
// outside any view.
func (r *Renderer) renderActions(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	ids := &nodeIDs{}
	drawn := map[symbols.ElementKey]*Node{}
	var others []*symbols.Symbol
	roots := map[*symbols.Symbol]*Node{}
	for _, elem := range exposed {
		if !behaviorKind(elem.Kind) {
			others = append(others, elem)
			continue
		}
		if node, ok := r.renderAction(view, elem, ids, out, recordDrawn(drawn)); ok {
			out.Roots = append(out.Roots, node)
			roots[elem] = node
		}
	}
	r.rootSuccessionEdges(view, roots, out)
	used := r.exposedRelationships(view, exposed, ids, drawn, out)
	for _, elem := range others {
		if !used[elem] {
			out.Notices = append(out.Notices, notShown(r, elem, "action", "action"))
		}
	}
}

// rootSuccessionEdges draws the successions of one action between actions of
// it exposed as roots of their own, read from the action's lowered graph and
// labelled as actionEdges labels them within the action.
func (r *Renderer) rootSuccessionEdges(view *symbols.Symbol, roots map[*symbols.Symbol]*Node, out *Rendering) {
	for owner, actions := range rootsByOwner(roots, symbols.SymbolKind.IsActionUsage, actionOwnerKind) {
		graph, err := lower.ToActionGraphWith(owner.Decl, declScope(owner), r.resolver)
		if err != nil {
			continue
		}
		// A node inherited from another document is looked up in that document:
		// its span could equally be a declaration of the owner's own.
		actionOf := func(node ast.Node) *Node {
			doc := docOf(graph, node, owner.DocName)
			root := r.documentRoot(doc)
			if root == nil {
				return nil
			}
			return actions[root.DeclaredAt(nodeOrigin(doc, node).Span)]
		}
		for _, src := range graph.NodeIDs() {
			for _, edge := range graph.EdgesOf(src) {
				from, to := actionOf(graph.Provenance.Node(src)), actionOf(graph.Provenance.Node(edge.TargetID))
				if from == nil || to == nil || from == to {
					continue
				}
				decl := edge.Declared()
				edgeDoc := docOf(graph, decl, owner.DocName)
				out.Edges = append(out.Edges, Edge{From: from.ID, To: to.ID, Kind: EdgeSuccession,
					Label:  r.successionLabel(edge, edgeDoc, owner.DocName, r.declaredNameSynthesized(owner, decl)),
					Origin: nodeOrigin(edgeDoc, decl), Route: r.declaredRouteOf(view, owner, decl, out),
					Style: r.declaredEdgeDress(view, owner, decl, from.ID, to.ID, out)})
			}
		}
	}
}

// actionOwnerKind reports whether an element of the kind owns the successions of the actions
// in its body: an action def or an action usage of any node kind, whose body the ActionGraph orders.
func actionOwnerKind(kind symbols.SymbolKind) bool {
	return kind == symbols.SymbolActionDef || kind.IsActionUsage()
}

// behaviorKind reports whether an element of the kind is drawn by an action
// rendering: an action def or usage, or a calculation def or usage — an action
// declared with the keyword `calc` (SysML v2 §7.19.2), drawn the same way.
func behaviorKind(kind symbols.SymbolKind) bool {
	switch kind {
	case symbols.SymbolActionDef, symbols.SymbolCalcDef, symbols.SymbolCalcUsage:
		return true
	}
	return kind.IsActionUsage()
}

// calculation reports whether a declared symbol is a calculation def or usage; a nil one is not.
func calculation(sym *symbols.Symbol) bool {
	return sym != nil && (sym.Kind == symbols.SymbolCalcDef || sym.Kind == symbols.SymbolCalcUsage)
}

// renderAction builds one action graph with shared IDs.
func (r *Renderer) renderAction(view, elem *symbols.Symbol, ids *nodeIDs, out *Rendering,
	record func(*symbols.Symbol, *Node)) (*Node, bool) {
	if !behaviorKind(elem.Kind) {
		out.Notices = append(out.Notices, notShown(r, elem, "action", "action"))
		return nil, false
	}
	subject := actionSubject{decl: elem.Decl, kind: declKind(elem), name: r.notationName(elem), typ: r.headType(elem),
		scope: declScope(elem), doc: elem.DocName, view: view, elem: elem}
	if performed := r.performedAction(elem); performed != nil {
		// A perform action usage declaring no flow shows the one it performs,
		// laned by the performers its own performer nests (§8.2.3.17).
		subject.decl, subject.scope, subject.doc, subject.elem = performed.Decl, declScope(performed), performed.DocName, performed
		subject.performing, subject.context = elem, elem.Owner()
		subject.kind = performActionKind
		if chain := performerName(subject.context); chain != "" && localName(performed) != "" {
			subject.name = chain + "." + localName(performed)
		}
	}
	return r.actionNode(subject, ids, out, map[ast.Node]bool{}, map[ast.Node]*Node{}, 0, record)
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
	// performing is the perform action usage whose performed flow is drawn and
	// context its performer, which scopes the lanes; nil for an action drawn on its own.
	performing *symbols.Symbol
	context    *symbols.Symbol
}

// actionNode renders one lowered action: its nodes as nested nodes, its
// successions, object flows and bindings as edges. A nested action declaring a
// body of its own is lowered in turn, so the rendering shows the flow within it
// as well; the node standing for it in the caller is its frame, carrying its
// nodes, geometry and notes, and its pins are the ones its bindings attach to.
// drawn collects the node drawn for each lowered node across the nesting, so a
// binding reaching into a nested flow finds its pin.
func (r *Renderer) actionNode(subject actionSubject, ids *nodeIDs, out *Rendering,
	lowered map[ast.Node]bool, drawn map[ast.Node]*Node, depth int, record func(*symbols.Symbol, *Node)) (*Node, bool) {
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
			Origin: nodeOrigin(doc, decl), InheritedFrom: inheritedOrigins(graph.Inherited())}
		root.Ports = r.inheritedPorts(subject.elem, decl, root.ID, actionPorts(root.ID, graph.Parameters, doc), declaresResult(graph.Parameters))
		root.Geometry = r.declaredGeometryOf(subject.view, subject.elem, decl, out)
		r.declaredDress(subject.view, subject.elem, decl, root, out)
		if shown := subject.performing; shown != nil {
			root.NameSynthesized, root.Origin = false, nodeOrigin(shown.DocName, shown.Decl)
			root.Geometry = r.declaredGeometryOf(subject.view, shown, shown.Decl, out)
			root.Style = nil
			r.declaredDress(subject.view, shown, shown.Decl, root, out)
		}
	}
	lowered[decl] = true
	owner := r.declaredSymbol(subject.elem, decl)
	if record != nil {
		record(subject.performing, root)
		record(owner, root)
	}
	if subject.performing != nil {
		owner = subject.performing // the nodes of the performed flow are drawn as the ones the perform action usage inherits
	}
	// Every action has a start even when no succession leaves it; draw it when
	// the view's Layout positions it, first among the body's nodes.
	if r.placedStart(subject.view, owner, graph, out) {
		start := &Node{ID: ids.take(), Kind: initialKind,
			Name: nameText("start"), NameSynthesized: true, Member: "start"}
		start.Geometry = r.memberGeometryOf(subject.view, owner, "start", out)
		r.memberDress(subject.view, owner, "start", start, out)
		if member, ok := r.model.LookupMember(owner, "start"); ok {
			start.Origin = nodeOrigin(member.DocName, member.Decl)
		}
		root.Children = append(root.Children, start)
	}
	r.flowNodes(subject, graph, root, owner, flowDrawing{ids, out, lowered, drawn, record}, depth)
	// A calculation yields a result rather than a flow: its node shows the result
	// expression, and is not noted for the nodes it does not declare.
	switch {
	case subject.frame != nil:
	case calculation(owner):
		root.Compartments = append(root.Compartments, r.resultCompartments(owner, ids)...)
	case len(graph.NodeIDs()) == 0:
		root.Detail = detailWith(root.Detail, "declares no nodes")
	}
	// An action no lane of an enclosing flow holds names who performs it.
	if subject.frame == nil && subject.performing == nil {
		if performers := r.performersOf(owner); len(performers) > 0 {
			root.Compartments = append(root.Compartments, r.performedBy(performers, ids))
		}
	}
	out.folds = append(out.folds, root)
	return root, true
}

// A flowDrawing is the state a behavior's nested flows share: the id supply,
// the rendering, the lowered declarations, the drawn nodes and the symbol recorder.
type flowDrawing struct {
	ids     *nodeIDs
	out     *Rendering
	lowered map[ast.Node]bool
	drawn   map[ast.Node]*Node
	record  func(*symbols.Symbol, *Node)
}

// flowNodes draws a lowered flow's nodes under frame, each statement node in
// the form its figure gives it, then the flow's successions, object flows and
// bindings as edges; owner is the symbol the nodes are placed under.
func (r *Renderer) flowNodes(subject actionSubject, graph *lower.ActionGraph, frame *Node, owner *symbols.Symbol, d flowDrawing, depth int) {
	ids, out, lowered, drawn, record := d.ids, d.out, d.lowered, d.drawn, d.record
	scope, doc := subject.scope, subject.doc
	nodes := map[program.NodeID]*Node{}
	elements, first := map[*Node]*symbols.Symbol{}, len(frame.Children)
	for _, id := range graph.NodeIDs() {
		node := graph.Provenance.Node(id)
		nodeDoc := docOf(graph, node, doc)
		child := &Node{ID: ids.take(), Kind: actionNodeKind(graph, id), Name: nameText(behaviorNodeName(node)),
			NameSynthesized: languageNamed(node) || r.declaredNameSynthesized(subject.elem, node), StandIn: r.declaredStandIn(subject.elem, node),
			Type: nodeType(node), Origin: nodeOrigin(nodeDoc, node)}
		child.Inherited = inheritedMember(r.declaredSymbol(subject.elem, node), owner)
		if languageNamed(node) {
			child.Member = behaviorNodeName(node)
			child.Geometry = r.memberGeometryOf(subject.view, owner, behaviorNodeName(node), out)
			r.memberDress(subject.view, owner, behaviorNodeName(node), child, out)
		} else {
			child.Geometry = r.nodeGeometryOf(subject.view, subject.elem, owner, node, behaviorNodeName(node), out)
			r.declaredDress(subject.view, subject.elem, node, child, out)
		}
		child.Ports = r.inheritedPorts(subject.elem, node, child.ID, actionPorts(child.ID, graph.FeaturesOf(id), nodeDoc), declaresResult(graph.FeaturesOf(id)))
		if record != nil {
			record(r.declaredSymbol(subject.elem, node), child)
		}
		r.statementForm(graph, id, child, nodeDoc)
		nodes[id], drawn[node] = child, child
		elements[child] = r.declaredSymbol(subject.elem, node)
		frame.Children = append(frame.Children, child)
		if nested, ok := nestedAction(node); ok && depth < maxBehaviorDepth && !lowered[node] {
			nestedScope := graph.ScopeOf(id)
			if nestedScope == nil {
				nestedScope = actionScope(scope, nested)
			}
			nestedSubject := actionSubject{decl: nested, kind: child.Kind, name: child.Name, typ: child.Type,
				scope: nestedScope, doc: nodeDoc, view: subject.view, elem: subject.elem, frame: child, context: subject.context}
			// The nested flow's own edges belong to the nested nodes, which the
			// sub-rendering adds to out.Edges; a flow with no nodes leaves nothing to show.
			if _, ok := r.actionNode(nestedSubject, ids, out, lowered, drawn, depth+1, record); ok && len(child.Children) > 0 {
				child.Compartments = append(child.Compartments, Compartment{Name: actionFlowCompartment})
			}
		}
		compartments, bodies := r.compoundCompartments(graph, id, nodeDoc, ids)
		child.Compartments = append(child.Compartments, compartments...)
		for _, body := range bodies {
			if depth >= maxBehaviorDepth {
				break
			}
			section := r.bodySection(body, ids)
			child.Children = append(child.Children, section)
			bodySubject := actionSubject{decl: body.block.Declared(), kind: section.Kind, name: child.Name, scope: body.block.Scope,
				doc: nodeDoc, view: subject.view, elem: subject.elem, frame: section, context: subject.context}
			r.flowNodes(bodySubject, body.block.Graph, section, r.declaredSymbol(subject.elem, node), d, depth+1)
		}
	}
	r.laneNodes(frame, first, elements, subject.context, ids)
	r.actionEdges(subject, graph, nodes, out)
	r.bindingEdges(subject, graph, frame, nodes, drawn, out)
}

// statementForm gives a statement node its figure's parts: the expression under
// its name compartment, or in place of a name it has none of; any other node
// headed by what it does, as before.
func (r *Renderer) statementForm(graph *lower.ActionGraph, node program.NodeID, child *Node, doc string) {
	unnamed := child.NameSynthesized || child.Name == ""
	if !statementKind(child.Kind) {
		if unnamed {
			child.Text = r.actionText(graph, node, doc)
		}
		return
	}
	expression := r.statementExpression(graph, node, doc)
	switch {
	case unnamed && expression != "":
		child.Text = expression
	case unnamed:
		child.Text = r.actionText(graph, node, doc)
	case expression != "":
		child.Detail = detailWith(child.Detail, expression)
	}
}

// placedStart reports whether view positions the start graph never draws: one
// no succession leaves lowers to no InitialNode, so a Layout about it has no
// node to hang on and is drawn as one of its own.
func (r *Renderer) placedStart(view, owner *symbols.Symbol, graph *lower.ActionGraph, out *Rendering) bool {
	if owner == nil {
		return false
	}
	for _, id := range graph.NodeIDs() {
		// A declared `action start` is an ordinary node named start: the
		// member already has a node and the Layout belongs to it.
		if node := graph.Provenance.Node(id); graph.FormOf(id) == lower.FormInitial || behaviorNodeName(node) == "start" {
			return false
		}
	}
	site, ok := r.model.MemberLayoutOf(view, owner, "start")
	if !ok {
		return false
	}
	// A Layout the declaring owner's start falls back to places that body's
	// start where it is drawn, not a start here: only one naming this body's adds one.
	if member, ok := r.model.LookupMember(owner, "start"); ok && member.Owner() != nil && member.Owner().Decl != owner.Decl {
		if via := r.model.MemberLayoutOwner(site, member); via == nil || via.Decl != owner.Decl {
			return false
		}
	}
	if site.Layout == nil || site.Layout.Unplaced {
		r.noteLayoutProblems(site, owner, out)
		return false
	}
	return true
}

// actionEdges draws the action's successions and object flows between its own
// nodes; one leaving them is noticed instead.
func (r *Renderer) actionEdges(subject actionSubject, graph *lower.ActionGraph, nodes map[program.NodeID]*Node, out *Rendering) {
	name, doc := subject.name, subject.doc
	for _, src := range graph.NodeIDs() {
		srcName := nameText(behaviorNodeName(graph.Provenance.Node(src)))
		for _, edge := range graph.EdgesOf(src) {
			if edge.Carries && !edge.Gated() {
				// The succession a succession flow states is its own glyph; one a
				// guarded succession gates keeps that succession's guard.
				continue
			}
			to, ok := nodes[edge.TargetID]
			if !ok {
				out.Notices = append(out.Notices, fmt.Sprintf("succession from %s in %s leaves the action's own nodes; no edge is drawn",
					srcName, name))
				continue
			}
			decl := edge.Declared()
			edgeDoc := docOf(graph, decl, doc)
			label := r.successionLabel(edge, edgeDoc, doc, r.declaredNameSynthesized(subject.elem, decl))
			out.Edges = append(out.Edges, Edge{From: nodes[src].ID, To: to.ID, Label: label,
				Kind: EdgeSuccession, Origin: nodeOrigin(edgeDoc, decl), Route: r.declaredRouteOf(subject.view, subject.elem, decl, out),
				Style: r.declaredEdgeDress(subject.view, subject.elem, decl, nodes[src].ID, to.ID, out)})
		}
		for _, flow := range graph.FlowsOf(src) {
			to, ok := nodes[flow.TargetID]
			if !ok {
				out.Notices = append(out.Notices, fmt.Sprintf("flow from %s in %s leaves the action's own nodes; no edge is drawn",
					srcName, name))
				continue
			}
			decl := flow.Declared()
			synthesized := r.declaredNameSynthesized(subject.elem, decl)
			kind, label := EdgeFlow, flowLabel(flow, synthesized)
			if flow.Kind == lower.FlowSuccession {
				kind, label = EdgeSuccessionFlow, successionFlowLabel(label)
			}
			out.Edges = append(out.Edges, Edge{From: nodes[src].ID, To: to.ID, Label: label, Name: edgeName(flow.Name, synthesized),
				FromPort: portNamed(nodes[src], flow.SourcePin, PortOut), ToPort: portNamed(to, flow.TargetPin, PortIn),
				Kind: kind, Origin: nodeOrigin(docOf(graph, decl, doc), decl), Route: r.declaredRouteOf(subject.view, subject.elem, decl, out),
				Style: r.declaredEdgeDress(subject.view, subject.elem, decl, nodes[src].ID, to.ID, out)})
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
func (r *Renderer) bindingEdges(subject actionSubject, graph *lower.ActionGraph, root *Node, nodes map[program.NodeID]*Node,
	drawn map[ast.Node]*Node, out *Rendering) {
	// An explicit bind between two nodes' pins lowers to an entry at each end;
	// an inout pin's value binding is in Bindings as well as ValueBindings.
	seen := map[*ast.Usage]bool{}
	for _, binding := range slices.Concat(graph.ValueBindings, graph.Bindings) {
		decl := binding.Declared()
		if seen[decl] {
			continue
		}
		seen[decl] = true
		at, atPin := bindingPin(nodes, drawn, bindingEnd{named: binding.AtNode(), at: binding.NodeID, path: binding.PathDecls(), pin: binding.Pin}, root)
		other, otherPin := bindingPin(nodes, drawn, bindingEnd{named: binding.OtherAtNode(), at: binding.OtherNodeID, path: binding.OtherPathDecls(), pin: binding.OtherPin, parameter: binding.OtherParameter}, root)
		if at == nil || other == nil {
			continue
		}
		from, fromPin, to, toPin := at, atPin, other, otherPin
		if pinGives(other, otherPin, root) && !pinGives(at, atPin, root) {
			from, fromPin, to, toPin = other, otherPin, at, atPin
		}
		synthesized := r.declaredNameSynthesized(subject.elem, decl)
		var name string
		if decl.Kind == ast.UsageBinding {
			name = decl.Ident.Name
		}
		out.Edges = append(out.Edges, Edge{From: from.ID, To: to.ID, FromPort: fromPin, ToPort: toPin,
			Label: edgeLabel(name, portLabelOf(from, fromPin)+" = "+portLabelOf(to, toPin), synthesized), Name: edgeName(name, synthesized),
			Kind: EdgeBinding, Origin: nodeOrigin(docOf(graph, decl, subject.doc), decl),
			Route: r.declaredRouteOf(subject.view, subject.elem, decl, out),
			Style: r.declaredEdgeDress(subject.view, subject.elem, decl, from.ID, to.ID, out)})
	}
}

// bindingEnd is one end of a binding: whether it names a node, that node, the path into a nested
// flow, its pin, and the frame parameter it is at otherwise.
type bindingEnd struct {
	named          bool
	at             program.NodeID
	path           []ast.Node
	pin, parameter string
}

// bindingPin is the drawn node and pin a binding end is at: the node the end
// names, reached through path where it reaches into a nested flow, else the
// frame's pin named parameter; nil for an end at no drawn pin, which no edge
// can attach to.
func bindingPin(nodes map[program.NodeID]*Node, drawn map[ast.Node]*Node, end bindingEnd, root *Node) (*Node, string) {
	named, at, path, pin, parameter := end.named, end.at, end.path, end.pin, end.parameter
	node := root
	if named {
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

// successionFlowKeyword is the text the graphical notation sets on a succession
// flow, which tells it from a flow where a form draws both alike.
const successionFlowKeyword = "«succession flow»"

// successionFlowLabel leads a succession flow's label with its keyword.
func successionFlowLabel(label string) string {
	return strings.TrimSpace(successionFlowKeyword + " " + label)
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
		ports = append(ports, Port{ID: portID(id, len(ports)), Parameter: true, Name: nameText(feature.Name),
			Direction: portDirection(feature), Origin: nodeOrigin(doc, feature.Declared())})
	}
	return ports
}

// inheritedPorts adds to a node's declared pins the directed parameters it takes
// from its type (`action provide : Provide` has Provide's `in`s and `out`s),
// each not already declared by name, in the type's member order. A result
// parameter redefines the one of the type it specializes, so the nearest one
// declared — the node's own when result is set, else the first inherited — is
// the only result pin (`return r` of a calc stands for Calculation::result).
func (r *Renderer) inheritedPorts(elem *symbols.Symbol, node ast.Node, id string, ports []Port, result bool) []Port {
	sym, ok := r.model.SymbolDeclaring(documentScope(elem), node)
	if !ok {
		return ports
	}
	for _, member := range r.model.MembersOf(sym) {
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || !lower.DeclaresNodeFeature(usage) || usage.Direction == ast.DirNone && !usage.IsResult {
			continue
		}
		if usage.IsResult {
			if result {
				continue
			}
			result = true
		}
		name := nameText(member.Name)
		if slices.ContainsFunc(ports, func(port Port) bool { return port.Name == name }) {
			continue
		}
		ports = append(ports, Port{ID: portID(id, len(ports)), Parameter: true, Name: name, Inherited: true,
			Direction: portDirection(lower.Feature{Direction: usage.Direction, IsResult: usage.IsResult}), Origin: member.Origin()})
	}
	return ports
}

// declaresResult reports whether the features hold a result parameter.
func declaresResult(features []lower.Feature) bool {
	return slices.ContainsFunc(features, func(feature lower.Feature) bool { return feature.IsResult })
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
	port := Port{ID: portID(node.ID, len(node.Ports)), Name: name, Direction: direction, Parameter: true}
	node.Ports = append(node.Ports, port)
	return port.ID
}

// actionText is what heads an action node whose name the model did not give,
// which is what it does: the event an accept waits for, the message a send
// sends, the one assignment its body makes, the literal a value specification's
// result is bound to. It is "" for a node its kind heads, and for one whose type
// names what it calls.
func (r *Renderer) actionText(graph *lower.ActionGraph, node program.NodeID, doc string) string {
	if accept, ok := graph.AcceptOf(node); ok {
		return r.acceptText(doc, accept)
	}
	body := graph.BodyOf(node)
	if text := r.sendText(doc, body); text != "" {
		return text
	}
	if nodeType(graph.Provenance.Node(node)) != "" {
		return ""
	}
	if len(body) > 0 {
		return r.assignmentText(doc, body)
	}
	if sub := graph.SubflowOf(node); sub != nil && sub.Graph != nil && len(sub.Graph.NodeIDs()) > 0 {
		return ""
	}
	return r.valueSpecification(doc, graph.FeaturesOf(node))
}

// acceptText is the event an accept waits for: the signal by the name it ends
// in, else the trigger as the transition label writes it, without the keyword.
func (r *Renderer) acceptText(doc string, accept lower.Accept) string {
	if signal, ok := accept.Signal(); ok {
		return segmentText(signal)
	}
	return strings.TrimPrefix(r.triggerLabel(doc, accept.TriggerWritten), acceptPrefix)
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
	return r.messageText(doc, send)
}

// messageText is the message a send sends: the type it constructs by the name it
// ends in, else the expression as written.
func (r *Renderer) messageText(doc string, send lower.Send) string {
	if constructed, ok := send.Constructs(); ok {
		return segmentText(constructed)
	}
	return r.extentText(doc, send.MessageAt())
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
	targetAt, valueAt, ok := assign.Sides()
	if !ok {
		return ""
	}
	target, value := r.extentText(doc, targetAt), r.extentText(doc, valueAt)
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
	if !output.Bound() {
		if typ := nodeType(output.Declared()); typ != "" {
			return ": " + source.ReferenceEndNames(typ)
		}
		return ""
	}
	if name, ok := output.ValueName(); ok {
		return name
	}
	if text := r.extentText(doc, output.ValueAt()); text != "" {
		return text
	}
	return output.ValueLiteral()
}

// successionLabel is the succession's guard in brackets, `else` for a decision's
// else branch, then its probability; one with none is labelled by its name,
// unless synthesized.
func (r *Renderer) successionLabel(edge lower.ActionEdge, edgeDoc, doc string, synthesized bool) string {
	label := ""
	if guard := edge.GuardAt(); guard.Written {
		if text := r.extentText(edgeDoc, guard); text != "" {
			label = "[" + text + "]"
		} else {
			label = "[guard]"
		}
	}
	if edge.Else && label == "" {
		label = "else"
	}
	if weight, ok := edge.WeightAt(); ok {
		label = strings.TrimSpace(label + " p = " + r.extentText(doc, weight))
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

// initialKind is the Kind of the node an action's flow starts at.
const initialKind = "initial"

// actionNodeKind names an action graph node the way the notation declares it, so
// a rendering says "fork" and "action" rather than a Go type name.
func actionNodeKind(graph *lower.ActionGraph, id program.NodeID) string {
	if graph.StatementRun(id) {
		return "statements"
	}
	switch n := graph.Provenance.Node(id).(type) {
	case *ast.InitialNode:
		return initialKind
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
		if kind := statementNodeKind(graph, id); kind != "" {
			return kind
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

// inheritedMember reports whether sym, drawn under owner, is declared by another element
// (inherited through specialization or typing); a node lowered from no declaration is owner's own.
func inheritedMember(sym, owner *symbols.Symbol) bool {
	return sym != nil && owner != nil && sym.Owner() != nil && sym.Owner() != owner
}
