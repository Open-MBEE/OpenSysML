package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// OpenWithKind parses a buffer whose name tells no language as the kind given,
// keeps the kind across an Update, parses unchanged text again when only the
// kind changes, and forgets the kind when the buffer closes.
func TestOpenWithKindParsesTheBufferAsTheKindGiven(t *testing.T) {
	ws := NewWorkspace()
	text := []byte("part def A;\n")
	ws.OpenWithKind("model.ipynb", text, 1, source.KindSysML)
	doc := ws.Document("model.ipynb")
	if doc == nil || doc.Kind() != source.KindSysML || len(doc.ParseDiagnostics) != 0 {
		t.Fatalf("opened as SysML: kind %v, diagnostics %v", doc.Kind(), doc.ParseDiagnostics)
	}
	ws.Update("model.ipynb", []byte("part def B;\n"), 2)
	if doc = ws.Document("model.ipynb"); doc.Kind() != source.KindSysML || len(doc.ParseDiagnostics) != 0 {
		t.Fatalf("updated: kind %v, diagnostics %v", doc.Kind(), doc.ParseDiagnostics)
	}
	ws.OpenWithKind("model.ipynb", []byte("part def B;\n"), 3, source.KindKerML)
	if doc = ws.Document("model.ipynb"); doc.Kind() != source.KindKerML || ws.index.DocumentKind("model.ipynb") != source.KindKerML {
		t.Fatalf("same text as KerML: document/index kinds = %v/%v, want KerML", doc.Kind(), ws.index.DocumentKind("model.ipynb"))
	}
	ws.Close("model.ipynb")
	if ws.Document("model.ipynb") != nil || len(ws.kinds) != 0 {
		t.Fatalf("after close: document %v, kinds %v", ws.Document("model.ipynb"), ws.kinds)
	}
}
