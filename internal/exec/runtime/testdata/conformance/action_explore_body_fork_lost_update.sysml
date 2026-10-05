package test {
	private import ScalarValues::*;

	action def ForkPlain {
		attribute c : Integer := 0;
		// Oracle (docs/project/behavior-semantic-oracle.md): two fork branches with one
		// leaf body each; either may start between the other's snapshot and its write.
		first start;
		then fork f;
			then a;
			then b;
		action a {
			attribute t : Integer := c;
			assign c := t + 1;
		}
		action b {
			attribute t : Integer := c;
			assign c := t + 1;
		}
		succession a then j;
		succession b then j;
		join j;
		then done;
	}
}
