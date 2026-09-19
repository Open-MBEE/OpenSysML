package codegen

import (
	"strings"
	"testing"
)

// The operation a library call denotes follows the interpreter's dispatch:
// kind-preserving functions keep Integer operands Integer, Integer-only
// libraries refuse a Real, and every operation spells in both targets.
func TestLibOpFor(t *testing.T) {
	ii := []Type{TypeInt, TypeInt}
	rr := []Type{TypeReal, TypeReal}
	for _, tc := range []struct {
		fqn  string
		args []Type
		op   LibOp
		why  string
	}{
		{"RealFunctions::sqrt", []Type{TypeInt}, LibSqrt, ""},
		{"RealFunctions::abs", []Type{TypeInt}, LibAbsReal, ""},
		{"RealFunctions::max", ii, LibMaxReal, ""},
		{"NumericalFunctions::abs", []Type{TypeInt}, LibAbsInt, ""},
		{"NumericalFunctions::abs", []Type{TypeReal}, LibAbsReal, ""},
		{"RationalFunctions::max", ii, LibMaxInt, ""},
		{"NumericalFunctions::min", []Type{TypeInt, TypeReal}, LibMinReal, ""},
		{"IntegerFunctions::abs", []Type{TypeReal}, 0, "requires Integer arguments"},
		{"NaturalFunctions::max", ii, LibMaxNatural, ""},
		{"NaturalFunctions::max", rr, 0, "requires Integer arguments"},
		{"TrigFunctions::tan", []Type{TypeReal}, LibTan, ""},
		{"OpenSysMLMathFunctions::log", rr, LibLog, ""},
		{"OpenSysMLMathFunctions::exp", []Type{TypeBool}, 0, "requires numeric arguments"},
		{"SequenceFunctions::size", []Type{TypeInt}, 0, "is not compiled"},
		{"RealFunctions::sum", []Type{TypeReal}, 0, "is not compiled"},
	} {
		op, why := libOpFor(tc.fqn, tc.args)
		if why != "" || tc.why != "" {
			if !strings.Contains(why, tc.why) || tc.why == "" {
				t.Errorf("%s%v: refusal %q, want %q", tc.fqn, tc.args, why, tc.why)
			}
			continue
		}
		if op != tc.op {
			t.Errorf("%s%v: op %s, want %s", tc.fqn, tc.args, op, tc.op)
		}
	}
	for op, spec := range libSpecs {
		n := len(spec.operands)
		if strings.Count(spec.c, "%s") != n || strings.Count(spec.g, "%s") != n {
			t.Errorf("%s: templates take %d operands", op, n)
		}
		if op.Result() == TypeInvalid || n == 0 {
			t.Errorf("%s: incomplete spec", op)
		}
	}
}
