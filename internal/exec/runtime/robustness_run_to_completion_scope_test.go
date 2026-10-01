package runtime

import (
	"errors"
	"os"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

func TestRuntimeRobustnessRunToCompletionScope(t *testing.T) {
	t.Run("scope_not_ancestor", func(t *testing.T) {
		err := stateExecutorError(t, readRunToCompletionFixture(t, "state_run_to_completion_scope_not_ancestor.sysml"), "Machine")
		var refusal *lower.RunToCompletionRedefinition
		if !errors.As(err, &refusal) {
			t.Fatalf("error = %v, want run-to-completion redefinition", err)
		}
		if !errors.Is(err, lower.ErrUnsupportedStateContent) {
			t.Fatalf("error = %v, want unsupported state content", err)
		}
		if !refusal.NotAncestor {
			t.Fatalf("refusal = %+v, want non-ancestor scope", refusal)
		}
	})

	t.Run("scope_no_occurrence", func(t *testing.T) {
		err := stateExecutorError(t, readRunToCompletionFixture(t, "state_run_to_completion_scope_no_occurrence.sysml"), "Machine")
		var refusal *lower.RunToCompletionRedefinition
		if !errors.As(err, &refusal) {
			t.Fatalf("error = %v, want run-to-completion redefinition", err)
		}
		if !errors.Is(err, lower.ErrUnsupportedStateContent) {
			t.Fatalf("error = %v, want unsupported state content", err)
		}
		if refusal.NotAncestor {
			t.Fatalf("refusal = %+v, want unresolved scope", refusal)
		}
	})

	t.Run("value_not_boolean", func(t *testing.T) {
		err := stateExecutorError(t, `
			package test {
				state def Machine {
					entry; then idle;
					state idle parallel {
						attribute :>> isRunToCompletion = 1;
						state inner {
							entry; then leaf;
							state leaf;
						}
					}
				}
			}
		`, "Machine")
		var valueErr *RunToCompletionValueError
		if !errors.As(err, &valueErr) {
			t.Fatalf("error = %v, want run-to-completion value error", err)
		}
	})
}

func readRunToCompletionFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/robustness/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
