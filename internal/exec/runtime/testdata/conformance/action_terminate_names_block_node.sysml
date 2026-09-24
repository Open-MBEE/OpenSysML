package test {
	private import ScalarValues::*;

	// `outer`, declared in an `if` branch, is a node of the branch's flow whose own
	// members are nodes in turn: a flow of their own, run one after another. The
	// `terminate outer;` in forked's forked branch ends outer where it stands: the
	// waiter is dropped, `later` never runs, and the branch goes on past outer.
	action host {
		out attribute inner : Integer = 0;
		out attribute skipped : Integer = 0;
		out attribute after : Integer = 0;

		action node {
			if true {
				action outer {
					action forked {
						fork split;
						action waiter accept sig : Integer;
						action quit { assign inner := 1; terminate outer; }
						first start;
						succession first start then split;
						succession first split then waiter;
						succession first split then quit;
						succession first waiter then done;
						succession first quit then done;
					}
					action later { assign skipped := 2; }
				}
				action next { assign after := 3; }
			}
		}
		succession first start then node;
		succession first node then done;
	}
}
