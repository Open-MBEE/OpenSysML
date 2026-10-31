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
	variation part def EngineChoices :> P {
		variant part e4 : P;
		part plain : P;
	}
}
`
	turtle := roundTripsExactly(t, src)
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	elmt := func(name string) string { return rdf.ElementIRI(name).Value }
	for _, varying := range []string{"Vary::P::q", "Vary::P::a", "Vary::EngineChoices::plain"} {
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
	// A variant is held by a VariantMembership, no FeatureMembership: no owningType.
	for _, constant := range []string{"Vary::P::act", "Vary::D::b", "Vary::EngineChoices::e4"} {
		for _, p := range []string{"isVariable", "mayTimeVary"} {
			if g.HasProperty(iri(elmt(constant)), rdf.SysML+p) {
				t.Errorf("%s states %s, which deriveUsageMayTimeVary denies it", constant, p)
			}
		}
	}
}

// An expression body's parameter is owned by the Expression, an Occurrence, so
// a SysML one varies in time (deriveUsageMayTimeVary); a KerML one is a Feature.
func TestBodyParameterStatesItsDerivedMayTimeVary(t *testing.T) {
	src := `package Vary {
	attribute values : Real[*];
	attribute positives = values->select { in v : Real; v > 0 };
}
`
	turtle := roundTripsExactly(t, src)
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	found := false
	for _, subject := range g.Subjects() {
		if name, ok := g.Lexical(subject, rdf.SysML+"declaredName"); !ok || name != "v" {
			continue
		}
		found = true
		wantLexical(t, g, subject.Value, rdf.SysML+"isVariable", "true")
		wantLexical(t, g, subject.Value, rdf.SysML+"mayTimeVary", "true")
	}
	if !found {
		t.Fatal("body parameter v not exported")
	}
}
