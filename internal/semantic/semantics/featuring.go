package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const occurrenceSnapshotsFQN = "Occurrences::Occurrence::snapshots"

// FeaturingType is one derived Feature::featuringType. The zero value is Base::Anything; Snapshots
// names the implied feature of Type's snapshots that features a variable feature (KerML 1.1 §8.3.3.1.6).
type FeaturingType struct {
	Type      *symbols.Symbol
	Snapshots bool
}

// IsAnything reports whether t is Base::Anything, the featuring type of a feature that has none.
func (t FeaturingType) IsAnything() bool {
	return t.Type == nil
}

// DerivedFeaturingTypes derives Feature::featuringType (KerML 1.1 §8.3.3.3.4 deriveFeatureFeaturingType):
// the declared `featured by` types, the type its FeatureMembership implies, and its first chaining feature's.
func (m *Model) DerivedFeaturingTypes(feature *symbols.Symbol) []FeaturingType {
	if m == nil || feature == nil {
		return nil
	}
	defer m.own(feature).LeaveDoc()
	if cached, ok := m.derivedFeaturing[feature]; ok {
		return cached
	}
	// A connector's default reads its related features' featuring types, which may lead back here.
	if depth := m.computingDerivedFeaturing[feature]; depth != 0 {
		m.resolver.CutShort(depth)
		return nil
	}
	m.computingDerivedFeaturing[feature] = m.resolver.Enter()
	defer delete(m.computingDerivedFeaturing, feature)
	out := m.derivedFeaturingTypes(feature, map[*symbols.Symbol]bool{})
	// A derivation a resolution guard cut short is recomputed, not memoized.
	if !m.resolver.Leave() {
		return out
	}
	journal(m, m.derivedFeaturing, feature, feature.Decl)
	m.derivedFeaturing[feature] = out
	return out
}

func (m *Model) derivedFeaturingTypes(feature *symbols.Symbol, deriving map[*symbols.Symbol]bool) []FeaturingType {
	if feature == nil || !(feature.IsFeature() || feature.Expr != nil) || deriving[feature] {
		return nil
	}
	deriving[feature] = true
	defer delete(deriving, feature)
	var out []FeaturingType
	add := func(t FeaturingType) {
		for _, have := range out {
			if sameFeaturingType(have, t) {
				return
			}
		}
		out = append(out, t)
	}
	for _, declared := range m.FeaturingTypes(feature) {
		add(m.plainFeaturingType(declared))
	}
	for _, implied := range m.impliedTypeFeaturings(feature, deriving) {
		add(implied)
	}
	if chaining := m.ChainingFeatures(feature); len(chaining) > 0 {
		for _, t := range m.derivedFeaturingTypes(chaining[0], deriving) {
			add(t)
		}
	}
	return out
}

// impliedTypeFeaturings derives ImpliedTypeFeaturings beside the features being derived.
func (m *Model) impliedTypeFeaturings(feature *symbols.Symbol, deriving map[*symbols.Symbol]bool) []FeaturingType {
	owner := m.ownerOf(feature)
	if feature.Expr != nil {
		owner = feature.Expr.Owner
	}
	if owner == nil {
		// The root namespace owns no feature through a FeatureMembership; a connector there has its default.
		return m.impliedConnectorFeaturing(feature)
	}
	switch {
	case m.owningEndOf(feature) != nil:
		// KerML checkFeatureOwnedCrossFeatureTypeFeaturing: the types of the one other end; the
		// Cartesian product of several other ends' types is not a type the model holds.
		var out []FeaturingType
		if others := m.otherEnds(owner); len(others) == 1 {
			for _, t := range m.FeatureTypes(others[0]) {
				if featuring := m.plainFeaturingType(t); featuring.Type != nil {
					out = append(out, featuring)
				}
			}
		}
		return out
	case isExpressionParameterSymbol(feature):
		// KerML checkFeatureFeatureMembershipTypeFeaturing: an expression owns its parameters.
		return []FeaturingType{m.plainFeaturingType(owner)}
	case feature.Kind == symbols.SymbolMultiplicity:
		// KerML checkMultiplicityTypeFeaturing: a feature's multiplicity shares its featuring types.
		return m.derivedFeaturingTypes(owner, deriving)
	case feature.Expr != nil && feature.Expr.Role == symbols.ExpressionBound:
		// KerML checkMultiplicityRangeExpressionTypeFeaturing: a bound shares its range's, and
		// the bound of an owned cross feature's range its owning end's.
		bounded := feature.Expr.Owner
		if end := m.owningEndOf(m.ownerOf(bounded)); end != nil {
			return m.derivedFeaturingTypes(end, deriving)
		}
		return m.impliedTypeFeaturings(bounded, deriving)
	case feature.Expr != nil && feature.Expr.Role == symbols.ExpressionValue:
		// KerML checkExpressionTypeFeaturing: a value shares its featureWithValue's.
		return m.derivedFeaturingTypes(owner, deriving)
	case IsVariant(feature) && IsVariation(owner):
		// SysML checkUsageVariationUsageTypeFeaturing: a variant shares its variation usage's.
		if !m.reflectiveMetaclassConforms(owner, "Usage") {
			return nil
		}
		return m.derivedFeaturingTypes(owner, deriving)
	case m.OwningType(feature) == nil && m.reflectiveMetaclassConforms(feature, "Connector"):
		return m.impliedConnectorFeaturing(feature)
	case m.ownedByFeatureMembership(owner, feature):
		// KerML checkFeatureFeatureMembershipTypeFeaturing.
		return []FeaturingType{m.impliedFeaturingType(owner, m.FeatureIsVariable(feature))}
	}
	return nil
}

// impliedConnectorFeaturing returns the default featuring type of a connector no type owns and
// no clause features (KerML checkConnectorTypeFeaturing).
func (m *Model) impliedConnectorFeaturing(feature *symbols.Symbol) []FeaturingType {
	if m.OwningType(feature) != nil || len(m.FeaturingTypes(feature)) != 0 || !m.reflectiveMetaclassConforms(feature, "Connector") {
		return nil
	}
	if featuring := m.connectorDefaultFeaturingType(feature); featuring.Type != nil {
		return []FeaturingType{featuring}
	}
	return nil
}

// OwningType derives Feature::owningType: the type owning feature through a FeatureMembership.
func (m *Model) OwningType(feature *symbols.Symbol) *symbols.Symbol {
	if m == nil || feature == nil || !feature.IsFeature() {
		return nil
	}
	if owner := m.ownerOf(feature); owner != nil && m.ownedByFeatureMembership(owner, feature) {
		return owner
	}
	return nil
}

// ownedByFeatureMembership reports whether owner is a type owning feature through a
// FeatureMembership, which a `member` declaration (KerML.xtext TypeFeatureMember) is not.
func (m *Model) ownedByFeatureMembership(owner, feature *symbols.Symbol) bool {
	base := m.Metaclass("FeatureMembership")
	if base == nil || m.MetaclassOf(owner) == nil {
		// Where no metaclass classifies the owner, any owner but a package is a type.
		if owner.Kind == symbols.SymbolPackage || owner.Kind == symbols.SymbolNamespace {
			return false
		}
	} else if meta := m.Metaclass(m.membershipMetaclassName(owner, feature)); meta == nil || !m.Conforms(meta, base) {
		return false
	}
	return !m.IsTypeFeatureMember(feature)
}

// IsTypeFeatureMember reports whether feature is declared `member` (KerML.xtext TypeFeatureMember),
// by its owner's declaration or by the ModTypeFeatureMember fact a recorded symbol carries instead.
func (m *Model) IsTypeFeatureMember(feature *symbols.Symbol) bool {
	if m == nil || feature == nil {
		return false
	}
	if feature.Recorded() {
		return feature.Facts.Modifiers.Has(symbols.ModTypeFeatureMember)
	}
	owner := m.ownerOf(feature)
	if owner == nil || feature.Decl == nil {
		return false
	}
	for _, member := range declMembers(owner) {
		if wrapper, ok := member.(*ast.Membership); ok && wrapper.IsTypeFeature && wrapper.Member == feature.Decl {
			return true
		}
	}
	return false
}

func (m *Model) plainFeaturingType(t *symbols.Symbol) FeaturingType {
	if t == nil || m.isTopType(t) {
		return FeaturingType{}
	}
	return FeaturingType{Type: t}
}

// impliedFeaturingType is the featuring type a FeatureMembership in owner implies: owner, or the
// feature of owner's snapshots for a variable feature (KerML 1.1 §8.3.3.3.4 Feature::isFeaturingType).
func (m *Model) impliedFeaturingType(owner *symbols.Symbol, variable bool) FeaturingType {
	if !variable {
		return m.plainFeaturingType(owner)
	}
	if snapshots := m.occurrenceSnapshots(); snapshots != nil && symbols.SameElement(m.ownerOf(snapshots), owner) {
		return FeaturingType{Type: snapshots}
	}
	return FeaturingType{Type: owner, Snapshots: true}
}

func (m *Model) occurrenceSnapshots() *symbols.Symbol {
	return m.typeByFQN(occurrenceSnapshotsFQN)
}

func sameFeaturingType(a, b FeaturingType) bool {
	if a.Type == nil || b.Type == nil {
		return a.Type == b.Type
	}
	return a.Snapshots == b.Snapshots && symbols.SameElement(a.Type, b.Type)
}

// featureAccess evaluates the mutually recursive Feature::canAccess, isFeaturedWithin and
// isCompatibleWith; a canAccess question re-entered while it is being answered answers false.
type featureAccess struct {
	m         *Model
	accessing map[[2]FeaturingType]bool
}

func (m *Model) featureAccess() *featureAccess {
	return &featureAccess{m: m, accessing: map[[2]FeaturingType]bool{}}
}

// CanAccess derives Feature::canAccess (KerML 1.1 §8.3.3.3.4): target is featured within one of
// feature's direct or indirect featuring types. A feature with no featuring type is featured by Anything.
func (m *Model) CanAccess(feature, target *symbols.Symbol) bool {
	if m == nil || feature == nil || target == nil {
		return false
	}
	return m.featureAccess().canAccess(FeaturingType{Type: feature}, FeaturingType{Type: target})
}

// featuringTypesOf is the featuringType of f read as a feature, the implied snapshots feature included.
func (a *featureAccess) featuringTypesOf(f FeaturingType) []FeaturingType {
	if f.Snapshots {
		return []FeaturingType{a.m.plainFeaturingType(f.Type)}
	}
	return a.m.DerivedFeaturingTypes(f.Type)
}

func isFeatureType(t FeaturingType) bool {
	return t.Snapshots || (t.Type != nil && t.Type.IsFeature())
}

// allFeaturingTypes is the closure canAccess walks: a featuring type that is a feature
// contributes its own featuring types, or Anything when it has none.
func (a *featureAccess) allFeaturingTypes(f FeaturingType) []FeaturingType {
	var out []FeaturingType
	var visit func(FeaturingType)
	visit = func(f FeaturingType) {
		direct := a.featuringTypesOf(f)
		if len(direct) == 0 {
			direct = []FeaturingType{{}}
		}
		for _, t := range direct {
			known := false
			for _, have := range out {
				known = known || sameFeaturingType(have, t)
			}
			if known {
				continue
			}
			out = append(out, t)
			if isFeatureType(t) {
				visit(t)
			}
		}
	}
	visit(f)
	return out
}

func (a *featureAccess) canAccess(feature, target FeaturingType) bool {
	key := [2]FeaturingType{feature, target}
	if a.accessing[key] {
		return false
	}
	a.accessing[key] = true
	defer delete(a.accessing, key)
	for _, t := range a.allFeaturingTypes(feature) {
		if a.featuredWithin(target, t) {
			return true
		}
	}
	return false
}

func (a *featureAccess) featuredWithin(feature, t FeaturingType) bool {
	compatible := true
	for _, f := range a.featuringTypesOf(feature) {
		if !a.compatible(t, f) {
			compatible = false
			break
		}
	}
	if compatible || feature.Snapshots || t.IsAnything() || t.Snapshots {
		return compatible
	}
	// A variable feature, or a chain starting at one, is also featured within its owning type.
	m := a.m
	if m.FeatureIsVariable(feature.Type) && m.Conforms(t.Type, m.OwningType(feature.Type)) {
		return true
	}
	chaining := m.ChainingFeatures(feature.Type)
	return len(chaining) > 0 && m.FeatureIsVariable(chaining[0]) && m.Conforms(t.Type, m.OwningType(chaining[0]))
}

// compatible derives Type::isCompatibleWith and its Feature redefinition: t specializes f, or both
// are features without owned features that redefine a common feature and t can access f.
func (a *featureAccess) compatible(t, f FeaturingType) bool {
	if f.IsAnything() || sameFeaturingType(t, f) {
		return true
	}
	if t.IsAnything() {
		return false
	}
	if a.specializes(t, f) {
		return true
	}
	if !isFeatureType(t) || !isFeatureType(f) || a.hasOwnedFeatures(t) || a.hasOwnedFeatures(f) {
		return false
	}
	theirs := a.allRedefined(f)
	for _, mine := range a.allRedefined(t) {
		for _, shared := range theirs {
			if symbols.SameElement(mine, shared) {
				return a.canAccess(t, f)
			}
		}
	}
	return false
}

func (a *featureAccess) specializes(t, f FeaturingType) bool {
	switch {
	case f.Snapshots:
		return false
	case t.Snapshots:
		// The implied snapshots feature redefines Occurrences::Occurrence::snapshots.
		snapshots := a.m.occurrenceSnapshots()
		return snapshots != nil && a.m.Conforms(snapshots, f.Type)
	}
	return a.m.specializesByReference(t.Type, f.Type, map[*symbols.Symbol]bool{})
}

// specializesByReference is Type::specializes with the ReferenceSubsetting edges Conforms leaves
// out: a feature specializes what the feature it, or a general of it, references specializes.
func (m *Model) specializesByReference(t, f *symbols.Symbol, seen map[*symbols.Symbol]bool) bool {
	if m.Conforms(t, f) {
		return true
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	for general := range m.eachSelfAndSupertype(t) {
		if referenced := m.ReferencedFeature(general); referenced != nil && m.specializesByReference(referenced, f, seen) {
			return true
		}
	}
	return false
}

func (a *featureAccess) hasOwnedFeatures(t FeaturingType) bool {
	if t.Snapshots {
		return false
	}
	// Type::ownedFeature is the ownedMemberFeature of the FeatureMemberships, which a `member` feature has none of.
	owned, _ := a.m.ReflectiveElements(t.Type, "ownedFeature")
	for _, feature := range owned {
		if symbols.SameElement(a.m.OwningType(feature), t.Type) {
			return true
		}
	}
	return false
}

func (a *featureAccess) allRedefined(t FeaturingType) []*symbols.Symbol {
	if !t.Snapshots {
		return a.m.AllRedefinedFeatures(t.Type)
	}
	snapshots := a.m.occurrenceSnapshots()
	if snapshots == nil {
		return nil
	}
	return append([]*symbols.Symbol{snapshots}, a.m.AllRedefinedFeatures(snapshots)...)
}

// RedefinitionFeaturingTypesDiffer derives validateRedefinitionFeaturingTypes (KerML 1.1 §8.3.3.3.8): the two
// features' featuring types, Anything included and a variable feature's read as its owning type, are not the same set.
func (m *Model) RedefinitionFeaturingTypesDiffer(redefining, redefined *symbols.Symbol) bool {
	if m == nil || redefining == nil || redefined == nil {
		return true
	}
	return featuringTypeSetsDiffer(m.redefinitionFeaturingTypes(redefining), m.redefinitionFeaturingTypes(redefined))
}

// ChainRedefinitionFeaturingTypesDiffer is RedefinitionFeaturingTypesDiffer for a redefined feature
// written as a chain, whose featuring types are its first chaining feature's.
func (m *Model) ChainRedefinitionFeaturingTypesDiffer(redefining, firstChaining *symbols.Symbol) bool {
	if m == nil || redefining == nil || firstChaining == nil {
		return true
	}
	return featuringTypeSetsDiffer(m.redefinitionFeaturingTypes(redefining), withAnything(m.DerivedFeaturingTypes(firstChaining)))
}

func featuringTypeSetsDiffer(mine, theirs []FeaturingType) bool {
	if len(mine) != len(theirs) {
		return true
	}
	for _, t := range mine {
		found := false
		for _, other := range theirs {
			found = found || sameFeaturingType(t, other)
		}
		if !found {
			return true
		}
	}
	return false
}

func (m *Model) redefinitionFeaturingTypes(feature *symbols.Symbol) []FeaturingType {
	if m.FeatureIsVariable(feature) {
		return []FeaturingType{m.plainFeaturingType(m.OwningType(feature))}
	}
	return withAnything(m.DerivedFeaturingTypes(feature))
}

func withAnything(types []FeaturingType) []FeaturingType {
	for _, t := range types {
		if t.IsAnything() {
			return types
		}
	}
	return append(types[:len(types):len(types)], FeaturingType{})
}

// otherEnds returns the ends of end's owning type, inherited ones included, other than end itself.
func (m *Model) otherEnds(end *symbols.Symbol) []*symbols.Symbol {
	return slices.DeleteFunc(slices.Clone(m.endsOf(m.ownerOf(end))), func(other *symbols.Symbol) bool {
		return other == end
	})
}

// connectorDefaultFeaturingType derives Connector::defaultFeaturingType (KerML 1.1 §8.3.4.5.3
// deriveConnectorDefaultFeaturingType): the innermost featuring type every related feature is featured within.
func (m *Model) connectorDefaultFeaturingType(connector *symbols.Symbol) FeaturingType {
	var related []FeaturingType
	for _, feature := range m.ConnectorRelatedFeaturesByEnd(connector) {
		if feature != nil {
			related = append(related, FeaturingType{Type: feature})
		}
	}
	access := m.featureAccess()
	var common []FeaturingType
	for _, feature := range related {
		for _, t := range access.allFeaturingTypes(feature) {
			if t.Type == nil || slices.ContainsFunc(common, func(have FeaturingType) bool { return sameFeaturingType(have, t) }) {
				continue
			}
			if !slices.ContainsFunc(related, func(f FeaturingType) bool { return !access.featuredWithin(f, t) }) {
				common = append(common, t)
			}
		}
	}
	for _, t := range common {
		inner := slices.ContainsFunc(common, func(other FeaturingType) bool {
			return !sameFeaturingType(other, t) && isFeatureType(other) &&
				slices.ContainsFunc(access.allFeaturingTypes(other), func(outer FeaturingType) bool { return sameFeaturingType(outer, t) })
		})
		if !inner {
			return t
		}
	}
	return FeaturingType{}
}
