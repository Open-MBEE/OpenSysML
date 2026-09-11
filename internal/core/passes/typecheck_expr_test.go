package passes

import (
	"fmt"
	"strings"
	"testing"
)

// scalarPrelude declares the stdlib scalar types the expression checker keys
// off, so these tests do not need the real library loaded.
const scalarPrelude = `package ScalarValues {
	attribute def ScalarValue;
	attribute def Boolean specializes ScalarValue;
	attribute def String specializes ScalarValue;
	attribute def Number specializes ScalarValue;
	attribute def Complex specializes Number;
	attribute def Real specializes Complex;
	attribute def Rational specializes Real;
	attribute def Integer specializes Rational;
	attribute def Natural specializes Integer;
}
`

// exprDiags runs the default registry over the scalar prelude plus src and
// returns the type-tier diagnostics.
func exprDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	return typeDiags(t, scalarPrelude+src)
}

// wantOneDiag asserts exactly one type diagnostic whose message contains want.
func wantOneDiag(t *testing.T, src, want string) {
	t.Helper()
	diags := exprDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, want) {
		t.Fatalf("expected message containing %q, got %q", want, diags[0].Message)
	}
}

// wantOneWarning asserts exactly one type diagnostic, a warning under code, whose message
// contains want.
func wantOneWarning(t *testing.T, src, code, want string) {
	t.Helper()
	diags := exprDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if diags[0].Severity != SeverityWarning || diags[0].Code != code {
		t.Fatalf("expected warning %q, got %s %q (%s)", code, diags[0].Severity, diags[0].Code, diags[0].Message)
	}
	if !strings.Contains(diags[0].Message, want) {
		t.Fatalf("expected message containing %q, got %q", want, diags[0].Message)
	}
}

func wantNoDiags(t *testing.T, src string) {
	t.Helper()
	if diags := exprDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestExprBindStringToIntegerAttribute(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x : ScalarValues::Integer = "hello"; }`,
		"cannot bind String value to a feature typed by Integer")
}

func TestExprBindRealToIntegerAttribute(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x : ScalarValues::Integer = 5.5; }`,
		"cannot bind Rational value to a feature typed by Integer")
}

func TestExprBindIntegerToRealAttributeOK(t *testing.T) {
	wantNoDiags(t, `package P { attribute x : ScalarValues::Real = 5; }`)
}

// A feature reference is typed by the feature's declaration, not by the value
// it happens to hold: `w` is a Real here, whatever the 3 it was given is.
func TestExprBindFeatureReferenceRespectsDeclaredType(t *testing.T) {
	wantOneDiag(t, `package P {
		attribute w : ScalarValues::Real = 3;
		attribute x : ScalarValues::String = w;
	}`, "cannot bind Real value to a feature typed by String")
}

// A type only bounds an expression's values, so a narrower-typed feature is not
// refused: the Real `w` may hold a whole number, known when the value is bound.
func TestExprBindWiderTypedReferenceToNarrowerFeatureOK(t *testing.T) {
	wantNoDiags(t, `package P {
		attribute w : ScalarValues::Real = 1.5;
		attribute x : ScalarValues::Integer = w;
	}`)
}

func TestExprBindNestedUsageValue(t *testing.T) {
	wantOneDiag(t, `package P {
		part def Car {
			part engine {
				attribute power : ScalarValues::Integer = "x";
			}
		}
	}`, "cannot bind String value to a feature typed by Integer")
}

func TestExprUntypedFeatureBindingSkipped(t *testing.T) {
	wantNoDiags(t, `package P { attribute x = "hello"; }`)
}

func TestExprAddIntegerAndStringRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { 1 + "s" } }`,
		"operator '+' is not defined for Natural and String")
}

func TestExprStringConcatenationOK(t *testing.T) {
	wantNoDiags(t, `package P { calc def c { "a" + "b" } }`)
}

func TestExprIntegerRealMixOK(t *testing.T) {
	wantNoDiags(t, `package P { calc def c { 1 + 2.5 } }`)
}

func TestExprDivisionOfStringsRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { "a" / "b" } }`,
		"operator '/' requires numeric operands")
}

// The reference implementation evaluates a whole-number quotient — Natural or
// Integer operands alike — as a Rational, so it binds to a Rational feature.
// Integer ** Natural stays Integer.
func TestExprWholeNumberDivisionAndPowerOK(t *testing.T) {
	wantNoDiags(t, `package P {
	attribute q : ScalarValues::Rational = 7 / 2;
	attribute p : ScalarValues::Integer = 3 ** 2;
}`)
}

// A quotient is a Rational whatever it divides (IntegerFunctions::'/' returns Rational),
// so a whole-number feature refuses it as the evaluation would; RationalFunctions::ToInteger
// converts, and a Real-typed feature may hold an Integer, so a reference to one still binds.
func TestExprQuotientDoesNotBindToWholeNumberFeature(t *testing.T) {
	wantDiags(t, `package P {
	attribute i : ScalarValues::Integer = -7;
	attribute q : ScalarValues::Natural = 7 / 2;
	attribute r : ScalarValues::Natural = i / 2;
	attribute s : ScalarValues::Integer = 1.5 / 2;
	attribute neg : ScalarValues::Integer = -(4 / 2);
	attribute pos : ScalarValues::Natural = +(4 / 2);
	calc def IntDiv { return : ScalarValues::Integer = 4 / 2; }
}`,
		"cannot bind Rational value to a feature typed by Natural",
		"cannot bind Rational value to a feature typed by Natural",
		"cannot bind Rational value to a feature typed by Integer",
		"cannot bind Rational value to a feature typed by Integer",
		"cannot bind Rational value to a feature typed by Natural",
		"cannot bind Rational value to a feature typed by Integer")
	wantNoDiags(t, `package P {
	attribute x : ScalarValues::Real = 4;
	attribute i : ScalarValues::Integer = x;
	attribute q : ScalarValues::Rational = 7 / 2;
	attribute w : ScalarValues::Integer = RationalFunctions::ToInteger(4 / 2);
}`)
}

// The quotient's type is still Rational, whatever the operands: it is observable
// where a Boolean is required, and where a disjoint type is.
func TestExprDivisionIsRational(t *testing.T) {
	wantOneDiag(t,
		`package P { constraint def c { 7 / 2 } }`,
		"constraint expression must be Boolean, found Rational")
	wantOneDiag(t,
		`package P {
	attribute i : ScalarValues::Integer = -7;
	constraint def c { i / 2 }
}`,
		"constraint expression must be Boolean, found Rational")
	wantOneDiag(t,
		`package P { constraint def c { 1.5 / 2 } }`,
		"constraint expression must be Boolean, found Rational")
	wantOneDiag(t,
		`package P { attribute s : ScalarValues::String = 7 / 2; }`,
		"cannot bind Rational value to a feature typed by String")
}

// A literal's type is exact, so a decimal that reads as no whole number is
// refused by a whole-number feature.
func TestExprBindDecimalLiteralToNaturalRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute n : ScalarValues::Natural = 2.5; }`,
		"cannot bind Rational value to a feature typed by Natural")
}

func TestExprNotOnIntegerRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { not 3 } }`,
		"operator 'not' requires a Boolean operand")
}

func TestExprAndOnIntegerRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { 1 and true } }`,
		"requires Boolean operands")
}

func TestExprComparisonOfBooleanRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { true < false } }`,
		"operator '<' is not defined for Boolean and Boolean")
}

func TestExprEqualityAcrossDisjointTypesWarns(t *testing.T) {
	diags := exprDiags(t, `package P { calc def c { 1 == "a" } }`)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if diags[0].Severity != SeverityWarning {
		t.Fatalf("expected a warning, got %v", diags[0].Severity)
	}
}

func TestExprInequalityAcrossDisjointTypesWarnsAlwaysTrue(t *testing.T) {
	diags := exprDiags(t, `package P { calc def c { 1 != "a" } }`)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "always true") {
		t.Fatalf("expected an always-true warning, got %q", diags[0].Message)
	}
}

func TestExprNumericEqualityOK(t *testing.T) {
	wantNoDiags(t, `package P { calc def c { 1 == 2.0 } }`)
}

func TestExprConstraintMustBeBoolean(t *testing.T) {
	wantOneDiag(t,
		`package P { constraint def c { 1 + 2 } }`,
		"constraint expression must be Boolean, found Natural")
}

func TestExprBooleanConstraintOK(t *testing.T) {
	wantNoDiags(t, `package P { constraint def c { 1 < 2 } }`)
}

// A condition `all T` holds instances of T, which is no Boolean unless T is one; a
// variation's extent holds values of the usage, typed as it is.
func TestExprExtentConditionMustBeBoolean(t *testing.T) {
	wantOneDiag(t,
		`package P { enum def Color { red; } assert constraint c { all Color } }`,
		"constraint expression must be Boolean, found Color")
	wantOneDiag(t,
		`package P { part def Car; constraint def c { all Car } }`,
		"constraint expression must be Boolean, found Car")
	wantOneDiag(t,
		`package P {
	part def Engine;
	variation part engineChoice : Engine { variant part v4 : Engine; }
	constraint def c { all engineChoice }
}`,
		"constraint expression must be Boolean, found Engine")
	wantNoDiags(t, `package P { constraint def c { all ScalarValues::Boolean } }`)
	wantNoDiags(t, `package P {
	attribute def Flag :> ScalarValues::Boolean;
	constraint def c { all Flag }
}`)
}

func TestExprTransitionGuardMustBeBoolean(t *testing.T) {
	wantOneDiag(t, `package P {
		part def M {
			attribute temp : ScalarValues::Integer = 3;
			state def S {
				state a;
				state b;
				transition first a if temp then b;
			}
		}
	}`, "transition guard must be Boolean, found Integer")
}

// A change-event condition is a bare expression in the parsed tree, so it must
// be checked there rather than in the lowered ast.ChangeEvent. A bare name is
// left alone: lowering reads it as a signal trigger, not a condition.
func TestExprChangeEventConditionMustBeBoolean(t *testing.T) {
	wantOneDiag(t, `package P {
		part def M {
			attribute temp : ScalarValues::Integer = 3;
			state def S {
				state a;
				state b;
				transition first a when temp + 1 then b;
			}
		}
	}`, "a 'when' trigger's condition must be Boolean, found Integer")
}

func TestExprChangeEventConditionOK(t *testing.T) {
	wantNoDiags(t, `package P {
		part def M {
			attribute temp : ScalarValues::Integer = 3;
			state def S {
				state a;
				state b;
				transition first a when temp > 5 then b;
			}
		}
	}`)
}

func TestExprTimedTransitionDelayIsNotACondition(t *testing.T) {
	wantNoDiags(t, `package P {
		part def M {
			attribute period : ScalarValues::Integer = 10;
			state def S {
				state a;
				accept after 10 then b;
				state b;
				accept at period then a;
			}
		}
	}`)
}

func TestExprAcceptWhenConditionMustBeBoolean(t *testing.T) {
	wantOneDiag(t, `package P {
		part def M {
			attribute temp : ScalarValues::Integer = 3;
			state def S {
				state a;
				accept when temp then b;
				state b;
			}
		}
	}`, "a 'when' trigger's condition must be Boolean, found Integer")
}

func TestExprTransitionGuardComparisonOK(t *testing.T) {
	wantNoDiags(t, `package P {
		part def M {
			attribute temp : ScalarValues::Integer = 3;
			state def S {
				state a;
				state b;
				transition first a if temp > 5 then b;
			}
		}
	}`)
}

const calcAdd = `calc def add {
	in a : ScalarValues::Integer;
	in b : ScalarValues::Integer;
	a + b
}
`

// A call leaving a default-less parameter unbound is well formed (KerML 1.0 §8.3.4.8.8) but
// cannot be evaluated, so it is advised of each parameter it leaves unbound.
func TestExprInvocationTooFewArguments(t *testing.T) {
	wantOneWarning(t,
		`package P { `+calcAdd+` calc c { add(1) } }`,
		CodeUnboundParameter, "add leaves parameter b unbound, so the call cannot be evaluated")
	// Every unbound parameter is named, and a supplied argument is still typed.
	diags := exprDiags(t, `package P { `+calcAdd+` calc c { add() } }`)
	if len(diags) != 2 || !strings.Contains(diags[0].Message, "parameter a unbound") ||
		!strings.Contains(diags[1].Message, "parameter b unbound") {
		t.Fatalf("expected an advisory per unbound parameter, got %v", diags)
	}
	diags = exprDiags(t, `package P { `+calcAdd+` calc c { add("one") } }`)
	if len(diags) != 2 || !strings.Contains(diags[0].Message, "parameter b unbound") ||
		!strings.Contains(diags[1].Message, "argument 1 of add expects Integer, found String") {
		t.Fatalf("expected the advisory and the mismatch, got %v", diags)
	}
}

func TestExprInvocationTooManyArguments(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(1, 2, 3) } }`,
		"add takes 2 argument(s), found 3")
}

// An argument expression is typed once, so an error inside it is reported once.
func TestExprInvocationArgumentErrorReportedOnce(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(1 + "s", 2) } }`,
		`operator '+' is not defined for Natural and String`)
}

func TestExprInvocationCorrectArityOK(t *testing.T) {
	wantNoDiags(t, `package P { `+calcAdd+` calc c { add(1, 2) } }`)
}

// A parameter whose multiplicity admits no value may go without an argument,
// as the library's `'-'(x)` and two-argument `'if'` do; one declaring `[1]` is advised of.
func TestExprInvocationOptionalParameterMayBeOmitted(t *testing.T) {
	const model = `package P {
		calc def scale {
			in x : ScalarValues::Integer;
			in by : ScalarValues::Integer[0..1];
			x
		}
		calc c { scale(%s) }
	}`
	wantNoDiags(t, fmt.Sprintf(model, `2`))
	wantNoDiags(t, fmt.Sprintf(model, `2, 3`))
	wantOneWarning(t, fmt.Sprintf(model, ``), CodeUnboundParameter, "scale leaves parameter x unbound")
	wantNoDiags(t, `package P { calc c { IntegerFunctions::'-'(5) } }`)
	wantNoDiags(t, `package P { calc c { ControlFunctions::'if'(false, 1) } }`)
}

func TestExprInvocationRedefinedParameterKeepsInheritedOptionality(t *testing.T) {
	// A redefinition stating no multiplicity or default keeps the inherited ones.
	const model = `package P {
		calc def Scale {
			in x : ScalarValues::Integer;
			in by : ScalarValues::Integer[0..1];
			in times : ScalarValues::Integer = 1;
			x
		}
		calc def Scaled :> Scale {
			in redefines x;
			in redefines by;
			in redefines times : ScalarValues::Integer;
			x
		}
		calc def Tight :> Scale {
			in redefines x;
			in redefines by : ScalarValues::Integer[1];
			x
		}
		calc c { %s }
	}`
	wantNoDiags(t, fmt.Sprintf(model, `Scaled(2)`))
	wantNoDiags(t, fmt.Sprintf(model, `Scaled(2, 3, 4)`))
	wantOneWarning(t, fmt.Sprintf(model, `Scaled()`), CodeUnboundParameter, "Scaled leaves parameter x unbound")
	wantOneWarning(t, fmt.Sprintf(model, `Tight(2)`), CodeUnboundParameter, "Tight leaves parameter by unbound")
}

func TestExprInvocationOptionalBeforeRequiredParameter(t *testing.T) {
	// Positional arguments bind in order, so an omittable parameter ahead of a
	// required one does not stand in for it.
	const model = `package P {
		calc def Scale {
			in by : ScalarValues::Integer[0..1];
			in offset : ScalarValues::Integer = 0;
			in x : ScalarValues::Integer;
			x
		}
		calc def Scaled :> Scale {
			in redefines by;
			in redefines offset;
			x
		}
		calc c { %s }
	}`
	wantNoDiags(t, fmt.Sprintf(model, `Scale(2, 0, 5)`))
	wantOneWarning(t, fmt.Sprintf(model, `Scale(5)`), CodeUnboundParameter, "Scale leaves parameter x unbound")
	wantOneWarning(t, fmt.Sprintf(model, `Scaled(5)`), CodeUnboundParameter, "Scaled leaves parameter x unbound")
	wantNoDiags(t, fmt.Sprintf(model, `Scale(x = 5)`))
}

func TestExprInvocationThroughAliasChecksArguments(t *testing.T) {
	const model = `package P {
		` + calcAdd + `
		alias addAlias for add;
		calc c { addAlias(%s) }
	}`
	wantOneWarning(t, fmt.Sprintf(model, `1`), CodeUnboundParameter, "add leaves parameter b unbound")
	wantOneDiag(t, fmt.Sprintf(model, `1, "two"`),
		"argument 2 of add expects Integer, found String")
	wantNoDiags(t, fmt.Sprintf(model, `1, 2`))
}

func TestExprInvocationArgumentTypeMismatch(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(1, "two") } }`,
		`argument 2 of add expects Integer, found String`)
}

// An argument binds to its parameter as a value binds to a feature: a decimal
// literal and a quotient are no Integer, a Real feature's value may be one.
func TestExprInvocationArgumentNarrowerThanExpression(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(1, 2.5) } }`,
		`argument 2 of add expects Integer, found Rational`)
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(7 / 2, 1) } }`,
		`argument 1 of add expects Integer, found Rational`)
	wantNoDiags(t, `package P {
		`+calcAdd+`
		attribute w : ScalarValues::Real = 1.5;
		calc c { add(RationalFunctions::ToInteger(7 / 2), w) }
	}`)
}

func TestExprInvocationDefaultedParameterOptional(t *testing.T) {
	wantNoDiags(t, `package P {
		calc def scale {
			in a : ScalarValues::Integer;
			in factor : ScalarValues::Integer = 2;
			a * factor
		}
		calc c { scale(3) }
	}`)
}

func TestExprInvocationUnknownNamedArgument(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(a = 1, c = 2) } }`,
		`add has no parameter named "c"`)
}

// A receiver binds by position, so a call whose arguments bind by name states
// no parameter for it (runtime/eval.go reports the same call).
func TestExprInvocationReceiverWithNamedArguments(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { 1->add(a = 1, b = 2) } }`,
		"add cannot be called with a receiver and named arguments")
	// The receiver may be the missing argument, so nothing else is reported.
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { 1->add(b = 2) } }`,
		"add cannot be called with a receiver and named arguments")
}

// calcHolder owns a calc feature reached through a feature chain, `holder.scale`.
const calcHolder = `part def Holder {
	calc scale { in x : ScalarValues::Real; in k : ScalarValues::Real = 2.0; return : ScalarValues::Real = x * k; }
}
part holder : Holder;
`

// A chain call `holder.scale(3.0)` applies the calc feature the chain names: its
// arguments are held to that calc's inputs and the call is typed by its result.
func TestExprChainInvocationChecked(t *testing.T) {
	wantNoDiags(t, `package P { `+calcHolder+` attribute a : ScalarValues::Real = holder.scale(3.0); }`)
	wantNoDiags(t, `package P { `+calcHolder+` attribute a : ScalarValues::Real = holder.scale(x = 3.0, k = 4.0); }`)
	wantOneDiag(t,
		`package P { `+calcHolder+` attribute a : ScalarValues::Real = holder.scale("three"); }`,
		"argument 1 of scale expects Real, found String")
	wantOneDiag(t,
		`package P { `+calcHolder+` attribute a : ScalarValues::Real = holder.scale(x = 3.0, factor = 4.0); }`,
		`scale has no parameter named "factor"`)
	wantOneDiag(t,
		`package P { `+calcHolder+` attribute a : ScalarValues::Real = holder.scale(1.0, 2.0, 3.0); }`,
		"scale takes 2 argument(s), found 3")
	wantOneWarning(t,
		`package P { `+calcHolder+` attribute a : ScalarValues::Real = holder.scale(); }`,
		CodeUnboundParameter, "scale leaves parameter x unbound, so the call cannot be evaluated")
	wantOneDiag(t,
		`package P { `+calcHolder+` attribute flag : ScalarValues::Boolean = holder.scale(3.0); }`,
		"cannot bind Real value to a feature typed by Boolean")
	wantOneDiag(t,
		`package P { part def Box { attribute n : ScalarValues::Real; } part box : Box; attribute a : ScalarValues::Real = box.n(3.0); }`,
		"Must invoke a behavior or a behavioral feature")
}

func TestExprInvocationNamedArgumentsOK(t *testing.T) {
	wantNoDiags(t, `package P { `+calcAdd+` calc c { add(a = 1, b = 2) } }`)
	wantNoDiags(t, `package P { `+calcAdd+` calc c { add(b = 2, a = 1) } }`)
}

// Arguments that bind by name are held to the parameters as positional ones
// are: each parameter once, in its type, and a default-less one left unbound advised of.
func TestExprInvocationNamedArgumentsChecked(t *testing.T) {
	wantOneWarning(t,
		`package P { `+calcAdd+` calc c { add(a = 1) } }`,
		CodeUnboundParameter, "add leaves parameter b unbound, so the call cannot be evaluated")
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(a = 1, a = 2, b = 3) } }`,
		`add binds parameter "a" twice`)
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(b = "two", a = 1) } }`,
		"argument b of add expects Integer, found String")
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(a = 1, c = 2) } }`,
		`add has no parameter named "c"`)
	const model = `package P {
		calc def Scale {
			in factor : ScalarValues::Integer[0..1];
			in offset : ScalarValues::Integer = 0;
			in x : ScalarValues::Integer;
			x
		}
		calc c { %s }
	}`
	wantNoDiags(t, fmt.Sprintf(model, `Scale(x = 5)`))
	wantNoDiags(t, fmt.Sprintf(model, `Scale(offset = 1, x = 5, factor = 2)`))
	wantOneWarning(t, fmt.Sprintf(model, `Scale(factor = 2, offset = 1)`),
		CodeUnboundParameter, "Scale leaves parameter x unbound")
}

// A node's body parameters redefine the invoked ones by position, so one the
// body binds a value to is not asked of the call, wherever the node is written.
func TestExprNodeBodyParameterSuppliesArgument(t *testing.T) {
	const model = `package P {
		action def Scale { in a : ScalarValues::Integer; in b : ScalarValues::Integer; }
		action outer {
			attribute seven : ScalarValues::Integer = 7;
			%s
		}
	}`
	wantNoDiags(t, fmt.Sprintf(model, `action scaled = Scale(a = seven) { in a = 1; in b = a * 10; }`))
	wantNoDiags(t, fmt.Sprintf(model, `action scaled = Scale(seven) { in a = 1; in b = a * 10; }`))
	wantNoDiags(t, fmt.Sprintf(model, `action scaled = Scale(a = seven) { in a; in b = 10; }`))
	wantNoDiags(t, fmt.Sprintf(model, `action scaled = Scale(a = seven) { in x = 1; in y redefines b = 10; }`))
	wantNoDiags(t, fmt.Sprintf(model, `action inner { action scaled = Scale(a = seven) { in a; in b = 10; } }`))
	wantNoDiags(t, `package P {
		action def Scale { in a : ScalarValues::Integer; in b : ScalarValues::Integer; }
		state def M {
			attribute seven : ScalarValues::Integer = 7;
			state s { entry action { action scaled = Scale(a = seven) { in a; in b = 10; } } }
		}
	}`)
	wantOneWarning(t, fmt.Sprintf(model, `action scaled = Scale(a = seven) { in a; in b; }`),
		CodeUnboundParameter, "Scale leaves parameter b unbound")
	wantOneWarning(t, fmt.Sprintf(model, `action scaled = Scale(a = seven) { in a = 1; }`),
		CodeUnboundParameter, "Scale leaves parameter b unbound")
	wantOneDiag(t, fmt.Sprintf(model, `action scaled = Scale(a = seven) { in b = "x"; }`),
		`Scale has no parameter named "a"`)
}

func TestExprLiteralConformsToNatural(t *testing.T) {
	wantNoDiags(t, `package P {
		attribute n : ScalarValues::Natural = 3;
		attribute r : ScalarValues::Rational = 1.5;
	}`)
}

// A signed literal spells its value out, so `-3` is refused by a Natural feature
// as a decimal literal is; a negated Integer feature may be a Natural.
func TestExprNegatedLiteralIsNotNatural(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute n : ScalarValues::Natural = -3; }`,
		"cannot bind Integer value to a feature typed by Natural")
	wantNoDiags(t, `package P {
		attribute i : ScalarValues::Integer = -3;
		attribute n : ScalarValues::Natural = -i;
	}`)
}

func TestExprArrowFormReceiverCountsAsFirstArgument(t *testing.T) {
	wantNoDiags(t, `package P { `+calcAdd+` calc c { 1->add(2) } }`)
}

func TestExprArrowFormArityStillChecked(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { 1->add(2, 3) } }`,
		"add takes 2 argument(s), found 3")
}

func TestExprInheritedParametersCounted(t *testing.T) {
	wantNoDiags(t, `package P {
		`+calcAdd+`
		calc def Add2 :> add;
		calc c { Add2(1, 2) }
	}`)
}

// A specialization may redefine a subset of the inherited parameters; the
// signature is still the full inherited one.
func TestExprPartiallyRedefinedParametersKeepInheritedSignature(t *testing.T) {
	wantNoDiags(t, `package P {
		`+calcAdd+`
		calc def AddPositive :> add {
			in a :>> a : ScalarValues::Real;
		}
		calc c { AddPositive(1, 2) }
	}`)
}

// Parameters inherited from several supertypes keep the order the supertypes
// were declared in, so `first` precedes `second` here.
func TestExprMultipleSupertypesKeepDeclarationOrder(t *testing.T) {
	const model = `package P {
		calc def First { in first : ScalarValues::String; }
		calc def Second { in second : ScalarValues::Integer; }
		calc def Both :> First, Second;
		calc c { Both(%s) }
	}`
	wantNoDiags(t, fmt.Sprintf(model, `"s", 1`))
	// Were the supertypes folded in reverse, `second` would come first and the
	// String would be reported against it as argument 1 instead.
	wantOneDiag(t, fmt.Sprintf(model, `1, "s"`),
		"argument 2 of Both expects Integer, found String")
}

// A redefined parameter's own type is what its argument is checked against.
func TestExprRedefinedParameterTypeChecked(t *testing.T) {
	wantOneDiag(t, `package P {
		`+calcAdd+`
		calc def AddText :> add {
			in a :>> a : ScalarValues::String;
		}
		calc c { AddText(1, 2) }
	}`, "argument 1 of AddText expects String, found Natural")
}

// A specialization may refine an inherited parameter under a new name, which
// redefines it by position instead of extending the signature.
func TestExprRenamedParameterRedefinesByPosition(t *testing.T) {
	const model = `package P {
		` + calcAdd + `
		calc def Convert :> add {
			in x : ScalarValues::String;
		}
		calc c { Convert(%s) }
	}`
	// `x` redefines `a`, so the signature stays (x, b), not (a, b, x).
	wantOneDiag(t, fmt.Sprintf(model, `1, 2`),
		"argument 1 of Convert expects String, found Natural")
	wantNoDiags(t, fmt.Sprintf(model, `"s", 2`))
}

// A redeclaration matches the inherited parameter at its own position, even
// when its name is that of a different inherited parameter — the same rule
// semantics/redefinition.go applies, so both tiers see one signature.
func TestExprRedeclaredParametersMatchByPositionNotName(t *testing.T) {
	const model = `package P {
		calc def Swap { in a : ScalarValues::String; in b : ScalarValues::Integer; }
		calc def Swapped :> Swap { in b : ScalarValues::String; in a : ScalarValues::Integer; }
		calc c { Swapped(%s) }
	}`
	wantNoDiags(t, fmt.Sprintf(model, `"s", 1`))
	// Matched by name instead, the signature would be (a : Integer, b : String)
	// and this would report against argument 2.
	wantOneDiag(t, fmt.Sprintf(model, `1, 1`),
		"argument 1 of Swapped expects String, found Natural")
}

// An `out` parameter occupies a position, so an input declared after one
// redefines the inherited parameter at that position, not the first. The input
// at the position the output took keeps its place in the list, since an output
// does not redefine an input — the list semantics.Model.parametersOf derives.
func TestExprOutParameterOccupiesAPosition(t *testing.T) {
	const model = `package P {
		calc def C { in a : ScalarValues::String; in b : ScalarValues::Integer; }
		calc def D :> C { out y; in x : ScalarValues::Integer; }
		calc c { D(%s) }
	}`
	// D's parameters are (y, x, a): `x` is at position 1, so it redefines `b`,
	// and `a` is inherited because `out y` does not redefine it.
	wantNoDiags(t, fmt.Sprintf(model, `1, "s"`))
	wantOneDiag(t, fmt.Sprintf(model, `"s", "s"`),
		"argument 1 of D expects Integer, found String")

	// Adding only an output leaves the inherited signature untouched.
	wantNoDiags(t, `package P {
		calc def C { in a : ScalarValues::Integer; in b : ScalarValues::Integer; }
		calc def D :> C { out y; }
		calc c { D(1, 2) }
	}`)
}

// An explicit `:>>` naming a parameter at another position claims that one, so
// the parameter left to inherit is the one no declaration redefines — again the
// list semantics.Model.parametersOf derives.
func TestExprExplicitRedefinitionClaimsItsTarget(t *testing.T) {
	const model = `package P {
		calc def C { in a : ScalarValues::String; in b : ScalarValues::Integer; }
		calc def D :> C { in bb :>> b; }
		calc c { D(%s) }
	}`
	// D's parameters are (bb, a), not (bb, b).
	wantNoDiags(t, fmt.Sprintf(model, `1, "s"`))
	wantOneDiag(t, fmt.Sprintf(model, `1, 1`),
		"argument 2 of D expects String, found Natural")
}

// A declaration claims the inherited parameter at its own position, not the
// next unclaimed one, so an explicit `:>>` further along does not shift the
// declarations after it.
func TestExprPositionalClaimIsByDeclarationIndex(t *testing.T) {
	const model = `package P {
		calc def C { in a : ScalarValues::String; in b : ScalarValues::Integer; in c : ScalarValues::Boolean; }
		calc def D :> C { in z :>> c; in w; }
		calc c { D(%s) }
	}`
	// D's parameters are (z, w, a): `z` claims `c`, `w` claims `b` by position.
	wantOneDiag(t, fmt.Sprintf(model, `true, 1, "s", 1`),
		"D takes 3 argument(s), found 4")
	wantNoDiags(t, fmt.Sprintf(model, `true, 1, "s"`))
}

func TestExprTypedCalcUsageInheritsParameters(t *testing.T) {
	wantNoDiags(t, `package P {
		`+calcAdd+`
		calc myAdd : add;
		calc c { myAdd(1, 2) }
	}`)
}

func TestExprParameterlessCalcRejectsArguments(t *testing.T) {
	wantOneDiag(t, `package P {
		calc def zero { 0 }
		calc c { zero(1) }
	}`, "zero takes 0 argument(s), found 1")
}

func TestExprUnresolvedNameProducesNoTypeDiagnostic(t *testing.T) {
	wantNoDiags(t, `package P { attribute x : ScalarValues::Integer = missing; }`)
}

// A type with no scalar ancestor is outside the lattice, so the scalar rules
// say nothing about it — but no literal is an instance of it either, which the
// value rules report (see typecheck_value_test.go).
func TestExprLiteralBoundToNonScalarType(t *testing.T) {
	wantOneDiag(t, `package P {
		attribute def Mass;
		part def Car { attribute m : Mass = 5; }
	}`, "cannot bind Natural value to a feature typed by Mass")
}

// Reaching a scalar through a user-declared type keeps the lattice rules, and
// their more precise message, in force.
func TestExprScalarSpecializationUsesLattice(t *testing.T) {
	wantOneDiag(t, `package P {
		attribute def Mass specializes ScalarValues::Integer;
		part def Car { attribute m : Mass = 5.5; }
	}`, "cannot bind Rational value to a feature typed by Integer")
}

// A body `{ … }` written as a value is the Expression itself, whatever its
// result would be, so it binds to no scalar and to no data type; the pilot
// reports each binding below. A body a type accepts, or an untyped feature,
// is left alone.
func TestExprBodyValueIsTheExpression(t *testing.T) {
	const prelude = `package P {
		private import ScalarValues::*;
		private import ISQ::*;
		private import SI::*;
		attribute def Temp;
		calc def KT { in t : Temp; return : Integer = 1; }
		calc def KA { in a : Base::Anything; return : Integer = 1; }
		calc def KE { in e : Performances::Evaluation; return : Integer = 1; }
		calc def KF { in calc f : Performances::Evaluation; return : Integer = 1; }
		calc def KC { in c : Base::Anything[0..*]; return : Integer = 1; }
		package A { calc def K { in t : Temp; return : Integer = 1; } }
		package B { calc def K { in e : Performances::Evaluation; return : Integer = 2; } }
		part def Box { attribute inner : Temp; }
		part def H {
			private import A::*;
			private import B::*;
			attribute flag : Boolean;
			attribute n : Integer;
			%s
		}
	}`
	for _, tc := range []struct{ decl, want string }{
		{"attribute b : Boolean = { 5 };", "cannot bind Expression value to a feature typed by Boolean"},
		{"attribute b : Boolean = { true };", "cannot bind Expression value to a feature typed by Boolean"},
		{"attribute i : Integer = { 5 };", "cannot bind Expression value to a feature typed by Integer"},
		{"attribute i : Integer = { true };", "cannot bind Expression value to a feature typed by Integer"},
		{"attribute s : String = { 5 };", "cannot bind Expression value to a feature typed by String"},
		{"attribute d : DurationValue = { 5 [s] };", "cannot bind Expression value to a feature typed by DurationValue"},
		{"attribute t : Temp = { 5 };", "cannot bind Expression value to a feature typed by Temp"},
		{"attribute i : Integer = 1 + { 5 };", "operator '+' is not defined for Natural and Expression"},
		{"attribute i : Integer = -{ 5 };", "operator '-' requires a numeric operand, found Expression"},
		{"attribute b : Boolean = 5 < { 5 };", "operator '<' is not defined for Natural and Expression"},
		{"attribute b : Boolean = not { true };", "operator 'not' requires a Boolean operand, found Expression"},
		{"attribute b : Boolean = { true } and flag;", "operator 'and' requires Boolean operands, found Expression and Boolean"},
		{"attribute b : Boolean = { true } == true;", "comparing Expression with Boolean is always false"},
		{"attribute a : Base::Anything = { \"x\" + 1 };", "operator '+' is not defined for String and Natural"},
		{"action a { assign n := { 5 }; }", "cannot bind Expression value to a feature typed by Integer"},
		{"attribute i : Integer = KT({ 5 });", "argument 1 of KT expects Temp, found Evaluation"},
		{"attribute i : Integer = KT(t = { 5 });", "argument t of KT expects Temp, found Evaluation"},
		{"attribute i : Integer = KT(({ 5 }, { 6 }));", "argument 1 of KT expects Temp, found Evaluation"},
		{"part b : Box = new Box({ 5 });", "inner of Box is typed by Temp; cannot bind a value of type Evaluation"},
	} {
		diags := libraryTypeDiags(t, fmt.Sprintf(prelude, tc.decl))
		if len(diags) != 1 || !strings.Contains(diags[0].Message, tc.want) {
			t.Errorf("%s: want one diagnostic containing %q, got %v", tc.decl, tc.want, diags)
		}
	}
	for _, decl := range []string{
		"attribute a : Base::Anything = { 5 };",
		"expr e = { 5 };",
		"attribute u = { 5 };",
		"attribute b : Boolean = flag.{in f; f};",
		"calc def K { in expr f; return : Integer = f(); } attribute i : Integer = K({ 5 });",
		"attribute i : Integer = KA({ 5 });",
		"attribute i : Integer = KE({ 5 });",
		"attribute i : Integer = KF({ 5 });",
		"attribute i : Integer = KC({ 5 });",
		"attribute i : Integer = K({ 5 });",
	} {
		if diags := libraryTypeDiags(t, fmt.Sprintf(prelude, decl)); len(diags) != 0 {
			t.Errorf("%s: want no diagnostics, got %v", decl, diags)
		}
	}
}
