package symbols

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// findBodyScopes returns every scope in the tree owned by a body expression.
func findBodyScopes(scope *Scope) []*Scope {
	var out []*Scope
	if _, ok := scope.Node().(*ast.BodyExpr); ok {
		out = append(out, scope)
	}
	for _, ch := range scope.Children() {
		out = append(out, findBodyScopes(ch)...)
	}
	return out
}

// Build links a body expression's parameters into the document scope tree, so
// every consumer — resolution, the type checker and the editor — sees the same
// scope rather than one built on the side.
func TestBuildLinksBodyExpressionScopes(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	action def Sample {
		in attribute samples : Real[*];
		assert constraint { samples->forAll { in s : Real; s > 0 } }
	}
}`
	root := build(t, src)

	bodies := findBodyScopes(root)
	if len(bodies) != 1 {
		t.Fatalf("body-expression scopes = %d, want 1", len(bodies))
	}
	sym, ok := bodies[0].LookupLocal("s")
	if !ok {
		t.Fatalf("parameter s not defined in the body scope")
	}
	// The parameter's own identifier, not the body's braces: the editor renames
	// through NameSpan and jumps to DeclSpan.
	if want := strings.Index(src, "s : Real; s > 0"); sym.NameSpan.Offset != want || sym.DeclSpan.Offset != want {
		t.Errorf("spans at %d/%d, want the parameter name at %d", sym.NameSpan.Offset, sym.DeclSpan.Offset, want)
	}
	if BodyExprScope(bodies[0].Parent(), bodies[0].Node().(*ast.BodyExpr)) != bodies[0] {
		t.Error("BodyExprScope did not return the linked scope")
	}
}

// Nested bodies nest their scopes, so an inner parameter shadows an outer one.
func TestBuildNestsBodyExpressionScopes(t *testing.T) {
	root := build(t, `package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	action def Sample {
		in attribute samples : Real[*];
		assert constraint { samples->forAll { in s : Real; samples->exists { in s : Real; s > 0 } } }
	}
}`)

	bodies := findBodyScopes(root)
	if len(bodies) != 2 {
		t.Fatalf("body-expression scopes = %d, want 2", len(bodies))
	}
	if bodies[1].Parent() != bodies[0] {
		t.Fatalf("inner body scope parent = %v, want the outer body scope", bodies[1].Parent().Node())
	}
	outer, _ := bodies[0].LookupLocal("s")
	inner, _ := bodies[1].LookupLocal("s")
	if outer == nil || inner == nil || outer == inner {
		t.Errorf("nested parameters share a symbol: outer=%v inner=%v", outer, inner)
	}
}

// A body expression without parameters declares nothing, so it owns no scope
// and its result resolves in the enclosing scope.
func TestBodyExprScopeWithoutParameters(t *testing.T) {
	root := build(t, `package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	action def Sample {
		in attribute samples : Real[*];
		assert constraint { samples->forAll { samples > 0 } }
	}
}`)

	if bodies := findBodyScopes(root); len(bodies) != 0 {
		t.Fatalf("body-expression scopes = %d, want 0", len(bodies))
	}
}

// An unnamed transition with a body is an anonymous member of its state that
// owns the body's scope, as a named one does; one declaring nothing is no member.
func TestUnnamedTransitionWithABodyIsAnAnonymousMember(t *testing.T) {
	root := build(t, `package P {
	state def S {
		state a; state b;
		transition first a then b { attribute count; }
		transition first b then a;
	}
}`)
	p, _ := root.LookupLocal("P")
	s, _ := p.Scope.LookupLocal("S")
	anonymous := s.Scope.AnonymousMembers()
	if len(anonymous) != 1 {
		t.Fatalf("anonymous members of S = %v, want the transition with a body", anonymous)
	}
	trans := anonymous[0]
	if _, ok := trans.Decl.(*ast.TransitionMember); !ok || trans.Kind != SymbolActionUsage {
		t.Fatalf("anonymous member is %v (%T), want a transition action usage", trans.Kind, trans.Decl)
	}
	if trans.Scope == nil || trans.Scope.Owner() != trans || trans.Scope.BodyLocal() {
		t.Fatalf("the transition does not own its body's scope as a feature: %+v", trans.Scope)
	}
	if count, ok := trans.Scope.LookupLocal("count"); !ok || count.OwnerScope != trans.Scope {
		t.Errorf("count is not a member of the transition")
	}
	if _, ok := s.Scope.LookupLocal("count"); ok {
		t.Errorf("the body's count escaped into the state")
	}
}

// An effect's members are the transition's own features (SysML v2 §7.19.2),
// whether it is an action or a `do send` with parameters; the trigger's are not.
func TestTriggeredTransitionOwnsItsEffectMembers(t *testing.T) {
	root := build(t, `package P {
	item def Warning;
	state S {
		port line;
		state a; state b;
		transition relay first a accept w : Warning
			do send w via line { in payload = w; } then b;
		transition alert first a accept v : Warning
			do action raise { in cause = v; } then b;
	}
}`)
	p, _ := root.LookupLocal("P")
	s, _ := p.Scope.LookupLocal("S")
	for _, tc := range []struct{ transition, member, param string }{
		{"relay", "payload", "w"},
		{"alert", "raise", "v"},
	} {
		trans, ok := s.Scope.LookupLocal(tc.transition)
		if !ok {
			t.Fatalf("transition %s not defined", tc.transition)
		}
		member, ok := trans.Scope.LookupLocal(tc.member)
		if !ok {
			t.Errorf("%s.%s is not a member of the transition", tc.transition, tc.member)
		} else if member.OwnerScope != trans.Scope {
			t.Errorf("%s.%s is owned by %v, want the transition", tc.transition, tc.member, member.OwnerScope.Node())
		}
		if _, ok := trans.Scope.LookupLocal(tc.param); ok {
			t.Errorf("trigger parameter %s is a member of transition %s", tc.param, tc.transition)
		}
	}
}
