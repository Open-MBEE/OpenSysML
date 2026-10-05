package test {
	private import ScalarValues::*;

	action def Race {
		attribute c : Integer := 0;
		first start then a;
		// Oracle (docs/project/behavior-semantic-oracle.md): each performance of a
		// snapshots c into t at its start shot and assigns t + 1 when its assignment
		// ends; the other performance may run between the two, so an update is lost.
		action a[2] {
			attribute t : Integer := c;
			assign c := t + 1;
		}
		then done;
	}
}
