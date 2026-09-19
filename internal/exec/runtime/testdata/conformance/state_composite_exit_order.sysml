// Two levels of nesting left by one transition out of the outermost composite
// state: exit behavior runs innermost first, then the transition effect, then the
// target's entry. The digits of log pin that order.
package Test {
	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Outer {
			exit { assign log := log * 10 + 3; }
			state Middle {
				exit { assign log := log * 10 + 2; }
				state Inner {
					exit { assign log := log * 10 + 1; }
				}
			}
		}
		state Done {
			entry { assign log := log * 10 + 5; }
		}

		succession first start then Inner;
		transition first Outer when Abort do assign log := log * 10 + 4 then Done;
	}
}
