// An Integer beyond int64 drives a state machine as any Integer does: the
// guard compares the exact value, and each assignment doubles it past int64.
package test {
	private import ScalarValues::*;

	state def Doubling {
		attribute n : Integer = 4611686018427387904;
		attribute doublings : Integer = 0;

		entry; then growing;
		state growing {
			entry action double {
				assign n := n * 2;
				assign doublings := doublings + 1;
			}
		}
		transition first growing if n < 2 ** 70 then growing;
		transition first growing if n >= 2 ** 70 then done;
	}
}
