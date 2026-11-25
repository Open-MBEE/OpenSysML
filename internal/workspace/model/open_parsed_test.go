package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A tree parsed elsewhere and installed without parsing gives the workspace the
// same document, index and diagnostics as opening the same text normally.
func TestOpenParsedMatchesOpeningTheSameText(t *testing.T) {
	const name, text = "a.sysml", "package A { part def Wheel; part w : Wheel; }\n"
	fresh := NewWorkspaceWithIndex(symbols.NewIndex())
	fresh.OpenWithKind(name, []byte(text), 1, source.KindSysML)
	want := fresh.Document(name)

	sf := source.NewWithKind(name, []byte(text), source.KindSysML)
	p := parser.New(sf)
	root := p.ParseFile()
	parsed := NewWorkspaceWithIndex(symbols.NewIndex())
	parsed.OpenParsed(name, []byte(text), 1, source.KindSysML, root, p.Diagnostics, p.Warnings)
	got := parsed.Document(name)

	if got == nil || got.AST != root {
		t.Fatal("the pre-parsed tree was not installed")
	}
	if got.Digest() != want.Digest() || got.Kind() != want.Kind() || got.Version != want.Version {
		t.Fatalf("installed document differs from one opened normally: %+v vs %+v", got, want)
	}
	if len(got.ParseDiagnostics) != len(want.ParseDiagnostics) || len(got.TopMembers()) != len(want.TopMembers()) {
		t.Fatalf("parsed facts differ: %d/%d diagnostics, %d/%d members",
			len(got.ParseDiagnostics), len(want.ParseDiagnostics), len(got.TopMembers()), len(want.TopMembers()))
	}
	for _, fqn := range []string{"A", "A::Wheel", "A::w"} {
		if len(parsed.LookupQualified(fqn)) == 0 || len(parsed.LookupQualified(fqn)) != len(fresh.LookupQualified(fqn)) {
			t.Errorf("%q resolves differently in the installed workspace", fqn)
		}
	}
}

// A nil tree is a caller with nothing to install: the text is parsed instead of
// indexing an empty document.
func TestOpenParsedWithNoTreeParsesTheText(t *testing.T) {
	const name, text = "b.sysml", "package B { part x; }\n"
	ws := NewWorkspaceWithIndex(symbols.NewIndex())
	ws.OpenParsed(name, []byte(text), 0, source.KindSysML, nil, nil, nil)
	doc := ws.Document(name)
	if doc == nil || doc.AST == nil || len(doc.TopMembers()) == 0 {
		t.Fatal("a nil tree left no usable document")
	}
	if len(ws.LookupQualified("B::x")) == 0 {
		t.Fatal("the text parsed on the fallback was not indexed")
	}
}
