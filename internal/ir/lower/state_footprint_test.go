package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
	goT := graph.TransitionFootprints()[transitionOut(t, graph, "idle", 0)]
	stop := graph.TransitionFootprints()[transitionOut(t, graph, "idle", 1)]
	if !hasPlace(goT.Reads, "y") || !hasPlace(goT.Reads, "idle") {
		t.Fatalf("Go reads %v, want the guard's y and the source's activity", placeNames(goT.Reads))
	}
	if !hasPlace(goT.Writes, "x") || !hasPlace(goT.Writes, "idle") || !hasPlace(goT.Writes, "busy") {
		t.Fatalf("Go writes %v, want busy's entry x and the activity of idle and busy", placeNames(goT.Writes))
	}
	// The event comes off the machine's own queue: the trigger is not a bus accept.
	if len(goT.Accepts) != 0 {
		t.Fatalf("Go accepts %v, want none", goT.Accepts)
	}
	if !hasPlace(stop.Writes, "z") || hasPlace(stop.Writes, "x") {
		t.Fatalf("Stop writes %v, want its effect's z and not busy's x", placeNames(stop.Writes))
	}
	if !strings.Contains(goT.String(), "active idle") {
		t.Fatalf("footprint renders as %q, want the activity spelt `active idle`", goT.String())
	}
	// Both leave idle: dependent through its activity, as two reactions of one leaf are.
	if !goT.Dependent(stop) {
		t.Fatal("two transitions out of one state must be dependent")
	}
}

// Entering a state starts its do behavior, whose steps may run within the move: the
// transition's footprint carries the do behavior's writes beside the entry's.
func TestTransitionFootprintsCoverEnteredDoBehaviors(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			attribute def Go;
			state Machine {
				attribute x : Integer = 0;
				attribute y : Integer = 0;
				attribute z : Integer = 0;
				entry; then idle;
				state idle {
					do action { assign z := z + 1; }
				}
				transition first idle accept Go then busy;
				state busy {
					entry action { assign x := x + 1; }
					do action { assign y := y + 1; }
				}
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	goT := graph.TransitionFootprints()[transitionOut(t, graph, "idle", 0)]
	if !hasPlace(goT.Writes, "x") || !hasPlace(goT.Writes, "y") {
		t.Fatalf("Go writes %v, want busy's entry x and its do behavior's y", placeNames(goT.Writes))
	}
	if hasPlace(goT.Writes, "z") || hasPlace(goT.Reads, "z") {
		t.Fatalf("Go touches %v / %v, want nothing of the do behavior of the state it leaves", placeNames(goT.Reads), placeNames(goT.Writes))
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

// Firing a join synchronizes every transition into it: each incoming
// transition's footprint reads the other segments' guards, triggers and sources'
// activity, and writes the activity and exit behaviors of every state the join
// leaves, the sibling regions' included, whichever state declares the join.
func TestTransitionFootprintsFoldJoinSiblingSegments(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			attribute def Go;
			attribute def Tick;
			state Machine {
				attribute armed : Boolean = false;
				attribute level : Integer = 0;
				attribute x : Integer = 0;
				attribute y : Integer = 0;
				entry; then work;
				state work parallel {
					state left {
						entry; then a1;
						state a1;
						transition first a1 accept Go then sync;
					}
					state right {
						entry; then b1;
						state b1 { exit action { assign x := x + 1; } }
						transition first b1 accept Go if armed then sync;
					}
					state third {
						entry; then c1;
						state c1 { exit action { assign y := 1; } }
						transition first c1 accept when level > 1 then sync;
					}
					state fourth {
						entry; then d1;
						state d1;
						transition first d1 accept Tick do assign armed := true then d1;
					}
				}
				join sync;
				transition first sync then finished;
				state finished;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	fp := graph.TransitionFootprints()[transitionOut(t, graph, "a1", 0)]
	for _, name := range []string{"armed", "level", "b1", "c1"} {
		if !hasPlace(fp.Reads, name) {
			t.Fatalf("out of a1 reads %v, want %s: the sibling segments' guards, triggers and sources", placeNames(fp.Reads), name)
		}
	}
	for _, name := range []string{"x", "y", "a1", "b1", "c1", "d1", "left", "right", "third", "fourth", "work"} {
		if !hasPlace(fp.Writes, name) {
			t.Fatalf("out of a1 writes %v, want %s: every state the join leaves and its exits", placeNames(fp.Writes), name)
		}
	}
	tick := graph.TransitionFootprints()[transitionOut(t, graph, "d1", 0)]
	if !fp.Dependent(tick) {
		t.Fatalf("the join reads the armed Tick writes:\njoin:\n%s\ntick:\n%s", fp, tick)
	}
}

// A segment pending into a join is classified by what it does on its own: its
// trigger, guard and effect and the exits from its source up to the join's
// owner, not the other transitions out of its source, nor the owner's exit and
// the join's way out, which follow whichever segment fires first.
func TestJoinSegmentFootprintsCoverOnlyTheSegment(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			attribute def Go;
			attribute def Other;
			state Machine {
				attribute a : Integer = 0;
				attribute b : Integer = 0;
				attribute log : Integer = 0;
				attribute armed : Boolean = false;
				entry; then Owner;
				state Owner parallel {
					exit action { assign log := log + 1; }
					state left {
						entry; then l1;
						state l1 { exit action { assign a := a + 1; } }
						transition first l1 accept Go do assign a := 1 then sync;
						transition first l1 accept Other do assign log := 10 then l1;
					}
					state right {
						entry; then r1;
						state r1;
						transition first r1 accept Go if armed do assign b := 2 then sync;
						transition first r1 accept Other do assign log := 20 then r1;
					}
					join sync;
					transition first sync do assign log := log + 100 then done;
				}
				state done;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	var join *ast.PseudostateNode
	for _, ps := range graph.Pseudostates {
		if ps.Kind == ast.PseudostateJoin {
			join = ps
		}
	}
	if join == nil {
		t.Fatal("no join lowered")
	}
	segs := []*Transition{transitionOut(t, graph, "l1", 0), transitionOut(t, graph, "r1", 0)}
	fps := graph.JoinSegmentFootprints(join, segs)
	left, right := fps[0], fps[1]
	for _, name := range []string{"a", "l1", "left"} {
		if !hasPlace(left.Writes, name) {
			t.Fatalf("the left segment writes %v, want %s: its effect and the states it exits", placeNames(left.Writes), name)
		}
	}
	if hasPlace(left.Writes, "log") || hasPlace(right.Writes, "log") {
		t.Fatalf("a segment writes log (left %v, right %v): the unrelated transitions, the owner's exit and the way out are not the segment's own", placeNames(left.Writes), placeNames(right.Writes))
	}
	if !hasPlace(right.Reads, "armed") || !hasPlace(right.Writes, "b") {
		t.Fatalf("the right segment reads %v, writes %v, want its guard read and its effect written", placeNames(right.Reads), placeNames(right.Writes))
	}
	if left.Dependent(right) {
		t.Fatalf("the segments conflict:\nleft:\n%s\nright:\n%s", left, right)
	}
	if fp := graph.TransitionFootprints()[segs[0]]; !hasPlace(fp.Writes, "log") {
		t.Fatalf("the whole transition out of l1 writes %v, want log: it follows the join out", placeNames(fp.Writes))
	}
}

// A join of the machine's own regions leaves every one of them.
func TestTransitionFootprintsFoldJoinOfTopRegions(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			attribute def Go;
			state Machine parallel {
				attribute x : Integer = 0;
				state left {
					entry; then l1;
					state l1;
					transition first l1 accept Go then sync;
				}
				state right {
					entry; then r1;
					state r1 { exit action { assign x := 1; } }
					transition first r1 accept Go then sync;
				}
				state third {
					entry; then t1;
					state t1;
				}
				join sync;
				transition first sync then done;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	fp := graph.TransitionFootprints()[transitionOut(t, graph, "l1", 0)]
	for _, name := range []string{"x", "l1", "r1", "t1"} {
		if !hasPlace(fp.Writes, name) {
			t.Fatalf("out of l1 writes %v, want %s: every region is left", placeNames(fp.Writes), name)
		}
	}
	if !hasPlace(fp.Reads, "r1") {
		t.Fatalf("out of l1 reads %v, want the sibling source r1", placeNames(fp.Reads))
	}
}

// A transition into a fork covers the regions the fork omits as well as its
// branches' targets: the omitted region starts by default, so its states'
// activity, entry behavior and completion are the fork's too.
func TestTransitionFootprintsCoverForkOmittedRegions(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			attribute def Go;
			attribute def Tick;
			state Machine {
				attribute k : Integer = 0;
				attribute n : Integer = 0;
				entry; then idle;
				state idle;
				state other;
				state work parallel {
					state left { state a; }
					state right { state b; }
					state third {
						entry; then c;
						state c { entry action { assign n := n + 1; } }
						transition first c then c2;
						state c2;
					}
				}
				fork split;
				transition first idle accept Go do assign k := 1 then split;
				transition first idle accept Tick do assign n := 0 then other;
				transition first split then a;
				transition first split then b;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	fork := graph.TransitionFootprints()[transitionOut(t, graph, "idle", 0)]
	for _, name := range []string{"k", "n", "work", "a", "b", "c", "c2"} {
		if !hasPlace(fork.Writes, name) {
			t.Fatalf("writes %v, want %s", placeNames(fork.Writes), name)
		}
	}
	if !fork.Completion {
		t.Fatal("entering c, whose completion transition fires, queues a completion")
	}
	tick := graph.TransitionFootprints()[transitionOut(t, graph, "idle", 1)]
	if hasPlace(tick.Writes, "c") || hasPlace(tick.Writes, "a") {
		t.Fatalf("Tick writes %v, want none of work's states", placeNames(tick.Writes))
	}
}

// A transition from inside a parallel composite into a fork over its regions
// restarts every region: the sibling regions' active states are exited too, so
// their activity and exit behaviors are the fork's.
func TestTransitionFootprintsCoverForkSiblingRegionExits(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			attribute def Go;
			attribute def Tick;
			state Machine {
				attribute x : Integer = 0;
				attribute y : Integer = 0;
				entry; then work;
				state work parallel {
					state left {
						entry; then a;
						state a;
						state c;
						transition first a accept Go then split;
					}
					state right {
						entry; then b;
						state b { exit action { assign x := x + 1; } }
						state d;
						transition first b accept Tick do assign y := x then d;
					}
				}
				fork split;
				transition first split then c;
				transition first split then d;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	fork := graph.TransitionFootprints()[transitionOut(t, graph, "a", 0)]
	for _, name := range []string{"x", "a", "b", "c", "d"} {
		if !hasPlace(fork.Writes, name) {
			t.Fatalf("writes %v, want %s", placeNames(fork.Writes), name)
		}
	}
	if hasPlace(fork.Writes, "work") {
		t.Fatalf("writes %v, want work's own activity untouched: it stays active", placeNames(fork.Writes))
	}
	tick := graph.TransitionFootprints()[transitionOut(t, graph, "b", 0)]
	if !fork.Dependent(tick) {
		t.Fatalf("the fork exits b, whose exit writes the x Tick reads:\nfork:\n%s\ntick:\n%s", fork, tick)
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

// A transition into a named terminate node carries the node body's places and the
// source's activity, and stays dynamic for the occurrence the node ends.
func TestTransitionFootprintsCoverNamedTerminateNode(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			attribute def Stop;
			attribute def Go;
			part worker;
			state Machine {
				attribute alarm : Integer = 0;
				attribute level : Integer = 0;
				entry; then busy;
				state busy;
				transition first busy accept Stop then halt;
				transition first busy accept Go then quiet;
				action halt terminate worker { assign alarm := level; }
				action stop terminate;
				state quiet;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	halt := graph.TransitionFootprints()[transitionOut(t, graph, "busy", 0)]
	if !hasPlace(halt.Writes, "alarm") || !hasPlace(halt.Reads, "level") {
		t.Fatalf("Stop reads %v writes %v, want the node body's level read and alarm written",
			placeNames(halt.Reads), placeNames(halt.Writes))
	}
	if !hasPlace(halt.Writes, "busy") {
		t.Fatalf("Stop writes %v, want the activity of the state it leaves", placeNames(halt.Writes))
	}
	if !halt.Dynamic {
		t.Fatal("ending another occurrence reaches beyond the machine: the footprint must be dynamic")
	}
	halting := graph.TerminateBehaviors[terminateNamed(t, graph, "halt")]
	if _, has := graph.BehaviorFootprints()[halting.Node]; !has {
		t.Fatal("the named terminate node's behavior has no footprint")
	}
}

// A branch out of a pseudostate into a named terminate node, taken by a transition
// whose origin is unknown, carries the node body's places and the completion entered.
func TestBranchFootprintsCoverNamedTerminateNode(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			part worker;
			state Machine {
				attribute alarm : Integer = 0;
				attribute level : Integer = 0;
				entry; then pick;
				choice pick;
				transition first pick if level > 1 then halt;
				transition first pick if level <= 1 then quiet;
				action halt terminate worker { assign alarm := level; }
				state quiet;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	pick := graph.Pseudostates[0]
	halt := graph.BranchFootprints()[graph.Transitions[pick][0]]
	if !hasPlace(halt.Writes, "alarm") || !hasPlace(halt.Reads, "level") {
		t.Fatalf("halt branch reads %v writes %v, want the node body's level read and alarm written",
			placeNames(halt.Reads), placeNames(halt.Writes))
	}
	completion := graph.TerminateCompletion[terminateNamed(t, graph, "halt")]
	if !hasPlace(halt.Writes, completion.Name) {
		t.Fatalf("halt branch writes %v, want the activity of the completion vertex %s", placeNames(halt.Writes), completion.Name)
	}
	if !halt.Dynamic {
		t.Fatal("ending another occurrence reaches beyond the machine: the footprint must be dynamic")
	}
	quiet := graph.BranchFootprints()[graph.Transitions[pick][1]]
	if hasPlace(quiet.Writes, "alarm") || quiet.Dynamic {
		t.Fatalf("quiet branch writes %v dynamic %v, want neither the node body nor dynamism", placeNames(quiet.Writes), quiet.Dynamic)
	}
}

func terminateNamed(t *testing.T, graph *StateGraph, name string) *ast.Usage {
	t.Helper()
	for usage := range graph.TerminateBehaviors {
		if n, _ := ast.EffectiveName(usage); n == name {
			return usage
		}
	}
	t.Fatalf("no named terminate node %q", name)
	return nil
}
