package test {
	private import ScalarValues::*;

	action run {
		attribute log : Integer = 0;

		first start;

		action iterate {
			for i in 1..2 {
				action b { assign log := log * 10 + 2; }
				action a { assign log := log * 10 + 1; }
				succession a then b;
			}
		}

		done;

		succession first start then iterate;
		succession first iterate then done;
	}
}
