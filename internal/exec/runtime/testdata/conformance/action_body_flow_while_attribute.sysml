package test {
	private import ScalarValues::*;

	action run {
		attribute i : Integer = 1;
		attribute total : Integer = 0;
		first start;
		action iterate {
			while i <= 3 {
				attribute step : Integer = i * 2;
				action a { out v : Integer; assign v := step; }
				action b {
					assign total := total + a.v;
					assign i := i + 1;
				}
				succession a then b;
			}
		}
		done;
		succession first start then iterate;
		succession first iterate then done;
	}
}
