package metamodel

import (
	"slices"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

type propertyKey struct {
	element Element
	class   string
	name    string
}

type cachedValue struct {
	value Value
	ok    bool
}

// Evaluator derives metamodel properties from a resolved semantic model.
type Evaluator struct {
	resolver *resolve.Resolver
	model    *semantics.Model
	mu       sync.Mutex
	cache    map[propertyKey]cachedValue
}

// New creates an evaluator over the resolver and semantic model for one model.
func New(res *resolve.Resolver, model *semantics.Model) *Evaluator {
	return &Evaluator{
		resolver: res,
		model:    model,
		cache:    make(map[propertyKey]cachedValue),
	}
}

// Property returns a faithfully derived property value when the evaluator serves it.
func (e *Evaluator) Property(el Element, definingClass, name string) (Value, bool) {
	if e == nil || e.model == nil {
		return Value{}, false
	}
	key := propertyKey{element: el, class: definingClass, name: name}
	e.mu.Lock()
	if cached, ok := e.cache[key]; ok {
		e.mu.Unlock()
		return cloneValue(cached.value), cached.ok
	}
	e.mu.Unlock()
	if _, served := ruleConstraints[keyName(definingClass, name)]; !served {
		return Value{}, false
	}
	value, ok := e.derive(el, definingClass, name)
	value.Success = ok
	e.mu.Lock()
	if cached, found := e.cache[key]; found {
		value, ok = cloneValue(cached.value), cached.ok
	} else {
		e.cache[key] = cachedValue{value: cloneValue(value), ok: ok}
	}
	e.mu.Unlock()
	return value, ok
}

func (e *Evaluator) derive(el Element, class, name string) (Value, bool) {
	sym := el.Symbol
	switch class {
	case "Element":
		return e.elementProperty(el, sym, name)
	case "Namespace":
		return e.namespaceProperty(el, sym, name)
	case "Type":
		return e.typeProperty(el, sym, name)
	case "Feature":
		return e.featureProperty(el, sym, name)
	case "Usage":
		return e.usageProperty(el, sym, name)
	case "Definition":
		return e.definitionProperty(el, sym, name)
	case "Membership":
		return e.membershipProperty(el, name)
	case "Relationship":
		return e.relationshipProperty(el, name)
	default:
		return e.reflectiveProperty(el, name)
	}
}

func (e *Evaluator) elementProperty(el Element, sym *symbols.Symbol, name string) (Value, bool) {
	switch name {
	case "name":
		if sym == nil || el.IsMembership {
			return Value{}, false
		}
		if name := e.model.EffectiveNameOf(sym); name != "" {
			return stringValue(name), true
		} else {
			return nullValue(), true
		}
	case "shortName":
		if sym == nil || el.IsMembership {
			return Value{}, false
		}
		if e.model.EffectiveShortNameOf(sym) == "" {
			return nullValue(), true
		}
		return stringValue(e.model.EffectiveShortNameOf(sym)), true
	case "owner":
		if el.IsMembership {
			if el.Membership.Owner != nil {
				return elementValue(ElementOf(el.Membership.Owner)), true
			}
			return nullValue(), true
		}
		if sym != nil {
			if owner := sym.Owner(); owner != nil {
				return elementValue(ElementOf(owner)), true
			}
		}
		if el.Container != nil {
			return elementValue(ElementOf(el.Container)), true
		}
		return nullValue(), true
	case "owningNamespace":
		if el.IsMembership {
			if isNamespace(el.Membership.Owner) {
				return elementValue(ElementOf(el.Membership.Owner)), true
			}
			return nullValue(), true
		}
		membership := el.Membership
		if !hasMembership(membership) && sym != nil {
			membership = ElementOf(sym).Membership
		}
		if isNamespace(membership.Owner) {
			return elementValue(ElementOf(membership.Owner)), true
		}
		return nullValue(), true
	case "owningMembership":
		if el.IsMembership || (sym != nil && sym.Kind == symbols.SymbolAlias) {
			return nullValue(), true
		}
		if el.Membership.Member != nil || el.Membership.Symbol != nil || el.Membership.Node != nil {
			return membershipValue(el.Membership), true
		}
		if sym != nil {
			if membership := ElementOf(sym).Membership; hasMembership(membership) {
				return membershipValue(membership), true
			}
		}
		return nullValue(), true
	case "owningRelationship":
		if el.IsMembership || (sym != nil && sym.Kind == symbols.SymbolAlias) {
			return sequence(), true
		}
		if el.Membership.Member != nil || el.Membership.Symbol != nil || el.Membership.Node != nil {
			return elementValue(MembershipElement(el.Membership)), true
		}
		if sym != nil {
			if membership := ElementOf(sym).Membership; hasMembership(membership) {
				return elementValue(MembershipElement(membership)), true
			}
		}
		return sequence(), true
	case "qualifiedName":
		if sym == nil || el.IsMembership || e.resolver == nil || e.resolver.Index() == nil {
			return Value{}, false
		}
		if e.model.EffectiveNameOf(sym) == "" {
			return nullValue(), true
		}
		return stringValue(e.resolver.Index().GetFQN(sym)), true
	case "isLibraryElement":
		if e.resolver == nil || e.resolver.Index() == nil {
			return Value{}, false
		}
		if sym != nil {
			return booleanValue(e.resolver.Index().Library(sym)), true
		}
		if el.IsMembership {
			candidate := el.Membership.Member
			if candidate == nil {
				candidate = el.Membership.Owner
			}
			if candidate != nil {
				return booleanValue(e.resolver.Index().Library(candidate)), true
			}
		}
		if el.Container != nil {
			return booleanValue(e.resolver.Index().Library(el.Container)), true
		}
		return Value{}, false
	case "isImpliedIncluded":
		return booleanValue(false), true
	case "ownedElement":
		values := e.ownedElements(sym)
		return elementHandles(values), true
	case "ownedRelationship":
		return elementHandles(e.ownedRelationships(sym)), true
	case "ownedAnnotation":
		if sym == nil {
			return Value{}, false
		}
		sites := e.model.AnnotationSitesOf(sym)
		values := make([]Value, 0, len(sites))
		for _, site := range sites {
			if site.Node == nil {
				return Value{}, false
			}
			values = append(values, elementValue(Element{Node: site.Node, Container: sym, Aspect: "annotation"}))
		}
		return sequence(values...), true
	case "documentation":
		if sym == nil {
			return Value{}, false
		}
		values, ok := e.model.ReflectiveElements(sym, "documentation")
		if !ok {
			return Value{}, false
		}
		return symbolSequence(values), true
	case "textualRepresentation":
		return symbolSequence(symbolsOfKind(sym, symbols.SymbolTextualRepresentation)), true
	}
	return Value{}, false
}

func (e *Evaluator) namespaceProperty(el Element, sym *symbols.Symbol, name string) (Value, bool) {
	if sym == nil {
		return Value{}, false
	}
	switch name {
	case "ownedMember":
		members, ok := e.model.ReflectiveElements(sym, "ownedMember")
		if !ok {
			return Value{}, false
		}
		ordered := make([]*symbols.Symbol, 0, len(members))
		for _, member := range directOwnedSymbols(e.model, sym) {
			if member.Kind == symbols.SymbolAlias {
				if e.resolver == nil {
					return Value{}, false
				}
				target, resolved := e.resolver.ResolveAliasTarget(member)
				if !resolved || target == nil {
					return Value{}, false
				}
				member = target
			} else if !hasMembership(ElementOf(member).Membership) {
				continue
			}
			ordered = appendUniqueSymbols(ordered, []*symbols.Symbol{member})
		}
		ordered = appendUniqueSymbols(ordered, members)
		return symbolSequence(ordered), true
	case "member":
		members, ok := e.namespaceMembers(sym)
		if !ok {
			return Value{}, false
		}
		members, ok = e.memberElements(members)
		if !ok {
			return Value{}, false
		}
		return symbolSequence(members), true
	case "ownedMembership":
		return membershipSequence(ownedMemberships(e.model, sym)), true
	case "membership":
		members, ok := e.namespaceMembers(sym)
		if !ok {
			return Value{}, false
		}
		return membershipSequence(uniqueMemberships(memberships(members))), true
	case "importedMembership":
		imported, ok := e.importedSymbols(sym)
		if !ok {
			return Value{}, false
		}
		return membershipSequence(memberships(imported)), true
	case "ownedImport":
		values := make([]Value, 0)
		if sym.Scope != nil {
			for _, imp := range sym.Scope.Imports() {
				values = append(values, elementValue(Element{Node: imp, Container: sym}))
			}
		}
		return sequence(values...), true
	}
	return Value{}, false
}

func (e *Evaluator) typeProperty(el Element, sym *symbols.Symbol, name string) (Value, bool) {
	if sym == nil {
		return Value{}, false
	}
	switch name {
	case "feature":
		members, ok := e.effectiveFeatures(sym)
		if !ok {
			return Value{}, false
		}
		return elementHandles(features(members)), true
	case "ownedFeature":
		return symbolSequence(ownedFeatureSymbols(e.model, sym)), true
	case "inheritedFeature":
		if !e.completeMembers(sym) {
			return Value{}, false
		}
		members, ok := e.model.ReflectiveElements(sym, "feature")
		if !ok {
			return Value{}, false
		}
		return symbolSequence(e.inheritedSymbols(sym, members)), true
	case "inheritedMembership":
		if !e.completeMembers(sym) {
			return Value{}, false
		}
		members, ok := e.model.ReflectiveElements(sym, "member")
		if !ok {
			return Value{}, false
		}
		return membershipSequence(memberships(e.inheritedSymbols(sym, members))), true
	case "featureMembership":
		members, ok := e.effectiveFeatures(sym)
		if !ok {
			return Value{}, false
		}
		return membershipSequence(featureMemberships(members)), true
	case "ownedFeatureMembership":
		return membershipSequence(featureMemberships(ownedFeatureSymbols(e.model, sym))), true
	case "input", "output", "directedFeature":
		members, ok := e.effectiveFeatures(sym)
		if !ok {
			return Value{}, false
		}
		directed, ok := e.directedFeatures(sym, members, name)
		if !ok {
			return Value{}, false
		}
		return symbolSequence(directed), true
	case "ownedSpecialization":
		return nodeSequence(ownedSpecializations(sym), sym)
	case "endFeature":
		members, ok := e.effectiveFeatures(sym)
		if !ok {
			return Value{}, false
		}
		return elementHandles(endFeatures(members)), true
	case "ownedEndFeature":
		return elementHandles(endFeatures(ownedFeatureSymbols(e.model, sym))), true
	case "multiplicity":
		if node := multiplicityOf(sym); node != nil {
			return elementValue(Element{Node: node, Container: sym}), true
		}
		return nullValue(), true
	case "differencingType", "intersectingType", "unioningType":
		kind := map[string]ast.RelationshipKind{
			"differencingType": ast.RelDifferences,
			"intersectingType": ast.RelIntersects,
			"unioningType":     ast.RelUnions,
		}[name]
		return e.relationshipTargets(sym, kind)
	case "isConjugated":
		if !e.completeSupertypes(sym) {
			return Value{}, false
		}
		return booleanValue(e.model.IsConjugated(sym)), true
	}
	return Value{}, false
}

func (e *Evaluator) featureProperty(el Element, sym *symbols.Symbol, name string) (Value, bool) {
	if name == "direction" && sym == nil {
		direction := e.featureDirectionOf(el)
		if direction == ast.DirNone {
			return nullValue(), true
		}
		return enumValue(direction.String()), true
	}
	if sym == nil || !sym.IsFeature() {
		return Value{}, false
	}
	switch name {
	case "type":
		types, ok := e.featureTypes(sym)
		if !ok {
			return Value{}, false
		}
		return symbolSequence(types), true
	case "owningType":
		if !e.completeSupertypes(sym) {
			return Value{}, false
		}
		membership := ElementOf(sym).Membership
		if membership.Feature && membership.Owner != nil {
			return elementValue(ElementOf(membership.Owner)), true
		}
		return nullValue(), true
	case "featuringType":
		types, ok := e.featuringTypes(sym, make(map[*symbols.Symbol]bool))
		if !ok {
			return Value{}, false
		}
		return symbolSequence(types), true
	case "ownedTyping":
		return relationshipNodes(sym, ast.RelTyping), true
	case "ownedSubsetting":
		return relationshipNodes(sym, ast.RelSubsets), true
	case "ownedRedefinition":
		return relationshipNodes(sym, ast.RelRedefines), true
	case "ownedReferenceSubsetting":
		return relationshipNodesWhere(sym, func(rel *ast.Relationship) bool { return rel.Kind.ReferenceSubsets() })
	case "featureTarget":
		if target := e.model.ReferencedFeature(sym); target != nil {
			return elementValue(ElementOf(target)), true
		}
		for _, rel := range semantics.RelationshipsOf(sym) {
			if rel != nil && rel.Kind.ReferenceSubsets() && rel.Target != nil {
				return Value{}, false
			}
		}
		if usage, ok := sym.Decl.(*ast.Usage); ok && usage.IsVariantReference() {
			return Value{}, false
		}
		return nullValue(), true
	case "isComposite":
		if _, ok := sym.Decl.(*ast.Usage); ok {
			return booleanValue(semantics.UsageIsComposite(sym)), true
		}
		if value, ok := e.model.ReflectiveFeatureValue(sym, "isComposite"); ok {
			if result, yes := filterBool(value); yes {
				return booleanValue(result), true
			}
		}
		return booleanValue(false), true
	case "direction":
		direction := e.featureDirectionOf(el)
		if direction == ast.DirNone {
			return nullValue(), true
		}
		return enumValue(direction.String()), true
	case "chainingFeature":
		return e.relationshipTargets(sym, ast.RelChains)
	case "crossFeature":
		for _, rel := range semantics.RelationshipsOf(sym) {
			if rel != nil && rel.Kind == ast.RelCrosses && rel.Target != nil &&
				e.model.RelationshipTarget(sym, rel) == nil {
				return Value{}, false
			}
		}
		if target := e.model.CrossFeature(sym); target != nil {
			return elementValue(ElementOf(target)), true
		}
		return nullValue(), true
	}
	return Value{}, false
}

func (e *Evaluator) usageProperty(el Element, sym *symbols.Symbol, name string) (Value, bool) {
	if sym == nil {
		return Value{}, false
	}
	switch name {
	case "definition":
		types, ok := e.featureTypes(sym)
		if !ok {
			return Value{}, false
		}
		return elementHandles(definitions(types)), true
	case "usage", "directedUsage":
		members, ok := e.effectiveFeatures(sym)
		if !ok {
			return Value{}, false
		}
		usages := usageSymbols(members)
		if name == "directedUsage" {
			directed, ok := e.directedFeatures(sym, members, "directedFeature")
			if !ok {
				return Value{}, false
			}
			usages = usageSymbols(directed)
		}
		return elementHandles(usages), true
	case "ownedUsage":
		members, ok := e.model.ReflectiveElements(sym, "ownedMember")
		if !ok {
			return Value{}, false
		}
		return elementHandles(usageSymbols(members)), true
	case "nestedUsage":
		if !e.completeMembers(sym) {
			return Value{}, false
		}
		if value, ok := e.model.ReflectiveElements(sym, name); ok {
			return symbolSequence(value), true
		}
		return Value{}, false
	case "isReference":
		if _, ok := sym.Decl.(*ast.Usage); ok {
			return booleanValue(semantics.UsageIsReferential(sym)), true
		}
		if value, ok := e.model.ReflectiveFeatureValue(sym, "isReference"); ok {
			if result, yes := filterBool(value); yes {
				return booleanValue(result), true
			}
		}
		return booleanValue(false), true
	case "variant":
		members, ok := e.effectiveFeatures(sym)
		if !ok {
			return Value{}, false
		}
		return elementHandles(variants(members)), true
	case "variantMembership":
		members, ok := e.effectiveFeatures(sym)
		if !ok {
			return Value{}, false
		}
		return membershipSequence(memberships(variantSymbols(members))), true
	case "owningUsage", "owningDefinition":
		for owner := sym.Owner(); owner != nil; owner = owner.Owner() {
			switch name {
			case "owningUsage":
				if _, ok := owner.Decl.(*ast.Usage); ok {
					return elementValue(ElementOf(owner)), true
				}
			case "owningDefinition":
				if _, ok := owner.Decl.(*ast.Definition); ok {
					return elementValue(ElementOf(owner)), true
				}
			}
		}
		return nullValue(), true
	}
	if strings.HasPrefix(name, "owned") || strings.HasPrefix(name, "nested") {
		if strings.HasPrefix(name, "nested") && !e.completeMembers(sym) {
			return Value{}, false
		}
		if value, ok := e.model.ReflectiveElements(sym, name); ok {
			return symbolSequence(value), true
		}
		return Value{}, false
	}
	return Value{}, false
}

func (e *Evaluator) definitionProperty(el Element, sym *symbols.Symbol, name string) (Value, bool) {
	if sym == nil {
		return Value{}, false
	}
	switch name {
	case "usage":
		members, ok := e.effectiveFeatures(sym)
		if !ok {
			return Value{}, false
		}
		return elementHandles(usageSymbols(members)), true
	case "ownedUsage":
		value, ok := e.model.ReflectiveElements(sym, name)
		if !ok {
			return Value{}, false
		}
		return symbolSequence(value), true
	case "directedUsage":
		members, ok := e.effectiveFeatures(sym)
		if !ok {
			return Value{}, false
		}
		directed, ok := e.directedFeatures(sym, members, "directedFeature")
		if !ok {
			return Value{}, false
		}
		return elementHandles(usageSymbols(directed)), true
	case "variant":
		members, ok := e.effectiveFeatures(sym)
		if !ok {
			return Value{}, false
		}
		return elementHandles(variants(members)), true
	case "variantMembership":
		members, ok := e.effectiveFeatures(sym)
		if !ok {
			return Value{}, false
		}
		return membershipSequence(memberships(variantSymbols(members))), true
	}
	if strings.HasPrefix(name, "owned") {
		if value, ok := e.model.ReflectiveElements(sym, name); ok {
			return symbolSequence(value), true
		}
		return Value{}, false
	}
	return Value{}, false
}

func (e *Evaluator) membershipProperty(el Element, name string) (Value, bool) {
	mem := el.Membership
	if mem.Member == nil && mem.Symbol != nil {
		mem.Member = mem.Symbol
	}
	if mem.Member == nil {
		return Value{}, false
	}
	switch name {
	case "memberElement":
		if mem.Member.Kind == symbols.SymbolAlias {
			if e.resolver == nil {
				return Value{}, false
			}
			target, ok := e.resolver.ResolveAliasTarget(mem.Member)
			if !ok || target == nil {
				return Value{}, false
			}
			return elementValue(ElementOf(target)), true
		}
		return elementValue(ElementOf(mem.Member)), true
	case "memberName":
		if name := e.model.EffectiveNameOf(mem.Member); name != "" {
			return stringValue(name), true
		}
		return nullValue(), true
	case "memberShortName":
		if name := e.model.EffectiveShortNameOf(mem.Member); name != "" {
			return stringValue(name), true
		}
		return nullValue(), true
	case "membershipOwningNamespace":
		if isNamespace(mem.Owner) {
			return elementValue(ElementOf(mem.Owner)), true
		}
		return nullValue(), true
	}
	return Value{}, false
}

func (e *Evaluator) relationshipProperty(el Element, name string) (Value, bool) {
	if el.IsMembership && hasMembership(el.Membership) {
		membership := el.Membership
		member := membership.Member
		if member == nil && membership.Symbol != nil {
			member = membership.Symbol
		}
		if member == nil {
			return Value{}, false
		}
		if member.Kind == symbols.SymbolAlias {
			if e.resolver == nil {
				return Value{}, false
			}
			target, ok := e.resolver.ResolveAliasTarget(member)
			if !ok || target == nil {
				return Value{}, false
			}
			member = target
		}
		switch name {
		case "source":
			if membership.Owner == nil {
				return Value{}, false
			}
			return symbolSequence([]*symbols.Symbol{membership.Owner}), true
		case "target", "ownedRelatedElement":
			return symbolSequence([]*symbols.Symbol{member}), true
		case "relatedElement":
			if membership.Owner == nil {
				return Value{}, false
			}
			return symbolSequence([]*symbols.Symbol{membership.Owner, member}), true
		case "owningRelatedElement":
			if membership.Owner == nil {
				return nullValue(), true
			}
			return elementValue(ElementOf(membership.Owner)), true
		}
	}
	switch node := el.Node.(type) {
	case *ast.Relationship:
		if el.Container == nil {
			return Value{}, false
		}
		switch name {
		case "source":
			return symbolSequence([]*symbols.Symbol{el.Container}), true
		case "target":
			target := e.model.RelationshipTarget(el.Container, node)
			if target == nil {
				return Value{}, false
			}
			return symbolSequence([]*symbols.Symbol{target}), true
		case "relatedElement":
			target := e.model.RelationshipTarget(el.Container, node)
			if target == nil {
				return Value{}, false
			}
			return symbolSequence([]*symbols.Symbol{el.Container, target}), true
		case "owningRelatedElement":
			return elementValue(ElementOf(el.Container)), true
		case "ownedRelatedElement":
			target := e.model.RelationshipTarget(el.Container, node)
			if target == nil {
				return Value{}, false
			}
			return symbolSequence([]*symbols.Symbol{target}), true
		}
	case *ast.RelationshipMember:
		if el.Symbol == nil {
			return Value{}, false
		}
		switch name {
		case "source":
			return e.resolveNode(node.Source, el.Symbol.OwnerScope)
		case "target":
			return e.resolveNode(node.Target, el.Symbol.OwnerScope)
		case "relatedElement":
			source, sourceOK := e.resolveNode(node.Source, el.Symbol.OwnerScope)
			target, targetOK := e.resolveNode(node.Target, el.Symbol.OwnerScope)
			if !sourceOK || !targetOK {
				return Value{}, false
			}
			return sequence(source.Values[0], target.Values[0]), true
		case "owningRelatedElement":
			if owner := el.Symbol.Owner(); owner != nil {
				return elementValue(ElementOf(owner)), true
			}
		case "ownedRelatedElement":
			return e.resolveNode(node.Target, el.Symbol.OwnerScope)
		}
	}
	return Value{}, false
}

func (e *Evaluator) resolveNode(node ast.Node, scope *symbols.Scope) (Value, bool) {
	if scope == nil || node == nil || e.resolver == nil {
		return Value{}, false
	}
	sym, ok := e.resolver.ResolveTarget(scope, node)
	if !ok || sym == nil {
		return Value{}, false
	}
	return sequence(elementValue(ElementOf(sym))), true
}

func (e *Evaluator) relationshipTargets(sym *symbols.Symbol, kind ast.RelationshipKind) (Value, bool) {
	var values []Value
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != kind {
			continue
		}
		target := e.model.RelationshipTarget(sym, rel)
		if target == nil {
			return Value{}, false
		}
		values = append(values, elementValue(ElementOf(target)))
	}
	return sequence(values...), true
}

func (e *Evaluator) completeSupertypes(sym *symbols.Symbol) bool {
	if !e.completeRelationships(sym) || e.model.SupertypesProvisional(sym) ||
		e.model.HasSpecializationCycle(sym) {
		return false
	}
	supers := append([]*symbols.Symbol{sym}, e.model.AllSupertypes(sym)...)
	for _, super := range supers {
		if e.model.SupertypesProvisional(super) || e.model.HasSpecializationCycle(super) {
			return false
		}
		for _, base := range e.model.KindBaseFQNs(super, source.KindOf(symbols.DocNameOf(super.Scope)) == source.KindKerML) {
			if e.resolver == nil || e.resolver.Index() == nil ||
				len(e.resolver.Index().LookupQualified(base)) == 0 {
				return false
			}
		}
	}
	return true
}

func (e *Evaluator) completeMembers(sym *symbols.Symbol) bool {
	if !e.completeSupertypes(sym) {
		return false
	}
	e.model.MemberSources(sym)
	return e.model.MemberSourcesStable(sym)
}

func (e *Evaluator) featureTypes(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if !e.completeSupertypes(sym) {
		return nil, false
	}
	types := e.model.FeatureTypeSet(sym)
	if len(types) == 0 {
		if _, hasBase := e.model.FeatureBaseFQN(sym); hasBase {
			return nil, false
		}
	}
	return types, true
}

func (e *Evaluator) completeRelationships(sym *symbols.Symbol) bool {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil {
			return false
		}
		if !semantics.GeneralizationKind(rel.Kind) && !rel.Kind.ReferenceSubsets() {
			continue
		}
		if rel.Target == nil || e.model.RelationshipTarget(sym, rel) == nil {
			return false
		}
	}
	return true
}

func (e *Evaluator) directedFeatures(owner *symbols.Symbol, members []*symbols.Symbol, property string) ([]*symbols.Symbol, bool) {
	out := make([]*symbols.Symbol, 0)
	for _, member := range members {
		if member == nil || !member.IsFeature() {
			continue
		}
		direction := e.model.EffectiveDirection(owner, member)
		if property == "directedFeature" {
			direction = e.featureDirection(member)
		}
		switch property {
		case "input":
			if direction == ast.DirIn || direction == ast.DirInOut {
				out = append(out, member)
			}
		case "output":
			if direction == ast.DirOut || direction == ast.DirInOut {
				out = append(out, member)
			}
		case "directedFeature":
			if direction != ast.DirNone {
				out = append(out, member)
			}
		}
	}
	return out, true
}

func (e *Evaluator) featureDirection(feature *symbols.Symbol) ast.FeatureDirection {
	if direction := e.model.DeclaredDirection(feature); direction != ast.DirNone {
		return direction
	}
	membership := ElementOf(feature).Membership
	switch membership.kind {
	case "ParameterMembership":
		return ast.DirIn
	case "ReturnParameterMembership":
		return ast.DirOut
	}
	if usage, ok := feature.Decl.(*ast.Usage); ok {
		if usage.IsBodyParameter {
			return ast.DirIn
		}
		if usage.IsResult {
			return ast.DirOut
		}
	}
	return ast.DirNone
}

func (e *Evaluator) featureDirectionOf(el Element) ast.FeatureDirection {
	if el.Symbol != nil {
		return e.featureDirection(el.Symbol)
	}
	if usage, ok := el.Node.(*ast.Usage); ok {
		if usage.Direction != ast.DirNone {
			return usage.Direction
		}
		if usage.IsBodyParameter {
			return ast.DirIn
		}
		if usage.IsResult {
			return ast.DirOut
		}
	}
	return ast.DirNone
}

func (e *Evaluator) featuringTypes(sym *symbols.Symbol, visiting map[*symbols.Symbol]bool) ([]*symbols.Symbol, bool) {
	if sym == nil || visiting[sym] || !e.completeSupertypes(sym) {
		return nil, false
	}
	visiting[sym] = true
	defer delete(visiting, sym)

	membership := ElementOf(sym).Membership
	var types []*symbols.Symbol
	if isType(membership.Owner) && membership.kind != "VariantMembership" {
		types = append(types, membership.Owner)
	}
	chainingFeatureFound := false
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelChains:
			if chainingFeatureFound {
				continue
			}
			chainingFeatureFound = true
			target := e.model.RelationshipTarget(sym, rel)
			if target == nil {
				return nil, false
			}
			chainTypes, ok := e.featuringTypes(target, visiting)
			if !ok {
				return nil, false
			}
			types = appendUniqueSymbols(types, chainTypes)
		case ast.RelFeaturedBy:
			target := e.model.RelationshipTarget(sym, rel)
			if target == nil {
				return nil, false
			}
			types = appendUniqueSymbols(types, []*symbols.Symbol{target})
		}
	}
	return types, true
}

func (e *Evaluator) effectiveFeatures(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if !e.completeMembers(sym) {
		return nil, false
	}
	members, ok := e.namespaceMembers(sym)
	if !ok {
		return nil, false
	}
	return e.memberElements(members)
}

func (e *Evaluator) memberElements(members []*symbols.Symbol) ([]*symbols.Symbol, bool) {
	out := make([]*symbols.Symbol, 0, len(members))
	for _, member := range members {
		if member == nil {
			return nil, false
		}
		if member.Kind == symbols.SymbolAlias {
			if e.resolver == nil {
				return nil, false
			}
			target, ok := e.resolver.ResolveAliasTarget(member)
			if !ok || target == nil {
				return nil, false
			}
			member = target
		}
		if !containsSymbol(out, member) {
			out = append(out, member)
		}
	}
	return out, true
}

func (e *Evaluator) namespaceMembers(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if sym == nil {
		return nil, false
	}
	if isType(sym) && !e.completeMembers(sym) {
		return nil, false
	}
	all, ok := e.model.ReflectiveElements(sym, "member")
	if !ok {
		return nil, false
	}
	if isType(sym) {
		visible := make([]*symbols.Symbol, 0, len(all))
		for _, member := range all {
			if member != nil && member.Owner() != sym &&
				(e.resolver == nil || !e.resolver.InheritedMemberVisible(sym, member)) {
				continue
			}
			visible = append(visible, member)
		}
		all = visible
	}
	members := make([]*symbols.Symbol, 0, len(all))
	seen := make(map[*symbols.Symbol]bool, len(members))
	appendMember := func(member *symbols.Symbol) {
		if member != nil && !seen[member] {
			seen[member] = true
			members = append(members, member)
		}
	}
	for _, member := range directOwnedSymbols(e.model, sym) {
		if member != nil {
			appendMember(member)
		}
	}
	for _, member := range all {
		if member != nil && member.Owner() == sym {
			appendMember(member)
		}
	}
	for _, member := range all {
		if member != nil && member.Owner() != sym {
			appendMember(member)
		}
	}
	if sym.Scope == nil {
		return members, true
	}
	for _, imp := range sym.Scope.Imports() {
		if e.resolver == nil {
			return nil, false
		}
		target, ok := e.resolver.ResolveTarget(sym.Scope, imp.Imported)
		if !ok || target == nil {
			return nil, false
		}
		if !e.importTreeComplete(target, imp, make(map[*symbols.Scope]bool)) {
			return nil, false
		}
		for _, imported := range e.resolver.ImportedElements(sym.Scope, imp) {
			if imported != nil && !seen[imported] {
				seen[imported] = true
				members = append(members, imported)
			}
		}
	}
	return members, true
}

func (e *Evaluator) importedSymbols(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	var out []*symbols.Symbol
	if sym == nil || sym.Scope == nil {
		return out, true
	}
	if e.resolver == nil {
		if len(sym.Scope.Imports()) > 0 {
			return nil, false
		}
		return out, true
	}
	seen := make(map[*symbols.Symbol]bool)
	for _, imp := range sym.Scope.Imports() {
		target, ok := e.resolver.ResolveTarget(sym.Scope, imp.Imported)
		if !ok || target == nil {
			return nil, false
		}
		if !e.importTreeComplete(target, imp, make(map[*symbols.Scope]bool)) {
			return nil, false
		}
		for _, imported := range e.resolver.ImportedElements(sym.Scope, imp) {
			if imported != nil && !seen[imported] {
				seen[imported] = true
				out = append(out, imported)
			}
		}
	}
	return out, true
}

func (e *Evaluator) importTreeComplete(target *symbols.Symbol, imp *ast.Import, seen map[*symbols.Scope]bool) bool {
	if target == nil || imp == nil || (imp.Kind != ast.ImportNamespace && !imp.IsRecursive) {
		return true
	}
	if target.Kind == symbols.SymbolAlias {
		if e.resolver == nil {
			return false
		}
		resolved, ok := e.resolver.ResolveAliasTarget(target)
		if !ok || resolved == nil {
			return false
		}
		target = resolved
	}
	scope := target.Scope
	if scope == nil || seen[scope] {
		return true
	}
	seen[scope] = true
	for _, child := range scope.Imports() {
		childTarget, ok := e.resolver.ResolveTarget(scope, child.Imported)
		if !ok || childTarget == nil || !e.importTreeComplete(childTarget, child, seen) {
			return false
		}
	}
	return true
}

func (e *Evaluator) ownedElements(sym *symbols.Symbol) []Element {
	if sym == nil {
		return nil
	}
	members, ok := e.model.ReflectiveElements(sym, "ownedElement")
	if !ok {
		return nil
	}
	out := make([]Element, 0, len(members))
	for _, member := range members {
		out = append(out, ElementOf(member))
	}
	return out
}

func (e *Evaluator) ownedRelationships(sym *symbols.Symbol) []Element {
	if sym == nil {
		return nil
	}
	var out []Element
	seen := make(map[Element]bool)
	appendElement := func(el Element) {
		if !seen[el] {
			seen[el] = true
			out = append(out, el)
		}
	}
	for _, member := range directOwnedSymbols(e.model, sym) {
		if member.Kind == symbols.SymbolRelationship {
			appendElement(ElementOf(member))
			continue
		}
		membership := ElementOf(member).Membership
		if hasMembership(membership) {
			if member.Kind == symbols.SymbolAlias {
				appendElement(ElementOf(member))
			} else {
				appendElement(MembershipElement(membership))
			}
		}
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel != nil {
			appendElement(Element{Node: rel, Container: sym})
		}
	}
	if sym.Scope != nil {
		for _, imp := range sym.Scope.Imports() {
			appendElement(Element{Node: imp, Container: sym})
		}
	}
	for _, prefix := range prefixesOf(sym.Decl) {
		appendElement(Element{Node: prefix, Container: sym, Aspect: "annotation"})
	}
	for _, site := range e.model.AnnotationSitesOf(sym) {
		if site.Node != nil {
			appendElement(Element{Node: site.Node, Container: sym, Aspect: "annotation"})
		}
	}
	return out
}

func (e *Evaluator) reflectiveProperty(el Element, name string) (Value, bool) {
	sym := el.Symbol
	if sym == nil {
		return Value{}, false
	}
	if elements, ok := e.model.ReflectiveElements(sym, name); ok {
		return symbolSequence(elements), true
	}
	values, ok := e.model.ReflectiveFeatureValues(sym, name)
	if !ok {
		return Value{}, false
	}
	out := make([]Value, 0, len(values))
	for _, value := range values {
		switch value.Kind {
		case symbols.FilterValueBool:
			out = append(out, booleanValue(value.Bool))
		case symbols.FilterValueInt:
			if value.BigInt != nil {
				return Value{}, false
			}
			out = append(out, integerValue(value.Int))
		case symbols.FilterValueReal:
			out = append(out, realValue(value.Real))
		case symbols.FilterValueString:
			out = append(out, stringValue(value.Str))
		case symbols.FilterValueRef:
			if e.resolver == nil || e.resolver.Index() == nil {
				return Value{}, false
			}
			targets := e.resolver.Index().LookupQualified(value.RefFQN)
			if len(targets) != 1 || targets[0] == nil {
				return Value{}, false
			}
			out = append(out, elementValue(ElementOf(targets[0])))
		case symbols.FilterValueEmpty:
		default:
			return Value{}, false
		}
	}
	return sequence(out...), true
}

func elementHandles(elements []Element) Value {
	values := make([]Value, 0, len(elements))
	for _, el := range elements {
		values = append(values, elementValue(el))
	}
	return sequence(values...)
}

func membershipSequence(members []Membership) Value {
	values := make([]Value, 0, len(members))
	for _, member := range members {
		values = append(values, membershipValue(member))
	}
	return sequence(values...)
}

func nodeSequence(nodes []*ast.Relationship, owner *symbols.Symbol) (Value, bool) {
	values := make([]Value, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			return Value{}, false
		}
		values = append(values, elementValue(Element{Node: node, Container: owner}))
	}
	return sequence(values...), true
}

func relationshipNodes(sym *symbols.Symbol, kind ast.RelationshipKind) Value {
	values, _ := relationshipNodesWhere(sym, func(rel *ast.Relationship) bool { return rel.Kind == kind })
	return values
}

func relationshipNodesWhere(sym *symbols.Symbol, keep func(*ast.Relationship) bool) (Value, bool) {
	values := make([]Value, 0)
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel != nil && keep(rel) {
			values = append(values, elementValue(Element{Node: rel, Container: sym}))
		}
	}
	return sequence(values...), true
}

func features(members []*symbols.Symbol) []Element {
	out := make([]Element, 0)
	for _, member := range members {
		if member != nil && member.IsFeature() {
			out = append(out, ElementOf(member))
		}
	}
	return out
}

func memberships(members []*symbols.Symbol) []Membership {
	out := make([]Membership, 0, len(members))
	for _, member := range members {
		if member != nil {
			membership := ElementOf(member).Membership
			if hasMembership(membership) {
				out = append(out, membership)
			}
		}
	}
	return out
}

func featureMemberships(members []*symbols.Symbol) []Membership {
	out := make([]Membership, 0)
	for _, member := range members {
		if member == nil || !member.IsFeature() {
			continue
		}
		membership := ElementOf(member).Membership
		if membership.Feature {
			out = append(out, membership)
		}
	}
	return out
}

func uniqueMemberships(members []Membership) []Membership {
	out := make([]Membership, 0, len(members))
	seen := make(map[Membership]bool, len(members))
	for _, member := range members {
		if !seen[member] {
			seen[member] = true
			out = append(out, member)
		}
	}
	return out
}

func endFeatures(members []*symbols.Symbol) []Element {
	var out []Element
	for _, member := range members {
		if member == nil {
			continue
		}
		if usage, ok := member.Decl.(*ast.Usage); ok && usage.IsEnd {
			out = append(out, ElementOf(member))
		}
		if member.Kind == symbols.SymbolConnectorEnd {
			out = append(out, ElementOf(member))
		}
	}
	return out
}

func usageSymbols(members []*symbols.Symbol) []Element {
	out := make([]Element, 0)
	for _, member := range members {
		if member != nil {
			if _, ok := member.Decl.(*ast.Usage); ok {
				out = append(out, ElementOf(member))
			}
		}
	}
	return out
}

func variants(members []*symbols.Symbol) []Element {
	var out []Element
	for _, member := range variantSymbols(members) {
		out = append(out, ElementOf(member))
	}
	return out
}

func variantSymbols(members []*symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, member := range members {
		if member != nil {
			if usage, ok := member.Decl.(*ast.Usage); ok && usage.IsVariant {
				out = append(out, member)
			}
		}
	}
	return out
}

func ownedSpecializations(sym *symbols.Symbol) []*ast.Relationship {
	var out []*ast.Relationship
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel != nil && semantics.GeneralizationKind(rel.Kind) {
			out = append(out, rel)
		}
	}
	return out
}

func definitions(members []*symbols.Symbol) []Element {
	var out []Element
	for _, member := range members {
		if member != nil && member.Kind.IsDefinition() {
			out = append(out, ElementOf(member))
		}
	}
	return out
}

func ownedSymbols(sym *symbols.Symbol, includeAliases bool) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	members := sym.Scope.Members()
	if includeAliases {
		return members
	}
	out := make([]*symbols.Symbol, 0, len(members))
	for _, member := range members {
		if member.Kind != symbols.SymbolAlias {
			out = append(out, member)
		}
	}
	return out
}

func isNamespace(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch sym.Decl.(type) {
	case *ast.Package, *ast.Namespace, *ast.Definition, *ast.Usage:
		return true
	default:
		return false
	}
}

func isType(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	_, definition := sym.Decl.(*ast.Definition)
	_, usage := sym.Decl.(*ast.Usage)
	return definition || usage || sym.Kind == symbols.SymbolKerMLType
}

func prefixesOf(node ast.Node) []*ast.PrefixMetadata {
	switch n := node.(type) {
	case *ast.Package:
		return n.Prefixes
	case *ast.Namespace:
		return n.Prefixes
	case *ast.Definition:
		return n.Prefixes
	case *ast.Usage:
		return n.Prefixes
	default:
		return nil
	}
}

func symbolsOfKind(sym *symbols.Symbol, kind symbols.SymbolKind) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, member := range ownedSymbols(sym, true) {
		if member != nil && member.Kind == kind {
			out = append(out, member)
		}
	}
	return out
}

func ownedMemberships(model *semantics.Model, sym *symbols.Symbol) []Membership {
	var out []Membership
	for _, member := range directOwnedSymbols(model, sym) {
		if member != nil {
			membership := ElementOf(member).Membership
			if hasMembership(membership) {
				out = append(out, membership)
			}
		}
	}
	return out
}

func ownedElementSymbols(model *semantics.Model, sym *symbols.Symbol) []*symbols.Symbol {
	if model == nil || sym == nil {
		return nil
	}
	members, ok := model.ReflectiveElements(sym, "ownedElement")
	if !ok {
		return nil
	}
	return members
}

func ownedFeatureSymbols(model *semantics.Model, sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, member := range directOwnedSymbols(model, sym) {
		if member != nil && member.IsFeature() {
			out = append(out, member)
		}
	}
	return out
}

func (e *Evaluator) inheritedSymbols(owner *symbols.Symbol, members []*symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, member := range members {
		if member != nil && member.Owner() != owner &&
			(e.resolver == nil || e.resolver.InheritedMemberVisible(owner, member)) {
			out = append(out, member)
		}
	}
	return out
}

func directOwnedSymbols(model *semantics.Model, sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	if sym.Scope != nil {
		for _, member := range sym.Scope.Members() {
			if member != nil && member.Owner() == sym && hasMembership(ElementOf(member).Membership) {
				out = appendUniqueSymbols(out, []*symbols.Symbol{member})
			}
		}
	}
	for _, member := range ownedSymbols(sym, true) {
		if member != nil && hasMembership(ElementOf(member).Membership) {
			out = appendUniqueSymbols(out, []*symbols.Symbol{member})
		}
	}
	seen := make(map[*symbols.Symbol]bool, len(out))
	for _, member := range out {
		seen[member] = true
	}
	for _, member := range ownedElementSymbols(model, sym) {
		if member != nil && member.Owner() == sym && hasMembership(ElementOf(member).Membership) && !seen[member] {
			seen[member] = true
			out = append(out, member)
		}
	}
	slices.SortStableFunc(out, func(left, right *symbols.Symbol) int {
		leftOffset, rightOffset := declarationOffset(left), declarationOffset(right)
		switch {
		case leftOffset < rightOffset:
			return -1
		case leftOffset > rightOffset:
			return 1
		default:
			return 0
		}
	})
	return out
}

func declarationOffset(sym *symbols.Symbol) int {
	if sym == nil || sym.Decl == nil {
		return int(^uint(0) >> 1)
	}
	return sym.Decl.Span().Offset
}

func hasMembership(membership Membership) bool {
	return membership.Member != nil || membership.Symbol != nil || membership.Node != nil
}

func multiplicityOf(sym *symbols.Symbol) ast.Node {
	if sym == nil {
		return nil
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		if d != nil && d.Multiplicity != nil {
			return d.Multiplicity
		}
	case *ast.Usage:
		if d != nil && d.Multiplicity != nil {
			return d.Multiplicity
		}
	case *ast.MultiplicityDecl:
		if d != nil && d.Range != nil {
			return d.Range
		}
	}
	return nil
}

func symbolSequence(symbolsList []*symbols.Symbol) Value {
	out := make([]Value, 0, len(symbolsList))
	for _, sym := range symbolsList {
		out = append(out, elementValue(ElementOf(sym)))
	}
	return sequence(out...)
}

func cloneValue(value Value) Value {
	if value.Kind == SequenceValue {
		value.Values = slices.Clone(value.Values)
		for i := range value.Values {
			value.Values[i] = cloneValue(value.Values[i])
		}
	}
	return value
}

func keyName(class, name string) string { return class + "::" + name }

func filterBool(value symbols.FilterValue) (bool, bool) {
	switch value.Kind {
	case symbols.FilterValueBool:
		return value.Bool, true
	}
	return false, false
}

func containsSymbol(symbolsList []*symbols.Symbol, candidate *symbols.Symbol) bool {
	for _, sym := range symbolsList {
		if sym == candidate {
			return true
		}
	}
	return false
}

func appendUniqueSymbols(dst, src []*symbols.Symbol) []*symbols.Symbol {
	for _, sym := range src {
		if sym != nil && !containsSymbol(dst, sym) {
			dst = append(dst, sym)
		}
	}
	return dst
}

func singleValue(values Value) (Value, bool) {
	if values.Kind != SequenceValue || len(values.Values) != 1 {
		return Value{}, false
	}
	return values.Values[0], true
}
