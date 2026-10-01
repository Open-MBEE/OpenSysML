package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestRuntimeRobustnessJunctionJoin(t *testing.T) {
	t.Run("junction_after_choice_keeps_runtime_error", func(t *testing.T) {
		_, _, err := executeStateSource(t, "Machine", `package test {
			state Machine {
				entry; then start;
				state start;
				state done;
				choice pick;
				junction dead;
				transition first start then pick;
				transition first pick then dead;
				transition first dead if false then done;
			}
		}`)
		if err == nil || !errors.Is(err, errNoWayThrough) || errors.Is(err, ErrChoiceWithoutBranch) ||
			!strings.Contains(err.Error(), "no guard evaluated to true") {
			t.Fatalf("run error = %v, want the dead junction error, not ErrChoiceWithoutBranch", err)
		}
	})

	t.Run("junction_cycle_keeps_runtime_error", func(t *testing.T) {
		_, _, err := executeStateSource(t, "Machine", `package test {
			state Machine {
				entry; then start;
				state start;
				choice done;
				junction firstRoute;
				junction secondRoute;
				transition first start then firstRoute;
				transition first firstRoute then secondRoute;
				transition first secondRoute then firstRoute;
			}
		}`)
		if err == nil || !strings.Contains(err.Error(), "outgoing transitions form a cycle") {
			t.Fatalf("run error = %v, want a pseudostate cycle error", err)
		}
	})

	t.Run("junction_unevaluable_guard_keeps_runtime_error", func(t *testing.T) {
		_, _, err := executeStateSource(t, "Machine", `package test {
			state Machine {
				attribute zero : Integer = 0;
				entry; then start;
				state start;
				state done;
				junction route;
				transition first start then route;
				transition first route if 1 / zero > 0 then done;
			}
		}`)
		if !errors.Is(err, ErrDivisionByZero) {
			t.Fatalf("run error = %v, want ErrDivisionByZero", err)
		}
	})

	t.Run("join_exit_failure_does_not_record_arrival", func(t *testing.T) {
		exec := stateExecutorForSource(t, "Machine", `package test {
			private import ScalarValues::*;
			attribute def X;
			attribute def Y;
			state Machine {
				attribute zero : Integer = 0;
				entry; then owner;
				state owner parallel {
					state left {
						entry; then source;
						state source {
							exit { assign zero := 1 / zero; }
						}
						transition first source accept X then sync;
					}
					state right {
						entry; then other;
						state other;
						transition first other accept Y then sync;
					}
					join sync;
					transition first sync then done;
				}
				state done;
			}
		}`)
		exec.SendSignal("X", nil)
		if err := exec.ProcessNextEvent(); !errors.Is(err, ErrDivisionByZero) {
			t.Fatalf("ProcessNextEvent(X) = %v, want ErrDivisionByZero", err)
		}
		if len(exec.joinArrived) != 0 {
			t.Fatalf("join arrivals after source exit failure = %v, want none", exec.joinArrived)
		}
	})

	t.Run("join_same_occurrence_exit_failure_records_no_arrivals", func(t *testing.T) {
		exec := stateExecutorForSource(t, "Machine", `package test {
			private import ScalarValues::*;
			attribute def Go;
			state Machine {
				attribute zero : Integer = 0;
				entry; then owner;
				state owner parallel {
					state left {
						entry; then first;
						state first;
						transition first first accept Go then sync;
					}
					state right {
						entry; then second;
						state second {
							exit { assign zero := 1 / zero; }
						}
						transition first second accept Go then sync;
					}
					join sync;
					transition first sync then done;
				}
				state done;
			}
		}`)
		exec.SendSignal("Go", nil)
		if err := exec.ProcessNextEvent(); !errors.Is(err, ErrDivisionByZero) {
			t.Fatalf("ProcessNextEvent(Go) = %v, want ErrDivisionByZero", err)
		}
		if len(exec.joinArrived) != 0 {
			t.Fatalf("join arrivals after segment exit failure = %v, want none", exec.joinArrived)
		}
	})

	t.Run("dead_join_exit_does_not_fire_same_occurrence_segments", func(t *testing.T) {
		exec := stateExecutorForSource(t, "Machine", `package test {
			private import ScalarValues::*;
			attribute def Go;
			state Machine {
				attribute open : Boolean = false;
				entry; then owner;
				state owner parallel {
					state left {
						entry; then first;
						state first;
						transition first first accept Go then sync;
					}
					state right {
						entry; then second;
						state second;
						transition first second accept Go then sync;
					}
					join sync;
					transition first sync if open then done;
				}
				state done;
			}
		}`)
		exec.SendSignal("Go", nil)
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("ProcessNextEvent(Go) = %v, want no error", err)
		}
		for _, name := range []string{"first", "second"} {
			if state := stateNamed(t, exec, name); !exec.inActiveConfiguration(state) {
				t.Errorf("state %s is not active after the dead join route", name)
			}
		}
		if len(exec.joinArrived) != 0 {
			t.Fatalf("join arrivals after dead join route = %v, want none", exec.joinArrived)
		}
	})

	t.Run("dead_timer_join_exit_disables_completing_segment", func(t *testing.T) {
		exec := stateExecutorForSource(t, "Machine", `package test {
			state Machine {
				attribute open : Boolean = false;
				entry; then owner;
				state owner parallel {
					state left {
						entry; then first;
						state first;
						transition first first accept after 1 then sync;
					}
					state right {
						entry; then second;
						state second;
						transition first second accept after 1 then sync;
					}
					join sync;
					transition first sync if open then done;
				}
				state done;
			}
		}`)
		if _, err := exec.ctx.Advance(2); err != nil {
			t.Fatalf("Advance(2) = %v, want no error", err)
		}
		var sync *ast.PseudostateNode
		for _, transitions := range exec.graph.Transitions {
			for _, transition := range transitions {
				join, ok := transition.Target.(*ast.PseudostateNode)
				if ok && join.Kind == ast.PseudostateJoin && join.Name == "sync" {
					sync = join
				}
			}
		}
		if sync == nil {
			t.Fatal("sync join not found")
		}
		if got := len(exec.joinArrived[sync]); got != 1 {
			t.Fatalf("sync arrivals after first timer expiry = %d, want 1", got)
		}
		activeSources := 0
		for _, name := range []string{"first", "second"} {
			if state := stateNamed(t, exec, name); exec.inActiveConfiguration(state) {
				activeSources++
			}
		}
		if activeSources != 1 {
			t.Fatalf("active join-segment sources = %d, want exactly one", activeSources)
		}
	})
}
