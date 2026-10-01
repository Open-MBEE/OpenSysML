package test {
	private import ScalarValues::*;

	action run {
		attribute total : Integer = 0;
		first start;
		action iterate {
			for i in 1..3 {
				if i > 1 {
					action d { assign total := total + e.v; }
					action e { out v : Integer; assign v := i * 10; }
					succession e then d;
				}
			}
		}
		done;
		succession first start then iterate;
		succession first iterate then done;
	}
}
