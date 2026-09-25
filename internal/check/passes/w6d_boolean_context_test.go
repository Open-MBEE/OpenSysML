package passes

import "testing"

// A condition naming a feature typed by a structure can hold no Boolean, so it
// is reported statically rather than left to the executor. The pinned pilot
// reports `constraint c { p }` too, as a "Bound features should have conforming
// types" warning.
func TestW6DNonScalarConditionIsReported(t *testing.T) {
	wantOneDiag(t, `package P {
		part def Engine;
		part e : Engine;
		constraint c { e }
	}`, "constraint expression must be Boolean, found Engine")
}

func TestW6DNonScalarConditionInEveryBooleanContext(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{"require", `requirement r { require constraint { e } }`,
			"constraint expression must be Boolean, found Engine"},
		{"assume", `requirement r { assume constraint { e } }`,
			"constraint expression must be Boolean, found Engine"},
		{"guard", `state def S { state a; state b; transition first a if e then b; }`,
			"transition guard must be Boolean, found Engine"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantOneDiag(t, `package P {
				part def Engine;
				part e : Engine;
				`+tc.src+`
			}`, tc.want)
		})
	}
}

// An enumeration literal is no Boolean either, and it is named rather than
// typed, so the check has to reach the enumeration definition through it.
func TestW6DEnumerationLiteralConditionIsReported(t *testing.T) {
	wantOneDiag(t, `package P {
		enum def Level { red; green; }
		constraint c { Level::red }
	}`, "constraint expression must be Boolean, found Level")
}

// A condition may name a constraint or a calc, whose result is (or may be) a
// Boolean, and a feature typed by `Anything` may hold one.
func TestW6DConditionNamingABehaviorIsNotReported(t *testing.T) {
	wantNoDiags(t, `package P {
		constraint def Positive { true }
		constraint inner : Positive;
		constraint c { inner }
	}`)
}

func TestW6DConditionTypedByABooleanSpecializationIsNotReported(t *testing.T) {
	wantNoDiags(t, `package P {
		attribute def Flag :> ScalarValues::Boolean;
		attribute f : Flag;
		constraint c { f }
	}`)
}

// A condition whose value comes from a calc is typed by the calc's result.
func TestW6DConditionFromANonBooleanCalcIsReported(t *testing.T) {
	wantOneDiag(t, `package P {
		part def Engine;
		part e : Engine;
		calc def MakeEngine { return : Engine = e; }
		constraint c { MakeEngine() }
	}`, "constraint expression must be Boolean, found Engine")
}

func TestW6DConditionFromABooleanCalcIsNotReported(t *testing.T) {
	wantNoDiags(t, `package P {
		calc def IsFine { return : ScalarValues::Boolean = true; }
		constraint c { IsFine() }
	}`)
}
