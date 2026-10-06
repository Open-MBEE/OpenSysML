package runtime

import (
	"strings"
	"testing"
)

// TestRuntimeRobustnessMigratedVariables exercises the forms the migrator
// writes for variable actions on edge inputs: sequence writes at an index the
// variable does not hold, and a read of a variable nothing assigned. Each must
// end in a typed error or a defined result, never a panic.
func TestRuntimeRobustnessMigratedVariables(t *testing.T) {
	t.Run("excludingAt_out_of_range", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
			action def Run {
				private attribute v : Integer[0..*] = (1, 2);
				first start;
				action removeAt {
					in removeAt : Integer = 9;
					assign v := SequenceFunctions::excludingAt(v, removeAt);
				}
				then done;
			}
		}`))
		if _, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Run")); err == nil {
			t.Error("excludingAt past the end ran without error")
		}
	})

	t.Run("includingAt_out_of_range", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
			action def Run {
				private attribute v : Integer[0..*] = (1, 2);
				first start;
				action addAt {
					in value : Integer = 7;
					in insertAt : Integer = 9;
					assign v := SequenceFunctions::includingAt(v, value, insertAt);
				}
				then done;
			}
		}`))
		if _, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Run")); err == nil {
			t.Error("includingAt past the end ran without error")
		}
	})

	t.Run("read_never_assigned_variable", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
			action def Run {
				private attribute v : Integer;
				out result : Integer[1];
				first start;
				action read {
					out r : Integer[0..1] = v;
				}
				then action assign { assign result := read.r; }
				then done;
			}
		}`))
		outputs, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Run"))
		if err != nil {
			if !strings.Contains(err.Error(), "no value") && !strings.Contains(err.Error(), "uninitialized") && !strings.Contains(err.Error(), "empty") {
				t.Fatalf("ExecuteAction: %v, want a typed error", err)
			}
			return
		}
		if _, ok := outputs["result"]; !ok {
			t.Errorf("outputs = %v, want result defined", outputs)
		}
	})
}
