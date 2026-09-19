package passes

import "testing"

// A value produced by a calc invocation is typed by the calc's result
// parameter, so binding it to an unrelated feature is reported statically. The
// pinned pilot reports the same binding ("Bound features should have conforming
// types").
func TestW6DInvocationResultTypeIsJudged(t *testing.T) {
	wantOneDiag(t, `package P {
		part def Engine;
		part def Wheel;
		part e : Engine;
		calc def MakeEngine { return : Engine = e; }
		part w : Wheel = MakeEngine();
	}`, "cannot bind a value of type Engine to a feature typed by Wheel")
}

func TestW6DInvocationResultConformingIsNotReported(t *testing.T) {
	wantNoDiags(t, `package P {
		part def Engine;
		part def Turbine :> Engine;
		part t : Turbine;
		calc def MakeTurbine { return : Turbine = t; }
		part e : Engine = MakeTurbine();
	}`)
}

// An inherited result parameter types the invocation too, and a calc with no
// result declared leaves the binding unjudged rather than guessed at.
func TestW6DInheritedInvocationResultTypeIsJudged(t *testing.T) {
	wantOneDiag(t, `package P {
		part def Engine;
		part def Wheel;
		part e : Engine;
		calc def MakeEngine { return : Engine = e; }
		calc def MakeEngineAgain :> MakeEngine;
		part w : Wheel = MakeEngineAgain();
	}`, "cannot bind a value of type Engine to a feature typed by Wheel")
}

func TestW6DInvocationWithoutAResultIsNotJudged(t *testing.T) {
	wantNoDiags(t, `package P {
		part def Engine;
		part def Wheel;
		action def Build;
		part w : Wheel = Build();
	}`)
}
