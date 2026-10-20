package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestReturnParameterReadsAgreeWithChecks reads the library result parameters of
// requirement and verification objects and compares each with the verdict the
// check surfaces already report for the same element: a package-level usage's
// `result` is its requirement check, a satisfaction assertion's verdict is the
// `result` of the requirement bound to the same subject, and a verification
// usage's `verdict` is the verdict its run produces, and a constraint usage read
// on its object is the constraint check on that object.
func TestReturnParameterReadsAgreeWithChecks(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "conformance", "read_return_parameters.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, idx := libraryShapeContext(t, string(src))
	root := idx.DocumentRoot("<test>")

	for _, usage := range []string{"test::r1", "test::r3"} {
		sym := requirementUsage(t, idx, usage)
		holds, err := ctx.EvaluateRequirementOn(sym, root, nil)
		if err != nil && !violationErr(err) {
			t.Fatalf("EvaluateRequirementOn(%s): %v", usage, err)
		}
		read := mustEvalIn(t, ctx, root, usage+".result")
		if read.Kind != ValConst || read.Const.Bool != holds {
			t.Errorf("%s.result = %s, want the requirement check's %v", usage, FormatValue(read), holds)
		}
	}

	for _, usage := range []string{"c", "c3"} {
		sym := oneSymbol(t, idx, "test::P3::"+usage)
		holds, err := ctx.EvaluateConstraintOn(sym, root, nil)
		if err != nil && !violationErr(err) {
			t.Fatalf("EvaluateConstraintOn(%s): %v", usage, err)
		}
		read := mustEvalIn(t, ctx, root, "test::p3."+usage)
		if read.Kind != ValConst || read.Const.Bool != holds {
			t.Errorf("test::p3.%s = %s, want the constraint check's %v", usage, FormatValue(read), holds)
		}
	}
	satisfied := map[string]bool{}
	for _, a := range ctx.SatisfyAssertionsIn(oneSymbol(t, idx, "test::Checks").Scope) {
		holds, err := ctx.EvaluateSatisfaction(a)
		if err != nil && !violationErr(err) {
			t.Fatalf("EvaluateSatisfaction(%s): %v", a.Text(), err)
		}
		satisfied[a.Text()] = holds
	}
	for text, usage := range map[string]string{"satisfy r1 by p2": "test::r1", "satisfy r3 by p2": "test::r3"} {
		holds, ok := satisfied[text]
		if !ok {
			t.Fatalf("no assertion %q among %v", text, satisfied)
		}
		read := mustEvalIn(t, ctx, root, usage+".result")
		if read.Kind != ValConst || read.Const.Bool != holds {
			t.Errorf("%s.result = %s, want the satisfaction's %v", usage, FormatValue(read), holds)
		}
	}

	for _, usage := range []string{"test::v1", "test::v3"} {
		sym := oneSymbol(t, idx, usage)
		result, err := ctx.RunVerification(sym, AnalysisArgs{}, root, nil)
		if err != nil {
			t.Fatalf("RunVerification(%s): %v", usage, err)
		}
		read := mustEvalIn(t, ctx, root, usage+".verdict")
		if kind, ok := ctx.verdictOf(read); !ok || kind != result.Verdict.Kind {
			t.Errorf("%s.verdict = %s, want the run's %s", usage, FormatValue(read), result.Verdict.Kind)
		}
	}
}

// requirementUsage is the requirement usage named fqn, not the satisfaction
// assertion that names it (`satisfy requirement r1 by p2` declares an `r1` too).
func requirementUsage(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	for _, sym := range idx.LookupQualified(fqn) {
		if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageRequirement {
			return sym
		}
	}
	t.Fatalf("%s: no requirement usage", fqn)
	return nil
}

func violationErr(err error) bool {
	var violation *ViolationError
	return errors.As(err, &violation)
}

// TestReturnParameterReadsAgreeWithChecksOnNestedUsages pins that a requirement
// usage a definition owns, its subject bound from the owning object's features
// (`subject s :>> s = sub;`, `subject s = sub;`, `subject = sub;`, a sibling part, a
// satisfy usage's `by`), reads the same `result` on an object of the definition as
// the requirement check on that object computes.
func TestReturnParameterReadsAgreeWithChecksOnNestedUsages(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "conformance", "nested_requirement_subject_binding.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, idx := libraryShapeContext(t, string(src))
	root := idx.DocumentRoot("<test>")
	inst := mustInstantiate(t, ctx, oneSymbol(t, idx, "test::P1"))

	for _, usage := range []string{"rq", "rqNamed", "rqBare", "rqPart", "rqFail", "rqNarrow"} {
		sym := requirementUsage(t, idx, "test::P1::"+usage)
		result, err := ctx.CheckRequirementOn(sym, inst.Type.Scope, inst)
		if err != nil && !violationErr(err) {
			t.Fatalf("CheckRequirementOn(%s): %v", usage, err)
		}
		read := mustEvalOn(t, ctx, inst, usage+".result")
		if read.Kind != ValConst || read.Const.Bool != result.Holds {
			t.Errorf("%s.result on an object = %s, want the requirement check's %v", usage, FormatValue(read), result.Holds)
		}
		modelRead := mustEvalIn(t, ctx, root, "test::p1."+usage+".result")
		if modelRead.Kind != ValConst || modelRead.Const.Bool != result.Holds {
			t.Errorf("test::p1.%s.result = %s, want the requirement check's %v", usage, FormatValue(modelRead), result.Holds)
		}
	}

	assertions := ctx.SatisfyAssertionsIn(oneSymbol(t, idx, "test::P1").Scope)
	if len(assertions) != 1 {
		t.Fatalf("found %d satisfaction assertions in P1, want 1", len(assertions))
	}
	pa, err := inst.GetFeatureValue(ctx, "pa")
	if err != nil {
		t.Fatal(err)
	}
	result, err := ctx.CheckSatisfactionOn(assertions[0], ctx.instances[pa.HeldValue().Instance])
	if err != nil && !violationErr(err) {
		t.Fatalf("CheckSatisfactionOn(%s): %v", assertions[0].Text(), err)
	}
	if read := mustEvalOn(t, ctx, inst, "sat.result"); read.Kind != ValConst || read.Const.Bool != result.Holds {
		t.Errorf("sat.result = %s, want the satisfaction's %v", FormatValue(read), result.Holds)
	}

	rqPkg := requirementUsage(t, idx, "test::rqPkg")
	holds, err := ctx.EvaluateRequirementOn(rqPkg, root, nil)
	if err != nil && !violationErr(err) {
		t.Fatalf("EvaluateRequirementOn(rqPkg): %v", err)
	}
	if read := mustEvalIn(t, ctx, root, "test::rqPkg.result"); read.Kind != ValConst || read.Const.Bool != holds {
		t.Errorf("test::rqPkg.result = %s, want the requirement check's %v", FormatValue(read), holds)
	}
	for _, a := range ctx.SatisfyAssertionsIn(oneSymbol(t, idx, "test").Scope) {
		holds, err := ctx.EvaluateSatisfaction(a)
		if err != nil && !violationErr(err) {
			t.Fatalf("EvaluateSatisfaction(%s): %v", a.Text(), err)
		}
		if read := mustEvalIn(t, ctx, root, "test::satPkg.result"); read.Kind != ValConst || read.Const.Bool != holds {
			t.Errorf("test::satPkg.result = %s, want the satisfaction's %v", FormatValue(read), holds)
		}
	}
}
