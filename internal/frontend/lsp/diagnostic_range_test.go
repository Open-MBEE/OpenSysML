package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// spacedModel separates each reference from the token after it.
const spacedModel = "package P {\n    part def A ;\n    part x : Strat ;\n    part def D :> Missing  {\n    }\n    part q : A  :>  A ;\n}\n"

// An unresolved reference is highlighted over its name alone, not over the
// whitespace before the `;` or `{` that follows it.
func TestPublishedDiagnosticRangesEndAtTheName(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc
	name := "spaced.sysml"
	ws.Open(name, []byte(spacedModel), 1)
	s.publishDiagnostics(context.Background(), name)

	want := map[string]protocol.Range{
		"unresolved reference: Strat":   {Start: protocol.Position{Line: 2, Character: 13}, End: protocol.Position{Line: 2, Character: 18}},
		"unresolved reference: Missing": {Start: protocol.Position{Line: 3, Character: 18}, End: protocol.Position{Line: 3, Character: 25}},
	}
	published := fc.all()
	if len(published) != 1 {
		t.Fatalf("published = %d, want 1", len(published))
	}
	for _, d := range published[0].Diagnostics {
		for msg, r := range want {
			if strings.HasPrefix(d.Message, msg) {
				if d.Range != r {
					t.Errorf("%q range = %v, want %v", msg, d.Range, r)
				}
				delete(want, msg)
			}
		}
	}
	for msg := range want {
		t.Errorf("no diagnostic %q among %v", msg, published[0].Diagnostics)
	}
}

// Rename rewrites each name in place and References ranges over each use's
// name and the declaration's own text, leaving the spacing around them untouched.
func TestRenameAndReferencesKeepSurroundingSpace(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/rename_spaced.sysml", spacedModel)

	got, err := applyRename(t, ws, name, "A ;", "Wheel")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	want := strings.ReplaceAll(spacedModel, " A ", " Wheel ")
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}

	s := NewServer(ws)
	src := []byte(spacedModel)
	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition(src, strings.Index(spacedModel, "A ;")),
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	if len(locs) != 3 {
		t.Fatalf("references = %v, want the declaration and two uses", locs)
	}
	wantText := []string{"part def A ;", "A", "A"}
	for i, l := range locs {
		start, end := positionToOffset(src, l.Range.Start), positionToOffset(src, l.Range.End)
		if text := spacedModel[start:end]; text != wantText[i] {
			t.Errorf("reference %d range %v covers %q, want %q", i, l.Range, text, wantText[i])
		}
	}
}
