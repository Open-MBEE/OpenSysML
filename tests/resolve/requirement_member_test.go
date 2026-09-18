package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The constraint usage an assume or require member owns is looked up by name
// from its requirement like any usage, and `:>>` on it resolves to the member
// of the requirement's general.
func TestRequirementConstraintMembersResolve(t *testing.T) {
	r, _, root := resolvedDocNamed(t, "req-members.sysml", `package P {
		constraint def C;
		part def Vehicle;
		requirement def R {
			subject v : Vehicle;
			assume constraint a : C;
			require constraint q : C;
		}
		requirement def S :> R {
			subject v2 :>> v;
			assume constraint a2 :>> a;
			require constraint q2 :>> q;
		}
	}`)
	for _, d := range r.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", d.Message)
	}

	pkg, _ := root.LookupLocal("P")
	req, _ := pkg.Scope.LookupLocal("R")
	spec, _ := pkg.Scope.LookupLocal("S")
	for _, tc := range []struct{ name, redefined string }{{"v2", "v"}, {"a2", "a"}, {"q2", "q"}} {
		sym, ok := spec.Scope.LookupLocal(tc.name)
		if !ok {
			t.Fatalf("S members = %v, want %s", spec.Scope.MemberNames(), tc.name)
		}
		want, _ := req.Scope.LookupLocal(tc.redefined)
		var rels []*ast.Relationship
		switch d := sym.Decl.(type) {
		case *ast.SubjectMember:
			rels = d.Relationships
		default:
			c, _ := ast.OwnedConstraintOf(d)
			rels = c.Relationships
		}
		if len(rels) != 1 || rels[0].Kind != ast.RelRedefines {
			t.Fatalf("%s relationships = %v, want one redefinition", tc.name, rels)
		}
		got, ok := r.ResolveTarget(sym.OwnerScope, rels[0].Target)
		if !ok || got != want {
			t.Errorf("%s :>> resolves to %v, want %s", tc.name, got, symbols.FQNOf(want))
		}
	}
}

// A redefining require constraint's body sees what is nested, at any depth, in
// the constraint it redefines (KerML 8.3.4.4), as a redefining usage's body does.
func TestRequirementConstraintBodySeesRedefinedNesting(t *testing.T) {
	r, _, _ := resolvedDocNamed(t, "req-nesting.sysml", `package P {
		requirement def R {
			require constraint q { attribute bounds { attribute limit; } true }
		}
		requirement def S :> R {
			require constraint q2 :>> q { attribute cap :> limit; true }
		}
	}`)
	for _, d := range r.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", d.Message)
	}
}
