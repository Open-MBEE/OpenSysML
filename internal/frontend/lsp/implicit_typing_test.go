package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// implicitStateSrc references `done`, which an untyped state usage inherits from
// its implicit standard library base States::StateAction.
const implicitStateSrc = "package P {\n\tstate machine {\n\t\tstate normal;\n\t\tconstraint { Time::TimeOf(normal.done) > 0 [SI::s] }\n\t}\n}\n"

// TestPublishDiagnosticsAcceptsImplicitlyInheritedMember covers the editor view
// of implicit usage typing: a member reached through the implicit base must not
// be squiggled, matching what the resolver reports.
func TestPublishDiagnosticsAcceptsImplicitlyInheritedMember(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc

	name := "implicit.sysml"
	ws.Open(name, []byte(implicitStateSrc), 1)
	s.publishDiagnostics(context.Background(), name)

	if len(fc.published) != 1 {
		t.Fatalf("published count = %d, want 1", len(fc.published))
	}
	if diags := fc.published[0].Diagnostics; len(diags) != 0 {
		t.Fatalf("diagnostics = %+v, want none", diags)
	}
}

// TestHoverOnUntypedUsageStillReportsItsKind covers hover over the untyped usage
// itself: implicit typing must not change how the declaration is described.
func TestHoverOnUntypedUsageStillReportsItsKind(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/implicit_hover.sysml").Filename()
	ws.Open(name, []byte(implicitStateSrc), 1)
	defer ws.Close(name)

	off := strings.Index(implicitStateSrc, "state normal") + len("state ")
	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(implicitStateSrc), off),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res == nil || !strings.Contains(res.Contents.Value, "state") ||
		!strings.Contains(res.Contents.Value, "normal") {
		t.Fatalf("hover on `normal` = %+v, want its own kind and name", res)
	}
}
