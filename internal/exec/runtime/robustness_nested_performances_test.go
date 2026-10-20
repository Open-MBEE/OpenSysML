package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessNestedPerformances reads the performance objects a part's
// nested action and state usages hold before anything performs them: the reads
// return typed answers, start nothing, and a usage of open multiplicity holds no
// made-up object.
func TestRuntimeRobustnessNestedPerformances(t *testing.T) {
	src := `package test {
		action def Act { action a1; action a2; }
		part def P {
			action a1;
			state s1;
			action opt [0..1];
			action acts : Act [2];
		}
		part p : P;
	}`

	t.Run("unperformed_usage_reads_without_starting", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::P"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		for _, name := range []string{"a1", "s1"} {
			val, err := evalOn(t, ctx, inst, "self."+name)
			if err != nil || val.Kind != ValInstance {
				t.Fatalf("self.%s = %s, %v; want an object", name, FormatValue(val), err)
			}
			if started := ctx.instances[val.Instance].behaviors; len(started) != 0 {
				t.Errorf("self.%s runs %d behaviors before anything performs it", name, len(started))
			}
		}
		if started := inst.behaviors; len(started) != 0 {
			t.Errorf("the owner runs %d behaviors after reading its unperformed usages, want 0", len(started))
		}
		for expr, want := range map[string]string{
			"self.ownedActions->SequenceFunctions::size()":             "4",
			"self.ownedStates->SequenceFunctions::size()":              "1",
			"self.acts#(2).subperformances->SequenceFunctions::size()": "2",
			"self.a1.this == self":                                     "true",
		} {
			val, err := evalOn(t, ctx, inst, expr)
			if err != nil {
				t.Fatalf("%s: %v", expr, err)
			}
			if got := FormatValue(val); got != want {
				t.Errorf("%s = %s, want %s", expr, got, want)
			}
		}
	})

	t.Run("open_multiplicity_usage_holds_no_object", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::P"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		val, err := evalOn(t, ctx, inst, "self.opt")
		if err != nil {
			t.Fatalf("self.opt: %v", err)
		}
		if got := FormatValue(val); got != "[]" {
			t.Errorf("self.opt = %s, want [] for a [0..1] usage nothing values", got)
		}
	})

	t.Run("missing_member_of_nested_performance_is_typed", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::P"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		if _, err := evalOn(t, ctx, inst, "self.a1.ownedActions"); !errors.Is(err, ErrNoSuchFeature) {
			t.Errorf("self.a1.ownedActions: %v, want ErrNoSuchFeature", err)
		}
	})
}
