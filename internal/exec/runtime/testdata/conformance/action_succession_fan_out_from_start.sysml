package test {
	private import ScalarValues::*;

	// Two successions out of the start node: each target is performed once.
	action twoFirsts {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		action a assign x := 1;
		action b assign y := 1;

		first start;
		succession first start then a;
		succession first start then b;
	}
}
