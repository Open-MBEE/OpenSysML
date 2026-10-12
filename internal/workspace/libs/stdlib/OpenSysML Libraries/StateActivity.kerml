library package StateActivity {
	doc
	/*
	 * NON-NORMATIVE. This package is an OpenSysML extension, not part of the OMG KerML
	 * or SysML standard library: it declares a readable Boolean that tells whether a
	 * state usage is in the active configuration of its state machine, which neither
	 * States.sysml (StateAction, stateActions, substates) nor StatePerformances.kerml
	 * declares. The vendored OMG files are unmodified.
	 *
	 * A model that reads it writes `import StateActivity::*;` and then names the feature
	 * on a state usage, `fill.isActive`. The import is what resolves the name: a feature
	 * declared `featured by` a type is read by OpenSysML as a feature of every usage
	 * conforming to that type reached by a feature chain written where the feature is
	 * visible, a reading no other tool makes. Without the import the member is unresolved,
	 * exactly as in a conforming tool, and `nonstandard-notation` reports the import.
	 */

	private import ScalarValues::Boolean;
	private import States::StateAction;

	derived feature isActive : Boolean[1] featured by StateAction {
		doc
		/*
		 * True exactly while the featuring state is in the active configuration of its
		 * state machine when the expression reading it is evaluated: a composite state
		 * while any of its substates is, a state of a parallel region together with the
		 * states of its sibling regions, and a state during its own entry and exit
		 * behaviors. False during a transition's effect, whose source has been exited and
		 * whose target not yet entered, and for a state of a machine that has not started,
		 * has ended or is not executing. Derived: no behavior writes it.
		 */
	}
}
