package lsp

import (
	"context"
	"sort"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// applyRename runs Rename at the first occurrence of cursor in the named
// document and returns each document's edited content.
func applyRename(t *testing.T, ws *model.Workspace, name, cursorAt, newName string) (map[string]string, error) {
	t.Helper()
	s := NewServer(ws)
	doc := ws.Document(name)
	off := strings.Index(string(doc.Content), cursorAt)
	if off < 0 {
		t.Fatalf("cursor anchor %q not found", cursorAt)
	}
	edit, err := s.Rename(context.Background(), &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition(doc.Content, off),
		},
		NewName: newName,
	})
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for u, edits := range edit.Changes {
		docName := uriToName(u)
		out[docName] = applyEdits(t, string(ws.Document(docName).Content), edits)
	}
	return out, nil
}

// renameEdits runs Rename at the first occurrence of cursor and returns the
// edits for that document.
func renameEdits(t *testing.T, ws *model.Workspace, name, cursorAt, newName string) []protocol.TextEdit {
	t.Helper()
	s := NewServer(ws)
	doc := ws.Document(name)
	off := strings.Index(string(doc.Content), cursorAt)
	if off < 0 {
		t.Fatalf("cursor anchor %q not found", cursorAt)
	}
	edit, err := s.Rename(context.Background(), &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition(doc.Content, off),
		},
		NewName: newName,
	})
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	return edit.Changes[nameToURI(name)]
}

// applyEdits applies non-overlapping edits back-to-front, as a client would.
func applyEdits(t *testing.T, content string, edits []protocol.TextEdit) string {
	t.Helper()
	spans := make([]protocol.TextEdit, len(edits))
	copy(spans, edits)
	sort.Slice(spans, func(i, j int) bool {
		a, b := spans[i].Range.Start, spans[j].Range.Start
		if a.Line != b.Line {
			return a.Line > b.Line
		}
		return a.Character > b.Character
	})
	src := []byte(content)
	for _, e := range spans {
		start := positionToOffset(src, e.Range.Start)
		end := positionToOffset(src, e.Range.End)
		src = append(src[:start], append([]byte(e.NewText), src[end:]...)...)
	}
	return string(src)
}

func openRenameDoc(t *testing.T, ws *model.Workspace, path, src string) string {
	t.Helper()
	name := uri.File(path).Filename()
	ws.Open(name, []byte(src), 1)
	return name
}

func TestRenameDeclarationAndReferences(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/rename_decl.sysml", "package P { namespace N; }\nimport P::N;\nimport P::N;\n")

	got, err := applyRename(t, ws, name, "N;", "M")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	want := "package P { namespace M; }\nimport P::M;\nimport P::M;\n"
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}
}

func TestRenameFromReferenceRenamesDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/rename_ref.sysml", "package P { namespace N; }\nimport P::N;\n")

	got, err := applyRename(t, ws, name, "P::N", "M")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	// The cursor sits on the `P` segment, so P is renamed, not N.
	want := "package M { namespace N; }\nimport M::N;\n"
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}
}

func TestRenameEditsQualifierSegments(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/rename_qual.sysml", "package P { namespace N; }\nimport P::N;\n")

	got, err := applyRename(t, ws, name, "P {", "Q")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if want := "package Q { namespace N; }\nimport Q::N;\n"; got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}
}

// References inside definition and usage bodies — typings, specializations and
// value expressions — must be renamed too, or the model stops resolving.
func TestRenameEditsReferencesInsideDeclarations(t *testing.T) {
	ws := model.NewWorkspace()
	src := `package P {
	part def Car;
	part def Coupe :> Car;
	part c : Car;
	part d : Car[1];
	attribute n : ScalarValues::Integer = 1;
	attribute twice : ScalarValues::Integer = n + n;
}
`
	name := openRenameDoc(t, ws, "/tmp/rename_decls.sysml", src)

	got, err := applyRename(t, ws, name, "Car;", "Vehicle")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if strings.Contains(got[name], "Car") {
		t.Errorf("Car still referenced after rename:\n%s", got[name])
	}
	for _, want := range []string{"part def Vehicle;", ":> Vehicle;", "part c : Vehicle;", "part d : Vehicle[1];"} {
		if !strings.Contains(got[name], want) {
			t.Errorf("missing %q in:\n%s", want, got[name])
		}
	}

	// A feature referenced from an expression.
	got, err = applyRename(t, ws, name, "n :", "count")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "= count + count;") {
		t.Errorf("expression reference not renamed:\n%s", got[name])
	}
}

func TestRenameRejectsKeywordAndInvalidNames(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/rename_bad.sysml", "package P { namespace N; }\n")

	for _, newName := range []string{"", "part", "2N", "has space", "N::M"} {
		if _, err := applyRename(t, ws, name, "N;", newName); err == nil {
			t.Errorf("Rename to %q succeeded, want error", newName)
		}
	}
}

func TestRenameRejectsLibrarySymbols(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/rename_lib.sysml", "package P { attribute x : ScalarValues::Integer; }\n")

	if _, err := applyRename(t, ws, name, "ScalarValues", "Scalars"); err == nil {
		t.Fatal("renaming a standard-library package succeeded, want error")
	}
}

func TestPrepareRenameReportsIdentifierRange(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	src := "package P { namespace N; }\n"
	name := openRenameDoc(t, ws, "/tmp/prepare.sysml", src)

	at := func(anchor string) protocol.Position {
		return offsetToPosition([]byte(src), strings.Index(src, anchor))
	}

	rng, err := s.PrepareRename(context.Background(), &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     at("N;"),
		},
	})
	if err != nil {
		t.Fatalf("PrepareRename err = %v", err)
	}
	if got := positionToOffset([]byte(src), rng.Start); got != strings.Index(src, "N;") {
		t.Errorf("range start offset = %d, want %d", got, strings.Index(src, "N;"))
	}

	// A position that names nothing is refused rather than silently accepted.
	if _, err := s.PrepareRename(context.Background(), &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     at("{"),
		},
	}); err == nil {
		t.Error("PrepareRename on '{' succeeded, want error")
	}
}

func TestRenameAcrossDocuments(t *testing.T) {
	ws := model.NewWorkspace()
	declName := openRenameDoc(t, ws, "/tmp/rename_decl_a.sysml", "package P { namespace N; }\n")
	refName := openRenameDoc(t, ws, "/tmp/rename_ref_b.sysml", "import P::N;\n")

	got, err := applyRename(t, ws, declName, "N;", "M")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if want := "package P { namespace M; }\n"; got[declName] != want {
		t.Errorf("declaring document:\ngot:  %q\nwant: %q", got[declName], want)
	}
	if want := "import P::M;\n"; got[refName] != want {
		t.Errorf("referencing document:\ngot:  %q\nwant: %q", got[refName], want)
	}
}

// A shorthand redefinition (`part redefines x;`) carries a parser-synthesised
// relationship target sharing the declaration's identifier span, so the
// declaration and reference edits land on the same range.
func TestRenameShorthandRedefinitionEditsOnce(t *testing.T) {
	ws := model.NewWorkspace()
	src := `package P {
	part def Base { part x [1]; }
	part def Sub :> Base { part redefines x; }
}
`
	name := openRenameDoc(t, ws, "/tmp/rename_shorthand.sysml", src)

	edits := renameEdits(t, ws, name, "x; }", "renamed") // the redefining member
	assertNoOverlap(t, edits)

	got, err := applyRename(t, ws, name, "x; }", "renamed")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "part redefines renamed;") {
		t.Errorf("shorthand redefinition not renamed cleanly:\n%s", got[name])
	}
}

// bodyParamSrc declares a body-expression parameter that shadows a same-named
// outer feature, so an edit that confuses the two is visible.
const bodyParamSrc = `package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	attribute s : Integer = 1;
	action def Sample {
		in attribute samples : Real[*];
		assert constraint { samples->forAll { in s : Real; s > 0 } }
	}
}
`

// A body expression's parameters are declarations like any other, so renaming
// with the cursor on the parameter itself must rewrite it and its uses.
func TestRenameBodyExpressionParameterFromDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/rename_bodyparam_decl.sysml", bodyParamSrc)

	edits := renameEdits(t, ws, name, "s : Real; s > 0", "sample")
	assertNoOverlap(t, edits)

	got, err := applyRename(t, ws, name, "s : Real; s > 0", "sample")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "in sample : Real; sample > 0") {
		t.Errorf("parameter declaration and use not both renamed:\n%s", got[name])
	}
	if !strings.Contains(got[name], "attribute s : Integer = 1;") {
		t.Errorf("outer feature was rewritten:\n%s", got[name])
	}
}

// Renaming from a use of the parameter must produce the same edits as renaming
// from its declaration.
func TestRenameBodyExpressionParameterFromUseMatchesDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/rename_bodyparam_use.sysml", bodyParamSrc)

	fromDecl, err := applyRename(t, ws, name, "s : Real; s > 0", "sample")
	if err != nil {
		t.Fatalf("Rename from declaration err = %v", err)
	}
	fromUse, err := applyRename(t, ws, name, "s > 0", "sample")
	if err != nil {
		t.Fatalf("Rename from use err = %v", err)
	}
	if fromDecl[name] != fromUse[name] {
		t.Errorf("rename from declaration and from use differ:\n%s\n---\n%s", fromDecl[name], fromUse[name])
	}
}

// PrepareRename must accept the parameter's own identifier, so the client
// offers the parameter name for editing rather than refusing the position.
func TestPrepareRenameAcceptsBodyExpressionParameter(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := openRenameDoc(t, ws, "/tmp/prepare_bodyparam.sysml", bodyParamSrc)

	want := strings.Index(bodyParamSrc, "s : Real; s > 0")
	rng, err := s.PrepareRename(context.Background(), &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(bodyParamSrc), want),
		},
	})
	if err != nil {
		t.Fatalf("PrepareRename err = %v", err)
	}
	if got := positionToOffset([]byte(bodyParamSrc), rng.Start); got != want {
		t.Errorf("range start offset = %d, want %d", got, want)
	}
}

// assertNoOverlap fails when two edits cover the same characters: clients reject
// a workspace edit with overlapping ranges outright.
func assertNoOverlap(t *testing.T, edits []protocol.TextEdit) {
	t.Helper()
	for i := range edits {
		for j := i + 1; j < len(edits); j++ {
			a, b := edits[i].Range, edits[j].Range
			if a.Start.Line == b.Start.Line && a.Start.Character == b.Start.Character {
				t.Fatalf("overlapping edits at %v: %d edits total %v", a.Start, len(edits), edits)
			}
		}
	}
}

// A binding's named end is a feature of the binding, so renaming it rewrites
// the chain that reaches it through the binding and leaves the referenced
// feature alone; renaming that feature rewrites the end's `::>` target.
func TestRenameBindingConnectorEnd(t *testing.T) {
	src := "package P {\n\tpart def V {\n\t\tattribute a;\n\t\tattribute b;\n\t\tbinding bb bind e1 ::> a = e2 references b;\n\t\tattribute viaEnd :> bb.e1;\n\t}\n}\n"
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/rename_binding_end.sysml", src)
	got, err := applyRename(t, ws, name, "e1 ::>", "left")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	want := strings.NewReplacer("bind e1 ::>", "bind left ::>", "bb.e1", "bb.left").Replace(src)
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}

	ws = model.NewWorkspace()
	name = openRenameDoc(t, ws, "/tmp/rename_binding_target.sysml", src)
	got, err = applyRename(t, ws, name, "a;", "alpha")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	want = strings.NewReplacer("attribute a;", "attribute alpha;", "e1 ::> a =", "e1 ::> alpha =").Replace(src)
	if got[name] != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got[name], want)
	}
}
