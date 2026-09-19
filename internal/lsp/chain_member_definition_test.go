package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Go-to-definition on a chain's last segment reaches the member a usage holds:
// named by redefinition, through an index, or inherited by subsetting.
func TestDefinitionChainMemberThroughUsage(t *testing.T) {
	src := `package P {
	item def Edge { item vertices [2]; }
	item def Surface;
	item def Polygon { item edges : Edge [3..*]; }
	item def Solid {
		item faces : Polygon [2..*] {
			ref :>> Polygon::edges;
		}
		item cf : Surface [1] :> faces;
		item edges = faces.edges;
		item firstVertices = edges#(1).vertices;
		item cfEdges = cf.edges;
	}
}`
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/chain_member.sysml").Filename()
	ws.Open(name, []byte(src), 1)

	declLine := func(anchor string) uint32 {
		t.Helper()
		off := strings.Index(src, anchor) + len(anchor) - 1
		locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     offsetToPosition([]byte(src), off),
			},
		})
		if err != nil {
			t.Fatalf("Definition at %q err = %v", anchor, err)
		}
		if len(locs) != 1 {
			t.Fatalf("locations at %q = %d, want 1", anchor, len(locs))
		}
		return locs[0].Range.Start.Line
	}

	// Lines are 0-based: `ref :>> Polygon::edges` is line 6, `item vertices` line 1.
	if got := declLine("= faces.edges"); got != 6 {
		t.Errorf("faces.edges defined at line %d, want 6", got)
	}
	if got := declLine("= edges#(1).vertices"); got != 1 {
		t.Errorf("edges#(1).vertices defined at line %d, want 1", got)
	}
	if got := declLine("= cf.edges"); got != 6 {
		t.Errorf("cf.edges defined at line %d, want 6", got)
	}
}
