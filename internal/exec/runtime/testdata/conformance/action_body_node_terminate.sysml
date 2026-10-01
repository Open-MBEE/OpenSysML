package test {
	private import ScalarValues::*;

	action run {
		attribute sum : Integer = 0;
		attribute after : Integer = 0;

		first start;

		action iterate {
			for i in 1..3 {
				action a {
					assign sum := sum + i;
					terminate;
					assign sum := sum + 1000;
				}
				assign after := after + 1;
			}
		}

		done;

		succession first start then iterate;
		succession first iterate then done;
	}
}
