package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const exposedParts = "package V {\n\tpart a { part p; }\n\tpart b { part p; }\n"

// A collision among a view's exposes alone is no name conflict: the view exposes
// each Expose's own memberships (SysML ViewUsage::exposedElement).
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

// A collision that a plain import or an inherited member takes part in hides
// the imported membership (KerML 7.2.5.4, 7.3.2); the hiding is no violation
// and is not reported, in a view as in a package.
func TestExposedCollisionsBesideImportsAreHidden(t *testing.T) {
	cases := map[string]string{
		"import and expose":              exposedParts + "\tview v { private import a::*; expose b::*; }\n}",
		"import of an exposed namespace": exposedParts + "\tview v { expose a::*; expose b::*; private import a::*; }\n}",
		"inherited member":               "package V {\n\tpart a { part p; }\n\tview def VD { part p; }\n\tview v : VD { expose a::*; }\n}",
		"two imports in a package":       exposedParts + "\tpackage U { private import a::*; private import b::*; }\n}",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			r, _, _ := resolvedDoc(t, src)
			if warnings, errors := diagnosticsOf(r); len(warnings)+len(errors) != 0 {
				t.Errorf("warnings = %v, errors = %v, want none", warnings, errors)
			}
		})
	}
}
