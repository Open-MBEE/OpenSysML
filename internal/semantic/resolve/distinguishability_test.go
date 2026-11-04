package resolve

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Two owned members of one name are each reported as a warning, not an error:
// the pilot's validateNamespaceDistinguishability warns at both declarations
// and the model still resolves (KerML 7.2.2, SysML 7.6.1).
func TestDuplicateOwnedMemberNamesAreWarnings(t *testing.T) {
	const src = "package P { part def Dup; part def Dup; }"
	r := resolveDoc(t, "d.sysml", src)
	if len(r.Diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %v", len(r.Diagnostics), r.Diagnostics)
	}
	for _, d := range r.Diagnostics {
		if !d.Warning {
			t.Errorf("%q reported as an error, want a warning", d.Message)
		}
		if d.Code != CodeNameConflict || d.Message != "Duplicate of other owned member name" {
			t.Errorf("got %s %q, want %s %q", d.Code, d.Message, CodeNameConflict, "Duplicate of other owned member name")
		}
		if got := src[d.Span.Offset:d.Span.End()]; got != "Dup" {
			t.Errorf("diagnostic sits on %q, want the repeated name", got)
		}
	}
}

// Without a semantic model no metaclass is known, so two members of one name
// are never distinguishable by metaclass (KerML 8.3.2.4.3) and both warn.
func TestDuplicateOwnedMemberNamesNoModel(t *testing.T) {
	const src = "package P { part def A; attribute def A; }"
	r := resolveDoc(t, "d.sysml", src)
	if len(r.Diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %v", len(r.Diagnostics), r.Diagnostics)
	}
	for _, d := range r.Diagnostics {
		if d.Code != CodeNameConflict || d.Message != "Duplicate of other owned member name" {
			t.Errorf("got %s %q, want %s %q", d.Code, d.Message, CodeNameConflict, "Duplicate of other owned member name")
		}
	}
}

// hiddenIn asserts that name is no member of the namespace ns declares — the
// imported memberships binding it collide and KerML 7.2.5.4 hides them — and
// that the resolver reports nothing for the hiding, which is no violation.
func hiddenIn(t *testing.T, r *Resolver, ns, name string) {
	t.Helper()
	if sym, ok := r.LookupName(r.idx.Declaring(ns).Scope, name); ok {
		t.Errorf("%s resolves to %v in %s, want nothing: the imported memberships are hidden", name, sym, ns)
	}
	for _, d := range r.Diagnostics {
		if d.Code == CodeNameConflict && d.Warning {
			t.Errorf("got %q, want no name-conflict warning: hiding is a derivation, not a violation", d.Message)
		}
	}
}

// Two wildcard imports bringing different members of one name make the
// importing namespace's memberships indistinguishable (KerML 8.3.2.4.5), and
// both memberships are hidden from it (KerML 7.2.5.4). The warning sits on the
// import bringing the later membership and names both members and both
// imports; the unqualified name resolves to nothing, and its diagnostic says why.
func TestImportedMemberNamesIndistinguishable(t *testing.T) {
	const src = `package A { part def Engine; }
package B { part def Engine; }
package C {
	private import A::*;
	private import B::*;
	part e : Engine;
}`
	r := resolveDoc(t, "d.sysml", src)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(r.Diagnostics), r.Diagnostics)
	}
	hiddenIn(t, r, "C", "Engine")
	unresolved := &r.Diagnostics[0]
	wantHint := "The name is hidden here: import A::* and import B::* bring distinct elements named 'Engine', so none is a member; qualify the one meant: A::Engine or B::Engine."
	if unresolved == nil || !strings.HasPrefix(unresolved.Message, "unresolved reference: Engine") || !strings.HasSuffix(unresolved.Message, wantHint) {
		t.Errorf("got %v, want an unresolved reference to Engine ending in %q", unresolved, wantHint)
	}
}

// A recursive import bringing two nested members of one name is one import
// bringing two indistinguishable memberships; both are hidden, silently.
func TestImportedMemberNamesRecursiveImport(t *testing.T) {
	const src = `package V { package P { part vb; } package R { part vb; } }
package C { private import V::**; }`
	r := resolveDoc(t, "d.sysml", src)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("got diagnostics %v, want none", r.Diagnostics)
	}
	hiddenIn(t, r, "C", "vb")
}

// Nothing is reported where no two memberships collide: one membership reached
// through two imports, an owned member hiding the imported name, a membership
// import beside the wildcard that also surfaces it, an alias beside the element
// it names, two imports of one namespace, and imports of different names.
func TestImportedMemberNamesDistinguishable(t *testing.T) {
	cases := map[string]string{
		"same membership via re-export": `package A { part def E; }
package Q { public import A::*; }
package C { private import A::*; private import Q::*; }`,
		"owned member hides imported": `package A { part def E; }
package B { part def E; }
package C { private import A::*; private import B::*; part def E; }`,
		"owned short name hides imported": `package A { part def E; }
package B { part def E; }
package C { private import A::*; private import B::*; part def <E> Engine; }`,
		"membership import beside wildcard": `package A { part def E; }
package C { private import A::E; private import A::*; }`,
		"alias beside its element": `package A { part def E; }
package B { alias E for A::E; }
package C { private import A::*; private import B::*; }`,
		"same import twice": `package A { part def E; }
package C { private import A::*; private import A::*; }`,
		"different names": `package A { part def E; }
package B { part def F; }
package C { private import A::*; private import B::*; }`,
		"private member not imported": `package A { part def E; }
package B { private part def E; }
package C { private import A::*; private import B::*; }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			r := resolveDoc(t, "d.sysml", src)
			if _, ok := r.LookupName(r.idx.Declaring("C").Scope, "E"); !ok {
				t.Errorf("E does not resolve in C, want A::E: nothing collides with it")
			}
		})
	}
}

// Short names take part as names do (KerML 8.3.2.4.3): an imported short name
// repeating another import's name collides, and `import all` surfaces the
// private members that collide too.
func TestImportedMemberNamesShortNameAndImportAll(t *testing.T) {
	const src = `package A { part def <E> Engine; }
package B { private part def E; }
package C { private import A::*; private import all B::*; }`
	r := resolveDoc(t, "d.sysml", src)
	for _, name := range []string{"E", "Engine"} {
		hiddenIn(t, r, "C", name)
	}
}

// Without a semantic model no metaclass is known, so imported members of one
// name are never distinguishable by metaclass and the pair is hidden, as the
// owned pair is reported.
func TestImportedMemberNamesNoModel(t *testing.T) {
	const src = `package A { part def E; }
package B { attribute def E; }
package C { private import A::*; private import B::*; }`
	r := resolveDoc(t, "d.sysml", src)
	hiddenIn(t, r, "C", "E")
}

// Two documents declaring one package and member make that package's own name
// repeat; the importer of `Landers::*` is not where that duplicate belongs.
func TestImportedMemberNamesOwnedRepeatAcrossDocuments(t *testing.T) {
	docs := map[string]string{
		"a.sysml": `package Landers { part def Lander; }`,
		"b.sysml": `package Landers { part def Lander; }`,
		"c.sysml": `package C { private import Landers::*; part l : Lander; }`,
	}
	idx := symbols.NewIndex()
	roots := map[string]*ast.RootNamespace{}
	for _, name := range []string{"a.sysml", "b.sysml", "c.sysml"} {
		p := parser.New(source.New(name, []byte(docs[name])))
		roots[name] = p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("%s: parse diagnostics: %v", name, p.Diagnostics)
		}
		idx.AddDocument(name, roots[name])
	}
	idx.ExpandWildcardImports()
	r := New(idx)
	r.ResolveDocument("c.sysml", roots["c.sysml"])
	for _, d := range r.Diagnostics {
		if d.Code == CodeNameConflict {
			t.Errorf("got %q, want no name conflict at the importer", d.Message)
		}
	}
}

// An alias binds its own name, not its target's: `Spare` reached first does not
// stand in for `A::Engine` under the name `Engine`, which still collides with a
// third `Engine`.
func TestImportedMemberNamesAliasUnderAnotherName(t *testing.T) {
	const src = `package A { part def Engine; }
package B { alias Spare for A::Engine; }
package C { part def Engine; }
package Use { private import B::*; private import A::*; private import C::*; }`
	r := resolveDoc(t, "d.sysml", src)
	hiddenIn(t, r, "Use", "Engine")
	if sym, ok := r.LookupName(r.idx.Declaring("Use").Scope, "Spare"); !ok || sym.Name != "Spare" {
		t.Errorf("Spare resolves to %v, want the alias: under its own name it collides with nothing", sym)
	}
}

// Two types of one name are two namespaces, whatever their qualified name: the
// members a recursive import takes from each collide at the importer. Only a
// package's declarations are one namespace across documents.
func TestImportedMemberNamesNestedInSameNamedTypes(t *testing.T) {
	const src = `package A { part def P { part x; } part def P { part x; } }
package Use { private import A::**; }`
	r := resolveDoc(t, "d.sysml", src)
	if sym, ok := r.LookupName(r.idx.Declaring("Use").Scope, "x"); ok {
		t.Errorf("x resolves to %v in Use, want nothing: the two nested memberships are hidden", sym)
	}
}

// Analyzing one document computes the collisions of its own importing
// namespaces alone: what a private import brings into the imported namespace
// is no member of it, so enumerating that namespace needs no collisions of its
// own. Without this each document walked its chain of imports to the end, and
// a workspace analyzed one document at a time did quadratic work.
func TestImportLookupComputesCollisionsOfReferrerOnly(t *testing.T) {
	const n = 20
	docs := map[string]string{}
	for k := 0; k < n; k++ {
		var src strings.Builder
		fmt.Fprintf(&src, "package P%d {\n", k)
		if k > 0 {
			fmt.Fprintf(&src, "    private import P%d::*;\n    part q : D%d;\n", k-1, k-1)
		}
		fmt.Fprintf(&src, "    part def D%d;\n}\n", k)
		docs[fmt.Sprintf("p%d.sysml", k)] = src.String()
	}
	idx := symbols.NewIndex()
	roots := map[string]*ast.RootNamespace{}
	for name, src := range docs {
		roots[name] = parsedRoot(t, name, src)
		idx.AddDocument(name, roots[name])
	}
	last := fmt.Sprintf("p%d.sysml", n-1)
	r := New(idx)
	r.ResolveDocument(last, roots[last])
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", r.Diagnostics)
	}
	if got := len(r.collisions); got != 1 {
		t.Errorf("collisions computed for %d namespaces, want 1: the importing package, none of the chain it imports", got)
	}
}
