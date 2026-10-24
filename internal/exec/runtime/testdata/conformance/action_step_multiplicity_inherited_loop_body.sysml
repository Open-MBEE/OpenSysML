// An inherited exact count on a step in a loop body: `tick` declares `[2]` in
// Base, Derived redefines the loop's owner `worker` without restating it, and
// each of the two passes performs `tick` twice, so `c` ends at 4.
package test {
	private import ScalarValues::*;

	action def Base {
		attribute c : Integer = 0;
		first start then worker;
		action worker {
			attribute i : Integer = 0;
			while i < 2 {
				first start then tick;
				action tick[2] { assign c := c + 1; }
				succession first [*] tick then [1] bump;
				action bump { assign i := i + 1; }
			}
		}
		then done;
	}

	action def Derived :> Base {
		action :>> worker;
	}
}
