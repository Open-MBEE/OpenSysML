package runtime

import (
	"strings"
	"testing"
)

// TestRuntimeRobustnessMigratedLoop exercises the loop the migrator writes for
// a LoopNode on edge inputs: a decider that never goes false must spend the
// action's step budget and stop with a typed error, not hang; a loop variable
// whose input pin receives no value must fail its read with a typed error.
func TestRuntimeRobustnessMigratedLoop(t *testing.T) {
	t.Run("decider_never_false_hits_step_budget", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
			action def Run {
				first start then 'loop';
				action 'loop' {
					out r : Integer;
					private attribute i : Integer := 0;
					private attribute ended : Boolean := false;
					first start then iterate;
					then action iterate {
						loop {
							action test {
								first start then le;
								action le {
									in x : Integer = i;
									out r : Boolean;
									assign r := true;
								}
							}
							then if not test.le.r {
								assign ended := true;
							} else {
								action body {
									first start then inc;
									action inc {
										in x : Integer = i;
										out r : Integer;
										assign r := x + 1;
									}
								}
								then action next {
									assign i := body.inc.r;
								}
							}
						} until ended;
					}
					then action results {
						assign r := i;
					}
				}
			}
		}`))
		if _, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Run")); err == nil || !strings.Contains(err.Error(), "step") {
			t.Fatalf("ExecuteAction: %v, want a typed step-budget error", err)
		}
	})

	t.Run("loop_variable_input_never_fed", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
			action def Run {
				first start then 'loop';
				action 'loop' {
					in iIn : Integer[1];
					out r : Integer;
					private attribute i : Integer := iIn;
					private attribute ended : Boolean := false;
					first start then iterate;
					then action iterate {
						loop {
							action test {
								first start then le;
								action le {
									in x : Integer = i;
									out r : Boolean;
									assign r := x >= 0;
								}
							}
							then if not test.le.r {
								assign ended := true;
							} else {
								assign ended := true;
							}
						} until ended;
					}
					then action results {
						assign r := i;
					}
				}
			}
		}`))
		if _, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Run")); err == nil {
			t.Error("a loop variable reading a pin that holds no value ran without error")
		}
	})
}
