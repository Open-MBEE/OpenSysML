package query

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestQualifiedIdentityRequiresAnIndexedQualifiedName(t *testing.T) {
	file := source.New("identity.sysml", []byte(`package Demo { part def Item; part : Item; }`))
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
		t.Fatal("model has no unnamed usage")
	}
	namedSymbols := index.LookupQualified("Demo::Item")
	if len(namedSymbols) == 0 {
		t.Fatal("index has no Demo::Item element")
	}
	named := namedSymbols[0]
	if got := QualifiedIdentity(index, named); got != "Demo::Item" {
		t.Errorf("QualifiedIdentity(named) = %q, want Demo::Item", got)
	}
	if got := QualifiedIdentity(index, unnamed); got != "" {
		t.Errorf("QualifiedIdentity(unnamed) = %q, want empty", got)
	}
}
