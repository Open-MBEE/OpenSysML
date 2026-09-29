package export_test

import (
	"encoding/json"
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
