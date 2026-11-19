package lower

import (
	"slices"
	"testing"
)

// The streaming flows a type declares between feature paths lower with both paths, stated after
// from and to or by the ends of the body; a succession flow, ends owning no feature and a binding are none.
func TestToFeatureFlows(t *testing.T) {
	src := `package test {
		part def Context {
			part pusher { perform action push { out output; } }
			part cart { perform action roll { in input; } }
			flow pusher.push.output to cart.roll.input;
			flow named from pusher.push.output to cart.roll.input;
			succession flow pusher.push.output to cart.roll.input;
			flow { end ::> pusher; end ::> cart; }
			flow { end ::> pusher.push.output; end ::> cart.roll.input; }
			flow { end ::> pusher.push { ref :>> output; } end ::> cart.roll { ref :>> input; } }
			flow of ScalarValues::Real { end ::> pusher.push { ref :>> output; } end ::> cart.roll { ref :>> input; } }
			flow {
				end redefines Transfers::Transfer::source references pusher.push {
					ref redefines Transfers::Transfer::source::sourceOutput, output;
				}
				end references cart.roll redefines Transfers::Transfer::target {
					ref redefines Transfers::Transfer::target::targetInput, input;
				}
			}
			bind pusher.push.output = cart.roll.input;
		}
	}`
	sym, _, _ := stateSpaceAction(t, src, "Context")
	flows := ToFeatureFlows(sym.Decl, sym.Scope)
	if len(flows) != 5 {
		t.Fatalf("flows = %d, want the five streaming flows between paths", len(flows))
	}
	for _, flow := range flows {
		if !slices.Equal(flow.Source, []string{"pusher", "push", "output"}) ||
			!slices.Equal(flow.Target, []string{"cart", "roll", "input"}) || flow.Decl == nil {
			t.Errorf("flow = %+v, want pusher.push.output to cart.roll.input", flow)
		}
	}
}
