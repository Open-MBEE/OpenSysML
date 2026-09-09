// A part whose type exhibits a state machine and owns an operation, so the REPL
// workflow after %instantiate has a machine to drive and an operation to invoke.
package Obj {
	part def Monitor {
		attribute count = 0;

		exhibit state modes {
			entry; then idle;
			state idle {
				entry action bump {
					assign count := count + 1;
				}
			}
			accept after 10 [SI::s] then awake;
			state awake {
				entry action mark {
					assign count := count + 10;
				}
			}
		}

		action bumpBy {
			in n;
			first apply;
			action apply {
				assign count := count + n;
			}
		}
	}
}
