package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// triggerFixture declares the durations, time instants, quantities and
// calculations the trigger cases below name; %s is the state def body.
const triggerFixture = `package P {
	private import ISQ::*;
	private import SI::*;
	private import SIPrefixes::milli;
	private import MeasurementReferences::*;
	private import Time::*;
	private import ScalarValues::*;
	private import Overloads::Durations::*;
	private import Overloads::Instants::*;
	private import Overloads::Flags::*;
	private import Overloads::Lengths::*;

	package Overloads {
		package Durations { calc def Pick { in d : DurationValue; return : DurationValue = d; } }
		package Instants { calc def Pick { in t : TimeInstantValue; return : TimeInstantValue = t; } }
		package Flags { calc def Pick { in b : Boolean; return : Boolean = b; } }
		package Lengths { calc def Pick { in l : LengthValue; return : LengthValue = l; } }
	}

	attribute <ms> millisecond : DurationUnit {
		:>> unitConversion : ConversionByPrefix { :>> prefix = milli; :>> referenceUnit = s; }
	}
	attribute def Delay :> DurationValue;

	part def Holder {
		attribute delay : Delay;
		attribute instant : Iso8601DateTime;
		attribute ok : Boolean;
	}
	calc def Twice { in d : DurationValue; return : DurationValue = d + d; }
	calc def Timed { in e : Performances::Evaluation; return : DurationValue = 1 [s]; }
	calc def Later { in t : TimeInstantValue; return : TimeInstantValue = t; }
	calc def Len { return : LengthValue; }
	calc def IsOk { return : Boolean; }

	state def Base {
		attribute d : DurationValue;
		attribute t : TimeInstantValue;
	}
	state def S :> Base {
		attribute :>> d;
		attribute :>> t;
		attribute d2 : DurationValue :> d;
		attribute t2 : TimeInstantValue :> t;
		attribute x : Integer;
		attribute len : LengthValue;
		attribute flag : Boolean;
		attribute untyped;
		attribute ready = false;
		attribute lazy default false;
		attribute count = 3;
		attribute total = count + count;
		attribute label : String;
		attribute wait = 5 [s];
		attribute waitTwice = d + d;
		attribute alsoReady = ready;
		attribute loop = loop;
		attribute pair = (true, false);
		attribute pairWait = (5 [s], 6 [s]);
		attribute flags : Boolean[2];
		attribute counts : Integer[2];
		attribute waits : DurationValue[2];
		attribute times : TimeInstantValue[2];
		attribute firstFlag = flags#(1);
		attribute firstWait = waits#(1);
		in attribute given = true;
		part holder : Holder;
		attribute viaDelay :> holder.delay;
		attribute viaInstant :> holder.instant;
		attribute viaOk :> holder.ok;
		state a;
		state b;
		%s
	}
}`

func triggerDiags(t *testing.T, body string) []Diagnostic {
	t.Helper()
	return libraryTypeDiags(t, strings.Replace(triggerFixture, "%s", body, 1))
}

// wantTriggerDiag asserts exactly one type error, under code, whose message
// contains want.
func wantTriggerDiag(t *testing.T, body, code, want string) {
	t.Helper()
	diags := triggerDiags(t, body)
	if len(diags) != 1 {
		t.Fatalf("want one type diagnostic for %q, got %v", body, diags)
	}
	d := diags[0]
	if d.Code != code || d.Severity != SeverityError {
		t.Errorf("%q: got code %q severity %v, want %q error", body, d.Code, d.Severity, code)
	}
	if !strings.Contains(d.Message, want) {
		t.Errorf("%q: message %q does not contain %q", body, d.Message, want)
	}
}

func wantTriggerSilent(t *testing.T, body string) {
	t.Helper()
	if diags := triggerDiags(t, body); len(diags) != 0 {
		t.Fatalf("want no type diagnostics for %q, got %v", body, diags)
	}
}

// The argument of `after` must be a DurationValue (SysML v2 §8.3.17,
// TriggerInvocationExpression; pilot validateTriggerInvocationActionAfterArgument).
func TestTriggerAfterRejectsNonDuration(t *testing.T) {
	for _, tc := range []struct{ trigger, found string }{
		{"after 5", "Natural"},
		{"after 5.5", "Rational"},
		{"after \"soon\"", "String"},
		{"after flag", "Boolean"},
		{"after x", "Integer"},
		{"after len", "LengthValue"},
		{"after t", "TimeInstantValue"},
		{"after holder.instant", "Iso8601DateTime"},
		{"after untyped", "an untyped feature"},
		{"after count", "Natural"},
		{"after wait", "ScalarQuantityValue"},
		{"after waitTwice", "ScalarQuantityValue"},
		{"after 5 [m]", "a quantity in metre (a LengthUnit)"},
		{"after 5 [kg]", "a quantity in kilogram (a MassUnit)"},
		{"after d * d", "a value of dimension T^2"},
		{"after d ** 2", "a value of dimension T^2"},
		{"after 2 ** d", "NumericalValue"},
		{"after 5 [m] / 2 [s]", "a value of dimension L·T^-1"},
		{"after Len()", "LengthValue"},
		{"after IsOk()", "Boolean"},
		{"after 5 % 2", "NumericalValue"},
		{"after x % 2", "NumericalValue"},
		{"after 5 + 5", "NumericalValue"},
		{"after -5", "NumericalValue"},
		{"after x + 1", "NumericalValue"},
		{"after 2 * d", "NumericalValue"},
		{"after d / 2", "NumericalValue"},
		{"after total", "NumericalValue"},
		{"after label + label", "String"},
		{"after (if flag ? 5 else 6)", "the result of `if`, typed Anything"},
		{"after (if flag ? d else d2)", "the result of `if`, typed Anything"},
		{"after (if flag ? 5 [s] else 6 [s])", "the result of `if`, typed Anything"},
		{"after x ?? 5", "the result of `??`, typed Anything"},
		{"after d ?? 5 [s]", "the result of `??`, typed Anything"},
	} {
		body := "transition first a accept " + tc.trigger + " then b;"
		wantTriggerDiag(t, body, "trigger-after-duration",
			"an 'after' trigger's delay must be a DurationValue, found "+tc.found+": write it with a duration unit (`after 5 [s]`)")
	}
}

func TestTriggerAfterAcceptsDurations(t *testing.T) {
	for _, trigger := range []string{
		"after 5 [s]",
		"after 5 [ms]",
		"after 5.5 [min]",
		"after 2 [h]",
		"after d",
		"after d2",
		"after holder.delay",
		"after viaDelay",
		"after d + 5 [s]",
		"after 2 [one] * d",
		"after -d",
		"after t2 - t",
		"after Twice(d)",
		"after 10 [m] / 2 [m/s]",
		"after (d ** 2) / d",
		"after (d ^ 2.0) / d",
	} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
}

// A call is typed by the result of the overload its arguments select, not the
// first declaration its name denotes.
func TestTriggerInvocationUsesSelectedOverload(t *testing.T) {
	for _, trigger := range []string{"after Pick(d2)", "at Pick(t2)", "when Pick(flag)"} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
	for _, tc := range []struct{ trigger, code, found string }{
		{"after Pick(t2)", "trigger-after-duration", "TimeInstantValue"},
		{"after Pick(flag)", "trigger-after-duration", "Boolean"},
		{"after Pick(len)", "trigger-after-duration", "LengthValue"},
		{"at Pick(d2)", "trigger-at-time-instant", "DurationValue"},
		{"at Pick(len)", "trigger-at-time-instant", "LengthValue"},
		{"when Pick(d2)", "trigger-when-boolean", "DurationValue"},
		{"when Pick(t2)", "trigger-when-boolean", "TimeInstantValue"},
	} {
		wantTriggerDiag(t, "transition first a accept "+tc.trigger+" then b;", tc.code, "found "+tc.found)
	}
}

// Arithmetic over calls is measured through the selected result's quantity type,
// so a computed delay of the wrong dimension is refused like a written one.
func TestTriggerInvocationArithmeticIsJudgedByDimension(t *testing.T) {
	for _, trigger := range []string{
		"after Twice(d) + 5 [s]",
		"after Twice(d) - Pick(d2)",
		"after Len() / 2 [m/s]",
		"after Twice(d) * 2 [one]",
		"after (Twice(d) ** 2) / Pick(d2)",
		"after Pick(len) / Len() * d",
		"at Later(t) + Twice(d)",
		"at Pick(t2) - Pick(d2)",
	} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
	for _, tc := range []struct{ trigger, code, found string }{
		{"after Len() * Len()", "trigger-after-duration", "a value of dimension L^2"},
		{"after Len() ** 2", "trigger-after-duration", "a value of dimension L^2"},
		{"after Len() + 5 [m]", "trigger-after-duration", "a value of dimension L"},
		{"after Pick(len) - Len()", "trigger-after-duration", "a value of dimension L"},
		{"after Len() / Twice(d)", "trigger-after-duration", "a value of dimension L·T^-1"},
		{"after Twice(d) * Pick(d2)", "trigger-after-duration", "a value of dimension T^2"},
		{"after Len() / 2 [m]", "trigger-after-duration", "a dimensionless value"},
		{"at Len() * 2 [one]", "trigger-at-time-instant", "a value of dimension L"},
		{"at Twice(d) * Pick(d2)", "trigger-at-time-instant", "a value of dimension T^2"},
		{"at Len() / Later(t)", "trigger-at-time-instant", "a value of dimension L·T^-1"},
		{"when Len() * Len()", "trigger-when-boolean", "a value of dimension L^2"},
	} {
		wantTriggerDiag(t, "transition first a accept "+tc.trigger+" then b;", tc.code, "found "+tc.found)
	}
}

// Time arithmetic is judged by dimension, as the pilot's isDuration/isTime
// admit it: a sum of instants or of an instant and a duration passes either.
func TestTriggerTimeArithmeticIsJudgedByDimension(t *testing.T) {
	for _, trigger := range []string{
		"after d + d",
		"after t + d",
		"at d + d",
		"at t - t",
	} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
}

// An incommensurable sum is a value of no dimension, so it is refused (as the
// pilot refuses it) next to the operator's warning; an unmeasured operand stays silent.
func TestTriggerIncommensurableArithmeticIsRejected(t *testing.T) {
	for _, tc := range []struct{ trigger, code string }{
		{"after 1 [s] + 1 [m]", "trigger-after-duration"},
		{"after d - len", "trigger-after-duration"},
		{"after (d + len) * 2 [one]", "trigger-after-duration"},
		{"after -(Twice(d) + Len())", "trigger-after-duration"},
		{"at t + 1 [m]", "trigger-at-time-instant"},
		{"at t - Pick(len)", "trigger-at-time-instant"},
		{"at Later(t) + Len() * 2 [one]", "trigger-at-time-instant"},
	} {
		body := "transition first a accept " + tc.trigger + " then b;"
		diags := triggerDiags(t, body)
		if len(diags) != 2 {
			t.Fatalf("want the trigger error and the operator warning for %q, got %v", body, diags)
		}
		var errors, warnings int
		for _, d := range diags {
			switch {
			case d.Severity == SeverityError && d.Code == tc.code && strings.Contains(d.Message, "over incommensurable quantities of dimension"):
				errors++
			case d.Severity == SeverityWarning && strings.Contains(d.Message, "combines incommensurable quantities"):
				warnings++
			}
		}
		if errors != 1 || warnings != 1 {
			t.Errorf("%q: want one %s error and one incommensurable-operator warning, got %v", body, tc.code, diags)
		}
	}
	for _, trigger := range []string{
		"after untyped + 1 [m]",
		"after d + missing",
		"at t - untyped",
		"at Twice(d) + holder.other",
	} {
		diags := triggerDiags(t, "transition first a accept "+trigger+" then b;")
		for _, d := range diags {
			if d.Severity == SeverityError {
				t.Errorf("%q: want no type error for an operand only evaluation measures, got %v", trigger, diags)
			}
		}
	}
}

// The argument of `at` must be a TimeInstantValue (pilot
// validateTriggerInvocationActionAtArgument).
func TestTriggerAtRejectsNonTimeInstant(t *testing.T) {
	for _, tc := range []struct{ trigger, found string }{
		{"at 5", "Natural"},
		{"at 5 [s]", "a quantity in second (a DurationUnit)"},
		{"at d", "DurationValue"},
		{"at holder.delay", "Delay"},
		{"at viaDelay", "Delay"},
		{"at viaOk", "Boolean"},
		{"at x", "Integer"},
		{"at flag", "Boolean"},
		{"at Twice(d)", "DurationValue"},
		{"at d * d", "a value of dimension T^2"},
		{"at x % 2", "NumericalValue"},
		{"at (if flag ? 5 else 6)", "the result of `if`, typed Anything"},
		{"at (if flag ? t else t2)", "the result of `if`, typed Anything"},
		{"at t ?? TimeOf(a)", "the result of `??`, typed Anything"},
	} {
		body := "transition first a accept " + tc.trigger + " then b;"
		wantTriggerDiag(t, body, "trigger-at-time-instant",
			"an 'at' trigger's time must be a TimeInstantValue, found "+tc.found+": name a feature typed by TimeInstantValue")
	}
}

func TestTriggerAtAcceptsTimeInstants(t *testing.T) {
	for _, trigger := range []string{
		"at t",
		"at t2",
		"at holder.instant",
		"at viaInstant",
		"at Later(t)",
		"at TimeOf(a)",
		"at t + d",
		"at t2 - d",
	} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
}

// The argument of `when` must be Boolean (pilot
// validateTriggerInvocationActionWhenArgument).
func TestTriggerWhenRejectsNonBoolean(t *testing.T) {
	for _, tc := range []struct{ trigger, found string }{
		{"when x", "Integer"},
		{"when 5", "Natural"},
		{"when \"yes\"", "String"},
		{"when d", "DurationValue"},
		{"when holder.delay", "Delay"},
		{"when viaDelay", "Delay"},
		{"when viaInstant", "Iso8601DateTime"},
		{"when untyped", "an untyped feature"},
		{"when lazy", "an untyped feature"},
		{"when given", "an untyped feature"},
		{"when count", "Natural"},
		{"when total", "NumericalValue"},
		{"when label + label", "String"},
		{"when wait", "ScalarQuantityValue"},
		{"when x + 1", "Integer"},
		{"when 5 [s]", "a quantity in second (a DurationUnit)"},
		{"when d * 2", "NumericalValue"},
		{"when d * d", "a value of dimension T^2"},
		{"when Twice(d)", "DurationValue"},
		{"when flag ?? ready", "the result of `??`, typed Anything"},
		{"when (if flag ? true else false)", "the result of `if`, typed Anything"},
		{"when (if flag ? ready else x > 3)", "the result of `if`, typed Anything"},
	} {
		body := "transition first a accept " + tc.trigger + " then b;"
		wantTriggerDiag(t, body, "trigger-when-boolean",
			"a 'when' trigger's condition must be Boolean, found "+tc.found+": compare the value (`when x > 3`)")
	}
}

func TestTriggerWhenAcceptsBooleans(t *testing.T) {
	for _, trigger := range []string{
		"when x > 3",
		"when flag",
		"when holder.ok",
		"when viaOk",
		"when x > 3 and flag",
		"when not flag",
		"when x == 1 or holder.ok",
		"when IsOk()",
		"when ready",
		"when alsoReady",
		"when loop",
	} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
}

// A conditional's result is typed Anything: a trigger judges it strictly, an
// ordinary condition leaves it to evaluation, as the pilot does.
func TestConditionalResultIsLeftToEvaluationOutsideTriggers(t *testing.T) {
	wantTriggerSilent(t, "assert constraint { flag ?? ready }")
	wantTriggerSilent(t, "entry action { if flag ?? ready { assign x := 1; } }")
	wantTriggerSilent(t, "assert constraint { if flag ? true else ready }")
	wantTriggerSilent(t, "entry action { if (if flag ? true else ready) { assign x := 1; } }")
	wantTriggerSilent(t, "transition first a if (if flag ? true else ready) then b;")
}

// `null` is the empty value, typed Anything: no trigger argument may be it (the
// pilot rejects all three), while an ordinary value or condition may hold
// nothing, as the pilot accepts.
func TestNullTriggerArgumentIsRejected(t *testing.T) {
	wantTriggerDiag(t, "transition first a accept after null then b;", "trigger-after-duration", "found `null`")
	wantTriggerDiag(t, "transition first a accept after () then b;", "trigger-after-duration", "found `null`")
	wantTriggerDiag(t, "transition first a accept at null then b;", "trigger-at-time-instant", "found `null`")
	wantTriggerDiag(t, "transition first a accept when null then b;", "trigger-when-boolean", "found `null`")
	wantTriggerSilent(t, "attribute empty : ISQ::DurationValue = null;")
	wantTriggerSilent(t, "entry action { if null { assign x := 1; } }")
	wantTriggerSilent(t, "assert constraint { null }")
}

// A sequence `a, b` is typed Anything, so no trigger argument may be one (the
// pilot rejects each shape); a parenthesized single value is that value.
func TestSequenceTriggerArgumentIsRejected(t *testing.T) {
	const found = "found a sequence of 2 elements, typed Anything"
	for _, tc := range []struct{ trigger, code string }{
		{"after (5 [s], 6 [s])", "trigger-after-duration"},
		{"after (d, d)", "trigger-after-duration"},
		{"after (5 [s], 5 [m])", "trigger-after-duration"},
		{"after ((5 [s], 6 [s]))", "trigger-after-duration"},
		{"after pairWait", "trigger-after-duration"},
		{"at (t, t2)", "trigger-at-time-instant"},
		{"at (t, null)", "trigger-at-time-instant"},
		{"when (true, false)", "trigger-when-boolean"},
		{"when (true, true)", "trigger-when-boolean"},
		{"when (true, null)", "trigger-when-boolean"},
		{"when (null, flag)", "trigger-when-boolean"},
		{"when (x > 3, x < 9)", "trigger-when-boolean"},
		{"when ((flag, ready))", "trigger-when-boolean"},
		{"when pair", "trigger-when-boolean"},
	} {
		wantTriggerDiag(t, "transition first a accept "+tc.trigger+" then b;", tc.code, found)
	}
	for _, trigger := range []string{"after (5 [s])", "after (d)", "at (t)", "when (x > 3)", "when (flag)"} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
	wantTriggerSilent(t, "entry action { if (flag, ready) { assign x := 1; } }")
	wantTriggerSilent(t, "assert constraint { (flag, ready) }")
}

// A workspace document that redeclares a library type's qualified name does not
// stand in for the library's declaration, so the trigger rules keep judging.
func TestTriggerTypesSurviveWorkspaceDuplicates(t *testing.T) {
	const fixture = `package ISQBase { attribute def DurationValue; }
	package Time { attribute def TimeInstantValue; }
	package ScalarValues { datatype Boolean; }
	package P {
		state def S {
			state a;
			state b;
			transition first a accept %s then b;
		}
	}`
	for _, tc := range []struct{ trigger, code, found string }{
		{"after 5", "trigger-after-duration", "found Natural"},
		{"after true", "trigger-after-duration", "found Boolean"},
		{"at 5", "trigger-at-time-instant", "found Natural"},
		{"at 5 [SI::s]", "trigger-at-time-instant", "found a quantity in second"},
		{"when 5", "trigger-when-boolean", "found Natural"},
		{"when 5 [SI::s]", "trigger-when-boolean", "found a quantity in second"},
	} {
		diags := libraryTypeDiags(t, strings.Replace(fixture, "%s", tc.trigger, 1))
		if len(diags) != 1 || diags[0].Code != tc.code || !strings.Contains(diags[0].Message, tc.found) {
			t.Errorf("%q beside duplicated library names: got %v, want one %s (%s)", tc.trigger, diags, tc.code, tc.found)
		}
	}
	for _, trigger := range []string{"after 5 [SI::s]", "when true", "when 5 > 3"} {
		if diags := libraryTypeDiags(t, strings.Replace(fixture, "%s", trigger, 1)); len(diags) != 0 {
			t.Errorf("%q beside duplicated library names: got %v, want silence", trigger, diags)
		}
	}
}

// `seq#(i)` is one element of seq, of seq's type; a Collection (every quantity
// value is one) is indexed as Anything. The pilot rejects each shape below.
func TestIndexedTriggerArgument(t *testing.T) {
	const anything = "found an element `#` selects from a Collection (every quantity value is one), typed Anything"
	for _, tc := range []struct{ trigger, code, found string }{
		{"after counts#(1)", "trigger-after-duration", "found Integer"},
		{"after flags#(1)", "trigger-after-duration", "found Boolean"},
		{"after waits#(1)", "trigger-after-duration", anything},
		{"after d#(1)", "trigger-after-duration", anything},
		{"after times#(1)", "trigger-after-duration", anything},
		{"after firstWait", "trigger-after-duration", anything},
		{"at counts#(2)", "trigger-at-time-instant", "found Integer"},
		{"at times#(1)", "trigger-at-time-instant", anything},
		{"at waits#(1)", "trigger-at-time-instant", anything},
		{"when counts#(1)", "trigger-when-boolean", "found Integer"},
		{"when counts#(1)#(1)", "trigger-when-boolean", "found Integer"},
		{"when untyped#(1)", "trigger-when-boolean", "found an untyped feature"},
	} {
		wantTriggerDiag(t, "transition first a accept "+tc.trigger+" then b;", tc.code, tc.found)
	}
	for _, trigger := range []string{
		"when flags#(1)",
		"when flags#(1)#(1)",
		"when firstFlag",
		"when waits#(1) == 0 [s]",
		"after undeclared#(1)",
	} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
}

// `xs.{…}` is the result of collect, typed as its body's result is — Anything where
// the body returns an untyped parameter; `xs.?{…}` keeps elements of xs and is
// typed as xs is. The pilot leaves every collect Anything, so it rejects the
// collect shapes kept silent too, and accepts the select shapes kept silent.
func TestCollectAndSelectTriggerArguments(t *testing.T) {
	const collected = "found a collection `.{…}` maps to, typed Anything"
	const nothing = "found an empty value over a collection holding nothing, typed Anything"
	for _, tc := range []struct{ trigger, code, found string }{
		{"when counts.{in n : Integer; n}", "trigger-when-boolean", "found Integer"},
		{"when flags.{in f; f}", "trigger-when-boolean", collected},
		{"when counts.?{in n; n > 3}", "trigger-when-boolean", "found Integer"},
		{"when holder.ok.{in f; f}", "trigger-when-boolean", collected},
		{"after waits.{in w; w}", "trigger-after-duration", collected},
		{"after counts.{in n : Integer; n}", "trigger-after-duration", "found Integer"},
		{"after counts.?{in n; n > 3}", "trigger-after-duration", "found Integer"},
		{"after times.?{in i; true}", "trigger-after-duration", "found TimeInstantValue"},
		{"at counts.?{in n; true}", "trigger-at-time-instant", "found Integer"},
		{"at times.{in i; i}", "trigger-at-time-instant", collected},
		{"when flags.{in f : Boolean; (f, 1)}", "trigger-when-boolean", "found Natural"},
		{"when flags.{in f; (f, 1)}", "trigger-when-boolean", "found Natural"},
		{"when flags.{in f : Boolean; (f, (1, label))}", "trigger-when-boolean", "found Natural and String"},
		{"when flags.{in f; (f, label)}", "trigger-when-boolean", "found String"},
		{"when counts.{in n : Integer; (n > 3, untyped)}", "trigger-when-boolean", collected},
		{"when counts->ControlFunctions::reduce {in a : Integer; in b : Integer; 5}", "trigger-when-boolean", "found Natural"},
		{"when ()->ControlFunctions::reduce {in a : Integer; in b : Integer; 5}", "trigger-when-boolean", nothing},
		{"when ()->ControlFunctions::collect {in a : Integer; 5}", "trigger-when-boolean", nothing},
		{"when ().{in a : Integer; 5}", "trigger-when-boolean", nothing},
	} {
		wantTriggerDiag(t, "transition first a accept "+tc.trigger+" then b;", tc.code, tc.found)
	}
	for _, trigger := range []string{
		"when counts.{in n; n > 3}",
		"when flags.{in f : Boolean; f}",
		"when flags.{in f : Boolean; (f, true)}",
		"when flags.{in f; (f > 3, true)}",
		"after waits.{in w : DurationValue; w}",
		"when flags.?{in f; f}",
		"when flags.?{in f; f}#(1)",
		"after waits.?{in w; w > 1 [s]}",
		"after d2.?{in w; true}",
		"at times.?{in i; true}",
		"when undeclared.?{in f; f}",
		"when undeclared.{in f; f}",
	} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
	wantTriggerSilent(t, "entry action { if ()->ControlFunctions::reduce {in a : Integer; in b : Integer; 5} { assign x := 1; } }")
	wantTriggerSilent(t, "entry action { if ().{in a : Integer; 5} { assign x := 1; } }")
}

// `{ … }` written as a trigger argument is the expression itself, an Evaluation,
// whatever its result would be; the pilot rejects each shape below.
func TestBodyTriggerArgumentIsRejected(t *testing.T) {
	const body = "found an expression body `{ … }`"
	wantTriggerDiag(t, "transition first a accept when { true } then b;", "trigger-when-boolean", body)
	wantTriggerDiag(t, "transition first a accept when { x > 3 } then b;", "trigger-when-boolean", body)
	wantTriggerDiag(t, "transition first a accept when { 5 } then b;", "trigger-when-boolean", body)
	wantTriggerDiag(t, "transition first a accept after { 5 [s] } then b;", "trigger-after-duration", body)
	wantTriggerDiag(t, "transition first a accept after { d } then b;", "trigger-after-duration", body)
	wantTriggerDiag(t, "transition first a accept at { t } then b;", "trigger-at-time-instant", body)
	wantTriggerDiag(t, "entry action { accept when { flag }; }", "trigger-when-boolean", body)
	wantTriggerDiag(t, "transition first a if { true } then b;", "type.expr", body)
	wantTriggerDiag(t, "transition first a accept when ({ true }) then b;", "trigger-when-boolean", body)
	wantTriggerDiag(t, "transition first a accept when { true }#(1) then b;", "trigger-when-boolean", body)
	wantTriggerDiag(t, "transition first a accept when not { true } then b;", "type.expr", "operator 'not' requires a Boolean operand, found Expression")
	wantTriggerDiag(t, "transition first a accept when { true } and flag then b;", "type.expr", "operator 'and' requires Boolean operands, found Expression and Boolean")
	wantTriggerDiag(t, "transition first a accept after { 5 [s] } + d then b;", "trigger-after-duration", "`+` over an expression body `{ … }`")
	wantTriggerDiag(t, "transition first a accept at { t } + d then b;", "trigger-at-time-instant", "`+` over an expression body `{ … }`")
	wantTriggerSilent(t, "transition first a accept when flags->ControlFunctions::forAll {in f; f} then b;")
	diags := triggerDiags(t, "transition first a accept when { true } == true then b;")
	if len(diags) != 1 || diags[0].Severity != SeverityWarning || !strings.Contains(diags[0].Message, "comparing Expression with Boolean is always false") {
		t.Errorf("`{ true } == true`: want the equality warning alone, got %v", diags)
	}
}

// A declaration a trigger argument's body makes is typed as one in any other
// body, and a trigger written inside it is checked; the pilot rejects each.
func TestTriggerBodyMembersAreTyped(t *testing.T) {
	const bad = "cannot bind Natural value to a feature typed by Boolean"
	for _, tc := range []struct{ trigger, want string }{
		{"accept after Timed({ attribute bad : Boolean = 5; d }) then b;", bad},
		{"accept when flags->ControlFunctions::forAll { in f; attribute bad : Boolean = 5; f } then b;", bad},
		{"accept when flags->ControlFunctions::forAll { in f; attribute wait : DurationValue = 5; f } then b;", "cannot bind Natural value to a feature typed by DurationValue"},
		{"accept after Timed({ action inner { accept after 7; } d }) then b;", "an 'after' trigger's delay must be a DurationValue, found Natural"},
		{"accept when flags->ControlFunctions::forAll { in f; action inner { accept when x; } f } then b;", "a 'when' trigger's condition must be Boolean, found Integer"},
	} {
		diags := triggerDiags(t, "transition first a "+tc.trigger)
		if len(diags) != 1 || !strings.Contains(diags[0].Message, tc.want) {
			t.Errorf("%q: want one diagnostic containing %q, got %v", tc.trigger, tc.want, diags)
		}
	}
	wantTriggerSilent(t, "transition first a accept after Timed({ attribute wait : DurationValue = 5 [s]; wait }) then b;")
	wantTriggerSilent(t, "transition first a accept when flags->ControlFunctions::forAll { in f; attribute ok : Boolean = f; ok } then b;")
	wantTriggerSilent(t, "transition first a accept after Timed({ action inner { accept after 7 [s]; } d }) then b;")
	diags := triggerDiags(t, "transition first a accept when { attribute bad : Boolean = 5; flag } then b;")
	if len(diags) != 2 || !strings.Contains(diags[0].Message, "found an expression body") || !strings.Contains(diags[1].Message, bad) {
		t.Errorf("bare body with a bad member: want the body and the binding diagnostics, got %v", diags)
	}
}

// A trigger written inside a `{ … }` body is checked wherever the body is
// written: as a feature value, an assignment value, an invocation argument, a
// guard, a sequence element, an indexed operand or a collection body.
func TestTriggerInBodyValuedExpression(t *testing.T) {
	const after = "an 'after' trigger's delay must be a DurationValue, found Natural"
	for _, tc := range []struct{ body, code, want string }{
		{"attribute v = { action inner { accept after 5; } d };", "trigger-after-duration", after},
		{"attribute v = Timed({ action inner { accept after 5; } d });", "trigger-after-duration", after},
		{"attribute v = Timed(e = { action inner { accept at 5; } d });", "trigger-at-time-instant", "an 'at' trigger's time must be a TimeInstantValue, found Natural"},
		{"entry { assign d := Timed({ action inner { accept when x; } d }); }", "trigger-when-boolean", "a 'when' trigger's condition must be Boolean, found Integer"},
		{"entry { assign untyped := { action inner { accept after 5; } d }; }", "trigger-after-duration", after},
		{"do action run { assign untyped := { action inner { accept after 5; } d }; }", "trigger-after-duration", after},
		{"attribute v = (true, { action inner { accept after 5; } d });", "trigger-after-duration", after},
		{"attribute v = { action inner { accept after 5; } d }#(1);", "trigger-after-duration", after},
		{"attribute v = { in p; action inner { accept after 5; } p };", "trigger-after-duration", after},
		{"attribute v = flags->ControlFunctions::forAll { in f; action inner { accept after 5; } f };", "trigger-after-duration", after},
		// A body-local declaration is what the trigger inside the body names.
		{"attribute v = { attribute inner : Integer = 5; action i2 { accept after inner; } d };", "trigger-after-duration", "an 'after' trigger's delay must be a DurationValue, found Integer"},
		// An unrelated unresolved name elsewhere does not hide the trigger.
		{"part broken : Missing; attribute v = { action inner { accept after 5; } d };", "trigger-after-duration", after},
	} {
		wantTriggerDiag(t, tc.body, tc.code, tc.want)
	}
	diags := triggerDiags(t, "transition first a if { action inner { accept after 5; } true } then b;")
	if len(diags) != 2 || !strings.Contains(diags[0].Message, "transition guard must be Boolean") || !strings.Contains(diags[1].Message, after) {
		t.Errorf("body guard with a bad trigger: want the guard and the trigger diagnostics, got %v", diags)
	}
	wantTriggerSilent(t, "attribute v = { action inner { accept after 5 [s]; } d };")
	wantTriggerSilent(t, "attribute v = { attribute inner : DurationValue = 5 [s]; action i2 { accept after inner; } d };")
	wantTriggerSilent(t, "attribute v = { action inner { accept after missing; } d };")
}

// Arithmetic over an operand that is no quantity, number or String selects no
// function, so it has no result type; the pilot rejects each shape below.
func TestTriggerArithmeticOverNonArithmeticOperand(t *testing.T) {
	wantTriggerDiag(t, "transition first a accept after true + d then b;", "trigger-after-duration", "found `+` over Boolean, which no arithmetic function takes")
	wantTriggerDiag(t, "transition first a accept after holder + d then b;", "trigger-after-duration", "found `+` over Holder")
	wantTriggerDiag(t, "transition first a accept after d * flag then b;", "trigger-after-duration", "found `*` over Boolean")
	wantTriggerDiag(t, "transition first a accept at t + holder then b;", "trigger-at-time-instant", "found `+` over Holder")
	wantTriggerSilent(t, "transition first a accept after untyped + d then b;")
	wantTriggerSilent(t, "transition first a accept after undeclared + d then b;")
}

// `transition ... when <expr>` without `accept` is a change trigger too; a bare
// name there names a signal and is not a condition.
func TestTriggerBareWhenTransition(t *testing.T) {
	wantTriggerSilent(t, "transition first a when x > 3 then b;")
	wantTriggerSilent(t, "transition first a when x then b;")
	wantTriggerDiag(t, "transition first a when x + 1 then b;", "trigger-when-boolean",
		"a 'when' trigger's condition must be Boolean, found Integer")
}

// A trigger written on an accept action — in a state body, as an action node,
// or with a named payload parameter — is checked like a transition's.
func TestTriggerOnAcceptActions(t *testing.T) {
	wantTriggerSilent(t, `state c {
		accept after 5 [s] then b;
		accept at t then b;
		accept when x > 3 then b;
	}
	do action body {
		action w1 accept after 5 [s];
		action w2 accept at t;
		action w3 accept when flag;
		action w4 accept p after Twice(d);
	}`)
	wantTriggerDiag(t, "state c { accept after 5 then b; }", "trigger-after-duration", "found Natural")
	wantTriggerDiag(t, "state c { accept at d then b; }", "trigger-at-time-instant", "found DurationValue")
	wantTriggerDiag(t, "state c { accept when x then b; }", "trigger-when-boolean", "found Integer")
	wantTriggerDiag(t, "do action body { action w accept p after 5 [m]; }", "trigger-after-duration",
		"found a quantity in metre (a LengthUnit)")
}

// The body an action target succession carries (`then starting { ... }`, or
// `first prep then starting { ... }`) is a behavior body like any other: its
// triggers, assignments and declarations see the enclosing scope and the
// declarations the body itself makes.
func TestTriggerInSuccessionBody(t *testing.T) {
	for _, succession := range []string{
		"do action body { action prep; action starting; first prep; then starting { %s } }",
		"do action body { action prep; action starting; first prep then starting { %s } }",
	} {
		checkSuccessionBody(t, succession)
	}
}

func checkSuccessionBody(t *testing.T, succession string) {
	t.Helper()
	body := func(stmt string) string { return strings.Replace(succession, "%s", stmt, 1) }
	wantTriggerSilent(t, body("accept after 5 [s]; assign x := 1;"))
	wantTriggerDiag(t, body("accept after 5;"), "trigger-after-duration", "found Natural")
	wantTriggerDiag(t, body("accept at d;"), "trigger-at-time-instant", "found DurationValue")
	wantTriggerDiag(t, body("accept when x;"), "trigger-when-boolean", "found Integer")
	diags := triggerDiags(t, body(`assign x := "s";`))
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "cannot bind String value to a feature typed by Integer") {
		t.Fatalf("want one assignment diagnostic, got %v", diags)
	}

	const local = "attribute wait : DurationValue; attribute instant : TimeInstantValue; attribute ok : Boolean; attribute n : Integer; "
	wantTriggerSilent(t, body(local+"accept after wait; accept at instant; accept when ok; accept when n > 3; assign n := 1;"))
	wantTriggerDiag(t, body(local+"accept after n;"), "trigger-after-duration", "found Integer")
	wantTriggerDiag(t, body(local+"accept at wait;"), "trigger-at-time-instant", "found DurationValue")
	wantTriggerDiag(t, body(local+"accept when n;"), "trigger-when-boolean", "found Integer")
	diags = triggerDiags(t, body(local+`assign n := "s";`))
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "cannot bind String value to a feature typed by Integer") {
		t.Fatalf("want one assignment diagnostic, got %v", diags)
	}
}

// A declaration a succession body makes is typed like one in any other body.
func TestSuccessionBodyDeclarationsTyped(t *testing.T) {
	for _, succession := range []string{"first prep; then starting { %s }", "first prep then starting { %s }"} {
		src := `package P { private import ScalarValues::*; action def A { action prep; action starting; ` +
			strings.Replace(succession, "%s", `attribute m : Integer = "s";`, 1) + " } }"
		diags := libraryTypeDiags(t, src)
		if len(diags) != 1 || !strings.Contains(diags[0].Message, "cannot bind String value to a feature typed by Integer") {
			t.Errorf("%s: want one declaration value diagnostic, got %v", succession, diags)
		}
	}
}

// A name a succession body declares resolves inside that body and nowhere
// else: the body is a namespace of its own, not part of the enclosing action.
func TestSuccessionBodyDeclarationScope(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		action def A {
			action prep;
			action starting;
			first prep;
			then starting { attribute n : Integer; attribute inner : Integer = n; }
			attribute outer : Integer = n;
		}
	}`
	idx := newTestIndex()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx.AddDocument("<t>", root)
	idx.ExpandWildcardImports()
	diags := Analyze("<t>", root, nil, idx)
	if len(diags) != 1 || diags[0].Code != "unresolved" || diags[0].Span.Offset != strings.LastIndex(src, "n;") {
		t.Fatalf("want one unresolved reference, the outer `= n`, got %v", diags)
	}
}

// A deferred event is a trigger like any other: each of its arguments is judged
// by the rule for its kind, and each is gated on its own lower-tier faults.
func TestTriggerOnDeferredEvents(t *testing.T) {
	const after = "an 'after' trigger's delay must be a DurationValue, found Natural"
	for _, tc := range []struct{ body, code, want string }{
		{"defer after 5;", "trigger-after-duration", after},
		{"defer at 5;", "trigger-at-time-instant", "an 'at' trigger's time must be a TimeInstantValue, found Natural"},
		{"defer when x;", "trigger-when-boolean", "a 'when' trigger's condition must be Boolean, found Integer"},
		{"defer Sig, after 5, at t;", "trigger-after-duration", after},
		{"defer after missing, when x;", "trigger-when-boolean", "a 'when' trigger's condition must be Boolean, found Integer"},
		{"state c { defer after 5; }", "trigger-after-duration", after},
	} {
		wantTriggerDiag(t, "attribute def Sig; "+tc.body, tc.code, tc.want)
	}
	diags := triggerDiags(t, "defer after 5, at 5, when x;")
	if len(diags) != 3 {
		t.Errorf("defer with three bad events: want three diagnostics, got %v", diags)
	}
	wantTriggerSilent(t, "attribute def Sig; defer Sig, after 5 [s], at t, when flag, after d + d;")
	wantTriggerSilent(t, "defer after missing, at missing, when missing;")
}

// An argument the type tier cannot type — an unresolved name, which the name
// tier reports — is not reported again here.
func TestTriggerUnresolvedArgumentIsSilent(t *testing.T) {
	for _, trigger := range []string{"after missing", "at missing", "when missing", "after 5 [missing]"} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
}

// A requirement or constraint named as a Boolean stands for its result, whose
// type reaches back through the library's nameless `return : Boolean`; that
// holds on every load path, the bundled snapshot included.
func TestRequireNamesRequirementResult(t *testing.T) {
	diags := libraryTypeDiags(t, `package P {
		requirement def R;
		requirement r1 : R;
		requirement r2;
		constraint c { true }
		requirement group {
			require r1;
			require r2;
			require c;
		}
	}`)
	if len(diags) != 0 {
		t.Fatalf("want no type diagnostics, got %v", diags)
	}
	diags = libraryTypeDiags(t, `package P {
		attribute n : ScalarValues::Integer;
		requirement group { require n; }
	}`)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "require expression must be Boolean, found Integer") {
		t.Fatalf("want one Integer diagnostic, got %v", diags)
	}
}

// An untyped feature holds nothing statically: a trigger's `when` requires a
// Boolean and reports it, while an `if`, `while` or `require` condition is
// left to evaluation, as before.
func TestUntypedConditionIsLeftToEvaluationOutsideTriggers(t *testing.T) {
	diags := libraryTypeDiags(t, `package P {
		calc def C { in x; if x { return : ScalarValues::Real = 1.0; } return : ScalarValues::Real = 0.0; }
		attribute u;
		action def A { while u { } }
		requirement group { require u; }
	}`)
	if len(diags) != 0 {
		t.Fatalf("want no type diagnostics, got %v", diags)
	}
	wantTriggerDiag(t, "transition first a accept when untyped then b;", "trigger-when-boolean", "found an untyped feature")
}
