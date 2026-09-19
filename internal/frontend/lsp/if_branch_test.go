package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// ifBranchSource declares `brake` twice — once in each branch of the same
// conditional — plus a same-named feature outside it, so editor features must
// distinguish the three.
const ifBranchSource = `package P {
	private import ScalarValues::*;
	action def Step { out done : Boolean; }
	action def Drive {
		in attribute fast : Boolean;
		if fast {
			action brake : Step { out done; }
			assign fast := brake.done;
		} else {
			action brake : Step { out done; }
			assign fast := brake.done;
		}
	}
	action brake : Step { out done; }
}
`

// Hover reports the innermost declaration containing the cursor, so a branch
// body's declaration must be reachable through the scope the branch owns rather
// than being swallowed by the enclosing action definition.
func TestHoverBranchLocalDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/if_branch_hover.sysml").Filename()
	ws.Open(name, []byte(ifBranchSource), 1)

	off := strings.Index(ifBranchSource, "brake : Step")
	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(ifBranchSource), off),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res == nil {
		t.Fatal("Hover result = nil, want the branch-local action")
	}
	if !strings.Contains(res.Contents.Value, "brake") {
		t.Errorf("hover value = %q, want the branch-local `brake`", res.Contents.Value)
	}
	if strings.Contains(res.Contents.Value, "Drive") {
		t.Errorf("hover value = %q, want the branch member rather than its enclosing action", res.Contents.Value)
	}
}

// Go-to-definition from a use inside a branch must land on that branch's
// declaration, not on the like-named declaration of the sibling branch or of
// the enclosing package.
func TestDefinitionBranchLocalDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/if_branch_def.sysml").Filename()
	ws.Open(name, []byte(ifBranchSource), 1)

	definitionAt := func(off int) []protocol.Location {
		t.Helper()
		locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     offsetToPosition([]byte(ifBranchSource), off),
			},
		})
		if err != nil {
			t.Fatalf("Definition err = %v", err)
		}
		return locs
	}

	// A definition location is the declaration's whole span, which starts at
	// the `action` keyword rather than at the name.
	thenDecl := strings.Index(ifBranchSource, "action brake : Step")
	thenUse := strings.Index(ifBranchSource, "brake.done")
	elseUse := strings.Index(ifBranchSource[thenUse+1:], "brake.done") + thenUse + 1
	elseDecl := strings.Index(ifBranchSource[thenDecl+1:], "action brake : Step") + thenDecl + 1

	for _, tc := range []struct{ use, decl int }{{thenUse, thenDecl}, {elseUse, elseDecl}} {
		locs := definitionAt(tc.use)
		if len(locs) != 1 {
			t.Fatalf("locations from use at %d = %d, want 1", tc.use, len(locs))
		}
		if got := positionToOffset([]byte(ifBranchSource), locs[0].Range.Start); got != tc.decl {
			t.Errorf("definition offset = %d, want %d (the branch's own declaration)", got, tc.decl)
		}
	}
}

// Renaming from a branch-local declaration must rewrite that branch's uses and
// leave the sibling branch and the enclosing package's like-named action alone.
func TestRenameBranchLocalDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/if_branch_rename.sysml", ifBranchSource)

	got, err := applyRename(t, ws, name, "brake : Step", "slow")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "action slow : Step { out done; }\n\t\t\tassign fast := slow.done;") {
		t.Errorf("branch declaration and its use not both renamed:\n%s", got[name])
	}
	if strings.Count(got[name], "brake") != 3 {
		t.Errorf("sibling branch or outer action was rewritten:\n%s", got[name])
	}
}

// Renaming from a use inside a branch must edit that branch's declaration.
func TestRenameBranchLocalFromUse(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/if_branch_rename_use.sysml", ifBranchSource)

	got, err := applyRename(t, ws, name, "brake.done", "slow")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "action slow : Step { out done; }\n\t\t\tassign fast := slow.done;") {
		t.Errorf("branch declaration and its use not both renamed:\n%s", got[name])
	}
	if strings.Count(got[name], "brake") != 3 {
		t.Errorf("sibling branch or outer action was rewritten:\n%s", got[name])
	}
}
