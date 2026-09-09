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

// One event enabling a transition in each of two orthogonal regions is a
// region-order choice under the default policy: counted in the summary line,
// and shown in the trace when it is on.
func TestAdvanceReportsRegionOrderChoice(t *testing.T) {
	src := `
package Debug {
	private import ScalarValues::*;
	attribute def Go;
	state def Machine {
		attribute last : Integer = 0;
		entry; then work;
		state work parallel {
			state a { entry; then a1; state a1; state a2; transition first a1 accept Go do assign last := 1 then a2; }
			state b { entry; then b1; state b1; state b2; transition first b1 accept Go do assign last := 2 then b2; }
		}
	}
	part def Rig {
		exhibit state run : Machine;
	}
	part rig : Rig;
}
`
	s := loadSource(t, src)
	wants(t, run(t, s, "%instantiate rig"), "✓ Created instance")
	wants(t, run(t, s, "%state rig"), "Current state: a1 | b1")
	run(t, s, "%send Go")
	out := run(t, s, "%advance 1")
	wants(t, out, "Current state: a2 | b2", "  1 choice point; %trace on to see them")
	rejects(t, out, "region order")

	s = loadSource(t, src)
	run(t, s, "%trace on")
	run(t, s, "%instantiate rig")
	run(t, s, "%state rig")
	run(t, s, "%send Go")
	out = run(t, s, "%advance 1")
	wants(t, out,
		"choice on accept Go: states a1, b1 react (unordered; took a1 first)",
		"  1 choice point")
	rejects(t, out, "%trace on to see them")
}

// A guard read only to report a choice that could not be evaluated is counted in
// the summary line beside the choices, and shown in the trace when it is on.
func TestStepReportsUnevaluableGuards(t *testing.T) {
	src := `
package Debug {
	private import ScalarValues::*;
	action route {
		attribute level : Integer = 75;
		attribute handler : Integer = 0;
		first start;
		then decide select;
			if level > 50 then warn;
			if 1 / (level - 75) > 0 then alarm;
		action warn { assign handler := 1; }
		then done;
		action alarm { assign handler := 2; }
		then done;
	}
	action mixed {
		attribute level : Integer = 75;
		attribute handler : Integer = 0;
		first start;
		then decide select;
			if level > 50 then warn;
			if level > 70 then alarm;
			if 1 / (level - 75) > 0 then halt;
		action warn { assign handler := 1; }
		then done;
		action alarm { assign handler := 2; }
		then done;
		action halt { assign handler := 3; }
		then done;
	}
}
`
	s := loadSource(t, src)
	run(t, s, "%action route")
	rejects(t, run(t, s, "%step"), "guard not evaluable")
	out := run(t, s, "%step")
	wants(t, out, "✓ Step complete", "  1 guard not evaluable; %trace on to see them")
	rejects(t, out, "choice point")

	s = loadSource(t, src)
	run(t, s, "%trace on")
	run(t, s, "%action route")
	run(t, s, "%step")
	out = run(t, s, "%step")
	wants(t, out,
		"unevaluable guard step 2: decision select branch 2->alarm: division by zero (not selected)",
		"  1 guard not evaluable")
	rejects(t, out, "%trace on to see them")

	s = loadSource(t, src)
	run(t, s, "%action route")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "  1 guard not evaluable; %trace on to see them")

	run(t, s, "%action mixed")
	wants(t, run(t, s, "%continue"), "✓ Action completed",
		"  1 choice point; 1 guard not evaluable; %trace on to see them")
}
