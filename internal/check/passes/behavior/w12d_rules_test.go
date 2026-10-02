package behavior_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/behavior"
)

// A parallel state orders nothing, so a succession written in one is an error
// the parser must reach for (SysML v2 §7.19.3).
func TestW12DParallelStateSuccession(t *testing.T) {
	got := transitionDiags(t, `package test {
	state def S1 parallel {
		state a;
		then b;
		state b;
	}
}`)
	if len(got) != 1 || got[0].Code != behavior.CodeParallelStateTransition {
		t.Fatalf("got %+v, want one %s", got, behavior.CodeParallelStateTransition)
	}
	if got[0].Message != behavior.MsgParallelStateTransition {
		t.Errorf("message = %q, want %q", got[0].Message, behavior.MsgParallelStateTransition)
	}
}

// The same rule holds for a state written as a usage, and for the standard
// `transition first … then …` spelling.
func TestW12DParallelStateUsageTransition(t *testing.T) {
	got := transitionDiags(t, `package test {
	state s1 parallel {
		state a;
		state b;
		transition first a then b;
	}
}`)
	if len(got) != 1 || got[0].Code != behavior.CodeParallelStateTransition {
		t.Fatalf("got %+v, want one %s", got, behavior.CodeParallelStateTransition)
	}
}

// Orthogonality is a property of each state, so the rule reaches a parallel
// state nested in a machine and leaves its non-parallel siblings alone.
func TestW12DNestedParallelState(t *testing.T) {
	got := transitionDiags(t, `package test {
	state def M {
		state outer {
			state s parallel {
				state a;
				state b;
				transition first a then b;
			}
			state ordered {
				state c;
				state d;
				transition first c then d;
			}
		}
	}
}`)
	if len(got) != 1 || got[0].Code != behavior.CodeParallelStateTransition {
		t.Fatalf("got %+v, want one %s", got, behavior.CodeParallelStateTransition)
	}
}

// An accepter waits while its source is performed, so a triggered transition
// out of an action names the accepter rule at the trigger.
func TestW12DAccepterSourceMustBeAState(t *testing.T) {
	src := `package test {
	state def S2 {
		entry action init;
		transition init accept A then S2_1;
		state S2_1;
	}
	}`
	got := transitionDiags(t, src)
	if len(got) != 1 || got[0].Code != behavior.CodeEntryTransitionShape {
		t.Fatalf("got %+v, want one %s", got, behavior.CodeEntryTransitionShape)
	}
	if !strings.Contains(got[0].Message, behavior.MsgAccepterSourceNotState) {
		t.Errorf("message = %q, want it to cite %q", got[0].Message, behavior.MsgAccepterSourceNotState)
	}
}

// A triggered transition out of a state is legal, whichever spelling declares
// the source.
func TestW12DAccepterSourceStateIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def S2 {
		state S2_0;
		state S2_1;
		transition S2_0 accept A then S2_1;
	}
}`)
}
