package view

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// maxTreeDepth bounds how deep a containment tree is walked, so a model whose
// members reach back into their own owner still renders.
const maxTreeDepth = 16

// treeDepth is the containment bound treeNode and treeDescendants share,
// which a test may lower through Renderer.treeDepthBound.
func (r *Renderer) treeDepth() int {
	if r.treeDepthBound > 0 {
		return r.treeDepthBound
	}
	return maxTreeDepth
}

// renderTree renders the exposed elements as a containment tree: each element
// with its kind and name, the elements declared in it beneath it, and each view
// nested in the rendered view as a subtree of its own. Each node is placed by
// the Layout that positions its element in the view it is shown under.
func (r *Renderer) renderTree(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	ids := &nodeIDs{}
	descendants := r.exposedDescendants(exposed, r.containedMembers)
	for _, elem := range exposed {
		if descendants[symbols.KeyOf(elem)] {
			continue
		}
		out.Roots = append(out.Roots, r.treeNode(view, elem, ids, map[*symbols.Symbol]bool{}, 0, true, out))
	}
	out.Roots = append(out.Roots, r.nestedViewNodes(view, ids, map[*symbols.Symbol]bool{view: true}, out)...)
}

// exposedDescendants is the key of each exposed element a surviving root's
// walk by members already draws — a fixpoint, since an element whose
// containers are all suppressed must stand as a root itself.
func (r *Renderer) exposedDescendants(exposed []*symbols.Symbol, members func(*symbols.Symbol) []*symbols.Symbol) map[symbols.ElementKey]bool {
	by := make([]map[symbols.ElementKey]bool, len(exposed))
	for i, elem := range exposed {
		by[i] = r.treeDescendants(elem, members)
	}
	suppressed := map[symbols.ElementKey]bool{}
	for range exposed {
		changed := false
		for i, elem := range exposed {
			key := symbols.KeyOf(elem)
			contained := false
			for j := range exposed {
				if j == i || suppressed[symbols.KeyOf(exposed[j])] {
					continue
				}
				if by[j][key] {
					contained = true
					break
				}
			}
			if contained && !suppressed[key] {
				suppressed[key] = true
				changed = true
			} else if !contained && suppressed[key] {
				delete(suppressed, key)
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return suppressed
}

// treeDescendants is the set of element keys the walk given by members draws
// below sym, dry-run by the same walk: a member is drawn even when the guard
// stubs its node.
func (r *Renderer) treeDescendants(sym *symbols.Symbol, members func(*symbols.Symbol) []*symbols.Symbol) map[symbols.ElementKey]bool {
	out := map[symbols.ElementKey]bool{}
	var walk func(sym *symbols.Symbol, seen map[*symbols.Symbol]bool, depth int)
	walk = func(sym *symbols.Symbol, seen map[*symbols.Symbol]bool, depth int) {
		if seen[sym] || depth >= r.treeDepth() {
			return
		}
		seen[sym] = true
		for _, member := range members(sym) {
			out[symbols.KeyOf(member)] = true
			walk(member, seen, depth+1)
		}
	}
	walk(sym, map[*symbols.Symbol]bool{}, 0)
	return out
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
		node := &Node{ID: ids.take(), Kind: declKind(sub), Name: r.notationName(sub), NameSynthesized: r.model.NameSynthesized(sub),
			Type: declType(sub), Typings: r.declTypings(sub), Origin: symbolOrigin(sub), Geometry: r.geometryOf(view, sub, out)}
		r.dress(view, sub, node, out)
		exposed, err := r.model.ExposedElements(sub)
		if err == nil {
			descendants := r.exposedDescendants(exposed, r.containedMembers)
			for _, elem := range exposed {
				if descendants[symbols.KeyOf(elem)] {
					continue
				}
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
		name = localName(sym)
	}
	node := &Node{ID: ids.take(), Kind: declKind(sym), Name: name, NameSynthesized: r.model.NameSynthesized(sym),
		Type: declType(sym), Typings: r.declTypings(sym), Origin: symbolOrigin(sym), Geometry: r.geometryOf(view, sym, out),
		Style: r.styleOf(view, sym, out)}
	if seen[sym] {
		node.Detail = detailWith(node.Detail, "already shown")
		return node
	}
	r.notesOf(view, sym, node.ID, out)
	if depth >= r.treeDepth() {
		if len(r.containedMembers(sym)) > 0 {
			node.Detail = detailWith(node.Detail, fmt.Sprintf("nested deeper than %d levels; not shown", r.treeDepth()))
		}
		return node
	}
	seen[sym] = true
	for _, member := range r.containedMembers(sym) {
		node.Children = append(node.Children, r.treeNode(view, member, ids, seen, depth+1, false, out))
	}
	return node
}

// containedMembers returns what an element declares in its own body, in
// declaration order: the elements a containment tree shows beneath it. A
// reference member is anonymous, so the two member lists a scope keeps are
// merged by source position rather than concatenated.
func (r *Renderer) containedMembers(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	for _, list := range [][]*symbols.Symbol{sym.Scope.Members(), sym.Scope.AnonymousMembers()} {
		for _, member := range list {
			if member == sym || seen[member] || !r.contentKind(member) {
				continue
			}
			seen[member] = true
			out = append(out, member)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DeclSpan.Offset < out[j].DeclSpan.Offset })
	return out
}

// contentKind reports whether a member is model content a rendering shows, as
// against what only describes a picture of it or its migration: a view's `render`
// members, the DiagramLayout annotations and the SynthesizedName markers.
// Ordinary metadata on an element is content.
func (r *Renderer) contentKind(sym *symbols.Symbol) bool {
	if !containedKind(sym) {
		return false
	}
	if sym.Kind == symbols.SymbolRenderingUsage && sym.OwnerScope != nil {
		if owner := sym.OwnerScope.Owner(); owner != nil && semantics.IsView(owner) {
			return false
		}
	}
	return !r.model.IsLayoutAnnotation(sym) && !r.model.IsSynthesizedNameAnnotation(sym)
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
