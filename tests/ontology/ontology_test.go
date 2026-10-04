package ontology_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// TestTableShape locks what the generated table must hold for the checks built on
// it to mean anything, including the counts § D8 of the roadmap reports.
func TestTableShape(t *testing.T) {
	properties := ontology.Properties()
	classes := ontology.Classes()
	// The counts SysML.ecore 20250201 (pilot 2026-08) declares; a metamodel bump is expected to move them.
	object, datatype := 0, 0
	for _, p := range properties {
		if p.Kind == ontology.ObjectProperty {
			object++
		} else {
			datatype++
		}
	}
	if len(classes) != 175 || object != 351 || datatype != 64 {
		t.Errorf("table holds %d classes, %d object and %d datatype properties; want 175, 351, 64",
			len(classes), object, datatype)
	}
	if got := len(ontology.AmbiguousNames()); got != 58 {
		t.Errorf("got %d unqualified names declared by more than one metaclass, want 58", got)
	}
	if ontology.Version == "" || ontology.SourceTag == "" || len(ontology.SourceCommit) != 40 {
		t.Errorf("table header not recorded: version %q, tag %q, commit %q",
			ontology.Version, ontology.SourceTag, ontology.SourceCommit)
	}
	qualified := make(map[string]bool, len(properties))
	for _, p := range properties {
		qualified[p.QualifiedName()] = true
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
		if p.Ordered && !p.Many {
			t.Errorf("%s: Ordered on a single-valued property", p.IRI)
		}
		for _, ref := range append(append(append([]string(nil), p.Redefines...), p.Subsets...), p.Opposite) {
			if ref != "" && !qualified[ref] {
				t.Errorf("%s: names %s, which is no declared property", p.IRI, ref)
			}
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

// TestLookups checks the accessors against declarations read out of SysML.ecore by
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
	if !reflect.DeepEqual(declaredName[0], want) {
		t.Errorf("declaredName: got %+v, want %+v", declaredName[0], want)
	}
	ownedFeature, ok := ontology.PropertyOf("Type", "ownedFeature")
	wantOwnedFeature := ontology.Property{
		Name:          "ownedFeature",
		DefiningClass: "Type",
		IRI:           rdf.SysML + "Type_ownedFeature",
		Kind:          ontology.ObjectProperty,
		Range:         rdf.SysML + "Feature",
		Many:          true,
		Ordered:       true,
		Derived:       true,
		Subsets:       []string{"Namespace::ownedMember"},
		Opposite:      "Feature::owningType",
	}
	if !ok || !reflect.DeepEqual(ownedFeature, wantOwnedFeature) {
		t.Errorf("ownedFeature: got %+v, want %+v", ownedFeature, wantOwnedFeature)
	}
	memberFeature, _ := ontology.PropertyOf("FeatureMembership", "ownedMemberFeature")
	if want := []string{"OwningMembership::ownedMemberElement"}; !reflect.DeepEqual(memberFeature.Redefines, want) {
		t.Errorf("ownedMemberFeature: redefines %v, want %v", memberFeature.Redefines, want)
	}
	if element, _ := ontology.LookupClass("Element"); !element.Abstract {
		t.Error("Element should be abstract")
	}
	if part, _ := ontology.LookupClass("PartUsage"); part.Abstract {
		t.Error("PartUsage should be concrete")
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

var pilotPinRe = regexp.MustCompile(`(?m)^PILOT_(TAG|COMMIT)="\$\{PILOT_(?:TAG|COMMIT):-([^}"]*)\}"`)

// TestTableFollowsThePilotPin fails when scripts/pilot-pin.sh moves without the
// table being regenerated from the metamodel of the new pin.
func TestTableFollowsThePilotPin(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "pilot-pin.sh"))
	if err != nil {
		t.Fatal(err)
	}
	pin := map[string]string{}
	for _, m := range pilotPinRe.FindAllStringSubmatch(string(data), -1) {
		pin[m[1]] = m[2]
	}
	if pin["TAG"] != ontology.SourceTag || pin["COMMIT"] != ontology.SourceCommit {
		t.Errorf("table generated from pilot %s %s, scripts/pilot-pin.sh pins %s %s; "+
			"regenerate with ./scripts/download-pilot-metamodel.sh && go run -C tools ./gen/ontology",
			ontology.SourceTag, ontology.SourceCommit, pin["TAG"], pin["COMMIT"])
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
	imp := rdf.ElementIRI("M::i")
	graph.Add(imp, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Import"))

	want := map[string]string{
		"unknown-property PartUsage notAProperty":               `named "notAProperty"`,
		"domain-mismatch PartUsage lowerBound":                  "declared on MultiplicityRange",
		"literal-for-object-property PartUsage owningNamespace": "owl:ObjectProperty",
		"iri-for-datatype-property PartUsage declaredName":      "owl:DatatypeProperty",
		"untyped-subject - -":                                   "no rdf:type",
		"unknown-class NotAMetaclass -":                         "not an owl:Class",
		"abstract-class Import -":                               "abstract in the metamodel",
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

// TestCheckReadsTheMostSpecificType checks a subject stating several rdf:types as
// the class the graph reader selects, whatever order the types are stated in.
func TestCheckReadsTheMostSpecificType(t *testing.T) {
	for _, order := range [][]string{{"Element", "PartUsage"}, {"PartUsage", "Element"}} {
		graph := rdf.NewGraph()
		part := rdf.ElementIRI("M::p")
		for _, class := range order {
			graph.Add(part, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(class))
		}
		graph.Add(part, rdf.SysMLTerm("isComposite"), rdf.Bool(true)) // declared on Feature
		if got := ontology.Check(graph); len(got) != 0 {
			t.Errorf("types %v: unexpected violations %v", order, got)
		}
	}
}
