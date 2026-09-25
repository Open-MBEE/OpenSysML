package passes

import "testing"

// A guard is a Boolean expression whichever transition form states it (SysML
// 8.3.18.8 validateTransitionFeatureMembershipGuardExpression): a transition
// with a source, one stated by its trigger or guard alone, and the guarded
// successions of an action body, which are transitions too.
func TestTransitionGuardNonBooleanInEveryTransitionForm(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"first-if-then", `state def S { state a; state b; transition first a if "yes" then b; }`, "String"},
		{"if-then", `state def S { state a; if "yes" then b; state b; }`, "String"},
		{"accept-if-then", `item def Sig; state def S { state a; accept Sig if 1 then b; state b; }`, "Natural"},
		{"transition-if-then", `state def S { state a; transition if 2.5 then b; state b; }`, "Rational"},
		{"transition-accept-if-do-then", `item def Sig; state def S { state a; transition accept Sig if "yes" do action e then b; state b; }`, "String"},
		{"nested-state", `state def S { state a { state a1; if "yes" then a2; state a2; } }`, "String"},
		{"state-usage-body", `part def P { state s { state a; state b; transition first a if "yes" then b; } }`, "String"},
		{"action-first-if-then", `action def AD { action a; action b; first a if "x" then b; }`, "String"},
		{"action-decision-if-then", `action def AD { action a; action b; first start then a; then decide; if "x" then b; else a; }`, "String"},
		{"action-first-decide-if-then", `action def AD { action a; action b; decide d; first a then d; first d if "x" then b; }`, "String"},
		{"action-usage-body", `part def P { action act { action a; action b; first a if 3 then b; } }`, "Natural"},
		{"action-succession-if-then", `action def AD { action a; action b; succession s first a if "x" then b; }`, "String"},
		{"integer-attribute", `attribute n : ScalarValues::Integer; action def AD { action a; action b; first a if n then b; }`, "Integer"},
		{"integer-calc", `calc def C { return : ScalarValues::Integer = 1; } action def AD { action a; action b; first a if C() then b; }`, "Integer"},
		{"chain-to-integer", `part def E { attribute n : ScalarValues::Integer; } part e : E; action def AD { action a; action b; first a if e.n then b; }`, "Integer"},
		{"part-typed", `part def E; part e : E; action def AD { action a; action b; first start then a; then decide; if e then b; else a; }`, "E"},
		{"enumeration-literal", `enum def Color { red; green; } action def AD { action a; action b; first a if Color::red then b; }`, "Color"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantOneDiag(t, `package P { `+tc.src+` }`, "transition guard must be Boolean, found "+tc.want)
		})
	}
}

// Boolean-valued guards stay silent: literals, Boolean and Boolean-subtyped
// features, Boolean operators, constraints and Boolean calcs, chains ending in
// a Boolean, and features whose type is unknown or declared by no type at all.
func TestTransitionGuardBooleanFormsAreSilent(t *testing.T) {
	wantNoDiags(t, `package P {
		attribute def Flag :> ScalarValues::Boolean;
		part def E { attribute ok : ScalarValues::Boolean; }
		part e : E;
		constraint def K { true }
		calc def CB { return : ScalarValues::Boolean = true; }
		item def Sig;
		state def S {
			attribute ok : ScalarValues::Boolean;
			attribute flag : Flag;
			attribute optional : ScalarValues::Boolean[0..1];
			attribute untyped;
			constraint k : K;
			state a; state b;
			transition first a if true then b;
			transition first a if ok then b;
			transition first a if flag then b;
			transition first a if optional then b;
			transition first a if untyped then b;
			transition first a if null then b;
			transition first a if not ok and (1 < 2 or flag == true) then b;
			transition first a if k then b;
			transition first a if CB() then b;
			transition first a if e.ok then b;
			if e.ok then b;
			accept Sig if ok then b;
			transition if flag then b;
		}
		action def AD {
			attribute ok : ScalarValues::Boolean;
			action a; action b; action c;
			first a if ok then b;
			first b if e.ok and not ok then c;
			first start then a;
			then decide;
			if ok then b;
			if CB() then c;
			else a;
		}
	}`)
}
