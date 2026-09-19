package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestAnonymousFeatureWithModifier(t *testing.T) {
	input := `action def Test {
		ref stateSpace: StateSpace;
	}`
	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	file := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Errorf("Parse error: %s", d.Message)
		}
		t.FailNow()
	}

	// File has one namespace member (action def)
	def := file.Members[0].(*ast.Membership).Member.(*ast.Definition)
	bodyMem := def.Members[0].(*ast.Membership)
	usage := bodyMem.Member.(*ast.Usage)

	if usage.Kind != ast.UsageAttribute {
		t.Errorf("Expected UsageAttribute, got %v", usage.Kind)
	}
	if usage.Ident.Name != "stateSpace" {
		t.Errorf("Expected name 'stateSpace', got %s", usage.Ident.Name)
	}
	if !usage.IsReference {
		t.Error("Expected IsReference=true")
	}
	if len(usage.Relationships) < 1 {
		t.Fatal("Expected at least 1 relationship")
	}
	if usage.Relationships[0].Kind != ast.RelTyping {
		t.Errorf("Expected RelTyping, got %v", usage.Relationships[0].Kind)
	}
}

func TestAnonymousFeatureUnrestrictedNameIsUnquoted(t *testing.T) {
	cases := []struct {
		name string
		body string
		ref  bool
	}{
		{"keyword subsets", "ref 'spare wheel' subsets base;", true},
		{"symbolic subsets", "ref 'spare wheel' :> base;", true},
		{"typed", "ref 'spare wheel' : Wheel;", true},
		{"bare", "ref 'spare wheel';", true},
		{"multiplicity", "ref 'spare wheel'[2];", true},
		{"typed without modifier", "'spare wheel' : Wheel;", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := "part def D {\n\t" + tc.body + "\n\tpart base;\n}"
			p := New(source.New("test.sysml", []byte(input)))
			file := p.ParseFile()
			for _, d := range p.Diagnostics {
				t.Errorf("parse error: %s", d.Message)
			}
			def := file.Members[0].(*ast.Membership).Member.(*ast.Definition)
			usage := def.Members[0].(*ast.Membership).Member.(*ast.Usage)
			if usage.Ident.Name != "spare wheel" {
				t.Errorf("name = %q, want %q", usage.Ident.Name, "spare wheel")
			}
			if usage.IsReference != tc.ref {
				t.Errorf("IsReference = %v, want %v", usage.IsReference, tc.ref)
			}
		})
	}
}
