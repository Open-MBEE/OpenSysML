package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// apiJSONCompletedPropertiesKey names, on an element object, the derived
// properties the writer completed from the metamodel rather than read from the graph.
const apiJSONCompletedPropertiesKey = "sysx:completedProperties"

// apiJSONCompletedProperty is a derived property of the metamodel the element
// form completes from the evaluator when the graph states nothing for it.
type apiJSONCompletedProperty struct {
	class string
	name  string
	// clause cites the derive* constraint the value follows (KerML 1.1 Beta 2 §8.3).
	clause string
}

// apiJSONCompletedProperties lists the completed properties in the order the
// element form writes them. A class's entry applies to every metaclass under it.
var apiJSONCompletedProperties = []apiJSONCompletedProperty{
	{class: "Element", name: "isLibraryElement", clause: "8.3.2.1.2 deriveElementIsLibraryElement"},
}

// apiJSONCompletedPropertyNames maps every completed property name to the
// classes that complete it, for the reader and the compact form.
var apiJSONCompletedPropertyNames = func() map[string][]string {
	out := make(map[string][]string, len(apiJSONCompletedProperties))
	for _, property := range apiJSONCompletedProperties {
		out[property.name] = append(out[property.name], property.class)
	}
	return out
}()

// apiJSONCompletesProperty reports whether the element form completes name on
// an element of metaclass.
func apiJSONCompletesProperty(metaclass, name string) bool {
	for _, class := range apiJSONCompletedPropertyNames[name] {
		if ontology.IsAncestorOrSelf(metaclass, class) {
			return true
		}
	}
	return false
}

// completeDerived appends the completed derived properties of subject the graph
// does not state, and the marker naming them.
func (w *apiJSONElementWriter) completeDerived(subject rdf.Term, metaclass string, element apiJSONObject) apiJSONObject {
	if metaclass == "" || w.evaluation == nil || w.evaluation.evaluator == nil || w.evaluation.structure == nil ||
		LibraryReference(w.graph, subject) || apiJSONExternalForm(w.graph, w.scratch) {
		return element
	}
	semanticElement, ok := w.evaluation.structure.element(subject)
	if !ok {
		return element
	}
	var completed []any
	for _, property := range apiJSONCompletedProperties {
		if !ontology.IsAncestorOrSelf(metaclass, property.class) ||
			w.graph.HasProperty(subject, rdf.SysML+property.name) || apiJSONHasMember(element, property.name) {
			continue
		}
		value, supported := w.evaluation.evaluator.PropertyReadOnly(semanticElement, property.class, property.name)
		if !supported {
			continue
		}
		spelled, ok := w.derivedJSONValue(value)
		if !ok {
			continue
		}
		element = append(element, apiJSONMember{key: property.name, value: spelled})
		completed = append(completed, property.name)
	}
	if len(completed) > 0 {
		element = append(element, apiJSONMember{key: apiJSONCompletedPropertiesKey, value: completed})
	}
	return element
}

// derivedJSONValue spells an evaluator value in the element form; a null or
// empty value, or one naming an element no id addresses, is not spelled.
func (w *apiJSONElementWriter) derivedJSONValue(value metamodel.Value) (any, bool) {
	switch value.Kind {
	case metamodel.BooleanValue:
		return value.Boolean, true
	case metamodel.StringValue:
		return value.String, true
	case metamodel.EnumValue:
		return value.Enum, true
	case metamodel.IntegerValue:
		return value.Integer, true
	case metamodel.RealValue:
		return value.Real, true
	case metamodel.ElementValue:
		id, ok := w.derivedElementID(value.Element)
		if !ok {
			return nil, false
		}
		return apiJSONReference{ID: id}, true
	case metamodel.MembershipValue:
		id, ok := w.derivedElementID(metamodel.MembershipElement(value.Membership))
		if !ok {
			return nil, false
		}
		return apiJSONReference{ID: id}, true
	case metamodel.SequenceValue:
		if len(value.Values) == 0 {
			return nil, false
		}
		values := make([]any, 0, len(value.Values))
		for _, member := range value.Values {
			spelled, ok := w.derivedJSONValue(member)
			if !ok {
				return nil, false
			}
			values = append(values, spelled)
		}
		return values, true
	}
	return nil, false
}

// derivedElementID is the "@id" of an element the evaluator names: a graph
// subject's id, or a library element's normative id.
func (w *apiJSONElementWriter) derivedElementID(element metamodel.Element) (string, bool) {
	if subject, ok := w.evaluation.structure.subject(element); ok {
		return rdf.SubjectID(subject)
	}
	symbol := element.Symbol
	if element.IsMembership {
		symbol = element.Membership.Member
	}
	if symbol == nil {
		return "", false
	}
	library, ok := w.evaluation.libraryCatalog().ElementForSymbol(symbol)
	if !ok || library == nil {
		return "", false
	}
	if element.IsMembership {
		return library.OwningMembershipID, library.OwningMembershipID != ""
	}
	return library.ID, library.ID != ""
}

// libraryCatalog is the catalog of normative library ids, built once per evaluation.
func (e *apiJSONEvaluation) libraryCatalog() *identity.Catalog {
	e.catalogOnce.Do(func() {
		e.catalog = identity.LibraryCatalogWithOpenSysML(libs.SharedBase())
	})
	return e.catalog
}
