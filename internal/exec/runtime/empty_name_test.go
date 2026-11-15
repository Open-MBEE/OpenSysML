package runtime

import "testing"

// A result parameter with the empty name is a named one: the tool answers it under that name
// and the calculation returns the answer.
func TestToolCalcReturnsAnEmptyNamedResult(t *testing.T) {
	ctx, scope := analysisFixture(t, `package test {
	private import ScalarValues::*;
	private import AnalysisTooling::*;

	calc def Bare {
		metadata ToolExecution { toolName = "Thermo"; uri = "u"; }
		in a : Real[1] { @ToolVariable { name = "a"; } }
		return '' : Real[1];
	}

	calc def Varied {
		metadata ToolExecution { toolName = "Thermo"; uri = "u"; }
		in a : Real[1] { @ToolVariable { name = "a"; } }
		return '' : Real[1] { @ToolVariable { name = "T"; } }
	}
}`)
	for calc, variable := range map[string]string{"Bare": "", "Varied": "T"} {
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{variable: {Value: toolReal(7)}}})
		result, err := ctx.InvokeCalc(calcNamed(t, scope, calc), []Value{realOf(1)}, scope)
		if err != nil {
			t.Fatalf("%s: InvokeCalc: %v", calc, err)
		}
		if got := FormatValue(result); got != "7.0" {
			t.Errorf("%s: result = %s, want 7.0", calc, got)
		}
	}
}

// A connector end with the empty name is a declared end: the connected object is
// bound under that name, as under any other.
func TestConnectorBindsAnEmptyNamedEnd(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
	package test {
		port def P;
		part def A { port p : P; }
		part def B { port q : P; }
		connection def Link {
			end '' : P;
			end target : P;
		}
		part def Sys {
			part a : A;
			part b : B;
			connection link : Link connect a.p to b.q;
			connection '' : Link connect a.p to b.q;
		}
	}`)
	port := fvInstance(t, ctx, inst, "a", "p")
	peer := fvInstance(t, ctx, inst, "b", "q")
	link := fvInstance(t, ctx, inst, "link")
	if got := fvInstance(t, ctx, link, ""); got.ID != port.ID {
		t.Errorf("the empty-named end is object %d, want a.p (%d)", got.ID, port.ID)
	}
	if got := fvInstance(t, ctx, link, "target"); got.ID != peer.ID {
		t.Errorf("link.target is object %d, want b.q (%d)", got.ID, peer.ID)
	}
	// The connector with the empty name is a named member, not an anonymous one.
	if got := fvInstance(t, ctx, fvInstance(t, ctx, inst, ""), "target"); got.ID != peer.ID {
		t.Errorf("the empty-named connector's target is object %d, want b.q (%d)", got.ID, peer.ID)
	}
	anonymous, err := inst.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	if len(anonymous) != 0 {
		t.Errorf("object owns %d anonymous connectors, want none", len(anonymous))
	}
}
