package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// overloadNavigationSrc imports two same-named calcs; each call's argument
// selects one of them.
const overloadNavigationSrc = `package Lib {
	private import ScalarValues::*;
	calc def pick { in x : Integer; return : Integer = x; }
	calc def pick { in x : String; return : String = x; }
}
package P {
	private import ScalarValues::*;
	private import Lib::*;
	attribute i : Integer = pick(2);
	attribute s : String = pick("s");
}
`

func definitionOf(t *testing.T, s *Server, name, src, needle string) []protocol.Location {
	t.Helper()
	off := strings.Index(src, needle)
	if off < 0 {
		t.Fatalf("%q not in source", needle)
	}
	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	return locs
}

// Go-to-definition on an overloaded call lands on the overload its argument
// selects, not the first declaration of the name.
func TestDefinitionFollowsSelectedOverload(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/overload_nav.sysml").Filename()
	ws.Open(name, []byte(overloadNavigationSrc), 1)

	lines := strings.Split(overloadNavigationSrc, "\n")
	lineOf := func(needle string) uint32 {
		for i, l := range lines {
			if strings.Contains(l, needle) {
				return uint32(i)
			}
		}
		t.Fatalf("%q not in source", needle)
		return 0
	}
	cases := []struct {
		call, decl string
	}{
		{`pick(2)`, "in x : Integer"},
		{`pick("s")`, "in x : String"},
	}
	for _, tc := range cases {
		locs := definitionOf(t, s, name, overloadNavigationSrc, tc.call)
		if len(locs) != 1 {
			t.Fatalf("%s: locations = %d, want 1", tc.call, len(locs))
		}
		if locs[0].URI != uri.File(name) {
			t.Errorf("%s: URI = %q, want %q", tc.call, locs[0].URI, uri.File(name))
		}
		if got, want := locs[0].Range.Start.Line, lineOf(tc.decl); got != want {
			t.Errorf("%s: decl line = %d, want %d (%s)", tc.call, got, want, tc.decl)
		}
	}
}

// Find-references from an overload's declaration reports only the calls that
// select it.
func TestReferencesDistinguishOverloads(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/overload_refs.sysml").Filename()
	ws.Open(name, []byte(overloadNavigationSrc), 1)

	src := overloadNavigationSrc
	off := strings.Index(src, "pick { in x : String")
	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("references = %d, want only the String call: %v", len(locs), locs)
	}
	at := positionToOffset([]byte(src), locs[0].Range.Start)
	if !strings.HasPrefix(src[at:], `pick("s")`) {
		t.Errorf("reference at %q, want the String call", src[at:at+9])
	}
}

// Go-to-definition on an imported library call opens the bundled declaration the checker
// and runtime bind it to; unimported, the call resolves to nothing.
func TestDefinitionReachesImportedLibraryFunction(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/unimported_nav.sysml").Filename()
	ws.Open(name, []byte(unimportedLibraryCallSrc), 1)
	if locs := definitionOf(t, s, name, unimportedLibraryCallSrc, "sqrt(4.0)"); len(locs) != 0 {
		t.Fatalf("unimported call: locations = %v, want none", locs)
	}

	name = uri.File("/tmp/imported_nav.sysml").Filename()
	ws.Open(name, []byte(importedLibraryCallSrc), 1)
	locs := definitionOf(t, s, name, importedLibraryCallSrc, "sqrt(4.0)")
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	if got := string(locs[0].URI); !strings.HasPrefix(got, LibraryScheme+":") || !strings.Contains(got, "RealFunctions") {
		t.Errorf("URI = %q, want the bundled RealFunctions document", got)
	}
	doc := s.document(uriToName(locs[0].URI))
	if doc == nil {
		t.Fatalf("library document %q not openable", locs[0].URI)
	}
	line := strings.Split(string(doc.Content), "\n")[locs[0].Range.Start.Line]
	if !strings.Contains(line, "function sqrt") {
		t.Errorf("definition line = %q, want the sqrt declaration", line)
	}
}

// Go-to-definition through a qualifier that both inherits and imports the name
// opens the inherited declaration, which hides the import, for a call and a plain reference.
func TestDefinitionQualifiedInheritedHidesImported(t *testing.T) {
	const src = `package P {
	private import ScalarValues::*;
	package Lib { calc def pick { in x : Integer; return : Integer = 1; } }
	part def Base { calc def pick { in x : String; return : Integer = 2; } }
	part def T :> Base { public import Lib::pick; }
	attribute s : Integer = T::pick("s");
	calc def again :> T::pick;
}
`
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/qualified_hidden_nav.sysml").Filename()
	ws.Open(name, []byte(src), 1)

	want := uint32(strings.Index(src, "in x : String"))
	wantLine := offsetToPosition([]byte(src), int(want)).Line
	for _, needle := range []string{`pick("s")`, "pick;\n}"} {
		locs := definitionOf(t, s, name, src, needle)
		if len(locs) != 1 {
			t.Fatalf("%s: locations = %d, want 1: %v", needle, len(locs), locs)
		}
		if locs[0].Range.Start.Line != wantLine {
			t.Errorf("%s: decl line = %d, want %d (the inherited pick)", needle, locs[0].Range.Start.Line, wantLine)
		}
	}
}
