package test {
	private import ScalarValues::*;

	action def IfOrder {
		attribute x : Integer := 0;
		attribute y : Integer := 0;
		first start then s;
		// Oracle (docs/project/behavior-semantic-oracle.md): the `if` and the assignment after
		// it are unordered subactions, but the `if`'s guard precedes its branch.
		action s {
			if x == 0 {
				assign y := 10;
			}
			assign x := 1;
		}
		then done;
	}
}
