package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// An alias declaration, a use of the alias name and a use qualified through it.
const w6bRenameSrc = "package Shapes {\n\tpart def Cube {\n\t\tattribute length;\n\t}\n" +
	"\talias Box for Cube;\n\tpart p : Box;\n}\npackage Uses {\n\tpart b : Shapes::Box;\n}\n"

// renameSpans returns the renamed text of the document, applying the edits the
// server produced back-to-front so offsets stay valid.
func w6bRename(t *testing.T, at string, newName string) string {
	t.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/w6b_alias_rename.sysml").Filename()
	ws.Open(name, []byte(w6bRenameSrc), 1)

	edit, err := s.Rename(context.Background(), &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(w6bRenameSrc), strings.Index(w6bRenameSrc, at)),
		},
		NewName: newName,
	})
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	edits := edit.Changes[nameToURI(name)]
	offsets := make([]int, len(edits))
	for i, e := range edits {
		offsets[i] = positionToOffset([]byte(w6bRenameSrc), e.Range.Start)
	}
	out := w6bRenameSrc
	for {
		last := -1
		for i, off := range offsets {
			if off >= 0 && (last < 0 || off > offsets[last]) {
				last = i
			}
		}
		if last < 0 {
			return out
		}
		e := edits[last]
		start := offsets[last]
		end := positionToOffset([]byte(w6bRenameSrc), e.Range.End)
		out = out[:start] + e.NewText + out[end:]
		offsets[last] = -1
	}
}

// Renaming the alias rewrites the uses written with the alias name: they are
// occurrences of that name, whichever element they reach.
func TestW6BRenameAliasRewritesItsUses(t *testing.T) {
	got := w6bRename(t, "Box for Cube", "Crate")
	for _, want := range []string{"alias Crate for Cube;", "part p : Crate;", "Shapes::Crate;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renamed source missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Box") {
		t.Fatalf("renamed source still writes Box:\n%s", got)
	}
}

// Renaming the aliased element leaves the alias and its uses alone: they write
// the alias's name, which the rename does not change.
func TestW6BRenameTargetLeavesAliasUsesAlone(t *testing.T) {
	got := w6bRename(t, "Cube {", "Cuboid")
	for _, want := range []string{"part def Cuboid {", "alias Box for Cuboid;", "part p : Box;", "Shapes::Box;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renamed source missing %q:\n%s", want, got)
		}
	}
}

// References of the aliased element include the uses written through the alias:
// they are places a reader reaches that element from.
func TestW6BReferencesOfTargetIncludeAliasUses(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/w6b_alias_refs.sysml").Filename()
	ws.Open(name, []byte(w6bRenameSrc), 1)

	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(w6bRenameSrc), strings.Index(w6bRenameSrc, "Cube {")),
		},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	// The alias target `Cube`, and the two uses written as `Box`.
	if len(locs) < 3 {
		t.Fatalf("references = %d, want at least 3 (the alias target and both alias uses)", len(locs))
	}
}

// References of the alias itself are the occurrences of its name.
func TestW6BReferencesOfAliasAreItsNameOccurrences(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/w6b_alias_refs2.sysml").Filename()
	ws.Open(name, []byte(w6bRenameSrc), 1)

	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(w6bRenameSrc), strings.Index(w6bRenameSrc, "Box for Cube")),
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	if len(locs) != 3 {
		t.Fatalf("references = %d, want 3 (the declaration and both uses)", len(locs))
	}
}
