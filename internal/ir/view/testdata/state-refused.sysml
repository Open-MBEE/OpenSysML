// Deliberately invalid: `right` scopes its run to `left::off`, no ancestor of it — the error is the point, as the rendering refuses the machine the way runtime lowering does.
// A state rendering with a scope outside the redefining state exercises the
// same typed non-ancestor refusal as runtime lowering.
package Machines {
	alias Scope for Occurrences::Occurrence::runToCompletionScope;

	state def Relaxed parallel {
		state left {
			state off;
		}
		state right {
			attribute :>> Scope = left::off;
			state on;
		}
	}
}

package MachineViews {
	private import StandardViewDefinitions::*;

	view relaxedStates : StateTransitionView {
		expose Machines::Relaxed;
	}
}
