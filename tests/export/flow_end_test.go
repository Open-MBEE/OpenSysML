package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

const flowEndModel = `package F {
    item def Fuel;
    port def P { out item fuel : Fuel; }
    part def Tank { out item fuel : Fuel; }
    part def Engine { in item fuel : Fuel; }
    part a { port p : P; }
    part b { port p : ~P; }
    part t : Tank;
    part e : Engine;
    flow of Fuel from a.p.fuel to b.p.fuel;
    flow t.fuel to e.fuel;
    flow feed of Fuel from t.fuel to e.fuel;
    action def Run {
        action pump { out item fuel : Fuel; }
        action burn { in item fuel : Fuel; }
        succession flow pump.fuel to burn.fuel;
    }
}
`

// A flow's end is a FlowEnd (SysML.xtext FlowEnd): a ReferenceSubsetting of
// all but its last segment — a chain for `a.p`, the feature itself for `t` —
// and an owned FlowFeature redefining the last one. Every form comes back from
// the graph alone as written.
func TestFlowEndsAreFlowEnds(t *testing.T) {
	turtle, back := graphOnlyRoundTrip(t, "f.sysml", []byte(flowEndModel))
	graph := string(turtle)
	for _, want := range []string{
		"expr:F___408_pend0\n    a sysml:FlowEnd ;",
		"expr:F___408_pend0_pff\n    a sysml:ReferenceUsage ;",
		"sysml:redefines elmt:F__P__fuel ;",
		"sysml:redefines elmt:F__Tank__fuel ;",
		"sysml:redefines elmt:F__Engine__fuel ;",
		"sysml:referencedFeature elmt:F__t ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	if strings.Contains(graph, "a sysml:ReferenceUsage ;\n    sysml:elementId \"F___408_pend0\"") {
		t.Errorf("a flow end is still a ReferenceUsage:\n%s", graph)
	}
	notation := string(back)
	for _, want := range []string{
		"flow of Fuel from a.p.fuel to b.p.fuel;\n",
		"flow t.fuel to e.fuel;\n",
		"flow feed of Fuel from t.fuel to e.fuel;\n",
		"succession flow pump.fuel to burn.fuel;\n",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
}

// An end whose first segment a nearer declaration shadows is qualified from
// the graph alone, so it still names what it named: a flow's end, and a
// connection's end chain the same way (review finding).
func TestShadowedEndChainsKeepTheirTargets(t *testing.T) {
	const model = `package S {
    item def Fuel;
    part def Tank { out item fuel : Fuel; }
    part def Engine { in item fuel : Fuel; }
    part t : Tank;
    part e : Engine;
    package Inner {
        part t : Tank;
        flow S::t.fuel to e.fuel;
    }
    package Wired {
        part t : Tank;
        connect S::t.fuel to e.fuel;
    }
}
`
	_, back := graphOnlyRoundTrip(t, "s.sysml", []byte(model))
	for _, want := range []string{"flow S::t.fuel to e.fuel;\n", "connect S::t.fuel to e.fuel;\n"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the notation should contain %q:\n%s", want, back)
		}
	}
}

// A FlowEnd that owns its FlowFeature only through the FeatureMembership, with
// no ownedFeature stated, still reads back (review finding).
func TestFlowEndFeatureFoundThroughItsMembership(t *testing.T) {
	turtle, err := convert.Convert("f.sysml", []byte(flowEndModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	g, err := rdf.ParseTurtle(withoutLayout(t, turtle))
	if err != nil {
		t.Fatal(err)
	}
	stripped := rdf.NewGraph()
	dropped := 0
	for _, triple := range g.Triples() {
		if triple.Predicate == rdf.SysMLTerm("ownedFeature") && strings.HasSuffix(triple.Object.Value, "_pff") {
			dropped++
			continue
		}
		stripped.AddTriple(triple)
	}
	if dropped == 0 {
		t.Fatal("no FlowFeature was stated through ownedFeature, so dropping it proves nothing")
	}
	back, err := convert.Convert("f.ttl", rdf.WriteTurtle(stripped), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("a flow end with a membership-only FlowFeature did not read back: %v", err)
	}
	if !strings.Contains(string(back), "flow of Fuel from a.p.fuel to b.p.fuel;\n") {
		t.Errorf("the flow did not read back as written:\n%s", back)
	}
}
