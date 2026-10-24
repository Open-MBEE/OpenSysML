package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestRuntimeRobustnessDefinitionOwnedCase covers a case a definition owns run
// with no object of the definition: it has nothing to be a feature of, which
// is reported as needing one, never as a body reading nothing or a panic.
func TestRuntimeRobustnessDefinitionOwnedCase(t *testing.T) {
	t.Run("verification_with_no_object_is_inconclusive", testDefinitionOwnedVerificationWithNoObject)
	t.Run("sweep_with_no_object_reports_the_need_once", testDefinitionOwnedSweepWithNoObject)
	t.Run("analysis_with_no_object_is_refused", testDefinitionOwnedAnalysisWithNoObject)
	t.Run("bad_arguments_are_refused_before_the_object", testDefinitionOwnedCaseBadArgumentsWithNoObject)
	t.Run("usage_owned_case_runs_as_before", testUsageOwnedCaseWithNoObjectHeld)
}

const definitionOwnedRobustnessModel = `package test {
	private import ScalarValues::Real;

	requirement def CostRequirement {
		subject product : Definition;
		require constraint costOk { product.cost <= 100.0 }
	}
	requirement costReq : CostRequirement;

	part def Definition {
		attribute cost : Real;
		verification costTest {
			objective costObjective { verify costReq; }
			return verdict = VerificationCases::PassIf(cost <= 100.0);
		}
		analysis costSum { out total : Real = cost; }
	}

	part holder : Definition {
		attribute :>> cost = 80.0;
		verification heldTest {
			return verdict = VerificationCases::PassIf(cost <= 100.0);
		}
	}
}`

func definitionOwnedRobustness(t *testing.T) (*symbols.Index, *Context) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, definitionOwnedRobustnessModel))
	return idx, ctx
}

// testDefinitionOwnedVerificationWithNoObject: the verdict is inconclusive and
// says what is missing, carrying no error and no object.
func testDefinitionOwnedVerificationWithNoObject(t *testing.T) {
	idx, ctx := definitionOwnedRobustness(t)
	sym := lookupOne(t, idx, "test::Definition::costTest")
	result, err := ctx.RunVerification(sym, AnalysisArgs{}, sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("RunVerification error = %v, want an inconclusive verdict", err)
	}
	v := result.Verdict
	if v.Kind != VerdictInconclusive || v.Subject != nil || v.Detail != "needs an object of test::Definition: none is instantiated" {
		t.Errorf("verdict = %s (%s) on %v, want inconclusive needing an object of the definition", v.Kind, v.Detail, v.Subject)
	}
	if len(result.Run.Outputs) != 0 || len(result.Run.Verdicts) != 0 {
		t.Errorf("a case that did not run states outputs %v and verdicts %v", result.Run.Outputs, result.Run.Verdicts)
	}
}

// testDefinitionOwnedSweepWithNoObject: a sweep over the requirement's cases
// reports the need once for the case, not an error verdict per root.
func testDefinitionOwnedSweepWithNoObject(t *testing.T) {
	idx, ctx := definitionOwnedRobustness(t)
	root := idx.DocumentRoot("<test>")
	var got []VerificationVerdict
	for _, v := range ctx.VerificationVerdictsIn([]*symbols.Scope{root}, lookupOne(t, idx, "test::costReq")) {
		if v.Case == "test::Definition::costTest" {
			got = append(got, v)
		}
	}
	if len(got) != 1 || got[0].Kind != VerdictInconclusive || !strings.Contains(got[0].Detail, "needs an object of test::Definition") {
		t.Errorf("sweep verdicts for costTest = %v, want one inconclusive verdict needing an object", got)
	}
}

// testDefinitionOwnedAnalysisWithNoObject: an analysis case the definition owns
// is refused with the typed error rather than run as a feature of nothing.
func testDefinitionOwnedAnalysisWithNoObject(t *testing.T) {
	idx, ctx := definitionOwnedRobustness(t)
	sym := lookupOne(t, idx, "test::Definition::costSum")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{}, sym.OwnerScope, nil)
	if !errors.Is(err, ErrNeedsObject) || !strings.Contains(err.Error(), "test::Definition") {
		t.Errorf("RunAnalysis with no object = %v, want ErrNeedsObject naming the definition", err)
	}
}

// testUsageOwnedCaseWithNoObjectHeld: a case a part usage owns reads the usage's
// bound values with no object held, as a usage's satisfy instantiates it.
func testUsageOwnedCaseWithNoObjectHeld(t *testing.T) {
	idx, ctx := definitionOwnedRobustness(t)
	sym := lookupOne(t, idx, "test::holder::heldTest")
	result, err := ctx.RunVerification(sym, AnalysisArgs{}, sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("RunVerification error = %v", err)
	}
	if result.Verdict.Kind != VerdictPass {
		t.Errorf("verdict = %s (%s), want pass from the usage's bound cost", result.Verdict.Kind, result.Verdict.Detail)
	}
}

// testDefinitionOwnedCaseBadArgumentsWithNoObject: an argument the case cannot
// take is refused as it is with an object, not answered with the missing one.
func testDefinitionOwnedCaseBadArgumentsWithNoObject(t *testing.T) {
	idx, ctx := definitionOwnedRobustness(t)
	sym := lookupOne(t, idx, "test::Definition::costTest")
	unknown := AnalysisArgs{Named: map[string]Value{"missing": realArg(1)}}
	if _, err := ctx.RunVerification(sym, unknown, sym.OwnerScope, nil); !errors.Is(err, errUnknownParameter) {
		t.Errorf("RunVerification(missing = 1) error = %v, want %v", err, errUnknownParameter)
	}
	extra := AnalysisArgs{Positional: []Value{realArg(1)}}
	if _, err := ctx.RunVerification(sym, extra, sym.OwnerScope, nil); !errors.Is(err, errCalcArity) {
		t.Errorf("RunVerification(1) error = %v, want %v", err, errCalcArity)
	}
	sum := lookupOne(t, idx, "test::Definition::costSum")
	if _, err := ctx.RunAnalysis(sum, unknown, sum.OwnerScope, nil); !errors.Is(err, errUnknownParameter) {
		t.Errorf("RunAnalysis(missing = 1) error = %v, want %v", err, errUnknownParameter)
	}
}
