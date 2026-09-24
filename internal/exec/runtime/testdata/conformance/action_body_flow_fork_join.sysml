package test {
	private import ScalarValues::*;

	action run {
		attribute total : Integer = 0;
		first start;
		action iterate {
			for i in 1..2 {
				action b { out v : Integer; assign v := i * 10; }
				action a { out v : Integer; assign v := i; }
				first f;
				fork f;
				succession f then a;
				succession f then b;
				join j;
				succession a then j;
				succession b then j;
				succession j then c;
				action c { assign total := total + a.v + b.v; }
			}
		}
		done;
		succession first start then iterate;
		succession first iterate then done;
	}
}
