// The entry action of an exhibited state is the action object the state holds,
// whether it is bodiless and untyped or declares a feature of its own, on the
// model's part and on a constructed one alike: a nested action usage is a
// performance its owner holds (SysML v2 §7.16.2, §7.18.2).
package test {
	private import ScalarValues::*;

	state def M;
	part def P {
		exhibit state sm : M { entry action e; then idle; state idle; state busy; transition idle then busy; }
	}
	part def Q {
		state sm : M { entry action e { attribute k : Integer = 1; } state idle; state busy; transition idle then busy; }
	}
	part def Bench {
		part p : P;
		part q : Q;
		attribute exhibitedEntry = p.sm.e;
		attribute declaredEntryK = q.sm.e.k;
	}
}
