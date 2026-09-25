package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// A short-named definition referenced by both of its names, unqualified and as
// a qualifier segment.
const shortNameRenameSrc = `package P {
	part def <O> Old { part x; }
	part a : O;
	part b : Old;
	part c : P::O::x;
	part d : P::Old::x;
}
`

// renameShortNameDoc runs Rename at cursorAt over src in a fresh workspace and
// returns the edited document.
func renameShortNameDoc(t *testing.T, src, cursorAt, newName string) string {
	t.Helper()
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/short_name_rename.sysml", src)
	got, err := applyRename(t, ws, name, cursorAt, newName)
	if err != nil {
		t.Fatalf("Rename at %q err = %v", cursorAt, err)
	}
	return got[name]
}

// wantLines fails unless every want is a substring of got.
func wantLines(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}

// Renaming the long name rewrites the declaration and the references spelled
// with it; a reference spelled with the short name still resolves, so it stays.
func TestRenameLongNameLeavesShortNameReferences(t *testing.T) {
	got := renameShortNameDoc(t, shortNameRenameSrc, "Old {", "Fresh")
	wantLines(t, got,
		"part def <O> Fresh { part x; }",
		"part a : O;",
		"part b : Fresh;",
		"part c : P::O::x;",
		"part d : P::Fresh::x;",
	)
	if strings.Contains(got, "Old") {
		t.Errorf("long name survives the rename:\n%s", got)
	}
}

// The cursor on the short name between its angle brackets renames the short name:
// its declaration and every reference spelled with it, and nothing spelled Old.
func TestRenameShortNameFromDeclaration(t *testing.T) {
	got := renameShortNameDoc(t, shortNameRenameSrc, "O> Old", "Q")
	wantLines(t, got,
		"part def <Q> Old { part x; }",
		"part a : Q;",
		"part b : Old;",
		"part c : P::Q::x;",
		"part d : P::Old::x;",
	)
}

// A reference spelled with the short name renames the short name, whether the
// reference is a whole name or one qualifier segment.
func TestRenameShortNameFromReference(t *testing.T) {
	for _, cursor := range []string{"O;", "O::x"} {
		got := renameShortNameDoc(t, shortNameRenameSrc, cursor, "Q")
		wantLines(t, got,
			"part def <Q> Old { part x; }",
			"part a : Q;",
			"part b : Old;",
			"part c : P::Q::x;",
			"part d : P::Old::x;",
		)
	}
}

// A declaration with only a short name is renamed through it like any other name.
func TestRenameShortNameOnlyDeclaration(t *testing.T) {
	src := "package P {\n\tpart def <O>;\n\tpart a : O;\n\tpart b : P::O;\n}\n"
	for _, cursor := range []string{"O>;", "O;", "O;\n}"} {
		got := renameShortNameDoc(t, src, cursor, "Q")
		if want := "package P {\n\tpart def <Q>;\n\tpart a : Q;\n\tpart b : P::Q;\n}\n"; got != want {
			t.Errorf("rename at %q:\ngot:\n%s\nwant:\n%s", cursor, got, want)
		}
	}
}

// Both rules hold for every declaration kind that carries a short name.
func TestRenameShortNamesOfEveryDeclarationKind(t *testing.T) {
	tests := []struct {
		kind, src, cursor, newName string
		want                       []string
	}{
		{
			kind:    "usage long name",
			src:     "package P {\n\tattribute <n> num : ScalarValues::Integer = 1;\n\tattribute t : ScalarValues::Integer = n + num;\n}\n",
			cursor:  "num :",
			newName: "count",
			want:    []string{"attribute <n> count : ScalarValues::Integer = 1;", "= n + count;"},
		},
		{
			kind:    "usage short name",
			src:     "package P {\n\tattribute <n> num : ScalarValues::Integer = 1;\n\tattribute t : ScalarValues::Integer = n + num;\n}\n",
			cursor:  "n> num",
			newName: "k",
			want:    []string{"attribute <k> num : ScalarValues::Integer = 1;", "= k + num;"},
		},
		{
			kind:    "package long name",
			src:     "package <p> Pkg { part def X; }\npackage Q {\n\tpart a : p::X;\n\tpart b : Pkg::X;\n}\n",
			cursor:  "Pkg {",
			newName: "Lib",
			want:    []string{"package <p> Lib { part def X; }", "part a : p::X;", "part b : Lib::X;"},
		},
		{
			kind:    "package short name from reference",
			src:     "package <p> Pkg { part def X; }\npackage Q {\n\tpart a : p::X;\n\tpart b : Pkg::X;\n}\n",
			cursor:  "p::X",
			newName: "l",
			want:    []string{"package <l> Pkg { part def X; }", "part a : l::X;", "part b : Pkg::X;"},
		},
		{
			kind:    "alias long name",
			src:     "package P {\n\tpart def B;\n\talias <a> A for B;\n\tpart p : a;\n\tpart q : A;\n\tpart r : B;\n}\n",
			cursor:  "A for",
			newName: "Alt",
			want:    []string{"alias <a> Alt for B;", "part p : a;", "part q : Alt;", "part r : B;"},
		},
		{
			kind:    "alias short name",
			src:     "package P {\n\tpart def B;\n\talias <a> A for B;\n\tpart p : a;\n\tpart q : A;\n\tpart r : B;\n}\n",
			cursor:  "a> A",
			newName: "z",
			want:    []string{"alias <z> A for B;", "part p : z;", "part q : A;", "part r : B;"},
		},
		{
			kind:    "alias target long name leaves the alias's own names",
			src:     "package P {\n\tpart def B;\n\talias <a> A for B;\n\tpart p : a;\n\tpart q : A;\n\tpart r : B;\n}\n",
			cursor:  "B;\n\talias",
			newName: "Blk",
			want:    []string{"alias <a> A for Blk;", "part p : a;", "part q : A;", "part r : Blk;"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got := renameShortNameDoc(t, tt.src, tt.cursor, tt.newName)
			wantLines(t, got, tt.want...)
		})
	}
}

// A workspace with the declaration in one document and references spelled both
// ways in another: each rename reaches the other document's matching spellings only.
func TestRenameShortNameAcrossDocuments(t *testing.T) {
	const decl = "package P {\n\tpart def <O> Old;\n}\n"
	const uses = "package Q {\n\tpart a : P::O;\n\tpart b : P::Old;\n}\n"
	for _, tt := range []struct {
		cursor, newName, wantDecl, wantUses string
	}{
		{"Old;", "Fresh", "package P {\n\tpart def <O> Fresh;\n}\n", "package Q {\n\tpart a : P::O;\n\tpart b : P::Fresh;\n}\n"},
		{"O> Old", "N", "package P {\n\tpart def <N> Old;\n}\n", "package Q {\n\tpart a : P::N;\n\tpart b : P::Old;\n}\n"},
	} {
		ws := model.NewWorkspace()
		declName := openRenameDoc(t, ws, "/tmp/short_decl.sysml", decl)
		usesName := openRenameDoc(t, ws, "/tmp/short_uses.sysml", uses)
		got, err := applyRename(t, ws, declName, tt.cursor, tt.newName)
		if err != nil {
			t.Fatalf("Rename at %q err = %v", tt.cursor, err)
		}
		if got[declName] != tt.wantDecl {
			t.Errorf("rename at %q, declaring document:\ngot:  %q\nwant: %q", tt.cursor, got[declName], tt.wantDecl)
		}
		if got[usesName] != tt.wantUses {
			t.Errorf("rename at %q, referencing document:\ngot:  %q\nwant: %q", tt.cursor, got[usesName], tt.wantUses)
		}
	}
}

// An alias whose target is written with the short name composes with the alias
// contract: renaming the long name leaves the alias target alone, renaming the
// short name rewrites it, and neither touches uses of the alias's own name.
func TestRenameShortNameLeavesAliasTargetAlone(t *testing.T) {
	const src = "package P {\n\tpart def <O> Old;\n\talias X for P::O;\n\tpart a : X;\n\tpart b : Old;\n}\n"
	got := renameShortNameDoc(t, src, "Old;", "Fresh")
	if want := "package P {\n\tpart def <O> Fresh;\n\talias X for P::O;\n\tpart a : X;\n\tpart b : Fresh;\n}\n"; got != want {
		t.Errorf("long name:\ngot:\n%s\nwant:\n%s", got, want)
	}
	got = renameShortNameDoc(t, src, "O> Old", "Q")
	if want := "package P {\n\tpart def <Q> Old;\n\talias X for P::Q;\n\tpart a : X;\n\tpart b : Old;\n}\n"; got != want {
		t.Errorf("short name:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// prepareRenameAt runs PrepareRename at the first occurrence of anchor.
func prepareRenameAt(t *testing.T, s *Server, name, src, anchor string) (*protocol.Range, error) {
	t.Helper()
	off := strings.Index(src, anchor)
	if off < 0 {
		t.Fatalf("anchor %q not found", anchor)
	}
	return s.PrepareRename(context.Background(), &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
	})
}

// PrepareRename offers the short name's own span — inside the angle brackets on
// the declaration, the segment on a reference — and the brackets themselves name
// nothing renameable.
func TestPrepareRenameOffersShortNameSpan(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := openRenameDoc(t, ws, "/tmp/prepare_short.sysml", shortNameRenameSrc)

	for _, anchor := range []string{"O> Old", "O;", "O::x"} {
		rng, err := prepareRenameAt(t, s, name, shortNameRenameSrc, anchor)
		if err != nil {
			t.Fatalf("PrepareRename at %q err = %v", anchor, err)
		}
		start, end := positionToOffset([]byte(shortNameRenameSrc), rng.Start), positionToOffset([]byte(shortNameRenameSrc), rng.End)
		if want := strings.Index(shortNameRenameSrc, anchor); start != want || end != want+1 {
			t.Errorf("PrepareRename at %q: range [%d,%d), want [%d,%d)", anchor, start, end, want, want+1)
		}
	}

	for _, anchor := range []string{"<O>", "> Old"} {
		if _, err := prepareRenameAt(t, s, name, shortNameRenameSrc, anchor); err == nil {
			t.Errorf("PrepareRename on the bracket at %q succeeded, want error", anchor)
		}
		if _, err := applyRename(t, ws, name, anchor, "Q"); err == nil {
			t.Errorf("Rename on the bracket at %q succeeded, want error", anchor)
		}
	}
}

// Find-all-references lists every spelling that reaches the element: a reference
// written with the short name is a reference, even though only a short-name
// rename rewrites it.
func TestReferencesIncludeShortNameSpellings(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := openRenameDoc(t, ws, "/tmp/short_refs.sysml", shortNameRenameSrc)

	want := []uint32{
		lineOfSrc(t, shortNameRenameSrc, "part a : O;"),
		lineOfSrc(t, shortNameRenameSrc, "part b : Old;"),
		lineOfSrc(t, shortNameRenameSrc, "part c : P::O::x;"),
		lineOfSrc(t, shortNameRenameSrc, "part d : P::Old::x;"),
	}
	for _, anchor := range []string{"Old {", "O> Old", "O;", "Old;"} {
		if got := startLines(referencesAt(t, s, name, shortNameRenameSrc, anchor)); !equalLines(got, want) {
			t.Errorf("References at %q: lines = %v, want %v", anchor, got, want)
		}
	}
}

// The new-name rules apply to a short name as to a long one.
func TestRenameShortNameRejectsInvalidNames(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/short_bad.sysml", shortNameRenameSrc)
	for _, newName := range []string{"", "part", "2Q", "a b"} {
		if _, err := applyRename(t, ws, name, "O> Old", newName); err == nil {
			t.Errorf("rename short name to %q succeeded, want error", newName)
		}
	}
}
