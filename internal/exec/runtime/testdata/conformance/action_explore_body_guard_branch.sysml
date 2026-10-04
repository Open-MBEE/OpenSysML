package test {
	private import ScalarValues::*;

	action def GuardBranch {
		attribute c : Integer := 0;
		// Oracle (docs/project/behavior-semantic-oracle.md): the guard is read before the
		// branch's assignment; the other performance may read it too before either writes.
		first start then a;
		action a[2] {
			if c < 1 {
				assign c := c + 1;
			}
		}
		then done;
	}
}
