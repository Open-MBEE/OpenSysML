// A named terminate node (`action halt terminate a;`) ends the performance it
// names, as the bare `terminate a;` statement does: the branch after it never
// runs, so `sum` is 1+2+3 plus 1 per iteration of b, never the 1000s.
package test {
	private import ScalarValues::*;

	action run {
		attribute sum : Integer = 0;
		first start;
		action iterate {
			for i in 1..3 {
				action a {
					assign sum := sum + i;
					then action halt terminate a;
					then assign sum := sum + 1000;
				}
				action b { assign sum := sum + 1; }
				succession a then b;
			}
		}
		done;
		succession first start then iterate;
		succession first iterate then done;
	}
}
