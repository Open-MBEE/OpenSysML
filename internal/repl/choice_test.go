package repl

import "testing"

const choiceForkSource = `
package Debug {
	private import ScalarValues::*;
	action tally {
		attribute leftCount : Integer = 0;
		attribute rightCount : Integer = 0;

		first start;
		fork split;
		action left { assign leftCount := leftCount + 1; }
		action right { assign rightCount := rightCount + 10; }
		join sync;
		done;

		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}
}
`

// A debugger step that picked among unordered tokens says so in one line, and
// points at %trace when the choices themselves are not being shown.
func TestStepReportsChoicePoints(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	run(t, s, "%action tally")

	// start -> split: one token, nothing to choose.
	rejects(t, run(t, s, "%step"), "choice point")
	// split forks: still one token acted.
	rejects(t, run(t, s, "%step"), "choice point")
	// left and right both act: the executor ordered them.
	wants(t, run(t, s, "%step"), "✓ Step complete", "  1 choice point; %trace on to see them")
}

// With tracing on the choice is in the trace, so the summary drops the hint.
func TestStepChoiceSummaryWithTraceOn(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	run(t, s, "%trace on")
	run(t, s, "%action tally")
	run(t, s, "%step")
	run(t, s, "%step")
	out := run(t, s, "%step")
	wants(t, out, "choice step 3: tokens 2@left, 3@right (unordered; took 3@right first)", "  1 choice point")
	rejects(t, out, "%trace on to see them")
}

// %continue counts the choices of the run it completed, and the plural form
// when there were several.
func TestContinueReportsChoicePoints(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	run(t, s, "%action tally")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "  1 choice point; %trace on to see them")

	s = loadSource(t, `
package Debug {
	private import ScalarValues::*;
	action race {
		attribute x : Integer = 0;
		first start;
		fork split;
		action left { assign x := 1; }
		action right { assign x := 2; }
		join sync;
		done;
		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}
}
`)
	run(t, s, "%action race")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "  2 choice points; %trace on to see them")
}

// A run that fails still reports the choices it made before failing, so the
// error can be told from a scheduling artefact.
func TestContinueReportsChoicePointsBeforeFailure(t *testing.T) {
	s := loadSource(t, `
package Debug {
	private import ScalarValues::*;
	action crash {
		attribute x : Integer = 0;
		attribute zero : Integer = 0;
		first start;
		fork split;
		action left { assign x := 1; }
		action right { assign x := 2; }
		join sync;
		action divide { assign x := x / zero; }
		done;
		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then divide;
		succession first divide then done;
	}
}
`)
	run(t, s, "%action crash")
	wants(t, run(t, s, "%continue"), "error: execution failed:", "  2 choice points; %trace on to see them")
}

// %advance on a state machine reports a transition conflict it dispatched
// through; an event with one enabled transition reports none.
func TestAdvanceReportsChoicePoints(t *testing.T) {
	s := loadSource(t, `
package Debug {
	private import ScalarValues::*;
	attribute def Go;
	attribute def Stop;
	state def Dispatcher {
		attribute level : Integer = 8;
		entry; then idle;
		state idle;
		state low;
		state high;
		state halted;
		transition first idle accept Go if level > 5 then low;
		transition first idle accept Go if level > 7 then high;
		transition first low accept Stop then halted;
	}
	part def Router {
		exhibit state dispatch : Dispatcher;
	}
	part router : Router;
}
`)
	wants(t, run(t, s, "%instantiate router"), "✓ Created instance")
	wants(t, run(t, s, "%state router"), "Current state: idle")
	run(t, s, "%send Go")
	wants(t, run(t, s, "%advance 1"), "Current state: low", "  1 choice point; %trace on to see them")
	run(t, s, "%send Stop")
	out := run(t, s, "%advance 1")
	wants(t, out, "Current state: halted")
	rejects(t, out, "choice point")
}
