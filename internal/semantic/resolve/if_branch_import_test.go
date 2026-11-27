package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// branchScopes returns the scopes of the if branches written directly in scope, in source order.
func branchScopes(scope *symbols.Scope) []*symbols.Scope {
	var out []*symbols.Scope
	for _, child := range scope.Children() {
		if _, ok := child.Node().(*ast.IfBranchNode); ok {
			out = append(out, child)
		}
	}
	return out
}

// An import in the body of an if branch brings its names into that branch, and only that one
// (SysML 8.2.2.17.7 ActionBodyParameter, KerML 8.2.3.5.4).
func TestIfBranchBodyImportResolvesInItsBranch(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want []string // per branch, the FQN `T` resolves to; "" where it is unresolved
	}{
		"then, wildcard":   {"if n < 3 { private import Q::*; part v : T; }", []string{"Q::T"}},
		"then, public":     {"if n < 3 { public import Q::*; part v : T; }", []string{"Q::T"}},
		"then, membership": {"if n < 3 { private import Q::T; part v : T; }", []string{"Q::T"}},
		"else":             {"if n < 3 { part v : Q::T; } else { private import Q::*; part v : T; }", []string{"", "Q::T"}},
		"else if": {
			"if n < 3 { part v : Q::T; } else if n < 5 { private import Q::*; part v : T; } else { part v : Q::T; }",
			[]string{"", "", "Q::T", ""}, // the else branch holding the second if, then that if's two
		},
	} {
		t.Run(name, func(t *testing.T) {
			idx := indexOf(t, map[string]string{"a.sysml": `package Q { part def T; }
package P {
    action def A {
        attribute n : ScalarValues::Integer;
        ` + tc.body + `
    }
}`})
			r := New(idx)
			a := scopeOf(t, scopeOf(t, idx.DocumentRoot("a.sysml"), "P"), "A")
			var branches []*symbols.Scope
			for next := branchScopes(a); len(next) != 0; next = branchScopes(next[len(next)-1]) {
				branches = append(branches, next...)
			}
			if len(branches) != len(tc.want) {
				t.Fatalf("found %d branch scopes, want %d", len(branches), len(tc.want))
			}
			for i, branch := range branches {
				sym, ok := r.LookupName(branch, "T")
				got := ""
				if ok {
					got = idx.GetFQN(sym)
				}
				if got != tc.want[i] {
					t.Errorf("branch %d: T resolves to %q, want %q", i, got, tc.want[i])
				}
			}
		})
	}
}

// A name a branch body imports is found before the same name in an enclosing namespace, as in
// a plain action body, and a call in the branch is matched against what the branch imports.
func TestIfBranchBodyImportShadowsOuterName(t *testing.T) {
	idx := indexOf(t, map[string]string{"a.sysml": `package Q { part def T; calc def f { in x : ScalarValues::Integer; } }
package P {
    part def T;
    calc def f { in x : ScalarValues::Integer; }
    action def A {
        attribute n : ScalarValues::Integer;
        if n < 3 { private import Q::*; part v : T; attribute a = f(1); }
    }
}`})
	r := New(idx)
	a := scopeOf(t, scopeOf(t, idx.DocumentRoot("a.sysml"), "P"), "A")
	branches := branchScopes(a)
	if len(branches) != 1 {
		t.Fatalf("found %d branch scopes, want 1", len(branches))
	}
	branch := branches[0]
	if sym, ok := r.LookupName(branch, "T"); !ok || idx.GetFQN(sym) != "Q::T" {
		t.Errorf("T in the branch resolves to %v, want Q::T", sym)
	}
	if sym, ok := r.LookupName(a, "T"); !ok || idx.GetFQN(sym) != "P::T" {
		t.Errorf("T outside the branch resolves to %v, want P::T", sym)
	}
	cands := r.InvocationCandidates(branch, qn(false, "f"))
	if got := fqnsOf(idx, cands); len(got) != 1 || got[0] != "Q::f" {
		t.Errorf("f in the branch has candidates %v, want Q::f", got)
	}
}

// A branch body's import reaches the namespaces nested in the branch.
func TestIfBranchBodyImportReachesNestedNamespace(t *testing.T) {
	idx := indexOf(t, map[string]string{"a.sysml": `package Q { part def T; }
package P {
    action def A {
        attribute n : ScalarValues::Integer;
        if n < 3 { private import Q::*; action inner { part w : T; } }
    }
}`})
	r := New(idx)
	branches := branchScopes(scopeOf(t, scopeOf(t, idx.DocumentRoot("a.sysml"), "P"), "A"))
	if len(branches) != 1 {
		t.Fatalf("found %d branch scopes, want 1", len(branches))
	}
	if sym, ok := r.LookupName(scopeOf(t, branches[0], "inner"), "T"); !ok || idx.GetFQN(sym) != "Q::T" {
		t.Errorf("T in an action nested in the branch resolves to %v, want Q::T", sym)
	}
}
