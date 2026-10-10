package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessUseCase: a use case runs as the action it is (SysML v2
// §7.19), so its failure modes are an action's, reported as typed errors.
func TestRuntimeRobustnessUseCase(t *testing.T) {
	t.Run("no_start_node", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
			part def Drone;
			use case loop {
				subject drone : Drone;
				attribute total = 0;
				action a { assign total := total + 1; }
				action b { assign total := total + 1; }
				succession first a then b;
				succession first b then a;
			}
		}`))
		_, err := ctx.CreateActionExecutor(oneSymbol(t, idx, "test::loop"))
		if !errors.Is(err, ErrInvalidActionFlow) {
			t.Fatalf("expected ErrInvalidActionFlow at initialization, got: %v", err)
		}
		if !strings.Contains(err.Error(), "no initial node found in action loop") {
			t.Errorf("error %q does not say what the flow lacks", err)
		}
	})

	t.Run("unbound_subject", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
			part def Drone { attribute mass = 3.5; }
			use case weigh {
				subject drone : Drone;
				out m;
				first start;
				then action read { assign m := drone.mass; }
				then done;
			}
		}`))
		_, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::weigh"))
		if !errors.Is(err, ErrNoValue) {
			t.Fatalf("expected ErrNoValue, got: %v", err)
		}
		var noValue *NoValueError
		if !errors.As(err, &noValue) || noValue.Feature != "drone" {
			t.Errorf("error %q does not name the unbound subject", err)
		}
	})

	t.Run("include_of_undefined_use_case", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
			part def Drone;
			use case fly {
				subject drone : Drone;
				first start;
				then include use case charge : Nope;
				then done;
			}
		}`))
		_, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::fly"))
		if err == nil {
			t.Fatal("expected an error for an include of an undefined use case")
		}
		if !errors.Is(err, ErrUnresolvedReference) {
			t.Errorf("expected ErrUnresolvedReference, got: %v", err)
		}
	})
}
