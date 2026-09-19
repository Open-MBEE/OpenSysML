// `terminate this;` in the do behavior of a state a part exhibits ends the part
// (SysML v2 §7.17.10; an exhibited state is a performance the object owns, so
// `this` there is the object), and with it every behavior the part performs: the
// machine is terminated where it stands, no state exited, rather than completing.
package test {
	private import ScalarValues::*;

	attribute def Never;

	part def Worker {
		attribute count : Integer = 0;
		attribute exits : Integer = 0;

		exhibit state cycle {
			entry; then busy;

			state busy {
				do action work {
					assign count := 1;
					terminate this;
					assign count := 2;
				}
				exit action leave {
					assign exits := 1;
				}
			}

			transition first busy accept Never then idle;
			state idle;
		}
	}
}
