package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ImplicitRelationships returns the specialization-family relationship objects
// sym owns — the relationships written as notation on its declaration, each
// reflected as an element of its own (KerML 8.3.3) — in textual order. The
// objects are synthesized per model, memoized so every call returns the same
// pointers, and are never registered in a scope or index. Kinds outside the
// family still consume their ordinal, so AST and recorded owners enumerate
// alike.
func (m *Model) ImplicitRelationships(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.implicitRels[sym]; ok {
		return cached
	}
	journal(m, m.implicitRels, sym, sym.Decl)
	out := m.implicitRelationshipsOf(sym)
	m.implicitRels[sym] = out
	return out
}

// implicitRelationshipsOf enumerates sym's written relationships: among the
// non-nil entries of RelationshipsOf for a declaration, among the recorded
// facts at the matching ordinals for a record.
func (m *Model) implicitRelationshipsOf(owner *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	if owner.Recorded() {
		for ordinal, rel := range owner.Facts.Relationships {
			if implicitRelationshipKind(rel.Kind) && !rel.Echo {
				out = append(out, implicitRelationshipSymbol(owner, ordinal, rel.Kind, rel.Conjugated, nil))
			}
		}
		return out
	}
	ordinal := 0
	rels := RelationshipsOf(owner)
	for i, rel := range rels {
		if rel == nil {
			continue
		}
		if implicitRelationshipKind(rel.Kind) && !IncludeUseCaseEcho(rels, i) {
			out = append(out, implicitRelationshipSymbol(owner, ordinal, rel.Kind, rel.Conjugated, rel))
		}
		ordinal++
	}
	// A connector end reference-subsets the feature it attaches to whether or
	// not a clause of its own says so (SysML v2 8.2.2.13.1).
	if end, ok := owner.Decl.(*ast.ConnectorEnd); ok {
		if target := ImpliedEndReference(end); target != nil {
			out = append(out, implicitRelationshipSymbol(owner, ordinal, ast.RelReferences, false, connectorEndAttachment(target)))
		}
	}
	// A multiplicity member's `subsets` is its one written relationship, held on
	// the declaration rather than in a Relationships list.
	if decl, ok := owner.Decl.(*ast.MultiplicityDecl); ok && decl.Subsets != nil {
		out = append(out, implicitRelationshipSymbol(owner, ordinal, ast.RelSubsets, false, nil))
	}
	return out
}

// implicitRelationshipSymbol is the synthetic symbol a written relationship
// reflects as: unnamed, owned by nothing but the element it is written on, and
// spanning that edge where the tree carries it.
func implicitRelationshipSymbol(owner *symbols.Symbol, ordinal int, kind ast.RelationshipKind, conjugated bool, node *ast.Relationship) *symbols.Symbol {
	span := owner.DeclSpan
	if node != nil {
		span = node.Span()
	}
	return &symbols.Symbol{
		Kind:     symbols.SymbolRelationship,
		DocName:  owner.DocName,
		DeclSpan: span,
		Implicit: &symbols.ImplicitRelationship{
			Owner: owner, Ordinal: ordinal, Kind: kind, Conjugated: conjugated, Node: node,
		},
	}
}

// IncludeUseCaseEcho reports whether rels[i] is the `includes` the parser
// echoes for `include use case uc : UC`: a RelIncludes immediately followed by
// the RelTyping whose target it repeats node-for-node. The full form carries
// no ReferenceSubsetting (SysML.xtext IncludeUseCaseUsage), so the echo is no
// relationship object — but it still holds its ordinal.
func IncludeUseCaseEcho(rels []*ast.Relationship, i int) bool {
	if i < 0 || i >= len(rels) || rels[i] == nil || rels[i].Kind != ast.RelIncludes {
		return false
	}
	for j := i + 1; j < len(rels); j++ {
		if rels[j] == nil {
			continue
		}
		return rels[j].Kind == ast.RelTyping && rels[j].Target == rels[i].Target
	}
	return false
}

// implicitRelationshipKind reports whether a written relationship kind belongs
// to the specialization family the reflective layer synthesizes objects for.
// Other written relationships keep their ordinal but get no object.
func implicitRelationshipKind(k ast.RelationshipKind) bool {
	switch k {
	case ast.RelSpecializes, ast.RelTyping, ast.RelSubsets, ast.RelRedefines,
		ast.RelReferences, ast.RelIncludes, ast.RelCrosses:
		return true
	}
	return false
}

// implicitMetaclassName is the reflective metaclass name of the relationship
// object sym reflects: the most specific metaclass the notation's kind and the
// owner's classification select (KerML 8.3.3, SysML 8.3.12).
func (m *Model) implicitMetaclassName(sym *symbols.Symbol) string {
	rel := sym.Implicit
	switch rel.Kind {
	case ast.RelSpecializes:
		if rel.Conjugated {
			return "Conjugation"
		}
		if m.reflectiveMetaclassConforms(rel.Owner, "Classifier") {
			return "Subclassification"
		}
		if m.reflectiveMetaclassConforms(rel.Owner, "Feature") {
			return "Subsetting"
		}
		return "Specialization"
	case ast.RelTyping:
		if rel.Conjugated {
			return "ConjugatedPortTyping"
		}
		return "FeatureTyping"
	case ast.RelSubsets:
		return "Subsetting"
	case ast.RelRedefines:
		return "Redefinition"
	case ast.RelReferences, ast.RelIncludes:
		return "ReferenceSubsetting"
	case ast.RelCrosses:
		return "CrossSubsetting"
	}
	return ""
}

// implicitRelationshipMetaclass is the library metaclass element classifying
// the relationship object sym reflects, or nil where none is known.
func (m *Model) implicitRelationshipMetaclass(sym *symbols.Symbol) *symbols.Symbol {
	return m.Metaclass(m.implicitMetaclassName(sym))
}

// specializationMetaclassNames are the metaclasses that conform to KerML
// Specialization, so `specific`/`general` and Type::ownedSpecialization reach
// them all.
var specializationMetaclassNames = map[string]bool{
	"Specialization": true, "Subclassification": true,
	"FeatureTyping": true, "ConjugatedPortTyping": true,
	"Subsetting": true, "Redefinition": true,
	"ReferenceSubsetting": true, "CrossSubsetting": true,
}

// subsettingMetaclassNames are the metaclasses that conform to KerML Subsetting.
var subsettingMetaclassNames = map[string]bool{
	"Subsetting": true, "Redefinition": true,
	"ReferenceSubsetting": true, "CrossSubsetting": true,
}

// featureTypingMetaclassNames are the metaclasses that conform to KerML
// FeatureTyping.
var featureTypingMetaclassNames = map[string]bool{
	"FeatureTyping": true, "ConjugatedPortTyping": true,
}

// relationshipEndFeature reports whether feature is the source- or target-side
// end the relationship metaclass meta declares. ConjugatedPortTyping answers
// only its `portDefinition`; its general end stays unimplemented.
func relationshipEndFeature(meta, feature string) (sourceSide, targetSide bool) {
	switch feature {
	case "subclassifier":
		sourceSide = meta == "Subclassification"
	case "typedFeature":
		sourceSide = featureTypingMetaclassNames[meta]
	case "subsettingFeature":
		sourceSide = subsettingMetaclassNames[meta]
	case "redefiningFeature":
		sourceSide = meta == "Redefinition"
	case "referencingFeature":
		sourceSide = meta == "ReferenceSubsetting"
	case "crossingFeature":
		sourceSide = meta == "CrossSubsetting"
	case "conjugatedType":
		sourceSide = meta == "Conjugation"
	case "superclassifier":
		targetSide = meta == "Subclassification"
	case "type":
		targetSide = meta == "FeatureTyping"
	case "portDefinition":
		targetSide = meta == "ConjugatedPortTyping"
	case "subsettedFeature":
		targetSide = subsettingMetaclassNames[meta]
	case "redefinedFeature":
		targetSide = meta == "Redefinition"
	case "referencedFeature":
		targetSide = meta == "ReferenceSubsetting"
	case "crossedFeature":
		targetSide = meta == "CrossSubsetting"
	case "originalType":
		targetSide = meta == "Conjugation"
	}
	return sourceSide, targetSide
}

// implicitRelationshipEnds resolves the ends of the relationship object sym
// reflects: its owning element as source, and the target named or, for a
// chain, denoted (chainTargetFeature); resolution is reported separately
// since an unresolved target leaves every target-side feature underived.
func (m *Model) implicitRelationshipEnds(sym *symbols.Symbol) (src, tgt *symbols.Symbol, tgtOK bool) {
	rel := sym.Implicit
	src = rel.Owner
	if rel.Node != nil {
		// A chain target's general is the implicit chaining feature the chain
		// denotes as a whole (KerML 8.3.3.3.9), never the chain's final feature.
		if ast.IsFeatureChain(rel.Node.Target) {
			tgt = m.chainTargetFeature(sym)
			return src, tgt, tgt != nil
		}
		if _, isEnd := rel.Owner.Decl.(*ast.ConnectorEnd); isEnd && rel.Kind.ReferenceSubsets() {
			tgt = m.ReferencedFeature(rel.Owner)
			return src, tgt, tgt != nil
		}
		tgt = m.RelationshipTarget(rel.Owner, rel.Node)
		return src, tgt, tgt != nil
	}
	if rel.Owner.Recorded() {
		if rel.Ordinal < len(rel.Owner.Facts.Relationships) {
			facts := rel.Owner.Facts.Relationships[rel.Ordinal]
			if facts.Chain {
				tgt = m.chainTargetFeature(sym)
				return src, tgt, tgt != nil
			}
			if facts.Target.IsZero() {
				return src, nil, false
			}
			tgt = m.recordedElement(facts.Target)
			return src, tgt, tgt != nil
		}
		return src, nil, false
	}
	// The object with no written edge is a multiplicity member's `subsets`.
	tgt = m.SubsettedMultiplicity(rel.Owner)
	return src, tgt, tgt != nil
}

// relationshipMemberEnds resolves the ends of a keyword-first relationship
// member, from the tree or from its recorded facts.
func (m *Model) relationshipMemberEnds(sym *symbols.Symbol) (src, tgt *symbols.Symbol) {
	if sym.Recorded() {
		if sym.Facts.Relationship == nil {
			return nil, nil
		}
		return m.recordedElement(sym.Facts.Relationship.Source),
			m.recordedElement(sym.Facts.Relationship.Target)
	}
	member, ok := sym.Decl.(*ast.RelationshipMember)
	if !ok || m.resolver == nil {
		return nil, nil
	}
	scope := sym.OwnerScope
	if scope == nil {
		scope = sym.Scope
	}
	if resolved, ok := m.resolver.ResolveTarget(scope, member.Source); ok {
		src = resolved
	}
	// A chain target names an implicit chaining feature the model has no
	// element for, so the end stays unresolved and its features stay underived.
	if !ast.IsFeatureChain(member.Target) {
		if resolved, ok := m.resolver.ResolveTarget(scope, member.Target); ok {
			tgt = resolved
		}
	}
	return src, tgt
}

// implicitRelationshipElements reads an element-valued feature of a reflected
// relationship object: the ends its metaclass declares, and the Element
// features a written edge answers. A feature the object's metaclass does not
// own, or one whose target side does not resolve, is underived.
func (m *Model) implicitRelationshipElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	meta := m.implicitMetaclassName(sym)
	src, tgt, tgtOK := m.implicitRelationshipEnds(sym)
	source := []*symbols.Symbol{src}
	empty := []*symbols.Symbol{}
	switch feature {
	case "source":
		return source, true
	case "target":
		if tgtOK && meta != "ConjugatedPortTyping" {
			return []*symbols.Symbol{tgt}, true
		}
		return nil, false
	case "relatedElement":
		if tgtOK && meta != "ConjugatedPortTyping" {
			return []*symbols.Symbol{src, tgt}, true
		}
		return nil, false
	case "owningRelatedElement", "owner":
		return source, true
	case "ownedRelatedElement":
		// The chaining feature a chain target denotes is owned through the
		// relationship that targets it (KerML 8.3.3.3.9).
		if chain := m.chainTargetFeature(sym); chain != nil {
			return []*symbols.Symbol{chain}, true
		}
		return empty, true
	case "ownedElement", "ownedRelationship",
		"owningRelationship", "owningMembership", "owningNamespace", "documentation":
		return empty, true
	}
	// Per-metaclass ends: the source end answers the owning element, the
	// target end what the edge resolves to.
	srcEnd, tgtEnd := relationshipEndFeature(meta, feature)
	switch {
	case feature == "specific" && specializationMetaclassNames[meta]:
		return source, true
	case feature == "general" && specializationMetaclassNames[meta] && meta != "ConjugatedPortTyping":
		if tgtOK {
			return []*symbols.Symbol{tgt}, true
		}
		return nil, false
	case srcEnd:
		return source, true
	case tgtEnd:
		if tgtOK {
			return []*symbols.Symbol{tgt}, true
		}
		return nil, false
	case feature == "owningType" && (specializationMetaclassNames[meta] || meta == "Conjugation"):
		if m.reflectiveMetaclassConforms(src, "Type") {
			return source, true
		}
		return empty, true
	case feature == "owningClassifier" && meta == "Subclassification":
		if m.reflectiveMetaclassConforms(src, "Classifier") {
			return source, true
		}
		return empty, true
	case feature == "owningFeature" && (featureTypingMetaclassNames[meta] || subsettingMetaclassNames[meta]):
		if m.reflectiveMetaclassConforms(src, "Feature") {
			return source, true
		}
		return empty, true
	}
	return nil, false
}

// relationshipMemberElements reads an element-valued feature of a keyword-first
// relationship member: the ends its metaclass declares. derived is false for a
// feature the member's metaclass does not own, leaving the ordinary Element
// derivations to answer instead.
func (m *Model) relationshipMemberElements(sym *symbols.Symbol, rel symbols.RelationshipDecl, feature string) (elems []*symbols.Symbol, derived bool) {
	meta := relationshipMetaclassName(rel)
	if !specializationMetaclassNames[meta] && meta != "Conjugation" {
		return nil, false
	}
	src, tgt := m.relationshipMemberEnds(sym)
	source := []*symbols.Symbol{}
	if src != nil {
		source = []*symbols.Symbol{src}
	}
	target := []*symbols.Symbol{}
	if tgt != nil {
		target = []*symbols.Symbol{tgt}
	}
	switch feature {
	case "source":
		if src == nil {
			return nil, false
		}
		return source, true
	case "target":
		if tgt == nil {
			return nil, false
		}
		return target, true
	case "relatedElement":
		if src == nil || tgt == nil {
			return nil, false
		}
		return append(append([]*symbols.Symbol{}, source...), target...), true
	case "owningRelatedElement", "ownedRelatedElement",
		"owningType", "owningClassifier", "owningFeature":
		return []*symbols.Symbol{}, true
	}
	srcEnd, tgtEnd := relationshipEndFeature(meta, feature)
	switch {
	case feature == "specific" && specializationMetaclassNames[meta]:
		if src == nil {
			return nil, false
		}
		return source, true
	case feature == "general" && specializationMetaclassNames[meta]:
		if tgt == nil {
			return nil, false
		}
		return target, true
	case srcEnd:
		if src == nil {
			return nil, false
		}
		return source, true
	case tgtEnd:
		if tgt == nil {
			return nil, false
		}
		return target, true
	}
	return nil, false
}

// implicitRelationshipValue is what a reflected relationship object states for
// a non-element metaclass feature of it; underived for any feature its
// metaclass does not own.
func (m *Model) implicitRelationshipValue(sym *symbols.Symbol, feature string) (symbols.FilterValue, bool) {
	switch feature {
	case "name", "declaredName", "shortName", "declaredShortName", "qualifiedName":
		return emptyValue(), true
	case "isImplied":
		return boolValue(false), true
	case "isLibraryElement":
		return m.reflectiveFeatureValue(sym.Implicit.Owner, feature)
	}
	return symbols.FilterValue{}, false
}

// implicitRelationshipValues is ReflectiveFeatureValues for a reflected
// relationship object.
func (m *Model) implicitRelationshipValues(sym *symbols.Symbol, feature string) ([]symbols.FilterValue, bool) {
	if feature == "documentation" {
		return []symbols.FilterValue{}, true
	}
	if elements, ok := m.implicitRelationshipElements(sym, feature); ok {
		values := make([]symbols.FilterValue, 0, len(elements))
		for _, element := range elements {
			fqn := m.fqnOf(element)
			if fqn == "" {
				// An unnamed end (an anonymous feature) cannot be read back,
				// so a shortened list is no answer at all.
				return nil, false
			}
			values = append(values, symbols.FilterValue{Kind: symbols.FilterValueRef, RefFQN: fqn})
		}
		return values, true
	}
	value, ok := m.implicitRelationshipValue(sym, feature)
	if !ok {
		return nil, false
	}
	if value.Kind == symbols.FilterValueEmpty {
		return nil, true
	}
	return []symbols.FilterValue{value}, true
}

// ownedImplicitRelationships are sym's reflected relationship objects whose
// reflected metaclass is one of kinds, in textual order.
func (m *Model) ownedImplicitRelationships(sym *symbols.Symbol, kinds map[string]bool) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range m.ImplicitRelationships(sym) {
		if kinds[m.implicitMetaclassName(rel)] {
			out = append(out, rel)
		}
	}
	return out
}

var (
	conjugationMetaclass   = map[string]bool{"Conjugation": true}
	redefinitionMetaclass  = map[string]bool{"Redefinition": true}
	referenceSubsetting    = map[string]bool{"ReferenceSubsetting": true}
	crossSubsetting        = map[string]bool{"CrossSubsetting": true}
	subclassificationKinds = map[string]bool{"Subclassification": true}
)

// implicitOwnerElements reads an owner-side element feature: the relationship
// objects a type or feature owns by writing their notation on itself.
func (m *Model) implicitOwnerElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	switch feature {
	case "ownedSpecialization":
		if !m.reflectiveMetaclassConforms(sym, "Type") {
			return nil, false
		}
		return m.ownedImplicitRelationships(sym, specializationMetaclassNames), true
	case "ownedConjugator":
		if !m.reflectiveMetaclassConforms(sym, "Type") {
			return nil, false
		}
		return m.ownedImplicitRelationships(sym, conjugationMetaclass), true
	case "ownedSubclassification":
		if !m.reflectiveMetaclassConforms(sym, "Classifier") {
			return nil, false
		}
		return m.ownedImplicitRelationships(sym, subclassificationKinds), true
	case "ownedTyping":
		if !m.reflectiveMetaclassConforms(sym, "Feature") {
			return nil, false
		}
		return m.ownedImplicitRelationships(sym, featureTypingMetaclassNames), true
	case "ownedSubsetting":
		if !m.reflectiveMetaclassConforms(sym, "Feature") {
			return nil, false
		}
		return m.ownedImplicitRelationships(sym, subsettingMetaclassNames), true
	case "ownedRedefinition":
		if !m.reflectiveMetaclassConforms(sym, "Feature") {
			return nil, false
		}
		return m.ownedImplicitRelationships(sym, redefinitionMetaclass), true
	case "ownedReferenceSubsetting":
		if !m.reflectiveMetaclassConforms(sym, "Feature") {
			return nil, false
		}
		return m.ownedImplicitRelationships(sym, referenceSubsetting), true
	case "ownedCrossSubsetting":
		if !m.reflectiveMetaclassConforms(sym, "Feature") {
			return nil, false
		}
		return m.ownedImplicitRelationships(sym, crossSubsetting), true
	}
	return nil, false
}
