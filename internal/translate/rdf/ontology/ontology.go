// Package ontology holds the SysML v2 metamodel term table generated from the
// OMG metamodel's SysML.ecore, plus the domain/range check built on it. The
// table qualifies each property by its defining metaclass in OWL style
// (sysml:Element_declaredName) where this tool writes the unqualified name
// (sysml:declaredName), so it records both spellings. See README.md.
package ontology

//go:generate go run -C ../../../../tools ./gen/ontology

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

// Property is one metamodel property, an ecore eStructuralFeature.
type Property struct {
	// Name is the unqualified name this tool's encoder writes ("declaredName").
	Name string
	// DefiningClass is the rdfs:domain, the metaclass declaring it ("Element").
	DefiningClass string
	// IRI is "https://www.omg.org/spec/SysML#Element_declaredName".
	IRI string
	// Kind is owl:ObjectProperty or owl:DatatypeProperty.
	Kind PropertyKind
	// Range is the rdfs:range IRI: a metaclass or enumeration in the SysML
	// namespace, or the XSD/OWL datatype of an ecore primitive.
	Range string
	// Many reports an unbounded upper multiplicity (ecore upperBound -1):
	// the API JSON shape is an array.
	Many bool
	// Ordered reports a Many property whose values are ordered (ecore
	// ordered, which defaults to true); a single-valued property is never Ordered.
	Ordered bool
	// Derived reports a property the metamodel computes (ecore derived="true")
	// rather than one an element owns.
	Derived bool
	// Redefines and Subsets name the properties this one redefines or subsets,
	// as "DefiningClass::name" ("Type::ownedFeature"), when the metamodel states any.
	Redefines []string
	Subsets   []string
	// Opposite is the "DefiningClass::name" of the ecore eOpposite, the
	// navigable property at the other end of the same association, if any.
	Opposite string
}

// QualifiedName returns the property as "DefiningClass::name", the form
// Redefines, Subsets and Opposite name properties in.
func (p Property) QualifiedName() string { return p.DefiningClass + "::" + p.Name }

// Class is one metaclass, an ecore EClass, and its eSuperTypes.
type Class struct {
	// Name is the metaclass name ("PartUsage"), its local name in the namespace.
	Name string
	// Abstract reports an abstract metaclass (ecore abstract="true"), one no
	// element is an instance of directly.
	Abstract bool
	// Parents are the metaclass names of the declared direct superclasses.
	Parents []string
}

// Enumeration is one metamodel enumeration, an ecore EEnum, with its literals in declaration order.
type Enumeration struct {
	Name     string
	Literals []string
}

// Properties returns every declared property, ordered by IRI.
func Properties() []Property { return properties }

// Classes returns every declared metaclass, ordered by name.
func Classes() []Class { return classes }

// Enumerations returns every declared enumeration, ordered by name.
func Enumerations() []Enumeration {
	out := make([]Enumeration, len(enumerations))
	for i, enumeration := range enumerations {
		out[i] = enumeration
		out[i].Literals = append([]string(nil), enumeration.Literals...)
	}
	return out
}

// Indexes over the immutable generated table, built once on first lookup.
var (
	indexOnce          sync.Once
	propertiesByName   map[string][]Property
	classesByName      map[string]Class
	enumerationsByName map[string]Enumeration
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
		enumerationsByName = make(map[string]Enumeration, len(enumerations))
		for _, enumeration := range enumerations {
			enumerationsByName[enumeration.Name] = enumeration
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

// LookupEnumeration returns the enumeration of a name and whether it is declared.
func LookupEnumeration(name string) (Enumeration, bool) {
	index()
	enumeration, ok := enumerationsByName[name]
	if ok {
		enumeration.Literals = append([]string(nil), enumeration.Literals...)
	}
	return enumeration, ok
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

// ManyAgreed reports whether every declaration of an unqualified property name
// agrees on Many, and that agreed value; it reports no agreement when the name
// is undeclared or the declarations disagree.
func ManyAgreed(name string) (many, agreed bool) {
	decls := LookupProperty(name)
	if len(decls) == 0 {
		return false, false
	}
	for _, p := range decls[1:] {
		if p.Many != decls[0].Many {
			return false, false
		}
	}
	return decls[0].Many, true
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
