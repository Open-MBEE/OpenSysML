package test {
	private import ScalarValues::*;

	// A loop body stating a flow (`a then b`) performs the assignment no succession
	// reaches beside it, once per iteration.
	action host {
		attribute x : Integer = 0;
		first start;
		then action iterate {
			for i in 1..2 {
				action a;
				action b;
				assign x := x + 1;
				succession a then b;
			}
		}
		then done;
	}
}
