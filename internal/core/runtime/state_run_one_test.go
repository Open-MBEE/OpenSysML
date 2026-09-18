package runtime

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// runOneMachine creates a machine whose two regions each run a do body that waits
// a second midway and leave by timers tying at t=2, every step appending a digit to log.
func runOneMachine(t *testing.T) (*Context, *StateExecutor) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import SI::*;
			private import ScalarValues::*;
			state pair {
				attribute log : Integer = 0;
				entry; then work;
				state work parallel {
					state ra {
						entry; then a1;
						state a1 {
							do action body {
								action s1 assign log := log * 10 + 1;
								then action w accept after 1 [s];
								then action s2 assign log := log * 10 + 2;
							}
						}
						state a2;
						transition a1 then a2 accept after 2 [s] do assign log := log * 10 + 3;
					}
					state rb {
						entry; then b1;
						state b1 {
							do action body {
								action s1 assign log := log * 10 + 4;
								then action w accept after 1 [s];
								then action s2 assign log := log * 10 + 5;
							}
						}
						state b2;
						transition b1 then b2 accept after 2 [s] do assign log := log * 10 + 6;
					}
				}
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pair", ast.DefState)
	if sym == nil {
		t.Fatal("state pair not found")
	}
	ctx.SetSchedule(mustPolicy(t, "declared"))
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	return ctx, exec
}

func describeChoices(choices []ChoicePoint) []string {
	out := make([]string, len(choices))
	for i, c := range choices {
		out[i] = c.Describe()
	}
	return out
}

// Driving a machine one atomic unit at a time — a do action of the round, a
// dispatch — reaches the outcome a run to quiescence reaches, through the same
// choices, in as many units as the run counts do steps and dispatches.
func TestRunOneMatchesRunToQuiescence(t *testing.T) {
	ctx, exec := runOneMachine(t)
	report, err := ctx.Advance(3)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	wantLog, wantChoices, wantOutcome := exec.StateData()["log"], describeChoices(ctx.Choices()), exec.Outcome().String()

	ctx, exec = runOneMachine(t)
	var progress dueProgress
	units := 0
	// Note into the clock's run, as Advance does, so the two lists start alike.
	leave := ctx.beginExecutorRun(&ctx.clockRun)
	for {
		moved, err := exec.runOne(&progress)
		if err != nil {
			t.Fatalf("unit %d: %v", units, err)
		}
		if moved {
			units++
			continue
		}
		if exec.state != StateSuspended {
			t.Fatalf("unit %d: machine %s after a unit that got nowhere, want suspended", units, exec.state)
		}
		if next, ok := ctx.clock.NextDue(); !ok || next > 3 || !ctx.advanceToNextDue(&progress) {
			break
		}
	}
	leave()
	ctx.clock.now = 3

	if got := exec.StateData()["log"]; got.Const.Int != wantLog.Const.Int {
		t.Errorf("log = %d one unit at a time, %d to quiescence", got.Const.Int, wantLog.Const.Int)
	}
	if got := exec.Outcome().String(); got != wantOutcome {
		t.Errorf("outcome = %q one unit at a time, %q to quiescence", got, wantOutcome)
	}
	if got := describeChoices(ctx.Choices()); !slices.Equal(got, wantChoices) {
		t.Errorf("choices = %q one unit at a time, %q to quiescence", got, wantChoices)
	}
	if want := int(report.Events + report.DoSteps); units != want {
		t.Errorf("%d units, want %d (%d dispatches and %d do steps)", units, want, report.Events, report.DoSteps)
	}
	if progress.events != report.Events || progress.doSteps != report.DoSteps {
		t.Errorf("counted %d events and %d do steps, the run %d and %d", progress.events, progress.doSteps, report.Events, report.DoSteps)
	}
}
