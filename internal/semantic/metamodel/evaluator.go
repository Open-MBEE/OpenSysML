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
	resolver *resolve.Resolver
	model    *semantics.Model
	options  Options
	mu       sync.Mutex
	cache    map[propertyKey]cachedValue
}

// New creates an evaluator over the resolver and semantic model for one model.
func New(res *resolve.Resolver, model *semantics.Model, options ...Options) *Evaluator {
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	return &Evaluator{
		resolver: res,
		model:    model,
		options:  opts,
		cache:    make(map[propertyKey]cachedValue),
	}
}

// Property returns a faithfully derived property value when the evaluator serves it.
func (e *Evaluator) Property(el Element, definingClass, name string) (Value, bool) {
	if e == nil || e.model == nil {
		return Value{}, false
	}
	key := propertyKey{element: el.Key(), class: definingClass, name: name}
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
	if value, handled, ok := e.structureProperty(el, class, name); handled {
		return value, ok
	}
	if structurePropertyRequired(class, name) {
		return Value{}, false
	}
	if expected, ok := typedDefinitionMetaclasses[name]; ok {
		return e.typedDefinitionProperty(sym, expected, typedDefinitionMany[name])
	}
	switch class {
	case "Element":
		if name == "isLibraryElement" {
			return e.libraryElementProperty(el)
		}
	case "Feature":
		if name == "direction" && sym != nil {
			direction, ok := e.featureDirectionOf(el)
			if !ok {
				return Value{}, false
			}
			if direction == ast.DirNone {
				return nullValue(), true
			}
			return enumValue(direction.String()), true
		}
	case "InstantiationExpression":
		if name == "instantiatedType" {
			return e.instantiationExpressionProperty(el, name)
		}
	case "Import":
		return e.importProperty(el, name)
	}
	return Value{}, false
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
		"isConjugated", "isVariable", "isComposite", "isReference",
		"mayTimeVary", "memberElement", "memberName", "memberShortName",
		"membershipOwningNamespace", "source", "target", "relatedElement",
		"ownedRelatedElement", "owningRelatedElement", "action", "ownedConstraint",
		"chainingFeature", "featureTarget", "featuringType", "crossFeature", "annotation",
		"annotatedElement", "annotatingElement", "documentedElement", "expression",
		"parameter", "step", "connectorEnd", "relatedFeature", "sourceFeature",
		"targetFeature", "isModelLevelEvaluable",
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
		return Value{}, false
	}
	imp, ok := el.Node.(*ast.Import)
	if !ok || imp == nil || el.Container == nil || el.Container.Scope == nil || e.resolver == nil {
		return Value{}, false
	}
	imported, ok := e.resolver.ImportTarget(el.Container.Scope, imp)
	if !ok || imported == nil ||
		!e.importTreeComplete(imported, imp, make(map[*symbols.Scope]bool)) {
		return Value{}, false
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
	if e.options.Structure == nil {
		return false, false
	}
	actual, ok := e.options.Structure.Metaclass(element)
	if !ok || actual == "" {
		return false, false
	}
	if actual == expected {
		return true, true
	}
	return e.options.Structure.Specializes(actual, expected), true
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

func isEndFeature(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.IsEnd {
		return true
	}
	return sym.Kind == symbols.SymbolConnectorEnd
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

func (e *Evaluator) typedDefinitionProperty(sym *symbols.Symbol, expected string, many bool) (Value, bool) {
	if sym == nil || !sym.IsFeature() {
		return Value{}, false
	}
	types, ok := e.featureTypes(sym)
	if !ok {
		return Value{}, false
	}
	var definitions []*symbols.Symbol
	for _, typ := range types {
		if typ == nil {
			return Value{}, false
		}
		matches, ok := e.isMetaclass(ElementOf(typ), expected)
		if !ok {
			return Value{}, false
		}
		if matches {
			definitions = append(definitions, typ)
		}
	}
	if len(definitions) > 1 && !many {
		return Value{}, false
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
	constructor, ok := element.Node.(*ast.ConstructorExpr)
	if !ok || name != "instantiatedType" || constructor.Type == nil ||
		element.Container == nil || element.Container.Scope == nil || e.resolver == nil {
		return Value{}, false
	}
	target, ok := e.resolver.ResolveTarget(element.Container.Scope, constructor.Type)
	if !ok || target == nil {
		return Value{}, false
	}
	return elementValue(ElementOf(target)), true
}

func flowEndOwnedFeature(end Element) Element {
	aspect := "end1/ff"
	if end.Aspect == "end0" {
		aspect = "end0/ff"
	}
	return Element{Node: end.Node, Container: end.Container, Aspect: aspect}
}

func flowEndAspect(index int) string {
	if index == 0 {
		return "end0"
	}
	return "end1"
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
