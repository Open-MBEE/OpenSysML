package model

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/edit"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// customIndexWorkspace is a workspace over a caller-built index: one library
// document the caller indexed and marked, and two documents the workspace opened.
func customIndexWorkspace(t *testing.T) *Workspace {
	t.Helper()
	idx := symbols.NewIndex()
	lib := []byte("package Lib {\n    part def Tank;\n}\n")
	idx.AddDocumentWithKind("lib/tanks.sysml", parser.New(source.New("lib/tanks.sysml", lib)).ParseFile(), source.KindSysML)
	idx.MarkLibraryDocument("lib/tanks.sysml", symbols.LibraryDocument{Tier: symbols.TierLibrary, Digest: symbols.TextDigest(lib)})
	idx.ExpandWildcardImports()

	ws := NewWorkspaceWithIndex(idx)
	ws.Open("engine.sysml", []byte("part def Engine;\n"), 1)
	ws.Open("car.sysml", []byte("part def Car {\n    part tank : Lib::Tank;\n}\n"), 1)
	return ws
}

func TestApplyEditCustomIndexValidatesSemantics(t *testing.T) {
	ws := customIndexWorkspace(t)

	t.Run("refuses an unresolved type", func(t *testing.T) {
		op := edit.AddMember("Car", "part", "wheel")
		op.Type = "Missing"
		_, _, ok, err := ws.ApplyEdit("car.sysml", []edit.Operation{op})
		if !ok {
			t.Fatal("document not found")
		}
		var e *edit.Error
		if !errors.As(err, &e) || e.Failure != edit.FailureResultInvalid {
			t.Fatalf("got %v, want %s", err, edit.FailureResultInvalid)
		}
		if !strings.Contains(e.Message, "Missing") {
			t.Fatalf("message %q does not name the type", e.Message)
		}
	})

	t.Run("refuses an unresolved connection endpoint", func(t *testing.T) {
		op := edit.AddConnection("Car", "connection", "tank", "nowhere", "")
		_, _, ok, err := ws.ApplyEdit("car.sysml", []edit.Operation{op})
		if !ok {
			t.Fatal("document not found")
		}
		var e *edit.Error
		if !errors.As(err, &e) || e.Failure != edit.FailureResultInvalid {
			t.Fatalf("got %v, want %s", err, edit.FailureResultInvalid)
		}
	})

	t.Run("resolves the caller's library and a sibling document", func(t *testing.T) {
		spare := edit.AddMember("Car", "part", "spare")
		spare.Type = "Lib::Tank"
		engine := edit.AddMember("Car", "part", "engine")
		engine.Type = "Engine"
		result, _, ok, err := ws.ApplyEdit("car.sysml", []edit.Operation{spare, engine})
		if !ok || err != nil {
			t.Fatalf("ok %v, err %v", ok, err)
		}
		want := "part def Car {\n    part tank : Lib::Tank;\n    part spare : Lib::Tank;\n    part engine : Engine;\n}\n"
		if string(result.Content) != want {
			t.Fatalf("content:\n%s\nwant:\n%s", result.Content, want)
		}
	})
}
