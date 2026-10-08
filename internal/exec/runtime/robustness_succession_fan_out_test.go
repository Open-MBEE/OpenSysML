package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessSuccessionFanOut exercises a token leaving an ordinary
// node along several successions where the flow beyond cannot finish: a pruned
// branch into a join deadlocks, a guard that cannot be evaluated fails the step,
// a fan-out in a loop spends the budget, and a merge with two outgoing
// successions is refused.
func TestRuntimeRobustnessSuccessionFanOut(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		steps int64
		want  error
	}{
		{
			name: "pruned_branch_into_a_join_deadlocks",
			body: `
				attribute armed : Integer = 0;
				first start;
				then action a;
				action b; action c;
				join sync;
				done;
				succession first a if armed > 0 then b;
				succession first a then c;
				succession first b then sync;
				succession first c then sync;
				succession first sync then done;
			`,
			want: ErrActionDeadlock,
		},
		{
			name: "guard_on_one_link_cannot_be_evaluated",
			body: `
				first start;
				then action a;
				action b; action c;
				succession first a if missing > 0 then b;
				succession first a then c;
			`,
			want: ErrUnresolvedReference,
		},
		{
			name: "fan_out_in_a_loop_spends_the_budget",
			body: `
				attribute n : Integer = 0;
				first start;
				merge m;
				action a;
				action b { assign n := n + 1; }
				succession first start then m;
				succession first m then a;
				succession first a then m;
				succession first a then b;
			`,
			steps: 200,
			want:  ErrActionStepLimitExceeded,
		},
		{
			name: "self_loop_fan_out_spends_the_budget",
			body: `
				attribute n : Integer = 0;
				first start;
				then action a;
				action b { assign n := n + 1; }
				succession first a then a;
				succession first a then b;
			`,
			steps: 200,
			want:  ErrActionStepLimitExceeded,
		},
		{
			name: "merge_with_two_outgoing_successions_is_refused",
			body: `
				first start;
				merge m;
				action b; action c;
				succession first start then m;
				succession first m then b;
				succession first m then c;
			`,
			want: ErrInvalidActionFlow,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test {\n private import ScalarValues::*;\n action fan {" + tc.body + "}\n}"
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			if tc.steps > 0 {
				ctx.maxActionSteps = tc.steps
			}
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "fan", ast.DefAction)
			if sym == nil {
				t.Fatal("action fan not found")
			}
			if _, err := ctx.ExecuteAction(sym); !errors.Is(err, tc.want) {
				t.Fatalf("ExecuteAction err = %v, want %v", err, tc.want)
			}
		})
	}
}
