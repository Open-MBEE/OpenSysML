package repl

import (
	"strings"
	"testing"
)

const performedActionDebugModel = `
package P {
	private import ScalarValues::*;
	action def Run {
		in ref ctx : Robot[1];
		out n : Integer[1];
	}
	part def Robot {
		attribute steps : Integer = 0;
		perform action run : Run {
			in ref :>> ctx = this;
			first start;
			then action wait accept after 5 [SI::s];
			then action tick assign steps := steps + 1;
			then action yield assign n := steps;
			then done;
		}
	}
	action def Ping;
	part def Twin { perform action morning : Ping; perform action evening : Ping; }
	part robot : Robot;
	part twin : Twin;
}
`

// %action <action> <object> attaches to the performance the object already runs
// of the action — named as the performed usage or as its definition — rather than
// starting a second one beside it: the object is written once, by that performance.
func TestActionOnItsPerformerAttachesToTheRunningPerformance(t *testing.T) {
	for _, name := range []string{"P::Robot::run", "P::Run"} {
		s := loadSource(t, performedActionDebugModel)
		run(t, s, "%instantiate P::robot")
		wants(t, run(t, s, "%action "+name+" P::robot"),
			`✓ Debugging action "run" performed by object #1 of "P::robot"`,
			`already performs "`+name+`", so this session attaches to that running performance rather than starting a second one`,
			"State: Waiting")
		wants(t, run(t, s, "%advance 10"), "Action state: Completed")
		wants(t, run(t, s, "%features P::robot"), "steps = 1")
	}
}

// Run to completion, the attached performance is the one that completes.
func TestActionOnItsPerformerContinuesTheRunningPerformance(t *testing.T) {
	s := loadSource(t, performedActionDebugModel)
	run(t, s, "%instantiate P::robot")
	run(t, s, "%action P::Robot::run P::robot")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "run.n = 1")
	wants(t, run(t, s, "%features P::robot"), "steps = 1")
}

// An object performing the definition under several usages names no one
// performance, so the session refuses and names the usages.
func TestActionOnAnObjectPerformingItTwiceIsRefused(t *testing.T) {
	s := loadSource(t, performedActionDebugModel)
	run(t, s, "%instantiate P::twin")
	out := run(t, s, "%action P::Ping P::twin")
	wants(t, out, `performs "P::Ping" as 2 actions, so naming the definition attaches to none of them: name the performed usage instead — P::Twin::morning or P::Twin::evening`)
	if strings.Contains(out, "Started action executor") {
		t.Errorf("an ambiguous action started a performance:\n%s", out)
	}
}

// -action "<action> <object>" -advance T is the same attachment: the duration
// bounds the object's own performance, so a short one stops it before its wait
// and a long one completes it, the object written once either way.
func TestRunForOnAPerformerDrivesItsOwnPerformance(t *testing.T) {
	s := loadSource(t, performedActionDebugModel)
	run(t, s, "%instantiate P::robot")
	short := s.RunFor([]Behavior{{Name: "P::Robot::run", Performer: []string{"P::robot"}}}, nil, 2)
	if len(short) != 1 || short[0].Status != VerdictUnresolved {
		t.Fatalf("advance 2 verdicts = %+v, want one unresolved", short)
	}
	wants(t, strings.Join(short[0].Lines, "\n"), "attaches to that running performance", "stopped at Waiting at simulation time 2.0 without completing", "t=5.0")
	wants(t, run(t, s, "%features P::robot"), "steps = 0")

	s = loadSource(t, performedActionDebugModel)
	run(t, s, "%instantiate P::robot")
	long := s.RunFor([]Behavior{{Name: "P::Robot::run", Performer: []string{"P::robot"}}}, nil, 10)
	if len(long) != 1 || long[0].Status != VerdictHolds {
		t.Fatalf("advance 10 verdicts = %+v, want one holding", long)
	}
	wants(t, strings.Join(long[0].Lines, "\n"), "attaches to that running performance", "✓ Action completed", "run.n = 1")
	wants(t, run(t, s, "%features P::robot"), "steps = 1")
}

const heldActionDebugModel = `
package H {
	private import ScalarValues::*;
	part def Pair {
		attribute marks : Integer = 0;
		perform action earlier { first start; then action w accept after 5 [SI::s]; then done; }
		perform action later { first start; then action mark assign marks := marks + 1; then done; }
		first earlier then later;
	}
	part pair : Pair;
}
`

// A performed action a succession order holds back has no performance to attach
// to yet: %action refuses it with the order's violation while the predecessor
// runs, and attaches to the object's own performance once the clock released it.
func TestActionHeldBySuccessionIsRefusedUntilReleased(t *testing.T) {
	s := loadSource(t, heldActionDebugModel)
	run(t, s, "%instantiate H::pair")
	wants(t, run(t, s, "%action H::Pair::later H::pair"),
		"error: succession-order-violated: succession first-then succession orders earlier of object #1 before later of object #1, but earlier of object #1 has already begun")
	wants(t, run(t, s, "%action H::Pair::earlier H::pair"), `✓ Debugging action "earlier" performed by object #1 of "H::pair"`, "State: Waiting")
	wants(t, run(t, s, "%advance 10"), "Action state: Completed")
	wants(t, run(t, s, "%action H::Pair::later H::pair"), `✓ Debugging action "later" performed by object #1 of "H::pair"`, "State: Completed")
	wants(t, run(t, s, "%features H::pair"), "marks = 1", "later: performed action, completed")
}
