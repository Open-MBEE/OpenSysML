package lsp

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// actionsFor opens src and asks for the code actions covering all of it.
func actionsFor(t *testing.T, file, src string, rng protocol.Range) []protocol.CodeAction {
	t.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File(file).Filename()
	ws.Open(name, []byte(src), 1)
	acts, err := s.CodeAction(context.Background(), &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
		Range:        rng,
	})
	if err != nil {
		t.Fatalf("CodeAction err = %v", err)
	}
	return acts
}

// wholeFile is a range covering any of the small sources these tests use.
var wholeFile = protocol.Range{End: protocol.Position{Line: 100}}

// edit returns the single text edit of an action, or fails.
func edit(t *testing.T, act protocol.CodeAction, file string) protocol.TextEdit {
	t.Helper()
	if act.Kind != protocol.QuickFix {
		t.Errorf("action %q kind = %q, want %q", act.Title, act.Kind, protocol.QuickFix)
	}
	if len(act.Diagnostics) != 1 {
		t.Errorf("action %q carries %d diagnostics, want the one it fixes", act.Title, len(act.Diagnostics))
	}
	if act.Edit == nil {
		t.Fatalf("action %q has no workspace edit", act.Title)
	}
	edits := act.Edit.Changes[uri.File(file)]
	if len(edits) != 1 {
		t.Fatalf("action %q edits = %v, want one for %s", act.Title, act.Edit.Changes, file)
	}
	return edits[0]
}

// apply returns src with an edit applied, so a test asserts on the text a client
// would end up with.
func apply(t *testing.T, src string, e protocol.TextEdit) string {
	t.Helper()
	content := []byte(src)
	start := positionToOffset(content, e.Range.Start)
	end := positionToOffset(content, e.Range.End)
	return string(content[:start]) + e.NewText + string(content[end:])
}

func TestCodeActionFixesNearMissName(t *testing.T) {
	const file = "/tmp/act_nearmiss.sysml"
	const src = "package P {\n    part def Wheel;\n    part w : Wheeel;\n}\n"
	acts := actionsFor(t, file, src, wholeFile)
	if len(acts) != 1 {
		t.Fatalf("actions = %+v, want one rename fix", acts)
	}
	if acts[0].Title != "Change 'Wheeel' to 'Wheel'" {
		t.Errorf("title = %q", acts[0].Title)
	}
	if !acts[0].IsPreferred {
		t.Error("the only candidate in scope should be the preferred fix")
	}
	if got := apply(t, src, edit(t, acts[0], file)); got != "package P {\n    part def Wheel;\n    part w : Wheel;\n}\n" {
		t.Errorf("applied =\n%s", got)
	}
}

func TestCodeActionImportsResolvableFQN(t *testing.T) {
	const file = "/tmp/act_import.sysml"
	const src = "package P {\n    attribute x : Integer;\n}\n"
	acts := actionsFor(t, file, src, wholeFile)
	titles := map[string]protocol.CodeAction{}
	for _, act := range acts {
		titles[act.Title] = act
	}
	qualify, ok := titles["Change 'Integer' to 'ScalarValues::Integer'"]
	if !ok {
		t.Fatalf("no qualifying fix in %v", titles)
	}
	if got := apply(t, src, edit(t, qualify, file)); got != "package P {\n    attribute x : ScalarValues::Integer;\n}\n" {
		t.Errorf("qualified =\n%s", got)
	}
	imp, ok := titles["Import 'ScalarValues::*'"]
	if !ok {
		t.Fatalf("no import fix in %v", titles)
	}
	// The import goes on its own line above the member that needed it, keeping
	// that member's indentation, and is explicitly private.
	if got := apply(t, src, edit(t, imp, file)); got != "package P {\n    private import ScalarValues::*;\n    attribute x : Integer;\n}\n" {
		t.Errorf("imported =\n%s", got)
	}
}

func TestCodeActionInsertsMissingSemicolon(t *testing.T) {
	const file = "/tmp/act_semi.sysml"
	const src = "package P {\n    action def A {\n        first start\n    }\n}\n"
	acts := actionsFor(t, file, src, wholeFile)
	if len(acts) == 0 {
		t.Fatal("no fix for a precisely reported missing semicolon")
	}
	if acts[0].Title != "Insert ';'" {
		t.Fatalf("titles = %+v, want an insert-semicolon fix", acts)
	}
	if got := apply(t, src, edit(t, acts[0], file)); got != "package P {\n    action def A {\n        first start;\n    }\n}\n" {
		t.Errorf("applied =\n%s", got)
	}
}

// A syntax error the parser cannot narrow to one repair — a declaration that may
// want either a body or a semicolon — carries no fix rather than a guess.
func TestCodeActionSkipsAmbiguousSyntaxError(t *testing.T) {
	const file = "/tmp/act_ambiguous.sysml"
	const src = "package P {\n    part def Wheel { attribute pressure }\n}\n"
	if acts := actionsFor(t, file, src, wholeFile); len(acts) != 0 {
		t.Errorf("actions = %+v, want none", acts)
	}
}

// A request is answered for the diagnostics its range touches only.
func TestCodeActionFiltersByRange(t *testing.T) {
	const file = "/tmp/act_range.sysml"
	const src = "package P {\n    part def Wheel;\n    part a : Wheeel;\n    part b : Whel;\n}\n"
	all := actionsFor(t, file, src, wholeFile)
	if len(all) != 2 {
		t.Fatalf("actions for the file = %+v, want one per unresolved name", all)
	}
	line3 := actionsFor(t, file, src, protocol.Range{
		Start: protocol.Position{Line: 3},
		End:   protocol.Position{Line: 3, Character: 18},
	})
	// The whole line selects b's header too, so its identity rewrite comes along.
	if len(line3) != 2 || line3[0].Title != "Change 'Whel' to 'Wheel'" || line3[1].Kind != identityActionKind {
		t.Errorf("actions for line 3 = %+v", line3)
	}
	// An empty range is a cursor: it still asks for the fixes of the diagnostic
	// it sits inside, alongside the identity rewrite of the declaration it is on.
	cursor := actionsFor(t, file, src, protocol.Range{
		Start: protocol.Position{Line: 2, Character: 15},
		End:   protocol.Position{Line: 2, Character: 15},
	})
	if len(cursor) != 2 || cursor[0].Title != "Change 'Wheeel' to 'Wheel'" || cursor[1].Kind != identityActionKind {
		t.Errorf("actions at the cursor = %+v", cursor)
	}
}

// A client asking only for kinds the server does not support gets nothing.
func TestCodeActionHonoursOnlyFilter(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/act_only.sysml").Filename()
	ws.Open(name, []byte("package P {\n    part def Wheel;\n    part w : Wheeel;\n}\n"), 1)
	for _, tc := range []struct {
		only []protocol.CodeActionKind
		want int
	}{
		{nil, 1},
		{[]protocol.CodeActionKind{protocol.QuickFix}, 1},
		{[]protocol.CodeActionKind{protocol.Refactor, protocol.SourceOrganizeImports}, 0},
	} {
		acts, err := s.CodeAction(context.Background(), &protocol.CodeActionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Range:        wholeFile,
			Context:      protocol.CodeActionContext{Only: tc.only},
		})
		if err != nil {
			t.Fatalf("CodeAction(only=%v) err = %v", tc.only, err)
		}
		if len(acts) != tc.want {
			t.Errorf("CodeAction(only=%v) = %d actions, want %d", tc.only, len(acts), tc.want)
		}
	}
}

// An unknown document has no actions rather than an error.
func TestCodeActionUnknownDocument(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	acts, err := s.CodeAction(context.Background(), &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File("/tmp/missing.sysml")},
		Range:        wholeFile,
	})
	if err != nil {
		t.Fatalf("CodeAction err = %v", err)
	}
	if len(acts) != 0 {
		t.Errorf("actions = %+v, want none", acts)
	}
}
