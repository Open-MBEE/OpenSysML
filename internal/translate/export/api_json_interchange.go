package export

import (
	"slices"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

var apiJSONInterchangePropertiesByMetaclass sync.Map

// apiJSONInterchangeProperties lists every property the metamodel gives a metaclass,
// each as its most specific declaration, in name order.
func apiJSONInterchangeProperties(metaclass string) []ontology.Property {
	if cached, ok := apiJSONInterchangePropertiesByMetaclass.Load(metaclass); ok {
		return cached.([]ontology.Property)
	}
	var properties []ontology.Property
	seen := map[string]bool{}
	for _, declared := range ontology.Properties() {
		if seen[declared.Name] {
			continue
		}
		seen[declared.Name] = true
		if property, ok := apiJSONNormativeProperty(metaclass, declared.Name); ok {
			properties = append(properties, property)
		}
	}
	slices.SortFunc(properties, func(a, b ontology.Property) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	cached, _ := apiJSONInterchangePropertiesByMetaclass.LoadOrStore(metaclass, properties)
	return cached.([]ontology.Property)
}

// apiJSONNormativeProperty is ontology.PropertyOf without InvocationExpression::operand, which
// the pilot's metamodel declares and the normative SysML.json does not.
func apiJSONNormativeProperty(metaclass, name string) (ontology.Property, bool) {
	property, ok := ontology.PropertyOf(metaclass, name)
	if ok && property.DefiningClass == "InvocationExpression" && property.Name == "operand" {
		return ontology.Property{}, false
	}
	return property, ok
}

// interchangeElement makes element the object the normative schema describes for
// metaclass: only the metaclass's properties, and every derived one stated, an empty
// one as [] or null. A derived property the evaluator declines stays absent.
func (w *apiJSONElementWriter) interchangeElement(subject rdf.Term, metaclass string, element apiJSONObject) apiJSONObject {
	if _, known := ontology.LookupClass(metaclass); !known {
		// The form writes no note, so an element the mapping types itself must not reference one.
		return slices.DeleteFunc(element, func(member apiJSONMember) bool { return member.key == "sysx:"+xNote })
	}
	element = slices.DeleteFunc(element, func(member apiJSONMember) bool {
		if member.key == "@id" || member.key == "@type" {
			return false
		}
		_, declared := apiJSONNormativeProperty(metaclass, member.key)
		return !declared
	})
	semanticElement, evaluated := w.completionElement(subject, metaclass)
	var redefined []apiJSONMetaclassCompletion
	for _, completion := range apiJSONCompletedPropertiesOf(metaclass) {
		if completion.redefinedBy != "" {
			redefined = append(redefined, completion)
		}
	}
	for _, property := range apiJSONInterchangeProperties(metaclass) {
		if apiJSONHasMember(element, property.Name) ||
			slices.ContainsFunc(redefined, func(c apiJSONMetaclassCompletion) bool { return c.name == property.Name }) {
			continue
		}
		// A property the graph owns is stated by the graph or not at all.
		if !property.Derived || !evaluated {
			continue
		}
		if value, ok := w.interchangeDerived(subject, metaclass, semanticElement, property); ok {
			element = append(element, apiJSONMember{key: property.Name, value: value})
		}
	}
	for _, completion := range redefined {
		if apiJSONHasMember(element, completion.name) {
			continue
		}
		for _, member := range element {
			if member.key == completion.redefinedBy {
				element = append(element, apiJSONMember{key: completion.name, value: member.value})
				break
			}
		}
	}
	return element
}

// interchangeEmpty spells a property that has no value: [] when multi-valued, else null.
func interchangeEmpty(property ontology.Property) any {
	if property.Many {
		return []any{}
	}
	return nil
}

// interchangeDerived evaluates a derived property of subject and spells it with its
// multiplicity; it reports false, and the omission, when the evaluator declines.
func (w *apiJSONElementWriter) interchangeDerived(
	subject rdf.Term, metaclass string, semanticElement metamodel.Element, property ontology.Property,
) (any, bool) {
	evaluator := w.evaluation.evaluator
	value, supported := evaluator.PropertyReadOnlyUncached(semanticElement, property.DefiningClass, property.Name)
	if !supported {
		reason, explained := evaluator.Explain(semanticElement, property.DefiningClass, property.Name)
		if !explained {
			reason = metamodel.Reason{Category: metamodel.ReasonNoRule, Detail: property.QualifiedName()}
		}
		w.completionOmitted(subject, metaclass, property.Name, reason)
		return nil, false
	}
	if derivedValueEmpty(value) {
		return interchangeEmpty(property), true
	}
	spelled, ok := w.derivedJSONValue(value)
	if !ok {
		w.completionOmitted(subject, metaclass, property.Name, metamodel.Reason{
			Category: metamodel.ReasonUnrepresentedElement, Detail: property.QualifiedName(),
		})
		return nil, false
	}
	values, sequence := spelled.([]any)
	switch {
	case property.Many && !sequence:
		return []any{spelled}, true
	case !property.Many && sequence && len(values) == 1:
		return values[0], true
	case !property.Many && sequence:
		w.completionOmitted(subject, metaclass, property.Name, metamodel.Reason{
			Category: metamodel.ReasonIncompleteStructure, Detail: property.QualifiedName() + " is single-valued",
		})
		return nil, false
	}
	return spelled, true
}
