package querytranslate

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// metaclassPackages are the library packages declaring the abstract syntax
// metaclasses, whose features are the metamodel properties.
var metaclassPackages = []string{"KerML::Root", "KerML::Core", "KerML::Kernel", "SysML::Systems"}

// derivedProperties are the properties the evaluator derives that no metaclass
// declares, spelled in the OpenSysML namespace by OSLC query text.
var derivedProperties = []string{
	query.PropertyID, query.PropertyType, query.PropertyMultiplicityLower, query.PropertyMultiplicityUpper,
}

// universalProperties are the properties the evaluator serves for every element
// row, so a projection of them never reads a property no row carries.
var universalProperties = []string{
	query.PropertyID, query.PropertyType, query.PropertyName, query.PropertyDeclaredName,
	query.PropertyShortName, query.PropertyDeclaredShortName, query.PropertyDocumentation,
	query.PropertyTextualRepresentation, query.PropertyQualifiedName, query.PropertyOwner,
	query.PropertyElementType, query.PropertyGeneral, query.PropertyIsAbstract, query.PropertyIsIndividual,
	query.PropertyMultiplicityLower, query.PropertyMultiplicityUpper,
}

// textProperties are the universal properties whose values are text, which
// free-text search reads.
var textProperties = []string{
	query.PropertyName, query.PropertyDeclaredName, query.PropertyShortName, query.PropertyDeclaredShortName,
	query.PropertyQualifiedName, query.PropertyDocumentation, query.PropertyTextualRepresentation,
}

// valueKind classifies what a property's declared types hold, for the operators
// a comparison may apply before any row is read.
type valueKind int

const (
	kindUnknown valueKind = iota
	kindText
	kindBoolean
	kindNumber
	kindElement
)

// Table is the reflective property table of a loaded model: every feature the
// metamodel library declares on a metaclass, and the evaluator's own properties.
type Table struct {
	metamodel map[string]valueKind
	derived   map[string]valueKind
}

// NewTable reads the metaclass declarations of the model's library. A model
// loaded without the library knows the evaluator's queryable properties alone.
func NewTable(index *symbols.Index, model *semantics.Model) *Table {
	t := &Table{metamodel: map[string]valueKind{}, derived: map[string]valueKind{}}
	for _, name := range query.PropertyNames() {
		t.metamodel[name] = kindText
	}
	for _, name := range derivedProperties {
		delete(t.metamodel, name)
		t.derived[name] = kindText
	}
	t.derived[query.PropertyMultiplicityLower] = kindNumber
	t.derived[query.PropertyMultiplicityUpper] = kindNumber
	t.metamodel[query.PropertyIsAbstract] = kindBoolean
	t.metamodel[query.PropertyIsIndividual] = kindBoolean
	for _, name := range []string{query.PropertyOwner, query.PropertyElementType, query.PropertyGeneral,
		query.PropertySatisfiedRequirement, query.PropertySatisfyingFeature} {
		t.metamodel[name] = kindElement
	}
	if index == nil || model == nil {
		return t
	}
	number := index.LookupUnique("ScalarValues::Number")
	boolean := index.LookupUnique("ScalarValues::Boolean")
	for _, pkg := range metaclassPackages {
		for _, owner := range index.LookupQualified(pkg) {
			if owner.Scope == nil {
				continue
			}
			for _, metaclass := range owner.Scope.Members() {
				if !isMetaclass(metaclass) {
					continue
				}
				t.addMetaclass(model, metaclass, number, boolean)
			}
		}
	}
	return t
}

// isMetaclass reports a declaration of the abstract syntax: KerML's
// `metaclass`, or the `metadata def` the SysML library declares its own with.
func isMetaclass(sym *symbols.Symbol) bool {
	return sym != nil && (sym.Kind == symbols.SymbolMetaclass || sym.Kind == symbols.SymbolMetadataDef)
}

// addMetaclass records the metaclass's features, its own and inherited from
// other metaclasses; one inherited from the library's occurrences and objects
// (dispatchScope, portionOf) is no property of the metamodel's.
func (t *Table) addMetaclass(model *semantics.Model, metaclass, number, boolean *symbols.Symbol) {
	for _, member := range model.MembersOfIncludingRedefined(metaclass) {
		if member == nil || !member.IsFeature() {
			continue
		}
		if member.OwnerScope != nil && !isMetaclass(member.OwnerScope.Owner()) {
			continue
		}
		name := symbols.LastSegment(member.Name)
		if name == "" {
			continue
		}
		kind := kindUnknown
		for _, typ := range model.FeatureTypes(member) {
			switch {
			case number != nil && (symbols.SameElement(typ, number) || model.Conforms(typ, number)):
				kind = kindNumber
			case boolean != nil && symbols.SameElement(typ, boolean):
				kind = kindBoolean
			case strings.HasSuffix(symbols.FQNOf(typ), "::String"):
				kind = kindText
			case model.IsDataType(typ):
				kind = kindText
			default:
				kind = kindElement
			}
		}
		if known, seen := t.metamodel[name]; seen && known != kindUnknown && known != kind {
			// A name typed differently by two metaclasses is compared as the row's value is.
			kind = kindUnknown
		}
		t.metamodel[name] = kind
	}
}

// Metamodel reports whether the metamodel declares a property so named.
func (t *Table) Metamodel(name string) bool {
	_, ok := t.metamodel[name]
	return ok
}

// Derived reports whether name is a property of the evaluator's own.
func (t *Table) Derived(name string) bool {
	_, ok := t.derived[name]
	return ok
}

// Known reports whether name is a property the evaluator can read: of the
// metamodel, of the evaluator, or a dotted path whose every segment is one.
func (t *Table) Known(name string) bool {
	if t.Metamodel(name) || t.Derived(name) {
		return true
	}
	segments := strings.Split(name, ".")
	if len(segments) < 2 {
		return false
	}
	for _, segment := range segments {
		if !t.Metamodel(segment) && !t.Derived(segment) {
			return false
		}
	}
	return true
}

// kind is what the property's declared types hold, kindUnknown where the
// table does not know or the metaclasses disagree.
func (t *Table) kind(name string) valueKind {
	if kind, ok := t.derived[name]; ok {
		return kind
	}
	if kind, ok := t.metamodel[name]; ok {
		return kind
	}
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return t.kind(name[i+1:])
	}
	return kindUnknown
}

// Names lists every property the table knows, sorted.
func (t *Table) Names() []string {
	names := make([]string, 0, len(t.metamodel)+len(t.derived))
	for name := range t.metamodel {
		names = append(names, name)
	}
	for name := range t.derived {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// MetamodelNames lists the metamodel's properties, sorted.
func (t *Table) MetamodelNames() []string {
	names := make([]string, 0, len(t.metamodel))
	for name := range t.metamodel {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DerivedNames lists the evaluator's own properties, sorted.
func (t *Table) DerivedNames() []string {
	names := make([]string, 0, len(t.derived))
	for name := range t.derived {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultProjection is what a query selecting nothing reports: the properties
// every element carries. The properties of particular metaclasses — body,
// satisfiedRequirement, … — and elementId are reported when selected.
func DefaultProjection() []string {
	return append([]string(nil), universalProperties...)
}
