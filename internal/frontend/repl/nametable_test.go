package repl

import (
	"errors"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// walkScopeTree is the reference the name table stands in for: every symbol
// named name in scope or a nested scope, skipping body-local scopes.
func walkScopeTree(scope *symbols.Scope, name string) []*symbols.Symbol {
	if scope == nil || scope.BodyLocal() {
		return nil
	}
	var out []*symbols.Symbol
	if syms := scope.LookupLocalAll(name); len(syms) > 0 {
		out = append(out, symbols.PreferDeclared(syms)...)
	}
	for _, child := range scope.Children() {
		out = append(out, walkScopeTree(child, name)...)
	}
	return out
}

// everyName is every key any scope in the tree registers, body-local ones too.
func everyName(scope *symbols.Scope, into map[string]bool) {
	if scope == nil {
		return
	}
	for _, name := range scope.MemberNames() {
		into[name] = true
	}
	for _, child := range scope.Children() {
		everyName(child, into)
	}
}

// The table answers exactly what a walk of the scope trees answers, symbol for
// symbol and in the same order: duplicates in one scope, the same name in two
// packages, a name borrowed from a redefined feature next to a declared one,
// body-local names, and both session documents.
func TestNameTableMatchesScopeTreeWalk(t *testing.T) {
	s := NewSession()
	s.SubmitFiles([]SourceFile{
		{Name: "a.sysml", Text: `package P {
			part def Vehicle { part engine; attribute mass; }
			part v : Vehicle { part :>> engine; attribute :>> mass = 1.0; }
			part w : Vehicle { part engine; part :>> engine; }
			part def Twice; part def Twice;
			action def Sample {
				in attribute samples;
				assert constraint { samples->forAll { in bodyParam; bodyParam > 0 } }
				loop action charging { } until true;
				if true { action thenLocal; } else { action elseLocal; }
			}
		}
		package Q { part def Vehicle; part def Twice; }`},
		{Name: "b.kerml", Text: `package K { class Vehicle; datatype Twice; feature engine; }`},
	})
	scopes := s.docScopes()
	if len(scopes) != 2 {
		t.Fatalf("want both session documents, got %d scope trees", len(scopes))
	}

	names := map[string]bool{"NoSuchName": true}
	for _, scope := range scopes {
		everyName(scope, names)
	}
	for _, must := range []string{"Vehicle", "Twice", "engine", "mass", "samples", "bodyParam", "charging", "thenLocal", "elseLocal"} {
		if !names[must] {
			t.Fatalf("the fixture no longer declares %s", must)
		}
	}

	table := s.nameTable()
	for name := range names {
		var want []*symbols.Symbol
		for _, scope := range scopes {
			want = append(want, walkScopeTree(scope, name)...)
		}
		if got := table.lookup(name); !slices.Equal(got, want) {
			t.Errorf("%s: table = %v, walk = %v", name, got, want)
		}
	}
	if got, want := table.lookup("Vehicle"), 3; len(got) != want {
		t.Errorf("Vehicle: %d declarations, want %d (P, Q and K)", len(got), want)
	}
	if got, want := table.lookup("Twice"), 4; len(got) != want {
		t.Errorf("Twice: %d declarations, want %d", len(got), want)
	}
	// In w the declared engine hides the borrowed one; in v only the borrowed
	// one exists, so engine has three: Vehicle's, v's, w's — plus K's feature.
	if got, want := table.lookup("engine"), 4; len(got) != want {
		t.Errorf("engine: %d declarations, want %d", len(got), want)
	}
	if got := s.declaredSymbolNames(); slices.Contains(got, "bodyParam") || !slices.Contains(got, "samples") {
		t.Errorf("declared names = %v: must hold samples and not bodyParam", got)
	}
}

// The table is built once per document: two lookups share it, and a submission
// or a reset that replaces the scope trees replaces it, so a lookup never
// answers from a document the session no longer holds.
func TestNameTableRebuiltOnlyWhenDocumentsChange(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def A; }")
	s.Submit("package Q { part def Old; }")

	first := s.nameTable()
	a := first.lookup("A")
	if len(a) != 1 {
		t.Fatalf("A: %d declarations, want 1", len(a))
	}
	if _, _, err := s.lookupSymbol("Old"); err != nil {
		t.Fatalf("Old: %v", err)
	}
	if s.nameTable() != first {
		t.Fatal("a lookup rebuilt the table although no document changed")
	}

	// An empty body empties the namespace rather than adding to it.
	s.Submit("package Q { }")
	second := s.nameTable()
	if second == first {
		t.Fatal("a submission left the table over the previous scope tree")
	}
	if got := second.lookup("A"); len(got) != 1 || got[0] == a[0] || got[0].Decl == a[0].Decl {
		t.Errorf("A after resubmit = %v: want the new document's declaration, not %v", got, a)
	}
	s.Submit("package Q { part def New; }")
	if sym, fqn, err := s.lookupSymbol("New"); err != nil || fqn != "Q::New" || sym != s.nameTable().lookup("New")[0] {
		t.Errorf("New after resubmit = %v, %q, %v", sym, fqn, err)
	}
	second = s.nameTable()
	if _, fqn, err := s.lookupSymbol("Old"); err == nil {
		t.Errorf("Old still resolves as %q after the submission dropped it", fqn)
	}
	if s.nameTable() != second {
		t.Fatal("a lookup rebuilt the table although no document changed")
	}

	s.Clear()
	if got := s.nameTable().sorted(); len(got) != 0 {
		t.Errorf("declared names after a reset = %v, want none", got)
	}
	if _, fqn, err := s.lookupSymbol("A"); err == nil {
		t.Errorf("A still resolves as %q after a reset", fqn)
	}
	s.Submit("package R { part def A; }")
	if _, fqn, err := s.lookupSymbol("A"); err != nil || fqn != "R::A" {
		t.Errorf("A after a reset and a new submission = %q, %v; want R::A", fqn, err)
	}
}

// A name two packages declare is ambiguous with both candidates named; once a
// submission takes one away the same name resolves to the other.
func TestSimpleNameAmbiguityFollowsResubmission(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def X; }")
	s.Submit("package Q { part def X; }")

	_, _, err := s.lookupSymbol("X")
	var ambiguous *AmbiguousNameError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("X: got %v, want an AmbiguousNameError", err)
	}
	if ambiguous.Name != "X" || !slices.Equal(ambiguous.FQNs, []string{"P::X", "Q::X"}) {
		t.Errorf("ambiguity = %q %v, want X [P::X Q::X]", ambiguous.Name, ambiguous.FQNs)
	}

	s.Submit("package Q { }")
	if _, fqn, err := s.lookupSymbol("X"); err != nil || fqn != "P::X" {
		t.Errorf("X after Q dropped it = %q, %v; want P::X", fqn, err)
	}
}

// A root declaration has no more qualified spelling than its simple name, so
// that name denotes it even where nested declarations share the name.
func TestRootDeclarationWinsOverNestedNamesakes(t *testing.T) {
	s := NewSession()
	s.Submit("package 'Scanning Electron Microscope' { part def 'Scanning Electron Microscope'; }")
	s.Submit("package Blocks { part def 'Scanning Electron Microscope'; }")

	_, fqn, err := s.lookupSymbol("'Scanning Electron Microscope'")
	if err != nil || fqn != "Scanning Electron Microscope" {
		t.Errorf("root 'Scanning Electron Microscope' = %q, %v; want the root package", fqn, err)
	}

	s.Submit("package R { part def 'Scanning Electron Microscope'; }")
	if _, fqn, err := s.lookupSymbol("'Scanning Electron Microscope'"); err != nil || fqn != "Scanning Electron Microscope" {
		t.Errorf("root 'Scanning Electron Microscope' with three namesakes = %q, %v; want the root package", fqn, err)
	}

	_, _, err = s.lookupSymbol("Blocks::'Scanning Electron Microscope'")
	if err != nil {
		t.Errorf("Blocks::'Scanning Electron Microscope' = %v; want the nested declaration", err)
	}
}

// A root declaration whose short name repeats its name is still one declaration,
// so it is chosen over a nested namesake rather than counted twice.
func TestRootDeclarationWithRepeatedShortNameWinsOverNestedNamesake(t *testing.T) {
	s := NewSession()
	s.Submit("package <P> P; package Outer { package P; }")
	_, fqn, err := s.lookupSymbol("P")
	if err != nil || fqn != "P" {
		t.Errorf("P = %q, %v; want the root package", fqn, err)
	}
}

// The root declaration a simple name resolves to is the document's own symbol,
// the one expressions resolve to, not the index's copy of it.
func TestRootDeclarationResolvesToTheDocumentSymbol(t *testing.T) {
	s := NewSession()
	s.Submit("part car; package P { part car; }")
	sym, fqn, err := s.lookupSymbol("car")
	if err != nil || fqn != "car" {
		t.Fatalf("car = %q, %v; want the root part", fqn, err)
	}
	if local := scopeSymbolForAny(s.docScopes(), sym.Decl); local != sym {
		t.Errorf("car resolved to %p, want the document's own symbol %p", sym, local)
	}
}

// A name a root import brings in is not a root declaration: nested namesakes
// stay ambiguous rather than being answered with the imported one.
func TestRootImportDoesNotResolveNestedNamesakes(t *testing.T) {
	s := NewSession()
	s.Submit("package Base { part def X; }")
	s.Submit("import Base::*;")
	s.Submit("package P { part def X; }")
	s.Submit("package Q { part def X; }")

	_, _, err := s.lookupSymbol("X")
	var ambiguous *AmbiguousNameError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("X: got %v, want an AmbiguousNameError", err)
	}
	if !slices.Equal(ambiguous.FQNs, []string{"Base::X", "P::X", "Q::X"}) {
		t.Errorf("ambiguity = %v, want [Base::X P::X Q::X]", ambiguous.FQNs)
	}
}

// A simple name the session declares nowhere is still resolved where the prompt
// evaluates, through the imports visible there.
func TestSimpleNameFallsBackToPromptImports(t *testing.T) {
	s := NewSession()
	s.Submit("package P { import ScalarValues::*; }")
	sym, fqn, err := s.lookupSymbol("Real")
	if err != nil || sym == nil || fqn != "ScalarValues::Real" {
		t.Fatalf("Real through the prompt's import = %v, %q, %v", sym, fqn, err)
	}
	if got := s.nameTable().lookup("Real"); len(got) != 0 {
		t.Errorf("the table holds Real (%v) although the session does not declare it", got)
	}
}
