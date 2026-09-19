package passes

import (
	"strings"
	"testing"
)

// f90PortMessage is the ConjugatedPortTyping rule's message, matched rather than
// counted so a fixture's other diagnostics do not stand in for it.
const f90PortMessage = "'~' names the conjugated port definition of a port definition"

// A KerML declaration conjugation is a Conjugation between Types, not a
// ConjugatedPortTyping, so the port-definition demand must not reach it — KerML
// has no ports at all. validate-kerml is silent on every shape in the fixture.
// The fixture names `Base::Anything`, so it is analyzed against the library:
// otherwise the unresolved name would skip the type tier and pass vacuously.
func TestF90KerMLConjugationIsNotAPortTyping(t *testing.T) {
	if diags := libraryFixtureDiags(t, "f90_conjugation.kerml"); len(diags) != 0 {
		t.Fatalf("expected no diagnostics on legal KerML conjugation, got %v", diags)
	}
}

// The same shapes stated inline, so a regression is attributable to the form
// that reintroduces it: the declaration part, the keyword-first relationship and
// the feature conjugation are three separate parser paths.
func TestF90KerMLConjugationFormsAreClean(t *testing.T) {
	for _, src := range []string{
		"class A; class B conjugates A;",
		"class A; type C ~ A;",
		"class A { in feature f; } feature g ~ A::f;",
		"type O; type C1; conjugation c1 conjugate C1 conjugates O;",
		"type O; type C2; conjugation c2 conjugate C2 ~ O;",
	} {
		if diags := diagsIn(t, "f90.kerml", src, "type"); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// The notation the SysML rule accepts stays clean: a port typed by the conjugate
// of a port definition.
func TestF90SysMLConjugatedPortTypingIsClean(t *testing.T) {
	if diags := libraryFixtureDiags(t, "f90_conjugation_ports.sysml"); len(diags) != 0 {
		t.Fatalf("expected no diagnostics on `port p : ~P`, got %v", diags)
	}
}

// The SysML rule survives the scoping: `~` there is a ConjugatedPortTyping, so a
// non-port target is still an error, as is a non-port usage carrying one. The
// pinned validate-sysml rejects `~Q` too, as an unresolvable ConjugatedPortDefinition.
func TestF90SysMLConjugatedPortTypingStillChecked(t *testing.T) {
	diags := diagsIn(t, "f90.sysml", "port def P; part def Q; part def X { port p : ~Q; }", "type")
	if len(diags) == 0 || !strings.Contains(diags[0].Message, f90PortMessage) {
		t.Fatalf("expected the conjugated-port-typing rule to reject `~Q`, got %v", diags)
	}
	diags = diagsIn(t, "f90.sysml", "port def P; part def X { part p : ~P; }", "type")
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "only a port usage or a connector end") {
		t.Fatalf("expected the port-usage restriction to fire, got %v", diags)
	}
}
