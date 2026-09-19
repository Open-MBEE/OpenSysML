package resolve

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// resolveVisibilityDoc resolves src with the specialization edges a live
// workspace supplies, so inherited and imported routes behave as they do there.
func resolveVisibilityDoc(t *testing.T, src string) *Resolver {
	t.Helper()
	root := parsedRoot(t, "d.kerml", src)
	idx := symbols.NewIndex()
	idx.AddDocument("d.kerml", root)
	idx.ExpandWildcardImports()
	r := resolverWithSpecializations(idx)
	r.ResolveDocument("d.kerml", root)
	return r
}

func diagTexts(r *Resolver) string {
	var b strings.Builder
	for _, d := range r.Diagnostics {
		b.WriteString(d.Message)
		b.WriteString("; ")
	}
	return b.String()
}

func wantUnresolved(t *testing.T, r *Resolver, refs ...string) {
	t.Helper()
	got := diagTexts(r)
	for _, ref := range refs {
		if !strings.Contains(got, "unresolved reference: "+ref) {
			t.Errorf("expected %q to be unresolved; diagnostics: %s", ref, got)
		}
	}
}

func wantNoUnresolved(t *testing.T, r *Resolver, refs ...string) {
	t.Helper()
	got := diagTexts(r)
	for _, ref := range refs {
		if strings.Contains(got, "unresolved reference: "+ref) {
			t.Errorf("expected %q to resolve; diagnostics: %s", ref, got)
		}
	}
}

// wantUnresolvedMember asserts the named links of a feature chain are the ones
// resolution refuses to walk.
func wantUnresolvedMember(t *testing.T, r *Resolver, names ...string) {
	t.Helper()
	got := diagTexts(r)
	for _, name := range names {
		if !strings.Contains(got, "unresolved member: "+name) {
			t.Errorf("expected member %q to be unreachable; diagnostics: %s", name, got)
		}
	}
}

// A private member is not visible outside the namespace that declares it, so a
// qualified name crossing that boundary does not resolve (KerML 8.2.3.5).
func TestPrivateMemberIsHiddenFromAQualifiedNameCrossingItsNamespace(t *testing.T) {
	r := resolveVisibilityDoc(t, `package P {
		package A { private class X; public class W; }
		class Y specializes A::X;
		class Z specializes A::W;
	}`)
	wantUnresolved(t, r, "A::X")
	wantNoUnresolved(t, r, "A::W")
}

// A protected member is visible to what specializes its namespace, and to that
// namespace itself, but not through a qualified name from outside.
func TestProtectedMemberIsVisibleThroughSpecializationOnly(t *testing.T) {
	r := resolveVisibilityDoc(t, `package P {
		class Base { protected class Inner; }
		class Sub specializes Base { class Use specializes Inner; }
		class Outside specializes Base::Inner;
	}`)
	wantNoUnresolved(t, r, "Inner")
	wantUnresolved(t, r, "Base::Inner")
}

// Qualified resolution uses public visibility; specialization does not admit protected tail segments.
// Simple-name resolution still reaches inherited protected members through resolveLocal.
func TestProtectedMemberRequiresSimpleNameFromSpecialization(t *testing.T) {
	r := resolveVisibilityDoc(t, `package P {
		class Base {
			protected class Inner { public class Leaf; }
			private class Hidden;
		}
		class Sub specializes Base {
			class Use specializes Inner;
			class A specializes Sub::Inner;
			class B specializes P::Sub::Inner::Leaf;
			class C specializes Base::Inner;
			class D specializes Base::Inner::Leaf;
		}
	}`)
	wantNoUnresolved(t, r, "Inner")
	wantUnresolved(t, r, "Sub::Inner", "P::Sub::Inner::Leaf", "Base::Inner", "Base::Inner::Leaf")

	outside := resolveVisibilityDoc(t, `package P {
		class Base {
			protected class Inner;
			private class Hidden;
		}
		class Sub specializes Base;
		class Outside specializes Sub::Inner;
		class PrivateOutside specializes Base::Hidden;
	}`)
	wantUnresolved(t, outside, "Sub::Inner", "Base::Hidden")
}

// Visibility applies to the members a feature chain walks: only the public link
// of the chain is reachable from outside the owning feature.
func TestFeatureChainAppliesVisibilityToEachLink(t *testing.T) {
	r := resolveVisibilityDoc(t, `package P {
		feature x { feature a; protected feature b; private feature c; }
		feature p1 redefines x.a;
		feature p2 redefines x.b;
		feature p3 redefines x.c;
	}`)
	wantUnresolvedMember(t, r, "b", "c")
}

// An alias carries the visibility of what it names, so aliasing a private
// member does not widen it. Whether the dangling alias is then itself an error
// where it is used is a validation question, not a resolution one.
func TestImportAsAliasDoesNotWidenVisibility(t *testing.T) {
	r := resolveVisibilityDoc(t, `package P {
		package A { private class Hidden; public class Shown; }
		alias H for A::Hidden;
		alias S for A::Shown;
		class U specializes H;
		class V specializes S;
	}`)
	wantUnresolved(t, r, "A::Hidden")
	wantNoUnresolved(t, r, "A::Shown", "S")
}

// `import all` takes the private memberships its target declares, but not one
// the target itself only holds through an import of its own (pilot
// imports/local/Import_All.kerml.xt).
func TestImportAllTakesTargetPrivatesNotHiddenReexports(t *testing.T) {
	root := parsedRoot(t, "d.kerml", `package Import_All {
		package A { private classifier X; }
		package B { public import A::*; private classifier Y1; public classifier Y2; }
		package C { private import B::*; private classifier Z; private package C1 { public classifier U; } }
		package D {
			public import all C::*;
			classifier ux specializes X;
			classifier uy1 specializes Y1;
			classifier uy2 specializes Y2;
			classifier uz specializes Z;
		}
		package E { public import all C::**; classifier uu specializes C1::U; }
	}`)
	idx := symbols.NewIndex()
	idx.AddDocument("d.kerml", root)
	idx.ExpandWildcardImports()
	r := resolverWithSpecializations(idx)
	r.ResolveDocument("d.kerml", root)
	wantUnresolved(t, r, "X", "Y1")
	wantNoUnresolved(t, r, "Y2", "Z", "C1::U")
}

// A qualified name whose segments mix '::' and '.' resolves through public
// links and stops at a hidden one (pilot ParsingTests_ScopeWithFourDotAndDot).
func TestFourDotAndDotPathAppliesVisibility(t *testing.T) {
	r := resolveVisibilityDoc(t, `package OuterPackage {
		feature B { feature b; protected feature hidden; }
	}
	package Use {
		feature c redefines OuterPackage::B.b;
		feature d redefines OuterPackage::B.hidden;
	}`)
	wantNoUnresolved(t, r, "OuterPackage::B.b", "OuterPackage::B")
	wantUnresolvedMember(t, r, "hidden")
}

// VisibleNames offers exactly what resolution admits, so the LSP surface and
// the resolver share one rule.
func TestPrivateMemberIsNotOfferedOrResolved(t *testing.T) {
	idx := indexOf(t, map[string]string{"d.kerml": `package P {
		package A { private class X; public class W; }
		class Use specializes A::W;
	}`})
	r := resolverWithSpecializations(idx)
	a := idx.Declaring("P::A")
	if a == nil {
		t.Fatal("P::A not indexed")
	}
	p := scopeOf(t, idx.DocumentRoot("d.kerml"), "P")
	offered := r.AdmittedChildrenOf(p, "P::A", idx.LookupDirectChildren("P::A"))
	for _, sym := range offered {
		if strings.HasSuffix(sym.Name, "X") {
			t.Errorf("private X offered as a visible name of P::A: %v", offered)
		}
	}
	if len(offered) == 0 {
		t.Fatal("nothing offered under P::A; the test proves nothing")
	}
}

// An `import all` lifts the boundary only for its own target: a plain import
// consulted while that target resolves still hides what it must, and nothing
// resolved under the lifted boundary is memoized for a later reference.
func TestImportAllDoesNotLeakThroughASiblingImport(t *testing.T) {
	r := resolveVisibilityDoc(t, `package P {
		package Vault { private class Secret; public class Open; }
		package Outer { package Deep { package Inner { public class Thing; } } }
		package Use {
			import all Deep::Inner::*;
			import Outer::*;
			import Vault::Secret;
			class A specializes Secret;
			class B specializes Thing;
		}
		class C specializes Vault::Secret;
	}`)
	wantUnresolved(t, r, "Vault::Secret", "Secret")
	wantNoUnresolved(t, r, "Thing")
}

// A recursive import cannot descend through a namespace it cannot surface, and
// enumeration must offer exactly what that lookup reaches.
func TestRecursiveImportOffersOnlyWhatItReaches(t *testing.T) {
	src := `package P {
		private package Hidden { public class Buried; }
		package Shown { public class Seen; }
	}
	package Use {
		import P::**;
		class A specializes Buried;
		class B specializes Seen;
	}`
	root := parsedRoot(t, "d.kerml", src)
	idx := symbols.NewIndex()
	idx.AddDocument("d.kerml", root)
	idx.ExpandWildcardImports()
	r := resolverWithSpecializations(idx)
	r.ResolveDocument("d.kerml", root)
	wantUnresolved(t, r, "Buried")
	wantNoUnresolved(t, r, "Seen")

	use := idx.LookupQualified("Use")
	if len(use) != 1 || use[0].Scope == nil {
		t.Fatalf("Use names %d symbols with a scope", len(use))
	}
	offered := map[string]bool{}
	for _, imp := range importsIn(t, use[0].Scope) {
		for _, sym := range r.ImportedElements(use[0].Scope, imp) {
			offered[sym.Name] = true
		}
	}
	if offered["Buried"] {
		t.Error("the recursive import offers Buried, which resolution refuses")
	}
	if !offered["Seen"] {
		t.Error("the recursive import does not offer Seen, which resolution reaches")
	}
}

// importsIn are the import declarations written in scope's own namespace.
func importsIn(t *testing.T, scope *symbols.Scope) []*ast.Import {
	t.Helper()
	pkg, ok := scope.Node().(*ast.Package)
	if !ok {
		t.Fatalf("the scope's node is %T, want a package", scope.Node())
	}
	var out []*ast.Import
	for _, member := range pkg.Members {
		if imp, isImport := member.(*ast.Import); isImport {
			out = append(out, imp)
		}
	}
	return out
}
