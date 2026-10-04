package BodyInterleavings {
	private import ScalarValues::*;

	part def Counter {
		attribute c : Integer = 0;
	}
	part counter : Counter;

	action def Inc {
		attribute t : Integer := counter.c;
		assign counter.c := t + 1;
	}

	action def Race {
		attribute c : Integer := 0;
		first start then a;
		action a[2] {
			attribute t : Integer := c;
			assign c := t + 1;
		}
		then done;
	}

	action def GuardBranch {
		attribute c : Integer := 0;
		first start then a;
		action a[2] {
			if c < 1 {
				assign c := c + 1;
			}
		}
		then done;
	}

	action def Callees {
		first start;
		then fork f;
			then a;
			then b;
		action a : Inc;
		action b {
			perform action pb : Inc;
		}
		succession a then j;
		succession b then j;
		join j;
		then done;
	}
}
