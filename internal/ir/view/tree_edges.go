package view

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// treeSite is the element a tree node draws and the view it is shown under,
// nil outside any view.
type treeSite struct {
	sym  *symbols.Symbol
	view *symbols.Symbol
}

// noteTreeSite records what a tree node draws, for the edges a tree adds once
// every node is drawn; a rendering of another kind records nothing.
func (r *Rendering) noteTreeSite(node *Node, view, sym *symbols.Symbol) {
	if r.sites != nil {
		r.sites[node] = treeSite{sym: sym, view: view}
	}
}

// treeGraph draws the relationship edges of a tree between the nodes it has,
// as the graphical notation's general view draws them: a specialization,
// subsetting or redefinition from an element to its general one; the
// composition (filled diamond) or reference (hollow diamond) from an element
// to the definition typing a structural usage it owns, labelled with that
// usage's name and multiplicity; and the typing of a usage drawn with no
// drawn owner. An edge is drawn only between two drawn nodes, never from a
// node to one nested in it, since the nesting already shows that membership.
type treeGraph struct {
	r      *Renderer
	out    *Rendering
	nodes  map[symbols.ElementKey]*Node // the first node drawn for each element
	parent map[string]string            // node ID -> the ID of the node it is nested in
	edges  map[treeEdgeKey]bool
	lined  map[symbols.ElementKey]bool // properties whose edge an association line stands for
}

// treeEdgeKey identifies an edge a tree draws once: a relationship between
// two nodes, or the membership of one usage, which two anonymous usages of one
// type never share.
type treeEdgeKey struct {
	from, to, label string
	kind            EdgeKind
	usage           symbols.ElementKey
}

// treeEdges adds to out the relationship edges between the nodes of its tree,
// in the order of the nodes they leave. An element drawn more than once draws
// its edges from the first node, the one that shows its members.
func (r *Renderer) treeEdges(out *Rendering) {
	g := &treeGraph{r: r, out: out, nodes: map[symbols.ElementKey]*Node{}, parent: map[string]string{}, edges: map[treeEdgeKey]bool{}, lined: map[symbols.ElementKey]bool{}}
	var index func(node *Node, parent string)
	index = func(node *Node, parent string) {
		if parent != "" {
			g.parent[node.ID] = parent
		}
		if site, ok := out.sites[node]; ok {
			key := symbols.KeyOf(site.sym)
			if _, drawn := g.nodes[key]; !drawn {
				g.nodes[key] = node
			}
		}
		for _, child := range node.Children {
			index(child, node.ID)
		}
	}
	for _, root := range out.Roots {
		index(root, "")
	}
	g.associationLines()
	var walk func(node *Node)
	walk = func(node *Node) {
		if site, ok := out.sites[node]; ok && g.nodes[symbols.KeyOf(site.sym)] == node {
			g.edgesOf(node, site)
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range out.Roots {
		walk(root)
	}
	out.sites = nil
}

// associationLines redraws each connection def the tree would show as a box
// as a line between the nodes of its two end types, the way a block definition
// diagram draws an association, and drops its node. Its ends are labelled on
// the line; its own name is, unless a migration made the name up.
func (g *treeGraph) associationLines() {
	var prune func(nodes []*Node) []*Node
	prune = func(nodes []*Node) []*Node {
		kept := nodes[:0]
		for _, node := range nodes {
			site := g.out.sites[node]
			if line, ok := g.associationLine(node, site); ok {
				def := site.sym
				delete(g.nodes, symbols.KeyOf(def))
				name := ""
				if !g.r.model.NameSynthesized(def) {
					name = localName(def)
				}
				edge := Edge{From: line.from.ID, To: line.to.ID, Kind: EdgeAssociation, Name: name, Origin: symbolOrigin(def),
					Route: g.r.routeOf(site.view, def, g.out), Style: node.Style}
				var labels []string
				for i, end := range line.ends {
					label := ""
					if p := line.crossed[i]; p != nil {
						g.lined[symbols.KeyOf(p)] = true
						label = g.r.usageLabel(p)
						if compositeUsage(p) && edge.Kind != EdgeComposition {
							edge.Kind, edge.From, edge.To = EdgeComposition, g.node(p.Owner()).ID, g.node(line.types[i]).ID
						}
						if edge.Route == nil {
							edge.Route = g.r.routeOf(site.view, p, g.out)
						}
						if edge.Style == nil {
							edge.Style = g.r.styleOf(site.view, p, g.out)
						}
					} else if !g.r.model.NameSynthesized(end) {
						label = g.r.usageLabel(end)
					}
					if label != "" {
						labels = append(labels, label)
					}
				}
				edge.Label = associationLabel(name, labels)
				g.add(edge, def)
				for i := range g.out.Notes {
					if n := &g.out.Notes[i]; n.Anchor == node.ID {
						n.Anchor, n.EdgeFrom, n.EdgeTo = "", edge.From, edge.To
					}
				}
				continue
			}
			node.Children = prune(node.Children)
			kept = append(kept, node)
		}
		return kept
	}
	g.out.Roots = prune(g.out.Roots)
}

// association is a connection def drawn as a line: the nodes of its two end
// types, its ends, and for each end the drawn property it crosses, nil for none.
type association struct {
	from, to *Node
	ends     []*symbols.Symbol
	types    [2]*symbols.Symbol
	crossed  [2]*symbols.Symbol
}

// associationLine is the line the connection def of node is drawn as: between
// the nodes of its two ends' types, each one drawn definition the def is not
// nested in. An end crossing a part or reference property the tree draws as
// an edge from its owner takes that edge over: the association is one line,
// a composition when the property is composite. A def with other ends, with
// inherited ends, with a specialization or other relationship a box would
// draw, with a member a box would show (a qualifier of an end included), or
// with an end type the tree does not draw stays a box.
func (g *treeGraph) associationLine(node *Node, site treeSite) (line association, ok bool) {
	def := site.sym
	if def == nil || def.Kind != symbols.SymbolConnectionDef || g.nodes[symbols.KeyOf(def)] != node {
		return line, false
	}
	line.ends = g.r.model.EndFeatures(def)
	if len(line.ends) != 2 {
		return line, false
	}
	for _, rel := range semantics.RelationshipsOf(def) {
		if _, _, ok := structuralEdgeKind(rel.Kind); ok && rel.Target != nil {
			return line, false
		}
	}
	members := g.r.containedMembers(def)
	var typed [2]*Node
	for i, end := range line.ends {
		if !slices.Contains(members, end) || g.endHasContent(end) {
			return line, false
		}
		types := g.r.model.DeclaredTypes(end)
		if len(types) != 1 {
			return line, false
		}
		line.types[i] = types[0]
		typed[i] = g.node(types[0])
		if typed[i] == nil || typed[i] == node || g.nested(node, typed[i]) || types[0].Kind == symbols.SymbolConnectionDef {
			return line, false
		}
	}
	for _, member := range members {
		if !slices.Contains(line.ends, member) {
			return line, false
		}
	}
	for i, end := range line.ends {
		p := g.r.model.CrossFeature(end)
		if p == nil || !structuralUsage(p) || g.node(p.Owner()) != typed[1-i] || !slices.Contains(g.r.model.DeclaredTypes(p), line.types[i]) {
			continue
		}
		line.crossed[i] = p
	}
	line.from, line.to = typed[0], typed[1]
	return line, true
}

// associationLabel labels the line a connection def is drawn as: its name, if
// it has one of its own, and the labels of its ends.
func associationLabel(name string, ends []string) string {
	label := strings.Join(ends, " / ")
	switch {
	case name == "":
		return label
	case label == "":
		return name
	}
	return name + ": " + label
}

// edgesOf draws what the element of node states of itself and of the usages
// drawn beneath it; a member the depth bound hides is not drawn as an edge
// either. Its typing is drawn from the node itself only when its owner is not
// drawn: a drawn owner's composition edge, or the nesting, stands for it. A
// usage's Style dresses the edge standing for it as it does the usage's node;
// its Notes stay anchored to the node, where the usage is named.
func (g *treeGraph) edgesOf(node *Node, site treeSite) {
	sym := site.sym
	ownerDrawn := g.node(sym.Owner()) != nil && structuralUsage(sym)
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil {
			continue
		}
		kind, label, ok := structuralEdgeKind(rel.Kind)
		if !ok || kind == EdgeTyping && ownerDrawn {
			continue
		}
		target, resolved := g.r.generalizationTarget(sym, rel)
		if !resolved {
			continue
		}
		if to := g.node(target); to != nil && to != node && !g.nested(node, to) {
			g.add(Edge{From: node.ID, To: to.ID, Kind: kind, Label: label, Origin: nodeOrigin(sym.DocName, rel)}, nil)
		}
	}
	for _, child := range node.Children {
		member := g.out.sites[child].sym
		if !structuralUsage(member) || g.lined[symbols.KeyOf(member)] {
			continue
		}
		kind := EdgeComposition
		if !compositeUsage(member) {
			kind = EdgeReference
		}
		for _, typ := range g.r.model.DeclaredTypes(member) {
			to := g.node(typ)
			if to == nil || to == node || g.nested(node, to) {
				continue
			}
			g.add(Edge{From: node.ID, To: to.ID, Kind: kind, Label: g.r.usageLabel(member), Origin: symbolOrigin(member),
				Route: g.r.routeOf(site.view, member, g.out), Style: g.r.styleOf(site.view, member, g.out)}, member)
		}
	}
}

// endHasContent reports whether an end declares more than its cross feature,
// such as a qualifier, which only a box shows.
func (g *treeGraph) endHasContent(end *symbols.Symbol) bool {
	for _, member := range g.r.containedMembers(end) {
		if _, cross := member.Decl.(*ast.CrossFeatureMember); !cross {
			return true
		}
	}
	return false
}

// node is the node drawing sym, nil when the tree draws none.
func (g *treeGraph) node(sym *symbols.Symbol) *Node {
	if sym == nil {
		return nil
	}
	return g.nodes[symbols.KeyOf(sym)]
}

// nested reports whether one of two nodes is drawn inside the other.
func (g *treeGraph) nested(a, b *Node) bool {
	return g.encloses(a.ID, b.ID) || g.encloses(b.ID, a.ID)
}

// encloses reports whether the node outer is an ancestor of the node inner.
func (g *treeGraph) encloses(outer, inner string) bool {
	for id, ok := g.parent[inner]; ok; id, ok = g.parent[id] {
		if id == outer {
			return true
		}
	}
	return false
}

// add records an edge once; usage is the member the edge stands for, nil for
// a relationship clause.
func (g *treeGraph) add(edge Edge, usage *symbols.Symbol) {
	key := treeEdgeKey{from: edge.From, to: edge.To, label: edge.Label, kind: edge.Kind, usage: symbols.KeyOf(usage)}
	if g.edges[key] {
		return
	}
	g.edges[key] = true
	g.out.Edges = append(g.out.Edges, edge)
}

// structuralUsage reports whether sym is a usage whose typing a block
// definition diagram draws as a composition or reference from its owner: a
// part, item, port, attribute, occurrence, enumeration or bare `ref` usage.
// A behavior, connector or other usage keeps its typing as an edge of its own.
func structuralUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if _, ok := sym.Decl.(*ast.Usage); !ok {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolPartUsage, symbols.SymbolItemUsage, symbols.SymbolPortUsage, symbols.SymbolAttributeUsage,
		symbols.SymbolOccurrenceUsage, symbols.SymbolIndividualUsage, symbols.SymbolEnumerationUsage, symbols.SymbolReferenceUsage:
		return true
	}
	return false
}

// usageLabel is a usage's name and multiplicity as the notation writes them,
// `wheels[4]`: the label of the edge standing for the usage's membership. A
// name a migration made up is left out, as the node drawing the usage leaves it.
func (r *Renderer) usageLabel(sym *symbols.Symbol) string {
	label := ""
	if !r.model.NameSynthesized(sym) {
		label = nameText(sym.Name)
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return label
	}
	mult := usage.Multiplicity
	if mult == nil && usage.CrossFeature != nil {
		// An end's cross multiplicity, `end [0..1] ref trailer`, is how many
		// of it one of the other end's type is linked to.
		mult = usage.CrossFeature.Multiplicity
	}
	if mult == nil {
		return label
	}
	return label + r.multiplicityText(sym.DocName, mult)
}

// multiplicityText spells a multiplicity as written, `[4]` or `[0..*]`, from
// the source when the rendering holds it and from the bounds otherwise.
func (r *Renderer) multiplicityText(doc string, m *ast.Multiplicity) string {
	if text := r.nodeText(doc, m); text != "" {
		return text
	}
	lower := multiplicityBound(m.Lower)
	if !m.IsRange {
		return "[" + lower + "]"
	}
	return "[" + lower + ".." + multiplicityBound(m.Upper) + "]"
}

// multiplicityBound spells one bound of a multiplicity: a literal as written,
// `*` for the unbounded one, a name as the notation refers to it, an
// expression over them with its operators.
func multiplicityBound(node ast.Node) string {
	if text := expressionText(node); text != "" {
		return text
	}
	return "?"
}

// expressionText spells an expression as the notation writes it: a literal, a
// name or feature chain, and an operator over such operands, grouped where the
// operators' precedence would otherwise read it back differently. An
// expression of another form spells as "".
func expressionText(node ast.Node) string {
	switch expr := node.(type) {
	case *ast.LiteralInfinity:
		return "*"
	case *ast.FeatureReference, *ast.QualifiedName, *ast.FeatureChainExpr:
		return referenceText(node)
	case *ast.OperatorExpr:
		return operatorExprText(expr)
	}
	return literalText(node)
}

// operatorExprText spells an operator application: a unary operator before
// its operand, a binary one between its two with a space on each side, a
// conditional as `if c ? a else b`, a classification or cast with its type.
func operatorExprText(expr *ast.OperatorExpr) string {
	operands := make([]string, len(expr.Operands))
	for i, operand := range expr.Operands {
		operands[i] = operandText(expr, i, operand)
		if operands[i] == "" {
			return ""
		}
	}
	op := expr.Operator.String()
	switch {
	case expr.Operator == ast.OpConditional && len(operands) == 3:
		return "if " + operands[0] + " ? " + operands[1] + " else " + operands[2]
	case expr.TypeRef != nil && len(operands) == 1:
		return operands[0] + " " + op + " " + referenceText(expr.TypeRef)
	case len(operands) == 1 && expr.Operator == ast.OpNot:
		return op + " " + operands[0]
	case len(operands) == 1:
		return op + operands[0]
	case len(operands) == 2:
		return operands[0] + " " + op + " " + operands[1]
	}
	return ""
}

// operandText spells the i-th operand of expr, parenthesised when it is an
// operator application binding no tighter than expr: looser, or as tight on
// the side the operator does not associate to; under a unary operator, any
// binary one.
func operandText(expr *ast.OperatorExpr, i int, operand ast.Node) string {
	text := expressionText(operand)
	inner, ok := operand.(*ast.OperatorExpr)
	if !ok || text == "" {
		return text
	}
	if len(expr.Operands) == 1 {
		if len(inner.Operands) > 1 {
			return "(" + text + ")"
		}
		return text
	}
	if len(expr.Operands) != 2 {
		return text
	}
	outer, in := operatorPrecedence(expr.Operator), operatorPrecedence(inner.Operator)
	rightAssociative := expr.Operator == ast.OpPow
	if in < outer || in == outer && (i == 1) != rightAssociative {
		return "(" + text + ")"
	}
	return text
}

// operatorPrecedence ranks the operators as the KerML expression grammar binds
// them, loosest first; operators of one rank associate to the left but `**`.
func operatorPrecedence(op ast.OperatorKind) int {
	switch op {
	case ast.OpConditional:
		return 0
	case ast.OpNullCoalesce:
		return 1
	case ast.OpImplies:
		return 2
	case ast.OpOr, ast.OpConditionalOr:
		return 3
	case ast.OpXor:
		return 4
	case ast.OpAnd, ast.OpConditionalAnd:
		return 5
	case ast.OpEq, ast.OpNeq, ast.OpEqEqEq, ast.OpNeqEqEq:
		return 6
	case ast.OpHasType, ast.OpIsType, ast.OpAt, ast.OpMetaAt, ast.OpAs, ast.OpMeta:
		return 7
	case ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe:
		return 8
	case ast.OpRange:
		return 9
	case ast.OpAdd, ast.OpSub:
		return 10
	case ast.OpMul, ast.OpDiv, ast.OpMod:
		return 11
	case ast.OpPow:
		return 12
	}
	return 13
}
