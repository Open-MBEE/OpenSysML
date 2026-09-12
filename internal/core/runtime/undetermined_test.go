package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// wantUndetermined fails unless val is an undetermined result whose values
// number count, written as a multiplicity such as `[0..*]`.
func wantUndetermined(t *testing.T, src string, val Value, err error, count string) {
	t.Helper()
	if err != nil {
		t.Errorf("%s: %v; want an undetermined result", src, err)
		return
	}
	u := val.Undetermined()
	if u == nil {
		t.Errorf("%s = %s, want %s", src, FormatValue(val), UndeterminedText)
		return
	}
	if got := u.Count().Text(); got != count {
		t.Errorf("%s counts %s values, want %s", src, got, count)
	}
	if u.Reason() == "" {
		t.Errorf("%s: undetermined for no stated reason", src)
	}
}

// wantFormatted fails unless src evaluates without error to a value spelled want.
func wantFormatted(t *testing.T, src string, val Value, err error, want string) {
	t.Helper()
	if err != nil {
		t.Errorf("%s: %v; want %s", src, err, want)
		return
	}
	if got := FormatValue(val); got != want {
		t.Errorf("%s = %s, want %s", src, got, want)
	}
}

// undeterminedModel declares unbound attributes and a part whose nested parts fix
// or leave open their multiplicities, read at model level below.
const undeterminedModel = `package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	part def D { attribute mass : Real; attribute tag : String = "d"; }
	part rack {
		part slots[3] : D;
		part gear[1..*] : D;
		part loose[0..2] : D;
		part lone : D;
		part many[10001..*] : D;
		part fixed : D :> gear;
	}
	enum def Mode { ON; OFF; }
	attribute u;
	attribute r : Real;
	attribute b : Boolean;
	attribute s : String;
	attribute xs : Real[2..4];
	attribute known : Real = 2.0;
	calc twice { in x : Real; return : Real = x * 2.0; }
	attribute doubled : Real = twice(u);
}`

func undeterminedContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, undeterminedModel))
	return ctx, oneSymbol(t, idx, "test").Scope
}

// A model-level read of a valueless feature is a successful undetermined result of
// its multiplicity naming it — not an error, not the `<unset>` a materialized one reads.
func TestUnboundFeatureReadsUndeterminedAtModelLevel(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, count := range map[string]string{"u": "[1]", "r": "[1]", "b": "[1]", "s": "[1]", "xs": "[2..4]"} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
		if u := val.Undetermined(); u != nil {
			if want := noValueReason(src); u.Reason() != want {
				t.Errorf("%s: reason %q, want %q", src, u.Reason(), want)
			}
		}
		if got := FormatValue(val); got != UndeterminedText {
			t.Errorf("%s formats as %s, want %s", src, got, UndeterminedText)
		}
		if ctx.HoldsNoValue(val) {
			t.Errorf("%s holds no value; an undetermined result is a value, not an unset one", src)
		}
	}
}

// Reading the declaration itself, as `%eval test::u` does, answers what a reference
// to it answers: undetermined, not ErrNoValue; a definition still has no value to read.
func TestDeclaredValueOfUnboundFeatureIsUndetermined(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	u, ok := scope.LookupLocal("u")
	if !ok {
		t.Fatal("test::u not declared")
	}
	val, err := ctx.EvalDeclaredValue(u)
	wantUndetermined(t, "test::u", val, err, "[1]")
	if u := val.Undetermined(); u != nil && u.Reason() != noValueReason("test::u") {
		t.Errorf("reason %q, want %q", u.Reason(), noValueReason("test::u"))
	}
	d, _ := scope.LookupLocal("D")
	if _, err := ctx.EvalDeclaredValue(d); err == nil || errors.Is(err, ErrNoValue) {
		t.Errorf("EvalDeclaredValue(test::D) = %v; want an error other than ErrNoValue", err)
	}
}

// A name no declaration answers, or a member the feature's type does not declare,
// is still an unresolved reference: undetermined is only for declared, open features.
func TestUnresolvedNamesStayErrorsBesideUndetermined(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for _, src := range []string{"nonexistent", "nonexistent + 1", "u.foo", "r.foo", "rack.gear.nonexistent", "rack.gear.mass.foo"} {
		val, err := evalIn(t, ctx, scope, src)
		if !errors.Is(err, ErrUnresolvedReference) {
			t.Errorf("%s = %s, %v; want an unresolved reference", src, FormatValue(val), err)
		}
	}
}

// Every operator over an undetermined operand is undetermined of one value: arithmetic,
// comparison, equality and identity, negation, and a conditional over an unknown test.
func TestOperatorsPropagateUndetermined(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, count := range map[string]string{
		"u + 5": "[1]", "(u + 5) * 2": "[1]", "known + u": "[1]", "-u": "[1]",
		"u > 3": "[1]", "u == 5": "[1]", "u != 5": "[1]", "not (u == 5)": "[1]", "u == u": "[1]", "u === u": "[1]",
		"s + \"a\"": "[1]", "StringFunctions::Length(s)": "[1]",
		"if u > 0 ? 1 else 2": "[0..*]", "if b ? 1 else 2": "[0..*]", "if true ? u else 2": "[1]",
		"u ?? 3": "[1]", "u as Real": "[0..1]", "u hastype Real": "[1]", "u @ Real": "[1]", "r hastype Real": "[1]",
		"1..u": "[0..*]", "u..5": "[0..*]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	// An operand the unknown never reaches still answers.
	for src, want := range map[string]string{
		"if false ? u else 2": "2", "known ?? u": "2.0", "r istype Real": "true", "r istype String": "false", "r @ String": "false",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
}

// The conditional Boolean functions answer whenever either operand fixes the result
// (`x and false`, `x or true`, `x implies true`); every other combination stays undetermined.
func TestBooleanOperatorsFoldOnEitherConstantOperand(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, want := range map[string]string{
		"false and (u > 3)": "false", "true or (u == 1)": "true", "false implies (u == 1)": "true",
		"(u > 3) and false": "false", "(u == 1) or true": "true", "(u == 1) implies true": "true",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for _, src := range []string{
		"(u > 3) and true", "(u == 1) or false", "true implies (u == 1)", "(u == 1) implies false",
		"true and (u > 3)", "false or (u == 1)", "b and b", "b xor true", "not b",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, "[1]")
	}
}

// A sequence with an undetermined element is undetermined, but its count adds up the
// elements' fixed multiplicities: `attribute u;` holds one value, so `size((1, u))` is 2.
func TestSequenceCountsAddUpFixedMultiplicities(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	val, err := evalIn(t, ctx, scope, "(1, u)")
	wantUndetermined(t, "(1, u)", val, err, "[2]")
	if u := val.Undetermined(); u != nil {
		if len(u.Known()) != 1 || FormatValue(u.Known()[0]) != "1" {
			t.Errorf("(1, u) certainly holds %v, want [1]", u.Known())
		}
	}
	for src, want := range map[string]string{
		"size((1, u))": "2", "size(u)": "1", "isEmpty(u)": "false", "notEmpty(u)": "true", "isEmpty(xs)": "false",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for src, count := range map[string]string{"size(xs)": "[1]", "(10, u, 30)#(2)": "[1]", "(10, u, 30)#(1)": "[1]", "(10, 20, 30)#(u)": "[1]", "xs#(1)": "[1]", "xs#(3)": "[1]"} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	// An index the multiplicity's upper bound rules out is a definite error.
	if _, err := evalIn(t, ctx, scope, "xs#(5)"); !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("xs#(5): err = %v; want index out of range", err)
	}
}

// Membership decides from the elements it knows: a sought element among the determined
// ones is included whatever the unknown holds; one unknown or absent is undetermined.
func TestMembershipDecidesFromKnownElements(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, want := range map[string]string{
		"includes((1, u + 1), 1)": "true", "excludes((1, u + 1), 1)": "false", "includes((1, 2), 1)": "true",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for _, src := range []string{"includes((1, 2), u + 1)", "excludes((1, 2), u + 1)", "includes((1, u + 1), 2)"} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, "[1]")
	}
}

// Invocations carry the unknown through their arguments: a calc, a library
// function, a feature valued by such a call, and a body over the unknown.
func TestInvocationsPropagateUndeterminedArguments(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, count := range map[string]string{
		"twice(u)": "[1]", "doubled": "[1]", "twice(rack.gear)": "[1]",
		"RealFunctions::sum((1, u))": "[1]", "RealFunctions::sum(rack.loose.mass)": "[1]", "RealFunctions::max(u, 3)": "[1]",
		"u->ControlFunctions::collect{in x; x + 1}":     "[0..*]",
		"(1, u)->ControlFunctions::select{in x; x > 0}": "[0..*]",
		"(1,2)->ControlFunctions::forAll{in x; x > u}":  "[1]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for src, want := range map[string]string{"twice(known)": "4.0", "known * 2.0": "4.0"} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
}

// A multiplicity the model fixes keeps its definite answer at model level: an
// exact count, a one-valued feature, and an enumeration literal all count.
func TestFixedCardinalitiesStayDefinite(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, want := range map[string]string{
		"size(rack.slots)": "3", "size(rack.lone)": "1", "size(Mode::ON)": "1", "size((rack.slots, rack.lone))": "4",
		"size(rack.slots.mass)": "3", "rack.slots.tag": `["d", "d", "d"]`, "rack.lone.tag": `"d"`, "rack.lone.tag + \"!\"": `"d!"`, "Mode::ON": "Mode::ON",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	if val, err := evalIn(t, ctx, scope, "rack.slots#(2)"); err != nil || val.Kind != ValInstance {
		t.Errorf("rack.slots#(2) = %s, %v; want the second slot", FormatValue(val), err)
	}
	if _, err := evalIn(t, ctx, scope, "rack.slots#(4)"); !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("rack.slots#(4): err = %v; want index out of range", err)
	}
}

// An open multiplicity has no definite count at model level (size, isEmpty, notEmpty,
// chains and indexing are undetermined), but what its bounds fix answers: `[1..*]` is not empty.
func TestOpenCardinalitiesAreUndetermined(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, count := range map[string]string{
		"size(rack.gear)": "[1]", "size(rack.loose)": "[1]", "isEmpty(rack.loose)": "[1]", "notEmpty(rack.loose)": "[1]",
		"size((rack.gear, rack.loose))": "[1]",
		"rack.gear#(7)":                 "[1]", "rack.gear#(1)": "[1]", "rack.loose#(1)": "[1]",
		"rack.gear.mass": "[1..*]", "size(rack.gear.tag)": "[1]",
		"rack.lone.mass": "[1]", "rack.lone.mass + 1.0": "[1]", "rack.slots.mass": "[3]", "RealFunctions::sum(rack.slots.mass)": "[1]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for src, want := range map[string]string{"notEmpty(rack.gear)": "true", "isEmpty(rack.gear)": "false"} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for _, src := range []string{"size(rack.gear)", "rack.gear#(7)"} {
		val, _ := evalIn(t, ctx, scope, src)
		if u := val.Undetermined(); u != nil && !strings.Contains(u.Reason(), "[1..*]") {
			t.Errorf("%s: reason %q does not state the open multiplicity", src, u.Reason())
		}
	}
}

// A model-level read of an open collection does not make up its lower bound: one too
// large to materialize is undetermined of its declared count rather than a violation.
func TestOpenCollectionIsNotMaterializedAtModelLevel(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, count := range map[string]string{"rack.many": "[10001..*]", "size(rack.many)": "[1]", "rack.many.mass": "[10001..*]"} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for src, want := range map[string]string{"notEmpty(rack.many)": "true", "isEmpty(rack.many)": "false"} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
}

// An open collection certainly holds what the features subsetting it contribute:
// membership and the count they fix answer, while the rest stays open.
func TestOpenCollectionKnowsItsSubsetters(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, want := range map[string]string{
		"includes(rack.gear, rack.fixed)": "true", "excludes(rack.gear, rack.fixed)": "false",
		"rack.gear->ControlFunctions::exists{in x; x == rack.fixed}": "true",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	val, err := evalIn(t, ctx, scope, "rack.gear")
	wantUndetermined(t, "rack.gear", val, err, "[1..*]")
	if u := val.Undetermined(); u != nil && (len(u.Known()) != 1 || u.Known()[0].Kind != ValInstance) {
		t.Errorf("rack.gear certainly holds %v, want the one object of fixed", u.Known())
	}
	for src, count := range map[string]string{"size(rack.gear)": "[1]", "includes(rack.gear, rack.lone)": "[1]", "rack.gear#(2)": "[1]"} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
}

// Quantifiers over an open collection decide from the elements it certainly holds: a
// witness proves exists, a counterexample refutes forAll, and Boolean truth likewise.
func TestQuantifiersDecideFromKnownElements(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, want := range map[string]string{
		"(1, u)->ControlFunctions::exists{in x; x == 1}":            "true",
		"(1, u)->ControlFunctions::forAll{in x; x > 2}":             "false",
		"ControlFunctions::anyTrue((true, b))":                      "true",
		"ControlFunctions::allTrue((false, b))":                     "false",
		"rack.gear->ControlFunctions::forAll{in x; x.tag == \"e\"}": "false",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for src, count := range map[string]string{
		"(1, u)->ControlFunctions::exists{in x; x == 2}":             "[1]",
		"(1, u)->ControlFunctions::forAll{in x; x > 0}":              "[1]",
		"ControlFunctions::anyTrue((false, b))":                      "[1]",
		"ControlFunctions::allTrue((true, b))":                       "[1]",
		"ControlFunctions::allTrue(b)":                               "[1]",
		"rack.loose->ControlFunctions::forAll{in x; x.tag == \"d\"}": "[1]",
		"rack.gear->ControlFunctions::forAll{in x; x.tag == \"d\"}":  "[1]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
}

// An instantiated object materializes its minimum multiplicity into real objects,
// so through it the same features answer definitely; that contract does not change.
func TestInstantiatedObjectKeepsMaterializedMinimums(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, undeterminedModel))
	scope := oneSymbol(t, idx, "test").Scope
	rack := oneSymbol(t, idx, "test::rack")
	inst, err := ctx.Instantiate(rack)
	if err != nil {
		t.Fatalf("instantiate rack: %v", err)
	}
	for src, want := range map[string]string{
		"size(slots)": "3", "size(gear)": "1", "notEmpty(gear)": "true", "isEmpty(loose)": "true", "size(loose)": "0",
		"size((gear, loose))": "1", "size(lone)": "1",
	} {
		val, err := ctx.EvalWithScopeOn(parseExpr(t, src), scope, inst)
		wantFormatted(t, src, val, err, want)
	}
	if _, err := ctx.EvalWithScopeOn(parseExpr(t, "gear#(7)"), scope, inst); !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("gear#(7) on an object: err = %v; want index out of range", err)
	}
}

// A required value an instantiated object does not hold is still unset there and
// ErrNoValue when used: only model-level evaluation is undetermined.
func TestInstanceLevelMissingValueStaysErrNoValue(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		part def Car { attribute mass : Real; attribute wheels : Integer = 4; }
		part car : Car;
	}`))
	scope := oneSymbol(t, idx, "test").Scope
	for src, count := range map[string]string{"car.mass": "[1]", "car.mass + 1.0": "[1]"} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	val, err := evalIn(t, ctx, scope, "(car.mass > 1.0) and false")
	wantFormatted(t, "(car.mass > 1.0) and false", val, err, "false")

	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::car"))
	if err != nil {
		t.Fatalf("instantiate car: %v", err)
	}
	val, err = ctx.EvalWithScopeOn(parseExpr(t, "mass"), scope, inst)
	if err != nil || !ctx.HoldsNoValue(val) {
		t.Errorf("mass on car = %s, %v; want the unset value the object holds", FormatValue(val), err)
	}
	for _, src := range []string{"mass + 1.0", "(mass > 1.0) and false", "car.mass + 1.0"} {
		val, err := ctx.EvalWithScopeOn(parseExpr(t, src), scope, inst)
		if !errors.Is(err, ErrNoValue) {
			t.Errorf("%s on car = %s, %v; want ErrNoValue", src, FormatValue(val), err)
		}
	}
	if val, err := ctx.EvalWithScopeOn(parseExpr(t, "wheels"), scope, inst); err != nil || FormatValue(val) != "4" {
		t.Errorf("wheels on car = %s, %v; want 4", FormatValue(val), err)
	}
	if _, err := evalIn(t, ctx, scope, "car.mass + 1.0"); !errors.Is(err, ErrNoValue) {
		t.Errorf("car.mass + 1.0 through the instantiated car: err = %v; want ErrNoValue", err)
	}
}

// An undetermined value is a value kind every dispatch handles: it formats,
// traces, compares unequal to unset and to null, and is never a NoValueError.
func TestUndeterminedIsAValueKind(t *testing.T) {
	val := NewUndeterminedValue("x has no value in the model", openRange())
	if val.Kind != ValUndetermined || val.Undetermined() == nil {
		t.Fatalf("NewUndeterminedValue = %v, want a ValUndetermined payload", val)
	}
	if got := FormatValue(val); got != UndeterminedText {
		t.Errorf("FormatValue = %s, want %s", got, UndeterminedText)
	}
	if got := val.Kind.String(); got != "undetermined" {
		t.Errorf("Kind.String() = %q, want undetermined", got)
	}
	ctx := NewContext(NewModel(nil, nil), 100)
	if ctx.HoldsNoValue(val) {
		t.Error("HoldsNoValue(undetermined) = true; an undetermined result is a value, not a missing one")
	}
	if (Value{Kind: ValUndetermined}).Undetermined() != nil {
		t.Error("a ValUndetermined without a payload reports one")
	}
	if (Value{Kind: ValConst}).Undetermined() != nil {
		t.Error("a determined value reports an undetermined payload")
	}
}
