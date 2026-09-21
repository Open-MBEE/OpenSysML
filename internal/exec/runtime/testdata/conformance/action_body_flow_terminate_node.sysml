package test {
	private import ScalarValues::*;

	action run {
		attribute sum : Integer = 0;
		first start;
		action iterate {
			for i in 1..3 {
				action a {
					assign sum := sum + i;
					terminate;
					assign sum := sum + 1000;
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
