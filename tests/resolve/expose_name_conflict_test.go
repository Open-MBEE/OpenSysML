package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const exposedParts = "package V {\n\tpart a { part p; }\n\tpart b { part p; }\n"

// Memberships that collide only among a view's exposes are no name conflict:
// the view exposes each Expose's own memberships (SysML ViewUsage::exposedElement),
// so both p's stay exposed.
func TestExposedCollisionsAreNoNameConflict(t *testing.T) {
	r, _, scope := resolvedDoc(t, exposedParts+"\tview v { expose a::**; expose b::**; }\n}")
	if warnings, errors := diagnosticsOf(r); len(warnings)+len(errors) != 0 {
		t.Fatalf("warnings = %v, errors = %v, want none", warnings, errors)
	}
	view := local(t, local(t, scope, "V").Scope, "v")
	exposed, err := semantics.NewModel(r).ExposedMembers(view)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, sym := range exposed {
		got[symbols.FQNOf(sym)] = true
	}
	for _, want := range []string{"V::a::p", "V::b::p"} {
		if !got[want] {
			t.Errorf("%s is not exposed; exposed = %v", want, got)
		}
	}
}

// A collision that a plain import or an inherited member takes part in is still
// reported in a view, as it changes what a name written there resolves to.
func TestExposedCollisionsBesideImportsAreNameConflicts(t *testing.T) {
	cases := map[string]struct{ src, want string }{
		"import and expose": {exposedParts + "\tview v { private import a::*; expose b::*; }\n}",
			"Duplicate of imported member name 'p': V::a::p (import a::*), V::b::p (import all b::*)"},
		"import of an exposed namespace": {exposedParts + "\tview v { expose a::*; expose b::*; private import a::*; }\n}",
			"Duplicate of imported member name 'p': V::a::p (import all a::*), V::b::p (import all b::*)"},
		"inherited member": {"package V {\n\tpart a { part p; }\n\tview def VD { part p; }\n\tview v : VD { expose a::*; }\n}",
			"Duplicate of imported member name 'p': V::a::p (import all a::*), V::VD::p (inherited)"},
		"two imports in a package": {exposedParts + "\tpackage U { private import a::*; private import b::*; }\n}",
			"Duplicate of imported member name 'p': V::a::p (import a::*), V::b::p (import b::*)"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r, _, _ := resolvedDoc(t, tc.src)
			warnings, errors := diagnosticsOf(r)
			if len(warnings) != 1 || warnings[0] != tc.want || len(errors) != 0 {
				t.Errorf("warnings = %v, errors = %v, want %q", warnings, errors, tc.want)
			}
		})
	}
}
