package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessBindingCrossMultiplicity covers a binding end's multiplicity as the cross multiplicity
// of KerML 1.1 §7.4.5.2, §8.4.4.6.1 over unlike, empty and absent ends: a value, a verdict or a typed error.
func TestRuntimeRobustnessBindingCrossMultiplicity(t *testing.T) {
	t.Run("required_ends_hold_the_same_values_whatever_their_number", testCrossRequiredEndsWhole)
	t.Run("verdict_follows_the_required_end", testCrossVerdictFollowsRequiredEnd)
	t.Run("end_reaching_no_object", testCrossEndReachingNoObject)
	t.Run("no_value_is_linked_to_several", testCrossUnlinkableEnd)
	t.Run("subsetter_draws_what_a_required_end_links", testCrossSubsetterDraws)
}

func crossRig(t *testing.T, body string) (*Context, *Instance) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<binding-cross-multiplicity>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		part def Hub { attribute spokes : Integer [2] = (1, 2); }
		part def Rig { `+body+` }
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Rig"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	return ctx, inst
}

func crossVerdicts(t *testing.T, ctx *Context, inst *Instance) map[string]ObjectVerdict {
	t.Helper()
	report, err := ctx.ValidateObject(inst, nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	verdicts := map[string]ObjectVerdict{}
	for _, v := range report.Verdicts {
		verdicts[v.Text] = v
	}
	return verdicts
}

// `[1]` ends link every value of each end to itself among the other's, so the ends hold the same
// values: a valueless end takes the other's, in either reading order, whatever their number.
func testCrossRequiredEndsWhole(t *testing.T) {
	for _, order := range [][]string{{"ys", "xs"}, {"xs", "ys"}} {
		ctx, inst := crossRig(t, `attribute xs : Integer [3] = (1, 2, 3);
			attribute ys : Integer [3];
			bind [1] xs = [1] ys;`)
		for _, name := range order {
			fv, err := inst.GetFeatureValue(ctx, name)
			if err != nil {
				t.Fatalf("%v: %s: %v", order, name, err)
			}
			if got := FormatValue(fv.HeldValue()); got != "[1, 2, 3]" {
				t.Fatalf("%v: %s = %s, want [1, 2, 3]", order, name, got)
			}
		}
	}
	// Both valued and unlike, the binding identifies values that are not the same.
	ctx, inst := crossRig(t, `attribute xs : Integer [2] = (1, 2);
		attribute ys : Integer [3] = (1, 2, 3);
		bind [1] xs = [1] ys;`)
	var conflict *bindingConflictError
	if _, err := inst.GetFeatureValue(ctx, "ys"); !errors.As(err, &conflict) {
		t.Fatalf("ys = %v, want bindingConflictError", err)
	}
}

// The end stating `[1]` is the one whose values cover the other's: a smaller optional end within
// a larger required one holds, the reverse is violated, and optional ends alone state nothing.
func testCrossVerdictFollowsRequiredEnd(t *testing.T) {
	ctx, inst := crossRig(t, `attribute few : Integer [2] = (1, 2);
		attribute many : Integer [3] = (1, 2, 3);
		attribute other : Integer [2] = (8, 9);
		bind [0..1] few = [1] many;
		bind [1] few = [0..1] many;
		bind [0..1] few = [0..1] other;
		binding [1] bind [0..1] few = [0..1] other;`)
	verdicts := crossVerdicts(t, ctx, inst)
	for text, want := range map[string]ValidationStatus{
		"bind [0..1] few = [1] many":                 ValidationHolds,
		"bind [1] few = [0..1] many":                 ValidationViolated,
		"bind [0..1] few = [0..1] other":             ValidationHolds,
		"binding [1] bind [0..1] few = [0..1] other": ValidationViolated,
	} {
		v, ok := verdicts[text]
		if !ok || v.Kind != AssertionBinding || v.Status != want {
			t.Errorf("verdict %q = %s (%v), want %s", text, v.Status, v.Err, want)
		}
	}
	if v := verdicts["bind [1] few = [0..1] many"]; v.Err == nil ||
		!strings.Contains(v.Err.Error(), "1 value(s) of many are not values of few") {
		t.Errorf("violation = %v, want it to name the value of many that few lacks", v.Err)
	}
}

// Over an end reaching no object nothing is linked: a binding requiring no link states nothing and
// leaves the other end unread; one whose connector or empty end requires a link is violated.
func testCrossEndReachingNoObject(t *testing.T) {
	ctx, inst := crossRig(t, `part hubs : Hub [0..*];
		attribute mine : Integer [1] = 1;
		attribute none : Integer [0..*];
		bind [0..1] hubs.spokes = [1] mine;
		bind [1] hubs.spokes = [0..1] mine;
		bind [1] hubs.spokes = [0..1] none;
		binding [1] bind [0..1] hubs.spokes = [0..1] mine;`)
	verdicts := crossVerdicts(t, ctx, inst)
	if _, ok := verdicts["bind [0..1] hubs.spokes = [1] mine"]; ok {
		t.Errorf("a binding requiring no link over no object has a verdict: %+v", verdicts)
	}
	for text, want := range map[string]ValidationStatus{
		"bind [1] hubs.spokes = [0..1] mine":                ValidationViolated,
		"bind [1] hubs.spokes = [0..1] none":                ValidationHolds,
		"binding [1] bind [0..1] hubs.spokes = [0..1] mine": ValidationViolated,
	} {
		v, ok := verdicts[text]
		if !ok || v.Status != want {
			t.Errorf("verdict %q = %s (%v), want %s", text, v.Status, v.Err, want)
		}
	}
	if fv, err := inst.GetFeatureValue(ctx, "mine"); err != nil || FormatValue(fv.HeldValue()) != "1" {
		t.Errorf("mine = %v, %v; want its own 1, no multiplicity error over the empty end", fv, err)
	}
}

// A binding links a value to itself alone, so an end requiring two or more per value of the other is a
// multiplicity violation where it determines a value, a violated verdict where it constrains, met over none.
func testCrossUnlinkableEnd(t *testing.T) {
	ctx, inst := crossRig(t, `attribute xs : Integer [2] = (1, 2);
		attribute ys : Integer [2];
		bind [2] xs = [2] ys;`)
	_, err := inst.GetFeatureValue(ctx, "ys")
	if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "links each value of ys to [2] of xs") {
		t.Fatalf("ys = %v, want ErrMultiplicityViolation naming the cross multiplicity", err)
	}
	ctx, inst = crossRig(t, `attribute xs : Integer [2] = (1, 2);
		attribute ys : Integer [2] = (1, 2);
		attribute none : Integer [0..*];
		bind [2] xs = [0..1] ys;
		bind [2] xs = [0..1] none;`)
	verdicts := crossVerdicts(t, ctx, inst)
	if v := verdicts["bind [2] xs = [0..1] ys"]; v.Status != ValidationViolated {
		t.Errorf("verdict = %s (%v), want violated: no value of ys is linked to two of xs", v.Status, v.Err)
	}
	if v := verdicts["bind [2] xs = [0..1] none"]; v.Status != ValidationHolds {
		t.Errorf("verdict = %s (%v), want holds: none holds no value to link", v.Status, v.Err)
	}
}

// A subsetter under a binding with a required end draws every value that end links, not one:
// the values the runtime chooses for it satisfy the binding, whichever end requires the link.
func testCrossSubsetterDraws(t *testing.T) {
	for _, bound := range []string{"bind [1] picked = [0..1] wanted;", "bind [0..1] picked = [1] wanted;"} {
		ctx, inst := crossRig(t, `attribute pool : Integer [3] = (1, 2, 3);
			attribute picked : Integer [2] :> pool;
			attribute wanted : Integer [2] = (2, 3);
			`+bound)
		fv, err := inst.GetFeatureValue(ctx, "picked")
		if err != nil {
			t.Fatalf("%s: picked: %v", bound, err)
		}
		if got := FormatValue(fv.HeldValue()); got != "[2, 3]" {
			t.Fatalf("%s: picked = %s, want [2, 3]", bound, got)
		}
		for text, v := range crossVerdicts(t, ctx, inst) {
			if v.Status != ValidationHolds {
				t.Errorf("%s: %s is %v, want holds", bound, text, v.Status)
			}
		}
	}
}
