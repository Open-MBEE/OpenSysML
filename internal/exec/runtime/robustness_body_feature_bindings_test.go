package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestRuntimeRobustnessBodyFeatureBindings(t *testing.T) {
	t.Run("cycle", func(t *testing.T) {
		ctx, action := loadAction(t, `package test {
			private import ScalarValues::*;
			action run {
				attribute left : Integer = right + 1;
				attribute right : Integer = left + 1;
				first start;
				then action read { assign left := right; }
			}
		}`, "run")
		exec, err := ctx.CreateActionExecutor(action)
		if err == nil {
			err = exec.RunToCompletion()
		}
		if !errors.Is(err, ErrCyclicFeatureValue) {
			t.Fatalf("RunToCompletion = %v, want ErrCyclicFeatureValue", err)
		}
	})

	t.Run("destroyed_object_read", func(t *testing.T) {
		ctx, action := loadAction(t, `package test {
			private import ScalarValues::*;
			part def Target { attribute n : Integer := 2; }
			action run {
				in ref target : Target;
				attribute live : Integer = target.n;
				first start;
				then action read { assign live := live + 1; }
			}
		}`, "run")
		idx := ctx.model.resolver.Index()
		target, err := ctx.Instantiate(oneSymbol(t, idx, "test::Target"))
		if err != nil {
			t.Fatalf("Instantiate Target: %v", err)
		}
		exec, err := ctx.CreateActionExecutorWithInputs(action, nil, map[string]Value{"target": objectValue(target)})
		if err != nil {
			t.Fatalf("CreateActionExecutorWithInputs: %v", err)
		}
		if err := ctx.destroy(target); err != nil {
			t.Fatalf("destroy Target: %v", err)
		}
		if _, _, err := ctx.readBodyValue(exec.root.cells, exec.root.data, "live"); !errors.Is(err, ErrOccurrenceDestroyed) {
			t.Fatalf("read live after destroying target = %v, want ErrOccurrenceDestroyed", err)
		}
	})

	t.Run("state_exit_invalidates_dependents", func(t *testing.T) {
		ctx, machine := loadState(t, `package test {
			private import ScalarValues::*;
			state machine {
				attribute source : Integer := 1;
				attribute top : Integer = active.value;
				entry; then active;
				state active { attribute value : Integer = source; }
				state done;
				transition first active accept when source > 0 do assign source := 2 then done;
			}
		}`, "machine")
		exec, err := ctx.CreateStateExecutor(machine)
		if err != nil {
			t.Fatalf("CreateStateExecutor: %v", err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
		data, err := exec.StateDataWithError()
		if err != nil {
			if !errors.Is(err, ErrNoValue) && !errors.Is(err, ErrUninitializedFeatureValue) && !errors.Is(err, ErrUnresolvedReference) {
				t.Fatalf("read top after state exit = %v, want a typed inactive-state value error", err)
			}
			return
		}
		if got := FormatValue(data["top"]); got != "<undetermined>" {
			t.Fatalf("top after state exit = %s, want <undetermined>", got)
		}
	})

	t.Run("write_stops_tracking", func(t *testing.T) {
		ctx, action := loadAction(t, `package test {
			private import ScalarValues::*;
			action run {
				attribute source : Integer := 1;
				attribute live : Integer = source * 2;
				out attribute result : Integer;
				first start;
				then action write {
					assign live := 99;
					assign source := 5;
					assign result := live;
				}
			}
		}`, "run")
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
		if got := FormatValue(exec.Results()["result"]); got != "99" {
			t.Fatalf("result = %s, want 99 after the binding is written", got)
		}
	})

	t.Run("read_only_body_features", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			decl string
			want string
		}{
			{"constant", "constant attribute locked : Integer = 1;", "locked is constant"},
			{"derived", "attribute source : Integer := 1; derived attribute locked : Integer = source * 2;", "locked is derived"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				ctx, action := loadAction(t, `package test {
					private import ScalarValues::*;
					action run {
						`+tc.decl+`
						first start;
						then action write { assign locked := 2; }
					}
				}`, "run")
				exec, err := ctx.CreateActionExecutor(action)
				if err != nil {
					t.Fatalf("CreateActionExecutor: %v", err)
				}
				if err := exec.RunToCompletion(); !errors.Is(err, ErrReadOnlyFeature) {
					t.Fatalf("RunToCompletion = %v, want ErrReadOnlyFeature (%s)", err, tc.want)
				}
			})
		}
	})

	t.Run("rederivation_obeys_step_budget", func(t *testing.T) {
		ctx, action := loadAction(t, `package test {
			private import ScalarValues::*;
			action run {
				attribute source : Integer := 1;
				attribute live : Integer = source + 1;
				first start;
				then action done;
			}
		}`, "run")
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		ctx.writeBodyValue(exec.root.cells, exec.root.data, "source", integerValue(2))
		ctx.run.steps = ctx.maxSteps
		if _, _, err := ctx.readBodyValue(exec.root.cells, exec.root.data, "live"); !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("read live at exhausted budget = %v, want ErrStepLimitExceeded", err)
		}
	})

	t.Run("snapshot_restore", func(t *testing.T) {
		ctx, action := loadAction(t, `package test {
			private import ScalarValues::*;
			action run {
				attribute source : Integer := 1;
				attribute live : Integer = source * 2;
				out attribute result : Integer;
				first start;
				then action prepare {
					assign source := 4;
					assign result := live;
				}
				then action finish {
					assign source := 5;
					assign result := live;
				}
			}
		}`, "run")
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		exec.SetBreakpoint("finish")
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("RunToCompletion to breakpoint: %v", err)
		}
		if got := exec.PausedAt(); got != "finish" {
			t.Fatalf("PausedAt = %q, want finish", got)
		}
		snapshot, err := exec.Snapshot()
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		defer snapshot.Release()
		finish := func() {
			t.Helper()
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("RunToCompletion: %v", err)
			}
			if got := FormatValue(exec.Results()["result"]); got != "10" {
				t.Fatalf("result = %s, want 10 after source := 5", got)
			}
		}
		finish()
		snapshot.Restore()
		if got := FormatValue(exec.Results()["live"]); got != "8" {
			t.Fatalf("live after restore = %s, want the captured 8", got)
		}
		finish()
	})

	t.Run("held_image", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "body-bindings-image.sysml", parseAndBuild(t, `package test {
			private import ScalarValues::*;
			action def Run {
				attribute source : Integer := 1;
				attribute live : Integer = source * 2;
				out attribute result : Integer;
				first start;
				then action prepare {
					assign source := 4;
					assign result := live;
				}
				then action finish {
					assign source := 5;
					assign result := live;
				}
			}
			part def Host { perform action run : Run; }
		}`))
		host, err := ctx.Instantiate(oneSymbol(t, idx, "test::Host"))
		if err != nil {
			t.Fatalf("Instantiate Host: %v", err)
		}
		behavior, ok := host.Behavior("run")
		if !ok || behavior.Action == nil {
			t.Fatal("Host has no run action")
		}
		behavior.Action.SetBreakpoint("finish")
		if err := behavior.Action.RunToCompletion(); err != nil {
			t.Fatalf("RunToCompletion to breakpoint: %v", err)
		}
		image := imageInto(t, ctx, host)
		copied, ok := image.Instance(host.ID)
		if !ok {
			t.Fatalf("image has no Host #%d", host.ID)
		}
		copiedBehavior, ok := copied.Behavior("run")
		if !ok || copiedBehavior.Action == nil {
			t.Fatal("image has no run action")
		}
		if err := copiedBehavior.Action.RunToCompletion(); err != nil {
			t.Fatalf("RunToCompletion of imaged action: %v", err)
		}
		if got := FormatValue(copiedBehavior.Action.Results()["result"]); got != "10" {
			t.Fatalf("imaged result = %s, want 10 after lazy rederivation", got)
		}
	})

	t.Run("replay", func(t *testing.T) {
		ctx, action := loadAction(t, `package test {
			private import ScalarValues::*;
			action run {
				attribute source : Integer := 1;
				attribute live : Integer = source * 2;
				out attribute result : Integer;
				first start;
				then action write {
					assign source := 5;
					assign result := live;
				}
			}
		}`, "run")
		fresh := func() (*Context, error) { return NewContext(ctx.Model(), 100000), nil }
		replayed, err := ReplaySchedule(context.Background(), fresh, starterOf(action), Witness{}, ScheduleEnd)
		if err != nil {
			t.Fatalf("ReplaySchedule: %v", err)
		}
		if replayed.Err != nil {
			t.Fatalf("replay run: %v", replayed.Err)
		}
		if got, err := replayed.FinalValue("result"); err != nil || got != "10" {
			t.Fatalf("replayed result = %s, %v, want 10", got, err)
		}
	})

	t.Run("calc_usage_remains_one_shot", func(t *testing.T) {
		ctx, action := loadAction(t, `package test {
			private import ScalarValues::*;
			calc def Twice {
				in x : Integer;
				out y : Integer = x * 2;
			}
			action run {
				attribute source : Integer := 2;
				calc value : Twice { in x = source; }
				out attribute result : Integer;
				first start;
				then action write {
					assign result := value.y;
					assign source := 5;
					assign result := value.y;
				}
			}
		}`, "run")
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
		if got := FormatValue(exec.Results()["result"]); got != "4" {
			t.Fatalf("calc usage result = %s, want its one-shot pin value 4", got)
		}
	})
}
