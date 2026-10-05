package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ownedRelationship is a relationship a declaration owns — `: T`, `:> s`,
// `:>> x`, `::> a.b`, or the attachment of a connector end — as its reflective
// metaobject reads it (KerML 8.3.4.8.15). A symbol stands for the relationship
// so a metaobject can denote it: the owning declaration is its source and the
// element it names its target.
type ownedRelationship struct {
	owner      *symbols.Symbol
	kind       ast.RelationshipKind
	conjugated bool
	// rel is the clause the owner's declaration writes, or the clause an end's
	// attachment implies; nil for a recorded owner.
	rel *ast.Relationship
	// ref is the target a recorded owner's record carries.
	ref symbols.ElementRef
	// chain is the feature a chain target (`s.y`) denotes as a whole, which the
	// relationship owns and targets; nil when the target is a name.
	chain *symbols.Symbol
}

// ownedRelationshipProperty is a reflective property holding the relationships
// an element owns: the metaclass deriving it and the one its values conform to.
type ownedRelationshipProperty struct {
	owner, relationship string
}

// ownedRelationshipProperties are the properties of Type and its subclasses that
// hold the specializations and featurings an element's declaration owns.
var ownedRelationshipProperties = map[string]ownedRelationshipProperty{
	"ownedSpecialization":      {"Type", "Specialization"},
	"ownedSubclassification":   {"Classifier", "Subclassification"},
	"ownedTyping":              {"Feature", "FeatureTyping"},
	"ownedSubsetting":          {"Feature", "Subsetting"},
	"ownedRedefinition":        {"Feature", "Redefinition"},
	"ownedReferenceSubsetting": {"Feature", "ReferenceSubsetting"},
	"ownedCrossSubsetting":     {"Feature", "CrossSubsetting"},
	"ownedFeatureInverting":    {"Feature", "FeatureInverting"},
	"ownedTypeFeaturing":       {"Feature", "TypeFeaturing"},
	"ownedDisjoining":          {"Type", "Disjoining"},
}

// OwnedRelationshipSymbols returns the symbols standing for the relationships
// sym's declaration owns, in declaration order, with the reference subsetting a
// connector end's attachment implies first. Memoized.
func (m *Model) OwnedRelationshipSymbols(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || m.resolver == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.ownedRelationships[sym]; ok {
		return cached
	}
	ownerScope := sym.Scope
	if ownerScope == nil {
		// A recorded leaf has no scope of its own; its relationships still
		// need one to be owned through.
		ownerScope = symbols.NewScope(sym.OwnerScope, nil)
		ownerScope.SetOwner(sym)
	}
	var out []*symbols.Symbol
	for _, info := range m.ownedRelationshipsOf(sym) {
		if rel := m.newOwnedRelationship(info, ownerScope); rel != nil {
			out = append(out, rel)
		}
	}
	journal(m, m.ownedRelationships, sym, sym.Decl)
	m.ownedRelationships[sym] = out
	return out
}

// ownedRelationshipsOf lists the relationships sym's declaration or record owns.
func (m *Model) ownedRelationshipsOf(sym *symbols.Symbol) []ownedRelationship {
	var out []ownedRelationship
	if sym.Recorded() {
		if sym.Facts.Node == symbols.NodeConnectorEnd && !sym.Facts.References.IsZero() &&
			!recordsReference(sym.Facts.Relationships, sym.Facts.References) {
			out = append(out, ownedRelationship{owner: sym, kind: ast.RelReferences, ref: sym.Facts.References})
		}
		for _, rel := range sym.Facts.Relationships {
			out = append(out, ownedRelationship{owner: sym, kind: rel.Kind, conjugated: rel.Conjugated, ref: rel.Target})
		}
		return out
	}
	// A connector end reference-subsets the feature it attaches to (SysML v2
	// 8.2.2.13.1), whether or not a clause of its own says so.
	if end, ok := sym.Decl.(*ast.ConnectorEnd); ok {
		if target := end.AttachedTarget(); target != nil && !referencesTarget(end.Relationships, target) {
			rel := &ast.Relationship{Kind: ast.RelReferences, Target: target}
			rel.NodeSpan = target.Span()
			out = append(out, ownedRelationship{owner: sym, kind: ast.RelReferences, rel: rel})
		}
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel != nil {
			out = append(out, ownedRelationship{owner: sym, kind: rel.Kind, conjugated: rel.Conjugated, rel: rel})
		}
	}
	return out
}

// recordsReference reports whether a reference subsetting among rels targets ref.
func recordsReference(rels []symbols.RelationshipFacts, ref symbols.ElementRef) bool {
	for _, rel := range rels {
		if rel.Kind.ReferenceSubsets() && rel.Target.FQN == ref.FQN && rel.Target.Doc == ref.Doc &&
			slices.Equal(rel.Target.Path, ref.Path) {
			return true
		}
	}
	return false
}

// referencesTarget reports whether a reference subsetting among rels names target.
func referencesTarget(rels []*ast.Relationship, target ast.Node) bool {
	for _, rel := range rels {
		if rel != nil && rel.Kind == ast.RelReferences && rel.Target == target {
			return true
		}
	}
	return false
}

// newOwnedRelationship is the symbol standing for info, owned through
// ownerScope, or nil for a relationship no library metaclass classifies.
func (m *Model) newOwnedRelationship(info ownedRelationship, ownerScope *symbols.Scope) *symbols.Symbol {
	if m.ownedRelationshipMetaclass(info) == nil {
		return nil
	}
	decl := info.rel
	if decl == nil {
		decl = &ast.Relationship{Kind: info.kind, Conjugated: info.conjugated}
	}
	rel := &symbols.Symbol{
		Kind:       symbols.SymbolRelationship,
		Decl:       decl,
		DeclSpan:   decl.Span(),
		Visibility: ast.VisibilityDefault,
		OwnerScope: ownerScope,
	}
	if chain := chainTargetNode(info.rel); chain != nil {
		scope := symbols.NewScope(ownerScope, decl)
		scope.SetOwner(rel)
		rel.Scope = scope
		info.chain = &symbols.Symbol{
			Kind:       symbols.SymbolReferenceUsage,
			Decl:       chain,
			DeclSpan:   chain.Span(),
			Visibility: ast.VisibilityDefault,
			OwnerScope: scope,
		}
		scope.DefineAnonymous(info.chain)
	}
	journal(m, m.relationshipInfo, rel, info.owner.Decl)
	m.relationshipInfo[rel] = info
	return rel
}

// chainTargetNode is the feature chain a relationship targets (`:>> a.b`), or
// nil when it targets a name.
func chainTargetNode(rel *ast.Relationship) *ast.FeatureChainExpr {
	if rel == nil {
		return nil
	}
	node := rel.Target
	if fr, ok := node.(*ast.FeatureReference); ok {
		node = fr.Name
	}
	chain, _ := node.(*ast.FeatureChainExpr)
	return chain
}

// ownedRelationshipMetaclass is the KerML metaclass classifying info, by its
// kind and the kind of element owning it (KerML 7.3.3, 7.3.4).
func (m *Model) ownedRelationshipMetaclass(info ownedRelationship) *symbols.Symbol {
	switch {
	case info.kind == ast.RelTyping && info.conjugated:
		if meta := m.sysmlMetaclass("ConjugatedPortTyping"); meta != nil {
			return meta
		}
		return m.kermlMetaclass("FeatureTyping")
	case info.kind == ast.RelSpecializes:
		if m.reflectiveMetaclassConforms(info.owner, "Feature") {
			return m.kermlMetaclass("Subsetting")
		}
		if m.reflectiveMetaclassConforms(info.owner, "Classifier") {
			return m.kermlMetaclass("Subclassification")
		}
		return m.kermlMetaclass("Specialization")
	case info.kind.ReferenceSubsets():
		return m.kermlMetaclass("ReferenceSubsetting")
	case info.kind == ast.RelCrosses:
		return m.kermlMetaclass("CrossSubsetting")
	}
	return m.kermlMetaclass(relationshipMetaclassNames[info.kind])
}

// ownedRelationshipTarget is Relationship::target of the relationship rel
// stands for: the feature its chain target denotes as a whole, else the element
// its target names.
func (m *Model) ownedRelationshipTarget(rel *symbols.Symbol) *symbols.Symbol {
	info, ok := m.relationshipInfo[rel]
	if !ok {
		return nil
	}
	if info.chain != nil {
		return info.chain
	}
	if info.rel == nil || info.rel.Target == nil {
		if info.ref.IsZero() {
			return nil
		}
		return m.recordedElement(info.ref)
	}
	if _, isEnd := info.owner.Decl.(*ast.ConnectorEnd); isEnd && info.kind.ReferenceSubsets() {
		return m.ReferencedFeature(info.owner)
	}
	return m.relationshipTarget(info.owner, info.rel)
}

// ownedRelationshipElements is a Relationship property of the relationship rel
// stands for; ok is false for any other symbol.
func (m *Model) ownedRelationshipElements(rel *symbols.Symbol, property string) ([]*symbols.Symbol, bool) {
	info, ok := m.relationshipInfo[rel]
	if !ok {
		return nil, false
	}
	var target []*symbols.Symbol
	if t := m.ownedRelationshipTarget(rel); t != nil {
		target = []*symbols.Symbol{t}
	}
	switch property {
	case "source", "owningRelatedElement":
		return []*symbols.Symbol{info.owner}, true
	case "target":
		return target, true
	case "relatedElement":
		return append([]*symbols.Symbol{info.owner}, target...), true
	case "ownedRelatedElement":
		if info.chain == nil {
			return nil, true
		}
		return []*symbols.Symbol{info.chain}, true
	case "owningType":
		if !m.reflectiveMetaclassConforms(info.owner, "Type") {
			return nil, true
		}
		return []*symbols.Symbol{info.owner}, true
	}
	return nil, false
}

// ownedRelationshipsConforming is the relationships sym owns whose metaclass
// conforms to metaclass.
func (m *Model) ownedRelationshipsConforming(sym *symbols.Symbol, metaclass string) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range m.OwnedRelationshipSymbols(sym) {
		if m.reflectiveMetaclassConforms(rel, metaclass) {
			out = append(out, rel)
		}
	}
	return out
}

// chainingFeaturesOf is Feature::chainingFeature of sym: for the feature a
// relationship's chain target (`s.y`) denotes as a whole, the features the
// chain is written as, in order; empty for any other feature.
func (m *Model) chainingFeaturesOf(sym *symbols.Symbol) []*symbols.Symbol {
	chain, ok := sym.Decl.(*ast.FeatureChainExpr)
	if !ok || sym.OwnerScope == nil {
		return nil
	}
	info, ok := m.relationshipInfo[sym.OwnerScope.Owner()]
	if !ok {
		return nil
	}
	scope := info.owner.OwnerScope
	if _, isEnd := info.owner.Decl.(*ast.ConnectorEnd); isEnd && info.kind.ReferenceSubsets() {
		scope = referenceScope(info.owner)
	}
	return m.attachmentPath(scope, chain)
}
