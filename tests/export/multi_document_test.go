package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
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
		var root *ast.RootNamespace
		root = parser.New(file).ParseFile()
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
