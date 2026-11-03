package export

import (
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

type apiJSONOwnedAbsenceProperty struct {
	property  ontology.Property
	effective ontology.Property
	group     []ontology.Property
}

type apiJSONOwnedAbsencePlanEntry struct {
	once       sync.Once
	properties []apiJSONOwnedAbsenceProperty
}

type apiJSONOwnedAbsenceOntology struct {
	propertiesByClass          map[string][]ontology.Property
	propertiesByName           map[string]ontology.Property
	redefinitionsByClass       map[string]map[string]ontology.Property
	libraryStructureProperties map[string]bool
}

var (
	apiJSONOwnedAbsenceOntologyOnce sync.Once
	apiJSONOwnedAbsenceMetadata     apiJSONOwnedAbsenceOntology
	apiJSONOwnedAbsencePlans        sync.Map
	apiJSONEmptyCollection          = []any{}
)

var apiJSONOwnedAbsenceTypeAliases = map[string]string{
	"AssociationUsage":     "Association",
	"BehaviorUsage":        "Behavior",
	"BooleanUsage":         "BooleanExpression",
	"ClassUsage":           "Class",
	"IndividualDefinition": "OccurrenceDefinition",
	"IndividualUsage":      "OccurrenceUsage",
	"InteractionUsage":     "Interaction",
	"PredicateUsage":       "Predicate",
	"StructureUsage":       "Structure",
}

var apiJSONOwnedAbsenceExtensionParents = map[string]string{
	"Pseudostate": "StateUsage",
}

// apiJSONDerivedVariablePair reports whether property is a usage's isVariable or
// mayTimeVary: Usage::mayTimeVary redefines Feature::isVariable and derives both.
func apiJSONDerivedVariablePair(metaclass, property string) bool {
	return (property == "isVariable" || property == "mayTimeVary") &&
		ontology.IsAncestorOrSelf(metaclass, "Usage")
}

func appendAPIJSONOwnedAbsencesWithScratch(
	graph *rdf.Graph,
	subject rdf.Term,
	element apiJSONObject,
	typ rdf.Term,
	scratch *apiJSONWriteScratch,
) apiJSONObject {
	metaclass := apiJSONOwnedAbsenceMetaclassInGraph(graph, typ.Value)
	if metaclass == "" {
		return element
	}
	libraryStub := LibraryReference(graph, subject)
	var absences apiJSONObject
	if scratch != nil {
		absences = scratch.absences[:0]
	}
	var mappingValues apiJSONObject
	for _, member := range element {
		if member.key == apiJSONMappingValuesKey {
			mappingValues, _ = member.value.(apiJSONObject)
			break
		}
	}
	external := apiJSONExternalForm(graph, scratch)
	plan := apiJSONOwnedAbsencePlanFor(metaclass)
	for _, planned := range plan {
		property := planned.property
		if apiJSONHasMember(element, property.Name) ||
			apiJSONHasMember(mappingValues, property.Name) {
			continue
		}
		if apiJSONDerivedVariablePair(metaclass, property.Name) {
			// A usage's isVariable is its derived mayTimeVary; the graph states the
			// pair only when true, and the OMG API JSON shape states it either way.
			if !libraryStub && !external {
				absences = append(absences, apiJSONMember{key: property.Name, value: false})
			}
			continue
		}
		if property.Derived {
			continue
		}
		var value any
		if property.Range == rdf.XSD+"boolean" {
			// Every non-derived boolean is stated, true or false (the OMG API JSON shape).
			// A boolean whose redefinition here is derived (Usage::mayTimeVary
			// over isVariable) is derived for this metaclass and not completed.
			stated, ok := apiJSONStatedBoolean(property, element)
			if !ok || planned.effective.Derived || libraryStub || external {
				continue
			}
			value = stated
		} else {
			if planned.effective.Derived ||
				(libraryStub && apiJSONOwnedAbsenceLibraryStructureProperty(property)) {
				continue
			}
			if len(planned.group) > 1 && apiJSONOwnedAbsenceRedefinitionGroupHasValue(element, planned.group) {
				continue
			}
			if !property.Many && property.Lower != 0 {
				continue
			}
			if property.Many {
				value = apiJSONEmptyCollection
			}
		}
		absences = append(absences, apiJSONMember{key: property.Name, value: value})
	}
	if explicit := apiJSONExplicitDefaults(typ, element); len(explicit) > 0 && !external {
		absences = append(absences, apiJSONMember{key: apiJSONExplicitDefaultsKey, value: explicit})
	}
	insertAt := len(element)
	for i, member := range element {
		if _, collection := member.value.([]any); collection {
			insertAt = i
			break
		}
	}
	if len(absences) > 0 {
		oldLength := len(element)
		newLength := oldLength + len(absences)
		if newLength > cap(element) {
			expanded := make(apiJSONObject, newLength)
			copy(expanded[:insertAt], element[:insertAt])
			copy(expanded[insertAt+len(absences):], element[insertAt:])
			element = expanded
		} else {
			element = element[:newLength]
			copy(element[insertAt+len(absences):], element[insertAt:oldLength])
		}
		copy(element[insertAt:insertAt+len(absences)], absences)
	}
	if scratch != nil {
		scratch.absences = absences
	}
	return element
}

func apiJSONHasMember(element apiJSONObject, key string) bool {
	for _, member := range element {
		if member.key == key {
			return true
		}
	}
	return false
}

func apiJSONOwnedAbsenceMetaclassInGraph(graph *rdf.Graph, metaclass string) string {
	if resolved := apiJSONOwnedAbsenceMetaclass(metaclass); resolved != "" {
		return resolved
	}
	const subClassOf = "http://www.w3.org/2000/01/rdf-schema#subClassOf"
	seen := map[string]bool{}
	var queue []string
	if graph != nil {
		queue = append(queue, metaclass)
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if seen[current] {
			continue
		}
		seen[current] = true
		if resolved := apiJSONOwnedAbsenceMetaclass(current); resolved != "" {
			return resolved
		}
		for _, parent := range graph.Objects(rdf.IRI(current), subClassOf) {
			if parent.IsIRI() {
				queue = append(queue, parent.Value)
			}
		}
	}
	if parent := apiJSONOwnedAbsenceExtensionParents[strings.TrimPrefix(metaclass, rdf.OpenSysML)]; parent != "" {
		return parent
	}
	return ""
}

func apiJSONOwnedAbsenceMetaclass(metaclass string) string {
	metaclass = strings.TrimPrefix(metaclass, rdf.SysML)
	if strings.HasPrefix(metaclass, rdf.OpenSysML) || strings.HasPrefix(metaclass, "sysx:") {
		return ""
	}
	if canonical, ok := apiJSONOwnedAbsenceTypeAliases[metaclass]; ok {
		metaclass = canonical
	}
	if _, ok := ontology.LookupClass(metaclass); !ok {
		return ""
	}
	return metaclass
}

func apiJSONOwnedAbsencePlanFor(metaclass string) []apiJSONOwnedAbsenceProperty {
	metaclass = apiJSONOwnedAbsenceMetaclass(metaclass)
	if metaclass == "" {
		return nil
	}
	value, _ := apiJSONOwnedAbsencePlans.LoadOrStore(metaclass, &apiJSONOwnedAbsencePlanEntry{})
	entry := value.(*apiJSONOwnedAbsencePlanEntry)
	entry.once.Do(func() {
		metadata := apiJSONOwnedAbsenceOntologyData()
		groups := apiJSONOwnedAbsenceRedefinitionGroups(metaclass)
		groupByName := make(map[string][]ontology.Property)
		for _, group := range groups {
			for _, property := range group {
				groupByName[property.QualifiedName()] = group
			}
		}
		for _, property := range apiJSONOwnedAbsenceProperties(metaclass) {
			effective := property
			if redefined, ok := metadata.redefinitionsByClass[metaclass][property.QualifiedName()]; ok {
				effective = redefined
			}
			entry.properties = append(entry.properties, apiJSONOwnedAbsenceProperty{
				property: property, effective: effective, group: groupByName[property.QualifiedName()],
			})
		}
	})
	return entry.properties
}

func apiJSONOwnedAbsenceProperties(metaclass string) []ontology.Property {
	metadata := apiJSONOwnedAbsenceOntologyData()
	var lineage []string
	seenClasses := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		if seenClasses[name] {
			return
		}
		seenClasses[name] = true
		lineage = append(lineage, name)
		class, ok := ontology.LookupClass(name)
		if !ok {
			return
		}
		for _, parent := range class.Parents {
			visit(parent)
		}
	}
	visit(metaclass)
	seenProperties := make(map[string]bool)
	var properties []ontology.Property
	for _, class := range lineage {
		for _, property := range metadata.propertiesByClass[class] {
			if seenProperties[property.Name] {
				continue
			}
			if resolved, ok := ontology.PropertyOf(metaclass, property.Name); ok {
				property = resolved
			}
			seenProperties[property.Name] = true
			properties = append(properties, property)
		}
	}
	return properties
}

func apiJSONOwnedAbsenceOntologyData() apiJSONOwnedAbsenceOntology {
	apiJSONOwnedAbsenceOntologyOnce.Do(func() {
		allProperties := ontology.Properties()
		classes := make(map[string]ontology.Class)
		apiJSONOwnedAbsenceMetadata.propertiesByClass = make(map[string][]ontology.Property)
		apiJSONOwnedAbsenceMetadata.propertiesByName = make(map[string]ontology.Property, len(allProperties))
		for _, class := range ontology.Classes() {
			classes[class.Name] = class
		}
		for _, property := range allProperties {
			apiJSONOwnedAbsenceMetadata.propertiesByClass[property.DefiningClass] =
				append(apiJSONOwnedAbsenceMetadata.propertiesByClass[property.DefiningClass], property)
			apiJSONOwnedAbsenceMetadata.propertiesByName[property.QualifiedName()] = property
		}

		lineageCache := make(map[string][]string, len(classes))
		lineage := func(name string) []string {
			if cached, ok := lineageCache[name]; ok {
				return cached
			}
			seen := make(map[string]bool)
			var out []string
			var visit func(string)
			visit = func(current string) {
				if seen[current] {
					return
				}
				seen[current] = true
				out = append(out, current)
				for _, parent := range classes[current].Parents {
					visit(parent)
				}
			}
			visit(name)
			lineageCache[name] = out
			return out
		}
		apiJSONOwnedAbsenceMetadata.redefinitionsByClass =
			make(map[string]map[string]ontology.Property, len(classes))
		for name := range classes {
			overrides := make(map[string]ontology.Property)
			for _, class := range lineage(name) {
				for _, candidate := range apiJSONOwnedAbsenceMetadata.propertiesByClass[class] {
					visited := make(map[string]bool)
					var collect func(string)
					collect = func(target string) {
						if visited[target] {
							return
						}
						visited[target] = true
						if _, ok := overrides[target]; !ok {
							overrides[target] = candidate
						}
						if redefined, ok := apiJSONOwnedAbsenceMetadata.propertiesByName[target]; ok {
							for _, ancestor := range redefined.Redefines {
								collect(ancestor)
							}
						}
					}
					for _, target := range candidate.Redefines {
						collect(target)
					}
				}
			}
			apiJSONOwnedAbsenceMetadata.redefinitionsByClass[name] = overrides
		}
		apiJSONOwnedAbsenceMetadata.libraryStructureProperties =
			apiJSONOwnedAbsenceLibraryStructureProperties(allProperties)
	})
	return apiJSONOwnedAbsenceMetadata
}

func apiJSONOwnedAbsenceRedefinitionGroups(metaclass string) [][]ontology.Property {
	var properties []ontology.Property
	byName := make(map[string]ontology.Property)
	parent := make(map[string]string)
	for _, property := range ontology.Properties() {
		if !ontology.IsAncestorOrSelf(metaclass, property.DefiningClass) {
			continue
		}
		name := property.QualifiedName()
		properties = append(properties, property)
		byName[name] = property
		parent[name] = name
	}
	var find func(string) string
	find = func(name string) string {
		root := parent[name]
		if root != name {
			root = find(root)
			parent[name] = root
		}
		return root
	}
	for _, property := range properties {
		for _, redefinedName := range property.Redefines {
			redefined, ok := byName[redefinedName]
			if !ok || property.Derived || redefined.Derived {
				continue
			}
			left, right := find(property.QualifiedName()), find(redefinedName)
			if left != right {
				parent[right] = left
			}
		}
	}
	grouped := make(map[string][]ontology.Property)
	var roots []string
	for _, property := range properties {
		root := find(property.QualifiedName())
		if _, exists := grouped[root]; !exists {
			roots = append(roots, root)
		}
		grouped[root] = append(grouped[root], property)
	}
	var groups [][]ontology.Property
	for _, root := range roots {
		if len(grouped[root]) > 1 {
			groups = append(groups, grouped[root])
		}
	}
	return groups
}

func apiJSONOwnedAbsenceRedefinitionGroupHasValue(element apiJSONObject, group []ontology.Property) bool {
	for _, property := range group {
		var value any
		var ok bool
		for _, member := range element {
			if member.key == property.Name {
				value, ok = member.value, true
				break
			}
		}
		if !ok || value == nil {
			continue
		}
		if values, ok := value.([]any); ok && len(values) == 0 {
			continue
		}
		return true
	}
	return false
}

func apiJSONOwnedAbsenceLibraryStructureProperty(property ontology.Property) bool {
	metadata := apiJSONOwnedAbsenceOntologyData()
	switch property.Name {
	case "ownedRelationship", "owningRelationship", "ownedRelatedElement", "owningRelatedElement":
		return true
	default:
		return metadata.libraryStructureProperties[property.QualifiedName()]
	}
}

func apiJSONOwnedAbsenceLibraryStructureProperties(properties []ontology.Property) map[string]bool {
	out := map[string]bool{
		"Element::ownedRelationship":         true,
		"Element::owningRelationship":        true,
		"Element::ownedElement":              true,
		"Element::owner":                     true,
		"Relationship::ownedRelatedElement":  true,
		"Relationship::owningRelatedElement": true,
	}
	for changed := true; changed; {
		changed = false
		for _, property := range properties {
			if out[property.QualifiedName()] {
				continue
			}
			for _, parent := range append(append([]string(nil), property.Subsets...), property.Redefines...) {
				if !out[parent] {
					continue
				}
				out[property.QualifiedName()] = true
				changed = true
				break
			}
		}
	}
	return out
}
