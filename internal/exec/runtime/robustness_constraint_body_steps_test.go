package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessConstraintBodySteps covers the failure and isolation modes of
// the steps a constraint body performs: a write reaching outside the body's own
// performance is refused and leaves the world it would have written unchanged, a
// parameter written is the performance's copy and binds nothing back, two checks of
// one body share no state, and a loop in the body spends the run's step budget.
func TestRuntimeRobustnessConstraintBodySteps(t *testing.T) {
	t.Run("a refused write of the constrained object leaves it unchanged", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			part def Rig { attribute z : Real = 1; constraint writes { assign z := 99; z > 0 } }
		}`
		ctx, idx := contextForSource(t, src)
		rig := lookupOne(t, idx, "test::Rig")
		feat := featureNamed(ctx, rig, "writes")
		if feat == nil || feat.Symbol == nil {
			t.Fatal("constraint writes not found")
		}
		if _, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), nil); !errors.Is(err, ErrConstraintExternalAssignment) {
			t.Fatalf("err = %v, want ErrConstraintExternalAssignment", err)
		}
		same, err := evalIn(t, ctx, rig.Scope, "z == 1")
		if err != nil || !(same.isBool() && same.Const.Bool) {
			t.Fatalf("z == 1 after the refused write = %v, %v: the refused write changed the constrained feature", FormatValue(same), err)
		}
	})

	t.Run("a refused chained write leaves the chain's object unchanged", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			part def Inner { attribute w : Real = 1; }
			part def Rig {
				part inner : Inner;
				constraint writes { attribute v : Inner; assign v.w := 99; v.w > 0 }
			}
			part rig : Rig;
		}`
		ctx, idx := contextForSource(t, src)
		rig := lookupOne(t, idx, "test::Rig")
		feat := featureNamed(ctx, rig, "writes")
		if feat == nil || feat.Symbol == nil {
			t.Fatal("constraint writes not found")
		}
		if _, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), nil); !errors.Is(err, ErrConstraintEffect) {
			t.Fatalf("err = %v, want ErrConstraintEffect", err)
		}
		same, err := evalIn(t, ctx, lookupOne(t, idx, "test").Scope, "rig.inner.w == 1")
		if err != nil || !(same.isBool() && same.Const.Bool) {
			t.Fatalf("rig.inner.w == 1 after the refused write = %v, %v: the refused write changed the chained object", FormatValue(same), err)
		}
	})

	t.Run("a parameter written binds nothing back to the check's binding", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			part def Rig { attribute z : Real = 3; }
			part rig : Rig;
			constraint def Moves {
				in p : Real;
				assign p := p + 10;
				p > 10
			}
			constraint moved : Moves { in p = rig.z; }
		}`
		ctx, idx := contextForSource(t, src)
		moved := lookupOne(t, idx, "test::moved")
		satisfied, err := ctx.EvaluateConstraintOn(moved, moved.OwnerScope, nil)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if !satisfied {
			t.Error("the parameter's copy took the step's write, and the condition holds")
		}
		same, err := evalIn(t, ctx, lookupOne(t, idx, "test").Scope, "rig.z == 3")
		if err != nil || !(same.isBool() && same.Const.Bool) {
			t.Fatalf("rig.z == 3 after the parameter write = %v, %v: the parameter write reached the bound feature", FormatValue(same), err)
		}
	})

	t.Run("two checks of one body share no state", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			constraint def Counted {
				attribute n : Real = 0;
				assign n := n + 1;
				n == 1
			}
		}`
		ctx, idx := contextForSource(t, src)
		counted := lookupOne(t, idx, "test::Counted")
		for i := 0; i < 2; i++ {
			satisfied, err := ctx.EvaluateConstraint(counted, counted.OwnerScope)
			if err != nil {
				t.Fatalf("check %d: err = %v", i, err)
			}
			if !satisfied {
				t.Fatalf("check %d: not satisfied — the body carried state between evaluations", i)
			}
		}
	})

	t.Run("a loop spends the check's step budget", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			constraint def Spins {
				attribute i : Real = 0;
				while true { assign i := i + 1; }
				i > 0
			}
		}`
		ctx, idx := contextForSource(t, src)
		spins := lookupOne(t, idx, "test::Spins")
		_, err := ctx.EvaluateConstraint(spins, spins.OwnerScope)
		if !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("err = %v, want ErrStepLimitExceeded", err)
		}
	})

	t.Run("a nested require body runs its own steps", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			requirement safing {
				attribute margin : Real = 0;
				require constraint { attribute m : Real = 0; assign m := margin + 9; m > 4 }
			}
		}`
		ctx, idx := contextForSource(t, src)
		safing := lookupOne(t, idx, "test::safing")
		satisfied, err := ctx.EvaluateRequirement(safing, safing.OwnerScope)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if !satisfied {
			t.Error("the nested body's steps made its condition hold, and it is not satisfied")
		}
	})

	t.Run("a body stating steps but no result is refused", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			constraint def Silent {
				attribute y : Real = 1;
				assign y := 2;
			}
		}`
		ctx, idx := contextForSource(t, src)
		silent := lookupOne(t, idx, "test::Silent")
		_, err := ctx.EvaluateConstraint(silent, silent.OwnerScope)
		if !errors.Is(err, ErrNoConditions) {
			t.Fatalf("err = %v, want ErrNoConditions", err)
		}
		if !strings.Contains(err.Error(), "no result expression") {
			t.Errorf("err = %v, want it to say the body states no result expression", err)
		}
	})
}
