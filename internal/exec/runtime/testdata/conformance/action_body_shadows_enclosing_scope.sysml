// Giving a body statement the scope it was written in must not let an enclosing
// declaration outrank a live value: the action's own `h` masks the package's
// `h`, and a block-local `h` masks the action's while the block is entered.
package test {
	public import ScalarValues::*;

	attribute h : Real = 999.0;

	action shadow {
		attribute h : Real = 1.0;
		attribute seen : Real = 0.0;
		attribute inBlock : Real = 0.0;
		attribute afterBlock : Real = 0.0;

		first start;

		action body {
			assign seen := h;
			if h < 2.0 {
				attribute h : Real = 50.0;
				assign inBlock := h;
			}
			assign afterBlock := h;
		}

		done;

		succession first start then body;
		succession first body then done;
	}
}
