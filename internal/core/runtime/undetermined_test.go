package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
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
	part def D { attribute mass : Real; attribute tag : String default = "d"; attribute w : Real = 2.0; }
	part rack {
		part slots[3] : D;
		part gear[1..*] : D;
		part loose[0..2] : D;
		part lone : D;
		part many[10001..*] : D;
		part fixed : D :> gear;
	}
	part shelf {
		part items[1..*] : D;
		part plain : D :> items;
		part tagged : D :> items { attribute :>> tag = "x"; }
	}
	part store {
		part base[0..*] : D;
		part sub[10001..*] : D :> base;
		part cap[0..3] : D;
		part three[3..*] : D :> cap;
		part pool[0..*] : D;
		part inner[2] : D :> pool;
		part outer[5..*] : D :> pool;
	}
	attribute def Flag;
	attribute flag : Flag;
	enum def Mode { ON; OFF; }
	attribute u;
	attribute r : Real;
	attribute b : Boolean;
	attribute s : String;
	attribute xs : Real[2..4];
	attribute ss : String[2..*];
	attribute bs : Boolean[2];
	attribute os : Real[0..4];
	attribute vast : Real[0..9223372036854775807];
	attribute big : Real[5000000];
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
		"if u > 0 ? 1 else 2": "[1]", "if b ? 1 else 2": "[1]", "if true ? u else 2": "[1]",
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
	for src, count := range map[string]string{"size(xs)": "[1]", "(10, u, 30)#(2)": "[1]", "(10, 20, 30)#(u)": "[1]", "xs#(1)": "[1]", "xs#(3)": "[1]"} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	// An index the multiplicity's upper bound rules out is a definite error.
	if _, err := evalIn(t, ctx, scope, "xs#(5)"); !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("xs#(5): err = %v; want index out of range", err)
	}
}

// A sequence fixes each position up to the first element of open count, so the
// determined ones answer positional reads even though the sequence as a whole is
// undetermined; a position the model leaves open stays undetermined, one past the
// count is a definite error, and positions after an element of open count are open.
func TestFixedPositionsOfOpenSequencesAnswer(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, want := range map[string]string{
		"(10, u, 30)#(1)": "10", "(10, u, 30)#(3)": "30", "((10, u), 30)#(3)": "30", "(10, (u, 30))#(3)": "30",
		"head((10, u, 30))": "10", "last((10, u, 30))": "30", "head(tail((10, u, 30)))": "<undetermined>",
		"last(tail((10, u, 30)))": "30", "size(tail((10, u, 30)))": "2",
		"subsequence((10, u, 30), 2, 3)#(2)": "30", "subsequence((10, u, 30), 3)#(1)": "30",
		"excludingAt((10, u, 30), 2)#(2)": "30", "excludingAt((10, u, 30), 1, 2)#(1)": "30",
		"includingAt((10, u, 30), 20, 2)#(2)": "20", "includingAt((10, u, 30), 20, 2)#(4)": "30",
		"includingAt((10, 30), u, 2)#(3)": "30", "includingAt((10, 30), u, 2)#(2)": "<undetermined>",
		"(10, u, 30)#(1) + (10, u, 30)#(3)": "40", "(10, u, 30)#(1) == 10": "true",
		"(u, r)#(2) == r":  "<undetermined>",
		"(10, xs, 30)#(1)": "10", "head((10, xs, 30))": "10", "(10, 20, xs)#(2)": "20", "subsequence((10, 20, xs), 1, 2)#(2)": "20",
		"(rack.gear, 30)#(1) == 30": "<undetermined>",
		"(1, big, 2)#(5000002)":     "2", "size((1, big))": "5000001", "last(tail((1, big, 2)))": "2",
		"size(subsequence((1, big, 2), 3, 5000001))": "4999999", "size(excludingAt((1, big, 2), 2, 4000000))": "1000003",
		"excludingAt((1, big, 2), 2, 5000001)#(2)": "2", "includingAt((1, big, 2), 3, 5000002)#(5000003)": "2",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for src, count := range map[string]string{
		"(10, u, 30)#(2)": "[1]", "(u, r)#(2)": "[1]", "tail((10, u, 30))": "[2]", "tail(xs)": "[1..3]", "head(xs)": "[1]",
		"(10, xs, 30)#(2)": "[1]", "(10, xs, 30)#(4)": "[1]", "last((10, xs, 30))": "[1]", "last(rack.loose)": "[0..1]",
		"(xs, 10)#(1)": "[1]", "head((xs, 10))": "[1]", "(rack.gear, 30)#(1)": "[1]",
		"(1, big, 2)#(2)": "[1]", "(1, big, 2)#(5000001)": "[1]", "subsequence((1, big, 2), 3, 5000001)": "[4999999]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for _, src := range []string{"(10, u, 30)#(4)", "(10, u, 30)#(0)", "(10, xs, 30)#(7)"} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, ErrIndexOutOfRange) {
			t.Errorf("%s: err = %v; want index out of range", src, err)
		}
	}
}

// Membership decides from the elements it knows: a sought element among the determined
// ones is included whatever the unknown holds, one a determined sequence lacks is not,
// and one both certainly hold is not excluded; one unknown or absent is undetermined.
func TestMembershipDecidesFromKnownElements(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, want := range map[string]string{
		"includes((1, u + 1), 1)": "true", "excludes((1, u + 1), 1)": "false", "includes((1, 2), 1)": "true",
		"includes((1), (2, u))": "false", "includes((1, 2), (3, u + 1))": "false",
		"excludes((1), (1, u))": "false", "excludes((1, u), (1, r))": "false", "excludes(rack.gear, (rack.fixed, u))": "false",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for _, src := range []string{
		"includes((1, 2), u + 1)", "excludes((1, 2), u + 1)", "includes((1, u + 1), 2)",
		"includes((1, 2), (1, u))", "includes((1, u), (2, r))", "excludes((1, 2), (3, u))", "excludes(rack.gear, (rack.lone, u))",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, "[1]")
	}
}

// Indexing with an undetermined index is undetermined, unless the sequence is
// certainly empty: no index reaches into it, so that is out of range as `()#(1)` is.
func TestUndeterminedIndexIntoEmptySequenceIsOutOfRange(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for _, src := range []string{"()#(u)", "SequenceFunctions::'#'((), u)", "()#(1)"} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, ErrIndexOutOfRange) {
			t.Errorf("%s: err = %v; want index out of range", src, err)
		}
	}
	for _, src := range []string{"(1, 2)#(u)", "rack.loose#(u)", "rack.gear#(u)", "u#(u)"} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, "[1]")
	}
}

// Invocations carry the unknown through their arguments: a calc, a library
// function, a feature valued by such a call, and a body over the unknown.
func TestInvocationsPropagateUndeterminedArguments(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, count := range map[string]string{
		"twice(u)": "[1]", "doubled": "[1]", "twice(rack.lone.mass)": "[1]",
		"RealFunctions::sum((1, u))": "[1]", "RealFunctions::sum(rack.loose.mass)": "[1]", "RealFunctions::max(u, 3)": "[1]",
		"u->ControlFunctions::collect{in x; x + 1}":     "[1]",
		"(1, u)->ControlFunctions::select{in x; x > 0}": "[1..2]",
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

// select, reject, selectOne and collect over an open collection apply their body to
// the elements it certainly holds and keep what results, leaving the unknown rest open.
func TestCollectionTransformsKeepKnownElements(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, want := range map[string]string{
		"includes((1, u)->ControlFunctions::select{in x; x == 1}, 1)":                                                "true",
		"includes((1, u)->ControlFunctions::collect{in x; x + 1}, 2)":                                                "true",
		"notEmpty((1, u)->ControlFunctions::select{in x; x == 1})":                                                   "true",
		"notEmpty(rack.gear->ControlFunctions::select{in x; x == rack.fixed})":                                       "true",
		"rack.gear->ControlFunctions::select{in x; x.tag == \"d\"}->ControlFunctions::exists{in x; x == rack.fixed}": "true",
		"(1, 2)->ControlFunctions::select{in x; x > 0}":                                                              "[1, 2]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for src, count := range map[string]string{
		"(1, u)->ControlFunctions::select{in x; x == 1}":                      "[1..2]",
		"(1, u)->ControlFunctions::reject{in x; x == 1}":                      "[0..1]",
		"(1, u)->ControlFunctions::collect{in x; x + 1}":                      "[2]",
		"(1, u)->ControlFunctions::selectOne{in x; x == 1}":                   "[1]",
		"(1, u)->ControlFunctions::selectOne{in x; x == 2}":                   "[0..1]",
		"(1, 2)->ControlFunctions::select{in x; x > u}":                       "[0..2]",
		"rack.gear->ControlFunctions::select{in x; x == rack.fixed}":          "[1..*]",
		"rack.gear->ControlFunctions::reject{in x; x == rack.fixed}":          "[0..*]",
		"rack.loose->ControlFunctions::select{in x; x.tag == \"d\"}":          "[0..2]",
		"rack.loose->ControlFunctions::collect{in x; x.mass}":                 "[0..2]",
		"rack.gear->ControlFunctions::select{in x; x.tag == \"e\"}":           "[0..*]",
		"isEmpty(rack.loose->ControlFunctions::select{in x; x.tag == \"e\"})": "[1]",
		"size((1, u)->ControlFunctions::select{in x; x == 1})":                "[1]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for src, known := range map[string]string{
		"(1, u)->ControlFunctions::select{in x; x == 1}":  "[1]",
		"(1, u)->ControlFunctions::reject{in x; x == 1}":  "[]",
		"(1, u)->ControlFunctions::collect{in x; x + 1}":  "[2]",
		"(1, u)->ControlFunctions::collect{in x; (x, x)}": "[1, 1]",
	} {
		val, _ := evalIn(t, ctx, scope, src)
		if u := val.Undetermined(); u != nil {
			if got := FormatValue(sequenceOf(u.Known())); got != known {
				t.Errorf("%s certainly holds %s, want %s", src, got, known)
			}
		}
	}
}

// A filter over an open collection tests one element standing for each the
// collection may hold beyond those known: a test that decides for it keeps or drops
// them all, so the count the model fixes survives a constant test and a test that
// fails whatever the element still fails; only an open test leaves them open.
func TestFiltersDecideOverUnknownElements(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, want := range map[string]string{
		"rack.gear->ControlFunctions::reject{in x; true}":                    "[]",
		"xs->ControlFunctions::select{in x; false}":                          "[]",
		"(1, u)->ControlFunctions::reject{in x; true}":                       "[]",
		"isEmpty(rack.gear->ControlFunctions::reject{in x; true})":           "true",
		"notEmpty(rack.gear->ControlFunctions::select{in x; true})":          "true",
		"size(xs->ControlFunctions::reject{in x; false}) == size(xs)":        "<undetermined>",
		"size((1, u)->ControlFunctions::select{in x; true})":                 "2",
		"size((1, u)->ControlFunctions::reject{in x; x == 1})":               "<undetermined>",
		"rack.loose->ControlFunctions::select{in x; x.tag == \"d\"} == ()":   "<undetermined>",
		"ControlFunctions::selectOne(rack.gear, {in x; true}) == rack.fixed": "<undetermined>",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for src, count := range map[string]string{
		"rack.gear->ControlFunctions::select{in x; true}":    "[1..*]",
		"xs->ControlFunctions::select{in x; true}":           "[2..4]",
		"xs->ControlFunctions::reject{in x; false}":          "[2..4]",
		"rack.loose->ControlFunctions::reject{in x; false}":  "[0..2]",
		"xs->ControlFunctions::select{in x; x > 1.0}":        "[0..4]",
		"(1, u)->ControlFunctions::select{in x; x == 1}":     "[1..2]",
		"rack.gear->ControlFunctions::selectOne{in x; true}": "[1]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for _, src := range []string{
		"xs->ControlFunctions::select{in x; 1 / 0 > 0}",
		"rack.gear->ControlFunctions::reject{in x; 1 % 0 == 0}",
	} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, ErrDivisionByZero) {
			t.Errorf("%s: %v, want a division by zero", src, err)
		}
	}
}

// A mapping over an open collection counts what the mapper yields per element the
// collection may hold beyond those known, so a mapper of fixed count keeps the count
// exact and only an open mapper or an open collection leaves it open.
func TestCollectOverOpenCollectionKeepsFiniteCounts(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, count := range map[string]string{
		"(1, u)->ControlFunctions::collect{in x; x + 1}":                    "[2]",
		"(1, u)->ControlFunctions::collect{in x; (x, x)}":                   "[4]",
		"xs->ControlFunctions::collect{in x; x * 2.0}":                      "[2..4]",
		"rack.loose->ControlFunctions::collect{in x; x.mass}":               "[0..2]",
		"rack.gear->ControlFunctions::collect{in x; x.mass}":                "[1..*]",
		"(1, u)->ControlFunctions::collect{in x; if x > 0 ? (1, 1) else 1}": "[3..4]",
		"(1, u)->ControlFunctions::collect{in x; xs}":                       "[4..8]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for src, want := range map[string]string{
		"size((1, u)->ControlFunctions::collect{in x; x + 1})":      "2",
		"size((1, u)->ControlFunctions::collect{in x; (x, x)})":     "4",
		"size(u->ControlFunctions::collect{in x; x + 1})":           "1",
		"size(rack.slots->ControlFunctions::collect{in x; x.mass})": "3",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
}

// A feature declared to hold no value, `[0]`, reads as the empty sequence at model
// level: the model fixes its count, so nothing about it is undetermined.
func TestExactlyZeroFeatureReadsEmptyAtModelLevel(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		private import ISQ::*;
		part def D;
		attribute none : Integer[0];
		attribute noMass :> ISQ::mass [0];
		part rack { part vacant[0] : D; part lone : D; }
	}`))
	scope := oneSymbol(t, idx, "test").Scope
	for src, want := range map[string]string{
		"none": "[]", "noMass": "[]", "rack.vacant": "[]",
		"size(none)": "0", "isEmpty(none)": "true", "isEmpty(rack.vacant)": "true", "size(rack.lone)": "1",
		"NumericalFunctions::sum(noMass)": "0 [kg]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
}

// A conditional over an open test holds as many values as either branch declares:
// the fixed count both share, else the range covering both; a branch the
// declarations leave open leaves the count open.
func TestOpenConditionalKeepsBranchCounts(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, count := range map[string]string{
		"if b ? 1 else 2":                              "[1]",
		"if b ? known else r":                          "[1]",
		"if b ? (1, 2) else xs":                        "[2..4]",
		"if b ? 1 else ()":                             "[0..1]",
		"if b ? rack.slots else rack.lone":             "[1..3]",
		"if b ? rack.gear else 1":                      "[1..*]",
		"if b ? 1 + 1 else 2":                          "[0..*]",
		"ControlFunctions::'if'(b, 1, 2)":              "[1]",
		"ControlFunctions::'if'(b, {1}, {(1, 2)})":     "[1..2]",
		"ControlFunctions::'if'(b, xs, ())":            "[0..4]",
		"ControlFunctions::'if'(b, {u + 1}, {(1, 2)})": "[0..*]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for src, want := range map[string]string{
		"size(if b ? 1 else 2)": "1", "notEmpty(if b ? rack.slots else rack.lone)": "true",
		"size(ControlFunctions::'if'(b, 1, 2))": "1",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
}

// `??` over an open first operand that may be empty yields either that operand,
// then holding at least one value, or the second: the count covers both, with
// the second's count read from the declarations, not by evaluating it.
func TestNullCoalescingKeepsOperandCounts(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, count := range map[string]string{
		"u ?? 3":                                       "[1]",
		"rack.loose ?? 3":                              "[1..2]",
		"rack.loose ?? (1, 2)":                         "[1..2]",
		"rack.loose ?? xs":                             "[1..4]",
		"rack.loose ?? ()":                             "[0..2]",
		"rack.loose ?? rack.gear":                      "[1..*]",
		"rack.loose ?? 1 + 1":                          "[0..*]",
		"size(rack.loose ?? 3)":                        "[1]",
		"ControlFunctions::'??'(rack.loose, 3)":        "[1..2]",
		"ControlFunctions::'??'(rack.loose, {(1, 2)})": "[1..2]",
		"ControlFunctions::'??'(rack.loose, {xs})":     "[1..4]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for src, want := range map[string]string{
		"size(u ?? 3)": "1", "notEmpty(rack.loose ?? 3)": "true", "notEmpty(rack.gear ?? ())": "true",
		"notEmpty(ControlFunctions::'??'(rack.loose, 3))": "true", "known ?? 3": "2.0",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
}

// A test whose answer does not depend on the element decides a quantifier over an
// open collection: over one certainly holding an element, a constant witness proves
// exists and a constant counterexample refutes forAll; a test that never holds
// refutes exists and one that always holds proves forAll, whatever the count. A
// test the element leaves open, or a witness in a collection that may hold none,
// stays open.
func TestQuantifiersDecideFromConstantTests(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, want := range map[string]string{
		"rack.gear->ControlFunctions::exists{in x; true}":    "true",
		"rack.gear->ControlFunctions::forAll{in x; false}":   "false",
		"rack.many->ControlFunctions::exists{in x; true}":    "true",
		"rack.many->ControlFunctions::forAll{in x; false}":   "false",
		"rack.loose->ControlFunctions::exists{in x; false}":  "false",
		"rack.loose->ControlFunctions::forAll{in x; true}":   "true",
		"rack.many->ControlFunctions::forAll{in x; true}":    "true",
		"xs->ControlFunctions::exists{in x; x == x or true}": "true",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for src, count := range map[string]string{
		"rack.loose->ControlFunctions::exists{in x; true}":        "[1]",
		"rack.loose->ControlFunctions::forAll{in x; false}":       "[1]",
		"rack.gear->ControlFunctions::forAll{in x; x.mass > 1.0}": "[1]",
		"rack.gear->ControlFunctions::exists{in x; x.mass > 1.0}": "[1]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	// The test is applied to the element the collection may hold, so its errors surface.
	if _, err := evalIn(t, ctx, scope, "rack.gear->ControlFunctions::exists{in x; 1 / 0 > 0}"); !errors.Is(err, ErrDivisionByZero) {
		t.Errorf("rack.gear->exists{1 / 0 > 0}: err = %v, want ErrDivisionByZero", err)
	}
}

// A determined operand that alone makes arithmetic fail — a zero divisor, the
// unbounded `*` — fails it whatever the open operand holds, in the operator
// notation and in the library's function forms alike; other arithmetic stays open.
func TestDefiniteArithmeticErrorsSurviveOpenOperands(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for _, src := range []string{
		"u / 0", "u % 0", "r / 0.0", "(u + 1) / (2 - 2)",
		"RealFunctions::'/'(r, 0.0)", "IntegerFunctions::'%'(u, 0)", "IntegerFunctions::'/'(u, 0)",
		"NaturalFunctions::'/'(u, 0)", "ScalarFunctions::'/'(u, 0)",
	} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, ErrDivisionByZero) {
			t.Errorf("%s: err = %v, want ErrDivisionByZero", src, err)
		}
	}
	for _, src := range []string{"u + *", "RealFunctions::'*'(r, *)"} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: err = %v, want ErrTypeMismatch", src, err)
		}
	}
	if _, err := evalIn(t, ctx, scope, "IntegerFunctions::'/'(u, 1.5)"); err == nil {
		t.Error("IntegerFunctions::'/'(u, 1.5): no error; want the Integer domain refused")
	}
	if _, err := evalIn(t, ctx, scope, "RealFunctions::'+'(xs, 1.0)"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("RealFunctions::'+'(xs, 1.0): err = %v, want ErrMultiplicityViolation", err)
	}
	for src, count := range map[string]string{
		"r / 2.0": "[1]", "u / u": "[1]", "u % 3": "[1]", "RealFunctions::'/'(r, 2.0)": "[1]",
		"RealFunctions::'-'(r)": "[1]", "NaturalFunctions::'/'(u, 2)": "[1]", "IntegerFunctions::'/'(6, u)": "[1]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
}

// A determined index or position that names no place in any value the open operand may
// hold fails the library function, as does one past the most the operand's count admits;
// determined positions that select nothing answer empty, and the rest stays open.
func TestDefiniteLibraryErrorsSurviveOpenOperands(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for _, src := range []string{
		"StringFunctions::Substring(s, 0, 2)", "StringFunctions::Substring(s, -1, u)",
		"StringFunctions::Substring(\"abc\", 2, 4)",
		"includingAt(xs, 1.0, 0)", "includingAt(xs, 1.0, 6)", "includingAt((1.0, 2.0), r, 4)",
		"subsequence(xs, 0, 1)", "subsequence(xs, 0)", "subsequence(xs, 2, 5)",
		"excludingAt(xs, 0)", "excludingAt(xs, 5)", "excludingAt(xs, 2, 5)", "excludingAt(xs, u, 5)",
		"excludingAt(xs, 3, 2)", "excludingAt((1.0, 2.0), u, 3)",
	} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, ErrIndexOutOfRange) {
			t.Errorf("%s: err = %v, want ErrIndexOutOfRange", src, err)
		}
	}
	for _, src := range []string{
		"StringFunctions::Substring(s, \"a\", 2)", "StringFunctions::Substring(1, u, 2)",
		"includingAt(xs, 1.0, 1.5)", "subsequence(xs, true)", "excludingAt(xs, 1, \"b\")",
	} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: err = %v, want ErrTypeMismatch", src, err)
		}
	}
	for src, want := range map[string]string{
		"StringFunctions::Substring(s, 3, 2)": `""`, "subsequence(xs, 3, 2)": "[]", "subsequence(xs, 5, 1)": "[]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for src, count := range map[string]string{
		"StringFunctions::Substring(s, 1, 2)": "[1]", "StringFunctions::Substring(\"abc\", u, 2)": "[1]",
		"StringFunctions::Substring(\"abc\", 1, u)": "[1]", "includingAt(xs, 1.0, 1)": "[3..5]",
		"includingAt(vast, 1.0, 9223372036854775807)": "[1..*]", "includingAt(vast, 1.0, 1)": "[1..*]",
		"includingAt(xs, 1.0, 5)": "[3..5]", "includingAt((1.0, 2.0), r, 3)": "[3]", "includingAt(xs, r, u)": "[3..5]",
		"subsequence(xs, 1, 2)": "[0..*]", "subsequence(xs, 2)": "[0..*]", "subsequence(xs, 4)": "[0..*]",
		"subsequence((1.0, 2.0), u, 2)": "[0..*]", "subsequence((1.0, 2.0), 1, u)": "[0..*]",
		"excludingAt(xs, 1)": "[0..*]", "excludingAt(xs, 2, 4)": "[0..*]",
		"excludingAt((1.0, 2.0), u)": "[0..*]", "excludingAt(xs, u, 4)": "[0..*]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
}

// A scalar numeric library function checks every argument the model determines
// against its parameter's domain before an open one leaves the result open.
func TestScalarFunctionsCheckDeterminedArguments(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, want := range map[string]error{
		"OpenSysMLMathFunctions::log(u, -1.0)":                            semantics.ErrArithmeticDomain,
		"OpenSysMLMathFunctions::log(u, 1.0)":                             semantics.ErrArithmeticDomain,
		"OpenSysMLMathFunctions::log(-1.0, u)":                            semantics.ErrArithmeticDomain,
		"OpenSysMLMathFunctions::log(0.0, u)":                             semantics.ErrArithmeticDomain,
		"OpenSysMLMathFunctions::ln(u) + OpenSysMLMathFunctions::ln(0.0)": semantics.ErrArithmeticDomain,
		"RationalFunctions::gcd(1.5, u)":                                  semantics.ErrArithmeticDomain,
		"IntegerFunctions::max(1.5, u)":                                   ErrTypeMismatch,
		"IntegerFunctions::min(u, 2.5)":                                   ErrTypeMismatch,
		"NaturalFunctions::min(-1, u)":                                    ErrTypeMismatch,
		"NaturalFunctions::max(u, -1)":                                    ErrTypeMismatch,
		"RationalFunctions::rat(1.5, u)":                                  ErrTypeMismatch,
		"RealFunctions::max(\"a\", u)":                                    ErrTypeMismatch,
		"RealFunctions::max(u, true)":                                     ErrTypeMismatch,
		"RealFunctions::max(xs, 1.0)":                                     ErrMultiplicityViolation,
		"RealFunctions::max(u, (1.0, 2.0))":                               ErrTypeMismatch,
		"OpenSysMLMathFunctions::log(-1.0, 0.0)":                          semantics.ErrArithmeticDomain,
		"NaturalFunctions::max(-1, 1.5)":                                  ErrTypeMismatch,
		"RationalFunctions::rat(u, 0)":                                    ErrDivisionByZero,
	} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", src, err, want)
		}
	}
	for _, src := range []string{
		"OpenSysMLMathFunctions::log(u, 10.0)", "OpenSysMLMathFunctions::log(100.0, u)",
		"OpenSysMLMathFunctions::log(u, r)", "OpenSysMLMathFunctions::ln(u)",
		"OpenSysMLMathFunctions::atan2(0.0, u)", "IntegerFunctions::max(1, u)",
		"IntegerFunctions::abs(u)", "NaturalFunctions::min(u, 0)", "RationalFunctions::gcd(6, u)",
		"RationalFunctions::rat(u, 2)", "RealFunctions::max(r, 1.0)", "RealFunctions::max(known, u)",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, "[1]")
	}
	for src, want := range map[string]string{
		"OpenSysMLMathFunctions::log(100.0, 10.0)": "2.0", "IntegerFunctions::max(1, 2)": "2",
		"RationalFunctions::gcd(6, 4)": "2", "NaturalFunctions::min(3, 4)": "3",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
}

// A multiplicity bound the model does not evaluate fixes no count either: a model-level
// read of such a feature is undetermined of those bounds, never a materialization error,
// and the counts derived from it keep the bounds it does fix.
func TestUnknownMultiplicityBoundsReadUndeterminedAtModelLevel(t *testing.T) {
	const model = `package test {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		part def D { attribute mass : Real; }
		attribute n : Natural;
		part rack {
			part vary[n] : D;
			part some[1..n] : D;
			part one[n..1] : D;
			part optn[0..n] : D;
			part held : D :> vary;
		}
		attribute a : Real[n];
		attribute an : Real[1..n];
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model))
	scope := oneSymbol(t, idx, "test").Scope
	for src, count := range map[string]string{
		"rack.vary": "[1..?]", "size(rack.vary)": "[1]", "rack.vary.mass": "[1..?]", "rack.optn": "[0..?]",
		"rack.some": "[1..?]", "size(rack.some)": "[1]", "rack.one": "[?..1]", "isEmpty(rack.one)": "[1]",
		"a": "[?..?]", "size(a)": "[1]", "a + 1.0": "[0..1]", "(a, 1.0)": "[1..?]", "a ?? 1.0": "[1..?]",
		"size((a, 1.0))": "[1]", "size((an, an))": "[1]", "isEmpty(a)": "[1]", "an->ControlFunctions::collect{in x; x * 2.0}": "[1..?]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for src, want := range map[string]string{
		"notEmpty(rack.vary)": "true", "isEmpty(rack.vary)": "false", "notEmpty(rack.some)": "true",
		"notEmpty(an)": "true", "notEmpty((a, 1.0))": "true", "includes(rack.vary, rack.held)": "true",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	// An object stands behind no count it cannot evaluate: reading one through it still fails.
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::rack"))
	if err != nil {
		t.Fatalf("instantiate rack: %v", err)
	}
	for _, src := range []string{"vary", "size(some)", "one"} {
		if _, err := ctx.EvalWithScopeOn(parseExpr(t, src), scope, inst); err == nil || !strings.Contains(err.Error(), "unknown multiplicity") {
			t.Errorf("%s on an object: err = %v, want an unknown-multiplicity error", src, err)
		}
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

// A body's local omitting its multiplicity keeps an open initializer's count, where
// a stated one judges it; the effective `[1]` of an omitted multiplicity is not applied.
func TestBodyLocalKeepsOpenInitializerCount(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		attribute xs : Integer[2..*];
		constraint def Open {
			attribute kept = xs;
			attribute one[1] = xs;
			attribute two[2..*] = xs;
			SequenceFunctions::size(kept) >= 2
		}
	}`))
	open := oneSymbol(t, idx, "test::Open")
	ec := NewEvalContext(ctx, open.Scope)
	initializer := func() Value {
		return NewUndeterminedValue(noValueReason("xs"), semantics.Range{
			Lower: semantics.Bound{Known: true, Value: 2}, Upper: semantics.Bound{Known: true, Infinite: true},
		})
	}
	local := func(name string) *symbols.Symbol {
		sym, ok := open.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("test::Open::%s not declared", name)
		}
		return sym
	}
	val, err := ec.conformBodyDeclared(local("kept"), initializer())
	wantUndetermined(t, "kept", val, err, "[2..*]")
	val, err = ec.conformBodyDeclared(local("two"), initializer())
	wantUndetermined(t, "two", val, err, "[2..*]")
	if _, err := ec.conformBodyDeclared(local("one"), initializer()); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("one[1] = xs: err = %v; want ErrMultiplicityViolation", err)
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

// An open operand is judged by the type its feature declares before the unknown is
// carried: an operator or function undefined for that type is the same type mismatch
// a determined value of it raises, while one it may hold stays undetermined.
func TestTypedOpenOperandsKeepOperatorDomains(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for _, src := range []string{
		"s - 1", "s + 1", "1 / s", "s * r", "s + r", "-s", "+s", "s > 1", "s < r", "b < b",
		"b - 1", "b > 1", "r + \"a\"", "flag - 1", "flag > 1",
		"not s", "not r", "s and true", "s and false", "true and s", "s or true", "s implies true",
		"r xor true", "s | true", "ControlFunctions::'and'(s, false)", "BooleanFunctions::'|'(true, r)",
		"RealFunctions::'-'(s, 1)", "RealFunctions::'<'(s, \"a\")", "IntegerFunctions::'+'(s, 1)",
		"if s ? 1 else 2", "if r ? 1 else 2", "(10, 20, 30)#(s)", "(10, 20, 30)#(b)",
		"RealFunctions::sqrt(s)", "RealFunctions::max(b, 1.0)", "StringFunctions::Length(r)",
		"twice(s)", "twice(rack.gear)",
	} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: err = %v; want ErrTypeMismatch", src, err)
		}
	}
	for src, count := range map[string]string{
		"s + \"a\"": "[1]", "\"a\" + s": "[1]", "s < \"a\"": "[1]", "s == 1": "[1]", "s == \"a\"": "[1]",
		"r - 1": "[1]", "-r": "[1]", "r > 1": "[1]", "r < r": "[1]", "s >= u": "[1]", "1 / r": "[1]", "r ** 2": "[1]",
		"RealFunctions::'-'(r, 1)": "[1]", "IntegerFunctions::'+'(r, 1)": "[1]", "DataFunctions::'<'(s, \"a\")": "[1]",
		"not b": "[1]", "b and true": "[1]", "b or false": "[1]", "if b ? 1 else 2": "[1]",
		"(10, 20, 30)#(r)": "[1]", "RealFunctions::sqrt(r)": "[1]", "StringFunctions::Length(s)": "[1]", "twice(r)": "[1]",
		"u - 1": "[1]", "not u": "[1]", "u and true": "[1]", "\"a\" + u": "[1]", "if u ? 1 else 2": "[1]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for src, want := range map[string]string{
		"b and false": "false", "false and s": "false", "b or true": "true", "b implies true": "true",
		"ControlFunctions::'and'(b, false)": "false", "ControlFunctions::'implies'(false, s)": "true",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	if _, err := evalIn(t, ctx, scope, "s - 1"); err == nil || !strings.Contains(err.Error(), "an undetermined String") {
		t.Errorf("s - 1: err = %v; want the operand described by its declared type", err)
	}
}

// A chain through an open collection reads its members from the values the
// collection certainly holds, so what their declarations fix answers membership and
// quantifiers, while the members of the unknown rest stay open.
func TestChainThroughOpenCollectionKeepsKnownValues(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, want := range map[string]string{
		"shelf.plain.tag": "\"d\"", "shelf.tagged.tag": "\"x\"",
		"includes(shelf.items.tag, \"d\")": "true", "includes(shelf.items.tag, \"x\")": "true",
		"excludes(shelf.items.tag, \"x\")": "false", "includes(shelf.items.w, 2.0)": "true",
		"shelf.items.tag->ControlFunctions::exists{in x; x == \"x\"}": "true",
		"shelf.items.tag->ControlFunctions::forAll{in x; x == \"d\"}": "false",
		"shelf.items.w->ControlFunctions::forAll{in x; x > 1.0}":      "<undetermined>",
		"notEmpty(shelf.items.tag)":                                   "true", "isEmpty(shelf.items.mass)": "false",
		"includes(rack.gear.tag, \"d\")": "true",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for src, count := range map[string]string{
		"shelf.items.tag": "[2..*]", "shelf.items.mass": "[2..*]", "shelf.items.w": "[2..*]",
		"includes(shelf.items.tag, \"zz\")": "[1]", "size(shelf.items.tag)": "[1]", "shelf.items.w == 2.0": "[1]",
		"rack.loose.tag": "[0..2]", "rack.gear.mass": "[1..*]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for src, known := range map[string]string{
		"shelf.items.tag": "[\"d\", \"x\"]", "shelf.items.w": "[2.0, 2.0]", "shelf.items.mass": "[]", "rack.gear.tag": "[\"d\"]",
	} {
		val, _ := evalIn(t, ctx, scope, src)
		if u := val.Undetermined(); u != nil {
			if got := FormatValue(sequenceOf(u.Known())); got != known {
				t.Errorf("%s certainly holds %s, want %s", src, got, known)
			}
		}
	}
}

// A named conditional checks that an open test may be Boolean before it stays open,
// as the `if ? :` operator does.
func TestNamedConditionalChecksOpenTest(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for _, src := range []string{
		"ControlFunctions::'if'(r, 1, 2)", "ControlFunctions::'if'(s, 1, 2)", "ControlFunctions::'if'(xs > 1.0, 1, 2)",
	} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: err = %v; want ErrTypeMismatch", src, err)
		}
	}
	for _, src := range []string{"ControlFunctions::'if'(bs, 1, 2)", "ControlFunctions::'if'(bs, xs, 2)"} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("%s: err = %v; want ErrMultiplicityViolation", src, err)
		}
	}
	for src, count := range map[string]string{
		"ControlFunctions::'if'(b, 1, 2)": "[1]", "ControlFunctions::'if'(u, 1, 2)": "[1]", "ControlFunctions::'if'(b, xs, 2)": "[1..4]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
}

// An open operand certainly holding several values is no scalar: one-valued operators and
// functions refuse it as they refuse a sequence; one that may hold one value stays open.
func TestSeveralOpenValuesAreNoScalar(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for _, src := range []string{
		"xs + 1", "1 - xs", "xs * xs", "xs > 1", "xs <= 1.0", "-xs", "+xs", "(10, 20)#(xs)",
		"ss + \"a\"", "bs and true", "false or bs", "not bs", "bs xor true", "if bs ? 1 else 2",
		"xs ** 2", "xs % 2",
	} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: err = %v; want ErrTypeMismatch", src, err)
		}
	}
	for _, src := range []string{
		"RealFunctions::'+'(xs, 1.0)", "RealFunctions::abs(xs)", "RealFunctions::sqrt(xs)", "RealFunctions::max(xs, 1.0)",
		"StringFunctions::Length(ss)", "StringFunctions::Substring(ss, 1, 1)", "BooleanFunctions::'|'(true, bs)",
		"ControlFunctions::'and'(bs, true)", "twice(xs)",
	} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("%s: err = %v; want ErrMultiplicityViolation", src, err)
		}
	}
	if _, err := evalIn(t, ctx, scope, "xs + 1"); err == nil || !strings.Contains(err.Error(), "an undetermined Real sequence") {
		t.Errorf("xs + 1: err = %v; want the operand described as a sequence", err)
	}
	for src, count := range map[string]string{
		"os + 1": "[0..1]", "-os": "[0..1]", "os > 1": "[0..1]", "(10, 20)#(os)": "[1]", "twice(os)": "[0..1]",
		"RealFunctions::abs(os)": "[1]", "xs == 1.0": "[1]", "xs != xs": "[1]", "xs === xs": "[1]", "xs ?? 3": "[2..4]",
		"size(xs)": "[1]", "head(xs)": "[1]", "includes(xs, 1.0)": "[1]", "xs#(1)": "[1]",
		"xs->ControlFunctions::forAll{in x; x > 1.0}": "[1]", "xs->ControlFunctions::collect{in x; x + 1.0}": "[2..4]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
}

// A model-level read reads a collection's open subsetters as it reads the collection:
// not made up to their lower bounds, contributing the fewest they hold and no objects.
func TestOpenSubsettersAreNotMaterializedAtModelLevel(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	for src, count := range map[string]string{
		"store.base": "[10001..*]", "store.sub": "[10001..*]", "store.base.mass": "[10001..*]",
		"store.pool": "[5..*]", "store.pool#(1)": "[1]", "store.cap": "[3]",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantUndetermined(t, src, val, err, count)
	}
	for src, want := range map[string]string{
		"notEmpty(store.base)": "true", "isEmpty(store.base)": "false", "size(store.cap)": "3",
		"notEmpty(store.pool)": "true", "includes(store.pool, store.inner#(1))": "true",
	} {
		val, err := evalIn(t, ctx, scope, src)
		wantFormatted(t, src, val, err, want)
	}
	for src, known := range map[string]int{"store.base": 0, "store.cap": 0, "store.pool": 2} {
		val, _ := evalIn(t, ctx, scope, src)
		if u := val.Undetermined(); u != nil && len(u.Known()) != known {
			t.Errorf("%s certainly holds %d objects, want %d", src, len(u.Known()), known)
		}
	}
}
