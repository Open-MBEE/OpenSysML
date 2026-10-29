package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// TestElementIDsByIdentityAreTheWrittenIDs: the id minted for a declaration
// without writing is the elementId the written graph carries for it.
func TestElementIDsByIdentityAreTheWrittenIDs(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "..", "examples", "*.sysml"))
	texts := map[string]string{"dup.sysml": readerModel, "then.sysml": thenDoneModel}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		texts[filepath.Base(f)] = string(data)
	}
	compared := 0
	for name, text := range texts {
		sf := source.New(name, []byte(text))
		p := parser.New(sf)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			continue
		}
		docs := []ModelDocument{{File: sf, Root: root}}
		graph, _, encoders, err := modelToRDF(docs, IDQualifiedName)
		if err != nil {
			continue
		}
		minted, err := NewElementIDs(docs)
		if err != nil {
			t.Fatalf("%s: NewElementIDs: %v", name, err)
		}
		for _, e := range encoders {
			for node, fqn := range e.fqn {
				written, ok := graph.Lexical(e.ids.subjectForNode(node, fqn), rdf.SysML+pElementID)
				got, has := minted.bySpan[spanKey{name, node.Span()}]
				if has != ok || got != written {
					t.Errorf("%s: %s minted %q (%v), written %q (%v)", name, fqn, got, has, written, ok)
				}
				compared++
			}
		}
	}
	if compared == 0 {
		t.Fatal("no declaration compared")
	}
}

const thenDoneModel = "package P {\n\taction def A {\n\t\taction a;\n\t\tfirst start then a;\n\t\tthen done;\n\t}\n}\n"

// TestElementIDReaderWritesNoIDForAnAbsorbedFinalNode: the final node `then
// done;` introduces is written as its succession's end, so it has no id of its own.
func TestElementIDReaderWritesNoIDForAnAbsorbedFinalNode(t *testing.T) {
	sf, _, doc := readerIndex(t, "then.sysml", thenDoneModel)
	e, err := newEncoder(sf, doc.Root, "", IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	absorbed := 0
	for node := range e.absorbed {
		absorbed++
		if id, ok := NewElementIDReader([]ElementIDDocument{doc}).Read("then.sysml", node, e.fqn[node]); ok != nil || id != "" {
			t.Fatalf("the absorbed final node reads id %q (%v), want none", id, ok)
		}
	}
	if absorbed != 1 {
		t.Fatalf("%d absorbed final nodes, want 1", absorbed)
	}
}

// TestElementIDReaderRefusesCollidingIDs: ids an author declares that land on one
// IRI, or on another element's membership, refuse the model as Convert does.
func TestElementIDReaderRefusesCollidingIDs(t *testing.T) {
	for name, text := range map[string]string{
		"shared":     "package P {\n\t@IdentityMetadata::ProjectRef { projectId = \"proj-1\"; }\n\tpart def A { @IdentityMetadata::ElementId { id = \"shared\"; } }\n\tpart def B { @IdentityMetadata::ElementId { id = \"shared\"; } }\n}\n",
		"membership": "package P {\n\t@IdentityMetadata::ProjectRef { projectId = \"proj-1\"; }\n\tpart def A { @IdentityMetadata::ElementId { id = \"stable\"; } }\n\tpart def B { @IdentityMetadata::ElementId { id = \"stable_om\"; } }\n}\n",
	} {
		_, _, doc := readerIndex(t, name+".sysml", text)
		_, err := NewElementIDReader([]ElementIDDocument{doc}).Read("", nil, "P::A")
		if err == nil || !strings.Contains(err.Error(), "lands on the same IRI") {
			t.Fatalf("%s: err = %v, want the collision refused", name, err)
		}
		_, err = NewElementIDReader([]ElementIDDocument{doc, {File: source.New("other.sysml", []byte("package Q { part def C; }\n"))}}).Read("", nil, "P::A")
		if err == nil || !strings.Contains(err.Error(), "lands on the same IRI") {
			t.Fatalf("%s, with another document: err = %v, want the collision refused", name, err)
		}
	}
}
