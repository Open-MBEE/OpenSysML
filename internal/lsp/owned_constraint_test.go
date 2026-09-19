package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

const ownedConstraintSrc = `package P {
	constraint def C;
	constraint c0 : C;
	requirement def R {
		require constraint c : C default = c0;
	}
	requirement def S :> R {
		require constraint :>> c = c0;
	}
}`

func openOwnedConstraintDoc(t *testing.T) (*Server, string) {
	t.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/owned_constraint.sysml").Filename()
	ws.Open(name, []byte(ownedConstraintSrc), 1)
	return s, name
}

// A named `require constraint c` is a member of its requirement, so `:>> c`
// in a specializing requirement jumps to its declaration.
func TestDefinitionNamedOwnedConstraintRedefinition(t *testing.T) {
	s, name := openOwnedConstraintDoc(t)
	off := strings.LastIndex(ownedConstraintSrc, ":>> c") + len(":>> ")
	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(ownedConstraintSrc), off),
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	if locs[0].Range.Start.Line != 4 {
		t.Errorf("decl line = %d, want 4", locs[0].Range.Start.Line)
	}
}

func TestHoverNamedOwnedConstraint(t *testing.T) {
	s, name := openOwnedConstraintDoc(t)
	off := strings.Index(ownedConstraintSrc, "constraint c :") + len("constraint ")
	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(ownedConstraintSrc), off),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res == nil || !strings.Contains(res.Contents.Value, "constraint") || !strings.Contains(res.Contents.Value, "c") {
		t.Fatalf("hover = %+v, want a constraint usage named c", res)
	}
}

func TestDocumentSymbolListsNamedOwnedConstraint(t *testing.T) {
	s, name := openOwnedConstraintDoc(t)
	res, err := s.DocumentSymbol(context.Background(), &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
	})
	if err != nil {
		t.Fatalf("DocumentSymbol err = %v", err)
	}
	pkg, ok := res[0].(protocol.DocumentSymbol)
	if !ok {
		t.Fatalf("result[0] type = %T, want protocol.DocumentSymbol", res[0])
	}
	for _, child := range pkg.Children {
		if child.Name != "R" {
			continue
		}
		for _, member := range child.Children {
			if member.Name == "c" {
				return
			}
		}
		t.Fatalf("R.Children = %+v, want a member c", child.Children)
	}
	t.Fatalf("pkg.Children = %+v, want R", pkg.Children)
}
