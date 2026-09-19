package test {
	private import ScalarValues::*;

	// A `terminate;` in a terminate action usage's body ends the usage with the pins it
	// has assigned so far: `code` reaches `result` through the binding, `other` is never
	// assigned so the enclosing one keeps its value, and the usage still ends the action.
	action stopper {
		out attribute x : Integer = 0;
		out attribute result : Integer = 0;
		out attribute other : Integer = 0;

		bind stop.code = result;

		first start;
		then action a1 { assign x := 1; }
		then stop;
		action stop terminate {
			out code : Integer;
			out other : Integer;
			assign code := 42;
			terminate;
			assign other := 7;
		}
		then action a2 { assign x := 2; }
		then done;
	}
}
