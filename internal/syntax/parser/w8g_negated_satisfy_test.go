package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A satisfaction is negated wherever it is written: a body member's `not` is the
// SatisfyRequirementUsage's isNegated, not a prefix of its kind
// (SysML.xtext:2118).
func TestNegatedSatisfyInBodyKeepsNegation(t *testing.T) {
	sf := source.New("w8g.sysml", []byte(`package P {
		requirement def R1;
		part vehicle;
		requirement r1 : R1;
		not satisfy r1 by vehicle;
		part context {
			not satisfy r1 by vehicle;
			not satisfy requirement r2 : R1 by vehicle;
			satisfy r1 by vehicle;
		}
	}`))
	p := New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}

	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	top := pkg.Members[len(pkg.Members)-2].(*ast.Membership).Member.(*ast.Usage)
	if top.Kind != ast.UsageSatisfy || !top.IsNegated {
		t.Errorf("namespace member: kind=%v negated=%v, want a negated satisfy", top.Kind, top.IsNegated)
	}

	ctx := pkg.Members[len(pkg.Members)-1].(*ast.Membership).Member.(*ast.Usage)
	want := []bool{true, true, false}
	if len(ctx.Members) != len(want) {
		t.Fatalf("body members = %d, want %d", len(ctx.Members), len(want))
	}
	for i, negated := range want {
		u, ok := ctx.Members[i].(*ast.Membership).Member.(*ast.Usage)
		if !ok {
			t.Fatalf("body member %d = %T, want *ast.Usage", i, ctx.Members[i])
		}
		if u.Kind != ast.UsageSatisfy || u.IsNegated != negated {
			t.Errorf("body member %d: kind=%v negated=%v, want satisfy with negated=%v", i, u.Kind, u.IsNegated, negated)
		}
	}
}
