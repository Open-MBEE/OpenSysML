// A transition inside a composite state into a terminate action the composite
// declares leaves the substate as any transition does (its exit behavior runs),
// performs its effect, then ends the whole machine's performance: the composite
// itself is not exited and its do behavior is abandoned where it waits.
package Test {
	private import ScalarValues::*;

	attribute def Abort;
	attribute def Never;

	state def Machine {
		attribute log : String = "";

		entry; then work;
		state work {
			entry { assign log := log + "work(entry);"; }
			do action watch {
				first start;
				then action w1 assign log := log + "watch;";
				then action wait accept Never;
				then action w2 assign log := log + "never;";
				then done;
			}
			exit { assign log := log + "work(exit);"; }

			entry; then inner;
			state inner {
				entry { assign log := log + "inner(entry);"; }
				exit { assign log := log + "inner(exit);"; }
			}
			action stop terminate;

			transition first inner accept Abort do { assign log := log + "abort(effect);"; } then stop;
		}
		state after;
		transition first work accept Never then after;
	}
}
