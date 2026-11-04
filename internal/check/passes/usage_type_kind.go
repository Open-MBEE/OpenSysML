package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// metaclassParents maps each KerML kernel classifier (KerML 1.1 §8.4) and SysML
// definition metaclass (SysML v2 §8.3) to the metaclasses it specializes.
var metaclassParents = map[string][]string{
	"DataType":             {"Classifier"},
	"Class":                {"Classifier"},
	"Structure":            {"Class"},
	"Association":          {"Classifier"},
	"AssociationStructure": {"Association", "Structure"},
	"Behavior":             {"Class"},
	"Function":             {"Behavior"},
	"Predicate":            {"Function"},
	"Interaction":          {"Association", "Behavior"},
	"Metaclass":            {"Structure"},

	"AttributeDefinition":        {"DataType"},
	"EnumerationDefinition":      {"AttributeDefinition"},
	"OccurrenceDefinition":       {"Class"},
	"ItemDefinition":             {"OccurrenceDefinition", "Structure"},
	"PartDefinition":             {"ItemDefinition"},
	"PortDefinition":             {"OccurrenceDefinition", "Structure"},
	"ConnectionDefinition":       {"PartDefinition", "AssociationStructure"},
	"InterfaceDefinition":        {"ConnectionDefinition"},
	"AllocationDefinition":       {"ConnectionDefinition"},
	"ActionDefinition":           {"OccurrenceDefinition", "Behavior"},
	"FlowDefinition":             {"ActionDefinition", "Interaction"},
	"StateDefinition":            {"ActionDefinition"},
	"CalculationDefinition":      {"ActionDefinition", "Function"},
	"CaseDefinition":             {"CalculationDefinition"},
	"AnalysisCaseDefinition":     {"CaseDefinition"},
	"VerificationCaseDefinition": {"CaseDefinition"},
	"UseCaseDefinition":          {"CaseDefinition"},
	"ConstraintDefinition":       {"OccurrenceDefinition", "Predicate"},
	"RequirementDefinition":      {"ConstraintDefinition"},
	"ConcernDefinition":          {"RequirementDefinition"},
	"ViewpointDefinition":        {"RequirementDefinition"},
	"ViewDefinition":             {"PartDefinition"},
	"RenderingDefinition":        {"PartDefinition"},
	"MetadataDefinition":         {"ItemDefinition", "Metaclass"},
}

// metaclassSpecializes reports whether metaclass m is want or specializes it.
func metaclassSpecializes(m, want string) bool {
	if m == want {
		return true
	}
	for _, parent := range metaclassParents[m] {
		if metaclassSpecializes(parent, want) {
			return true
		}
	}
	return false
}

// defKindMetaclass is the metaclass a definition of each kind instantiates; an
// `individual def` is an OccurrenceDefinition with isIndividual (SysML v2 §8.3.9.3).
var defKindMetaclass = map[symbols.SymbolKind]string{
	symbols.SymbolAttributeDef:        "AttributeDefinition",
	symbols.SymbolEnumerationDef:      "EnumerationDefinition",
	symbols.SymbolOccurrenceDef:       "OccurrenceDefinition",
	symbols.SymbolIndividualDef:       "OccurrenceDefinition",
	symbols.SymbolItemDef:             "ItemDefinition",
	symbols.SymbolPartDef:             "PartDefinition",
	symbols.SymbolPortDef:             "PortDefinition",
	symbols.SymbolConnectionDef:       "ConnectionDefinition",
	symbols.SymbolInterfaceDef:        "InterfaceDefinition",
	symbols.SymbolAllocationDef:       "AllocationDefinition",
	symbols.SymbolActionDef:           "ActionDefinition",
	symbols.SymbolFlowDef:             "FlowDefinition",
	symbols.SymbolStateDef:            "StateDefinition",
	symbols.SymbolCalcDef:             "CalculationDefinition",
	symbols.SymbolCaseDef:             "CaseDefinition",
	symbols.SymbolAnalysisCaseDef:     "AnalysisCaseDefinition",
	symbols.SymbolVerificationCaseDef: "VerificationCaseDefinition",
	symbols.SymbolUseCaseDef:          "UseCaseDefinition",
	symbols.SymbolConstraintDef:       "ConstraintDefinition",
	symbols.SymbolRequirementDef:      "RequirementDefinition",
	symbols.SymbolConcernDef:          "ConcernDefinition",
	symbols.SymbolViewpointDef:        "ViewpointDefinition",
	symbols.SymbolViewDef:             "ViewDefinition",
	symbols.SymbolRenderingDef:        "RenderingDefinition",
	symbols.SymbolMetadataDef:         "MetadataDefinition",
	symbols.SymbolMetaclass:           "Metaclass",
}

// defKindInstantiates reports whether a definition of kind k is an instance of
// metaclass want, directly or through the metaclasses its own specializes.
func defKindInstantiates(k symbols.SymbolKind, want string) bool {
	m, ok := defKindMetaclass[k]
	return ok && metaclassSpecializes(m, want)
}

// kermlTypeMetaclasses is the metaclass a KerML type written with a keyword
// instantiates (KerML 1.1 §8.4).
var kermlTypeMetaclasses = map[string]string{
	"datatype":           "DataType",
	"class":              "Class",
	"struct":             "Structure",
	"assoc":              "Association",
	"association":        "Association",
	"assoc struct":       "AssociationStructure",
	"association struct": "AssociationStructure",
	"behavior":           "Behavior",
	"function":           "Function",
	"predicate":          "Predicate",
	"interaction":        "Interaction",
	"metaclass":          "Metaclass",
}

// usageTypeMetaclasses is the type of each usage kind's `/…Definition` property,
// which redefines Feature::type through Usage::definition (SysML v2 §8.3.6.4, §8.3.7–27).
// A property that only subsets another (item, part, verification) demands its parent's;
// a connection takes any Association (the pilot's reading: the Kernel Semantic
// Library's own `connection :MatesWith` is an `assoc`); an absent kind takes any.
var usageTypeMetaclasses = map[ast.UsageKind]string{
	ast.UsageAttribute:        "DataType",
	ast.UsageEnumeration:      "EnumerationDefinition",
	ast.UsageOccurrence:       "Class",
	ast.UsageIndividual:       "Class",
	ast.UsageItem:             "Class",
	ast.UsagePart:             "Class",
	ast.UsageActor:            "Class",
	ast.UsageStakeholder:      "Class",
	ast.UsagePort:             "PortDefinition",
	ast.UsageConnection:       "Association",
	ast.UsageInterface:        "InterfaceDefinition",
	ast.UsageAllocation:       "AllocationDefinition",
	ast.UsageFlow:             "Interaction",
	ast.UsageAction:           "Behavior",
	ast.UsageState:            "Behavior",
	ast.UsageCalc:             "Function",
	ast.UsageConstraint:       "Predicate",
	ast.UsageRequirement:      "RequirementDefinition",
	ast.UsageObjective:        "RequirementDefinition",
	ast.UsageSatisfy:          "RequirementDefinition",
	ast.UsageConcern:          "ConcernDefinition",
	ast.UsageFramedConcern:    "ConcernDefinition",
	ast.UsageViewpoint:        "ViewpointDefinition",
	ast.UsageView:             "ViewDefinition",
	ast.UsageRendering:        "RenderingDefinition",
	ast.UsageViewRendering:    "RenderingDefinition",
	ast.UsageCase:             "CaseDefinition",
	ast.UsageAnalysisCase:     "AnalysisCaseDefinition",
	ast.UsageVerificationCase: "CaseDefinition",
	ast.UsageUseCase:          "UseCaseDefinition",
	ast.UsageMetadata:         "Metaclass",
}

// kermlTypeTypesUsage reports whether a KerML type written with keyword may type a
// usage of decl's kind; an `individual`/`snapshot`/`timeslice` usage takes any Class.
func kermlTypeTypesUsage(decl declKind, keyword string) bool {
	meta, ok := kermlTypeMetaclasses[keyword]
	if !ok {
		return false
	}
	if decl.isOccurrenceUsage() && metaclassSpecializes(meta, "Class") {
		return true
	}
	want, ok := usageTypeMetaclasses[decl.useKind]
	return !ok || metaclassSpecializes(meta, want)
}

// usageKindBases returns the standard-library definitions every usage of usage's
// kind specializes implicitly (SysML v2 §8.3, the check…Specialization constraints).
func usageKindBases(model *semantics.Model, resolver *resolve.Resolver, usage *symbols.Symbol) []*symbols.Symbol {
	return kindBases(model, resolver, usage, false)
}

// kindBases is usageKindBases for a declaration in either language: a KerML
// feature's implied base is its keyword's (KerML 1.1 §8.4), not a usage kind's.
func kindBases(model *semantics.Model, resolver *resolve.Resolver, usage *symbols.Symbol, isKerML bool) []*symbols.Symbol {
	if model == nil || resolver == nil || resolver.Index() == nil || usage == nil {
		return nil
	}
	var bases []*symbols.Symbol
	for _, fqn := range model.KindBaseFQNs(usage, isKerML) {
		for _, base := range resolver.Index().LookupQualified(fqn) {
			if base != nil {
				bases = append(bases, base)
			}
		}
	}
	return bases
}

// usageTypeAllowed is the one verdict on a type a usage has, declared or
// inherited. The kind's base is among the usage's types too, so a type above the
// base is redundant and the base is what the kind rule judges, and a kind typed
// by exactly one definition takes no type beside its base (pilot removeRedundantTypes).
func usageTypeAllowed(model *semantics.Model, resolver *resolve.Resolver, decl declKind, typ *symbols.Symbol) bool {
	if typ == nil || decl.isEnd || decl.isReferenceUsage() {
		return true
	}
	_, oneType := oneTypeUsageMessages[decl.useKind]
	for _, base := range usageKindBases(model, resolver, decl.sym) {
		if symbols.SameElement(base, typ) {
			continue
		}
		if model.Conforms(base, typ) {
			return typeOfKindAllowed(decl, base)
		}
		if oneType && !model.Conforms(typ, base) {
			return false
		}
	}
	return typeOfKindAllowed(decl, typ)
}

// typeOfKindAllowed judges one type by decl's kind rule: a KerML type by the
// metaclass its keyword names, a definition by its kind.
func typeOfKindAllowed(decl declKind, typ *symbols.Symbol) bool {
	if typ.Kind == symbols.SymbolKerMLType {
		return kermlTypeTypesUsage(decl, ast.NotationKeyword(typ.Notation()))
	}
	return isCompatibleTyping(decl.useKind, typ.Kind, decl.isOccurrenceUsage())
}
