// A `perform` written in a loop body is a node of the block's own flow, so it
// runs once per iteration, in succession with the statements around it. The
// performed action's `in i` binds from the loop variable of the iteration
// performing it, and its `out doubled` comes back to the body, so the sum is
// 2+4+6 = 12 and the last performance leaves doubled at 6.
package test {
	private import ScalarValues::*;

	action run {
		attribute total : Integer = 0;
		attribute doubled : Integer = 0;

		first start;

		action iterate {
			for i in 1..3 {
				perform Bump;
				assign total := total + doubled;
			}
		}

		done;

		succession first start then iterate;
		succession first iterate then done;
	}
	action def Bump {
		in i : Integer;
		out doubled : Integer;

		first start;

		action compute {
			assign doubled := i * 2;
		}

		done;

		succession first start then compute;
		succession first compute then done;
	}

}
