// A note beside a behaviour a type binds describes the binding, not the
// behaviour: the object still runs the machine and the action the binding names.
// A behaviour written on its own writes its own features, which the object reads
// through the usage binding it.
package test {
	private import ScalarValues::*;

	state def Counting {
		attribute counted : Integer = 0;

		entry; then idle;

		state idle {
			entry action bump {
				assign counted := counted + 1;
			}
		}
	}

	action def Report {
		attribute logged : Boolean = false;

		first start;

		action log {
			assign logged := true;
		}

		succession first start then log;
	}

	part def Counter {
		exhibit state counting : Counting {
			doc /* how the counter counts */
		}

		perform action report : Report {
			doc /* what the counter reports */
		}

		attribute count : Integer = counting.counted;
		attribute reported : Boolean = report.logged;
	}
}
