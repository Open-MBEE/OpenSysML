package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// F64: `ReturnParameterMember` is `'return' UsageElement` (SysML.xtext:1961), so
// a specialization after `return` declares the result parameter rather than
// returning the value of an expression.
func TestF64ReturnUsage(t *testing.T) {
	returned := func(t *testing.T, src string) *ast.Usage {
		t.Helper()
		sf := source.New("f64.sysml", []byte(src))
		p := New(sf)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
		}
		pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
		def := pkg.Members[len(pkg.Members)-1].(*ast.Membership).Member.(*ast.Definition)
		last := def.Members[len(def.Members)-1]
		if m, ok := last.(*ast.Membership); ok {
			last = m.Member
		}
		u, ok := last.(*ast.Usage)
		if !ok {
			t.Fatalf("last calc member = %T, want *ast.Usage", last)
		}
		if !u.IsResult {
			t.Error("IsResult = false, want a result parameter")
		}
		return u
	}

	t.Run("subsetting_names_the_result", func(t *testing.T) {
		u := returned(t, `package B {
			part def Engine;
			calc def SelectEngine { in engine : Engine[2]; return selectedEngine :> engine; }
		}`)
		if u.Ident.Name != "selectedEngine" {
			t.Errorf("name = %q, want selectedEngine", u.Ident.Name)
		}
		if len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelSubsets {
			t.Fatalf("relationships = %v, want one subsets", u.Relationships)
		}
	})

	t.Run("kind_specialization_multiplicity_and_value", func(t *testing.T) {
		u := returned(t, `package B {
			calc def AccelProfile { return attribute accelerationProfile :> ISQ::acceleration[*] := (); }
		}`)
		if u.Ident.Name != "accelerationProfile" || u.Kind != ast.UsageAttribute {
			t.Errorf("got name=%q kind=%v, want accelerationProfile as an attribute", u.Ident.Name, u.Kind)
		}
		if len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelSubsets {
			t.Fatalf("relationships = %v, want one subsets", u.Relationships)
		}
		if u.Multiplicity == nil {
			t.Error("multiplicity = nil, want [*]")
		}
		if u.Value == nil {
			t.Error("value = nil, want the empty sequence")
		}
	})

	t.Run("redefinition_names_the_result", func(t *testing.T) {
		u := returned(t, `package B {
			calc def C { return result :>> outer : ScalarValues::Real; }
		}`)
		if u.Ident.Name != "result" {
			t.Errorf("name = %q, want result", u.Ident.Name)
		}
		if len(u.Relationships) != 2 || u.Relationships[0].Kind != ast.RelRedefines {
			t.Fatalf("relationships = %v, want redefines then typing", u.Relationships)
		}
	})
}

// A `return` states a declaration, so an expression after it is refused while
// the trailing expression that states the computed result is read.
func TestF64ReturnExpressionRefused(t *testing.T) {
	src := `package B {
		calc def C { in n : ScalarValues::Integer; return n + 1; }
		calc def D { in n : ScalarValues::Integer; n * 2 }
	}`
	sf := source.New("f64.sysml", []byte(src))
	p := New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want one for `return n + 1;`", p.Diagnostics)
	}
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	c := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
	if _, ok := c.Members[1].(*ast.ErrorNode); !ok {
		t.Errorf("returned expression = %T, want *ast.ErrorNode", c.Members[1])
	}
	d := pkg.Members[1].(*ast.Membership).Member.(*ast.Definition)
	if _, ok := d.Members[1].(*ast.OperatorExpr); !ok {
		t.Errorf("trailing expression = %T, want *ast.OperatorExpr", d.Members[1])
	}
}

// A malformed returned usage must produce diagnostics, never a panic.
func TestF64ReturnUsageNegative(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"subsets_no_target", "calc def C { return selected :> ; }"},
		{"subsets_no_terminator", "calc def C { return selected :> engine }"},
		{"kind_subsets_no_target", "calc def C { return attribute profile :> ; }"},
		{"subsets_value_no_expr", "calc def C { return selected :> engine := ; }"},
		{"subsets_unclosed_multiplicity", "calc def C { return selected :> engine[ ; }"},
		{"subsets_at_eof", "calc def C { return selected :>"},
		{"kind_only_at_eof", "calc def C { return attribute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := source.New("f64_neg.sysml", []byte(tt.input))
			p := New(sf)
			if p.ParseFile() == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) == 0 {
				t.Errorf("expected diagnostics for %q, got none", tt.input)
			}
		})
	}
}
