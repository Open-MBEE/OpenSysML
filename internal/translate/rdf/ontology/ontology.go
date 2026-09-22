// Package ontology holds the SysML v2 metamodel term table generated from the
// Open-MBEE OWL rendering, plus the domain/range check built on it. The ontology
// qualifies each property by its defining metaclass (sysml:Element_declaredName)
// where this tool writes the unqualified name (sysml:declaredName), so the table
// records both spellings. It records no ecore abstractness, so a "the metaclass
// must be concrete" check is not possible here. See README.md.
package ontology

//go:generate go run -C ../../../../tools ./gen/ontology -ontology $SYSMLV2_RDF_ONTOLOGY

import (
	"strings"
	"sync"
)

// PropertyKind distinguishes the two kinds of property the ontology declares.
type PropertyKind int

const (
	// ObjectProperty is an owl:ObjectProperty: its values are IRIs.
	ObjectProperty PropertyKind = iota
	// DatatypeProperty is an owl:DatatypeProperty: its values are literals.
	DatatypeProperty
)

// String returns the OWL name of the kind.
func (k PropertyKind) String() string {
	if k == DatatypeProperty {
		return "owl:DatatypeProperty"
	}
	return "owl:ObjectProperty"
}

// Property is one metamodel property as the ontology declares it.
type Property struct {
	// Name is the unqualified name this tool's encoder writes ("declaredName").
	Name string
	// DefiningClass is the rdfs:domain, the metaclass declaring it ("Element").
	DefiningClass string
	// IRI is "https://www.omg.org/spec/SysML#Element_declaredName".
	IRI string
	// Kind is owl:ObjectProperty or owl:DatatypeProperty.
	Kind PropertyKind
	// Range is the declared rdfs:range IRI, empty when none is declared.
	Range string
	// Many reports an unbounded upper multiplicity (ecore upperBound -1):
	// the API JSON shape is an array.
	Many bool
}

// Class is one metaclass and its named rdfs:subClassOf parents; the anonymous
// cardinality restrictions the ontology also states are not recorded.
type Class struct {
	// Name is the metaclass name ("PartUsage"), its local name in the namespace.
	Name string
	// Parents are the metaclass names of the declared direct superclasses.
	Parents []string
}

// Properties returns every declared property, ordered by IRI.
func Properties() []Property { return properties }

// Classes returns every declared metaclass, ordered by name.
func Classes() []Class { return classes }

// Indexes over the immutable generated table, built once on first lookup.
var (
	indexOnce        sync.Once
	propertiesByName map[string][]Property
	classesByName    map[string]Class
)

func index() {
	indexOnce.Do(func() {
		propertiesByName = make(map[string][]Property, len(properties))
		for _, p := range properties {
			propertiesByName[p.Name] = append(propertiesByName[p.Name], p)
		}
		classesByName = make(map[string]Class, len(classes))
		for _, c := range classes {
			classesByName[c.Name] = c
		}
	})
}

// LookupProperty returns every declaration of an unqualified property name, in
// table order; several metaclasses may declare one name (see AmbiguousNames).
func LookupProperty(name string) []Property {
	index()
	return propertiesByName[name]
}

// LookupClass returns the metaclass of a name and whether it is declared.
func LookupClass(name string) (Class, bool) {
	index()
	c, ok := classesByName[name]
	return c, ok
}

// PropertyOf returns the declaration of an unqualified property name whose
// defining metaclass is metaclass or one of its ancestors; when several
// declarations qualify, the one declared on the most specific class. It
// reports false when no declaration qualifies.
func PropertyOf(metaclass, name string) (Property, bool) {
	var best Property
	found := false
	for _, p := range LookupProperty(name) {
		if !IsAncestorOrSelf(metaclass, p.DefiningClass) {
			continue
		}
		if found && IsAncestorOrSelf(best.DefiningClass, p.DefiningClass) {
			continue
		}
		best = p
		found = true
	}
	return best, found
}

// AmbiguousNames returns the unqualified names more than one metaclass declares.
func AmbiguousNames() []string {
	index()
	var out []string
	seen := make(map[string]bool)
	for _, p := range properties {
		if len(propertiesByName[p.Name]) > 1 && !seen[p.Name] {
			seen[p.Name] = true
			out = append(out, p.Name)
		}
	}
	return out
}

// IsAncestorOrSelf reports whether ancestor is class or a transitive
// rdfs:subClassOf parent of it, as a domain check needs.
func IsAncestorOrSelf(class, ancestor string) bool {
	index()
	if class == ancestor {
		return true
	}
	seen := map[string]bool{class: true}
	queue := []string{class}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, parent := range classesByName[current].Parents {
			if parent == ancestor {
				return true
			}
			if !seen[parent] {
				seen[parent] = true
				queue = append(queue, parent)
			}
		}
	}
	return false
}

// LocalName returns the part of an IRI after the '#', or the IRI itself.
func LocalName(iri string) string {
	if cut := strings.LastIndex(iri, "#"); cut >= 0 {
		return iri[cut+1:]
	}
	return iri
}
