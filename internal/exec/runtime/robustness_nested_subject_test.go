package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessNestedSubjectBinding: a nested requirement usage whose subject binding
// supplies nothing reads undetermined or a typed refusal, never a verdict.
func TestRuntimeRobustnessNestedSubjectBinding(t *testing.T) {
	t.Run("unbound_subject_stays_undetermined", testUnboundNestedSubjectStaysUndetermined)
	t.Run("subject_binding_that_cannot_be_evaluated", testNestedSubjectBindingThatCannotBeEvaluated)
}

const nestedSubjectRobustnessSrc = `
	package test {
		requirement def R1 { subject s : ScalarValues::Integer; require constraint { s > 0 } }
		part def P1 {
			attribute sub : ScalarValues::Integer = 3;
			attribute zero : ScalarValues::Integer = 0;
			requirement rqUnbound : R1 { subject s :>> s; }
			requirement rqBroken : R1 { subject s :>> s = sub / zero; }
		}
		part p1 : P1;
	}
`

// testUnboundNestedSubjectStaysUndetermined: an unbound redeclared subject reads undetermined
// at model level and uninitialized on an object, and the check refuses it.
func testUnboundNestedSubjectStaysUndetermined(t *testing.T) {
	ctx, idx := libraryShapeContext(t, nestedSubjectRobustnessSrc)
	root := idx.DocumentRoot("<test>")
	for _, read := range []string{"test::p1.rqUnbound.s", "test::p1.rqUnbound.result"} {
		val := mustEvalIn(t, ctx, root, read)
		if val.Kind != ValUndetermined {
			t.Errorf("%s = %s, want <undetermined>", read, FormatValue(val))
		}
	}

	inst := mustInstantiate(t, ctx, oneSymbol(t, idx, "test::P1"))
	if val, err := evalOn(t, ctx, inst, "rqUnbound.s"); !errors.Is(err, errUninitializedFeatureValue) {
		t.Errorf("rqUnbound.s on an object = %s, %v; want it reported uninitialized", FormatValue(val), err)
	}
	if val, err := evalOn(t, ctx, inst, "rqUnbound.result"); err != nil || val.Kind != ValUndetermined {
		t.Errorf("rqUnbound.result on an object = %s, %v; want <undetermined>", FormatValue(val), err)
	}
	sym := requirementUsage(t, idx, "test::P1::rqUnbound")
	result, err := ctx.CheckRequirementOn(sym, inst.Type.Scope, inst)
	if err == nil || violationErr(err) || !strings.Contains(err.Error(), "s") {
		t.Errorf("CheckRequirementOn(rqUnbound) = %v, %v; want the unbound subject s refused", result.Holds, err)
	}
}

// testNestedSubjectBindingThatCannotBeEvaluated: a binding whose expression fails
// is that failure on every surface, never a verdict about the model.
func testNestedSubjectBindingThatCannotBeEvaluated(t *testing.T) {
	ctx, idx := libraryShapeContext(t, nestedSubjectRobustnessSrc)
	root := idx.DocumentRoot("<test>")
	for _, read := range []string{"test::p1.rqBroken.s", "test::p1.rqBroken.result"} {
		if val, err := evalIn(t, ctx, root, read); err == nil && val.Kind == ValConst {
			t.Errorf("%s = %s, want an error or an undetermined value", read, FormatValue(val))
		}
	}
	inst := mustInstantiate(t, ctx, oneSymbol(t, idx, "test::P1"))
	for _, read := range []string{"rqBroken.s", "rqBroken.result"} {
		if val, err := evalOn(t, ctx, inst, read); err == nil {
			t.Errorf("%s on an object = %s, want the binding's evaluation error", read, FormatValue(val))
		}
	}
	sym := requirementUsage(t, idx, "test::P1::rqBroken")
	if result, err := ctx.CheckRequirementOn(sym, inst.Type.Scope, inst); err == nil || violationErr(err) {
		t.Errorf("CheckRequirementOn(rqBroken) = %v, %v; want the binding's evaluation error", result.Holds, err)
	}
}
