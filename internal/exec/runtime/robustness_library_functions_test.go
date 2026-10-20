package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestRuntimeRobustnessLibraryFunctions covers library and extension function calls: unknown
// names, operands of the wrong kind, bodies by reference and the step budget.
func TestRuntimeRobustnessLibraryFunctions(t *testing.T) {
	t.Run("library_function_outside_its_domain", testLibraryFunctionOutsideItsDomain)
	t.Run("library_function_wrong_arity", testLibraryFunctionWrongArity)
	t.Run("extension_library_function_outside_its_domain", testExtensionLibraryFunctionOutsideItsDomain)
	t.Run("sequence_index_names_no_position", testSequenceIndexNamesNoPosition)
	t.Run("collection_operand_of_the_wrong_kind", testCollectionOperandOfTheWrongKind)
	t.Run("numeric_library_call_that_has_no_value", testNumericLibraryCallThatHasNoValue)
	t.Run("named_library_call_that_has_no_value", testNamedLibraryCallThatHasNoValue)
	t.Run("builtin_named_argument_that_binds_nothing", testBuiltinNamedArgumentThatBindsNothing)
	t.Run("body_by_reference_that_cannot_be_applied", testBodyByReferenceThatCannotBeApplied)
	t.Run("bodiless_model_calc_named_as_a_builtin", testBodilessModelCalcNamedAsABuiltin)
	t.Run("data_equality_over_a_part", testDataEqualityOverAPart)
	t.Run("base_index_with_several_indexes", testBaseIndexWithSeveralIndexes)
	t.Run("structured_value_outside_the_declared_shape", testStructuredValueOutsideTheDeclaredShape)
	t.Run("real_literal_that_underflows", testRealLiteralThatUnderflows)
	t.Run("string_operand_of_the_wrong_kind", testStringOperandOfTheWrongKind)
	t.Run("ordering_operand_with_no_library_ordering", testOrderingOperandWithNoLibraryOrdering)
	t.Run("collection_body_of_the_wrong_arity", testCollectionBodyOfTheWrongArity)
	t.Run("select_predicate_is_not_a_condition", testSelectPredicateIsNotACondition)
	t.Run("collection_operation_step_budget", testCollectionOperationStepBudget)
}

// testSequenceIndexNamesNoPosition: an index outside the sequence, or one that
// is not a whole number, is reported. Answering nothing would make `seq#(i)`
// read as an empty value everywhere a model indexes past the end.
func testSequenceIndexNamesNoPosition(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{"xs#(4)", ErrIndexOutOfRange},
		{"xs#(0)", ErrIndexOutOfRange},
		{"xs#(0 - 1)", ErrIndexOutOfRange},
		{"()#(1)", ErrIndexOutOfRange},
		{"xs#(1.5)", ErrTypeMismatch},
		{"xs#(ys)", ErrTypeMismatch},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testNumericLibraryCallThatHasNoValue: a vector, Complex or sequence library
// declaration that cannot answer reports itself — a malformed argument by kind or
// dimension, an undefined result, or a declaration this runtime has no
// representation for the values of — rather than computing something else.
func testNumericLibraryCallThatHasNoValue(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{"VectorFunctions::cartesianInner(xs, ys)", ErrTypeMismatch},
		{"VectorFunctions::'cartesian+'(xs, ys)", ErrTypeMismatch},
		{"VectorFunctions::cartesianNorm(flags)", ErrTypeMismatch},
		{"VectorFunctions::cartesianAngle(xs, (0.0, 0.0, 0.0))", semantics.ErrArithmeticDomain},
		{"VectorFunctions::vectorScalarDiv(xs, 0)", ErrDivisionByZero},
		{"VectorFunctions::cartesianInner(xs)", errCalcArity},
		// Flat numbers are no collection of vectors; unequal dimensions have no
		// sum or inner product; Booleans or a two-component three-vector are no vector.
		{"VectorFunctions::sum(xs)", ErrTypeMismatch},
		{"VectorFunctions::sum0(xs, VectorFunctions::VectorOf(xs))", ErrTypeMismatch},
		{"VectorFunctions::sum0((), VectorFunctions::VectorOf(xs))", ErrTypeMismatch},
		{"VectorFunctions::sum((VectorFunctions::VectorOf(xs), VectorFunctions::VectorOf(ys)))", ErrTypeMismatch},
		{"VectorFunctions::inner(VectorFunctions::VectorOf(xs), VectorFunctions::VectorOf(ys))", ErrTypeMismatch},
		{"VectorFunctions::VectorOf(xs) + VectorFunctions::VectorOf(ys)", ErrTypeMismatch},
		{"VectorFunctions::VectorOf(flags)", ErrTypeMismatch},
		{"VectorFunctions::CartesianThreeVectorOf(ys)", ErrMultiplicityViolation},
		{"VectorFunctions::angle(VectorFunctions::VectorOf((0.0, 0.0)), VectorFunctions::VectorOf(ys))", semantics.ErrArithmeticDomain},
		// A quantity's num is Number[1..*]: a vector of no components takes no unit,
		// whether written `[m]`, scaled by a scalar quantity, or divided by one.
		{"VectorFunctions::CartesianVectorOf(()) [SI::m]", ErrMultiplicityViolation},
		{"VectorFunctions::norm(VectorFunctions::CartesianVectorOf(()) [SI::m])", ErrMultiplicityViolation},
		{"VectorCalculations::scalarQuantityVectorMult(2 [SI::m], VectorFunctions::CartesianVectorOf(()))", ErrMultiplicityViolation},
		{"VectorCalculations::vectorScalarQuantityMult(VectorFunctions::VectorOf(()), 2 [SI::m])", ErrMultiplicityViolation},
		{"VectorCalculations::vectorScalarQuantityDiv(VectorFunctions::VectorOf(()), 2 [SI::s])", ErrMultiplicityViolation},
		{"VectorFunctions::VectorOf(xs) / 0", ErrDivisionByZero},
		{"VectorCalculations::vectorScalarQuantityDiv(VectorFunctions::VectorOf(xs) [SI::m], 0 [SI::s])", ErrDivisionByZero},
		{"VectorCalculations::scalarQuantityVectorMult(2 [SI::m], flags)", ErrTypeMismatch},
		{"ComplexFunctions::'/'(ComplexFunctions::rect(0.0, 1.0), ComplexFunctions::rect(0.0, 0.0))", ErrDivisionByZero},
		{"ComplexFunctions::re(xs)", ErrTypeMismatch},
		{"ComplexFunctions::re(ys)", ErrTypeMismatch},
		{"ComplexFunctions::ToString(ComplexFunctions::rect(0.0, 1.0))", errUnevaluableLibraryFunction},
		// includingAt inserts before a position of 1..size+1, so an index past the
		// end of the sequence names no insertion point and is reported rather than
		// appending or dropping the values.
		{"SequenceFunctions::includingAt(xs, 9, 5)", ErrIndexOutOfRange},
		{"SequenceFunctions::includingAt(xs, 9, 0)", ErrIndexOutOfRange},
		{"SequenceFunctions::includingAt((), 9, 2)", ErrIndexOutOfRange},
		{"SequenceFunctions::includingAt(xs, 9, 1.5)", ErrTypeMismatch},
		{"SequenceFunctions::includingAt(xs, 9)", errCalcArity},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testNamedLibraryCallThatHasNoValue: a conversion, operator-call form, control
// function, aggregation or unevaluable declaration called by name reports itself
// by a typed error — no panic, no zero, no answer of another kind.
func testNamedLibraryCallThatHasNoValue(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{`RealFunctions::ToReal("1.5 meters")`, errInvalidNotation},
		{`RealFunctions::ToReal("NaN")`, errInvalidNotation},
		{`RealFunctions::ToReal(" 1.5 ")`, errInvalidNotation},
		{`IntegerFunctions::ToInteger(" 7")`, errInvalidNotation},
		{`RationalFunctions::ToRational("1/x")`, errInvalidNotation},
		{`IntegerFunctions::ToInteger("2.0")`, errInvalidNotation},
		{`BooleanFunctions::ToBoolean("yes")`, errInvalidNotation},
		{`IntegerFunctions::ToNatural(-1)`, semantics.ErrArithmeticDomain},
		{`NaturalFunctions::ToNatural("-1")`, semantics.ErrArithmeticDomain},
		{`RealFunctions::ToReal(xs)`, ErrTypeMismatch},
		{`RationalFunctions::gcd(1.5, 2)`, semantics.ErrArithmeticDomain},
		{`RationalFunctions::gcd("1", 2)`, ErrTypeMismatch},
		{`RationalFunctions::rat(1, 0)`, ErrDivisionByZero},
		{`RationalFunctions::rat(1.5, 3)`, ErrTypeMismatch},
		{`RationalFunctions::rat(xs, 3)`, ErrTypeMismatch},
		{`RationalFunctions::numer("0.5")`, ErrTypeMismatch},
		{`CollectionFunctions::'array#'(xs, (1, 1))`, ErrTypeMismatch},
		{`OccurrenceFunctions::isDuring(xs)`, ErrMultiplicityViolation},
		{`OccurrenceFunctions::isDuring(factor)`, errNotAnOccurrence},
		{`OccurrenceFunctions::'==='(xs, xs)`, ErrMultiplicityViolation},
		{`OccurrenceFunctions::'==='(factor, factor)`, errNotAnOccurrence},
		{`OccurrenceFunctions::create(factor)`, errNotAnOccurrence},
		{`OccurrenceFunctions::destroy(factor)`, errNotAnOccurrence},
		{`OccurrenceFunctions::addNew(xs)`, errCalcArity},
		{`OccurrenceFunctions::addNew(occ = xs)`, ErrMultiplicityViolation},
		{`OccurrenceFunctions::addNew(xs, factor)`, errNotAnOccurrence},
		{`OccurrenceFunctions::addNewAt(xs, xs)`, errCalcArity},
		{`OccurrenceFunctions::addNewAt(occ = xs, index = 1)`, ErrMultiplicityViolation},
		{`OccurrenceFunctions::addNewAt((), factor, 0)`, errNotAnOccurrence},
		{`IntegerFunctions::'+'("a", 1)`, ErrTypeMismatch},
		{`IntegerFunctions::'/'(1, 0)`, ErrDivisionByZero},
		{`NaturalFunctions::'/'(7, 2)`, semantics.ErrArithmeticDomain},
		{`NaturalFunctions::'/'(6, 0)`, ErrDivisionByZero},
		{`NaturalFunctions::'/'(6, -3)`, ErrTypeMismatch},
		{`IntegerFunctions::'=='(2, 2.0)`, ErrTypeMismatch},
		{`BooleanFunctions::'=='(true, 1)`, ErrTypeMismatch},
		{`BaseFunctions::ToString(xs)`, ErrMultiplicityViolation},
		{`IntegerFunctions::'%'(1, 0)`, ErrDivisionByZero},
		{`RealFunctions::'**'(-8.0, 0.5)`, semantics.ErrArithmeticDomain},
		{`ScalarFunctions::'<'("a", 1)`, ErrTypeMismatch},
		{`BooleanFunctions::'xor'(true, 1)`, ErrTypeMismatch},
		{`DataFunctions::max(true, false)`, ErrTypeMismatch},
		{`ScalarFunctions::min(xs, ys)`, ErrMultiplicityViolation},
		{`IntegerFunctions::'+'(xs, 1)`, ErrMultiplicityViolation},
		{`IntegerFunctions::'+'(1, xs)`, ErrMultiplicityViolation},
		{`IntegerFunctions::'+'((), 1)`, ErrMultiplicityViolation},
		{`IntegerFunctions::'-'(xs)`, ErrMultiplicityViolation},
		{`RealFunctions::'<'(xs, 2.0)`, ErrMultiplicityViolation},
		{`BooleanFunctions::'xor'(flags, true)`, ErrMultiplicityViolation},
		{`BooleanFunctions::'not'(flags)`, ErrMultiplicityViolation},
		{`BooleanFunctions::'not'(())`, ErrMultiplicityViolation},
		{`NaturalFunctions::'/'(xs, 2)`, ErrMultiplicityViolation},
		{`ScalarFunctions::'..'(1.5, 3)`, ErrTypeMismatch},
		{`BaseFunctions::'#'(xs, 0)`, ErrIndexOutOfRange},
		{`ControlFunctions::'if'(1, 2, 3)`, ErrTypeMismatch},
		{`ControlFunctions::'if'(true, {in x; x}, 3)`, ErrBodyArity},
		{`ControlFunctions::'and'(true, 1)`, ErrTypeMismatch},
		{`ControlFunctions::'and'(1, true)`, ErrTypeMismatch},
		{`ControlFunctions::'and'(true)`, ErrMultiplicityViolation},
		{`ControlFunctions::'or'(false)`, ErrMultiplicityViolation},
		{`ControlFunctions::'implies'(true)`, ErrMultiplicityViolation},
		{`ControlFunctions::'implies'(true, xs)`, ErrTypeMismatch},
		{`NumericalFunctions::sum0(xs, 1)`, ErrTypeMismatch},
		{`NumericalFunctions::product1(xs, 0)`, ErrTypeMismatch},
		{`NumericalFunctions::sum0(flags, 0)`, ErrTypeMismatch},
		{`NumericalFunctions::sum0(xs)`, errCalcArity},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testBuiltinNamedArgumentThatBindsNothing: a named argument to a built-in that
// names no parameter, names one twice, sits beside a positional argument, or
// leaves a required parameter unbound is reported rather than bound by position;
// a body passed by reference that denotes no body is reported too.
func testBuiltinNamedArgumentThatBindsNothing(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{`NumericalFunctions::sum0(zero = 0, elements = xs)`, errUnknownParameter},
		{`NumericalFunctions::sum0(zero = 0, zero = 1)`, errCalcArity},
		{`NumericalFunctions::sum0(zero = 0, zero = 1, elements = xs)`, errCalcArity},
		{`NumericalFunctions::sum0(collection = xs)`, errCalcArity},
		{`ControlFunctions::'if'(thenValue = 1, elseValue = 2)`, errCalcArity},
		{`ControlFunctions::'if'(test = true, thenValue = {in x; x})`, ErrBodyArity},
		{`ControlFunctions::'and'(secondValue = true)`, errCalcArity},
		{`ControlFunctions::'if'()`, errCalcArity},
		{`ControlFunctions::'and'()`, errCalcArity},
		{`SequenceFunctions::subsequence(xs)`, errCalcArity},
		{`SequenceFunctions::subsequence(seq = xs)`, errCalcArity},
		{`SequenceFunctions::size(xs, 1)`, errCalcArity},
		{`SequenceFunctions::'#'(xs, 1, 2)`, errCalcArity},
		{`SequenceFunctions::'#'(seq = xs, index = 0)`, ErrIndexOutOfRange},
		{`ControlFunctions::select(collection = xs, selector = factor)`, ErrTypeMismatch},
		{`xs->select factor`, ErrTypeMismatch},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testBodilessModelCalcNamedAsABuiltin: a model's own calc declared under a
// library function's qualified name — a collection built-in, a conversion, an
// operator form — is the model's, so without a body it computes nothing rather
// than what the library's declaration of that name computes.
func testBodilessModelCalcNamedAsABuiltin(t *testing.T) {
	src := `
		package NumericalFunctions {
			private import ScalarValues::*;
			calc def sum0 { in collection : Integer[*]; in zero : Integer; return : Integer; }
		}
		package RealFunctions {
			private import ScalarValues::*;
			calc def ToReal { in x : String; return : Real; }
			calc def '+' { in x : Real; in y : Real; return : Real; }
		}
		package test {
			private import ScalarValues::*;
			calc def Total { return : Integer = NumericalFunctions::sum0((1, 2, 3), 0); }
			calc def size { in seq : Integer[*]; return : Integer; }
			calc def Size { return : Integer = size((1, 2, 3)); }
			calc def Parsed { return : Real = RealFunctions::ToReal("1.5"); }
			calc def Added { return : Real = RealFunctions::'+'(1.0, 2.0); }
		}
	`
	for _, calc := range []string{"Total", "Size", "Parsed", "Added"} {
		err := calcErrorWithLibraries(t, src, calc, nil, 10000)
		if !errors.Is(err, errNoResultExpression) {
			t.Errorf("%s = %v, want %v", calc, err, errNoResultExpression)
		}
	}
}

// testDataEqualityOverAPart: DataFunctions' `'=='` and `'==='` are declared over
// DataValue, so a part given to either is refused rather than compared.
func testDataEqualityOverAPart(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Widget;
			attribute def Point { attribute x : Integer; }
			calc def SameData { in a : Point; in b : Point; return : Boolean = DataFunctions::'=='(a, b); }
			calc def SamePart { in a : Widget; in b : Widget; return : Boolean = DataFunctions::'=='(a, b); }
			calc def IdenticalPart { in a : Widget; in b : Widget; return : Boolean = DataFunctions::'==='(a, b); }
			calc def SameAnything { in a : Widget; in b : Widget; return : Boolean = BaseFunctions::'=='(a, b); }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	object := func(name string) Value {
		matches := idx.LookupQualified("test::" + name)
		if len(matches) != 1 {
			t.Fatalf("test::%s: %d matching symbols, want 1", name, len(matches))
		}
		inst, err := ctx.Instantiate(matches[0])
		if err != nil {
			t.Fatalf("Instantiate(%s): %v", name, err)
		}
		return Value{Kind: ValInstance, Instance: inst.ID}
	}
	point, widget := object("Point"), object("Widget")
	invoke := func(calc string, args ...Value) (Value, error) {
		sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", calc)
		return ctx.InvokeCalc(sym, args, scope)
	}
	for _, calc := range []string{"SamePart", "IdenticalPart"} {
		_, err := invoke(calc, widget, widget)
		if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), "a DataValue") {
			t.Errorf("%s = %v, want %v naming DataValue", calc, err, ErrTypeMismatch)
		}
	}
	for calc, args := range map[string][]Value{"SameData": {point, point}, "SameAnything": {widget, widget}} {
		got, err := invoke(calc, args...)
		if err != nil || !valueIdentical(got, constBool(true)) {
			t.Errorf("%s = %s, %v; want true", calc, FormatValue(got), err)
		}
	}
}

// testBaseIndexWithSeveralIndexes: several indexes address an Array, so a flat
// sequence, a rank mismatch, an out-of-range, ragged or oversized Array is each
// reported.
func testBaseIndexWithSeveralIndexes(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			private import Collections::*;
			attribute a : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }
			attribute ragged : Array { :>> dimensions = (2, 2); :>> elements = (1, 2, 3); }
			attribute vast : Array { :>> dimensions = (4611686018427387904, 4); :>> elements = (); }
			calc def Cell { return : Integer = BaseFunctions::'#'((1, 2, 3, 4), (2, 2)); }
			calc def NoIndex { return : Integer = BaseFunctions::'#'((1, 2, 3, 4), ()); }
			calc def OneIndex { return : Integer = a#(4); }
			calc def ThreeIndexes { return : Integer = a#(1, 1, 1); }
			calc def PastRow { return : Integer = a#(3, 1); }
			calc def PastColumn { return : Integer = a#(1, 4); }
			calc def ZeroIndex { return : Integer = a#(0, 1); }
			calc def Ragged { return : Integer = ragged#(1, 1); }
			calc def Vast { return : Integer = vast#(4611686018427387904, 4); }
		}
	`
	for calc, want := range map[string]error{
		"Cell":         ErrTypeMismatch,
		"NoIndex":      ErrMultiplicityViolation,
		"OneIndex":     ErrMultiplicityViolation,
		"ThreeIndexes": ErrMultiplicityViolation,
		"PastRow":      ErrIndexOutOfRange,
		"PastColumn":   ErrIndexOutOfRange,
		"ZeroIndex":    ErrIndexOutOfRange,
	} {
		err := calcErrorWithLibraries(t, src, calc, nil, 10000)
		if !errors.Is(err, want) || !strings.Contains(err.Error(), "BaseFunctions::'#'") {
			t.Errorf("%s = %v, want %v naming BaseFunctions::'#'", calc, err, want)
		}
	}
	err := calcErrorWithLibraries(t, src, "Ragged", nil, 10000)
	if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "flattenedSize") {
		t.Errorf("Ragged = %v, want %v naming flattenedSize", err, ErrMultiplicityViolation)
	}
	err = calcErrorWithLibraries(t, src, "Vast", nil, 10000)
	if !errors.Is(err, errIntegerUnaddressable) || !strings.Contains(err.Error(), "flattenedSize") || !strings.Contains(err.Error(), "18446744073709551616") {
		t.Errorf("Vast = %v, want %v naming flattenedSize and its exact value", err, errIntegerUnaddressable)
	}
}

// testStructuredValueOutsideTheDeclaredShape: a type specializing Array or a
// vector type fixes a shape or element type, so a value of another shape or
// element type is refused, while one that fits is held — including by a
// NumericalVectorValue specialization beside the Cartesian ones.
func testStructuredValueOutsideTheDeclaredShape(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			private import Collections::*;
			private import VectorValues::*;
			private import VectorFunctions::*;
			private import Quantities::*;
			private import ISQ::*;
			private import SI::*;
			attribute def Grid :> Array { :>> dimensions = (2, 2); }
			attribute def Row4 :> Array { :>> rank = 1; :>> flattenedSize = 4; }
			attribute def IntArray :> Array { :>> elements : Integer; }
			attribute def Fixed3 :> CartesianThreeVectorValue;
			attribute def OneDim :> NumericalVectorValue;
			attribute def IntVec :> NumericalVectorValue { :>> elements : Integer; }
			attribute def IntThree :> ThreeVectorValue { :>> elements : Integer; }
			attribute def Fixed2 :> NumericalVectorValue { :>> dimension = 2; }
			attribute def Four :> Array { :>> elements : Integer[4]; }
			attribute def Vel3 :> VectorQuantityValue { :>> num : Real[3]; }
			attribute def IntVQ :> VectorQuantityValue { :>> num : Integer; }
			attribute square : Array { :>> dimensions = (2, 2); :>> elements = (1, 2, 3, 4); }
			attribute wide : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }
			attribute row : Array { :>> dimensions = 4; :>> elements = (1, 2, 3, 4); }
			attribute reals : Array { :>> dimensions = 2; :>> elements = (1.5, 2.5); }
			calc def TwoAsThree { return : CartesianThreeVectorValue = VectorOf((1.0, 2.0)); }
			calc def TwoAsFixed3 { return : Fixed3 = VectorOf((1.0, 2.0)); }
			calc def ThreeAsThree { return : CartesianThreeVectorValue = VectorOf((1.0, 2.0, 3.0)); }
			calc def ThreeAsFixed3 { return : Fixed3 = VectorOf((1.0, 2.0, 3.0)); }
			calc def TwoAsThreeVector { return : ThreeVectorValue = VectorOf((1, 2)); }
			calc def ThreeAsThreeVector { return : ThreeVectorValue = VectorOf((1, 2, 3)); }
			calc def IntsAsIntVec { return : IntVec = VectorOf((1, 2)); }
			calc def RealsAsIntVec { return : IntVec = VectorOf((1.5, 2.0)); }
			calc def IntsAsIntThree { return : IntThree = VectorOf((1, 2, 3)); }
			calc def TwoAsIntThree { return : IntThree = VectorOf((1, 2)); }
			calc def RealsAsIntThree { return : IntThree = VectorOf((1.5, 2.0, 3.0)); }
			calc def TwoAsFixed2 { return : Fixed2 = VectorOf((1.0, 2.0)); }
			calc def ThreeAsFixed2 { return : Fixed2 = VectorOf((1.0, 2.0, 3.0)); }
			calc def VectorAsGrid { return : Grid = VectorOf((1, 2, 3, 4)); }
			calc def VectorAsString { return : String = VectorOf((1, 2)); }
			calc def WideAsGrid { return : Grid = wide; }
			calc def SquareAsGrid { return : Grid = square; }
			calc def SquareAsRow4 { return : Row4 = square; }
			calc def SquareAsOneDim { return : OneDim = square; }
			calc def RowAsRow4 { return : Row4 = row; }
			calc def RealsAsIntArray { return : IntArray = reals; }
			calc def RowAsIntArray { return : IntArray = row; }
			calc def RealsAsFour { return : Four = reals; }
			calc def SquareAsFour { return : Four = square; }
			calc def TwoAsVel3 { return : Vel3 = VectorOf((1.0, 2.0)) [m]; }
			calc def ThreeAsVel3 { return : Vel3 = VectorOf((1.0, 2.0, 3.0)) [m]; }
			calc def RealsAsIntVQ { return : IntVQ = VectorOf((1.5, 2.5)) [m]; }
			calc def IntsAsIntVQ { return : IntVQ = VectorOf((1, 2)) [m]; }
			calc def TwoAsScalar { return : ScalarQuantityValue = VectorOf((1.0, 2.0)) [m]; }
			calc def TwoAsLength { return : LengthValue = VectorOf((1.0, 2.0)) [m]; }
		}
	`
	for calc, fixes := range map[string]string{
		"TwoAsThree":       "it declares dimension = 3",
		"TwoAsFixed3":      "it declares dimension = 3",
		"TwoAsThreeVector": "it declares dimension = 3",
		"TwoAsIntThree":    "it declares dimension = 3",
		"ThreeAsFixed2":    "it declares dimension = 2",
		"RealsAsIntVec":    "it declares elements : Integer, got element 1.5 (a Rational)",
		"RealsAsIntThree":  "it declares elements : Integer, got element 1.5 (a Rational)",
		"VectorAsGrid":     "cannot write ⟨1, 2, 3, 4⟩ (vector) to a feature typed by Grid",
		"VectorAsString":   "cannot write ⟨1, 2⟩ (vector) to a feature typed by String",
		"WideAsGrid":       "it declares dimensions = [2, 2]",
		"SquareAsRow4":     "it declares rank = 1",
		"SquareAsOneDim":   "it declares dimension : Positive[0..1], got 2 dimension(s)",
		"RealsAsIntArray":  "it declares elements : Integer, got element 1.5 (a Rational)",
		"RealsAsFour":      "it declares elements : Integer[4], got 2 element(s)",
		"TwoAsVel3":        "it declares num : Real[3], got 2 element(s)",
		"RealsAsIntVQ":     "it declares num : Integer, got element 1.5 (a Rational)",
		"TwoAsScalar":      "to a feature typed by ScalarQuantityValue: it is a ScalarValue, which holds one scalar",
		"TwoAsLength":      "to a feature typed by LengthValue: it is a ScalarValue, which holds one scalar",
	} {
		err := calcErrorWithLibraries(t, src, calc, nil, 10000)
		if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), fixes) {
			t.Errorf("%s = %v, want %v naming %q", calc, err, ErrTypeMismatch, fixes)
		}
	}
	for calc, want := range map[string]string{
		"ThreeAsThree":       "⟨1.0, 2.0, 3.0⟩",
		"ThreeAsFixed3":      "⟨1.0, 2.0, 3.0⟩",
		"ThreeAsThreeVector": "⟨1, 2, 3⟩",
		"IntsAsIntVec":       "⟨1, 2⟩",
		"IntsAsIntThree":     "⟨1, 2, 3⟩",
		"TwoAsFixed2":        "⟨1.0, 2.0⟩",
		"SquareAsGrid":       "Array(2, 2)[1, 2, 3, 4]",
		"RowAsRow4":          "Array(4)[1, 2, 3, 4]",
		"RowAsIntArray":      "Array(4)[1, 2, 3, 4]",
		"SquareAsFour":       "Array(2, 2)[1, 2, 3, 4]",
		"ThreeAsVel3":        "⟨1.0, 2.0, 3.0⟩ [m]",
		"IntsAsIntVQ":        "⟨1, 2⟩ [m]",
	} {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
		sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", calc)
		got, err := invokeValue(ctx, sym, nil, scope)
		if err != nil || FormatValue(got) != want {
			t.Errorf("%s = (%s, %v), want %s", calc, FormatValue(got), err, want)
		}
	}
}

// testBodyByReferenceThatCannotBeApplied: a body reaching a built-in through an
// `expr` parameter is applied in the scope it was written in, and one selected
// by a control function that declares parameters is reported.
func testBodyByReferenceThatCannotBeApplied(t *testing.T) {
	src := `
		package test {
			private import ControlFunctions::*;
			calc def Pick { in expr chosen; return : Integer = ControlFunctions::'if'(true, chosen, 0); }
			calc def Keep { in xs : Integer[*]; in expr pred; return : Integer[*] = xs->select pred; }
			calc def PicksUnary { return : Integer = Pick({ in x; x }); }
			calc def KeepsUnbound { in xs : Integer[*]; return : Integer[*] = Keep(xs, { in x; x > bound }); }
			calc def KeepsPriorFrame {
				in xs : Integer[*];
				in pred : Integer;
				return : Integer[*] = Keep(xs, { in x; x > pred });
			}
		}
	`
	for _, tt := range []struct {
		calc string
		args []Value
		want error
	}{
		{"PicksUnary", nil, ErrBodyArity},
		{"KeepsUnbound", []Value{constSequence(1, 2)}, ErrUnresolvedReference},
	} {
		err := calcErrorWithLibraries(t, src, tt.calc, tt.args, 10000)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = %v, want %v", tt.calc, err, tt.want)
		}
	}
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", "KeepsPriorFrame")
	got, err := ctx.InvokeCalc(sym, []Value{constSequence(1, 2, 3), constInt(1)}, scope)
	if err != nil {
		t.Fatalf("KeepsPriorFrame = %v", err)
	}
	if want := constSequence(2, 3); !valueIdentical(got, want) {
		t.Errorf("KeepsPriorFrame = %s, want %s: the body's pred is its caller's Integer, not Keep's body", FormatValue(got), FormatValue(want))
	}
}

// testRealLiteralThatUnderflows: a decimal literal beyond the binary64 range is the
// exact Rational, one beyond the size budget is reported, and a Real too small for
// a binary64 is reported rather than read as zero.
func testRealLiteralThatUnderflows(t *testing.T) {
	for _, src := range []string{`1.0e-400`, `1.0e400`} {
		got, err := evalCollectionExpr(t, src)
		if err != nil || got.Kind != ValConst || got.Const.Kind != semantics.ValRational || got.Const.RatSign() != 1 {
			t.Errorf("%s = (%v, %v), want the exact Rational", src, got, err)
		}
	}
	for _, tt := range []struct {
		expr string
		want error
	}{
		{`1.0e-400000`, semantics.ErrRationalSizeLimit},
		{`RealFunctions::ToReal("1e-400")`, semantics.ErrArithmeticOverflow},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
	got, err := evalCollectionExpr(t, `0.0e-400`)
	if err != nil || got.Kind != ValConst || got.Const.Kind != semantics.ValRational || got.Const.RatSign() != 0 {
		t.Errorf("0.0e-400 = (%v, %v), want the Rational 0", got, err)
	}
}

// testStringOperandOfTheWrongKind: an operator or StringFunctions call given a
// value that is not the String its signature declares is reported rather than
// coerced, and a Substring position naming no character is reported rather than
// clamped.
func testStringOperandOfTheWrongKind(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{`"a" + 1`, ErrTypeMismatch},
		{`1 + "a"`, ErrTypeMismatch},
		{`"a" < 1`, ErrTypeMismatch},
		{`"a" >= factor`, ErrTypeMismatch},
		{`"a" < xs`, ErrTypeMismatch},
		{`"a" - "b"`, ErrTypeMismatch},
		{`StringFunctions::Length(1)`, ErrTypeMismatch},
		{`StringFunctions::Length(xs)`, ErrTypeMismatch},
		{`StringFunctions::Substring("abc", 1, 9)`, ErrIndexOutOfRange},
		{`StringFunctions::Substring("héllo", 1, 6)`, ErrIndexOutOfRange},
		{`StringFunctions::Substring("abc", 0, 2)`, ErrIndexOutOfRange},
		{`StringFunctions::Substring("abc", "1", 2)`, ErrTypeMismatch},
		{`StringFunctions::Substring("abc", 1)`, errCalcArity},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testOrderingOperandWithNoLibraryOrdering: an ordering operator over a value
// the Kernel Function Library declares no ordering for — DataFunctions::'<' and
// ScalarFunctions::'<' are abstract; the numeric libraries and StringFunctions
// alone declare it — is a type mismatch naming the operator, both operands and
// the library function that would have to declare it, never a claim that the
// operands are not constants. Where the library does order (a numeric
// enumeration, Strings, quantities) the answer is unchanged.
func testOrderingOperandWithNoLibraryOrdering(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			private import SequenceFunctions::*;
			private import ISQ::*;
			private import SI::*;
			enum def Color { red; green; blue; }
			enum def Level :> Integer { low = 1; high = 3; }
			part def Widget;
			part widget : Widget;
			attribute def Point { attribute x : Integer = 1; }
			attribute point : Point;
			metadata def Tag;
			#Tag part tagged : Widget;
			calc twice { in x : Integer[1]; x * 2 }
			attribute xs : Integer[*] = (1, 2);
			part other : Widget;
			attribute widgets : Widget[*] = (widget, other);
			attribute nothing : Integer[*] = xs->excluding(1)->excluding(2);
			attribute side : LengthValue = 2 [m];
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	const (
		color  = "DataFunctions::'%s' is abstract and no library function declares '%s' for the enumeration Color, which is no ScalarValue"
		oneVal = "DataFunctions::'%s' takes one DataValue per operand"
	)
	refused := []struct {
		expr, op string
		what     string
		library  string
	}{
		{"Color::red < Color::blue", "<", "the enumeration literal Color::red and the enumeration literal Color::blue", color},
		{"Color::red > Color::blue", ">", "the enumeration literal Color::red and the enumeration literal Color::blue", color},
		{"Color::red <= Color::blue", "<=", "the enumeration literal Color::red and the enumeration literal Color::blue", color},
		{"Color::red >= Color::blue", ">=", "the enumeration literal Color::red and the enumeration literal Color::blue", color},
		{"1 < Color::red", "<", "an Integer and the enumeration literal Color::red", color},
		{"Level::low < Color::red", "<", "an Integer and the enumeration literal Color::red", color},
		{"widget < widget", "<", "an instance and an instance", "DataFunctions::'%s' takes DataValue operands and an instance of the part def Widget is none"},
		{"tagged.metadata#(1) < 1", "<", "an instance and an Integer", "DataFunctions::'%s' takes DataValue operands and an instance of the metadata def Tag is none"},
		{"(tagged meta Tag) < 1", "<", "an instance and an Integer", "DataFunctions::'%s' takes DataValue operands and an instance of the metadata def Tag is none"},
		{"tagged.metadata < 1", "<", "a sequence and an Integer", oneVal},
		{"point < point", "<", "an instance and an instance", "DataFunctions::'%s' is abstract and no library function declares '%s' for an instance of the attribute def Point, which is no ScalarValue"},
		{"twice < 1", "<", "the function test::twice and an Integer", "DataFunctions::'%s' takes DataValue operands and the function test::twice is none"},
		{"widgets < 1", "<", "a sequence and an Integer", oneVal},
		{"xs > 1", ">", "a sequence and an Integer", oneVal},
		{"1 <= xs->including(3)", "<=", "an Integer and a sequence", oneVal},
		{"nothing < 1", "<", "a sequence and an Integer", oneVal},
		{"null >= 1", ">=", "null and an Integer", oneVal},
		{"true < false", "<", "a Boolean and a Boolean", "ScalarFunctions::'%s' is abstract and BooleanFunctions declares no '%s' for Boolean"},
		{"1 < true", "<", "an Integer and a Boolean", "ScalarFunctions::'%s' is abstract and BooleanFunctions declares no '%s' for Boolean"},
		{"2 [m] < true", "<", "a quantity and a Boolean", "ScalarFunctions::'%s' is abstract and BooleanFunctions declares no '%s' for Boolean"},
		{"2 [m] < *", "<", "a quantity and an infinity", "QuantityCalculations::'%s' takes ScalarQuantityValue operands and an infinity is none"},
		{"* >= side", ">=", "an infinity and a quantity", "QuantityCalculations::'%s' takes ScalarQuantityValue operands and an infinity is none"},
		{"side < Color::red", "<", "a quantity and the enumeration literal Color::red", color},
		{"m < s", "<", "a measurement reference and a measurement reference", "DataFunctions::'%s' is abstract and no library function declares '%s' for a measurement reference, which is no ScalarValue"},
	}
	for _, tt := range refused {
		library := strings.ReplaceAll(tt.library, "%s", tt.op)
		want := fmt.Sprintf("type mismatch: operator '%s' is not defined for %s; %s", tt.op, tt.what, library)
		got, err := evalIn(t, ctx, pkg.Scope, tt.expr)
		var opErr *OperandTypeError
		if !errors.As(err, &opErr) || !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s = (%s, %v), want an OperandTypeError wrapping %v", tt.expr, FormatValue(got), err, ErrTypeMismatch)
			continue
		}
		if err.Error() != want {
			t.Errorf("%s:\n got  %q\n want %q", tt.expr, err.Error(), want)
		}
		if strings.Contains(err.Error(), "must be constants") {
			t.Errorf("%s: %q claims the operands are not constants", tt.expr, err)
		}
		if opErr.Span == (source.Span{}) {
			t.Errorf("%s: the error carries no span to locate the operator", tt.expr)
		}
	}
	ordered := map[string]bool{
		"Level::low < Level::high":  true,
		"Level::high <= Level::low": false,
		`"a" < "b"`:                 true,
		"2 [m] > 1 [m]":             true,
		"side >= 200 [cm]":          true,
		"1 < 2.5":                   true,
		"3 > *":                     false,
	}
	for expr, want := range ordered {
		got, err := evalIn(t, ctx, pkg.Scope, expr)
		if err != nil || !valueIdentical(got, constBool(want)) {
			t.Errorf("%s = %s, %v; want %v", expr, FormatValue(got), err, want)
		}
	}
}

// testCollectionOperandOfTheWrongKind: an operation given something that is not
// the kind of value it operates on reports it rather than reading the value as a
// collection of itself or as nothing.
func testCollectionOperandOfTheWrongKind(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{"xs->select(2)", ErrTypeMismatch},
		{"xs->collect(xs)", ErrTypeMismatch},
		{`sum((1, "a"))`, ErrTypeMismatch},
		{"product(flags)", ErrTypeMismatch},
		{"xs->subsequence(1, 4)", ErrIndexOutOfRange},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testCollectionBodyOfTheWrongArity: a body an operation calls with one element
// but that declares no parameter, or two, is reported. Binding what it declares
// and dropping the rest would answer from a parameter that was never given a
// value.
func testCollectionBodyOfTheWrongArity(t *testing.T) {
	for _, expr := range []string{
		"xs->collect {}",
		"xs->collect {in x; in y; x}",
		"xs->select {in x; in y; x > 0}",
		"xs.{in x; in y; x}",
	} {
		got, err := evalCollectionExpr(t, expr)
		if !errors.Is(err, ErrBodyArity) {
			t.Errorf("%s = (%v, %v), want ErrBodyArity", expr, got, err)
		}
	}
}

// testSelectPredicateIsNotACondition: a selector answering something that is not
// a boolean is reported. Reading a non-boolean as false would silently drop
// every element, which is a wrong answer rather than a failure.
func testSelectPredicateIsNotACondition(t *testing.T) {
	for _, expr := range []string{
		"xs.?{in x; x + 1}",
		"xs->select {in x; x * 2}",
		"xs->reject {in x; 1}",
		"xs->forAll {in x; x}",
		`xs->exists {in x; "yes"}`,
	} {
		got, err := evalCollectionExpr(t, expr)
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s = (%v, %v), want ErrTypeMismatch", expr, got, err)
		}
	}
}

// testCollectionOperationStepBudget: an operation calls its body once per
// element, and each call spends the context's budget, so a collection large
// enough for the budget fails the evaluation instead of running unbounded.
func testCollectionOperationStepBudget(t *testing.T) {
	got, err := evalCollectionExprBounded(t, "xs.{in x; x * factor}", 3)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("collect under a budget of 3 = (%v, %v), want ErrStepLimitExceeded", got, err)
	}
}

// testLibraryFunctionOutsideItsDomain: a library function whose argument has no
// result reports a domain error rather than returning a NaN.
func testLibraryFunctionOutsideItsDomain(t *testing.T) {
	src := `
		package test {
			private import RealFunctions::*;
			calc root {
				in x : Real;
				return : Real = sqrt(x);
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "root", ast.DefCalc)
	if sym == nil {
		t.Fatal("root calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: -1}}
	got, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if !errors.Is(err, semantics.ErrArithmeticDomain) {
		t.Fatalf("sqrt(-1.0) = %+v, %v; want a domain error", got, err)
	}
}

// testLibraryFunctionWrongArity: a library function called with the wrong number
// of arguments reports an arity error rather than reading past its arguments.
func testLibraryFunctionWrongArity(t *testing.T) {
	fn, ok := libraryFunctionByName("RealFunctions::max")
	if !ok {
		t.Fatal("RealFunctions::max not registered")
	}
	_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, "package test { }"))

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1}}
	if _, err := fn.invoke(ctx, calcArgs{positional: []Value{arg}}); !errors.Is(err, errCalcArity) {
		t.Fatalf("max(1.0) error = %v, want errCalcArity", err)
	}
}

// testExtensionLibraryFunctionOutsideItsDomain: an OpenSysML extension library
// function reports a domain error the same way a vendored one does — the
// logarithm of zero has no Real value, and is not returned as an infinity.
func testExtensionLibraryFunctionOutsideItsDomain(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			private import OpenSysMLMathFunctions::*;
			calc root {
				in x : Real;
				return : Real = ln(x);
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "root", ast.DefCalc)
	if sym == nil {
		t.Fatal("root calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 0}}
	got, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if !errors.Is(err, semantics.ErrArithmeticDomain) {
		t.Fatalf("ln(0.0) = %+v, %v; want a domain error", got, err)
	}
}
