package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// The function form of each operator answers what the operator notation does.
func TestOperatorFunctionValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want Value
	}{
		{"IntegerFunctions::+", []Value{constInt(1), constInt(2)}, constInt(3)},
		{"IntegerFunctions::+", []Value{constInt(5)}, constInt(5)},
		{"IntegerFunctions::-", []Value{constInt(5)}, constInt(-5)},
		{"IntegerFunctions::-", []Value{constInt(5), constInt(7)}, constInt(-2)},
		{"IntegerFunctions::*", []Value{constInt(3), constInt(4)}, constInt(12)},
		{"IntegerFunctions::/", []Value{constInt(1), constInt(4)}, constReal(0.25)},
		{"IntegerFunctions::%", []Value{constInt(7), constInt(3)}, constInt(1)},
		{"IntegerFunctions::**", []Value{constInt(2), constInt(10)}, constInt(1024)},
		{"IntegerFunctions::^", []Value{constInt(2), constInt(0)}, constInt(1)},
		{"IntegerFunctions::<", []Value{constInt(1), constInt(2)}, constBool(true)},
		{"IntegerFunctions::>=", []Value{constInt(1), constInt(2)}, constBool(false)},
		{"IntegerFunctions::==", []Value{constInt(2), constInt(2)}, constBool(true)},
		{"RealFunctions::==", []Value{constInt(2), constReal(2)}, constBool(true)},
		{"RationalFunctions::==", []Value{constReal(0.5), constReal(0.5)}, constBool(true)},
		{"NaturalFunctions::==", []Value{constInt(2), constInt(3)}, constBool(false)},
		{"NaturalFunctions::==", []Value{constInt(2), nullValue()}, constBool(false)},
		{"IntegerFunctions::==", nil, constBool(true)},
		{"IntegerFunctions::==", []Value{constInt(2)}, constBool(false)},
		{"IntegerFunctions::==", []Value{emptySequence(), nullValue()}, constBool(true)},
		{"BaseFunctions::==", []Value{emptySequence(), nullValue()}, constBool(true)},
		{"BaseFunctions::==", []Value{nullValue(), NewSetValue(NewSet())}, constBool(true)},
		{"BaseFunctions::==", []Value{emptySequence(), constInt(0)}, constBool(false)},
		{"BaseFunctions::!=", []Value{emptySequence(), nullValue()}, constBool(false)},
		{"BaseFunctions::===", []Value{nullValue(), emptySequence()}, constBool(true)},
		{"BaseFunctions::!==", []Value{emptySequence(), NewStringValue("")}, constBool(true)},
		{"DataFunctions::==", []Value{emptySequence(), nullValue()}, constBool(true)},
		{"DataFunctions::===", []Value{emptySequence(), nullValue()}, constBool(true)},
		{"NaturalFunctions::+", []Value{constInt(1), constInt(2)}, constInt(3)},
		{"NaturalFunctions::/", []Value{constInt(6), constInt(3)}, constInt(2)},
		{"NaturalFunctions::/", []Value{constInt(0), constInt(3)}, constInt(0)},
		{"RealFunctions::+", []Value{constReal(1.5), constInt(2)}, constReal(3.5)},
		{"RealFunctions::+", []Value{constInt(1), constInt(2)}, constReal(3)},
		{"RealFunctions::+", []Value{constInt(5)}, constReal(5)},
		{"RealFunctions::-", []Value{constInt(5)}, constReal(-5)},
		{"RealFunctions::-", []Value{constInt(5), constInt(7)}, constReal(-2)},
		{"RealFunctions::*", []Value{constInt(3), constInt(4)}, constReal(12)},
		{"RealFunctions::*", []Value{constInt(1 << 40), constInt(1 << 40)}, constReal(1 << 80)},
		{"RealFunctions::/", []Value{constInt(6), constInt(3)}, constReal(2)},
		{"RealFunctions::**", []Value{constInt(2), constInt(3)}, constReal(8)},
		{"RealFunctions::^", []Value{constInt(2), constInt(-1)}, constReal(0.5)},
		{"RealFunctions::<", []Value{constInt(1), constInt(2)}, constBool(true)},
		{"RealFunctions::==", []Value{constInt(2), constInt(2)}, constBool(true)},
		{"RealFunctions::-", []Value{constReal(1.5)}, constReal(-1.5)},
		{"RationalFunctions::+", []Value{constInt(1), constInt(2)}, constInt(3)},
		{"RationalFunctions::/", []Value{constInt(1), constInt(4)}, constReal(0.25)},
		{"RealFunctions::**", []Value{constReal(2), constReal(-1)}, constReal(0.5)},
		{"RealFunctions::<=", []Value{constReal(2), constInt(2)}, constBool(true)},
		{"RationalFunctions::*", []Value{constReal(0.5), constReal(0.5)}, constReal(0.25)},
		{"RationalFunctions::>", []Value{constReal(0.5), constReal(0.25)}, constBool(true)},
		{"NumericalFunctions::+", []Value{constInt(1), constInt(2)}, constInt(3)},
		{"NumericalFunctions::%", []Value{constReal(7.5), constInt(2)}, constReal(1.5)},
		{"NumericalFunctions::*", []Value{cx(0, 1), cx(0, 1)}, cx(-1, 0)},
		{"ScalarFunctions::+", []Value{NewStringValue("a"), NewStringValue("b")}, NewStringValue("ab")},
		{"ScalarFunctions::<", []Value{NewStringValue("a"), NewStringValue("b")}, constBool(true)},
		{"DataFunctions::+", []Value{constInt(1), constReal(0.5)}, constReal(1.5)},
		{"DataFunctions::==", []Value{NewStringValue("a"), NewStringValue("a")}, constBool(true)},
		{"DataFunctions::===", []Value{constInt(2), constReal(2)}, constBool(false)},
		{"DataFunctions::===", []Value{constInt(2), constInt(2)}, constBool(true)},
		{"BaseFunctions::==", []Value{constInt(2), constReal(2)}, constBool(true)},
		{"BaseFunctions::!=", []Value{constInt(1), constInt(2)}, constBool(true)},
		{"BaseFunctions::!=", nil, constBool(false)},
		{"BaseFunctions::===", []Value{NewStringValue("x"), NewStringValue("x")}, constBool(true)},
		{"BaseFunctions::!==", []Value{constInt(2), constReal(2)}, constBool(true)},
		{"BooleanFunctions::not", []Value{constBool(true)}, constBool(false)},
		{"BooleanFunctions::xor", []Value{constBool(true), constBool(false)}, constBool(true)},
		{"BooleanFunctions::xor", []Value{constBool(true), constBool(true)}, constBool(false)},
		{"BooleanFunctions::|", []Value{constBool(false), constBool(true)}, constBool(true)},
		{"BooleanFunctions::&", []Value{constBool(false), constBool(true)}, constBool(false)},
		{"BooleanFunctions::==", []Value{constBool(true), constBool(true)}, constBool(true)},
		{"DataFunctions::not", []Value{constBool(false)}, constBool(true)},
		{"ScalarFunctions::xor", []Value{constBool(false), constBool(true)}, constBool(true)},
		{"DataFunctions::max", []Value{constInt(2), constInt(7)}, constInt(7)},
		{"DataFunctions::max", []Value{constInt(2), constReal(7.5)}, constReal(7.5)},
		{"DataFunctions::min", []Value{constReal(-1), constInt(3)}, constReal(-1)},
		{"DataFunctions::max", []Value{NewStringValue("apple"), NewStringValue("pear")}, NewStringValue("pear")},
		{"DataFunctions::min", []Value{NewStringValue("apple"), NewStringValue("pear")}, NewStringValue("apple")},
		{"ScalarFunctions::max", []Value{NewStringValue("b"), NewStringValue("b")}, NewStringValue("b")},
		{"ScalarFunctions::min", []Value{constInt(4), constInt(4)}, constInt(4)},
	}
	for _, tc := range cases {
		name := tc.fn
		if len(tc.args) > 0 {
			name += "/" + FormatValue(tc.args[0])
		}
		t.Run(name, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			if !valueIdentical(got, tc.want) {
				t.Fatalf("%s = %s (%v), want %s (%v)", tc.fn, FormatValue(got), got.Kind, FormatValue(tc.want), tc.want.Kind)
			}
		})
	}
}

// Operator functions declare their operands `[1]`: one element stands for itself;
// none or several is a multiplicity violation, judged before the operand domain.
func TestOperatorFunctionOperandMultiplicity(t *testing.T) {
	one := vec(constInt(3))
	oneSet := NewSetValue(NewSet())
	oneSet.Set().Add(constInt(3))
	none := emptySequence()
	several := vec(constInt(1), constInt(2))

	values := []struct {
		fn   string
		args []Value
		want Value
	}{
		{"IntegerFunctions::+", []Value{one, constInt(2)}, constInt(5)},
		{"IntegerFunctions::+", []Value{constInt(2), oneSet}, constInt(5)},
		{"IntegerFunctions::-", []Value{one}, constInt(-3)},
		{"IntegerFunctions::-", []Value{one, none}, constInt(-3)},
		{"IntegerFunctions::<", []Value{one, vec(constInt(4))}, constBool(true)},
		{"NaturalFunctions::/", []Value{vec(constInt(6)), one}, constInt(2)},
		{"RealFunctions::*", []Value{one, vec(constReal(0.5))}, constReal(1.5)},
		{"BooleanFunctions::not", []Value{vec(constBool(true))}, constBool(false)},
		{"BooleanFunctions::xor", []Value{vec(constBool(true)), constBool(false)}, constBool(true)},
		{"BooleanFunctions::&", []Value{vec(constBool(true)), vec(constBool(true))}, constBool(true)},
		{"DataFunctions::max", []Value{one, vec(constInt(7))}, constInt(7)},
		{"ScalarFunctions::min", []Value{vec(NewStringValue("b")), NewStringValue("a")}, NewStringValue("a")},
	}
	for _, tc := range values {
		t.Run(tc.fn+"/"+FormatValue(tc.args[0]), func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			if !valueIdentical(got, tc.want) {
				t.Fatalf("%s = %s, want %s", tc.fn, FormatValue(got), FormatValue(tc.want))
			}
		})
	}

	faults := []struct {
		fn    string
		args  []Value
		param string
	}{
		{"IntegerFunctions::+", []Value{none, constInt(2)}, "x"},
		{"IntegerFunctions::+", []Value{constInt(2), several}, "y"},
		{"IntegerFunctions::+", []Value{several, NewStringValue("a")}, "x"},
		{"IntegerFunctions::-", []Value{none}, "x"},
		{"IntegerFunctions::-", []Value{several}, "x"},
		{"IntegerFunctions::<", []Value{constInt(1), none}, "y"},
		{"RealFunctions::<=", []Value{several, constReal(2)}, "x"},
		{"NaturalFunctions::/", []Value{several, constInt(2)}, "x"},
		{"NaturalFunctions::/", []Value{constInt(6), none}, "y"},
		{"BooleanFunctions::not", []Value{none}, "x"},
		{"BooleanFunctions::not", []Value{vec(constBool(true), constBool(false))}, "x"},
		{"BooleanFunctions::xor", []Value{constBool(true), none}, "y"},
		{"BooleanFunctions::|", []Value{several, constBool(true)}, "x"},
		{"DataFunctions::max", []Value{none, constInt(1)}, "x"},
		{"ScalarFunctions::min", []Value{constInt(1), several}, "y"},
	}
	for _, tc := range faults {
		t.Run(tc.fn+"/"+FormatValue(tc.args[0]), func(t *testing.T) {
			_, err := applyLibrary(t, tc.fn, tc.args...)
			if !errors.Is(err, ErrMultiplicityViolation) {
				t.Fatalf("%s error = %v, want %v", tc.fn, err, ErrMultiplicityViolation)
			}
			if !strings.Contains(err.Error(), writtenName(tc.fn)) || !strings.Contains(err.Error(), `"`+tc.param+`"`) {
				t.Fatalf("%s error %q does not name the function and parameter %q", tc.fn, err, tc.param)
			}
		})
	}
}

// DataFunctions declares its operands DataValue: an attribute definition's
// object is one, a part's is not, however the operator itself would compare
// them. BaseFunctions, declared over Anything, still admits either.
func TestDataOperatorFunctionsRequireDataValues(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {
		private import ScalarValues::*;
		attribute def Point { attribute x : Integer; }
		part def Widget;
	}`)
	object := func(name string) Value {
		inst, err := ctx.Instantiate(lookupOne(t, idx, "test::"+name))
		if err != nil {
			t.Fatalf("Instantiate(%s): %v", name, err)
		}
		return Value{Kind: ValInstance, Instance: inst.ID}
	}
	point, widget := object("Point"), object("Widget")
	mixed := NewSequence()
	mixed.Append(point)
	mixed.Append(widget)
	apply := func(fn string, args ...Value) (Value, error) {
		lib, ok := libraryFunctionByName(fn)
		if !ok {
			t.Fatalf("no library function %s registered", fn)
		}
		return lib.invoke(ctx, calcArgs{positional: args})
	}

	for _, fn := range []string{"DataFunctions::==", "DataFunctions::===", "BaseFunctions::==", "BaseFunctions::==="} {
		got, err := apply(fn, point, point)
		if err != nil || !valueIdentical(got, constBool(true)) {
			t.Errorf("%s(point, point) = %s, %v; want true", fn, FormatValue(got), err)
		}
		if got, err := apply(fn, constInt(2), constInt(2)); err != nil || !valueIdentical(got, constBool(true)) {
			t.Errorf("%s(2, 2) = %s, %v; want true", fn, FormatValue(got), err)
		}
	}
	for _, fn := range []string{"DataFunctions::==", "DataFunctions::==="} {
		for _, args := range [][]Value{{widget, widget}, {widget, nullValue()}, {point, widget}} {
			_, err := apply(fn, args...)
			if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), "a DataValue") {
				t.Errorf("%s over a part = %v, want %v naming DataValue", fn, err, ErrTypeMismatch)
			}
		}
	}
	for _, fn := range []string{"DataFunctions::==", "DataFunctions::===", "BaseFunctions::==", "BaseFunctions::!=="} {
		for _, args := range [][]Value{{NewSequenceValue(mixed), nullValue()}, {constInt(1), NewSequenceValue(mixed)}} {
			_, err := apply(fn, args...)
			if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "holds 2 values") {
				t.Errorf("%s over two values = %v, want %v", fn, err, ErrMultiplicityViolation)
			}
		}
	}
	if _, err := apply("DataFunctions::+", widget, constInt(1)); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("DataFunctions::+ over a part = %v, want %v", err, ErrTypeMismatch)
	}
	for _, fn := range []string{"BaseFunctions::==", "BaseFunctions::==="} {
		if got, err := apply(fn, widget, widget); err != nil || !valueIdentical(got, constBool(true)) {
			t.Errorf("%s(widget, widget) = %s, %v; want true", fn, FormatValue(got), err)
		}
	}
}

// An operator function refuses an argument its package's parameter type does
// not admit, and reports the operator's own errors, all typed and named.
func TestOperatorFunctionErrors(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want error
	}{
		{"IntegerFunctions::+", []Value{constInt(1), constReal(2)}, ErrTypeMismatch},
		{"IntegerFunctions::+", []Value{NewStringValue("a"), NewStringValue("b")}, ErrTypeMismatch},
		{"IntegerFunctions::**", []Value{constInt(2), constInt(-1)}, ErrTypeMismatch},
		{"IntegerFunctions::*", []Value{constInt(1 << 62), constInt(4)}, semantics.ErrArithmeticOverflow},
		{"IntegerFunctions::/", []Value{constInt(1), constInt(0)}, ErrDivisionByZero},
		{"IntegerFunctions::%", []Value{constInt(1), constInt(0)}, ErrDivisionByZero},
		{"NaturalFunctions::+", []Value{constInt(-1), constInt(2)}, ErrTypeMismatch},
		{"NaturalFunctions::/", []Value{constInt(7), constInt(2)}, semantics.ErrArithmeticDomain},
		{"NaturalFunctions::/", []Value{constInt(6), constInt(0)}, ErrDivisionByZero},
		{"NaturalFunctions::/", []Value{constInt(6), constInt(-3)}, ErrTypeMismatch},
		{"NaturalFunctions::/", []Value{constInt(6), constReal(3)}, ErrTypeMismatch},
		{"NaturalFunctions::<", []Value{constInt(1), constReal(2)}, ErrTypeMismatch},
		{"IntegerFunctions::==", []Value{constInt(2), constReal(2)}, ErrTypeMismatch},
		{"IntegerFunctions::==", []Value{NewStringValue("2"), constInt(2)}, ErrTypeMismatch},
		{"NaturalFunctions::==", []Value{constInt(-1), constInt(-1)}, ErrTypeMismatch},
		{"RealFunctions::==", []Value{constReal(1), NewStringValue("1")}, ErrTypeMismatch},
		{"RationalFunctions::==", []Value{constBool(true), constBool(true)}, ErrTypeMismatch},
		{"BooleanFunctions::==", []Value{constBool(true), constInt(1)}, ErrTypeMismatch},
		{"RealFunctions::+", []Value{cx(1, 1), constReal(2)}, ErrTypeMismatch},
		{"RealFunctions::+", []Value{NewStringValue("1"), constReal(2)}, ErrTypeMismatch},
		{"RealFunctions::/", []Value{constReal(1), constReal(0)}, ErrDivisionByZero},
		{"RealFunctions::*", []Value{constReal(1e200), constReal(1e200)}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::<", []Value{constReal(1), NewStringValue("2")}, ErrTypeMismatch},
		{"NumericalFunctions::+", []Value{NewStringValue("a"), constInt(1)}, ErrTypeMismatch},
		{"ScalarFunctions::+", []Value{NewStringValue("a"), constInt(1)}, ErrTypeMismatch},
		{"DataFunctions::<", []Value{NewStringValue("a"), constInt(1)}, ErrTypeMismatch},
		{"DataFunctions::-", []Value{NewStringValue("a")}, ErrTypeMismatch},
		{"BooleanFunctions::not", []Value{constInt(1)}, ErrTypeMismatch},
		{"BooleanFunctions::xor", []Value{constBool(true), constInt(1)}, ErrTypeMismatch},
		{"DataFunctions::not", []Value{NewStringValue("true")}, ErrTypeMismatch},
		{"DataFunctions::max", []Value{constBool(true), constBool(false)}, ErrTypeMismatch},
		{"DataFunctions::max", []Value{constInt(1), NewStringValue("1")}, ErrTypeMismatch},
		{"IntegerFunctions::+", []Value{constInt(1), constInt(2), constInt(3)}, ErrCalcArity},
		{"IntegerFunctions::*", []Value{constInt(1)}, ErrCalcArity},
	}
	for _, tc := range cases {
		t.Run(tc.fn+"/"+FormatValue(tc.args[0]), func(t *testing.T) {
			_, err := applyLibrary(t, tc.fn, tc.args...)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s error = %v, want %v", tc.fn, err, tc.want)
			}
			if !strings.Contains(err.Error(), writtenName(tc.fn)) {
				t.Fatalf("%s error %q does not name the function", tc.fn, err)
			}
		})
	}
}
