package runtime

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// TestStateSpaceIntegratorsAgainstClosedForm runs the first-order decay fixtures
// and compares each integrator's x(2) with e^-1: RK4 lands within its
// fourth-order error, Euler a first-order error away, visibly worse.
func TestStateSpaceIntegratorsAgainstClosedForm(t *testing.T) {
	const exact = 1 / math.E
	rk4 := stateSpaceFinalState(t, "state_space_rk4_first_order.sysml")
	euler := stateSpaceFinalState(t, "state_space_euler_first_order.sysml")

	if err := math.Abs(rk4 - exact); err > 1e-6 {
		t.Errorf("RK4 x(2) = %v, %v from e^-1; want within 1e-6", rk4, err)
	}
	eulerErr := math.Abs(euler - exact)
	if eulerErr < 1e-3 {
		t.Errorf("Euler x(2) = %v, only %v from e^-1; a step of 0.1 should err by about 1e-2", euler, eulerErr)
	}
	if eulerErr <= math.Abs(rk4-exact) {
		t.Errorf("Euler's error %v is not worse than RK4's %v", eulerErr, math.Abs(rk4-exact))
	}
}

// stateSpaceFinalState runs the conformance fixture's action test::decay to
// completion and returns the one component of its final state.
func stateSpaceFinalState(t *testing.T, fixture string) float64 {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", "conformance", fixture))
	if err != nil {
		t.Fatal(err)
	}
	idx, _, ctx := buildRuntimeWithLibraries(t, fixture, parseAndBuild(t, string(src)))
	sym := findSymbolByName(idx.DocumentRoot(fixture), "decay", ast.DefAction)
	if sym == nil {
		t.Fatalf("%s: action decay not found", fixture)
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("%s: %v", fixture, err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("%s: %v", fixture, err)
	}
	state := exec.Results()["stateSpace"]
	vq := state.VectorQuantity()
	if vq == nil || len(vq.Num) != 1 || vq.Num[0].Kind != semantics.ValReal {
		t.Fatalf("%s: stateSpace = %s, want a one-component real vector quantity", fixture, FormatValue(state))
	}
	return vq.Num[0].Real
}
