package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// TestDefinitionFlowPayloadDeclaration covers navigation to the payload a
// message declares in its `of` clause: the declaration carries its own span, so
// the two same-named payloads below are distinct declarations rather than one
// zero-span symbol at the top of the file.
func TestDefinitionFlowPayloadDeclaration(t *testing.T) {
	src := `package P {
	item def FuelCommand;
	part def Endpoint { event occurrence sent; event occurrence received; }
	occurrence def I {
		ref part sender : Endpoint;
		ref part receiver : Endpoint;
		message m of fuelCommand : FuelCommand from sender.sent to receiver.received;
		message n of fuelCommand : FuelCommand = m.fuelCommand from sender.sent to receiver.received;
	}
}`
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/flow_payload.sysml").Filename()
	ws.Open(name, []byte(src), 1)

	firstDecl := strings.Index(src, "of fuelCommand") + len("of ")
	secondDecl := strings.Index(src, "message n") + strings.Index(src[strings.Index(src, "message n"):], "of fuelCommand") + len("of ")
	declLine := func(off int) uint32 {
		t.Helper()
		locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     offsetToPosition([]byte(src), off),
			},
		})
		if err != nil {
			t.Fatalf("Definition err = %v", err)
		}
		if len(locs) != 1 {
			t.Fatalf("locations = %d, want 1", len(locs))
		}
		return locs[0].Range.Start.Line
	}

	// Lines are 0-based: `message m` is line 6, `message n` line 7.
	if got := declLine(firstDecl); got != 6 {
		t.Errorf("first payload declaration line = %d, want 6", got)
	}
	if got := declLine(secondDecl); got != 7 {
		t.Errorf("second payload declaration line = %d, want 7", got)
	}
}
