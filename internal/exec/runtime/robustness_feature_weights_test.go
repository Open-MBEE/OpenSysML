package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// TestRuntimeRobustnessFeatureWeights exercises Probability weights read from
// features rather than written as constants: a weight of no numeric type is
// refused before the run, and the checks lowering makes of constants — the
// range and the sum to 1 — are made of the values read when the decision is
// reached, each a typed ErrBranchWeights, never a silent renormalization.
func TestRuntimeRobustnessFeatureWeights(t *testing.T) {
	t.Run("weight_of_no_numeric_type_is_refused_before_the_run", testFeatureWeightOfNoNumericTypeIsRefusedBeforeTheRun)
	t.Run("weights_read_must_sum_to_one", testFeatureWeightsReadMustSumToOne)
	t.Run("weight_read_outside_the_unit_interval_is_refused", testFeatureWeightReadOutsideTheUnitIntervalIsRefused)
	t.Run("weight_read_from_the_performer_is_refused_out_of_range", testFeatureWeightReadFromThePerformerIsRefusedOutOfRange)
}

// testFeatureWeightOfNoNumericTypeIsRefusedBeforeTheRun: a p bound to a String
// or Boolean feature is a type error of the model, reported where it is written.
func testFeatureWeightOfNoNumericTypeIsRefusedBeforeTheRun(t *testing.T) {
	for _, tc := range []struct{ typ, value, want string }{
		{"String", `"often"`, "cannot bind String value to a feature typed by Real"},
		{"Boolean", "true", "cannot bind Boolean value to a feature typed by Real"},
	} {
		src := `package test {
			private import ScalarValues::*;
			private import Stochastic::*;
			action route {
				attribute w : ` + tc.typ + ` = ` + tc.value + `;
				first start; then decide select;
				first select then fast { @Probability { p = w; } }
				first select then slow { @Probability { p = 0.5; } }
				action fast; then done;
				action slow; then done;
			}
		}`
		file := parseAndBuild(t, src)
		idx := libs.NewModelIndex()
		idx.AddDocument("<test>", file)
		idx.ExpandWildcardImports()
		var refusals []string
		for _, d := range passes.Analyze("<test>", file, nil, idx) {
			if d.Severity == diag.SeverityError {
				refusals = append(refusals, d.Message)
			}
		}
		if len(refusals) != 1 || !strings.Contains(refusals[0], tc.want) {
			t.Errorf("%s weight: errors %v, want one saying %q", tc.typ, refusals, tc.want)
		}
	}
}

// testFeatureWeightsReadMustSumToOne: the weights read out of one decision are
// its whole distribution, a guard excluding a branch or not: 0.6 and 0.6 are
// refused for their sum, as constants are by lowering, whereas 0.6, 0.3 and 0.1
// with the 0.3 branch guarded out leave the other two renormalized.
func testFeatureWeightsReadMustSumToOne(t *testing.T) {
	err := weightedActionError(t, `
		attribute w : Real = 0.6;
		first start; then decide select;
		first select then fast { @Probability { p = w; } }
		first select then slow { @Probability { p = w; } }
		action fast; then done;
		action slow; then done;`, 3)
	if !errors.Is(err, ErrBranchWeights) || !strings.Contains(err.Error(), "sum to 1.2, not 1.0") {
		t.Fatalf("error = %v, want ErrBranchWeights naming the sum 1.2", err)
	}
	err = weightedActionError(t, `
		attribute w : Real = 0.6;
		attribute ready : Boolean = false;
		first start; then decide select;
		first select then fast { @Probability { p = w; } }
		first select if ready then slow { @Probability { p = w / 2.0; } }
		first select then steady { @Probability { p = 1.0 - w - w / 2.0; } }
		action fast; then done;
		action slow; then done;
		action steady; then done;`, 3)
	if err != nil {
		t.Fatalf("a guard excluding a branch of a distribution summing to 1: %v, want the rest renormalized", err)
	}
	err = weightedActionError(t, `
		attribute w : Real = 0.6;
		attribute ready : Boolean = false;
		first start; then decide select;
		first select then fast { @Probability { p = w; } }
		first select if ready then slow { @Probability { p = w; } }
		first select then steady { @Probability { p = 1.0 - w; } }
		action fast; then done;
		action slow; then done;
		action steady; then done;`, 3)
	if !errors.Is(err, ErrBranchWeights) || !strings.Contains(err.Error(), "sum to 1.6, not 1.0") {
		t.Fatalf("a guarded-out branch still counts in the sum: error = %v, want ErrBranchWeights naming 1.6", err)
	}
}

// testFeatureWeightReadOutsideTheUnitIntervalIsRefused: a weight read as 1.3 or
// -0.3 is refused when the decision is reached, naming the branch and the value.
func testFeatureWeightReadOutsideTheUnitIntervalIsRefused(t *testing.T) {
	for _, tc := range []struct{ w, want string }{
		{"1.3", "branch 0 weighs 1.3"},
		{"-0.3", "branch 0 weighs -0.3"},
	} {
		err := weightedActionError(t, `
			attribute w : Real = `+tc.w+`;
			first start; then decide select;
			first select then fast { @Probability { p = w; } }
			first select then slow { @Probability { p = 1.0 - w; } }
			action fast; then done;
			action slow; then done;`, 3)
		if !errors.Is(err, ErrBranchWeights) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("w = %s: error = %v, want ErrBranchWeights saying %q", tc.w, err, tc.want)
		}
	}
}

// testFeatureWeightReadFromThePerformerIsRefusedOutOfRange: an action def
// nested in a part def weighs its branches by the part's feature as the performer
// redefines it: a performer weighing 1.0 or 0.0 decides the branch, and one whose
// redefinition puts the weight outside [0, 1] is refused when the decision is reached.
func testFeatureWeightReadFromThePerformerIsRefusedOutOfRange(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		private import Stochastic::*;
		part def Analysis {
			attribute pFast : Real default = 0.5;
			action def Route {
				attribute taken : Integer = 0;
				first start; then decide select;
				first select then fast { @Probability { p = pFast; } }
				first select then slow { @Probability { p = 1.0 - pFast; } }
				action fast { assign taken := 1; } then done;
				action slow { assign taken := 2; } then done;
			}
		}
		part def Fast :> Analysis { attribute :>> pFast = 1.0; }
		part def Slow :> Analysis { attribute :>> pFast = 0.0; }
		part def Over :> Analysis { attribute :>> pFast = 1.5; }
	}`
	m := parseLibraryModel(t, src)
	root := m.idx.DocumentRoot(m.path)
	route := namedOrFoundSymbol(t, m.idx, "test::Analysis::Route", root, ast.DefAction, ast.UsageAction)
	for _, tc := range []struct {
		performer string
		taken     int64
	}{{"Fast", 1}, {"Slow", 2}} {
		ctx, _ := m.fresh()
		ctx.SetModelSeed(3)
		performer, err := ctx.Instantiate(findSymbolByName(root, tc.performer, ast.DefPart))
		if err != nil {
			t.Fatal(err)
		}
		out, err := ctx.ExecuteActionPerformedBy(route, performer, nil)
		if err != nil {
			t.Fatalf("on %s: %v", tc.performer, err)
		}
		if got := takenInt(t, out, "taken"); got != tc.taken {
			t.Errorf("taken = %d on %s, want %d, the branch its pFast weighs", got, tc.performer, tc.taken)
		}
	}
	ctx, _ := m.fresh()
	ctx.SetModelSeed(3)
	over, err := ctx.Instantiate(findSymbolByName(root, "Over", ast.DefPart))
	if err != nil {
		t.Fatal(err)
	}
	_, err = ctx.ExecuteActionPerformedBy(route, over, nil)
	if !errors.Is(err, ErrBranchWeights) || !strings.Contains(err.Error(), "branch 0 weighs 1.5") {
		t.Fatalf("on Over: error = %v, want ErrBranchWeights naming the weight 1.5", err)
	}
}
