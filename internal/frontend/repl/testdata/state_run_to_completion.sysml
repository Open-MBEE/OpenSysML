// A machine redefining isRunToCompletion away from the library default: the
// debugger refuses to start it with the lowering error, not a run under the default.
package Refused {
	attribute def Go;

	state def Machine {
		attribute :>> isRunToCompletion = false;
		entry; then idle;
		state idle;
		accept Go then done;
		state done;
	}
}
