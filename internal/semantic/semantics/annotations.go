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
	// bound is what the body binds; defaults is typ's declared values, one map
	// shared by every annotation of typ, read for what the body leaves unbound.
	bound    map[string]symbols.FilterValue
	defaults map[string]symbols.FilterValue
	// node states the annotation: the prefix-metadata node or metadata usage.
	node ast.Node
	// scope is where the annotating node is declared.
	scope *symbols.Scope
	// about marks an annotation stated elsewhere with an `about` clause.
	about bool
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
	typ := m.recordedElement(symbols.ElementRef{FQN: facts.TypeFQN})
	if typ == nil {
		return annotation{}, false
	}
	bound := make(map[string]symbols.FilterValue, len(facts.Values))
	for _, v := range facts.Values {
		bound[v.Feature] = v.Value
	}
	return annotation{typ: typ, bound: bound, scope: scope}, true
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

// AnnotatedElementsOf returns the elements a metadata usage annotates through
// its `about` clause, resolved; nil for any other symbol.
func (m *Model) AnnotatedElementsOf(sym *symbols.Symbol) []*symbols.Symbol {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || sym.Kind != symbols.SymbolMetadataUsage || !annotatesOthers(usage) {
		return nil
	}
	return m.annotatedElements(sym.OwnerScope, usage)
}

// AnnotationFactsOf states the metadata annotating sym as names and constants,
// so that how an element filter classifies it can be compared across loads. The
// values an annotation binds are reported as read; a binding whose value is not
// constant is reported with an unknown value, which a condition reading it
// reports as unevaluable rather than silently treating as absent.
func (m *Model) AnnotationFactsOf(sym *symbols.Symbol) []symbols.AnnotationFacts {
	return annotationFacts(m, m.annotationsOf(sym))
}

// annotationFacts states annotations as names and constants.
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
		facts := symbols.AnnotationFacts{TypeFQN: typFQN}
		for _, feature := range a.featureNames() {
			value, _ := a.value(feature)
			facts.Values = append(facts.Values, symbols.AnnotationValueFacts{
				Feature: feature,
				Value:   value,
			})
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
	// Scope is where the annotating node is declared; for an `about`-form
	// annotation that may be another document than the annotated element's.
	Scope  *symbols.Scope
	About  bool
	Values []symbols.AnnotationValueFacts
}

// AnnotationSitesOf returns the metadata annotating sym with the nodes stating
// it, inline annotations first and `about`-form ones after, each in
// declaration order. An annotation a recorded symbol's own declaration stated
// has no node: its record carries the type and values, not the tree.
func (m *Model) AnnotationSitesOf(sym *symbols.Symbol) []AnnotationSite {
	var out []AnnotationSite
	for _, a := range m.annotationsOf(sym) {
		var typFQN string
		if a.typ != nil {
			typFQN = m.fqnOf(a.typ)
		}
		if typFQN == "" || (a.node == nil && !sym.Recorded()) {
			continue
		}
		site := AnnotationSite{TypeFQN: typFQN, Node: a.node, Scope: a.scope, About: a.about}
		for _, feature := range a.featureNames() {
			value, _ := a.value(feature)
			site.Values = append(site.Values, symbols.AnnotationValueFacts{
				Feature: feature,
				Value:   value,
			})
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

// value is what the annotation binds feature to: by its body, else by its type's default.
func (a annotation) value(feature string) (symbols.FilterValue, bool) {
	if v, ok := a.bound[feature]; ok {
		return v, true
	}
	v, ok := a.defaults[feature]
	return v, ok
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
func (m *Model) typeDefaults(typ *symbols.Symbol) map[string]symbols.FilterValue {
	if typ == nil {
		return nil
	}
	defer m.own(typ).LeaveDoc()
	if cached, ok := m.metadataDefaults[typ]; ok {
		return cached
	}
	journal(m, m.metadataDefaults, typ, typ.Decl)
	m.metadataDefaults[typ] = nil
	var values map[string]symbols.FilterValue
	for _, member := range m.MembersOf(typ) {
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || usage.Value == nil {
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
			values = make(map[string]symbols.FilterValue)
		}
		values[name] = m.annotationValue(member.OwnerScope, usage.Value)
	}
	m.metadataDefaults[typ] = values
	return values
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
	if len(sym.Facts.Annotations) == 0 || len(sym.Facts.About) == 0 {
		return
	}
	a, ok := m.annotationFromFacts(sym.Facts.Annotations[0], sym.OwnerScope)
	if !ok {
		return
	}
	a.about = true
	m.indexAbout(a, m.recordedElements(nil, sym.Facts.About))
}

// indexAbout files a as an annotation of each target.
func (m *Model) indexAbout(a annotation, targets []*symbols.Symbol) {
	for _, target := range targets {
		if _, known := m.aboutAnnots[target]; !known {
			m.aboutOrder = append(m.aboutOrder, target)
		}
		m.aboutAnnots[target] = append(m.aboutAnnots[target], a)
		if target.Decl != nil {
			m.aboutByDecl[target.Decl] = append(m.aboutByDecl[target.Decl], a)
		}
	}
}

// annotatedElements resolves the elements a metadata usage's `about` clause
// names.
func (m *Model) annotatedElements(scope *symbols.Scope, u *ast.Usage) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelAnnotates {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		if target, ok := m.resolver.ResolveQualified(scope, qn); ok && target != nil {
			out = append(out, target)
		}
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
func (m *Model) annotationValues(scope *symbols.Scope, body []ast.Node) map[string]symbols.FilterValue {
	var values map[string]symbols.FilterValue
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
			values = make(map[string]symbols.FilterValue)
		}
		values[name] = m.annotationValue(scope, usage.Value)
	}
	return values
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

// ConstantFeatureValues returns a feature's ordered constant values.
func (m *Model) ConstantFeatureValues(sym *symbols.Symbol, feature string) ([]symbols.FilterValue, bool) {
	if m == nil || sym == nil || feature == "" {
		return nil, false
	}
	if values, ok := m.ReflectiveFeatureValues(sym, feature); ok {
		return values, true
	}
	member, ok := m.LookupMember(sym, feature)
	if !ok || member == nil {
		return nil, false
	}
	return m.constantFeatureValues(member, make(map[*symbols.Symbol]bool))
}

// DeclaredFeatureValues returns a declared member feature's ordered constant
// values, never answering reflective metaclass features of the same name.
func (m *Model) DeclaredFeatureValues(sym *symbols.Symbol, feature string) ([]symbols.FilterValue, bool) {
	if m == nil || sym == nil || feature == "" {
		return nil, false
	}
	member, ok := m.LookupMember(sym, feature)
	if !ok || member == nil {
		return nil, false
	}
	return m.constantFeatureValues(member, make(map[*symbols.Symbol]bool))
}

func (m *Model) constantFeatureValues(member *symbols.Symbol, seen map[*symbols.Symbol]bool) ([]symbols.FilterValue, bool) {
	if member == nil || seen[member] {
		return nil, false
	}
	seen[member] = true
	defer delete(seen, member)
	usage, ok := member.Decl.(*ast.Usage)
	if !ok {
		return []symbols.FilterValue{{}}, true
	}
	if usage.Value == nil {
		var values []symbols.FilterValue
		found := false
		for _, redefined := range m.RedefinedFeatures(member) {
			inherited, ok := m.constantFeatureValues(redefined, seen)
			if !ok {
				continue
			}
			found = true
			values = append(values, inherited...)
		}
		if found {
			return values, true
		}
		return nil, true
	}
	if sequence, ok := usage.Value.(*ast.SequenceExpr); ok {
		values := make([]symbols.FilterValue, 0, len(sequence.Elements))
		for _, element := range sequence.Elements {
			if _, empty := element.(*ast.NullExpr); empty {
				continue
			}
			values = append(values, m.declaredValue(member.OwnerScope, element))
		}
		return values, true
	}
	if _, empty := usage.Value.(*ast.NullExpr); empty {
		return nil, true
	}
	return []symbols.FilterValue{m.declaredValue(member.OwnerScope, usage.Value)}, true
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
	if sym == nil || (sym.Kind != symbols.SymbolAttributeUsage && sym.Kind != symbols.SymbolEnumerationUsage) {
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
	if rel, ok := sym.Decl.(*ast.RelationshipMember); ok {
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
	switch sym.Kind {
	case symbols.SymbolConnectorEnd:
		return ConnectorEndMetaclassName(sym)
	case symbols.SymbolUnknown:
		if usage, ok := sym.Decl.(*ast.Usage); ok {
			return usageMetaclassNames[usage.Kind]
		}
	case symbols.SymbolActionUsage:
		if _, ok := sym.Decl.(*ast.TransitionMember); ok {
			return usageMetaclassNames[ast.UsageTransition]
		}
	}
	return metaclassName(sym.Kind)
}

// ConnectorEndMetaclassName is the SysML metaclass of a connector end: a
// PortUsage as an interface's end, a ReferenceUsage otherwise (SysML.xtext).
func ConnectorEndMetaclassName(sym *symbols.Symbol) string {
	if sym.OwnerScope != nil {
		if usage, ok := sym.OwnerScope.Node().(*ast.Usage); ok && usage.Kind == ast.UsageInterface {
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
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind == ast.UsageMetadata {
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
		if _, feature := owner.Decl.(*ast.Usage); !feature {
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

// relationshipMetaclassName is the metaclass of a keyword-first relationship,
// which conjugation writes as a form of its own (KerML §7.2).
func relationshipMetaclassName(rel *ast.RelationshipMember) string {
	if rel.Conjugated {
		return "Conjugation"
	}
	return relationshipMetaclassNames[rel.Kind]
}

// MultiplicityMetaclassName is the metaclass of a named multiplicity: a range
// (`multiplicity m [1..2]`) is a MultiplicityRange, a subset a Multiplicity.
func MultiplicityMetaclassName(sym *symbols.Symbol) string {
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
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return kermlMetaclassNames[d.Keyword]
	case *ast.Usage:
		return kermlMetaclassNames[d.Keyword]
	case *ast.PrefixMetadata:
		return kermlMetaclassNames["metadata"]
	case *ast.ConnectorEnd, *ast.CrossFeatureMember:
		return kermlMetaclassNames["feature"]
	}
	return ""
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
	case "owner":
		if sym.OwnerScope == nil || sym.OwnerScope.Owner() == nil {
			return nil, true
		}
		return []*symbols.Symbol{sym.OwnerScope.Owner()}, true
	case "ownedMember":
		return ownedMembersOf(sym), true
	case "documentation":
		return m.documentationSymbols(sym), true
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
		return m.FeatureTypeSet(sym), true
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
	return nil, false
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

// ReflectiveDirection is the direction sym's feature declaration states
// (Feature::direction); ok is false where sym declares no feature.
func ReflectiveDirection(sym *symbols.Symbol) (ast.FeatureDirection, bool) {
	traits, ok := featureTraitsOf(sym)
	if !ok {
		return ast.DirNone, false
	}
	return traits.Direction, true
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
			return boolValue(d.IsAbstract), true
		case "isSufficient":
			return boolValue(d.IsAll), true
		case "isConstant":
			return boolValue(d.IsConstant), true
		case "isVariation":
			return boolValue(IsVariation(sym)), true
		case "isIndividual":
			return boolValue(d.IsIndividual), true
		case "isParallel":
			return boolValue(d.IsParallel), true
		}
	case *ast.Usage:
		switch feature {
		case "isAbstract":
			return boolValue(d.IsAbstract), true
		case "isSufficient":
			return boolValue(d.IsAll), true
		case "isComposite":
			return boolValue(d.IsComposite || !usageIsReferential(d)), true
		case "isDerived":
			return boolValue(d.IsDerived), true
		case "isEnd":
			return boolValue(d.IsEnd), true
		case "isOrdered":
			return boolValue(d.IsOrdered), true
		case "isUnique":
			return boolValue(!d.IsNonunique), true
		case "isVariable":
			return boolValue(d.IsVariable), true
		case "isConstant":
			return boolValue(d.IsConstant), true
		case "isPortion":
			return boolValue(d.Portion != ast.PortionNone), true
		case "isVariation":
			return boolValue(IsVariation(sym)), true
		case "isVariant":
			return boolValue(IsVariant(sym)), true
		case "isReference":
			return boolValue(!d.IsComposite && usageIsReferential(d)), true
		case "isIndividual":
			return boolValue(d.IsIndividual), true
		case "isParallel":
			return boolValue(d.IsParallel), true
		}
	}
	return symbols.FilterValue{}, false
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
