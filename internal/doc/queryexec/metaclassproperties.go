package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"sort"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// metaclassProperty is a feature the library's metaclass declaration has,
// its own or inherited, with the class declaring it.
type metaclassProperty struct {
	Name          string
	DefiningClass string
}

// metaclassProperties are the metaclass's features by name, inherited and
// redefined ones included (partDefinition and definition), memoized.
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
			definingClass = symbols.LastSegment(member.OwnerScope.Owner().Name)
		}
		properties[name] = metaclassProperty{Name: name, DefiningClass: definingClass}
	}
	e.metaclassFeatures[metaclass] = properties
	return properties
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
// row.
func (e *executor) note(seen *candidates, row Value) {
	if sym, ok := row.Element(); ok {
		if metaclass := e.context.Model.MetaclassOf(sym); metaclass != nil {
			seen.metaclasses[metaclass] = true
		}
		return
	}
	if inst, _, ok := row.Object(); ok && e.context.Runtime != nil {
		for _, of := range e.context.Runtime.FeaturesOfObject(inst) {
			seen.features[source.NameText(of.Name)] = true
		}
	}
}

// names lists, sorted, the properties of the metaclasses and the features
// seen.
func (e *executor) names(seen *candidates) []string {
	set := map[string]bool{}
	for metaclass := range seen.metaclasses {
		for name := range e.metaclassProperties(metaclass) {
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
