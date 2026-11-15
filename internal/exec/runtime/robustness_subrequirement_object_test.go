package runtime

import (
	"errors"
	"testing"
)

// subrequirementObjectModel holds requirement usages in a part whose subrequirements read a subject
// the container leaves unbound, declare one nothing binds, or are typed for another subject.
const subrequirementObjectModel = `
package test {
	private import ScalarValues::*;
	part def Veh { attribute mass : Real; }
	part def Eng { attribute power : Real; }
	requirement def LightReq { subject v : Veh; require constraint { v.mass < 10.0 } }
	requirement def PowerReq { subject e : Eng; require constraint { e.power > 1.0 } }
	requirement def R { subject v : Veh; requirement light : LightReq; requirement inl { require constraint { v.mass < 10.0 } } }
	requirement def Owned { subject v : Veh; requirement own { subject s : Veh; require constraint { s.mass < 10.0 } } }
	requirement def Mistyped { subject v : Veh; requirement pow : PowerReq; }
	part veh : Veh { attribute :>> mass = 20.0; }
	part box {
		requirement bare : R;
		requirement owned : Owned { subject v = veh; }
		requirement mistyped : Mistyped { subject v = veh; }
	}
}
`

// TestRuntimeRobustnessSubrequirementObject covers the reads through a subrequirement's object that reach
// no verdict: each is undetermined or a typed error, never a verdict, a panic or a value made up.
func TestRuntimeRobustnessSubrequirementObject(t *testing.T) {
	read := func(t *testing.T, src string) (Value, error) {
		ctx, idx := libraryShapeContext(t, subrequirementObjectModel)
		return evalIn(t, ctx, idx.DocumentRoot("<test>"), src)
	}
	for _, src := range []string{
		"test::box.bare.light.result", "test::box.bare.inl.result", "test::box.bare.inl.subj", "test::box.bare.light.v",
	} {
		t.Run("container_subject_unbound/"+src, func(t *testing.T) {
			val, err := read(t, src)
			wantUndetermined(t, src, val, err, "[1]")
		})
	}
	for _, src := range []string{"test::box.owned.own.result", "test::box.owned.own.s", "test::box.owned.result"} {
		t.Run("owned_subject_unbound/"+src, func(t *testing.T) {
			val, err := read(t, src)
			wantUndetermined(t, src, val, err, "[1]")
		})
	}
	t.Run("container_subject_refused_by_type", func(t *testing.T) {
		if val, err := read(t, "test::box.mistyped.pow.e"); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("pow.e = %s, %v; want the subject's type to refuse the container's", FormatValue(val), err)
		}
		if val, err := read(t, "test::box.mistyped.pow.result"); !errors.Is(err, ErrNoSuchFeature) {
			t.Errorf("pow.result = %s, %v; want the condition to find no power on a Veh", FormatValue(val), err)
		}
	})
}

// boundSubrequirementModel binds subrequirements to no value, one and several, where the check of a
// value reaches no verdict: a feature of it is unbound, the binding is no requirement, or reads nothing.
const boundSubrequirementModel = `
package test {
	private import ScalarValues::*;
	part def Veh { attribute mass : Real; }
	part veh : Veh { attribute :>> mass = 20.0; }
	attribute unset : Integer;
	requirement def Limit { subject v : Veh; attribute max : Real; require constraint { v.mass < max } }
	requirement def Bad { subject v : Veh; require constraint { 1 / 0 > 0 } }
	requirement def Heavy { subject v : Veh; require constraint { v.mass > 10.0 } }
	requirement def Read { subject v : Veh; requirement s[0..1] : Heavy = new Heavy(); }
	requirement def Empty { subject v : Veh; requirement s[*] : Bad = (); }
	requirement def One { subject v : Veh; requirement s[0..1] : Limit = new Limit(); }
	requirement def Many { subject v : Veh; requirement s[*] : Limit = (new Limit(max = 30.0), new Limit()); }
	requirement def ManyFail { subject v : Veh; requirement s[*] : Limit = (new Limit(max = 5.0), new Limit()); }
	requirement def OverBound { subject v : Veh; requirement s[0..1] : Limit = (new Limit(max = 30.0), new Limit(max = 40.0)); }
	part box {
		requirement empty : Empty { subject v = veh; }
		requirement bare : Empty;
		requirement read : Read { subject v = veh; }
		requirement one : One { subject v = veh; }
		requirement oneBare : One;
		requirement many : Many { subject v = veh; }
		requirement manyFail : ManyFail { subject v = veh; }
		requirement overBound : OverBound { subject v = veh; }
	}
}
`

// TestRuntimeRobustnessBoundSubrequirement covers a container's check over the values bound to a
// subrequirement: none states nothing, and a value reaching no verdict leaves it undetermined or a typed error.
func TestRuntimeRobustnessBoundSubrequirement(t *testing.T) {
	read := func(t *testing.T, src string) (Value, error) {
		ctx, idx := libraryShapeContext(t, boundSubrequirementModel)
		return evalIn(t, ctx, idx.DocumentRoot("<test>"), src)
	}
	for _, src := range []string{"test::box.empty.result", "test::box.bare.result"} {
		t.Run("empty_checks_nothing/"+src, func(t *testing.T) {
			if val, err := read(t, src); err != nil || FormatValue(val) != "true" {
				t.Errorf("%s = %s, %v; want true: no value, so the failing definition is not judged", src, FormatValue(val), err)
			}
		})
	}
	for _, src := range []string{"test::box.one.result", "test::box.oneBare.result", "test::box.many.result"} {
		t.Run("value_feature_unbound/"+src, func(t *testing.T) {
			val, err := read(t, src)
			wantUndetermined(t, src, val, err, "[1]")
		})
	}
	t.Run("multiple_one_violated_one_unbound", func(t *testing.T) {
		if val, err := read(t, "test::box.manyFail.result"); err != nil || FormatValue(val) != "false" {
			t.Errorf("manyFail.result = %s, %v; want false: one value is violated whatever the other reads", FormatValue(val), err)
		}
	})
	t.Run("read_through_a_bound_value", func(t *testing.T) {
		// The value is not held as the container's own, so its subject stays unbound where the check binds it.
		if val, err := read(t, "test::box.read.result"); err != nil || FormatValue(val) != "true" {
			t.Errorf("read.result = %s, %v; want true", FormatValue(val), err)
		}
		val, err := read(t, "test::box.read.s.result")
		wantUndetermined(t, "test::box.read.s.result", val, err, "[1]")
	})
	t.Run("multiple_over_the_multiplicity", func(t *testing.T) {
		if val, err := read(t, "test::box.overBound.result"); !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("overBound.result = %s, %v; want the two values refused by [0..1]", FormatValue(val), err)
		}
	})
}
