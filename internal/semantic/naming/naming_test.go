package naming

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func indexed(t *testing.T, name, text string) (*source.SourceFile, *ast.RootNamespace, *symbols.Index) {
	t.Helper()
	file := source.New(name, []byte(text))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics = %v", p.Diagnostics)
	}
	index := symbols.NewIndex()
	index.AddDocument(file.Name(), root)
	return file, root, index
}

func built(t *testing.T, file *source.SourceFile, root *ast.RootNamespace, index *symbols.Index) *Table {
	t.Helper()
	res := resolve.New(index)
	model := semantics.NewModel(res)
	res.SetModel(model)
	res.ResolveDocument(file.Name(), root)
	names, err := Build(file, root, model, identity.Build(model, res, index.DocumentRoot(file.Name())))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return names
}

func TestBuildNamesUnnamedMemberByPosition(t *testing.T) {
	file, root, index := indexed(t, "small.sysml", `package Demo { part def Item; part : Item; }`)
	names := built(t, file, root, index)
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	unnamed := pkg.Members[1].(*ast.Membership).Member.(*ast.Usage)
	if got := names.FQN[unnamed]; got != "Demo::@1" {
		t.Errorf("FQN[unnamed] = %q, want Demo::@1", got)
	}
}

func TestBuildKeepsNamedAndPositionalIdentitiesDistinct(t *testing.T) {
	file, root, index := indexed(t, "collision.sysml", `package Collision {
		metadata def M;
		#M part def Car {
			part def '@1';
		}
	}`)
	names := built(t, file, root, index)
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	car := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition)
	member := car.Members[0].(*ast.Membership).Member.(*ast.Definition)
	if got := names.FQN[member]; got != "Collision::Car::'@1'" {
		t.Errorf("named member identity = %q, want Collision::Car::'@1'", got)
	}
	if got := names.FQN[car.Prefixes[0]]; got != "Collision::Car::@1" {
		t.Errorf("prefix identity = %q, want Collision::Car::@1", got)
	}
}

func TestPositionalIdentitiesNameUnnamedMembers(t *testing.T) {
	file, root, index := indexed(t, "small.sysml", `package Demo { part def Item; part : Item; }`)
	pkgs := index.LookupQualified("Demo")
	if len(pkgs) == 0 {
		t.Fatal("index has no Demo package")
	}
	var unnamed *symbols.Symbol
	for _, sym := range pkgs[0].Scope.AllMembers() {
		if sym.Name == "" {
			unnamed = sym
			break
		}
	}
	if unnamed == nil {
		t.Fatal("index has no unnamed usage")
	}
	positional, reverse := PositionalIdentities(index, []Document{{
		File:  file,
		Root:  root,
		Scope: index.DocumentRoot(file.Name()),
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

func TestPositionalIdentitiesNamePrefixMetadataMembers(t *testing.T) {
	file, root, index := indexed(t, "prefix.sysml", `package P {
		part def Holder { part; part named; }
		metadata def M;
		part def Pump { #M part x; }
	}`)
	positional, _ := PositionalIdentities(index, []Document{{File: file, Root: root, Scope: index.DocumentRoot(file.Name())}},
		func(sym *symbols.Symbol) bool { return index.QualifiedIdentity(sym) != "" })
	got := map[string]bool{}
	for _, name := range positional {
		got[name] = true
	}
	for _, want := range []string{"P::Holder::@0", "P::Pump::x::@0"} {
		if !got[want] {
			t.Errorf("positional identities %v lack %s", got, want)
		}
	}
}

func TestPositionalIdentityIgnoresImpliedConjugate(t *testing.T) {
	file, root, index := indexed(t, "short-port.sysml", `package P { port def <s>; }`)
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
	names := built(t, file, root, index)
	if got := names.FQN[short.Decl]; got != "P::@0" {
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
