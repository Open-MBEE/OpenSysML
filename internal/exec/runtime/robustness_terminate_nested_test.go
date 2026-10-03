package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessTerminateNestedFlowTarget pins which performances a `terminate`
// in a loop body names: those of its own loop body's performance, never another's.
func TestRuntimeRobustnessTerminateNestedFlowTarget(t *testing.T) {
	t.Run("terminate_in_a_loop_body_of_its_own_ended_performance", testTerminateInALoopBodyOfItsOwnEndedPerformance)
	t.Run("terminate_in_a_loop_body_leaves_other_loop_performances", testTerminateInALoopBodyLeavesOtherLoopPerformances)
	t.Run("terminate_in_a_loop_body_before_its_node_began", testTerminateInALoopBodyBeforeItsNodeBegan)
}

// executeLibraryActionSource executes the named action of src over the standard library.
func executeLibraryActionSource(t *testing.T, name, src string) (map[string]Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefAction)
	if sym == nil {
		t.Fatalf("action %s not found", name)
	}
	return ctx.ExecuteAction(sym)
}

// loopPerformancesHost reaches the loop node with three tokens a step apart, each
// performance of its body running slow (and the first two napping in it) before body.
func loopPerformancesHost(slow, body string) string {
	return `package test {
		private import ScalarValues::*;
		private import ISQ::*;
		private import SI::*;
		action host {
			out attribute entered : Integer = 0;
			out attribute napped : Integer = 0;
			out attribute later : Integer = 0;
			fork split;
			action pre;
			action pre1;
			action pre2;
			succession first start then split;
			succession first split then gate;
			succession first split then pre;
			succession first pre then gate;
			succession first split then pre1;
			succession first pre1 then pre2;
			succession first pre2 then gate;
			merge gate;
			then loop {
				action slow {
					assign entered := entered + 1;
					` + slow + `
				}
				` + body + `
			} until true;
			then done;
		}
	}`
}

const nappingSlow = `if entered <= 2 {
						action inner {
							first start;
							then action nap accept after 10 [s];
							then done;
						}
						assign napped := napped + 1;
					}`

// testTerminateInALoopBodyOfItsOwnEndedPerformance: the third loop performance's
// slow has ended when its body runs `terminate slow;`, while the other two loop
// performances' slows are still paused. Those are not its own, so it names an ended
// performance, which TerminateAction cannot end during its performance.
func testTerminateInALoopBodyOfItsOwnEndedPerformance(t *testing.T) {
	_, err := executeLibraryActionSource(t, "host", loopPerformancesHost(nappingSlow, `if entered == 3 {
					terminate slow;
				}
				assign later := later + 1;`))
	if !errors.Is(err, ErrPerformanceEnded) {
		t.Fatalf("error = %v, want ErrPerformanceEnded", err)
	}
	if !strings.Contains(err.Error(), "slow") {
		t.Fatalf("error = %v, want it to name slow", err)
	}
}

// testTerminateInALoopBodyLeavesOtherLoopPerformances: the third loop performance's
// slow ends itself from a watch due before the other two naps end; those two are
// another loop performance's and nap on to completion.
func testTerminateInALoopBodyLeavesOtherLoopPerformances(t *testing.T) {
	values, err := executeLibraryActionSource(t, "host", loopPerformancesHost(nappingSlow+` else {
						action guard {
							first start;
							then fork f;
							succession first f then nap;
							succession first f then watch;
							action nap accept after 10 [s];
							action watch accept after 5 [s];
							succession first watch then stop;
							action stop { terminate slow; }
						}
						assign napped := napped + 10;
					}`, `assign later := later + 1;`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertIntOutput(t, values, "entered", 3)
	assertIntOutput(t, values, "napped", 2)
	assertIntOutput(t, values, "later", 3)
}

// testTerminateInALoopBodyBeforeItsNodeBegan: `terminate slow;` ordered before slow
// in a loop body has no performance of slow to end, its own or another's.
func testTerminateInALoopBodyBeforeItsNodeBegan(t *testing.T) {
	_, err := executeActionSource(t, "host", `package test {
		private import ScalarValues::*;
		action host {
			out attribute x : Integer = 0;
			first start;
			then loop {
				terminate slow;
				then action slow { assign x := 1; }
			} until true;
			then done;
		}
	}`)
	if !errors.Is(err, ErrTerminateTarget) {
		t.Fatalf("error = %v, want ErrTerminateTarget", err)
	}
}
