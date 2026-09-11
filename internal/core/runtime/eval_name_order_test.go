package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// nameOrderModel declares k at every level a bare name may resolve through:
// a body-local member, a calc parameter and a package member.
const nameOrderModel = `
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	attribute k : Integer = 100;
	calc def Local {
		in k : Integer;
		return : Integer[*] = (1, 2)->collect { in i; attribute k = 5; k * i };
	}
	calc def Param { in k : Integer; return : Integer = k; }
	calc def Outer { in n : Integer; return : Integer = n + k; }
	calc def Named { in k : Integer; out r : Integer = k; }
}
`

func wantNamedInt(t *testing.T, what string, got Value, err error, want int64) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if got.Kind != ValConst || got.Const.Kind != semantics.ValInt || got.Const.Int != want {
		t.Fatalf("%s = %s, want %d", what, FormatTraceValue(got), want)
	}
}

// TestSimpleNameShadowingOrder: a bare name answers from the innermost source
// that declares it — a body-local declaration over a bound parameter, the
// parameter over the calc's lexical scope — however the frames are stored.
func TestSimpleNameShadowingOrder(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, nameOrderModel)
	ctx := NewContext(NewModel(model, resolver), 10000)
	pkg, _ := root.LookupLocal("test")
	scope := pkg.Scope

	got, err := ctx.InvokeCalc(findSymbolByName(scope, "Local", ast.DefCalc), []Value{constInt(7)}, scope)
	if err != nil {
		t.Fatalf("Local(7): %v", err)
	}
	if ints := intsOf(t, got); !equalInts(ints, []int64{5, 10}) {
		t.Errorf("Local(7) = %v, want the body-local k = 5 over the parameter: [5 10]", ints)
	}

	got, err = ctx.InvokeCalc(findSymbolByName(scope, "Param", ast.DefCalc), []Value{constInt(7)}, scope)
	wantNamedInt(t, "Param(7)", got, err, 7)

	got, err = ctx.InvokeCalc(findSymbolByName(scope, "Outer", ast.DefCalc), []Value{constInt(1)}, scope)
	wantNamedInt(t, "Outer(1)", got, err, 101)
}

// TestSimpleNameFallbackOrder: with no body-local declaration, a frame binding
// masks an output of the calc run being computed, which masks the evaluated
// element's own features, which mask the scope's member.
func TestSimpleNameFallbackOrder(t *testing.T) {
	model, resolver, root := parseAndBuildLibraryModel(t, nameOrderModel)
	ctx := NewContext(NewModel(model, resolver), 10000)
	pkg, _ := root.LookupLocal("test")
	scope := pkg.Scope
	shape, err := ctx.calcShapeOf(findSymbolByName(scope, "Named", ast.DefCalc))
	if err != nil {
		t.Fatal(err)
	}
	// r is Named's output, so a run of it answers the bare name r once computed.
	const name = "r"
	expr := parseExpr(t, name)

	run := newCalcRun(shape, scope, nil, mapFrame(map[string]Value{}))
	run.outputs[name] = constInt(3)
	feature := scopedExpr{expr: parseExpr(t, "9"), scope: scope}

	ec := NewEvalContext(ctx, scope)
	ec.calcRun = run
	ec.features = map[string]scopedExpr{name: feature}
	ec.Push(map[string]Value{name: constInt(7)})
	got, err := ec.Eval(expr)
	wantNamedInt(t, "frame over run output and features", got, err, 7)

	ec.Pop()
	got, err = ec.Eval(expr)
	wantNamedInt(t, "run output over features", got, err, 3)

	ec.calcRun = nil
	got, err = ec.Eval(expr)
	wantNamedInt(t, "features over scope", got, err, 9)

	ec.features = nil
	if _, err = ec.Eval(expr); err == nil {
		t.Fatalf("%s resolved with every source gone; want an unresolved reference", name)
	}
}
