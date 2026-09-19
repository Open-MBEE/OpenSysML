// A state machine a part exhibits in its own declaration names that part: an
// exhibited state is a performance the object owns, so `this` there is the
// object, and its entry behaviours write the object's features in state order.
package test {
	private import ScalarValues::*;

	part def Counter {
		attribute count : Integer = 0;
		attribute trail : String = "";

		exhibit state counting {
			entry; then alpha;

			state alpha {
				entry action bump {
					assign this.count := this.count + 1;
					assign this.trail := this.trail + "f";
				}
			}

			transition alpha_to_beta
				first alpha
				then beta;

			state beta {
				entry action bumpAgain {
					assign this.count := this.count + 1;
					assign this.trail := this.trail + "s";
				}
			}
		}
	}
}
