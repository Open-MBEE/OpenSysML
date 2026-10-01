// A machine whose run-to-completion scope names a sibling state remains a
// typed lowering refusal, so the debugger reports the invalid scope.
package Refused {
	alias Scope for Occurrences::Occurrence::runToCompletionScope;

	state def Machine parallel {
		state left {
			state idle;
		}
		state right {
			attribute :>> Scope = left.idle;
			state done;
		}
	}
}
