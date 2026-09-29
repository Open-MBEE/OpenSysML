package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// multiDocModel is the issue's library/application pair: the application
// document's `part e : Engine` resolves into the library document, so its
// FeatureTyping links to Lib::Engine rather than carrying the name as text.
var multiDocModel = []convert.Source{
	{Name: "lib.sysml", Data: []byte("package Lib {\n\titem def Engine;\n}\n")},
	{Name: "app.sysml", Data: []byte("package App {\n\timport Lib::*;\n\tpart e : Engine;\n}\n")},
}

// refsOf is rootRefs where the property may be a lone reference rather than a
// one-element array, as single-valued ends are spelled in the element form.
func refsOf(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var refs []json.RawMessage
	if err := json.Unmarshal(raw, &refs); err == nil {
		out := make([]string, 0, len(refs))
		for _, ref := range refs {
			out = append(out, rootRef(t, ref))
		}
		return out
	}
	return []string{rootRef(t, raw)}
}

func multiDocGraph(t *testing.T) *rdf.Graph {
	t.Helper()
	docs := make([]export.Document, len(multiDocModel))
	for i, src := range multiDocModel {
		file := source.New(src.Name, src.Data)
		root := parser.New(file).ParseFile()
		docs[i] = export.Document{File: file, Root: root}
	}
	graph, err := export.ToRDFDocuments(docs, export.IDQualifiedName)
	if err != nil {
		t.Fatalf("ToRDFDocuments: %v", err)
	}
	return graph
}

// A document of a multi-document model resolves against the others, so a
// reference into another document links to the element it declares there, in
// Turtle and in the API element form alike, and each document's top-level
// elements carry its name as sysx:sourceDocument.
func TestCrossDocumentReferenceLinks(t *testing.T) {
	turtle := rdf.WriteTurtle(multiDocGraph(t))
	for _, want := range []string{
		"sysx:sourceDocument \"lib.sysml\"",
		"sysx:sourceDocument \"app.sysml\"",
		"sysml:type elmt:Lib__Engine",
	} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the graph should state %q:\n%s", want, turtle)
		}
	}

	out, err := convert.ConvertDocuments(multiDocModel, convert.FormatAPIJSON, convert.Options{})
	if err != nil {
		t.Fatalf("ConvertDocuments: %v", err)
	}
	elements := rootElements(t, out)
	var typingFound, namespaces int
	namespaceMembers := map[string][]string{}
	for _, el := range elements {
		switch rootString(t, el["@type"]) {
		case "FeatureTyping":
			for _, target := range refsOf(t, el["type"]) {
				if target == "Lib__Engine" && rootRef(t, el["typedFeature"]) == "App__e" {
					typingFound++
				}
			}
		case "Namespace":
			namespaces++
			for _, member := range refsOf(t, el["ownedMember"]) {
				namespaceMembers[rootString(t, el["@id"])] = append(namespaceMembers[rootString(t, el["@id"])], member)
			}
		}
	}
	if typingFound != 1 {
		t.Errorf("expected one FeatureTyping App__e -> Lib__Engine, found %d:\n%s", typingFound, out)
	}
	if namespaces != 2 {
		t.Errorf("expected one root Namespace per document, found %d:\n%s", namespaces, out)
	}
	var libNS, appNS string
	for ns, members := range namespaceMembers {
		for _, member := range members {
			switch member {
			case "Lib":
				libNS = ns
			case "App":
				appNS = ns
			}
		}
	}
	if libNS == "" || appNS == "" {
		t.Fatalf("the documents' roots are not each under a root Namespace: %v", namespaceMembers)
	}
	if libNS == appNS {
		t.Errorf("Lib and App share the root Namespace %s; each document's roots wrap under their own", libNS)
	}
}

// Two documents declaring one element IRI are refused the way one document
// declaring it twice is, rather than merged into one subject.
func TestTwoDocumentsMintingOneIRIAreRefused(t *testing.T) {
	sources := []convert.Source{
		{Name: "a.sysml", Data: []byte("package P {\n\tpart def A;\n}\n")},
		{Name: "b.sysml", Data: []byte("package P {\n\tpart def B;\n}\n")},
	}
	if _, err := convert.ConvertDocuments(sources, convert.FormatTurtle, convert.Options{}); err == nil {
		t.Error("two documents declaring package P were merged into one element")
	}
}

// The API element form of a multi-document model reads back to one notation
// holding both documents' roots in order, and re-encoding that notation agrees
// structurally once the document provenance and the root-namespace ids are
// ignored.
func TestMultiDocumentGraphReadsBack(t *testing.T) {
	out, err := convert.ConvertDocuments(multiDocModel, convert.FormatAPIJSON, convert.Options{})
	if err != nil {
		t.Fatalf("ConvertDocuments: %v", err)
	}
	back, err := convert.Convert("m.json", out, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	text := string(back)
	if !strings.Contains(text, "package Lib") || !strings.Contains(text, "package App") {
		t.Fatalf("the notation lost a document's roots:\n%s", text)
	}
	if strings.Index(text, "package Lib") > strings.Index(text, "package App") {
		t.Errorf("the documents' roots are out of order:\n%s", text)
	}
	if !strings.Contains(text, "part e : Engine") {
		t.Errorf("the cross-document reference lost its link:\n%s", text)
	}
}

// ConvertDocuments refuses a target that is not a graph, and reports the
// second document's syntax errors against its own name.
func TestConvertDocumentsRefusals(t *testing.T) {
	if _, err := convert.ConvertDocuments(multiDocModel, convert.FormatSysML, convert.Options{}); err == nil ||
		!strings.Contains(err.Error(), "graph") {
		t.Errorf("a notation target should be refused, got %v", err)
	}
	broken := []convert.Source{
		{Name: "ok.sysml", Data: []byte("package Ok {}\n")},
		{Name: "broken.sysml", Data: []byte("part def {")},
	}
	_, err := convert.ConvertDocuments(broken, convert.FormatTurtle, convert.Options{})
	if err == nil {
		t.Fatal("a broken second document converted")
	}
	if !strings.Contains(err.Error(), "broken.sysml") {
		t.Errorf("the syntax error is not reported against the second document: %v", err)
	}
}

// A `verify` reference in a later document of the model materializes the same
// way it does in one document: the materialization pass reads the whole model's
// shared reference set, not one document's.
func TestVerifyReferenceAcrossDocumentsMaterializesTheSame(t *testing.T) {
	docV := "package V {\n\trequirement r;\n\trequirement def P {\n\t\trequirement req;\n\t}\n}\n"
	docW := "package W {\n\tverification def T {\n\t\tobjective : V::P {\n\t\t\tverify V::r :>> req;\n\t\t}\n\t}\n}\n"

	graphOf := func(sources []convert.Source) *rdf.Graph {
		docs := make([]export.Document, len(sources))
		for i, src := range sources {
			file := source.New(src.Name, src.Data)
			docs[i] = export.Document{File: file, Root: parser.New(file).ParseFile()}
		}
		graph, err := export.ToRDFDocuments(docs, export.IDQualifiedName)
		if err != nil {
			t.Fatalf("ToRDFDocuments: %v", err)
		}
		return graph
	}
	oneDocument := graphOf([]convert.Source{{Name: "one.sysml", Data: []byte(docV + docW)}})
	twoDocuments := graphOf([]convert.Source{
		{Name: "v.sysml", Data: []byte(docV)},
		{Name: "w.sysml", Data: []byte(docW)},
	})

	// The metaclass of the relationship the verify member's `verify V::r`
	// materializes: the relationship element owned by the verify usage whose
	// general end is V::r, in each graph.
	verifyRelationship := func(graph *rdf.Graph) string {
		var metaclass string
		for _, subject := range graph.Subjects() {
			for _, target := range graph.Objects(subject, rdf.SysML+"subsets") {
				if name, _ := graph.Lexical(target, rdf.SysML+"qualifiedName"); name != "V::r" {
					continue
				}
				for _, rel := range graph.Objects(subject, rdf.SysML+"ownedRelationship") {
					switch ontology.LocalName(graph.Type(rel)) {
					case "Subsetting", "ReferenceSubsetting":
						if general, ok := graph.Object(rel, rdf.SysML+"general"); ok && general == target {
							metaclass = ontology.LocalName(graph.Type(rel))
						}
					}
				}
			}
		}
		return metaclass
	}
	want := verifyRelationship(oneDocument)
	if want == "" {
		t.Fatal("the single-document graph holds no materialized verify relationship")
	}
	if got := verifyRelationship(twoDocuments); got != want {
		t.Errorf("the two-document graph materialized the verify's :>> as %s, the one-document graph as %s", got, want)
	}

	// Stripped of its source text the two-document graph reads back the same
	// notation the one-document graph does, keeping the verify's reference
	// form (`verify V::r`, with its redefinition spelled canonically).
	readBack := func(graph *rdf.Graph) string {
		turtle := rdf.WriteTurtle(graph)
		stripped := withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:sourceTail")
		back, err := convert.Convert("m.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
		if err != nil {
			t.Fatalf("back to notation: %v", err)
		}
		return string(back)
	}
	back, wantBack := readBack(twoDocuments), readBack(oneDocument)
	if back != wantBack {
		t.Errorf("the two-document read-back differs from the one-document one:\n--- two ---\n%s--- one ---\n%s", back, wantBack)
	}
	if !strings.Contains(back, "verify V::r") {
		t.Errorf("the notation should keep the verify's reference form:\n%s", back)
	}
}

// A document that declares no elements contributes no root Namespace: a graph
// states elements, not files.
func TestAnEmptyDocumentAddsNoRootNamespace(t *testing.T) {
	sources := []convert.Source{
		{Name: "lib.sysml", Data: []byte("package Lib {\n\titem def Engine;\n}\n")},
		{Name: "empty.sysml", Data: []byte("")},
	}
	out, err := convert.ConvertDocuments(sources, convert.FormatAPIJSON, convert.Options{})
	if err != nil {
		t.Fatalf("ConvertDocuments: %v", err)
	}
	namespaces := 0
	for _, el := range rootElements(t, out) {
		if rootString(t, el["@type"]) == "Namespace" {
			namespaces++
		}
	}
	if namespaces != 1 {
		t.Errorf("expected exactly the one root Namespace the declaring document wraps under, found %d:\n%s", namespaces, out)
	}
}

// Every document group's root Namespace and membership ids draw from one
// minted-id set: a namespace one group mints is not in the graph when the next
// group mints, and a root whose own id ends in `_ns` or `_om` must not land
// its wrapper's id on the one another group already picked. Element ids escape
// underscores, so the colliding names can only arrive written out in a graph.
func TestRootNamespacesOfSeveralDocumentsDoNotCollide(t *testing.T) {
	turtle := []byte(`@prefix elmt: <urn:sysmlv2:element:> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .

elmt:B
    a sysml:Package ;
    sysml:qualifiedName "B" ;
    sysx:sourceDocument "b.sysml" ;
    sysml:declaredName "B" .

elmt:B_ns
    a sysml:Package ;
    sysml:qualifiedName "B_ns" ;
    sysx:sourceDocument "c.sysml" ;
    sysml:declaredName "B_ns" .

elmt:B_om
    a sysml:Package ;
    sysml:qualifiedName "B_om" ;
    sysx:sourceDocument "d.sysml" ;
    sysml:declaredName "B_om" .
`)
	out, err := convert.Convert("collide.ttl", turtle, convert.FormatTurtle, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	namespaces := map[string][]string{}
	for _, el := range rootElements(t, out) {
		if rootString(t, el["@type"]) == "Namespace" {
			for _, member := range refsOf(t, el["ownedMember"]) {
				namespaces[rootString(t, el["@id"])] = append(namespaces[rootString(t, el["@id"])], member)
			}
		}
	}
	if len(namespaces) != 3 {
		t.Fatalf("expected one root Namespace per document, found %d: %v", len(namespaces), namespaces)
	}
	for _, root := range []string{"B", "B_ns", "B_om"} {
		var owners int
		for ns, members := range namespaces {
			for _, member := range members {
				if member == root {
					owners++
					_ = ns
				}
			}
		}
		if owners != 1 {
			t.Errorf("expected exactly one root Namespace owning %s, found %d: %v", root, owners, namespaces)
		}
	}
}
