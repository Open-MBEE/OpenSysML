// A calculation body stating a fork and a join: both branches run, the join waits
// for both, and the result is computed once the flow has ended.
package test {
	private import ScalarValues::*;

	calc forked {
		in n : Integer;
		attribute x : Integer := 0;
		attribute y : Integer := 0;
		first start then f;
		fork f;
			then left;
			then right;
		action left { assign x := n; }
		action right { assign y := n + 1; }
		join j;
		first left then j;
		first right then j;
		first j then done;
		return : Integer = x + y;
	}
}
