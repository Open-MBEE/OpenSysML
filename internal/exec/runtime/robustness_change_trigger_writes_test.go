package runtime

import (
	"errors"
	"testing"
)

func TestRuntimeRobustnessChangeTriggerWrites(t *testing.T) {
	t.Run("write-time condition error is deferred to poll", func(t *testing.T) {
		exec := stateExecutorForSource(t, "Machine", `package test {
			state Machine {
				attribute divisor : Integer = 1;
				entry; then changing;
				state changing {
					do action poison {
						assign divisor := 0;
					}
				}
				accept when divisor == 0 and 1 / divisor > 0 then done;
				state done;
			}
		}`)
		if err := exec.RunToCompletion(); !errors.Is(err, ErrDivisionByZero) {
			t.Fatalf("RunToCompletion() = %v, want ErrDivisionByZero", err)
		}
	})

	t.Run("many writes remain step bounded", func(t *testing.T) {
		exec := stateExecutorForSource(t, "Machine", `package test {
			state Machine {
				attribute ready : Boolean = false;
				attribute armed : Boolean = false;
				entry; then start;
				state start;
				state spinning {
					do action spin {
						while true {
							assign ready := not ready;
						}
					}
				}
				accept when ready and armed then done;
				state done;
				succession first start then spinning;
			}
		}`)
		exec.ctx.maxSteps = 40
		if err := exec.RunToCompletion(); !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("RunToCompletion() = %v, want ErrStepLimitExceeded", err)
		}
	})

	t.Run("snapshot restores a pending rise", func(t *testing.T) {
		ctx, holder, exec := changeTriggerWriteHolder(t)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("initial run: %v", err)
		}
		if err := holder.SetFeatureValue(ctx, "ready", boolValue(true)); err != nil {
			t.Fatalf("raise ready: %v", err)
		}
		if err := holder.SetFeatureValue(ctx, "ready", boolValue(false)); err != nil {
			t.Fatalf("lower ready: %v", err)
		}
		snapshot, err := exec.Snapshot()
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		defer snapshot.Release()

		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run with pending rise: %v", err)
		}
		assertCurrentState(t, exec, "done")
		snapshot.Restore()
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run after restore: %v", err)
		}
		assertCurrentState(t, exec, "done")
	})

	t.Run("check state distinguishes a pending rise", func(t *testing.T) {
		ctx, holder, exec := changeTriggerWriteHolder(t)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("initial run: %v", err)
		}
		before := (&Invocation{States: []*StateExecutor{exec}}).canonicalState(nil).text
		if err := holder.SetFeatureValue(ctx, "ready", boolValue(true)); err != nil {
			t.Fatalf("raise ready: %v", err)
		}
		if err := holder.SetFeatureValue(ctx, "ready", boolValue(false)); err != nil {
			t.Fatalf("lower ready: %v", err)
		}
		after := (&Invocation{States: []*StateExecutor{exec}}).canonicalState(nil).text
		if before == after {
			t.Fatalf("canonical state did not change after the rise:\n%s", before)
		}
	})
}

func changeTriggerWriteHolder(t *testing.T) (*Context, *Instance, *StateExecutor) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		part def Holder {
			attribute ready : Boolean = false;
			exhibit state main {
				entry; then waiting;
				state waiting;
				accept when ready then done;
				state done;
			}
		}
		part holder : Holder;
	`))
	holder, err := ctx.occurrenceOf(resolveSymbol(t, idx.DocumentRoot("<test>"), "holder"))
	if err != nil {
		t.Fatalf("occurrenceOf(holder): %v", err)
	}
	return ctx, holder, holder.ExhibitedStates()[0].State
}
