package view

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// treeParameter reports whether member is a parameter of owner — a directed
// usage of a calculation, action or constraint definition or usage — which a
// tree draws as a pin on its owner's border, where the graphical notation sets
// a calc's, action's and constraint's parameters (SysML v2.1 §8.2.3.17,
// §8.2.3.19, §8.2.3.20), rather than as a member of its own.
func treeParameter(owner, member *symbols.Symbol) bool {
	switch owner.Kind {
	case symbols.SymbolCalcDef, symbols.SymbolCalcUsage, symbols.SymbolActionDef, symbols.SymbolActionUsage,
		symbols.SymbolConstraintDef, symbols.SymbolConstraintUsage:
	default:
		return false
	}
	return member.IsFeature() && pinDirection(member) != PortUndirected
}

// pinParameter draws member as a parameter pin of node, which stands for owner.
// Its Layout and Notes are the ones naming it as owner's member (`about
// A::x`), or the declaring owner's, so a note about one usage's inherited
// parameter does not reach another's; a note is anchored to the node the pin
// sits on, as a note at a pin a display leaves out is.
func (r *Renderer) pinParameter(view *symbols.Symbol, node *Node, owner, member *symbols.Symbol, out *Rendering) {
	name := member.Name
	port := Port{ID: portID(node.ID, len(node.Ports)), Name: r.parameterName(member), NameSynthesized: r.model.NameSynthesized(member),
		Type: r.headType(member), Direction: pinDirection(member), Parameter: true, Origin: symbolOrigin(member), Inherited: member.Owner() != owner}
	if name == "" {
		port.Geometry = r.geometryOf(view, member, out)
		r.notesOf(view, member, node.ID, out)
	} else {
		port.Geometry = r.memberGeometryOf(view, owner, name, out)
		r.memberNotesOf(view, owner, name, node.ID, out)
	}
	node.Ports = append(node.Ports, port)
}

// parameterName is the name a parameter pin is drawn by: the declared one, or
// the effective name an unnamed parameter takes from the feature it redefines;
// a `return` declaring no name redefines the `result` every function has
// (KerML §7.4.6, SysML §7.19.2) and is named by it.
func (r *Renderer) parameterName(sym *symbols.Symbol) string {
	if name := localName(sym); name != "" {
		return name
	}
	if name := nameText(r.model.EffectiveNameOf(sym)); name != "" {
		return name
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.IsResult {
		return "result"
	}
	return ""
}

// parameterPinLine is a parameter pin as the text form and the label lines
// write it, direction first: `in x : Integer`, `out ^result : Real` for one the
// owner has from its type.
func parameterPinLine(port Port) string {
	return strings.TrimSpace(port.Direction.String() + " " + port.label())
}
