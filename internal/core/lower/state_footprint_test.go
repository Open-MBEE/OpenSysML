package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// transitionOut is the transition out of the named state at position i.
func transitionOut(t *testing.T, graph *StateGraph, state string, i int) *Transition {
	t.Helper()
	s := stateNamed(graph, state)
	if s == nil {
		t.Fatalf("state %s was not lowered", state)
	}
	transitions := graph.Transitions[s]
	if i >= len(transitions) {
		t.Fatalf("state %s has %d transitions, want one at %d", state, len(transitions), i)
	}
	return transitions[i]
}

// A transition reads its guard and its source's activity, writes its effect's
// targets and the activity of the states it exits and enters; two transitions
// keeping to their own data and states are independent.
func TestTransitionFootprintsProjectGuardEffectAndActivity(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			attribute def Go;
			attribute def Stop;
			state Machine {
				attribute x : Integer = 0;
				attribute y : Integer = 0;
				attribute z : Integer = 0;
				entry; then idle;
				state idle;
				transition first idle accept Go if y > 0 then busy;
				transition first idle accept Stop do assign z := 1 then quiet;
				state busy { entry action { assign x := x + 1; } }
				state quiet;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	go_ := graph.TransitionFootprints()[transitionOut(t, graph, "idle", 0)]
	stop := graph.TransitionFootprints()[transitionOut(t, graph, "idle", 1)]
	if !hasPlace(go_.Reads, "y") || !hasPlace(go_.Reads, "idle") {
		t.Fatalf("Go reads %v, want the guard's y and the source's activity", placeNames(go_.Reads))
	}
	if !hasPlace(go_.Writes, "x") || !hasPlace(go_.Writes, "idle") || !hasPlace(go_.Writes, "busy") {
		t.Fatalf("Go writes %v, want busy's entry x and the activity of idle and busy", placeNames(go_.Writes))
	}
	// The event comes off the machine's own queue: the trigger is not a bus accept.
	if len(go_.Accepts) != 0 {
		t.Fatalf("Go accepts %v, want none", go_.Accepts)
	}
	if !hasPlace(stop.Writes, "z") || hasPlace(stop.Writes, "x") {
		t.Fatalf("Stop writes %v, want its effect's z and not busy's x", placeNames(stop.Writes))
	}
	if !strings.Contains(go_.String(), "active idle") {
		t.Fatalf("footprint renders as %q, want the activity spelt `active idle`", go_.String())
	}
	// Both leave idle: dependent through its activity, as two reactions of one leaf are.
	if !go_.Dependent(stop) {
		t.Fatal("two transitions out of one state must be dependent")
	}
}

// Transitions in two orthogonal regions keeping to their own states and data are
// independent; one leaving the parallel composite writes every region's activity.
func TestTransitionFootprintsKeepRegionsApart(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			attribute def Tick;
			attribute def Quit;
			state Machine {
				attribute a : Integer = 0;
				attribute b : Integer = 0;
				entry; then both;
				state both parallel {
					state left {
						entry; then l1;
						state l1;
						transition first l1 accept Tick do assign a := 1 then l2;
						state l2;
					}
					state right {
						entry; then r1;
						state r1;
						transition first r1 accept Tick do assign b := 1 then r2;
						state r2;
					}
				}
				transition first both accept Quit then stopped;
				state stopped;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	left := graph.TransitionFootprints()[transitionOut(t, graph, "l1", 0)]
	right := graph.TransitionFootprints()[transitionOut(t, graph, "r1", 0)]
	quit := graph.TransitionFootprints()[transitionOut(t, graph, "both", 0)]
	if left.Dependent(right) {
		t.Fatalf("region-local transitions must be independent:\nleft:\n%s\nright:\n%s", left, right)
	}
	for _, name := range []string{"both", "l1", "l2", "r1", "r2"} {
		if !hasPlace(quit.Writes, name) {
			t.Fatalf("Quit writes %v, want the activity of %s it exits", placeNames(quit.Writes), name)
		}
	}
	if !quit.Dependent(left) || !quit.Dependent(right) {
		t.Fatal("a transition leaving the composite must depend on each region's")
	}
}

// A transition into a choice covers every branch: their guards, effects and
// targets; entering a state with a completion transition queues a completion.
func TestTransitionFootprintsFollowChoiceBranchesAndCompletion(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			attribute def Go;
			state Machine {
				attribute k : Integer = 0;
				attribute p : Integer = 0;
				attribute q : Integer = 0;
				entry; then idle;
				state idle;
				choice pick;
				transition first idle accept Go do assign k := k + 1 then pick;
				transition first pick if k > 1 do assign p := 1 then high;
				transition first pick if k <= 1 do assign q := 1 then low;
				state high;
				state low;
				transition first high then done;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	fp := graph.TransitionFootprints()[transitionOut(t, graph, "idle", 0)]
	for _, name := range []string{"k", "p", "q", "high", "low", "idle"} {
		if !hasPlace(fp.Writes, name) {
			t.Fatalf("writes %v, want %s", placeNames(fp.Writes), name)
		}
	}
	if !hasPlace(fp.Reads, "k") {
		t.Fatalf("reads %v, want the branch guards' k", placeNames(fp.Reads))
	}
	if !fp.Completion {
		t.Fatal("entering high, whose completion transition fires, queues a completion")
	}
	// A pseudostate's branches are segments of the transitions into it, not moves.
	pick := graph.Pseudostates[0]
	for _, branch := range graph.Transitions[pick] {
		if _, has := graph.TransitionFootprints()[branch]; has {
			t.Fatal("a branch out of a pseudostate has no footprint of its own")
		}
	}
	var done *ast.StateNode
	for _, state := range graph.States {
		if graph.Completes(state) {
			done = state
		}
	}
	if done == nil {
		t.Fatal("no done vertex")
	}
}

// Firing a join runs the effect of every transition into it, so each incoming
// transition's footprint writes what all the incoming effects write.
func TestTransitionFootprintsFoldJoinIncomingEffects(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine parallel {
				attribute p : Integer = 0;
				attribute q : Integer = 0;
				state left {
					entry; then l1;
					state l1;
					transition first l1 do assign p := 1 then sync;
				}
				state right {
					entry; then r1;
					state r1;
					transition first r1 do assign q := 1 then sync;
				}
				join sync;
				transition first sync then done;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	for _, source := range []string{"l1", "r1"} {
		fp := graph.TransitionFootprints()[transitionOut(t, graph, source, 0)]
		if !hasPlace(fp.Writes, "p") || !hasPlace(fp.Writes, "q") {
			t.Fatalf("out of %s writes %v, want both incoming effects' p and q", source, placeNames(fp.Writes))
		}
	}
}

// Every entry, do and exit behavior has a footprint: what its statements touch
// and the activity of the state it belongs to.
func TestBehaviorFootprintsCoverEveryBehavior(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				attribute n : Integer = 0;
				attribute m : Integer = 0;
				entry; then s;
				state s {
					entry action { assign n := 1; }
					do action { assign m := n; }
					exit action { assign n := 0; }
				}
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	s := stateNamed(graph, "s")
	behaviors := graph.Behaviors[s]
	do := graph.BehaviorFootprints()[behaviors.Do[0].Node]
	if !hasPlace(do.Writes, "m") || !hasPlace(do.Reads, "n") || !hasPlace(do.Reads, "s") {
		t.Fatalf("do footprint reads %v writes %v, want n and s's activity read, m written",
			placeNames(do.Reads), placeNames(do.Writes))
	}
	for _, behavior := range append(behaviors.Entry, behaviors.Exit...) {
		if _, has := graph.BehaviorFootprints()[behavior.Node]; !has {
			t.Fatalf("behavior %v has no footprint", behavior.Node)
		}
	}
}
