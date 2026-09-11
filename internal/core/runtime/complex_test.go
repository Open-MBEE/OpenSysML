package runtime

import (
	"errors"
	"math"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// complexContext builds a runtime over the standard library and evaluates
// expressions in the scope of a package importing ComplexFunctions.
func complexContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			private import ComplexFunctions::*;
			attribute pair = (3.0, 4.0);
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// A Complex is one scalar value: it counts as one, is no collection, and a
// sequence of them keeps each as an element.
func TestComplexIsOneValue(t *testing.T) {
	z := cx(1, 2)
	if elementCount(&z) != 1 {
		t.Fatalf("elementCount(%s) = %d, want 1", FormatValue(z), elementCount(&z))
	}
	if !isScalarConstant(&z) {
		t.Fatalf("%s is not a scalar constant", FormatValue(z))
	}
	if got := elementsOf(z); len(got) != 1 || got[0].Kind != ValComplex {
		t.Fatalf("elementsOf(%s) = %v, want the Complex itself", FormatValue(z), got)
	}
	seq := NewSequence()
	seq.Append(cx(1, 2))
	seq.Append(cx(3, 4))
	zs := NewSequenceValue(seq)
	if elementCount(&zs) != 2 {
		t.Fatalf("elementCount(%s) = %d, want 2", FormatTraceValue(zs), elementCount(&zs))
	}
	if got := FormatTraceValue(zs); got != "(1.0 + 2.0i, 3.0 + 4.0i)" {
		t.Fatalf("FormatTraceValue = %s", got)
	}
	if got := describeValue(z); got != "a Complex" {
		t.Fatalf("describeValue = %q", got)
	}
}

// Formatting writes one number, `re + imi`, with the sign of the imaginary part
// between the parts; a negative zero imaginary part is written as a difference.
func TestFormatComplex(t *testing.T) {
	for _, tc := range []struct {
		z    complex128
		want string
	}{
		{complex(0, 1), "0.0 + 1.0i"},
		{complex(3, -4), "3.0 - 4.0i"},
		{complex(-1, 0), "-1.0 + 0.0i"},
		{complex(2.5, math.Copysign(0, -1)), "2.5 - 0.0i"},
		{complex(1e21, 1), "1e+21 + 1.0i"},
	} {
		if got := FormatValue(NewComplex(tc.z)); got != tc.want {
			t.Errorf("FormatValue(%v) = %q, want %q", tc.z, got, tc.want)
		}
		if got := FormatTraceValue(NewComplex(tc.z)); got != tc.want {
			t.Errorf("FormatTraceValue(%v) = %q, want %q", tc.z, got, tc.want)
		}
	}
}

// Two Complex values are equal by value; one on the real axis equals the Real
// (or Integer) it is, and a Complex equals no string, null or sequence.
func TestComplexEquality(t *testing.T) {
	for _, tc := range []struct {
		a, b Value
		want bool
	}{
		{cx(1, 2), cx(1, 2), true},
		{cx(1, 2), cx(1, -2), false},
		{cx(2, 0), constReal(2), true},
		{constReal(2), cx(2, 0), true},
		{cx(2, 0), constInt(2), true},
		{cx(2, 1), constReal(2), false},
		{cx(1, 2), realVec(1, 2), false},
		{realVec(1, 2), cx(1, 2), false},
		{cx(1, 2), NewStringValue("1.0 + 2.0i"), false},
		{cx(0, 0), Value{Kind: ValNull}, false},
	} {
		if got := valueEqual(tc.a, tc.b); got != tc.want {
			t.Errorf("valueEqual(%s, %s) = %v, want %v", FormatValue(tc.a), FormatValue(tc.b), got, tc.want)
		}
	}
}

// Hashing agrees with equality: equal values share a key, and a set holds one
// Complex once, a Complex and the Real on the real axis it equals once, and two
// Complex values differing only in imaginary part twice.
func TestComplexHashing(t *testing.T) {
	if valueKeyFunc(cx(1, 2)) != valueKeyFunc(NewComplex(complex(0.5+0.5, 4.0/2))) {
		t.Fatal("equal Complex values have different keys")
	}
	if valueKeyFunc(cx(1, 2)) == valueKeyFunc(cx(1, -2)) {
		t.Fatal("conjugates share a key")
	}
	if valueKeyFunc(cx(2, 0)) != valueKeyFunc(constReal(2)) {
		t.Fatal("a Complex on the real axis and the Real it equals have different keys")
	}
	if valueKeyFunc(cx(1, 2)) == valueKeyFunc(realVec(1, 2)) {
		t.Fatal("a Complex shares the key of the numeric pair")
	}
	set := NewSet()
	for _, v := range []Value{cx(1, 2), cx(1, 2), cx(2, 0), constReal(2), cx(1, -2), realVec(1, 2)} {
		set.Add(v)
	}
	if set.Size() != 4 {
		t.Fatalf("set holds %d elements, want 4: %s", set.Size(), FormatTraceValue(NewSetValue(set)))
	}
	if !set.Contains(constReal(2)) || !set.Contains(cx(2, 0)) || !set.Contains(cx(1, -2)) {
		t.Fatal("set lost a member it was given")
	}
}

// One number is one set member whichever of Integer, Real or Complex carries it,
// so its key agrees with valueEqual across the three.
func TestEqualNumbersShareOneSetMember(t *testing.T) {
	for _, tc := range []struct {
		name  string
		forms []Value
	}{
		{"whole", []Value{constInt(2), constReal(2), cx(2, 0)}},
		{"negative whole", []Value{constInt(-7), constReal(-7), cx(-7, 0)}},
		{"zero", []Value{constInt(0), constReal(0), constReal(math.Copysign(0, -1)), cx(0, 0), cx(math.Copysign(0, -1), 0)}},
		{"fraction", []Value{constReal(2.5), cx(2.5, 0)}},
		{"large whole", []Value{constInt(1 << 53), constReal(1 << 53), cx(1<<53, 0)}},
		{"beyond Integer", []Value{constReal(1e19), cx(1e19, 0)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set := NewSet()
			for _, v := range tc.forms {
				set.Add(v)
			}
			if set.Size() != 1 {
				t.Fatalf("set holds %d members, want 1: %s", set.Size(), FormatTraceValue(NewSetValue(set)))
			}
			for _, v := range tc.forms {
				if !set.Contains(v) {
					t.Fatalf("set does not contain %s", FormatValue(v))
				}
				for _, w := range tc.forms {
					if !valueEqual(v, w) || valueKeyFunc(v) != valueKeyFunc(w) {
						t.Fatalf("%s and %s: equal=%v, same key=%v", FormatValue(v), FormatValue(w),
							valueEqual(v, w), valueKeyFunc(v) == valueKeyFunc(w))
					}
				}
			}
		})
	}
	set := NewSet()
	for _, v := range []Value{constInt(2), constReal(2.5), cx(2, 1), cx(2, 0)} {
		set.Add(v)
	}
	if set.Size() != 3 {
		t.Fatalf("set holds %d members, want 3", set.Size())
	}
}

// A Complex is represented as a Complex whether or not it lies on the real axis: what
// ComplexFunctions return is a Complex, not the Real it may equal (KerML 1.0 §8.4.4.9.2).
func TestComplexRepresentationPrim(t *testing.T) {
	for _, tc := range []struct {
		z    complex128
		want semantics.PrimType
	}{
		{complex(0, 1), semantics.PrimComplex},
		{complex(2.5, -1), semantics.PrimComplex},
		{complex(2.5, 0), semantics.PrimComplex},
		{complex(-2, 0), semantics.PrimComplex},
		{complex(2, 0), semantics.PrimComplex},
		{complex(math.Inf(1), 0), semantics.PrimComplex},
		{complex(math.NaN(), 1), semantics.PrimUnknown},
	} {
		z := NewComplex(tc.z)
		if got := representationPrim(z); got != tc.want {
			t.Errorf("representationPrim(%s) = %v, want %v", FormatComplex(tc.z), got, tc.want)
		}
	}
}

// The arithmetic operators over a Complex operand compute as ComplexFunctions'
// declarations of the same name, a number standing for the Complex it is; `%`
// has no Complex declaration and is reported, as is a pairing with a string.
func TestComplexOperators(t *testing.T) {
	ctx, scope := complexContext(t)
	for _, tc := range []struct {
		expr string
		want string
	}{
		{"i * i", "-1.0 + 0.0i"},
		{"i + 1", "1.0 + 1.0i"},
		{"2.0 - i", "2.0 - 1.0i"},
		{"rect(1.0, 1.0) * rect(1.0, -1.0)", "2.0 + 0.0i"},
		{"rect(4.0, 2.0) / 2", "2.0 + 1.0i"},
		{"i ** 2", "-1.0 + 0.0i"},
		{"i ^ 2", "-1.0 + 0.0i"},
		{"-i", "-0.0 - 1.0i"},
		{"+i", "0.0 + 1.0i"},
		{"i == rect(0.0, 1.0)", "true"},
		{"i != rect(0.0, 1.0)", "false"},
		{"rect(2.0, 0.0) == 2", "true"},
		{"rect(2.0, 0.0) == 2.0", "true"},
		{"i == (0.0, 1.0)", "false"},
		{"abs(rect(3.0, 4.0))", "5.0"},
		{"re(polar(2.0, 0.0))", "2.0"},
		{"sum((i, i, rect(1.0, 0.0)))", "1.0 + 2.0i"},
		{"product((i, i))", "-1.0 + 0.0i"},
		{"NumericalFunctions::sum((1, i))", "1.0 + 1.0i"},
		{"isZero(rect(0.0, 0.0))", "true"},
		{"isUnit(i)", "false"},
		{"isUnit(rect(1.0, 0.0))", "true"},
		{"i ** -1", "0.0 - 1.0i"},
		{"rect(2.0, 0.0) ** 0.5", "1.4142135623730951 + 0.0i"},
	} {
		got, err := evalIn(t, ctx, scope, tc.expr)
		if err != nil {
			t.Errorf("%s: %v", tc.expr, err)
			continue
		}
		if s := FormatTraceValue(got); s != tc.want {
			t.Errorf("%s = %s, want %s", tc.expr, s, tc.want)
		}
	}
	for _, tc := range []struct {
		expr string
		want error
	}{
		{"i / rect(0.0, 0.0)", ErrDivisionByZero},
		{"i / 0", ErrDivisionByZero},
		{"rect(0.0, 0.0) ** rect(0.0, -1.0)", semantics.ErrArithmeticDomain},
		{"re(pair)", ErrTypeMismatch},
		{"abs(pair)", ErrTypeMismatch},
		{"i + pair", ErrTypeMismatch},
	} {
		if _, err := evalIn(t, ctx, scope, tc.expr); !errors.Is(err, tc.want) {
			t.Errorf("%s: error = %v, want %v", tc.expr, err, tc.want)
		}
	}
	for _, expr := range []string{`i % 2`, `i + "s"`, `"s" * i`} {
		var opErr *OperandTypeError
		if _, err := evalIn(t, ctx, scope, expr); !errors.As(err, &opErr) {
			t.Errorf("%s: error = %v, want an operand type error", expr, err)
		}
	}
}
