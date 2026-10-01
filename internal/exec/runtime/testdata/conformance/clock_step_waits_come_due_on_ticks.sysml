// Under a clock stepping by 1.0 s (the case pins "clockStep": 1.0), a wait comes
// due at the first tick not before it ends: 2.3 s from 0 is read at 3.0, a
// further 0.4 s at 4.0 (not 3.4), and a wait ending on a tick stays there. On a
// continuous clock the same run would read 2.3, 2.7 and 4.7.
package test {
	private import ScalarValues::*;
	private import SI::*;

	action def Stepped {
		out t1 : Real;
		out t2 : Real;
		out t3 : Real;

		action w1 accept after 2.3 [s];
		then action r1 { assign t1 := localClock.currentTime; }
		then action w2 accept after 0.4 [s];
		then action r2 { assign t2 := localClock.currentTime; }
		then action w3 accept after 2.0 [s];
		then action r3 { assign t3 := localClock.currentTime; }
	}
}
