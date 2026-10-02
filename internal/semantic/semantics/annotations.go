package semantics

import (
	"sort"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The metadata annotating an element is what an element filter classifies it by
// (`@Safety`), and SysML v2 7.27 lets it be written in three ways, all of which
// count here:
//
//   - prefix metadata on the declaration, `#Safety part def P;` and the
//     `@Safety{...}` form written inside the element's body;
//   - a metadata usage in the element's body, `part p { metadata safety : Safety; }`;
//   - a metadata usage annotating the element from elsewhere,
//     `metadata safety : Safety about p;`.
//
// An element is also classified by its own metaclass — a part usage by
// SysML::PartUsage — which is what the reflective metadata types in the standard
// library name, and what the corpus filters on (`filter @SysML::PartUsage`).
// That is answered by metaclassOf rather than collected here, since it is not a
// declared annotation.

// annotation is one metadata annotation of an element: the metadata type
// annotating it and the values its body binds that type's features to.
type annotation struct {
	typ *symbols.Symbol
	// bound is what the body binds, each feature to the sequence of values its
	// expression lists; defaults is typ's declared values, one map shared by
	// every annotation of typ, read for what the body leaves unbound.
	bound    map[string][]symbols.FilterValue
	defaults map[string][]symbols.FilterValue
	// node states the annotation: the prefix-metadata node or metadata usage.
	node ast.Node
	// span locates node, or the node a recorded annotation was written from.
	span source.Span
	// scope is where the annotating node is declared.
	scope *symbols.Scope
	// about marks an annotation stated elsewhere with an `about` clause.
	about bool
	// via is the namespace an `about` clause named the element through
	// (`about Acquire::start` names start via Acquire); nil for an unqualified one.
	via *symbols.Symbol
	// recorded marks an annotation read from an interface record: it has no
	// node, its document being held without its tree.
	recorded bool
}

// annotationsOf returns the metadata annotating sym, memoized: an element filter
// asks for it once per candidate and per import enumeration.
func (m *Model) annotationsOf(sym *symbols.Symbol) []annotation {
	if sym == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.annotations[sym]; ok {
		return cached
	}
	// Recorded first so that a value that resolves back to this element cannot
	// re-enter the collection of its own annotations.
	journal(m, m.annotations, sym, sym.Decl)
	m.annotations[sym] = nil

	var out []annotation
	if sym.Recorded() {
		out = append(out, m.recordedAnnotations(sym)...)
		out = append(out, m.aboutAnnotations(sym)...)
	} else if sym.Decl != nil {
		out = append(out, m.declaredAnnotations(sym)...)
		out = append(out, m.aboutAnnotations(sym)...)
	}
	m.annotations[sym] = out
	return out
}

// recordedAnnotations restores the annotations a recorded symbol's own
// declaration stated, from the facts its record carries.
func (m *Model) recordedAnnotations(sym *symbols.Symbol) []annotation {
	var out []annotation
	for _, facts := range sym.Facts.Annotations {
		a, ok := m.annotationFromFacts(facts, sym.OwnerScope)
		if ok {
			out = append(out, a)
		}
	}
	return out
}

// annotationFromFacts is the annotation a fact states; the values it carries
// are the ones the declaration bound or its type defaulted, already evaluated.
func (m *Model) annotationFromFacts(facts symbols.AnnotationFacts, scope *symbols.Scope) (annotation, bool) {
	if facts.Type.IsZero() {
		return annotation{}, false
	}
	typ := m.recordedElement(facts.Type)
	if typ == nil {
		return annotation{}, false
	}
	bound := make(map[string][]symbols.FilterValue, len(facts.Values))
	for _, v := range facts.Values {
		bound[v.Feature] = v.Values
	}
	return annotation{typ: typ, bound: bound, span: facts.Span, scope: scope, recorded: true}, true
}

// DeclaredAnnotationFactsOf is AnnotationFactsOf over the annotations sym's own
// declaration states — what its interface record carries; an `about` annotation
// stated elsewhere travels with the document stating it.
func (m *Model) DeclaredAnnotationFactsOf(sym *symbols.Symbol) []symbols.AnnotationFacts {
	if sym == nil || sym.Decl == nil {
		return nil
	}
	return annotationFacts(m, m.declaredAnnotations(sym))
}

// AboutAnnotationFactsOf is the annotation a metadata usage with an `about`
// clause states on the elements it names — its type and bound values — as an
// interface record keeps it; nil for any other symbol.
func (m *Model) AboutAnnotationFactsOf(sym *symbols.Symbol) *symbols.AnnotationFacts {
	if sym == nil || sym.Kind != symbols.SymbolMetadataUsage {
		return nil
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !annotatesOthers(usage) {
		return nil
	}
	a, ok := m.usageAnnotation(sym.OwnerScope, usage)
	if !ok {
		return nil
	}
	facts := annotationFacts(m, []annotation{a})
	if len(facts) == 0 {
		return nil
	}
	return &facts[0]
}

// AnnotatedElementsOf returns the elements a metadata usage annotates through
// its `about` clause, resolved; nil for any other symbol.
func (m *Model) AnnotatedElementsOf(sym *symbols.Symbol) []*symbols.Symbol {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || sym.Kind != symbols.SymbolMetadataUsage || !annotatesOthers(usage) {
		return nil
	}
	targets := m.annotatedElements(sym.OwnerScope, usage)
	out := make([]*symbols.Symbol, len(targets))
	for i, target := range targets {
		out[i] = target.sym
	}
	return out
}

// AnnotationFactsOf states the metadata annotating sym as names and constants,
// so that how an element filter classifies it can be compared across loads. The
// values an annotation binds are reported as read; a binding whose value is not
// constant is reported with an unknown value, which a condition reading it
// reports as unevaluable rather than silently treating as absent.
func (m *Model) AnnotationFactsOf(sym *symbols.Symbol) []symbols.AnnotationFacts {
	return annotationFacts(m, m.annotationsOf(sym))
}

// annotationFacts states annotations as names and constants. Type is the
// reference that restores the metadata type, zero when none reaches it.
func annotationFacts(m *Model, annots []annotation) []symbols.AnnotationFacts {
	var out []symbols.AnnotationFacts
	for _, a := range annots {
		var typFQN string
		if a.typ != nil {
			typFQN = m.fqnOf(a.typ)
		}
		if typFQN == "" {
			continue
		}
		facts := symbols.AnnotationFacts{TypeFQN: typFQN, Span: a.span}
		if m.resolver != nil && m.resolver.Index() != nil {
			facts.Type, _ = m.resolver.Index().RefTo(a.typ)
		}
		for _, feature := range a.featureNames() {
			facts.Values = append(facts.Values, a.valueFacts(feature))
		}
		out = append(out, facts)
	}
	return out
}

// AnnotationSite is one metadata annotation of an element together with the
// node stating it — a prefix/body annotation of the element itself, or a
// metadata usage annotating it from elsewhere with an `about` clause.
type AnnotationSite struct {
	TypeFQN string
	Node    ast.Node
	// Span locates the node stating the annotation, also when the annotation
	// comes from a record and Node is nil.
	Span source.Span
	// Scope is where the annotating node is declared; for an `about`-form
	// annotation that may be another document than the annotated element's.
	Scope  *symbols.Scope
	About  bool
	Values []symbols.AnnotationValueFacts
}

// AnnotationSitesOf returns the metadata annotating sym with the nodes stating
// it, inline annotations first and `about`-form ones after, each in
// declaration order. An annotation a recorded document states, on its own
// declaration or `about` an element elsewhere, has no node: its record carries
// the type, values and span, not the tree.
func (m *Model) AnnotationSitesOf(sym *symbols.Symbol) []AnnotationSite {
	var out []AnnotationSite
	for _, a := range m.annotationsOf(sym) {
		var typFQN string
		if a.typ != nil {
			typFQN = m.fqnOf(a.typ)
		}
		if typFQN == "" || (a.node == nil && !a.recorded) {
			continue
		}
		site := AnnotationSite{TypeFQN: typFQN, Node: a.node, Span: a.span, Scope: a.scope, About: a.about}
		for _, feature := range a.featureNames() {
			site.Values = append(site.Values, a.valueFacts(feature))
		}
		out = append(out, site)
	}
	return out
}

// MetadataBinding is one feature an annotation body binds: the value it is bound
// to, if any, and the bindings its own body nests under it.
type MetadataBinding struct {
	Feature string
	Value   ast.Node
	// Node is the body member stating the binding.
	Node *ast.Usage
	// Scope is where the value resolves names: the body's own scope, which sees
	// the metadata type's members before those around the annotated element.
	Scope  *symbols.Scope
	Nested []MetadataBinding
}

// ElementMetadata is one metadata annotation of an element as `.metadata` reads
// it: the metadata type to materialize and the values its body binds. Values
// the body leaves unbound come from the type's own declarations.
type ElementMetadata struct {
	Type *symbols.Symbol
	Node ast.Node
	// Doc is the document stating the annotation, which an `about` form states
	// away from the element it annotates.
	Doc      string
	About    bool
	Bindings []MetadataBinding
}

// ElementMetadataOf returns the metadata annotating sym — the same side table
// an element filter classifies by — in textual order: by the document stating
// each annotation (in the index's document order), then by source position, so
// an `about` annotation written before an inline one comes first.
func (m *Model) ElementMetadataOf(sym *symbols.Symbol) []ElementMetadata {
	var out []ElementMetadata
	for _, a := range m.annotationsOf(sym) {
		if a.typ == nil || a.node == nil {
			continue
		}
		out = append(out, ElementMetadata{
			Type:     a.typ,
			Node:     a.node,
			Doc:      symbols.DocNameOf(a.scope),
			About:    a.about,
			Bindings: metadataBindings(valueScope(a.scope, a.node), metadataBody(a.node)),
		})
	}
	if len(out) < 2 {
		return out
	}
	rank := m.documentRanks()
	sort.SliceStable(out, func(i, j int) bool {
		if ri, rj := rank[out[i].Doc], rank[out[j].Doc]; ri != rj {
			return ri < rj
		}
		return out[i].Node.Span().Offset < out[j].Node.Span().Offset
	})
	return out
}

// documentRanks orders the documents of the index as Index.Documents lists
// them; a document the index does not hold sorts after every one it does.
func (m *Model) documentRanks() map[string]int {
	if m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	m.shared(sharedDocs, func() bool { return m.docRanks != nil }, func() {
		docs := m.gatheredDocs()
		m.docRanks = make(map[string]int, len(docs))
		for i, doc := range docs {
			m.docRanks[doc] = i - len(docs)
		}
	}, func() { m.docRanks = nil })
	return m.docRanks
}

// metadataBody is the body an annotation node binds feature values in.
func metadataBody(node ast.Node) []ast.Node {
	switch n := node.(type) {
	case *ast.PrefixMetadata:
		return n.Body
	case *ast.Usage:
		return n.Members
	default:
		return nil
	}
}

// metadataBindings is the features an annotation body binds, in declaration
// order, each with the bindings its own body states under it.
func metadataBindings(scope *symbols.Scope, body []ast.Node) []MetadataBinding {
	var out []MetadataBinding
	for _, member := range body {
		usage := metadataBodyFeature(member)
		if usage == nil {
			continue
		}
		name := redefinedFeatureName(usage)
		if name == "" {
			continue
		}
		nested := metadataBindings(valueScope(scope, usage), usage.Members)
		if usage.Value == nil && len(nested) == 0 {
			continue
		}
		out = append(out, MetadataBinding{
			Feature: name,
			Value:   usage.Value,
			Node:    usage,
			Scope:   scope,
			Nested:  nested,
		})
	}
	return out
}

// values is what the annotation binds feature to: by its body, else by its
// type's default; a sequence expression binds each of its elements.
func (a annotation) values(feature string) ([]symbols.FilterValue, bool) {
	if v, ok := a.bound[feature]; ok {
		return v, true
	}
	v, ok := a.defaults[feature]
	return v, ok
}

// value is the one constant the annotation binds feature to; a sequence of
// several is not one constant and reads as unknown.
func (a annotation) value(feature string) (symbols.FilterValue, bool) {
	values, ok := a.values(feature)
	if !ok {
		return symbols.FilterValue{}, false
	}
	if len(values) != 1 {
		return symbols.FilterValue{}, true
	}
	return values[0], true
}

// valueFacts states what the annotation binds feature to, as one constant and
// as the sequence.
func (a annotation) valueFacts(feature string) symbols.AnnotationValueFacts {
	value, _ := a.value(feature)
	values, _ := a.values(feature)
	return symbols.AnnotationValueFacts{Feature: feature, Value: value, Values: append([]symbols.FilterValue(nil), values...)}
}

// featureNames orders the annotation's valued features by name, so that what
// is reported does not depend on map iteration order.
func (a annotation) featureNames() []string {
	out := make([]string, 0, len(a.bound)+len(a.defaults))
	for name := range a.bound {
		out = append(out, name)
	}
	for name := range a.defaults {
		if _, bound := a.bound[name]; !bound {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// declaredAnnotations returns the annotations sym's own declaration states: its
// prefix metadata, the prefix metadata written among its members, and the
// metadata usages in its body.
func (m *Model) declaredAnnotations(sym *symbols.Symbol) []annotation {
	prefixes, members, ok := ast.DeclaredMetadata(sym.Decl)
	if !ok {
		return nil
	}

	scope := sym.OwnerScope
	var out []annotation
	for _, p := range prefixes {
		if a, ok := m.prefixAnnotation(scope, p); ok {
			out = append(out, a)
		}
	}
	for _, member := range members {
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		switch decl := member.(type) {
		case *ast.PrefixMetadata:
			// `part seatBelt {@Safety{isMandatory = true;}}`: prefix metadata
			// written as a member annotates the element owning the body.
			if a, ok := m.prefixAnnotation(memberScope(sym, scope), decl); ok {
				out = append(out, a)
			}
		case *ast.Usage:
			if decl.Kind != ast.UsageMetadata || annotatesOthers(decl) {
				continue
			}
			if a, ok := m.usageAnnotation(memberScope(sym, scope), decl); ok {
				out = append(out, a)
			}
		}
	}
	return out
}

// memberScope is the scope the members of sym's body resolve names against,
// falling back to the scope sym itself was declared in.
func memberScope(sym *symbols.Symbol, outer *symbols.Scope) *symbols.Scope {
	if sym.Scope != nil {
		return sym.Scope
	}
	return outer
}

// prefixAnnotation reads one prefix-metadata annotation.
func (m *Model) prefixAnnotation(scope *symbols.Scope, p *ast.PrefixMetadata) (annotation, bool) {
	if p == nil || p.Type == nil {
		return annotation{}, false
	}
	typ, ok := m.resolver.ResolveQualified(scope, p.Type)
	if !ok || typ == nil {
		return annotation{}, false
	}
	if resolved, aliasOK := m.resolver.ResolveAliasTarget(typ); aliasOK {
		typ = resolved
	} else {
		return annotation{}, false
	}
	a := m.annotationOfType(typ, scope, p.Body)
	a.node = p
	a.span = p.Span()
	a.scope = scope
	return a, true
}

// usageAnnotation reads one metadata-usage annotation, whose type is what the
// usage is typed by.
func (m *Model) usageAnnotation(scope *symbols.Scope, u *ast.Usage) (annotation, bool) {
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		typ, ok := m.resolver.ResolveQualified(scope, qn)
		if !ok || typ == nil {
			continue
		}
		if resolved, aliasOK := m.resolver.ResolveAliasTarget(typ); aliasOK {
			typ = resolved
		} else {
			continue
		}
		a := m.annotationOfType(typ, bodyScope(u, scope), u.Members)
		a.node = u
		a.span = u.Span()
		a.scope = scope
		return a, true
	}
	return annotation{}, false
}

// annotationOfType is one annotation of metadata type typ, valued by what its
// body binds plus the defaults typ declares for what the body leaves unbound.
func (m *Model) annotationOfType(typ *symbols.Symbol, scope *symbols.Scope, body []ast.Node) annotation {
	return annotation{typ: typ, bound: m.annotationValues(scope, body), defaults: m.typeDefaults(typ)}
}

// typeDefaults is the value a metadata type declares for each of its features,
// memoized per type: an annotation inherits its type's values, and a workspace
// may annotate thousands of elements with one type.
func (m *Model) typeDefaults(typ *symbols.Symbol) map[string][]symbols.FilterValue {
	if typ == nil {
		return nil
	}
	defer m.own(typ).LeaveDoc()
	if cached, ok := m.metadataDefaults[typ]; ok {
		return cached
	}
	journal(m, m.metadataDefaults, typ, typ.Decl)
	m.metadataDefaults[typ] = nil
	var values map[string][]symbols.FilterValue
	for _, member := range m.MembersOf(typ) {
		value, ok := m.declaredDefault(member)
		if !ok {
			continue
		}
		name := simpleSymbolName(member)
		if name == "" {
			continue
		}
		if _, valued := values[name]; valued {
			continue
		}
		if values == nil {
			values = make(map[string][]symbols.FilterValue)
		}
		values[name] = value
	}
	m.metadataDefaults[typ] = values
	return values
}

// declaredDefault returns the value member's declaration binds it to, read
// from its record when its document is recorded.
func (m *Model) declaredDefault(member *symbols.Symbol) ([]symbols.FilterValue, bool) {
	if member.Recorded() {
		if member.Facts.Default == nil && !member.Facts.Modifiers.Has(symbols.ModValued) {
			return nil, false
		}
		return member.Facts.Default, true
	}
	usage, ok := member.Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		return nil, false
	}
	return m.annotationSequence(member.OwnerScope, usage.Value), true
}

// MetadataDefaultOf returns the default a feature of a metadata definition or
// usage declares, one value per element of a sequence expression, as an
// annotation of that type reads it when it leaves the feature unbound; false
// for any other symbol.
func (m *Model) MetadataDefaultOf(sym *symbols.Symbol) ([]symbols.FilterValue, bool) {
	if m == nil || sym == nil || sym.OwnerScope == nil || !metadataTyped(sym.OwnerScope.Owner()) {
		return nil, false
	}
	return m.declaredDefault(sym)
}

// metadataTyped reports whether sym is a metadata definition or usage.
func metadataTyped(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if kind, ok := sym.DefinitionKind(); ok {
		return kind == ast.DefMetadata
	}
	kind, ok := sym.UsageKind()
	return ok && kind == ast.UsageMetadata
}

// bodyScope is the scope a metadata usage's body resolves names against. The
// usage's own scope is not reachable from its declaration, so the annotation's
// values are read against the scope the usage was declared in, which is the
// scope its type reference resolved in too.
func bodyScope(_ *ast.Usage, declared *symbols.Scope) *symbols.Scope { return declared }

// aboutAnnotations returns the annotations that `metadata m about sym;`
// declarations elsewhere in the workspace state about sym.
func (m *Model) aboutAnnotations(sym *symbols.Symbol) []annotation {
	about := m.annotationsAbout()
	// Usages may have resolved sym across re-indexed trees, to as many symbols
	// of one declaration; the declaration gathers what every one was told.
	if sym.Decl != nil {
		return m.aboutByDecl[sym.Decl]
	}
	return about[sym]
}

// AboutAnnotatedSymbols returns every element an `about` metadata usage
// annotates, in the deterministic order the index is built in — bundled
// library elements a workspace annotation targets included.
func (m *Model) AboutAnnotatedSymbols() []*symbols.Symbol {
	m.annotationsAbout()
	return m.aboutOrder
}

// annotationsAbout indexes every `about` metadata usage in the workspace by the
// element it annotates. It is built once: an `about` annotation is stated away
// from the element it applies to, so there is no way to it from the element
// itself.
func (m *Model) annotationsAbout() map[*symbols.Symbol][]annotation {
	if m.resolver == nil || m.resolver.Index() == nil {
		if m.aboutAnnots == nil {
			m.aboutAnnots = make(map[*symbols.Symbol][]annotation)
			m.aboutByDecl = make(map[ast.Node][]annotation)
		}
		return m.aboutAnnots
	}
	m.shared(sharedAbout, func() bool { return m.aboutAnnots != nil }, func() {
		if m.aboutShared == nil {
			m.buildAbout()
			return
		}
		m.aboutShared.once.Do(func() {
			m.buildAbout()
			m.aboutShared.annots, m.aboutShared.byDecl, m.aboutShared.order = m.aboutAnnots, m.aboutByDecl, m.aboutOrder
		})
		m.aboutAnnots, m.aboutByDecl, m.aboutOrder = m.aboutShared.annots, m.aboutShared.byDecl, m.aboutShared.order
	}, func() { m.aboutAnnots, m.aboutByDecl, m.aboutOrder, m.aboutShared = nil, nil, nil, nil })
	return m.aboutAnnots
}

// buildAbout indexes the `about` metadata usages of every gathered document.
func (m *Model) buildAbout() {
	m.aboutAnnots = make(map[*symbols.Symbol][]annotation)
	m.aboutByDecl = make(map[ast.Node][]annotation)
	gathers := m.gathers()
	for _, doc := range m.gatheredDocs() {
		for _, sym := range gathers[doc].about {
			m.indexAboutUsage(sym)
		}
	}
}

// AboutIndex is the `about` annotation index built once and read by every model
// sharing it, so concurrent models over one index need not each build their own.
type AboutIndex struct {
	once   sync.Once
	annots map[*symbols.Symbol][]annotation
	byDecl map[ast.Node][]annotation
	order  []*symbols.Symbol
}

// NewAboutIndex is an about index no model has built yet.
func NewAboutIndex() *AboutIndex { return &AboutIndex{} }

// ShareAbout has m read its `about` annotations from shared, building it if m is
// the first to ask. The models sharing an index must be over one and the same
// symbol index, which must not change while they share it: a model whose index
// changes drops the shared one and indexes on its own from then on.
func (m *Model) ShareAbout(shared *AboutIndex) {
	if m == nil || shared == nil {
		return
	}
	m.aboutShared = shared
	m.aboutAnnots, m.aboutByDecl, m.aboutOrder = nil, nil, nil
}

// indexAboutUsage records one `about` metadata usage against every element it
// annotates.
func (m *Model) indexAboutUsage(sym *symbols.Symbol) {
	if sym.Recorded() {
		m.indexRecordedAboutUsage(sym)
		return
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !annotatesOthers(usage) {
		return
	}
	a, ok := m.usageAnnotation(sym.OwnerScope, usage)
	if !ok {
		return
	}
	a.about = true
	m.indexAbout(a, m.annotatedElements(sym.OwnerScope, usage))
}

// indexRecordedAboutUsage indexes an `about` metadata usage a record carries:
// its one annotation, on the elements the record names.
func (m *Model) indexRecordedAboutUsage(sym *symbols.Symbol) {
	if sym.Facts.Annotation == nil || len(sym.Facts.About) == 0 {
		return
	}
	a, ok := m.annotationFromFacts(*sym.Facts.Annotation, sym.OwnerScope)
	if !ok {
		return
	}
	a.about = true
	var targets []aboutTarget
	for _, target := range m.recordedElements(nil, sym.Facts.About) {
		targets = append(targets, aboutTarget{sym: target})
	}
	m.indexAbout(a, targets)
}

// indexAbout files a as an annotation of each target, through the namespace
// the target was named by.
func (m *Model) indexAbout(a annotation, targets []aboutTarget) {
	for _, target := range targets {
		a.via = target.via
		if _, known := m.aboutAnnots[target.sym]; !known {
			m.aboutOrder = append(m.aboutOrder, target.sym)
		}
		m.aboutAnnots[target.sym] = append(m.aboutAnnots[target.sym], a)
		if target.sym.Decl != nil {
			m.aboutByDecl[target.sym.Decl] = append(m.aboutByDecl[target.sym.Decl], a)
		}
	}
}

// aboutTarget is one element an `about` clause names, with the namespace the
// clause reached it through when the name was qualified (nil otherwise, and
// for a recorded clause, whose site reads no layout).
type aboutTarget struct {
	sym, via *symbols.Symbol
}

// annotatedElements resolves the elements a metadata usage's `about` clause
// names.
func (m *Model) annotatedElements(scope *symbols.Scope, u *ast.Usage) []aboutTarget {
	var out []aboutTarget
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelAnnotates {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		target, ok := m.resolver.ResolveQualified(scope, qn)
		if !ok || target == nil {
			continue
		}
		var via *symbols.Symbol
		if n := len(qn.Parts); n > 1 {
			via, _ = m.resolver.PartSymbol(qn, n-2)
		}
		out = append(out, aboutTarget{sym: target, via: via})
	}
	return out
}

// annotatesOthers reports whether a metadata usage states what it annotates
// (`metadata m about p;`), rather than annotating the element owning it.
func annotatesOthers(u *ast.Usage) bool { return symbols.UsageAnnotatesOthers(u) }

// annotationValues reads the feature values an annotation body binds, as in
// `@Safety{isMandatory = true;}`. A binding whose value is not a constant or an
// element reference is recorded with an unknown value, which a condition reading
// it reports as unevaluable rather than treating as absent.
func (m *Model) annotationValues(scope *symbols.Scope, body []ast.Node) map[string][]symbols.FilterValue {
	var values map[string][]symbols.FilterValue
	for _, member := range body {
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		usage, ok := member.(*ast.Usage)
		if !ok || usage.Value == nil {
			continue
		}
		name := boundFeatureName(usage)
		if name == "" {
			continue
		}
		if values == nil {
			values = make(map[string][]symbols.FilterValue)
		}
		values[name] = m.annotationSequence(scope, usage.Value)
	}
	return values
}

// annotationSequence evaluates the values an annotation binds a feature to: one
// per element of a sequence expression, else the one value the expression has.
func (m *Model) annotationSequence(scope *symbols.Scope, value ast.Node) []symbols.FilterValue {
	if seq, ok := value.(*ast.SequenceExpr); ok {
		out := make([]symbols.FilterValue, 0, len(seq.Elements))
		for _, element := range seq.Elements {
			out = append(out, m.annotationValue(scope, element))
		}
		return out
	}
	return []symbols.FilterValue{m.annotationValue(scope, value)}
}

// boundFeatureName is the annotation feature a body member binds: the name it
// declares, or the feature it redefines (`:>> isMandatory = true`).
func boundFeatureName(u *ast.Usage) string {
	if u.Ident.Name != "" {
		return u.Ident.Name
	}
	return redefinitionTargetName(u)
}

// redefinedFeatureName is the metadata feature a body member writes to: the one
// it redefines, which a name of its own renames rather than replaces.
func redefinedFeatureName(u *ast.Usage) string {
	if name := redefinitionTargetName(u); name != "" {
		return name
	}
	return u.Ident.Name
}

// redefinitionTargetName is the feature a `:>> f` clause names, or "".
func redefinitionTargetName(u *ast.Usage) string {
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		return qn.Parts[len(qn.Parts)-1].Text
	}
	return ""
}

// annotationValue evaluates one value an annotation binds: a constant, a
// quantity, or a reference to an element such as an enumeration literal, which
// is compared by identity.
func (m *Model) annotationValue(scope *symbols.Scope, value ast.Node) symbols.FilterValue {
	if v, ok := EvalConst(value); ok {
		return constValue(v)
	}
	if q, ok := m.EvalQuantity(scope, value); ok {
		return quantityValue(q)
	}
	switch e := value.(type) {
	case *ast.LiteralString:
		return symbols.FilterValue{Kind: symbols.FilterValueString, Str: unquote(e.Value)}
	case *ast.FeatureReference:
		if sym, ok := m.resolver.ResolveQualified(scope, e.Name); ok && sym != nil {
			if fqn := m.fqnOf(sym); fqn != "" {
				return symbols.FilterValue{Kind: symbols.FilterValueRef, RefFQN: fqn}
			}
		}
	}
	return symbols.FilterValue{}
}

// ConstantFeatureValues returns a feature's ordered constant values. The error
// is a symbols.NeedsHydration when the value is written in a recorded document.
func (m *Model) ConstantFeatureValues(sym *symbols.Symbol, feature string) ([]symbols.FilterValue, bool, error) {
	if m == nil || sym == nil || feature == "" {
		return nil, false, nil
	}
	if values, ok := m.ReflectiveFeatureValues(sym, feature); ok {
		return values, true, nil
	}
	member, ok := m.LookupMember(sym, feature)
	if !ok || member == nil {
		return nil, false, nil
	}
	return m.constantFeatureValues(member, make(map[*symbols.Symbol]bool))
}

// DeclaredFeatureValues returns a declared member feature's ordered constant
// values, never answering reflective metaclass features of the same name. The
// error is a symbols.NeedsHydration when the value is written in a recorded document.
func (m *Model) DeclaredFeatureValues(sym *symbols.Symbol, feature string) ([]symbols.FilterValue, bool, error) {
	if m == nil || sym == nil || feature == "" {
		return nil, false, nil
	}
	member, ok := m.LookupMember(sym, feature)
	if !ok || member == nil {
		return nil, false, nil
	}
	return m.constantFeatureValues(member, make(map[*symbols.Symbol]bool))
}

// constantFeatureValues reads the values member's declaration writes, or those
// of the features it redefines when it writes none. A recorded feature's record
// says whether it writes a value but not what: that reading needs its tree.
func (m *Model) constantFeatureValues(member *symbols.Symbol, seen map[*symbols.Symbol]bool) ([]symbols.FilterValue, bool, error) {
	if member == nil || seen[member] {
		return nil, false, nil
	}
	seen[member] = true
	defer delete(seen, member)
	if !member.DeclaresUsage() {
		return []symbols.FilterValue{{}}, true, nil
	}
	if member.Recorded() && member.Facts.Modifiers.Has(symbols.ModValued) {
		return nil, false, &symbols.NeedsHydration{Doc: member.DocName, Question: "the value " + symbols.FQNOf(member) + " declares"}
	}
	usage, _ := member.Decl.(*ast.Usage)
	if usage == nil || usage.Value == nil {
		var values []symbols.FilterValue
		found := false
		for _, redefined := range m.RedefinedFeatures(member) {
			inherited, ok, err := m.constantFeatureValues(redefined, seen)
			if err != nil {
				return nil, false, err
			}
			if !ok {
				continue
			}
			found = true
			values = append(values, inherited...)
		}
		if found {
			return values, true, nil
		}
		return nil, true, nil
	}
	if sequence, ok := usage.Value.(*ast.SequenceExpr); ok {
		values := make([]symbols.FilterValue, 0, len(sequence.Elements))
		for _, element := range sequence.Elements {
			if _, empty := element.(*ast.NullExpr); empty {
				continue
			}
			values = append(values, m.declaredValue(member.OwnerScope, element))
		}
		return values, true, nil
	}
	if _, empty := usage.Value.(*ast.NullExpr); empty {
		return nil, true, nil
	}
	return []symbols.FilterValue{m.declaredValue(member.OwnerScope, usage.Value)}, true, nil
}

// declaredValue is annotationValue for a feature's own value, where a reference
// to an attribute reads that attribute's value — as seen from the carrier when
// the attribute is one of its features — rather than naming it. A unit, an
// enumeration literal or a non-value element stays the element it names.
func (m *Model) declaredValue(scope *symbols.Scope, value ast.Node) symbols.FilterValue {
	result := m.annotationValue(scope, value)
	ref, ok := value.(*ast.FeatureReference)
	if result.Kind != symbols.FilterValueRef || !ok {
		return result
	}
	if sym, ok := m.resolver.ResolveQualified(scope, ref.Name); ok && m.readsValueOf(sym) {
		return symbols.FilterValue{}
	}
	return result
}

// readsValueOf reports whether a reference to sym denotes the value the attribute
// holds rather than the element sym itself: a unit and an enumeration literal are
// values by identity, a definition or an object feature is no value at all.
func (m *Model) readsValueOf(sym *symbols.Symbol) bool {
	if sym == nil || (!sym.Kind.IsAttributeLike() && sym.Kind != symbols.SymbolEnumerationUsage) {
		return false
	}
	return EnumerationOwning(sym) == nil && !m.IsMeasurementUnit(sym)
}

// metaclassOf is the candidate's own metaclass — what `@@T` tests: a KerML
// declaration by its keyword (its kind cannot tell `struct` from `datatype`),
// anything else by its symbol kind, so a cached element classifies alike.
func (m *Model) metaclassOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	// A relationship written keyword-first is classified by its own kind in
	// either language, since no symbol kind distinguishes its forms.
	if rel, ok := sym.RelationshipDecl(); ok {
		if meta := m.kermlMetaclass(relationshipMetaclassName(rel)); meta != nil {
			return meta
		}
	}
	// A named multiplicity is a KerML element in either language, and a cached
	// one keeps the kind without the declaration.
	if sym.Kind == symbols.SymbolMultiplicity {
		if meta := m.kermlMetaclass(MultiplicityMetaclassName(sym)); meta != nil {
			return meta
		}
	}
	if isMetadataBodyFeature(sym) {
		return m.metadataBodyFeatureMetaclass(m.isKerMLDoc(sym))
	}
	// Annotating elements and dependencies are KerML elements in either language.
	switch sym.Kind {
	case symbols.SymbolComment:
		return m.kermlMetaclass("Comment")
	case symbols.SymbolDocumentation:
		return m.kermlMetaclass("Documentation")
	case symbols.SymbolTextualRepresentation:
		return m.kermlMetaclass("TextualRepresentation")
	case symbols.SymbolDependency:
		return m.kermlMetaclass("Dependency")
	}
	if meta := m.kermlMetaclass(kermlMetaclassName(sym, m.isKerMLDoc(sym))); meta != nil {
		return meta
	}
	return m.sysmlMetaclass(sysmlMetaclassName(sym))
}

// sysmlMetaclassName is the SysML metaclass of sym's declaration: by its symbol
// kind, or by the declaration where the kind spans several (SysML.xtext).
func sysmlMetaclassName(sym *symbols.Symbol) string {
	if isVariantReferenceSymbol(sym) {
		return "ReferenceUsage"
	}
	if sym.Recorded() && sym.Facts.Modifiers.Has(symbols.ModEvent) {
		return "EventOccurrenceUsage"
	}
	switch decl := sym.Decl.(type) {
	case *ast.ForkNode:
		return "ForkNode"
	case *ast.JoinNode:
		return "JoinNode"
	case *ast.MergeNode:
		return "MergeNode"
	case *ast.DecisionNode:
		return "DecisionNode"
	case *ast.Usage:
		if decl.IsEvent {
			return "EventOccurrenceUsage"
		}
	}
	switch sym.Kind {
	case symbols.SymbolConnectorEnd:
		return ConnectorEndMetaclassName(sym)
	case symbols.SymbolUnknown:
		if kind, ok := sym.UsageKind(); ok {
			return usageMetaclassNames[kind]
		}
	case symbols.SymbolActionUsage:
		if sym.DeclaresTransition() {
			return usageMetaclassNames[ast.UsageTransition]
		}
	}
	if sym.Recorded() {
		switch sym.Facts.Node {
		case symbols.NodeFork:
			return "ForkNode"
		case symbols.NodeJoin:
			return "JoinNode"
		case symbols.NodeMerge:
			return "MergeNode"
		case symbols.NodeDecision:
			return "DecisionNode"
		}
	}
	return metaclassName(sym.Kind)
}

// ConnectorEndMetaclassName is the SysML metaclass of a connector end: a
// PortUsage as an interface's end, a ReferenceUsage otherwise (SysML.xtext).
func ConnectorEndMetaclassName(sym *symbols.Symbol) string {
	if sym.OwnerScope != nil {
		if kind, ok := connectorKind(sym.OwnerScope); ok && kind == ast.UsageInterface {
			return metaclassName(symbols.SymbolPortUsage)
		}
	}
	return referenceUsageMetaclassName
}

// usageMetaclassNames maps the usage kinds the symbol taxonomy keeps no kind
// of their own for to their SysML metaclasses (SysML.xtext).
var usageMetaclassNames = map[ast.UsageKind]string{
	ast.UsageBinding:    "BindingConnectorAsUsage",
	ast.UsageTransition: "TransitionUsage",
}

// isMetadataBodyFeature reports whether sym is a feature a metadata body declares,
// at any depth, other than a metadata feature annotating the body's owner.
func isMetadataBodyFeature(sym *symbols.Symbol) bool {
	kind, ok := sym.UsageKind()
	if !ok || kind == ast.UsageMetadata {
		return false
	}
	for scope := sym.OwnerScope; scope != nil; {
		if scope.BodyLocal() {
			_, prefix := scope.Node().(*ast.PrefixMetadata)
			return prefix
		}
		owner := scope.Owner()
		if owner == nil {
			return false
		}
		if owner.Kind == symbols.SymbolMetadataUsage {
			return true
		}
		if !owner.DeclaresUsage() {
			return false
		}
		scope = owner.OwnerScope
	}
	return false
}

// metadataBodyFeatureMetaclass is the metaclass of a metadata body's feature:
// KerML.xtext MetadataBodyFeature is a Feature, SysML.xtext MetadataBodyUsage a ReferenceUsage.
func (m *Model) metadataBodyFeatureMetaclass(isKerML bool) *symbols.Symbol {
	if isKerML {
		return m.kermlMetaclass(kermlMetaclassNames["feature"])
	}
	return m.sysmlMetaclass(referenceUsageMetaclassName)
}

// referenceUsageMetaclassName is the SysML metaclass of a usage written with no kind.
const referenceUsageMetaclassName = "ReferenceUsage"

// sysmlMetaclass is the library element declaring the named SysML metaclass,
// or nil for an unnamed or undeclared one.
func (m *Model) sysmlMetaclass(name string) *symbols.Symbol {
	if name == "" {
		return nil
	}
	if meta := m.symbolByFQN(sysmlMetaclassPrefix + name); meta != nil {
		return meta
	}
	return m.symbolByFQN(name)
}

// kermlMetaclass is the library element declaring the named KerML metaclass,
// or nil for an unnamed or undeclared one.
func (m *Model) kermlMetaclass(name string) *symbols.Symbol {
	if name == "" {
		return nil
	}
	for _, prefix := range kermlMetaclassPrefixes {
		if meta := m.symbolByFQN(prefix + name); meta != nil {
			return meta
		}
	}
	return nil
}

// connectorKind is the usage kind of the connector whose scope owns an end,
// from the scope's node or, for a recorded connector, its owner's record.
func connectorKind(scope *symbols.Scope) (ast.UsageKind, bool) {
	if usage, ok := scope.Node().(*ast.Usage); ok {
		return usage.Kind, true
	}
	if owner := scope.Owner(); owner != nil && owner.Recorded() {
		return owner.UsageKind()
	}
	return 0, false
}

// relationshipMetaclassName is the metaclass of a keyword-first relationship,
// which conjugation writes as a form of its own (KerML §7.2).
func relationshipMetaclassName(rel symbols.RelationshipDecl) string {
	if rel.Conjugated {
		return "Conjugation"
	}
	return relationshipMetaclassNames[rel.Kind]
}

// MultiplicityMetaclassName is the metaclass of a named multiplicity: a range
// (`multiplicity m [1..2]`) is a MultiplicityRange, a subset a Multiplicity.
func MultiplicityMetaclassName(sym *symbols.Symbol) string {
	if sym.Recorded() {
		// A multiplicity member's record carries bounds exactly when it declared a range.
		if sym.Facts.Multiplicity != nil {
			return "MultiplicityRange"
		}
		return "Multiplicity"
	}
	if mult, ok := sym.Decl.(*ast.MultiplicityDecl); ok && mult.Range != nil {
		return "MultiplicityRange"
	}
	return "Multiplicity"
}

// relationshipMetaclassNames maps the kind of a relationship written
// keyword-first to the KerML metaclass classifying it (KerML §7.2).
var relationshipMetaclassNames = map[ast.RelationshipKind]string{
	ast.RelSpecializes: "Specialization",
	ast.RelTyping:      "FeatureTyping",
	ast.RelSubsets:     "Subsetting",
	ast.RelRedefines:   "Redefinition",
	ast.RelInverseOf:   "FeatureInverting",
	ast.RelFeaturedBy:  "TypeFeaturing",
	ast.RelDisjoint:    "Disjoining",
}

// sysmlMetaclassPrefix qualifies the reflective metadata types of the SysML
// abstract syntax, which the standard library declares in SysML::Systems and
// re-exports through SysML.
const sysmlMetaclassPrefix = "SysML::Systems::"

// kermlMetaclassPrefixes qualify the KerML abstract syntax metaclasses, which
// the library declares across KerML's three packages.
var kermlMetaclassPrefixes = []string{"KerML::Kernel::", "KerML::Core::", "KerML::Root::"}

// kermlMetaclassNames maps a KerML declaration keyword to the metaclass it
// implies (KerML 1.1 §8.2, §9.2).
var kermlMetaclassNames = map[string]string{
	"type":         "Type",
	"classifier":   "Classifier",
	"class":        "Class",
	"struct":       "Structure",
	"assoc":        "Association",
	"association":  "Association",
	"assoc struct": "AssociationStructure",
	"datatype":     "DataType",
	"behavior":     "Behavior",
	"function":     "Function",
	"predicate":    "Predicate",
	"interaction":  "Interaction",
	"metaclass":    "Metaclass",
	"metadata":     "MetadataFeature",
	"feature":      "Feature",
	"step":         "Step",
	"expr":         "Expression",
	"bool":         "BooleanExpression",
	"inv":          "Invariant",
	"connector":    "Connector",
	"binding":      "BindingConnector",
	"bind":         "BindingConnector",
	"flow":         "Flow",
	"message":      "Flow",
	"succession":   "Succession",
	"multiplicity": "Multiplicity",
}

// kermlMetaclassName is the metaclass the keyword of sym's KerML declaration
// implies, or "" for a declaration written in SysML or restored from a cache,
// which is classified by its symbol kind instead.
func kermlMetaclassName(sym *symbols.Symbol, isKerML bool) string {
	if !isKerML {
		return ""
	}
	if sym.Recorded() {
		switch sym.Facts.Node {
		case symbols.NodeDefinition, symbols.NodeUsage:
			if name := kermlMetaclassNames[sym.Facts.Keyword]; name != "" {
				return name
			}
			if sym.Facts.Node == symbols.NodeUsage {
				return "Feature"
			}
			return ""
		case symbols.NodePrefixMetadata:
			return kermlMetaclassNames["metadata"]
		case symbols.NodeConnectorEnd, symbols.NodeCrossFeature:
			return kermlMetaclassNames["feature"]
		}
		return ""
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return kermlMetaclassNames[d.Keyword]
	case *ast.Usage:
		if name := kermlMetaclassNames[d.Keyword]; name != "" {
			return name
		}
		return "Feature"
	case *ast.BodyExpr:
		// Body-expression parameters are features but have no Usage declaration node.
		if sym.Kind.IsFeature() {
			return "Feature"
		}
	case *ast.PrefixMetadata:
		return kermlMetaclassNames["metadata"]
	case *ast.ConnectorEnd, *ast.CrossFeatureMember:
		return kermlMetaclassNames["feature"]
	}
	return ""
}

// Metaclass is the library element declaring the SysML or KerML metaclass of
// the simple name, or nil where the loaded libraries declare none.
func (m *Model) Metaclass(name string) *symbols.Symbol {
	if m == nil || name == "" {
		return nil
	}
	if meta := m.symbolByFQN(sysmlMetaclassPrefix + name); meta != nil {
		return meta
	}
	return m.kermlMetaclass(name)
}

// MetaclassOf is the reflective metaclass classifying sym's declaration — the
// library element `x meta T` yields an instance of — or nil where none is known.
func (m *Model) MetaclassOf(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	return m.metaclassOf(sym)
}

// ReflectiveElements reads an element-valued metaclass feature of sym as the
// elements it holds, in model order; ok is false where the feature is not derived.
func (m *Model) ReflectiveElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	if m == nil || sym == nil {
		return nil, false
	}
	switch feature {
	case "owningNamespace":
		if !m.reflectiveMetaclassConforms(sym, "Element") {
			return nil, false
		}
		owner := m.ownerOf(sym)
		if owner == nil || !m.reflectiveMetaclassConforms(owner, "Namespace") {
			return nil, true
		}
		return []*symbols.Symbol{owner}, true
	case "owningType":
		if !m.reflectiveMetaclassConforms(sym, "Feature") {
			return nil, false
		}
		owner := m.ownerOf(sym)
		if owner == nil || !m.reflectiveMetaclassConforms(owner, "Type") {
			return nil, true
		}
		return []*symbols.Symbol{owner}, true
	case "featuringType":
		if !m.reflectiveMetaclassConforms(sym, "Feature") {
			return nil, false
		}
		var out []*symbols.Symbol
		if owner := m.ownerOf(sym); owner != nil && m.reflectiveMetaclassConforms(owner, "Type") {
			out = append(out, owner)
		}
		if sym.Recorded() {
			for _, target := range m.RecordedRelationshipTargets(sym, ast.RelFeaturedBy) {
				if m.reflectiveMetaclassConforms(target, "Type") && !containsElement(out, target) {
					out = append(out, target)
				}
			}
		} else {
			for _, rel := range RelationshipsOf(sym) {
				if rel == nil || rel.Kind != ast.RelFeaturedBy {
					continue
				}
				target := m.RelationshipTarget(sym, rel)
				if target != nil && m.reflectiveMetaclassConforms(target, "Type") && !containsElement(out, target) {
					out = append(out, target)
				}
			}
		}
		return out, true
	case "input":
		if !m.reflectiveMetaclassConforms(sym, "Type") {
			return nil, false
		}
		if behaviorLike(sym) {
			out := m.InputParametersOf(sym)
			return includeSubjectParameterInOrder(m, sym, out), true
		}
		out := m.directedFeatures(sym, ast.DirIn, ast.DirInOut)
		if m.reflectiveMetaclassConforms(sym, "RequirementUsage") {
			out = includeSubjectParameterInOrder(m, sym, out)
		}
		return out, true
	case "output":
		if !m.reflectiveMetaclassConforms(sym, "Type") {
			return nil, false
		}
		if behaviorLike(sym) {
			var out []*symbols.Symbol
			for _, parameter := range m.BehaviorParametersOf(sym) {
				if parameter.Symbol == nil || parameter.IsResult ||
					(parameter.Direction != ast.DirOut && parameter.Direction != ast.DirInOut) {
					continue
				}
				out = append(out, parameter.Symbol)
			}
			for _, parameter := range m.BehaviorParametersOf(sym) {
				if parameter.Symbol != nil && parameter.IsResult {
					out = append(out, parameter.Symbol)
				}
			}
			return out, true
		}
		return m.directedFeatures(sym, ast.DirOut, ast.DirInOut), true
	case "parameter":
		if !m.reflectiveMetaclassConforms(sym, "Behavior") &&
			!m.reflectiveMetaclassConforms(sym, "Step") {
			return nil, false
		}
		var out []*symbols.Symbol
		for _, parameter := range m.BehaviorParametersOf(sym) {
			if parameter.Symbol != nil {
				out = append(out, parameter.Symbol)
			}
		}
		return includeSubjectParameterInOrder(m, sym, out), true
	case "directedFeature":
		if !m.reflectiveMetaclassConforms(sym, "Type") {
			return nil, false
		}
		return m.directedFeatures(sym, ast.DirIn, ast.DirInOut, ast.DirOut), true
	case "subjectParameter":
		if !m.ownsSubjectParameter(sym) {
			return nil, false
		}
		if subject := m.SubjectParameterOf(sym); subject != nil {
			return []*symbols.Symbol{subject}, true
		}
		return nil, true
	case "objectiveRequirement":
		if !m.ownsObjectiveRequirement(sym) {
			return nil, false
		}
		owned, inherited := m.ObjectivesOf(sym)
		objectives := append(owned, inherited...)
		if len(objectives) > 0 && objectives[0] != nil {
			return []*symbols.Symbol{objectives[0]}, true
		}
		return nil, true
	case "result":
		if !m.reflectiveMetaclassConforms(sym, "Expression") &&
			!m.reflectiveMetaclassConforms(sym, "Function") {
			return nil, false
		}
		if result := m.ResultParameterOf(sym); result != nil {
			return []*symbols.Symbol{result}, true
		}
		return nil, true
	case "individualDefinition":
		if !m.reflectiveMetaclassConforms(sym, "OccurrenceUsage") {
			return nil, false
		}
		for _, definition := range m.definitionTypedFeatures(sym, "occurrenceDefinition") {
			if value, ok := m.ReflectiveFeatureValue(definition, "isIndividual"); ok && value.Kind == symbols.FilterValueBool && value.Bool {
				return []*symbols.Symbol{definition}, true
			}
		}
		return nil, true
	case "metaclass":
		if !m.reflectiveMetaclassConforms(sym, "MetadataFeature") {
			return nil, false
		}
		return m.conformingTypes(sym, "Metaclass"), true
	case "endFeature", "ownedEndFeature":
		if !m.reflectiveMetaclassConforms(sym, "Type") {
			return nil, false
		}
		ends := m.EndFeatures(sym)
		if feature == "endFeature" {
			return ends, true
		}
		owned := make([]*symbols.Symbol, 0, len(ends))
		for _, end := range ends {
			if end != nil && m.ownerOf(end) == sym {
				owned = append(owned, end)
			}
		}
		return owned, true
	case "unioningType":
		if !m.reflectiveMetaclassConforms(sym, "Type") {
			return nil, false
		}
		return m.UnioningTypes(sym), true
	case "intersectingType":
		if !m.reflectiveMetaclassConforms(sym, "Type") {
			return nil, false
		}
		return m.IntersectingTypes(sym), true
	case "differencingType":
		if !m.reflectiveMetaclassConforms(sym, "Type") {
			return nil, false
		}
		return m.DifferencingTypes(sym), true
	case "owner":
		if sym.OwnerScope == nil || sym.OwnerScope.Owner() == nil {
			return nil, true
		}
		return []*symbols.Symbol{sym.OwnerScope.Owner()}, true
	case "ownedMember":
		return ownedMembersOf(sym), true
	case "ownedElement":
		return m.ownedElementsOf(sym), true
	case "member":
		return m.membersIncludingAnonymous(sym), true
	case "feature":
		var features []*symbols.Symbol
		for _, member := range m.membersIncludingAnonymous(sym) {
			if member.IsFeature() {
				features = append(features, member)
			}
		}
		return features, true
	case "documentation":
		return m.documentationSymbols(sym), true
	case "relatedFeature", "sourceFeature", "targetFeature":
		if !m.reflectiveMetaclassConforms(sym, "Connector") {
			return nil, false
		}
		return m.connectorRelatedFeatures(sym, feature), true
	case "ownedFeature":
		var features []*symbols.Symbol
		for _, member := range ownedMembersOf(sym) {
			if member.IsFeature() {
				features = append(features, member)
			}
		}
		return features, true
	case "type":
		if !sym.IsFeature() {
			return nil, false
		}
		return m.reflectiveFeatureTypes(sym), true
	case "client", "supplier":
		dep, ok := sym.Decl.(*ast.Dependency)
		if !ok {
			return nil, false
		}
		if feature == "client" {
			return m.dependencyEnds(sym, dep.Clients), true
		}
		return m.dependencyEnds(sym, dep.Suppliers), true
	case "representedElement":
		if _, ok := sym.Decl.(*ast.TextualRepresentation); !ok || sym.OwnerScope == nil || sym.OwnerScope.Owner() == nil {
			return nil, false
		}
		return []*symbols.Symbol{sym.OwnerScope.Owner()}, true
	}
	// A usage's `nested*` and a definition's `owned*` derive its owned usages of
	// the metaclass the suffix names (see reflective_usages.go).
	if elems, ok := m.reflectiveOwnedUsages(sym, feature); ok {
		return elems, true
	}
	if typed, ok := reflectiveDefinitionFeatures[feature]; ok {
		if !m.reflectiveMetaclassConforms(sym, typed.owner) {
			return nil, false
		}
		return m.definitionTypedFeatures(sym, feature), true
	}
	return nil, false
}

type reflectiveDefinitionFeature struct {
	owner  string
	target string
}

var reflectiveDefinitionFeatures = map[string]reflectiveDefinitionFeature{
	"occurrenceDefinition":       {owner: "OccurrenceUsage", target: "Class"},
	"itemDefinition":             {owner: "ItemUsage", target: "Structure"},
	"partDefinition":             {owner: "PartUsage", target: "PartDefinition"},
	"portDefinition":             {owner: "PortUsage", target: "PortDefinition"},
	"actionDefinition":           {owner: "ActionUsage", target: "Behavior"},
	"attributeDefinition":        {owner: "AttributeUsage", target: "DataType"},
	"stateDefinition":            {owner: "StateUsage", target: "Behavior"},
	"constraintDefinition":       {owner: "ConstraintUsage", target: "Predicate"},
	"requirementDefinition":      {owner: "RequirementUsage", target: "RequirementDefinition"},
	"calculationDefinition":      {owner: "CalculationUsage", target: "Function"},
	"caseDefinition":             {owner: "CaseUsage", target: "CaseDefinition"},
	"analysisCaseDefinition":     {owner: "AnalysisCaseUsage", target: "AnalysisCaseDefinition"},
	"verificationCaseDefinition": {owner: "VerificationCaseUsage", target: "VerificationCaseDefinition"},
	"useCaseDefinition":          {owner: "UseCaseUsage", target: "UseCaseDefinition"},
	"viewDefinition":             {owner: "ViewUsage", target: "ViewDefinition"},
	"viewpointDefinition":        {owner: "ViewpointUsage", target: "ViewpointDefinition"},
	"renderingDefinition":        {owner: "RenderingUsage", target: "RenderingDefinition"},
	"metadataDefinition":         {owner: "MetadataUsage", target: "Metaclass"},
	"flowDefinition":             {owner: "FlowUsage", target: "Interaction"},
	"connectionDefinition":       {owner: "ConnectionUsage", target: "AssociationStructure"},
	"interfaceDefinition":        {owner: "InterfaceUsage", target: "InterfaceDefinition"},
	"allocationDefinition":       {owner: "AllocationUsage", target: "AllocationDefinition"},
	"enumerationDefinition":      {owner: "EnumerationUsage", target: "EnumerationDefinition"},
}

func (m *Model) reflectiveMetaclassConforms(sym *symbols.Symbol, name string) bool {
	if sym == nil {
		return false
	}
	meta := m.MetaclassOf(sym)
	target := m.Metaclass(name)
	return meta != nil && target != nil && m.Conforms(meta, target)
}

func (m *Model) ownsSubjectParameter(sym *symbols.Symbol) bool {
	return m.reflectiveMetaclassConforms(sym, "CaseDefinition") ||
		m.reflectiveMetaclassConforms(sym, "CaseUsage") ||
		m.reflectiveMetaclassConforms(sym, "RequirementDefinition") ||
		m.reflectiveMetaclassConforms(sym, "RequirementUsage")
}

func (m *Model) ownsObjectiveRequirement(sym *symbols.Symbol) bool {
	return m.reflectiveMetaclassConforms(sym, "CaseDefinition") ||
		m.reflectiveMetaclassConforms(sym, "CaseUsage")
}

func (m *Model) directedFeatures(sym *symbols.Symbol, directions ...ast.FeatureDirection) []*symbols.Symbol {
	allowed := make(map[ast.FeatureDirection]bool, len(directions))
	for _, direction := range directions {
		allowed[direction] = true
	}
	var out []*symbols.Symbol
	for _, member := range m.membersIncludingAnonymous(sym) {
		direction, ok := ReflectiveDirection(member)
		if ok && allowed[direction] {
			out = append(out, member)
		}
	}
	return out
}

func includeSubjectParameterInOrder(m *Model, owner *symbols.Symbol, values []*symbols.Symbol) []*symbols.Symbol {
	subject := m.SubjectParameterOf(owner)
	if subject == nil {
		return values
	}
	members := m.membersIncludingAnonymous(owner)
	subjectPosition := -1
	memberPositions := make(map[*symbols.Symbol]int, len(members))
	for i, member := range members {
		memberPositions[member] = i
		if member == subject {
			subjectPosition = i
		}
	}
	ordered := make([]*symbols.Symbol, 0, len(values)+1)
	for _, value := range values {
		if value != subject {
			ordered = append(ordered, value)
		}
	}
	position := len(ordered)
	if subjectPosition >= 0 {
		for i, value := range ordered {
			if valuePosition, ok := memberPositions[value]; ok && valuePosition > subjectPosition {
				position = i
				break
			}
		}
	} else if m.ownerOf(subject) == owner && subject.Decl != nil {
		for i, value := range ordered {
			if value != nil && value.OwnerScope == subject.OwnerScope &&
				subject.DeclSpan.Offset < value.DeclSpan.Offset {
				position = i
				break
			}
		}
	}
	out := make([]*symbols.Symbol, 0, len(ordered)+1)
	out = append(out, ordered[:position]...)
	out = append(out, subject)
	out = append(out, ordered[position:]...)
	return out
}

func (m *Model) reflectiveFeatureTypes(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	if sym.Recorded() && sym.Facts.Node == symbols.NodePrefixMetadata {
		if typ := m.recordedElement(sym.Facts.MetadataType); typ != nil {
			return []*symbols.Symbol{typ}
		}
		return nil
	}
	if prefix, ok := sym.Decl.(*ast.PrefixMetadata); ok {
		if annot, resolved := m.prefixAnnotation(sym.OwnerScope, prefix); resolved && annot.typ != nil {
			return []*symbols.Symbol{annot.typ}
		}
	}
	types := append([]*symbols.Symbol(nil), m.FeatureTypeSet(sym)...)
	if !m.reflectiveMetaclassConforms(sym, "PartUsage") || m.resolver == nil || m.resolver.Index() == nil {
		return types
	}
	for _, base := range m.resolver.Index().LookupQualified("Parts::parts") {
		if base == nil || !base.IsFeature() {
			continue
		}
		for _, typ := range m.baseFeatureTypes(base, nil) {
			typFQN := m.fqnOf(typ)
			found := false
			for _, existing := range types {
				if existing == typ || m.fqnOf(existing) == typFQN {
					found = true
					break
				}
			}
			if !found {
				types = append(types, typ)
			}
		}
		break
	}
	return m.mostSpecificTypes(types)
}

func (m *Model) conformingTypes(sym *symbols.Symbol, target string) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, typ := range m.reflectiveFeatureTypes(sym) {
		if m.reflectiveMetaclassConforms(typ, target) {
			out = append(out, typ)
		}
	}
	return out
}

func (m *Model) definitionTypedFeatures(sym *symbols.Symbol, feature string) []*symbols.Symbol {
	typed, ok := reflectiveDefinitionFeatures[feature]
	if !ok {
		return nil
	}
	return m.conformingTypes(sym, typed.target)
}

// dependencyEnds is the elements one side of a dependency names, in order; a
// name resolving to nothing is a resolver diagnostic, not an element.
func (m *Model) dependencyEnds(sym *symbols.Symbol, names []*ast.QualifiedName) []*symbols.Symbol {
	ends := make([]*symbols.Symbol, 0, len(names))
	for _, name := range names {
		if end, ok := m.resolver.ResolveQualified(sym.OwnerScope, name); ok && end != nil {
			ends = append(ends, m.resolver.AliasedElement(end))
		}
	}
	return ends
}

// ownedElementsOf is Element::ownedElement: the members sym's body declares,
// the named ones in declaration order and then those declared without a
// name, and its documentation.
func (m *Model) ownedElementsOf(sym *symbols.Symbol) []*symbols.Symbol {
	members := ownedMembersOf(sym)
	seen := make(map[*symbols.Symbol]bool, len(members))
	for _, member := range members {
		seen[member] = true
	}
	if sym.Scope != nil {
		sym.Scope.ForEachAnonymousMember(func(member *symbols.Symbol) bool {
			if member.Kind != symbols.SymbolAlias && !seen[member] {
				seen[member] = true
				members = append(members, member)
			}
			return true
		})
	}
	for _, doc := range m.documentationSymbols(sym) {
		if !seen[doc] {
			seen[doc] = true
			members = append(members, doc)
		}
	}
	return members
}

// connectorRelatedFeatures is Connector::relatedFeature, sourceFeature or
// targetFeature: the features its ends reference, all, the first, or the rest,
// in end order.
func (m *Model) connectorRelatedFeatures(sym *symbols.Symbol, feature string) []*symbols.Symbol {
	var related []*symbols.Symbol
	if sym.Recorded() {
		related = m.recordedElements(sym, sym.Facts.RelatedFeatures)
	} else {
		ends := m.EndFeatures(sym)
		paths := m.reflectiveConnectorEndPaths(sym)
		successionEnds := m.connectorSuccessionRelatedFeatures(sym)
		count := max(len(ends), len(paths), len(successionEnds))
		for i := 0; i < count; i++ {
			var target *symbols.Symbol
			if i < len(paths) && len(paths[i].Features) > 0 {
				path := paths[i].Features
				target = path[len(path)-1]
			}
			if target == nil && i < len(successionEnds) {
				target = successionEnds[i]
			}
			if target == nil && i < len(ends) {
				target = m.ReferencedFeature(ends[i])
			}
			if target != nil {
				related = append(related, target)
			}
		}
	}
	switch feature {
	case "sourceFeature":
		if len(related) > 1 {
			return related[:1]
		}
	case "targetFeature":
		if len(related) > 1 {
			return related[1:]
		}
		return nil
	}
	return related
}

func (m *Model) reflectiveConnectorEndPaths(sym *symbols.Symbol) []ConnectorEndPath {
	if usage, ok := sym.Decl.(*ast.Usage); ok &&
		usage.Kind == ast.UsageFlow && usage.Keyword == "message" && usage.FlowEnds != nil {
		return []ConnectorEndPath{
			{Name: "source", Features: m.attachmentPath(sym.OwnerScope, usage.FlowEnds.From)},
			{Name: "target", Features: m.attachmentPath(sym.OwnerScope, usage.FlowEnds.To)},
		}
	}
	return m.ConnectorEndPaths(sym)
}

func (m *Model) connectorSuccessionRelatedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Recorded() || sym.Decl == nil || sym.OwnerScope == nil {
		return nil
	}
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		if decl.Kind != ast.UsageSuccession && !decl.IsSuccessionFlow() {
			return nil
		}
	case *ast.SuccessionEdge, *ast.InitialNode, *ast.ControlFlowEdge, *ast.TransitionMember:
	default:
		return nil
	}
	owner := m.ownerOf(sym)
	if owner == nil {
		return nil
	}
	var ends []ActionSuccessionEnd
	found := false
	for _, succession := range m.ActionSuccessions(owner) {
		if succession.Decl != sym.Decl {
			continue
		}
		ends = []ActionSuccessionEnd{succession.Source, succession.Target}
		found = true
		break
	}
	if !found {
		for _, succession := range m.DeclaredSuccessions(sym.OwnerScope, owner, []ast.Node{sym.Decl}) {
			if succession.Decl == sym.Decl {
				ends = []ActionSuccessionEnd{succession.Source, succession.Target}
				break
			}
		}
	}
	var related []*symbols.Symbol
	for _, end := range ends {
		target := end.Symbol
		if target == nil && end.Node != nil {
			target = memberSymbol(sym.OwnerScope, end.Node)
		}
		if target != nil {
			related = append(related, target)
		}
	}
	return related
}

// ownedMembersOf is every element sym's own body declares, in declaration order:
// an alias is a membership rather than an element, and a name registered twice
// (short and primary) is one element.
func ownedMembersOf(sym *symbols.Symbol) []*symbols.Symbol {
	if sym.Scope == nil {
		return nil
	}
	var members []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind != symbols.SymbolAlias && !seen[member] {
			seen[member] = true
			members = append(members, member)
		}
		return true
	})
	return members
}

// membersIncludingAnonymous is Namespace::member: the named and anonymous
// members MembersOf reports, in declaration order, with inherited members that
// no feature of sym redefines.
func (m *Model) membersIncludingAnonymous(sym *symbols.Symbol) []*symbols.Symbol {
	named := m.MembersOf(sym)
	namedSet := make(map[*symbols.Symbol]bool, len(named))
	for _, s := range named {
		namedSet[s] = true
	}
	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool, len(named))
	mask := m.redefinitionMask(sym, true)
	collect := func(scope *symbols.Scope, inherited bool) {
		if scope == nil {
			return
		}
		anonymous := make(map[*symbols.Symbol]bool)
		for _, s := range scope.AnonymousMembers() {
			anonymous[s] = true
		}
		scope.ForEachMember(func(s *symbols.Symbol) bool {
			if s.Kind == symbols.SymbolAlias || seen[s] ||
				(!anonymous[s] && !namedSet[s]) ||
				(inherited && m.maskedBy(mask, s)) {
				return true
			}
			seen[s] = true
			out = append(out, s)
			return true
		})
	}
	collect(sym.Scope, false)
	for _, src := range m.MemberSources(sym) {
		collect(src.Scope, true)
	}
	for _, s := range named {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ReflectiveDirection is the direction sym's feature declaration states
// (Feature::direction); ok is false where sym declares no feature.
func ReflectiveDirection(sym *symbols.Symbol) (ast.FeatureDirection, bool) {
	if sym != nil && sym.Recorded() {
		if !sym.IsFeature() {
			return ast.DirNone, false
		}
		if sym.Facts.Node == symbols.NodeSubject {
			return ast.DirIn, true
		}
		return sym.Facts.Direction, true
	}
	if sym == nil || !sym.IsFeature() {
		return ast.DirNone, false
	}
	if _, ok := sym.Decl.(*ast.SubjectMember); ok {
		return ast.DirIn, true
	}
	traits, ok := featureTraitsOf(sym)
	if ok {
		return traits.Direction, true
	}
	return ast.DirNone, true
}

// ReflectiveFeatureValue reads a metaclass feature derived from the
// candidate's declaration; ok is false where none is derived.
func (m *Model) ReflectiveFeatureValue(sym *symbols.Symbol, feature string) (symbols.FilterValue, bool) {
	if m == nil || sym == nil {
		return symbols.FilterValue{}, false
	}
	return m.reflectiveFeatureValue(sym, feature)
}

// ReflectiveFeatureValues is ReflectiveFeatureValue for a feature that may hold
// several values: Element::documentation reads one per `doc` body, in order.
func (m *Model) ReflectiveFeatureValues(sym *symbols.Symbol, feature string) ([]symbols.FilterValue, bool) {
	if m == nil || sym == nil {
		return nil, false
	}
	if feature == "documentation" {
		bodies := m.DocumentationOf(sym)
		values := make([]symbols.FilterValue, 0, len(bodies))
		for _, body := range bodies {
			values = append(values, symbols.FilterValue{Kind: symbols.FilterValueString, Str: body})
		}
		return values, true
	}
	if elements, ok := m.ReflectiveElements(sym, feature); ok {
		values := make([]symbols.FilterValue, 0, len(elements))
		for _, element := range elements {
			if fqn := m.fqnOf(element); fqn != "" {
				values = append(values, symbols.FilterValue{Kind: symbols.FilterValueRef, RefFQN: fqn})
			}
		}
		return values, true
	}
	value, ok := m.reflectiveFeatureValue(sym, feature)
	if !ok {
		return nil, false
	}
	if value.Kind == symbols.FilterValueEmpty {
		return nil, true
	}
	return []symbols.FilterValue{value}, true
}

// reflectiveFeatureValue is what the candidate's declaration states for a
// metaclass feature of it, and whether that feature is derived here at all
// (KerML 1.1 §8.2.4); an underived one is unevaluable, not false.
func (m *Model) reflectiveFeatureValue(sym *symbols.Symbol, feature string) (symbols.FilterValue, bool) {
	switch feature {
	case "name", "declaredName":
		return stringOrEmpty(simpleSymbolName(sym)), true
	case "shortName":
		return stringOrEmpty(m.EffectiveShortNameOf(sym)), true
	case "declaredShortName":
		return stringOrEmpty(sym.ShortName), true
	case "qualifiedName":
		// Element::qualifiedName is null for an unnamed element (KerML 1.1 §8.3.2.1).
		if simpleSymbolName(sym) == "" {
			return emptyValue(), true
		}
		return stringOrEmpty(m.fqnOf(sym)), true
	case "direction":
		direction, ok := ReflectiveDirection(sym)
		if !ok {
			return symbols.FilterValue{}, false
		}
		if direction == ast.DirNone {
			return emptyValue(), true
		}
		return stringOrEmpty(direction.String()), true
	case "portionKind":
		if !m.reflectiveMetaclassConforms(sym, "OccurrenceUsage") {
			return symbols.FilterValue{}, false
		}
		if sym.Recorded() {
			return stringOrEmpty(sym.Facts.Portion.Keyword()), true
		}
		if usage, ok := sym.Decl.(*ast.Usage); ok {
			return stringOrEmpty(usage.Portion.Keyword()), true
		}
		return emptyValue(), true
	}
	if feature == "isAbstract" && sym.Recorded() && sym.Kind.IsDefinition() {
		return boolValue(sym.Facts.Abstract || IsVariation(sym)), true
	}
	if feature == "isSufficient" && sym.Recorded() && m.reflectiveMetaclassConforms(sym, "Type") {
		return boolValue(sym.Facts.Modifiers.Has(symbols.ModAll) ||
			m.reflectiveMetaclassConforms(sym, "ConnectionDefinition")), true
	}
	if value, ok := m.reflectiveFeatureBoolean(sym, feature); ok {
		return value, true
	}
	switch feature {
	case "isVariation":
		isDefinition := sym.Kind.IsDefinition() &&
			sym.Kind != symbols.SymbolMetaclass && sym.Kind != symbols.SymbolKerMLType &&
			!m.isKerMLDoc(sym)
		if !isDefinition &&
			!m.reflectiveMetaclassConforms(sym, "Usage") {
			return symbols.FilterValue{}, false
		}
		return boolValue(IsVariation(sym)), true
	case "isIndividual":
		if !m.reflectiveMetaclassConforms(sym, "OccurrenceDefinition") &&
			!m.reflectiveMetaclassConforms(sym, "OccurrenceUsage") {
			return symbols.FilterValue{}, false
		}
		hasIndividualModifier := sym.Facts != nil &&
			sym.Facts.Modifiers.Has(symbols.ModIndividual)
		if sym.Recorded() {
			return boolValue(hasIndividualModifier), true
		}
		switch d := sym.Decl.(type) {
		case *ast.Definition:
			return boolValue(d.IsIndividual), true
		case *ast.Usage:
			return boolValue(d.IsIndividual), true
		default:
			return boolValue(hasIndividualModifier), true
		}
	}
	switch d := sym.Decl.(type) {
	case *ast.Comment:
		switch feature {
		case "body":
			return m.reflectiveCommentBody(sym, d.BodySpan)
		case "locale":
			return stringOrEmpty(source.StringValue(d.Locale)), true
		}
	case *ast.Documentation:
		switch feature {
		case "body":
			return m.reflectiveCommentBody(sym, d.BodySpan)
		case "locale":
			return stringOrEmpty(source.StringValue(d.Locale)), true
		}
	case *ast.TextualRepresentation:
		switch feature {
		case "body":
			return m.reflectiveCommentBody(sym, d.BodySpan)
		case "language":
			return stringOrEmpty(source.StringValue(d.Language)), true
		}
	case *ast.Definition:
		switch feature {
		case "isAbstract":
			return boolValue(d.IsAbstract || IsVariation(sym)), true
		case "isSufficient":
			return boolValue(d.IsAll || m.reflectiveMetaclassConforms(sym, "ConnectionDefinition")), true
		case "isConstant":
			return boolValue(d.IsConstant), true
		case "isParallel":
			return boolValue(d.IsParallel), true
		}
	case *ast.Usage:
		switch feature {
		case "isSufficient":
			return boolValue(d.IsAll || m.reflectiveMetaclassConforms(sym, "ConnectionDefinition")), true
		case "isVariant":
			return boolValue(IsVariant(sym)), true
		case "isParallel":
			return boolValue(d.IsParallel), true
		}
	}
	if sym.Recorded() && feature == "isIndividual" {
		if sym.Facts.Node == symbols.NodeDefinition || sym.Facts.Node == symbols.NodeUsage {
			return boolValue(sym.Facts.Modifiers.Has(symbols.ModIndividual)), true
		}
	}
	return symbols.FilterValue{}, false
}

type reflectiveFeatureFlags struct {
	isEnd       bool
	isPortion   bool
	isConstant  bool
	isVariable  bool
	isComposite bool
	isDerived   bool
	isAbstract  bool
	isOrdered   bool
	isUnique    bool
	isReference bool
}

func (m *Model) reflectiveFeatureBoolean(sym *symbols.Symbol, feature string) (symbols.FilterValue, bool) {
	if !m.isReflectiveFeature(sym) {
		return symbols.FilterValue{}, false
	}
	if sym.Recorded() && !m.isKerMLDoc(sym) && m.metaclassConforms(sym, sysmlMetaclassPrefix+"Usage") {
		switch feature {
		case "isVariable":
			// Records omit the Usage::portion prefix, so mayTimeVary is not decidable without the AST.
			return symbols.FilterValue{}, false
		case "isConstant":
			if sym.Facts.Modifiers.Has(symbols.ModEnd) && !sym.Facts.Modifiers.Has(symbols.ModConstant) {
				return symbols.FilterValue{}, false
			}
		}
	}
	flags, isUsage := m.reflectiveFeatureFlags(sym)
	switch feature {
	case "isEnd":
		return boolValue(flags.isEnd), true
	case "isPortion":
		return boolValue(flags.isPortion), true
	case "isConstant":
		return boolValue(flags.isConstant), true
	case "isVariable":
		return boolValue(flags.isVariable), true
	case "isComposite":
		return boolValue(flags.isComposite), true
	case "isDerived":
		return boolValue(flags.isDerived), true
	case "isAbstract":
		return boolValue(flags.isAbstract), true
	case "isOrdered":
		return boolValue(flags.isOrdered), true
	case "isUnique":
		return boolValue(flags.isUnique), true
	case "isReference":
		if isUsage {
			return boolValue(flags.isReference), true
		}
	}
	return symbols.FilterValue{}, false
}

func (m *Model) isReflectiveFeature(sym *symbols.Symbol) bool {
	return sym.IsFeature() || m.metaclassConforms(sym, "KerML::Core::Feature")
}

func (m *Model) reflectiveFeatureFlags(sym *symbols.Symbol) (reflectiveFeatureFlags, bool) {
	flags := reflectiveFeatureFlags{isUnique: true}
	isUsage := !m.isKerMLDoc(sym) && m.metaclassConforms(sym, sysmlMetaclassPrefix+"Usage")

	if sym.Recorded() {
		mods := sym.Facts.Modifiers
		flags.isEnd = mods.Has(symbols.ModEnd)
		flags.isPortion = mods.Has(symbols.ModPortion)
		flags.isConstant = mods.Has(symbols.ModConstant)
		flags.isVariable = mods.Has(symbols.ModVariable) ||
			(m.isKerMLDoc(sym) && flags.isConstant)
		flags.isDerived = mods.Has(symbols.ModDerived)
		flags.isAbstract = sym.Facts.Abstract || IsVariation(sym)
		flags.isOrdered = mods.Has(symbols.ModOrdered)
		flags.isUnique = !mods.Has(symbols.ModNonunique)
		if isUsage {
			flags.isComposite = mods.Has(symbols.ModComposite) || !recordedUsageIsReferential(sym)
		} else {
			flags.isComposite = mods.Has(symbols.ModComposite)
		}
	} else {
		flags.isVariable = m.FeatureIsVariable(sym)
		switch d := sym.Decl.(type) {
		case *ast.Usage:
			flags.isEnd = d.IsEnd
			flags.isPortion = d.IsPortion || d.Portion != ast.PortionNone
			flags.isConstant = d.IsConstant
			if !m.isKerMLDoc(sym) && d.IsEnd && m.FeatureIsVariable(sym) {
				// Mirror the pilot's implicit constant ends.
				flags.isConstant = true
			}
			flags.isDerived = d.IsDerived
			flags.isAbstract = d.IsAbstract || IsVariation(sym)
			flags.isOrdered = d.IsOrdered
			flags.isUnique = !d.IsNonunique
			if isUsage {
				flags.isComposite = d.IsComposite || !usageIsReferential(d)
			} else {
				flags.isComposite = d.IsComposite
			}
		case *ast.CrossFeatureMember:
			flags.isEnd = true
			flags.isPortion = d.IsPortion
			flags.isConstant = d.IsConstant
			flags.isDerived = d.IsDerived
			flags.isAbstract = d.IsAbstract
			flags.isOrdered = d.IsOrdered
			flags.isUnique = !d.IsNonunique
			flags.isComposite = d.IsComposite
		case *ast.ConnectorEnd:
			flags.isEnd = true
		default:
			if isUsage {
				flags.isComposite = defaultUsageComposite(sym.Kind)
			}
		}
	}

	if isUsage && reflectiveMessageFlowUsage(sym) &&
		len(m.connectorRelatedFeatures(sym, "relatedFeature")) < 2 {
		flags.isAbstract = true
	}

	owner := sym.Owner()
	// The census records pilot silence for attribute-composite members
	// (validateAttributeDefinitionFeatures, validateAttributeUsageFeatures; docs/project/validation-constraints.md).
	if m.reflectiveAttributeFeatureOwner(owner) {
		flags.isComposite = false
	}
	if isUsage && m.metaclassConforms(sym, sysmlMetaclassPrefix+"PortUsage") &&
		(owner == nil ||
			(!m.reflectiveMetaclassConforms(owner, "PortDefinition") &&
				!m.reflectiveMetaclassConforms(owner, "PortUsage"))) {
		flags.isComposite = false
	}
	if m.metaclassConforms(sym, sysmlMetaclassPrefix+"ControlNode") {
		flags.isComposite = true
	}
	if isUsage {
		direction, _ := ReflectiveDirection(sym)
		if flags.isEnd || direction != ast.DirNone || !m.reflectiveUsageHasFeaturingType(sym) {
			flags.isComposite = false
		}
	}
	flags.isReference = isUsage && !flags.isComposite
	return flags, isUsage
}

func reflectiveMessageFlowUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	kind, ok := sym.UsageKind()
	return ok && kind == ast.UsageFlow && sym.Keyword() == "message"
}

func (m *Model) reflectiveUsageHasFeaturingType(sym *symbols.Symbol) bool {
	if IsVariant(sym) {
		owner := m.ownerOf(sym)
		if owner == nil || !m.reflectiveMetaclassConforms(owner, "Usage") || !IsVariation(owner) {
			return false
		}
		return m.reflectiveUsageHasFeaturingType(owner)
	}
	featuringTypes, ok := m.ReflectiveElements(sym, "featuringType")
	return ok && len(featuringTypes) > 0
}

func defaultUsageComposite(kind symbols.SymbolKind) bool {
	switch kind {
	case symbols.SymbolAttributeUsage, symbols.SymbolReferenceUsage,
		symbols.SymbolEnumerationUsage, symbols.SymbolConnectorEnd:
		return false
	default:
		return true
	}
}

func recordedUsageIsReferential(sym *symbols.Symbol) bool {
	if sym.Kind.IsAttributeLike() || sym.Kind == symbols.SymbolEnumerationUsage ||
		sym.Kind == symbols.SymbolReferenceUsage || sym.Kind == symbols.SymbolConnectorEnd {
		return true
	}
	mods := sym.Facts.Modifiers
	if mods.Has(symbols.ModReference) || mods.Has(symbols.ModEnd) || mods.Has(symbols.ModEvent) ||
		sym.Facts.Direction != ast.DirNone ||
		sym.Facts.UsageKind == ast.UsageEnumeration {
		return true
	}
	if isVariantReferenceSymbol(sym) {
		return true
	}
	for _, rel := range sym.Facts.Relationships {
		if rel.Kind == ast.RelReferences {
			return true
		}
	}
	return false
}

func isVariantReferenceSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok {
		return usage.IsVariantReference()
	}
	if !sym.Recorded() {
		return false
	}
	mods := sym.Facts.Modifiers
	prefixes := symbols.ModReference | symbols.ModVariable | symbols.ModConstant |
		symbols.ModEnd | symbols.ModDerived | symbols.ModComposite | symbols.ModPortion |
		symbols.ModVariation | symbols.ModChain | symbols.ModEvent | symbols.ModIndividual
	return sym.Facts.Keyword == "variant" && mods.Has(symbols.ModVariant) &&
		!sym.Facts.Abstract && mods&prefixes == 0 && sym.Facts.Direction == ast.DirNone
}

func (m *Model) reflectiveAttributeFeatureOwner(owner *symbols.Symbol) bool {
	return owner != nil &&
		(m.metaclassConforms(owner, sysmlMetaclassPrefix+"AttributeDefinition") ||
			m.metaclassConforms(owner, sysmlMetaclassPrefix+"AttributeUsage"))
}

// reflectiveCommentBody is Comment::body, String[1..1]: "" for a blank comment, and
// underived for a model whose notation was never given (SetSourceText).
func (m *Model) reflectiveCommentBody(sym *symbols.Symbol, span source.Span) (symbols.FilterValue, bool) {
	if m.sourceText == nil {
		return symbols.FilterValue{}, false
	}
	return symbols.FilterValue{Kind: symbols.FilterValueString, Str: m.commentBody(sym, span)}, true
}

// stringOrEmpty is a string value, or the empty sequence for a name the
// declaration does not have.
func stringOrEmpty(s string) symbols.FilterValue {
	if s == "" {
		return emptyValue()
	}
	return symbols.FilterValue{Kind: symbols.FilterValueString, Str: s}
}

// metaclassNames maps each declaration kind to its reflective SysML metadata
// type.
var metaclassNames = map[symbols.SymbolKind]string{
	symbols.SymbolPackage:                 "Package",
	symbols.SymbolNamespace:               "Namespace",
	symbols.SymbolPartDef:                 "PartDefinition",
	symbols.SymbolPartUsage:               "PartUsage",
	symbols.SymbolAttributeDef:            "AttributeDefinition",
	symbols.SymbolAttributeUsage:          "AttributeUsage",
	symbols.SymbolReferenceUsage:          "ReferenceUsage",
	symbols.SymbolItemDef:                 "ItemDefinition",
	symbols.SymbolItemUsage:               "ItemUsage",
	symbols.SymbolOccurrenceDef:           "OccurrenceDefinition",
	symbols.SymbolOccurrenceUsage:         "OccurrenceUsage",
	symbols.SymbolIndividualUsage:         "OccurrenceUsage",
	symbols.SymbolIndividualDef:           "OccurrenceDefinition",
	symbols.SymbolMetadataDef:             "MetadataDefinition",
	symbols.SymbolMetadataUsage:           "MetadataUsage",
	symbols.SymbolEnumerationDef:          "EnumerationDefinition",
	symbols.SymbolEnumerationUsage:        "EnumerationUsage",
	symbols.SymbolViewDef:                 "ViewDefinition",
	symbols.SymbolViewUsage:               "ViewUsage",
	symbols.SymbolViewpointDef:            "ViewpointDefinition",
	symbols.SymbolViewpointUsage:          "ViewpointUsage",
	symbols.SymbolRenderingDef:            "RenderingDefinition",
	symbols.SymbolRenderingUsage:          "RenderingUsage",
	symbols.SymbolConcernDef:              "ConcernDefinition",
	symbols.SymbolConcernUsage:            "ConcernUsage",
	symbols.SymbolConnectionDef:           "ConnectionDefinition",
	symbols.SymbolConnectionUsage:         "ConnectionUsage",
	symbols.SymbolBindingUsage:            "BindingConnectorAsUsage",
	symbols.SymbolSuccessionUsage:         "SuccessionAsUsage",
	symbols.SymbolFlowDef:                 "FlowDefinition",
	symbols.SymbolFlowUsage:               "FlowUsage",
	symbols.SymbolPortDef:                 "PortDefinition",
	symbols.SymbolPortUsage:               "PortUsage",
	symbols.SymbolInterfaceDef:            "InterfaceDefinition",
	symbols.SymbolInterfaceUsage:          "InterfaceUsage",
	symbols.SymbolAllocationDef:           "AllocationDefinition",
	symbols.SymbolAllocationUsage:         "AllocationUsage",
	symbols.SymbolActionDef:               "ActionDefinition",
	symbols.SymbolActionUsage:             "ActionUsage",
	symbols.SymbolStateDef:                "StateDefinition",
	symbols.SymbolStateUsage:              "StateUsage",
	symbols.SymbolCalcDef:                 "CalculationDefinition",
	symbols.SymbolCalcUsage:               "CalculationUsage",
	symbols.SymbolConstraintDef:           "ConstraintDefinition",
	symbols.SymbolConstraintUsage:         "ConstraintUsage",
	symbols.SymbolRequirementDef:          "RequirementDefinition",
	symbols.SymbolRequirementUsage:        "RequirementUsage",
	symbols.SymbolCaseDef:                 "CaseDefinition",
	symbols.SymbolCaseUsage:               "CaseUsage",
	symbols.SymbolAnalysisCaseDef:         "AnalysisCaseDefinition",
	symbols.SymbolAnalysisCaseUsage:       "AnalysisCaseUsage",
	symbols.SymbolVerificationCaseDef:     "VerificationCaseDefinition",
	symbols.SymbolVerificationCaseUsage:   "VerificationCaseUsage",
	symbols.SymbolUseCaseDef:              "UseCaseDefinition",
	symbols.SymbolUseCaseUsage:            "UseCaseUsage",
	symbols.SymbolSatisfyRequirementUsage: "SatisfyRequirementUsage",
	symbols.SymbolCrossFeature:            referenceUsageMetaclassName,
}

// metaclassName is the reflective SysML metadata type classifying a declaration
// of the given kind, or "" for a kind the abstract syntax has no metaclass for.
func metaclassName(kind symbols.SymbolKind) string {
	return metaclassNames[kind]
}
