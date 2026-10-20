package view

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// RelatedKind names one kind of relationship Related groups elements by; the
// names are what a client asks a rendering to include elements by.
type RelatedKind string

const (
	// RelatedGeneralizations are the elements sym specializes, subsets or redefines.
	RelatedGeneralizations RelatedKind = "generalizations"
	// RelatedSpecializations are the elements specializing, subsetting or redefining sym.
	RelatedSpecializations RelatedKind = "specializations"
	// RelatedTypes are the definitions typing sym.
	RelatedTypes RelatedKind = "types"
	// RelatedTypedUsages are the usages sym types.
	RelatedTypedUsages RelatedKind = "typedUsages"
	// RelatedOwner is the element declaring sym.
	RelatedOwner RelatedKind = "owner"
	// RelatedOwnedMembers are the elements sym declares.
	RelatedOwnedMembers RelatedKind = "ownedMembers"
	// RelatedConnections are the elements connectors join sym to.
	RelatedConnections RelatedKind = "connections"
	// RelatedFlows are the elements flows join sym to.
	RelatedFlows RelatedKind = "flows"
	// RelatedBindings are the elements bindings join sym to.
	RelatedBindings RelatedKind = "bindings"
	// RelatedTransitions are the states sym's transitions reach or leave.
	RelatedTransitions RelatedKind = "transitions"
	// RelatedSuccessions are the actions sym's successions reach or leave.
	RelatedSuccessions RelatedKind = "successions"
)

// relatedKinds is every RelatedKind in the order Related lists groups.
var relatedKinds = []RelatedKind{
	RelatedGeneralizations, RelatedSpecializations, RelatedTypes, RelatedTypedUsages,
	RelatedOwner, RelatedOwnedMembers, RelatedConnections, RelatedFlows, RelatedBindings,
	RelatedTransitions, RelatedSuccessions,
}

// relatedLabels say what each group holds, as a client lists them.
var relatedLabels = map[RelatedKind]string{
	RelatedGeneralizations: "Generalizations (specializes, subsets, redefines)",
	RelatedSpecializations: "Specializations (specialized, subsetted, redefined by)",
	RelatedTypes:           "Types (typed by)",
	RelatedTypedUsages:     "Typed usages (types)",
	RelatedOwner:           "Owner",
	RelatedOwnedMembers:    "Owned members",
	RelatedConnections:     "Connections (connected to)",
	RelatedFlows:           "Flows (flows from or to)",
	RelatedBindings:        "Bindings (bound to)",
	RelatedTransitions:     "Transitions (states reached or left)",
	RelatedSuccessions:     "Successions (actions reached or left)",
}

// RelatedKinds lists every kind Related groups by, in listing order.
func RelatedKinds() []RelatedKind { return append([]RelatedKind(nil), relatedKinds...) }

// RelatedGroup is the elements one kind of relationship relates an element to.
type RelatedGroup struct {
	Kind RelatedKind
	// Label says what the group holds, for a client listing it.
	Label string
	// Elements are the named related elements in model order, each once, never
	// the element itself and never a standard library element: those are what
	// a rendering draws when a view exposes them.
	Elements []*symbols.Symbol
}

// Related groups the model's own elements related to sym by the relationships
// a rendering draws edges for: its generalizations and specializations, its
// types and the usages it types, its owner and owned members, the elements
// connectors, flows and bindings join it to, and the states or actions its
// transitions or successions reach or leave. Empty groups are left out.
func (r *Renderer) Related(sym *symbols.Symbol) []RelatedGroup {
	if sym == nil {
		return nil
	}
	c := &relatedCollector{r: r, sym: sym, groups: map[RelatedKind]*RelatedGroup{}, seen: map[RelatedKind]map[*symbols.Symbol]bool{}}
	c.generalizations()
	c.ownership()
	c.modelWide()
	c.transitions()
	c.successions()
	var out []RelatedGroup
	for _, kind := range relatedKinds {
		if g := c.groups[kind]; g != nil {
			out = append(out, *g)
		}
	}
	return out
}

type relatedCollector struct {
	r      *Renderer
	sym    *symbols.Symbol
	groups map[RelatedKind]*RelatedGroup
	seen   map[RelatedKind]map[*symbols.Symbol]bool
}

// add records elem under kind unless it is the element itself, unnamed (no
// qualified name includes it), a library element, or already recorded there.
func (c *relatedCollector) add(kind RelatedKind, elem *symbols.Symbol) {
	if elem == nil || elem == c.sym || elem.Name == "" || c.library(elem) {
		return
	}
	if c.seen[kind] == nil {
		c.seen[kind] = map[*symbols.Symbol]bool{}
	}
	if c.seen[kind][elem] {
		return
	}
	c.seen[kind][elem] = true
	if c.groups[kind] == nil {
		c.groups[kind] = &RelatedGroup{Kind: kind, Label: relatedLabels[kind]}
	}
	c.groups[kind].Elements = append(c.groups[kind].Elements, elem)
}

func (c *relatedCollector) library(sym *symbols.Symbol) bool {
	idx := c.r.resolver.Index()
	return idx != nil && idx.IsLibraryDocument(sym.DocName)
}

// generalizations are what the element's own relationships name: the targets a
// tree or general rendering draws specialization and typing edges to.
func (c *relatedCollector) generalizations() {
	for _, rel := range semantics.RelationshipsOf(c.sym) {
		if rel == nil || rel.Target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines:
			if target, ok := c.r.generalizationTarget(c.sym, rel); ok {
				c.add(RelatedGeneralizations, target)
			}
		}
	}
	for _, typ := range c.r.model.DeclaredTypes(c.sym) {
		c.add(RelatedTypes, typ)
	}
}

// ownership is the element declaring sym and the elements sym declares.
func (c *relatedCollector) ownership() {
	if c.sym.OwnerScope != nil {
		c.add(RelatedOwner, c.sym.OwnerScope.Owner())
	}
	for _, member := range ownedMembers(c.sym) {
		c.add(RelatedOwnedMembers, member)
	}
}

// ownedMembers are the elements sym's body declares, in declaration order.
func ownedMembers(sym *symbols.Symbol) []*symbols.Symbol {
	if sym.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range append(append([]*symbols.Symbol(nil), sym.Scope.Members()...), sym.Scope.AnonymousMembers()...) {
		if member.OwnerScope == sym.Scope {
			out = append(out, member)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DeclSpan.Offset < out[j].DeclSpan.Offset })
	return out
}

// modelWide walks the model's own elements once for what names sym from the
// other side: its specializations, the usages it types, and the connectors,
// flows and bindings with an end on it.
func (c *relatedCollector) modelWide() {
	for _, elem := range c.r.modelElements() {
		if elem == c.sym {
			continue
		}
		for _, rel := range semantics.RelationshipsOf(elem) {
			if rel == nil || rel.Target == nil {
				continue
			}
			switch rel.Kind {
			case ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines:
				if target, ok := c.r.generalizationTarget(elem, rel); ok && target == c.sym {
					c.add(RelatedSpecializations, elem)
				}
			case ast.RelTyping:
				for _, typ := range c.r.model.DeclaredTypes(elem) {
					if typ == c.sym {
						c.add(RelatedTypedUsages, elem)
					}
				}
			}
		}
		if c.r.drawsConnector(elem) {
			c.connector(elem)
		}
	}
}

// connector records the elements the other ends of a connector on sym attach
// to: the part an end is written from (`car` of `car.port`), else the feature.
func (c *relatedCollector) connector(connector *symbols.Symbol) {
	ends, edgeKind := c.r.connectorEnds(connector)
	kind := RelatedConnections
	switch edgeKind {
	case EdgeFlow:
		kind = RelatedFlows
	case EdgeBinding:
		kind = RelatedBindings
	}
	anchors := make([][2]*symbols.Symbol, len(ends))
	touches := false
	for i, end := range ends {
		anchors[i] = c.endAnchors(connector.OwnerScope, end.attachment)
		if anchors[i][0] == c.sym || anchors[i][1] == c.sym {
			touches = true
		}
	}
	if !touches {
		return
	}
	for _, anchor := range anchors {
		if anchor[0] == c.sym || anchor[1] == c.sym {
			continue
		}
		if anchor[0] != nil {
			c.add(kind, anchor[0])
		} else {
			c.add(kind, anchor[1])
		}
	}
}

// endAnchors resolves a connector end to the element it is written from and
// the feature it names: `car.port` anchors at car and port, `car` at car twice.
func (c *relatedCollector) endAnchors(scope *symbols.Scope, attachment ast.Node) [2]*symbols.Symbol {
	feature, _ := c.r.resolveEnd(scope, attachment)
	head := attachment
	for {
		chain, ok := head.(*ast.FeatureChainExpr)
		if !ok {
			break
		}
		head = chain.Operand
	}
	var from *symbols.Symbol
	if qname := ast.AsQualifiedName(head); qname != nil && len(qname.Parts) > 1 && qname.Parts[0].Text != "" {
		from, _ = c.r.resolver.LookupName(scope, qname.Parts[0].Text)
	} else if head != attachment {
		from, _ = c.r.resolveEnd(scope, head)
	} else {
		from = feature
	}
	return [2]*symbols.Symbol{from, feature}
}

// transitions are the states the transitions of the state machine owning sym
// reach from it or leave for it, read from the lowered graph a state rendering
// draws its edges from.
func (c *relatedCollector) transitions() {
	if c.sym.Kind != symbols.SymbolStateUsage {
		return
	}
	machine := c.behaviorOwner(symbols.SymbolStateDef, symbols.SymbolStateUsage)
	if machine == nil {
		return
	}
	graph, err := lower.ToStateGraphWithEndpoints(machine.Decl, declScope(machine), lower.NewLibraryStateTypes(c.r.resolver))
	if err != nil {
		return
	}
	root := c.documentRoot(machine.DocName)
	if root == nil {
		return
	}
	stateOf := func(node ast.Node) *symbols.Symbol {
		return root.DeclaredAt(nodeOrigin(machine.DocName, node).Span)
	}
	for src, transitions := range graph.Transitions {
		for _, transition := range transitions {
			if stateOf(src) == c.sym {
				c.add(RelatedTransitions, stateOf(transition.Target))
			} else if stateOf(transition.Target) == c.sym {
				c.add(RelatedTransitions, stateOf(src))
			}
		}
	}
}

// successions are the actions the successions of the action owning sym reach
// from it or leave for it.
func (c *relatedCollector) successions() {
	if c.sym.Kind != symbols.SymbolActionUsage {
		return
	}
	owner := c.behaviorOwner(symbols.SymbolActionDef, symbols.SymbolActionUsage)
	if owner == nil {
		return
	}
	for _, succession := range c.r.model.ActionSuccessions(owner) {
		switch c.sym {
		case succession.Source.Symbol:
			c.add(RelatedSuccessions, succession.Target.Symbol)
		case succession.Target.Symbol:
			c.add(RelatedSuccessions, succession.Source.Symbol)
		}
	}
}

// behaviorOwner is the nearest element declaring sym whose kind is one of kinds.
func (c *relatedCollector) behaviorOwner(kinds ...symbols.SymbolKind) *symbols.Symbol {
	for scope := c.sym.OwnerScope; scope != nil; scope = scope.Owner().OwnerScope {
		owner := scope.Owner()
		if owner == nil {
			return nil
		}
		for _, kind := range kinds {
			if owner.Kind == kind {
				return owner
			}
		}
	}
	return nil
}

func (c *relatedCollector) documentRoot(doc string) *symbols.Scope {
	idx := c.r.resolver.Index()
	if idx == nil {
		return nil
	}
	return idx.DocumentRoot(doc)
}
