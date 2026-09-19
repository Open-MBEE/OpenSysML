package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

const w6bAliasSrc = "package ShapeItems {\n\tpart def Cube {\n\t\tattribute length;\n\t}\n" +
	"\talias Box for Cube;\n}\npackage Uses {\n\tprivate import ShapeItems::Box;\n" +
	"\tpart b : Box;\n}\n"

// Go-to-definition on a reference written through an alias lands on the aliased
// element's declaration, which is the element the reference reaches.
func TestW6BDefinitionThroughAliasLandsOnTheTarget(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/w6b_alias_def.sysml").Filename()
	ws.Open(name, []byte(w6bAliasSrc), 1)

	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(w6bAliasSrc), strings.Index(w6bAliasSrc, "part b : Box")+len("part b : ")),
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	// `part def Cube` is on line 1; the alias declaration is on line 4.
	if locs[0].Range.Start.Line != 1 {
		t.Fatalf("definition line = %d, want 1 (part def Cube)", locs[0].Range.Start.Line)
	}
}

// Hover on the alias declaration still describes the alias: it has a name of its
// own, which is what the cursor is on.
func TestW6BHoverOnTheAliasDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/w6b_alias_hover.sysml").Filename()
	ws.Open(name, []byte(w6bAliasSrc), 1)

	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(w6bAliasSrc), strings.Index(w6bAliasSrc, "Box for Cube")),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res == nil || !strings.Contains(res.Contents.Value, "alias Box") {
		t.Fatalf("hover = %v, want \"alias Box\"", res)
	}
}

// The alias still has a name, so completion in its namespace offers it.
func TestW6BCompletionStillOffersTheAliasName(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/w6b_alias_completion.sysml").Filename()
	src := "package ShapeItems {\n\tpart def Cube;\n\talias Box for Cube;\n\tpart p : \n}\n"
	ws.Open(name, []byte(src), 1)

	list, err := s.Completion(context.Background(), &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), strings.Index(src, "part p : ")+len("part p : ")),
		},
	})
	if err != nil {
		t.Fatalf("Completion err = %v", err)
	}
	var hasAlias, hasTarget bool
	for _, it := range list.Items {
		switch it.Label {
		case "Box":
			hasAlias = true
		case "Cube":
			hasTarget = true
		}
	}
	if !hasAlias || !hasTarget {
		t.Fatalf("completion has Box = %v, Cube = %v; want both", hasAlias, hasTarget)
	}
}

// A qualifier written as an alias reaches the target's members: completion after
// `ShapeItems::Box::` lists what Cube declares.
func TestW6BMembersThroughAliasAreTheTargets(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/w6b_alias_members.sysml").Filename()
	src := "package ShapeItems {\n\tpart def Cube {\n\t\tattribute length;\n\t}\n" +
		"\talias Box for Cube;\n}\npackage Uses {\n\tpart b : ShapeItems::Box::"
	ws.Open(name, []byte(src), 1)

	list, err := s.Completion(context.Background(), &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), len(src)),
		},
	})
	if err != nil {
		t.Fatalf("Completion err = %v", err)
	}
	for _, it := range list.Items {
		if it.Label == "length" {
			return
		}
	}
	t.Fatalf("completion through the alias does not offer Cube's 'length'")
}
