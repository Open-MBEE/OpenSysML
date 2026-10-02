package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

type acceptSuspensionResult[T any] struct {
	value T
	err   error
}

func acceptSuspensionWatchdog[T any](t *testing.T, name string, run func() (T, error)) (T, error) {
	t.Helper()
	done := make(chan acceptSuspensionResult[T], 1)
	go func() {
		value, err := run()
		done <- acceptSuspensionResult[T]{value: value, err: err}
	}()
	select {
	case result := <-done:
		return result.value, result.err
	case <-watchdog(10 * time.Second):
		var zero T
		t.Fatalf("%s did not terminate", name)
		return zero, nil
	}
}

func acceptSuspensionExecutor(t *testing.T, source, name string) (*Context, *ActionExecutor) {
	t.Helper()
	const path = "<accept-suspension>"
	idx, _, ctx := buildRuntimeWithLibraries(t, path, parseAndBuild(t, source))
	sym := findSymbolByName(idx.DocumentRoot(path), name, ast.DefAction)
	if sym == nil {
		t.Fatalf("action %s not found", name)
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor(%s): %v", name, err)
	}
	return ctx, exec
}

func acceptSuspensionChainSource(depth int, mainBody string) string {
	var definitions strings.Builder
	definitions.WriteString(`package test {
		private import ScalarValues::*;
		private import SI::*;
		action def Reader {
			out total : Integer[1] = 0;
			first start;
			then action r accept n : Integer;
			then action keep { assign total := r.n; }
			then done;
		}
	`)
	callee := "Reader"
	for level := 1; level < depth; level++ {
		name := fmt.Sprintf("Mid%d", level)
		fmt.Fprintf(&definitions, `
			action def %s {
				out total : Integer[1] = 0;
				first start;
				then perform action caller : %s;
				then action keep { assign total := caller.total; }
				then done;
			}
		`, name, callee)
		callee = name
	}
	mainBody = strings.ReplaceAll(mainBody, "$CALL", callee)
	fmt.Fprintf(&definitions, `
		action def Main {
			out total : Integer[1] = 0;
			%s
		}
	}`, mainBody)
	return definitions.String()
}

func acceptSuspensionCallBody() string {
	return `first start;
		then perform action caller : $CALL;
		then action keep { assign total := caller.total; }
		then done;`
}

func acceptSuspensionClockSource(depth int) string {
	source := acceptSuspensionChainSource(depth, acceptSuspensionCallBody())
	source = strings.Replace(source, "action r accept n : Integer;", "action r accept after 5 [s];", 1)
	return strings.Replace(source, "assign total := r.n;", "assign total := 1;", 1)
}

func acceptSuspensionForkBody(choice bool) string {
	body := `first start;
		fork split;
		action caller {
			out attribute total : Integer = 0;
			perform action nested : $CALL;
		}
		action gate1;
		action gate2;
		action gate3;
		action sender { send 7; }
		action output { assign total := caller.total; }
		join sync;
		done;
		succession first start then split;
		succession first split then caller;
		succession first split then gate1;
		succession first gate1 then gate2;
		succession first gate2 then gate3;
		succession first gate3 then sender;
		succession first caller then output;
		succession first output then sync;
		succession first sender then sync;
		succession first sync then done;`
	if !choice {
		return body
	}
	body = strings.Replace(body, "\t\tjoin sync;\n\t\tdone;", `		join sync;
		decide choose;
		action left { assign pick := 1; }
		action right { assign pick := 2; }
		merge chosen;
		done;`, 1)
	body = strings.Replace(body, "\t\tsuccession first sync then done;", `		succession first sync then choose;
		succession first choose if pick == 0 then left;
		succession first choose if pick == 0 then right;
		succession first left then chosen;
		succession first right then chosen;
		succession first chosen then done;`, 1)
	return "attribute pick : Integer = 0;\n\t\t" + body
}

func TestRuntimeRobustnessAcceptSuspension(t *testing.T) {
	t.Run("deadlocks_name_nested_accept_and_call_chain", func(t *testing.T) {
		for depth := 1; depth <= 3; depth++ {
			t.Run(fmt.Sprintf("depth_%d", depth), func(t *testing.T) {
				_, exec := acceptSuspensionExecutor(t, acceptSuspensionChainSource(depth, acceptSuspensionCallBody()), "Main")
				_, err := acceptSuspensionWatchdog(t, "RunToCompletion", func() (struct{}, error) {
					return struct{}{}, exec.RunToCompletion()
				})
				if !errors.Is(err, ErrAcceptDeadlock) {
					t.Fatalf("error = %v, want ErrAcceptDeadlock", err)
				}
				if depth == 3 {
					const want = "invoke action Mid2: execute action: accept deadlock in action Mid2: nothing can post the awaited message " +
						"(accept n waiting since step 2 for a message of type Integer (in Reader, performed by caller) " +
						"(in Mid1, performed by caller))"
					if err.Error() != want {
						t.Errorf("depth-3 deadlock = %q, want %q", err, want)
					}
				}
				for _, want := range []string{"accept n", "in Reader", "performed by caller"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("deadlock %q does not name %q", err, want)
					}
				}
				for level := 1; level < depth; level++ {
					if want := fmt.Sprintf("Mid%d", level); !strings.Contains(err.Error(), want) {
						t.Errorf("depth-%d deadlock %q does not name %s", depth, err, want)
					}
				}
			})
		}
	})

	t.Run("unsent_loop_accept_deadlocks", func(t *testing.T) {
		_, exec := acceptSuspensionExecutor(t, `package test {
			private import ScalarValues::*;
			action Main {
				first start;
				action waiter {
					loop {
						accept n : Integer;
					} until false;
				}
				done;
				succession first start then waiter;
				succession first waiter then done;
			}
		}`, "Main")
		_, err := acceptSuspensionWatchdog(t, "RunToCompletion", func() (struct{}, error) {
			return struct{}{}, exec.RunToCompletion()
		})
		if !errors.Is(err, ErrAcceptDeadlock) || !strings.Contains(err.Error(), "accept n") {
			t.Fatalf("error = %v, want ErrAcceptDeadlock naming accept n", err)
		}
	})

	for _, tc := range []struct {
		name        string
		msg         Message
		laterAccept bool
	}{
		{name: "wrong_type", msg: Message{SignalType: "String"}},
		{name: "wrong_accept", msg: Message{SignalType: "Integer", Target: "later"}, laterAccept: true},
	} {
		t.Run(tc.name+"_does_not_wake_chain", func(t *testing.T) {
			source := acceptSuspensionChainSource(2, acceptSuspensionCallBody())
			if tc.laterAccept {
				source = strings.Replace(source,
					"then action keep { assign total := r.n; }\n\t\t\tthen done;",
					"then action keep { assign total := r.n; }\n\t\t\tthen action later accept m : Integer;\n\t\t\tthen done;", 1)
			}
			ctx, exec := acceptSuspensionExecutor(t, source, "Main")
			_, err := acceptSuspensionWatchdog(t, "RunToQuiescence", func() (struct{}, error) {
				return struct{}{}, exec.RunToQuiescence()
			})
			if err != nil {
				t.Fatalf("RunToQuiescence: %v", err)
			}
			if exec.State() != StateWaiting {
				t.Fatalf("State() = %v, want StateWaiting", exec.State())
			}
			ctx.PostMessage(tc.msg)
			_, err = acceptSuspensionWatchdog(t, "RunToCompletion after mismatched message", func() (struct{}, error) {
				return struct{}{}, exec.RunToCompletion()
			})
			if !errors.Is(err, ErrAcceptDeadlock) {
				t.Fatalf("error = %v, want ErrAcceptDeadlock", err)
			}
			if len(ctx.messages) != 1 {
				t.Errorf("message queue holds %d messages, want the unconsumed message", len(ctx.messages))
			}
		})
	}

	t.Run("sibling_wakes_nested_chain", func(t *testing.T) {
		source := acceptSuspensionChainSource(2, acceptSuspensionForkBody(false))
		_, exec := acceptSuspensionExecutor(t, source, "Main")
		_, err := acceptSuspensionWatchdog(t, "RunToCompletion", func() (struct{}, error) {
			return struct{}{}, exec.RunToCompletion()
		})
		if err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
		assertIntOutput(t, exec.Results(), "total", 7)
	})

	t.Run("clock_wakes_nested_chain_at_requested_instant", func(t *testing.T) {
		ctx, exec := acceptSuspensionExecutor(t, acceptSuspensionClockSource(2), "Main")
		_, err := acceptSuspensionWatchdog(t, "RunToCompletion", func() (struct{}, error) {
			return struct{}{}, exec.RunToCompletion()
		})
		if err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
		assertIntOutput(t, exec.Results(), "total", 1)
		if got := ctx.Clock().Now(); got != 5 {
			t.Errorf("clock = %v, want 5 seconds", got)
		}
	})

	t.Run("snapshot_restores_parked_chain_twice", func(t *testing.T) {
		ctx, exec := acceptSuspensionExecutor(t, acceptSuspensionChainSource(2, acceptSuspensionCallBody()), "Main")
		_, err := acceptSuspensionWatchdog(t, "RunToQuiescence", func() (struct{}, error) {
			return struct{}{}, exec.RunToQuiescence()
		})
		if err != nil {
			t.Fatalf("RunToQuiescence: %v", err)
		}
		if exec.State() != StateWaiting {
			t.Fatalf("State() = %v, want StateWaiting", exec.State())
		}
		inv := &Invocation{Actions: []*ActionExecutor{exec}}
		canonical := inv.canonicalState(nil).text
		snapshot, err := exec.Snapshot()
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		defer snapshot.Release()
		for pass := 1; pass <= 2; pass++ {
			if pass > 1 {
				snapshot.Restore()
			}
			if got := inv.canonicalState(nil).text; got != canonical {
				t.Fatalf("restore %d canonical state differs:\n%s\nwant:\n%s", pass, got, canonical)
			}
			value := integerValue(7)
			ctx.PostMessage(Message{SignalType: "Integer", Value: &value})
			_, err = acceptSuspensionWatchdog(t, "RunToCompletion after restore", func() (struct{}, error) {
				return struct{}{}, exec.RunToCompletion()
			})
			if err != nil {
				t.Fatalf("restore %d: RunToCompletion: %v", pass, err)
			}
			assertIntOutput(t, exec.Results(), "total", 7)
		}
		snapshot.Restore()
		if got := inv.canonicalState(nil).text; got != canonical {
			t.Fatalf("canonical state after final restore differs:\n%s\nwant:\n%s", got, canonical)
		}
	})

	t.Run("explore_and_check_are_deterministic_with_parked_chain", func(t *testing.T) {
		parkedKey := func() stateKey {
			_, exec := acceptSuspensionExecutor(t, acceptSuspensionChainSource(2, acceptSuspensionCallBody()), "Main")
			_, err := acceptSuspensionWatchdog(t, "RunToQuiescence", func() (struct{}, error) {
				return struct{}{}, exec.RunToQuiescence()
			})
			if err != nil {
				t.Fatalf("RunToQuiescence: %v", err)
			}
			return (&Invocation{Actions: []*ActionExecutor{exec}}).canonicalState(nil).key()
		}
		if first, second := parkedKey(), parkedKey(); first != second {
			t.Fatalf("parked-chain state keys differ: %s and %s", first, second)
		}
		model := parseLibraryModel(t, acceptSuspensionChainSource(2, acceptSuspensionForkBody(true)))
		action := model.action(t, "Main")
		policy := mustPolicy(t, "explore")
		explore := func() *Exploration {
			result, err := acceptSuspensionWatchdog(t, "Explore", func() (*Exploration, error) {
				return Explore(context.Background(), policy, model.fresh, func(ctx *Context) (Outcome, error) {
					outputs, err := ctx.ExecuteAction(action)
					if err != nil {
						return Outcome{}, err
					}
					return ctx.ActionOutcome(outputs), nil
				})
			})
			if err != nil {
				t.Fatalf("Explore: %v", err)
			}
			return result
		}
		first, second := explore(), explore()
		if !first.Complete() || first.Runs < 2 || first.Status() != second.Status() ||
			first.Runs != second.Runs || len(first.Outcomes) != len(second.Outcomes) {
			t.Fatalf("exploration results differ: %s %v; %s %v", first.Status(), outcomeTexts(first), second.Status(), outcomeTexts(second))
		}
		for i, outcome := range first.Outcomes {
			again := second.Outcomes[i]
			if outcome.Outcome.String() != again.Outcome.String() || outcome.Linearizations != again.Linearizations ||
				FormatChoices(outcome.Witness) != FormatChoices(again.Witness) || outcome.WitnessRun != again.WitnessRun {
				t.Errorf("exploration outcome %d differs: %+v; %+v", i, outcome, again)
			}
		}

		check := func() *CheckReport {
			report, err := acceptSuspensionWatchdog(t, "Check", func() (*CheckReport, error) {
				return Check(context.Background(), model.fresh, starterOf(action), CheckBudget{},
					CheckOptions{Reduce: true, Diverge: []string{"pick"}}, nil)
			})
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			return report
		}
		firstCheck, secondCheck := check(), check()
		if firstCheck.States < 2 || firstCheck.States != secondCheck.States || firstCheck.Moves != secondCheck.Moves ||
			firstCheck.Verdict != CheckDivergent || secondCheck.Verdict != CheckDivergent ||
			strings.Join(divergentValues(firstCheck, "pick"), "|") != "1|2" ||
			strings.Join(divergentValues(secondCheck, "pick"), "|") != "1|2" ||
			len(firstCheck.Finals) != len(secondCheck.Finals) {
			t.Fatalf("check results differ: %s with %d finals; %s with %d finals",
				firstCheck.Status(), len(firstCheck.Finals), secondCheck.Status(), len(secondCheck.Finals))
		}
		for i, final := range firstCheck.Finals {
			again := secondCheck.Finals[i]
			if final.identity != again.identity || FormatChoices(final.Witness.Choices) != FormatChoices(again.Witness.Choices) {
				t.Errorf("check final %d differs: %+v; %+v", i, final, again)
			}
		}
	})

	t.Run("breakpoint_inside_parked_callee_resumes", func(t *testing.T) {
		ctx, exec := acceptSuspensionExecutor(t, acceptSuspensionChainSource(2, acceptSuspensionCallBody()), "Main")
		exec.SetBreakpoint("keep")
		_, err := acceptSuspensionWatchdog(t, "RunToQuiescence", func() (struct{}, error) {
			return struct{}{}, exec.RunToQuiescence()
		})
		if err != nil {
			t.Fatalf("RunToQuiescence: %v", err)
		}
		value := integerValue(7)
		ctx.PostMessage(Message{SignalType: "Integer", Value: &value})
		_, err = acceptSuspensionWatchdog(t, "RunToCompletion to breakpoint", func() (struct{}, error) {
			return struct{}{}, exec.RunToCompletion()
		})
		if err != nil {
			t.Fatalf("RunToCompletion to breakpoint: %v", err)
		}
		if got := exec.PausedAt(); got != "keep" {
			t.Fatalf("PausedAt() = %q, want keep inside Reader", got)
		}
		if exec.State() != StateSuspended {
			t.Fatalf("State() = %v, want StateSuspended", exec.State())
		}
		_, err = acceptSuspensionWatchdog(t, "resume from callee breakpoint", func() (struct{}, error) {
			return struct{}{}, exec.RunToCompletion()
		})
		if err != nil {
			t.Fatalf("resume: %v", err)
		}
		assertIntOutput(t, exec.Results(), "total", 7)
	})
}
