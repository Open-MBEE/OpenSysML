package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestRuntimeRobustnessSteppedStateBehaviorClockWait(t *testing.T) {
	for _, behavior := range []string{"entry", "exit"} {
		t.Run(behavior, func(t *testing.T) {
			testSteppedStateBehaviorClockWait(t, behavior)
		})
	}
}

func testSteppedStateBehaviorClockWait(t *testing.T, behavior string) {
	src := fmt.Sprintf(`package test {
		private import SI::*;
		state Machine {
			entry; then active;
			state active {
				%s action {
					first start then nap;
					action nap accept after 1 [s];
				}
			}
			succession first active then done;
		}
	}`, behavior)
	model := parseLibraryModel(t, src)
	machine := model.state(t, "Machine")

	t.Run("explore", func(t *testing.T) {
		exploration, err := Explore(context.Background(), mustPolicy(t, "explore"), model.fresh,
			func(ctx *Context) (Outcome, error) {
				return ctx.StateOutcomeWithEvents(machine, nil)
			})
		if err != nil {
			t.Fatalf("Explore: %v", err)
		}
		if exploration == nil || !exploration.Complete() || len(exploration.Outcomes) != 1 {
			t.Fatalf("exploration = %v, want one complete run", exploration)
		}
		assertStateBehaviorWaitError(t, exploration.Outcomes[0].Outcome.Err)
	})

	t.Run("fixed", func(t *testing.T) {
		ctx, err := model.fresh()
		if err != nil {
			t.Fatalf("fresh context: %v", err)
		}
		if err := ctx.SetSchedule(mustPolicy(t, "declared")); err != nil {
			t.Fatalf("SetSchedule(declared): %v", err)
		}
		outcome, err := ctx.StateOutcomeWithEvents(machine, nil)
		waitErr := outcome.Err
		if waitErr == nil {
			waitErr = err
		}
		assertStateBehaviorWaitError(t, waitErr)
	})
}

func assertStateBehaviorWaitError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrStateBehaviorWaits) {
		t.Fatalf("state behavior wait error = %v, want ErrStateBehaviorWaits", err)
	}
	if errors.Is(err, errPaused) || strings.Contains(err.Error(), errPaused.Error()) {
		t.Fatalf("state behavior wait leaked %q: %v", errPaused, err)
	}
}
