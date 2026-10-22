package passes

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// kermlTypeMetaclasses lists the metaclasses a KerML type written with a keyword
// instantiates (KerML 1.0 §8.4): a function is a Behavior and a Class too.
var kermlTypeMetaclasses = map[string][]string{
	"datatype":           {"DataType"},
	"class":              {"Class"},
	"struct":             {"Structure", "Class"},
	"assoc":              {"Association"},
	"association":        {"Association"},
	"assoc struct":       {"AssociationStructure", "Association", "Structure", "Class"},
	"association struct": {"AssociationStructure", "Association", "Structure", "Class"},
	"behavior":           {"Behavior", "Class"},
	"function":           {"Function", "Behavior", "Class"},
	"predicate":          {"Predicate", "Function", "Behavior", "Class"},
	"interaction":        {"Interaction", "Association", "Behavior", "Class"},
	"metaclass":          {"Metaclass", "Structure", "Class"},
}

// usageTypeMetaclasses is the metaclass every type of a usage of a kind must
// instantiate (SysML v2 §8.3.9.4 validateAttributeUsageType, §8.3.9.7
// validateOccurrenceUsageType, §8.3.14.5, §8.3.15.4, §8.3.16.6, §8.3.17.5,
// §8.3.18.3, §8.3.19.3, §8.3.27.3). A kind absent here demands a SysML definition.
var usageTypeMetaclasses = map[ast.UsageKind]string{
	ast.UsageAttribute:  "DataType",
	ast.UsageOccurrence: "Class",
	ast.UsageIndividual: "Class",
	ast.UsagePart:       "Class",
	ast.UsageItem:       "Class",
	ast.UsageAction:     "Behavior",
	ast.UsageState:      "Behavior",
	ast.UsageConnection: "Association",
	ast.UsageFlow:       "Interaction",
	ast.UsageCalc:       "Function",
	ast.UsageConstraint: "Predicate",
	ast.UsageMetadata:   "Metaclass",
}

// kermlTypeTypesUsage reports whether a KerML type written with keyword may type
// a usage of decl's kind. A directed attribute is a parameter and, like one typed
// by a definition, takes a Class as well as a DataType.
func kermlTypeTypesUsage(decl declKind, keyword string) bool {
	metas := kermlTypeMetaclasses[keyword]
	// An `individual`/`snapshot`/`timeslice` usage is an occurrence whatever its
	// keyword, so a Class types it as an occurrence definition would.
	if decl.isOccurrenceUsage() && slices.Contains(metas, "Class") {
		return true
	}
	if decl.useKind == ast.UsageAttribute && decl.direction != ast.DirNone {
		return slices.Contains(metas, "DataType") || slices.Contains(metas, "Class")
	}
	want, ok := usageTypeMetaclasses[decl.useKind]
	return ok && slices.Contains(metas, want)
}

// usageKindBases returns the standard-library definitions every usage of usage's
// kind specializes implicitly (SysML v2 §8.3, the check…Specialization constraints).
func usageKindBases(model *semantics.Model, resolver *resolve.Resolver, usage *symbols.Symbol) []*symbols.Symbol {
	if model == nil || resolver == nil || resolver.Index() == nil || usage == nil {
		return nil
	}
	var bases []*symbols.Symbol
	for _, fqn := range model.KindBaseFQNs(usage, false) {
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
	return isCompatibleTyping(decl.useKind, decl.direction, typ.Kind, decl.isOccurrenceUsage())
}
