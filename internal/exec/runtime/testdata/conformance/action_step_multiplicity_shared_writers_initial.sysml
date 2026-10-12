package test {
	private import ScalarValues::*;

	action def SharedWriters {
		attribute c : Integer = 0;
		first start then a;
		// Each performance snapshots shared c in its own l; the final value depends on
		// how many snapshots happen before the last write.
		action a[3] {
			attribute l : Integer := c;
			first start then w;
			action w { assign c := l + 1; }
			then done;
		}
		then done;
	}
}
