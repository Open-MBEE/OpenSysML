package lsp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// openLargeDocument opens one document of count part definitions and returns
// the server with the position of the last one's name.
func openLargeDocument(tb testing.TB, count int) (*Server, protocol.TextDocumentPositionParams) {
	tb.Helper()
	var text strings.Builder
	text.WriteString("package Large {\n")
	for i := range count {
		fmt.Fprintf(&text, "\tpart def D%05d { attribute mass : ScalarValues::Real; }\n", i)
	}
	text.WriteString("}\n")
	name := uri.File("/tmp/posbench/large.sysml")
	ws := model.NewWorkspace()
	ws.Open(name.FsPath(), []byte(text.String()), 1)
	last := fmt.Sprintf("D%05d", count-1)
	return NewServer(ws), protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: name},
		Position:     offsetToPosition([]byte(text.String()), strings.Index(text.String(), last)),
	}
}

// A position request converts through the document's cached line index: the
// range it answers is the name under the cursor, however large the document.
func TestPrepareRenameAnswersTheNameRange(t *testing.T) {
	s, at := openLargeDocument(t, 300)
	got, err := s.PrepareRename(context.Background(), &protocol.PrepareRenameParams{TextDocumentPositionParams: at})
	if err != nil {
		t.Fatal(err)
	}
	want := protocol.Range{Start: at.Position, End: protocol.Position{Line: at.Position.Line, Character: at.Position.Character + 6}}
	if rng, ok := got.(*protocol.Range); !ok || *rng != want {
		t.Fatalf("PrepareRename = %+v, want %+v", got, want)
	}
}

// BenchmarkSignatureHelpLarge converts one position late in a 20,000-line document.
func BenchmarkSignatureHelpLarge(b *testing.B) {
	s, at := openLargeDocument(b, 20000)
	params := &protocol.SignatureHelpParams{TextDocumentPositionParams: at}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.SignatureHelp(context.Background(), params); err != nil {
			b.Fatal(err)
		}
	}
}
