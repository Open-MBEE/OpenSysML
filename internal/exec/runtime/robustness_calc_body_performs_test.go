package runtime

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessCalcBodyPerforms: a calculation is an action (SysML v2
// §7.19.1) whose body admits every action body item (CalculationBodyItem), so the
// steps it states are performed, and fail, as an action's are.
func TestRuntimeRobustnessCalcBodyPerforms(t *testing.T) {
	cases := []struct {
		name, body, want string
		err              error
	}{
		{name: "nested_action_assigns_body_value",
			body: `in n : Integer = 3; attribute x : Integer := n; action a { assign x := x * 2; } return : Integer = x;`,
			want: "(6)"},
		{name: "flow_between_nested_actions",
			body: `in n : Integer = 3; attribute x : Integer := n; action a { out o : Integer = n + 1; } action b { in p : Integer; assign x := x + p; } flow a.o to b.p; return : Integer = x;`,
			want: "(7)"},
		{name: "perform_output_read_by_result",
			body: `in n : Integer = 3; perform action i : Inner { in a = n; } return : Integer = i.r;`,
			want: "(30)"},
		{name: "perform_of_declared_action",
			body: `in n : Integer = 3; action i : Inner { in a = n; out r; } perform i; return : Integer = i.r;`,
			want: "(30)"},
		{name: "perform_output_read_by_later_step",
			body: `in n : Integer = 3; attribute x : Integer := 0; perform action i : Inner { in a = n; out r; } assign x := i.r + 1; return : Integer = x;`,
			want: "(31)"},
		{name: "nested_calc_beside_action_step",
			body: `in n : Integer = 3; attribute x : Integer := 0; calc sq { in a : Integer = n; return : Integer = a * a; } action k { assign x := 1; } return : Integer = sq + x;`,
			want: "(10)"},
		{name: "nested_case_returning_on_a_path",
			body: `analysis tally { attribute got : Integer := 0; first start; then action a { assign got := 5; } if got == 5 { return : Integer = 7; } } return : Integer = tally;`,
			want: "(7)"},
		{name: "return_in_nested_action_ends_the_calc",
			body: `in n : Integer = 3; attribute x : Integer := n; action check { if n > 1 { return : Integer = 42; } } then action late { assign x := 0; } return : Integer = x;`,
			want: "(42)"},
		{name: "nested_action_reads_this",
			body: `in n : Integer = 3; attribute x : Integer := 0; action a { attribute me = this; assign x := 1; } return : Integer = x;`,
			want: "(1)"},
		{name: "terminate_skips_a_conditional_result",
			body: `in n : Integer = 3; attribute x : Integer := 0; if n > 1 { terminate; } if n > 1 { assign x := 50; return : Integer = x; } return : Integer = x;`,
			want: "(0)"},
		{name: "accept_takes_the_message_sent",
			body: `in n : Integer = 3; send new Ping(7) to self; accept p : Ping; return : Integer = n;`,
			want: "(3)"},
		{name: "accept_in_nested_action",
			body: `in n : Integer = 3; attribute x : Integer := n; action tx send new Ping(7) to self; action rx accept p : Ping; then action k { assign x := x + rx.p.v; } return : Integer = x;`,
			want: "(10)"},
		{name: "accept_nothing_posts_deadlocks",
			body: `in n : Integer = 3; accept p : Ping; return : Integer = n;`,
			err:  ErrAcceptDeadlock},
		{name: "accept_of_another_type_deadlocks",
			body: `in n : Integer = 3; send new Pong() to self; accept p : Ping; return : Integer = n;`,
			err:  ErrAcceptDeadlock},
		{name: "terminate_ends_the_steps",
			body: `in n : Integer = 3; attribute x : Integer := n; if n > 2 { terminate; } assign x := 7; return : Integer = x;`,
			want: "(3)"},
		{name: "terminate_in_loop_ends_the_steps",
			body: `in n : Integer = 3; attribute x : Integer := 0; while x < 10 { assign x := x + 1; if x == n { terminate; } } return : Integer = x;`,
			want: "(3)"},
		{name: "terminate_in_step_ends_the_step",
			body: `in n : Integer = 3; attribute x : Integer := n; action a { assign x := 9; then terminate; then assign x := 11; } return : Integer = x;`,
			want: "(9)"},
		{name: "terminate_between_steps_ends_the_flow",
			body: `in n : Integer = 3; attribute x : Integer := n; action a { assign x := 9; } then terminate; then action b { assign x := 11; } return : Integer = x;`,
			want: "(9)"},
		{name: "terminate_of_waiting_sibling",
			body: `in n : Integer = 3; attribute x : Integer := n; action w accept p : Ping; action k { terminate w; then assign x := 5; } return : Integer = x;`,
			want: "(5)"},
		{name: "terminate_keeps_initial_output",
			body: `in n : Integer = 3; out y : Integer := 1; terminate; assign y := 2;`,
			want: "1"},
		{name: "terminate_of_unresolved_name",
			body: `in n : Integer = 3; action a { terminate nothere; } return : Integer = n;`,
			err:  errTerminateTarget},
		{name: "terminate_of_ended_step",
			body: `in n : Integer = 3; action a { } then action b { terminate a; } return : Integer = n;`,
			err:  errPerformanceEnded},
		{name: "send_to_unaddressable_target",
			body: `in n : Integer = 3; send new Ping(1) to Host::self; return : Integer = n;`,
			err:  errUnroutableSend},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := calcBodyOnHost(t, tc.body)
			switch {
			case tc.err != nil:
				if !errors.Is(err, tc.err) {
					t.Fatalf("error = %v, want %v", err, tc.err)
				}
			case err != nil:
				t.Fatal(err)
			case got != tc.want:
				t.Fatalf("result = %s, want %s", got, tc.want)
			}
		})
	}
}

// calcBodyOnHost invokes a calc def with the given body on a part owning it.
func calcBodyOnHost(t *testing.T, body string) (string, error) {
	t.Helper()
	src := fmt.Sprintf(`package test {
	private import ScalarValues::*;
	item def Ping { attribute v : Integer; }
	item def Pong;
	action def Inner { in a : Integer; out r : Integer = a * 10; }
	part def Host {
		calc def C { %s }
	}
}`, body)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	root := idx.DocumentRoot("<test>")
	host := mustInstantiate(t, ctx, findSymbolByName(root, "Host", ast.DefPart))
	value, err := ctx.invokeCalcWithSelf(oneSymbol(t, idx, "test::Host::C"), calcArgs{}, root, host)
	if err != nil {
		return "", err
	}
	return FormatTraceValue(value), nil
}
