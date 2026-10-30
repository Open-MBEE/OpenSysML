package export_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A SysML usage's isVariable is its derived mayTimeVary (Usage::mayTimeVary
// redefines Feature::isVariable): stated by the graph as the reflective read
// answers it, and derivable on the way back, so no `var` keyword appears.
func TestUsageIsVariableIsItsDerivedMayTimeVary(t *testing.T) {
	src := `package Vary {
	part def P {
		part q : P;
		attribute a;
		action act;
	}
	attribute def D {
		attribute b;
	}
	item def Fuel;
	part def Car {
		part tank;
		part eng;
		flow fuelFlow of fuel : Fuel from tank to eng;
		flow airFlow of Fuel from tank to eng;
	}
}
`
	turtle := roundTripsExactly(t, src)
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	elmt := func(name string) string { return rdf.ElementIRI(name).Value }
	for _, varying := range []string{"Vary::P::q", "Vary::P::a"} {
		wantLexical(t, g, elmt(varying), rdf.SysML+"isVariable", "true")
		wantLexical(t, g, elmt(varying), rdf.SysML+"mayTimeVary", "true")
	}
	// A flow's payload, declared (`of fuel : Fuel`) or not (`of Fuel`), is a
	// PayloadFeature: a KerML Feature, so neither usage-derived flag applies.
	payloads := 0
	for _, payload := range g.Subjects() {
		if g.Type(payload) != rdf.SysML+"PayloadFeature" {
			continue
		}
		payloads++
		for _, p := range []string{"isVariable", "mayTimeVary"} {
			if g.HasProperty(payload, rdf.SysML+p) {
				t.Errorf("payload %s states %s; a PayloadFeature is no usage", payload.Value, p)
			}
		}
	}
	if payloads != 2 {
		t.Errorf("found %d PayloadFeature subjects, want the two flows' payloads", payloads)
	}
	for _, constant := range []string{"Vary::P::act", "Vary::D::b"} {
		for _, p := range []string{"isVariable", "mayTimeVary"} {
			if g.HasProperty(iri(elmt(constant)), rdf.SysML+p) {
				t.Errorf("%s states %s, which deriveUsageMayTimeVary denies it", constant, p)
			}
		}
	}
}
