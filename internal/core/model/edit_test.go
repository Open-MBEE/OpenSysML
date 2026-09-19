package model

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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
		_, _, ok, err := ws.ApplyEdit("car.sysml", []edit.Operation{op}, nil)
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
		_, _, ok, err := ws.ApplyEdit("car.sysml", []edit.Operation{op}, nil)
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
		result, _, ok, err := ws.ApplyEdit("car.sysml", []edit.Operation{spare, engine}, nil)
		if !ok || err != nil {
			t.Fatalf("ok %v, err %v", ok, err)
		}
		want := "part def Car {\n    part tank : Lib::Tank;\n    part spare : Lib::Tank;\n    part engine : Engine;\n}\n"
		if string(result.Documents[0].Content) != want {
			t.Fatalf("content:\n%s\nwant:\n%s", result.Documents[0].Content, want)
		}
	})
}

// A rename reaches every document of the workspace referring to the target,
// open or read from disk, each reported at the version the workspace holds; a
// document the caller indexed but the workspace does not hold cannot be
// rewritten, so a reference from it refuses the edit.
func TestApplyEditFollowsReferencesIntoWorkspaceDocuments(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("engine.sysml", []byte("package Engines {\n    part def Engine;\n}\n"), 1)
	ws.Open("car.sysml", []byte("package Cars {\n    part def Car {\n        part e : Engines::Engine;\n    }\n}\n"), 4)
	ws.SetOnDisk("boat.sysml", []byte("package Boats {\n    part motor : Engines::Engine;\n}\n"))

	result, version, ok, err := ws.ApplyEdit("engine.sysml", []edit.Operation{edit.Rename("Engines::Engine", "Motor")}, nil)
	if !ok || err != nil {
		t.Fatalf("ok %v, err %v", ok, err)
	}
	if version != 1 {
		t.Errorf("version = %d, want 1", version)
	}
	want := []DocumentEdit{
		{Name: "engine.sysml", Version: 1, Open: true, Content: []byte("package Engines {\n    part def Motor;\n}\n")},
		{Name: "boat.sysml", Version: 0, Open: false, Content: []byte("package Boats {\n    part motor : Engines::Motor;\n}\n")},
		{Name: "car.sysml", Version: 4, Open: true, Content: []byte("package Cars {\n    part def Car {\n        part e : Engines::Motor;\n    }\n}\n")},
	}
	if len(result.Documents) != len(want) {
		t.Fatalf("documents = %d, want %d", len(result.Documents), len(want))
	}
	for i, doc := range result.Documents {
		if doc.Name != want[i].Name || doc.Version != want[i].Version || doc.Open != want[i].Open {
			t.Errorf("document %d = %s v%d open %v, want %s v%d open %v", i,
				doc.Name, doc.Version, doc.Open, want[i].Name, want[i].Version, want[i].Open)
		}
		if string(doc.Content) != string(want[i].Content) {
			t.Errorf("%s:\n%s\nwant:\n%s", doc.Name, doc.Content, want[i].Content)
		}
		if string(doc.Original) != string(ws.Document(doc.Name).Content) {
			t.Errorf("%s original:\n%s\nwant the workspace's content", doc.Name, doc.Original)
		}
		if len(doc.Applied) == 0 {
			t.Errorf("%s reports no applied range", doc.Name)
		}
	}
	for _, name := range []string{"engine.sysml", "car.sysml", "boat.sysml"} {
		if strings.Contains(string(ws.Document(name).Content), "Motor") {
			t.Errorf("%s was rewritten in place", name)
		}
	}
}

func TestApplyEditRefusesReferenceFromDocumentItCannotRewrite(t *testing.T) {
	idx := symbols.NewIndex()
	fleet := []byte("package Fleet {\n    part truck : Engines::Engine;\n}\n")
	idx.AddDocumentWithKind("fleet.sysml", parser.New(source.New("fleet.sysml", fleet)).ParseFile(), source.KindSysML)
	idx.ExpandWildcardImports()
	ws := NewWorkspaceWithIndex(idx)
	ws.Open("engine.sysml", []byte("package Engines {\n    part def Engine;\n}\n"), 1)

	for _, op := range []edit.Operation{
		edit.Rename("Engines::Engine", "Motor"),
		edit.Delete("Engines::Engine", true),
	} {
		result, _, ok, err := ws.ApplyEdit("engine.sysml", []edit.Operation{op}, nil)
		if !ok {
			t.Fatal("document not found")
		}
		var e *edit.Error
		if !errors.As(err, &e) || e.Failure != edit.FailureReferencedElsewhere {
			t.Fatalf("%v: got %v, want %s", op.Kind, err, edit.FailureReferencedElsewhere)
		}
		if result != nil {
			t.Errorf("%v: result %+v alongside a refusal", op.Kind, result)
		}
		if want := []string{"Fleet::truck (fleet.sysml)"}; strings.Join(e.Referring, ",") != strings.Join(want, ",") {
			t.Errorf("%v: referring = %v, want %v", op.Kind, e.Referring, want)
		}
	}
}

// A document read for an operation's target and not rewritten is among the
// result's documents all the same, once, unchanged and at the version it was
// read at, in name order with the rewritten ones; the edited document is
// reported once whether or not it was read.
func TestApplyEditReportsReadDocumentsItLeftUnchanged(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("views.sysml", []byte("package Fleet {\n    private import Views::*;\n    private import StandardViewDefinitions::*;\n    view v : InterconnectionView {\n        expose Machinery::Engine;\n    }\n}\n"), 1)
	ws.Open("parts.sysml", []byte("package Machinery {\n    part def Engine {\n        part rotor;\n    }\n}\n"), 4)
	ws.Open("aux.sysml", []byte("package Aux {\n    part def Spare;\n}\n"), 6)
	place := edit.SetLayout("Machinery::Engine::rotor", "Fleet::v", &semantics.Layout{X: 1, Y: 2}).DeclaredIn("parts.sysml")

	parts, aux, views := ws.Document("parts.sysml"), ws.Document("aux.sysml"), ws.Document("views.sysml")
	result, version, ok, err := ws.ApplyEdit("views.sysml", []edit.Operation{place}, []*Document{parts, views, parts, aux})
	if !ok || err != nil {
		t.Fatalf("ok %v, err %v", ok, err)
	}
	if version != 1 {
		t.Errorf("version = %d, want views.sysml's 1", version)
	}
	want := []DocumentEdit{
		{Name: "views.sysml", Version: 1, Open: true},
		{Name: "aux.sysml", Version: 6, Open: true},
		{Name: "parts.sysml", Version: 4, Open: true},
	}
	if len(result.Documents) != len(want) {
		t.Fatalf("documents = %+v, want %d: views.sysml, then aux.sysml and parts.sysml read", result.Documents, len(want))
	}
	for i, doc := range result.Documents {
		if doc.Name != want[i].Name || doc.Version != want[i].Version || doc.Open != want[i].Open {
			t.Errorf("document %d = %s v%d open %v, want %s v%d open %v", i,
				doc.Name, doc.Version, doc.Open, want[i].Name, want[i].Version, want[i].Open)
		}
		if string(doc.Original) != string(ws.Document(doc.Name).Content) {
			t.Errorf("%s original:\n%s\nwant the workspace's content", doc.Name, doc.Original)
		}
		if i > 0 && (string(doc.Content) != string(doc.Original) || len(doc.Applied) != 0) {
			t.Errorf("%s = %+v, want it read and left as it was", doc.Name, doc)
		}
	}
	if got := string(result.Documents[0].Content); !strings.Contains(got, "metadata DiagramLayout::Layout about Machinery::Engine::rotor { x = 1; y = 2; }") {
		t.Errorf("views.sysml:\n%s", got)
	}
}

// An edit computed from snapshots of other documents is pinned to them: when
// the workspace has since replaced one — by other text or by the same text at a
// new version — the edit is refused as stale, and nothing is rewritten.
func TestApplyEditRefusesWhenAReadDocumentWasReplaced(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("engine.sysml", []byte("package Engines {\n    part def Engine;\n}\n"), 1)
	ws.Open("car.sysml", []byte("package Cars {\n    part def Car {\n        part e : Engines::Engine;\n    }\n}\n"), 4)
	rename := []edit.Operation{edit.Rename("Engines::Engine", "Motor")}

	read := ws.Document("car.sysml")
	if _, _, ok, err := ws.ApplyEdit("engine.sysml", rename, []*Document{read}); !ok || err != nil {
		t.Fatalf("with the snapshot the workspace holds: ok %v, err %v", ok, err)
	}
	ws.Update("car.sysml", read.Content, 5)
	_, version, ok, err := ws.ApplyEdit("engine.sysml", rename, []*Document{read})
	var stale *StaleError
	if !ok || !errors.As(err, &stale) || stale.Name != "car.sysml" {
		t.Fatalf("after car.sysml was replaced: ok %v, err %v, want a StaleError naming car.sysml", ok, err)
	}
	if version != 1 {
		t.Errorf("version = %d, want engine.sysml's 1", version)
	}
	if got := ws.Document("car.sysml"); got == read || got.Version != 5 {
		t.Errorf("car.sysml = v%d, want the workspace's v5 untouched", got.Version)
	}
}
