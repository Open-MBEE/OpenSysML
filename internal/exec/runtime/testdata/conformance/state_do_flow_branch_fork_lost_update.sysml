package test {
	private import ScalarValues::*;

	state def DoFlowBranchFork {
		attribute c : Integer := 0;
		// Oracle (docs/project/behavior-semantic-oracle.md): the `if` node of the do body's
		// stated flow forks two leaf bodies; either may start between the other's snapshot
		// and its write, as in a statement-list body.
		entry; then idle;
		state idle;
		state work {
			do action {
				assign c := 0;
				then if true {
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
		}
		state finished;

		succession first idle then work;
		succession first work then finished;
	}
}
