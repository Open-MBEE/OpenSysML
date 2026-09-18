package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const requirementMemberShortNameModel = `package P {
	part def T { attribute v; }
	constraint def C;
	requirement def R {
		subject <s> x : T;
		assume constraint <a> ac : C;
		require constraint <r> rc : C;
		require constraint { s.v == s.v and a and r }
	}
	requirement def R2 :> R {
		subject :>> s;
		assume constraint :>> a;
		require constraint :>> r;
	}
	requirement def R3 :> R {
		subject <s2> y :>> R::s;
	}
	requirement def R4 :> R {
		subject <al> :>> x;
		require constraint { al.v == al.v }
	}
}`

// The short name of a subject, assume or require member resolves wherever the
// short name of a `part <p>` does: qualified from outside, as the target of a
// redefinition in a specializing requirement, and from an expression.
func TestRequirementMemberShortNamesResolve(t *testing.T) {
	r, root, rootScope := resolvedDoc(t, requirementMemberShortNameModel)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("resolution diagnostics: %v", r.Diagnostics)
	}
	pkg := local(t, rootScope, "P")
	req := local(t, pkg.Scope, "R")
	want := map[string]*symbols.Symbol{
		"s": local(t, req.Scope, "x"),
		"a": local(t, req.Scope, "ac"),
		"r": local(t, req.Scope, "rc"),
	}

	for short, sym := range want {
		got, ok := r.ResolveQualified(rootScope, qualified("P", "R", short))
		if !ok || got != sym {
			t.Errorf("P::R::%s = %v, %v; want %s", short, got, ok, sym.Name)
		}
	}

	// Every `s`, `a` and `r` written in the model — the redefinitions in R2 and
	// R3 and the operands of the anonymous constraint — names the R member.
	seen := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		if ref.QN == nil {
			continue
		}
		last := ref.QN.Parts[len(ref.QN.Parts)-1].Text
		sym, isShort := want[last]
		if !isShort {
			continue
		}
		got, ok := r.ResolveReference(ref)
		if !ok || got != sym {
			t.Errorf("reference %s at %d = %v, %v; want %s", last, ref.QN.Span().Offset, got, ok, sym.Name)
		}
		seen[last]++
	}
	if seen["s"] != 4 || seen["a"] != 2 || seen["r"] != 2 {
		t.Errorf("references seen = %v, want s:4 a:2 r:2", seen)
	}

	// A redefining subject takes both names of the subject it redefines (KerML
	// 7.3.4.5), so `R2::s` and `R2::x` reach it; a requirement constraint is
	// named only by a constraint it references, so a redefining one has no name.
	r2 := local(t, pkg.Scope, "R2")
	subject := local(t, r2.Scope, "s")
	if got, ok := r.ResolveQualified(rootScope, qualified("P", "R2", "x")); !ok || got != subject {
		t.Errorf("P::R2::x = %v, %v; want R2's subject", got, ok)
	}
	for _, name := range []string{"a", "r", "ac", "rc"} {
		if _, ok := r2.Scope.LookupLocal(name); ok {
			t.Errorf("R2 answers to %q; a redefining requirement constraint derives no name", name)
		}
		if _, ok := r.ResolveQualified(rootScope, qualified("P", "R2", name)); ok {
			t.Errorf("P::R2::%s resolves; want unresolved", name)
		}
	}
	if anon := r2.Scope.AnonymousMembers(); len(anon) != 2 {
		t.Errorf("R2 anonymous members = %d, want the two constraints", len(anon))
	}
	r3 := local(t, pkg.Scope, "R3")
	if local(t, r3.Scope, "s2") != local(t, r3.Scope, "y") {
		t.Error("R3's subject is not one symbol under both its names")
	}

	// A redefining subject with a short name of its own declares a name (KerML
	// 7.3.4.5), so it takes none from the redefined feature: R4's subject is a
	// member by `al` alone and its `:>> x` names R's subject.
	r4 := local(t, pkg.Scope, "R4")
	aliased := local(t, r4.Scope, "al")
	if _, ok := r4.Scope.LookupLocal("x"); ok {
		t.Error("R4 answers to `x`; a short-named redefinition derives no name")
	}
	if aliased.EffectiveName() || aliased.NamingTarget != nil || aliased.Name != "al" || aliased.ShortName != "al" {
		t.Errorf("R4's subject = %q <%s> effective=%v target=%v; want a member by <al> alone",
			aliased.Name, aliased.ShortName, aliased.EffectiveName(), aliased.NamingTarget)
	}
	inherited := local(t, req.Scope, "x")
	var redefines, reads int
	for _, ref := range resolve.References(root, rootScope) {
		if ref.QN == nil || !within(ref.Scope, r4.Scope) {
			continue
		}
		got, ok := r.ResolveReference(ref)
		switch last := ref.QN.Parts[len(ref.QN.Parts)-1].Text; {
		case ref.Redefines:
			redefines++
			if !ok || got != inherited {
				t.Errorf("R4 :>> %s = %v, %v; want R::x", last, got, ok)
			}
		case last == "al":
			reads++
			if !ok || got != aliased {
				t.Errorf("R4 condition %s = %v, %v; want R4's own subject", last, got, ok)
			}
		}
	}
	if redefines != 1 || reads != 2 {
		t.Errorf("R4 references: %d redefinition, %d reads; want 1 and 2", redefines, reads)
	}
}

// within reports whether scope is outer or nested in it.
func within(scope, outer *symbols.Scope) bool {
	for ; scope != nil; scope = scope.Parent() {
		if scope == outer {
			return true
		}
	}
	return false
}

func qualified(parts ...string) *ast.QualifiedName {
	q := &ast.QualifiedName{}
	for _, p := range parts {
		q.Parts = append(q.Parts, ast.NameSegment{Text: p})
	}
	return q
}
