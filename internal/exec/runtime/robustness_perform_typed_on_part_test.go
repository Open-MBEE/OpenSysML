package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessPerformTypedOnPart exercises `perform action x : Def ::> part.action`:
// the typing must not make the caller perform Def itself, and a reference denoting no action is refused.
func TestRuntimeRobustnessPerformTypedOnPart(t *testing.T) {
	t.Run("reference_names_the_performer", testPerformTypedOnPartPerformer)
	t.Run("performer_holds_no_object", testPerformTypedOnPartHoldingNoObject)
	t.Run("chain_ends_in_no_action", testPerformTypedOnPartChainEndsInNoAction)
}

// testPerformTypedOnPartPerformer: the telescope, not the station, performs a
// typed perform that subsets its action, so the station's write of `azimuth`
// through the callee's `this` would fail were the station the performer.
func testPerformTypedOnPartPerformer(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, `package test {
		private import ScalarValues::*;
		part def Telescope {
			attribute azimuth : Real default = 0.0;
			action def Point {
				first start then turn;
				action turn { assign this.azimuth := 45.0; }
				first turn then done;
			}
			action point : Point;
		}
		part def Station {
			part tel : Telescope;
			action def Observe {
				first start then pt;
				perform action pt : Telescope::Point ::> tel.point;
				first pt then done;
			}
			exhibit state run {
				entry; then observing;
				state observing { entry action observe : Observe; }
				state pointed;
				transition first observing if tel.azimuth == 45.0 then pointed;
			}
		}
	}`, "test::Station")
	if err != nil {
		t.Fatalf("the typed perform did not run as the telescope: %v", err)
	}
}

// testPerformTypedOnPartHoldingNoObject: an optional part holding nothing
// performs nothing, typing or not.
func testPerformTypedOnPartHoldingNoObject(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, stationPerforming(
		"part tel : Telescope[0..1];",
		performing("perform action pt : Telescope::Point ::> tel.point;"),
	), "test::Station")
	if !errors.Is(err, ErrPerformerNotObject) || !strings.Contains(err.Error(), "tel.point is performed by [], which is no one object") {
		t.Fatalf("error = %v, want ErrPerformerNotObject over an empty tel", err)
	}
}

// testPerformTypedOnPartChainEndsInNoAction: the reference's last member is what
// is performed, so an attribute there is refused even though the typing names an action.
func testPerformTypedOnPartChainEndsInNoAction(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, stationPerforming(
		"part tel : Telescope;",
		performing("perform action pt : Telescope::Point ::> tel.azimuth;"),
	), "test::Station")
	if err == nil || !strings.Contains(err.Error(), "azimuth is not an action (attributeUsage)") {
		t.Fatalf("error = %v, want the chain's last member reported as no action", err)
	}
}
