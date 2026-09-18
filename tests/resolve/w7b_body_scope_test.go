package resolve_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
)

// bodyPrelude stands in for the library types and collection functions these
// models name, so the tests need no stdlib loaded.
const bodyPrelude = `package ScalarValues {
	attribute def Integer;
}
package ControlFunctions {
	calc def collect { in source : ScalarValues::Integer[*]; in body; }
	calc def select { in source : ScalarValues::Integer[*]; in body; }
}
`

// bodyScopeModel declares locals inside expression bodies, references them from
// the bodies' results, and never from outside them.
const bodyScopeModel = bodyPrelude + `package Lib {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	calc def Doubles {
		in xs : Integer[*];
		return ys : Integer[*] = xs->collect { in i; attribute k : Integer = i * 2; k };
	}
	calc def Big {
		in xs : Integer[*];
		return ys : Integer[*] = xs->select { in i; private attribute limit : Integer = 2; i > limit };
	}
}`

// A declaration inside an expression body is a member of that body's scope, so a
// reference to it from the body resolves, and both the declaration's own value
// and the reference are collected for the editor (F64).
func TestW7BBodyDeclarationsResolveAndAreCollected(t *testing.T) {
	walk, root, rootScope := resolvedDoc(t, bodyScopeModel)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("every name in a body must resolve: %v", walk.Diagnostics)
	}
	byName := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		byName[nameText(ref.QN)]++
	}
	// `k` and `limit` are each named once, from their own body's result.
	for _, name := range []string{"k", "limit"} {
		if byName[name] != 1 {
			t.Errorf("`%s` is referenced %d time(s), want 1", name, byName[name])
		}
	}
	// A body-local declaration's own value names the body's parameter.
	if byName["i"] != 2 {
		t.Errorf("`i` is referenced %d time(s), want 2 (one per body)", byName["i"])
	}
	query, _, _ := resolvedDoc(t, bodyScopeModel)
	for _, ref := range resolve.References(root, rootScope) {
		if _, ok := query.ResolveReference(ref); !ok {
			t.Errorf("%s does not resolve on its own", nameText(ref.QN))
		}
	}
}

// A body's declarations are visible to the body alone: a name from one body is
// not in scope in a sibling body, nor in the enclosing definition.
func TestW7BBodyDeclarationsDoNotEscapeToConsumers(t *testing.T) {
	src := bodyPrelude + `package Lib {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	calc def Leak {
		in xs : Integer[*];
		attribute ys = xs->collect { in i; attribute k : Integer = i; k };
		attribute outside : Integer = k;
	}
}`
	walk, _, _ := resolvedDoc(t, src)
	var found bool
	for _, d := range walk.Diagnostics {
		if strings.Contains(d.Message, "unresolved reference: k") {
			found = true
		}
	}
	if !found {
		t.Errorf("a body-local declaration must not be visible outside its body: %v", walk.Diagnostics)
	}
}
