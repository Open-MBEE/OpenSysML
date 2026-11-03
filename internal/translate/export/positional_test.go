package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestDeclarationNamesUsesPositionalExporterName(t *testing.T) {
	file := source.New("small.sysml", []byte(`package Demo { part def Item; part : Item; }`))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics = %v", p.Diagnostics)
	}
	names, err := DeclarationNames(file, root)
	if err != nil {
		t.Fatalf("DeclarationNames: %v", err)
	}
	if names == nil {
		t.Fatal("DeclarationNames returned a nil map")
	}
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	unnamed := pkg.Members[1].(*ast.Membership).Member.(*ast.Usage)
	if got := names[unnamed.Span()]; got != "Demo::@1" {
		t.Errorf("DeclarationNames[%v] = %q, want Demo::@1", unnamed.Span(), got)
	}
}

func TestDeclarationNamesKeepsNamedAndPositionalIdentitiesDistinct(t *testing.T) {
	file := source.New("collision.sysml", []byte(`package Collision {
		metadata def M;
		#M part def Car {
			part def '@1';
		}
	}`))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics = %v", p.Diagnostics)
	}
	names, err := DeclarationNames(file, root)
	if err != nil {
		t.Fatalf("DeclarationNames: %v", err)
	}
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	car := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition)
	member := car.Members[0].(*ast.Membership).Member.(*ast.Definition)
	if got := names[member.Span()]; got != "Collision::Car::'@1'" {
		t.Errorf("named member identity = %q, want Collision::Car::'@1'", got)
	}
	if got := names[car.Prefixes[0].Span()]; got != "Collision::Car::@1" {
		t.Errorf("prefix identity = %q, want Collision::Car::@1", got)
	}
}

func TestPositionalIdentitiesUsesExporterNames(t *testing.T) {
	file := source.New("small.sysml", []byte(`package Demo { part def Item; part : Item; }`))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics = %v", p.Diagnostics)
	}
	index := symbols.NewIndex()
	index.AddDocument(file.Name(), root)
	pkgs := index.LookupQualified("Demo")
	if len(pkgs) == 0 {
		t.Fatal("index has no Demo package")
	}
	pkg := pkgs[0]
	var unnamed *symbols.Symbol
	for _, sym := range pkg.Scope.AllMembers() {
		if sym.Name == "" {
			unnamed = sym
			break
		}
	}
	if unnamed == nil {
		t.Fatal("index has no unnamed usage")
	}
	positional, reverse := PositionalIdentities(index, []PositionalDocument{{
		File: file,
		Root: index.DocumentRoot(file.Name()),
	}}, func(sym *symbols.Symbol) bool {
		fqn := index.GetFQN(sym)
		for _, candidate := range index.LookupQualified(fqn) {
			if candidate == sym {
				return fqn != ""
			}
		}
		return false
	})
	if got := positional[unnamed]; got != "Demo::@1" {
		t.Errorf("PositionalIdentities[%p] = %q, want Demo::@1", unnamed, got)
	}
	if got := reverse["Demo::@1"]; got != unnamed {
		t.Errorf("reverse positional identity = %p, want %p", got, unnamed)
	}
}

func TestPositionalIdentityIgnoresImpliedConjugate(t *testing.T) {
	file := source.New("short-port.sysml", []byte(`package P { port def <s>; }`))
	p := parser.New(file)
	root := p.ParseFile()
	index := symbols.NewIndex()
	index.AddDocument(file.Name(), root)
	pkg := index.LookupQualified("P")[0]
	short, ok := pkg.Scope.LookupLocal("s")
	if !ok {
		t.Fatal("index has no short-named port definition")
	}
	var conjugate *symbols.Symbol
	for _, member := range short.Scope.AllMembers() {
		if member.IsConjugatedPortDefinition() {
			conjugate = member
			break
		}
	}
	if conjugate == nil || conjugate.DeclSpan != short.DeclSpan {
		t.Fatalf("conjugate %v does not share the original declaration span", conjugate)
	}
	names, err := DeclarationNames(file, root)
	if err != nil {
		t.Fatal(err)
	}
	if got := names[short.Decl.Span()]; got != "P::@0" {
		t.Fatalf("short port declaration name = %q, want P::@0", got)
	}
	identifies := func(sym *symbols.Symbol) bool {
		if sym == short || sym == conjugate {
			return false
		}
		return index.GetFQN(sym) != ""
	}
	if !positionalSymbol(short, file.Name(), identifies) {
		t.Fatal("short-named port definition is not eligible for its positional identity")
	}
	if positionalSymbol(conjugate, file.Name(), identifies) {
		t.Fatal("implied conjugate is eligible for its original's positional identity")
	}
}

func TestIsPositionalIdentityRequiresCanonicalIndex(t *testing.T) {
	for _, test := range []struct {
		name string
		want bool
	}{
		{"Demo::@0", true},
		{"Demo::@1::wheel", true},
		{"Demo::@01", false},
		{"Demo::@-1", false},
		{"P::'@2'", false},
		{"P::'@2'::@0", true},
		{"Demo::wheel", false},
	} {
		if got := IsPositionalIdentity(test.name); got != test.want {
			t.Errorf("IsPositionalIdentity(%q) = %t, want %t", test.name, got, test.want)
		}
	}
}
