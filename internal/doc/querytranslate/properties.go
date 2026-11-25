package querytranslate

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// derivedProperties are the properties the evaluator derives that no metaclass
// declares, spelled in the OpenSysML namespace by OSLC query text.
var derivedProperties = []string{
	query.PropertyID, query.PropertyType, query.PropertyMultiplicityLower, query.PropertyMultiplicityUpper,
	query.PropertySymbolKind, query.PropertyAnnotations, query.PropertyVerifyingCase,
	query.PropertyIsActive, query.PropertyBoundSubject,
}

// objectProjection is what a query over held objects reports when it selects
// nothing: the properties the evaluator serves for every row, none a metaclass's.
var objectProjection = []string{
	query.PropertyID, query.PropertyType, query.PropertyName, query.PropertyDeclaredName,
	query.PropertyShortName, query.PropertyDeclaredShortName, query.PropertyDocumentation,
	query.PropertyTextualRepresentation, query.PropertyQualifiedName, query.PropertyOwner,
	query.PropertyElementType, query.PropertyGeneral, query.PropertyIsAbstract, query.PropertyIsIndividual,
	query.PropertyMultiplicityLower, query.PropertyMultiplicityUpper, query.PropertySymbolKind,
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
	// features are the metaclass features not yet classified, by name; a
	// query classifies the ones it names, not the metamodel's thousands.
	features map[string][]*symbols.Symbol
	// shadowed holds evaluator properties whose bare name a model feature
	// declares: their type is decided by the row that resolves the name.
	shadowed        map[string]bool
	model           *semantics.Model
	number, boolean *symbols.Symbol
}

// NewTable reads the metaclass declarations of the model's library. A model
// loaded without the library knows the evaluator's queryable properties alone.
func NewTable(index *symbols.Index, model *semantics.Model) *Table {
	t := &Table{metamodel: map[string]valueKind{}, derived: map[string]valueKind{},
		features: map[string][]*symbols.Symbol{}, shadowed: map[string]bool{}}
	for _, name := range query.PropertyNames() {
		t.metamodel[name] = kindText
	}
	for _, name := range derivedProperties {
		delete(t.metamodel, name)
		t.derived[name] = kindText
	}
	t.derived[query.PropertyMultiplicityLower] = kindNumber
	t.derived[query.PropertyMultiplicityUpper] = kindNumber
	t.derived[query.PropertyVerifyingCase] = kindElement
	t.derived[query.PropertyIsActive] = kindBoolean
	t.derived[query.PropertyBoundSubject] = kindElement
	t.metamodel[query.PropertyIsAbstract] = kindBoolean
	t.metamodel[query.PropertyIsIndividual] = kindBoolean
	for _, name := range []string{query.PropertyOwner, query.PropertyElementType, query.PropertyGeneral,
		query.PropertySatisfiedRequirement, query.PropertySatisfyingFeature, query.PropertyVerifiedRequirement} {
		t.metamodel[name] = kindElement
	}
	if index == nil || model == nil {
		return t
	}
	t.model = model
	t.number = index.LookupUnique("ScalarValues::Number")
	t.boolean = index.LookupUnique("ScalarValues::Boolean")
	t.features = queryexec.DeclaredMetaclassFeatures(index)
	// The library's own isActive is the built-in, not a feature shadowing it.
	for _, name := range []string{query.PropertyIsActive, query.PropertyAnnotations} {
		for _, sym := range index.LookupUniqueCandidates(name) {
			if sym.IsFeature() && !semantics.IsStateActivity(sym) {
				t.shadowed[name] = true
				break
			}
		}
	}
	return t
}

// classify settles the kind of a metaclass property from its declarations'
// types, over the evaluator's default for the name (documentation holds elements).
func (t *Table) classify(name string) valueKind {
	members := t.features[name]
	delete(t.features, name)
	kind, seen := kindUnknown, false
	for _, member := range members {
		declared := t.declaredKind(member)
		switch {
		case !seen:
			kind, seen = declared, true
		case kind != declared:
			// A name typed differently by two metaclasses is compared as the row's value is.
			kind = kindUnknown
		}
	}
	t.metamodel[name] = kind
	return kind
}

// declaredKind is the kind every type of one declaration agrees on; a feature
// whose types disagree is compared as the row's value is.
func (t *Table) declaredKind(member *symbols.Symbol) valueKind {
	declared := kindUnknown
	for i, typ := range t.model.FeatureTypes(member) {
		kind := t.kindOf(typ)
		if i > 0 && kind != declared {
			return kindUnknown
		}
		declared = kind
	}
	return declared
}

// kindOf is what values of a declared type are: numbers, booleans, text for
// strings and other data, elements otherwise.
func (t *Table) kindOf(typ *symbols.Symbol) valueKind {
	switch {
	case t.number != nil && (symbols.SameElement(typ, t.number) || t.model.Conforms(typ, t.number)):
		return kindNumber
	case t.boolean != nil && symbols.SameElement(typ, t.boolean):
		return kindBoolean
	case strings.HasSuffix(symbols.FQNOf(typ), "::String"), t.model.IsDataType(typ):
		return kindText
	default:
		return kindElement
	}
}

// Metamodel reports whether the metamodel declares a property so named.
func (t *Table) Metamodel(name string) bool {
	_, ok := t.metamodel[name]
	_, declared := t.features[name]
	return ok || declared
}

// Derived reports whether name is a property of the evaluator's own, including
// the unambiguous "@" spelling of one a model feature may shadow.
func (t *Table) Derived(name string) bool {
	if _, ok := query.BuiltinProperty(name); ok {
		return true
	}
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
	if canonical, ok := query.BuiltinProperty(name); ok {
		if !query.IsBuiltinAlias(name) && t.shadowed[canonical] {
			return kindUnknown
		}
		name = canonical
	}
	if kind, ok := t.derived[name]; ok {
		return kind
	}
	if _, ok := t.features[name]; ok {
		return t.classify(name)
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
	names := t.MetamodelNames()
	for name := range t.derived {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// MetamodelNames lists the metamodel's properties, sorted.
func (t *Table) MetamodelNames() []string {
	names := make([]string, 0, len(t.metamodel)+len(t.features))
	for name := range t.metamodel {
		names = append(names, name)
	}
	for name := range t.features {
		if _, seen := t.metamodel[name]; !seen {
			names = append(names, name)
		}
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

// DefaultProjection is what a query over elements selecting nothing reports: every queryable
// property but elementId, read from a conversion of the whole model and so reported only when named.
func DefaultProjection() []string {
	out := make([]string, 0, len(query.PropertyNames()))
	for _, name := range query.PropertyNames() {
		if name != query.PropertyElementID {
			out = append(out, name)
		}
	}
	return out
}
