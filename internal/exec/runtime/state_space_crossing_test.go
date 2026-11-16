package runtime

import (
	"math"
	"regexp"
	"strconv"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// crossingInstants runs action test::dyn of src to completion under a trace and
// returns the instant of each zero crossing it raised, with the executor.
func crossingInstants(t *testing.T, src string) ([]float64, *ActionExecutor) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "dyn", ast.DefAction)
	if sym == nil {
		t.Fatal("action dyn not found")
	}
	tr := NewTraceRecorder()
	ctx.SetTrace(tr)
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatal(err)
	}
	var instants []float64
	for _, m := range regexp.MustCompile(`(?m)^\s*event: zero crossing \S+ of dyn \(t=([^)]+)\)$`).FindAllStringSubmatch(tr.String(), -1) {
		at, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatalf("crossing instant %q: %v", m[1], err)
		}
		instants = append(instants, at)
	}
	return instants, exec
}

// rampCalcs is x' = 1 from the state's start, so the state is the time elapsed.
const rampCalcs = `
	calc :>> getDerivative {
		in input : Input;
		in stateSpace : StateSpace;
		return : StateDerivative = VectorOf((1.0)) / 1 [s];
	}
	calc :>> getOutput {
		in input : Input;
		in stateSpace : StateSpace;
		return : Output = stateSpace;
	}
`

func expectInstants(t *testing.T, got, want []float64, within float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("crossings at %v, want %d at %v", got, len(want), want)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > within {
			t.Errorf("crossing %d at %v, want %v within %v", i+1, got[i], want[i], within)
		}
	}
}

// TestZeroCrossingLocatedWithinStep runs dynamics whose guards cross zero inside
// a step and checks each event's instant against the root, not the step's end.
func TestZeroCrossingLocatedWithinStep(t *testing.T) {
	const fixed = "ContinuousStateSpaceDynamics, FixedStepDynamics"

	t.Run("sign_change_inside_a_step", func(t *testing.T) {
		got, _ := crossingInstants(t, stateSpaceSource(fixed, `
			:>> stateSpace = VectorOf((0.0));
			:>> timeStep = 1 [s];
			:>> stopTime = 2 [s];
			event occurrence third : ZeroCrossing {
				:>> guard = stateSpace.elements#(1) - 1.3;
			}
		`+rampCalcs))
		expectInstants(t, got, []float64{1.3}, 1e-12)
	})

	// The guard is positive at both ends of the step and at every quarter of it.
	t.Run("crosses_and_recrosses_between_samples", func(t *testing.T) {
		got, _ := crossingInstants(t, stateSpaceSource(fixed, `
			:>> stateSpace = VectorOf((0.0));
			:>> timeStep = 1 [s];
			:>> stopTime = 2 [s];
			event occurrence band : ZeroCrossing {
				:>> guard = (stateSpace.elements#(1) - 0.3) * (stateSpace.elements#(1) - 0.4);
			}
		`+rampCalcs))
		expectInstants(t, got, []float64{0.3, 0.4}, 1e-12)
	})

	// The guard turns back across the instant one step ends and the next begins.
	t.Run("crosses_and_recrosses_about_a_step_end", func(t *testing.T) {
		got, _ := crossingInstants(t, stateSpaceSource(fixed, `
			:>> stateSpace = VectorOf((0.0));
			:>> timeStep = 1 [s];
			:>> stopTime = 3 [s];
			event occurrence band : ZeroCrossing {
				:>> guard = (stateSpace.elements#(1) - 0.95) * (stateSpace.elements#(1) - 1.05);
			}
		`+rampCalcs))
		expectInstants(t, got, []float64{0.95, 1.05}, 1e-12)
	})

	t.Run("guard_over_time", func(t *testing.T) {
		got, _ := crossingInstants(t, stateSpaceSource(fixed, `
			:>> stateSpace = VectorOf((0.0));
			:>> timeStep = 1 [s];
			:>> stopTime = 2 [s];
			event occurrence late : ZeroCrossing {
				:>> guard = (time - 1.7 [s]) / 1 [s];
			}
		`+rampCalcs))
		expectInstants(t, got, []float64{1.7}, 1e-12)
	})

	t.Run("later_guard_declared_first_in_one_quarter", func(t *testing.T) {
		got, _ := crossingInstants(t, stateSpaceSource(fixed, `
			:>> stateSpace = VectorOf((0.0));
			:>> timeStep = 1 [s];
			:>> stopTime = 1 [s];
			event occurrence later : ZeroCrossing {
				:>> guard = stateSpace.elements#(1) - 0.2;
			}
			event occurrence sooner : ZeroCrossing {
				:>> guard = 0.1 - stateSpace.elements#(1);
			}
		`+rampCalcs))
		expectInstants(t, got, []float64{0.1, 0.2}, 1e-12)
	})

	t.Run("exact_zero_inside_a_step_and_at_its_end", func(t *testing.T) {
		got, _ := crossingInstants(t, stateSpaceSource(fixed, `
			:>> stateSpace = VectorOf((0.0));
			:>> timeStep = 1 [s];
			:>> stopTime = 2 [s];
			event occurrence band : ZeroCrossing {
				:>> guard = (stateSpace.elements#(1) - 0.5) * (stateSpace.elements#(1) - 1.0);
			}
		`+rampCalcs))
		expectInstants(t, got, []float64{0.5, 1}, 1e-12)
	})

	// The crossing inside the step leaves the guard at an exact zero, and the step's
	// end is an exact zero again: the guard left zero in between, so it crossed twice.
	t.Run("exact_zero_inside_a_step_then_at_its_end", func(t *testing.T) {
		got, _ := crossingInstants(t, stateSpaceSource(fixed, `
			:>> stateSpace = VectorOf((0.0));
			:>> timeStep = 1 [s];
			:>> stopTime = 1 [s];
			event occurrence band : ZeroCrossing {
				:>> guard = (stateSpace.elements#(1) - 0.3) * (stateSpace.elements#(1) - 1.0);
			}
		`+rampCalcs))
		expectInstants(t, got, []float64{0.3, 1}, 1e-12)
	})

	// The run starts at an exact zero, which raises nothing, and is back at one at
	// the step's end.
	t.Run("exact_zero_at_the_start_and_at_the_step_end", func(t *testing.T) {
		got, _ := crossingInstants(t, stateSpaceSource(fixed, `
			:>> stateSpace = VectorOf((0.0));
			:>> timeStep = 0.3 [s];
			:>> stopTime = 0.3 [s];
			event occurrence back : ZeroCrossing {
				:>> guard = stateSpace.elements#(1) * (stateSpace.elements#(1) - 0.3);
			}
		`+rampCalcs))
		expectInstants(t, got, []float64{0.3}, 0)
	})

	t.Run("two_guards_in_one_step", func(t *testing.T) {
		got, _ := crossingInstants(t, stateSpaceSource(fixed, `
			:>> stateSpace = VectorOf((0.0));
			:>> timeStep = 1 [s];
			:>> stopTime = 1 [s];
			event occurrence later : ZeroCrossing {
				:>> guard = stateSpace.elements#(1) - 0.8;
			}
			event occurrence sooner : ZeroCrossing {
				:>> guard = 0.1 - stateSpace.elements#(1);
			}
		`+rampCalcs))
		expectInstants(t, got, []float64{0.1, 0.8}, 1e-12)
	})

	// x' = -x/2 from 1 reaches 1/2 at 2 ln 2; the terminal crossing leaves the state there.
	for _, c := range []struct {
		name, integrate string
		within          float64
	}{
		{"terminal_under_rk4", "", 1e-4},
		{"terminal_under_euler", ":>> integrate : Euler;", 0.2},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, exec := crossingInstants(t, stateSpaceSource(fixed, `
				:>> stateSpace = VectorOf((1.0));
				:>> timeStep = 0.5 [s];
				:>> stopTime = 4 [s];
				`+c.integrate+`
				event occurrence half : ZeroCrossing {
					:>> guard = stateSpace.elements#(1) - 0.5;
					:>> terminal = true;
				}
			`+decayCalcs))
			expectInstants(t, got, []float64{2 * math.Ln2}, c.within)
			if got[0] <= 1 || got[0] >= 1.5 {
				t.Errorf("crossing at %v, want inside the step from 1 to 1.5", got[0])
			}
			state, err := readVector("stateSpace", "stateSpace", exec.Results()["stateSpace"])
			if err != nil || len(state.num) != 1 {
				t.Fatalf("stateSpace = %s (%v)", FormatValue(exec.Results()["stateSpace"]), err)
			}
			if x := asReal(state.num[0]); x > 0.5 || 0.5-x > 1e-12 {
				t.Errorf("state at the crossing = %v, want just past 0.5", x)
			}
		})
	}

	// A getNextState the model bodies is asked for the span to the crossing as its timeStep.
	t.Run("custom_next_state", func(t *testing.T) {
		got, _ := crossingInstants(t, stateSpaceSource("ContinuousStateSpaceDynamics", `
			:>> stateSpace = VectorOf((0.0));
			attribute timeStep : DurationValue = 1 [s];
			attribute stopTime : DurationValue = 2 [s];
			event occurrence third : ZeroCrossing {
				:>> guard = stateSpace.elements#(1) - 1.3;
			}
			calc :>> getNextState {
				in input : Input;
				in stateSpace : StateSpace;
				in timeStep : DurationValue;
				return : StateSpace = stateSpace + VectorOf((1.0)) * (timeStep / 1 [s]);
			}
		`+rampCalcs))
		expectInstants(t, got, []float64{1.3}, 1e-12)
	})
}

// TestCrossingSearch pins the search over previewed samples: sign changes, turns
// towards zero with and without a crossing, and a guard that cannot be evaluated.
func TestCrossingSearch(t *testing.T) {
	spans := []float64{0, 0.25, 0.5, 0.75, 1, 1.25}
	sample := func(guard func(float64) float64) []float64 {
		guards := make([]float64, len(spans))
		for i, span := range spans {
			guards[i] = guard(span)
		}
		return guards
	}
	search := func(guard func(float64) float64, limit float64) (float64, float64, bool, bool) {
		s := crossingSearch{guard: func(span float64) (float64, bool) { return guard(span), true }, limit: limit}
		return s.first(spans, sample(guard), 0, 0)
	}

	line := func(x float64) float64 { return x - 0.6 }
	_, at, found, ok := search(line, 1)
	if !ok || !found || line(at) < 0 || line(math.Nextafter(at, 0)) >= 0 {
		t.Errorf("sign change: at %v (found %v, ok %v), want the first span at or past 0.6", at, found, ok)
	}

	band := func(x float64) float64 { return (x - 0.3) * (x - 0.31) }
	_, at, found, ok = search(band, 1)
	if !ok || !found || band(at) > 0 || band(math.Nextafter(at, 0)) <= 0 || math.Abs(at-0.3) > 1e-12 {
		t.Errorf("cross and recross: at %v (found %v, ok %v), want the first span at or past 0.3", at, found, ok)
	}

	if _, at, found, ok = search(func(x float64) float64 { return (x-0.3)*(x-0.3) + 1e-9 }, 1); !ok || found {
		t.Errorf("turn short of zero: at %v (found %v, ok %v), want no crossing", at, found, ok)
	}
	if _, at, found, ok = search(line, 0.5); !ok || found {
		t.Errorf("crossing past the limit: at %v (found %v, ok %v), want none", at, found, ok)
	}
	if _, at, found, ok = search(func(x float64) float64 { return 1 - x }, 1); !ok || !found || at != 1 {
		t.Errorf("zero at the step's end: at %v (found %v, ok %v), want 1", at, found, ok)
	}
	if _, at, found, ok = search(func(x float64) float64 { return -x }, 1); !ok || found {
		t.Errorf("leaving zero: at %v (found %v, ok %v), want no crossing", at, found, ok)
	}

	// The guard rests at zero at the latest sample, dips to the other side and is back by the next.
	back := func(x float64) float64 { return x * (x - 0.1) }
	resting := crossingSearch{guard: func(span float64) (float64, bool) { return back(span), true }, limit: 1}
	if _, at, found, ok = resting.first(spans, sample(back), 0, 1); !ok || !found || math.Abs(at-0.1) > 1e-12 {
		t.Errorf("back across zero after resting: at %v (found %v, ok %v), want 0.1", at, found, ok)
	}

	blind := crossingSearch{guard: func(float64) (float64, bool) { return 0, false }, limit: 1}
	if _, _, _, ok := blind.first(spans, sample(line), 0, 0); ok {
		t.Error("a guard that cannot be evaluated: search reported ok")
	}

	// A sample before the latest one shows a turn at the latest: only what follows is searched.
	late := func(x float64) float64 { return (x - 0.02) * (x - 0.05) }
	s := crossingSearch{guard: func(span float64) (float64, bool) { return late(span), true }, limit: 1}
	_, at, found, ok = s.first(append([]float64{-0.25}, spans...), append([]float64{late(-0.25)}, sample(late)...), 1, 0)
	if !ok || !found || math.Abs(at-0.02) > 1e-12 {
		t.Errorf("turn at the latest sample: at %v (found %v, ok %v), want 0.02", at, found, ok)
	}
}
