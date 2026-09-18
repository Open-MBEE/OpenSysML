package behavior_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes/behavior"
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
	if len(got) != 1 || got[0].Code != behavior.CodeAccepterSourceNotState {
		t.Fatalf("got %+v, want one %s", got, behavior.CodeAccepterSourceNotState)
	}
	if got[0].Message != behavior.MsgAccepterSourceNotState {
		t.Errorf("message = %q, want %q", got[0].Message, behavior.MsgAccepterSourceNotState)
	}
	at := src[got[0].Span.Offset : got[0].Span.Offset+got[0].Span.Len]
	if !strings.Contains(at, "A") || strings.Contains(at, "then") {
		t.Errorf("reported at %q, want the trigger", at)
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

// Type::multiplicity is single-valued (KerML 8.3.3.1.1), so a second
// multiplicity member is a validation error, not a syntax error.
func TestW12DOnlyOneMultiplicity(t *testing.T) {
	src := `package Type_Multiplicity {
	classifier C {
		multiplicity subsets Base::zeroOrOne;

		multiplicity subsets Base::zeroToMany;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if w8cCount(msgs, msgOnlyOneMultiplicity) != 1 {
		t.Errorf("want one %q, got %v", msgOnlyOneMultiplicity, msgs)
	}
}

// One multiplicity member is what a type may own, so it reports nothing.
func TestW12DOneMultiplicityIsLegal(t *testing.T) {
	src := `package Type_Multiplicity {
	classifier C {
		multiplicity subsets Base::zeroOrOne;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if w8cCount(msgs, msgOnlyOneMultiplicity) != 0 {
		t.Errorf("want no %q, got %v", msgOnlyOneMultiplicity, msgs)
	}
}

// An end that owns its cross feature inline and also crosses another one
// declares two cross features, and only one is allowed (KerML 8.3.4.5).
func TestW12DDeclaredCrossFeature(t *testing.T) {
	src := `package P {
	class C1 { feature a : C2; }
	class C2 { feature b : C1; }
	assoc A3 {
		end x1 [0..1] feature x : C1 crosses y.b {
			public import y::y1;
		}
		end feature y : C2 crosses x.y1 {
			member feature y1 [0..1] featured by C1;
		}
	}
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if w8cCount(msgs, msgMustBeCrossFeature) != 1 {
		t.Errorf("want one %q, got %v", msgMustBeCrossFeature, msgs)
	}
}

// An end that only crosses does not report the rule.
func TestW12DCrossesAloneIsLegal(t *testing.T) {
	src := `package P {
	class C1 { feature a : C2; }
	class C2 { feature b : C1; }
	assoc A1 {
		end x : C1 crosses y.b;
		end y : C2 crosses x.a;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if w8cCount(msgs, msgMustBeCrossFeature) != 0 {
		t.Errorf("want no %q, got %v", msgMustBeCrossFeature, msgs)
	}
}
