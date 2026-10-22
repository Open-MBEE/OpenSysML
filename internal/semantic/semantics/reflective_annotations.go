package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// OwnedAnnotations returns the Annotations sym owns as their annotated element
// (KerML 8.3.2.1 Element::ownedAnnotation), each owning its annotating element:
// one per `#M` prefix metadata feature, and one per annotating element in a
// relationship body (KerML 8.2.3.3.1 OwnedAnnotation). An annotating element
// in a namespace body is owned through a membership and annotates the
// namespace through no Annotation (8.3.2.3.2 deriveAnnotatingElementAnnotatedElement).
func (m *Model) OwnedAnnotations(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.ownedAnnotations[sym]; ok {
		return cached
	}
	journal(m, m.ownedAnnotations, sym, sym.Decl)
	var out []*symbols.Symbol
	// A record keeps each prefix annotation as a fact, not as a symbol: the
	// Annotation is reflected with no annotating element to read through it.
	for _, span := range m.recordedPrefixAnnotationSpans(sym) {
		out = append(out, &symbols.Symbol{
			Kind:     symbols.SymbolRelationship,
			DocName:  sym.DocName,
			DeclSpan: span,
			Implicit: &symbols.ImplicitRelationship{
				Owner: sym, Ordinal: len(out), Form: symbols.ImplicitOwnedAnnotation,
			},
		})
	}
	for _, prefix := range m.prefixMetadataSymbols(sym) {
		out = append(out, &symbols.Symbol{
			Kind:     symbols.SymbolRelationship,
			DocName:  sym.DocName,
			DeclSpan: prefix.DeclSpan,
			Implicit: &symbols.ImplicitRelationship{
				Owner: sym, Ordinal: len(out), Form: symbols.ImplicitOwnedAnnotation, Member: prefix,
			},
		})
	}
	seen := make(map[*symbols.Symbol]bool)
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member == nil || seen[member] || !ownedThroughAnnotation(sym, member) {
			return true
		}
		seen[member] = true
		out = append(out, &symbols.Symbol{
			Kind:     symbols.SymbolRelationship,
			DocName:  sym.DocName,
			DeclSpan: member.DeclSpan,
			Implicit: &symbols.ImplicitRelationship{
				Owner: sym, Ordinal: len(out), Form: symbols.ImplicitOwnedAnnotation, Member: member,
			},
		})
		return true
	})
	m.ownedAnnotations[sym] = out
	return out
}

// recordedPrefixAnnotationSpans locates the `#M` prefixes of a recorded symbol:
// its record states every annotation its declaration made, the metadata usages
// in its body among them, and those are recorded members located by the same span.
func (m *Model) recordedPrefixAnnotationSpans(sym *symbols.Symbol) []source.Span {
	if !sym.Recorded() {
		return nil
	}
	body := make(map[source.Span]bool)
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member != nil && member.Kind == symbols.SymbolMetadataUsage {
			body[member.DeclSpan] = true
		}
		return true
	})
	var out []source.Span
	for _, fact := range sym.Facts.Annotations {
		if !body[fact.Span] {
			out = append(out, fact.Span)
		}
	}
	return out
}

// AnnotatingRelationships returns the Annotations the annotating element sym
// owns for the elements its `about` clause names, in clause order (KerML
// 8.3.2.3.2 AnnotatingElement::ownedAnnotatingRelationship); a name resolving
// to nothing is a resolver diagnostic, not an annotation.
func (m *Model) AnnotatingRelationships(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || !isAnnotatingSymbol(sym) {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.annotatingRels[sym]; ok {
		return cached
	}
	journal(m, m.annotatingRels, sym, sym.Decl)
	out := []*symbols.Symbol{}
	for _, target := range m.aboutTargets(sym) {
		out = append(out, &symbols.Symbol{
			Kind:     symbols.SymbolRelationship,
			DocName:  sym.DocName,
			DeclSpan: sym.DeclSpan,
			Implicit: &symbols.ImplicitRelationship{
				Owner: sym, Ordinal: len(out), Form: symbols.ImplicitAnnotatingRelationship, Member: target,
			},
		})
	}
	m.annotatingRels[sym] = out
	return out
}

// aboutTargets resolves the elements sym's `about` clause names, in order.
func (m *Model) aboutTargets(sym *symbols.Symbol) []*symbols.Symbol {
	if sym.Recorded() {
		return m.recordedElements(sym, sym.Facts.About)
	}
	var refs []*ast.QualifiedName
	switch d := sym.Decl.(type) {
	case *ast.Comment:
		refs = d.About
	default:
		refs = symbols.MetadataAboutRefs(sym.Decl)
	}
	var out []*symbols.Symbol
	for _, qn := range refs {
		target, ok := m.resolver.ResolveQualified(sym.OwnerScope, qn)
		if !ok || target == nil {
			continue
		}
		if resolved, ok := m.resolver.ResolveAliasTarget(target); ok && resolved != nil {
			target = resolved
		}
		out = append(out, target)
	}
	return out
}

// owningAnnotatingRelationship is the Annotation that owns the annotating
// element sym (KerML 8.3.2.3.2 AnnotatingElement::owningAnnotatingRelationship),
// or nil for one owned through a membership.
func (m *Model) owningAnnotatingRelationship(sym *symbols.Symbol) *symbols.Symbol {
	if !isAnnotatingSymbol(sym) {
		return nil
	}
	owner := m.ownerOf(sym)
	if isPrefixMetadataSymbol(sym) {
		owner = m.prefixMetadataOwner(sym)
	}
	if owner == nil {
		return nil
	}
	for _, ann := range m.OwnedAnnotations(owner) {
		if ann.Implicit.Member == sym {
			return ann
		}
	}
	return nil
}

// annotationsOfAnnotatingElement is AnnotatingElement::annotation: the owning
// annotating relationship, if any, then the owned ones (KerML 8.3.2.3.2).
func (m *Model) annotationsOfAnnotatingElement(sym *symbols.Symbol) []*symbols.Symbol {
	out := []*symbols.Symbol{}
	if owning := m.owningAnnotatingRelationship(sym); owning != nil {
		out = append(out, owning)
	}
	return append(out, m.AnnotatingRelationships(sym)...)
}

// annotatedElementsOf is AnnotatingElement::annotatedElement: the annotated
// elements of its annotations, else its owning namespace (KerML 8.3.2.3.2
// deriveAnnotatingElementAnnotatedElement).
func (m *Model) annotatedElementsOf(sym *symbols.Symbol) []*symbols.Symbol {
	annotations := m.annotationsOfAnnotatingElement(sym)
	if len(annotations) == 0 {
		if isPrefixMetadataSymbol(sym) {
			return []*symbols.Symbol{}
		}
		if owner := m.ownerOf(sym); owner != nil && m.reflectiveMetaclassConforms(owner, "Namespace") {
			return []*symbols.Symbol{owner}
		}
		return []*symbols.Symbol{}
	}
	out := make([]*symbols.Symbol, 0, len(annotations))
	for _, ann := range annotations {
		out = append(out, m.annotationAnnotatedElement(ann))
	}
	return out
}

// annotationAnnotatedElement is Annotation::annotatedElement of a reflected
// Annotation: the owner of an owned annotation, the named element of an
// annotating relationship.
func (m *Model) annotationAnnotatedElement(ann *symbols.Symbol) *symbols.Symbol {
	if ann.Implicit.Form == symbols.ImplicitOwnedAnnotation {
		return ann.Implicit.Owner
	}
	return ann.Implicit.Member
}

// annotationAnnotatingElement is Annotation::annotatingElement of a reflected
// Annotation.
func (m *Model) annotationAnnotatingElement(ann *symbols.Symbol) *symbols.Symbol {
	if ann.Implicit.Form == symbols.ImplicitOwnedAnnotation {
		return ann.Implicit.Member
	}
	return ann.Implicit.Owner
}

// annotatingElementElements is ReflectiveElements for the features of
// AnnotatingElement and its subclasses; ok is false for any other feature.
func (m *Model) annotatingElementElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	if !isAnnotatingSymbol(sym) {
		return nil, false
	}
	switch feature {
	case "annotation":
		return m.annotationsOfAnnotatingElement(sym), true
	case "ownedAnnotatingRelationship":
		return m.AnnotatingRelationships(sym), true
	case "owningAnnotatingRelationship":
		if owning := m.owningAnnotatingRelationship(sym); owning != nil {
			return []*symbols.Symbol{owning}, true
		}
		return []*symbols.Symbol{}, true
	case "annotatedElement":
		return m.annotatedElementsOf(sym), true
	case "documentedElement", "representedElement":
		// Documentation::documentedElement and TextualRepresentation::
		// representedElement redefine annotatedElement to the one owner.
		if feature == "documentedElement" && sym.Kind != symbols.SymbolDocumentation ||
			feature == "representedElement" && sym.Kind != symbols.SymbolTextualRepresentation {
			return nil, false
		}
		annotated := m.annotatedElementsOf(sym)
		if len(annotated) > 1 {
			annotated = annotated[:1]
		}
		return annotated, true
	}
	return nil, false
}

// annotationElements is ReflectiveElements for a reflected Annotation object.
func (m *Model) annotationElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	annotated := []*symbols.Symbol{m.annotationAnnotatedElement(sym)}
	owned := sym.Implicit.Form == symbols.ImplicitOwnedAnnotation
	empty := []*symbols.Symbol{}
	switch feature {
	case "annotatedElement", "target":
		return annotated, true
	case "owningRelatedElement", "owner":
		return []*symbols.Symbol{sym.Implicit.Owner}, true
	case "owningAnnotatedElement":
		if owned {
			return annotated, true
		}
		return empty, true
	case "ownedRelationship", "ownedElement", "ownedAnnotation", "owningRelationship", "owningMembership",
		"owningNamespace", "documentation", "textualRepresentation":
		return empty, true
	}
	annotatingElement := m.annotationAnnotatingElement(sym)
	if annotatingElement == nil {
		return nil, false
	}
	annotating := []*symbols.Symbol{annotatingElement}
	switch feature {
	case "annotatingElement", "source":
		return annotating, true
	case "relatedElement":
		return append(annotating, annotated...), true
	case "owningAnnotatingElement":
		if owned {
			return empty, true
		}
		return annotating, true
	case "ownedAnnotatingElement", "ownedRelatedElement":
		if owned {
			return annotating, true
		}
		return empty, true
	}
	return nil, false
}

// prefixMetadataOwner is the element a synthesized prefix metadata feature is
// written on, found through the annotation that owns it.
func (m *Model) prefixMetadataOwner(sym *symbols.Symbol) *symbols.Symbol {
	for owner, annotations := range m.ownedAnnotations {
		for _, ann := range annotations {
			if ann.Implicit.Member == sym {
				return owner
			}
		}
	}
	return nil
}
