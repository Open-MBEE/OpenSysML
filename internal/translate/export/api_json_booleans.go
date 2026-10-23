package export

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// apiJSONStatedDefaultsKey marks the root element objects of a document this
// mapping wrote with every non-derived boolean stated (the OMG API JSON shape):
// there a boolean at the value the graph implies is the writer's, not a statement.
const apiJSONStatedDefaultsKey = "sysx:statedDefaults"

// apiJSONExplicitDefaultsKey lists, on an element object of a marked document,
// the booleans the graph itself states at the value it would imply anyway, so
// the reader keeps them as the statements they are.
const apiJSONExplicitDefaultsKey = "sysx:explicitDefaults"

// apiJSONReservedPredicate reports whether a sysx: predicate of the graph is one
// the element form reserves for its own markers and so never writes as a graph fact.
func apiJSONReservedPredicate(predicate string) bool {
	return predicate == rdf.OpenSysML+strings.TrimPrefix(apiJSONStatedDefaultsKey, "sysx:") ||
		predicate == rdf.OpenSysML+strings.TrimPrefix(apiJSONExplicitDefaultsKey, "sysx:")
}

// apiJSONExternalForm reports whether the graph is already an external
// writer's element form — one stating Element::isImpliedIncluded, which this
// mapping never does — and so is written as it is: nothing completed, no marker.
func apiJSONExternalForm(graph *rdf.Graph, scratch *apiJSONWriteScratch) bool {
	if scratch != nil && scratch.externalForm != nil {
		return *scratch.externalForm
	}
	external := false
	for _, subject := range graph.SubjectsView() {
		if graph.HasProperty(subject, rdf.SysML+"isImpliedIncluded") {
			external = true
			break
		}
	}
	if scratch != nil {
		scratch.externalForm = &external
	}
	return external
}

// apiJSONExplicitDefaults lists the boolean members of an element object, as the
// graph stated them, that a reader of a marked document would take for completions.
func apiJSONExplicitDefaults(typ rdf.Term, element apiJSONObject) []any {
	var explicit []any
	for _, member := range element {
		if apiJSONImpliedBoolean(typ, member, element) {
			explicit = append(explicit, member.key)
		}
	}
	return explicit
}

// apiJSONGraphStatedBooleans are the booleans the graph itself states on every
// element they apply to, whatever their value (a Feature's isComposite, a
// LiteralBoolean's value), so they are never completed or dropped.
var apiJSONGraphStatedBooleans = map[string]bool{"isComposite": true, "value": true}

// apiJSONStatedBoolean reports whether property is a non-derived boolean the
// element form states whatever its value, and the value the graph implies for an
// element with the given members when it states none: the metaclass default, or
// the flag another spelling of the graph already carries.
func apiJSONStatedBoolean(property ontology.Property, members apiJSONObject) (value bool, stated bool) {
	if property.Derived || property.Range != rdf.XSD+"boolean" || !property.HasDefault ||
		apiJSONGraphStatedBooleans[property.Name] {
		return false, false
	}
	switch property.Name {
	case "isVariable":
		if prefix, ok := apiJSONMemberValue(members, "sysx:declaredPrefix").(string); ok && prefix == "var" {
			return true, true
		}
	case "isSufficient":
		if all, ok := apiJSONMemberValue(members, "isAll").(bool); ok && all {
			return true, true
		}
	}
	return property.Default == "true", true
}

// apiJSONImpliedBoolean reports whether a member of an element object of the
// given type, in a document carrying apiJSONStatedDefaultsKey, states only what
// the graph implies.
func apiJSONImpliedBoolean(typ rdf.Term, member apiJSONMember, members apiJSONObject) bool {
	value, ok := member.value.(bool)
	if !ok {
		return false
	}
	property, ok := ontology.PropertyOf(apiJSONOwnedAbsenceMetaclassInGraph(nil, typ.Value), member.key)
	if !ok {
		return false
	}
	implied, stated := apiJSONStatedBoolean(property, members)
	return stated && implied == value
}

func apiJSONMemberValue(members apiJSONObject, key string) any {
	for _, member := range members {
		if member.key == key {
			return member.value
		}
	}
	return nil
}

// apiJSONRootSubject reports whether subject is a root of the document, where
// the writer places apiJSONStatedDefaultsKey: an unowned element of the model.
func apiJSONRootSubject(graph *rdf.Graph, subject rdf.Term) bool {
	return strings.HasPrefix(subject.Value, rdf.Element) && strings.HasPrefix(graph.Type(subject), rdf.SysML) &&
		!hasOwner(graph, subject) && !LibraryReference(graph, subject)
}
