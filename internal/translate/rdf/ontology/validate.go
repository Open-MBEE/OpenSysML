package ontology

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// ViolationKind names the ways a graph can disagree with the ontology.
type ViolationKind string

const (
	// UnknownClass is an rdf:type the ontology declares no owl:Class for.
	UnknownClass ViolationKind = "unknown-class"
	// UntypedSubject carries SysML properties with no rdf:type, which leaves
	// their domain uncheckable.
	UntypedSubject ViolationKind = "untyped-subject"
	// UnknownProperty is a predicate whose name no metaclass declares.
	UnknownProperty ViolationKind = "unknown-property"
	// DomainMismatch is a property whose declaring metaclass is neither the
	// subject's metaclass nor an ancestor of it.
	DomainMismatch ViolationKind = "domain-mismatch"
	// LiteralForObjectProperty is an owl:ObjectProperty with a literal object.
	LiteralForObjectProperty ViolationKind = "literal-for-object-property"
	// IRIForDatatypeProperty is an owl:DatatypeProperty with an IRI object.
	IRIForDatatypeProperty ViolationKind = "iri-for-datatype-property"
)

// Violation is one triple's disagreement with the ontology.
type Violation struct {
	Kind ViolationKind
	// Class is the local name of the subject's rdf:type, "-" when it has none.
	Class string
	// Property is the unqualified name of the predicate, "-" for a violation
	// about the subject's type rather than a property.
	Property string
	// Subject is the subject IRI, for diagnostics only; Key omits it.
	Subject string
	// Detail explains the specific disagreement.
	Detail string
}

// Key identifies a violation by kind, metaclass and property. It deliberately
// omits the subject, so a committed list of known violations survives a change of
// element identity.
func (v Violation) Key() string {
	return fmt.Sprintf("%s %s %s", v.Kind, v.Class, v.Property)
}

// String renders the violation for a test failure message.
func (v Violation) String() string {
	return fmt.Sprintf("%s: %s (subject %s)", v.Key(), v.Detail, v.Subject)
}

const missing = "-"

// Check reports every way the triples of g disagree with the ontology. Only
// SysML-namespace predicates are checked; the RDF vocabulary and this tool's own
// extension namespace are out of the ontology's scope. The ontology records no
// abstractness, so an abstract metaclass is not a violation here.
func Check(g *rdf.Graph) []Violation {
	var out []Violation
	hasSysMLProperty := make(map[rdf.Term]bool)
	for _, triple := range g.Triples() {
		if strings.HasPrefix(triple.Predicate.Value, rdf.SysML) {
			hasSysMLProperty[triple.Subject] = true
		}
	}
	declared := make(map[rdf.Term]bool)
	for _, subject := range g.Subjects() {
		typeIRI := g.Type(subject)
		if typeIRI == "" {
			if hasSysMLProperty[subject] {
				out = append(out, Violation{
					Kind: UntypedSubject, Class: missing, Property: missing, Subject: subject.Value,
					Detail: "subject carries SysML properties but no rdf:type",
				})
			}
			continue
		}
		if _, ok := LookupClass(classNameOf(typeIRI)); ok {
			declared[subject] = true
			continue
		}
		out = append(out, Violation{
			Kind: UnknownClass, Class: classNameOf(typeIRI), Property: missing, Subject: subject.Value,
			Detail: fmt.Sprintf("rdf:type %s is not an owl:Class in the ontology", typeIRI),
		})
	}
	for _, triple := range g.Triples() {
		if !strings.HasPrefix(triple.Predicate.Value, rdf.SysML) {
			continue
		}
		class := classNameOf(g.Type(triple.Subject))
		if class == "" {
			continue // already reported as an untyped subject
		}
		if v, ok := checkTriple(class, declared[triple.Subject], triple); ok {
			out = append(out, v)
		}
	}
	return out
}

// checkTriple applies the property checks to one triple. An undeclared class has
// no place in the hierarchy to compare a domain against, so only the property's
// existence and object kind are checked for it.
func checkTriple(class string, classDeclared bool, triple rdf.Triple) (Violation, bool) {
	name := strings.TrimPrefix(triple.Predicate.Value, rdf.SysML)
	found := Violation{Class: class, Property: name, Subject: triple.Subject.Value}
	declarations := LookupProperty(name)
	if len(declarations) == 0 {
		found.Kind = UnknownProperty
		found.Detail = fmt.Sprintf("no metaclass in the ontology declares a property named %q", name)
		return found, true
	}
	applicable := declarations
	if classDeclared {
		applicable = nil
		for _, declaration := range declarations {
			if IsAncestorOrSelf(class, declaration.DefiningClass) {
				applicable = append(applicable, declaration)
			}
		}
		if len(applicable) == 0 {
			found.Kind = DomainMismatch
			found.Detail = fmt.Sprintf("%s is declared on %s, which is not %s or an ancestor of it",
				name, definingClasses(declarations), class)
			return found, true
		}
	}
	// A name several metaclasses declare is satisfied if any applicable
	// declaration accepts the object.
	for _, declaration := range applicable {
		if objectMatchesKind(declaration.Kind, triple.Object) {
			return Violation{}, false
		}
	}
	if applicable[0].Kind == DatatypeProperty {
		found.Kind = IRIForDatatypeProperty
		found.Detail = fmt.Sprintf("%s is an owl:DatatypeProperty (range %s) but the object is the IRI %s",
			applicable[0].IRI, shortIRI(applicable[0].Range), triple.Object.Value)
		return found, true
	}
	found.Kind = LiteralForObjectProperty
	found.Detail = fmt.Sprintf("%s is an owl:ObjectProperty (range %s) but the object is the literal %s",
		applicable[0].IRI, shortIRI(applicable[0].Range), triple.Object)
	return found, true
}

func objectMatchesKind(kind PropertyKind, object rdf.Term) bool {
	if kind == DatatypeProperty {
		return object.IsLiteral()
	}
	return object.IsIRI()
}

// classNameOf returns the local name of an rdf:type IRI, or "" when there is
// none. An IRI outside the SysML namespace is kept whole, to stay recognizable.
func classNameOf(typeIRI string) string {
	if typeIRI == "" {
		return ""
	}
	if strings.HasPrefix(typeIRI, rdf.SysML) {
		return strings.TrimPrefix(typeIRI, rdf.SysML)
	}
	return typeIRI
}

func definingClasses(declarations []Property) string {
	names := make([]string, len(declarations))
	for i, declaration := range declarations {
		names[i] = declaration.DefiningClass
	}
	return strings.Join(names, ", ")
}

func shortIRI(iri string) string {
	switch {
	case iri == "":
		return "unstated"
	case strings.HasPrefix(iri, rdf.SysML):
		return "sysml:" + strings.TrimPrefix(iri, rdf.SysML)
	case strings.HasPrefix(iri, rdf.XSD):
		return "xsd:" + strings.TrimPrefix(iri, rdf.XSD)
	}
	return iri
}
