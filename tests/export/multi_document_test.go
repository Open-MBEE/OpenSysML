package export_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

func modelDocuments(t *testing.T, named ...string) []export.ModelDocument {
	t.Helper()
	var docs []export.ModelDocument
	for i := 0; i+1 < len(named); i += 2 {
		file := source.New(named[i], []byte(named[i+1]))
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Fatalf("%s: %v", named[i], p.Diagnostics)
		}
		docs = append(docs, export.ModelDocument{File: file, Root: root})
	}
	return docs
}

// A model of several documents converts as one graph: a reference from one
// document to an element another declares links that element's @id, as a
// reference within a document does, and each root names its document (#653).
func TestModelOfSeveralDocumentsLinksAcrossThem(t *testing.T) {
	graph, err := export.ModelToRDFWith(modelDocuments(t,
		"lib.sysml", "package Lib { part def Engine; }\n",
		"app.sysml", "package App { private import Lib::*; part e : Engine; }\n",
	), export.IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	out, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	documents := map[string]any{}
	var typing map[string]any
	for _, e := range elements {
		if name, ok := e["qualifiedName"].(string); ok {
			ids[name], _ = e["@id"].(string)
			if doc, ok := e["sysx:sourceDocument"]; ok {
				documents[name] = doc
			}
		}
		if e["@type"] == "FeatureTyping" {
			typing = e
		}
	}
	engine := ids["Lib::Engine"]
	if engine == "" {
		t.Fatalf("no Lib::Engine element in the model:\n%s", out)
	}
	if typing == nil {
		t.Fatalf("no FeatureTyping for e:\n%s", out)
	}
	if ref, _ := typing["type"].(map[string]any); ref["@id"] != engine {
		t.Errorf("e is typed by %v, want {\"@id\": %q}", typing["type"], engine)
	}
	if documents["Lib"] != "lib.sysml" || documents["App"] != "app.sysml" {
		t.Errorf("roots name their documents as %v, want Lib in lib.sysml and App in app.sysml", documents)
	}
}

// Every reference across the documents links a subject the model writes: a
// second reference to the same element, a member imported by name, a metadata
// usage, an enum value, and a redefinition of an unnamed redefining feature,
// whose name is its position in its own document.
func TestModelReferencesAcrossDocumentsLinkWrittenSubjects(t *testing.T) {
	graph, err := export.ModelToRDFWith(modelDocuments(t,
		"lib.sysml", `package Lib {
    metadata def Tag { attribute level : ScalarValues::Integer; }
    enum def Kind { enum a; enum b; }
    part def Base { attribute x : ScalarValues::Integer; }
    part def Engine :> Base {
        attribute :>> x default = 1;
        attribute kind : Kind;
    }
}
`,
		"app.sysml", `package App {
    private import Lib::*;
    private import Lib::Tag;
    part e1 : Engine { @Tag { level = 2; } attribute :>> kind = Kind::b; }
    part e2 : Engine { attribute :>> x = 3; }
}
`,
	), export.IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	out, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, e := range elements {
		id, _ := e["@id"].(string)
		ids[id] = true
	}
	var check func(owner string, key string, v any)
	check = func(owner, key string, v any) {
		switch v := v.(type) {
		case map[string]any:
			if ref, ok := v["@ref"]; ok {
				t.Errorf("%s.%s is the unresolved name %v", owner, key, ref)
			}
			if id, ok := v["@id"].(string); ok && (strings.HasPrefix(id, "Lib") || strings.HasPrefix(id, "App")) && !ids[id] {
				t.Errorf("%s.%s links %s, which the model does not write", owner, key, id)
			}
		case []any:
			for _, item := range v {
				check(owner, key, item)
			}
		}
	}
	for _, e := range elements {
		for key, v := range e {
			check(e["@id"].(string), key, v)
		}
	}
}

// Two documents declaring the same element would merge two elements into one
// subject; the model is refused, naming both declarations.
func TestModelOfSeveralDocumentsRefusesOneSubjectDeclaredTwice(t *testing.T) {
	_, err := export.ModelToRDFWith(modelDocuments(t,
		"a.sysml", "package P { part def A; }\n",
		"b.sysml", "package P { part def B; }\n",
	), export.IDQualifiedName)
	if err == nil || !strings.Contains(err.Error(), "a.sysml") || !strings.Contains(err.Error(), "b.sysml") {
		t.Errorf("a package declared in two documents: err = %v, want a refusal naming both", err)
	}
}

// Ids are scope-qualified when the model declares two identity scopes, though
// each document declares one: the same declared id in two projects is two
// elements, not one.
func TestModelOfSeveralDocumentsQualifiesIdsAcrossScopes(t *testing.T) {
	graph, err := export.ModelToRDFWith(modelDocuments(t,
		"one.sysml", `package P {
    @IdentityMetadata::ProjectRef { projectId = "one"; branch = "main"; }
    part def A { @IdentityMetadata::ElementId { id = "shared"; } }
}
`,
		"two.sysml", `package Q {
    @IdentityMetadata::ProjectRef { projectId = "two"; branch = "main"; }
    part def B { @IdentityMetadata::ElementId { id = "shared"; } }
}
`,
	), export.IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	subjects := map[string]string{}
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value == "https://www.omg.org/spec/SysML#qualifiedName" {
			subjects[triple.Object.Value] = triple.Subject.Value
		}
	}
	a, b := subjects["P::A"], subjects["Q::B"]
	if a == "" || b == "" || a == b {
		t.Errorf("P::A is %q and Q::B is %q, want two subjects", a, b)
	}
}

// Each document of a model is a RootNamespace of its own (KerML textual BNF,
// RootNamespace): in the API element form its roots are owned by an unnamed
// Namespace per document, which reading the form back drops again.
func TestModelOfSeveralDocumentsHasARootNamespacePerDocument(t *testing.T) {
	graph, err := export.ModelToRDFWith(modelDocuments(t,
		"lib.sysml", "package Lib { part def Engine; }\npackage Spares;\n",
		"app.sysml", "package App { private import Lib::*; part e : Engine; }\n",
	), export.IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	out, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	owners := map[string]string{}
	namespaces := 0
	for _, e := range elements {
		if e["@type"] == "Namespace" {
			namespaces++
		}
		if owner, ok := e["owner"].(map[string]any); ok && e["@type"] == "Package" {
			owners[e["@id"].(string)], _ = owner["@id"].(string)
		}
	}
	if namespaces != 2 {
		t.Errorf("%d root Namespaces, want one per document", namespaces)
	}
	if owners["Lib"] == "" || owners["Lib"] != owners["Spares"] || owners["App"] == "" || owners["App"] == owners["Lib"] {
		t.Errorf("packages are owned by %v, want Lib and Spares by one namespace and App by another", owners)
	}
	back, err := export.ReadAPIJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, subject := range back.Subjects() {
		if back.Type(subject) == "https://www.omg.org/spec/SysML#Namespace" {
			t.Errorf("reading the form back keeps the root namespace %s", subject.Value)
		}
	}
}

// A root Namespace is minted free of the ids every document's wrapper took,
// not only of the graph's subjects: a document whose root's id is `P_ns` next
// to one whose root is `P` gets its own namespace, and roots `Q` and `Q_om`
// their own memberships. Reading the form back drops them all.
func TestModelOfSeveralDocumentsMintsDistinctRootNamespaces(t *testing.T) {
	for _, model := range []struct {
		name string
		ids  [2]string
	}{
		{"namespace suffix", [2]string{"P", "P_ns"}},
		{"membership suffix", [2]string{"Q", "Q_om"}},
	} {
		for _, id := range []export.IDForm{export.IDQualifiedName, export.IDUUID} {
			t.Run(model.name+"/"+idFormName(id), func(t *testing.T) {
				graph, err := export.ModelToRDFWith(modelDocuments(t,
					"one.sysml", "package First { @IdentityMetadata::ElementId { id = \""+model.ids[0]+"\"; } part def A; }\n",
					"two.sysml", "package Second { @IdentityMetadata::ElementId { id = \""+model.ids[1]+"\"; } part def B; }\n",
				), id)
				if err != nil {
					t.Fatal(err)
				}
				out, err := export.WriteAPIJSON(graph)
				if err != nil {
					t.Fatal(err)
				}
				elements := rootElements(t, out)
				seen := map[string]bool{}
				namespaces := map[string]bool{}
				memberships := map[string]bool{}
				owners := map[string]string{}
				for _, element := range elements {
					elementID := rootString(t, element["@id"])
					if seen[elementID] {
						t.Errorf("@id %q appears twice:\n%s", elementID, out)
					}
					seen[elementID] = true
					switch rootString(t, element["@type"]) {
					case "Namespace":
						namespaces[elementID] = true
					case "OwningMembership":
						if namespaces[rootRef(t, element["owningRelatedElement"])] {
							memberships[elementID] = true
						}
					case "Package":
						owners[rootString(t, element["declaredName"])] = rootRef(t, element["owner"])
					}
				}
				if len(namespaces) != 2 {
					t.Errorf("%d root Namespaces, want one per document:\n%s", len(namespaces), out)
				}
				if len(memberships) != 2 {
					t.Errorf("%d root OwningMemberships, want one per document:\n%s", len(memberships), out)
				}
				first, second := owners["First"], owners["Second"]
				if first == "" || second == "" || first == second {
					t.Errorf("roots are owned by %v, want each by the namespace of its own document", owners)
				}
				back, err := export.ReadAPIJSON(out)
				if err != nil {
					t.Fatal(err)
				}
				if !sameTriples(tripleSet(graph), tripleSet(back)) {
					t.Errorf("reading the form back differs from the model:\n%v", diffTriples(tripleSet(back), tripleSet(graph)))
				}
			})
		}
	}
}

func idFormName(id export.IDForm) string {
	if id == export.IDUUID {
		return "uuid"
	}
	return "qualified"
}

// referencing marks every document but the named ones Referenced.
func referencing(docs []export.ModelDocument, written ...string) []export.ModelDocument {
	out := make([]export.ModelDocument, len(docs))
	for i, doc := range docs {
		doc.Referenced = !slices.Contains(written, doc.File.Name())
		out[i] = doc
	}
	return out
}

// apiElements converts a model to API JSON and keys each element by its @id.
func apiElements(t *testing.T, docs []export.ModelDocument) map[string]string {
	t.Helper()
	graph, err := export.ModelToRDFWith(docs, export.IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	out, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	byID := map[string]string{}
	for _, e := range elements {
		text, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		byID[e["@id"].(string)] = string(text)
	}
	return byID
}

// A model converted with only some of its documents written holds exactly the
// elements those documents contribute to the whole model's conversion: over
// every document, the conversions written one at a time partition it.
func TestModelWritesOnlyItsUnreferencedDocuments(t *testing.T) {
	docs := modelDocuments(t,
		"lib.sysml", `package Lib {
    metadata def Tag { attribute level : ScalarValues::Integer; }
    enum def Kind { enum a; enum b; }
    part def Base { attribute x : ScalarValues::Integer; }
    part def Engine :> Base {
        attribute :>> x default = 1;
        attribute kind : Kind;
    }
    state def Modes { entry; then off; state off; state on; transition off then on; }
}
`,
		"app.sysml", `package App {
    private import Lib::*;
    private import Lib::Tag;
    part e1 : Engine { @Tag { level = 2; } attribute :>> kind = Kind::b; }
    part e2 : Engine { attribute :>> x = 3; }
}
`,
		"fleet.sysml", `package Fleet {
    private import App::*;
    part spare :> e2;
    part pair[2] : Lib::Engine;
}
`,
	)
	whole := apiElements(t, docs)
	parts := map[string]string{}
	for _, doc := range docs {
		name := doc.File.Name()
		for id, element := range apiElements(t, referencing(docs, name)) {
			if prior, taken := parts[id]; taken {
				t.Errorf("%s writes %s, which another document also wrote:\n%s", name, id, prior)
			}
			parts[id] = element
		}
	}
	for id, element := range whole {
		if parts[id] != element {
			t.Errorf("%s: the whole model writes\n%s\nthe document declaring it writes\n%s", id, element, parts[id])
		}
	}
	for id := range parts {
		if _, ok := whole[id]; !ok {
			t.Errorf("%s is written by one document but not by the whole model", id)
		}
	}
}

// A referenced document is not written, but a reference into it links the
// element its own conversion writes.
func TestModelLinksIntoAReferencedDocument(t *testing.T) {
	docs := modelDocuments(t,
		"lib.sysml", "package Lib { part def Engine; }\n",
		"app.sysml", "package App { private import Lib::*; part e : Engine; }\n",
	)
	written := apiElements(t, referencing(docs, "app.sysml"))
	lib := apiElements(t, referencing(docs, "lib.sysml"))
	if _, ok := written["Lib__Engine"]; ok {
		t.Errorf("Lib::Engine is written though lib.sysml is referenced")
	}
	if _, ok := lib["Lib__Engine"]; !ok {
		t.Fatalf("lib.sysml written alone has no Lib__Engine: %v", lib)
	}
	linked := false
	for _, element := range written {
		linked = linked || strings.Contains(element, `"type":{"@id":"Lib__Engine"}`)
	}
	if !linked {
		t.Errorf("no element of app.sysml is typed by Lib__Engine: %v", written)
	}
}

// A referenced document still declares its elements: one whose id lands on a
// subject a written document mints is refused, as in the whole model, whether
// the id is derived from the name or declared.
func TestModelRefusesASubjectAReferencedDocumentAlsoDeclares(t *testing.T) {
	for _, tc := range []struct{ name, a, b string }{
		{"derived", "package P { part def A; }\n", "package P { part def B; }\n"},
		{"declared", `package P { part def A { @IdentityMetadata::ElementId { id = "shared"; } } }` + "\n",
			`package Q { part def B { @IdentityMetadata::ElementId { id = "shared"; } } }` + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			docs := modelDocuments(t, "a.sysml", tc.a, "b.sysml", tc.b)
			for _, written := range []string{"a.sysml", "b.sysml"} {
				_, err := export.ModelToRDFWith(referencing(docs, written), export.IDQualifiedName)
				if err == nil || !strings.Contains(err.Error(), "a.sysml") || !strings.Contains(err.Error(), "b.sysml") {
					t.Errorf("only %s written: err = %v, want a refusal naming both", written, err)
				}
			}
		})
	}
}
