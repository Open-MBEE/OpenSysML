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

// A usage the exporter mints for a declaration — a connector end, a
// transition's or trigger's parameter, a satisfy's subject, a flow end's
// feature — varies in time when its owning type is an Occurrence
// (deriveUsageMayTimeVary): a connection or action yes, a succession (a Link) no.
func TestMintedUsagesStateTheirDerivedMayTimeVary(t *testing.T) {
	src := `package Minted {
	item def Fuel;
	part def Tank { out port fuelOut; }
	part def Engine { in port fuelIn; }
	requirement def R { subject engineOfInterest : Engine; }
	part def System {
		part tank : Tank;
		part engine : Engine;
		connection c connect tank to engine;
		succession s first tank then engine;
		flow f of Fuel from tank.fuelOut to engine.fuelIn;
		satisfy requirement r : R by engine;
		state def M {
			state off;
			state on;
			transition t first off accept go : Fuel via engine.fuelIn then on;
		}
	}
}
`
	turtle := roundTripsExactly(t, src)
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	owners := map[string]string{}
	for _, triple := range g.Triples() {
		if triple.Predicate.Value == rdf.SysML+"owner" {
			owners[triple.Subject.Value] = triple.Object.Value
		}
	}
	var (
		connectionEnds, successionEnds, flowEnds, flowFeatures, transitionParams, acceptParams, subjects, declaredSubjects int
	)
	for _, subject := range g.Subjects() {
		owner := owners[subject.Value]
		ownerClass := rdf.LocalName(g.Type(rdf.IRI(owner)))
		class := rdf.LocalName(g.Type(subject))
		isEnd, _ := g.Lexical(subject, rdf.SysML+"isEnd")
		switch {
		case ownerClass == "ConnectionUsage" && isEnd == "true":
			connectionEnds++
			wantLexical(t, g, subject.Value, rdf.SysML+"isVariable", "true")
			wantLexical(t, g, subject.Value, rdf.SysML+"mayTimeVary", "true")
		case ownerClass == "SuccessionAsUsage" && isEnd == "true":
			successionEnds++
			wantNoProperty(t, g, subject, rdf.SysML+"isVariable")
			wantNoProperty(t, g, subject, rdf.SysML+"mayTimeVary")
		case class == "FlowEnd":
			flowEnds++
			wantNoProperty(t, g, subject, rdf.SysML+"isVariable")
			wantNoProperty(t, g, subject, rdf.SysML+"mayTimeVary")
		case ownerClass == "FlowEnd" && class == "ReferenceUsage":
			flowFeatures++
			wantLexical(t, g, subject.Value, rdf.SysML+"isVariable", "true")
			wantLexical(t, g, subject.Value, rdf.SysML+"mayTimeVary", "true")
		case ownerClass == "TransitionUsage" && class == "ReferenceUsage":
			transitionParams++
			wantLexical(t, g, subject.Value, rdf.SysML+"isVariable", "true")
			wantLexical(t, g, subject.Value, rdf.SysML+"mayTimeVary", "true")
		case ownerClass == "AcceptActionUsage" && class == "ReferenceUsage":
			acceptParams++
			wantLexical(t, g, subject.Value, rdf.SysML+"isVariable", "true")
			wantLexical(t, g, subject.Value, rdf.SysML+"mayTimeVary", "true")
		case ownerClass == "RequirementDefinition" && class == "ReferenceUsage":
			declaredSubjects++
			wantLexical(t, g, subject.Value, rdf.SysML+"isVariable", "true")
			wantLexical(t, g, subject.Value, rdf.SysML+"mayTimeVary", "true")
		case ownerClass == "SatisfyRequirementUsage" && class == "ReferenceUsage":
			subjects++
			wantLexical(t, g, subject.Value, rdf.SysML+"isVariable", "true")
			wantLexical(t, g, subject.Value, rdf.SysML+"mayTimeVary", "true")
		}
	}
	for name, n := range map[string]int{
		"connection ends": connectionEnds, "succession ends": successionEnds, "flow ends": flowEnds,
		"flow end features": flowFeatures, "transition parameters": transitionParams,
		"accept parameters": acceptParams, "satisfy subjects": subjects, "declared subjects": declaredSubjects,
	} {
		if n == 0 {
			t.Errorf("no %s exported", name)
		}
	}
}

func wantNoProperty(t *testing.T, g *rdf.Graph, subject rdf.Term, predicate string) {
	t.Helper()
	if g.HasProperty(subject, predicate) {
		t.Errorf("%s states %s, want none", subject.Value, rdf.LocalName(predicate))
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
