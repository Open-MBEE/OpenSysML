// A bodiless, untyped entry action of an exhibited state is held as nested action
// usages of a part are held today — undetermined at model level, an uninitialized
// feature on a constructed object — while an entry action declaring a feature of
// its own is the action object the state holds. Pinned so that holding nested
// action and state usages as unstarted performance objects moves this case.
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
