package view

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// maxTreeDepth bounds how deep a containment tree is walked, so a model whose
// members reach back into their own owner still renders.
const maxTreeDepth = 16

// renderTree renders the exposed elements as a containment tree: each element
// with its kind and name, the elements declared in it beneath it, and each view
// nested in the rendered view as a subtree of its own. Each node is placed by
// the Layout that positions its element in the view it is shown under.
func (r *Renderer) renderTree(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	ids := &nodeIDs{}
	for _, elem := range exposed {
		out.Roots = append(out.Roots, r.treeNode(view, elem, ids, map[*symbols.Symbol]bool{}, 0, true, out))
	}
	out.Roots = append(out.Roots, r.nestedViewNodes(view, ids, map[*symbols.Symbol]bool{view: true}, out)...)
}

// nestedViewNodes renders the views nested in view as subtrees, each holding
// what it exposes and the views nested in it in turn.
func (r *Renderer) nestedViewNodes(view *symbols.Symbol, ids *nodeIDs, rendered map[*symbols.Symbol]bool, out *Rendering) []*Node {
	nested, err := r.model.NestedViews(view)
	if err != nil || len(nested) == 0 {
		return nil
	}
	var nodes []*Node
	for _, sub := range nested {
		if rendered[sub] {
			out.Notices = append(out.Notices, fmt.Sprintf("nested view %s is nested in itself; rendered once", r.notationName(sub)))
			continue
		}
		rendered[sub] = true
		node := &Node{ID: ids.take(), Kind: declKind(sub), Name: r.notationName(sub), Detail: declType(sub), Origin: symbolOrigin(sub),
			Geometry: r.geometryOf(view, sub, out)}
		exposed, err := r.model.ExposedElements(sub)
		if err == nil {
			for _, elem := range exposed {
				node.Children = append(node.Children, r.treeNode(sub, elem, ids, map[*symbols.Symbol]bool{}, 0, true, out))
			}
		}
		node.Children = append(node.Children, r.nestedViewNodes(sub, ids, rendered, out)...)
		if len(node.Children) == 0 {
			node.Detail = detailWith(node.Detail, "exposes nothing")
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// treeNode renders one element and what it declares. qualified names the node by
// qualified name, which is what an exposed element is reported by; a member
// nested in it is named as it was declared. view is the view the element is
// shown in, nil outside any view.
func (r *Renderer) treeNode(view, sym *symbols.Symbol, ids *nodeIDs, seen map[*symbols.Symbol]bool, depth int, qualified bool, out *Rendering) *Node {
	name := r.notationName(sym)
	if !qualified {
		name = notationName(simpleName(r.fqn(sym)))
	}
	node := &Node{ID: ids.take(), Kind: declKind(sym), Name: name, Detail: declType(sym), Origin: symbolOrigin(sym),
		Geometry: r.geometryOf(view, sym, out)}
	if seen[sym] {
		node.Detail = detailWith(node.Detail, "already shown")
		return node
	}
	if depth >= maxTreeDepth {
		if len(containedMembers(sym)) > 0 {
			node.Detail = detailWith(node.Detail, fmt.Sprintf("nested deeper than %d levels; not shown", maxTreeDepth))
		}
		return node
	}
	seen[sym] = true
	for _, member := range containedMembers(sym) {
		node.Children = append(node.Children, r.treeNode(view, member, ids, seen, depth+1, false, out))
	}
	return node
}

// containedMembers returns what an element declares in its own body, in
// declaration order: the elements a containment tree shows beneath it. A
// reference member is anonymous, so the two member lists a scope keeps are
// merged by source position rather than concatenated.
func containedMembers(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	for _, list := range [][]*symbols.Symbol{sym.Scope.Members(), sym.Scope.AnonymousMembers()} {
		for _, member := range list {
			if member == sym || seen[member] || !containedKind(member) {
				continue
			}
			seen[member] = true
			out = append(out, member)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DeclSpan.Offset < out[j].DeclSpan.Offset })
	return out
}

// containedKind reports whether a member is an element of the model a rendering
// shows, as against the annotations and bookkeeping declarations that carry no
// structure: documentation, comments, aliases and the ends of a connect clause,
// which are shown as the connection itself.
func containedKind(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolComment, symbols.SymbolDocumentation, symbols.SymbolTextualRepresentation,
		symbols.SymbolAlias, symbols.SymbolDependency, symbols.SymbolRelationship,
		symbols.SymbolConnectorEnd, symbols.SymbolUnknown:
		return false
	}
	return true
}

// detailWith adds a note to a node's detail, keeping what it already said.
func detailWith(detail, note string) string {
	if detail == "" {
		return note
	}
	return detail + ", " + note
}
