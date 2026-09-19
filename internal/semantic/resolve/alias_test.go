package resolve

import (
	"testing"
)

func TestAliasResolvesTarget(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { namespace Real; alias A for P::Real; }",
	})
	r := New(idx)
	pScope := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")
	aSym, ok := pScope.LookupLocal("A")
	if !ok {
		t.Fatalf("alias A not found")
	}
	target, ok := r.ResolveAliasTarget(aSym)
	if !ok {
		t.Fatalf("alias A target unresolved; diags=%v", r.Diagnostics)
	}
	if target.Name != "Real" {
		t.Fatalf("alias target = %q, want Real", target.Name)
	}
}

func TestAliasTransitive(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { namespace Real; alias A for P::Real; alias B for P::A; }",
	})
	r := New(idx)
	pScope := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")
	bSym, _ := pScope.LookupLocal("B")
	target, ok := r.ResolveAliasTarget(bSym)
	if !ok {
		t.Fatalf("transitive alias B unresolved; diags=%v", r.Diagnostics)
	}
	if target.Name != "Real" {
		t.Fatalf("transitive target = %q, want Real", target.Name)
	}
}

func TestAliasCycleGuard(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { alias A for P::B; alias B for P::A; }",
	})
	r := New(idx)
	pScope := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")
	aSym, _ := pScope.LookupLocal("A")
	if _, ok := r.ResolveAliasTarget(aSym); ok {
		t.Fatalf("cyclic alias should not resolve")
	}
}

// An alias declared in a namespace reaches a target that namespace holds only
// through a private wildcard import, which the stdlib does throughout:
// ISQThermodynamics aliases TemperatureValue for a ThermodynamicTemperatureValue
// it sees only via `private import ISQBase::*`. A qualified reference from
// anywhere else must not reach that name (KerML 8.2.3.3), so the alias resolves
// its target *from* its own namespace.
func TestAliasResolvesAPrivatelyImportedTargetInALibrary(t *testing.T) {
	idx := libraryIndexOf(t, map[string]string{
		"base.sysml": "package Base { part def Hidden; }",
		"mid.sysml":  "package Mid { private import Base::*; alias HiddenAlias for Hidden; }",
	})
	idx.ExpandWildcardImports()

	aliases := idx.LookupQualified("Mid::HiddenAlias")
	if len(aliases) != 1 {
		t.Fatalf("LookupQualified(Mid::HiddenAlias) len = %d, want 1", len(aliases))
	}
	target, ok := New(idx).ResolveAliasTarget(aliases[0])
	if !ok {
		t.Fatalf("alias of a privately imported type unresolved")
	}
	if target.Name != "Hidden" {
		t.Fatalf("alias target = %q, want Hidden", target.Name)
	}
}

// The same for an alias in a parsed document, which reaches its target through
// the private import declared in its own scope.
func TestAliasResolvesAPrivatelyImportedTargetWhenParsed(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"base.sysml": "package Base { part def Hidden; }",
		"mid.sysml":  "package Mid { private import Base::*; alias H for Hidden; }",
	})
	idx.ExpandWildcardImports()
	r := New(idx)

	mid := scopeOf(t, idx.DocumentRoot("mid.sysml"), "Mid")
	hSym, ok := mid.LookupLocal("H")
	if !ok {
		t.Fatalf("alias H not found in Mid")
	}
	target, ok := r.ResolveAliasTarget(hSym)
	if !ok {
		t.Fatalf("alias H unresolved; diags=%v", r.Diagnostics)
	}
	if target.Name != "Hidden" {
		t.Fatalf("alias target = %q, want Hidden", target.Name)
	}
}

func TestResolveAliasTargetNonAlias(t *testing.T) {
	idx := indexOf(t, map[string]string{"a.sysml": "package P { namespace Real; }"})
	r := New(idx)
	pScope := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")
	realSym, _ := pScope.LookupLocal("Real")
	target, ok := r.ResolveAliasTarget(realSym)
	if !ok || target != realSym {
		t.Fatalf("non-alias symbol should resolve to itself")
	}
}
