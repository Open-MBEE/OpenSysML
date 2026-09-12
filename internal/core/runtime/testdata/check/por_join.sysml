// Join convergence: three branches of unequal length count their arrivals; the
// join fires once, after the slowest, and what follows sees every arrival.
package test {
	private import ScalarValues::*;

	action gather {
		attribute arrived : Integer = 0;
		attribute seen : Integer = -1;
		attribute last : Integer = 0;
		first start;
		fork split;
		action a { assign arrived := arrived + 1; assign last := 1; }
		action b1;
		action b2 { assign arrived := arrived + 1; assign last := 2; }
		action c1;
		action c2;
		action c3 { assign arrived := arrived + 1; assign last := 3; }
		join sync;
		action after { assign seen := arrived; }
		done;
		succession first start then split;
		succession first split then a;
		succession first split then b1;
		succession first split then c1;
		succession first b1 then b2;
		succession first c1 then c2;
		succession first c2 then c3;
		succession first a then sync;
		succession first b2 then sync;
		succession first c3 then sync;
		succession first sync then after;
		succession first after then done;
	}
}
