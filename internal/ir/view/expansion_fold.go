package view

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const defaultExpansionFoldThreshold = 10000

type expansionSizeKey struct {
	treeQueryKey
	depth int
}

type expansionRepresentative struct {
	depth int
	name  string
}

type expansionFold struct {
	q               *treeQuery
	walk            *featureWalk
	kind            Kind
	limit           int
	sizes           map[expansionSizeKey]int
	representatives map[treeQueryKey]expansionRepresentative
	protected       []bool
	pinned          map[*symbols.Symbol]bool
	descendants     map[symbols.ElementKey]bool
	pruned          map[*symbols.Symbol]bool
	total           int
}

func (r *Renderer) prepareExpansionFold(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering, kind Kind) *expansionFold {
	if out.query || r.expansionFoldThreshold < 0 {
		return nil
	}
	if out.expansionPlans != nil {
		if out.expansionFolding {
			return out.expansionPlans[view]
		}
		return nil
	}
	limit := r.expansionFoldThreshold
	if limit == 0 {
		limit = defaultExpansionFoldThreshold
	}
	out.expansionPlans = map[*symbols.Symbol]*expansionFold{}
	var measure func(*symbols.Symbol, []*symbols.Symbol, Kind, bool) int
	measure = func(view *symbols.Symbol, roots []*symbols.Symbol, kind Kind, top bool) int {
		if out.expansionPlans[view] != nil {
			return 0
		}
		q := r.treeQuery(view, &Rendering{query: true}, roots)
		f := &expansionFold{q: q, kind: kind, limit: limit,
			sizes: map[expansionSizeKey]int{}, representatives: map[treeQueryKey]expansionRepresentative{}, pinned: map[*symbols.Symbol]bool{}}
		out.expansionPlans[view] = f
		var descendants map[symbols.ElementKey]bool
		if kind == KindTree {
			descendants = q.exposedDescendants(roots, top)
		} else {
			w := &featureWalk{r: r, view: view, pruned: map[*symbols.Symbol]bool{}}
			f.walk = w
			var features []*symbols.Symbol
			for _, sym := range roots {
				if w.drawsFeature(sym) && !r.drawsConnector(sym) {
					features = append(features, sym)
				}
			}
			descendants = r.exposedDescendants(view, features, w.nestedFeatures)
			f.descendants = descendants
			f.pruned = w.pruned
		}
		size := 0
		var ports []*symbols.Symbol
		for _, sym := range roots {
			if descendants[symbols.KeyOf(sym)] || kind == KindInterconnection && (!f.walk.drawsFeature(sym) || r.drawsConnector(sym)) {
				continue
			}
			if kind == KindInterconnection && sym.Kind == symbols.SymbolPortUsage {
				ports = append(ports, sym)
				continue
			}
			if kind == KindTree && top && annotatingSymbol(sym) {
				if r.annotationNode(&nodeIDs{}, view, sym, &Rendering{}) != nil {
					size = f.add(size, 1)
				}
				continue
			}
			size = f.add(size, f.size(sym, nil, expansion{}, 0, true))
		}
		for _, port := range ports {
			if !f.pinned[port] {
				size = f.add(size, f.size(port, nil, expansion{}, 0, true))
			}
		}
		if kind == KindTree {
			nested, _ := r.model.NestedViews(view)
			for _, sub := range nested {
				if out.expansionPlans[sub] != nil {
					continue
				}
				elements, err := r.model.ExposedElements(sub)
				if err != nil {
					elements = nil
				}
				size = f.add(size, f.add(1, measure(sub, elements, KindTree, false)))
			}
		}
		f.total = size
		return size
	}
	if measure(view, exposed, kind, true) <= limit {
		return nil
	}
	out.expansionFolding = true
	for _, f := range out.expansionPlans {
		f.protect()
	}
	return out.expansionPlans[view]
}

func (f *expansionFold) add(a, b int) int {
	if a > f.limit || b > f.limit-a {
		return f.limit + 1
	}
	return a + b
}

// size counts occurrence nodes without constructing their tree; depth is part of the memo key.
func (f *expansionFold) size(sym, owner *symbols.Symbol, path expansion, depth int, qualified bool) int {
	key := expansionSizeKey{f.q.key(sym, owner, path, qualified), depth}
	if size, ok := f.sizes[key]; key.sym != nil && ok {
		return size
	}
	size := 1
	if path[sym] {
		return size
	}
	f.q.ancestors[sym] = true
	defer delete(f.q.ancestors, sym)
	r := f.q.r
	if f.kind == KindInterconnection {
		for _, port := range r.typedPorts(sym) {
			if r.admits(r.walkFilters(f.q.view), port) {
				f.pinned[port] = true
			}
		}
	}
	recursive, added := r.enter(path, sym)
	defer leave(path, added)
	geometry := f.q.geometryOf(sym, owner, &Rendering{})
	cut := depth >= f.q.bound || geometry != nil && geometry.Collapsed
	for _, member := range f.q.members(sym) {
		if recursive != nil && member.Owner() != sym {
			continue
		}
		if !f.drawsMember(sym, member) {
			continue
		}
		if f.kind == KindInterconnection && (r.drawsConnector(member) ||
			casePin(sym, member) || constraintParameter(sym, member) || member.Kind == symbols.SymbolPortUsage && member.Owner() != sym) {
			continue
		}
		if cut && !pinnedAt(sym, member) {
			continue
		}
		size = f.add(size, f.size(member, sym, path, depth+1, false))
		if size > f.limit {
			break
		}
	}
	if f.kind == KindTree && recursive == nil && !cut && size <= f.limit {
		for _, annotation := range r.annotatingMembers(sym) {
			if r.admits(r.walkFilters(f.q.view), annotation) && r.annotationNode(&nodeIDs{}, f.q.view, annotation, &Rendering{}) != nil {
				size = f.add(size, 1)
			}
		}
	}
	if key.sym != nil {
		f.sizes[key] = size
	}
	return size
}

// protect propagates positioned elements and relationship ends through the condensed graph.
func (f *expansionFold) protect() {
	q, r := f.q, f.q.r
	targets := map[*symbols.Symbol]bool{}
	for sym := range q.component {
		if _, ok := r.model.LayoutOf(q.view, sym); ok {
			targets[sym] = true
		}
		for _, member := range q.members(sym) {
			if _, ok := r.model.MemberLayoutOf(q.view, sym, localName(member)); ok {
				targets[member] = true
			}
		}
		if f.kind == KindInterconnection && r.drawsConnector(sym) {
			ends, _ := r.connectorEnds(sym)
			for _, end := range ends {
				if target, ok := r.resolver.ResolveTarget(sym.OwnerScope, end.attachment); ok {
					targets[target] = true
				}
			}
		}
		if f.kind == KindInterconnection {
			if value := featureValue(sym); value != nil {
				if target, ok := r.resolver.ResolveTarget(sym.OwnerScope, value); ok {
					targets[sym], targets[target] = true, true
				}
			}
		}
	}
	if f.kind == KindTree {
		out, err := r.render(q.view, newDrawn(), true)
		if err == nil {
			origins := map[Origin]*symbols.Symbol{}
			for sym := range q.component {
				origins[symbolOrigin(sym)] = sym
			}
			nodes := map[string]*symbols.Symbol{}
			seen := map[*Node]bool{}
			var index func([]*Node)
			index = func(list []*Node) {
				for _, node := range list {
					if seen[node] {
						continue
					}
					seen[node] = true
					nodes[node.ID] = origins[node.Origin]
					index(node.Children)
				}
			}
			index(out.Roots)
			for _, edge := range out.Edges {
				if sym := nodes[edge.From]; sym != nil {
					targets[sym] = true
				}
				if sym := nodes[edge.To]; sym != nil {
					targets[sym] = true
				}
			}
		}
	}
	f.protected = make([]bool, len(q.children))
	for sym := range targets {
		if component, ok := q.component[sym]; ok {
			f.protected[component] = true
		}
	}
	for component, children := range q.children {
		for _, child := range children {
			f.protected[component] = f.protected[component] || f.protected[child]
		}
	}
}

func (f *expansionFold) keeps(member *symbols.Symbol) bool {
	component, ok := f.q.component[member]
	return ok && f.protected[component]
}

func (f *expansionFold) omits(owner, member *symbols.Symbol) bool {
	return member.Owner() != owner && len(f.q.r.model.MaskingRedefinedFeatures(member)) == 0 && !f.keeps(member)
}

func (f *expansionFold) drawsMember(owner, member *symbols.Symbol) bool {
	if f.walk != nil {
		return f.walk.drawsMember(owner, member)
	}
	return !treeParameter(owner, member)
}

func (f *expansionFold) representative(sym, owner *symbols.Symbol, path expansion, depth int, qualified bool) (treeQueryKey, expansionRepresentative, bool) {
	key := f.q.key(sym, owner, path, qualified)
	// Neither a rewritten box nor its descendants can stand for features drawn elsewhere.
	if key.sym == nil || key.rewrites != "" || f.q.r.rewritesTreeBox(sym) {
		return treeQueryKey{}, expansionRepresentative{}, false
	}
	representative, ok := f.representatives[key]
	return key, representative, ok && representative.depth <= depth
}

func (f *expansionFold) remember(key treeQueryKey, depth int, name string) {
	if key.sym == nil {
		return
	}
	if previous, ok := f.representatives[key]; !ok || depth < previous.depth {
		f.representatives[key] = expansionRepresentative{depth: depth, name: name}
	}
}

func (f *expansionFold) note(out *Rendering, sym *symbols.Symbol, node *Node, representative expansionRepresentative) {
	var definitions []string
	if out.foldedDefinitions == nil {
		out.foldedDefinitions = map[*symbols.Symbol]bool{}
	}
	for _, typ := range f.q.r.drawnTypes(sym) {
		definitions = append(definitions, f.q.r.notationName(typ))
		out.foldedDefinitions[typ] = true
	}
	if len(definitions) == 0 {
		seen := map[*symbols.Symbol]bool{}
		for _, member := range f.q.members(sym) {
			if owner := member.Owner(); owner != nil && f.drawsMember(sym, member) && f.omits(sym, member) && !seen[owner] {
				seen[owner], out.foldedDefinitions[owner] = true, true
				definitions = append(definitions, f.q.r.notationName(owner))
			}
		}
	}
	node.Detail = detailWith(node.Detail, fmt.Sprintf("features of %s shown at %s; repeated expansion folded", strings.Join(definitions, ", "), representative.name))
	out.foldedOccurrences++
}

func (r *Rendering) finishExpansionFold() {
	if r.foldedOccurrences > 0 {
		r.ExpansionFoldNotice = fmt.Sprintf("Repeated definition expansions folded: %d later occurrences of %d definitions. Each definition's features are shown at an earlier occurrence expanded at least as deeply; later occurrences retain their own and redefined features.", r.foldedOccurrences, len(r.foldedDefinitions))
	}
	r.expansionPlans = nil
	r.foldedDefinitions = nil
}

func (r *Rendering) notices() []string {
	if r.ExpansionFoldNotice == "" {
		return r.Notices
	}
	return slices.Concat(r.Notices, []string{r.ExpansionFoldNotice})
}
