package view

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A GeneralView is "a graph of nodes and edges" that its element filters
// specialize (StandardViewDefinitions::GeneralView). A view specializing it
// whose every filter condition classifies by SysML or KerML metaclasses, alone
// or joined by `or`, is drawn as the graph those metaclasses select: any
// requirement metaclass selects the requirement graph, else any package
// metaclass the package graph, else any definition or usage metaclass the
// definition graph. Relationship metaclasses select nothing on their own. Any
// other filter, and no filter at all, leaves the containment tree.

// generalViewName is the standard view definition the graphs specialize.
const generalViewName = viewDefinitionsPackage + "GeneralView"

// The metaclass names each specialization is selected by, matched against a
// filter's metaclass and every metaclass it specializes.
var (
	requirementMetaclasses  = []string{"RequirementDefinition", "RequirementUsage"}
	packageMetaclasses      = []string{"Package"}
	caseMetaclasses         = []string{"CaseDefinition", "CaseUsage"}
	relationshipMetaclasses = []string{"Relationship"}
	definitionMetaclasses   = []string{"Definition", "Usage"}
)

// generalSpecialization reports the graph a GeneralView's filters select and
// the filter text that selects it; false leaves the view a tree.
func (r *Renderer) generalSpecialization(view *symbols.Symbol) (Kind, string, bool) {
	if !r.nearestStandardIsGeneral(view) {
		return "", "", false
	}
	filters := r.viewFilters(view)
	if len(filters) == 0 {
		return "", "", false
	}
	selected := map[Kind]bool{}
	texts := make([]string, 0, len(filters))
	for _, filter := range filters {
		predicate := r.model.CompileElementFilter(filter)
		text, ok := r.classifyFilter(predicate, selected)
		if !ok {
			return "", "", false
		}
		texts = append(texts, text)
	}
	for _, kind := range []Kind{KindRequirement, KindPackage, KindCase, KindDefinition} {
		if selected[kind] {
			return kind, strings.Join(texts, "; "), true
		}
	}
	return "", "", false
}

// nearestStandardIsGeneral reports whether the standard view definition that
// decides view's kind is GeneralView itself, not a specialization of it.
func (r *Renderer) nearestStandardIsGeneral(view *symbols.Symbol) bool {
	for _, sym := range append([]*symbols.Symbol{view}, r.model.AllSupertypes(view)...) {
		fqn := r.fqn(sym)
		if _, ok := standardKind(standardViewDefinitions, viewDefinitionsPackage, fqn); ok {
			return fqn == generalViewName
		}
	}
	return false
}

// viewFilters are the conditions a view and the views it specializes state:
// each `filter` member and each filtered `expose`, in declaration order.
func (r *Renderer) viewFilters(view *symbols.Symbol) []symbols.ElementFilter {
	var out []symbols.ElementFilter
	for _, sym := range append([]*symbols.Symbol{view}, r.model.AllSupertypes(view)...) {
		if !semantics.IsView(sym) || sym.Scope == nil {
			continue
		}
		out = append(out, symbols.NamespaceFiltersIn(sym.Scope)...)
		for _, imp := range sym.Scope.Imports() {
			if imp.IsExpose && imp.FilterExpr != nil {
				out = append(out, symbols.ElementFilter{Expr: imp.FilterExpr, Scope: sym.Scope, Span: imp.FilterExpr.Span()})
			}
		}
	}
	return out
}

// classifyFilter records the graphs a compiled filter selects and spells it;
// false is a condition that is not a disjunction of metaclass classifications.
func (r *Renderer) classifyFilter(p *symbols.FilterPredicate, selected map[Kind]bool) (string, bool) {
	if p == nil {
		return "", false
	}
	switch p.Op {
	case symbols.FilterClassify, symbols.FilterMetaClassify:
		kind, ok := r.metaclassGraph(p.TypeFQN)
		if !ok {
			return "", false
		}
		if kind != "" {
			selected[kind] = true
		}
		op := "@"
		if p.Op == symbols.FilterMetaClassify {
			op = "@@"
		}
		return op + symbols.LastSegment(p.TypeFQN), true
	case symbols.FilterOr:
		parts := make([]string, 0, len(p.Operands))
		for _, operand := range p.Operands {
			text, ok := r.classifyFilter(operand, selected)
			if !ok {
				return "", false
			}
			parts = append(parts, text)
		}
		return strings.Join(parts, " or "), true
	}
	return "", false
}

// metaclassGraph is the graph a SysML or KerML metaclass selects (a case
// metaclass the case diagram a CaseView draws), "" for a
// relationship metaclass, which selects none; false for any other type.
func (r *Renderer) metaclassGraph(fqn string) (Kind, bool) {
	if !strings.HasPrefix(fqn, "SysML::") && !strings.HasPrefix(fqn, "KerML::") {
		return "", false
	}
	idx := r.resolver.Index()
	if idx == nil {
		return "", false
	}
	metas := idx.LookupQualified(fqn)
	if len(metas) == 0 {
		return "", false
	}
	names := map[string]bool{}
	for _, sym := range append([]*symbols.Symbol{metas[0]}, r.model.AllSupertypes(metas[0])...) {
		names[symbols.LastSegment(r.fqn(sym))] = true
	}
	has := func(set []string) bool { return slices.ContainsFunc(set, func(n string) bool { return names[n] }) }
	switch {
	case has(requirementMetaclasses):
		return KindRequirement, true
	case has(packageMetaclasses):
		return KindPackage, true
	case has(caseMetaclasses):
		return KindCase, true
	case has(relationshipMetaclasses):
		return "", true
	case has(definitionMetaclasses):
		return KindDefinition, true
	}
	return "", false
}

// generalGraph is one requirement, definition or package rendering being built.
type generalGraph struct {
	r     *Renderer
	view  *symbols.Symbol
	kind  Kind
	out   *Rendering
	ids   *nodeIDs
	nodes map[symbols.ElementKey]*Node
	order []*symbols.Symbol
	edges map[generalEdgeKey]bool
	// exposed is what the view exposes when the graph overlays a rendering of
	// another kind, whose ends it draws only when exposed; nil for a general graph.
	exposed map[symbols.ElementKey]bool
}

type generalEdgeKey struct {
	from, to, label string
	kind            EdgeKind
	// origin tells apart the import edges distinct declarations draw.
	origin Origin
}

// renderGeneral draws the exposed elements the graph's kind admits as nodes,
// in exposure order, and the relationships between them as edges.
func (r *Renderer) renderGeneral(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	g := &generalGraph{r: r, view: view, kind: out.Kind, out: out, ids: &nodeIDs{},
		nodes: map[symbols.ElementKey]*Node{}, edges: map[generalEdgeKey]bool{}}
	library, other := 0, 0
	viewInLibrary := view != nil && r.libraryDeclared(view)
	for _, sym := range exposed {
		switch {
		case !viewInLibrary && r.libraryDeclared(sym):
			library++
		case g.admits(sym):
			g.node(sym)
		default:
			other++
		}
	}
	exposedNodes := slices.Clone(g.order)
	var related []Edge
	if g.kind == KindRequirement {
		related = g.requirementRelationships()
	}
	for _, sym := range exposedNodes {
		if g.kind == KindPackage {
			g.packageEdges(sym)
			continue
		}
		g.structuralEdges(sym)
	}
	for _, edge := range related {
		g.add(edge)
	}
	if library > 0 {
		out.Notices = append(out.Notices, fmt.Sprintf("%d exposed standard library element(s) not drawn; a %s rendering draws the model's own elements", library, g.kind))
	}
	if other > 0 {
		out.Notices = append(out.Notices, fmt.Sprintf("%d exposed element(s) not drawn; a %s rendering draws %s", other, g.kind, generalDraws(g.kind)))
	}
	if r.verdicts != nil && g.kind != KindRequirement {
		out.Notices = append(out.Notices, fmt.Sprintf("verdicts are drawn on a requirement rendering, not on a %s rendering", g.kind))
	}
}

// generalDraws says what a graph of the kind draws as nodes.
func generalDraws(kind Kind) string {
	switch kind {
	case KindPackage:
		return "packages"
	case KindRequirement:
		return "requirements, and what relates to them"
	}
	return "definitions and usages"
}

// admits reports whether the graph draws sym as a node.
func (g *generalGraph) admits(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch g.kind {
	case KindPackage:
		return sym.Kind == symbols.SymbolPackage
	case KindRequirement:
		return isRequirement(sym) && generalNodeKind(g.r, sym)
	}
	return generalNodeKind(g.r, sym)
}

// generalNodeKind reports whether a requirement or definition graph draws sym
// as a node: a definition or usage that is no relationship, view or annotation.
func generalNodeKind(r *Renderer, sym *symbols.Symbol) bool {
	if !r.contentKind(sym) || semantics.IsView(sym) || generalRelationship(sym) {
		return false
	}
	switch sym.Decl.(type) {
	case *ast.Definition:
		return sym.Kind != symbols.SymbolMetadataDef
	case *ast.Usage:
		return sym.Kind != symbols.SymbolMetadataUsage
	case *ast.RequireMember, *ast.AssumeMember:
		// The constraint a `require constraint` or `assume constraint` member
		// declares; one naming a constraint instead is drawn as the edge to it.
		_, declares := ast.OwnedConstraintOf(sym.Decl)
		return declares && sym.Kind == symbols.SymbolConstraintUsage
	}
	return false
}

// generalRelationship reports whether sym is drawn as an edge rather than a
// node: a satisfy, a connector of any kind, a dependency.
func generalRelationship(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolSatisfyRequirementUsage, symbols.SymbolConnectionUsage, symbols.SymbolBindingUsage,
		symbols.SymbolSuccessionUsage, symbols.SymbolFlowUsage, symbols.SymbolInterfaceUsage,
		symbols.SymbolAllocationUsage, symbols.SymbolDependency, symbols.SymbolConnectorEnd:
		return true
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok {
		return usage.IsVerifiedRequirement()
	}
	return false
}

// node is the node drawing sym, made the first time it is asked for.
func (g *generalGraph) node(sym *symbols.Symbol) *Node {
	key := symbols.KeyOf(sym)
	if node, ok := g.nodes[key]; ok {
		return node
	}
	r := g.r
	node := &Node{ID: g.ids.take(), Kind: declKind(sym), Name: r.notationName(sym), NameSynthesized: r.model.NameSynthesized(sym),
		Type: r.headType(sym), Typings: r.headTypings(sym), Origin: symbolOrigin(sym),
		Geometry: r.geometryOf(g.view, sym, g.out), Style: r.styleOf(g.view, sym, g.out)}
	if g.kind == KindRequirement && isRequirement(sym) {
		node.Detail = r.requirementDetail(sym)
		g.overlayVerdicts(sym, node)
	}
	r.notesOf(g.view, sym, node.ID, g.out)
	g.nodes[key] = node
	g.order = append(g.order, sym)
	g.out.Roots = append(g.out.Roots, node)
	return node
}

// drawn is the node already drawing sym, nil when there is none.
func (g *generalGraph) drawn(sym *symbols.Symbol) *Node {
	if sym == nil {
		return nil
	}
	return g.nodes[symbols.KeyOf(sym)]
}

// add records an edge once: an import edge once per import declaration.
func (g *generalGraph) add(edge Edge) {
	key := generalEdgeKey{from: edge.From, to: edge.To, label: edge.text(), kind: edge.Kind}
	// Distinct declarations draw distinct import edges, and distinct exposed
	// relationships distinct edges, each with its own route.
	if edge.Kind == EdgeImport || g.exposed != nil {
		key.origin = edge.Origin
	}
	if g.edges[key] {
		return
	}
	g.edges[key] = true
	g.out.Edges = append(g.out.Edges, edge)
}

// isRequirement reports whether sym is a requirement definition or usage, a
// concern included.
func isRequirement(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolRequirementDef, symbols.SymbolRequirementUsage, symbols.SymbolConcernDef, symbols.SymbolConcernUsage:
		return true
	}
	return false
}

// requirementExcerptRunes bounds the documentation a requirement node quotes.
const requirementExcerptRunes = 60

// requirementDetail is what a requirement node says besides its name: its id,
// the short name, and the start of its documentation.
func (r *Renderer) requirementDetail(sym *symbols.Symbol) string {
	var parts []string
	if sym.ShortName != "" {
		parts = append(parts, "id "+sym.ShortName)
	}
	if docs := r.documentation(sym); len(docs) > 0 {
		parts = append(parts, "“"+excerpt(docs[0], requirementExcerptRunes)+"”")
	}
	return strings.Join(parts, ", ")
}

// excerpt is text on one line, cut at a word to at most limit runes, with an
// ellipsis when cut.
func excerpt(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	cut := string(runes[:limit])
	if i := strings.LastIndexByte(cut, ' '); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:.") + "..."
}

// resolveEnd resolves a relationship end written in scope, through an alias.
func (r *Renderer) resolveEnd(scope *symbols.Scope, target ast.Node) (*symbols.Symbol, bool) {
	if target == nil {
		return nil, false
	}
	sym, ok := r.resolver.ResolveTarget(scope, target)
	if !ok || sym == nil {
		return nil, false
	}
	if alias, ok := r.resolver.ResolveAliasTarget(sym); ok && alias != nil {
		sym = alias
	}
	return sym, true
}

// generalizationTarget is the element a specialization, subsetting,
// redefinition or typing relationship of sym names: a redefinition is resolved
// among the inherited features, a same-named subsetting reaches the inherited
// feature it subsets. False when the target does not resolve.
func (r *Renderer) generalizationTarget(sym *symbols.Symbol, rel *ast.Relationship) (*symbols.Symbol, bool) {
	var target *symbols.Symbol
	var resolved bool
	if rel.Kind == ast.RelRedefines {
		target, resolved = r.resolver.ResolveRedefinitionTarget(sym.OwnerScope, sym.Decl, rel.Target)
	} else {
		target, resolved = r.resolveEnd(sym.OwnerScope, rel.Target)
	}
	if !resolved || target == nil {
		return nil, false
	}
	if rel.Kind == ast.RelSubsets {
		if inherited := r.model.GeneralizationTargetOf(sym, rel); inherited != nil {
			target = inherited
		}
	}
	return target, true
}

// unresolved reports a relationship end that names nothing.
func (g *generalGraph) unresolved(relationship string, of *symbols.Symbol, target ast.Node) {
	g.out.Notices = append(g.out.Notices, fmt.Sprintf("the %s of %s names %s, which does not resolve; not drawn",
		relationship, g.r.relationshipOwnerName(of), qualifiedText(target)))
}

// relationshipOwnerName names a relationship by the element it is, or by the
// nearest named element owning an anonymous one.
func (r *Renderer) relationshipOwnerName(sym *symbols.Symbol) string {
	for s := sym; s != nil; {
		if s.Name != "" {
			return r.notationName(s)
		}
		if s.OwnerScope == nil {
			break
		}
		s = s.OwnerScope.Owner()
	}
	return "the model"
}

// structuralEdges draws what a definition or usage node states of itself:
// its specializations and typings, and its ownership by a drawn owner.
func (g *generalGraph) structuralEdges(sym *symbols.Symbol) {
	from := g.drawn(sym)
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil {
			continue
		}
		kind, label, ok := structuralEdgeKind(rel.Kind)
		if !ok {
			continue
		}
		target, resolved := g.r.generalizationTarget(sym, rel)
		if !resolved {
			g.unresolved(rel.Kind.String()+" relationship", sym, rel.Target)
			continue
		}
		if to := g.drawn(target); to != nil && to != from {
			g.add(Edge{From: from.ID, To: to.ID, Kind: kind, Label: label, Origin: symbolOrigin(sym)})
		}
	}
	g.featureEdges(sym, from)
	g.statedEdges(sym, from)
	if sym.OwnerScope == nil || !ownedUsage(sym) {
		return
	}
	if owner := g.drawn(sym.OwnerScope.Owner()); owner != nil && owner != from {
		kind, label := membershipEdge(sym)
		g.add(Edge{From: owner.ID, To: from.ID, Kind: kind, Label: label, Origin: symbolOrigin(sym)})
	}
}

// ownedUsage reports whether sym is a usage its owner draws a membership edge
// to: one declared as such, or the constraint a require or assume member declares.
func ownedUsage(sym *symbols.Symbol) bool {
	switch sym.Decl.(type) {
	case *ast.Usage, *ast.RequireMember, *ast.AssumeMember:
		return true
	}
	return false
}

// membershipEdge is the edge a drawn owner draws to a member it owns, with its
// label: the notation's require, assume, assert, frame, event and portion
// glyphs for the memberships that have one, else the composition (filled
// diamond) or reference (hollow diamond) of a feature membership.
func membershipEdge(member *symbols.Symbol) (EdgeKind, string) {
	switch decl := member.Decl.(type) {
	case *ast.RequireMember:
		return EdgeRequire, EdgeRequire.String()
	case *ast.AssumeMember:
		return EdgeAssume, EdgeAssume.String()
	case *ast.Usage:
		switch {
		case decl.PrefixKeyword == "assert":
			return EdgeAssert, EdgeAssert.String()
		case decl.Kind == ast.UsageFramedConcern:
			return EdgeFrame, EdgeFrame.String()
		case decl.IsEvent:
			return EdgeEvent, EdgeEvent.String()
		case decl.Portion != ast.PortionNone:
			return EdgePortion, ""
		}
	}
	if compositeUsage(member) {
		return EdgeComposition, ""
	}
	return EdgeReference, ""
}

// statedEdges draws what a member of sym states of another drawn element by
// naming it rather than declaring it: `require r1;`, `assume c;`, `assert c;`
// and `frame concern;`, from sym to the named element.
func (g *generalGraph) statedEdges(sym *symbols.Symbol, from *Node) {
	if sym.Scope == nil {
		return
	}
	for _, member := range append(slices.Clone(sym.Scope.Members()), sym.Scope.AnonymousMembers()...) {
		kind, ref, ok := statedReference(member)
		if !ok {
			continue
		}
		target, resolved := g.r.resolveEnd(member.OwnerScope, ref)
		if !resolved {
			g.unresolved(kind.String()+" membership", sym, ref)
			continue
		}
		if to := g.drawn(target); to != nil && to != from {
			g.add(Edge{From: from.ID, To: to.ID, Kind: kind, Label: kind.String(), Origin: symbolOrigin(member)})
		}
	}
}

// statedReference is the element a require, assume, assert or frame member
// names rather than declares, and the edge the membership draws to it.
func statedReference(member *symbols.Symbol) (EdgeKind, ast.Node, bool) {
	switch decl := member.Decl.(type) {
	case *ast.RequireMember:
		ref := ast.ConstraintReferenceOf(decl)
		return EdgeRequire, ref, ref != nil
	case *ast.AssumeMember:
		ref := ast.ConstraintReferenceOf(decl)
		return EdgeAssume, ref, ref != nil
	case *ast.Usage:
		rel := decl.ReferenceSubsetting()
		if rel == nil || rel.Target == nil {
			return 0, nil, false
		}
		switch {
		case decl.PrefixKeyword == "assert":
			return EdgeAssert, rel.Target, true
		case decl.Kind == ast.UsageFramedConcern:
			return EdgeFrame, rel.Target, true
		}
	}
	return 0, nil, false
}

// structuralEdgeKind is the edge a declared relationship draws, and its label.
// featureEdges draws each feature of sym that is not itself drawn as an edge
// from sym to the drawn types of the feature, named by the feature: the
// association a block diagram draws for a part property.
func (g *generalGraph) featureEdges(sym *symbols.Symbol, from *Node) {
	if sym.Scope == nil {
		return
	}
	for _, member := range sym.Scope.Members() {
		if _, usage := member.Decl.(*ast.Usage); !usage || g.drawn(member) != nil || !generalNodeKind(g.r, member) {
			continue
		}
		kind := EdgeComposition
		if !compositeUsage(member) {
			kind = EdgeReference
		}
		for _, typ := range g.r.model.DeclaredTypes(member) {
			if to := g.drawn(typ); to != nil {
				g.add(Edge{From: from.ID, To: to.ID, Kind: kind, ToLabel: symbols.LastSegment(g.r.fqn(member)), Origin: symbolOrigin(member)})
			}
		}
	}
}

func structuralEdgeKind(kind ast.RelationshipKind) (EdgeKind, string, bool) {
	switch kind {
	case ast.RelSpecializes:
		return EdgeSpecialization, "", true
	case ast.RelSubsets:
		return EdgeSpecialization, "subsets", true
	case ast.RelRedefines:
		return EdgeSpecialization, "redefines", true
	case ast.RelReferences:
		return EdgeReferenceSubsetting, "references", true
	case ast.RelTyping:
		return EdgeTyping, "", true
	}
	return 0, "", false
}

// compositeUsage reports whether a usage is composite: one not declared `ref`
// and not an attribute or reference usage, which are always referential.
func compositeUsage(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolAttributeUsage, symbols.SymbolReferenceUsage, symbols.SymbolEnumerationUsage:
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && !usage.IsReference
}

// packageEdges draws a package's containment by a drawn package and its
// imports of drawn packages, or of members of them.
func (g *generalGraph) packageEdges(pkg *symbols.Symbol) {
	node := g.drawn(pkg)
	if pkg.OwnerScope != nil {
		if owner := g.drawn(pkg.OwnerScope.Owner()); owner != nil && owner != node {
			g.add(Edge{From: owner.ID, To: node.ID, Kind: EdgeContainment, Origin: symbolOrigin(pkg)})
		}
	}
	if pkg.Scope == nil {
		return
	}
	for _, member := range pkg.Scope.Members() {
		if member.Kind != symbols.SymbolAlias {
			continue
		}
		target, ok := g.r.resolver.ResolveAliasTarget(member)
		if !ok || target == nil {
			if alias, isAlias := member.Decl.(*ast.Alias); isAlias && alias.For != nil {
				g.unresolved("alias", pkg, alias.For)
			}
			continue
		}
		if to := g.drawn(target); to != nil && to != node {
			g.add(Edge{From: node.ID, To: to.ID, Kind: EdgeUnownedMembership, ToLabel: member.Name, Origin: symbolOrigin(member)})
		}
	}
	for _, imp := range pkg.Scope.Imports() {
		if imp == nil || imp.IsExpose || imp.Imported == nil {
			continue
		}
		target, ok := g.r.resolver.ImportTarget(pkg.Scope, imp)
		if !ok || target == nil {
			g.unresolved("import", pkg, imp.Imported)
			continue
		}
		label := importLabel(imp)
		to := g.drawn(target)
		if to == nil && target.OwnerScope != nil && imp.Kind == ast.ImportMembership {
			if to = g.drawn(target.OwnerScope.Owner()); to != nil {
				label += " ::" + target.Name
			}
		}
		if to != nil && to != node {
			g.add(Edge{From: node.ID, To: to.ID, Kind: EdgeImport, Label: label, Origin: nodeOrigin(pkg.DocName, imp)})
		}
	}
}

// importLabel is an import edge's label: what the import brings in.
func importLabel(imp *ast.Import) string {
	label := "import"
	if imp.Visibility == ast.VisibilityPrivate {
		label = "private import"
	}
	return label + importSuffix(imp)
}

// exposeLabel is an expose edge's label, suffixed as an import's is.
func exposeLabel(imp *ast.Import) string {
	return "expose" + importSuffix(imp)
}

// importSuffix is what an import or expose brings in beyond the named element.
func importSuffix(imp *ast.Import) string {
	switch {
	case imp.Kind == ast.ImportNamespace && imp.IsRecursive:
		return " ::*::**"
	case imp.Kind == ast.ImportNamespace:
		return " ::*"
	case imp.IsRecursive:
		return " ::**"
	}
	return ""
}

// requirementRelationships finds, in declaration order across the model's own
// documents, the satisfy, verify, derive, refine and allocate relationships
// with an end the rendering draws, drawing the other end beside it.
func (g *generalGraph) requirementRelationships() []Edge {
	var out []Edge
	for _, sym := range g.r.modelElements() {
		switch {
		case sym.Kind == symbols.SymbolSatisfyRequirementUsage:
			out = append(out, g.satisfyEdges(sym)...)
		case sym.Kind == symbols.SymbolVerificationCaseDef || sym.Kind == symbols.SymbolVerificationCaseUsage:
			out = append(out, g.verifyEdges(sym)...)
		case sym.Kind == symbols.SymbolAllocationUsage:
			out = append(out, g.allocateEdges(sym)...)
		case sym.Kind == symbols.SymbolConnectionUsage && g.r.annotatedBy(sym, derivationMetadata):
			out = append(out, g.deriveEdges(sym)...)
		case sym.Kind == symbols.SymbolDependency && g.r.annotatedBy(sym, refinementMetadata):
			out = append(out, g.refineEdges(sym)...)
		}
	}
	return out
}

// The library metadata that marks a derivation connection and its ends
// (RequirementDerivation) and a refinement dependency (ModelingMetadata).
const (
	derivationMetadata = "RequirementDerivation::DerivationMetadata"
	originalMetadata   = "RequirementDerivation::OriginalRequirementMetadata"
	derivedMetadata    = "RequirementDerivation::DerivedRequirementMetadata"
	refinementMetadata = "ModelingMetadata::Refinement"
)

// annotatedBy reports whether metadata of the named library type annotates sym.
func (r *Renderer) annotatedBy(sym *symbols.Symbol, typeFQN string) bool {
	for _, md := range r.model.ElementMetadataOf(sym) {
		if r.fqn(md.Type) == typeFQN {
			return true
		}
	}
	return false
}

// modelElements are the elements the model's own documents declare, each
// document in index order and each body in declaration order.
func (r *Renderer) modelElements() []*symbols.Symbol {
	idx := r.resolver.Index()
	if idx == nil {
		return nil
	}
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	var walk func(scope *symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil {
			return
		}
		members := append(slices.Clone(scope.Members()), scope.AnonymousMembers()...)
		sort.SliceStable(members, func(i, j int) bool { return members[i].DeclSpan.Offset < members[j].DeclSpan.Offset })
		for _, member := range members {
			if seen[member] || member.OwnerScope != scope {
				continue
			}
			seen[member] = true
			out = append(out, member)
			walk(member.Scope)
		}
	}
	for _, doc := range idx.Documents() {
		if !idx.IsLibraryDocument(doc) {
			walk(idx.DocumentRoot(doc))
		}
	}
	return out
}

// relate is the edge between two relationship ends when the rendering draws
// one of them, drawing the other beside it; a library element is not drawn.
// Overlaying another rendering, it draws an undrawn end only when exposed.
func (g *generalGraph) relate(from, to *symbols.Symbol, kind EdgeKind, origin *symbols.Symbol) (Edge, bool) {
	if g.exposed == nil && g.drawn(from) == nil && g.drawn(to) == nil {
		return Edge{}, false
	}
	for _, end := range []*symbols.Symbol{from, to} {
		if g.drawn(end) != nil {
			continue
		}
		if g.r.libraryDeclared(end) || g.exposed != nil && !g.exposed[symbols.KeyOf(end)] {
			return Edge{}, false
		}
	}
	return Edge{From: g.node(from).ID, To: g.node(to).ID, Kind: kind, Label: kind.String(), Origin: symbolOrigin(origin)}, true
}

// resolvedEnds resolves both ends of a relationship, reporting an end that
// does not resolve when the other is drawn.
func (g *generalGraph) resolvedEnds(name string, rel *symbols.Symbol, scope *symbols.Scope, from, to ast.Node) (*symbols.Symbol, *symbols.Symbol, bool) {
	source, okFrom := g.r.resolveEnd(scope, from)
	target, okTo := g.r.resolveEnd(scope, to)
	switch {
	case okFrom && okTo:
		return source, target, true
	case okFrom && g.drawn(source) != nil && to != nil:
		g.unresolved(name, rel, to)
	case okTo && g.drawn(target) != nil && from != nil:
		g.unresolved(name, rel, from)
	}
	return nil, nil, false
}

// satisfyEdges draws `satisfy R by x`: from x, or from the element owning the
// satisfy when it names no subject, to R.
func (g *generalGraph) satisfyEdges(sym *symbols.Symbol) []Edge {
	ends, ok := semantics.SatisfyEndsOf(sym)
	if !ok {
		return nil
	}
	requirement := sym
	if !ends.DeclaresRequirement {
		if ends.Requirement == nil {
			return nil
		}
		resolved, ok := g.r.resolveEnd(sym.OwnerScope, ends.Requirement)
		if !ok {
			return nil
		}
		requirement = resolved
	} else {
		requirement = g.declaredRequirement(sym)
	}
	var satisfier *symbols.Symbol
	if ends.Satisfier != nil {
		resolved, ok := g.r.resolveEnd(sym.OwnerScope, ends.Satisfier)
		if !ok {
			if g.drawn(requirement) != nil {
				g.unresolved("satisfy", sym, ends.Satisfier)
			}
			return nil
		}
		satisfier = resolved
	} else if sym.OwnerScope != nil {
		satisfier = sym.OwnerScope.Owner()
	}
	if satisfier == nil {
		return nil
	}
	if edge, ok := g.relate(satisfier, requirement, EdgeSatisfy, sym); ok {
		return []Edge{edge}
	}
	return nil
}

// verifyEdges draws, from a verification case, an edge to each requirement
// the `verify` members of its objectives name.
func (g *generalGraph) verifyEdges(verification *symbols.Symbol) []Edge {
	var out []Edge
	for _, objective := range g.objectivesOf(verification) {
		for _, member := range append(slices.Clone(objective.Scope.Members()), objective.Scope.AnonymousMembers()...) {
			for _, requirement := range g.verified(verification, member) {
				if edge, ok := g.relate(verification, requirement, EdgeVerify, member); ok {
					out = append(out, edge)
				}
			}
		}
	}
	return out
}

// objectivesOf is the objectives a verification case runs, as the runtime
// places them: inherited ones first, each replaced by one redefining or renaming it.
func (g *generalGraph) objectivesOf(verification *symbols.Symbol) []*symbols.Symbol {
	m := g.r.model
	var out []*symbols.Symbol
	place := func(obj *symbols.Symbol) {
		for i, prev := range out {
			if prev == obj || slices.Contains(m.AllRedefinedFeatures(prev), obj) {
				return
			}
			name := m.EffectiveNameOf(obj)
			if slices.Contains(m.AllRedefinedFeatures(obj), prev) || name != "" && name == m.EffectiveNameOf(prev) {
				out[i] = obj
				return
			}
		}
		out = append(out, obj)
	}
	sources := m.MemberSources(verification)
	for i := len(sources) - 1; i >= 0; i-- {
		for _, obj := range ownObjectives(sources[i]) {
			place(obj)
		}
	}
	for _, obj := range ownObjectives(verification) {
		place(obj)
	}
	return out
}

// ownObjectives is the objectives sym declares in its own body.
func ownObjectives(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range append(slices.Clone(sym.Scope.Members()), sym.Scope.AnonymousMembers()...) {
		if usage, ok := member.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageObjective && member.Scope != nil {
			out = append(out, member)
		}
	}
	return out
}

// verified is the requirement a `verify` member names: the one it subsets, or
// the definition a declared requirement is typed by.
func (g *generalGraph) verified(verification, member *symbols.Symbol) []*symbols.Symbol {
	usage, ok := member.Decl.(*ast.Usage)
	if !ok || !usage.IsVerifiedRequirement() {
		return nil
	}
	if usage.DeclaresRequirement {
		return []*symbols.Symbol{g.declaredRequirement(member)}
	}
	rel := usage.ReferenceSubsetting()
	if rel == nil || rel.Target == nil {
		return nil
	}
	target, ok := g.r.resolveEnd(member.OwnerScope, rel.Target)
	if !ok {
		if g.drawn(verification) != nil {
			g.unresolved("verify", member, rel.Target)
		}
		return nil
	}
	return []*symbols.Symbol{target}
}

// allocateEdges draws a binary `allocate x to y` from x to y.
func (g *generalGraph) allocateEdges(sym *symbols.Symbol) []Edge {
	ends := g.r.model.ConnectorEndAttachments(sym)
	if len(ends) != 2 {
		return nil
	}
	source, target, ok := g.resolvedEnds("allocation", sym, sym.OwnerScope, ends[0].Attachment, ends[1].Attachment)
	if !ok {
		return nil
	}
	if edge, ok := g.relate(source, target, EdgeAllocate, sym); ok {
		return []Edge{edge}
	}
	return nil
}

// deriveEdges draws, for a #derivation connection, an edge from each #derive
// end's requirement to each #original end's.
func (g *generalGraph) deriveEdges(sym *symbols.Symbol) []Edge {
	if sym.Scope == nil {
		return nil
	}
	var originals, derived []*symbols.Symbol
	for _, end := range append(slices.Clone(sym.Scope.Members()), sym.Scope.AnonymousMembers()...) {
		usage, ok := end.Decl.(*ast.Usage)
		if !ok || !usage.IsEnd {
			continue
		}
		isOriginal, isDerived := g.r.annotatedBy(end, originalMetadata), g.r.annotatedBy(end, derivedMetadata)
		if !isOriginal && !isDerived {
			continue
		}
		target, ok := g.endTarget(sym, end, "derivation")
		if !ok {
			continue
		}
		if isOriginal {
			originals = append(originals, target)
		} else {
			derived = append(derived, target)
		}
	}
	var out []Edge
	for _, d := range derived {
		for _, o := range originals {
			if edge, ok := g.relate(d, o, EdgeDerive, sym); ok {
				out = append(out, edge)
			}
		}
	}
	return out
}

// declaredRequirement is the requirement a member declaring one stands for:
// the type it is declared with, else the requirement it subsets or redefines,
// else the member itself.
func (g *generalGraph) declaredRequirement(member *symbols.Symbol) *symbols.Symbol {
	if types := g.r.model.DeclaredTypes(member); len(types) > 0 {
		return types[0]
	}
	for _, rel := range semantics.RelationshipsOf(member) {
		if rel == nil || rel.Target == nil || rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines {
			continue
		}
		if target, ok := g.r.generalizationTarget(member, rel); ok {
			return target
		}
	}
	return member
}

// endTarget is the element an end of the relationship rel names, by reference
// subsetting or by subsetting; one naming no element is skipped, one naming
// an element that does not resolve is noticed.
func (g *generalGraph) endTarget(rel, end *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	usage, ok := end.Decl.(*ast.Usage)
	if !ok {
		return nil, false
	}
	ref := usage.ReferenceSubsetting()
	if ref == nil {
		for _, r := range semantics.RelationshipsOf(end) {
			if r != nil && r.Target != nil && r.Kind == ast.RelSubsets {
				ref = r
				break
			}
		}
	}
	if ref == nil || ref.Target == nil {
		return nil, false
	}
	target, ok := g.r.resolveEnd(end.OwnerScope, ref.Target)
	if !ok {
		g.unresolved(name, rel, ref.Target)
		return nil, false
	}
	return target, true
}

// derivation reports whether a connection, or a connection def, has an end
// annotated as a derivation's original or derived requirement.
func (g *generalGraph) derivation(sym *symbols.Symbol) bool {
	if sym.Scope == nil {
		return false
	}
	for _, end := range append(slices.Clone(sym.Scope.Members()), sym.Scope.AnonymousMembers()...) {
		if g.r.annotatedBy(end, originalMetadata) || g.r.annotatedBy(end, derivedMetadata) {
			return true
		}
	}
	return false
}

// refineEdges draws a #refinement dependency from each client to each supplier.
func (g *generalGraph) refineEdges(sym *symbols.Symbol) []Edge {
	return g.dependencyEdges(sym, EdgeRefine, "refinement")
}

// dependencyEdges draws a dependency as an edge of the kind from each client
// to each supplier.
func (g *generalGraph) dependencyEdges(sym *symbols.Symbol, kind EdgeKind, name string) []Edge {
	dep, ok := sym.Decl.(*ast.Dependency)
	if !ok {
		return nil
	}
	var out []Edge
	for _, client := range dep.Clients {
		for _, supplier := range dep.Suppliers {
			source, target, ok := g.resolvedEnds(name, sym, sym.OwnerScope, client, supplier)
			if !ok {
				continue
			}
			edge, ok := g.relate(source, target, kind, sym)
			if !ok {
				continue
			}
			if kind == EdgeDependency {
				edge.Label = g.dependencyLabel(sym)
			}
			out = append(out, edge)
		}
	}
	return out
}

// dependencyLabel is the name a dependency of no particular kind is drawn
// with, which the notation makes optional; a name a migration made up is none.
func (g *generalGraph) dependencyLabel(sym *symbols.Symbol) string {
	if sym.Name == "" || g.r.model.NameSynthesized(sym) {
		return ""
	}
	return localName(sym)
}

// connectionEdges draws a binary connection between the elements it connects,
// labelled with its name.
func (g *generalGraph) connectionEdges(sym *symbols.Symbol) []Edge {
	ends := g.r.model.ConnectorEndAttachments(sym)
	if len(ends) != 2 {
		return nil
	}
	source, target, ok := g.resolvedEnds("connection", sym, sym.OwnerScope, ends[0].Attachment, ends[1].Attachment)
	if !ok {
		return nil
	}
	edge, ok := g.relate(source, target, EdgeConnection, sym)
	if !ok {
		return nil
	}
	edge.Label = g.r.connectorLabel(sym)
	return []Edge{edge}
}

// verifyMemberEdges draws, from the verification case whose objective holds
// a `verify` member, an edge to each requirement the member names.
func (g *generalGraph) verifyMemberEdges(member *symbols.Symbol) []Edge {
	objective := member.Owner()
	if objective == nil || objective.Owner() == nil {
		return nil
	}
	verification := objective.Owner()
	var out []Edge
	for _, requirement := range g.verified(verification, member) {
		if edge, ok := g.relate(verification, requirement, EdgeVerify, member); ok {
			out = append(out, edge)
		}
	}
	return out
}

// relationshipEdges are the edges a dependency-like relationship draws: a
// verify member, a satisfy, an allocation, a dependency (a refinement when so
// annotated), a connection or connection def whose ends are a derivation's, a
// plain connection. ok is false when sym is no such relationship.
func (g *generalGraph) relationshipEdges(sym *symbols.Symbol) (edges []Edge, ok bool) {
	if usage, isUsage := sym.Decl.(*ast.Usage); isUsage && usage.IsVerifiedRequirement() {
		return g.verifyMemberEdges(sym), true
	}
	switch {
	case sym.Kind == symbols.SymbolSatisfyRequirementUsage:
		return g.satisfyEdges(sym), true
	case sym.Kind == symbols.SymbolAllocationUsage:
		return g.allocateEdges(sym), true
	case sym.Kind == symbols.SymbolDependency && g.r.annotatedBy(sym, refinementMetadata):
		return g.refineEdges(sym), true
	case sym.Kind == symbols.SymbolDependency:
		return g.dependencyEdges(sym, EdgeDependency, "dependency"), true
	case sym.Kind == symbols.SymbolConnectionDef && g.derivation(sym):
		return g.deriveEdges(sym), true
	case sym.Kind == symbols.SymbolConnectionUsage && (g.r.annotatedBy(sym, derivationMetadata) || g.derivation(sym)):
		return g.deriveEdges(sym), true
	case sym.Kind == symbols.SymbolConnectionUsage:
		return g.connectionEdges(sym), true
	}
	return nil, false
}

// exposedRelationships draws, on a rendering of another kind whose nodes drawn
// stand for their elements, the dependency-like relationships view exposes
// between the nodes of their ends, following their Route annotations: the
// graphical notation draws a dependency between any two element nodes a view
// shows. An exposed end no node draws yet gets a plain node, so the line has
// both ends. It returns the exposed elements drawn this way.
func (r *Renderer) exposedRelationships(view *symbols.Symbol, exposed []*symbols.Symbol, ids *nodeIDs,
	drawn map[symbols.ElementKey]*Node, out *Rendering) map[*symbols.Symbol]bool {
	g := &generalGraph{r: r, view: view, kind: out.Kind, out: out, ids: ids, nodes: drawn,
		edges: map[generalEdgeKey]bool{}, exposed: map[symbols.ElementKey]bool{}}
	for _, sym := range exposed {
		g.exposed[symbols.KeyOf(sym)] = true
	}
	used := map[*symbols.Symbol]bool{}
	for _, sym := range exposed {
		edges, ok := g.relationshipEdges(sym)
		if !ok {
			continue
		}
		used[sym] = true
		if len(edges) == 0 {
			out.Notices = append(out.Notices, fmt.Sprintf("%s %s is drawn between the nodes of its ends; the view does not show both, so no edge is drawn",
				declKind(sym), r.notationName(sym)))
		}
		for _, edge := range edges {
			edge.Route = r.routeOf(view, sym, out)
			edge.Style = r.edgeDress(view, sym, edge.From, edge.To, out)
			g.add(edge)
		}
	}
	for _, sym := range g.order {
		used[sym] = true
	}
	return used
}
