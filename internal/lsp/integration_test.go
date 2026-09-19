package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func TestLSPEndToEnd(t *testing.T) {
	ctx := context.Background()
	ws := model.NewWorkspace()
	s := NewServer(ws)

	// Lifecycle.
	initRes, err := s.Initialize(ctx, &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	if initRes.Capabilities.HoverProvider == nil {
		t.Fatal("HoverProvider not advertised")
	}
	if err := s.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatalf("Initialized err = %v", err)
	}

	// Open a document.
	name := "e2e.sysml"
	src := "package P { namespace N; }\nimport P::N;\n"
	docURI := uri.File(name)
	if err := s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        docURI,
			LanguageID: "sysml",
			Version:    1,
			Text:       src,
		},
	}); err != nil {
		t.Fatalf("DidOpen err = %v", err)
	}

	posOf := func(sub string, nth int) protocol.Position {
		idx := -1
		for i := 0; i < nth; i++ {
			idx = strings.Index(src[idx+1:], sub) + idx + 1
		}
		return offsetToPosition([]byte(src), idx)
	}

	// Hover on the declaration N (first "N").
	hov, err := s.Hover(ctx, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Position:     posOf("N", 1),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if hov == nil || !strings.Contains(hov.Contents.Value, "N") {
		t.Errorf("Hover = %+v, want content mentioning N", hov)
	}

	// Definition from the reference "N" in "import P::N" (second "N").
	defs, err := s.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Position:     posOf("N", 2),
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(defs) != 1 {
		t.Errorf("Definition = %d locations, want 1", len(defs))
	}

	// References for the declaration P (IncludeDeclaration).
	refs, err := s.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Position:     posOf("P", 1),
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	if len(refs) < 1 {
		t.Errorf("References = %d, want >= 1", len(refs))
	}

	// documentSymbol.
	syms, err := s.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
	})
	if err != nil {
		t.Fatalf("DocumentSymbol err = %v", err)
	}
	if len(syms) != 1 {
		t.Errorf("DocumentSymbol top-level = %d, want 1", len(syms))
	}

	// workspace/symbol.
	wsyms, err := s.Symbols(ctx, &protocol.WorkspaceSymbolParams{Query: "P"})
	if err != nil {
		t.Fatalf("Symbols err = %v", err)
	}
	if len(wsyms) < 1 {
		t.Errorf("Symbols(P) = %d, want >= 1", len(wsyms))
	}

	// completion.
	comp, err := s.Completion(ctx, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Position:     offsetToPosition([]byte(src), len(src)),
		},
	})
	if err != nil {
		t.Fatalf("Completion err = %v", err)
	}
	if comp == nil || len(comp.Items) == 0 {
		t.Error("Completion returned no items")
	}

	// Shutdown / Exit.
	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown err = %v", err)
	}
	if err := s.Exit(ctx); err != nil {
		t.Fatalf("Exit err = %v", err)
	}
}
