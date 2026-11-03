package metamodel

import (
	"slices"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

type propertyKey struct {
	element ElementKey
	class   string
	name    string
}

type cachedValue struct {
	value Value
	ok    bool
}

// Evaluator derives metamodel properties from a resolved semantic model.
type Evaluator struct {
	resolver                     *resolve.Resolver
	model                        *semantics.Model
	options                      Options
	mu                           sync.Mutex
	cache                        map[propertyKey]cachedValue
	reasons                      map[propertyKey]Reason
	internedStrings              map[string]string
	structureQualifiedNames      map[ElementKey]structureQualifiedNameCacheEntry
	structureQualifiedNameBases  map[ElementKey]structureQualifiedNameCacheEntry
	qualifiedNameNamespaceCaches map[ElementKey]structureQualifiedNameNamespaceCache
}

// New creates an evaluator over the resolver and semantic model for one model.
func New(res *resolve.Resolver, model *semantics.Model, options ...Options) *Evaluator {
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	return &Evaluator{
		resolver:                     res,
		model:                        model,
		options:                      opts,
		cache:                        make(map[propertyKey]cachedValue),
		reasons:                      make(map[propertyKey]Reason),
		internedStrings:              make(map[string]string),
		structureQualifiedNames:      make(map[ElementKey]structureQualifiedNameCacheEntry),
		structureQualifiedNameBases:  make(map[ElementKey]structureQualifiedNameCacheEntry),
		qualifiedNameNamespaceCaches: make(map[ElementKey]structureQualifiedNameNamespaceCache),
	}
}

func (e *Evaluator) recordUnsupportedReason(el Element, definingClass, name string, reason Reason) {
	if e == nil {
		return
	}
	key := propertyKey{element: el.Key(), class: definingClass, name: name}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, recorded := e.reasons[key]; !recorded {
		e.reasons[key] = reason
	}
}

func (e *Evaluator) unsupported(el Element, definingClass, name string, reason Reason) (Value, bool) {
	e.recordUnsupportedReason(el, definingClass, name, reason)
	return Value{}, false
}

// Explain returns a recorded cause without evaluating the property.
func (e *Evaluator) Explain(el Element, definingClass, name string) (Reason, bool) {
	if e == nil {
		return Reason{}, false
	}
	key := propertyKey{element: el.Key(), class: definingClass, name: name}
	e.mu.Lock()
	defer e.mu.Unlock()
	reason, ok := e.reasons[key]
	return reason, ok
}

// Property returns a faithfully derived property value when the evaluator serves it.
func (e *Evaluator) Property(el Element, definingClass, name string) (Value, bool) {
	return e.property(el, definingClass, name, true)
}

// PropertyReadOnly returns a derived value whose sequence is shared with the
// evaluator cache; callers must not modify Values or any nested sequence.
func (e *Evaluator) PropertyReadOnly(el Element, definingClass, name string) (Value, bool) {
	return e.property(el, definingClass, name, false)
}

func (e *Evaluator) property(el Element, definingClass, name string, clone bool) (Value, bool) {
	if e == nil {
		return Value{}, false
	}
	key := propertyKey{element: el.Key(), class: definingClass, name: name}
	e.mu.Lock()
	if cached, ok := e.cache[key]; ok {
		e.mu.Unlock()
		if clone {
			return cloneValue(cached.value), cached.ok
		}
		return cached.value, cached.ok
	}
	if _, served := ruleConstraints[keyName(definingClass, name)]; !served {
		if _, recorded := e.reasons[key]; !recorded {
			e.reasons[key] = Reason{Category: ReasonNoRule, Detail: keyName(definingClass, name)}
		}
		e.cache[key] = cachedValue{}
		e.mu.Unlock()
		return Value{}, false
	}
	if e.model == nil && e.options.Structure == nil {
		e.reasons[key] = Reason{Category: ReasonMissingSemanticModel, Detail: keyName(definingClass, name)}
		e.cache[key] = cachedValue{}
		e.mu.Unlock()
		return Value{}, false
	}
	e.mu.Unlock()
	value, ok := e.derive(el, definingClass, name)
	if !ok {
		e.recordUnsupportedReason(el, definingClass, name, Reason{
			Category: ReasonNoRule,
			Detail:   keyName(definingClass, name),
		})
	}
	value.Success = ok
	e.mu.Lock()
	if cached, found := e.cache[key]; found {
		value, ok = cached.value, cached.ok
		if clone {
			value = cloneValue(value)
		}
	} else {
		interned := e.internValueStrings(cloneValue(value))
		e.cache[key] = cachedValue{value: interned, ok: ok}
		if !clone {
			value = interned
		}
	}
	e.mu.Unlock()
	return value, ok
}

func (e *Evaluator) internValueStrings(value Value) Value {
	switch value.Kind {
	case StringValue:
		if stored, ok := e.internedStrings[value.String]; ok {
			value.String = stored
		} else {
			e.internedStrings[value.String] = value.String
		}
	case SequenceValue:
		for i := range value.Values {
			value.Values[i] = e.internValueStrings(value.Values[i])
		}
	}
	return value
}

func operatorSpelling(operator ast.OperatorKind) string {
	if operator == ast.OpIndex {
		return "["
	}
	return operator.String()
}

func (e *Evaluator) derive(el Element, class, name string) (Value, bool) {
	sym := el.Symbol
	libraryTypeCollection := class == "Type" && (name == "feature" || name == "input")
	if e.options.Structure == nil ||
		(libraryTypeCollection && sym != nil && e.resolver != nil &&
			e.resolver.Index() != nil && e.resolver.Index().Library(sym)) {
		if value, handled, ok := e.modelProperty(el, class, name); handled {
			return value, ok
		}
	}
	if value, handled, ok := e.structureProperty(el, class, name); handled {
		return value, ok
	}
	if structurePropertyRequired(class, name) {
		return e.unsupported(el, class, name, Reason{
			Category: ReasonMissingStructure,
			Detail:   keyName(class, name) + " has no supported Structure projection",
		})
	}
	if e.model == nil {
		return e.unsupported(el, class, name, Reason{
			Category: ReasonMissingSemanticModel,
			Detail:   keyName(class, name),
		})
	}
	if expected, ok := typedDefinitionMetaclasses[name]; ok {
		return e.typedDefinitionProperty(el, sym, expected, typedDefinitionMany[name], class, name)
	}
	switch class {
	case "Element":
		if name == "isLibraryElement" {
			value, supported := e.libraryElementProperty(el)
			if supported {
				return value, true
			}
			if e.resolver == nil || e.resolver.Index() == nil {
				return e.unsupported(el, class, name, Reason{
					Category: ReasonMissingSemanticModel,
					Detail:   "Element::isLibraryElement requires a resolver index",
				})
			}
			return e.unsupported(el, class, name, Reason{
				Category: ReasonUnrepresentedElement,
				Detail:   "Element::isLibraryElement has no semantic identity",
			})
		}
	case "Feature":
		switch name {
		case "direction":
			if sym == nil {
				return e.unsupported(el, class, name, Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   "Feature has no semantic symbol",
				})
			}
			direction, ok := e.featureDirectionOf(el)
			if !ok {
				return e.unsupported(el, class, name, Reason{
					Category: ReasonIncompleteStructure,
					Detail:   "feature direction is unavailable",
				})
			}
			if direction == ast.DirNone {
				return nullValue(), true
			}
			return enumValue(direction.String()), true
		case "isComposite":
			if sym == nil || e.model == nil {
				if isUsage, known := e.isMetaclass(el, "Usage"); known && isUsage {
					composite, handled, ok := e.structureUsageComposite(el)
					if handled && ok {
						return booleanValue(composite), true
					}
				}
				if e.options.Structure != nil {
					value, known := e.options.Structure.Attribute(el, "isComposite")
					if known && value.Kind == BooleanValue {
						return value, true
					}
				}
				return e.unsupported(el, class, name, Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   "Feature has no semantic symbol",
				})
			}
			return booleanValue(e.model.EffectiveIsComposite(sym)), true
		case "isVariable":
			if sym == nil {
				return e.unsupported(el, class, name, Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   "Feature has no semantic symbol",
				})
			}
			return booleanValue(e.model.FeatureIsVariable(sym)), true
		}
	case "Usage":
		if sym == nil {
			return e.unsupported(el, class, name, Reason{
				Category: ReasonUnrepresentedElement,
				Detail:   "Usage has no semantic symbol",
			})
		}
		switch name {
		case "isReference":
			return booleanValue(!e.model.EffectiveIsComposite(sym)), true
		case "mayTimeVary":
			return booleanValue(e.model.UsageMayTimeVary(sym)), true
		}
	case "InstantiationExpression":
		if name == "instantiatedType" {
			return e.instantiationExpressionProperty(el, name)
		}
	case "Import":
		return e.importProperty(el, name)
	}
	return e.unsupported(el, class, name, Reason{
		Category: ReasonNotApplicable,
		Detail:   keyName(class, name) + " rule does not apply to this element",
	})
}

func (e *Evaluator) modelProperty(element Element, class, name string) (Value, bool, bool) {
	sym := element.Symbol
	if sym == nil {
		return Value{}, false, false
	}
	unsupported := func(reason Reason) (Value, bool, bool) {
		value, ok := e.unsupported(element, class, name, reason)
		return value, true, ok
	}
	switch class {
	case "Element":
		switch name {
		case "name":
			if element.IsMembership {
				return nullValue(), true, true
			}
			if value := e.model.EffectiveNameOf(sym); value != "" {
				return stringValue(value), true, true
			}
			return nullValue(), true, true
		case "shortName":
			if element.IsMembership {
				return nullValue(), true, true
			}
			if sym.Naming == symbols.NamedByRedefinition && !e.symbolNamingSourceResolved(sym) {
				return unsupported(Reason{
					Category: ReasonUnresolvedReference,
					Detail:   "Element::shortName naming redefinition target is unresolved",
				})
			}
			if value := e.model.EffectiveShortNameOf(sym); value != "" {
				return stringValue(value), true, true
			}
			return nullValue(), true, true
		case "owner":
			return ownerValue(sym.Owner()), true, true
		case "owningNamespace":
			namespace, ok := e.modelOwningNamespace(sym)
			if !ok {
				return unsupported(Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   "Element::owningNamespace has an unresolved owner",
				})
			}
			if namespace == nil {
				return nullValue(), true, true
			}
			return elementValue(ElementOf(namespace)), true, true
		case "qualifiedName":
			value, ok := e.modelQualifiedName(sym, make(map[*symbols.Symbol]bool))
			if !ok {
				return unsupported(Reason{
					Category: ReasonIncompleteStructure,
					Detail:   "Element::qualifiedName ownership chain is incomplete or cyclic",
				})
			}
			return value, true, ok
		}
	case "Type":
		switch name {
		case "input":
			isPortUsage, known := e.isMetaclass(element, "PortUsage")
			if known && isPortUsage {
				return e.structureDirectedFeatures(element, name)
			}
			isSendAction, known := e.isMetaclass(element, "SendActionUsage")
			if known && isSendAction {
				value, handled, ok := e.structureDirectedFeatures(element, name)
				if handled {
					return value, true, ok
				}
			}
			if !e.completeMembers(sym) {
				return unsupported(e.structureMemberCompletenessReason(sym))
			}
			return symbolSequence(e.model.InputParametersOf(sym)), true, true
		case "output":
			isPortUsage, known := e.isMetaclass(element, "PortUsage")
			if known && isPortUsage {
				return e.structureDirectedFeatures(element, name)
			}
		case "feature":
			if !e.completeMembers(sym) {
				return unsupported(e.structureMemberCompletenessReason(sym))
			}
			features, ok := e.model.ReflectiveElements(sym, "feature")
			if !ok {
				return unsupported(Reason{
					Category: ReasonIncompleteStructure,
					Detail:   "Type::feature semantic projection is unavailable",
				})
			}
			return symbolSequence(features), true, true
		case "inheritedMembership":
			if !e.completeMembers(sym) {
				return unsupported(e.structureMemberCompletenessReason(sym))
			}
			members, ok := e.model.ReflectiveElements(sym, "member")
			if !ok {
				return unsupported(Reason{
					Category: ReasonIncompleteStructure,
					Detail:   "Type::inheritedMembership semantic projection is unavailable",
				})
			}
			return membershipSequence(memberships(e.inheritedSymbols(sym, members))), true, true
		case "featureMembership":
			if !e.completeMembers(sym) {
				return unsupported(e.structureMemberCompletenessReason(sym))
			}
			features, ok := e.model.ReflectiveElements(sym, "feature")
			if !ok {
				return unsupported(Reason{
					Category: ReasonIncompleteStructure,
					Detail:   "Type::featureMembership semantic projection is unavailable",
				})
			}
			var membershipsForFeatures []Membership
			for _, feature := range features {
				if membership := ElementOf(feature).Membership; membership.Feature {
					membershipsForFeatures = append(membershipsForFeatures, membership)
				}
			}
			return membershipSequence(membershipsForFeatures), true, true
		}
	case "Feature":
		switch name {
		case "type":
			types, ok := e.featureTypes(sym)
			if !ok {
				return unsupported(Reason{
					Category: ReasonIncompleteStructure,
					Detail:   "Feature::type relationships are incomplete",
				})
			}
			return symbolSequence(types), true, true
		case "direction":
			direction := e.model.DeclaredDirection(sym)
			if direction == ast.DirNone {
				membership := ElementOf(sym).Membership
				if membership.kind == "ReturnParameterMembership" {
					direction = ast.DirOut
				} else if membership.kind == "ParameterMembership" || membership.kind == "ActorMembership" ||
					membership.kind == "SubjectMembership" || membership.kind == "StakeholderMembership" {
					direction = ast.DirIn
				}
			}
			if direction == ast.DirNone {
				return nullValue(), true, true
			}
			return enumValue(direction.String()), true, true
		case "isComposite":
			return booleanValue(e.model.EffectiveIsComposite(sym)), true, true
		case "featuringType":
			var values []Value
			seen := make(map[*symbols.Symbol]bool)
			for _, relationship := range semantics.RelationshipsOf(sym) {
				if relationship == nil || relationship.Kind != ast.RelFeaturedBy {
					continue
				}
				typ := e.model.RelationshipTarget(sym, relationship)
				if typ == nil {
					return unsupported(Reason{
						Category: ReasonUnresolvedReference,
						Detail:   "featured-by relationship target is unresolved",
					})
				}
				if !seen[typ] {
					seen[typ] = true
					values = append(values, elementValue(ElementOf(typ)))
				}
			}
			owner := sym.Owner()
			if owner != nil {
				isType, known := e.isMetaclass(ElementOf(owner), "Type")
				if !known {
					return unsupported(Reason{
						Category: ReasonUnrepresentedElement,
						Detail:   "Feature::featuringType owner has no represented metaclass",
					})
				}
				if isType && !seen[owner] {
					values = append(values, elementValue(ElementOf(owner)))
				}
			}
			return sequence(values...), true, true
		}
	case "Usage":
		if name == "definition" {
			value, ok := e.typedDefinitionProperty(
				element, sym, "Classifier", true, class, name,
			)
			return value, true, ok
		}
	}
	return Value{}, false, false
}

func (e *Evaluator) modelOwningNamespace(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	if e.model == nil {
		return nil, false
	}
	for owner := sym.Owner(); owner != nil; owner = owner.Owner() {
		if owner.Kind == symbols.SymbolPackage {
			return owner, true
		}
		isNamespace, known := e.isMetaclass(ElementOf(owner), "Namespace")
		if !known {
			return nil, false
		}
		if isNamespace {
			return owner, true
		}
	}
	return nil, true
}

func (e *Evaluator) modelQualifiedName(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) (Value, bool) {
	if e.model == nil || sym == nil {
		return Value{}, false
	}
	if seen[sym] {
		return Value{}, false
	}
	seen[sym] = true
	name := e.model.EffectiveNameOf(sym)
	if name == "" {
		return nullValue(), true
	}
	if sym.OwnerScope != nil {
		for _, member := range sym.OwnerScope.Members() {
			if member == sym {
				break
			}
			if e.model.EffectiveNameOf(member) == name {
				return nullValue(), true
			}
		}
	}
	parts := []string{identity.EscapeName(name)}
	for namespace, ok := e.modelOwningNamespace(sym); namespace != nil && ok; namespace, ok = e.modelOwningNamespace(namespace) {
		if seen[namespace] {
			return Value{}, false
		}
		seen[namespace] = true
		if namespaceName := e.model.EffectiveNameOf(namespace); namespaceName != "" {
			parts = append(parts, identity.EscapeName(namespaceName))
		}
	}
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	return stringValue(strings.Join(parts, "::")), true
}

func (e *Evaluator) libraryElementProperty(element Element) (Value, bool) {
	if e.resolver == nil || e.resolver.Index() == nil {
		return Value{}, false
	}
	if element.Aspect == "api-root" || element.IsMembership && element.Membership.Aspect == "api-root" {
		return booleanValue(false), true
	}
	if element.Symbol != nil {
		return booleanValue(e.resolver.Index().Library(element.Symbol)), true
	}
	if element.IsMembership {
		candidate := element.Membership.Member
		if candidate == nil {
			candidate = element.Membership.Owner
		}
		if candidate != nil {
			return booleanValue(e.resolver.Index().Library(candidate)), true
		}
		if element.Membership.Node != nil || element.Membership.Aspect != "" {
			return booleanValue(false), true
		}
	}
	if element.Container != nil {
		return booleanValue(e.resolver.Index().Library(element.Container)), true
	}
	if element.Node != nil || element.Aspect != "" {
		return booleanValue(false), true
	}
	return Value{}, false
}

func structurePropertyRequired(class, name string) bool {
	if strings.HasPrefix(name, "owned") || strings.HasPrefix(name, "owning") ||
		strings.HasPrefix(name, "nested") {
		return true
	}
	switch name {
	case "name", "shortName", "qualifiedName", "owner",
		"ownedElement", "ownedRelationship", "ownedAnnotation", "documentation",
		"textualRepresentation", "ownedMember", "member", "membership",
		"ownedMembership", "ownedImport", "importedMembership", "feature",
		"featureMembership", "inheritedFeature", "inheritedMembership",
		"input", "output", "directedFeature", "endFeature", "multiplicity",
		"type", "definition", "behavior", "association", "interaction", "result",
		"function",
		"isConjugated",
		"mayTimeVary", "memberElement", "memberName", "memberShortName",
		"membershipOwningNamespace", "source", "target", "relatedElement",
		"ownedRelatedElement", "owningRelatedElement", "action", "ownedConstraint",
		"chainingFeature", "featureTarget", "featuringType", "crossFeature", "annotation",
		"annotatedElement", "annotatingElement", "documentedElement", "expression",
		"parameter", "step", "connectorEnd", "relatedFeature", "sourceFeature",
		"targetFeature", "isModelLevelEvaluable", "isVariable",
		"lowerBound", "upperBound", "bound", "triggerAction", "guardExpression",
		"effectAction", "actorParameter", "subjectParameter", "objectiveRequirement",
		"stakeholderParameter", "framedConcern", "assumedConstraint", "requiredConstraint",
		"state", "doAction", "entryAction", "exitAction":
		return true
	}
	switch class {
	case "Namespace", "Type", "Definition", "Membership", "OwningMembership",
		"FeatureMembership", "VariantMembership", "ParameterMembership",
		"ReturnParameterMembership", "EndFeatureMembership", "Relationship",
		"Specialization", "FeatureTyping", "Subsetting", "Redefinition",
		"FeatureChaining", "CrossSubsetting", "FeatureInverting", "TypeFeaturing",
		"Conjugation", "Disjoining", "Differencing", "Intersecting", "Unioning",
		"Subclassification", "Import", "Annotation", "AnnotatingElement",
		"Documentation", "FeatureValue", "Dependency", "ReferenceSubsetting",
		"PortConjugation", "ConjugatedPortTyping", "CaseDefinition", "CaseUsage",
		"RequirementDefinition", "RequirementUsage", "StateDefinition", "StateUsage",
		"TransitionUsage", "Flow", "MultiplicityRange", "Classifier":
		return true
	}
	if class == "Step" && name == "parameter" || class == "Behavior" && (name == "step" || name == "parameter") {
		return true
	}
	return false
}

func (e *Evaluator) importProperty(el Element, name string) (Value, bool) {
	if name != "importedElement" {
		return e.unsupported(el, "Import", name, Reason{
			Category: ReasonNotApplicable,
			Detail:   "Import::" + name + " has no evaluator rule",
		})
	}
	imp, ok := el.Node.(*ast.Import)
	if !ok || imp == nil {
		return e.unsupported(el, "Import", name, Reason{
			Category: ReasonUnrepresentedElement,
			Detail:   "Import::importedElement has no syntax-backed import",
		})
	}
	if el.Container == nil || el.Container.Scope == nil {
		return e.unsupported(el, "Import", name, Reason{
			Category: ReasonMissingStructure,
			Detail:   "Import::importedElement has no containing namespace",
		})
	}
	if e.resolver == nil {
		return e.unsupported(el, "Import", name, Reason{
			Category: ReasonMissingSemanticModel,
			Detail:   "Import::importedElement requires a resolver",
		})
	}
	imported, ok := e.resolver.ImportTarget(el.Container.Scope, imp)
	if !ok || imported == nil ||
		!e.importTreeComplete(imported, imp, make(map[*symbols.Scope]bool)) {
		return e.unsupported(el, "Import", name, Reason{
			Category: ReasonUnresolvedReference,
			Detail:   "Import::importedElement target is unresolved or incomplete",
		})
	}
	return elementValue(ElementOf(imported)), true
}

func ownerValue(owner *symbols.Symbol) Value {
	if owner == nil {
		return nullValue()
	}
	return elementValue(ElementOf(owner))
}

func (e *Evaluator) isMetaclass(element Element, expected string) (bool, bool) {
	actual := ""
	ok := false
	if element.Symbol != nil && e.resolver != nil && e.resolver.Index() != nil &&
		e.resolver.Index().Library(element.Symbol) && e.model != nil {
		if metaclass := e.model.MetaclassOf(element.Symbol); metaclass != nil {
			actual, ok = metaclass.Name, true
		}
	}
	if e.options.Structure != nil {
		if !ok {
			actual, ok = e.options.Structure.Metaclass(element)
		}
	}
	if !ok && element.Symbol != nil && e.model != nil {
		if metaclass := e.model.MetaclassOf(element.Symbol); metaclass != nil {
			actual, ok = metaclass.Name, true
		}
	}
	if !ok && element.IsMembership && element.Membership.kind != "" {
		actual, ok = element.Membership.kind, true
	}
	if !ok || actual == "" {
		return false, false
	}
	if strings.HasPrefix(actual, ExtensionMetaclassPrefix) {
		actual = strings.TrimPrefix(actual, ExtensionMetaclassPrefix)
		return actual == expected, actual != ""
	}
	if declared, ok := kermlTypeUsageMetaclasses[actual]; ok {
		actual = declared
	}
	if actual == expected {
		return true, true
	}
	if e.options.Structure != nil {
		return e.options.Structure.Specializes(actual, expected), true
	}
	if element.IsMembership {
		return membershipMetaclassConforms(element.Membership, expected), true
	}
	if element.Symbol != nil && e.model != nil {
		actualMetaclass := e.model.MetaclassOf(element.Symbol)
		expectedMetaclass := e.model.Metaclass(expected)
		if actualMetaclass != nil && expectedMetaclass != nil {
			return e.model.Conforms(actualMetaclass, expectedMetaclass), true
		}
	}
	return false, false
}

func membershipMetaclassConforms(membership Membership, expected string) bool {
	if membership.kind == expected {
		return true
	}
	switch expected {
	case "Membership", "Relationship":
		return membership.kind != ""
	case "FeatureMembership":
		return membership.Feature
	case "ParameterMembership":
		switch membership.kind {
		case "ActorMembership", "ParameterMembership", "ReturnParameterMembership", "StakeholderMembership", "SubjectMembership":
			return true
		}
	case "OwningMembership":
		return membership.kind != "Alias"
	}
	return false
}

// kermlTypeUsageMetaclasses maps the metaclass names a graph records KerML type
// declarations under to the KerML metaclass each declaration builds.
var kermlTypeUsageMetaclasses = map[string]string{
	"AssociationUsage": "Association",
	"BehaviorUsage":    "Behavior",
	"ClassUsage":       "Class",
	"InteractionUsage": "Interaction",
	"PredicateUsage":   "Predicate",
	"StructureUsage":   "Structure",
}

func (e *Evaluator) ownedUsageMetaclass(property string) (string, bool) {
	var prefix string
	if strings.HasPrefix(property, "nested") {
		prefix = "nested"
	} else if strings.HasPrefix(property, "owned") {
		prefix = "owned"
	} else {
		return "", false
	}
	suffix := strings.TrimPrefix(property, prefix)
	metaclasses := map[string]string{
		"Usage": "Usage", "Reference": "ReferenceUsage", "Attribute": "AttributeUsage",
		"Enumeration": "EnumerationUsage", "Occurrence": "OccurrenceUsage", "Item": "ItemUsage",
		"Part": "PartUsage", "Port": "PortUsage", "Connection": "ConnectorAsUsage",
		"Flow": "FlowUsage", "Interface": "InterfaceUsage", "Allocation": "AllocationUsage",
		"Action": "ActionUsage", "State": "StateUsage", "Transition": "TransitionUsage",
		"Calculation": "CalculationUsage", "Constraint": "ConstraintUsage",
		"Requirement": "RequirementUsage", "Concern": "ConcernUsage", "Case": "CaseUsage",
		"AnalysisCase": "AnalysisCaseUsage", "VerificationCase": "VerificationCaseUsage",
		"UseCase": "UseCaseUsage", "View": "ViewUsage", "Viewpoint": "ViewpointUsage",
		"Rendering": "RenderingUsage", "Metadata": "MetadataUsage",
	}
	metaclass, ok := metaclasses[suffix]
	return metaclass, ok
}

func (e *Evaluator) completeSupertypes(sym *symbols.Symbol) bool {
	if !e.completeRelationships(sym) {
		return false
	}
	if e.model.SupertypesProvisional(sym) || e.model.HasSpecializationCycle(sym) {
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
	if !e.completeFeatureTypeInputs(sym) {
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

func (e *Evaluator) completeFeatureTypeInputs(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	seen := make(map[*symbols.Symbol]bool)
	queue := []*symbols.Symbol{sym}
	for len(queue) > 0 {
		candidate := queue[0]
		queue = queue[1:]
		if candidate == nil || seen[candidate] {
			continue
		}
		seen[candidate] = true
		if e.model.SupertypesProvisional(candidate) ||
			e.model.HasSpecializationCycle(candidate) ||
			!e.completeRelationships(candidate) {
			return false
		}
		for _, relationship := range semantics.RelationshipsOf(candidate) {
			if relationship == nil || relationship.Kind == ast.RelTyping ||
				!semantics.GeneralizationKind(relationship.Kind) {
				continue
			}
			target := e.model.RelationshipTarget(candidate, relationship)
			if target == nil {
				return false
			}
			if target.IsFeature() {
				queue = append(queue, target)
			}
		}
	}
	return true
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

func (e *Evaluator) featureDirection(element Element) (ast.FeatureDirection, bool) {
	sym := element.Symbol
	if sym != nil {
		if direction := e.model.DeclaredDirection(sym); direction != ast.DirNone {
			return direction, true
		}
	} else if usage, ok := element.Node.(*ast.Usage); ok && usage.Direction != ast.DirNone {
		return usage.Direction, true
	}
	membership := element.Membership
	if !hasMembership(membership) && sym != nil {
		membership = ElementOf(sym).Membership
	}
	if !hasMembership(membership) {
		if usage, ok := element.Node.(*ast.Usage); ok && (usage.IsBodyParameter || usage.IsResult) {
			return ast.DirNone, false
		}
		return ast.DirNone, true
	}
	isReturn, ok := e.isMetaclass(MembershipElement(membership), "ReturnParameterMembership")
	if !ok {
		return ast.DirNone, false
	}
	if isReturn {
		return ast.DirOut, true
	}
	isParameter, ok := e.isMetaclass(MembershipElement(membership), "ParameterMembership")
	if !ok {
		return ast.DirNone, false
	}
	if isParameter {
		return ast.DirIn, true
	}
	return ast.DirNone, true
}

func (e *Evaluator) featureDirectionOf(el Element) (ast.FeatureDirection, bool) {
	return e.featureDirection(el)
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
		childTarget, ok := e.resolver.ImportTarget(scope, child)
		if !ok || childTarget == nil || !e.importTreeComplete(childTarget, child, seen) {
			return false
		}
	}
	return true
}

var typedDefinitionMetaclasses = map[string]string{
	"actionDefinition":           "Behavior",
	"allocationDefinition":       "AllocationDefinition",
	"analysisCaseDefinition":     "AnalysisCaseDefinition",
	"attributeDefinition":        "DataType",
	"calculationDefinition":      "Function",
	"caseDefinition":             "CaseDefinition",
	"concernDefinition":          "ConcernDefinition",
	"connectionDefinition":       "AssociationStructure",
	"constraintDefinition":       "Predicate",
	"enumerationDefinition":      "EnumerationDefinition",
	"flowDefinition":             "Interaction",
	"interfaceDefinition":        "InterfaceDefinition",
	"itemDefinition":             "Structure",
	"metadataDefinition":         "Metaclass",
	"occurrenceDefinition":       "Class",
	"individualDefinition":       "OccurrenceDefinition",
	"partDefinition":             "PartDefinition",
	"portDefinition":             "PortDefinition",
	"renderingDefinition":        "RenderingDefinition",
	"requirementDefinition":      "RequirementDefinition",
	"stateDefinition":            "Behavior",
	"useCaseDefinition":          "UseCaseDefinition",
	"verificationCaseDefinition": "VerificationCaseDefinition",
	"viewDefinition":             "ViewDefinition",
	"viewpointDefinition":        "ViewpointDefinition",
}

var typedDefinitionMany = map[string]bool{
	"actionDefinition":     true,
	"allocationDefinition": true,
	"attributeDefinition":  true,
	"connectionDefinition": true,
	"flowDefinition":       true,
	"interfaceDefinition":  true,
	"itemDefinition":       true,
	"occurrenceDefinition": true,
	"partDefinition":       true,
	"portDefinition":       true,
	"stateDefinition":      true,
}

func (e *Evaluator) typedDefinitionProperty(
	element Element,
	sym *symbols.Symbol,
	expected string,
	many bool,
	class, name string,
) (Value, bool) {
	unsupported := func(reason Reason) (Value, bool) {
		return e.unsupported(element, class, name, reason)
	}
	if sym == nil || !sym.IsFeature() {
		return unsupported(Reason{
			Category: ReasonUnrepresentedElement,
			Detail:   class + "::" + name + " has no semantic feature",
		})
	}
	types, ok := e.featureTypes(sym)
	if !ok {
		return unsupported(Reason{
			Category:   ReasonDependsOnUnsupported,
			Dependency: "Feature::type",
		})
	}
	var definitions []*symbols.Symbol
	for _, typ := range types {
		if typ == nil {
			return unsupported(Reason{
				Category: ReasonUnrepresentedElement,
				Detail:   class + "::" + name + " has an unrepresented type target",
			})
		}
		matches, ok := e.isMetaclass(ElementOf(typ), expected)
		if !ok {
			return unsupported(Reason{
				Category: ReasonUnrepresentedElement,
				Detail:   class + "::" + name + " type target has no represented metaclass",
			})
		}
		if matches {
			definitions = append(definitions, typ)
		}
	}
	if len(definitions) > 1 && !many {
		return unsupported(Reason{
			Category: ReasonIncompleteStructure,
			Detail:   class + "::" + name + " has multiple targets for a single-valued property",
		})
	}
	if many {
		values := make([]Value, 0, len(definitions))
		for _, definition := range definitions {
			values = append(values, elementValue(ElementOf(definition)))
		}
		return sequence(values...), true
	}
	if len(definitions) == 0 {
		return nullValue(), true
	}
	return elementValue(ElementOf(definitions[0])), true
}

func (e *Evaluator) instantiationExpressionProperty(element Element, name string) (Value, bool) {
	unsupported := func(reason Reason) (Value, bool) {
		return e.unsupported(element, "InstantiationExpression", name, reason)
	}
	if name != "instantiatedType" {
		return unsupported(Reason{
			Category: ReasonNotApplicable,
			Detail:   "InstantiationExpression::" + name + " has no instantiation rule",
		})
	}
	expressionNode := expressionNodeOf(element)
	if operator, ok := expressionNode.(*ast.OperatorExpr); ok {
		if e.model == nil {
			return unsupported(Reason{
				Category: ReasonMissingSemanticModel,
				Detail:   "operator function lookup requires a semantic model",
			})
		}
		function := e.model.OperatorFunction(operator.Operator)
		if function == nil || !semantics.IsType(function) {
			return e.unresolvedInstantiationOperator(element, operatorSpelling(operator.Operator))
		}
		return elementValue(ElementOf(function)), true
	}
	if index, ok := expressionNode.(*ast.IndexExpr); ok && index.Bracket {
		if e.model == nil {
			return unsupported(Reason{
				Category: ReasonMissingSemanticModel,
				Detail:   "operator function lookup requires a semantic model",
			})
		}
		function := e.model.OperatorFunction(ast.OpIndex)
		if function == nil || !semantics.IsType(function) {
			return e.unresolvedInstantiationOperator(element, "[")
		}
		return elementValue(ElementOf(function)), true
	}
	if _, ok := expressionNode.(*ast.SequenceExpr); ok {
		if e.model == nil {
			return unsupported(Reason{
				Category: ReasonMissingSemanticModel,
				Detail:   "operator function lookup requires a semantic model",
			})
		}
		function := e.model.OperatorFunctionNamed(",")
		if function == nil || !semantics.IsType(function) {
			return e.unresolvedInstantiationOperator(element, ",")
		}
		return elementValue(ElementOf(function)), true
	}
	if _, ok := expressionNode.(*ast.CastExpr); ok {
		if e.model == nil {
			return unsupported(Reason{
				Category: ReasonMissingSemanticModel,
				Detail:   "operator function lookup requires a semantic model",
			})
		}
		function := e.model.OperatorFunction(ast.OpAs)
		if function == nil || !semantics.IsType(function) {
			return e.unresolvedInstantiationOperator(element, "as")
		}
		return elementValue(ElementOf(function)), true
	}
	if _, ok := expressionNode.(*ast.FeatureChainExpr); ok {
		if e.model == nil {
			return unsupported(Reason{
				Category: ReasonMissingSemanticModel,
				Detail:   "operator function lookup requires a semantic model",
			})
		}
		function := e.model.OperatorFunctionNamed(".")
		if function == nil || !semantics.IsType(function) {
			return e.unresolvedInstantiationOperator(element, ".")
		}
		return elementValue(ElementOf(function)), true
	}
	if isTrigger, known := e.isMetaclass(element, "TriggerInvocationExpression"); known && isTrigger {
		if e.options.Structure == nil {
			return unsupported(Reason{
				Category: ReasonMissingStructure,
				Detail:   "TriggerInvocationExpression::kind is unavailable without Structure",
			})
		}
		if e.resolver == nil || e.model == nil {
			return unsupported(Reason{
				Category: ReasonMissingSemanticModel,
				Detail:   "trigger type lookup requires a resolver and semantic model",
			})
		}
		kind, ok := e.options.Structure.Attribute(element, "kind")
		if !ok || kind.Kind != StringValue {
			return unsupported(Reason{
				Category: ReasonMissingStructure,
				Detail:   "TriggerInvocationExpression::kind is unavailable",
			})
		}
		triggerType := map[string]string{
			"when":  "TriggerWhen",
			"at":    "TriggerAt",
			"after": "TriggerAfter",
		}[kind.String]
		if triggerType == "" {
			return unsupported(Reason{
				Category: ReasonNotApplicable,
				Detail:   "TriggerInvocationExpression has an unsupported kind",
			})
		}
		var scope *symbols.Scope
		if element.Container != nil {
			scope = element.Container.Scope
		}
		if scope == nil && element.Symbol != nil {
			if owner := element.Symbol.Owner(); owner != nil {
				scope = owner.Scope
			}
		}
		name := ast.QualifiedNameOf("Triggers", triggerType)
		name.Global = true
		target, ok := e.resolver.ResolveQualified(scope, name)
		if !ok || target == nil || !semantics.IsType(target) {
			return unsupported(Reason{
				Category: ReasonUnresolvedReference,
				Detail:   "Triggers::" + triggerType + " is unresolved",
			})
		}
		return elementValue(ElementOf(target)), true
	}
	if e.options.Structure != nil {
		if invocation, ok := expressionNode.(*ast.InvocationExpr); ok {
			if callee := semantics.ChainCallee(invocation); callee != nil {
				chain := Element{Node: callee, Container: element.Container}
				target, handled, ok := e.structureFeatureReferenceReferent(chain)
				if handled && ok && target.Kind == ElementValue {
					function, handled, ok := e.structureTypedFeatureTypes(target.Element, "Function", true)
					if handled && ok && function.Kind == ElementValue {
						return function, true
					}
				}
			}
		}
		memberships, handled, ok := e.structureOwnedFilter(element, "Membership", true)
		if !handled || !ok || memberships.Kind != SequenceValue {
			return unsupported(Reason{
				Category: ReasonMissingStructure,
				Detail:   "InvocationExpression memberships are unavailable",
			})
		}
		for _, value := range memberships.Values {
			if value.Kind != MembershipValue {
				return unsupported(Reason{
					Category: ReasonIncompleteStructure,
					Detail:   "InvocationExpression has a non-membership owned value",
				})
			}
			membership := MembershipElement(value.Membership)
			isFeatureMembership, known := e.isMetaclass(membership, "FeatureMembership")
			if !known {
				return unsupported(Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   "InvocationExpression membership has no represented metaclass",
				})
			}
			if isFeatureMembership {
				continue
			}
			members, ok := e.structureMembershipMembers(membership)
			if !ok {
				return unsupported(Reason{
					Category: ReasonMissingStructure,
					Detail:   "InvocationExpression membership has no represented member",
				})
			}
			if len(members) == 0 {
				return unsupported(Reason{
					Category: ReasonIncompleteStructure,
					Detail:   "InvocationExpression has an empty non-feature membership",
				})
			}
			isType, known := e.isMetaclass(members[0], "Type")
			if !known {
				return unsupported(Reason{
					Category: ReasonUnrepresentedElement,
					Detail:   "InvocationExpression target has no represented metaclass",
				})
			}
			if !isType {
				return unsupported(Reason{
					Category: ReasonNotApplicable,
					Detail:   "InvocationExpression membership target is not a Type",
				})
			}
			return elementValue(members[0]), true
		}
	}
	if e.resolver == nil || e.model == nil {
		return unsupported(Reason{
			Category: ReasonMissingSemanticModel,
			Detail:   "instantiation target lookup requires a resolver and semantic model",
		})
	}
	if element.Container == nil || element.Container.Scope == nil {
		return unsupported(Reason{
			Category: ReasonMissingStructure,
			Detail:   "instantiation target lookup has no containing namespace",
		})
	}
	var callee *ast.QualifiedName
	switch expression := expressionNode.(type) {
	case *ast.ConstructorExpr:
		callee = expression.Type
	case *ast.InvocationExpr:
		callee = expression.Type
	default:
		return unsupported(Reason{
			Category: ReasonNotApplicable,
			Detail:   "expression has no instantiation target syntax",
		})
	}
	if callee == nil {
		return unsupported(Reason{
			Category: ReasonUnresolvedReference,
			Detail:   "instantiation target name is missing",
		})
	}
	target, ok := e.resolver.ResolveTarget(element.Container.Scope, callee)
	if !ok || target == nil {
		return unsupported(Reason{
			Category: ReasonUnresolvedReference,
			Detail:   "instantiation target reference is unresolved",
		})
	}
	if !semantics.IsType(target) {
		return unsupported(Reason{
			Category: ReasonNotApplicable,
			Detail:   "instantiation target is not a Type",
		})
	}
	return elementValue(ElementOf(target)), true
}

func (e *Evaluator) unresolvedInstantiationOperator(element Element, spelling string) (Value, bool) {
	return e.unsupported(element, "InstantiationExpression", "instantiatedType", Reason{
		Category: ReasonUnresolvedOperatorFunction,
		Detail:   spelling,
	})
}

func expressionNodeOf(element Element) ast.Node {
	switch node := element.Node.(type) {
	case *ast.ConstraintMember:
		if node.Expression != nil {
			return node.Expression
		}
	}
	return element.Node
}

func membershipSequence(members []Membership) Value {
	values := make([]Value, 0, len(members))
	for _, member := range members {
		values = append(values, membershipValue(member))
	}
	return sequence(values...)
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

func isType(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	_, definition := sym.Decl.(*ast.Definition)
	_, usage := sym.Decl.(*ast.Usage)
	return definition || usage || sym.Kind == symbols.SymbolKerMLType
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

func hasMembership(membership Membership) bool {
	return membership.Member != nil || membership.Symbol != nil || membership.Node != nil
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
