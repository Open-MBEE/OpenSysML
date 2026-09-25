// A nested action declared in a loop body is a node of the block's own flow: it
// is reached in the order it was declared, runs, and the flow goes on to the
// statement after it. It declares parameters and a nested action of its own, two
// levels deep, so each iteration computes i*10 + 1 and adds it to the sum:
// 11 + 21 + 31 = 63, with scaled left at the last iteration's 30.
package test {
	private import ScalarValues::*;

	action run {
		attribute sum : Integer = 0;
		attribute scaled : Integer = 0;

		first start;

		action iterate {
			for i in 1..3 {
				assign sum := sum + 1;
				action step {
					in factor = 10;
					assign scaled := i * factor;
					action inner {
						assign sum := sum + scaled;
					}
				}
			}
		}

		done;

		succession first start then iterate;
		succession first iterate then done;
	}
}
