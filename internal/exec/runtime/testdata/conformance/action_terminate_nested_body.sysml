package test {
	private import ScalarValues::*;

	// A `terminate;` in a nested node's body ends that node only: the statement
	// after it does not run, and the parent continues along the node's succession.
	action host {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;
		out attribute z : Integer = 0;

		first start;
		then action c1 {
			assign x := 1;
			terminate;
			assign y := 2;
		}
		then action after { assign z := 3; }
		then done;
	}
}
