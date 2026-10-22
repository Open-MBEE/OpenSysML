package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A second performance of a dynamics action on a runtime whose clock the first advanced
// steps as many steps from its own bound state: stopTime and time are its own durations.
func TestStateSpacePerformedAgainIntegratesAfresh(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "conformance", "state_space_performed_twice.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, string(src)))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "fall", ast.DefAction)
	if sym == nil {
		t.Fatal("action fall not found")
	}
	var results []string
	for i := 0; i < 3; i++ {
		exec, err := ctx.CreateActionExecutor(sym)
		if err != nil {
			t.Fatalf("performance %d: %v", i+1, err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("performance %d: %v", i+1, err)
		}
		got := FormatValue(exec.Results()["stateSpace"]) + " at " + FormatValue(exec.Results()["time"])
		results = append(results, got)
		if want := float64(3 * (i + 1)); ctx.clock.now != want {
			t.Errorf("clock after performance %d = %v, want %v", i+1, ctx.clock.now, want)
		}
	}
	want := "⟨55.855000000000004, -29.429999999999996⟩ at 3.0 [s]"
	for i, got := range results {
		if got != want {
			t.Errorf("performance %d ended at %s, want %s", i+1, got, want)
		}
	}
}

// A late-starting performance reads time as its own stepped duration at each stage and
// step, not the clock's instant less its start, which rounds at microsecond steps past 1e8 s.
func TestStateSpaceTimeExactAfterLongClockAdvance(t *testing.T) {
	src := stateSpaceSource("ContinuousStateSpaceDynamics, FixedStepDynamics, RungeKutta4Integrator", `
		:>> stateSpace = VectorOf((0.0));
		:>> timeStep = 0.000001 [s];
		:>> stopTime = 0.000003 [s];
		calc :>> getDerivative {
			in input : Input;
			in stateSpace : StateSpace;
			return : StateDerivative = (time / 1 [s]) * VectorOf((1000000.0)) / 1 [s];
		}
		calc :>> getOutput {
			in input : Input;
			in stateSpace : StateSpace;
			return : Output = stateSpace;
		}
	`)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "dyn", ast.DefAction)
	if sym == nil {
		t.Fatal("action dyn not found")
	}
	perform := func() string {
		exec, err := ctx.CreateActionExecutor(sym)
		if err != nil {
			t.Fatal(err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatal(err)
		}
		return FormatValue(exec.Results()["stateSpace"]) + " at " + FormatValue(exec.Results()["time"])
	}
	first := perform()
	ctx.clock.now = 1e8
	if later := perform(); later != first {
		t.Errorf("performance from clock 1e8 ended at %s, the one from zero at %s", later, first)
	}
	if want := " at 3e-06 [s]"; !strings.HasSuffix(first, want) {
		t.Errorf("time after three microsecond steps: %s, want%s", first, want)
	}
}
