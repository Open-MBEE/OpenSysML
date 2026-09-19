package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A reference written as an alias name reaches the aliased element: the alias
// declares a Membership whose memberElement is that element (KerML §8.2.3.2),
// so the name resolves to `test::A` and not to a second element `test::A_alias`.
func TestW6BReferenceThroughAliasReachesTheTarget(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.kerml": "package test { classifier A; alias A_alias for A; }",
	})
	r := New(idx)
	scope := scopeOf(t, idx.DocumentRoot("a.kerml"), "test")

	name := qn(false, "A_alias")
	sym, ok := r.ResolveQualified(scope, name)
	if !ok {
		t.Fatalf("A_alias unresolved; diags=%v", r.Diagnostics)
	}
	if sym.Kind == symbols.SymbolAlias {
		t.Fatalf("A_alias resolved to the alias itself, want the element it names")
	}
	if got := symbols.FQNOf(sym); got != "test::A" {
		t.Fatalf("resolved FQN = %q, want test::A", got)
	}
	// The segment as written is still readable: the name is the alias's.
	alias, ok := r.PartAlias(name, 0)
	if !ok || alias.Name != "A_alias" {
		t.Fatalf("PartAlias = %v, %v; want the A_alias membership", alias, ok)
	}
	if seg, ok := r.PartSymbol(name, 0); !ok || symbols.FQNOf(seg) != "test::A" {
		t.Fatalf("PartSymbol = %v, %v; want test::A", seg, ok)
	}
}

// Reaching the element does not remove the alias's name: it is still a member of
// its namespace, which is what completion and the member list enumerate.
func TestW6BAliasStaysAVisibleMember(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.kerml": "package test { classifier A; alias A_alias for A; }",
	})
	scope := scopeOf(t, idx.DocumentRoot("a.kerml"), "test")

	sym, ok := scope.LookupLocal("A_alias")
	if !ok || sym.Kind != symbols.SymbolAlias {
		t.Fatalf("A_alias is not a local member of test: %v, %v", sym, ok)
	}
	var found bool
	for _, m := range scope.Members() {
		if m.Name == "A_alias" {
			found = true
		}
	}
	if !found {
		t.Fatalf("A_alias missing from the member list of test")
	}
}

// A qualifying segment written as an alias reaches the target's members, and the
// member itself reports the target's qualified name.
func TestW6BAliasQualifiesItsTargetsMembers(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.kerml": "package test { classifier A { feature f; } alias A_alias for A; }",
	})
	r := New(idx)
	scope := scopeOf(t, idx.DocumentRoot("a.kerml"), "test")

	sym, ok := r.ResolveQualified(scope, qn(false, "A_alias", "f"))
	if !ok {
		t.Fatalf("A_alias::f unresolved; diags=%v", r.Diagnostics)
	}
	if got := symbols.FQNOf(sym); got != "test::A::f" {
		t.Fatalf("resolved FQN = %q, want test::A::f", got)
	}
}

// An alias of an alias reaches the element at the end of the chain, and a cyclic
// alias resolves to no element rather than recursing.
func TestW6BAliasChainAndCycle(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.kerml": "package test { classifier A; alias A1 for A; alias A2 for A1; " +
			"alias C1 for C2; alias C2 for C1; }",
	})
	r := New(idx)
	scope := scopeOf(t, idx.DocumentRoot("a.kerml"), "test")

	sym, ok := r.ResolveQualified(scope, qn(false, "A2"))
	if !ok || symbols.FQNOf(sym) != "test::A" {
		t.Fatalf("A2 resolved to %v (ok=%v), want test::A", sym, ok)
	}
	cyclic, ok := r.ResolveQualified(scope, qn(false, "C1"))
	if !ok || cyclic.Kind != symbols.SymbolAlias {
		t.Fatalf("cyclic alias C1 = %v (ok=%v), want the alias itself", cyclic, ok)
	}
}

// A membership import of an alias imports that name (KerML 8.2.3.2), and a
// reference to it still reaches the target element.
func TestW6BImportedAliasNameResolvesToTheTarget(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.kerml": "package defs { classifier Vehicle; alias Car for Vehicle; }",
		"b.kerml": "package uses { private import defs::Car; }",
	})
	r := New(idx)
	scope := scopeOf(t, idx.DocumentRoot("b.kerml"), "uses")

	sym, ok := r.ResolveQualified(scope, qn(false, "Car"))
	if !ok {
		t.Fatalf("imported alias name Car unresolved; diags=%v", r.Diagnostics)
	}
	if got := symbols.FQNOf(sym); got != "defs::Vehicle" {
		t.Fatalf("resolved FQN = %q, want defs::Vehicle", got)
	}
}
