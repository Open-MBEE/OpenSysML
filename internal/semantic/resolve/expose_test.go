package resolve

import (
	"testing"
)

const exposeLib = `package Lib {
	public part def Pub;
	private part def Sec;
	part def Outer { public part def Deep; private part def DeepSec; }
}`

// `expose Pkg::*` is a NamespaceExpose (SysML v2 8.3.26.4), so the members of
// the exposed namespace — not just its own name — are visible in the view body.
func TestExposeNamespaceSurfacesMembers(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": exposeLib,
		"app.sysml": "package App { view v { expose Lib::*; } }",
	})
	view := scopeOf(t, scopeOf(t, idx.DocumentRoot("app.sysml"), "App"), "v")

	r := New(idx)
	if _, ok := r.ResolveName(view, "Pub", ident("Pub")); !ok {
		t.Fatalf("member Pub of the exposed namespace is not visible in the view body; diags=%v", r.Diagnostics)
	}
	r2 := New(idx)
	if _, ok := r2.ResolveName(view, "Deep", ident("Deep")); ok {
		t.Fatalf("Deep is nested one level below Lib and must need a recursive expose")
	}
}

// An Expose always imports all elements regardless of visibility
// (isImportAll = true, SysML v2 8.3.26.2), so a private member of the exposed
// namespace is exposed — unlike a plain namespace import, which hides it.
func TestExposeImportsPrivateMembers(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": exposeLib,
		"app.sysml": "package App { view v { expose Lib::*; } view w { expose Lib::**; } }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	r := New(idx)
	if _, ok := r.ResolveName(scopeOf(t, app, "v"), "Sec", ident("Sec")); !ok {
		t.Fatalf("expose ignores visibility (isImportAll): private member Sec must be exposed")
	}
	r2 := New(idx)
	if _, ok := r2.ResolveName(scopeOf(t, app, "w"), "DeepSec", ident("DeepSec")); !ok {
		t.Fatalf("recursive expose must reach the private nested member DeepSec")
	}
}

// An Expose always has protected visibility (SysML v2 8.3.26.2): the exposed
// elements are not publicly visible outside the view usage.
func TestExposeDoesNotLeakOutsideTheView(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": exposeLib,
		"app.sysml": "package App { view v { expose Lib::**; } import v::*; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	for _, name := range []string{"Pub", "Sec", "Deep", "DeepSec"} {
		r := New(idx)
		if _, ok := r.ResolveName(app, name, ident(name)); ok {
			t.Errorf("%s is exposed by view v and must not be visible outside it", name)
		}
	}
}

// A view definition body is a namespace too, so an expose declared there
// behaves the same way.
func TestExposeInViewDefinitionBody(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": exposeLib,
		"app.sysml": "package App { view def V { expose Lib::*; } }",
	})
	view := scopeOf(t, scopeOf(t, idx.DocumentRoot("app.sysml"), "App"), "V")

	r := New(idx)
	if _, ok := r.ResolveName(view, "Pub", ident("Pub")); !ok {
		t.Fatalf("member Pub of the exposed namespace is not visible in the view def body; diags=%v", r.Diagnostics)
	}
}
