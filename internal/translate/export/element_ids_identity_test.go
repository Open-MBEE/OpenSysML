package export

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// TestElementIDsByIdentityAreTheWrittenIDs: the id minted for a declaration
// without writing is the elementId the written graph carries for it.
func TestElementIDsByIdentityAreTheWrittenIDs(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "..", "examples", "*.sysml"))
	if len(files) > 8 {
		files = files[:8]
	}
	texts := map[string]string{"dup.sysml": readerModel}
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
				if !ok {
					continue
				}
				if got := minted.bySpan[spanKey{name, node.Span()}]; got != written {
					t.Errorf("%s: %s minted %q, written %q", name, fqn, got, written)
				}
				compared++
			}
		}
	}
	if compared == 0 {
		t.Fatal("no declaration compared")
	}
}
