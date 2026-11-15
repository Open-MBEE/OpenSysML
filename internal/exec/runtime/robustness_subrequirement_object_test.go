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
