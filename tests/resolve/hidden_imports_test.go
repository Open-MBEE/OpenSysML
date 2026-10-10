package resolve_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// hiddenImportHint is the use-site explanation of a name two imports hide.
const hiddenImportHint = "The name is hidden here: import Left::* and import Right::* bring distinct elements named 'Engine', so none is a member; qualify the one meant: Left::Engine or Right::Engine."

// bindingOf resolves the reference written as text in the document and
// returns the fully-qualified name it binds, or "" when it is unresolved.
func bindingOf(t *testing.T, r *resolve.Resolver, root *ast.RootNamespace, scope *symbols.Scope, text string) string {
	t.Helper()
	var found *resolve.Reference
	for _, ref := range resolve.References(root, scope) {
		if nameText(ref.QN) == text {
			ref := ref
			if found != nil {
				t.Fatalf("%q is written more than once", text)
			}
			found = &ref
		}
	}
	if found == nil {
		t.Fatalf("%q is not written", text)
	}
	sym, ok := r.ResolveReference(*found)
	if !ok {
		return ""
	}
	return symbols.FQNOf(sym)
}

// diagnosticsOf partitions r's diagnostics into warnings and errors.
func diagnosticsOf(r *resolve.Resolver) (warnings, errors []string) {
	for _, d := range r.Diagnostics {
		if d.Warning {
			warnings = append(warnings, d.Message)
		} else {
			errors = append(errors, d.Message)
		}
	}
	return warnings, errors
}

// Two imports bringing distinct elements under one name hide both
// memberships from the importing namespace (KerML 7.2.5.4): the unqualified
// name resolves through the enclosing namespaces or not at all, the warning
// marks the hidden memberships on the import, and the use-site diagnostic
// names the imports and the qualified names that still reach each element.
func TestHiddenImportedMemberships(t *testing.T) {
	const left, right = "package Left { part def Engine; }\n", "package Right { part def Engine; }\n"
	cases := []struct {
		name, src string
		ref       string // the reference examined
		binds     string // FQN it binds, "" for unresolved
		warnings  int    // import warnings expected
		hint      bool   // the unresolved diagnostic carries the hidden-import hint
	}{
		{"root namespace", left + right + "private import Left::*; private import Right::*; part e : Engine;", "Engine", "", 1, true},
		{"package", left + right + "package Use { private import Left::*; private import Right::*; part e : Engine; }", "Engine", "", 1, true},
		{"one element via two paths", left + "package Alias { public import Left::Engine; }\npackage Use { private import Left::*; private import Alias::*; part e : Engine; }", "Engine", "Left::Engine", 0, false},
		{"membership import beside wildcard", left + "package Use { private import Left::Engine; private import Left::*; part e : Engine; }", "Engine", "Left::Engine", 0, false},
		{"alias beside its target", "package Left { part def Engine; alias Motor for Engine; }\npackage Use { private import Left::*; part e : Engine; part m : Motor; }", "Motor", "Left::Engine", 0, false},
		{"owned member hides the imports", left + right + "package Use { private import Left::*; private import Right::*; part def Engine; part e : Engine; }", "Engine", "Use::Engine", 0, false},
		{"protected imports of a general", left + right + "part def Gen { protected import Left::*; protected import Right::*; }\npart def Sub :> Gen { part e : Engine; }", "Engine", "", 1, true},
		{"re-export namespace", left + right + "package Both { public import Left::*; public import Right::*; }\npackage Use { private import Both::*; part e : Engine; }", "Engine", "", 1, true},
		{"enclosing namespace supplies the name", left + right + "part def Engine;\npackage Use { private import Left::*; private import Right::*; part e : Engine; }", "Engine", "Engine", 1, false},
		{"qualified into the importing namespace", left + right + "package Use { private import Left::*; private import Right::*; }\npart u : Use::Engine;", "Use::Engine", "", 1, false},
		{"qualified into the importing namespace through a re-export", left + right + "package Both { public import Left::*; public import Right::*; }\npackage Use { private import Both::*; }\npart u : Use::Engine;", "Use::Engine", "", 1, false},
		{"qualified into the exporter", left + right + "package Use { private import Left::*; private import Right::*; part l : Left::Engine; }", "Left::Engine", "Left::Engine", 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, root, scope := resolvedDoc(t, tc.src)
			if got := bindingOf(t, r, root, scope, tc.ref); got != tc.binds {
				t.Errorf("%s binds %q, want %q", tc.ref, got, tc.binds)
			}
			warnings, errors := diagnosticsOf(r)
			if len(warnings) != tc.warnings {
				t.Errorf("got %d warnings, want %d: %v", len(warnings), tc.warnings, warnings)
			}
			for _, w := range warnings {
				if !strings.HasPrefix(w, "Duplicate of imported member name 'Engine': Left::Engine (import Left::*), Right::Engine (import Right::*)") {
					t.Errorf("unexpected warning %q", w)
				}
			}
			if tc.binds != "" {
				if len(errors) != 0 {
					t.Errorf("unexpected errors: %v", errors)
				}
				return
			}
			if len(errors) != 1 || !strings.HasPrefix(errors[0], "unresolved reference: "+tc.ref) {
				t.Fatalf("got errors %v, want one unresolved reference to %s", errors, tc.ref)
			}
			if strings.HasSuffix(errors[0], hiddenImportHint) != tc.hint {
				t.Errorf("hint present = %v in %q, want %v", !tc.hint, errors[0], tc.hint)
			}
		})
	}
}

// Qualified names reach the colliding elements wherever they are written.
func TestHiddenImportedMembershipsQualifiedNamesStay(t *testing.T) {
	r, root, scope := resolvedDoc(t, `package Left { part def Engine; }
package Right { part def Engine; }
package Use { private import Left::*; private import Right::*; part l : Left::Engine; part rr : Right::Engine; }`)
	if got := bindingOf(t, r, root, scope, "Left::Engine"); got != "Left::Engine" {
		t.Errorf("Left::Engine binds %q", got)
	}
	if got := bindingOf(t, r, root, scope, "Right::Engine"); got != "Right::Engine" {
		t.Errorf("Right::Engine binds %q", got)
	}
	if _, errors := diagnosticsOf(r); len(errors) != 0 {
		t.Errorf("unexpected errors: %v", errors)
	}
}

// A membership is hidden whole: a short name two imports share hides both
// memberships under their names too.
func TestHiddenImportedMembershipsShortName(t *testing.T) {
	r, root, scope := resolvedDoc(t, `package Left { part def <E> Motor; }
package Right { part def <E> Engine; }
package Use { private import Left::*; private import Right::*; part m : Motor; part e : E; }`)
	for _, ref := range []string{"Motor", "E"} {
		if got := bindingOf(t, r, root, scope, ref); got != "" {
			t.Errorf("%s binds %q, want nothing", ref, got)
		}
	}
	warnings, errors := diagnosticsOf(r)
	if len(warnings) != 1 || warnings[0] != "Duplicate of imported member name 'E': Left::Motor (import Left::*), Right::Engine (import Right::*)" {
		t.Errorf("warnings = %v", warnings)
	}
	if len(errors) != 2 {
		t.Errorf("errors = %v, want Motor and E unresolved", errors)
	}
}

// One recursive import bringing two nested elements of one name hides both.
func TestHiddenImportedMembershipsRecursiveImport(t *testing.T) {
	r, root, scope := resolvedDoc(t, `package V { package P { part def Engine; } package R { part def Engine; } }
package Use { private import V::**; part e : Engine; part p : P::Engine; }`)
	if got := bindingOf(t, r, root, scope, "Engine"); got != "" {
		t.Errorf("Engine binds %q, want nothing", got)
	}
	if got := bindingOf(t, r, root, scope, "P::Engine"); got != "V::P::Engine" {
		t.Errorf("P::Engine binds %q", got)
	}
	_, errors := diagnosticsOf(r)
	want := "The name is hidden here: import V::** brings distinct elements named 'Engine', so none is a member; qualify the one meant: V::P::Engine or V::R::Engine."
	if len(errors) != 1 || !strings.HasSuffix(errors[0], want) {
		t.Errorf("errors = %v, want one ending in %q", errors, want)
	}
}

// An inherited member and an imported one of the same name are both
// memberships of the type (KerML 8.3.3.1.10 nonPrivateMemberships adds the
// inherited to the owned and imported): neither hides the other, the type is
// ill-formed and warns, and the name keeps resolving to the inherited member.
func TestInheritedAndImportedMembershipsHideNothing(t *testing.T) {
	r, root, scope := resolvedDoc(t, `package A { part x; }
part def Gen { part x; }
part def Child :> Gen { private import A::*; part y :> x; }`)
	if got := bindingOf(t, r, root, scope, "x"); got != "Gen::x" {
		t.Errorf("x binds %q, want Gen::x", got)
	}
	warnings, errors := diagnosticsOf(r)
	if len(warnings) != 1 || warnings[0] != "Duplicate of imported member name 'x': A::x (import A::*), Gen::x (inherited)" {
		t.Errorf("warnings = %v", warnings)
	}
	if len(errors) != 0 {
		t.Errorf("errors = %v", errors)
	}
}

// A called name denotes an overload set: every behavior imported under it is
// a candidate, hidden or not, while the same name as an ordinary reference
// resolves to nothing.
func TestHiddenImportedMembershipsOverloadSet(t *testing.T) {
	r, root, scope := resolvedDoc(t, `package A { calc def pick { in x; return r = x; } }
package B { calc def pick { in s; in t; return r = s; } }
package Use { private import A::*; private import B::*; calc c = pick(1); attribute p : pick; }`)
	useScope := scope
	for _, ref := range resolve.References(root, scope) {
		if nameText(ref.QN) == "pick" {
			useScope = ref.Scope
		}
	}
	cands := r.InvocationCandidates(useScope, spelling(false, "pick"))
	var names []string
	for _, c := range cands {
		names = append(names, symbols.FQNOf(c))
	}
	if strings.Join(names, ",") != "A::pick,B::pick" {
		t.Errorf("invocation candidates = %v, want A::pick and B::pick", names)
	}
	if sym, ok := r.LookupName(useScope, "pick"); ok {
		t.Errorf("pick as an ordinary name resolves to %v, want nothing", sym)
	}
	_, errors := diagnosticsOf(r)
	if len(errors) != 1 || !strings.HasPrefix(errors[0], "unresolved reference: pick") {
		t.Errorf("errors = %v, want the typing `: pick` unresolved only", errors)
	}
}

// The colliding packages may be declared in other documents than the importer.
func TestHiddenImportedMembershipsAcrossDocuments(t *testing.T) {
	const name = "use.sysml"
	idx := symbols.NewIndex()
	docs := map[string]string{
		"left.sysml":  "package Left { part def Engine; }",
		"right.sysml": "package Right { part def Engine; }",
		name:          "package Use { private import Left::*; private import Right::*; part e : Engine; }",
	}
	var root *ast.RootNamespace
	for doc, src := range docs {
		p := parser.New(source.New(doc, []byte(src)))
		parsed := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics: %v", p.Diagnostics)
		}
		idx.AddDocument(doc, parsed)
		if doc == name {
			root = parsed
		}
	}
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	r.SetModel(semantics.NewModel(r))
	r.ResolveDocument(name, root)
	if got := bindingOf(t, r, root, idx.DocumentRoot(name), "Engine"); got != "" {
		t.Errorf("Engine binds %q, want nothing", got)
	}
}

// Hiding settles over packages importing one another in a cycle, each
// namespace's collisions computed once rather than once per path through the
// cycle: a name two of them bring distinct elements under is hidden in each.
func TestHiddenImportedMembershipsThroughAnImportCycle(t *testing.T) {
	const n = 8
	var src strings.Builder
	src.WriteString("package Left { part def Engine; }\npackage Right { part def Engine; }\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&src, "package P%d {\n", i)
		for k := 1; k <= n; k++ {
			if k != i {
				fmt.Fprintf(&src, "\tpublic import P%d::*;\n", k)
			}
		}
		if i <= 2 {
			fmt.Fprintf(&src, "\tpublic import %s::*;\n", []string{"Left", "Right"}[i-1])
		}
		fmt.Fprintf(&src, "\tpart def D%d;\n}\n", i)
	}
	fmt.Fprintf(&src, "package Use { private import P1::*; part d : D%d; part e : Engine; }\n", n)
	src.WriteString("package Other { private import P2::*; part e : Engine; }\n")

	done := make(chan struct{})
	var r *resolve.Resolver
	var root *ast.RootNamespace
	var scope *symbols.Scope
	go func() {
		defer close(done)
		r, root, scope = resolvedDoc(t, src.String())
		resolve.References(root, scope)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("resolving over the import cycle did not finish in 10s")
	}
	if got := bindingOf(t, r, root, scope, fmt.Sprintf("D%d", n)); got != fmt.Sprintf("P%d::D%d", n, n) {
		t.Errorf("D%d binds %q, want P%d::D%d", n, got, n, n)
	}
	warnings, errors := diagnosticsOf(r)
	if len(warnings) != n {
		t.Errorf("got %d warnings %v, want Engine hidden in each of the %d packages", len(warnings), warnings, n)
	}
	if len(errors) != 2 {
		t.Fatalf("got errors %v, want Engine unresolved in Use and in Other: Left::Engine and Right::Engine meet in the cycle", errors)
	}
	for _, e := range errors {
		if !strings.HasPrefix(e, "unresolved reference: Engine") || !strings.Contains(e, "distinct elements named 'Engine'") {
			t.Errorf("unexpected error %q", e)
		}
	}
}
