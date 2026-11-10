package queryexec

import (
	"sort"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// MetaclassPackages are the library packages declaring the abstract syntax
// metaclasses, whose features are the metamodel properties.
var MetaclassPackages = []string{"KerML::Root", "KerML::Core", "KerML::Kernel", "SysML::Systems"}

// ownProperties are the evaluator's properties of every element row beyond
// the metamodel's: a wildcard projects them and a misspelling is matched
// against them.
var ownProperties = []string{
	query.PropertyID, query.PropertyType, query.PropertyElementType, query.PropertyGeneral,
	query.PropertyMultiplicityLower, query.PropertyMultiplicityUpper,
}

// metaclassProperty is a feature the library's metaclass declaration has,
// its own or inherited, with the class declaring it.
type metaclassProperty struct {
	Name          string
	DefiningClass string
}

// metaclassProperties are the metaclass's features by name, inherited and
// redefined ones included (partDefinition and definition), memoized. A feature
// a metaclass inherits from the library's occurrences and objects (dispatchScope,
// portionOf) is no property of the metamodel's and is left out.
func (e *executor) metaclassProperties(metaclass *symbols.Symbol) map[string]metaclassProperty {
	if properties, ok := e.metaclassFeatures[metaclass]; ok {
		return properties
	}
	if e.metaclassFeatures == nil {
		e.metaclassFeatures = map[*symbols.Symbol]map[string]metaclassProperty{}
	}
	properties := map[string]metaclassProperty{}
	for _, member := range e.context.Model.MembersOfIncludingRedefined(metaclass) {
		if member == nil || !member.IsFeature() {
			continue
		}
		name := symbols.LastSegment(member.Name)
		if name == "" {
			continue
		}
		if _, seen := properties[name]; seen {
			continue
		}
		definingClass := ""
		if member.OwnerScope != nil && member.OwnerScope.Owner() != nil {
			owner := member.OwnerScope.Owner()
			if !isMetaclass(owner) {
				continue
			}
			definingClass = symbols.LastSegment(owner.Name)
		}
		properties[name] = metaclassProperty{Name: name, DefiningClass: definingClass}
	}
	e.metaclassFeatures[metaclass] = properties
	return properties
}

// isMetaclass reports a declaration of the abstract syntax: KerML's
// `metaclass`, or the `metadata def` the SysML library declares its own with.
func isMetaclass(sym *symbols.Symbol) bool {
	return sym != nil && (sym.Kind == symbols.SymbolMetaclass || sym.Kind == symbols.SymbolMetadataDef)
}

// unservedReasons are the metamodel evaluator's reasons for the derived
// properties it does not serve, by DefiningClass::property.
var unservedReasons = sync.OnceValue(func() map[string]string {
	reasons := map[string]string{}
	for _, omission := range metamodel.Omitted() {
		reasons[omission.DefiningClass+"::"+omission.Property] = omission.Reason
	}
	return reasons
})

// metamodelDeclares reports a property some metaclass of the loaded library declares,
// or one of the evaluator's own (each segment of a dotted path): known even when no row has it.
func (e *executor) metamodelDeclares(property string) bool {
	if e.metamodelProperties == nil {
		e.metamodelProperties = map[string]bool{}
		for _, name := range ownProperties {
			e.metamodelProperties[name] = true
		}
		if e.context.Index != nil {
			for _, pkg := range MetaclassPackages {
				for _, owner := range e.context.Index.LookupQualified(pkg) {
					if owner.Scope == nil {
						continue
					}
					for _, metaclass := range owner.Scope.Members() {
						if !isMetaclass(metaclass) {
							continue
						}
						for name := range e.metaclassProperties(metaclass) {
							e.metamodelProperties[name] = true
						}
					}
				}
			}
		}
	}
	for _, segment := range strings.Split(property, ".") {
		if !e.metamodelProperties[segment] {
			return false
		}
	}
	return true
}

// unknownUnlessDeclared is the verdict on a property no row read carried: unknown when no
// metaclass declares it or no row is an element, nil when element rows merely lack it.
func (e *executor) unknownUnlessDeclared(expression queryplan.Expression, property string, seen *candidates) error {
	if len(seen.metaclasses) > 0 && e.metamodelDeclares(property) {
		return nil
	}
	return e.unknownPropertyAmong(expression, property, e.names(seen))
}

// metaclassPropertyOf is the property so named of the row's own metaclass,
// when it has one.
func (e *executor) metaclassPropertyOf(sym *symbols.Symbol, property string) (metaclassProperty, bool) {
	metaclass := e.context.Model.MetaclassOf(sym)
	if metaclass == nil {
		return metaclassProperty{}, false
	}
	declared, ok := e.metaclassProperties(metaclass)[property]
	return declared, ok
}

// candidates collects what the rows read could have named, for an
// unknown-property error: the properties of the element rows' metaclasses
// and the features of the object rows.
type candidates struct {
	metaclasses map[*symbols.Symbol]bool
	features    map[string]bool
}

func newCandidates() *candidates {
	return &candidates{metaclasses: map[*symbols.Symbol]bool{}, features: map[string]bool{}}
}

// note records the metaclass of an element row, or the features of an object
// row the model declares — not those its types inherit from the library
// (shape, portionOf), which are no properties a query names.
func (e *executor) note(seen *candidates, row Value) {
	if sym, ok := row.Element(); ok {
		if metaclass := e.context.Model.MetaclassOf(sym); metaclass != nil {
			seen.metaclasses[metaclass] = true
		}
		return
	}
	if inst, _, ok := row.Object(); ok && e.context.Runtime != nil {
		for _, of := range e.context.Runtime.FeaturesOfObject(inst) {
			if e.declaredInLibrary(of.Feature) {
				continue
			}
			seen.features[source.NameText(of.Name)] = true
		}
	}
}

// declaredInLibrary reports a feature a bundled library document declares.
func (e *executor) declaredInLibrary(feature *runtime.EffectiveFeature) bool {
	if feature == nil || feature.OwnerType == nil || e.context.Index == nil {
		return false
	}
	return e.context.Index.IsLibraryDocument(feature.OwnerType.DocName)
}

// names lists, sorted, the properties of the metaclasses and the features
// seen, the evaluator's own properties of an element row included.
func (e *executor) names(seen *candidates) []string {
	set := map[string]bool{}
	for metaclass := range seen.metaclasses {
		for name := range e.metaclassProperties(metaclass) {
			set[name] = true
		}
		for _, name := range ownProperties {
			set[name] = true
		}
	}
	for name := range seen.features {
		set[name] = true
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// metaclassPropertyValues reads a property of the row's metaclass through the
// reflective evaluator: elements for a reference,
// typed values for an attribute. Present only for such a property; one the
// evaluator does not derive is the unserved-property path error.
func (e *executor) metaclassPropertyValues(sym *symbols.Symbol, property string) ([]Value, bool, error) {
	declared, ok := e.metaclassPropertyOf(sym, property)
	if !ok {
		return nil, false, nil
	}
	model := e.context.Model
	if elements, ok := model.ReflectiveElements(sym, property); ok {
		return reflectiveElementValues(sym, elements), true, nil
	}
	values, ok := model.ReflectiveFeatureValues(sym, property)
	if !ok {
		return nil, true, e.unservedProperty(declared, sym)
	}
	result := make([]Value, 0, len(values))
	for _, value := range values {
		converted, ok := e.filterValue(value, sym)
		if !ok {
			return nil, true, e.featureError(queryplan.Expression{}, property, ElementValue(sym))
		}
		result = append(result, converted)
	}
	return result, true, nil
}

// unservedProperty is the path error for a metaclass property the evaluator
// does not derive: the defining class names the gap, the evaluator's reason when
// it records one.
func (e *executor) unservedProperty(declared metaclassProperty, sym *symbols.Symbol) error {
	key := declared.DefiningClass + "::" + declared.Name
	reason := unservedReasons()[key]
	return &Error{
		Kind:     ErrorUnservedProperty,
		Query:    e.definition.Name(),
		Property: declared.Name,
		Target:   symbols.FQNOf(sym),
		Expected: key,
		Actual:   reason,
		Outcome: &Outcome{Path: &PathError{Failures: []Failure{{
			Step:     Step{Kind: StepFeature, Argument: declared.Name},
			Expected: key + " derived by OpenSysML",
			Frontier: Frontier{At: symbols.FQNOf(sym)},
		}}}},
	}
}
