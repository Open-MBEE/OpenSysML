package model

import (
	"bytes"
	"sync"
	"sync/atomic"
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

// Opens of one name under different kinds never show a reader the buffer of one
// parsed as the kind of the other.
func TestOpenWithKindKeepsKindAndBufferTogether(t *testing.T) {
	sysml, kerml := []byte("part def A;\n"), []byte("classifier B;\n")
	ws := NewWorkspace()
	ws.OpenWithKind("model.ipynb", sysml, 1, source.KindSysML)
	var writers, readers sync.WaitGroup
	var done atomic.Bool
	var mixed atomic.Int64
	for i := range 4 {
		writers.Go(func() {
			for range 2000 {
				if i%2 == 0 {
					ws.OpenWithKind("model.ipynb", sysml, 1, source.KindSysML)
				} else {
					ws.OpenWithKind("model.ipynb", kerml, 1, source.KindKerML)
				}
			}
		})
	}
	for range 2 {
		readers.Go(func() {
			for !done.Load() {
				doc := ws.Document("model.ipynb")
				want := source.KindSysML
				if bytes.Equal(doc.Content, kerml) {
					want = source.KindKerML
				}
				if doc.Kind() != want {
					mixed.Add(1)
				}
			}
		})
	}
	writers.Wait()
	done.Store(true)
	readers.Wait()
	if n := mixed.Load(); n != 0 {
		t.Fatalf("%d reads saw a buffer parsed as the other open's kind", n)
	}
}
