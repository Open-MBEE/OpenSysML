package lower

import (
	"slices"
	"testing"
)

// The streaming flows a type declares between feature paths lower with both paths; a
// succession flow, a flow stating its ends in its body and a binding are none.
func TestToFeatureFlows(t *testing.T) {
	src := `package test {
		part def Context {
			part pusher { perform action push { out output; } }
			part cart { perform action roll { in input; } }
			flow pusher.push.output to cart.roll.input;
			flow named from pusher.push.output to cart.roll.input;
			succession flow pusher.push.output to cart.roll.input;
			flow { end ::> pusher; end ::> cart; }
			bind pusher.push.output = cart.roll.input;
		}
	}`
	sym, _, _ := stateSpaceAction(t, src, "Context")
	flows := ToFeatureFlows(sym.Decl, sym.Scope)
	if len(flows) != 2 {
		t.Fatalf("flows = %d, want the two streaming flows between paths", len(flows))
	}
	for _, flow := range flows {
		if !slices.Equal(flow.Source, []string{"pusher", "push", "output"}) ||
			!slices.Equal(flow.Target, []string{"cart", "roll", "input"}) || flow.Decl == nil {
			t.Errorf("flow = %+v, want pusher.push.output to cart.roll.input", flow)
		}
	}
}
