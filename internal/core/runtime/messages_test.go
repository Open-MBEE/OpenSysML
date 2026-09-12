package runtime

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// TestErrorMessages pins the text of the runtime errors a user reads: they name
// SysML concepts rather than Go types, and a recursion reports a frame count
// rather than one wrapped line per frame.
func TestErrorMessages(t *testing.T) {
	src := `
		package test {
			part def Wheel;
			calc countdown {
				in n: Integer;
				countdown(n - 1)
			}
			calc inner {
				in n: Integer;
				n / 0
			}
			calc outer {
				in n: Integer;
				inner(n)
			}
			constraint def Bounded {
				1 < 2
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	wheel := findSymbolByName(rootScope, "Wheel", ast.DefPart)
	countdown := findSymbolByName(rootScope, "countdown", ast.DefCalc)
	outer := findSymbolByName(rootScope, "outer", ast.DefCalc)
	if wheel == nil || countdown == nil || outer == nil {
		t.Fatal("fixture symbols not found")
	}

	t.Run("calc_of_wrong_kind", func(t *testing.T) {
		_, err := ctx.InvokeCalc(wheel, nil, rootScope)
		assertMessage(t, err, "not a calc: test::Wheel is a part def, not a calc definition or usage")
	})

	t.Run("constraint_of_wrong_kind", func(t *testing.T) {
		_, err := ctx.EvaluateConstraint(wheel, rootScope)
		assertMessage(t, err, "not a constraint: Wheel is a part def, not a constraint definition or usage")
	})

	t.Run("requirement_of_wrong_kind", func(t *testing.T) {
		_, err := ctx.EvaluateRequirement(wheel, rootScope)
		assertMessage(t, err, "not a requirement: Wheel is a part def, not a requirement definition or usage")
	})

	t.Run("recursion_collapses_to_a_frame_count", func(t *testing.T) {
		// A shallow depth budget, so the bound the message is about is the one
		// this recursion reaches first.
		defer func(was int64) { ctx.maxCalcDepth = was }(ctx.maxCalcDepth)
		ctx.maxCalcDepth = 8

		arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 10}}
		_, err := ctx.InvokeCalc(countdown, []Value{arg}, rootScope)
		if err == nil {
			t.Fatal("expected the recursion to be bounded")
		}
		got := err.Error()
		if !regexp.MustCompile(`^calc test::countdown: … \d+ frames: `).MatchString(got) {
			t.Errorf("err = %q; want a leading frame count", got)
		}
		if !strings.Contains(got, "calc recursion limit exceeded") {
			t.Errorf("err = %q; want the recursion limit reported", got)
		}
		if n := strings.Count(got, "calc test::countdown: "); n != 1 {
			t.Errorf("err names the calc frame %d times, want once: %q", n, got)
		}
	})
}

// TestNestedCalcNamesTheFailingCalc keeps a distinct nested calc named: only a
// calc the chain repeats is collapsed into the frame count.
func TestNestedCalcNamesTheFailingCalc(t *testing.T) {
	src := `
		package test {
			calc inner {
				in n: Integer;
				n / 0
			}
			calc outer {
				in n: Integer;
				inner(n)
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	outer := findSymbolByName(rootScope, "outer", ast.DefCalc)
	if outer == nil {
		t.Fatal("fixture symbols not found")
	}
	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	_, err := ctx.InvokeCalc(outer, []Value{arg}, rootScope)
	if err == nil {
		t.Fatal("expected the division by zero to fail")
	}
	got := err.Error()
	for _, want := range []string{"calc test::outer", "calc test::inner", "division by zero"} {
		if !strings.Contains(got, want) {
			t.Errorf("err = %q; want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "frames") {
		t.Errorf("err = %q; want no frame count for distinct calcs", got)
	}
}

// TestMutualRecursionNamesTheFrameItCollapsed pins the collapsed frame naming
// the calc it was entered for, not one an inner frame recorded.
func TestMutualRecursionNamesTheFrameItCollapsed(t *testing.T) {
	inner := calcFrame("calc", "a", errors.New("boom"))
	got := calcFrame("calc", "a", calcFrame("calc", "b", inner)).Error()
	if !strings.HasPrefix(got, "calc a: … 2 frames: ") {
		t.Errorf("err = %q; want the outer frame named a", got)
	}
}

// TestOperandTypeErrorMessage pins the type-mismatch text: the operator and both
// operand types, and the span for a surface that echoes the source.
func TestOperandTypeErrorMessage(t *testing.T) {
	err := &OperandTypeError{Op: "+", Left: "an Integer", Right: "a string"}
	assertMessage(t, err, "type mismatch: operator '+' is not defined for an Integer and a string")
	err = &OperandTypeError{Op: "<", Left: "the enumeration literal Color::red", Right: "the enumeration literal Color::blue",
		Library: "DataFunctions::'<' is abstract and no library function declares '<' for the enumeration Color, which is no ScalarValue"}
	assertMessage(t, err, "type mismatch: operator '<' is not defined for the enumeration literal Color::red and the enumeration literal Color::blue; DataFunctions::'<' is abstract and no library function declares '<' for the enumeration Color, which is no ScalarValue")
}

// assertMessage asserts err reads exactly want and names no Go type.
func assertMessage(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %q, got nil", want)
	}
	if err.Error() != want {
		t.Errorf("err = %q; want %q", err.Error(), want)
	}
	if strings.Contains(err.Error(), "*ast.") {
		t.Errorf("err names a Go type: %q", err.Error())
	}
}
