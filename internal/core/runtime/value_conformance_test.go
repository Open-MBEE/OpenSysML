package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// valueConformanceContext builds a runtime over a model whose feature values and calc
// arguments are typed wider than their targets, so the type checker defers them.
func valueConformanceContext(t *testing.T) (*Context, *symbols.Index, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			attribute seven : Real = 7.0;
			attribute two : Real = 2.0;
			attribute xs = (1, 2, 3);

			part def P {
				attribute exact : Integer = 3;
				attribute whole : Integer = 4 / 2;
				attribute half : Integer = 7 / 2;
				attribute nat : Natural = 4 / 2;
				attribute neg : Natural = -4 / 2;
				attribute fromReal : Integer = two;
				attribute computed : Integer = seven - two;
				attribute computedHalf : Integer = seven / two;
			}
			part p : P;
			part q : P { attribute :>> half = 4 / 2; }

			calc def Half { in x : Integer; return : Rational = x / 2; }
			calc def Twice { in x : Integer = 7 / 2; return : Integer = x * 2; }
			calc def Base { in x : Integer; return : Integer = x; }
			calc def Derived :> Base;
			calc def Redef :> Base { in :>> x = 7 / 2; }
			calc def IntDiv { return : Integer = 7 / 2; }
			calc def WholeDiv { return : Integer = 4 / 2; }
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, idx, pkg.Scope
}

func instantiateNamed(t *testing.T, ctx *Context, idx *symbols.Index, qualified string) *Instance {
	t.Helper()
	matches := idx.LookupQualified(qualified)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", qualified, len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate %s: %v", qualified, err)
	}
	return inst
}

// A feature value is a binding, so the value materialized for a default must be
// an instance of the declared type: 4 / 2 is the Integer 2, 7 / 2 is not one.
func TestDefaultValueMustConformAtMaterialization(t *testing.T) {
	ctx, idx, _ := valueConformanceContext(t)
	inst := instantiateNamed(t, ctx, idx, "test::p")

	for _, tc := range []struct {
		feature string
		want    string
	}{
		{"exact", "3"},
		{"whole", "2.0"},
		{"nat", "2.0"},
		{"fromReal", "2.0"},
		{"computed", "5.0"},
	} {
		fv, err := inst.GetFeatureValue(ctx, tc.feature)
		if err != nil {
			t.Errorf("%s: %v", tc.feature, err)
			continue
		}
		if got := FormatTraceValue(fv.HeldValue()); got != tc.want {
			t.Errorf("%s = %s, want %s", tc.feature, got, tc.want)
		}
	}
	for _, feature := range []string{"half", "neg", "computedHalf"} {
		if _, err := inst.GetFeatureValue(ctx, feature); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", feature, err)
		}
	}
}

// A usage restating a default binds its own value, which is checked the same way.
func TestRestatedDefaultConformsByItsOwnValue(t *testing.T) {
	ctx, idx, _ := valueConformanceContext(t)
	inst := instantiateNamed(t, ctx, idx, "test::q")
	fv, err := inst.GetFeatureValue(ctx, "half")
	if err != nil {
		t.Fatalf("half: %v", err)
	}
	if got := FormatTraceValue(fv.HeldValue()); got != "2.0" {
		t.Errorf("q.half = %s, want 2.0", got)
	}
}

// Reading a package-level declaration checks its value against its type too.
func TestDeclaredValueReadChecksItsType(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			attribute whole : Integer = 4 / 2;
			attribute half : Integer = 7 / 2;
		}
	`))
	symbol := func(name string) *symbols.Symbol {
		matches := idx.LookupQualified("test::" + name)
		if len(matches) != 1 {
			t.Fatalf("test::%s: %d matching symbols, want 1", name, len(matches))
		}
		return matches[0]
	}
	if got, err := ctx.EvalDeclaredValue(symbol("whole")); err != nil || FormatTraceValue(got) != "2.0" {
		t.Errorf("whole = %s, %v; want 2.0", FormatTraceValue(got), err)
	}
	if _, err := ctx.EvalDeclaredValue(symbol("half")); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("half: error = %v, want ErrTypeMismatch", err)
	}
}

// A model declaring its own scalar types gives the run time no value semantics
// for them, so a value is refused only where the two types are disjoint: KerML
// asks a binding's types to conform in either direction, and the value is not judged.
func TestUserDeclaredScalarTypeDefersToTheBinding(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			item def Real;
			item def Integer :> Real;
			item def Natural :> Integer;
			item def String;
			part def P {
				attribute nat : Natural = 3;
				attribute narrowed : Integer = 2.5;
				attribute widened : Real = 3;
				attribute disjoint : String = 3;
			}
			part p : P;
		}
	`))
	inst := instantiateNamed(t, ctx, idx, "test::p")
	for feature, want := range map[string]string{"nat": "3", "narrowed": "2.5", "widened": "3"} {
		fv, err := inst.GetFeatureValue(ctx, feature)
		if err != nil {
			t.Errorf("%s: %v", feature, err)
			continue
		}
		if got := FormatTraceValue(fv.HeldValue()); got != want {
			t.Errorf("%s = %s, want %s", feature, got, want)
		}
	}
	if _, err := inst.GetFeatureValue(ctx, "disjoint"); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("disjoint: error = %v, want ErrTypeMismatch", err)
	}
}

// A positional, named or default argument — through an inherited or redefined
// parameter too — must be a value of the parameter's type; so must the result.
func TestCalcParameterAndResultMustConform(t *testing.T) {
	ctx, _, scope := valueConformanceContext(t)

	for _, tc := range []struct {
		expr string
		want string
	}{
		{"Half(4 / 2)", "1.0"},
		{"Half(x = 4 / 2)", "1.0"},
		{"Half(two)", "1.0"},
		{"Twice(2)", "4"},
		{"Twice(x = 4 / 2)", "4.0"},
		{"Derived(4 / 2)", "2.0"},
		{"Redef(4 / 2)", "2.0"},
		{"Redef(x = 4 / 2)", "2.0"},
		{"WholeDiv()", "2.0"},
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
	for _, expr := range []string{
		"Half(7 / 2)",
		"Half(1.5)",
		"Half(x = 1.5)",
		"Half(seven / two)",
		"Twice()",
		"Derived(1.5)",
		"Redef(1.5)",
		"Redef()",
		"IntDiv()",
	} {
		if _, err := evalIn(t, ctx, scope, expr); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", expr, err)
		}
	}
}

// calcMultiplicityContext builds a runtime over calcs declaring every
// multiplicity form a parameter or result value is counted against.
func calcMultiplicityContext(t *testing.T) (*Context, *symbols.Index, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			private import SequenceFunctions::*;

			calc def One { in x : Integer; return : Integer = x; }
			calc def Untyped { in x; return : Integer = x; }
			calc def Opt { in x : Integer[0..1] = null; return : Integer = x ?? 0; }
			calc def Many { in xs : Integer[*]; return : Integer = xs->size(); }
			calc def UpTo2 { in xs : Integer[1..2]; return : Integer = xs->size(); }
			calc def BadDefault { in x : Integer = (1, 2); return : Integer = x; }
			calc def DerivedMany :> Many;
			calc def RedefOne :> Many { in :>> xs : Integer[1]; }
			calc def RedeclMany :> Many { in :>> xs; }
			calc def RedeclManyTyped :> Many { in :>> xs : Integer; }

			calc def TwoResults { return : Integer = (1, 2); }
			calc def NoResult { return : Integer = null; }
			calc def ManyResults { return : Integer[*] = (1, 2); }
			calc def Returned {
				in xs : Integer[*];
				attribute n : Integer = xs->size();
				return : Integer = xs;
			}
			calc def ManyOut { in xs : Integer[*]; out ys : Integer[*] = xs; }
			calc def OneOut { in xs : Integer[*]; out y : Integer = xs; }
			calc def ManyOutRedecl :> ManyOut { out :>> ys; }
			calc def ManyResultsRedecl :> ManyResults { return : Integer; }
			calc def BoundOne { in xs : Integer[*]; return : Integer; bind result = xs; }
			calc def BoundMany { in xs : Integer[*]; return : Integer[*]; bind result = xs; }
			calc def BoundDerived :> BoundOne {
				in :>> xs;
				attribute pair : Integer[2] nonunique = (xs, xs);
				bind result = pair;
			}
			calc def BoundString { attribute s : String = "s"; return : Integer; bind result = s; }

			calc def Unit { return : Complex = ComplexFunctions::i; }
			calc def UnitOut { out z : Complex = ComplexFunctions::i; }
			calc def UnitReal { return : Real = ComplexFunctions::i; }
			calc def Units { return : Complex[2] = (ComplexFunctions::i, ComplexFunctions::rect(1.0, 0.0)); }
			calc def UnitsOne { return : Complex = (ComplexFunctions::i, ComplexFunctions::rect(1.0, 0.0)); }
			part def P {
				attribute z : Complex = ComplexFunctions::rect(0.0, 1.0);
				attribute r : Complex = 2.0;
				attribute onAxis : Real = ComplexFunctions::rect(2.0, 0.0);
				attribute whole : Integer = ComplexFunctions::rect(2.0, 0.0);
				attribute half : Integer = ComplexFunctions::rect(2.5, 0.0);
				attribute pair : Complex = (1.0, 2.0);
				attribute triple : Complex = (1.0, 2.0, 3.0);
				attribute offAxis : Real = ComplexFunctions::rect(0.0, 1.0);
			}
			part p : P;
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, idx, pkg.Scope
}

// A positional, named or default argument — through an inherited or redefined
// parameter too — and the result, returned or bound, answer to the declared
// (else 1..1) multiplicity.
func TestCalcParameterAndResultMultiplicity(t *testing.T) {
	ctx, _, scope := calcMultiplicityContext(t)

	for _, tc := range []struct {
		expr string
		want string
	}{
		{"One(1)", "1"},
		{"Untyped(1)", "1"},
		{"Opt()", "0"},
		{"Opt(null)", "0"},
		{"Opt(x = 5)", "5"},
		{"Many((1, 2, 3))", "3"},
		{"Many(1)", "1"},
		{"Many(null)", "0"},
		{"UpTo2((1, 2))", "2"},
		{"DerivedMany((1, 2))", "2"},
		{"RedefOne(1)", "1"},
		{"RedeclMany((1, 2))", "2"},
		{"RedeclManyTyped((1, 2))", "2"},
		{"ManyOutRedecl((1, 2))", "(1, 2)"},
		{"ManyResultsRedecl()", "(1, 2)"},
		{"ManyResults()", "(1, 2)"},
		{"Returned(1)", "1"},
		{"ManyOut((1, 2))", "(1, 2)"},
		{"OneOut(1)", "1"},
		{"BoundOne(1)", "1"},
		{"BoundMany((1, 2))", "(1, 2)"},
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
	for _, expr := range []string{
		"One((1, 2))",
		"One(x = (1, 2))",
		"One(null)",
		"Untyped((1, 2))",
		"Opt((1, 2))",
		"UpTo2((1, 2, 3))",
		"UpTo2(null)",
		"BadDefault()",
		"RedefOne((1, 2))",
		"TwoResults()",
		"NoResult()",
		"Returned((1, 2))",
		"OneOut((1, 2))",
		"BoundOne((1, 2))",
		"BoundDerived(1)",
	} {
		if _, err := evalIn(t, ctx, scope, expr); !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("%s: error = %v, want ErrMultiplicityViolation", expr, err)
		}
	}
	for _, expr := range []string{"BoundString()", `RedeclMany("s")`} {
		if _, err := evalIn(t, ctx, scope, expr); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", expr, err)
		}
	}
}

// A violation is reported at the binding, before the body runs, under the label
// a type refusal of that binding carries.
func TestCalcMultiplicityViolationNamesTheBinding(t *testing.T) {
	ctx, _, scope := calcMultiplicityContext(t)
	for _, tc := range []struct {
		expr string
		want string
	}{
		{"One((1, 2))", `calc test::One: argument for parameter "x": multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1`},
		{"BadDefault()", `calc test::BadDefault: default for parameter "x": multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1`},
		{"UpTo2(null)", `calc test::UpTo2: argument for parameter "xs": multiplicity violation: 0 value(s) bound to a feature with multiplicity lower bound 1`},
		{"TwoResults()", `calc test::TwoResults: result: multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1`},
	} {
		_, err := evalIn(t, ctx, scope, tc.expr)
		if err == nil || err.Error() != tc.want {
			t.Errorf("%s: error = %v\nwant %s", tc.expr, err, tc.want)
		}
	}
}

// One Complex is one value, however a feature is typed: a Complex feature holds
// it, a Real feature refuses it by type, and a numeric pair is two values.
func TestComplexCountsAsOneValue(t *testing.T) {
	ctx, idx, scope := calcMultiplicityContext(t)

	for _, tc := range []struct {
		expr string
		want string
	}{
		{"Unit()", "0.0 + 1.0i"},
		{"UnitOut()", "0.0 + 1.0i"},
		{"Units()", "(0.0 + 1.0i, 1.0 + 0.0i)"},
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
	if _, err := evalIn(t, ctx, scope, "UnitReal()"); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("UnitReal(): error = %v, want ErrTypeMismatch", err)
	}
	if _, err := evalIn(t, ctx, scope, "UnitsOne()"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("UnitsOne(): error = %v, want ErrMultiplicityViolation", err)
	}

	inst := instantiateNamed(t, ctx, idx, "test::p")
	for feature, want := range map[string]string{
		"z": "0.0 + 1.0i", "r": "2.0", "onAxis": "2.0 + 0.0i", "whole": "2.0 + 0.0i",
	} {
		fv, err := inst.GetFeatureValue(ctx, feature)
		if err != nil {
			t.Errorf("%s: %v", feature, err)
			continue
		}
		if got := FormatTraceValue(fv.HeldValue()); got != want {
			t.Errorf("%s = %s, want %s", feature, got, want)
		}
	}
	for _, feature := range []string{"pair", "triple"} {
		if _, err := inst.GetFeatureValue(ctx, feature); !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("%s: error = %v, want ErrMultiplicityViolation", feature, err)
		}
	}
	for _, feature := range []string{"half", "offAxis"} {
		if _, err := inst.GetFeatureValue(ctx, feature); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", feature, err)
		}
	}
}

// A whole-valued Real names a position; a fractional one names none, and is
// never truncated to one.
func TestSequenceIndexAcceptsAWholeValuedReal(t *testing.T) {
	ctx, _, scope := valueConformanceContext(t)

	for _, tc := range []struct {
		expr string
		want string
	}{
		{"xs#(2)", "2"},
		{"xs#(4 / 2)", "2"},
		{"xs#(two)", "2"},
		{"xs#(6 / 2)", "3"},
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
	for _, expr := range []string{"xs#(1.5)", "xs#(7 / 2)", "xs#(seven / two)", "xs#(true)"} {
		if _, err := evalIn(t, ctx, scope, expr); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", expr, err)
		}
	}
	for _, expr := range []string{"xs#(0)", "xs#(-1)", "xs#(8 / 2)", "xs#(0 / 2)"} {
		if _, err := evalIn(t, ctx, scope, expr); !errors.Is(err, ErrIndexOutOfRange) {
			t.Errorf("%s: error = %v, want ErrIndexOutOfRange", expr, err)
		}
	}
}
