// An action whose loop body declares a node, so the debugger can break on a
// node a block performs.
package Debug {
	private import ScalarValues::*;

	action count {
		attribute total : Integer = 0;

		first start;
		then action iterate {
			for i in 1..3 {
				action add { in n : Integer = i; assign total := total + n; }
			}
		}
		then done;
	}
}
