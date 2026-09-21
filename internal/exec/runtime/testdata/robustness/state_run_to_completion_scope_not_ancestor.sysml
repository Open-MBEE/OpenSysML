// A scope redefinition naming a sibling rather than an ancestor is rejected
// with a typed non-ancestor refusal.
package test {
	state def Machine parallel {
		state left {
			entry; then l;
			state l;
		}
		state right {
			ref :>> runToCompletionScope = Machine::left::l;
			entry; then r;
			state r;
		}
	}
}
