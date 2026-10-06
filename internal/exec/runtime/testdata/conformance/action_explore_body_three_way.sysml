package test {
	private import ScalarValues::*;

	action def Race3 {
		attribute c : Integer := 0;
		first start then a;
		// Oracle (docs/project/behavior-semantic-oracle.md): three performances of one
		// leaf body; how many snapshots precede the last assignment is open.
		action a[3] {
			attribute t : Integer := c;
			assign c := t + 1;
		}
		then done;
	}
}
