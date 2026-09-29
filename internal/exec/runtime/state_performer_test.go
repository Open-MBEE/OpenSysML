package runtime

import "testing"

// statePerformerFixture has two cars of one definition differing only in speed,
// each exhibiting a machine whose guards and entry actions read and write the car,
// and a truck performing the same machine definition through its `in ref` parameter.
const statePerformerFixture = `
package Road {
	private import ScalarValues::*;
	part def Vehicle {
		attribute speed : Integer default 0;
		attribute seen : Integer default -1;
		attribute shifted : Integer default -1;
	}
	state def Mode {
		in ref vehicle : Vehicle;
		entry; then idle;
		state idle;
		transition first idle if vehicle.speed > 5 then fast;
		transition first idle if vehicle.speed <= 5 then slow;
		state fast { entry assign vehicle.seen := vehicle.speed * 10; }
		state slow { entry assign vehicle.seen := vehicle.speed; }
	}
	part def Car :> Vehicle {
		exhibit state gear {
			entry; then idle;
			state idle;
			transition first idle if speed > 5 then fast;
			transition first idle if speed <= 5 then slow;
			state fast { entry assign shifted := speed * 10; }
			state slow { entry assign shifted := speed; }
		}
		exhibit state mode : Mode;
	}
	part def Truck :> Vehicle;
	part slowCar : Car { attribute :>> speed = 2; }
	part fastCar : Car { attribute :>> speed = 9; }
	part truck : Truck { attribute :>> speed = 8; }
}
`

// A state machine run on a performer evaluates its guards against the performer's
// feature values and its actions write the performer: an exhibited usage reads the
// exhibiting part's features by name, a definition reads them through the `in ref`
// parameter the performer binds, and the outcome reports them under `this.`.
func TestStateMachineRunsOnItsPerformersFeatureValues(t *testing.T) {
	for _, tc := range []struct {
		machine, performer, final, feature string
		want                               int64
	}{
		{"Road::Car::gear", "Road::slowCar", "slow", "shifted", 2},
		{"Road::Car::gear", "Road::fastCar", "fast", "shifted", 90},
		{"Road::Mode", "Road::slowCar", "slow", "seen", 2},
		{"Road::Mode", "Road::fastCar", "fast", "seen", 90},
		{"Road::Car::mode", "Road::fastCar", "fast", "seen", 90},
		{"Road::Mode", "Road::truck", "fast", "seen", 80},
	} {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, statePerformerFixture))
		self, err := ctx.Instantiate(oneSymbol(t, idx, tc.performer))
		if err != nil {
			t.Fatalf("Instantiate %s: %v", tc.performer, err)
		}
		_, visited, err := ctx.ExecuteStatePerformedBy(oneSymbol(t, idx, tc.machine), self, nil)
		if err != nil {
			t.Fatalf("%s on %s: %v", tc.machine, tc.performer, err)
		}
		if len(visited) == 0 || visited[len(visited)-1] != tc.final {
			t.Errorf("%s on %s visited %v, want to end in %s", tc.machine, tc.performer, visited, tc.final)
		}
		fv, err := self.GetFeatureValue(ctx, tc.feature)
		if err != nil {
			t.Fatalf("read %s of %s: %v", tc.feature, tc.performer, err)
		}
		if got := intOutput(t, map[string]Value{tc.feature: fv.HeldValue()}, tc.feature); got != tc.want {
			t.Errorf("%s on %s left %s = %d, want %d", tc.machine, tc.performer, tc.feature, got, tc.want)
		}

		idx, _, ctx = buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, statePerformerFixture))
		self, err = ctx.Instantiate(oneSymbol(t, idx, tc.performer))
		if err != nil {
			t.Fatalf("Instantiate %s: %v", tc.performer, err)
		}
		outcome, err := ctx.StateOutcomePerformedBy(oneSymbol(t, idx, tc.machine), self, nil)
		if err != nil {
			t.Fatalf("outcome of %s on %s: %v", tc.machine, tc.performer, err)
		}
		if outcome.FinalState != tc.final {
			t.Errorf("outcome of %s on %s rests in %q, want %s", tc.machine, tc.performer, outcome.FinalState, tc.final)
		}
		if got := intOutput(t, outcome.Outputs, "this."+tc.feature); got != tc.want {
			t.Errorf("outcome of %s on %s: this.%s = %d, want %d", tc.machine, tc.performer, tc.feature, got, tc.want)
		}
	}
}
