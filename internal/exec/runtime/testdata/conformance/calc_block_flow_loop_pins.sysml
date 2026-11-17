// A loop body of a calculation connecting the pins of its action nodes: each
// iteration performs the nodes and the flow carries `make.o` to `take.p`.
package test {
	private import ScalarValues::*;

	calc piped {
		in n : Integer;
		attribute x : Integer := 0;
		for i in 1..2 {
			action make { out o : Integer = n + i; }
			action take { in p : Integer; assign x := x + p; }
			flow make.o to take.p;
		}
		return : Integer = x;
	}
}
