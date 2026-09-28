package export_test

import (
	"strings"
	"testing"
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
