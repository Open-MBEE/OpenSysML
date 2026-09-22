package export_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// apiJSONModels lists every notation fixture under testdata, recursively, plus
// the interop model, whose element form the inverse contract is checked on.
func apiJSONModels(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.Walk("testdata", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		ext := filepath.Ext(path)
		if info.IsDir() || (ext != ".sysml" && ext != ".kerml") || strings.Contains(path, ".golden.") {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, filepath.Join("..", "..", "internal", "translate", "interop", "flexo", "testdata", "model.sysml"))
	sort.Strings(out)
	return out
}

// tripleSet is a graph's triples as a set; the API element form preserves the
// statements, not the spelling order inside a subject.
func tripleSet(g *rdf.Graph) map[rdf.Triple]bool {
	out := map[rdf.Triple]bool{}
	for _, triple := range g.Triples() {
		out[triple] = true
	}
	return out
}

// The API element form is a serializer over the same graph as Turtle: writing
// it and reading it back must yield the graph's triple set exactly, and writing
// the graph it read must spell the document the same way again.
func TestAPIJSONInverseOverAllFixtures(t *testing.T) {
	refused := 0
	for _, path := range apiJSONModels(t) {
		name, _ := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := convert.SysMLToRDF(path, src)
			var unsupported *export.UnsupportedError
			if errors.As(err, &unsupported) {
				refused++
				t.Skipf("ToRDF refuses this model: %v", err)
			}
			if err != nil {
				t.Fatalf("SysMLToRDF: %v", err)
			}
			document, err := export.WriteAPIJSON(graph)
			if err != nil {
				t.Fatalf("WriteAPIJSON: %v", err)
			}
			reread, err := export.ReadAPIJSON(document)
			if err != nil {
				t.Fatalf("ReadAPIJSON: %v\n%s", err, document)
			}
			if !sameTriples(tripleSet(graph), tripleSet(reread)) {
				missing := diffTriples(tripleSet(graph), tripleSet(reread))
				added := diffTriples(tripleSet(reread), tripleSet(graph))
				t.Fatalf("the graph changed through the element form\nmissing: %v\nadded: %v", missing, added)
			}
			again, err := export.WriteAPIJSON(reread)
			if err != nil {
				t.Fatalf("WriteAPIJSON of the read graph: %v", err)
			}
			if !bytes.Equal(document, again) {
				t.Fatalf("the element form was not written the same way again\n--- first ---\n%s\n--- again ---\n%s", document, again)
			}
		})
	}
	if refused == len(apiJSONModels(t)) {
		t.Fatal("ToRDF refused every fixture; the gate checked nothing")
	}
}

func sameTriples(a, b map[rdf.Triple]bool) bool {
	return len(diffTriples(a, b)) == 0 && len(diffTriples(b, a)) == 0
}

func diffTriples(a, b map[rdf.Triple]bool) []rdf.Triple {
	var out []rdf.Triple
	for triple := range a {
		if !b[triple] {
			out = append(out, triple)
		}
	}
	return out
}

// apiJSONDocument converts the interop model to the element form and decodes it
// into objects for shape assertions.
func apiJSONDocument(t *testing.T) []map[string]json.RawMessage {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "internal", "translate", "interop", "flexo", "testdata", "model.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := convert.SysMLToRDF("model.sysml", src)
	if err != nil {
		t.Fatalf("SysMLToRDF: %v", err)
	}
	document, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("the document is not a JSON array: %v\n%s", err, document)
	}
	return elements
}

func apiJSONElement(t *testing.T, elements []map[string]json.RawMessage, id string) map[string]json.RawMessage {
	t.Helper()
	for _, element := range elements {
		var elementID string
		if err := json.Unmarshal(element["@id"], &elementID); err == nil && elementID == id {
			return element
		}
	}
	t.Fatalf("no element object with @id %q", id)
	return nil
}

// The interop model's element objects carry the shape the API serves: keywords,
// scalar properties, {"@id": …} references and arrays for collections.
func TestAPIJSONShapeOnTheInteropModel(t *testing.T) {
	elements := apiJSONDocument(t)
	pkg := apiJSONElement(t, elements, "Interop")
	var typ string
	if err := json.Unmarshal(pkg["@type"], &typ); err != nil || typ != "Package" {
		t.Errorf(`Interop "@type" = %s, want "Package"`, pkg["@type"])
	}
	for _, key := range []string{"elementId", "qualifiedName", "declaredName"} {
		var value string
		if err := json.Unmarshal(pkg[key], &value); err != nil || value == "" {
			t.Errorf("Interop %s = %s, want a string", key, pkg[key])
		}
	}
	var members []map[string]string
	if err := json.Unmarshal(pkg["ownedMember"], &members); err != nil || len(members) == 0 {
		t.Fatalf("Interop ownedMember = %s, want an array of references", pkg["ownedMember"])
	}
	for i, member := range members {
		if len(member) != 1 || member["@id"] == "" {
			t.Errorf("ownedMember[%d] = %v, want a {\"@id\": …} reference", i, member)
		}
	}
	component := apiJSONElement(t, elements, "Interop__Component")
	var owner map[string]string
	if err := json.Unmarshal(component["owner"], &owner); err != nil || owner["@id"] != "Interop" {
		t.Errorf("Interop__Component owner = %s, want a single reference object, not an array", component["owner"])
	}
	var index json.Number
	if err := json.Unmarshal(pkg["sysx:memberIndex"], &index); err != nil {
		t.Errorf("Interop sysx:memberIndex = %s, want a JSON number", pkg["sysx:memberIndex"])
	}
	var text string
	if err := json.Unmarshal(pkg["sysx:sourceText"], &text); err != nil || !strings.Contains(text, "package Interop {") {
		t.Errorf("Interop sysx:sourceText = %s, want the notation string", pkg["sysx:sourceText"])
	}
	node := apiJSONElement(t, elements, "Interop__Component__mass_pvalue")
	if err := json.Unmarshal(node["@type"], &typ); err != nil || typ != "LiteralRational" {
		t.Errorf(`mass expression "@type" = %s, want "LiteralRational"`, node["@type"])
	}
	var value json.Number
	if err := json.Unmarshal(node["value"], &value); err != nil {
		t.Errorf("mass expression value = %s, want a JSON number", node["value"])
	}
	apiJSONElement(t, elements, "Interop__Component__mass_pvalue_om")
	for _, element := range elements {
		for key := range element {
			if strings.HasPrefix(key, "json:") {
				t.Errorf("element %s carries a json: key", element["@id"])
			}
		}
	}
}

// The reader reports malformed documents and objects that are not elements.
func TestReadAPIJSONRejectsNonElements(t *testing.T) {
	for name, data := range map[string]string{
		"malformed":             `{`,
		"top-level string":      `"hello"`,
		"missing @type":         `[{"@id": "X"}]`,
		"missing @id":           `[{"@type": "Package"}]`,
		"empty @id":             `[{"@type": "Package", "@id": ""}]`,
		"duplicate @id":         `[{"@type": "Package", "@id": "X"}, {"@type": "PartUsage", "@id": "X"}]`,
		"repeated @id key":      `[{"@type": "Package", "@id": "A", "@id": "B"}]`,
		"repeated @type key":    `[{"@type": "Package", "@type": "PartUsage", "@id": "X"}]`,
		"repeated member key":   `[{"@type": "Package", "@id": "X", "name": "a", "name": "b"}]`,
		"repeated array member": `[{"@type": "Package", "@id": "X", "ownedMember": [{"@id": "A"}, {"@id": "A"}]}]`,
		"unknown @key":          `[{"@type": "Package", "@id": "X", "@foo": 1}]`,
		"object without @id":    `[{"@type": "Package", "@id": "X", "ownedMember": {"@type": "Y"}}]`,
		"nested array":          `[{"@type": "Package", "@id": "X", "ownedMember": [[{"@id": "Y"}]]}]`,
		"null member":           `[{"@type": "Package", "@id": "X", "ownedMember": [null]}]`,
		"prefixed key":          `[{"@type": "Package", "@id": "X", "sysml:name": "n"}]`,
		"trailing JSON":         `[{"@type": "Package", "@id": "X"}] 42`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := export.ReadAPIJSON([]byte(data)); err == nil {
				t.Errorf("ReadAPIJSON(%s) succeeded, want an error", data)
			}
		})
	}
}

// Each JSON value form lands as the triple the mapping states it as.
func TestReadAPIJSONValueForms(t *testing.T) {
	document := `[{
		"@type": "Package", "@id": "P",
		"integer": 42, "decimal": 0.0, "double": 1.5e3, "flag": true,
		"absent": null, "empty": [], "declaredName": "P",
		"specializes": "A < B", "ownedMember": [{"@id": "Q"}, {"@id": "R"}],
		"sysx:sourceText": "package P {}",
		"sysx:indexes": [0, 1]
	}]`
	graph, err := export.ReadAPIJSON([]byte(document))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	subject := rdf.ElementIRIForID("P")
	for _, want := range []rdf.Triple{
		{Subject: subject, Predicate: rdf.SysMLTerm("integer"), Object: rdf.TypedLiteral("42", rdf.XSD+"integer")},
		{Subject: subject, Predicate: rdf.SysMLTerm("decimal"), Object: rdf.TypedLiteral("0.0", rdf.XSD+"decimal")},
		{Subject: subject, Predicate: rdf.SysMLTerm("double"), Object: rdf.TypedLiteral("1.5e3", rdf.XSD+"double")},
		{Subject: subject, Predicate: rdf.SysMLTerm("flag"), Object: rdf.Bool(true)},
		{Subject: subject, Predicate: rdf.SysMLTerm("declaredName"), Object: rdf.String("P")},
		{Subject: subject, Predicate: rdf.SysMLTerm("specializes"), Object: rdf.TypedLiteral("A < B", rdf.OpenSysML+"Expression")},
		{Subject: subject, Predicate: rdf.SysMLTerm("ownedMember"), Object: rdf.ElementIRIForID("Q")},
		{Subject: subject, Predicate: rdf.SysMLTerm("ownedMember"), Object: rdf.ElementIRIForID("R")},
		{Subject: subject, Predicate: rdf.OpenSysMLTerm("sourceText"), Object: rdf.String("package P {}")},
		{Subject: subject, Predicate: rdf.OpenSysMLTerm("indexes"), Object: rdf.TypedLiteral("0", rdf.XSD+"integer")},
		{Subject: subject, Predicate: rdf.OpenSysMLTerm("indexes"), Object: rdf.TypedLiteral("1", rdf.XSD+"integer")},
	} {
		if !graph.Has(want) {
			t.Errorf("missing triple %s %s %s", want.Subject, want.Predicate, want.Object)
		}
	}
	for _, absent := range []string{"absent", "empty"} {
		if graph.HasProperty(subject, rdf.SysML+absent) {
			t.Errorf("%s was dropped in the document but stated in the graph", absent)
		}
	}
	if graph.HasProperty(subject, rdf.AnnotationJSON+"indexes") {
		t.Error("a sysx: array was annotated; only a sysml: collection carries the annotation")
	}
	annotation, ok := graph.Object(subject, rdf.AnnotationJSON+"ownedMember")
	if !ok {
		t.Fatal("a sysml: array did not state its json: annotation")
	}
	want, err := rdf.CollectionJSON(subject, graph.Objects(subject, rdf.SysML+"ownedMember"))
	if err != nil {
		t.Fatal(err)
	}
	if annotation.Value != want {
		t.Errorf("json:ownedMember = %s, want %s", annotation.Value, want)
	}
}

// A scoped @id writes a scoped element IRI, and a single object document reads
// like a one-element array.
func TestReadAPIJSONScopedAndSingle(t *testing.T) {
	graph, err := export.ReadAPIJSON([]byte(`{"@type": "Package", "@id": "org.proj:X"}`))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	subject := rdf.ScopedElementIRIForID("org.proj", "X")
	if !graph.Has(rdf.Triple{Subject: subject, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("Package")}) {
		t.Errorf("the scoped element is not a subject:\n%s", rdf.WriteTurtle(graph))
	}
}

// An id under `_p` with an owner in the document and no qualifiedName names an
// expression node; every other shape names an element.
func TestReadAPIJSONExpressionClassification(t *testing.T) {
	document := `[
		{"@type": "AttributeUsage", "@id": "X", "value": {"@id": "X_pvalue"}},
		{"@type": "LiteralRational", "@id": "X_pvalue", "value": 0.0},
		{"@type": "OwningMembership", "@id": "X_pvalue_om", "memberElement": {"@id": "X_pvalue"}},
		{"@type": "LiteralRational", "@id": "Y_pvalue", "qualifiedName": "Y::pvalue"},
		{"@type": "PartUsage", "@id": "A_pfoo"},
		{"@type": "AttributeUsage", "@id": "W", "value": {"@id": "X_p2"}},
		{"@type": "LiteralRational", "@id": "X_p2", "qualifiedName": null, "value": 1.0}
	]`
	graph, err := export.ReadAPIJSON([]byte(document))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	for _, expr := range []string{"X_pvalue", "X_pvalue_om", "X_p2"} {
		subject := rdf.IRI(rdf.Expression + expr)
		if len(graph.Predicates(subject)) == 0 {
			t.Errorf("%q is not an expression-namespace subject:\n%s", expr, rdf.WriteTurtle(graph))
		}
	}
	for _, element := range []string{"Y_pvalue", "A_pfoo"} {
		subject := rdf.ElementIRIForID(element)
		if len(graph.Predicates(subject)) == 0 {
			t.Errorf("%q is not an element-namespace subject:\n%s", element, rdf.WriteTurtle(graph))
		}
	}
	owner := rdf.ElementIRIForID("X")
	value, ok := graph.Object(owner, rdf.SysML+"value")
	if !ok || value.Value != rdf.Expression+"X_pvalue" {
		t.Errorf("X's reference to X_pvalue = %v, want the expression IRI", value)
	}
	w := rdf.ElementIRIForID("W")
	value, ok = graph.Object(w, rdf.SysML+"value")
	if !ok || value.Value != rdf.Expression+"X_p2" {
		t.Errorf("W's reference to X_p2 = %v, want the expression IRI", value)
	}
}

// A real's JSON spelling is its XSD lexical, respelled only where JSON cannot
// carry it; the exponent letter is kept so the Turtle round trip does not move.
func TestWriteAPIJSONRealLexicals(t *testing.T) {
	subject := rdf.ElementIRIForID("X")
	for _, test := range []struct {
		lexical  string
		datatype string
		want     string
	}{
		{"1.5e3", rdf.XSD + "double", "1.5e3"},
		{"8.0E-9", rdf.XSD + "double", "8.0E-9"},
		{"2.E3", rdf.XSD + "double", "2.0E3"},
		{".5", rdf.XSD + "decimal", "0.5"},
		{"-.25", rdf.XSD + "decimal", "-0.25"},
		{"+2.5", rdf.XSD + "decimal", "2.5"},
		{"5.", rdf.XSD + "decimal", "5.0"},
		{"0.0", rdf.XSD + "decimal", "0.0"},
	} {
		lexical, want := test.lexical, test.want
		g := rdf.NewGraph()
		g.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Package"))
		g.Add(subject, rdf.SysMLTerm("value"), rdf.TypedLiteral(lexical, test.datatype))
		out, err := export.WriteAPIJSON(g)
		if err != nil {
			t.Fatalf("WriteAPIJSON(%q): %v", lexical, err)
		}
		if !strings.Contains(string(out), `"value": `+want) {
			t.Errorf("WriteAPIJSON(%q) wrote %s, want the number %s", lexical, out, want)
		}
	}
}

// The writer refuses what the element form cannot say rather than dropping it.
func TestWriteAPIJSONRefuses(t *testing.T) {
	element := rdf.ElementIRIForID("X")
	typed := func(objects ...rdf.Term) *rdf.Graph {
		g := rdf.NewGraph()
		for _, object := range objects {
			g.Add(element, rdf.IRI(rdf.RDFType), object)
		}
		return g
	}
	for name, build := range map[string]func() *rdf.Graph{
		"two rdf:types": func() *rdf.Graph {
			return typed(rdf.SysMLTerm("Package"), rdf.SysMLTerm("PartUsage"))
		},
		"no rdf:type": func() *rdf.Graph {
			g := typed()
			g.Add(element, rdf.SysMLTerm("declaredName"), rdf.String("x"))
			return g
		},
		"foreign predicate": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.IRI("http://example.com/prop"), rdf.String("x"))
			return g
		},
		"language tag": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("declaredName"), rdf.Term{Kind: rdf.TermLiteral, Value: "x", Lang: "en"})
			return g
		},
		"foreign subject": func() *rdf.Graph {
			g := rdf.NewGraph()
			other := rdf.IRI("http://example.com/subject")
			g.Add(other, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Package"))
			return g
		},
		"collection without annotation": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("ownedMember"), rdf.ElementIRIForID("A"))
			g.Add(element, rdf.SysMLTerm("ownedMember"), rdf.ElementIRIForID("B"))
			return g
		},
		"xsd:float": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("value"), rdf.TypedLiteral("1.5", rdf.XSD+"float"))
			return g
		},
		"xsd:int": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("value"), rdf.TypedLiteral("7", rdf.XSD+"int"))
			return g
		},
		"xsd:boolean spelled 1": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("value"), rdf.TypedLiteral("1", rdf.XSD+"boolean"))
			return g
		},
		"xsd:double without exponent": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("value"), rdf.TypedLiteral("1.5", rdf.XSD+"double"))
			return g
		},
		"expression text on a sysx: key": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.OpenSysMLTerm("sourceText"), rdf.TypedLiteral("a + b", rdf.OpenSysML+"Expression"))
			return g
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := export.WriteAPIJSON(build())
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Errorf("WriteAPIJSON = %v, want an UnsupportedError", err)
			}
		})
	}
}
