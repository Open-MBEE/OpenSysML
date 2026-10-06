package test {
	private import ScalarValues::*;

	part def Counter {
		attribute c : Integer = 0;
	}
	part counter : Counter;

	action def Inc {
		attribute t : Integer := counter.c;
		assign counter.c := t + 1;
	}

	action def PerformedCallees {
		attribute seen : Integer = 0;
		// Oracle (docs/project/behavior-semantic-oracle.md): each branch's body performs
		// Inc; either performance may start between the other's snapshot and its write.
		first start;
		then fork f;
			then a;
			then b;
		action a {
			perform action pa : Inc;
		}
		action b {
			perform action pb : Inc;
		}
		succession a then j;
		succession b then j;
		join j;
		then action look assign seen := counter.c;
		then done;
	}
}
