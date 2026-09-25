package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// requirementMembers parses src and returns the members of the requirement
// definition it declares last.
func requirementMembers(t *testing.T, src string) []ast.Node {
	t.Helper()
	sf := source.New("w8g.sysml", []byte(src))
	p := New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	def := pkg.Members[len(pkg.Members)-1].(*ast.Membership).Member.(*ast.Definition)
	return def.Members
}

// A requirement constraint member may reference a requirement instead of
// stating a condition: `OwnedReferenceSubsetting FeatureSpecialization*
// CalculationBody` (SysML.xtext RequirementConstraintUsage), so the body may be
// `;` and specializations may be written before it.
func TestConstraintReferenceMemberForms(t *testing.T) {
	members := requirementMembers(t, `package Q {
		requirement def C;
		requirement a : C;
		requirement b : C;
		requirement c : C;
		requirement d : C;
		requirement def R {
			require Q::a;
			assume Q::b;
			require Q::c : C;
			require Q::d { require constraint { 1 < 2 } }
		}
	}`)
	if len(members) != 4 {
		t.Fatalf("members = %d, want 4", len(members))
	}

	first, ok := members[0].(*ast.RequireMember)
	if !ok {
		t.Fatalf("member 0 = %T, want *ast.RequireMember", members[0])
	}
	if first.Reference == nil || len(first.Reference.Parts) != 2 || first.Reference.Parts[1].Text != "a" {
		t.Errorf("member 0 reference = %v, want Q::a", first.Reference)
	}
	if first.Expression != nil || first.HasBody {
		t.Errorf("member 0: expression=%v hasBody=%v, want a body-less reference", first.Expression, first.HasBody)
	}

	second, ok := members[1].(*ast.AssumeMember)
	if !ok {
		t.Fatalf("member 1 = %T, want *ast.AssumeMember", members[1])
	}
	if second.Reference == nil || second.Expression != nil {
		t.Errorf("member 1: reference=%v expression=%v, want a reference", second.Reference, second.Expression)
	}

	third, ok := members[2].(*ast.RequireMember)
	if !ok {
		t.Fatalf("member 2 = %T, want *ast.RequireMember", members[2])
	}
	if third.Reference == nil || len(third.Relationships) != 1 {
		t.Fatalf("member 2: reference=%v relationships=%d, want a reference with one specialization",
			third.Reference, len(third.Relationships))
	}
	if kind := third.Relationships[0].Kind; kind != ast.RelTyping {
		t.Errorf("member 2 specialization kind = %v, want %v", kind, ast.RelTyping)
	}

	fourth, ok := members[3].(*ast.RequireMember)
	if !ok {
		t.Fatalf("member 3 = %T, want *ast.RequireMember", members[3])
	}
	if fourth.Reference == nil || !fourth.HasBody || len(fourth.Body) != 1 {
		t.Errorf("member 3: reference=%v hasBody=%v body=%d, want a reference with a one-condition body",
			fourth.Reference, fourth.HasBody, len(fourth.Body))
	}
}

// A body-less bare name stays the condition expression it has always been: the
// expression forms this member also accepts need it to be one.
func TestConstraintReferenceBareNameStaysExpression(t *testing.T) {
	members := requirementMembers(t, `package Q {
		requirement def R { require massLimit; }
	}`)
	if len(members) != 1 {
		t.Fatalf("members = %d, want 1", len(members))
	}
	m, ok := members[0].(*ast.RequireMember)
	if !ok {
		t.Fatalf("member = %T, want *ast.RequireMember", members[0])
	}
	if m.Expression == nil || m.Reference != nil {
		t.Errorf("expression=%v reference=%v, want a condition expression", m.Expression, m.Reference)
	}
}
