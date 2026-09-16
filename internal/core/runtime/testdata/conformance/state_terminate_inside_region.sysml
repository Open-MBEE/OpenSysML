// A transition in one region of an orthogonal state into a terminate action ends
// the whole machine: the sibling region's state is not exited (its exit behavior
// does not run) and its do behavior is abandoned with the terminating region's.
package Test {
	private import ScalarValues::*;

	attribute def Abort;
	attribute def Never;

	state def Machine parallel {
		attribute log : String = "";

		state left {
			entry; then l1;
			state l1 {
				entry { assign log := log + "l1(entry);"; }
				exit { assign log := log + "l1(exit);"; }
			}
			action stop terminate;
			transition first l1 accept Abort do { assign log := log + "abort(effect);"; } then stop;
		}

		state right {
			entry; then r1;
			state r1 {
				entry { assign log := log + "r1(entry);"; }
				do action hold {
					first start;
					then action h1 assign log := log + "hold;";
					then action wait accept Never;
					then action h2 assign log := log + "never;";
					then done;
				}
				exit { assign log := log + "r1(exit);"; }
			}
			state r2;
			transition first r1 accept Never then r2;
		}
	}
}
