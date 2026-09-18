package ontology_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf/ontology"
)

// TestTableShape locks what the generated table must hold for the checks built on
// it to mean anything, including the counts § D8 of the roadmap reports.
func TestTableShape(t *testing.T) {
	properties := ontology.Properties()
	classes := ontology.Classes()
	// The counts SysML.owl 202407 declares; an ontology bump is expected to move them.
	object, datatype := 0, 0
	for _, p := range properties {
		if p.Kind == ontology.ObjectProperty {
			object++
		} else {
			datatype++
		}
	}
	if len(classes) != 172 || object != 348 || datatype != 63 {
		t.Errorf("table holds %d classes, %d object and %d datatype properties; want 172, 348, 63",
			len(classes), object, datatype)
	}
	if got := len(ontology.AmbiguousNames()); got != 59 {
		t.Errorf("got %d unqualified names declared by more than one metaclass, want 59", got)
	}
	if ontology.Version == "" || len(ontology.SourceCommit) != 40 {
		t.Errorf("table header not recorded: version %q, commit %q",
			ontology.Version, ontology.SourceCommit)
	}
	for _, p := range properties {
		if p.IRI != rdf.SysML+p.DefiningClass+"_"+p.Name {
			t.Errorf("%s: IRI is not <namespace><metaclass>_<name>", p.IRI)
		}
		if _, ok := ontology.LookupClass(p.DefiningClass); !ok {
			t.Errorf("%s: defining metaclass %s is not a declared class", p.IRI, p.DefiningClass)
		}
		if p.Range == "" {
			t.Errorf("%s: no declared range", p.IRI)
		}
		rangeName := ontology.LocalName(p.Range)
		_, rangeIsClass := ontology.LookupClass(rangeName)
		if (p.Kind == ontology.ObjectProperty) != rangeIsClass {
			t.Errorf("%s: %s ranges over %s, which is%s a declared metaclass",
				p.IRI, p.Kind, p.Range, map[bool]string{true: "", false: " not"}[rangeIsClass])
		}
	}
	for _, c := range classes {
		for _, parent := range c.Parents {
			if _, ok := ontology.LookupClass(parent); !ok {
				t.Errorf("%s: parent %s is not a declared class", c.Name, parent)
			}
		}
		if c.Name != "Element" && !ontology.IsAncestorOrSelf(c.Name, "Element") {
			t.Errorf("%s does not specialize Element", c.Name)
		}
	}
}

// TestLookups checks the accessors against declarations read out of SysML.owl by
// hand, including a name two metaclasses declare.
func TestLookups(t *testing.T) {
	declaredName := ontology.LookupProperty("declaredName")
	if len(declaredName) != 1 {
		t.Fatalf("declaredName: got %d declarations, want 1", len(declaredName))
	}
	want := ontology.Property{
		Name:          "declaredName",
		DefiningClass: "Element",
		IRI:           rdf.SysML + "Element_declaredName",
		Kind:          ontology.DatatypeProperty,
		Range:         rdf.XSD + "string",
	}
	if declaredName[0] != want {
		t.Errorf("declaredName: got %+v, want %+v", declaredName[0], want)
	}
	owner := ontology.LookupProperty("owningNamespace")
	if len(owner) != 1 || owner[0].Kind != ontology.ObjectProperty ||
		owner[0].Range != rdf.SysML+"Namespace" {
		t.Errorf("owningNamespace: got %+v, want one object property ranging over Namespace", owner)
	}
	if got := ontology.LookupProperty("noSuchProperty"); got != nil {
		t.Errorf("noSuchProperty: got %+v, want none", got)
	}
	if len(ontology.LookupProperty("visibility")) < 2 {
		t.Error("visibility: want the ambiguous name declared by more than one metaclass")
	}
	if names := ontology.AmbiguousNames(); len(names) == 0 {
		t.Error("AmbiguousNames: want the names more than one metaclass declares")
	}
	if !ontology.IsAncestorOrSelf("PartUsage", "Element") {
		t.Error("PartUsage should specialize Element transitively")
	}
	if ontology.IsAncestorOrSelf("Element", "PartUsage") {
		t.Error("Element does not specialize PartUsage")
	}
	if _, ok := ontology.LookupClass("NotAMetaclass"); ok {
		t.Error("NotAMetaclass should not be declared")
	}
}

// TestCheckReportsEachKind exercises the check on a hand-built graph, one triple
// per kind, independently of what the export fixtures happen to contain.
func TestCheckReportsEachKind(t *testing.T) {
	graph := rdf.NewGraph()
	part := rdf.ElementIRI("M::p")
	graph.Add(part, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("PartUsage"))
	graph.Add(part, rdf.SysMLTerm("declaredName"), rdf.String("p")) // conformant
	graph.Add(part, rdf.SysMLTerm("owner"), rdf.ElementIRI("M"))    // conformant
	graph.Add(part, rdf.SysMLTerm("notAProperty"), rdf.String("x"))
	graph.Add(part, rdf.SysMLTerm("lowerBound"), rdf.Int(1))
	graph.Add(part, rdf.SysMLTerm("owningNamespace"), rdf.String("M"))
	graph.Add(part, rdf.SysMLTerm("declaredName"), rdf.ElementIRI("M::q"))
	graph.Add(part, rdf.OpenSysMLTerm("memberIndex"), rdf.Int(0)) // outside the ontology
	untyped := rdf.ElementIRI("M::u")
	graph.Add(untyped, rdf.SysMLTerm("declaredName"), rdf.String("u"))
	odd := rdf.ElementIRI("M::o")
	graph.Add(odd, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("NotAMetaclass"))

	want := map[string]string{
		"unknown-property PartUsage notAProperty":               `named "notAProperty"`,
		"domain-mismatch PartUsage lowerBound":                  "declared on MultiplicityRange",
		"literal-for-object-property PartUsage owningNamespace": "owl:ObjectProperty",
		"iri-for-datatype-property PartUsage declaredName":      "owl:DatatypeProperty",
		"untyped-subject - -":                                   "no rdf:type",
		"unknown-class NotAMetaclass -":                         "not an owl:Class",
	}
	got := make(map[string]string)
	for _, violation := range ontology.Check(graph) {
		got[violation.Key()] = violation.Detail
	}
	for key, detail := range want {
		found, ok := got[key]
		if !ok {
			t.Errorf("missing violation %q", key)
			continue
		}
		if !strings.Contains(found, detail) {
			t.Errorf("%q: detail %q does not mention %q", key, found, detail)
		}
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			t.Errorf("unexpected violation %q", key)
		}
	}
}
