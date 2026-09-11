package repl

import (
	"slices"
	"testing"
)

// heldBehaviourModel declares held objects whose types run a behaviour: a beacon
// exhibiting a state machine whose transition writes a feature, and a tug performing
// an action that writes one once the clock reaches its wait (with a machine of its own
// for %state to attach to, so %advance can move the clock), and a case reading its
// subject.
const heldBehaviourModel = `package Held {
	private import ScalarValues::*;
	private import SI::*;
	attribute def Lit;
	part def Ship {
		attribute cost : Real = 5.0;
	}
	part def Beacon :> Ship {
		exhibit state blinking {
			entry; then off;
			state off;
			transition first off accept Lit do assign cost := cost + 10.0 then on;
			state on;
		}
	}
	part beacon : Beacon;
	part def Tug :> Ship {
		exhibit state lamp {
			entry; then off;
			state off;
			transition first off accept Lit then on;
			state on;
		}
		perform action tow {
			first start;
			then action wait accept after 2 [s];
			then action pull { assign cost := cost + 1.0; }
			then done;
		}
	}
	part tug : Tug;
	analysis def Quote {
		subject s : Ship;
		in tax : Real;
		out total : Real = s.cost * (1.0 + tax);
	}
}`

// heldQuoteTable is the table the sequential form printed for a sweep of Quote over a
// held object fresh from %instantiate, before rows ran in contexts of their own.
var heldQuoteTable = []string{"3 run(s)", "0.0 | 5.0 |", "0.25 | 6.25 |", "0.5 | 7.5 |"}

// A sweep over a held object fresh from %instantiate whose type exhibits a state
// machine, or performs an action, prints on one job and on eight the table the
// sequential form printed, and leaves the held object as it was.
func TestSweepOverAFreshObjectRunningABehaviourReadsAsTheSequentialFormDid(t *testing.T) {
	s := loadSource(t, heldBehaviourModel)
	run(t, s, "%instantiate Held::beacon")
	run(t, s, "%instantiate Held::tug")
	for _, object := range []string{"Held::beacon", "beacon", "#1", "Held::tug", "tug", "#3"} {
		sweepsAlike(t, s, "%sweep Held::Quote "+object+" tax=0.0..0.5:0.25", heldQuoteTable...)
	}
	wants(t, run(t, s, "%features Held::beacon"), "ID: 1", "cost = 5.0", "blinking: exhibited state machine, current state off")
	wants(t, run(t, s, "%features Held::tug"), "ID: 3", "cost = 5.0", "tow: performed action, waiting")
}

// A held object whose performed action has moved — the clock advanced to its wait, the
// action gone on to write a feature and complete — is swept from an image of that
// state: each row reads the written feature, on one job and on eight, and the held
// object is byte for byte as it was. Advanced short of the wait, the action still
// waits in every row, at the instant the held clock stood at.
func TestSweepOverAMovedPerformedActionRunsOnItsImage(t *testing.T) {
	s := loadSource(t, heldBehaviourModel)
	run(t, s, "%instantiate Held::tug")
	wants(t, run(t, s, "%state tug"), `Debugging state machine "lamp" exhibited by object #1`)
	wants(t, run(t, s, "%advance 1"), "Advanced to 1.0", "t=2.0: action tow of object #1")
	before := run(t, s, "%features Held::tug all json")
	sweepsAlike(t, s, "%sweep Held::Quote tug tax=0.0..0.5:0.25", heldQuoteTable...)
	if after := run(t, s, "%features Held::tug all json"); after != before {
		t.Errorf("the sweeps left the held tug as\n%s\nwas\n%s", after, before)
	}

	wants(t, run(t, s, "%advance 1"), "Advanced to 2.0", "Action steps taken: 3")
	wants(t, run(t, s, "%features Held::tug"), "cost = 6.0", "tow: performed action, completed")
	before = run(t, s, "%features Held::tug all json")
	held := s.heldIDs()
	for _, object := range []string{"Held::tug", "#1"} {
		sweepsAlike(t, s, "%sweep Held::Quote "+object+" tax=0.0..0.5:0.25", "3 run(s)", "0.0 | 6.0 |", "0.25 | 7.5 |", "0.5 | 9.0 |")
	}
	if after := run(t, s, "%features Held::tug all json"); after != before {
		t.Errorf("the sweeps left the held tug as\n%s\nwas\n%s", after, before)
	}
	if ids := s.heldIDs(); !slices.Equal(ids, held) {
		t.Errorf("the sweeps changed the session's held objects: %v, was %v", ids, held)
	}
	wants(t, run(t, s, "%current"), "Current state: off")
	wants(t, run(t, s, "%analysis Held::Quote(tax=0.5) tug"), "total = 9.0")
}
