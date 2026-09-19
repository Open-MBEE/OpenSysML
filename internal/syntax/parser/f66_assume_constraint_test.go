package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// F66: `AssumeConstraintUsage` is `'assume' ConstraintUsageKeyword
// ConstraintUsageDeclaration CalculationBody` (SysML.xtext:2070, :2015), and
// both the declaration and the body are optional — so the constraint an
// assumption owns may be named, typed and ended with `;` instead of a body.
func TestF66AssumeConstraintDeclaration(t *testing.T) {
	reqBody := func(t *testing.T, src string) []ast.Node {
		t.Helper()
		sf := source.New("f66.sysml", []byte(src))
		p := New(sf)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
		}
		pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
		def := pkg.Members[len(pkg.Members)-1].(*ast.Membership).Member.(*ast.Definition)
		return def.Members
	}

	t.Run("named_and_typed", func(t *testing.T) {
		members := reqBody(t, `package B {
			constraint def C;
			requirement def R { assume constraint c1 : C; }
		}`)
		a, ok := members[0].(*ast.AssumeMember)
		if !ok {
			t.Fatalf("member = %T, want *ast.AssumeMember", members[0])
		}
		if a.Ident.Name != "c1" {
			t.Errorf("name = %q, want c1", a.Ident.Name)
		}
		if len(a.Relationships) != 1 || a.Relationships[0].Kind != ast.RelTyping {
			t.Fatalf("relationships = %v, want one typing", a.Relationships)
		}
		if a.HasBody {
			t.Error("HasBody = true, want a declaration ended with ';'")
		}
	})

	t.Run("named_with_body", func(t *testing.T) {
		members := reqBody(t, `package B {
			part def Vehicle { attribute fuelLevel : ScalarValues::Real; }
			requirement def R {
				subject vehicle : Vehicle;
				assume constraint fuelConstraint { vehicle.fuelLevel >= 0.0 }
			}
		}`)
		a, ok := members[1].(*ast.AssumeMember)
		if !ok {
			t.Fatalf("member = %T, want *ast.AssumeMember", members[1])
		}
		if a.Ident.Name != "fuelConstraint" || !a.HasBody || len(a.Body) == 0 {
			t.Errorf("got name=%q hasBody=%v conditions=%d, want a named constraint with a body",
				a.Ident.Name, a.HasBody, len(a.Body))
		}
	})

	t.Run("anonymous_with_body_unchanged", func(t *testing.T) {
		members := reqBody(t, `package B {
			requirement def R { assume constraint { 1 < 2 } }
		}`)
		a, ok := members[0].(*ast.AssumeMember)
		if !ok {
			t.Fatalf("member = %T, want *ast.AssumeMember", members[0])
		}
		if a.Ident.Name != "" || !a.HasBody || len(a.Body) != 1 {
			t.Errorf("got name=%q hasBody=%v conditions=%d, want one anonymous condition",
				a.Ident.Name, a.HasBody, len(a.Body))
		}
	})

	t.Run("require_reads_the_same_declaration", func(t *testing.T) {
		members := reqBody(t, `package B {
			constraint def C;
			requirement def R { require constraint r1 : C; }
		}`)
		r, ok := members[0].(*ast.RequireMember)
		if !ok {
			t.Fatalf("member = %T, want *ast.RequireMember", members[0])
		}
		if r.Ident.Name != "r1" || len(r.Relationships) != 1 || r.HasBody {
			t.Errorf("got name=%q relationships=%v hasBody=%v, want r1 typed by C without a body",
				r.Ident.Name, r.Relationships, r.HasBody)
		}
	})
}

// A malformed assumed or required constraint must produce diagnostics, never a
// panic.
func TestF66AssumeConstraintNegative(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"named_no_terminator", "requirement def R { assume constraint c1 : C }"},
		{"typing_no_target", "requirement def R { assume constraint c1 : ; }"},
		{"named_unclosed_body", "requirement def R { assume constraint c1 { 1 < 2 }"},
		{"named_at_eof", "requirement def R { assume constraint c1"},
		{"require_typing_no_target", "requirement def R { require constraint r1 : ; }"},
		{"require_at_eof", "requirement def R { require constraint"},
		{"value_no_expression", "requirement def R { assume constraint c1 : C = ; }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := source.New("f66_neg.sysml", []byte(tt.input))
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
