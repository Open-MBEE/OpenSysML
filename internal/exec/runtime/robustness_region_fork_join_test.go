package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessRegionForkJoin covers forks and joins across state regions: shared
// regions, triggers on branches, unjoined branches and time segments.
func TestRuntimeRobustnessRegionForkJoin(t *testing.T) {
	t.Run("fork_branches_share_region", testForkBranchesShareRegion)
	t.Run("fork_branch_with_a_trigger", testForkBranchWithATrigger)
	t.Run("join_with_one_incoming_branch", testJoinWithOneIncomingBranch)
	t.Run("join_incoming_effect_that_fails", testJoinIncomingEffectThatFails)
	t.Run("join_with_two_segments_from_one_source", testJoinWithTwoSegmentsFromOneSource)
	t.Run("join_from_nested_states_wrapper_exit_that_fails", testJoinFromNestedStatesWrapperExitThatFails)
	t.Run("join_from_composite_source_substate_exit_that_fails", testJoinFromCompositeSourceSubstateExitThatFails)
	t.Run("join_of_machine_regions_nested_source_owner_exit_that_fails", testJoinOfMachineRegionsNestedSourceOwnerExitThatFails)
	t.Run("join_time_segment_sibling_guard_that_fails", testJoinTimeSegmentSiblingGuardThatFails)
}

// testForkBranchesShareRegion: a fork whose branches land in the same region
// cannot produce one active state per region.
func testForkBranchesShareRegion(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state ready;
			state working parallel {
				state left {
					entry; then ls;
					state ls;
					state a;
					state b;
					succession first ls then a;
				}
				state right {
					entry; then rs;
					state rs;
					state c;
					succession first rs then c;
				}
			}
			fork split;

			succession first init then ready;
			transition first ready then split;
			transition first split then a;
			transition first split then b;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for fork branches in the same region")
	}
	if !strings.Contains(err.Error(), "in the same region") {
		t.Errorf("expected a same-region error, got: %v", err)
	}
}

// testForkBranchWithATrigger: a fork's branches fire as one with the fork, so a
// branch that waits for an occurrence is refused before the machine runs rather
// than firing regardless of it.
func testForkBranchWithATrigger(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		attribute def Go;
		state Machine {
			entry; then init;
			state init;
			state working parallel {
				state left { state a; }
				state right { state b; }
			}
			fork split;

			transition first init then split;
			transition first split accept Go then a;
			transition first split then b;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for the triggered fork branch")
	}
	if !strings.Contains(err.Error(), "fork split: outgoing transitions cannot have triggers") {
		t.Errorf("expected the triggered branch to be refused, got: %v", err)
	}
}

// testJoinWithOneIncomingBranch: a join synchronizes branches, so a single
// incoming transition is a modeling error rather than a pass-through.
func testJoinWithOneIncomingBranch(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state ready;
			join sync;

			succession first init then ready;
			transition first ready then sync;
			transition first sync then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for a join with one incoming transition")
	}
	if !strings.Contains(err.Error(), "at least two incoming transitions") {
		t.Errorf("expected an incoming-branch-count error, got: %v", err)
	}
}

// testJoinIncomingEffectThatFails: firing a join runs the effect of every
// transition into it, so an effect on a segment other than the firing one that
// fails surfaces as the step's error rather than being skipped.
// A source with two transitions into one join is refused when lowered: the
// alternative a trigger does not enable would otherwise fire with the join.
func testJoinWithTwoSegmentsFromOneSource(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		attribute def Go;
		attribute def Stop;
		state Machine parallel {
			attribute log : String = "";

			state left {
				entry; then l1;
				state l1;
				transition first l1 accept Go do assign log := log + "go;" then sync;
				transition first l1 accept Stop do assign log := log + "stop;" then sync;
			}
			state right {
				entry; then r1;
				state r1;
				transition first r1 then sync;
			}
			join sync;
			transition first sync then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for a join two transitions of one source enter")
	}
	if !strings.Contains(err.Error(), "two incoming transitions leave l1") {
		t.Errorf("expected a shared-source error, got: %v", err)
	}
}

func testJoinIncomingEffectThatFails(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine parallel {
			attribute x : Integer = 0;
			attribute zero : Integer = 0;

			state left {
				entry; then l1;
				state l1;
				transition first l1 do assign x := 1 then sync;
			}
			state right {
				entry; then r1;
				state r1;
				transition first r1 do assign x := 1 / zero then sync;
			}
			join sync;
			transition first sync then done;
		}
	}`)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want the failing incoming effect's division by zero", err)
	}
}

// testJoinFromNestedStatesWrapperExitThatFails: a join whose sources lie nested
// below its owner's region states exits the composite states between each source
// and its region, so a failing exit action on one of them is the step's error.
func testJoinFromNestedStatesWrapperExitThatFails(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			attribute x : Integer = 0;
			attribute zero : Integer = 0;

			entry; then work;
			state work parallel {
				state left {
					entry; then il;
					state il {
						exit action { assign x := 1 / zero; }
						entry; then l1;
						state l1;
					}
					transition first l1 then sync;
				}
				state right {
					entry; then ir;
					state ir {
						entry; then r1;
						state r1;
					}
					transition first r1 then sync;
				}
			}
			join sync;
			transition first sync then done;
		}
	}`)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want the nested wrapper's failing exit action's division by zero", err)
	}
}

// testJoinFromCompositeSourceSubstateExitThatFails: a join segment leaving a
// composite state whose substate is active exits that substate first, so its
// failing exit action is the step's error rather than the join never firing.
func testJoinFromCompositeSourceSubstateExitThatFails(t *testing.T) {
	src := `package test {
		attribute def Go;
		state Machine {
			attribute x : Integer = 0;
			attribute zero : Integer = 0;

			entry; then work;
			state work parallel {
				state left {
					entry; then il;
					state il {
						entry; then l1;
						state l1 {
							exit action { assign x := 1 / zero; }
						}
					}
					transition first il accept Go then sync;
				}
				state right {
					entry; then r1;
					state r1;
					transition first r1 accept Go then sync;
				}
			}
			join sync;
			transition first sync then done;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine Machine not found")
	}
	_, _, err := ctx.ExecuteStateWithEvents(sym, []string{"Go"})
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want the active substate's failing exit action's division by zero", err)
	}
}

// testJoinOfMachineRegionsNestedSourceOwnerExitThatFails: a segment into a join of
// the machine's own regions leaves a state nested in an orthogonal state of one
// region, so that state is exited once, as part of the segment, and its failing
// exit action is the step's error rather than a second exit of a left state.
func testJoinOfMachineRegionsNestedSourceOwnerExitThatFails(t *testing.T) {
	src := `package test {
		attribute def Go;
		state def Machine parallel {
			attribute x : Integer = 0;
			attribute zero : Integer = 0;

			state left {
				entry; then inner;
				state inner parallel {
					exit action { assign x := 1 / zero; }
					state l1 { entry; then a; state a; }
					state l2 { entry; then c; state c; }
				}
				transition first a accept Go then sync;
			}
			state right {
				entry; then b;
				state b;
				transition first b accept Go then sync;
			}
			join sync;
			transition first sync then done;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine Machine not found")
	}
	_, _, err := ctx.ExecuteStateWithEvents(sym, []string{"Go"})
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want the nested orthogonal state's failing exit action's division by zero", err)
	}
}

// testJoinTimeSegmentSiblingGuardThatFails: a timer coming due on one segment
// into a join reads the other due timer segments' guards to know whether the
// join is enabled, so one that cannot be evaluated then is the step's error.
func testJoinTimeSegmentSiblingGuardThatFails(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			attribute zero : Integer = 1;

			entry; then work;
			state work parallel {
				state left {
					entry; then l1;
					state l1;
					transition first l1 accept after 2 then sync;
				}
				state right {
					entry; then r1;
					state r1;
					transition first r1 accept after 2 if 1 / zero > 0 then sync;
				}
				state aux {
					entry; then c1;
					state c1;
					state c2;
					transition first c1 accept after 1 do assign zero := 0 then c2;
				}
			}
			join sync;
			transition first sync then done;
		}
	}`)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want the sibling segment's guard's division by zero", err)
	}
}
