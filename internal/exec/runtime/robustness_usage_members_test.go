package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestRuntimeRobustnessUsageMembers: a calc or constraint usage's own members read
// through a chain as any usage's do; a name the usage declares nowhere is a typed
// error, and an input nothing supplies stays undetermined rather than failing.
func TestRuntimeRobustnessUsageMembers(t *testing.T) {
	t.Run("calc member nothing declares", testUsageMembersCalcUndeclared)
	t.Run("unsupplied calc input is undetermined", testUsageMembersUnsuppliedCalcInput)
	t.Run("constraint member nothing declares", testUsageMembersConstraintUndeclared)
	t.Run("unbound constraint input is undetermined", testUsageMembersUnboundConstraintInput)
	t.Run("unsupplied input is undetermined whatever its lower bound", testUsageMembersStatedLowerBound)
	t.Run("default reading an unsupplied input is open with it", testUsageMembersDependentDefault)
	t.Run("open-input read survives a scope switch", testUsageMembersOpenInputsScope)
	t.Run("state body members are features of the state", testUsageMembersStateBody)
	t.Run("parameter of an uninvoked action is undetermined on an instance", testUsageMembersUninvokedParameter)
	t.Run("bodiless entry action of an exhibited state is a known gap", testUsageMembersExhibitedState)
}

const usageMembersProbeModel = `package Probe {
	part def Rig {
		attribute base : ScalarValues::Real = 4.0;
		calc area { in w : ScalarValues::Real = base; attribute scale : ScalarValues::Real = 10.0; return : ScalarValues::Real = w * scale; }
		calc scaled { in w : ScalarValues::Real; in k : ScalarValues::Real = 2.0; return : ScalarValues::Real = w * k; }
		constraint fits { in limit : ScalarValues::Real = 100.0; attribute margin : ScalarValues::Real = 5.0; base < limit }
		constraint under { in limit : ScalarValues::Real; limit > base }
		constraint masked { in base : ScalarValues::Real; base > 1.0 }
		calc scaledOne { in w : ScalarValues::Real[1]; in k : ScalarValues::Real = 2.0; attribute s : ScalarValues::Real = 5.0; return : ScalarValues::Real = w * k; }
		constraint underOne { in limit : ScalarValues::Real[1]; limit > base }
		analysis def Rated { subject s : ScalarValues::Real; in n : ScalarValues::Real[1]; return : ScalarValues::Real = s * 2.0; }
		analysis rated : Rated { subject s = base; }
		analysis def Scored { subject s : ScalarValues::Real; in n : ScalarValues::Real[1]; attribute loc : ScalarValues::Real = n + 1.0; return : ScalarValues::Real = s * n; }
		analysis scored : Scored { subject s = base; }
		verification def Checked { subject s : ScalarValues::Real; in n : ScalarValues::Real[1]; return verdict : VerificationCases::VerdictKind = VerificationCases::PassIf(s > n); }
		verification checked : Checked { subject s = base; }
		action act { in i : ScalarValues::Integer; out o : ScalarValues::Integer; attribute k : ScalarValues::Integer = 3; }
		state def M;
		state sm : M { entry action e { attribute who = this; } state idle; state busy; transition idle then busy; }
	}
	part rig : Rig;
}`

func usageMembersEval(t *testing.T) func(string) (Value, error) {
	t.Helper()
	ctx, idx := libraryShapeContext(t, usageMembersProbeModel)
	root := idx.DocumentRoot("<test>")
	return func(src string) (Value, error) { return evalIn(t, ctx, root, src) }
}

func testUsageMembersCalcUndeclared(t *testing.T) {
	eval := usageMembersEval(t)
	if val, err := eval("Probe::rig.area.scale"); err != nil || FormatValue(val) != "10.0" {
		t.Fatalf("Probe::rig.area.scale = %s, %v; want the body local 10.0", FormatValue(val), err)
	}
	_, err := eval("Probe::rig.area.nothing")
	if !errors.Is(err, errUnknownOutput) {
		t.Fatalf("Probe::rig.area.nothing = %v, want errUnknownOutput", err)
	}
}

func testUsageMembersUnsuppliedCalcInput(t *testing.T) {
	eval := usageMembersEval(t)
	val, err := eval("Probe::rig.scaled.w")
	if err != nil || val.Kind != ValUndetermined || !strings.Contains(val.Undetermined().reason, "binds no value") {
		t.Fatalf("Probe::rig.scaled.w = %s, %v; want undetermined for the input nothing supplies", FormatValue(val), err)
	}
	if val, err := eval("Probe::rig.scaled.result"); err != nil || val.Kind != ValUndetermined || !strings.Contains(val.Undetermined().reason, "nothing supplies") {
		t.Fatalf("Probe::rig.scaled.result = %s, %v; want undetermined, the usage computing nothing without w", FormatValue(val), err)
	}
	if val, err := eval("Probe::rig.scaled.k"); err != nil || FormatValue(val) != "2.0" {
		t.Fatalf("Probe::rig.scaled.k = %s, %v; want the default 2.0", FormatValue(val), err)
	}
	if val, err := eval("Probe::rig.scaled"); err != nil || val.Kind != ValFunction {
		t.Fatalf("Probe::rig.scaled = %s, %v; want the function value the usage reads as", FormatValue(val), err)
	}
}

func testUsageMembersConstraintUndeclared(t *testing.T) {
	eval := usageMembersEval(t)
	if val, err := eval("Probe::rig.fits.margin"); err != nil || FormatValue(val) != "5.0" {
		t.Fatalf("Probe::rig.fits.margin = %s, %v; want the body local 5.0", FormatValue(val), err)
	}
	_, err := eval("Probe::rig.fits.nothing")
	if !errors.Is(err, ErrNoSuchFeature) || !strings.Contains(err.Error(), "constraint fits has no member nothing") {
		t.Fatalf("Probe::rig.fits.nothing = %v, want ErrNoSuchFeature naming the constraint", err)
	}
}

func testUsageMembersUnboundConstraintInput(t *testing.T) {
	eval := usageMembersEval(t)
	val, err := eval("Probe::rig.under.limit")
	if err != nil || val.Kind != ValUndetermined {
		t.Fatalf("Probe::rig.under.limit = %s, %v; want undetermined for the input nothing binds", FormatValue(val), err)
	}
	// A parameter stating no multiplicity is held to [0..*], so its one default is a one-element sequence.
	if val, err := eval("Probe::rig.fits.limit"); err != nil || FormatValue(val) != "[100.0]" {
		t.Fatalf("Probe::rig.fits.limit = %s, %v; want the default held as [100.0]", FormatValue(val), err)
	}
	if val, err := eval("Probe::rig.fits"); err != nil || FormatValue(val) != "true" {
		t.Fatalf("Probe::rig.fits = %s, %v; want the verdict", FormatValue(val), err)
	}
	if val, err := eval("Probe::rig.masked.base"); err != nil || val.Kind != ValUndetermined {
		t.Fatalf("Probe::rig.masked.base = %s, %v; want undetermined: the input, not the owner's same-named feature", FormatValue(val), err)
	}
	for _, src := range []string{"Probe::rig.under", "Probe::rig.under.result", "new Probe::Rig().under", "new Probe::Rig().under.result"} {
		val, err := eval(src)
		if err != nil || val.Kind != ValUndetermined || !strings.Contains(val.Undetermined().reason, "no argument and no default") {
			t.Fatalf("%s = %s, %v; want an undetermined verdict naming the input nothing binds, not a failed read", src, FormatValue(val), err)
		}
	}
}

// An input declared `[1]` that nothing supplies reads as the unstated form does:
// undetermined, with the outputs and body locals the usage computes nothing for, on a
// calc and a constraint alike, and the count of an unstated one is the same on both.
func testUsageMembersStatedLowerBound(t *testing.T) {
	eval := usageMembersEval(t)
	for _, src := range []string{
		"Probe::rig.scaledOne.w", "Probe::rig.scaledOne.s", "Probe::rig.scaledOne.result", "new Probe::Rig().scaledOne.result",
		"Probe::rig.underOne.limit", "Probe::rig.underOne", "Probe::rig.underOne.result", "new Probe::Rig().underOne",
	} {
		val, err := eval(src)
		if err != nil || val.Kind != ValUndetermined {
			t.Fatalf("%s = %s, %v; want undetermined: the lower bound does not make an unsupplied input an error", src, FormatValue(val), err)
		}
	}
	for _, src := range []string{"Probe::rig.rated.n", "new Probe::Rig().rated.n"} {
		if val, err := eval(src); err != nil || val.Kind != ValUndetermined {
			t.Fatalf("%s = %s, %v; want undetermined: a case usage read as a feature leaves the input open too", src, FormatValue(val), err)
		}
	}
	for _, src := range []string{
		"Probe::rig.rated.result", "new Probe::Rig().rated.result",
		"Probe::rig.scored.loc", "Probe::rig.scored.result", "new Probe::Rig().scored.result",
		"Probe::rig.checked.verdict", "new Probe::Rig().checked.verdict",
	} {
		val, err := eval(src)
		if err != nil || val.Kind != ValUndetermined || !strings.Contains(val.Undetermined().reason, `parameter "n"`) {
			t.Fatalf("%s = %s, %v; want undetermined naming n: a case computes nothing without the input, its body never running on a null", src, FormatValue(val), err)
		}
	}
	if val, err := eval("Probe::rig.scaledOne.k"); err != nil || FormatValue(val) != "2.0" {
		t.Fatalf("Probe::rig.scaledOne.k = %s, %v; want the default 2.0", FormatValue(val), err)
	}
	if val, err := eval("Probe::rig.scaledOne"); err != nil || val.Kind != ValFunction {
		t.Fatalf("Probe::rig.scaledOne = %s, %v; want the function value the usage reads as", FormatValue(val), err)
	}
	calcInput, err := eval("Probe::rig.scaled.w")
	if err != nil || calcInput.Kind != ValUndetermined {
		t.Fatalf("Probe::rig.scaled.w = %s, %v; want undetermined", FormatValue(calcInput), err)
	}
	constraintInput, err := eval("Probe::rig.under.limit")
	if err != nil || constraintInput.Kind != ValUndetermined {
		t.Fatalf("Probe::rig.under.limit = %s, %v; want undetermined", FormatValue(constraintInput), err)
	}
	if calcInput.Undetermined().count != constraintInput.Undetermined().count {
		t.Fatalf("unsupplied input counts differ: calc %v, constraint %v; want the effective parameter range on both", calcInput.Undetermined().count, constraintInput.Undetermined().count)
	}
	for _, src := range []string{"Probe::rig.scaled.w->SequenceFunctions::size()", "Probe::rig.under.limit->SequenceFunctions::size()"} {
		if val, err := eval(src); err != nil || val.Kind != ValUndetermined {
			t.Fatalf("%s = %s, %v; want undetermined: the input may hold any number of values", src, FormatValue(val), err)
		}
	}
}

const usageMembersDependentDefaultModel = `package Probe {
	part def P {
		calc f { in x : ScalarValues::Integer[1]; in y : ScalarValues::Integer = x + 1; return : ScalarValues::Integer = y; }
		constraint c { in x : ScalarValues::Integer[1]; in y : ScalarValues::Integer = x + 1; y > 0 }
		calc h { in x : ScalarValues::Integer[0..1]; return : ScalarValues::Integer[0..1] = x; }
	}
	part p : P;
	analysis def A { subject s : ScalarValues::Integer; in n : ScalarValues::Integer[1]; in m : ScalarValues::Integer = n * 2; return : ScalarValues::Integer = m; }
	analysis a : A { subject s = 1; }
	analysis def S { subject s : ScalarValues::Integer; in m : ScalarValues::Integer[0..1]; return : ScalarValues::Integer[0..1] = m; }
	analysis sopt : S { subject s = 1; }
}`

// A default reading an input nothing supplies is open with it: a usage read as a
// feature binds that input undetermined, so the default, the result and the check read
// undetermined rather than failing on a null; an invocation still wants the argument.
func testUsageMembersDependentDefault(t *testing.T) {
	ctx, idx := libraryShapeContext(t, usageMembersDependentDefaultModel)
	root := idx.DocumentRoot("<test>")
	for _, src := range []string{
		"Probe::p.f.x", "Probe::p.f.y", "Probe::p.f.result", "new Probe::P().f.y", "new Probe::P().f.result",
		"Probe::p.c.x", "Probe::p.c.y", "Probe::p.c", "new Probe::P().c.y", "new Probe::P().c",
		"Probe::a.n", "Probe::a.m", "Probe::a.result",
	} {
		val, err := evalIn(t, ctx, root, src)
		if err != nil || val.Kind != ValUndetermined {
			t.Fatalf("%s = %s, %v; want undetermined: nothing supplies the input the default reads", src, FormatValue(val), err)
		}
	}
	// An unsupplied input whose range admits none binds empty, as an omitted argument
	// does, and the run still computes: its result is the empty value, not undetermined.
	for _, src := range []string{"Probe::p.h.result", "new Probe::P().h.result", "Probe::sopt.result"} {
		if val, err := evalIn(t, ctx, root, src); err != nil || val.Kind != ValNull {
			t.Fatalf("%s = %s, %v; want null: the optional input binds empty and the body runs", src, FormatValue(val), err)
		}
	}
	for _, src := range []string{"Probe::p.h.x", "Probe::sopt.m"} {
		if val, err := evalIn(t, ctx, root, src); err != nil || val.Kind != ValUndetermined {
			t.Fatalf("%s = %s, %v; want undetermined: nothing supplies the optional input", src, FormatValue(val), err)
		}
	}
	if val, err := evalIn(t, ctx, root, "Probe::p.f(2)"); err != nil || FormatValue(val) != "[3]" {
		t.Fatalf("Probe::p.f(2) = %s, %v; want [3]", FormatValue(val), err)
	}
	if _, err := evalIn(t, ctx, root, "Probe::p.f()"); !errors.Is(err, errUnboundParameter) {
		t.Fatalf("Probe::p.f() = %v, want errUnboundParameter: an invocation binds its parameters", err)
	}
}

// A member read of a condition stays an open-input read in every context derived from
// it: a default declared in another scope (an inherited one) must not fall back to the
// object's same-named feature when it reads an input nothing binds.
func testUsageMembersOpenInputsScope(t *testing.T) {
	ctx, idx := libraryShapeContext(t, usageMembersDependentDefaultModel)
	ec := NewEvalContextIn(ctx, idx.DocumentRoot("<test>"), nil)
	ec.openInputs = true
	other := new(symbols.Scope)
	if !ec.evalIn(other).openInputs {
		t.Fatalf("evalIn dropped the open-input read")
	}
	if !ec.over(other, nil).openInputs {
		t.Fatalf("over dropped the open-input read")
	}
	if ec.evalIn(ec.scope) != ec {
		t.Fatalf("evalIn of the same scope is a new context")
	}
}

func testUsageMembersStateBody(t *testing.T) {
	eval := usageMembersEval(t)
	if val, err := eval("Probe::rig.sm.e"); err != nil || val.Kind != ValInstance {
		t.Fatalf("Probe::rig.sm.e = %s, %v; want the entry action the state holds", FormatValue(val), err)
	}
	for _, src := range []string{"Probe::rig.sm.idle", "Probe::rig.sm.busy"} {
		if val, err := eval(src); err != nil || val.Kind == ValInvalid {
			t.Fatalf("%s = %s, %v; want a member of the state, not a missing feature", src, FormatValue(val), err)
		}
	}
	_, err := eval("Probe::rig.sm.nothing")
	if !errors.Is(err, ErrNoSuchFeature) {
		t.Fatalf("Probe::rig.sm.nothing = %v, want ErrNoSuchFeature", err)
	}
}

func testUsageMembersUninvokedParameter(t *testing.T) {
	eval := usageMembersEval(t)
	for _, src := range []string{"Probe::rig.act.i", "Probe::rig.act.o", "new Probe::Rig().act.i", "new Probe::Rig().act.o"} {
		val, err := eval(src)
		if err != nil || val.Kind != ValUndetermined {
			t.Fatalf("%s = %s, %v; want undetermined: nothing invoked act to bind its parameter", src, FormatValue(val), err)
		}
	}
	for _, src := range []string{"Probe::rig.act.k", "new Probe::Rig().act.k"} {
		if val, err := eval(src); err != nil || FormatValue(val) != "3" {
			t.Fatalf("%s = %s, %v; want the body attribute 3", src, FormatValue(val), err)
		}
	}
}

const usageMembersExhibitedModel = `package Probe {
	part def P {
		state def M;
		exhibit state sm : M { entry action e; then idle; state idle; state busy; transition idle then busy; }
	}
	part p : P;
}`

// A bodiless, untyped entry action of an exhibited state is the action object the
// state holds, at model level and on a constructed object alike (SysML v2 §7.18.2).
func testUsageMembersExhibitedState(t *testing.T) {
	ctx, idx := libraryShapeContext(t, usageMembersExhibitedModel)
	root := idx.DocumentRoot("<test>")
	for _, src := range []string{"Probe::p.sm.e", "new Probe::P().sm.e"} {
		val, err := evalIn(t, ctx, root, src)
		if err != nil || val.Kind != ValInstance {
			t.Fatalf("%s = %s, %v; want the held entry action", src, FormatValue(val), err)
		}
	}
}
