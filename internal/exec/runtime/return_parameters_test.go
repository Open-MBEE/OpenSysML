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
