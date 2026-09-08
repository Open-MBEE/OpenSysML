// `accept at` waits for an absolute instant of the shared clock. An instant the
// clock has already passed is not waited for: the clock advances monotonically,
// so the accept ends at once, and the clock stays where it is.
package test {
	private import SI::*;
	private import Time::*;
	private import ScalarValues::*;

	action scheduled {
		attribute count : Integer = 0;
		attribute later : TimeInstantValue = 3 [s];
		attribute earlier : TimeInstantValue = 1 [s];
		attribute latest : TimeInstantValue = 8 [s];

		first start;
		then action waitLater accept at later;
		then action tick assign count := count + 1;
		then action waitEarlier accept at earlier;
		then action tock assign count := count + 10;
		then action waitLatest accept at latest;
		then action tack assign count := count + 100;
		then done;
	}
}
