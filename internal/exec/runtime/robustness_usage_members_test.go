package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessUsageMembers: a calc or constraint usage's own members read
// through a chain as any usage's do; a name the usage declares nowhere is a typed
// error, and an input nothing supplies stays undetermined rather than failing.
func TestRuntimeRobustnessUsageMembers(t *testing.T) {
	t.Run("calc member nothing declares", testUsageMembersCalcUndeclared)
	t.Run("unsupplied calc input is undetermined", testUsageMembersUnsuppliedCalcInput)
	t.Run("constraint member nothing declares", testUsageMembersConstraintUndeclared)
	t.Run("unbound constraint input is undetermined", testUsageMembersUnboundConstraintInput)
	t.Run("state body members are features of the state", testUsageMembersStateBody)
	t.Run("parameter of an uninvoked action is undetermined on an instance", testUsageMembersUninvokedParameter)
}

const usageMembersProbeModel = `package Probe {
	part def Rig {
		attribute base : ScalarValues::Real = 4.0;
		calc area { in w : ScalarValues::Real = base; attribute scale : ScalarValues::Real = 10.0; return : ScalarValues::Real = w * scale; }
		calc scaled { in w : ScalarValues::Real; in k : ScalarValues::Real = 2.0; return : ScalarValues::Real = w * k; }
		constraint fits { in limit : ScalarValues::Real = 100.0; attribute margin : ScalarValues::Real = 5.0; base < limit }
		constraint under { in limit : ScalarValues::Real; limit > base }
		attribute fitsVerdict : ScalarValues::Boolean = fits;
		attribute underVerdict : ScalarValues::Boolean = under;
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
	if err != nil || val.Kind != ValUndetermined || !strings.Contains(val.Undetermined().reason, "nothing supplies") {
		t.Fatalf("Probe::rig.scaled.w = %s, %v; want undetermined for the input nothing supplies", FormatValue(val), err)
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
	if val, err := eval("Probe::rig.fits.limit"); err != nil || FormatValue(val) != "100.0" {
		t.Fatalf("Probe::rig.fits.limit = %s, %v; want the default 100.0", FormatValue(val), err)
	}
	if val, err := eval("Probe::rig.fits"); err != nil || FormatValue(val) != "true" {
		t.Fatalf("Probe::rig.fits = %s, %v; want the verdict", FormatValue(val), err)
	}
	if val, err := eval("Probe::rig.fitsVerdict"); err != nil || FormatValue(val) != "true" {
		t.Fatalf("Probe::rig.fitsVerdict = %s, %v; want the verdict the bare name reads in the owner's body", FormatValue(val), err)
	}
	for _, src := range []string{"Probe::rig.under", "Probe::rig.under.result", "Probe::rig.underVerdict", "new Probe::Rig().under", "new Probe::Rig().under.result", "new Probe::Rig().underVerdict"} {
		val, err := eval(src)
		if err != nil || val.Kind != ValUndetermined || !strings.Contains(val.Undetermined().reason, "no argument and no default") {
			t.Fatalf("%s = %s, %v; want an undetermined verdict naming the input nothing binds, not a failed read", src, FormatValue(val), err)
		}
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
