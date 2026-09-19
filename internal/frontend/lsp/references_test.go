package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func TestReferencesFindsUses(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	// Use an absolute name so uri.File(name).Filename() round-trips to the same key.
	name := uri.File("/tmp/ref.sysml").Filename()
	// "N" is declared once (namespace N) and referenced twice (P::N in two imports).
	src := "package P { namespace N; }\nimport P::N;\nimport P::N;"
	ws.Open(name, []byte(src), 1)

	// Cursor on the declaration "N" inside "namespace N;".
	off := strings.Index(src, "N")
	pos := offsetToPosition([]byte(src), off)

	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     pos,
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	// The two "P::N" imports each resolve their terminal segment to N => 2 references.
	if len(locs) != 2 {
		t.Fatalf("references = %d, want 2", len(locs))
	}
	for _, l := range locs {
		if l.URI != uri.File(name) {
			t.Errorf("URI = %q, want %q", l.URI, uri.File(name))
		}
	}

	// The reference range must narrow to the terminal "N" segment, not the whole
	// "P::N" dotted name. In "import P::N;", "N" sits one column after "P::".
	for _, l := range locs {
		off := positionToOffset([]byte(src), l.Range.Start)
		if got := src[off]; got != 'N' {
			t.Errorf("reference range starts at %q (offset %d), want 'N'", got, off)
		}
		endOff := positionToOffset([]byte(src), l.Range.End)
		if endOff-off != 1 {
			t.Errorf("reference range length = %d, want 1 (just \"N\")", endOff-off)
		}
	}
}

func TestReferencesFromUseSiteIncludingDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/ref2.sysml").Filename()
	src := "package P { namespace N; }\nimport P::N;\nimport P::N;"
	ws.Open(name, []byte(src), 1)

	// Cursor on the terminal "N" of the FIRST "P::N" import (a use site, exercising
	// the refAtOffset branch), with IncludeDeclaration:true.
	useLine := strings.Index(src, "import P::N;")
	off := strings.Index(src[useLine:], "N") + useLine
	pos := offsetToPosition([]byte(src), off)

	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     pos,
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	// Declaration + two references = 3 locations.
	if len(locs) != 3 {
		t.Fatalf("references = %d, want 3", len(locs))
	}
}

func TestReferencesSpanDocuments(t *testing.T) {
	s, _, _, lib, main := multiFileWorkspace(t)
	openFile(t, s, main, mainSource)

	// Cursor on the declaration of Widget in lib.sysml, which main.sysml uses.
	pos := offsetToPosition([]byte(libSource), strings.Index(libSource, "Widget"))
	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(lib)},
			Position:     pos,
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("references = %d (%v), want 1 in main.sysml", len(locs), locs)
	}
	if locs[0].URI != uri.File(main) {
		t.Errorf("URI = %q, want %q", locs[0].URI, uri.File(main))
	}
	off := positionToOffset([]byte(mainSource), locs[0].Range.Start)
	if got := mainSource[off : off+len("Widget")]; got != "Widget" {
		t.Errorf("reference range covers %q, want %q", got, "Widget")
	}
}

func TestReferencesFromUseSiteSpanDocuments(t *testing.T) {
	s, _, _, lib, main := multiFileWorkspace(t)
	openFile(t, s, main, mainSource)

	// Cursor on the use of Widget in main.sysml: the declaration it resolves to
	// lives in another document, and is reported there.
	pos := offsetToPosition([]byte(mainSource), strings.Index(mainSource, "Widget"))
	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(main)},
			Position:     pos,
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	if len(locs) != 2 {
		t.Fatalf("references = %d (%v), want declaration + one use", len(locs), locs)
	}
	if locs[0].URI != uri.File(lib) {
		t.Errorf("declaration URI = %q, want %q", locs[0].URI, uri.File(lib))
	}
}

func TestReferencesCursorOnNothing(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/ref3.sysml").Filename()
	src := "package P { namespace N; }\nimport P::N;"
	ws.Open(name, []byte(src), 1)

	// Cursor on the "import" keyword (not a symbol declaration, not a ref QN).
	pos := offsetToPosition([]byte(src), strings.Index(src, "import"))
	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     pos,
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	if len(locs) != 0 {
		t.Fatalf("references = %d, want 0", len(locs))
	}
}
