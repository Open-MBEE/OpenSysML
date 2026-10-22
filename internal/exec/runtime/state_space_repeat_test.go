package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A second performance of a dynamics action on a runtime whose clock the first
// advanced steps the same number of steps from its own bound state: stopTime and
// time are durations from the performance's start, not instants of the clock.
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
