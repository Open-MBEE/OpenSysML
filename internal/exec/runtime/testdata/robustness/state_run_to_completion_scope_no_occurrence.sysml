// A scope redefinition that names no occurrence is rejected with a typed refusal.
package test {
	state def Machine {
		ref :>> runToCompletionScope = Missing;
		entry; then idle;
		state idle;
	}
}
