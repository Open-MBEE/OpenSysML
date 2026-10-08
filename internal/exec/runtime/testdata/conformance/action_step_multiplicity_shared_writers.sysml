package test {
	private import ScalarValues::*;

	action def SharedWriters {
		attribute c : Integer = 0;
		first start then a;
		// Each performance binds l to the current c; the repeated writes therefore
		// advance the shared value to 3 under every interleaving.
		action a[3] {
			attribute l : Integer = c;
			first start then w;
			action w { assign c := l + 1; }
			then done;
		}
		then done;
	}
}
