package test {
	private import ScalarValues::*;

	// A terminate action usage's out pin is bound as the usage completes, before it ends
	// the action: `code`, assigned in stop's body, reaches `result` through the binding,
	// as does the output an action it performs returns; a2 does not run.
	action def Report {
		out issued : Integer;
		first start;
		then action issue { assign issued := 100; }
		then done;
	}

	action stopper {
		out attribute x : Integer = 0;
		out attribute result : Integer = 0;
		out attribute issued : Integer = 0;

		bind stop.code = result;

		first start;
		then action a1 { assign x := 1; }
		then stop;
		action stop terminate {
			out code : Integer;
			perform action report : Report;
			assign code := 42;
		}
		then action a2 { assign x := 2; }
		then done;
	}
}
