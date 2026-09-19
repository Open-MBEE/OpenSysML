package lsp

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func TestDocumentSymbolReturnsTree(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	// Open under the resolved absolute name so it round-trips through
	// uri.File(name).Filename() inside the handler.
	name := uri.File("/tmp/d.sysml").Filename()
	ws.Open(name, []byte("package P { namespace N; }"), 1)

	res, err := s.DocumentSymbol(context.Background(), &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
	})
	if err != nil {
		t.Fatalf("DocumentSymbol err = %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("top-level symbols = %d, want 1", len(res))
	}
	pkg, ok := res[0].(protocol.DocumentSymbol)
	if !ok {
		t.Fatalf("result[0] type = %T, want protocol.DocumentSymbol", res[0])
	}
	if pkg.Name != "P" || pkg.Kind != protocol.SymbolKindPackage {
		t.Errorf("pkg = %q kind %v, want P/Package", pkg.Name, pkg.Kind)
	}
	if len(pkg.Children) != 1 || pkg.Children[0].Name != "N" {
		t.Fatalf("pkg.Children = %+v, want [N]", pkg.Children)
	}
	if pkg.Children[0].Kind != protocol.SymbolKindNamespace {
		t.Errorf("N kind = %v, want Namespace", pkg.Children[0].Kind)
	}
}
