package test {
	private import ScalarValues::*;

	action def ConcurrentArrivals {
		attribute c : Integer = 0;

		first start then split;
		fork split;
		action left;
		action right;
		merge joined;
		action p;
		action a[2] { assign c := c + 1; }

		succession first split then left;
		succession first split then right;
		succession first left then joined;
		succession first right then joined;
		succession first joined then p;
		succession first [1] p then [2] a;
		succession first a then done;
	}
}
