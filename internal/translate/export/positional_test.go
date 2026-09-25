package export

import (
	"testing"

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

func TestDeclarationNamesRefusesPositionalCollision(t *testing.T) {
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
	if _, err := DeclarationNames(file, root); err == nil {
		t.Fatal("DeclarationNames succeeded, want the exporter's positional collision error")
	}
}
