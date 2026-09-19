// `accept after` in an action body parks the token on the shared clock until
// the clock has moved on by the delay, counted from the moment the accept is
// reached: the first wait ends at t=5, the second, reached then, at t=7.5.
package test {
	private import SI::*;
	private import ScalarValues::*;

	action timed {
		attribute count : Integer = 0;

		first start;
		then action waitA accept after 5 [s];
		then action tick assign count := count + 1;
		then action waitB accept after 2.5 [s];
		then action tock assign count := count * 10;
		then done;
	}
}
