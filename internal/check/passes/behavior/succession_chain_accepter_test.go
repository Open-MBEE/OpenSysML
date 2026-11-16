package behavior_test

import "testing"

// A succession end chaining through a part (SysML v2 §7.17.4) and the accepter
// rule (§8.3.18.9 validateTransitionUsageTriggerActions) judge the same chain
// independently: the end is a legal occurrence, the accepter needs a state.
func TestChainedSuccessionEndBesideAccepterRule(t *testing.T) {
	const vehicle = `
	item def Sig;
	part def Vehicle {
		action close;
		state machine {
			entry; then off;
			state off;
			accept Sig then on;
			state on;
		}
	}`
	t.Run("end reaching a state with an accepter transition", func(t *testing.T) {
		got := analyzeTransitions(t, `package test {`+vehicle+`
	action def A {
		part v : Vehicle;
		action a;
		first start then v.close;
		first v.close then v.machine;
		first v.machine then a;
	}
}`)
		if len(got) != 0 {
			t.Fatalf("got %+v, want the chained ends accepted", got)
		}
	})
	t.Run("the chained action as an accepter's source in an action body", func(t *testing.T) {
		wantOnlyAccepterRule(t, `package test {`+vehicle+`
	action def A {
		part v : Vehicle;
		action a;
		first start then v.close;
		transition first v.close accept Sig then a;
	}
}`, "accept Sig")
	})
	t.Run("the chained action as an accepter's source in a state body", func(t *testing.T) {
		wantOnlyAccepterRule(t, `package test {`+vehicle+`
	state def M {
		part v : Vehicle;
		entry; then idle;
		state idle;
		transition first v.close accept Sig then idle;
	}
}`, "accept Sig")
	})
}
