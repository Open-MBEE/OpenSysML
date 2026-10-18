// A guard written in a state's body names that state's attribute `x` (KerML 8.2.3.5),
// which nothing values: the read is no value, as it is when no other `x` is in
// reach, not the same-named machine attribute an enclosing frame holds.
package test {
	private import ScalarValues::*;

	state def Modes {
		attribute x : Integer = 7;
		attribute seen : Integer = 0;

		entry; then idle;
		state idle {
			attribute x : Integer;
			entry; then wait;
			state wait;
			state high;
			state low;
			transition first wait if x > 5 then high;
			transition first wait if x <= 5 then low;
		}
	}
}
