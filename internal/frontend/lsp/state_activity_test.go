package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// stateActivitySrc imports the StateActivity extension, so `fill.isActive` is
// the extension's Boolean feature read off the state usage fill.
const stateActivitySrc = `package P {
	private import ScalarValues::*;
	private import StateActivity::*;
	state def Step {
		attribute level : Real = 0.0;
		state fill;
		state drain;
		transition first fill accept when level > 1.0 then drain;
		assert constraint c { fill.isActive == true }
		attribute probe : Boolean = fill.
	}
}
`

// Completion after `fill.` offers the extension's isActive with the import, and
// not without it: the member is visible only through the import.
func TestCompletionOnStateOffersIsActiveOnlyWithTheImport(t *testing.T) {
	items := completionAt(t, stateActivitySrc, "attribute probe : Boolean = fill.")
	item, ok := items["isActive"]
	if !ok {
		t.Fatalf("completion after 'fill.' missing 'isActive'; got %v", labelsOf(items))
	}
	if !strings.Contains(item.Detail, "Boolean") {
		t.Errorf("'isActive' detail = %q, want it to name Boolean", item.Detail)
	}
	if _, ok := items["substates"]; !ok {
		t.Errorf("completion after 'fill.' lost the standard member 'substates'; got %v", labelsOf(items))
	}

	without := strings.Replace(stateActivitySrc, "\tprivate import StateActivity::*;\n", "", 1)
	items = completionAt(t, without, "attribute probe : Boolean = fill.")
	if _, ok := items["isActive"]; ok {
		t.Errorf("completion after 'fill.' offered 'isActive' without the StateActivity import")
	}
}

// Hover over the isActive of `fill.isActive` shows the extension feature and
// its Boolean type.
func TestHoverOnIsActiveShowsTheBooleanFeature(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/state_activity.sysml").Filename()
	ws.Open(name, []byte(stateActivitySrc), 1)

	off := strings.Index(stateActivitySrc, "fill.isActive") + len("fill.")
	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(stateActivitySrc), off),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res == nil {
		t.Fatal("Hover result = nil, want the isActive feature")
	}
	if !strings.Contains(res.Contents.Value, "isActive") || !strings.Contains(res.Contents.Value, "Boolean") {
		t.Errorf("hover value = %q, want the feature isActive typed Boolean", res.Contents.Value)
	}
}
