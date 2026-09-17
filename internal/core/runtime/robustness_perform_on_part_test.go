package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessPerformOnPart exercises the failure modes of a
// `perform action x ::> part.action` whose operand denotes no one object to
// perform the action, one destroyed already, or whose chain ends in no action.
func TestRuntimeRobustnessPerformOnPart(t *testing.T) {
	t.Run("performer_holds_no_object", testPerformOnPartHoldingNoObject)
	t.Run("performer_holds_several_objects", testPerformOnPartHoldingSeveralObjects)
	t.Run("performer_was_destroyed", testPerformOnPartDestroyed)
	t.Run("chain_ends_in_no_action", testPerformOnPartChainEndsInNoAction)
}

// stationPerforming is a station whose exhibited machine performs Observe on
// entry, with the given telescope part and the given body of Observe.
func stationPerforming(part, body string) string {
	return `package test {
		private import ScalarValues::*;
		private import OccurrenceFunctions::*;
		part def Telescope {
			attribute azimuth : Real default = 0.0;
			action def Point;
			action point : Point;
		}
		part def Station {
			` + part + `
			action def Observe {
				` + body + `
			}
			exhibit state run {
				entry; then observing;
				state observing { entry action observe : Observe; }
			}
		}
	}`
}

// performing is the body of Observe performing `pt` alone, as given.
func performing(pt string) string {
	return "first start then pt; " + pt + " first pt then done;"
}

// testPerformOnPartHoldingNoObject: an optional part holding nothing performs
// nothing; the run reports that instead of running the action as the caller.
func testPerformOnPartHoldingNoObject(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, stationPerforming(
		"part tel : Telescope[0..1];",
		performing("perform action pt ::> tel.point;"),
	), "test::Station")
	if !errors.Is(err, ErrPerformerNotObject) || !strings.Contains(err.Error(), "tel.point is performed by [], which is no one object") {
		t.Fatalf("error = %v, want ErrPerformerNotObject over an empty tel", err)
	}
}

// testPerformOnPartHoldingSeveralObjects: a part holding two objects names no
// single performer, so the run refuses rather than picking one.
func testPerformOnPartHoldingSeveralObjects(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, stationPerforming(
		"part tels : Telescope[2];",
		performing("perform action pt ::> tels.point;"),
	), "test::Station")
	if !errors.Is(err, ErrPerformerNotObject) || !strings.Contains(err.Error(), "tels.point is performed by [instance(") {
		t.Fatalf("error = %v, want ErrPerformerNotObject over two telescopes", err)
	}
}

// testPerformOnPartDestroyed: a part whose object was destroyed before the
// performance performs nothing; the run says the object ended, not that it ran.
func testPerformOnPartDestroyed(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, stationPerforming(
		"part tel : Telescope;",
		`first start then gone;
		action gone { assign tel := destroy(tel); }
		first gone then pt;
		perform action pt ::> tel.point;
		first pt then done;`,
	), "test::Station")
	if !errors.Is(err, ErrOccurrenceDestroyed) || !strings.Contains(err.Error(), "(Telescope) was destroyed at") {
		t.Fatalf("error = %v, want ErrOccurrenceDestroyed over the destroyed telescope", err)
	}
}

// testPerformOnPartChainEndsInNoAction: a chain ending in an attribute of the
// part performs nothing and says what the member is.
func testPerformOnPartChainEndsInNoAction(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, stationPerforming(
		"part tel : Telescope;",
		performing("perform action pt ::> tel.azimuth;"),
	), "test::Station")
	if err == nil || !strings.Contains(err.Error(), "azimuth is not an action (attributeUsage)") {
		t.Fatalf("error = %v, want the chain's last member reported as no action", err)
	}
}
