package view

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// performerKeyword heads a swimlane's name compartment, as the figure of the
// perform-actions swimlanes writes it (SysML v2 §8.2.3.17).
const performerKeyword = "performer"

// performActionKind heads a perform action usage drawn with the flow it performs («perform action»).
const performActionKind = "perform action"

// performActionsLabel is the line under a swimlane's name, heading the actions
// its performer performs.
const performActionsLabel = "perform actions"

// performedByCompartment lists the performers of an action no swimlane holds
// (§8.2.3.17 performed-by-compartment).
const performedByCompartment = "performed by"

// performedByRowKind is the kind of a row of the `performed by` compartment.
const performedByRowKind = "performer"

// performedAction is the action a perform action usage performs without
// declaring a flow of its own: the action its reference subsetting names
// (`perform a.b;`, SysML v2 §7.17.6), nil for a usage that declares its own
// body or names no action.
func (r *Renderer) performedAction(sym *symbols.Symbol) *symbols.Symbol {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !usage.IsPerformedAction() || usage.HasBody {
		return nil
	}
	target := r.model.ReferencedFeature(sym)
	if target == nil || !actionOwnerKind(target.Kind) {
		return nil
	}
	return target
}

// performersOf are the elements the model states perform action, in declaration
// order: the owner of each perform action usage that names it, the action's own
// flow left out.
func (r *Renderer) performersOf(action *symbols.Symbol) []*symbols.Symbol {
	if action == nil {
		return nil
	}
	if r.performers == nil {
		r.performers = map[symbols.ElementKey][]*symbols.Symbol{}
		for _, sym := range r.modelElements() {
			usage, ok := sym.Decl.(*ast.Usage)
			if !ok || !usage.IsPerformedAction() {
				continue
			}
			target, performer := r.model.ReferencedFeature(sym), sym.Owner()
			if target == nil || performer == nil || !performerNodeKind(performer.Kind) || flowMember(performer, target) {
				continue
			}
			key := symbols.KeyOf(target)
			if !containsElement(r.performers[key], performer) {
				r.performers[key] = append(r.performers[key], performer)
			}
		}
	}
	return r.performers[symbols.KeyOf(action)]
}

// flowMember reports whether performer is declared in the flow action is a node
// of: a node there performing its sibling is a step of the flow, no performer of it.
func flowMember(performer, action *symbols.Symbol) bool {
	flow := action.Owner()
	return flow != nil && actionOwnerKind(flow.Kind) && within(performer, flow)
}

// performerNodeKind reports whether an element of the kind is a performer the
// notation draws (§8.2.3.17 performer-node): a part or an action, usage or definition.
func performerNodeKind(kind symbols.SymbolKind) bool {
	return kind == symbols.SymbolPartDef || kind == symbols.SymbolPartUsage || actionOwnerKind(kind)
}

// within reports whether sym is owner or declared, at any depth, in it.
func within(sym, owner *symbols.Symbol) bool {
	if owner == nil {
		return false
	}
	for ; sym != nil; sym = sym.Owner() {
		if symbols.KeyOf(sym) == symbols.KeyOf(owner) {
			return true
		}
	}
	return false
}

// containsElement reports whether list holds the element sym was built from.
func containsElement(list []*symbols.Symbol, sym *symbols.Symbol) bool {
	for _, each := range list {
		if symbols.KeyOf(each) == symbols.KeyOf(sym) {
			return true
		}
	}
	return false
}

// performersIn are the performers of action a flow drawn for context shows:
// all of them for an action drawn on its own, and only context and the elements
// nested in it for the action a perform action usage of context performs.
func (r *Renderer) performersIn(action, context *symbols.Symbol) []*symbols.Symbol {
	all := r.performersOf(action)
	if context == nil {
		return all
	}
	var out []*symbols.Symbol
	for _, performer := range all {
		if within(performer, context) {
			out = append(out, performer)
		}
	}
	return out
}

// performerName is a performer as a swimlane names it: the feature chain from
// its outermost owning usage (`part0.part1`), a definition by its own name.
func performerName(sym *symbols.Symbol) string {
	var names []string
	for each := sym; each != nil && each.Named(); each = each.Owner() {
		names = append([]string{localName(each)}, names...)
		if _, usage := each.Decl.(*ast.Usage); !usage {
			if each != sym {
				names = names[1:]
			}
			break
		}
	}
	return strings.Join(names, ".")
}

// laneNodes moves each node of a flow one performer performs into that
// performer's swimlane, the lanes placed where their first node stood, and
// lists on a node several performers perform all of them in its `performed by`
// compartment, as one lane cannot hold it. symbols gives each node's element.
func (r *Renderer) laneNodes(frame *Node, from int, elements map[*Node]*symbols.Symbol, context *symbols.Symbol, ids *nodeIDs) {
	lanes := map[symbols.ElementKey]*Node{}
	kept := frame.Children[:from:from]
	for _, child := range frame.Children[from:] {
		performers := r.performersIn(elements[child], context)
		switch len(performers) {
		case 0:
			kept = append(kept, child)
		case 1:
			key := symbols.KeyOf(performers[0])
			lane := lanes[key]
			if lane == nil {
				lane = r.laneNode(performers[0], ids)
				lanes[key] = lane
				kept = append(kept, lane)
			}
			lane.Children = append(lane.Children, child)
		default:
			child.Compartments = append(child.Compartments, r.performedBy(performers, ids))
			kept = append(kept, child)
		}
	}
	frame.Children = kept
}

// performerLabel names a performer in a swimlane or a `performed by` row: its
// feature chain, or the typing of one declared without a name (`: Robot`).
func (r *Renderer) performerLabel(performer *symbols.Symbol) string {
	if name := performerName(performer); name != "" {
		return name
	}
	if typed := r.headType(performer); typed != "" {
		return ": " + typed
	}
	return ""
}

// laneNode is the swimlane of one performer, headed by its name.
func (r *Renderer) laneNode(performer *symbols.Symbol, ids *nodeIDs) *Node {
	return &Node{ID: ids.take(), Kind: swimlaneKind, Name: r.performerLabel(performer),
		Detail: performActionsLabel, Origin: nodeOrigin(performer.DocName, performer.Decl)}
}

// performedBy is the `performed by` compartment naming performers.
func (r *Renderer) performedBy(performers []*symbols.Symbol, ids *nodeIDs) Compartment {
	compartment := Compartment{Name: performedByCompartment}
	for _, performer := range performers {
		compartment.Rows = append(compartment.Rows, &Node{ID: ids.take(), Kind: performedByRowKind, Name: r.performerLabel(performer),
			Origin: nodeOrigin(performer.DocName, performer.Decl)})
	}
	return compartment
}
