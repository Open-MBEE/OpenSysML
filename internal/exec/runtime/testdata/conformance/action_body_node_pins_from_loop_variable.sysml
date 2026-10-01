package test {
	private import ScalarValues::*;

	action run {
		attribute sum : Integer = 0;
		attribute log : Integer = 0;

		first start;

		action iterate {
			for i in 1..3 {
				action a {
					in factor = i;
					assign sum := sum + factor;
				}
				then action b {
					assign log := log * 10 + i;
				}
				then action c {
					if i == 2 {
						action d {
							assign sum := sum + 100;
						}
					}
				}
			}
		}

		done;

		succession first start then iterate;
		succession first iterate then done;
	}
}
