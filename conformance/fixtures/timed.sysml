package Timed {
	private import ScalarValues::*;
	action delayed {
		attribute count : Integer = 0;
		first start;
		then action wait accept after 5 [SI::s];
		then action tick assign count := count + 1;
		then done;
	}
}
