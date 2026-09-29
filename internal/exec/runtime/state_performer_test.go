package runtime

import (
	"strings"
	"testing"
)

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
		attribute logged : Integer default -1;
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
		action rate {
			out rated : Integer;
			first start;
			then action read { assign rated := speed * 2; assign logged := speed + 1; }
			then done;
		}
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

// An action run on a performer reads and writes the performer's features, and
// reports the performer's attributes exactly as its explored outcome does, apart
// from its output parameters; without a performer it reports none.
func TestActionRunReportsItsPerformersAttributesAsItsOutcomeDoes(t *testing.T) {
	for _, tc := range []struct {
		performer     string
		rated, logged int64
	}{
		{"Road::slowCar", 4, 3},
		{"Road::fastCar", 18, 10},
	} {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, statePerformerFixture))
		self, err := ctx.Instantiate(oneSymbol(t, idx, tc.performer))
		if err != nil {
			t.Fatalf("Instantiate %s: %v", tc.performer, err)
		}
		outputs, performer, err := ctx.ExecuteActionReportingPerformer(oneSymbol(t, idx, "Road::Car::rate"), self, nil)
		if err != nil {
			t.Fatalf("rate on %s: %v", tc.performer, err)
		}
		if got := intOutput(t, outputs, "rated"); got != tc.rated {
			t.Errorf("rate on %s: rated = %d, want %d", tc.performer, got, tc.rated)
		}
		if got := intOutput(t, performer, "this.logged"); got != tc.logged {
			t.Errorf("rate on %s: this.logged = %d, want %d", tc.performer, got, tc.logged)
		}
		for name := range outputs {
			if strings.HasPrefix(name, "this.") {
				t.Errorf("rate on %s reports %s among its outputs", tc.performer, name)
			}
		}

		idx, _, ctx = buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, statePerformerFixture))
		self, err = ctx.Instantiate(oneSymbol(t, idx, tc.performer))
		if err != nil {
			t.Fatalf("Instantiate %s: %v", tc.performer, err)
		}
		outcome, err := ctx.ActionOutcomePerformedBy(oneSymbol(t, idx, "Road::Car::rate"), self, nil)
		if err != nil {
			t.Fatalf("outcome of rate on %s: %v", tc.performer, err)
		}
		if len(outcome.Outputs) != len(outputs)+len(performer) {
			t.Errorf("outcome of rate on %s: %v, want the outputs %v and performer %v", tc.performer, outcome.Outputs, outputs, performer)
		}
		for _, run := range []map[string]Value{outputs, performer} {
			for name, value := range run {
				if got, held := outcome.Outputs[name]; !held || !valueEqual(got, value) {
					t.Errorf("outcome of rate on %s: %s = %v, want the executed %v", tc.performer, name, got, value)
				}
			}
		}
	}

	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, statePerformerFixture))
	_, performer, err := ctx.ExecuteActionReportingPerformer(oneSymbol(t, idx, "Road::Car::rate"), nil, nil)
	if err != nil {
		t.Fatalf("rate alone: %v", err)
	}
	if len(performer) != 0 {
		t.Errorf("rate alone reports %v: no object performs it", performer)
	}
}
